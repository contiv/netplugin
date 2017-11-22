package networkpolicy

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/contivmodel/client"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/netmaster/objApi"
	"github.com/contiv/netplugin/netmaster/resources"
	"github.com/contiv/netplugin/objdb"
	"github.com/contiv/netplugin/state"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/ofnet"
	etcdclient "github.com/coreos/etcd/client"
	"github.com/gorilla/mux"
	"golang.org/x/net/context"
	"hash/fnv"
	"net/http"
	"os"
	"testing"
	"time"
)

var k8sut k8sContext
var nptLog *log.Entry

// assert on true & print error message
func assertOnTrue(t *testing.T, val bool, msg string) {
	if val {
		nptLog.Errorf("%s", msg)
		t.FailNow()
	}
	// else continue
}

func TestUpdateDefaultIngressPolicy(t *testing.T) {
	tData := []struct {
		ns     string
		action string
	}{
		{"default", "allow"},
		{"default", "none"},
		{"default", "deny"},
		{"default", "none"},
		{"default", "allow"},
		{"default", "deny"},
		{"default", "deny"},
		{"default", "none"},
		{"tenant101", "deny"},
		{"tenant101", "allow"},
		{"tenant101", "deny"},
		{"default", "deny"},
		{"default", "deny"},
	}
	for _, d := range tData {

		if d.action == "none" {
			k8sut.deleteDefaultIngressPolicy(d.ns)
			_, err := k8sut.contivClient.RuleGet(defaultTenantName, d.ns+"-"+defaultPolicyName, defaultRuleID)
			assertOnTrue(t, err == nil, fmt.Sprintf("rule %s not deleted !", defaultRuleID))
			continue
		}
		if d.ns != "default" {
			h := fnv.New32a()
			h.Write([]byte(d.ns + "-" + defaultNetworkName))

			if _, err := k8sut.contivClient.NetworkGet(defaultTenantName, d.ns+"-"+defaultNetworkName); err != nil {
				err := k8sut.contivClient.NetworkPost(&client.Network{
					TenantName:  defaultTenantName,
					NetworkName: d.ns + "-" + defaultNetworkName,
					Subnet:      fmt.Sprintf("10.36.%d.0/24", h.Sum32()%17),
					Encap:       "vxlan",
				})
				assertOnTrue(t, err != nil, fmt.Sprintf("failed to create network %s", err))
			}
		}
		k8sut.updateDefaultIngressPolicy(d.ns, d.action)
		val, err := k8sut.contivClient.RuleGet(defaultTenantName, d.ns+"-"+defaultPolicyName, defaultRuleID)
		assertOnTrue(t, err != nil, fmt.Sprintf("failed to get rule %s, %s", defaultRuleID, err))
		assertOnTrue(t, val.Action != d.action, fmt.Sprintf("expected action [%s], got [%s]", d.action, val.Action))

	}
}

func TestGetIsolationPolicy(t *testing.T) {
	tData := []struct {
		annotation string
		policy     string
	}{
		{"", "none"},
		{"{\"ingress\":\"ABC\"}", "allow"},
		{"{\"ingress\":{\"isolation\":\"DEF\"}}", "allow"},
		{"{\"ingress\":{\"isolation\":\"DefaultDeny\"}}", "deny"},
		{"{\"ingress\":{\"isolation\":\"DefaultAllow\"}}", "allow"},
	}

	for _, d := range tData {
		np := make(map[string]string)
		if len(d.annotation) > 0 {
			np["net.beta.kubernetes.io/network-policy"] = d.annotation
		}
		retPol := k8sut.getIsolationPolicy(np)
		assertOnTrue(t, retPol != d.policy, fmt.Sprintf("received [%s] expected %+v", retPol, d))
	}
}

const (
	netmasterTestURL       = "http://localhost:9230"
	netmasterTestListenURL = ":9230"
)

// initStateDriver initialize etcd state driver
func initStateDriver() (core.StateDriver, error) {
	instInfo := core.InstanceInfo{DbURL: "etcd://127.0.0.1:2379"}

	return utils.NewStateDriver(utils.EtcdNameStr, &instInfo)
}

// cleanupState cleans up default tenant and other global state
func cleanupState() {

	if el, err := contivClient.EndpointGroupList(); err == nil {
		for _, e := range *el {
			nptLog.Infof("cleanup epg %s", e.GroupName)
			contivClient.EndpointGroupDelete(e.TenantName, e.GroupName)
		}
	}
	if pl, err := contivClient.PolicyList(); err == nil {
		for _, p := range *pl {
			nptLog.Infof("cleanup policy %s", p.PolicyName)
			contivClient.PolicyDelete(p.TenantName, p.PolicyName)
		}
	}
	if nl, err := contivClient.NetworkList(); err == nil {
		for _, p := range *nl {
			nptLog.Infof("cleanup network %s", p.NetworkName)
			contivClient.NetworkDelete(p.TenantName, p.NetworkName)
		}
	}
	// delete default tenant
	err := contivClient.TenantDelete("default")
	if err != nil {
		nptLog.Fatalf("Error deleting default tenant. Err: %v", err)
	}

	// clear global state
	err = contivClient.GlobalDelete("global")
	if err != nil {
		nptLog.Fatalf("Error deleting global state. Err: %v", err)
	}
}

var contivClient *client.ContivClient
var stateStore core.StateDriver

func TestMain(m *testing.M) {
	var err error

	npLog = log.WithField("k8s", "netpolicy")
	nptLog = log.WithField("k8sTest", "netpolicy")

	// Setup state store
	stateStore, err = initStateDriver()
	if err != nil {
		nptLog.Fatalf("Error initializing state store. Err: %v", err)
	}
	// little hack to clear all state from etcd
	stateStore.(*state.EtcdStateDriver).KeysAPI.Delete(context.Background(), "/contiv.io", &etcdclient.DeleteOptions{Recursive: true})

	// Setup resource manager
	if _, err = resources.NewStateResourceManager(stateStore); err != nil {
		nptLog.Fatalf("Failed to init resource manager. Error: %s", err)
	}

	router := mux.NewRouter()

	// create objdb client
	objdbClient, err := objdb.NewClient("etcd://127.0.0.1:2379")
	if err != nil {
		nptLog.Fatalf("Error connecting to state store: etcd://127.0.0.1:2379. Err: %v", err)
	}

	// Create a new api controller
	apiConfig := &objApi.APIControllerConfig{
		NetForwardMode: "bridge",
		NetInfraType:   "default",
	}
	if apiController := objApi.NewAPIController(router, objdbClient, apiConfig); apiController == nil {
		nptLog.Fatalf("failed to create api controller")
	}

	ofnetMaster := ofnet.NewOfnetMaster("127.0.0.1", ofnet.OFNET_MASTER_PORT)
	if ofnetMaster == nil {
		nptLog.Fatalf("Error creating ofnet master")
	}

	// initialize policy manager
	mastercfg.InitPolicyMgr(stateStore, ofnetMaster)

	// Create HTTP server
	go http.ListenAndServe(netmasterTestListenURL, router)

	time.Sleep(time.Second)

	// create a new contiv client
	contivClient, err = client.NewContivClient(netmasterTestURL)
	if err != nil {
		nptLog.Fatalf("Error creating contiv client. Err: %v", err)
	}

	k8sut.contivClient = contivClient
	k8sut.isLeader = func() bool { return true }
	exitCode := m.Run()
	if exitCode == 0 {
		cleanupState()
	}
	os.Exit(exitCode)
}
