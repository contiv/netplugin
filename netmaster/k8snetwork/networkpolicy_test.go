package networkpolicy

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/contiv/client-go/pkg/api/v1"
	"github.com/contiv/client-go/pkg/apis/extensions/v1beta1"
	metav1 "github.com/contiv/client-go/pkg/apis/meta/v1"
	"github.com/contiv/client-go/pkg/util/intstr"
	"github.com/contiv/contivmodel/client"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/netmaster/objApi"
	"github.com/contiv/netplugin/netmaster/resources"
	"github.com/contiv/netplugin/state"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/netplugin/utils/k8sutils"
	"github.com/contiv/objdb"
	"github.com/contiv/ofnet"
	etcdclient "github.com/coreos/etcd/client"
	"github.com/gorilla/mux"
	"golang.org/x/net/context"
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
	err := contivClient.TenantDelete(defaultTenantName)
	if err != nil {
		nptLog.Fatalf("Error deleting default tenant. Err: %v", err)
	}

	// clear global state
	err = contivClient.GlobalDelete("global")
	if err != nil {
		nptLog.Fatalf("Error deleting global state. Err: %v", err)
	}
}

func TestK8sNpPolicy(t *testing.T) {
	tData := []struct {
		tenantName string
		epgName    string
		status     bool
	}{
		{defaultTenantName, "epg1", true},
		{defaultTenantName, "epg2", true},
		{"noTenant", "epg3", false},
		{defaultTenantName, "epg2", true},
		{"newTenant", "newEpg1", true},
	}

	// create
	for _, d := range tData {
		err := k8sut.createPolicy(d.tenantName, d.epgName)
		if d.status {
			assertOnTrue(t, err != nil, fmt.Sprintf("failed to create policy %+v", d))
			_, err := k8sut.contivClient.PolicyGet(d.tenantName, k8sutils.EpgNameToPolicy(d.epgName))
			assertOnTrue(t, err != nil, fmt.Sprintf("failed to verify policy %+v", d))
		} else {
			assertOnTrue(t, err == nil, fmt.Sprintf("created policy %+v !", d))
		}
	}

	// delete
	for _, d := range tData {
		err := k8sut.deletePolicy(d.tenantName, d.epgName)
		if d.status {
			assertOnTrue(t, err != nil, fmt.Sprintf("failed to delete policy %+v", d))
			_, err := k8sut.contivClient.PolicyGet(d.tenantName, k8sutils.EpgNameToPolicy(d.epgName))
			assertOnTrue(t, err == nil, fmt.Sprintf("policy %+v exists after delete operation", d))
		} else {
			assertOnTrue(t, err != nil, fmt.Sprintf("deleted non-existing policy %+v !", d))
		}
	}
}

func TestK8sNpDefaultPolicy(t *testing.T) {
	tData := []struct {
		tenantName string
		epgName    string
		status     bool
	}{
		{defaultTenantName, "epg1", true},
		{defaultTenantName, "epg2", true},
		{"noTenant", "epg3", false},
		{defaultTenantName, "epg2", true},
		{"newTenant", "newEpg1", true},
	}

	// create
	for _, d := range tData {

		policyName := k8sutils.EpgNameToPolicy(d.epgName)
		err := k8sut.createDefaultPolicy(d.tenantName, d.epgName)
		if d.status {
			assertOnTrue(t, err != nil, fmt.Sprintf("failed to create default policy %+v", d))
			_, err := k8sut.contivClient.PolicyGet(d.tenantName, k8sutils.EpgNameToPolicy(d.epgName))
			assertOnTrue(t, err != nil, fmt.Sprintf("failed to verify default policy %+v", d))
			for _, dir := range []string{"in", "out"} {
				_, err := k8sut.contivClient.RuleGet(d.tenantName,
					policyName,
					k8sutils.DenyAllRuleID+dir)
				assertOnTrue(t, err != nil, fmt.Sprintf("failed to verify rules %+v, error :%s !",
					d, err))
			}
		} else {
			assertOnTrue(t, err == nil, fmt.Sprintf("created default policy %+v !", d))
		}
	}

	// delete
	for _, d := range tData {
		err := k8sut.deletePolicy(d.tenantName, d.epgName)
		if d.status {
			assertOnTrue(t, err != nil, fmt.Sprintf("failed to delete default policy %+v", d))
			_, err := k8sut.contivClient.PolicyGet(d.tenantName, k8sutils.EpgNameToPolicy(d.epgName))
			assertOnTrue(t, err == nil, fmt.Sprintf("default policy %+v exists after delete operation", d))
		} else {
			assertOnTrue(t, err != nil, fmt.Sprintf("deleted non-existing default policy %+v !", d))
		}
	}
}

