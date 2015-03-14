package twohosts

import (
	"log"
	"os"
	"testing"

	"github.com/contiv/netplugin/systemtests/utils"
)

var vagrant *utils.Vagrant

func TestMain(m *testing.M) {

	vagrant = &utils.Vagrant{}
	log.Printf("Starting vagrant up...")
	err := vagrant.Setup(os.Getenv("CONTIV_ENV"), 2)
	log.Printf("Done with vagrant up...")
	if err != nil {
		log.Printf("Vagrant setup failed. Error: %s", err)
		vagrant.Teardown()
		os.Exit(1)
	}

	exitCode := m.Run()

	vagrant.Teardown()

	os.Exit(exitCode)
}

func TestTwoHostsSingleVlanPingSuccess(t *testing.T) {
	defer func() {
		//utils.ConfigCleanupCommon(t, vagrant.GetNodes())
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

	utils.StartAndWait(t, node1, "myContainer1")
	defer func() {
		utils.DockerCleanup(node1, "myContainer1")
	}()

	ipAddress := utils.GetIpAddress(t, node2, "orange-myContainer1")
	utils.StartAndPing(t, node2, "myContainer2", ipAddress)
	defer func() {
		utils.DockerCleanup(node2, "myContainer2")
	}()
}

func TestTwoHostsMultiVlanPingSuccess(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, vagrant.GetNodes())
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

	utils.StartAndWait(t, node1, "myContainer1")
	defer func() {
		utils.DockerCleanup(node1, "myContainer1")
	}()

	ipAddress := utils.GetIpAddress(t, node2, "orange-myContainer1")
	utils.StartAndPing(t, node2, "myContainer2", ipAddress)
	defer func() {
		utils.DockerCleanup(node2, "myContainer2")
	}()

	utils.StartAndWait(t, node2, "myContainer4")
	defer func() {
		utils.DockerCleanup(node2, "myContainer4")
	}()

	ipAddress = utils.GetIpAddress(t, node1, "purple-myContainer4")
	utils.StartAndPing(t, node1, "myContainer3", ipAddress)
	defer func() {
		utils.DockerCleanup(node1, "myContainer3")
	}()
}

func TestTwoHostsMultiVlanPingFailure(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, vagrant.GetNodes())
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

	utils.StartAndWait(t, node1, "myContainer1")
	defer func() {
		utils.DockerCleanup(node1, "myContainer1")
	}()

	ipAddress := utils.GetIpAddress(t, node2, "orange-myContainer1")
	utils.StartAndEnsurePingFailure(t, node2, "myContainer2", ipAddress)
	defer func() {
		utils.DockerCleanup(node2, "myContainer2")
	}()
}

func TestTwoHostsMultiVxlanPingSuccess(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, vagrant.GetNodes())
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
            "VXlans"                    : "10001-20000",
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

	utils.StartAndWait(t, node1, "myContainer1")
	defer func() {
		utils.DockerCleanup(node1, "myContainer1")
	}()

	utils.StartAndWait(t, node1, "myContainer3")
	defer func() {
		utils.DockerCleanup(node1, "myContainer3")
	}()

	ipAddress := utils.GetIpAddress(t, node2, "orange-myContainer1")
	utils.StartAndPing(t, node2, "myContainer2", ipAddress)
	defer func() {
		utils.DockerCleanup(node2, "myContainer2")
	}()

	ipAddress = utils.GetIpAddress(t, node2, "purple-myContainer3")
	utils.StartAndPing(t, node2, "myContainer4", ipAddress)
	defer func() {
		utils.DockerCleanup(node2, "myContainer4")
	}()
}

func TestTwoHostsMultiVxlanPingFailure(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, vagrant.GetNodes())
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
            "VXlans"                    : "10001-20000",
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

	utils.StartAndWait(t, node1, "myContainer1")
	defer func() {
		utils.DockerCleanup(node1, "myContainer1")
	}()

	ipAddress := utils.GetIpAddress(t, node2, "orange-myContainer1")
	utils.StartAndEnsurePingFailure(t, node2, "myContainer4", ipAddress)
	defer func() {
		utils.DockerCleanup(node2, "myContainer4")
	}()

	utils.StartAndEnsurePingFailure(t, node2, "myContainer3", ipAddress)
	defer func() {
		utils.DockerCleanup(node2, "myContainer3")
	}()
}

