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
	"testing"

	"github.com/contiv/netplugin/systemtests/utils"
)

func TestTwoHostsSingleVlanPingSuccess_sanity(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, vagrant.GetNodes())
		utils.StopOnError(t.Failed())
	}()

	jsonCfg :=
		`{
        "Hosts" : [{
            "Name"                      : "host1",
            "Intf"                      : "eth2"
        },
        {
            "Name"                      : "host2",
            "Intf"                      : "eth2"
        }],
        "Tenants" : [ {
            "Name"                      : "tenant-one",
            "DefaultNetType"            : "vlan",
            "SubnetPool"                : "11.1.0.0/16",
            "AllocSubnetLen"            : 24,
            "Vlans"                     : "11-48",
            "Networks"  : [ {
                "Name"                  : "orange",
                "Endpoints" : [
                {
                    "Container"         : "myContainer1",
                    "Host"              : "host1"
                },
                {
                    "Container"         : "myContainer2",
                    "Host"              : "host2"
                } ]
            } ]
        } ]
        }`

	utils.ConfigSetupCommon(t, jsonCfg, vagrant.GetNodes())

	node1 := vagrant.GetNodes()[0]
	node2 := vagrant.GetNodes()[1]

	utils.StartServer(t, node1, "myContainer1")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer1")
	}()

	ipAddress := utils.GetIpAddress(t, node2, "orange-myContainer1")
	utils.StartClient(t, node2, "myContainer2", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer2")
	}()
}

func TestTwoHostsMultiVlanPingSuccess_sanity(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, vagrant.GetNodes())
		utils.StopOnError(t.Failed())
	}()

	jsonCfg :=
		`{
        "Hosts" : [{
            "Name"                      : "host1",
            "Intf"                      : "eth2"
        },
        {
            "Name"                      : "host2",
            "Intf"                      : "eth2"
        }],
        "Tenants" : [ {
            "Name"                      : "tenant-one",
            "DefaultNetType"            : "vlan",
            "SubnetPool"                : "11.1.0.0/16",
            "AllocSubnetLen"            : 24,
            "Vlans"                     : "11-48",
            "Networks"  : [ {
                "Name"                  : "orange",
                "Endpoints" : [
                {
                    "Container"         : "myContainer1",
                    "Host"              : "host1"
                },
                {
                    "Container"         : "myContainer2",
                    "Host"              : "host2"
                } ]
            },
            {
                "Name"                  : "purple",
                "Endpoints" : [
                {
                    "Container"         : "myContainer3",
                    "Host"              : "host1"
                },
                {
                    "Container"         : "myContainer4",
                    "Host"              : "host2"
                } ]
            } ]
        } ]
        }`

	utils.ConfigSetupCommon(t, jsonCfg, vagrant.GetNodes())

	node1 := vagrant.GetNodes()[0]
	node2 := vagrant.GetNodes()[1]

	utils.StartServer(t, node1, "myContainer1")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer1")
	}()

	ipAddress := utils.GetIpAddress(t, node2, "orange-myContainer1")
	utils.StartClient(t, node2, "myContainer2", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer2")
	}()

	utils.StartServer(t, node2, "myContainer4")
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer4")
	}()

	ipAddress = utils.GetIpAddress(t, node1, "purple-myContainer4")
	utils.StartClient(t, node1, "myContainer3", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer3")
	}()
}

func TestTwoHostsMultiVlanPingFailure_sanity(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, vagrant.GetNodes())
		utils.StopOnError(t.Failed())
	}()

	jsonCfg :=
		`{
        "Hosts" : [{
            "Name"                      : "host1",
            "Intf"                      : "eth2"
        },
        {
            "Name"                      : "host2",
            "Intf"                      : "eth2"
        }],
        "Tenants" : [ {
            "Name"                      : "tenant-one",
            "DefaultNetType"            : "vlan",
            "SubnetPool"                : "11.1.0.0/16",
            "AllocSubnetLen"            : 24,
            "Vlans"                     : "11-48",
            "Networks"  : [ {
                "Name"                  : "orange",
                "Endpoints" : [
                {
                    "Container"         : "myContainer1",
                    "Host"              : "host1"
                } ]
            },
            {
                "Name"                  : "purple",
                "Endpoints" : [
                {
                    "Container"         : "myContainer2",
                    "Host"              : "host2"
                } ]
            } ]
        } ]
        }`

	utils.ConfigSetupCommon(t, jsonCfg, vagrant.GetNodes())

	node1 := vagrant.GetNodes()[0]
	node2 := vagrant.GetNodes()[1]

	utils.StartServer(t, node1, "myContainer1")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer1")
	}()

	ipAddress := utils.GetIpAddress(t, node2, "orange-myContainer1")
	utils.StartClientFailure(t, node2, "myContainer2", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer2")
	}()
}