func TestK8sNpEpg(t *testing.T) {
	tData := []struct {
		TenantName  string
		NetworkName string
		GroupName   string
		status      bool
	}{
		{TenantName: defaultTenantName, NetworkName: defaultNetworkName, GroupName: "epg1", status: true},
		{TenantName: "newTenant", NetworkName: "newNet", GroupName: "epg2", status: true},
		{TenantName: defaultTenantName, NetworkName: "no-net", GroupName: "epg3", status: false},
		{TenantName: "noTenant", NetworkName: "newNet", GroupName: "epg4", status: false},
	}

	err := k8sut.createPolicy(defaultTenantName, "epg1")
	assertOnTrue(t, err != nil, fmt.Sprintf("failed to create default: policy"))

	err = k8sut.createPolicy("newTenant", "epg2")
	assertOnTrue(t, err != nil, fmt.Sprintf("failed to create nweTenant: policy"))

	// create
	for _, d := range tData {
		err := k8sut.createEpg(d.TenantName, d.NetworkName, d.GroupName)
		if d.status {
			assertOnTrue(t, err != nil, fmt.Sprintf("failed to create epg %+v", d))
			_, err := k8sut.contivClient.EndpointGroupGet(d.TenantName, d.GroupName)
			assertOnTrue(t, err != nil, fmt.Sprintf("failed to verify epg %+v", d))
		} else {
			assertOnTrue(t, err == nil, fmt.Sprintf("created epg %+v !", d))
		}
	}

	// delete
	for _, d := range tData {
		err := k8sut.deleteEpg(d.TenantName, d.GroupName)
		if d.status {
			assertOnTrue(t, err != nil, fmt.Sprintf("failed to delete epg %+v", d))
			_, err := k8sut.contivClient.EndpointGroupGet(d.TenantName, d.GroupName)
			assertOnTrue(t, err == nil, fmt.Sprintf("epg %+v exists after delete operation", d))
		} else {
			assertOnTrue(t, err != nil, fmt.Sprintf("deleted non-existing epg %+v !", d))
		}
	}

	err = k8sut.deletePolicy(defaultTenantName, "epg1")
	assertOnTrue(t, err != nil, fmt.Sprintf("failed to delete default: policy"))

	err = k8sut.deletePolicy("newTenant", "epg2")
	assertOnTrue(t, err != nil, fmt.Sprintf("failed to delete nweTenant: policy"))
}

