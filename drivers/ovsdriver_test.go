package drivers

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/contiv/netplugin/core"
)

const (
	createEpId  = "testCreateEp"
	deleteEpId  = "testDeleteEp"
	testOvsNwId = "testNetId"
	testVlanTag = 100
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

func (d *testOvsStateDriver) ClearState(key string) error {
	return nil
}

func (d *testOvsStateDriver) readStateHelper(isCreateEp bool, value core.State) error {
	if cfgNw, ok := value.(*OvsCfgNetworkState); ok {
		cfgNw.Id = testOvsNwId
		return nil
	}

	if operNw, ok := value.(*OvsOperNetworkState); ok {
		operNw.Id = testOvsNwId
		return nil
	}

	if cfgEp, ok := value.(*OvsCfgEndpointState); ok {
		cfgEp.Id = createEpId
		if !isCreateEp {
			cfgEp.Id = deleteEpId
		}
		cfgEp.NetId = testOvsNwId
		cfgEp.VlanTag = testVlanTag
		return nil
	}

	if operEp, ok := value.(*OvsOperEndpointState); ok {
		operEp.Id = createEpId
		operEp.PortName = ""
		if !isCreateEp {
			operEp.Id = deleteEpId
			//XXX: assuming only one port in db
			operEp.PortName = fmt.Sprintf(PORT_NAME_FMT, 1)
		}
		operEp.NetId = testOvsNwId
		return nil
	}

	return &core.Error{Desc: "unknown value type"}
}

func (d *testOvsStateDriver) ReadState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) error {
	if strings.Contains(key, createEpId) {
		return d.readStateHelper(true, value)
	}
	if strings.Contains(key, deleteEpId) {
		return d.readStateHelper(false, value)
	}
	if strings.Contains(key, testOvsNwId) {
		return d.readStateHelper(false, value)
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

func TestOvsDriverDeleteEndpoint(t *testing.T) {
	driver := initOvsDriver(t)
	defer func() { driver.Deinit() }()
	id := deleteEpId

	err := driver.CreateEndpoint(id)
	if err != nil {
		t.Fatalf("endpoint Creation failed. Error: %s", err)
	}

	err = driver.DeleteEndpoint(id)
	if err != nil {
		t.Fatalf("endpoint Deletion failed. Error: %s", err)
	}

	output, err := exec.Command("ovs-vsctl", "list", "Port").Output()
	expectedPortName := fmt.Sprintf(PORT_NAME_FMT, driver.currPortNum+1)
	if err != nil || strings.Contains(string(output), expectedPortName) {
		t.Fatalf("port lookup succeeded after delete. Error: %s Output: %s", err, output)
	}

	output, err = exec.Command("ovs-vsctl", "list", "Interface").Output()
	if err != nil || strings.Contains(string(output), expectedPortName) {
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
