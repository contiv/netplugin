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

package ofnet

import (
	"fmt"
	"net"
	"testing"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/ofnet/ovsdbDriver"
)

func TestPolicyAddDelete(t *testing.T) {
	var resp bool
	rpcPort := uint16(9600)
	ovsPort := uint16(9601)
	lclIP := net.ParseIP("10.10.10.10")
	ofnetAgent, err := NewOfnetAgent("vrouter", lclIP, rpcPort, ovsPort)
	if err != nil {
		t.Fatalf("Error creating ofnet agent. Err: %v", err)
	}

	defer func() { ofnetAgent.Delete() }()

	// Override MyAddr to local host
	ofnetAgent.MyAddr = "127.0.0.1"

	// Create a Master
	ofnetMaster := NewOfnetMaster(uint16(9602))

	defer func() { ofnetMaster.Delete() }()

	masterInfo := OfnetNode{
		HostAddr: "127.0.0.1",
		HostPort: uint16(9602),
	}

	// connect vrtr agent to master
	err = ofnetAgent.AddMaster(&masterInfo, &resp)
	if err != nil {
		t.Errorf("Error adding master %+v. Err: %v", masterInfo, err)
	}

	log.Infof("Created vrouter ofnet agent: %v", ofnetAgent)

	brName := "ovsbr60"
	ovsDriver := ovsdbDriver.NewOvsDriver(brName)
	err = ovsDriver.AddController("127.0.0.1", ovsPort)
	if err != nil {
		t.Fatalf("Error adding controller to ovs: %s", brName)
	}

	// Wait for switch to connect to controller
	ofnetAgent.WaitForSwitchConnection()

	// Create a vlan for the endpoint
	ofnetAgent.AddNetwork(1, 1, "")

	macAddr, _ := net.ParseMAC("00:01:02:03:04:05")
	endpoint := EndpointInfo{
		EndpointGroup: 100,
		PortNo:        12,
		MacAddr:       macAddr,
		Vlan:          1,
		IpAddr:        net.ParseIP("10.2.2.2"),
	}

	log.Infof("Adding Local endpoint: %+v", endpoint)

	// Add an Endpoint
	err = ofnetAgent.AddLocalEndpoint(endpoint)
	if err != nil {
		t.Errorf("Error adding endpoint. Err: %v", err)
		return
	}

	tcpRule := &OfnetPolicyRule{
		RuleId:           "tcpRule",
		Priority:         100,
		SrcEndpointGroup: 100,
		DstEndpointGroup: 200,
		SrcIpAddr:        "10.10.10.10/24",
		DstIpAddr:        "10.1.1.1/24",
		IpProtocol:       6,
		DstPort:          100,
		SrcPort:          200,
		Action:           "accept",
	}

	log.Infof("Adding rule: %+v", tcpRule)

	// Add a policy
	err = ofnetMaster.AddRule(tcpRule)
	if err != nil {
		t.Errorf("Error installing tcpRule {%+v}. Err: %v", tcpRule, err)
		return
	}

	udpRule := &OfnetPolicyRule{
		RuleId:           "udpRule",
		Priority:         100,
		SrcEndpointGroup: 300,
		DstEndpointGroup: 400,
		SrcIpAddr:        "20.20.20.20/24",
		DstIpAddr:        "20.2.2.2/24",
		IpProtocol:       17,
		DstPort:          300,
		SrcPort:          400,
		Action:           "deny",
	}

	log.Infof("Adding rule: %+v", udpRule)

	// Add the policy
	err = ofnetMaster.AddRule(udpRule)
	if err != nil {
		t.Errorf("Error installing udpRule {%+v}. Err: %v", udpRule, err)
		return
	}

	// Get all the flows
	flowList, err := ofctlFlowDump(brName)
	if err != nil {
		t.Errorf("Error getting flow entries. Err: %v", err)
		return
	}

	// verify src group flow
	srcGrpFlowMatch := fmt.Sprintf("priority=100,in_port=12 actions=write_metadata:0x640000/0x7fff0000")
	if !ofctlFlowMatch(flowList, VLAN_TBL_ID, srcGrpFlowMatch) {
		t.Errorf("Could not find the flow %s on ovs %s", srcGrpFlowMatch, brName)
		return
	}

	log.Infof("Found src group %s on ovs %s", srcGrpFlowMatch, brName)

	// verify dst group flow
	dstGrpFlowMatch := fmt.Sprintf("priority=100,ip,nw_dst=10.2.2.2 actions=write_metadata:0xc8/0xfffe")
	if !ofctlFlowMatch(flowList, DST_GRP_TBL_ID, dstGrpFlowMatch) {
		t.Errorf("Could not find the flow %s on ovs %s", dstGrpFlowMatch, brName)
		return
	}

	log.Infof("Found dst group %s on ovs %s", dstGrpFlowMatch, brName)

	// verify tcp rule flow entry exists
	tcpFlowMatch := fmt.Sprintf("priority=110,tcp,metadata=0x640190/0x7ffffffe,nw_src=10.10.10.0/24,nw_dst=10.1.1.0/24,tp_src=200,tp_dst=100")
	if !ofctlFlowMatch(flowList, POLICY_TBL_ID, tcpFlowMatch) {
		t.Errorf("Could not find the flow %s on ovs %s", tcpFlowMatch, brName)
		return
	}

	log.Infof("Found tcp rule %s on ovs %s", tcpFlowMatch, brName)

	// verify udp rule flow
	udpFlowMatch := fmt.Sprintf("priority=110,udp,metadata=0x12c0320/0x7ffffffe,nw_src=20.20.20.0/24,nw_dst=20.2.2.0/24,tp_src=400,tp_dst=300")
	if !ofctlFlowMatch(flowList, POLICY_TBL_ID, udpFlowMatch) {
		t.Errorf("Could not find the flow %s on ovs %s", udpFlowMatch, brName)
		return
	}

	log.Infof("Found udp rule %s on ovs %s", udpFlowMatch, brName)

	// Delete policies
	err = ofnetMaster.DelRule(tcpRule)
	if err != nil {
		t.Errorf("Error deleting tcpRule {%+v}. Err: %v", tcpRule, err)
		return
	}
	err = ofnetMaster.DelRule(udpRule)
	if err != nil {
		t.Errorf("Error deleting udpRule {%+v}. Err: %v", udpRule, err)
		return
	}
	err = ofnetAgent.RemoveLocalEndpoint(endpoint.PortNo)
	if err != nil {
		t.Errorf("Error deleting endpoint: %+v. Err: %v", endpoint, err)
		return
	}

	log.Infof("Deleted all policy entries")

	// Get the flows again
	flowList, err = ofctlFlowDump(brName)
	if err != nil {
		t.Errorf("Error getting flow entries. Err: %v", err)
		return
	}

	// Make sure flows are gone
	if ofctlFlowMatch(flowList, VLAN_TBL_ID, srcGrpFlowMatch) {
		t.Errorf("Still found the flow %s on ovs %s", srcGrpFlowMatch, brName)
		return
	}
	if ofctlFlowMatch(flowList, DST_GRP_TBL_ID, dstGrpFlowMatch) {
		t.Errorf("Still found the flow %s on ovs %s", dstGrpFlowMatch, brName)
		return
	}
	if ofctlFlowMatch(flowList, POLICY_TBL_ID, tcpFlowMatch) {
		t.Errorf("Still found the flow %s on ovs %s", tcpFlowMatch, brName)
		return
	}
	if ofctlFlowMatch(flowList, POLICY_TBL_ID, udpFlowMatch) {
		t.Errorf("Still found the flow %s on ovs %s", udpFlowMatch, brName)
		return
	}

	log.Infof("Verified all flows are deleted")

	// Delete the bridge
	ovsDriver.DeleteBridge(brName)
}