func TestK8sNpRules(t *testing.T) {
	tData := []struct {
		rule   client.Rule
		status bool
	}{
		{client.Rule{
			TenantName: defaultTenantName,
			PolicyName: k8sutils.EpgNameToPolicy("pol1"),
			RuleID:     "101",
			Protocol:   "udp",
			Port:       400,
			Direction:  "in",
			Action:     "deny",
		}, true},
		{client.Rule{
			TenantName: defaultTenantName,
			PolicyName: k8sutils.EpgNameToPolicy("pol1"),
			RuleID:     "102",
			Protocol:   "tcp",
			Port:       400,
			Direction:  "out",
			Action:     "deny",
		},
			true},
		{client.Rule{
			TenantName: "newTenant",
			PolicyName: k8sutils.EpgNameToPolicy("pol2"),
			RuleID:     "201",
			Protocol:   "udp",
			Port:       500,
			Direction:  "in",
			Action:     "deny",
		}, true},
		{client.Rule{
			TenantName: "newTenant",
			PolicyName: k8sutils.EpgNameToPolicy("pol2"),
			RuleID:     "202",
			Protocol:   "tcp",
			Port:       500, Direction: "in",
			Action: "deny",
		}, true},
		{client.Rule{
			TenantName: defaultTenantName,
			PolicyName: k8sutils.EpgNameToPolicy("nopolicy"),
			RuleID:     "101",
			Protocol:   "udp", Port: 400,
			Direction: "in",
			Action:    "deny",
		}, false},
		{client.Rule{
			TenantName: "abcd",
			PolicyName: k8sutils.EpgNameToPolicy("pol1"),
			RuleID:     "101", Protocol: "udp",
			Port:      400,
			Direction: "in",
			Action:    "deny",
		}, false},
	}

	err := k8sut.createPolicy(defaultTenantName, "pol1")
	assertOnTrue(t, err != nil, fmt.Sprintf("failed to create default: policy"))

	err = k8sut.createPolicy("newTenant", "pol2")
	assertOnTrue(t, err != nil, fmt.Sprintf("failed to create nweTenant: policy"))

	// create
	for _, d := range tData {
		err := k8sut.createRule(&d.rule)
		if d.status {
			assertOnTrue(t, err != nil, fmt.Sprintf("failed to create rule %+v", d))
			_, err := k8sut.contivClient.RuleGet(d.rule.TenantName, d.rule.PolicyName, d.rule.RuleID)
			assertOnTrue(t, err != nil, fmt.Sprintf("failed to verify rule %+v", d))
		} else {
			assertOnTrue(t, err == nil, fmt.Sprintf("created rule %+v !", d))
		}
	}

	// delete
	for _, d := range tData {
		err := k8sut.deleteRule(d.rule.TenantName, d.rule.PolicyName, d.rule.RuleID)
		if d.status {
			assertOnTrue(t, err != nil, fmt.Sprintf("failed to delete rule %+v", d))
			_, err := k8sut.contivClient.RuleGet(d.rule.TenantName, d.rule.PolicyName, d.rule.RuleID)
			assertOnTrue(t, err == nil, fmt.Sprintf("rule %+v exists after delete operation", d))
		} else {
			assertOnTrue(t, err != nil, fmt.Sprintf("deleted non-existing rule %+v !", d))
		}
	}
	err = k8sut.deletePolicy(defaultTenantName, "epg1")
	assertOnTrue(t, err != nil, fmt.Sprintf("failed to delete default: policy"))

	err = k8sut.deletePolicy("newTenant", "epg2")
	assertOnTrue(t, err != nil, fmt.Sprintf("failed to delete nweTenant: policy"))
}

func TestK8sNpPodSelector(t *testing.T) {
	dTest := []struct {
		label  map[string]string
		sel    k8sPodSelector
		status bool
	}{
		{
			label: map[string]string{k8sutils.K8sTenantLabel: defaultTenantName,
				k8sutils.K8sNetworkLabel: defaultNetworkName,
				k8sutils.K8sGroupLabel:   "epg1"},
			sel:    k8sPodSelector{TenantName: defaultTenantName, NetworkName: defaultNetworkName, GroupName: "epg1"},
			status: true,
		},
		{
			label: map[string]string{
				k8sutils.K8sNetworkLabel: defaultNetworkName,
				k8sutils.K8sGroupLabel:   "epg1"},
			sel:    k8sPodSelector{TenantName: defaultTenantName, NetworkName: defaultNetworkName, GroupName: "epg1"},
			status: true,
		},
		{
			label: map[string]string{
				k8sutils.K8sGroupLabel: "epg1"},
			sel:    k8sPodSelector{TenantName: defaultTenantName, NetworkName: defaultNetworkName, GroupName: "epg1"},
			status: true,
		},
		{
			label:  map[string]string{},
			status: false,
		},
		{
			label: map[string]string{k8sutils.K8sTenantLabel: "newTenant",
				k8sutils.K8sNetworkLabel: "newNet",
				k8sutils.K8sGroupLabel:   "epg1"},
			sel:    k8sPodSelector{TenantName: "newTenant", NetworkName: "newNet", GroupName: "epg1"},
			status: true,
		},
		{
			label: map[string]string{k8sutils.K8sTenantLabel: "noTenant",
				k8sutils.K8sNetworkLabel: "newNet",
				k8sutils.K8sGroupLabel:   "epg1"},
			status: false,
		},
		{
			label: map[string]string{k8sutils.K8sTenantLabel: defaultTenantName,
				k8sutils.K8sNetworkLabel: "noNetwork",
				k8sutils.K8sGroupLabel:   "epg1"},
			status: false,
		},
		{
			label: map[string]string{k8sutils.K8sTenantLabel: defaultTenantName,
				k8sutils.K8sNetworkLabel: defaultNetworkName,
			},
			status: false,
		},
	}

	for _, d := range dTest {
		p, err := k8sut.getPodSelector(d.label)
		if d.status {
			assertOnTrue(t, err != nil, fmt.Sprintf("failed to get pod selector for %+v", d))
			assertOnTrue(t, p.TenantName != d.sel.TenantName,
				fmt.Sprintf("failed to get tenant for %+v, %+v", d, p))
			assertOnTrue(t, p.NetworkName != d.sel.NetworkName,
				fmt.Sprintf("failed to get network for %+v, %+v", d, p))
			assertOnTrue(t, p.GroupName != d.sel.GroupName,
				fmt.Sprintf("failed to get group for %+v, %+v", d, p))
		} else {
			assertOnTrue(t, err == nil, fmt.Sprintf("for pod selector for %+v, %+v !", d, p))
		}
	}
}

