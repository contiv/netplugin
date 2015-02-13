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

	"github.com/contiv/netplugin/core"
)

const (
	createEpId         = "testCreateEp"
	deleteEpId         = "testDeleteEp"
	createEpWithIntfId = "testCreateEpWithIntf"
	deleteEpWithIntfId = "testDeleteEpWithIntf"
	createVxlanEpId    = "testCreateVxlanEp"
	deleteVxlanEpId    = "testDeleteVxlanEp"
	vxlanPeerIp        = "12.1.1.1"
	testOvsNwId        = "testNetId"
	testPktTag         = 100
	testExtPktTag      = 10000
	testIntfName       = "testIntf"

	READ_EP int = iota
	READ_EP_WITH_INTF
	READ_VXLAN_EP
	READ_NW
)

var ovsStateDriver = &testOvsStateDriver{}

type testOvsStateDriver struct {
}

func (d *testOvsStateDriver) Init(config *core.Config) error {
	return &core.Error{Desc: "Shouldn't be called!"}
}

func (d *testOvsStateDriver) Deinit() {
}

func (d *testOvsStateDriver) Write(key string, value []byte) error {
	return &core.Error{Desc: "Shouldn't be called!"}
}

func (d *testOvsStateDriver) Read(key string) ([]byte, error) {
	return []byte{}, &core.Error{Desc: "Shouldn't be called!"}
}

func (d *testOvsStateDriver) ReadRecursive(baseKey string) ([]string, error) {
	return []string{}, &core.Error{Desc: "Shouldn't be called!"}
}

func (d *testOvsStateDriver) ClearState(key string) error {
	return nil
}

func (d *testOvsStateDriver) readStateHelper(isCreateEp bool, oper int,
	value core.State) error {
	if cfgNw, ok := value.(*OvsCfgNetworkState); ok {
		cfgNw.Id = testOvsNwId
		return nil
	}

	if operNw, ok := value.(*OvsOperNetworkState); ok {
		operNw.Id = testOvsNwId
		operNw.PktTag = testPktTag
		operNw.ExtPktTag = testExtPktTag
		return nil
	}

	if cfgEp, ok := value.(*OvsCfgEndpointState); ok {
		if isCreateEp {
			if oper == READ_VXLAN_EP {
				cfgEp.Id = createVxlanEpId
				cfgEp.IntfName = testIntfName
				cfgEp.VtepIp = vxlanPeerIp
				cfgEp.NetId = testOvsNwId
			} else if oper == READ_EP_WITH_INTF {
				cfgEp.Id = createEpWithIntfId
				cfgEp.IntfName = testIntfName
			} else {
				cfgEp.Id = createEpId
				cfgEp.IntfName = ""
			}
		} else {
			if oper == READ_VXLAN_EP {
				cfgEp.Id = deleteVxlanEpId
				cfgEp.VtepIp = vxlanPeerIp
				cfgEp.NetId = testOvsNwId
			} else if oper == READ_EP_WITH_INTF {
				cfgEp.Id = deleteEpWithIntfId
			} else {
				cfgEp.Id = deleteEpId
			}
		}
		cfgEp.NetId = testOvsNwId
		return nil
	}

	if operEp, ok := value.(*OvsOperEndpointState); ok {
		if isCreateEp {
			if oper == READ_VXLAN_EP {
				operEp.Id = createVxlanEpId
			} else if oper == READ_EP_WITH_INTF {
				operEp.Id = createEpWithIntfId
			} else {
				operEp.Id = createEpId
			}
		} else {
			if oper == READ_VXLAN_EP {
				operEp.Id = deleteVxlanEpId
				operEp.NetId = testOvsNwId
				operEp.VtepIp = vxlanPeerIp
			} else if oper == READ_EP_WITH_INTF {
				operEp.Id = deleteEpWithIntfId
			} else {
				operEp.Id = deleteEpId
			}
		}
		operEp.NetId = testOvsNwId
		return nil
	}

	return &core.Error{Desc: "unknown value type"}
}

func (d *testOvsStateDriver) ReadState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) error {
	if strings.Contains(key, createVxlanEpId) {
		return d.readStateHelper(true, READ_VXLAN_EP, value)
	}
	if strings.Contains(key, createEpWithIntfId) {
		return d.readStateHelper(true, READ_EP_WITH_INTF, value)
	}
	if strings.Contains(key, createEpId) {
		return d.readStateHelper(true, READ_EP, value)
	}

	if strings.Contains(key, deleteVxlanEpId) {
		return d.readStateHelper(false, READ_VXLAN_EP, value)
	}
	if strings.Contains(key, deleteEpWithIntfId) {
		return d.readStateHelper(false, READ_EP_WITH_INTF, value)
	}
	if strings.Contains(key, deleteEpId) {
		return d.readStateHelper(false, READ_EP, value)
	}
	if strings.Contains(key, testOvsNwId) {
		return d.readStateHelper(false, READ_NW, value)
	}

	return &core.Error{Desc: fmt.Sprintf("unknown key! %s", key)}
}

func (d *testOvsStateDriver) WriteState(key string, value core.State,
	marshal func(interface{}) ([]byte, error)) error {
	return nil
}

