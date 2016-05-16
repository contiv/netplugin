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

package drivers

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/state"
)

const (
	createEpID                 = "testCreateEp"
	createEpIDStateful         = "testCreateEpStateful"
	createEpIDStatefulMismatch = "testCreateEpStatefulMismatch"
	deleteEpID                 = "testDeleteEp"
	testOvsNwID                = "testNetID"
	testOvsNwIDStateful        = "testNetIDStateful"
	testOvsEpGroupID           = "10"
	testOvsEpGroupIDStateful   = "11"
	testOvsEpgHandle           = 10
	testOvsEpgHandleStateful   = 11
	testPktTag                 = 100
	testPktTagStateful         = 200
	testExtPktTag              = 10000
	testIntfName               = "testIntf"
	testSubnetIP               = "10.1.1.0"
	testSubnetLen              = 24
	testEpAddress              = "10.1.1.1"
	testEpMacAddress           = "02:02:0A:01:01:01"
	testHostLabel              = "testHost"
	testHostLabelStateful      = "testHostStateful"
	testCurrPortNum            = 10
	testVlanUplinkPort         = "eth2"
	testGateway                = "10.1.1.254"
	testTenant                 = "default"
)

func createCommonState(stateDriver core.StateDriver) error {
	//create all the common config state required by the tests

	{
		cfgNw := &mastercfg.CfgNetworkState{}
		cfgNw.ID = testOvsNwID
		cfgNw.PktTag = testPktTag
		cfgNw.ExtPktTag = testExtPktTag
		cfgNw.SubnetIP = testSubnetIP
		cfgNw.SubnetLen = testSubnetLen
		cfgNw.Gateway = testGateway
		cfgNw.Tenant = testTenant
		cfgNw.StateDriver = stateDriver
		if err := cfgNw.Write(); err != nil {
			return err
		}
	}

	{
		cfgNw := &mastercfg.CfgNetworkState{}
		cfgNw.ID = testOvsNwIDStateful
		cfgNw.PktTag = testPktTagStateful
		cfgNw.ExtPktTag = testExtPktTag
		cfgNw.SubnetIP = testSubnetIP
		cfgNw.SubnetLen = testSubnetLen
		cfgNw.Gateway = testGateway
		cfgNw.Tenant = testTenant
		cfgNw.StateDriver = stateDriver
		if err := cfgNw.Write(); err != nil {
			return err
		}
	}

	{
		cfgEpGroup := &mastercfg.EndpointGroupState{}
		cfgEpGroup.StateDriver = stateDriver
		cfgEpGroup.ID = testOvsEpGroupID
		cfgEpGroup.PktTagType = "vlan"
		cfgEpGroup.PktTag = testPktTag
		if err := cfgEpGroup.Write(); err != nil {
			return err
		}
	}

	{
		cfgEpGroup := &mastercfg.EndpointGroupState{}
		cfgEpGroup.StateDriver = stateDriver
		cfgEpGroup.ID = testOvsEpGroupIDStateful
		cfgEpGroup.PktTagType = "vlan"
		cfgEpGroup.PktTag = testPktTagStateful
		if err := cfgEpGroup.Write(); err != nil {
			return err
		}
	}

	{
		cfgEp := &mastercfg.CfgEndpointState{}
		cfgEp.ID = createEpID
		cfgEp.EndpointGroupID = testOvsEpgHandle
		cfgEp.NetID = testOvsNwID
		cfgEp.IntfName = ""
		cfgEp.IPAddress = testEpAddress
		cfgEp.MacAddress = testEpMacAddress
		cfgEp.StateDriver = stateDriver
		if err := cfgEp.Write(); err != nil {
			return err
		}
	}

	{
		cfgEp := &mastercfg.CfgEndpointState{}
		cfgEp.ID = createEpIDStateful
		cfgEp.EndpointGroupID = testOvsEpgHandleStateful
		cfgEp.NetID = testOvsNwIDStateful
		cfgEp.IntfName = ""
		cfgEp.IPAddress = testEpAddress
		cfgEp.MacAddress = testEpMacAddress
		cfgEp.StateDriver = stateDriver
		if err := cfgEp.Write(); err != nil {
			return err
		}
	}

	{
		cfgEp := &mastercfg.CfgEndpointState{}
		cfgEp.ID = createEpIDStatefulMismatch
		cfgEp.EndpointGroupID = testOvsEpgHandle
		cfgEp.NetID = testOvsNwID
		cfgEp.IntfName = ""
		cfgEp.IPAddress = testEpAddress
		cfgEp.MacAddress = testEpMacAddress
		cfgEp.StateDriver = stateDriver
		if err := cfgEp.Write(); err != nil {
			return err
		}
	}

	{
		cfgEp := &mastercfg.CfgEndpointState{}
		cfgEp.ID = deleteEpID
		cfgEp.EndpointGroupID = testOvsEpgHandle
		cfgEp.NetID = testOvsNwID
		cfgEp.IPAddress = testEpAddress
		cfgEp.MacAddress = testEpMacAddress
		cfgEp.StateDriver = stateDriver
		if err := cfgEp.Write(); err != nil {
			return err
		}
	}

	return nil
}