func TestK8sNpGetPolicyPorts(t *testing.T) {
	tcp := v1.ProtocolTCP
	udp := v1.ProtocolUDP

	intToPort := func(port int) *intstr.IntOrString {
		v := intstr.FromInt(port)
		return &v
	}

	dTest := []struct {
		pol []v1beta1.NetworkPolicyPort
		ps  []k8sPolicyPorts
	}{
		{
			pol: []v1beta1.NetworkPolicyPort{{Protocol: &tcp, Port: intToPort(505)}},
			ps:  []k8sPolicyPorts{{Protocol: "TCP", Port: 505}},
		},
		{
			pol: []v1beta1.NetworkPolicyPort{{Protocol: &tcp, Port: intToPort(405)}, {Protocol: &udp, Port: intToPort(605)}},
			ps:  []k8sPolicyPorts{{Protocol: "TCP", Port: 405}, {Protocol: "UDP", Port: 605}},
		},
		{
			pol: []v1beta1.NetworkPolicyPort{{Protocol: &tcp}},
			ps:  []k8sPolicyPorts{{Protocol: "TCP", Port: 0}},
		},
		{
			pol: []v1beta1.NetworkPolicyPort{{Port: intToPort(1505)}},
			ps:  []k8sPolicyPorts{{Protocol: "TCP", Port: 1505}},
		},
		{
			pol: []v1beta1.NetworkPolicyPort{{}},
			ps:  []k8sPolicyPorts{{Protocol: "TCP", Port: 0}},
		},
	}

	for _, d := range dTest {
		p := k8sut.getPolicyPorts(d.pol)
		assertOnTrue(t, len(p) != len(d.ps), fmt.Sprintf("policy port didn't match %+v, %+v", d, p))
		for i, v := range d.ps {
			assertOnTrue(t, v.Protocol != p[i].Protocol, fmt.Sprintf("protocol didn't match %+v, %+v", v, p[i]))
			assertOnTrue(t, v.Port != p[i].Port, fmt.Sprintf("port didn't match %+v, %+v", v, p[i]))
		}
	}
}

