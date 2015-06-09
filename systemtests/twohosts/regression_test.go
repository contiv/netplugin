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

package twohosts

import (
	"io/ioutil"
	"testing"

	"github.com/contiv/netplugin/systemtests/utils"
)

func TestMultipleEpsInContainer_regress(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, testbed.GetNodes())
		utils.StopOnError(t.Failed())
	}()

	cfgFile := utils.GetCfgFile("multiple_eps_in_container")
	jsonCfg, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("failed to read config file %s \n", err)
	}
	utils.ConfigSetupCommon(t, string(jsonCfg), testbed.GetNodes())

	node1 := testbed.GetNodes()[0]
	node2 := testbed.GetNodes()[1]

	// Container2 is reachable on both orange and purple networks
	utils.StartServer(t, node1, "myContainer2")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer2")
	}()

	ipAddress := utils.GetIPAddress(t, node1, "orange-myContainer2")
	utils.StartClient(t, node1, "myContainer3", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer3")
	}()

	ipAddress = utils.GetIPAddress(t, node2, "purple-myContainer2")
	utils.StartClient(t, node2, "myContainer4", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer4")
	}()

	// Container1 is reachable on only on orange network
	utils.StartServer(t, node1, "myContainer1")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer1")
	}()

	ipAddress = utils.GetIPAddress(t, node2, "orange-myContainer1")
	utils.StartClient(t, node2, "myContainer3", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer3")
	}()
	utils.DockerCleanup(t, node2, "myContainer4")
	utils.StartClientFailure(t, node2, "myContainer4", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer4")
	}()
}

func TestTwoHostVlan_regress(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, testbed.GetNodes())
		utils.StopOnError(t.Failed())
	}()

	cfgFile := utils.GetCfgFile("two_host_vlan")
	jsonCfg, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("failed to read config file %s \n", err)
	}
	utils.ConfigSetupCommon(t, string(jsonCfg), testbed.GetNodes())

	node1 := testbed.GetNodes()[0]
	node2 := testbed.GetNodes()[1]

	// all four containers can talk to each other
	utils.StartServer(t, node1, "myContainer2")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer2")
	}()
	utils.StartServer(t, node2, "myContainer4")
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer4")
	}()

	ipAddress := utils.GetIPAddress(t, node2, "orange-myContainer2")
	utils.StartClient(t, node2, "myContainer3", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer3")
	}()
	utils.StartClient(t, node1, "myContainer1", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer1")
	}()

	ipAddress = utils.GetIPAddress(t, node2, "orange-myContainer4")
	utils.DockerCleanup(t, node2, "myContainer3")
	utils.StartClient(t, node2, "myContainer3", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer3")
	}()
	utils.DockerCleanup(t, node1, "myContainer1")
	utils.StartClient(t, node1, "myContainer1", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer1")
	}()

}

func TestTwoHostVxlan_regress(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, testbed.GetNodes())
		utils.StopOnError(t.Failed())
	}()

	cfgFile := utils.GetCfgFile("two_host_vxlan")
	jsonCfg, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("failed to read config file %s \n", err)
	}
	utils.ConfigSetupCommon(t, string(jsonCfg), testbed.GetNodes())

	node1 := testbed.GetNodes()[0]
	node2 := testbed.GetNodes()[1]

	// all four containers can talk to each other
	utils.StartServer(t, node1, "myContainer2")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer2")
	}()
	utils.StartServer(t, node2, "myContainer4")
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer4")
	}()

	ipAddress := utils.GetIPAddress(t, node2, "orange-myContainer2")
	utils.StartClient(t, node2, "myContainer3", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer3")
	}()
	utils.StartClient(t, node1, "myContainer1", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer1")
	}()

	ipAddress = utils.GetIPAddress(t, node2, "orange-myContainer4")
	utils.DockerCleanup(t, node2, "myContainer3")
	utils.StartClient(t, node2, "myContainer3", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer3")
	}()
	utils.DockerCleanup(t, node1, "myContainer1")
	utils.StartClient(t, node1, "myContainer1", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer1")
	}()

}

func TestTwoHostsMultipleTenants_regress(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, testbed.GetNodes())
		utils.StopOnError(t.Failed())
	}()

	cfgFile := utils.GetCfgFile("two_hosts_multiple_tenants")
	jsonCfg, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("failed to read config file %s \n", err)
	}
	utils.ConfigSetupCommon(t, string(jsonCfg), testbed.GetNodes())

	node1 := testbed.GetNodes()[0]
	node2 := testbed.GetNodes()[1]

	// Container1 and Container3 are on orange network
	utils.StartServer(t, node1, "myContainer1")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer1")
	}()

	ipAddress := utils.GetIPAddress(t, node2, "orange-myContainer1")
	utils.StartClient(t, node2, "myContainer3", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer3")
	}()

	// Container2 and Container4 are on purple network
	utils.StartServer(t, node1, "myContainer2")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer2")
	}()

	ipAddress = utils.GetIPAddress(t, node2, "purple-myContainer2")
	utils.StartClient(t, node2, "myContainer4", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer4")
	}()
}