func initOvsDriver(t *testing.T) *OvsDriver {
	driver := &OvsDriver{}
	fMode := "bridge"
	stateDriver := &state.FakeStateDriver{}
	stateDriver.Init(nil)
	instInfo := &core.InstanceInfo{HostLabel: testHostLabel,
		StateDriver: stateDriver, FwdMode: fMode}

	err := createCommonState(stateDriver)
	if err != nil {
		t.Fatalf("common state creation failed. Error: %s", err)
	}

	err = driver.Init(instInfo)
	if err != nil {
		t.Fatalf("driver init failed. Error: %s", err)
	}

	return driver
}

func TestOvsDriverInit(t *testing.T) {
	driver := initOvsDriver(t)
	defer func() { driver.Deinit() }()
}

func TestOvsDriverInitStatefulStart(t *testing.T) {
	driver := &OvsDriver{}
	fMode := "bridge"
	stateDriver := &state.FakeStateDriver{}
	stateDriver.Init(nil)
	instInfo := &core.InstanceInfo{HostLabel: testHostLabelStateful,
		StateDriver: stateDriver, FwdMode: fMode}

	operOvs := &OvsDriverOperState{CurrPortNum: 10}
	operOvs.StateDriver = stateDriver
	operOvs.ID = testHostLabelStateful
	err := operOvs.Write()
	if err != nil {
		t.Fatalf("writing driver oper state failed. Error: %s", err)
	}

	err = driver.Init(instInfo)
	if err != nil {
		t.Fatalf("driver init failed. Error: %s", err)
	}

	if driver.oper.CurrPortNum != testCurrPortNum {
		t.Fatalf("Unexpected driver oper state. Expected port num: %d, rcvd port number: %d",
			testCurrPortNum, driver.oper.CurrPortNum)
	}

	defer func() { driver.Deinit() }()
}

func TestOvsDriverInitInvalidConfig(t *testing.T) {
	driver := &OvsDriver{}
	instInfo := &core.InstanceInfo{HostLabel: testHostLabel,
		StateDriver: nil, FwdMode: "bridge"}

	err := driver.Init(nil)
	if err == nil {
		t.Fatalf("driver init succeeded. Should have failed!")
	}

	err = driver.Init(instInfo)
	if err == nil {
		t.Fatalf("driver init succeeded. Should have failed!")
	}

	defer func() { driver.Deinit() }()
}

func TestOvsDriverInitInvalidState(t *testing.T) {
	driver := &OvsDriver{}
	fMode := "bridge"
	instInfo := &core.InstanceInfo{HostLabel: testHostLabel, StateDriver: nil,
		FwdMode: fMode}

	err := driver.Init(instInfo)
	if err == nil {
		t.Fatalf("driver init succeeded. Should have failed!")
	}
	defer func() { driver.Deinit() }()
}

func TestOvsDriverInitInvalidInstanceInfo(t *testing.T) {
	driver := &OvsDriver{}

	err := driver.Init(nil)
	if err == nil {
		t.Fatalf("driver init succeeded. Should have failed!")
	}
	defer func() { driver.Deinit() }()
}

func TestOvsDriverDeinit(t *testing.T) {
	driver := initOvsDriver(t)

	driver.Deinit()

	output, err := exec.Command("ovs-vsctl", "list", "Bridge").CombinedOutput()
	if err != nil || strings.Contains(string(output), vlanBridgeName) ||
		strings.Contains(string(output), vxlanBridgeName) {
		t.Fatalf("deinit failed. Error: %s Output: %s", err, output)
	}

}

func TestOvsDriverCreateEndpoint(t *testing.T) {
	driver := initOvsDriver(t)
	defer func() { driver.Deinit() }()
	id := createEpID

	// create network
	err := driver.CreateNetwork(testOvsNwID)
	if err != nil {
		t.Fatalf("network creation failed. Error: %s", err)
	}
	defer func() { driver.DeleteNetwork(testOvsNwID, "", "", testPktTag, testExtPktTag, testGateway, testTenant) }()

	// create endpoint
	err = driver.CreateEndpoint(id)
	if err != nil {
		t.Fatalf("endpoint creation failed. Error: %s", err)
	}

	defer func() { driver.DeleteEndpoint(id) }()

	output, err := exec.Command("ovs-vsctl", "list", "Port").CombinedOutput()
	expectedPortName := fmt.Sprintf("port%d", driver.oper.CurrPortNum)
	if err != nil || !strings.Contains(string(output), expectedPortName) {
		t.Fatalf("port lookup failed. Error: %s expected port: %s Output: %s",
			err, expectedPortName, output)
	}

	output, err = exec.Command("ovs-vsctl", "list", "Interface").CombinedOutput()
	if err != nil || !strings.Contains(string(output), expectedPortName) {
		t.Fatalf("interface lookup failed. Error: %s expected port: %s Output: %s",
			err, expectedPortName, output)
	}
}