func TestK8sNpGetIngressPodSelectorList(t *testing.T) {
	dTest := []struct {
		label  []map[string]string
		sel    []k8sPodSelector
		status bool
	}{
		{
			label: []map[string]string{
				{
					k8sutils.K8sTenantLabel:  defaultTenantName,
					k8sutils.K8sNetworkLabel: defaultNetworkName,
					k8sutils.K8sGroupLabel:   "epg1",
				},
				{
					k8sutils.K8sTenantLabel:  defaultTenantName,
					k8sutils.K8sNetworkLabel: defaultNetworkName,
					k8sutils.K8sGroupLabel:   "epg2",
				},
			},
			sel: []k8sPodSelector{
				{
					TenantName:  defaultTenantName,
					NetworkName: defaultNetworkName,
					GroupName:   "epg1",
				},
				{
					TenantName:  defaultTenantName,
					NetworkName: defaultNetworkName,
					GroupName:   "epg2",
				},
			},
			status: true,
		},
		{
			label: []map[string]string{
				{
					k8sutils.K8sTenantLabel:  defaultTenantName,
					k8sutils.K8sNetworkLabel: defaultNetworkName,
					k8sutils.K8sGroupLabel:   "epg1",
				},
				{
					k8sutils.K8sTenantLabel:  defaultTenantName,
					k8sutils.K8sNetworkLabel: defaultNetworkName,
					k8sutils.K8sGroupLabel:   "epg4",
				},
				{
					k8sutils.K8sTenantLabel:  defaultTenantName,
					k8sutils.K8sNetworkLabel: defaultNetworkName,
					k8sutils.K8sGroupLabel:   "epg7",
				},
			},
			sel: []k8sPodSelector{
				{
					TenantName:  defaultTenantName,
					NetworkName: defaultNetworkName,
					GroupName:   "epg1",
				},
				{
					TenantName:  defaultTenantName,
					NetworkName: defaultNetworkName,
					GroupName:   "epg4",
				},
				{
					TenantName:  defaultTenantName,
					NetworkName: defaultNetworkName,
					GroupName:   "epg7",
				},
			},
			status: true,
		},
		{
			label: []map[string]string{
				{
					k8sutils.K8sTenantLabel:  defaultTenantName,
					k8sutils.K8sNetworkLabel: defaultNetworkName,
					k8sutils.K8sGroupLabel:   "epg3",
				},
				{
					k8sutils.K8sTenantLabel:  "no-tenant",
					k8sutils.K8sNetworkLabel: defaultNetworkName,
					k8sutils.K8sGroupLabel:   "epg6",
				},
			},
			status: false,
		},
		{
			label: []map[string]string{
				{
					k8sutils.K8sTenantLabel:  defaultTenantName,
					k8sutils.K8sNetworkLabel: defaultNetworkName,
					k8sutils.K8sGroupLabel:   "epg1",
				},
				{
					k8sutils.K8sTenantLabel:  defaultTenantName,
					k8sutils.K8sNetworkLabel: "no-net",
					k8sutils.K8sGroupLabel:   "epg9",
				},
			},

			status: false,
		},
		{
			label: []map[string]string{
				{
					k8sutils.K8sTenantLabel:  defaultTenantName,
					k8sutils.K8sNetworkLabel: defaultNetworkName,
					k8sutils.K8sGroupLabel:   "epg1",
				},
				{
					k8sutils.K8sTenantLabel:  defaultTenantName,
					k8sutils.K8sNetworkLabel: defaultNetworkName,
				},
			},
			status: false,
		},
	}

	for _, d := range dTest {
		nwPeers := make([]v1beta1.NetworkPolicyPeer, len(d.label))
		for i, l := range d.label {
			nwPeers[i].PodSelector = &metav1.LabelSelector{MatchLabels: l}
		}

		l, err := k8sut.getIngressPodSelectorList(nwPeers)
		if d.status {
			assertOnTrue(t, err != nil, fmt.Sprintf("failed to get podselector %+v, %s", nwPeers, err))
			assertOnTrue(t, len(l) != len(nwPeers), fmt.Sprintf("podselector didn't match%+v, %+v",
				nwPeers, l))
			for i, kp := range d.sel {
				assertOnTrue(t, kp.TenantName != l[i].TenantName,
					fmt.Sprintf("tenant didn't match %+v, %+v", kp, l[i]))
				assertOnTrue(t, kp.NetworkName != l[i].NetworkName,
					fmt.Sprintf("network didn't match %+v, %+v", kp, l[i]))
				assertOnTrue(t, kp.GroupName != l[i].GroupName,
					fmt.Sprintf("group didn't match %+v, %+v", kp, l[i]))
			}
		} else {
			assertOnTrue(t, err == nil, fmt.Sprintf("got podselector %+v, %v", d, nwPeers))
		}
	}

}