func TestTwoHostsMultipleTenantsMixVlanVxlan_regress(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, testbed.GetNodes())
		utils.StopOnError(t.Failed())
	}()

	cfgFile := utils.GetCfgFile("two_hosts_multiple_tenants_mix_vlan_vxlan")
	jsonCfg, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("failed to read config file %s \n", err)
	}
	utils.ConfigSetupCommon(t, string(jsonCfg), testbed.GetNodes())

	node1 := testbed.GetNodes()[0]
	node2 := testbed.GetNodes()[1]

	// Container1 and Container3 are on orange network
	utils.StartServer(t, node1, "myContainer1")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer1")
	}()

	ipAddress := utils.GetIPAddress(t, node2, "orange-myContainer1")
	utils.StartClient(t, node2, "myContainer3", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer3")
	}()

	// Container2 and Container4 are on purple network
	utils.StartServer(t, node1, "myContainer2")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer2")
	}()

	ipAddress = utils.GetIPAddress(t, node2, "purple-myContainer2")
	utils.StartClient(t, node2, "myContainer4", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer4")
	}()
}

func TestTwoHostsMultipleVlansNets_regress(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, testbed.GetNodes())
		utils.StopOnError(t.Failed())
	}()

	cfgFile := utils.GetCfgFile("two_hosts_multiple_vlans_nets")
	jsonCfg, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("failed to read config file %s \n", err)
	}
	utils.ConfigSetupCommon(t, string(jsonCfg), testbed.GetNodes())

	node1 := testbed.GetNodes()[0]
	node2 := testbed.GetNodes()[1]

	// Container1 and Container3 are on orange network
	utils.StartServer(t, node1, "myContainer1")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer1")
	}()

	ipAddress := utils.GetIPAddress(t, node2, "orange-myContainer1")
	utils.DockerCleanup(t, node2, "myContainer3")
	utils.StartClient(t, node2, "myContainer3", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer3")
	}()

	// Container2 and Container4 are on purple network
	utils.StartServer(t, node1, "myContainer2")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer2")
	}()

	ipAddress = utils.GetIPAddress(t, node2, "purple-myContainer2")
	utils.StartClient(t, node2, "myContainer4", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer4")
	}()
}

func TestTwoHostsMultipleVxlansNets_regress(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, testbed.GetNodes())
		utils.StopOnError(t.Failed())
	}()

	cfgFile := utils.GetCfgFile("two_hosts_multiple_vxlan_nets")
	jsonCfg, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("failed to read config file %s \n", err)
	}
	utils.ConfigSetupCommon(t, string(jsonCfg), testbed.GetNodes())

	node1 := testbed.GetNodes()[0]
	node2 := testbed.GetNodes()[1]

	// Container1 and Container3 are on orange network
	utils.StartServer(t, node1, "myContainer1")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer1")
	}()

	ipAddress := utils.GetIPAddress(t, node2, "orange-myContainer1")
	utils.StartClient(t, node2, "myContainer3", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer3")
	}()

	// Container2 and Container4 are on purple network
	utils.StartServer(t, node1, "myContainer2")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer2")
	}()

	ipAddress = utils.GetIPAddress(t, node2, "purple-myContainer2")
	utils.StartClient(t, node2, "myContainer4", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer4")
	}()
}

func TestTwoHostsMultipleVxlansNetsLateHostBindings_regress(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, testbed.GetNodes())
		utils.StopOnError(t.Failed())
	}()

	cfgFile := utils.GetCfgFile("late_bindings/multiple_vxlan_nets")
	jsonCfg, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("failed to read config file %s \n", err)
	}
	utils.ConfigSetupCommon(t, string(jsonCfg), testbed.GetNodes())
	node1 := testbed.GetNodes()[0]
	node2 := testbed.GetNodes()[1]

	cfgFile = utils.GetCfgFile("late_bindings/multiple_vxlan_nets_host_bindings")
	jsonCfg, err = ioutil.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("failed to read config file %s \n", err)
	}
	utils.ApplyHostBindingsConfig(t, string(jsonCfg), node1)

	// Container1 and Container3 are on orange network
	utils.StartServer(t, node1, "myContainer1")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer1")
	}()

	ipAddress := utils.GetIPAddress(t, node2, "orange-myContainer1")
	utils.StartClient(t, node2, "myContainer3", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer3")
	}()

	// Container2 and Container4 are on purple network
	utils.StartServer(t, node1, "myContainer2")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer2")
	}()

	ipAddress = utils.GetIPAddress(t, node2, "purple-myContainer2")
	utils.StartClient(t, node2, "myContainer4", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer4")
	}()
}