func TestOvsDriverCreateEndpointStateful(t *testing.T) {
	driver := initOvsDriver(t)
	defer func() { driver.Deinit() }()
	id := createEpIDStateful

	// create network
	err := driver.CreateNetwork(testOvsNwIDStateful)
	if err != nil {
		t.Fatalf("network creation failed. Error: %s", err)
	}
	defer func() {
		driver.DeleteNetwork(testOvsNwIDStateful, "", "", testPktTagStateful, testExtPktTag, testGateway, testTenant)
	}()

	// Create endpoint
	err = driver.CreateEndpoint(id)
	if err != nil {
		t.Fatalf("endpoint creation failed. Error: %s", err)
	}

	defer func() { driver.DeleteEndpoint(id) }()

	err = driver.CreateEndpoint(id)
	if err != nil {
		t.Fatalf("stateful endpoint creation failed. Error: %s", err)
	}

	output, err := exec.Command("ovs-vsctl", "list", "Port").CombinedOutput()
	expectedPortName := fmt.Sprintf("port%d", driver.oper.CurrPortNum)
	if err != nil || !strings.Contains(string(output), expectedPortName) {
		t.Fatalf("port lookup failed. Error: %s expected port: %s Output: %s",
			err, expectedPortName, output)
	}

	output, err = exec.Command("ovs-vsctl", "list", "Interface").CombinedOutput()
	if err != nil || !strings.Contains(string(output), expectedPortName) {
		t.Fatalf("interface lookup failed. Error: %s expected port: %s Output: %s",
			err, expectedPortName, output)
	}
}

func TestOvsDriverCreateEndpointStatefulStateMismatch(t *testing.T) {
	driver := initOvsDriver(t)
	defer func() { driver.Deinit() }()
	id := createEpIDStatefulMismatch

	// create network
	err := driver.CreateNetwork(testOvsNwID)
	if err != nil {
		t.Fatalf("network creation failed. Error: %s", err)
	}
	defer func() { driver.DeleteNetwork(testOvsNwID, "", "", testPktTag, testExtPktTag, testGateway, testTenant) }()

	// create second network
	err = driver.CreateNetwork(testOvsNwIDStateful)
	if err != nil {
		t.Fatalf("network creation failed. Error: %s", err)
	}
	defer func() {
		driver.DeleteNetwork(testOvsNwIDStateful, "", "", testPktTagStateful, testExtPktTag, testGateway, testTenant)
	}()

	// create endpoint
	err = driver.CreateEndpoint(id)
	if err != nil {
		t.Fatalf("endpoint creation failed. Error: %s", err)
	}

	cfgEp := mastercfg.CfgEndpointState{}
	cfgEp.StateDriver = driver.oper.StateDriver
	err = cfgEp.Read(id)
	if err != nil {
		t.Fatalf("failed to read ep config. Error: %s", err)
	}
	cfgEp.NetID = testOvsNwIDStateful

	err = cfgEp.Write()
	if err != nil {
		t.Fatalf("failed to write ep config. Error: %s", err)
	}

	err = driver.CreateEndpoint(id)
	if err != nil {
		t.Fatalf("stateful endpoint creation failed. Error: %s", err)
	}

	defer func() { driver.DeleteEndpoint(id) }()

	output, err := exec.Command("ovs-vsctl", "list", "Port").CombinedOutput()
	expectedPortName := fmt.Sprintf("port%d", driver.oper.CurrPortNum)
	if err != nil || !strings.Contains(string(output), expectedPortName) {
		t.Fatalf("port lookup failed. Error: %s expected port: %s Output: %s",
			err, expectedPortName, output)
	}

	output, err = exec.Command("ovs-vsctl", "list", "Interface").CombinedOutput()
	if err != nil || !strings.Contains(string(output), expectedPortName) {
		t.Fatalf("interface lookup failed. Error: %s expected port: %s Output: %s",
			err, expectedPortName, output)
	}
}