func TestK8sNpGetIngressPolicy(t *testing.T) {
	tcp := v1.ProtocolTCP
	udp := v1.ProtocolUDP

	type k8sIngressTest struct {
		// pod selector
		label []map[string]string
		sel   []k8sPodSelector
		// ports
		pol []v1beta1.NetworkPolicyPort
		ps  []k8sPolicyPorts
	}

	intToPort := func(port int) *intstr.IntOrString {
		v := intstr.FromInt(port)
		return &v
	}

	testDataSuccess := []k8sIngressTest{
		{
			label: []map[string]string{
				{
					k8sutils.K8sTenantLabel:  defaultTenantName,
					k8sutils.K8sNetworkLabel: defaultNetworkName,
					k8sutils.K8sGroupLabel:   "epg1",
				},
				{
					k8sutils.K8sTenantLabel:  defaultTenantName,
					k8sutils.K8sNetworkLabel: defaultNetworkName,
					k8sutils.K8sGroupLabel:   "epg2",
				},
			},

			sel: []k8sPodSelector{
				{
					TenantName:  defaultTenantName,
					NetworkName: defaultNetworkName,
					GroupName:   "epg1",
				},
				{
					TenantName:  defaultTenantName,
					NetworkName: defaultNetworkName,
					GroupName:   "epg2",
				},
			},

			pol: []v1beta1.NetworkPolicyPort{
				{Protocol: &tcp, Port: intToPort(505)},
				{Protocol: &udp, Port: intToPort(905)},
				{Protocol: &udp, Port: intToPort(5001)},
			},
			ps: []k8sPolicyPorts{
				{Protocol: "TCP", Port: 505},
				{Protocol: "UDP", Port: 905},
				{Protocol: "UDP", Port: 5001},
			},
		},
	}

	testDataEmptyPort := []k8sIngressTest{
		{
			label: []map[string]string{
				{
					k8sutils.K8sTenantLabel:  defaultTenantName,
					k8sutils.K8sNetworkLabel: defaultNetworkName,
					k8sutils.K8sGroupLabel:   "epg1",
				},
				{
					k8sutils.K8sTenantLabel:  defaultTenantName,
					k8sutils.K8sNetworkLabel: defaultNetworkName,
					k8sutils.K8sGroupLabel:   "epg2",
				},
			},

			sel: []k8sPodSelector{
				{
					TenantName:  defaultTenantName,
					NetworkName: defaultNetworkName,
					GroupName:   "epg1",
				},
				{
					TenantName:  defaultTenantName,
					NetworkName: defaultNetworkName,
					GroupName:   "epg2",
				},
			},
		},
	}

	testDataEmptyPodSelector := []k8sIngressTest{
		{
			pol: []v1beta1.NetworkPolicyPort{
				{Protocol: &tcp, Port: intToPort(505)},
				{Protocol: &udp, Port: intToPort(905)},
				{Protocol: &udp, Port: intToPort(5001)},
			},
			ps: []k8sPolicyPorts{
				{Protocol: "TCP", Port: 505},
				{Protocol: "UDP", Port: 905},
				{Protocol: "UDP", Port: 5001},
			},
		},
	}

	testDataMissingPodSelector := []k8sIngressTest{
		{
			label: []map[string]string{
				{
					k8sutils.K8sTenantLabel:  defaultTenantName,
					k8sutils.K8sNetworkLabel: defaultNetworkName,
					k8sutils.K8sGroupLabel:   "epg1",
				},
				{
					k8sutils.K8sTenantLabel:  defaultTenantName,
					k8sutils.K8sNetworkLabel: defaultNetworkName,
				},
			},

			pol: []v1beta1.NetworkPolicyPort{
				{Protocol: &tcp, Port: intToPort(505)},
				{Protocol: &udp, Port: intToPort(905)},
				{Protocol: &udp, Port: intToPort(5001)},
			},
			ps: []k8sPolicyPorts{
				{Protocol: "TCP", Port: 505},
				{Protocol: "UDP", Port: 905},
				{Protocol: "UDP", Port: 5001},
			},
		},
	}

	testDataList := []struct {
		data   []k8sIngressTest
		status bool
	}{
		{testDataSuccess, true},
		{testDataEmptyPort, false},
		{testDataEmptyPodSelector, false},
		{testDataMissingPodSelector, false},
	}

	for _, dataSet := range testDataList {
		npIngress := []v1beta1.NetworkPolicyIngressRule{}
		for _, d := range dataSet.data {
			nwPeers := []v1beta1.NetworkPolicyPeer{}
			for _, l := range d.label {
				nwPeers = append(nwPeers, v1beta1.NetworkPolicyPeer{PodSelector: &metav1.LabelSelector{MatchLabels: l}})
			}
			npIngress = append(npIngress, v1beta1.NetworkPolicyIngressRule{Ports: d.pol, From: nwPeers})
		}
		k8spolicy, err := k8sut.getIngressPolicy(npIngress)
		if dataSet.status {
			assertOnTrue(t, err != nil, fmt.Sprintf("failed to get ingress policy %+v, error: %s",
				npIngress, err))

			assertOnTrue(t, len(k8spolicy) != len(dataSet.data), fmt.Sprintf("mismatch in ingress policy %+v, %+v",
				k8spolicy, dataSet.data))
			for i, d := range dataSet.data {

				assertOnTrue(t, len(d.sel) != len(k8spolicy[i].IngressPodSelector),
					fmt.Sprintf("mismatch in portselector %+v, %+v", d.sel,
						k8spolicy[i].IngressPodSelector))
				for j, c := range d.sel {
					assertOnTrue(t, c.GroupName != k8spolicy[i].IngressPodSelector[j].GroupName,
						fmt.Sprintf("mismatch in group  %+v, %+v", c,
							k8spolicy[i].IngressPodSelector[j]))
					assertOnTrue(t, c.TenantName != k8spolicy[i].IngressPodSelector[j].TenantName,
						fmt.Sprintf("mismatch in tenant  %+v, %+v", c,
							k8spolicy[i].IngressPodSelector[j]))
					assertOnTrue(t, c.NetworkName != k8spolicy[i].IngressPodSelector[j].NetworkName,
						fmt.Sprintf("mismatch in network %+v, %+v", c,
							k8spolicy[i].IngressPodSelector[j]))
				}

				assertOnTrue(t, len(d.ps) != len(k8spolicy[i].IngressRules),
					fmt.Sprintf("mismatch in policy ports  %+v, %+v", d.ps, k8spolicy[i].IngressRules))
				for j, c := range d.ps {
					assertOnTrue(t, c.Port != k8spolicy[i].IngressRules[j].Port,
						fmt.Sprintf("mismatch in port  %+v,  %+v",
							c, k8spolicy[i].IngressRules[j]))
					assertOnTrue(t, c.Protocol != k8spolicy[i].IngressRules[j].Protocol,
						fmt.Sprintf("mismatch in protocol  %+v, %+v", c,
							k8spolicy[i].IngressRules[j]))
				}
			}

		} else {
			assertOnTrue(t, err == nil, fmt.Sprintf("got ingress policy %+v, %+v!", npIngress, k8spolicy))
		}

	}
}

