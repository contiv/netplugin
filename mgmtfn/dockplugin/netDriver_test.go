package dockplugin

import (
	"fmt"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/docknet"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/state"
	"github.com/contiv/netplugin/utils"
	"testing"
)

var stateDriver *state.FakeStateDriver

// initStateDriver initialize etcd state driver
func initFakeStateDriver(t *testing.T) {
	instInfo := core.InstanceInfo{}
	d, err := utils.NewStateDriver("fakedriver", &instInfo)
	if err != nil {
		t.Fatalf("failed to init statedriver. Error: %s", err)
		t.Fail()
	}
	stateDriver = d.(*state.FakeStateDriver)
}

func deinitStateDriver() {
	utils.ReleaseStateDriver()
}

// Ensure deleteNetworkHelper can delete docker network without issue
func TestCreateAndDeleteNetwork(t *testing.T) {
	// Update plugin driver for unit test
	docknet.UpdateDockerV2PluginName("bridge", "default")

	initFakeStateDriver(t)
	defer deinitStateDriver()

	tenantName := "t1"
	networkName := "net1"
	serviceName := ""
	nwcfg := mastercfg.CfgNetworkState{
		Tenant:      tenantName,
		NetworkName: networkName,
		PktTagType:  "vlan",
		PktTag:      1,
		ExtPktTag:   1,
		SubnetIP:    "10.0.0.0",
		SubnetLen:   24,
		Gateway:     "10.0.0.1",
	}

	err := docknet.CreateDockNet(tenantName, networkName, "", &nwcfg)
	if err != nil {
		t.Fatalf("Error creating docker network. Error: %v", err)
		t.Fail()
	}

	// Get Docker network UUID
	dnetOper := docknet.DnetOperState{}
	dnetOper.StateDriver = stateDriver

	err = dnetOper.Read(fmt.Sprintf("%s.%s.%s", tenantName, networkName, serviceName))
	if err != nil {
		t.Fatalf("Unable to read network state. Error: %v", err)
		t.Fail()
	}

	testData := make([]byte, 3)
	networkID := mastercfg.StateConfigPath + "nets/" + dnetOper.DocknetUUID + ".default"
	dnetOper.StateDriver.Write(networkID, testData)

	// Delete Docker network by using helper
	err = deleteNetworkHelper(dnetOper.DocknetUUID)

	if err != nil {
		t.Fatalf("Unable to delete docker network. Error: %v", err)
		t.Fail()
	}

	// ensure network is not in the oper state
	err = dnetOper.Read(fmt.Sprintf("%s.%s.%s", tenantName, networkName, serviceName))
	if err == nil {
		t.Fatalf("Fail to delete docket network")
		t.Fail()
	}
}
