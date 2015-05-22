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
	createEpId                      = "testCreateEp"
	createEpIdStateful              = "testCreateEpStateful"
	createEpIdStatefulMismatch      = "testCreateEpStatefulMismatch"
	deleteEpId                      = "testDeleteEp"
	createEpWithIntfId              = "testCreateEpWithIntf"
	createEpWithIntfIdStateful      = "testCreateEpWithIntfStateful"
	deleteEpWithIntfId              = "testDeleteEpWithIntf"
	createVxlanEpId                 = "testCreateVxlanEp"
	createVxlanEpIdStateful         = "testCreateVxlanEpStateful"
	createVxlanEpIdStatefulMismatch = "testCreateVxlanEpStatefulMismatch"
	deleteVxlanEpId                 = "testDeleteVxlanEp"
	vxlanPeerIp                     = "12.1.1.1"
	testOvsNwId                     = "testNetId"
	testOvsNwIdStateful             = "testNetIdStateful"
	testPktTag                      = 100
	testPktTagStateful              = 200
	testExtPktTag                   = 10000
	testIntfName                    = "testIntf"
	testSubnetIp                    = "10.1.1.0"
	testSubnetLen                   = 24
	testEpAddress                   = "10.1.1.1"
	testHostLabel                   = "testHost"
	testHostLabelStateful           = "testHostStateful"
	testCurrPortNum                 = 10
)

