package dockplugin

import (
	"fmt"
    "testing"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/docknet"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/utils"
)

// initStateDriver initialize etcd state driver
func initStateDriver() (core.StateDriver, error) {
	instInfo := core.InstanceInfo{DbURL: "etcd://127.0.0.1:2379"}

	return utils.NewStateDriver(utils.EtcdNameStr, &instInfo)
}

// Ensure deleteNetworkHelper can delete docker network without issue
func TestCreateAndDeleteNetwork(t *testing.T) {
	// Update plugin driver for unit test
	docknet.UpdatePluginName("bridge", "default")
    initStateDriver()

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
	stateDriver, err := utils.GetStateDriver()
	dnetOper := docknet.DnetOperState{}
	dnetOper.StateDriver = stateDriver

	dnetOperErr := dnetOper.Read(
		fmt.Sprintf("%s.%s.%s", tenantName, networkName, serviceName))
	if dnetOperErr != nil {
		t.Fatalf("Unable to read network state. Error: %v", dnetOperErr)
		t.Fail()
	}

	// Delete Docker network by using helper
	delErr := deleteNetworkHelper(dnetOper.DocknetUUID)

	if delErr != nil {
		t.Fatalf("Unable to delete docker network. Error: %v", delErr)
		t.Fail()
	}
}