func initOvsDriver(t *testing.T) *OvsDriver {
	driver := &OvsDriver{}
	ovsConfig := &OvsDriverConfig{}
	ovsConfig.Ovs.DbIp = ""
	ovsConfig.Ovs.DbPort = 0
	config := &core.Config{V: ovsConfig}

	err := driver.Init(config, ovsStateDriver)
	if err != nil {
		t.Fatalf("driver init failed. Error: %s", err)
		return nil
	}

	return driver
}

func TestOvsDriverInit(t *testing.T) {
	driver := initOvsDriver(t)
	defer func() { driver.Deinit() }()
}

func TestOvsDriverInitInvalidConfig(t *testing.T) {
	driver := &OvsDriver{}
	config := &core.Config{V: nil}

	err := driver.Init(config, ovsStateDriver)
	if err == nil {
		t.Fatalf("driver init succeeded. Should have failed!")
	}

	err = driver.Init(nil, ovsStateDriver)
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

	err := driver.Init(config, nil)
	if err == nil {
		t.Fatalf("driver init succeeded. Should have failed!")
	}
}

func TestOvsDriverDeinit(t *testing.T) {
	driver := initOvsDriver(t)

	driver.Deinit()

	output, err := exec.Command("ovs-vsctl", "list", "Bridge").Output()
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

	output, err := exec.Command("ovs-vsctl", "list", "Port").Output()
	expectedPortName := fmt.Sprintf("port%d", driver.currPortNum)
	if err != nil || !strings.Contains(string(output), expectedPortName) {
		t.Fatalf("port lookup failed. Error: %s expected port: %s Output: %s",
			err, expectedPortName, output)
	}

	output, err = exec.Command("ovs-vsctl", "list", "Interface").Output()
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

	output, err := exec.Command("ovs-vsctl", "list", "Port").Output()
	expectedPortName := fmt.Sprintf("port%d", driver.currPortNum)
	if err != nil || !strings.Contains(string(output), expectedPortName) {
		t.Fatalf("port lookup failed. Error: %s expected port: %s Output: %s",
			err, expectedPortName, output)
	}

	output, err = exec.Command("ovs-vsctl", "list", "Interface").Output()
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

	cfgEp := &OvsCfgEndpointState{}
	err = driver.stateDriver.ReadState(id, cfgEp, nil)
	if err != nil {
		t.Fatalf("error '%s' reading state for id %s \n", err, id)
	}
	value, err := cfgEp.Marshal()
	if err != nil {
		t.Fatalf("error marshaling config '%s' \n", err)
	}
	err = driver.DeleteEndpoint(value)
	if err != nil {
		t.Fatalf("endpoint Deletion failed. Error: %s", err)
	}

	output, err := exec.Command("ovs-vsctl", "list", "Port").Output()
	expectedPortName := fmt.Sprintf(PORT_NAME_FMT, driver.currPortNum+1)
	if err != nil || strings.Contains(string(output), expectedPortName) {
		t.Fatalf("port lookup succeeded after delete. Error: %s Output: %s", err, output)
	}

	output, err = exec.Command("ovs-vsctl", "list", "Interface").Output()
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

	cfgEp := &OvsCfgEndpointState{}
	err = driver.stateDriver.ReadState(id, cfgEp, nil)
	if err != nil {
		t.Fatalf("error '%s' reading state for id %s \n", err, id)
	}
	value, err := cfgEp.Marshal()
	if err != nil {
		t.Fatalf("error marshaling config '%s' \n", err)
	}
	err = driver.DeleteEndpoint(value)
	if err != nil {
		t.Fatalf("endpoint Deletion failed. Error: %s", err)
	}

	output, err := exec.Command("ovs-vsctl", "list", "Port").Output()
	expectedPortName := fmt.Sprintf(PORT_NAME_FMT, driver.currPortNum+1)
	if err != nil || strings.Contains(string(output), expectedPortName) {
		t.Fatalf("port lookup succeeded after delete. Error: %s Output: %s", err, output)
	}

	output, err = exec.Command("ovs-vsctl", "list", "Interface").Output()
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
	output, err := exec.Command("ovs-vsctl", "list", "Port").Output()
	if err != nil || !strings.Contains(string(output), expectedPortName) {
		t.Fatalf("port lookup failed. Error: %s expected port: %s Output: %s",
			err, expectedPortName, output)
	}

	output, err = exec.Command("ovs-vsctl", "list", "Interface").Output()
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

	cfgEp := &OvsCfgEndpointState{}
	err = driver.stateDriver.ReadState(deleteVxlanEpId, cfgEp, nil)
	if err != nil {
		t.Fatalf("error '%s' reading state for id %s \n", err,
			deleteVxlanEpId)
	}
	value, err := cfgEp.Marshal()
	if err != nil {
		t.Fatalf("error marshaling config '%s' \n", err)
	}
	err = driver.DeleteEndpoint(value)
	if err != nil {
		t.Fatalf("endpoint Deletion failed. Error: %s", err)
	}

	expectedPortName := vxlanIfName(testOvsNwId, vxlanPeerIp)
	output, err := exec.Command("ovs-vsctl", "list", "Port").Output()
	if err != nil || strings.Contains(string(output), expectedPortName) {
		t.Fatalf("port lookup succeeded after delete. Error: %s Output: %s",
			err, output)
	}

	output, err = exec.Command("ovs-vsctl", "list", "Interface").Output()
	if err != nil || strings.Contains(string(output), expectedPortName) {
		t.Fatalf("interface lookup succeeded after delete. Error: %s Output: %s", err, output)
	}
}