func TestApplyContivNetworkPolicy(t *testing.T) {
	testData := []k8sNetworkPolicy{
		{
			PodSelector: &k8sPodSelector{TenantName: defaultTenantName, NetworkName: defaultNetworkName, GroupName: "epg1"},
			Ingress: []k8sIngress{
				{
					IngressPodSelector: []*k8sPodSelector{{
						TenantName:  defaultTenantName,
						NetworkName: defaultNetworkName,
						GroupName:   "epg2"},
					},
					IngressRules: []k8sPolicyPorts{
						{Protocol: "TCP", Port: 500},
						{Protocol: "TCP", Port: 400},
					},
				},
				{
					IngressPodSelector: []*k8sPodSelector{{
						TenantName:  defaultTenantName,
						NetworkName: defaultNetworkName,
						GroupName:   "epg1"},
					},
					IngressRules: []k8sPolicyPorts{
						{Protocol: "TCP", Port: 500},
						{Protocol: "TCP", Port: 400},
						{Protocol: "UDP", Port: 500},
						{Protocol: "UDP", Port: 400},
					},
				},
			},
		},
	}

	for _, testSet := range testData {
		err := k8sut.applyContivNetworkPolicy(&testSet)
		assertOnTrue(t, err != nil, fmt.Sprintf("failed to configure contiv policy %+v, %s",
			testSet, err))
		// verify epg &  default rules
		_, err1 := k8sut.contivClient.EndpointGroupGet(testSet.PodSelector.TenantName,
			testSet.PodSelector.GroupName)
		assertOnTrue(t, err1 != nil, fmt.Sprintf("failed to get epg  %+v, %s", testSet.PodSelector, err))

		for _, d := range []string{"in", "out"} {
			_, err := k8sut.contivClient.RuleGet(testSet.PodSelector.TenantName,
				k8sutils.EpgNameToPolicy(testSet.PodSelector.GroupName),
				k8sutils.DenyAllRuleID+d)
			assertOnTrue(t, err != nil, fmt.Sprintf("failed to get rule  %+v, %s",
				k8sutils.DenyAllRuleID+d, err))
		}

		// verify rules
		for _, ingressSet := range testSet.Ingress {
			for _, fromEpg := range ingressSet.IngressPodSelector {
				// verify epg & default rules
				_, err := k8sut.contivClient.EndpointGroupGet(fromEpg.TenantName,
					fromEpg.GroupName)
				assertOnTrue(t, err != nil, fmt.Sprintf("failed to get epg  %+v, %s",
					fromEpg, err))

				for _, d := range []string{"in", "out"} {
					_, err := k8sut.contivClient.RuleGet(fromEpg.TenantName,
						k8sutils.EpgNameToPolicy(fromEpg.GroupName),
						k8sutils.DenyAllRuleID+d)
					assertOnTrue(t, err != nil, fmt.Sprintf("failed to get rule  %+v %s",
						k8sutils.DenyAllRuleID+d, err))
				}

				// verify rules
				for _, fromPort := range ingressSet.IngressRules {
					for _, d := range []string{"in", "out"} {
						_, err := k8sut.contivClient.RuleGet(
							testSet.PodSelector.TenantName,
							k8sutils.EpgNameToPolicy(testSet.PodSelector.GroupName),
							k8sutils.PolicyToRuleID(fromEpg.GroupName,
								fromPort.Protocol, fromPort.Port, d))
						assertOnTrue(t, err != nil,
							fmt.Sprintf("failed to get rule %+v (%s) %s",
								fromPort, d, err))
					}
				}
			}
		}

	}

	// clean up
	for _, testSet := range testData {
		err := k8sut.cleanupContivNetworkPolicy(&testSet)
		assertOnTrue(t, err != nil, fmt.Sprintf("failed to delete contiv policy %+v, %s",
			testSet, err))
		// verify epg &  default rules
		_, err1 := k8sut.contivClient.EndpointGroupGet(testSet.PodSelector.TenantName,
			testSet.PodSelector.GroupName)
		assertOnTrue(t, err1 == nil, fmt.Sprintf("epg exists after cleanup %+v, %s", testSet.PodSelector, err))

		for _, d := range []string{"in", "out"} {
			_, err := k8sut.contivClient.RuleGet(testSet.PodSelector.TenantName,
				k8sutils.EpgNameToPolicy(testSet.PodSelector.GroupName),
				k8sutils.DenyAllRuleID+d)
			assertOnTrue(t, err == nil, fmt.Sprintf("default rule exists after cleanup  %+v, %s",
				k8sutils.DenyAllRuleID+d, err))
		}

		// verify rules
		for _, ingressSet := range testSet.Ingress {
			for _, fromEpg := range ingressSet.IngressPodSelector {
				// verify epg & default rules
				_, err := k8sut.contivClient.EndpointGroupGet(fromEpg.TenantName,
					fromEpg.GroupName)
				assertOnTrue(t, err == nil, fmt.Sprintf("epg exists after cleasnup %+v, %s",
					fromEpg, err))

				for _, d := range []string{"in", "out"} {
					_, err := k8sut.contivClient.RuleGet(fromEpg.TenantName,
						k8sutils.EpgNameToPolicy(fromEpg.GroupName),
						k8sutils.DenyAllRuleID+d)
					assertOnTrue(t, err == nil, fmt.Sprintf("rule exists after clreanup  %+v %s",
						k8sutils.DenyAllRuleID+d, err))
				}

				// verify rules
				for _, fromPort := range ingressSet.IngressRules {
					for _, d := range []string{"in", "out"} {
						_, err := k8sut.contivClient.RuleGet(
							testSet.PodSelector.TenantName,
							k8sutils.EpgNameToPolicy(testSet.PodSelector.GroupName),
							k8sutils.PolicyToRuleID(fromEpg.GroupName,
								fromPort.Protocol, fromPort.Port, d))
						assertOnTrue(t, err == nil,
							fmt.Sprintf("rule exists after cleanup %+v (%s) %s",
								fromPort, d, err))
					}
				}
			}
		}
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
	if apiController := objApi.NewAPIController(router, objdbClient, "etcd://127.0.0.1:2379"); apiController == nil {
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

	// create tenant/network
	if err := k8sut.contivClient.NetworkPost(&client.Network{
		TenantName:  defaultTenantName,
		NetworkName: defaultNetworkName,
		Subnet:      "20.20.20.0/24",
		Gateway:     "20.20.20.1",
		Encap:       "vxlan"}); err != nil {
		nptLog.Fatalf("failed to create %s, %s", defaultNetworkName, err)
	}

	if err := k8sut.contivClient.TenantPost(&client.Tenant{TenantName: "newTenant"}); err != nil {
		nptLog.Fatalf("failed to create newTenant, %s", err)
	}

	if err := k8sut.contivClient.NetworkPost(&client.Network{
		TenantName:  "newTenant",
		NetworkName: "newNet",
		Subnet:      "30.20.20.0/24",
		Gateway:     "30.20.20.1",
		Encap:       "vxlan"}); err != nil {
		nptLog.Fatalf("failed to create network, %s", err)
	}

	k8sut.isLeader = func() bool { return true }
	exitCode := m.Run()
	if exitCode == 0 {
		cleanupState()
	}
	os.Exit(exitCode)
}
