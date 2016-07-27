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

package integration

import (
	"fmt"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
	"time"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/daemon"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/netplugin/agent"
	"github.com/contiv/netplugin/netplugin/cluster"
	"github.com/contiv/netplugin/netplugin/plugin"
	"github.com/contiv/ofnet"

	log "github.com/Sirupsen/logrus"
	. "gopkg.in/check.v1"
)

type clusterState struct {
	masterDaemon *daemon.MasterDaemon // master instance
	pluginAgent  *agent.Agent         // netplugin agent
	hostLabel    string               // local host name
	localIP      string               // local ip addr
	uniqEPID     uint64               // rolling int to generate unique EP IDs
}

func NewCluster(fwdMode, clusterStore string) (*clusterState, error) {
	// get local host name
	hostLabel, err := os.Hostname()
	if err != nil {
		log.Fatalf("Failed to fetch hostname. Error: %s", err)
	}

	// get local IP addr
	localIP, err := cluster.GetLocalAddr()
	if err != nil {
		log.Fatalf("Error getting local address. Err: %v", err)
	}

	// create master daemon
	md := &daemon.MasterDaemon{
		ListenURL:    ":9999",
		ClusterStore: clusterStore,
		ClusterMode:  "test",
		DnsEnabled:   false,
	}

	// initialize the plugin config
	pluginConfig := plugin.Config{
		Drivers: plugin.Drivers{
			Network: "ovs",
			State:   strings.Split(clusterStore, "://")[0],
		},
		Instance: core.InstanceInfo{
			HostLabel:  hostLabel,
			CtrlIP:     localIP,
			VtepIP:     localIP,
			VlanIntf:   "eth2",
			FwdMode:    fwdMode,
			DbURL:      clusterStore,
			PluginMode: "test",
		},
	}

	// initialize master daemon
	md.Init()

	// Run daemon FSM
	go md.RunMasterFsm()

	// Create a new agent
	ag := agent.NewAgent(&pluginConfig)

	// Process all current state
	ag.ProcessCurrentState()

	// post initialization processing
	ag.PostInit()

	// handle events
	go func() {
		if err := ag.HandleEvents(); err != nil {
			log.Infof("Netplugin exiting due to error: %v", err)
			os.Exit(1)
		}
	}()

	// Wait for a second for things to settle down
	time.Sleep(time.Second)

	cl := &clusterState{
		masterDaemon: md,
		pluginAgent:  ag,
		hostLabel:    hostLabel,
		localIP:      localIP,
	}

	return cl, nil
}

// assertNoErr utility function to assert no error
func assertNoErr(err error, c *C, msg string) {
	if err != nil {
		log.Errorf("Error %s. Err: %v", msg, err)
		debug.PrintStack()
		c.Fatalf("Error %s. Err: %v", msg, err)
	}
}

// allocAddress gets an address from the master
func (its *integTestSuite) allocAddress(addrPool, networkID, prefAddress string) (string, error) {
	// Build an alloc request to be sent to master
	allocReq := master.AddressAllocRequest{
		AddressPool:          addrPool,
		NetworkID:            networkID,
		PreferredIPv4Address: prefAddress,
	}

	// Make a REST call to master
	var allocResp master.AddressAllocResponse
	err := cluster.MasterPostReq("/plugin/allocAddress", &allocReq, &allocResp)
	if err != nil {
		log.Errorf("master failed to allocate address. Err: %v", err)
		return "", err
	}

	return strings.Split(allocResp.IPv4Address, "/")[0], nil
}

// createEndpoint creates an endpoint using netplugin api
func (its *integTestSuite) createEndpoint(tenantName, netName, epgName, v4Addr, v6Addr string) (*mastercfg.CfgEndpointState, error) {
	its.cluster.uniqEPID++
	epID := fmt.Sprintf("%s-%s-%s-%d", tenantName, netName, epgName, its.cluster.uniqEPID)
	// Build endpoint request
	mreq := master.CreateEndpointRequest{
		TenantName:  tenantName,
		NetworkName: netName,
		ServiceName: epgName,
		EndpointID:  epID,
		ConfigEP: intent.ConfigEP{
			Container:   epID,
			Host:        its.cluster.hostLabel,
			IPAddress:   v4Addr,
			IPv6Address: v6Addr,
			ServiceName: epgName,
		},
	}

	var mresp master.CreateEndpointResponse
	err := cluster.MasterPostReq("/plugin/createEndpoint", &mreq, &mresp)
	if err != nil {
		log.Errorf("master failed to create endpoint. Err: %v", err)
		return nil, err
	}

	log.Infof("Got endpoint create resp from master: %+v", mresp)
	netID := netName + "." + tenantName

	// Ask netplugin to create the endpoint
	err = its.cluster.pluginAgent.Plugin().CreateEndpoint(netID + "-" + epID)
	if err != nil {
		log.Errorf("Endpoint creation failed. Error: %s", err)
		return nil, err
	}

	return &mresp.EndpointConfig, nil
}