func createCommonState(stateDriver core.StateDriver) error {
	//create all the common config state required by the tests

	{
		cfgNw := &OvsCfgNetworkState{}
		cfgNw.Id = testOvsNwId
		cfgNw.PktTag = testPktTag
		cfgNw.ExtPktTag = testExtPktTag
		cfgNw.SubnetIp = testSubnetIp
		cfgNw.SubnetLen = testSubnetLen
		cfgNw.StateDriver = stateDriver
		if err := cfgNw.Write(); err != nil {
			return err
		}
	}

	{
		cfgNw := &OvsCfgNetworkState{}
		cfgNw.Id = testOvsNwIdStateful
		cfgNw.PktTag = testPktTagStateful
		cfgNw.ExtPktTag = testExtPktTag
		cfgNw.SubnetIp = testSubnetIp
		cfgNw.SubnetLen = testSubnetLen
		cfgNw.StateDriver = stateDriver
		if err := cfgNw.Write(); err != nil {
			return err
		}
	}

	{
		cfgEp := &OvsCfgEndpointState{}
		cfgEp.Id = createVxlanEpId
		cfgEp.NetId = testOvsNwId
		cfgEp.VtepIp = vxlanPeerIp
		cfgEp.NetId = testOvsNwId
		cfgEp.IpAddress = testEpAddress
		cfgEp.StateDriver = stateDriver
		if err := cfgEp.Write(); err != nil {
			return err
		}
	}

	{
		cfgEp := &OvsCfgEndpointState{}
		cfgEp.Id = createVxlanEpIdStateful
		cfgEp.NetId = testOvsNwId
		cfgEp.VtepIp = vxlanPeerIp
		cfgEp.NetId = testOvsNwId
		cfgEp.IpAddress = testEpAddress
		cfgEp.StateDriver = stateDriver
		if err := cfgEp.Write(); err != nil {
			return err
		}
	}

	{
		cfgEp := &OvsCfgEndpointState{}
		cfgEp.Id = createVxlanEpIdStatefulMismatch
		cfgEp.NetId = testOvsNwId
		cfgEp.VtepIp = vxlanPeerIp
		cfgEp.NetId = testOvsNwId
		cfgEp.IpAddress = testEpAddress
		cfgEp.StateDriver = stateDriver
		if err := cfgEp.Write(); err != nil {
			return err
		}
	}
	{
		cfgEp := &OvsCfgEndpointState{}
		cfgEp.Id = createEpWithIntfId
		cfgEp.NetId = testOvsNwId
		cfgEp.IntfName = testIntfName
		cfgEp.IpAddress = testEpAddress
		cfgEp.StateDriver = stateDriver
		if err := cfgEp.Write(); err != nil {
			return err
		}
	}

	{
		cfgEp := &OvsCfgEndpointState{}
		cfgEp.Id = createEpWithIntfIdStateful
		cfgEp.NetId = testOvsNwId
		cfgEp.IntfName = testIntfName
		cfgEp.IpAddress = testEpAddress
		cfgEp.StateDriver = stateDriver
		if err := cfgEp.Write(); err != nil {
			return err
		}
	}

	{
		cfgEp := &OvsCfgEndpointState{}
		cfgEp.Id = createEpId
		cfgEp.NetId = testOvsNwId
		cfgEp.IntfName = ""
		cfgEp.IpAddress = testEpAddress
		cfgEp.StateDriver = stateDriver
		if err := cfgEp.Write(); err != nil {
			return err
		}
	}

	{
		cfgEp := &OvsCfgEndpointState{}
		cfgEp.Id = createEpIdStateful
		cfgEp.NetId = testOvsNwId
		cfgEp.IntfName = ""
		cfgEp.IpAddress = testEpAddress
		cfgEp.StateDriver = stateDriver
		if err := cfgEp.Write(); err != nil {
			return err
		}
	}

	{
		cfgEp := &OvsCfgEndpointState{}
		cfgEp.Id = createEpIdStatefulMismatch
		cfgEp.NetId = testOvsNwId
		cfgEp.IntfName = ""
		cfgEp.IpAddress = testEpAddress
		cfgEp.StateDriver = stateDriver
		if err := cfgEp.Write(); err != nil {
			return err
		}
	}

	{
		cfgEp := &OvsCfgEndpointState{}
		cfgEp.Id = deleteVxlanEpId
		cfgEp.NetId = testOvsNwId
		cfgEp.VtepIp = vxlanPeerIp
		cfgEp.NetId = testOvsNwId
		cfgEp.IpAddress = testEpAddress
		cfgEp.StateDriver = stateDriver
		if err := cfgEp.Write(); err != nil {
			return err
		}
	}

	{
		cfgEp := &OvsCfgEndpointState{}
		cfgEp.Id = deleteEpWithIntfId
		cfgEp.NetId = testOvsNwId
		cfgEp.IntfName = testIntfName
		cfgEp.IpAddress = testEpAddress
		cfgEp.StateDriver = stateDriver
		if err := cfgEp.Write(); err != nil {
			return err
		}
	}

	{
		cfgEp := &OvsCfgEndpointState{}
		cfgEp.Id = deleteEpId
		cfgEp.NetId = testOvsNwId
		cfgEp.IpAddress = testEpAddress
		cfgEp.StateDriver = stateDriver
		if err := cfgEp.Write(); err != nil {
			return err
		}
	}

	return nil
}

func initOvsDriver(t *testing.T) *OvsDriver {
	driver := &OvsDriver{}
	ovsConfig := &OvsDriverConfig{}
	ovsConfig.Ovs.DbIp = ""
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
	ovsConfig.Ovs.DbIp = ""
	ovsConfig.Ovs.DbPort = 0
	config := &core.Config{V: ovsConfig}
	stateDriver := &state.FakeStateDriver{}
	stateDriver.Init(nil)
	instInfo := &core.InstanceInfo{HostLabel: testHostLabelStateful,
		StateDriver: stateDriver}

	operOvs := &OvsDriverOperState{CurrPortNum: 10}
	operOvs.StateDriver = stateDriver
	operOvs.Id = testHostLabelStateful
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
}

func TestOvsDriverInitInvalidState(t *testing.T) {
	driver := &OvsDriver{}
	ovsConfig := &OvsDriverConfig{}
	ovsConfig.Ovs.DbIp = ""
	ovsConfig.Ovs.DbPort = 0
	config := &core.Config{V: ovsConfig}
	instInfo := &core.InstanceInfo{HostLabel: testHostLabel, StateDriver: nil}

	err := driver.Init(config, instInfo)
	if err == nil {
		t.Fatalf("driver init succeeded. Should have failed!")
	}
}