func TestTwoHostsMultiVxlanPingSuccess_sanity(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, vagrant.GetNodes())
		utils.StopOnError(t.Failed())
	}()

	jsonCfg :=
		`{
        "Hosts" : [{
            "Name"                      : "host1",
            "VtepIp"                    : "192.168.2.10"
        },
        {
            "Name"                      : "host2",
            "VtepIp"                    : "192.168.2.11"
        }],
        "Tenants" : [ {
            "Name"                      : "tenant-one",
            "DefaultNetType"            : "vxlan",
            "SubnetPool"                : "11.1.0.0/16",
            "AllocSubnetLen"            : 24,
            "VXlans"                    : "10001-14000",
            "Networks"  : [ 
            {
                "Name"                  : "orange",
                "Endpoints" : [
                {
                    "Container"         : "myContainer1",
                    "Host"              : "host1"
                },
                {
                    "Container"         : "myContainer2",
                    "Host"              : "host2"
                } ]
            },
            {
                "Name"                  : "purple",
                "Endpoints" : [
                {
                    "Container"         : "myContainer3",
                    "Host"              : "host1"
                },
                {
                    "Container"         : "myContainer4",
                    "Host"              : "host2"
                } ]
            } ]
        } ]
        }`

	utils.ConfigSetupCommon(t, jsonCfg, vagrant.GetNodes())

	node1 := vagrant.GetNodes()[0]
	node2 := vagrant.GetNodes()[1]

	utils.StartServer(t, node1, "myContainer1")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer1")
	}()

	utils.StartServer(t, node1, "myContainer3")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer3")
	}()

	ipAddress := utils.GetIpAddress(t, node2, "orange-myContainer1")
	utils.StartClient(t, node2, "myContainer2", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer2")
	}()

	ipAddress = utils.GetIpAddress(t, node2, "purple-myContainer3")
	utils.StartClient(t, node2, "myContainer4", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer4")
	}()
}

func TestTwoHostsMultiVxlanPingFailure_sanity(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, vagrant.GetNodes())
		utils.StopOnError(t.Failed())
	}()

	//create a single vlan network, with two endpoints
	jsonCfg :=
		`{
        "Hosts" : [{
            "Name"                      : "host1",
            "VtepIp"                    : "192.168.2.10"
        },
        {
            "Name"                      : "host2",
            "VtepIp"                    : "192.168.2.11"
        }],
        "Tenants" : [ {
            "Name"                      : "tenant-one",
            "DefaultNetType"            : "vxlan",
            "SubnetPool"                : "11.1.0.0/16",
            "AllocSubnetLen"            : 24,
            "VXlans"                    : "10001-14000",
            "Networks"  : [ 
            {
                "Name"                  : "orange",
                "Endpoints" : [
                {
                    "Container"         : "myContainer1",
                    "Host"              : "host1"
                },
                {
                    "Container"         : "myContainer2",
                    "Host"              : "host2"
                } ]
            },
            {
                "Name"                  : "purple",
                "Endpoints" : [
                {
                    "Container"         : "myContainer3",
                    "Host"              : "host1"
                },
                {
                    "Container"         : "myContainer4",
                    "Host"              : "host2"
                } ]
            } ]
        } ]
        }`

	utils.ConfigSetupCommon(t, jsonCfg, vagrant.GetNodes())

	node1 := vagrant.GetNodes()[0]
	node2 := vagrant.GetNodes()[1]

	utils.StartServer(t, node1, "myContainer1")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer1")
	}()

	ipAddress := utils.GetIpAddress(t, node2, "orange-myContainer1")
	utils.StartClientFailure(t, node2, "myContainer4", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer4")
	}()

	utils.StartClientFailure(t, node2, "myContainer3", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer3")
	}()
}