// createEndpointsParallel creates endpoints in parallel
func (its *integTestSuite) createEndpointsParallel(tenantName, netName, epgName string) ([]*mastercfg.CfgEndpointState, error) {
	epChan := make(chan *mastercfg.CfgEndpointState, its.parallels*2)
	errChan := make(chan error, its.parallels*2)

	// create endpoints in parallel
	for j := 0; j < its.parallels; j++ {
		go func() {
			addr, err := its.allocAddress("", fmt.Sprintf("%s.%s", netName, tenantName), "")
			if err != nil {
				errChan <- err
				return
			}

			// create an endpoint in the network
			epCfg, err := its.createEndpoint(tenantName, netName, epgName, addr, "")
			if err != nil {
				errChan <- err
				return
			}

			// return the endpoint id
			epChan <- epCfg
			errChan <- nil
		}()
	}

	// wait for all epIDs
	epCfgList := []*mastercfg.CfgEndpointState{}
	for j := 0; j < its.parallels; j++ {
		err := <-errChan
		if err != nil {
			return nil, err
		}
		epCfg := <-epChan
		epCfgList = append(epCfgList, epCfg)
	}

	return epCfgList, nil
}

// deleteEndpoint deletes an endpoint using netplugin api
func (its *integTestSuite) deleteEndpoint(tenantName, netName, epgName string, epCfg *mastercfg.CfgEndpointState) error {
	// Build endpoint delete request
	delreq := master.DeleteEndpointRequest{
		TenantName:  tenantName,
		NetworkName: netName,
		ServiceName: epgName,
		EndpointID:  epCfg.ContName,
	}

	var delResp master.DeleteEndpointResponse
	err := cluster.MasterPostReq("/plugin/deleteEndpoint", &delreq, &delResp)
	if err != nil {
		log.Errorf("master failed to delete endpoint. Err: %v", err)
		return err
	}

	// delete the endpoint
	err = its.cluster.pluginAgent.Plugin().DeleteEndpoint(epCfg.ID)
	if err != nil {
		log.Errorf("Error deleting endpoint %s. Err: %v", epCfg.ID, err)
		return err
	}

	return nil
}

// deleteEndpointsParallel deletes endpoints in parallel
func (its *integTestSuite) deleteEndpointsParallel(tenantName, netName, epgName string, epCfgList []*mastercfg.CfgEndpointState) error {
	count := 0
	errChan := make(chan error, its.parallels*2)

	// delete all epgs
	for _, epCfg := range epCfgList {
		go func(epCfg *mastercfg.CfgEndpointState) {
			err := its.deleteEndpoint(tenantName, netName, epgName, epCfg)
			errChan <- err
		}(epCfg)
		count++
	}
	// wait for all deletes to complete
	for j := 0; j < count; j++ {
		err := <-errChan
		if err != nil {
			return err
		}
	}

	return nil
}

func (its *integTestSuite) verifyEndpointInspect(tenantName, netName string, epCfg *mastercfg.CfgEndpointState, c *C) {
	// verify network & endpoint inspect output
	insp, err := its.client.NetworkInspect(tenantName, netName)
	assertNoErr(err, c, "inspecting network")
	log.Infof("Inspecting network: %+v", insp)

	// walk all endpoints and verify endpoint exists
	foundCount := 0
	for _, ep := range insp.Oper.Endpoints {
		if ep.Name == epCfg.ContName {
			c.Assert(len(ep.IpAddress), Equals, 2)
			c.Assert(ep.IpAddress[0], Equals, epCfg.IPAddress)
			c.Assert(ep.Network, Equals, fmt.Sprintf("%s.%s", netName, tenantName))
			c.Assert(ep.MacAddress, Equals, epCfg.MacAddress)
			c.Assert(ep.HomingHost, Equals, its.cluster.hostLabel)
			foundCount++
		}
	}

	if foundCount == 0 {
		c.Fatalf("Endpoint %s not found in network %s.%s", epCfg.ContName, netName, tenantName)
	} else if foundCount > 1 {
		c.Fatalf("Endpoint %s found multiple times in network %s.%s", epCfg.ContName, netName, tenantName)
	}
}

// Run an ovs-ofctl command
func runOfctlCmd(cmd, brName string) ([]byte, error) {
	cmdStr := fmt.Sprintf("sudo /usr/bin/ovs-ofctl -O Openflow13 %s %s", cmd, brName)
	out, err := exec.Command("/bin/sh", "-c", cmdStr).Output()
	if err != nil {
		log.Errorf("error running ovs-ofctl %s %s. Error: %v", cmd, brName, err)
		return nil, err
	}

	return out, nil
}

