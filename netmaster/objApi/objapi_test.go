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
	"golang.org/x/net/context"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/contiv/contivmodel"
	"github.com/contiv/contivmodel/client"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/gstate"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/netmaster/resources"
	"github.com/contiv/netplugin/state"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/netplugin/utils/netutils"
	"github.com/contiv/ofnet"
	"github.com/gorilla/mux"
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

// setup the test netmaster REST server and client
func TestMain(m *testing.M) {
	var err error

	// Setup state store
	stateStore, err = initStateDriver()
	if err != nil {
		log.Fatalf("Error initializing state store. Err: %v", err)
	}
	// little hack to clear all state from etcd
	stateStore.(*state.EtcdStateDriver).KeysAPI.Delete(context.Background(), "/contiv.io", nil)

	// Setup resource manager
	if _, err = resources.NewStateResourceManager(stateStore); err != nil {
		log.Fatalf("Failed to init resource manager. Error: %s", err)
	}

	router := mux.NewRouter()
	s := router.Headers("Content-Type", "application/json").Methods("Post").Subrouter()
	s.HandleFunc("/plugin/svcProviderUpdate", makeHTTPHandler(master.ServiceProviderUpdateHandler))
	s = router.Methods("Get").Subrouter()

	// Create a new api controller
	apiController = NewAPIController(router, "etcd://127.0.0.1:4001")

	ofnetMaster := ofnet.NewOfnetMaster("127.0.0.1", ofnet.OFNET_MASTER_PORT)
	if ofnetMaster == nil {
		log.Fatalf("Error creating ofnet master")
	}

	// initialize policy manager
	mastercfg.InitPolicyMgr(stateStore, ofnetMaster)

	// Create HTTP server
	go http.ListenAndServe(netmasterTestListenURL, router)
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

// checkError checks for error and fails teh test
func checkError(t *testing.T, testStr string, err error) {
	if err != nil {
		t.Fatalf("Error during %s. Err: %v", testStr, err)
	}
}

// checkCreateNetwork creates networks and checks for error
func checkCreateNetwork(t *testing.T, expError bool, tenant, network, nwType, encap, subnet, gw string, tag int) {
	net := client.Network{
		TenantName:  tenant,
		NetworkName: network,
		NwType:      nwType,
		Encap:       encap,
		Subnet:      subnet,
		Gateway:     gw,
		PktTag:      tag,
	}
	err := contivClient.NetworkPost(&net)
	if err != nil && !expError {
		t.Fatalf("Error creating network {%+v}. Err: %v", net, err)
	} else if err == nil && expError {
		t.Fatalf("Create network {%+v} succeded while expecing error", net)
	} else if err == nil {
		// verify network is created
		_, err := contivClient.NetworkGet(tenant, network)
		if err != nil {
			t.Fatalf("Error getting network %s/%s. Err: %v", tenant, network, err)
		}
	}
}

// verifyNetworkState verifies network state es as expected
func verifyNetworkState(t *testing.T, tenant, network, nwType, encap, subnet, gw string, subnetLen uint, pktTag, extTag int) {
	networkID := network + "." + tenant
	nwCfg := &mastercfg.CfgNetworkState{}
	nwCfg.StateDriver = stateStore
	err := nwCfg.Read(networkID)
	if err != nil {
		t.Fatalf("Network state for %s not found. Err: %v", networkID, err)
	}

	// verify network params
	if nwCfg.Tenant != tenant || nwCfg.NetworkName != network || nwCfg.NwType != nwType ||
		nwCfg.PktTagType != encap || nwCfg.SubnetIP != netutils.GetSubnetAddr(subnet, subnetLen) || nwCfg.Gateway != gw {
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
		t.Fatalf("Delete network %s/%s succeded while expecing error", tenant, network)
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

// checkGlobalSet sets global state and verifies state
func checkGlobalSet(t *testing.T, expError bool, fabMode, vlans, vxlans string) {
	gl := client.Global{
		Name:             "global",
		NetworkInfraType: fabMode,
		Vlans:            vlans,
		Vxlans:           vxlans,
	}
	err := contivClient.GlobalPost(&gl)
	if err != nil && !expError {
		t.Fatalf("Error setting global {%+v}. Err: %v", gl, err)
	} else if err == nil && expError {
		t.Fatalf("Set global {%+v} succeded while expecing error", gl)
	} else if err == nil {
		// verify global state
		gotGl, err := contivClient.GlobalGet("global")
		if err != nil {
			t.Fatalf("Error getting global object. Err: %v", err)
		}

		// verify expected values
		if gotGl.NetworkInfraType != fabMode || gotGl.Vlans != vlans || gotGl.Vxlans != vxlans {
			t.Fatalf("Error Got global state {%+v} does not match expected %s, %s, %s", gotGl, fabMode, vlans, vxlans)
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
		t.Fatalf("Create policy {%+v} succeded while expecing error", pol)
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
		t.Fatalf("Delete policy %s/%s succeded while expecing error", tenant, policy)
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
		t.Fatalf("Create rule {%+v} succeded while expecing error", pol)
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
		t.Fatalf("Delete rule %s/%s/%s succeded while expecing error", tenant, policy, ruleID)
	} else if err == nil {
		// verify rule is gone
		_, err := contivClient.RuleGet(tenant, policy, ruleID)
		if err == nil {
			t.Fatalf("Policy %s/%s/%s not deleted", tenant, policy, ruleID)
		}
	}
}

// checkCreateEpg creates an EPG
func checkCreateEpg(t *testing.T, expError bool, tenant, network, group string, policies []string) {
	epg := client.EndpointGroup{
		TenantName:  tenant,
		NetworkName: network,
		GroupName:   group,
		Policies:    policies,
	}
	err := contivClient.EndpointGroupPost(&epg)
	if err != nil && !expError {
		t.Fatalf("Error creating epg {%+v}. Err: %v", epg, err)
	} else if err == nil && expError {
		t.Fatalf("Create epg {%+v} succeded while expecing error", epg)
	} else if err == nil {
		// verify epg is created
		_, err := contivClient.EndpointGroupGet(tenant, network, group)
		if err != nil {
			t.Fatalf("Error getting epg %s/%s/%s. Err: %v", tenant, network, group, err)
		}
	}
}

// verifyEpgPolicy verifies an EPG policy state
func verifyEpgPolicy(t *testing.T, tenant, network, group, policy string) {
	epgKey := tenant + ":" + network + ":" + group
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

	// verify epg ids
	if epg.EndpointGroupID != gp.EndpointGroupID {
		t.Fatalf("EPG policy has incorrect epg-id %d. expecting %d", gp.EndpointGroupID, epg.EndpointGroupID)
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
	epgKey := tenant + ":" + network + ":" + group
	policyKey := tenant + ":" + policy
	epgpKey := epgKey + ":" + policyKey

	// See if it already exists
	gp := mastercfg.FindEpgPolicy(epgpKey)
	if gp != nil {
		t.Fatalf("Found EPG policy %s while expecing it to be deleted", epgpKey)
	}

}

// checkDeleteEpg deletes EPG
func checkDeleteEpg(t *testing.T, expError bool, tenant, network, group string) {
	err := contivClient.EndpointGroupDelete(tenant, network, group)
	if err != nil && !expError {
		t.Fatalf("Error deleting epg %s/%s/%s. Err: %v", tenant, network, group, err)
	} else if err == nil && expError {
		t.Fatalf("Delete epg %s/%s/%s succeded while expecing error", tenant, network, group)
	} else if err == nil {
		// verify epg is gone
		_, err := contivClient.EndpointGroupGet(tenant, network, group)
		if err == nil {
			t.Fatalf("EndpointGroup %s/%s/%s not deleted", tenant, network, group)
		}
	}
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

	// Get the tenant and verify it exists
	gotTenant, err := contivClient.TenantGet("tenant1")
	checkError(t, "get tenant", err)

	if gotTenant.TenantName != tenant.TenantName {
		t.Fatalf("Got invalid tenant name. expecting %s. Got %s", tenant.TenantName, gotTenant.TenantName)
	}

	// delete tenant
	err = contivClient.TenantDelete("tenant1")
	checkError(t, "delete tenant", err)

	// find again and make sure its gone
	_, err = contivClient.TenantGet("tenant1")
	if err == nil {
		t.Fatalf("Tenant was not deleted")
	}
}

// TestNetworkAddDelete tests network create/delete REST api
func TestNetworkAddDelete(t *testing.T) {
	// Basic vlan network
	checkCreateNetwork(t, false, "default", "contiv", "", "vlan", "10.1.1.1/24", "10.1.1.254", 1)
	verifyNetworkState(t, "default", "contiv", "data", "vlan", "10.1.1.1", "10.1.1.254", 24, 1, 0)
	checkDeleteNetwork(t, false, "default", "contiv")

	// Basic Vxlan network
	checkCreateNetwork(t, false, "default", "contiv", "", "vxlan", "10.1.1.1/16", "10.1.1.254", 1)
	verifyNetworkState(t, "default", "contiv", "data", "vxlan", "10.1.1.1", "10.1.1.254", 16, 1, 1)
	checkDeleteNetwork(t, false, "default", "contiv")

	// Infra vlan network create and delete
	checkCreateNetwork(t, false, "default", "infraNw", "infra", "vlan", "10.1.1.1/24", "10.1.1.254", 1)
	time.Sleep(time.Second)
	verifyNetworkState(t, "default", "infraNw", "infra", "vlan", "10.1.1.1", "10.1.1.254", 24, 1, 0)
	checkDeleteNetwork(t, false, "default", "infraNw")
	time.Sleep(time.Second)

	// Try creating network with invalid network type
	checkCreateNetwork(t, true, "default", "infraNw", "infratest", "vlan", "10.1.1.1/24", "10.1.1.254", 1)
	checkCreateNetwork(t, true, "default", "infraNw", "testinfra", "vlan", "10.1.1.1/24", "10.1.1.254", 1)
	checkCreateNetwork(t, true, "default", "infraNw", "testdata", "vlan", "10.1.1.1/24", "10.1.1.254", 1)
	checkCreateNetwork(t, true, "default", "infraNw", "datatest", "vlan", "10.1.1.1/24", "10.1.1.254", 1)

	// Basic IP range network checks
	checkCreateNetwork(t, false, "default", "contiv", "data", "vxlan", "10.1.1.10-20/24", "10.1.1.254", 1)
	verifyNetworkState(t, "default", "contiv", "data", "vxlan", "10.1.1.10", "10.1.1.254", 24, 1, 1)
	checkDeleteNetwork(t, false, "default", "contiv")

	// Try network create with invalid network range
	checkCreateNetwork(t, true, "default", "contiv", "data", "vxlan", "10.1.1.1-70/26", "10.1.1.63", 1)

	// Try network create with invalid subnet length
	checkCreateNetwork(t, true, "default", "contiv", "data", "vxlan", "10.1.1.1/32", "10.1.1.1", 1)

	// try creating network without tenant
	checkCreateNetwork(t, true, "tenant1", "contiv", "data", "vxlan", "10.1.1.1/24", "10.1.1.254", 1)

	// try invalid encap
	checkCreateNetwork(t, true, "default", "contiv", "data", "vvvv", "10.1.1.1/24", "10.1.1.254", 1)

	// try invalid pkt tags
	checkCreateNetwork(t, true, "default", "contiv", "data", "vlan", "10.1.1.1/24", "10.1.1.254", 5000)
	checkCreateNetwork(t, true, "default", "contiv", "data", "vxlan", "10.1.1.1/24", "10.1.1.254", 20000)

	// Try gateway outside the network
	checkCreateNetwork(t, true, "default", "contiv", "data", "vxlan", "10.1.1.1/24", "10.1.2.254", 1)
	checkCreateNetwork(t, true, "default", "contiv", "data", "vxlan", "10.1.1.65-70/26", "10.1.1.1", 2)

	// Try deleting a non-existing network
	checkDeleteNetwork(t, true, "default", "contiv")
}

// TestGlobalSetting tests global REST api
func TestGlobalSetting(t *testing.T) {
	// try basic modification
	checkGlobalSet(t, false, "default", "1-4094", "1-10000")
	// set aci mode
	checkGlobalSet(t, false, "aci", "1-4094", "1-10000")

	// modify vlan/vxlan range
	checkGlobalSet(t, false, "default", "1-1000", "1001-2000")

	// try invalid fabric mode
	checkGlobalSet(t, true, "xyz", "1-4094", "1-10000")

	// try invalid vlan/vxlan range
	checkGlobalSet(t, true, "default", "1-5000", "1-10000")
	checkGlobalSet(t, true, "default", "0-4094", "1-10000")
	checkGlobalSet(t, true, "default", "1", "1-10000")
	checkGlobalSet(t, true, "default", "1?2", "1-10000")
	checkGlobalSet(t, true, "default", "abcd", "1-10000")
	checkGlobalSet(t, true, "default", "1-4094", "1-100000")
	checkGlobalSet(t, true, "default", "1-4094", "1-20000")

	// reset back to default values
	checkGlobalSet(t, false, "default", "1-4094", "1-10000")
}

// TestNetworkPktRanges tests pkt-tag ranges in network REST api
func TestNetworkPktRanges(t *testing.T) {
	// verify auto allocation of vlans
	checkCreateNetwork(t, false, "default", "contiv", "data", "vlan", "10.1.1.1/24", "10.1.1.254", 0)
	verifyNetworkState(t, "default", "contiv", "data", "vlan", "10.1.1.1", "10.1.1.254", 24, 1, 0)
	checkDeleteNetwork(t, false, "default", "contiv")

	// auto allocation of vxlan
	checkCreateNetwork(t, false, "default", "contiv", "data", "vxlan", "10.1.1.1/24", "10.1.1.254", 0)
	verifyNetworkState(t, "default", "contiv", "data", "vxlan", "10.1.1.1", "10.1.1.254", 24, 1, 1)
	checkCreateNetwork(t, false, "default", "contiv2", "data", "vxlan", "10.1.2.1/24", "10.1.2.254", 0)
	verifyNetworkState(t, "default", "contiv2", "data", "vxlan", "10.1.2.1", "10.1.2.254", 24, 2, 2)
	checkCreateNetwork(t, false, "default", "contiv3", "data", "vxlan", "10.1.3.1/24", "10.1.3.254", 1000)
	verifyNetworkState(t, "default", "contiv3", "data", "vxlan", "10.1.3.1", "10.1.3.254", 24, 3, 1000)
	checkDeleteNetwork(t, false, "default", "contiv")
	checkDeleteNetwork(t, false, "default", "contiv2")
	checkDeleteNetwork(t, false, "default", "contiv3")

	// verify duplicate values fail
	checkCreateNetwork(t, false, "default", "contiv1", "data", "vlan", "10.1.1.1/24", "10.1.1.254", 1)
	checkCreateNetwork(t, true, "default", "contiv2", "data", "vlan", "10.1.1.1/24", "10.1.1.254", 1)
	checkDeleteNetwork(t, false, "default", "contiv1")

	checkCreateNetwork(t, false, "default", "contiv1", "data", "vxlan", "10.1.1.1/24", "10.1.1.254", 0)
	checkCreateNetwork(t, true, "default", "contiv2", "data", "vxlan", "10.1.1.1/24", "10.1.1.254", 1)
	checkDeleteNetwork(t, false, "default", "contiv1")

	// shrink ranges and try allocating
	checkGlobalSet(t, false, "default", "100-1000", "1001-2000")
	checkCreateNetwork(t, true, "default", "contiv1", "data", "vlan", "10.1.1.1/24", "10.1.1.254", 1001)
	checkCreateNetwork(t, true, "default", "contiv1", "data", "vlan", "10.1.1.1/24", "10.1.1.254", 99)
	checkCreateNetwork(t, true, "default", "contiv2", "data", "vxlan", "10.1.2.1/24", "10.1.2.254", 2001)
	checkCreateNetwork(t, true, "default", "contiv2", "data", "vxlan", "10.1.2.1/24", "10.1.2.254", 1000)

	// reset back to default values
	checkGlobalSet(t, false, "default", "1-4094", "1-10000")
}

// TestPolicyRules tests policy and rule REST objects
func TestPolicyRules(t *testing.T) {
	// create policy
	checkCreatePolicy(t, false, "default", "policy1")

	// verify policy on unknown tenant fails
	checkCreatePolicy(t, true, "tenant1", "policy1")

	// add rules
	checkCreateRule(t, false, "default", "policy1", "1", "in", "contiv", "", "", "", "", "", "tcp", "allow", 1, 80)
	checkCreateRule(t, false, "default", "policy1", "2", "in", "contiv", "", "", "", "", "", "", "deny", 1, 0)
	checkCreateRule(t, false, "default", "policy1", "3", "out", "", "", "", "contiv", "", "", "tcp", "allow", 1, 80)
	checkCreateRule(t, false, "default", "policy1", "4", "in", "", "", "10.1.1.1/24", "", "", "", "tcp", "allow", 1, 80)
	checkCreateRule(t, false, "default", "policy1", "5", "out", "", "", "", "", "", "10.1.1.1/24", "tcp", "allow", 1, 80)

	// verify duplicate rule id fails
	checkCreateRule(t, true, "default", "policy1", "1", "in", "contiv", "", "", "", "", "", "tcp", "allow", 1, 80)

	// verify unknown directions fail
	checkCreateRule(t, true, "default", "policy1", "100", "both", "", "", "", "", "", "", "tcp", "allow", 1, 0)
	checkCreateRule(t, true, "default", "policy1", "100", "xyz", "", "", "", "", "", "", "tcp", "allow", 1, 0)

	// verify unknown protocol fails
	checkCreateRule(t, true, "default", "policy1", "100", "in", "contiv", "", "", "", "", "", "xyz", "allow", 1, 80)

	// verify unknown action fails
	checkCreateRule(t, true, "default", "policy1", "100", "in", "contiv", "", "", "", "", "", "tcp", "xyz", 1, 80)
	checkCreateRule(t, true, "default", "policy1", "100", "in", "contiv", "", "", "", "", "", "tcp", "accept", 1, 80)

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

	// checkCreateRule(t, true, tenant, policy, ruleID, dir, fnet, fepg, fip, tnet, tepg, tip, proto, prio, port)

	// delete rules
	checkDeleteRule(t, false, "default", "policy1", "1")
	checkDeleteRule(t, false, "default", "policy1", "2")
	checkDeleteRule(t, false, "default", "policy1", "3")
	checkDeleteRule(t, false, "default", "policy1", "4")
	checkDeleteRule(t, false, "default", "policy1", "5")

	// verify cant delete a rule and policy that doesnt exist
	checkDeleteRule(t, true, "default", "policy1", "100")
	checkDeletePolicy(t, true, "default", "policy2")

	// delete policy
	checkDeletePolicy(t, false, "default", "policy1")
}

// TestEpgPolicies tests attaching policy to EPG
func TestEpgPolicies(t *testing.T) {
	// create network
	checkCreateNetwork(t, false, "default", "contiv", "data", "vxlan", "10.1.1.1/16", "10.1.1.254", 1)

	// create policy
	checkCreatePolicy(t, false, "default", "policy1")

	// add rules
	checkCreateRule(t, false, "default", "policy1", "1", "in", "contiv", "", "", "", "", "", "tcp", "allow", 1, 80)
	checkCreateRule(t, false, "default", "policy1", "2", "in", "contiv", "", "", "", "", "", "", "deny", 1, 0)
	checkCreateRule(t, false, "default", "policy1", "3", "out", "", "", "", "contiv", "", "", "tcp", "allow", 1, 80)

	// create EPG and attach policy to it
	checkCreateEpg(t, false, "default", "contiv", "group1", []string{"policy1"})
	verifyEpgPolicy(t, "default", "contiv", "group1", "policy1")

	// create a policy and rule that matches on other policy
	checkCreatePolicy(t, false, "default", "policy2")
	checkCreateRule(t, false, "default", "policy2", "1", "in", "contiv", "group1", "", "", "", "", "tcp", "allow", 1, 80)
	checkCreateRule(t, false, "default", "policy2", "2", "out", "", "", "", "contiv", "group1", "", "tcp", "allow", 1, 80)
	checkCreateEpg(t, false, "default", "contiv", "group2", []string{"policy2"})
	verifyEpgPolicy(t, "default", "contiv", "group2", "policy2")

	// verify cant match on non-existing EPGs
	checkCreateRule(t, true, "default", "policy2", "100", "in", "invalid", "group1", "", "", "", "", "tcp", "allow", 1, 80)
	checkCreateRule(t, true, "default", "policy2", "100", "in", "contiv", "invalid", "", "", "", "", "tcp", "allow", 1, 80)
	checkCreateRule(t, true, "default", "policy2", "100", "out", "", "", "", "invalid", "group1", "", "tcp", "allow", 1, 80)
	checkCreateRule(t, true, "default", "policy2", "100", "out", "", "", "", "contiv", "invalid", "", "tcp", "allow", 1, 80)

	// verify cant create/update EPGs that uses non-existing policies
	checkCreateEpg(t, true, "default", "contiv", "group3", []string{"invalid"})
	checkCreateEpg(t, true, "default", "contiv", "group2", []string{"invalid"})

	// verify cant create EPGs without tenant/network
	checkCreateEpg(t, true, "invalid", "contiv", "group3", []string{})
	checkCreateEpg(t, true, "default", "invalid", "group3", []string{})

	// change policy and verify EPG policy changes
	checkCreateEpg(t, false, "default", "contiv", "group3", []string{"policy1"})
	checkCreateEpg(t, false, "default", "contiv", "group3", []string{"policy2"})
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
}

func TestServiceProviderUpdate(t *testing.T) {

	labels := []string{"key1=value1", "key2=value2"}
	port := []string{"80:8080:TCP"}

	createNetwork(t, "yellow", "default", "vxlan", "10.1.1.0/24", "10.1.1.254")
	createNetwork(t, "orange", "default", "vxlan", "11.1.1.0/24", "11.1.1.254")

	checkServiceCreate(t, "default", "yellow", "redis", port, labels, "")
	verifyServiceCreate(t, "default", "yellow", "redis", port, labels, "")

	containerID := "723e55bf5b244f47c1b184cb786a1c2ad8870cc3a3db723c49ac09f68a9d1e69"
	container := "657355bf5b244f47c1b184cb786a14535d8870cc3a3db723c49ac09f68a9d6a5"

	createEP(t, "20.1.1.1", "orange", containerID, "default", container, labels)
	triggerProviderUpdate(t, "20.1.1.1", "orange", containerID, container, "default", "start", labels)
	verifyProviderUpdate(t, "20.1.1.1", "orange", containerID, "default", "start", "redis", labels)
	triggerProviderUpdate(t, "20.1.1.1", "orange", containerID, container, "default", "die", labels)
	verifyProviderUpdate(t, "20.1.1.1", "orange", containerID, "default", "die", "redis", labels)
	deleteEP(t, "orange", "default", container)
	checkServiceDelete(t, "default", "redis")
	verifyServiceDelete(t, "default", "redis")
	deleteNetwork(t, "orange", "default")
	deleteNetwork(t, "yellow", "default")
}

func TestServiceProviderUpdateServiceAdd(t *testing.T) {

	labels := []string{"key1=value1", "key2=value2"}
	port := []string{"80:8080:TCP"}

	containerID := "723e55bf5b244f47c1b184cb786a1c2ad8870cc3a3db723c49ac09f68a9d1e69"
	container := "657355bf5b244f47c1b184cb786a14535d8870cc3a3db723c49ac09f68a9d6a5"

	createNetwork(t, "orange", "default", "vxlan", "11.1.1.0/24", "11.1.1.254")
	createEP(t, "20.1.1.1", "orange", containerID, "default", container, labels)
	triggerProviderUpdate(t, "20.1.1.1", "orange", containerID, container, "default", "start", labels)
	createNetwork(t, "yellow", "default", "vxlan", "10.1.1.0/24", "10.1.1.254")
	checkServiceCreate(t, "default", "yellow", "redis", port, labels, "")
	verifyServiceCreate(t, "default", "yellow", "redis", port, labels, "")

	verifyProviderUpdate(t, "20.1.1.1", "orange", containerID, "default", "start", "redis", labels)
	triggerProviderUpdate(t, "20.1.1.1", "orange", containerID, container, "default", "die", labels)
	verifyProviderUpdate(t, "20.1.1.1", "orange", containerID, "default", "die", "redis", labels)
	deleteEP(t, "orange", "default", container)
	checkServiceDelete(t, "default", "redis")
	verifyServiceDelete(t, "default", "redis")
	deleteNetwork(t, "orange", "default")
	deleteNetwork(t, "yellow", "default")
}

func TestServicePreferredIP(t *testing.T) {

	labels := []string{"key1=value1", "key2=value2"}
	port := []string{"80:8080:TCP"}

	createNetwork(t, "yellow", "default", "vxlan", "10.1.1.0/24", "10.1.1.254")
	checkServiceCreate(t, "default", "yellow", "redis", port, labels, "10.1.1.3")
	verifyServiceCreate(t, "default", "yellow", "redis", port, labels, "10.1.1.3")
	checkServiceDelete(t, "default", "redis")
	deleteNetwork(t, "yellow", "default")
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
		t.Fatalf("Service Created does not have an ip addres allocated")
	}

	if preferredIP != "" && serviceLbState.IPAddress != preferredIP {
		t.Fatalf("Service Created does not have preferred ip addres allocated")
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

func triggerProviderUpdate(t *testing.T, providerIP, network, containerID, container,
	tenant, event string, labels []string) {

	providerUpdReq := master.SvcProvUpdateRequest{}
	providerUpdReq.IPAddress = providerIP
	providerUpdReq.ContainerID = containerID
	providerUpdReq.Tenant = tenant
	providerUpdReq.Network = network
	providerUpdReq.Event = event
	providerUpdReq.Labels = make(map[string]string)
	providerUpdReq.Container = container

	for _, v := range labels {
		key := strings.Split(v, "=")[0]
		value := strings.Split(v, "=")[1]
		providerUpdReq.Labels[key] = value
	}
	//var svcProvResp master.SvcProvUpdateResponse

	jsonStr, err := json.Marshal(providerUpdReq)
	if err != nil {
		t.Fatalf("Error converting request data(%#v) to Json. Err: %v", providerUpdReq, err)
	}
	url := netmasterTestURL + "/plugin/svcProviderUpdate"
	// Perform HTTP POST operation
	res, err := http.Post(url, "application/json", strings.NewReader(string(jsonStr)))
	if err != nil {
		t.Fatalf("Error during http get. Err: %v", err)
	}

	// Check the response code
	if res.StatusCode != http.StatusOK {
		t.Fatalf("HTTP error response. Status: %s, StatusCode: %d", res.Status, res.StatusCode)
	}

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

// Simple Wrapper for http handlers
func makeHTTPHandler(handlerFunc httpAPIFunc) http.HandlerFunc {
	// Create a closure and return an anonymous function
	return func(w http.ResponseWriter, r *http.Request) {
		// Call the handler
		resp, err := handlerFunc(w, r, mux.Vars(r))
		if err != nil {
			// Log error

			// Send HTTP response
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			// Send HTTP response as Json
			err = writeJSON(w, http.StatusOK, resp)
			if err != nil {
			}
		}
	}
}

// writeJSON: writes the value v to the http response stream as json with standard
// json encoding.
func writeJSON(w http.ResponseWriter, code int, v interface{}) error {
	// Set content type as json
	w.Header().Set("Content-Type", "application/json")

	// write the HTTP status code
	w.WriteHeader(code)

	// Write the Json output
	return json.NewEncoder(w).Encode(v)
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
				core.Errorf("marshalling json failed. Error: %s", err).Error(),
				http.StatusInternalServerError)
			return
		}

		w.Write(resp)
		return
	}
}

func createEP(t *testing.T, providerIP, network, containerID, tenant, container string, labels []string) {

	epCfg := &mastercfg.CfgEndpointState{
		NetID:      network,
		ContName:   containerID,
		AttachUUID: container,
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
	epCfg.ID = netID + "-" + container
	err := epCfg.Write()
	if err != nil {
		t.Errorf("Error creating Ep :%s", err)
	}
}
func deleteEP(t *testing.T, network, tenant, container string) {
	epCfg := &mastercfg.CfgEndpointState{}
	epCfg.StateDriver = stateStore
	netID := network + "." + tenant
	epCfg.ID = netID + "-" + container
	err := epCfg.Clear()
	if err != nil {
		t.Errorf("Error deleting Ep :%s", err)
	}
}

type httpAPIFunc func(w http.ResponseWriter, r *http.Request, vars map[string]string) (interface{}, error)