func TestTwoHostsVxlanDeltaConfig_sanity_sanity(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, vagrant.GetNodes())
		utils.StopOnError(t.Failed())
	}()

	jsonCfg :=
		`{
        "Hosts" : [{
            "Name"                      : "host1",
            "VtepIp"                    : "192.168.2.10"
        },
        {
            "Name"                      : "host2",
            "VtepIp"                    : "192.168.2.11"
        }],
        "Tenants" : [ {
            "Name"                      : "tenant-one",
            "DefaultNetType"            : "vxlan",
            "SubnetPool"                : "11.1.0.0/16",
            "AllocSubnetLen"            : 24,
            "VXlans"                    : "10001-14000",
            "Networks"  : [ 
            {
                "Name"                  : "orange",
                "Endpoints" : [
                {
                    "Container"         : "myContainer1",
                    "Host"              : "host1"
                },
                {
                    "Container"         : "myContainer2",
                    "Host"              : "host2"
                } ]
            },
            {
                "Name"                  : "purple",
                "Endpoints" : [
                {
                    "Container"         : "myContainer3",
                    "Host"              : "host1"
                },
                {
                    "Container"         : "myContainer4",
                    "Host"              : "host2"
                } ]
            } ]
        } ]
        }`

	utils.ConfigSetupCommon(t, jsonCfg, vagrant.GetNodes())

	node1 := vagrant.GetNodes()[0]
	node2 := vagrant.GetNodes()[1]

	utils.StartServer(t, node1, "myContainer1")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer1")
	}()

	utils.StartServer(t, node1, "myContainer3")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer3")
	}()

	ipAddress := utils.GetIpAddress(t, node2, "orange-myContainer1")
	utils.StartClient(t, node2, "myContainer2", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer2")
	}()

	ipAddress = utils.GetIpAddress(t, node2, "purple-myContainer3")
	utils.StartClient(t, node2, "myContainer4", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer4")
	}()

	jsonCfg =
		`{
        "Hosts" : [{
            "Name"                      : "host1",
            "VtepIp"                    : "192.168.2.10"
        },
        {
            "Name"                      : "host2",
            "VtepIp"                    : "192.168.2.11"
        }],
        "Tenants" : [ {
            "Name"                      : "tenant-one",
            "DefaultNetType"            : "vxlan",
            "SubnetPool"                : "11.1.0.0/16",
            "AllocSubnetLen"            : 24,
            "VXlans"                    : "10001-14000",
            "Networks"  : [ 
            {
                "Name"                  : "orange",
                "Endpoints" : [
                {
                    "Container"         : "myContainer1",
                    "Host"              : "host1"
                },
                {
                    "Container"         : "myContainer2",
                    "Host"              : "host2"
                } ]
            },
            {
                "Name"                  : "purple",
                "Endpoints" : [
                {
                    "Container"         : "myContainer3",
                    "Host"              : "host1"
                },
                {
                    "Container"         : "myContainer2",
                    "Host"              : "host2"
                },
                {
                    "Container"         : "myContainer4",
                    "Host"              : "host2"
                } ]
            } ]
        } ]
        }`
	utils.ApplyDesiredConfig(t, jsonCfg, vagrant.GetNodes()[0])

	ipAddress = utils.GetIpAddress(t, node2, "purple-myContainer3")
	utils.DockerCleanup(t, node2, "myContainer2")
	utils.StartClient(t, node2, "myContainer2", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer2")
	}()
}

func TestTwoHostsVxlanAddDelEp_sanity(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, vagrant.GetNodes())
		utils.StopOnError(t.Failed())
	}()

	jsonCfg :=
		`{
        "Hosts" : [{
            "Name"                      : "host1",
            "VtepIp"                    : "192.168.2.10"
        },
        {
            "Name"                      : "host2",
            "VtepIp"                    : "192.168.2.11"
        }],
        "Tenants" : [ {
            "Name"                      : "tenant-one",
            "DefaultNetType"            : "vxlan",
            "SubnetPool"                : "11.1.0.0/16",
            "AllocSubnetLen"            : 24,
            "VXlans"                    : "10001-14000",
            "Networks"  : [ 
            {
                "Name"                  : "orange",
                "Endpoints" : [
                {
                    "Container"         : "myContainer1",
                    "Host"              : "host1"
                },
                {
                    "Container"         : "myContainer2",
                    "Host"              : "host2"
                } ]
            },
            {
                "Name"                  : "purple",
                "Endpoints" : [
                {
                    "Container"         : "myContainer3",
                    "Host"              : "host1"
                },
                {
                    "Container"         : "myContainer4",
                    "Host"              : "host2"
                } ]
            } ]
        } ]
        }`

	utils.ConfigSetupCommon(t, jsonCfg, vagrant.GetNodes())

	node1 := vagrant.GetNodes()[0]
	node2 := vagrant.GetNodes()[1]

	utils.StartServer(t, node1, "myContainer1")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer1")
	}()

	utils.StartServer(t, node1, "myContainer3")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer3")
	}()

	ipAddress := utils.GetIpAddress(t, node2, "orange-myContainer1")
	utils.StartClient(t, node2, "myContainer2", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer2")
	}()

	ipAddress = utils.GetIpAddress(t, node2, "purple-myContainer3")
	utils.StartClient(t, node2, "myContainer4", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer4")
	}()

	jsonCfg = `
	{
	"Tenants" : [ {
		"Name"                      : "tenant-one",
		"Networks"  : [
		{
			"Name"                  : "orange",
			"Endpoints" : [
			{
				"Container"         : "myContainer5",
				"Host"              : "host1"
			}
			]
		}
		]
	} ]
	}`
	utils.AddConfig(t, jsonCfg, vagrant.GetNodes()[0])

	utils.StartServer(t, node1, "myContainer5")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer5")
	}()

	ipAddress = utils.GetIpAddress(t, node2, "orange-myContainer5")
	utils.DockerCleanup(t, node2, "myContainer2")
	utils.StartClient(t, node2, "myContainer2", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer2")
	}()

	utils.DelConfig(t, jsonCfg, vagrant.GetNodes()[0])

	utils.DockerCleanup(t, node2, "myContainer2")
	utils.StartClientFailure(t, node2, "myContainer2", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer2")
	}()

	jsonCfg = `
	{
	"Tenants" : [ {
		"Name"                      : "tenant-one",
		"Networks"  : [
		{
			"Name"                  : "purple",
			"Endpoints" : [
			{
				"Container"         : "myContainer5",
				"Host"              : "host1"
			}
			]
		}
		]
	} ]
	}`

	utils.AddConfig(t, jsonCfg, vagrant.GetNodes()[0])
	ipAddress = utils.GetIpAddress(t, node2, "purple-myContainer5")
	utils.DockerCleanup(t, node2, "myContainer4")
	utils.StartClient(t, node2, "myContainer4", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer4")
	}()
}