func TestTwoHostsMultipleVxlansNetsLateContainerBindings_regress(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, testbed.GetNodes())
	}()

	cfgFile := utils.GetCfgFile("late_bindings/multiple_vxlan_nets")
	jsonCfg, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("failed to read config file %s \n", err)
	}
	utils.ConfigSetupCommon(t, string(jsonCfg), testbed.GetNodes())
	node1 := testbed.GetNodes()[0]
	node2 := testbed.GetNodes()[1]

	// Start server containers: Container1 and Container2
	utils.StartServer(t, node1, "myContainer1")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer1")
	}()
	// Container2 and Container4 are on purple network
	utils.StartServer(t, node1, "myContainer2")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer2")
	}()

	// apply uuid base config on started containers
	cfgFile = utils.GetCfgFile("late_bindings/multiple_vxlan_nets_host_bindings")
	jsonCfg, err = ioutil.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("failed to read config file %s \n", err)
	}
	jsonCfgStr, _ := utils.FixUpContainerUUIDs(t, testbed.GetNodes(), string(jsonCfg))
	utils.ApplyHostBindingsConfig(t, jsonCfgStr, node1)

	// start client containers and test ping: myContainer1 and myContainer4
	ipAddress := utils.GetIPAddress(t, node2, "orange-myContainer1")
	utils.StartClient(t, node2, "myContainer3", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer3")
	}()
	ipAddress = utils.GetIPAddress(t, node2, "purple-myContainer2")
	utils.StartClient(t, node2, "myContainer4", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer4")
	}()
}

func TestTwoHostsMultipleVxlansNetsInfraContainerBindings_regress(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, testbed.GetNodes())
	}()

	cfgFile := utils.GetCfgFile("container_bindings/multiple_vxlan_nets")
	jsonCfg, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("failed to read config file %s \n", err)
	}
	utils.ConfigSetupCommon(t, string(jsonCfg), testbed.GetNodes())
	node1 := testbed.GetNodes()[0]
	node2 := testbed.GetNodes()[1]

	// Start server containers: Container1 and Container2
	utils.StartServer(t, node1, "myContainer1")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer1")
	}()
	// Container2 and Container4 are on purple network
	utils.StartServer(t, node1, "myContainer2")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer2")
	}()

	// read host bindings and infra container mappings
	cfgFile = utils.GetCfgFile("container_bindings/multiple_vxlan_nets_host_bindings")
	jsonCfg, err = ioutil.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("failed to read config file %s \n", err)
	}

	cfgFile = utils.GetCfgFile("container_bindings/multiple_vxlan_nets_infra_container_bindings")
	infraContMappings, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("failed to read config file %s \n", err)
	}

	jsonCfgStr, _ := utils.FixUpInfraContainerUUIDs(t, testbed.GetNodes(), string(jsonCfg), string(infraContMappings))
	utils.ApplyHostBindingsConfig(t, jsonCfgStr, node1)

	// start client containers and test ping: myContainer1 and myContainer4
	ipAddress := utils.GetIPAddress(t, node2, "orange-myPod1")
	utils.StartClient(t, node2, "myContainer3", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer3")
	}()
	ipAddress = utils.GetIPAddress(t, node2, "purple-myPod2")
	utils.StartClient(t, node2, "myContainer4", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer4")
	}()
}

// XXX: don't run this until we upgrade docker to a recent version that supports
// labels and build-time env
func TestTwoHostVlanPowerstripDocker(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, testbed.GetNodes())
		utils.StopOnError(t.Failed())
	}()

	cfgFile := utils.GetCfgFile("late_bindings/powerstrip_demo_vlan_nets")
	jsonCfg, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("failed to read config file %s \n", err)
	}

	node1 := testbed.GetNodes()[0]
	node2 := testbed.GetNodes()[1]

	utils.StartNetPlugin(t, testbed.GetNodes(), true)

	utils.StartPowerStripAdapter(t, testbed.GetNodes())

	utils.ApplyDesiredConfig(t, string(jsonCfg), node1)

	env := []string{"DOCKER_HOST=localhost:2375"}
	utils.StartServerWithEnvAndArgs(t, node1, "server1", env,
		[]string{"--label", "netid=orange", "--label", "tenantid=tenant-one"})
	defer func() {
		utils.DockerCleanupWithEnv(t, node1, "server1", env)
	}()
	ipAddress := utils.GetIPAddressFromNetworkAndContainerName(t, node1,
		"orange", "server1")

	// test ping success between containers in same network
	utils.StartClientWithEnvAndArgs(t, node2, "client1", ipAddress, env,
		[]string{"--label", "netid=orange", "--label", "tenantid=tenant-one"})
	defer func() {
		utils.DockerCleanupWithEnv(t, node2, "client1", env)
	}()

	// test ping failure between containers in different networks
	utils.StartClientFailureWithEnvAndArgs(t, node2, "client2", ipAddress, env,
		[]string{"--label", "netid=purple", "--label", "tenantid=tenant-one"})
	defer func() {
		utils.DockerCleanupWithEnv(t, node2, "client2", env)
	}()
}
