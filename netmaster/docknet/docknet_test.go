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
	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
)

// initStateDriver initialize etcd state driver
func initStateDriver() (core.StateDriver, error) {
	instInfo := core.InstanceInfo{DbURL: "etcd://127.0.0.1:2379"}

	return utils.NewStateDriver(utils.EtcdNameStr, &instInfo)
}

// getDocknetState gets docknet oper state
func getDocknetState(tenantName, networkName, serviceName string) *DnetOperState {
	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		log.Warnf("Couldn't read global config %v", err)
		return nil
	}

	// save docknet oper state
	dnetOper := DnetOperState{}
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
		t.Fatalf("Error creating docker network. Err: %v", err)
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
	ninfo, err := docker.NetworkInspect(context.Background(), docknetName)
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

func checkDocknetCreateIPv6(t *testing.T, tenantName, networkName, serviceName, subnet, gw, ipv6subnet, ipv6gw string) {
	docknetName := GetDocknetName(tenantName, networkName, serviceName)
	subnetIP, subnetLen, _ := netutils.ParseCIDR(subnet)
	ipv6subnetAddr, ipv6subnetLen, _ := netutils.ParseCIDR(ipv6subnet)

	nwcfg := mastercfg.CfgNetworkState{
		Tenant:        tenantName,
		NetworkName:   networkName,
		PktTagType:    "vlan",
		PktTag:        1,
		ExtPktTag:     1,
		SubnetIP:      subnetIP,
		SubnetLen:     subnetLen,
		Gateway:       gw,
		IPv6Subnet:    ipv6subnetAddr,
		IPv6SubnetLen: ipv6subnetLen,
		IPv6Gateway:   ipv6gw,
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
	ninfo, err := docker.NetworkInspect(context.Background(), docknetName)
	if err != nil {
		t.Fatalf("Error getting network info for %s. Err: %v", docknetName, err)
	}

	// verify params are correct
	if ninfo.Scope != "local" || ninfo.Driver != netDriverName || ninfo.IPAM.Driver != ipamDriverName ||
		ninfo.IPAM.Config[0].Subnet != subnet || ninfo.IPAM.Config[0].Gateway != gw {
		t.Fatalf("Docker network {%+v} does not match expected values", ninfo)
	}
	// verify ipv6 params are correct
	if ninfo.IPAM.Config[1].Subnet != ipv6subnet || ninfo.IPAM.Config[1].Gateway != ipv6gw {
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
	_, err = docker.NetworkInspect(context.Background(), docknetName)
	if err == nil {
		t.Fatalf("docker net %s was not deleted. Err: %v", docknetName, err)
	}
}

func TestMain(m *testing.M) {
	// change driver names for unit-testing
	UpdateDockerV2PluginName("bridge", "default")

	initStateDriver()

	os.Exit(m.Run())
}

func TestDocknetCreateDelete(t *testing.T) {
	// test network creation
	checkDocknetCreate(t, "unit-test", "net1", "", "10.1.1.1/24", "10.1.1.254")
	checkDocknetDelete(t, "unit-test", "net1", "")

	// test ipv6 network creation
	checkDocknetCreateIPv6(t, "unit-test", "net1", "", "10.1.1.1/24", "10.1.1.254", "2016:0430::/100", "2016:0430::254")
	checkDocknetDelete(t, "unit-test", "net1", "")

	// test service names
	checkDocknetCreate(t, "unit-test", "net1", "srv1", "10.1.1.1/24", "10.1.1.254")
	checkDocknetDelete(t, "unit-test", "net1", "srv1")
}

func TestUpdateDockerV2PluginName(t *testing.T) {
	expectNetDriver := "bridge"
	expectIPAMDriver := "default"
	UpdateDockerV2PluginName("bridge", "default")

	if expectNetDriver != netDriverName {
		t.Fatalf("Unexpected netdriver name. Expected: %s. Actual: %s",
			expectNetDriver, netDriverName)
		t.Fail()
	}

	if expectIPAMDriver != ipamDriverName {
		t.Fatalf("Unexpected ipamdriver name. Expected: %s. Actual: %s",
			expectIPAMDriver, ipamDriverName)
		t.Fail()
	}
}