func TestTwoHostsVxlanAddDelNetwork_sanity(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, vagrant.GetNodes())
		utils.StopOnError(t.Failed())
	}()

	jsonCfg :=
		`{
        "Hosts" : [{
            "Name"                      : "host1",
            "VtepIp"                    : "192.168.2.10"
        },
        {
            "Name"                      : "host2",
            "VtepIp"                    : "192.168.2.11"
        }],
        "Tenants" : [ {
            "Name"                      : "tenant-one",
            "DefaultNetType"            : "vxlan",
            "SubnetPool"                : "11.1.0.0/16",
            "AllocSubnetLen"            : 24,
            "VXlans"                    : "10001-14000",
            "Networks"  : [ 
            {
                "Name"                  : "orange",
                "Endpoints" : [
                {
                    "Container"         : "myContainer1",
                    "Host"              : "host1"
                },
                {
                    "Container"         : "myContainer2",
                    "Host"              : "host2"
                } ]
            },
            {
                "Name"                  : "purple",
                "Endpoints" : [
                {
                    "Container"         : "myContainer3",
                    "Host"              : "host1"
                },
                {
                    "Container"         : "myContainer4",
                    "Host"              : "host2"
                } ]
            } ]
        } ]
        }`

	utils.ConfigSetupCommon(t, jsonCfg, vagrant.GetNodes())

	node1 := vagrant.GetNodes()[0]
	node2 := vagrant.GetNodes()[1]

	utils.StartServer(t, node1, "myContainer1")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer1")
	}()

	utils.StartServer(t, node1, "myContainer3")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer3")
	}()

	ipAddress := utils.GetIpAddress(t, node2, "orange-myContainer1")
	utils.StartClient(t, node2, "myContainer2", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer2")
	}()

	ipAddress = utils.GetIpAddress(t, node2, "purple-myContainer3")
	utils.StartClient(t, node2, "myContainer4", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer4")
	}()

	jsonCfg = `
	{
	"Tenants" : [ {
		"Name"                      : "tenant-one",
		"Networks"  : [
		{
			"Name"                  : "green",
			"Endpoints" : [
			{
				"Container"         : "myContainer5",
				"Host"              : "host1"
			},
			{
				"Container"         : "myContainer6",
				"Host"              : "host2"
			}
			]
		}
		]
	} ]
	}`
	utils.AddConfig(t, jsonCfg, vagrant.GetNodes()[0])

	utils.StartServer(t, node1, "myContainer5")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer5")
	}()

	ipAddress = utils.GetIpAddress(t, node2, "green-myContainer5")
	utils.StartClient(t, node2, "myContainer6", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer6")
	}()

	utils.DelConfig(t, jsonCfg, vagrant.GetNodes()[0])

	utils.DockerCleanup(t, node2, "myContainer6")
	utils.StartClientFailure(t, node2, "myContainer6", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node2, "myContainer6")
	}()

	jsonCfg = `
	{
	"Tenants" : [ {
		"Name"                      : "tenant-one",
		"Networks"  : [
		{
			"Name"                  : "green"
		}
		]
	} ]
	}`

	utils.DelConfig(t, jsonCfg, vagrant.GetNodes()[0])

	if utils.NetworkStateExists(node2, "green") == nil {
		t.Fatalf("Error - network %s doesn't seem to be deleted \n", "green")
	}
}
