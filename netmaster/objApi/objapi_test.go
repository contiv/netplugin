/***
Copyright 2014 Cisco Systems Inc. All rights reserved.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package objApi

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	contivModel "github.com/contiv/netplugin/contivmodel"
	"github.com/contiv/netplugin/contivmodel/client"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/gstate"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/netmaster/resources"
	"github.com/contiv/netplugin/objdb"
	"github.com/contiv/netplugin/state"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/netplugin/utils/netutils"
	"github.com/contiv/ofnet"
	etcdclient "github.com/coreos/etcd/client"
	"github.com/gorilla/mux"
	"golang.org/x/net/context"
)

const (
	netmasterTestURL       = "http://localhost:9230"
	netmasterTestListenURL = ":9230"
)

var contivClient *client.ContivClient
var apiController *APIController
var stateStore core.StateDriver

// initStateDriver initialize etcd state driver
func initStateDriver() (core.StateDriver, error) {
	instInfo := core.InstanceInfo{DbURL: "etcd://127.0.0.1:2379"}

	return utils.NewStateDriver(utils.EtcdNameStr, &instInfo)
}

type restAPIFunc func(r *http.Request) (interface{}, error)

func httpWrapper(handlerFunc restAPIFunc) http.HandlerFunc {
	// Create a closure and return an anonymous function
	return func(w http.ResponseWriter, r *http.Request) {
		// Call the handler
		resp, err := handlerFunc(r)
		if err != nil {
			// Log error
			log.Fatalf("Handler for %s %s returned error: %s", r.Method, r.URL, err)
		} else {
			// Send HTTP response as Json
			content, err := json.Marshal(resp)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Write(content)
		}
	}
}

func bgpInspector(r *http.Request) (interface{}, error) {
	return nil, nil
}

// setupBgpInspectServer
func setupBgpInspectServer() (net.Listener, error) {
	r := mux.NewRouter()
	r.HandleFunc("/inspect/bgp", httpWrapper(bgpInspector))
	listener, err := net.Listen("tcp", "127.0.0.1:9090")
	if err != nil {
		log.Fatalf("Error creating listener: %v", err)
		return nil, err
	}
	srv := &http.Server{
		Handler:      r,
		WriteTimeout: 20 * time.Second,
		ReadTimeout:  20 * time.Second,
	}

	go srv.Serve(listener)

	return listener, nil
}

// setup the test netmaster REST server and client
func TestMain(m *testing.M) {
	var err error

	// Setup state store
	stateStore, err = initStateDriver()
	if err != nil {
		log.Fatalf("Error initializing state store. Err: %v", err)
	}
	// little hack to clear all state from etcd
	stateStore.(*state.EtcdStateDriver).KeysAPI.Delete(context.Background(), "/contiv.io", &etcdclient.DeleteOptions{Recursive: true})

	// Setup resource manager
	if _, err = resources.NewStateResourceManager(stateStore); err != nil {
		log.Fatalf("Failed to init resource manager. Error: %s", err)
	}

	router := mux.NewRouter()
	s := router.Headers("Content-Type", "application/json").Methods("Post").Subrouter()
	s.HandleFunc("/plugin/updateEndpoint", utils.MakeHTTPHandler(master.UpdateEndpointHandler))
	s.HandleFunc("/plugin/createEndpoint", utils.MakeHTTPHandler(master.CreateEndpointHandler))
	s = router.Methods("Get").Subrouter()

	// create objdb client
	objdbClient, err := objdb.NewClient("etcd://127.0.0.1:2379")
	if err != nil {
		log.Fatalf("Error connecting to state store: etcd://127.0.0.1:2379. Err: %v", err)
	}

	// Create a new api controller
	apiConfig := &APIControllerConfig{
		NetForwardMode: "bridge",
		NetInfraType:   "default",
	}
	apiController = NewAPIController(router, objdbClient, apiConfig)

	ofnetMaster := ofnet.NewOfnetMaster("127.0.0.1", ofnet.OFNET_MASTER_PORT)
	if ofnetMaster == nil {
		log.Fatalf("Error creating ofnet master")
	}

	// initialize policy manager
	mastercfg.InitPolicyMgr(stateStore, ofnetMaster)

	// Create HTTP server
	go http.ListenAndServe(netmasterTestListenURL, router)
	// Create bgp inspect server
	l, _ := setupBgpInspectServer()
	defer l.Close()
	time.Sleep(time.Second)

	// create a new contiv client
	contivClient, err = client.NewContivClient(netmasterTestURL)
	if err != nil {
		log.Fatalf("Error creating contiv client. Err: %v", err)
	}

	exitCode := m.Run()
	if exitCode == 0 {
		cleanupState()
	}
	os.Exit(exitCode)
}

// cleanupState cleans up default tenant and other global state
func cleanupState() {
	// delete default tenant
	err := contivClient.TenantDelete("default")
	if err != nil {
		log.Fatalf("Error deleting default tenant. Err: %v", err)
	}

	// clear global state
	err = contivClient.GlobalDelete("global")
	if err != nil {
		log.Fatalf("Error deleting global state. Err: %v", err)
	}
}

// checkError checks for error and fails the test
func checkError(t *testing.T, testStr string, err error) {
	if err != nil {
		t.Fatalf("Error during %s. Err: %v", testStr, err)
	}
}

// checkCreateNetwork creates networks and checks for error
func checkCreateNetwork(t *testing.T, expError bool, tenant, network, nwType, encap, subnet, gw string, tag int, v6subnet, v6gw, nwTag string) {
	net := client.Network{
		TenantName:  tenant,
		NetworkName: network,
		NwType:      nwType,
		Encap:       encap,
		Subnet:      subnet,
		Gateway:     gw,
		Ipv6Subnet:  v6subnet,
		Ipv6Gateway: v6gw,
		PktTag:      tag,
		CfgdTag:     nwTag,
	}
	err := contivClient.NetworkPost(&net)
	if err != nil && !expError {
		t.Fatalf("Error creating network {%+v}. Err: %v", net, err)
	} else if err == nil && expError {
		t.Fatalf("Create network {%+v} succeeded while expecting error", net)
	} else if err == nil {
		// verify network is created
		_, err := contivClient.NetworkGet(tenant, network)
		if err != nil {
			t.Fatalf("Error getting network %s/%s. Err: %v", tenant, network, err)
		}
	}
}

// checkInspectNetwork inspects network and checks for error
func checkInspectNetwork(t *testing.T, expError bool, tenant, network, allocedIPs string, pktTag, addrCount int) {
	insp, err := contivClient.NetworkInspect(tenant, network)
	if err != nil && !expError {
		t.Fatalf("Error inspecting network {%s.%s}. Err: %v", network, tenant, err)
	} else if err == nil && expError {
		t.Fatalf("Inspect network {%s.%s} succeeded while expecting error", network, tenant)
	} else if err == nil {
		if insp.Oper.AllocatedAddressesCount != addrCount || insp.Oper.PktTag != pktTag ||
			insp.Oper.AllocatedIPAddresses != allocedIPs {
			log.Printf("Inspect network {%+v}", insp)
			t.Fatalf("addrCount exp: %d got: %d pktTag exp: %d got: %d, allocedIPs exp: %s got: %s",
				addrCount, insp.Oper.AllocatedAddressesCount, pktTag, insp.Oper.PktTag,
				allocedIPs, insp.Oper.AllocatedIPAddresses)
		}
	}
}

// verifyNetworkState verifies network state es as expected
func verifyNetworkState(t *testing.T, tenant, network, nwType, encap, subnet, gw string, subnetLen uint, pktTag, extTag int, v6subnet, v6gw string, v6subnetLen uint) {
	networkID := network + "." + tenant
	nwCfg := &mastercfg.CfgNetworkState{}
	nwCfg.StateDriver = stateStore
	err := nwCfg.Read(networkID)
	if err != nil {
		t.Fatalf("Network state for %s not found. Err: %v", networkID, err)
	}

	// verify network params
	if nwCfg.Tenant != tenant || nwCfg.NetworkName != network || nwCfg.NwType != nwType ||
		nwCfg.PktTagType != encap || nwCfg.SubnetIP != netutils.GetSubnetAddr(subnet, subnetLen) || nwCfg.Gateway != gw ||
		nwCfg.IPv6Subnet != v6subnet || nwCfg.IPv6SubnetLen != v6subnetLen || nwCfg.IPv6Gateway != v6gw {
		t.Fatalf("Network state {%+v} did not match expected state", nwCfg)
	}

	// verify network tags
	if (pktTag != 0 && nwCfg.PktTag != pktTag) ||
		(extTag != 0 && nwCfg.ExtPktTag != extTag) {
		t.Fatalf("Network tags %d/%d did not match expected %d/%d",
			nwCfg.PktTag, nwCfg.ExtPktTag, pktTag, extTag)
	}
}

// checkDeleteNetwork deletes networks and looks for error
func checkDeleteNetwork(t *testing.T, expError bool, tenant, network string) {
	err := contivClient.NetworkDelete(tenant, network)
	if err != nil && !expError {
		t.Fatalf("Error deleting network %s/%s. Err: %v", tenant, network, err)
	} else if err == nil && expError {
		t.Fatalf("Delete network %s/%s succeeded while expecting error", tenant, network)
	} else if err == nil {
		// verify network is gone
		_, err := contivClient.NetworkGet(tenant, network)
		if err == nil {
			t.Fatalf("Network %s/%s not deleted", tenant, network)
		}

		// verify network state is gone too
		networkID := network + "." + tenant
		nwCfg := &mastercfg.CfgNetworkState{}
		nwCfg.StateDriver = stateStore
		err = nwCfg.Read(networkID)
		if err == nil {
			t.Fatalf("Network state %s not deleted", networkID)
		}
	}
}

// checkInspectGlobal inspects network and checks for error
func checkInspectGlobal(t *testing.T, expError bool, allocedVlans, allocedVxlans string) {
	insp, err := contivClient.GlobalInspect("global")
	if err != nil && !expError {
		t.Fatalf("Error inspecting global info. Err: %v", err)
	} else if err == nil && expError {
		t.Fatalf("Inspect global info succeeded while expecting error")
	} else if err == nil {
		if insp.Oper.VlansInUse != allocedVlans || insp.Oper.VxlansInUse != allocedVxlans {
			t.Fatalf("Inspect global {%+v} failed with mismatching vlan/vxlan allocated ", insp)
		}
	}
}

// checkGlobalSet sets global state and verifies state
// the return error can be used for validating the error produced
// by contivClient.GlobalPost, a non-nil return is not an error, 'expError'
// parameter determines if an err for GlobalPost is a failure for the test
func checkGlobalSet(t *testing.T, expError bool, fabMode, vlans, vxlans, fwdMode, arpMode, pvtSubnet string) error {
	gl := client.Global{
		Name:             "global",
		NetworkInfraType: fabMode,
		Vlans:            vlans,
		Vxlans:           vxlans,
		FwdMode:          fwdMode,
		ArpMode:          arpMode,
		PvtSubnet:        pvtSubnet,
	}
	err := contivClient.GlobalPost(&gl)
	if err != nil && !expError {
		t.Fatalf("Error setting global {%+v}. Err: %v", gl, err)
	} else if err == nil && expError {
		t.Fatalf("Set global {%+v} succeeded while expecting error", gl)
	} else if err == nil {
		// verify global state
		gotGl, err := contivClient.GlobalGet("global")
		if err != nil {
			t.Fatalf("Error getting global object. Err: %v", err)
		}

		// verify expected values
		if gotGl.NetworkInfraType != fabMode || gotGl.Vlans != vlans || gotGl.Vxlans != vxlans || gotGl.PvtSubnet != pvtSubnet {
			t.Fatalf("Error Got global state {%+v} does not match expected %s, %s, %s %s", gotGl, fabMode, vlans, vxlans, pvtSubnet)
		}

		// verify the state created
		gCfg := &gstate.Cfg{}
		gCfg.StateDriver = stateStore
		err = gCfg.Read("")
		if err != nil {
			t.Fatalf("Error reading global cfg state. Err: %v", err)
		}

		if gCfg.Auto.VLANs != vlans || gCfg.Auto.VXLANs != vxlans {
			t.Fatalf("global config Vlan/Vxlan ranges %s/%s are not same as %s/%s",
				gCfg.Auto.VLANs, gCfg.Auto.VXLANs, vlans, vxlans)
		}

		// verify global oper state
		gOper := &gstate.Oper{}
		gOper.StateDriver = stateStore
		err = gOper.Read("")
		if err != nil {
			t.Fatalf("Error reading global oper state. Err: %v", err)
		}

		// verify vxlan resources
		vxlanRsrc := &resources.AutoVXLANCfgResource{}
		vxlanRsrc.StateDriver = stateStore
		if err := vxlanRsrc.Read("global"); err != nil {
			t.Fatalf("Error reading vxlan resource. Err: %v", err)
		}

		// verify vlan resource
		vlanRsrc := &resources.AutoVLANCfgResource{}
		vlanRsrc.StateDriver = stateStore
		if err := vlanRsrc.Read("global"); err != nil {
			t.Fatalf("Error reading vlan resource. Err: %v", err)
		}
		return nil
	}
	return err
}

// checkAciGwSet sets AciGw state and verifies verifies it
func checkAciGwSet(t *testing.T, expError bool, enf, comTen, paths, nodes, domain string) {
	aciConf := client.AciGw{
		Name:                "aciGw",
		IncludeCommonTenant: comTen,
		EnforcePolicies:     enf,
		PathBindings:        paths,
		NodeBindings:        nodes,
		PhysicalDomain:      domain,
	}
	err := contivClient.AciGwPost(&aciConf)
	if err != nil && !expError {
		t.Fatalf("Error setting aci {%+v}. Err: %v", aciConf, err)
	} else if err == nil && expError {
		t.Fatalf("Set aci {%+v} succeeded while expecting error", aciConf)
	} else if err == nil {
		// verify global state
		gotAciGw, err := contivClient.AciGwGet("aciGw")
		if err != nil {
			t.Fatalf("Error getting aci object. Err: %v", err)
		}

		// verify expected values
		if gotAciGw.EnforcePolicies != enf || gotAciGw.PathBindings != paths || gotAciGw.NodeBindings != nodes || gotAciGw.PhysicalDomain != domain || gotAciGw.IncludeCommonTenant != comTen {
			t.Fatalf("Error Got aci state {%+v} does not match expected %s, %s, %s, %s, %s", gotAciGw, enf, comTen, paths, nodes, domain)
		}

	}
}

func checkAciGwDelete(t *testing.T, expError bool, name string) {
	err := contivClient.AciGwDelete(name)
	if err != nil && !expError {
		t.Fatalf("Error deleting aci. Err: %v", err)
	} else if err == nil && expError {
		t.Fatalf("Delete aci succeeded while expecting error")
	}
}

// checkCreatePolicy creates policy and verifies
func checkCreatePolicy(t *testing.T, expError bool, tenant, policy string) {
	pol := client.Policy{
		TenantName: tenant,
		PolicyName: policy,
	}
	err := contivClient.PolicyPost(&pol)
	if err != nil && !expError {
		t.Fatalf("Error creating policy {%+v}. Err: %v", pol, err)
	} else if err == nil && expError {
		t.Fatalf("Create policy {%+v} succeeded while expecting error", pol)
	} else if err == nil {
		// verify policy is created
		_, err := contivClient.PolicyGet(tenant, policy)
		if err != nil {
			t.Fatalf("Error getting policy %s/%s. Err: %v", tenant, policy, err)
		}
	}
}

// checkDeletePolicy deletes policy and verifies
func checkDeletePolicy(t *testing.T, expError bool, tenant, policy string) {
	err := contivClient.PolicyDelete(tenant, policy)
	if err != nil && !expError {
		t.Fatalf("Error deleting policy %s/%s. Err: %v", tenant, policy, err)
	} else if err == nil && expError {
		t.Fatalf("Delete policy %s/%s succeeded while expecting error", tenant, policy)
	} else if err == nil {
		// verify policy is gone
		_, err := contivClient.PolicyGet(tenant, policy)
		if err == nil {
			t.Fatalf("Policy %s/%s not deleted", tenant, policy)
		}
	}
}

// checkCreateRule creates rule and checks for error
func checkCreateRule(t *testing.T, expError bool, tenant, policy, ruleID, dir, fnet, fepg, fip, tnet, tepg, tip, proto, act string, prio, port int) {
	pol := client.Rule{
		TenantName:        tenant,
		PolicyName:        policy,
		RuleID:            ruleID,
		Direction:         dir,
		Priority:          prio,
		FromNetwork:       fnet,
		FromEndpointGroup: fepg,
		FromIpAddress:     fip,
		ToNetwork:         tnet,
		ToEndpointGroup:   tepg,
		ToIpAddress:       tip,
		Protocol:          proto,
		Port:              port,
		Action:            act,
	}
	err := contivClient.RulePost(&pol)
	if err != nil && !expError {
		t.Fatalf("Error creating rule {%+v}. Err: %v", pol, err)
	} else if err == nil && expError {
		t.Fatalf("Create rule {%+v} succeeded while expecting error", pol)
	} else if err == nil {
		// verify rule is created
		_, err := contivClient.RuleGet(tenant, policy, ruleID)
		if err != nil {
			t.Fatalf("Error getting rule %s/%s/%s. Err: %v", tenant, policy, ruleID, err)
		}
	}
}

// checkDeleteRule deletes rule
func checkDeleteRule(t *testing.T, expError bool, tenant, policy, ruleID string) {
	err := contivClient.RuleDelete(tenant, policy, ruleID)
	if err != nil && !expError {
		t.Fatalf("Error deleting rule %s/%s/%s. Err: %v", tenant, policy, ruleID, err)
	} else if err == nil && expError {
		t.Fatalf("Delete rule %s/%s/%s succeeded while expecting error", tenant, policy, ruleID)
	} else if err == nil {
		// verify rule is gone
		_, err := contivClient.RuleGet(tenant, policy, ruleID)
		if err == nil {
			t.Fatalf("Policy %s/%s/%s not deleted", tenant, policy, ruleID)
		}
	}
}

func checkCreateNetProfile(t *testing.T, expError bool, dscp, burst int, bandwidth, profileName, tenantName string) {
	np := client.Netprofile{
		DSCP:        dscp,
		Burst:       burst,
		Bandwidth:   bandwidth,
		TenantName:  tenantName,
		ProfileName: profileName,
	}
	err := contivClient.NetprofilePost(&np)
	if err != nil && !expError {
		t.Fatalf("Error creating Netprofile {%+v}. Err: %v", np, err)
	} else if err == nil && expError {
		t.Fatalf("Create NetProfile {%+v} succeeded while expecing error", np)
	} else if err == nil {
		//check if netprofile is created.
		_, err := contivClient.NetprofileGet(tenantName, profileName)
		if err != nil {
			t.Fatalf("Error getting netprofile %s/%s. Err: %v", tenantName, profileName, err)
		}
	}
}

func checkverifyNetProfile(t *testing.T, expError bool, dscp, burst int, bandwidth, profileName, tenantName string) {
	paramsKey := GetNetprofileKey(tenantName, profileName)
	netprofile := contivModel.FindNetprofile(paramsKey)
	if netprofile.Bandwidth != bandwidth {
		t.Fatalf("error matching netprofile bandwidth: %s to: %s ", netprofile.Bandwidth, bandwidth)
	}
	if netprofile.DSCP != dscp {
		t.Fatalf("error matching netprofile bandwidth: %d to: %d ", netprofile.DSCP, dscp)
	}
	if netprofile.Burst != burst {
		t.Fatalf("error matching netprofile burst rate: %d to: %d ", netprofile.Burst, burst)
	}
}

//checkDeleteNetProfile checks if the netprofile is deleted.
func checkDeleteNetProfile(t *testing.T, expError bool, profileName, tenantName string) {
	err := contivClient.NetprofileDelete(tenantName, profileName)
	if err != nil && !expError {
		t.Fatalf("Error deleting Netprofile %s/%s Err: %v", profileName, tenantName, err)
	} else if err == nil && expError {
		t.Fatalf("delete NetProfile %s/%s succeeded while expecing error", profileName, tenantName)
	} else if err == nil {
		//check if netprofile is deleted.
		_, err := contivClient.NetprofileGet(tenantName, profileName)
		if err == nil {
			t.Fatalf("Error deleting netprofile %s/%s. Netprofile still exists. Err: %v", tenantName, profileName, err)
		}
	}
}

// verifyEpgnetProfile verifies an EPG policy state
func verifyEpgnetProfile(t *testing.T, tenant, group, Bandwidth string, DSCP, burst int) {
	// Get the state driver - get the etcd driver state
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		t.Fatalf("Error getting the state of etcd driver %v", err)
	}

	epgKey := group + ":" + tenant
	//read from etcd
	epCfg := mastercfg.EndpointGroupState{}
	epCfg.StateDriver = stateDriver

	err = epCfg.Read(epgKey)
	if err != nil {
		t.Fatalf("Error finding endpointgroup %s. Err: %v", epgKey, err)
	}

	if epCfg.Bandwidth != Bandwidth {
		t.Fatalf("Error updating endpoint group %s with bandwidth: %s, prev bandwidth: %s", epCfg.GroupName, Bandwidth, epCfg.Bandwidth)
	}
	if epCfg.DSCP != DSCP {
		t.Fatalf("Error updating endpoint group %s with DSCP %d", epCfg.GroupName, DSCP)
	}
	if epCfg.Burst != burst {
		t.Fatalf("Error updating endpoint group %s with burst %d applied burst:%d", epCfg.GroupName, burst, epCfg.Burst)
	}
	err = epCfg.Write()
	if err != nil {
		t.Fatalf("Error saving the etcd state")
	}
}

//checkCreateEpgNp creates a group
func checkCreateEpgNp(t *testing.T, expError bool, tenant, ProfileName, network, group string, extContracts []string) {
	epg := client.EndpointGroup{
		TenantName:       tenant,
		NetProfile:       ProfileName,
		NetworkName:      network,
		GroupName:        group,
		ExtContractsGrps: extContracts,
	}
	err := contivClient.EndpointGroupPost(&epg)
	if err != nil && !expError {
		t.Fatalf("Error creating epg {%+v}. Err: %v", epg, err)
	} else if err == nil && expError {
		t.Fatalf("Create epg {%+v} succeeded while expecing error", epg)
	} else if err == nil {
		// verify epg is created
		_, err := contivClient.EndpointGroupGet(tenant, group)
		if err != nil {
			t.Fatalf("Error getting epg %s/%s/%s. Err: %v", tenant, network, group, err)
		}
	}
}

// checkEpgnetProfileDeleted verifies EPG netprofile is deleted
func checkEpgnetProfileDetached(t *testing.T, tenant, network, group, nProfile string) {

	// Get the state driver - get the etcd driver state
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		t.Fatalf("Error getting the state of etcd driver %v", err)
	}

	epgKey := tenant + ":" + group
	epgCfgKey := group + ":" + tenant

	epCfg := mastercfg.EndpointGroupState{}
	epCfg.StateDriver = stateDriver

	//read the etcd
	err = epCfg.Read(epgCfgKey)
	if err != nil {
		t.Fatalf("Error finding endpointgroup %s. Err: %v", epgCfgKey, err)
	}

	if epCfg.Bandwidth != "" {
		t.Fatalf("Error removing bandwidth %s", epCfg.Bandwidth)
	}

	if epCfg.DSCP != 0 {
		t.Fatalf("Error removing DSCP %d", epCfg.DSCP)
	}

	epg := contivModel.FindEndpointGroup(epgKey)
	if epg == nil {
		t.Fatalf("Error finding EPG %s", epgKey)
	}
	if epg.NetProfile != "" {
		t.Fatalf("Error detaching net profile %s", epg.NetProfile)
	}

	//save the etcd state.
	err = epCfg.Write()
	if err != nil {
		t.Fatalf("Error saving the etcd state")
	}
}

// checkCreateEpg creates an EPG
func checkCreateEpg(t *testing.T, expError bool, tenant, network, group string, policies, extContracts []string, groupTag string) {
	epg := client.EndpointGroup{
		TenantName:       tenant,
		NetworkName:      network,
		GroupName:        group,
		Policies:         policies,
		ExtContractsGrps: extContracts,
		CfgdTag:          groupTag,
	}
	err := contivClient.EndpointGroupPost(&epg)
	if err != nil && !expError {
		t.Fatalf("Error creating epg {%+v}. Err: %v", epg, err)
	} else if err == nil && expError {
		t.Fatalf("Create epg {%+v} succeeded while expecting error", epg)
	} else if err == nil {
		// verify epg is created
		_, err := contivClient.EndpointGroupGet(tenant, group)
		if err != nil {
			t.Fatalf("Error getting epg %s/%s/%s. Err: %v", tenant, network, group, err)
		}
	}
}

// checkCreateExtContractsGrp creates an external contracts group
func checkCreateExtContractsGrp(t *testing.T, expError bool, tenant, grpName, grpType string, contracts []string) {
	extContractsGrp := client.ExtContractsGroup{
		TenantName:         tenant,
		ContractsGroupName: grpName,
		ContractsType:      grpType,
		Contracts:          contracts,
	}

	err := contivClient.ExtContractsGroupPost(&extContractsGrp)
	if err != nil && !expError {
		t.Fatalf("Error creating extContractsGrp {%+v}. Err: %v", extContractsGrp, err)
	} else if err == nil && expError {
		t.Fatalf("Create extContrctsGrp {%+v} succeeded while expecting error", extContractsGrp)
	} else if err == nil {
		// verify extContractsGrp is created
		_, err := contivClient.ExtContractsGroupGet(tenant, grpName)
		if err != nil {
			t.Fatalf("Error getting extContractsGrp %s/%s. Err: %v", tenant, grpName, err)
		}
	}
}

// checkDeleteExtContractsGrp deletes external contracts group
func checkDeleteExtContractsGrp(t *testing.T, expError bool, tenant, grpName string) {
	err := contivClient.ExtContractsGroupDelete(tenant, grpName)
	if err != nil && !expError {
		t.Fatalf("Error deleting extContractsGrp %s/%s. Err: %v", tenant, grpName, err)
	} else if err == nil && expError {
		t.Fatalf("Delete extContractsGrp %s/%s succeeded while expecting error", tenant, grpName)
	} else if err == nil {
		// verify epg is gone
		_, err := contivClient.ExtContractsGroupGet(tenant, grpName)
		if err == nil {
			t.Fatalf("ExtContractsGroup %s/%s not deleted", tenant, grpName)
		}
	}
}

// verifyEpgPolicy verifies an EPG policy state
func verifyEpgPolicy(t *testing.T, tenant, network, group, policy string) {
	epgKey := tenant + ":" + group
	policyKey := tenant + ":" + policy
	epgpKey := epgKey + ":" + policyKey

	// find the endpoint group
	epg := contivModel.FindEndpointGroup(epgKey)
	if epg == nil {
		t.Fatalf("Error finding EPG %s", epgKey)
	}

	// find the policy
	pol := contivModel.FindPolicy(policyKey)
	if pol == nil {
		t.Fatalf("Error finding policy %s", policyKey)
	}

	// See if it already exists
	gp := mastercfg.FindEpgPolicy(epgpKey)
	if gp == nil {
		t.Fatalf("Error finding EPG policy %s", epgpKey)
	}

	// verify all rules exist in epg policy
	for ruleKey := range pol.LinkSets.Rules {
		if gp.RuleMaps[ruleKey] == nil {
			t.Fatalf("Rule %s not found in EPG policy %s", ruleKey, epgpKey)
		}
	}
}

// checkEpgPolicyDeleted verifies EPG policy is deleted
func checkEpgPolicyDeleted(t *testing.T, tenant, network, group, policy string) {
	epgKey := tenant + ":" + group
	policyKey := tenant + ":" + policy
	epgpKey := epgKey + ":" + policyKey

	// See if it already exists
	gp := mastercfg.FindEpgPolicy(epgpKey)
	if gp != nil {
		t.Fatalf("Found EPG policy %s while expecting it to be deleted", epgpKey)
	}

}

// checkDeleteEpg deletes EPG
func checkDeleteEpg(t *testing.T, expError bool, tenant, network, group string) {
	err := contivClient.EndpointGroupDelete(tenant, group)
	if err != nil && !expError {
		t.Fatalf("Error deleting epg %s/%s. Err: %v", tenant, group, err)
	} else if err == nil && expError {
		t.Fatalf("Delete epg %s/%s succeeded while expecting error", tenant, group)
	} else if err == nil {
		// verify epg is gone
		_, err := contivClient.EndpointGroupGet(tenant, group)
		if err == nil {
			t.Fatalf("EndpointGroup %s/%s/%s not deleted", tenant, network, group)
		}
	}
}

// checkCreateAppProfile creates app-profiles and checks for error
func checkCreateAppProfile(t *testing.T, expError bool, tenant, profName string, epgs []string) {
	prof := client.AppProfile{
		TenantName:     tenant,
		AppProfileName: profName,
		EndpointGroups: epgs,
	}
	err := contivClient.AppProfilePost(&prof)
	if err != nil && !expError {
		t.Fatalf("Error creating AppProfile {%+v}. Err: %v", prof, err)
	} else if err == nil && expError {
		t.Fatalf("Create AppProfile {%+v} succeeded while expecting error", prof)
	} else if err == nil {
		// verify AppProfile is created
		_, err := contivClient.AppProfileGet(tenant, profName)
		if err != nil {
			t.Fatalf("Error getting AppProfile %s/%s. Err: %v", tenant, profName, err)
		}
	}
}

// verifyAppProfile creates app-profiles and checks for error
func verifyAppProfile(t *testing.T, expError bool, tenant, profName string, epgs []string) {
	profKey := tenant + ":" + profName
	prof := contivModel.FindAppProfile(profKey)
	if prof == nil {
		if expError {
			return
		}
		t.Fatalf("Error AppProfile %s/%s not found.", tenant, profName)
	}

	// Verify the epg map
	if len(epgs) != len(prof.EndpointGroups) {
		t.Fatalf("Error epgs %v dont match profile %v", epgs, prof.EndpointGroups)
	}

	epgMap := make(map[string]bool)
	for _, epg := range epgs {
		epgMap[epg] = true
	}

	for _, epg := range prof.EndpointGroups {
		found, res := epgMap[epg]
		if !found || !res {
			t.Fatalf("Error epgs %v dont match profile %v", epgs, prof.EndpointGroups)
		}
	}

}

// checkDeleteAppProfile deletes AppProf and looks for error
func checkDeleteAppProfile(t *testing.T, expError bool, tenant, prof string) {
	err := contivClient.AppProfileDelete(tenant, prof)
	if err != nil && !expError {
		t.Fatalf("Error deleting AppProfile %s/%s. Err: %v", tenant, prof, err)
	} else if err == nil && expError {
		t.Fatalf("Delete AppProfile %s/%s succeeded while expecting error", tenant, prof)
	} else if err == nil {
		// verify AppProfile is gone
		_, err := contivClient.AppProfileGet(tenant, prof)
		if err == nil {
			t.Fatalf("AppProfile %s/%s not deleted", tenant, prof)
		} else {
			t.Logf("AppProfile %s/%s successfully deleted", tenant, prof)
		}
	}
}

// checkCreateTenant creates Tenant and checks for error
func checkCreateTenant(t *testing.T, expError bool, tenantName string) {
	// tenant params
	tenant := client.Tenant{
		TenantName: tenantName,
	}
	// create a tenant
	err := contivClient.TenantPost(&tenant)
	if err != nil && !expError {
		t.Fatalf("Error creating tenant {%+v}. Err: %v", tenant, err)
	} else if err == nil && expError {
		t.Fatalf("Create tenant {%+v} succeeded while expecting error", tenant)
	} else if err == nil {
		// verify tenant is created
		_, err := contivClient.TenantGet(tenantName)
		if err != nil {
			t.Fatalf("Error getting tenant %s. Err: %v", tenantName, err)
		}
	}
}

// checkDeleteTenant deletes tenant and looks for error
func checkDeleteTenant(t *testing.T, expError bool, tenant string) {
	err := contivClient.TenantDelete(tenant)
	if err != nil && !expError {
		t.Fatalf("Error deleting tenant %s. Err: %v", tenant, err)
	} else if err == nil && expError {
		t.Fatalf("Delete tenant %s succeeded while expecting error", tenant)
	} else if err == nil {
		// verify network is gone
		_, err := contivClient.TenantGet(tenant)
		if err == nil {
			t.Fatalf("Tenant %s not deleted", tenant)
		}
	}
}

// TestTenantDelete tests deletion of tenant
func TestTenantDelete(t *testing.T) {
	// ensure global configs set
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")
	// Create one tenant, two networks and 3 epgs
	checkCreateTenant(t, false, "tenant1")
	checkCreateNetwork(t, false, "tenant1", "net1", "data", "vlan", "10.1.1.1/16", "10.1.1.254", 1, "", "", "")
	checkCreateNetwork(t, false, "tenant1", "net2", "data", "vlan", "20.1.1.1/16", "20.1.1.254", 2, "", "", "test-tag")
	checkCreateEpg(t, false, "tenant1", "net1", "group1", []string{}, []string{}, "")
	checkCreateEpg(t, false, "tenant1", "net1", "group2", []string{}, []string{}, "")
	checkCreateEpg(t, false, "tenant1", "net2", "group3", []string{}, []string{}, "epg-tag")
	checkCreateAppProfile(t, false, "tenant1", "profile1", []string{})
	checkCreateAppProfile(t, false, "tenant1", "profile2", []string{"group1"})
	checkCreateAppProfile(t, false, "tenant1", "profile3", []string{"group1", "group3"})
	// Verify that tenant can not be deleted while app-profile is attached to it
	checkDeleteTenant(t, true, "tenant1")
	// Delete app-Profiles now
	checkDeleteAppProfile(t, false, "tenant1", "profile1")
	checkDeleteAppProfile(t, false, "tenant1", "profile2")
	checkDeleteAppProfile(t, false, "tenant1", "profile3")

	// Verify that tenant can not deleted while EPGs are attached to it.
	checkDeleteTenant(t, true, "tenant1")
	// Delete EPGs
	checkDeleteEpg(t, false, "tenant1", "net1", "group1")
	checkDeleteEpg(t, false, "tenant1", "net1", "group2")
	checkDeleteEpg(t, false, "tenant1", "net2", "group3")

	// Verify that tenant can not deleted while Networks are attached to it.
	checkDeleteTenant(t, true, "tenant1")
	// Delete Networks
	checkDeleteNetwork(t, false, "tenant1", "net1")
	checkDeleteNetwork(t, false, "tenant1", "net2")

	// Verify that tenant can be delete now
	checkDeleteTenant(t, false, "tenant1")
}

// TestTenantAddDelete tests tenant add delete
func TestTenantAddDelete(t *testing.T) {
	// tenant params
	tenant := client.Tenant{
		TenantName: "tenant1",
	}

	// create a tenant
	err := contivClient.TenantPost(&tenant)
	checkError(t, "create tenant", err)
	err = contivClient.TenantPost(&client.Tenant{TenantName: "tenant-valid"})
	checkError(t, "create tenant", err)

	// Get the tenant and verify it exists
	gotTenant, err := contivClient.TenantGet("tenant1")
	checkError(t, "get tenant", err)

	if gotTenant.TenantName != tenant.TenantName {
		t.Fatalf("Got invalid tenant name. expecting %s. Got %s", tenant.TenantName, gotTenant.TenantName)
	}

	// Try creating invalid names and verify we get an error
	if contivClient.TenantPost(&client.Tenant{TenantName: "tenant:invalid"}) == nil {
		t.Fatalf("tenant create succeeded while expecting error")
	}
	if contivClient.TenantPost(&client.Tenant{TenantName: "tenant|invalid"}) == nil {
		t.Fatalf("tenant create succeeded while expecting error")
	}
	if contivClient.TenantPost(&client.Tenant{TenantName: "tenant\\invalid"}) == nil {
		t.Fatalf("tenant create succeeded while expecting error")
	}
	if contivClient.TenantPost(&client.Tenant{TenantName: "tenant#invalid"}) == nil {
		t.Fatalf("tenant create succeeded while expecting error")
	}
	if contivClient.TenantPost(&client.Tenant{TenantName: "-tenant"}) == nil {
		t.Fatalf("tenant create succeeded while expecting error")
	}
	if contivClient.TenantPost(&client.Tenant{TenantName: "tenant@invalid"}) == nil {
		t.Fatalf("tenant create succeeded while expecting error")
	}
	if contivClient.TenantPost(&client.Tenant{TenantName: "tenant!invalid"}) == nil {
		t.Fatalf("tenant create succeeded while expecting error")
	}
	if contivClient.TenantPost(&client.Tenant{TenantName: "tenant~invalid"}) == nil {
		t.Fatalf("tenant create succeeded while expecting error")
	}
	if contivClient.TenantPost(&client.Tenant{TenantName: "tenant*invalid"}) == nil {
		t.Fatalf("tenant create succeeded while expecting error")
	}
	if contivClient.TenantPost(&client.Tenant{TenantName: "tenant^invalid"}) == nil {
		t.Fatalf("tenant create succeeded while expecting error")
	}

	// delete tenant
	err = contivClient.TenantDelete("tenant1")
	checkError(t, "delete tenant", err)
	err = contivClient.TenantDelete("tenant-valid")
	checkError(t, "delete tenant", err)

	// find again and make sure its gone
	_, err = contivClient.TenantGet("tenant1")
	if err == nil {
		t.Fatalf("Tenant was not deleted")
	}
}

// TestOverlappingSubnets tests overlapping network create/delete REST api
func TestOverlappingSubnets(t *testing.T) {
	// ensure global configs set
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")
	// Non-overlapping subnet for same tenant - vlan & gateway
	checkCreateNetwork(t, false, "default", "contiv1", "", "vlan", "10.1.1.0/24", "10.1.1.220", 1, "", "", "")
	checkCreateNetwork(t, false, "default", "contiv2", "", "vlan", "10.1.2.0/24", "10.1.2.254", 2, "", "", "")
	checkInspectNetwork(t, false, "default", "contiv1", "10.1.1.220", 1, 0)
	checkInspectNetwork(t, false, "default", "contiv2", "10.1.2.254", 2, 0)
	checkDeleteNetwork(t, false, "default", "contiv1")
	checkDeleteNetwork(t, false, "default", "contiv2")

	// overlapping subnet for same tenant -- vlan - vxlan combination
	checkCreateNetwork(t, false, "default", "contiv", "", "vlan", "10.1.1.0/24", "", 1, "", "", "")
	checkCreateNetwork(t, true, "default", "contiv1", "", "vxlan", "10.1.1.0/24", "", 2, "", "", "")
	checkInspectNetwork(t, false, "default", "contiv", "", 1, 0)
	checkDeleteNetwork(t, false, "default", "contiv")

	// Overlapping subnet for same tenant - vxlan
	checkCreateNetwork(t, false, "default", "contiv", "", "vxlan", "10.1.1.0/24", "", 1, "", "", "")
	checkCreateNetwork(t, true, "default", "contiv1", "", "vxlan", "10.1.1.0/24", "", 2, "", "", "")
	checkInspectNetwork(t, false, "default", "contiv", "", 1, 0)
	checkDeleteNetwork(t, false, "default", "contiv")

	// Overlapping subnet for same tenant - including gateway
	checkCreateNetwork(t, false, "default", "contiv", "", "vlan", "10.1.1.0/24", "", 1, "", "", "")
	checkCreateNetwork(t, true, "default", "contiv1", "", "vlan", "10.1.1.0/24", "10.1.1.233", 2, "", "", "")
	checkInspectNetwork(t, false, "default", "contiv", "", 1, 0)
	checkDeleteNetwork(t, false, "default", "contiv")

	// Overlapping subnet for same tenant - vlan
	checkCreateNetwork(t, false, "default", "contiv", "", "vlan", "10.1.1.0/16", "", 1, "", "", "")
	checkCreateNetwork(t, true, "default", "contiv1", "", "vlan", "10.1.2.0/24", "", 2, "", "", "")
	checkInspectNetwork(t, false, "default", "contiv", "", 1, 0)
	checkDeleteNetwork(t, false, "default", "contiv")

	// Overlapping subnet for different tenant - vlan
	checkCreateNetwork(t, false, "default", "contiv", "", "vlan", "10.1.1.0/24", "", 1, "", "", "")
	checkCreateTenant(t, false, "tenant1")
	checkCreateNetwork(t, false, "tenant1", "contiv1", "", "vlan", "10.1.0.0/16", "", 2, "", "", "")
	checkInspectNetwork(t, false, "default", "contiv", "", 1, 0)
	checkInspectNetwork(t, false, "tenant1", "contiv1", "", 2, 0)
	checkDeleteNetwork(t, false, "default", "contiv")
	checkDeleteNetwork(t, false, "tenant1", "contiv1")
	checkDeleteTenant(t, false, "tenant1")

	// Non-overlapping subnet ranges for same tenant
	checkCreateNetwork(t, false, "default", "contiv", "", "vlan", "20.1.1.10-20.1.1.45/24", "", 1, "", "", "")
	checkCreateNetwork(t, false, "default", "contiv2", "", "vlan", "20.1.1.46-20.1.1.100/24", "", 2, "", "", "")
	checkCreateNetwork(t, false, "default", "contiv3", "", "vlan", "20.1.1.0-20.1.1.9/24", "", 3, "", "", "")
	checkInspectNetwork(t, false, "default", "contiv", "", 1, 0)
	checkInspectNetwork(t, false, "default", "contiv2", "", 2, 0)
	checkInspectNetwork(t, false, "default", "contiv3", "", 3, 0)
	checkDeleteNetwork(t, false, "default", "contiv")
	checkDeleteNetwork(t, false, "default", "contiv2")
	checkDeleteNetwork(t, false, "default", "contiv3")

	// Overlapping subnet ranges for same tenant
	checkCreateNetwork(t, false, "default", "contiv", "", "vlan", "20.1.1.10-20.1.1.45/24", "", 1, "", "", "")
	checkCreateNetwork(t, true, "default", "contiv2", "", "vlan", "20.1.1.45-20.1.1.100/24", "", 2, "", "", "")
	checkCreateNetwork(t, true, "default", "contiv3", "", "vlan", "20.1.1.0-20.1.1.10/24", "", 3, "", "", "")
	checkInspectNetwork(t, false, "default", "contiv", "", 1, 0)
	checkDeleteNetwork(t, false, "default", "contiv")

	// Non-Overlapping subnetv6 for same tenant
	checkCreateNetwork(t, false, "default", "contiv", "", "vlan", "20.1.1.0/24", "", 1, "::/64", "", "")
	checkCreateNetwork(t, false, "default", "contiv1", "", "vlan", "20.1.2.0/24", "", 2, "1::/64", "", "")
	checkInspectNetwork(t, false, "default", "contiv", "", 1, 0)
	checkInspectNetwork(t, false, "default", "contiv1", "", 2, 0)
	checkDeleteNetwork(t, false, "default", "contiv")
	checkDeleteNetwork(t, false, "default", "contiv1")

	// Overlapping subnetv6 for same tenant
	checkCreateNetwork(t, false, "default", "contiv", "", "vlan", "20.1.1.0/24", "", 1, "2001:3332:3244:2422::/64", "", "")
	checkCreateNetwork(t, true, "default", "contiv1", "", "vlan", "20.1.2.0/24", "", 2, "2001:3332:3244:2422::/64", "", "")
	checkInspectNetwork(t, false, "default", "contiv", "", 1, 0)
	checkDeleteNetwork(t, false, "default", "contiv")

	// Overlapping subnetv6 for same tenant - vxlan
	checkCreateNetwork(t, false, "default", "contiv", "", "vxlan", "10.1.1.0/16", "", 1, "2001:2000::/32", "", "")
	checkCreateNetwork(t, true, "default", "contiv1", "", "vxlan", "10.2.0.0/16", "", 2, "2001:2000:1234:4422::/64", "", "")
	checkInspectNetwork(t, false, "default", "contiv", "", 1, 0)
	checkDeleteNetwork(t, false, "default", "contiv")

	// Oerlapping subnetv6 for different tenant
	checkCreateNetwork(t, false, "default", "contiv", "", "vlan", "10.1.1.0/24", "", 1, "ffff:ffff:ffff::/48", "", "")
	checkCreateTenant(t, false, "tenant1")
	checkCreateNetwork(t, false, "tenant1", "contiv1", "", "vlan", "10.1.0.0/16", "", 2, "ffff:ffff:ffff::/48", "", "")
	checkInspectNetwork(t, false, "default", "contiv", "", 1, 0)
	checkInspectNetwork(t, false, "tenant1", "contiv1", "", 2, 0)
	checkDeleteNetwork(t, false, "default", "contiv")
	checkDeleteNetwork(t, false, "tenant1", "contiv1")
	checkDeleteTenant(t, false, "tenant1")

}

// TestNetworkAddDeleteACIMode tests network create/delete REST api when ACI mode is on
func TestNetworkAddDeleteACIMode(t *testing.T) {

	// set aci mode
	checkGlobalSet(t, false, "aci", "1-4094", "1-10000", "bridge", "flood", "172.19.0.0/16")

	// Create Network and Delete it
	checkCreateNetwork(t, false, "default", "contiv", "", "vlan", "10.1.1.1/24", "10.1.1.254", 1, "", "", "")
	checkInspectNetwork(t, false, "default", "contiv", "10.1.1.254", 1, 0)
	checkInspectGlobal(t, false, "1", "")
	verifyNetworkState(t, "default", "contiv", "data", "vlan", "10.1.1.1", "10.1.1.254", 24, 1, 0, "", "", 0)
	checkDeleteNetwork(t, false, "default", "contiv")

	// Create network without gateway
	checkCreateNetwork(t, false, "default", "contiv-gw", "", "vxlan", "10.1.1.1/16", "", 1, "", "", "")
	checkInspectNetwork(t, false, "default", "contiv-gw", "", 1, 0)
	checkInspectGlobal(t, false, "", "1")
	verifyNetworkState(t, "default", "contiv-gw", "data", "vxlan", "10.1.1.1", "", 16, 1, 1, "", "", 0)
	checkDeleteNetwork(t, false, "default", "contiv-gw")

	// Create network with ipv6
	checkCreateNetwork(t, false, "default", "contiv-ipv6", "", "vxlan", "10.1.1.1/16", "", 1, "2016:0617::/120", "", "")
	verifyNetworkState(t, "default", "contiv-ipv6", "data", "vxlan", "10.1.1.1", "", 16, 1, 1, "2016:0617::", "", 120)
	checkDeleteNetwork(t, false, "default", "contiv-ipv6")
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")

}

// TestNetworkAddDelete tests network create/delete REST api
func TestNetworkAddDelete(t *testing.T) {
	// ensure global configs set
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")

	// Basic vlan network
	checkCreateNetwork(t, false, "default", "contiv", "", "vlan", "10.1.1.1/24", "10.1.1.254", 1, "", "", "")
	checkInspectNetwork(t, false, "default", "contiv", "10.1.1.254", 1, 0)
	checkInspectGlobal(t, false, "1", "")
	verifyNetworkState(t, "default", "contiv", "data", "vlan", "10.1.1.1", "10.1.1.254", 24, 1, 0, "", "", 0)
	checkDeleteNetwork(t, false, "default", "contiv")

	// Basic Vxlan network
	checkCreateNetwork(t, false, "default", "contiv", "", "vxlan", "10.1.1.1/16", "10.1.1.254", 1, "", "", "")
	checkInspectNetwork(t, false, "default", "contiv", "10.1.1.254", 1, 0)
	checkInspectGlobal(t, false, "", "1")
	verifyNetworkState(t, "default", "contiv", "data", "vxlan", "10.1.1.1", "10.1.1.254", 16, 1, 1, "", "", 0)
	checkDeleteNetwork(t, false, "default", "contiv")

	// Basic network with '-' in the name
	checkCreateNetwork(t, false, "default", "contiv-valid", "", "vxlan", "10.1.1.1/16", "10.1.1.254", 1, "", "", "")
	verifyNetworkState(t, "default", "contiv-valid", "data", "vxlan", "10.1.1.1", "10.1.1.254", 16, 1, 1, "", "", 0)
	checkDeleteNetwork(t, false, "default", "contiv-valid")

	// Basic network without gateway
	checkCreateNetwork(t, false, "default", "contiv-gw", "", "vxlan", "10.1.1.1/16", "", 1, "", "", "")
	checkInspectNetwork(t, false, "default", "contiv-gw", "", 1, 0)
	checkInspectGlobal(t, false, "", "1")
	verifyNetworkState(t, "default", "contiv-gw", "data", "vxlan", "10.1.1.1", "", 16, 1, 1, "", "", 0)
	checkDeleteNetwork(t, false, "default", "contiv-gw")

	// Basic network with ipv6
	checkCreateNetwork(t, false, "default", "contiv-ipv6", "", "vxlan", "10.1.1.1/16", "", 1, "2016:0617::/120", "", "")
	verifyNetworkState(t, "default", "contiv-ipv6", "data", "vxlan", "10.1.1.1", "", 16, 1, 1, "2016:0617::", "", 120)
	checkDeleteNetwork(t, false, "default", "contiv-ipv6")

	// Infra vlan network create and delete
	checkCreateNetwork(t, false, "default", "infraNw", "infra", "vlan", "10.1.1.1/24", "10.1.1.254", 1, "", "", "")
	checkInspectNetwork(t, false, "default", "infraNw", "10.1.1.254", 1, 0)
	checkInspectGlobal(t, false, "1", "")
	time.Sleep(time.Second)
	verifyNetworkState(t, "default", "infraNw", "infra", "vlan", "10.1.1.1", "10.1.1.254", 24, 1, 0, "", "", 0)
	checkDeleteNetwork(t, false, "default", "infraNw")
	time.Sleep(time.Second)

	// Try creating network with invalid names
	checkCreateNetwork(t, true, "default", "contiv:invalid", "infra", "vlan", "10.1.1.1/24", "10.1.1.254", 1, "", "", "")
	checkCreateNetwork(t, true, "default", "contiv|invalid", "infra", "vlan", "10.1.1.1/24", "10.1.1.254", 1, "", "", "")
	checkCreateNetwork(t, true, "default", "-invalid", "infra", "vlan", "10.1.1.1/24", "10.1.1.254", 1, "", "", "")

	// Try creating network with invalid network type
	checkCreateNetwork(t, true, "default", "infraNw", "infratest", "vlan", "10.1.1.1/24", "10.1.1.254", 1, "", "", "")
	checkCreateNetwork(t, true, "default", "infraNw", "testinfra", "vlan", "10.1.1.1/24", "10.1.1.254", 1, "", "", "")
	checkCreateNetwork(t, true, "default", "infraNw", "testdata", "vlan", "10.1.1.1/24", "10.1.1.254", 1, "", "", "")
	checkCreateNetwork(t, true, "default", "infraNw", "datatest", "vlan", "10.1.1.1/24", "10.1.1.254", 1, "", "", "")

	// Basic IP range network checks
	checkCreateNetwork(t, false, "default", "contiv", "data", "vxlan", "10.1.1.10-10.1.1.20/24", "10.1.1.254", 1, "", "", "")
	verifyNetworkState(t, "default", "contiv", "data", "vxlan", "10.1.1.10", "10.1.1.254", 24, 1, 1, "", "", 0)
	checkDeleteNetwork(t, false, "default", "contiv")
	checkCreateNetwork(t, false, "default", "contiv", "data", "vxlan", "10.1.1.10-10.1.5.254/16", "10.1.254.254", 1, "", "", "")
	verifyNetworkState(t, "default", "contiv", "data", "vxlan", "10.1.1.10", "10.1.254.254", 16, 1, 1, "", "", 0)
	checkDeleteNetwork(t, false, "default", "contiv")

	// Try invalid values for ip addr range
	checkCreateNetwork(t, true, "default", "contiv", "data", "vxlan", "10.1.1.10-20/24", "10.1.1.254", 1, "", "", "")
	checkCreateNetwork(t, true, "default", "contiv", "data", "vxlan", "10.1.1.10-10.1.20/24", "10.1.1.254", 1, "", "", "")
	checkCreateNetwork(t, true, "default", "contiv", "data", "vxlan", "10.1.1.10-10.1.1.1/24", "10.1.1.254", 1, "", "", "")
	checkCreateNetwork(t, true, "default", "contiv", "data", "vxlan", "10.1.1.10-10.1.2.30/24", "10.1.1.254", 1, "", "", "")
	checkCreateNetwork(t, true, "default", "contiv", "data", "vxlan", "10.1.1.10-10.2.1.30/16", "10.1.1.254", 1, "", "", "")
	checkCreateNetwork(t, true, "default", "contiv", "data", "vxlan", "10.1.1.10-10.1.1.1.30/24", "10.1.1.254", 1, "", "", "")
	checkCreateNetwork(t, true, "default", "contiv", "data", "vxlan", "10.1.1/24", "10.1.1.254", 1, "", "", "")
	checkCreateNetwork(t, true, "default", "contiv", "data", "vxlan", "10.1.1.256/24", "10.1.1.254", 1, "", "", "")

	// Valid values for network tag
	checkCreateNetwork(t, false, "default", "contiv", "data", "vxlan", "10.1.1.0/24", "10.1.1.254", 1, "", "", "contiv-net")
	verifyNetworkState(t, "default", "contiv", "data", "vxlan", "10.1.1.1", "10.1.1.254", 24, 1, 1, "", "", 0)
	checkDeleteNetwork(t, false, "default", "contiv")
	checkCreateNetwork(t, false, "default", "contiv", "data", "vxlan", "10.1.1.0/24", "10.1.1.254", 1, "", "", "io.contiv.network.test")
	verifyNetworkState(t, "default", "contiv", "data", "vxlan", "10.1.1.1", "10.1.1.254", 24, 1, 1, "", "", 0)
	checkDeleteNetwork(t, false, "default", "contiv")

	// Try network create with invalid network tag
	checkCreateNetwork(t, true, "default", "contiv", "data", "vxlan", "10.1.1.0/24", "10.1.1.254", 1, "", "", "tag=contiv-net")
	checkCreateNetwork(t, true, "default", "contiv", "data", "vxlan", "10.1.2.0/24", "10.1.1.254", 1, "", "", "io.contiv.network/test")

	// Try network create with invalid network range
	checkCreateNetwork(t, true, "default", "contiv", "data", "vxlan", "10.1.1.1-70/26", "10.1.1.63", 1, "", "", "")

	// Try network create with invalid subnet length
	checkCreateNetwork(t, true, "default", "contiv", "data", "vxlan", "10.1.1.1/32", "10.1.1.1", 1, "", "", "")
	checkCreateNetwork(t, true, "default", "contiv", "data", "vxlan", "10.1.1.1/24", "10.1.1.1", 1, "2016:0617::/128", "", "")

	// try creating network without tenant
	checkCreateNetwork(t, true, "tenant1", "contiv", "data", "vxlan", "10.1.1.1/24", "10.1.1.254", 1, "", "", "")

	// try invalid encap
	checkCreateNetwork(t, true, "default", "contiv", "data", "vvvv", "10.1.1.1/24", "10.1.1.254", 1, "", "", "")

	// try invalid pkt tags
	checkCreateNetwork(t, true, "default", "contiv", "data", "vlan", "10.1.1.1/24", "10.1.1.254", 5000, "", "", "")
	checkCreateNetwork(t, true, "default", "contiv", "data", "vxlan", "10.1.1.1/24", "10.1.1.254", 20000, "", "", "")

	// Try gateway outside the network
	checkCreateNetwork(t, true, "default", "contiv", "data", "vxlan", "10.1.1.1/24", "10.1.2.254", 1, "", "", "")
	checkCreateNetwork(t, true, "default", "contiv", "data", "vxlan", "10.1.1.65-70/26", "10.1.1.1", 2, "", "", "")

	// Try deleting a non-existing network
	checkDeleteNetwork(t, true, "default", "contiv")
}

func TestDynamicGlobalVlanRange(t *testing.T) {
	// ensure global configs set
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")

	// Basic vlan network creation
	checkCreateNetwork(t, false, "default", "contiv1", "", "vlan", "10.1.1.1/24", "10.1.1.254", 10, "", "", "")
	checkInspectNetwork(t, false, "default", "contiv1", "10.1.1.254", 10, 0)
	//checkInspectGlobal(t, false, "1", "")
	verifyNetworkState(t, "default", "contiv1", "data", "vlan", "10.1.1.1", "10.1.1.254", 24, 10, 0, "", "", 0)

	checkCreateNetwork(t, false, "default", "contiv2", "", "vlan", "11.1.1.1/24", "11.1.1.254", 11, "", "", "")
	checkInspectNetwork(t, false, "default", "contiv2", "11.1.1.254", 11, 0)
	//checkInspectGlobal(t, false, "2", "")
	verifyNetworkState(t, "default", "contiv2", "data", "vlan", "11.1.1.1", "11.1.1.254", 24, 11, 0, "", "", 0)

	checkCreateNetwork(t, false, "default", "contiv3", "", "vlan", "12.1.1.1/24", "12.1.1.254", 300, "", "", "")
	checkInspectNetwork(t, false, "default", "contiv3", "12.1.1.254", 300, 0)
	//checkInspectGlobal(t, false, "3", "")
	verifyNetworkState(t, "default", "contiv3", "data", "vlan", "12.1.1.1", "12.1.1.254", 24, 300, 0, "", "", 0)

	//Change global state
	checkGlobalSet(t, false, "default", "9-301", "1-10000", "bridge", "proxy", "172.19.0.0/16")

	//verify creation of network with active tag fails after change in global vlan range
	checkCreateNetwork(t, true, "default", "contiv4", "", "vlan", "13.1.1.1/24", "13.1.1.254", 10, "", "", "")

	//verify creation of network with out of range tag fails after change in global vlan range
	checkCreateNetwork(t, true, "default", "contiv5", "", "vlan", "14.1.1.1/24", "14.1.1.254", 302, "", "", "")

	//verify creation of network succeeds after vlan change
	checkCreateNetwork(t, false, "default", "contiv6", "", "vlan", "15.1.1.1/24", "15.1.1.254", 299, "", "", "")
	checkInspectNetwork(t, false, "default", "contiv6", "15.1.1.254", 299, 0)
	//checkInspectGlobal(t, false, "4", "")
	verifyNetworkState(t, "default", "contiv6", "data", "vlan", "15.1.1.1", "15.1.1.254", 24, 299, 0, "", "", 0)

	//check global state change fails if there are active tags
	checkGlobalSet(t, true, "default", "50-301", "1-10000", "bridge", "proxy", "172.19.0.0/16")
	//check global state change fails if there are active tags
	checkGlobalSet(t, true, "default", "1-200", "1-10000", "bridge", "proxy", "172.19.0.0/16")

	//check global vlan range can be changed to bigger range
	checkGlobalSet(t, false, "default", "1-800", "1-10000", "bridge", "flood", "172.19.0.0/16")
	checkCreateNetwork(t, false, "default", "contiv7", "", "vlan", "16.1.1.1/24", "16.1.1.254", 700, "", "", "")
	checkInspectNetwork(t, false, "default", "contiv7", "16.1.1.254", 700, 0)
	//checkInspectGlobal(t, false, "5", "")
	verifyNetworkState(t, "default", "contiv7", "data", "vlan", "16.1.1.1", "16.1.1.254", 24, 700, 0, "", "", 0)

	//Delete and recreate the network
	checkDeleteNetwork(t, false, "default", "contiv7")

	checkCreateNetwork(t, false, "default", "contiv7", "", "vlan", "16.1.1.1/24", "16.1.1.254", 700, "", "", "")
	checkInspectNetwork(t, false, "default", "contiv7", "16.1.1.254", 700, 0)
	//checkInspectGlobal(t, false, "5", "")
	verifyNetworkState(t, "default", "contiv7", "data", "vlan", "16.1.1.1", "16.1.1.254", 24, 700, 0, "", "", 0)

	checkDeleteNetwork(t, false, "default", "contiv7")
	checkDeleteNetwork(t, false, "default", "contiv6")
	checkDeleteNetwork(t, false, "default", "contiv3")
	checkDeleteNetwork(t, false, "default", "contiv2")
	checkDeleteNetwork(t, false, "default", "contiv1")

	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")

}

func TestDynamicGlobalVxlanRange(t *testing.T) {
	// ensure global configs set
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")
	insp, _ := contivClient.GlobalInspect("global") // Basic vxlan network creation

	log.Printf("THE GLOBAL DUMP: %#v \n", insp)
	checkCreateNetwork(t, false, "default", "contiv1", "", "vxlan", "10.1.1.1/24", "10.1.1.254", 1000, "", "", "")
	verifyNetworkState(t, "default", "contiv1", "data", "vxlan", "10.1.1.1", "10.1.1.254", 24, 1, 1000, "", "", 0)

	checkCreateNetwork(t, false, "default", "contiv2", "", "vxlan", "11.1.1.1/24", "11.1.1.254", 1001, "", "", "")
	verifyNetworkState(t, "default", "contiv2", "data", "vxlan", "11.1.1.1", "11.1.1.254", 24, 2, 1001, "", "", 0)

	checkCreateNetwork(t, false, "default", "contiv3", "", "vxlan", "12.1.1.1/24", "12.1.1.254", 2000, "", "", "")
	verifyNetworkState(t, "default", "contiv3", "data", "vxlan", "12.1.1.1", "12.1.1.254", 24, 3, 2000, "", "", 0)

	//Change global state
	checkGlobalSet(t, false, "default", "1-4094", "1000-2500", "bridge", "proxy", "172.19.0.0/16")

	//verify creation of network with active tag fails after change in global vlan range
	checkCreateNetwork(t, true, "default", "contiv4", "", "vxlan", "13.1.1.1/24", "13.1.1.254", 1000, "", "", "")

	//verify creation of network with out of range tag fails after change in global vlan range
	checkCreateNetwork(t, true, "default", "contiv5", "", "vxlan", "13.1.1.1/24", "13.1.1.254", 2501, "", "", "")

	//verify creation of network succeeds after vlan change
	checkCreateNetwork(t, false, "default", "contiv6", "", "vxlan", "14.1.1.1/24", "14.1.1.254", 2200, "", "", "")
	verifyNetworkState(t, "default", "contiv6", "data", "vxlan", "14.1.1.1", "14.1.1.254", 24, 4, 2200, "", "", 0)

	//check global state change fails if there are active tags
	checkGlobalSet(t, true, "default", "1-4094", "2000-10000", "bridge", "proxy", "172.19.0.0/16")
	//check global state change fails if there are active tags
	checkGlobalSet(t, true, "default", "1-4094", "1000-2000", "bridge", "proxy", "172.19.0.0/16")

	//check global vlan range can be changed to bigger range
	checkGlobalSet(t, false, "default", "1-4094", "1000-9000", "bridge", "proxy", "172.19.0.0/16")
	checkCreateNetwork(t, false, "default", "contiv7", "", "vxlan", "15.1.1.1/24", "15.1.1.254", 3200, "", "", "")
	verifyNetworkState(t, "default", "contiv7", "data", "vxlan", "15.1.1.1", "15.1.1.254", 24, 5, 3200, "", "", 0)

	//Delete and recreate the network
	checkDeleteNetwork(t, false, "default", "contiv7")

	checkCreateNetwork(t, false, "default", "contiv7", "", "vxlan", "16.1.1.1/24", "16.1.1.254", 3200, "", "", "")
	verifyNetworkState(t, "default", "contiv7", "data", "vxlan", "16.1.1.1", "16.1.1.254", 24, 5, 3200, "", "", 0)

	checkDeleteNetwork(t, false, "default", "contiv7")
	checkDeleteNetwork(t, false, "default", "contiv6")
	checkDeleteNetwork(t, false, "default", "contiv3")
	checkDeleteNetwork(t, false, "default", "contiv2")
	checkDeleteNetwork(t, false, "default", "contiv1")

	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")

}

// TestGlobalSetting tests global REST api
func TestGlobalSettingFwdMode(t *testing.T) {
	// set to default values (no-op)
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")

	// Create a vxlan network to verify you cannot change forward mode
	checkCreateNetwork(t, false, "default", "contiv7", "", "vxlan", "16.1.1.1/24", "16.1.1.254", 3200, "", "", "")

	// Should fail when changing the forwarding mode whenever there is a network
	var err error
	var expErr string
	err = checkGlobalSet(t, true, "default", "1-4094", "1-10000", "routing", "proxy", "172.19.0.0/16")
	expErr = "Unable to update forwarding mode due to existing 1 vxlans"
	if strings.TrimSpace(err.Error()) != expErr {
		t.Fatalf("Wrong error message, expected: '%v', got '%v'", expErr, err.Error())
	}
	// remove the vxlan network and add a vlan network
	checkDeleteNetwork(t, false, "default", "contiv7")
	checkCreateNetwork(t, false, "default", "contiv7", "", "vlan", "16.1.1.1/24", "16.1.1.254", 3200, "", "", "")
	err = checkGlobalSet(t, true, "default", "1-4094", "1-10000", "routing", "proxy", "172.19.0.0/16")
	expErr = "Unable to update forwarding mode due to existing 1 vlans"
	if strings.TrimSpace(err.Error()) != expErr {
		t.Fatalf("Wrong error message, expected: '%v', got '%v'", expErr, err.Error())
	}

	// remove the vlan network
	checkDeleteNetwork(t, false, "default", "contiv7")

	// make sure can change forwarding mode after network deleted
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "routing", "proxy", "172.21.0.0/16")

	// reset back to default values
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")
}

func TestGlobalSettingSubnet(t *testing.T) {
	// set to default values (no-op)
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")
	// Create a network to see if global subnet changes are blocked
	checkCreateNetwork(t, false, "default", "contiv7", "", "vxlan", "16.1.1.1/24", "16.1.1.254", 3200, "", "", "")

	var err error
	var expErr string
	// This should fail
	err = checkGlobalSet(t, true, "default", "1-4094", "1-10000", "bridge", "proxy", "172.21.0.0/16")
	expErr = "Unable to update private subnet due to existing 1 vxlans"
	if strings.TrimSpace(err.Error()) != expErr {
		t.Fatalf("Wrong error message, expected: '%v', got '%v'", expErr, err.Error())
	}

	// remove the vxlan network and add a vlan network
	checkDeleteNetwork(t, false, "default", "contiv7")
	checkCreateNetwork(t, false, "default", "contiv7", "", "vlan", "16.1.1.1/24", "16.1.1.254", 3200, "", "", "")

	// This should still fail
	err = checkGlobalSet(t, true, "default", "1-4094", "1-10000", "bridge", "proxy", "172.21.0.0/16")
	expErr = "Unable to update private subnet due to existing 1 vlans"
	if strings.TrimSpace(err.Error()) != expErr {
		t.Fatalf("Wrong error message, expected: '%v', got '%v'", expErr, err.Error())
	}

	// remove the network
	checkDeleteNetwork(t, false, "default", "contiv7")
	// make sure can change subnet after network deleted
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.21.0.0/16")
	// reset back to default values
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")
}

// TestGlobalSetting tests global REST api
func TestGlobalSetting(t *testing.T) {
	// set to default values (no-op)
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")

	// try basic modification
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.20.0.0/16")
	// set aci mode
	checkGlobalSet(t, false, "aci", "1-4094", "1-10000", "bridge", "flood", "172.20.0.0/16")

	// modify vlan/vxlan range
	checkGlobalSet(t, false, "default", "1-1000", "1001-2000", "bridge", "proxy", "172.20.0.0/16")

	// try invalid fabric mode
	checkGlobalSet(t, true, "xyz", "1-4094", "1-10000", "bridge", "proxy", "172.20.0.0/16")

	// try invalid vlan/vxlan range
	checkGlobalSet(t, true, "default", "1-5000", "1-10000", "bridge", "proxy", "172.20.0.0/16")
	checkGlobalSet(t, true, "default", "0-4094", "1-10000", "bridge", "proxy", "172.20.0.0/16")
	checkGlobalSet(t, true, "default", "1", "1-10000", "bridge", "proxy", "172.20.0.0/16")
	checkGlobalSet(t, true, "default", "1?2", "1-10000", "bridge", "proxy", "172.20.0.0/16")
	checkGlobalSet(t, true, "default", "abcd", "1-10000", "bridge", "proxy", "172.20.0.0/16")
	checkGlobalSet(t, true, "default", "1-4094", "1-100000", "bridge", "proxy", "172.20.0.0/16")
	checkGlobalSet(t, true, "default", "1-4094", "1-20000", "bridge", "proxy", "172.20.0.0/16")
	// modify pvt subnet
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.21.0.0/16")
	// Try invalid pvt subnet
	checkGlobalSet(t, true, "default", "1-4094", "1-10000", "bridge", "proxy", "172.21.0.0/24")

	// reset back to default values
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")
}

// TestAciGwSetting tests AciGw object REST api
func TestAciGwSetting(t *testing.T) {
	// basic create and delete
	checkAciGwSet(t, false, "yes", "yes", "topology/pod-1/paths-101/pathep-[eth1/14]", "", "testDom")
	checkAciGwDelete(t, false, "aciGw")

	checkAciGwSet(t, true, "yes", "yes", "pod-1/paths-101/pathep-[eth1/14]", "", "testDom")
	checkAciGwSet(t, false, "yes", "no", "topology/pod-1/paths-101/pathep-[eth1/14]", "", "testDom")
	checkAciGwSet(t, true, "yes", "no", "topology/pod-1/paths-101/pathep-[eth1/14],pod-1/paths-101/pathep-[eth1/15]", "", "testDom")
	checkAciGwSet(t, false, "yes", "no", "topology/pod-1/paths-101/pathep-[eth1/14],topology/pod-1/paths-101/pathep-[eth1/15]", "", "testDom")
	checkAciGwSet(t, true, "yes", "yes", "topology/pod-1/paths-101/pathep-[eth1/14],topology/pod-1/paths-101/pathep-[eth1/15]", "topology/tor-1/node-101", "testDom")
	checkAciGwSet(t, false, "yes", "yes", "topology/pod-1/paths-101/pathep-[eth1/14],topology/pod-1/paths-101/pathep-[eth1/15]", "topology/pod-1/node-101", "testDom")
	checkAciGwSet(t, false, "yes", "yes", "topology/pod-1/paths-101/pathep-[eth1/14],topology/pod-1/paths-101/pathep-[eth1/15],topology/pod-1/paths-102/pathep-[eth1/5],topology/pod-1/paths-101/pathep-[eth1/6]", "topology/pod-1/node-101,topology/pod-1/node-102,topology/pod-1/node-103", "testDom")

	// create an app-profile and verify aci delete is rejected.
	checkCreateNetwork(t, false, "default", "aci-net", "data", "vlan", "23.1.2.1/16", "23.1.2.254", 1, "", "", "")
	checkCreateEpg(t, false, "default", "aci-net", "epg1", []string{}, []string{}, "")
	checkCreateAppProfile(t, false, "default", "app-prof-1", []string{"epg1"})
	checkAciGwDelete(t, true, "aciGw")
	// delete the app-prof
	checkDeleteAppProfile(t, false, "default", "app-prof-1")
	checkAciGwDelete(t, false, "aciGw")
	checkDeleteEpg(t, false, "default", "", "epg1")
	checkDeleteNetwork(t, false, "default", "aci-net")
}

// TestNetworkPktRanges tests pkt-tag ranges in network REST api
func TestNetworkPktRanges(t *testing.T) {
	// ensure global configs set
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")

	// verify auto allocation of vlans
	checkCreateNetwork(t, false, "default", "contiv", "data", "vlan", "10.1.1.1/24", "10.1.1.254", 0, "", "", "")
	verifyNetworkState(t, "default", "contiv", "data", "vlan", "10.1.1.1", "10.1.1.254", 24, 1, 0, "", "", 0)
	checkDeleteNetwork(t, false, "default", "contiv")

	// auto allocation of vxlan
	checkCreateNetwork(t, false, "default", "contiv", "data", "vxlan", "10.1.1.1/24", "10.1.1.254", 0, "", "", "")
	verifyNetworkState(t, "default", "contiv", "data", "vxlan", "10.1.1.1", "10.1.1.254", 24, 1, 1, "", "", 0)
	checkCreateNetwork(t, false, "default", "contiv2", "data", "vxlan", "10.1.2.1/24", "10.1.2.254", 0, "", "", "")
	verifyNetworkState(t, "default", "contiv2", "data", "vxlan", "10.1.2.1", "10.1.2.254", 24, 2, 2, "", "", 0)
	checkCreateNetwork(t, false, "default", "contiv3", "data", "vxlan", "10.1.3.1/24", "10.1.3.254", 1000, "", "", "")
	verifyNetworkState(t, "default", "contiv3", "data", "vxlan", "10.1.3.1", "10.1.3.254", 24, 3, 1000, "", "", 0)
	checkDeleteNetwork(t, false, "default", "contiv")
	checkDeleteNetwork(t, false, "default", "contiv2")
	checkDeleteNetwork(t, false, "default", "contiv3")

	// verify duplicate values fail
	checkCreateNetwork(t, false, "default", "contiv1", "data", "vlan", "10.1.1.1/24", "10.1.1.254", 1, "", "", "")
	checkCreateNetwork(t, true, "default", "contiv2", "data", "vlan", "10.1.1.1/24", "10.1.1.254", 1, "", "", "")
	checkDeleteNetwork(t, false, "default", "contiv1")

	checkCreateNetwork(t, false, "default", "contiv1", "data", "vxlan", "10.1.1.1/24", "10.1.1.254", 0, "", "", "")
	checkCreateNetwork(t, true, "default", "contiv2", "data", "vxlan", "10.1.1.1/24", "10.1.1.254", 1, "", "", "")
	checkDeleteNetwork(t, false, "default", "contiv1")

	// shrink ranges and try allocating
	checkGlobalSet(t, false, "default", "100-1000", "1001-2000", "bridge", "proxy", "172.20.0.0/16")
	checkCreateNetwork(t, true, "default", "contiv1", "data", "vlan", "10.1.1.1/24", "10.1.1.254", 1001, "", "", "")
	checkCreateNetwork(t, true, "default", "contiv1", "data", "vlan", "10.1.1.1/24", "10.1.1.254", 99, "", "", "")
	checkCreateNetwork(t, true, "default", "contiv2", "data", "vxlan", "10.1.2.1/24", "10.1.2.254", 2001, "", "", "")
	checkCreateNetwork(t, true, "default", "contiv2", "data", "vxlan", "10.1.2.1/24", "10.1.2.254", 1000, "", "", "")

	// reset back to default values
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")
}

// TestPolicyRules tests policy and rule REST objects
func TestPolicyRules(t *testing.T) {
	// ensure global configs set
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")

	containerID1 := "723e55bf5b244f47c1b184cb786a1c2ad8870cc3a3db723c49ac09f68a9d1e69"
	ep1 := "657355bf5b244f47c1b184cb786a14535d8870cc3a3db723c49ac09f68a9d6a5"
	checkCreateNetwork(t, false, "default", "contiv", "data", "vxlan", "10.1.1.1/16", "10.1.1.254", 1, "", "", "")
	checkCreateEpg(t, false, "default", "contiv", "group1", []string{}, []string{}, "")
	createEPinEPG(t, "10.1.1.15", "default", "group1", containerID1, "default", ep1, []string{})

	// create policy
	checkCreatePolicy(t, false, "default", "policy1")

	// verify policy on unknown tenant fails
	checkCreatePolicy(t, true, "tenant1", "policy1")

	// add rules
	checkCreateRule(t, false, "default", "policy1", "1", "in", "", "", "", "", "", "", "tcp", "allow", 1, 80)
	checkCreateRule(t, false, "default", "policy1", "2", "in", "", "", "", "", "", "", "", "deny", 1, 0)
	checkCreateRule(t, false, "default", "policy1", "3", "out", "", "", "", "", "", "", "tcp", "allow", 1, 80)
	checkCreateRule(t, false, "default", "policy1", "4", "in", "", "", "10.1.1.1/24", "", "", "", "tcp", "allow", 1, 80)
	checkCreateRule(t, false, "default", "policy1", "5", "out", "", "", "", "", "", "10.1.1.1/24", "tcp", "allow", 1, 80)
	checkCreateRule(t, false, "default", "policy1", "6", "in", "", "group1", "", "", "", "", "", "deny", 1, 0)
	checkCreateRule(t, false, "default", "policy1", "7", "out", "", "", "", "", "group1", "", "tcp", "allow", 1, 80)

	// verify --to-ip, no linked epg fails
	checkCreateRule(t, true, "default", "policy1", "to-ip", "in", "", "", "10.1.1.15", "", "", "10.2.1.31", "tcp", "allow", 1, 80)
	// verify --to-ip not in epg fails
	checkCreateEpg(t, false, "default", "contiv", "group1", []string{"policy1"}, []string{}, "")
	checkCreateRule(t, true, "default", "policy1", "to-ip", "in", "", "", "10.2.1.115", "", "", "10.1.1.19", "tcp", "allow", 1, 80)
	checkCreateRule(t, false, "default", "policy1", "to-ip", "in", "", "", "10.2.1.11", "", "", "10.1.1.15", "tcp", "allow", 1, 80)
	checkCreateEpg(t, false, "default", "contiv", "group1", []string{}, []string{}, "")

	// verify duplicate rule id fails
	checkCreateRule(t, true, "default", "policy1", "1", "in", "", "", "", "", "", "", "tcp", "allow", 1, 80)

	// verify unknown directions fail
	checkCreateRule(t, true, "default", "policy1", "100", "both", "", "", "", "", "", "", "tcp", "allow", 1, 0)
	checkCreateRule(t, true, "default", "policy1", "100", "xyz", "", "", "", "", "", "", "tcp", "allow", 1, 0)

	// verify unknown protocol fails
	checkCreateRule(t, true, "default", "policy1", "100", "in", "", "", "", "", "", "", "xyz", "allow", 1, 80)

	// verify unknown action fails
	checkCreateRule(t, true, "default", "policy1", "100", "in", "", "", "", "", "", "", "tcp", "xyz", 1, 80)
	checkCreateRule(t, true, "default", "policy1", "100", "in", "", "", "", "", "", "", "tcp", "accept", 1, 80)

	// verify rule on unknown tenant/policy fails
	checkCreateRule(t, true, "default", "policy2", "100", "in", "", "", "", "", "", "", "", "allow", 1, 0)
	checkCreateRule(t, true, "tenant", "policy1", "100", "in", "", "", "", "", "", "", "", "allow", 1, 0)

	// verify invalid to/from and direction combos fail
	checkCreateRule(t, true, "default", "policy1", "100", "in", "", "", "", "invalid", "", "", "tcp", "allow", 1, 80)
	checkCreateRule(t, true, "default", "policy1", "100", "in", "", "", "", "", "invalid", "", "tcp", "allow", 1, 80)
	checkCreateRule(t, true, "default", "policy1", "100", "in", "", "", "", "", "", "invalid", "tcp", "allow", 1, 80)
	checkCreateRule(t, true, "default", "policy1", "100", "out", "invalid", "", "", "", "", "", "tcp", "allow", 1, 80)
	checkCreateRule(t, true, "default", "policy1", "100", "out", "", "invalid", "", "", "", "", "tcp", "allow", 1, 80)
	checkCreateRule(t, true, "default", "policy1", "100", "out", "", "", "invalid", "", "", "", "tcp", "allow", 1, 80)

	// verify cant specify both from/to network and from/to ip addresses
	checkCreateRule(t, true, "default", "policy1", "100", "in", "contiv", "", "10.1.1.1/24", "", "", "", "tcp", "allow", 1, 80)
	checkCreateRule(t, true, "default", "policy1", "100", "out", "", "", "", "contiv", "", "10.1.1.1/24", "tcp", "allow", 1, 80)

	// verify specifying epg and network fails
	checkCreateRule(t, true, "default", "policy1", "100", "in", "contiv", "group1", "", "", "", "", "tcp", "allow", 1, 80)
	checkCreateRule(t, true, "default", "policy1", "100", "out", "", "", "", "contiv", "group1", "", "tcp", "allow", 1, 80)
	// verify cant match on non-existing networks
	checkCreateRule(t, true, "default", "policy1", "100", "in", "invalid", "", "", "", "", "", "tcp", "allow", 1, 80)
	checkCreateRule(t, true, "default", "policy1", "100", "out", "", "", "", "invalid", "", "", "tcp", "allow", 1, 80)

	// verify cant match on non-existing EPGs
	checkCreateRule(t, true, "default", "policy1", "100", "in", "", "invalid", "", "", "", "", "tcp", "allow", 1, 80)
	checkCreateRule(t, true, "default", "policy1", "100", "out", "", "", "", "", "invalid", "", "tcp", "allow", 1, 80)

	// checkCreateRule(t, true, tenant, policy, ruleID, dir, fnet, fepg, fip, tnet, tepg, tip, proto, prio, port)

	// delete rules
	checkDeleteRule(t, false, "default", "policy1", "to-ip")
	checkDeleteRule(t, false, "default", "policy1", "1")
	checkDeleteRule(t, false, "default", "policy1", "2")
	checkDeleteRule(t, false, "default", "policy1", "3")
	checkDeleteRule(t, false, "default", "policy1", "4")
	checkDeleteRule(t, false, "default", "policy1", "5")
	checkDeleteRule(t, false, "default", "policy1", "6")
	checkDeleteRule(t, false, "default", "policy1", "7")

	// verify cant delete a rule and policy that doesnt exist
	checkDeleteRule(t, true, "default", "policy1", "100")
	checkDeletePolicy(t, true, "default", "policy2")

	// delete policy
	checkDeletePolicy(t, false, "default", "policy1")

	deleteEP(t, "default", "default", ep1)
	// delete the EPG
	checkDeleteEpg(t, false, "default", "contiv", "group1")
	// delete the network
	checkDeleteNetwork(t, false, "default", "contiv")
}

// TestEpgPolicies tests attaching policy to EPG
func TestEpgPolicies(t *testing.T) {
	// ensure global configs set
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")

	// create network
	checkCreateNetwork(t, false, "default", "contiv", "data", "vxlan", "10.1.1.1/16", "10.1.1.254", 1, "", "", "")

	// create policy
	checkCreatePolicy(t, false, "default", "policy1")

	// add rules
	checkCreateRule(t, false, "default", "policy1", "1", "in", "contiv", "", "", "", "", "", "tcp", "allow", 1, 80)
	checkCreateRule(t, false, "default", "policy1", "2", "in", "contiv", "", "", "", "", "", "", "deny", 1, 0)
	checkCreateRule(t, false, "default", "policy1", "3", "out", "", "", "", "contiv", "", "", "tcp", "allow", 1, 80)

	// create EPG and attach policy to it
	checkCreateEpg(t, false, "default", "contiv", "group1", []string{"policy1"}, []string{}, "")
	verifyEpgPolicy(t, "default", "contiv", "group1", "policy1")

	// create a policy and rule that matches on other policy
	checkCreatePolicy(t, false, "default", "policy2")
	checkCreateRule(t, false, "default", "policy2", "1", "in", "", "group1", "", "", "", "", "tcp", "allow", 1, 80)
	checkCreateRule(t, false, "default", "policy2", "2", "out", "", "", "", "", "group1", "", "tcp", "allow", 1, 80)
	checkCreateRule(t, false, "default", "policy2", "3", "in", "contiv", "", "", "", "", "", "tcp", "allow", 1, 80)
	checkCreateRule(t, false, "default", "policy2", "4", "out", "", "", "", "contiv", "", "", "tcp", "allow", 1, 80)
	checkCreateEpg(t, false, "default", "contiv", "group2", []string{"policy2"}, []string{}, "")
	verifyEpgPolicy(t, "default", "contiv", "group2", "policy2")

	// verify cant create/update EPGs that uses non-existing policies
	checkCreateEpg(t, true, "default", "contiv", "group3", []string{"invalid"}, []string{}, "")
	checkCreateEpg(t, true, "default", "contiv", "group2", []string{"invalid"}, []string{}, "")

	// verify cant create EPGs without tenant/network
	checkCreateEpg(t, true, "invalid", "contiv", "group3", []string{}, []string{}, "")
	checkCreateEpg(t, true, "default", "invalid", "group3", []string{}, []string{}, "")

	// verify name clash between network and epg is rejected
	checkCreateEpg(t, true, "default", "contiv", "contiv", []string{}, []string{}, "")
	checkCreateNetwork(t, true, "default", "group1", "data", "vxlan", "20.1.1.1/16", "20.1.1.254", 1, "", "", "")
	// verify network association cant be changed on epg
	checkCreateNetwork(t, false, "default", "newnet", "data", "vxlan", "20.1.1.1/16", "20.1.1.254", 2, "", "", "")
	checkCreateEpg(t, true, "default", "newnet", "group1", []string{}, []string{}, "")

	// change policy and verify EPG policy changes
	checkCreateEpg(t, false, "default", "contiv", "group3", []string{"policy1"}, []string{}, "")
	checkCreateEpg(t, false, "default", "contiv", "group3", []string{"policy2"}, []string{}, "")
	checkEpgPolicyDeleted(t, "default", "contiv", "group3", "policy1")
	verifyEpgPolicy(t, "default", "contiv", "group3", "policy2")

	// delete the EPG
	checkDeleteEpg(t, false, "default", "contiv", "group1")
	checkDeleteEpg(t, false, "default", "contiv", "group2")
	checkDeleteEpg(t, false, "default", "contiv", "group3")

	// verify epg policies are gone
	checkEpgPolicyDeleted(t, "default", "contiv", "group1", "policy1")
	checkEpgPolicyDeleted(t, "default", "contiv", "group2", "policy2")
	checkEpgPolicyDeleted(t, "default", "contiv", "group3", "policy2")

	// delete the policy
	checkDeletePolicy(t, false, "default", "policy1")
	checkDeletePolicy(t, false, "default", "policy2")

	// delete the network
	checkDeleteNetwork(t, false, "default", "contiv")
	checkDeleteNetwork(t, false, "default", "newnet")
}

// TestExtContractsGroups tests management of external contracts groups
func TestExtContractsGroups(t *testing.T) {
	// ensure global configs set
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")
	// create network for the test
	checkCreateNetwork(t, false, "default", "test-net", "data", "vlan", "23.1.1.1/16", "23.1.1.254", 1, "", "", "")
	// create contract groups used for the test
	checkCreateExtContractsGrp(t, false, "default", "ext-contracts-prov", "provided", []string{"uni/tn-common/brc-default", "uni/tn-common/brc-icmp-contract"})
	checkCreateExtContractsGrp(t, false, "default", "ext-contracts-cons", "consumed", []string{"uni/tn-common/brc-default", "uni/tn-common/brc-icmp-contract"})
	// Try creating a contract group which is neither "provided" nor "consumed"
	checkCreateExtContractsGrp(t, true, "default", "ext-contracts-blah", "something", []string{"uni/tn-common/brc-default", "uni/tn-common/brc-icmp-contract"})

	// epg can have a provided contract group
	checkCreateEpg(t, false, "default", "test-net", "group1", []string{}, []string{"ext-contracts-prov"}, "")
	// epg can have a consumed contract group
	checkCreateEpg(t, false, "default", "test-net", "group2", []string{}, []string{"ext-contracts-cons"}, "")
	// epg can have both provided and consumed contract groups
	checkCreateEpg(t, false, "default", "test-net", "group3", []string{}, []string{"ext-contracts-prov", "ext-contracts-cons"}, "")
	// Try deleting a contract group when it is being used by an EPG. Should fail
	checkDeleteExtContractsGrp(t, true, "default", "ext-contracts-prov")
	// Try creating an EPG with a contract group that does not exist. Must fail
	checkCreateEpg(t, true, "default", "test-net", "group4", []string{}, []string{"ext-contracts-blah"}, "")

	// create an app profile with the epgs with external contracts
	checkCreateAppProfile(t, false, "default", "app-prof-test", []string{"group1", "group2", "group3"})

	// delete the app profile
	checkDeleteAppProfile(t, false, "default", "app-prof-test")

	// delete the groups
	checkDeleteEpg(t, false, "default", "test-net", "group1")
	checkDeleteEpg(t, false, "default", "test-net", "group2")
	checkDeleteEpg(t, false, "default", "test-net", "group3")
	// delete the external contract groups.
	// since there are no references any more, they should be deleted.
	checkDeleteExtContractsGrp(t, false, "default", "ext-contracts-prov")
	checkDeleteExtContractsGrp(t, false, "default", "ext-contracts-cons")
	checkDeleteNetwork(t, false, "default", "test-net")
}

// TestAppProfile tests app-profile REST objects
func TestAppProfile(t *testing.T) {
	// ensure global configs set
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")
	// Create two networks and 3 epgs
	checkCreateNetwork(t, false, "default", "net1", "data", "vlan", "10.1.1.1/16", "10.1.1.254", 1, "", "", "")
	checkCreateNetwork(t, false, "default", "net2", "data", "vlan", "20.1.1.1/16", "20.1.1.254", 2, "", "", "")
	checkCreateEpg(t, false, "default", "net1", "group1", []string{}, []string{}, "")
	checkCreateEpg(t, false, "default", "net1", "group2", []string{}, []string{}, "")
	checkCreateEpg(t, false, "default", "net2", "group3", []string{}, []string{}, "")
	checkCreateAppProfile(t, false, "default", "profile1", []string{})
	checkCreateAppProfile(t, false, "default", "profile2", []string{"group1"})
	checkCreateAppProfile(t, false, "default", "profile3", []string{"group1", "group3"})
	// Verify epg cant be deleted while part of app profile
	checkDeleteEpg(t, true, "default", "net1", "group1")
	verifyAppProfile(t, false, "default", "profile3", []string{"group1", "group3"})
	checkCreateAppProfile(t, false, "default", "profile3", []string{"group1", "group2", "group3"})
	verifyAppProfile(t, false, "default", "profile3", []string{"group1", "group2", "group3"})
	checkCreateAppProfile(t, true, "default", "profile4", []string{"group1", "invalid"})
	verifyAppProfile(t, true, "default", "profile4", []string{})
	verifyAppProfile(t, false, "default", "profile2", []string{"group1"})
	verifyAppProfile(t, false, "default", "profile1", []string{})
	checkDeleteAppProfile(t, false, "default", "profile1")
	checkDeleteAppProfile(t, false, "default", "profile2")
	checkDeleteAppProfile(t, false, "default", "profile3")
	checkDeleteEpg(t, false, "default", "net1", "group1")
	checkDeleteEpg(t, false, "default", "net1", "group2")
	checkDeleteEpg(t, false, "default", "net2", "group3")
	checkDeleteNetwork(t, false, "default", "net1")
	checkDeleteNetwork(t, false, "default", "net2")
}

//TestEpgNetprofile tests the netprofile netprofile REST objects.
func TestEpgnpTenant(t *testing.T) {
	// ensure global configs set
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")

	//create a network,
	checkCreateNetwork(t, false, "default", "np-net", "data", "vxlan", "10.1.1.1/24", "10.1.1.254", 1, "", "", "")

	//verify that groups cannot be created with invalid/without tenantname.
	checkCreateEpgNp(t, true, "invalid", "netp", "np-net", "g1", []string{})
	checkCreateEpgNp(t, true, "", "netp", "np-net", "g1", []string{})

	//verify that netprofile can be attached to a tenant.
	checkCreateTenant(t, false, "blue")
	checkCreateNetwork(t, false, "blue", "netT", "data", "vlan", "10.1.2.1/24", "10.1.2.254", 1, "", "", "")
	checkCreateNetProfile(t, false, 3, 1500, "5Mb", "netprofile", "blue")
	checkCreateEpgNp(t, false, "blue", "netprofile", "netT", "g1", []string{})
	verifyEpgnetProfile(t, "blue", "g1", "5Mb", 3, 1500)

	//verify that netprofile can be updated which reflects in the attached epg under that tenant.
	checkCreateNetProfile(t, false, 9, 1650, "900Mb", "netprofile", "blue")
	verifyEpgnetProfile(t, "blue", "g1", "900Mb", 9, 1650)

	//verify that tenant cannot be deleted when it it linked to another network/netprofile.
	checkDeleteTenant(t, true, "blue")

	//detach the netprofile and check.
	checkCreateEpgNp(t, false, "blue", "", "netT", "g1", []string{})
	checkverifyNetProfile(t, false, 9, 1650, "900Mb", "netprofile", "blue")
	verifyEpgnetProfile(t, "blue", "g1", "", 0, 0)

	//verify groups cannot be created with invalid/without networkname.
	checkCreateEpgNp(t, true, "default", "netp", "invalid", "g2", []string{})
	checkCreateEpgNp(t, true, "default", "netp", "", "g2", []string{})

	//verify that groups cannot be created with invalid/without group name.
	checkCreateEpgNp(t, true, "default", "netp", "np-net", "in_valid", []string{})
	checkCreateEpgNp(t, true, "default", "netp", "np-net", "", []string{})

	//delete netprofile and network
	checkDeleteEpg(t, false, "blue", "netT", "g1")
	checkDeleteNetProfile(t, false, "netprofile", "blue")
	checkDeleteNetwork(t, false, "default", "np-net")
	checkDeleteNetwork(t, false, "blue", "netT")
	checkDeleteTenant(t, false, "blue")

	//add the same network , netprofile and group to the tenant and verify there are no stale variables left.
	checkCreateTenant(t, false, "blue")
	checkCreateNetwork(t, false, "blue", "netT", "data", "vlan", "10.1.2.1/24", "10.1.2.254", 1, "", "", "")
	checkCreateNetProfile(t, false, 3, 1500, "5Mb", "netprofile", "blue")
	checkCreateEpgNp(t, false, "blue", "netprofile", "netT", "g1", []string{})
	verifyEpgnetProfile(t, "blue", "g1", "5Mb", 3, 1500)

	//delete netprofile and network
	checkDeleteEpg(t, false, "blue", "netT", "g1")
	checkDeleteNetProfile(t, false, "netprofile", "blue")
	checkDeleteNetwork(t, false, "blue", "netT")
	checkDeleteTenant(t, false, "blue")

}

func TestEpgnp(t *testing.T) {
	// ensure global configs set
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")

	//create a network and  netprofile
	checkCreateNetwork(t, false, "default", "np-net", "data", "vxlan", "10.1.1.1/24", "10.1.1.254", 1, "", "", "")
	checkCreateNetProfile(t, false, 5, 1500, "2gbps", "netprofile", "default")

	//verify that groups cannot be created with invalid netprofile. & can be created without netprofile
	checkCreateEpgNp(t, true, "default", "invalid", "np-net", "group1", []string{})
	checkCreateEpgNp(t, false, "default", "", "np-net", "group1", []string{})

	//delete group, netprofile and network
	checkDeleteEpg(t, false, "default", "np-net", "group1")
	checkDeleteNetProfile(t, false, "netprofile", "default")
	checkDeleteNetwork(t, false, "default", "np-net")
}

func TestEpgUpdate(t *testing.T) {
	// ensure global configs set
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")

	//create a network, netprofile and group.
	checkCreateNetwork(t, false, "default", "np-net", "data", "vxlan", "10.1.1.1/24", "10.1.1.254", 1, "", "", "")
	checkCreateNetProfile(t, false, 5, 1500, "2gbps", "netprofile", "default")
	checkCreateEpgNp(t, false, "default", "", "np-net", "group1", []string{})

	//create a group and attach a netprofile.
	checkCreateEpgNp(t, false, "default", "netprofile", "np-net", "group1", []string{})
	verifyEpgnetProfile(t, "default", "group1", "2gbps", 5, 1500)

	//create another netprofile and group, attach netprofile to that group.
	checkCreateNetProfile(t, false, 2, 1500, "5gbps", "netp", "default")
	checkCreateEpgNp(t, false, "default", "netp", "np-net", "NpGroup", []string{})
	verifyEpgnetProfile(t, "default", "NpGroup", "5gbps", 2, 1500)

	//update netp and check if group is getting updated.
	checkCreateNetProfile(t, false, 7, 1500, "10gbps", "netp", "default")
	verifyEpgnetProfile(t, "default", "NpGroup", "10gbps", 7, 1500)

	//attach netp to group1.
	checkCreateEpgNp(t, false, "default", "netp", "np-net", "group1", []string{})
	verifyEpgnetProfile(t, "default", "group1", "10gbps", 7, 1500)

	//attach netprofile to Npgroup.
	checkCreateEpgNp(t, false, "default", "netprofile", "np-net", "NpGroup", []string{})
	verifyEpgnetProfile(t, "default", "NpGroup", "2gbps", 5, 1500)

	//detach the netprofile from Npgroup.
	checkCreateEpgNp(t, false, "default", "", "np-net", "NpGroup", []string{})
	checkEpgnetProfileDetached(t, "default", "np-net", "NpGroup", "netprofile")
	verifyEpgnetProfile(t, "default", "NpGroup", "", 0, 0)

	//detach the netprofile from group1.
	checkCreateEpgNp(t, false, "default", "", "np-net", "group1", []string{})
	checkEpgnetProfileDetached(t, "default", "np-net", "group1", "netp")
	verifyEpgnetProfile(t, "default", "group1", "", 0, 0)

	//attach the netprofile to another group.
	checkCreateEpgNp(t, false, "default", "netprofile", "np-net", "group1", []string{})
	verifyEpgnetProfile(t, "default", "group1", "2gbps", 5, 1500)

	//delete the netprofile as no group is using it
	checkDeleteNetProfile(t, false, "netp", "default")

	//delete the group and network
	checkDeleteEpg(t, false, "default", "np-net", "group1")
	checkDeleteEpg(t, false, "default", "np-net", "NpGroup")
	checkDeleteNetProfile(t, false, "netprofile", "default")
	checkDeleteNetwork(t, false, "default", "np-net")
}

func TestDeleteEpgNp(t *testing.T) {
	// ensure global configs set
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")

	//create a network, netprofile and group.
	checkCreateNetwork(t, false, "default", "np-net", "data", "vxlan", "10.1.1.1/24", "10.1.1.254", 1, "", "", "")
	checkCreateNetProfile(t, false, 5, 1500, "2gbps", "netprofile", "default")
	checkCreateEpgNp(t, false, "default", "netprofile", "np-net", "group1", []string{})

	//try deleting netprofile when the group is using it
	checkDeleteNetProfile(t, true, "netprofile", "default")

	//update the group without any netprofile and check if np has been detached.
	checkCreateEpgNp(t, false, "default", "", "np-net", "group1", []string{})
	checkEpgnetProfileDetached(t, "default", "np-net", "group1", "netprofile")

	//delete group,nerofile and network
	checkDeleteEpg(t, false, "default", "np-net", "group1")
	checkDeleteNetProfile(t, false, "netprofile", "default")
	checkDeleteNetwork(t, false, "default", "np-net")

}

func TestNetProfileupdate(t *testing.T) {
	// ensure global configs set
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")

	//create a network, netprofile.
	checkCreateNetwork(t, false, "default", "np-net", "data", "vxlan", "10.1.1.1/24", "10.1.1.254", 1, "", "", "")
	checkCreateNetProfile(t, false, 5, 1500, "2gbps", "netprofile", "default")
	checkCreateNetProfile(t, false, 3, 1500, "10gbps", "netprofile1", "default")
	checkCreateEpgNp(t, false, "default", "netprofile", "np-net", "group1", []string{})

	//verify that same group can be assigned to another netprofile.
	checkCreateEpgNp(t, false, "default", "netprofile1", "np-net", "groupNp", []string{})

	//update the netprofile && check if epg has been updated
	checkCreateNetProfile(t, false, 5, 1500, "10gbps", "netprofile", "default")
	checkverifyNetProfile(t, false, 5, 1500, "10gbps", "netprofile", "default")
	verifyEpgnetProfile(t, "default", "group1", "10gbps", 5, 1500)

	//update with dscp 0
	checkCreateNetProfile(t, false, 0, 150, "10gbps", "netprofile", "default")
	verifyEpgnetProfile(t, "default", "group1", "10gbps", 0, 150)

	//update with no bandwidth(Default bandwidth)
	checkCreateNetProfile(t, false, 0, 1500, "", "netprofile", "default")
	verifyEpgnetProfile(t, "default", "group1", "", 0, 1500)

	//update netprofile without burst value.
	checkCreateNetProfile(t, false, 0, 0, "20mb", "netprofile", "default")
	verifyEpgnetProfile(t, "default", "group1", "20mb", 0, 0)

	//delete group,nerofile and network
	checkDeleteEpg(t, false, "default", "np-net", "group1")
	checkDeleteEpg(t, false, "default", "np-net", "groupNp")
	checkDeleteNetProfile(t, false, "netprofile", "default")
	checkDeleteNetProfile(t, false, "netprofile1", "default")
	checkDeleteNetwork(t, false, "default", "np-net")
}

func TestNetprofile(t *testing.T) {
	// ensure global configs set
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")

	//create a network & netprofile
	checkCreateNetwork(t, false, "default", "net", "data", "vxlan", "10.1.1.1/24", "10.1.1.254", 1, "", "", "")
	checkCreateNetProfile(t, false, 5, 1500, "2gbps", "profile1", "default")

	//verify that bandwidth and DSCP can be updated to a particular netprofile
	checkCreateNetProfile(t, false, 6, 1500, "5gbps", "profile1", "default")

	//verify that a netprofile can be created without dscp & bandwidth.
	checkCreateNetProfile(t, false, 0, 1500, "", "profile2", "default")

	//verify that netprofile cannot be created without proper bandwidth
	checkCreateNetProfile(t, true, 5, 1500, "invalid", "profile3", "default")
	//verify that netprofile cannot be created with invalid DSCP
	checkCreateNetProfile(t, true, 72, 1500, "2gbps", "profile2", "default")
	//verify netprofile cannot be created with an invalid burst size 2 - 10Mbyte
	checkCreateNetProfile(t, true, 72, 1, "2gbps", "profile2", "default")
	//verify that burst size can be given in integer format.
	checkCreateNetProfile(t, false, 22, 450, "20mbps", "profile2", "default")

	//verify that bandwidth is not case sensitive and user friendly format.
	checkCreateNetProfile(t, false, 5, 1500, "10 Gbps", "profile3", "default")
	checkCreateNetProfile(t, false, 5, 1500, "10 Gb", "profile3", "default")
	checkCreateNetProfile(t, false, 5, 1500, "10 gb", "profile3", "default")
	checkCreateNetProfile(t, false, 5, 1500, "5Gb", "profile3", "default")
	checkCreateNetProfile(t, false, 5, 1500, "6gb", "profile3", "default")
	checkCreateNetProfile(t, false, 5, 1500, "3gbps", "profile3", "default")
	checkCreateNetProfile(t, false, 5, 1500, "3Gbps", "profile3", "default")
	checkCreateNetProfile(t, false, 5, 1500, "4g", "profile3", "default")
	checkCreateNetProfile(t, false, 5, 1500, "4G", "profile3", "default")

	//netprofile must throw error for invalid bandwidth format.
	checkCreateNetProfile(t, true, 5, 1500, "10 Gkms", "profile3", "default")
	checkCreateNetProfile(t, true, 5, 1500, "10 G12ms", "profile3", "default")
	checkCreateNetProfile(t, true, 5, 1500, "10 ms", "profile3", "default")
	checkCreateNetProfile(t, true, 5, 1500, "10 bms", "profile3", "default")
	checkCreateNetProfile(t, true, 5, 1500, "Gkms", "profile3", "default")
	checkCreateNetProfile(t, true, 5, 1500, "Gbps", "profile3", "default")
	checkCreateNetProfile(t, true, 5, 1500, "0 Gbps", "profile3", "default")
	checkCreateNetProfile(t, true, 5, 1500, " 10 Gbps", "profile3", "default")

	//verify that netprofile cannot be deleted with an invalid name/tenant name
	checkDeleteNetProfile(t, true, "invalid", "default")
	checkDeleteNetProfile(t, true, "invalid", "default")

	//verify that tenant cannot be deleted when it has a netprofile.
	checkCreateTenant(t, false, "blue")
	checkCreateNetProfile(t, false, 3, 0, "13mb", "profile3", "blue")
	checkDeleteTenant(t, true, "blue")

	//delete the netprofile and network
	checkDeleteNetProfile(t, false, "profile1", "default")
	checkDeleteNetProfile(t, false, "profile2", "default")
	checkDeleteNetProfile(t, false, "profile3", "default")
	checkDeleteNetProfile(t, false, "profile3", "blue")
	checkDeleteNetwork(t, false, "default", "net")
	checkDeleteTenant(t, false, "blue")
}

func TestServiceProviderUpdate(t *testing.T) {
	// ensure global configs set
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")

	labels := []string{"key1=value1", "key2=value2"}
	port := []string{"80:8080:TCP"}
	ch := make(chan error, 1)

	createNetwork(t, "yellow", "default", "vxlan", "10.1.1.0/24", "10.1.1.254")
	createNetwork(t, "orange", "default", "vxlan", "11.1.1.0/24", "11.1.1.254")

	checkServiceCreate(t, "default", "yellow", "redis", port, labels, "")
	verifyServiceCreate(t, "default", "yellow", "redis", port, labels, "")

	containerID1 := "723e55bf5b244f47c1b184cb786a1c2ad8870cc3a3db723c49ac09f68a9d1e69"
	containerID2 := "823e55bf5b244f47c1b184cb786a1c2ad8870cc3a3db723c49ac09f68a9d1e69"
	containerID3 := "023e55bf5b244f47c1b184cb786a1c2ad8870cc3a3db723c49ac09f68a9d1e69"
	containerID4 := "123e55bf5b244f47c1b184cb786a1c2ad8870cc3a3db723c49ac09f68a9d1e69"
	ep1 := "657355bf5b244f47c1b184cb786a14535d8870cc3a3db723c49ac09f68a9d6a5"
	ep2 := "757355bf5b244f47c1b184cb786a14535d8870cc3a3db723c49ac09f68a9d6a5"
	ep3 := "857355bf5b244f47c1b184cb786a14535d8870cc3a3db723c49ac09f68a9d6a5"
	ep4 := "957355bf5b244f47c1b184cb786a14535d8870cc3a3db723c49ac09f68a9d6a5"

	createEP(t, "20.1.1.1", "orange", containerID1, "default", ep1, labels)
	createEP(t, "20.1.1.2", "orange", containerID2, "default", ep2, labels)
	createEP(t, "20.1.1.3", "orange", containerID3, "default", ep3, labels)
	createEP(t, "20.1.1.4", "orange", containerID4, "default", ep4, labels)

	go triggerProviderUpdate(t, "20.1.1.1", "orange", containerID1, ep1, "container1", "default", "start", labels, ch)
	go triggerProviderUpdate(t, "20.1.1.2", "orange", containerID2, ep2, "container2", "default", "start", labels, ch)
	go triggerProviderUpdate(t, "20.1.1.3", "orange", containerID3, ep3, "container3", "default", "start", labels, ch)
	go triggerProviderUpdate(t, "20.1.1.4", "orange", containerID4, ep4, "container4", "default", "start", labels, ch)

	for i := 0; i < 4; i++ {
		<-ch
	}

	verifyProviderUpdate(t, "20.1.1.1", "orange", containerID1, "default", "start", "redis", labels)
	verifyProviderUpdate(t, "20.1.1.2", "orange", containerID2, "default", "start", "redis", labels)
	verifyProviderUpdate(t, "20.1.1.3", "orange", containerID3, "default", "start", "redis", labels)
	verifyProviderUpdate(t, "20.1.1.4", "orange", containerID4, "default", "start", "redis", labels)

	go triggerProviderUpdate(t, "20.1.1.1", "orange", containerID1, ep1, "container1", "default", "die", labels, ch)
	go triggerProviderUpdate(t, "20.1.1.2", "orange", containerID2, ep2, "container2", "default", "die", labels, ch)
	go triggerProviderUpdate(t, "20.1.1.3", "orange", containerID3, ep3, "container3", "default", "die", labels, ch)
	go triggerProviderUpdate(t, "20.1.1.4", "orange", containerID4, ep4, "container4", "default", "die", labels, ch)

	for i := 0; i < 4; i++ {
		<-ch
	}

	verifyProviderUpdate(t, "20.1.1.1", "orange", containerID1, "default", "die", "redis", labels)
	verifyProviderUpdate(t, "20.1.1.2", "orange", containerID2, "default", "die", "redis", labels)
	verifyProviderUpdate(t, "20.1.1.3", "orange", containerID3, "default", "die", "redis", labels)
	verifyProviderUpdate(t, "20.1.1.4", "orange", containerID4, "default", "die", "redis", labels)

	deleteEP(t, "orange", "default", ep1)
	deleteEP(t, "orange", "default", ep2)
	deleteEP(t, "orange", "default", ep3)
	deleteEP(t, "orange", "default", ep4)

	checkServiceDelete(t, "default", "redis")
	verifyServiceDelete(t, "default", "redis")
	deleteNetwork(t, "orange", "default")
	deleteNetwork(t, "yellow", "default")
}

func TestServiceProviderUpdateServiceAdd(t *testing.T) {
	// ensure global configs set
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")

	labels := []string{"key1=value1", "key2=value2"}
	port := []string{"80:8080:TCP"}
	ch := make(chan error, 1)

	containerID1 := "723e55bf5b244f47c1b184cb786a1c2ad8870cc3a3db723c49ac09f68a9d1e69"
	containerID2 := "823e55bf5b244f47c1b184cb786a1c2ad8870cc3a3db723c49ac09f68a9d1e69"
	containerID3 := "023e55bf5b244f47c1b184cb786a1c2ad8870cc3a3db723c49ac09f68a9d1e69"
	containerID4 := "123e55bf5b244f47c1b184cb786a1c2ad8870cc3a3db723c49ac09f68a9d1e69"
	ep1 := "657355bf5b244f47c1b184cb786a14535d8870cc3a3db723c49ac09f68a9d6a5"
	ep2 := "757355bf5b244f47c1b184cb786a14535d8870cc3a3db723c49ac09f68a9d6a5"
	ep3 := "857355bf5b244f47c1b184cb786a14535d8870cc3a3db723c49ac09f68a9d6a5"
	ep4 := "957355bf5b244f47c1b184cb786a14535d8870cc3a3db723c49ac09f68a9d6a5"

	createNetwork(t, "orange", "default", "vxlan", "11.1.1.0/24", "11.1.1.254")

	createEP(t, "20.1.1.1", "orange", containerID1, "default", ep1, labels)
	createEP(t, "20.1.1.2", "orange", containerID2, "default", ep2, labels)
	createEP(t, "20.1.1.3", "orange", containerID3, "default", ep3, labels)
	createEP(t, "20.1.1.4", "orange", containerID4, "default", ep4, labels)

	go triggerProviderUpdate(t, "20.1.1.1", "orange", containerID1, ep1, "container1", "default", "start", labels, ch)
	go triggerProviderUpdate(t, "20.1.1.2", "orange", containerID2, ep2, "container2", "default", "start", labels, ch)
	go triggerProviderUpdate(t, "20.1.1.3", "orange", containerID3, ep3, "container3", "default", "start", labels, ch)
	go triggerProviderUpdate(t, "20.1.1.4", "orange", containerID4, ep4, "container4", "default", "start", labels, ch)

	for i := 0; i < 4; i++ {
		<-ch
	}

	createNetwork(t, "yellow", "default", "vxlan", "10.1.1.0/24", "10.1.1.254")
	checkServiceCreate(t, "default", "yellow", "redis", port, labels, "")
	verifyServiceCreate(t, "default", "yellow", "redis", port, labels, "")

	verifyProviderUpdate(t, "20.1.1.1", "orange", containerID1, "default", "start", "redis", labels)
	verifyProviderUpdate(t, "20.1.1.2", "orange", containerID2, "default", "start", "redis", labels)
	verifyProviderUpdate(t, "20.1.1.3", "orange", containerID3, "default", "start", "redis", labels)
	verifyProviderUpdate(t, "20.1.1.4", "orange", containerID4, "default", "start", "redis", labels)

	go triggerProviderUpdate(t, "20.1.1.1", "orange", containerID1, ep1, "container1", "default", "die", labels, ch)
	go triggerProviderUpdate(t, "20.1.1.2", "orange", containerID2, ep2, "container2", "default", "die", labels, ch)
	go triggerProviderUpdate(t, "20.1.1.3", "orange", containerID3, ep3, "container3", "default", "die", labels, ch)
	go triggerProviderUpdate(t, "20.1.1.4", "orange", containerID4, ep4, "container4", "default", "die", labels, ch)
	for i := 0; i < 4; i++ {
		<-ch
	}

	verifyProviderUpdate(t, "20.1.1.1", "orange", containerID1, "default", "die", "redis", labels)
	verifyProviderUpdate(t, "20.1.1.2", "orange", containerID2, "default", "die", "redis", labels)
	verifyProviderUpdate(t, "20.1.1.3", "orange", containerID3, "default", "die", "redis", labels)
	verifyProviderUpdate(t, "20.1.1.4", "orange", containerID4, "default", "die", "redis", labels)

	deleteEP(t, "orange", "default", ep1)
	deleteEP(t, "orange", "default", ep2)
	deleteEP(t, "orange", "default", ep3)
	deleteEP(t, "orange", "default", ep4)

	checkServiceDelete(t, "default", "redis")
	verifyServiceDelete(t, "default", "redis")
	deleteNetwork(t, "orange", "default")
	deleteNetwork(t, "yellow", "default")
}

func TestServicePreferredIP(t *testing.T) {
	// ensure global configs set
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")

	labels := []string{"key1=value1", "key2=value2"}
	port := []string{"80:8080:TCP"}

	createNetwork(t, "yellow", "default", "vxlan", "10.1.1.0/24", "10.1.1.254")
	checkServiceCreate(t, "default", "yellow", "redis", port, labels, "10.1.1.3")
	verifyServiceCreate(t, "default", "yellow", "redis", port, labels, "10.1.1.3")
	checkServiceDelete(t, "default", "redis")
	deleteNetwork(t, "yellow", "default")
}

func TestBgp(t *testing.T) {

	bgpCfg := &client.Bgp{
		As:         "65001",
		Hostname:   "hostA",
		Neighbor:   "16.1.2.3",
		NeighborAs: "65002",
		Routerip:   "65.1.1.1/24",
	}

	err := contivClient.BgpPost(bgpCfg)
	if err != nil {
		t.Fatalf("Error creating bgp object. Err: %v", err)
	}
	gotBgp, err := contivClient.BgpGet("hostA")
	if err != nil {
		t.Fatalf("Error getting bgp object. Err: %v", err)
	}

	if gotBgp.As != "65001" || gotBgp.Hostname != "hostA" || gotBgp.Neighbor != "16.1.2.3" || gotBgp.NeighborAs != "65002" || gotBgp.Routerip != "65.1.1.1/24" {
		t.Fatalf("Error bgp object exp %+v, got %+v", bgpCfg, gotBgp)
	}

	_, err = contivClient.BgpInspect("hostA")
	if err != nil {
		t.Fatalf("Error inspecting bgp object. Err: %v", err)
	}
}

func checkServiceCreate(t *testing.T, tenant, network, serviceName string, port []string, label []string,
	preferredIP string) {

	serviceLB := &client.ServiceLB{
		TenantName:  tenant,
		NetworkName: network,
		ServiceName: serviceName,
	}
	if preferredIP != "" {
		serviceLB.IpAddress = preferredIP
	}
	serviceLB.Selectors = append(serviceLB.Selectors, label...)

	serviceLB.Ports = append(serviceLB.Ports, port...)

	err := contivClient.ServiceLBPost(serviceLB)
	if err != nil {
		log.Fatalf("Error creating Service. Err: %v", err)
	}

}

func verifyServiceCreate(t *testing.T, tenant, network, serviceName string, port []string, label []string,
	preferredIP string) {
	service, err := contivClient.ServiceLBGet(tenant, serviceName)
	if err != nil {
		t.Fatalf("Error retrieving the service created %s ", serviceName)
	}

	if service.NetworkName != network {
		t.Fatalf("Service Created does not have a valid network")
	}
	if !reflect.DeepEqual(service.Selectors, label) || !reflect.DeepEqual(service.Ports, port) {
		t.Fatalf("Service Created has mismatched Selectors or port information")
	}

	serviceLbState := mastercfg.CfgServiceLBState{}
	serviceLbState.StateDriver = stateStore
	serviceLbState.ID = serviceName + ":" + tenant

	err = serviceLbState.Read(serviceLbState.ID)
	if err != nil {
		t.Fatalf("Error reading from service load balancer state:%s", err)
	}

	if serviceLbState.IPAddress == "" {
		t.Fatalf("Service Created does not have an ip address allocated")
	}

	if preferredIP != "" && serviceLbState.IPAddress != preferredIP {
		t.Fatalf("Service Created does not have preferred ip address allocated")
	}

}

func checkServiceDelete(t *testing.T, tenant, serviceName string) {

	err := contivClient.ServiceLBDelete(tenant, serviceName)
	if err != nil {
		log.Fatalf("Error creating Service. Err: %v", err)
	}

}

func verifyServiceDelete(t *testing.T, tenant, serviceName string) {

	serviceLbState := mastercfg.CfgServiceLBState{}
	serviceLbState.StateDriver = stateStore
	serviceLbState.ID = serviceName + ":" + tenant

	err := serviceLbState.Read(serviceLbState.ID)
	if err == nil {
		t.Fatalf("Servicer Load balancer state not cleared after delete")
	}
}

func triggerProviderUpdate(t *testing.T, providerIP, network, containerID, endpointID, containerName,
	tenant, event string, labels []string, ch chan error) {

	endpointUpdReq := master.UpdateEndpointRequest{}
	endpointUpdReq.IPAddress = providerIP
	endpointUpdReq.ContainerID = containerID
	endpointUpdReq.Tenant = tenant
	endpointUpdReq.Network = network
	endpointUpdReq.Event = event
	endpointUpdReq.Labels = make(map[string]string)
	endpointUpdReq.EndpointID = endpointID
	endpointUpdReq.EPCommonName = containerName

	for _, v := range labels {
		key := strings.Split(v, "=")[0]
		value := strings.Split(v, "=")[1]
		endpointUpdReq.Labels[key] = value
	}

	jsonStr, err := json.Marshal(endpointUpdReq)
	if err != nil {
		ch <- err
		t.Fatalf("Error converting request data(%#v) to Json. Err: %v", endpointUpdReq, err)
	}
	url := netmasterTestURL + "/plugin/updateEndpoint"
	// Perform HTTP POST operation
	res, err := http.Post(url, "application/json", strings.NewReader(string(jsonStr)))
	if err != nil {
		t.Fatalf("Error during http get. Err: %v", err)
		ch <- err
	}

	// Check the response code
	if res.StatusCode != http.StatusOK {
		ch <- err
		t.Fatalf("HTTP error response. Status: %s, StatusCode: %d", res.Status, res.StatusCode)
	}

	ch <- nil

}
func verifyProviderUpdate(t *testing.T, providerIP, network, containerID,
	tenant, event, service string, labels []string) {

	svcProvider := &mastercfg.SvcProvider{}
	svcProvider.StateDriver = stateStore
	svcProvider.ID = service + ":" + tenant
	svcProvider.ServiceName = service + ":" + tenant
	err := svcProvider.Read(svcProvider.ID)
	if err != nil {
		t.Fatalf("Error reading from service provider state %s", err)
	}

	found := false
	for _, ipAddress := range svcProvider.Providers {
		if ipAddress == providerIP {
			found = true
			break
		}
	}
	if found == false && event == "start" {
		t.Fatalf("Service Provider update failed to update the new provider %s", providerIP)
	} else if found == true && event == "die" {
		t.Fatalf("Service Provider update failed to delete the provider %s", providerIP)
	}
}

func createNetwork(t *testing.T, network, tenant, encap, subnet, gw string) {
	net := client.Network{
		TenantName:  tenant,
		NetworkName: network,
		Encap:       encap,
		Subnet:      subnet,
		Gateway:     gw,
	}
	err := contivClient.NetworkPost(&net)
	if err != nil {
		t.Fatalf("Error creating network {%+v}. Err: %v", net, err)
	}

}

func deleteNetwork(t *testing.T, network, tenant string) {
	err := contivClient.NetworkDelete(tenant, network)
	if err != nil {
		t.Fatalf("Error deleting network {%+v}. Err: %v", network, err)
	}
}

func get(getAll bool, hook func(id string) ([]core.State, error)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			idStr  string
			states []core.State
			resp   []byte
			ok     bool
			err    error
		)

		if getAll {
			idStr = "all"
		} else if idStr, ok = mux.Vars(r)["id"]; !ok {
			http.Error(w,
				core.Errorf("Failed to find the id string in the request.").Error(),
				http.StatusInternalServerError)
		}

		if states, err = hook(idStr); err != nil {
			http.Error(w,
				err.Error(),
				http.StatusInternalServerError)
			return
		}

		if resp, err = json.Marshal(states); err != nil {
			http.Error(w,
				core.Errorf("marshaling json failed. Error: %s", err).Error(),
				http.StatusInternalServerError)
			return
		}

		w.Write(resp)
		return
	}
}

func createEPinEPG(t *testing.T, providerIP, network, epg, containerID, tenant, endpointID string, labels []string) {

	epCfg := &mastercfg.CfgEndpointState{
		NetID:            network,
		EndpointID:       endpointID,
		IPAddress:        providerIP,
		EndpointGroupKey: epg + ":" + tenant,
	}
	epCfg.Labels = make(map[string]string)
	for _, v := range labels {
		key := strings.Split(v, "=")[0]
		value := strings.Split(v, "=")[1]
		epCfg.Labels[key] = value
	}
	epCfg.StateDriver = stateStore
	netID := network + "." + tenant
	epCfg.ID = netID + "-" + endpointID
	err := epCfg.Write()
	if err != nil {
		t.Errorf("Error creating Ep :%s", err)
	}
}

func createEP(t *testing.T, providerIP, network, containerID, tenant, endpointID string, labels []string) {

	epCfg := &mastercfg.CfgEndpointState{
		NetID:      network,
		EndpointID: endpointID,
		IPAddress:  providerIP,
	}
	epCfg.Labels = make(map[string]string)
	for _, v := range labels {
		key := strings.Split(v, "=")[0]
		value := strings.Split(v, "=")[1]
		epCfg.Labels[key] = value
	}
	epCfg.StateDriver = stateStore
	netID := network + "." + tenant
	epCfg.ID = netID + "-" + endpointID
	err := epCfg.Write()
	if err != nil {
		t.Errorf("Error creating Ep :%s", err)
	}
}
func deleteEP(t *testing.T, network, tenant, endpointID string) {
	epCfg := &mastercfg.CfgEndpointState{}
	epCfg.StateDriver = stateStore
	netID := network + "." + tenant
	epCfg.ID = netID + "-" + endpointID
	err := epCfg.Clear()
	if err != nil {
		t.Errorf("Error deleting Ep :%s", err)
	}
}

func AddEP(tenant, nw, epg, id string) error {
	// Build endpoint request
	mreq := master.CreateEndpointRequest{
		TenantName:  tenant,
		NetworkName: nw,
		ServiceName: epg,
		EndpointID:  id,
		ConfigEP: intent.ConfigEP{
			Container:   id,
			ServiceName: epg,
		},
	}

	var mresp master.CreateEndpointResponse
	url := netmasterTestURL + "/plugin/createEndpoint"
	return utils.HTTPPost(url, &mreq, &mresp)
}

func TestEPCreate(t *testing.T) {
	// ensure global configs set
	checkGlobalSet(t, false, "default", "1-4094", "1-10000", "bridge", "proxy", "172.19.0.0/16")

	checkCreateTenant(t, false, "teatwo")
	checkCreateNetwork(t, false, "teatwo", "t2-net", "data", "vlan",
		"60.1.1.1/24", "60.1.1.254", 1, "", "", "")
	checkCreateEpg(t, false, "teatwo", "t2-net", "t2-epgA", []string{}, []string{}, "")
	checkInspectNetwork(t, false, "teatwo", "t2-net", "60.1.1.254", 1, 0)

	err := AddEP("teatwo", "t2-net", "t2-epgA", "c1231")
	if err != nil {
		log.Fatalf("Error creating ep c1231")
	}

	checkInspectNetwork(t, false, "teatwo", "t2-net", "60.1.1.1, 60.1.1.254", 1, 1)

	// create an endpoint with non-existent epg
	err = AddEP("teatwo", "t2-net", "t2-epgB", "c1232")
	if err == nil {
		log.Fatalf("Error succeeded creating ep c1232, expected failure")
	}
	checkInspectNetwork(t, false, "teatwo", "t2-net", "60.1.1.1, 60.1.1.254", 1, 1)

	err = AddEP("teatwo", "t2-net", "t2-epgA", "c1233")
	if err != nil {
		log.Fatalf("Error creating ep c1233")
	}
	checkInspectNetwork(t, false, "teatwo", "t2-net", "60.1.1.1-60.1.1.2, 60.1.1.254", 1, 2)

	checkCreateEpg(t, false, "teatwo", "t2-net", "t2-epgB", []string{}, []string{}, "")
	err = AddEP("teatwo", "t2-net", "t2-epgB", "c1232")
	if err != nil {
		log.Fatalf("Error creating ep c1232")
	}
	checkInspectNetwork(t, false, "teatwo", "t2-net", "60.1.1.1-60.1.1.3, 60.1.1.254", 1, 3)
}

// TestClusterMode verifies cluster mode is correctly reflected.
func TestClusterMode(t *testing.T) {

	master.SetClusterMode(core.Kubernetes)
	insp, err := contivClient.GlobalInspect("global")
	if err != nil {
		t.Fatalf("Error inspecting global %v", err)
	}
	if insp.Oper.ClusterMode != core.Kubernetes {
		t.Fatalf("Error expected kubernetes, got %s", insp.Oper.ClusterMode)
	}

	master.SetClusterMode(core.Docker)
	insp, err = contivClient.GlobalInspect("global")
	if err != nil {
		t.Fatalf("Error inspecting global %v", err)
	}
	if insp.Oper.ClusterMode != core.Docker {
		t.Fatalf("Error expected docker, got %s", insp.Oper.ClusterMode)
	}

	master.SetClusterMode(core.SwarmMode)
	insp, err = contivClient.GlobalInspect("global")
	if err != nil {
		t.Fatalf("Error inspecting global %v", err)
	}
	if insp.Oper.ClusterMode != core.SwarmMode {
		t.Fatalf("Error expected docker, got %s", insp.Oper.ClusterMode)
	}
}