func TestOvsDriverDeleteEndpoint(t *testing.T) {
	driver := initOvsDriver(t)
	defer func() { driver.Deinit() }()
	id := deleteEpID

	// create network
	err := driver.CreateNetwork(testOvsNwID)
	if err != nil {
		t.Fatalf("network creation failed. Error: %s", err)
	}
	defer func() { driver.DeleteNetwork(testOvsNwID, "", "", testPktTag, testExtPktTag, testGateway, testTenant) }()

	// create endpoint
	err = driver.CreateEndpoint(id)
	if err != nil {
		t.Fatalf("endpoint Creation failed. Error: %s", err)
	}

	// XXX: DeleteEndpoint() depends on the ovsdb cache to have been updated
	// once the port is created. The cache update happens asynchronously through
	// a libovsdb callback. So there is a timing window where cache might not yet
	// have been updated. Adding a delay to workaround.
	// Also see contiv/netplugin/issues/78
	time.Sleep(1 * time.Second)

	err = driver.DeleteEndpoint(id)
	if err != nil {
		t.Fatalf("endpoint Deletion failed. Error: %s", err)
	}

	output, err := exec.Command("ovs-vsctl", "list", "Port").CombinedOutput()
	expectedPortName := fmt.Sprintf(portNameFmt, driver.oper.CurrPortNum+1)
	if err != nil || strings.Contains(string(output), expectedPortName) {
		t.Fatalf("port lookup succeeded after delete. Error: %s Output: %s", err, output)
	}

	output, err = exec.Command("ovs-vsctl", "list", "Interface").CombinedOutput()
	if err != nil || strings.Contains(string(output), testIntfName) {
		t.Fatalf("interface lookup succeeded after delete. Error: %s Output: %s", err, output)
	}
}

func TestOvsDriverAddUplink(t *testing.T) {
	driver := initOvsDriver(t)
	defer func() { driver.Deinit() }()

	// Add uplink
	err := driver.switchDb["vlan"].AddUplinkPort(testVlanUplinkPort)
	if err != nil {
		t.Fatalf("Could not add uplink %s to vlan OVS. Err: %v", testVlanUplinkPort, err)
	}

	time.Sleep(300 * time.Millisecond)

	// verify uplink port
	output, err := exec.Command("ovs-vsctl", "list", "Interface").CombinedOutput()
	if err != nil || !strings.Contains(string(output), testVlanUplinkPort) {
		t.Fatalf("interface lookup failed. Error: %s expected port: %s Output: %s",
			err, testVlanUplinkPort, output)
	}
}

func TestOvsDriverVethNameConflict(t *testing.T) {
	driver := initOvsDriver(t)
	defer func() { driver.Deinit() }()

	// Create conflicting Veth interface pairs
	intfNum := driver.oper.CurrPortNum + 1
	createVethPair(fmt.Sprintf("vport%d", intfNum), fmt.Sprintf("vvport%d", intfNum))
	createVethPair(fmt.Sprintf("vport%d", intfNum+1), fmt.Sprintf("vvport%d", intfNum+1))
	createVethPair(fmt.Sprintf("uport%d", intfNum+2), fmt.Sprintf("vvport%d", intfNum+2))
	defer func() { deleteVethPair(fmt.Sprintf("vport%d", intfNum), fmt.Sprintf("vvport%d", intfNum)) }()
	defer func() { deleteVethPair(fmt.Sprintf("vport%d", intfNum+1), fmt.Sprintf("vvport%d", intfNum+1)) }()
	defer func() { deleteVethPair(fmt.Sprintf("uport%d", intfNum+2), fmt.Sprintf("vvport%d", intfNum+2)) }()

	// add a duplicate interface entry into OVS bridge
	exec.Command("sudo", "ovs-vsctl", "add-port", "contivVlanBridge", fmt.Sprintf("vvport%d", intfNum+3)).CombinedOutput()

	// create network
	err := driver.CreateNetwork(testOvsNwID)
	if err != nil {
		t.Fatalf("network creation failed. Error: %s", err)
	}
	defer func() { driver.DeleteNetwork(testOvsNwID, "", "", testPktTag, testExtPktTag, testGateway, testTenant) }()

	// create endpoint
	err = driver.CreateEndpoint(createEpID)
	if err != nil {
		t.Fatalf("endpoint creation failed. Error: %s", err)
	}
	defer func() { driver.DeleteEndpoint(createEpID) }()

	// verify interface got creates
	output, err := exec.Command("ovs-vsctl", "list", "Interface").CombinedOutput()
	if err != nil || !strings.Contains(string(output), fmt.Sprintf("vport%d", intfNum+3)) ||
		strings.Contains(string(output), fmt.Sprintf("tag                 : %d", testPktTag)) {
		t.Fatalf("interface lookup failed. Error: %s expected port: %s Output: %s",
			err, fmt.Sprintf("vport%d", intfNum+3), output)
	}
}