// dump the flows and parse the Output
func ofctlFlowDump(brName string) ([]string, error) {
	flowDump, err := runOfctlCmd("dump-flows", brName)
	if err != nil {
		log.Errorf("Error running dump-flows on %s. Err: %v", brName, err)
		return nil, err
	}

	log.Debugf("Flow dump: %s", flowDump)
	flowOutStr := string(flowDump)
	flowDb := strings.Split(flowOutStr, "\n")[1:]

	log.Debugf("flowDb: %+v", flowDb)

	var flowList []string
	for _, flow := range flowDb {
		felem := strings.Fields(flow)
		if len(felem) > 2 {
			felem = append(felem[:1], felem[2:]...)
			felem = append(felem[:2], felem[4:]...)
			fstr := strings.Join(felem, " ")
			flowList = append(flowList, fstr)
		}
	}

	log.Debugf("flowList: %+v", flowList)

	return flowList, nil
}

// Find a flow in flow list and match its action
func ofctlFlowMatch(flowList []string, tableId int, matchStr string) bool {
	tblStr := fmt.Sprintf("table=%d", tableId)
	for _, flowEntry := range flowList {
		log.Debugf("Looking for %s in %s", matchStr, flowEntry)
		if strings.Contains(flowEntry, tblStr) && strings.Contains(flowEntry, matchStr) {
			return true
		}
	}

	return false
}

// verifyEndpointFlow verifies endpoint flow exists
func (its *integTestSuite) verifyEndpointFlow(epCfg *mastercfg.CfgEndpointState, c *C) {
	// determine the bridge name
	ovsBridgeName := "contivVlanBridge"
	if its.encap == "vxlan" {
		ovsBridgeName = "contivVxlanBridge"
	}

	// get the flow dump
	ofdump, err := ofctlFlowDump(ovsBridgeName)
	assertNoErr(err, c, "dumping flow entries")

	// verify dst group entry exists
	dstGrpFmt := fmt.Sprintf("nw_dst=%s actions=write_metadata:%d/0xfffe", epCfg.IPAddress, epCfg.EndpointGroupID)
	dstGrpTbl := ofnet.DST_GRP_TBL_ID
	c.Assert(ofctlFlowMatch(ofdump, dstGrpTbl, dstGrpFmt), Equals, true)

	// vxlan mode specific checks
	if its.encap == "vxlan" {
		if its.fwdMode == "routing" {
			// routing mode
			ipFlow := fmt.Sprintf("nw_dst=%s actions=set_field:00:00:11:11:11:11->eth_src,set_field:%s->eth_dst,output:", epCfg.IPAddress, epCfg.MacAddress)
			ipTbl := ofnet.IP_TBL_ID
			c.Assert(ofctlFlowMatch(ofdump, ipTbl, ipFlow), Equals, true)
		} else {
			// bridging mode
			macFlow := fmt.Sprintf("dl_dst=%s actions=pop_vlan,output:", epCfg.MacAddress)
			macTbl := ofnet.MAC_DEST_TBL_ID
			c.Assert(ofctlFlowMatch(ofdump, macTbl, macFlow), Equals, true)
		}
	}
}

// verifyEndpointFlowRemoved verifies endpoint flow does not exist
func (its *integTestSuite) verifyEndpointFlowRemoved(epCfg *mastercfg.CfgEndpointState, c *C) {
	// determine the bridge name
	ovsBridgeName := "contivVlanBridge"
	if its.encap == "vxlan" {
		ovsBridgeName = "contivVxlanBridge"
	}

	// get the flow dump
	ofdump, err := ofctlFlowDump(ovsBridgeName)
	assertNoErr(err, c, "dumping flow entries")

	// verify dst group entry exists
	dstGrpFmt := fmt.Sprintf("nw_dst=%s actions=write_metadata:%d/0xfffe", epCfg.IPAddress, epCfg.EndpointGroupID)
	dstGrpTbl := ofnet.DST_GRP_TBL_ID
	c.Assert(ofctlFlowMatch(ofdump, dstGrpTbl, dstGrpFmt), Equals, false)

	// vxlan mode specific checks
	if its.encap == "vxlan" {
		if its.fwdMode == "routing" {
			// routing mode
			ipFlow := fmt.Sprintf("nw_dst=%s actions=set_field:00:00:11:11:11:11->eth_src,set_field:%s->eth_dst,output:", epCfg.IPAddress, epCfg.MacAddress)
			ipTbl := ofnet.IP_TBL_ID
			c.Assert(ofctlFlowMatch(ofdump, ipTbl, ipFlow), Equals, false)
		} else {
			// bridging mode
			macFlow := fmt.Sprintf("dl_dst=%s actions=pop_vlan,output:", epCfg.MacAddress)
			macTbl := ofnet.MAC_DEST_TBL_ID
			c.Assert(ofctlFlowMatch(ofdump, macTbl, macFlow), Equals, false)
		}
	}
}
