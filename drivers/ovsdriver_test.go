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
	"github.com/contiv/netplugin/state"
)

const (
	createEpID                 = "testCreateEp"
	createEpIDStateful         = "testCreateEpStateful"
	createEpIDStatefulMismatch = "testCreateEpStatefulMismatch"
	deleteEpID                 = "testDeleteEp"
	peerHostID                 = "testPeerHost"
	peerHostName               = "peerHost1"
	peerHostIP                 = "127.0.0.1"
	testOvsNwID                = "testNetID"
	testOvsNwIDStateful        = "testNetIDStateful"
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
)

func createCommonState(stateDriver core.StateDriver) error {
	//create all the common config state required by the tests

	{
		cfgNw := &OvsCfgNetworkState{}
		cfgNw.ID = testOvsNwID
		cfgNw.PktTag = testPktTag
		cfgNw.ExtPktTag = testExtPktTag
		cfgNw.SubnetIP = testSubnetIP
		cfgNw.SubnetLen = testSubnetLen
		cfgNw.StateDriver = stateDriver
		if err := cfgNw.Write(); err != nil {
			return err
		}
	}

	{
		cfgNw := &OvsCfgNetworkState{}
		cfgNw.ID = testOvsNwIDStateful
		cfgNw.PktTag = testPktTagStateful
		cfgNw.ExtPktTag = testExtPktTag
		cfgNw.SubnetIP = testSubnetIP
		cfgNw.SubnetLen = testSubnetLen
		cfgNw.StateDriver = stateDriver
		if err := cfgNw.Write(); err != nil {
			return err
		}
	}

	{
		cfgEp := &OvsCfgEndpointState{}
		cfgEp.ID = createEpID
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
		cfgEp := &OvsCfgEndpointState{}
		cfgEp.ID = createEpIDStateful
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
		cfgEp := &OvsCfgEndpointState{}
		cfgEp.ID = createEpIDStatefulMismatch
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
		cfgEp := &OvsCfgEndpointState{}
		cfgEp.ID = deleteEpID
		cfgEp.NetID = testOvsNwID
		cfgEp.IPAddress = testEpAddress
		cfgEp.MacAddress = testEpMacAddress
		cfgEp.StateDriver = stateDriver
		if err := cfgEp.Write(); err != nil {
			return err
		}
	}

	{
		cfgPeer := &PeerHostState{}
		cfgPeer.ID = peerHostID
		cfgPeer.Hostname = peerHostName
		cfgPeer.HostAddr = peerHostIP
		cfgPeer.VtepIPAddr = peerHostIP
		cfgPeer.StateDriver = stateDriver
		if err := cfgPeer.Write(); err != nil {
			return err
		}
	}

	return nil
}

func initOvsDriver(t *testing.T) *OvsDriver {
	driver := &OvsDriver{}
	ovsConfig := &OvsDriverConfig{}
	ovsConfig.Ovs.DbIP = ""
	ovsConfig.Ovs.DbPort = 0
	config := &core.Config{V: ovsConfig}
	stateDriver := &state.FakeStateDriver{}
	stateDriver.Init(nil)
	instInfo := &core.InstanceInfo{HostLabel: testHostLabel,
		StateDriver: stateDriver}

	err := createCommonState(stateDriver)
	if err != nil {
		t.Fatalf("common state creation failed. Error: %s", err)
	}

	err = driver.Init(config, instInfo)
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
	ovsConfig := &OvsDriverConfig{}
	ovsConfig.Ovs.DbIP = ""
	ovsConfig.Ovs.DbPort = 0
	config := &core.Config{V: ovsConfig}
	stateDriver := &state.FakeStateDriver{}
	stateDriver.Init(nil)
	instInfo := &core.InstanceInfo{HostLabel: testHostLabelStateful,
		StateDriver: stateDriver}

	operOvs := &OvsDriverOperState{CurrPortNum: 10}
	operOvs.StateDriver = stateDriver
	operOvs.ID = testHostLabelStateful
	err := operOvs.Write()
	if err != nil {
		t.Fatalf("writing driver oper state failed. Error: %s", err)
	}

	err = driver.Init(config, instInfo)
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
	config := &core.Config{V: nil}
	stateDriver := &state.FakeStateDriver{}
	stateDriver.Init(nil)
	instInfo := &core.InstanceInfo{HostLabel: testHostLabel,
		StateDriver: stateDriver}

	err := driver.Init(config, instInfo)
	if err == nil {
		t.Fatalf("driver init succeeded. Should have failed!")
	}

	err = driver.Init(nil, instInfo)
	if err == nil {
		t.Fatalf("driver init succeeded. Should have failed!")
	}

	defer func() { driver.Deinit() }()
}

func TestOvsDriverInitInvalidState(t *testing.T) {
	driver := &OvsDriver{}
	ovsConfig := &OvsDriverConfig{}
	ovsConfig.Ovs.DbIP = ""
	ovsConfig.Ovs.DbPort = 0
	config := &core.Config{V: ovsConfig}
	instInfo := &core.InstanceInfo{HostLabel: testHostLabel, StateDriver: nil}

	err := driver.Init(config, instInfo)
	if err == nil {
		t.Fatalf("driver init succeeded. Should have failed!")
	}
	defer func() { driver.Deinit() }()
}

func TestOvsDriverInitInvalidInstanceInfo(t *testing.T) {
	driver := &OvsDriver{}
	ovsConfig := &OvsDriverConfig{}
	ovsConfig.Ovs.DbIP = ""
	ovsConfig.Ovs.DbPort = 0
	config := &core.Config{V: ovsConfig}

	err := driver.Init(config, nil)
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

	err := driver.CreateEndpoint(id)
	if err != nil {
		t.Fatalf("endpoint creation failed. Error: %s", err)
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

func TestOvsDriverCreateEndpointStateful(t *testing.T) {
	driver := initOvsDriver(t)
	defer func() { driver.Deinit() }()
	id := createEpIDStateful

	err := driver.CreateEndpoint(id)
	if err != nil {
		t.Fatalf("endpoint creation failed. Error: %s", err)
	}

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

	err := driver.CreateEndpoint(id)
	if err != nil {
		t.Fatalf("endpoint creation failed. Error: %s", err)
	}

	cfgEp := OvsCfgEndpointState{}
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

	err := driver.CreateEndpoint(id)
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

func TestOvsDriverAddDeletePeer(t *testing.T) {
	driver := initOvsDriver(t)
	defer func() { driver.Deinit() }()

	// create peer host
	err := driver.CreatePeerHost(peerHostID)
	if err != nil {
		t.Fatalf("Error creating peer host. Err: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	// verify VTEP exists
	expVtepName := vxlanIfName(peerHostIP)
	output, err := exec.Command("ovs-vsctl", "list", "Interface").CombinedOutput()
	if err != nil || !strings.Contains(string(output), expVtepName) {
		t.Fatalf("interface lookup failed. Error: %s expected port: %s Output: %s",
			err, expVtepName, output)
	}

	// delete peer host
	err = driver.DeletePeerHost(peerHostID)
	if err != nil {
		t.Fatalf("Error deleting peer host. Err: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	// verify VTEP is gone.
	output, err = exec.Command("ovs-vsctl", "list", "Interface").CombinedOutput()
	if err != nil || strings.Contains(string(output), expVtepName) {
		t.Fatalf("interface still exists. Error: %s expected port: %s Output: %s",
			err, expVtepName, output)
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
