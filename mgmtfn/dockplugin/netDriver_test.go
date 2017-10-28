package dockplugin

import (
	"fmt"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/docknet"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/netplugin/state"
	"testing"
)

var stateDriver *state.FakeStateDriver

// initStateDriver initialize etcd state driver
func initFakeStateDriver() {
	instInfo := core.InstanceInfo{}
	d, _ := utils.NewStateDriver("fakedriver", &instInfo)

	stateDriver = d.(*state.FakeStateDriver)
}


func deinitStateDriver() {
	utils.ReleaseStateDriver()
}

// Ensure deleteNetworkHelper can delete docker network without issue
func TestCreateAndDeleteNetwork(t *testing.T) {
	// Update plugin driver for unit test
	docknet.UpdateDockerV2PluginName("bridge", "default")

	initFakeStateDriver()
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

	dnetOperErr := dnetOper.Read(fmt.Sprintf("%s.%s.%s", tenantName, networkName, serviceName))
	if dnetOperErr != nil {
		t.Fatalf("Unable to read network state. Error: %v", dnetOperErr)
		t.Fail()
	}
	testData := make([]byte, 3)
	networkID := dnetOper.DocknetUUID + ".default"
	stateDriver.Write(networkID, testData)

	// Delete Docker network by using helper
	delErr := deleteNetworkHelper(dnetOper.DocknetUUID)

	if delErr != nil {
		t.Fatalf("Unable to delete docker network. Error: %v", delErr)
		t.Fail()
	}
}

func TestDeleteNonExistingNetwork(t *testing.T) {
	// Update plugin driver for unit test
	docknet.UpdateDockerV2PluginName("bridge", "default")

	initFakeStateDriver()

	// Delete Docker network by using helper
	delErr := deleteNetworkHelper("non_existing_network_id")

	if delErr == nil {
		t.Fatalf("Expect deleteNetworkHelper returns error when network is not presented")
		t.Fail()
	}
}