func TestOvsDriverInitInvalidInstanceInfo(t *testing.T) {
	driver := &OvsDriver{}
	ovsConfig := &OvsDriverConfig{}
	ovsConfig.Ovs.DbIp = ""
	ovsConfig.Ovs.DbPort = 0
	config := &core.Config{V: ovsConfig}

	err := driver.Init(config, nil)
	if err == nil {
		t.Fatalf("driver init succeeded. Should have failed!")
	}
}

func TestOvsDriverDeinit(t *testing.T) {
	driver := initOvsDriver(t)

	driver.Deinit()

	output, err := exec.Command("ovs-vsctl", "list", "Bridge").CombinedOutput()
	if err != nil || strings.Contains(string(output), DEFAULT_BRIDGE_NAME) {
		t.Fatalf("deinit failed. Error: %s Output: %s", err, output)
	}

}

func TestOvsDriverCreateEndpoint(t *testing.T) {
	driver := initOvsDriver(t)
	defer func() { driver.Deinit() }()
	id := createEpId

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
	id := createEpIdStateful

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
	id := createEpIdStatefulMismatch

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
	cfgEp.NetId = testOvsNwIdStateful

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

func TestOvsDriverCreateEndpointWithIntfName(t *testing.T) {
	driver := initOvsDriver(t)
	defer func() { driver.Deinit() }()
	id := createEpWithIntfId

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
	if err != nil || !strings.Contains(string(output), testIntfName) {
		t.Fatalf("interface lookup failed. Error: %s expected port: %s Output: %s",
			err, expectedPortName, output)
	}
}

func TestOvsDriverCreateEndpointWithIntfNameStateful(t *testing.T) {
	driver := initOvsDriver(t)
	defer func() { driver.Deinit() }()
	id := createEpWithIntfIdStateful

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
	if err != nil || !strings.Contains(string(output), testIntfName) {
		t.Fatalf("interface lookup failed. Error: %s expected port: %s Output: %s",
			err, expectedPortName, output)
	}
}

func TestOvsDriverDeleteEndpoint(t *testing.T) {
	driver := initOvsDriver(t)
	defer func() { driver.Deinit() }()
	id := deleteEpId

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
	expectedPortName := fmt.Sprintf(PORT_NAME_FMT, driver.oper.CurrPortNum+1)
	if err != nil || strings.Contains(string(output), expectedPortName) {
		t.Fatalf("port lookup succeeded after delete. Error: %s Output: %s", err, output)
	}

	output, err = exec.Command("ovs-vsctl", "list", "Interface").CombinedOutput()
	if err != nil || strings.Contains(string(output), testIntfName) {
		t.Fatalf("interface lookup succeeded after delete. Error: %s Output: %s", err, output)
	}
}

func TestOvsDriverDeleteEndpointiWithIntfName(t *testing.T) {
	driver := initOvsDriver(t)
	defer func() { driver.Deinit() }()
	id := deleteEpWithIntfId

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
	expectedPortName := fmt.Sprintf(PORT_NAME_FMT, driver.oper.CurrPortNum+1)
	if err != nil || strings.Contains(string(output), expectedPortName) {
		t.Fatalf("port lookup succeeded after delete. Error: %s Output: %s", err, output)
	}

	output, err = exec.Command("ovs-vsctl", "list", "Interface").CombinedOutput()
	if err != nil || strings.Contains(string(output), testIntfName) {
		t.Fatalf("interface lookup succeeded after delete. Error: %s Output: %s", err, output)
	}
}

func TestOvsDriverMakeEndpointAddress(t *testing.T) {
	driver := initOvsDriver(t)
	defer func() { driver.Deinit() }()

	_, err := driver.MakeEndpointAddress()
	if err == nil {
		t.Fatalf("make endpoint address succeeded. Should have failed!!")
	}
}

func TestOvsDriverCreateVxlanPeer(t *testing.T) {
	driver := initOvsDriver(t)
	defer func() { driver.Deinit() }()

	err := driver.CreateEndpoint(createVxlanEpId)
	if err != nil {
		t.Fatalf("vxlan peer creation failed. Error: %s", err)
	}

	expectedPortName := vxlanIfName(testOvsNwId, vxlanPeerIp)
	output, err := exec.Command("ovs-vsctl", "list", "Port").CombinedOutput()
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

func TestOvsDriverCreateVxlanPeerStateful(t *testing.T) {
	driver := initOvsDriver(t)
	defer func() { driver.Deinit() }()

	err := driver.CreateEndpoint(createVxlanEpIdStateful)
	if err != nil {
		t.Fatalf("vxlan peer creation failed. Error: %s", err)
	}

	err = driver.CreateEndpoint(createVxlanEpIdStateful)
	if err != nil {
		t.Fatalf("stateful vxlan peer creation failed. Error: %s", err)
	}

	expectedPortName := vxlanIfName(testOvsNwId, vxlanPeerIp)
	output, err := exec.Command("ovs-vsctl", "list", "Port").CombinedOutput()
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

func TestOvsDriverCreateVxlanPeerStatefulMismatch(t *testing.T) {
	driver := initOvsDriver(t)
	defer func() { driver.Deinit() }()
	id := createVxlanEpIdStatefulMismatch

	err := driver.CreateEndpoint(id)
	if err != nil {
		t.Fatalf("vxlan peer creation failed. Error: %s", err)
	}

	cfgEp := OvsCfgEndpointState{}
	cfgEp.StateDriver = driver.oper.StateDriver
	err = cfgEp.Read(id)
	if err != nil {
		t.Fatalf("failed to read ep config. Error: %s", err)
	}
	cfgEp.NetId = testOvsNwIdStateful

	err = cfgEp.Write()
	if err != nil {
		t.Fatalf("failed to write ep config. Error: %s", err)
	}

	err = driver.CreateEndpoint(id)
	if err != nil {
		t.Fatalf("stateful vxlan peer creation failed. Error: %s", err)
	}

	expectedPortName := vxlanIfName(testOvsNwIdStateful, vxlanPeerIp)
	output, err := exec.Command("ovs-vsctl", "list", "Port").CombinedOutput()
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

func TestOvsDriverDeleteVxlanPeer(t *testing.T) {
	driver := initOvsDriver(t)
	defer func() { driver.Deinit() }()

	err := driver.CreateEndpoint(deleteVxlanEpId)
	if err != nil {
		t.Fatalf("endpoint Creation failed. Error: %s", err)
	}

	// XXX: DeleteEndpoint() depends on the ovsdb cache to have been updated
	// once the port is created. The cache update happens asynchronously through
	// a libovsdb callback. So there is a timing window where cache might not yet
	// have been updated. Adding a delay to workaround.
	// Also see contiv/netplugin/issues/78
	time.Sleep(1 * time.Second)

	err = driver.DeleteEndpoint(deleteVxlanEpId)
	if err != nil {
		t.Fatalf("endpoint Deletion failed. Error: %s", err)
	}

	expectedPortName := vxlanIfName(testOvsNwId, vxlanPeerIp)
	output, err := exec.Command("ovs-vsctl", "list", "Port").CombinedOutput()
	if err != nil || strings.Contains(string(output), expectedPortName) {
		t.Fatalf("port lookup succeeded after delete. Error: %s Output: %s",
			err, output)
	}

	output, err = exec.Command("ovs-vsctl", "list", "Interface").CombinedOutput()
	if err != nil || strings.Contains(string(output), expectedPortName) {
		t.Fatalf("interface lookup succeeded after delete. Error: %s Output: %s", err, output)
	}
}