func TestTwoHostsVxlanDeltaConfig(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, vagrant.GetNodes())
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
            "VXlans"                    : "10001-20000",
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

	utils.StartAndWait(t, node1, "myContainer1")
	defer func() {
		utils.DockerCleanup(node1, "myContainer1")
	}()

	utils.StartAndWait(t, node1, "myContainer3")
	defer func() {
		utils.DockerCleanup(node1, "myContainer3")
	}()

	ipAddress := utils.GetIpAddress(t, node2, "orange-myContainer1")
	utils.StartAndPing(t, node2, "myContainer2", ipAddress)
	defer func() {
		utils.DockerCleanup(node2, "myContainer2")
	}()

	ipAddress = utils.GetIpAddress(t, node2, "purple-myContainer3")
	utils.StartAndPing(t, node2, "myContainer4", ipAddress)
	defer func() {
		utils.DockerCleanup(node2, "myContainer4")
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
            "VXlans"                    : "10001-20000",
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
	utils.DockerCleanup(node2, "myContainer2")
	utils.StartAndPing(t, node2, "myContainer2", ipAddress)
	defer func() {
		utils.DockerCleanup(node2, "myContainer2")
	}()
}

func TestTwoHostsVxlanAddDelEp(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, vagrant.GetNodes())
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
            "VXlans"                    : "10001-20000",
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

	utils.StartAndWait(t, node1, "myContainer1")
	defer func() {
		utils.DockerCleanup(node1, "myContainer1")
	}()

	utils.StartAndWait(t, node1, "myContainer3")
	defer func() {
		utils.DockerCleanup(node1, "myContainer3")
	}()

	ipAddress := utils.GetIpAddress(t, node2, "orange-myContainer1")
	utils.StartAndPing(t, node2, "myContainer2", ipAddress)
	defer func() {
		utils.DockerCleanup(node2, "myContainer2")
	}()

	ipAddress = utils.GetIpAddress(t, node2, "purple-myContainer3")
	utils.StartAndPing(t, node2, "myContainer4", ipAddress)
	defer func() {
		utils.DockerCleanup(node2, "myContainer4")
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

	utils.StartAndWait(t, node1, "myContainer5")
	defer func() {
		utils.DockerCleanup(node1, "myContainer5")
	}()

	ipAddress = utils.GetIpAddress(t, node2, "orange-myContainer5")
	utils.DockerCleanup(node2, "myContainer2")
	utils.StartAndPing(t, node2, "myContainer2", ipAddress)
	defer func() {
		utils.DockerCleanup(node2, "myContainer2")
	}()

	utils.DelConfig(t, jsonCfg, vagrant.GetNodes()[0])

	utils.DockerCleanup(node2, "myContainer2")
	utils.StartAndEnsurePingFailure(t, node2, "myContainer2", ipAddress)
	defer func() {
		utils.DockerCleanup(node2, "myContainer2")
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
	utils.DockerCleanup(node2, "myContainer4")
	utils.StartAndPing(t, node2, "myContainer4", ipAddress)
	defer func() {
		utils.DockerCleanup(node2, "myContainer4")
	}()
}

func TestTwoHostsVxlanAddDelNetwork(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, vagrant.GetNodes())
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
            "VXlans"                    : "10001-20000",
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

	utils.StartAndWait(t, node1, "myContainer1")
	defer func() {
		utils.DockerCleanup(node1, "myContainer1")
	}()

	utils.StartAndWait(t, node1, "myContainer3")
	defer func() {
		utils.DockerCleanup(node1, "myContainer3")
	}()

	ipAddress := utils.GetIpAddress(t, node2, "orange-myContainer1")
	utils.StartAndPing(t, node2, "myContainer2", ipAddress)
	defer func() {
		utils.DockerCleanup(node2, "myContainer2")
	}()

	ipAddress = utils.GetIpAddress(t, node2, "purple-myContainer3")
	utils.StartAndPing(t, node2, "myContainer4", ipAddress)
	defer func() {
		utils.DockerCleanup(node2, "myContainer4")
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

	utils.StartAndWait(t, node1, "myContainer5")
	defer func() {
		utils.DockerCleanup(node1, "myContainer5")
	}()

	ipAddress = utils.GetIpAddress(t, node2, "green-myContainer5")
	utils.StartAndPing(t, node2, "myContainer6", ipAddress)
	defer func() {
		utils.DockerCleanup(node2, "myContainer6")
	}()

	utils.DelConfig(t, jsonCfg, vagrant.GetNodes()[0])

	utils.DockerCleanup(node2, "myContainer6")
	utils.StartAndEnsurePingFailure(t, node2, "myContainer6", ipAddress)
	defer func() {
		utils.DockerCleanup(node2, "myContainer6")
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
