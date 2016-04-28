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

package docknet

import (
	"fmt"
	"os"
	"testing"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/netplugin/utils/netutils"

	log "github.com/Sirupsen/logrus"
)

// initStateDriver initialize etcd state driver
func initStateDriver() (core.StateDriver, error) {
	instInfo := core.InstanceInfo{DbURL: "etcd://127.0.0.1:2379"}

	return utils.NewStateDriver(utils.EtcdNameStr, &instInfo)
}

// getDocknetState gets docknet oper state
func getDocknetState(tenantName, networkName, serviceName string) *OperState {
	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		log.Warnf("Couldn't read global config %v", err)
		return nil
	}

	// save docknet oper state
	dnetOper := OperState{}
	dnetOper.StateDriver = stateDriver

	// write the dnet oper state
	err = dnetOper.Read(fmt.Sprintf("%s.%s.%s", tenantName, networkName, serviceName))
	if err != nil {
		return nil
	}

	return &dnetOper
}

func checkDocknetCreate(t *testing.T, tenantName, networkName, serviceName, subnet, gw string) {
	docknetName := GetDocknetName(tenantName, networkName, serviceName)
	subnetIP, subnetLen, _ := netutils.ParseCIDR(subnet)

	nwcfg := mastercfg.CfgNetworkState{
		Tenant:      tenantName,
		NetworkName: networkName,
		PktTagType:  "vlan",
		PktTag:      1,
		ExtPktTag:   1,
		SubnetIP:    subnetIP,
		SubnetLen:   subnetLen,
		Gateway:     gw,
	}

	// create a docker network
	err := CreateDockNet(tenantName, networkName, serviceName, &nwcfg)
	if err != nil {
		t.Fatalf("Error creating docker ntework. Err: %v", err)
	}

	// verify docknet state is created
	dnetOper := getDocknetState(tenantName, networkName, serviceName)
	if dnetOper == nil {
		t.Fatalf("Error finding docknet state for %s", docknetName)
	}

	// check if docker has the state
	docker, err := utils.GetDockerClient()
	if err != nil {
		t.Fatalf("Unable to connect to docker. Error %v", err)
	}
	ninfo, err := docker.InspectNetwork(docknetName)
	if err != nil {
		t.Fatalf("Error getting network info for %s. Err: %v", docknetName, err)
	}

	// verify params are correct
	if ninfo.Scope != "local" || ninfo.Driver != netDriverName || ninfo.IPAM.Driver != ipamDriverName ||
		ninfo.IPAM.Config[0].Subnet != subnet || ninfo.IPAM.Config[0].Gateway != gw {
		t.Fatalf("Docker network {%+v} does not match expected values", ninfo)
	}

	// make sure FindDocknetByUUID returns correct UUID
	tmpOper, err := FindDocknetByUUID(dnetOper.DocknetUUID)
	if err != nil {
		t.Fatalf("Error getting docknet by UUID")
	}

	if tmpOper.TenantName != tenantName || tmpOper.NetworkName != networkName ||
		tmpOper.ServiceName != serviceName {
		t.Fatalf("Got unexpected docknet oper state %+v for network UUID %s", tmpOper, dnetOper.DocknetUUID)
	}
}

func checkDocknetDelete(t *testing.T, tenantName, networkName, serviceName string) {
	docknetName := GetDocknetName(tenantName, networkName, serviceName)

	// delete the docknet
	err := DeleteDockNet(tenantName, networkName, serviceName)
	if err != nil {
		t.Fatalf("Error deleting docker network %s. Err: %v", docknetName, err)
	}

	// verify docknet state is deleted
	dnetOper := getDocknetState(tenantName, networkName, serviceName)
	if dnetOper != nil {
		t.Fatalf("docknet state for %s was not deleted", docknetName)
	}

	// make sure docker has removed the state
	docker, err := utils.GetDockerClient()
	if err != nil {
		t.Fatalf("Unable to connect to docker. Error %v", err)
	}
	_, err = docker.InspectNetwork(docknetName)
	if err == nil {
		t.Fatalf("docker net %s was not deleted. Err: %v", docknetName, err)
	}
}

func TestMain(m *testing.M) {
	// change driver names for unit-testing
	netDriverName = "bridge"
	ipamDriverName = "default"

	initStateDriver()

	os.Exit(m.Run())
}

func TestDocknetCreateDelete(t *testing.T) {
	// test network creation
	checkDocknetCreate(t, "unit-test", "net1", "", "10.1.1.1/24", "10.1.1.254")
	checkDocknetDelete(t, "unit-test", "net1", "")

	// test service names
	checkDocknetCreate(t, "unit-test", "net1", "srv1", "10.1.1.1/24", "10.1.1.254")
	checkDocknetDelete(t, "unit-test", "net1", "srv1")
}
