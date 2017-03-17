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

package master

import (
	"encoding/json"
	"strings"
	"testing"

	log "github.com/Sirupsen/logrus"

	"fmt"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/gstate"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/netmaster/resources"
	"github.com/contiv/netplugin/state"
	"github.com/contiv/netplugin/utils"
)

var fakeDriver *state.FakeStateDriver

func applyConfig(t *testing.T, cfgBytes []byte) {
	cfg := &intent.Config{}
	err := json.Unmarshal(cfgBytes, cfg)
	if err != nil {
		t.Fatalf("error '%s' parsing config '%s'\n", err, cfgBytes)
	}

	_, err = resources.NewStateResourceManager(fakeDriver)
	if err != nil {
		log.Fatalf("state store initialization failed. Error: %s", err)
	}
	defer func() { resources.ReleaseStateResourceManager() }()

	// setup global state
	gCfg := &gstate.Cfg{}
	gCfg.StateDriver = fakeDriver
	gCfg.Auto.VLANs = "1-4094"
	gCfg.Auto.VXLANs = "1-10000"

	// setup resources
	err = gCfg.Process("vxlan")
	if err != nil {
		t.Fatalf("Error updating the config %+v. Error: %s", gCfg, err)
	}
	err = gCfg.Process("vlan")
	if err != nil {
		t.Fatalf("Error updating the config %+v. Error: %s", gCfg, err)
	}
	err = gCfg.Write()
	if err != nil {
		t.Fatalf("error updating global config.Error: %s", err)
	}

	for _, tenant := range cfg.Tenants {
		err = CreateTenant(fakeDriver, &tenant)
		if err != nil {
			t.Fatalf("error '%s' creating tenant\n", err)
		}

		err = CreateNetworks(fakeDriver, &tenant)
		if err != nil {
			t.Fatalf("error '%s' creating networks\n", err)
		}

		err = CreateEndpoints(fakeDriver, &tenant)
		if err != nil {
			t.Fatalf("error '%s' creating endpoints\n", err)
		}
	}

	fakeDriver.DumpState()
}

func verifyKeys(t *testing.T, keys []string) {

	for _, key := range keys {
		found := false
		for stateKey := range fakeDriver.TestState {
			if found = strings.Contains(stateKey, key); found {
				break
			}
		}
		if !found {
			t.Fatalf("key '%s' was not populated in db", key)
		}
	}
}

func verifyKeysDoNotExist(t *testing.T, keys []string) {

	for _, key := range keys {
		found := false
		for stateKey := range fakeDriver.TestState {
			if found = strings.Contains(stateKey, key); found {
				t.Fatalf("key '%s' was populated in db", key)
			}
		}
	}
}

func initFakeStateDriver(t *testing.T) {
	instInfo := core.InstanceInfo{}
	d, err := utils.NewStateDriver("fakedriver", &instInfo)
	if err != nil {
		t.Fatalf("failed to init statedriver. Error: %s", err)
	}

	fakeDriver = d.(*state.FakeStateDriver)
}

func deinitFakeStateDriver() {
	utils.ReleaseStateDriver()
}

func TestVlanConfig(t *testing.T) {
	cfgBytes := []byte(`{
    "Tenants" : [{
        "Name"                      : "tenant-one",
        "Networks"  : [{
            "Name"                  : "orange",
			"SubnetCIDR"			: "10.1.1.1/24",
			"Gateway"				: "10.1.1.254",
            "Endpoints" : [{
                "Container"         : "myContainer1"
            },
            {
                "Container"         : "myContainer2"
            }]
        },
        {
            "Name"                  : "purple",
			"SubnetCIDR"			: "10.1.2.1/24",
			"Gateway"				: "10.1.2.254",
            "Endpoints" : [{
                "Container"         : "myContainer3"
            },
            {
                "Container"         : "myContainer4"
            }]
        }]
    }]}`)

	initFakeStateDriver(t)
	defer deinitFakeStateDriver()

	applyConfig(t, cfgBytes)

	keys := []string{"tenant-one", "orange", "myContainer1", "myContainer2",
		"purple", "myContainer3", "myContainer4"}

	verifyKeys(t, keys)
}

func TestVlanWithUnderlayConfig(t *testing.T) {
	cfgBytes := []byte(`{
    "Tenants" : [{
        "Name"                      : "tenant-one",
        "Networks"  : [{
            "Name"                : "orange",
			"SubnetCIDR"			: "10.1.1.1/24",
			"Gateway"				: "10.1.1.254",
            "Endpoints" : [{
                "Container"       : "myContainer1",
                "Host"            : "host1"
            },
            {
                "Container"       : "myContainer3",
                "Host"            : "host2"
            }]
        },
        {
            "Name"                : "purple",
			"SubnetCIDR"			: "10.1.2.1/24",
			"Gateway"				: "10.1.2.254",
            "Endpoints" : [{
                "Container"       : "myContainer2",
                "Host"            : "host1"
            },
            {
                "Container"       : "myContainer4",
                "Host"            : "host2"
            }]
        }
        ]
    }]}`)

	initFakeStateDriver(t)
	defer deinitFakeStateDriver()

	applyConfig(t, cfgBytes)

	keys := []string{"tenant-one", "nets/orange", "nets/purple",
		"myContainer1", "myContainer2",
		"myContainer3", "myContainer4"}

	verifyKeys(t, keys)
}

func TestVxlanConfig(t *testing.T) {
	cfgBytes := []byte(`{
    "Tenants" : [{
        "Name"                  : "tenant-one",
        "Networks"  : [{
            "Name"              : "orange",
			"SubnetCIDR"			: "10.1.1.1/24",
			"Gateway"				: "10.1.1.254",
            "Endpoints" : [
            {
                "Container"     : "myContainer1",
                "Host"          : "host1"
            },
            {
                "Container"     : "myContainer3",
                "Host"          : "host2"
            }
            ]
        },
        {
            "Name"              : "purple",
			"SubnetCIDR"			: "10.1.2.1/24",
			"Gateway"				: "10.1.2.254",
            "Endpoints" : [{
                "Container"     : "myContainer2",
                "Host"          : "host1"
            },
            {
                "Container"     : "myContainer4",
                "Host"          : "host2"
            }]
        }]
    }]}`)

	initFakeStateDriver(t)
	defer deinitFakeStateDriver()

	applyConfig(t, cfgBytes)

	keys := []string{"tenant-one", "nets/orange", "nets/purple",
		"myContainer1", "myContainer2",
		"myContainer3", "myContainer4"}

	verifyKeys(t, keys)
}

func TestVxlanConfigWithLateHostBindings(t *testing.T) {
	cfgBytes := []byte(`{
    "Tenants" : [{
        "Name"                  : "tenant-one",
        "Networks"  : [{
            "Name"              : "orange",
			"SubnetCIDR"			: "10.1.1.1/24",
			"Gateway"				: "10.1.1.254",
            "Endpoints" : [
            {
                "Container"     : "myContainer1"
            },
            {
                "Container"     : "myContainer3"
            }
            ]
        },
        {
            "Name"              : "purple",
			"SubnetCIDR"			: "10.1.2.1/24",
			"Gateway"				: "10.1.2.254",
            "Endpoints" : [{
                "Container"     : "myContainer2"
            },
            {
                "Container"     : "myContainer4"
            }]
        }]
    }]}`)

	initFakeStateDriver(t)
	defer deinitFakeStateDriver()

	applyConfig(t, cfgBytes)
	fakeDriver.DumpState()

	keys := []string{"tenant-one", "nets/orange", "nets/purple"}

	verifyKeys(t, keys)

	epBindings := []intent.ConfigEP{{
		Container: "myContainer1",
		Host:      "host1",
	}, {
		Container: "myContainer2",
		Host:      "host1",
	}, {
		Container: "myContainer3",
		Host:      "host2",
	}, {
		Container: "myContainer4",
		Host:      "host2",
	}}

	err := CreateEpBindings(&epBindings)
	if err != nil {
		t.Fatalf("error '%s' creating tenant\n", err)
	}

	keys = []string{"tenant-one", "nets/orange", "nets/purple",
		"myContainer1", "myContainer2",
		"myContainer3", "myContainer4"}

	fakeDriver.DumpState()

	verifyKeys(t, keys)
}

// Tests for https://github.com/contiv/netplugin/issues/214
func TestConfigPktTagOutOfRange(t *testing.T) {
	CfgBytes := []byte(`{
    "Tenants" : [{
        "Name"                  : "tenant1",
        "Networks"  : [{
            "Name"              : "net1",
			"SubnetCIDR"		: "10.1.1.1/24",
			"Gateway"			: "10.1.1.254",
            "PktTag"            : 2001,
            "PktTagType"        : "vxlan"
        }]
    }]}`)
	applyVerifyRangeTag(t, CfgBytes, true)

	CfgBytes = []byte(`{
    "Tenants" : [{
        "Name"                  : "tenant2",
        "Networks"  : [{
            "Name"              : "net2",
			"SubnetCIDR"		: "10.1.1.1/24",
			"Gateway"			: "10.1.1.254",
            "PktTag"            : 2000,
            "PktTagType"        : "vxlan"
        }]
    }]}`)
	applyVerifyRangeTag(t, CfgBytes, false)

	CfgBytes = []byte(`{
    "Tenants" : [{
        "Name"                  : "tenant4",
        "Networks"  : [{
            "Name"              : "net4",
			"SubnetCIDR"		: "10.1.1.1/24",
			"Gateway"			: "10.1.1.254",
            "PktTag"            : 3001,
            "PktTagType"        : "vxlan"
        }]
    }]}`)
	applyVerifyRangeTag(t, CfgBytes, true)

	CfgBytes = []byte(`{
    "Tenants" : [{
        "Name"                  : "tenant5",
        "Networks"  : [{
            "Name"              : "net5",
			"SubnetCIDR"		: "10.1.1.1/24",
			"Gateway"			: "10.1.1.254",
            "PktTag"            : 1200,
            "PktTagType"        : "vlan"
        }]
    }]}`)
	applyVerifyRangeTag(t, CfgBytes, true)

	CfgBytes = []byte(`{
    "Tenants" : [{
        "Name"                  : "tenant6",
        "Networks"  : [{
            "Name"              : "net6",
			"SubnetCIDR"		: "10.1.1.1/24",
			"Gateway"			: "10.1.1.254",
            "PktTag"            : 12,
            "PktTagType"        : "vlan"
        }]
    }]}`)
	applyVerifyRangeTag(t, CfgBytes, false)

	CfgBytes = []byte(`{
    "Tenants" : [{
        "Name"                  : "tenant8",
        "Networks"  : [{
            "Name"              : "net8",
			"SubnetCIDR"		: "10.1.1.1/24",
			"Gateway"			: "10.1.1.254",
            "PktTag"            : 1501,
            "PktTagType"        : "vlan"
        }]
    }]}`)
	applyVerifyRangeTag(t, CfgBytes, true)

}

func applyVerifyRangeTag(t *testing.T, cfgBytes []byte, shouldFail bool) {
	initFakeStateDriver(t)
	defer deinitFakeStateDriver()

	cfg := &intent.Config{}
	err := json.Unmarshal(cfgBytes, cfg)
	if err != nil {
		t.Fatalf("error '%s' parsing config '%s'\n", err, cfgBytes)
	}

	_, err = resources.NewStateResourceManager(fakeDriver)
	if err != nil {
		log.Fatalf("state store initialization failed. Error: %s", err)
	}
	defer func() { resources.ReleaseStateResourceManager() }()

	// setup global state
	gCfg := &gstate.Cfg{}
	gCfg.StateDriver = fakeDriver
	gCfg.Auto.VLANs = "11-1000"
	gCfg.Auto.VXLANs = "1001-2000"

	// setup resources
	err = gCfg.Process("vlan")
	if err != nil {
		t.Fatalf("Error updating the config %+v. Error: %s", gCfg, err)
	}
	err = gCfg.Process("vxlan")
	if err != nil {
		t.Fatalf("Error updating the config %+v. Error: %s", gCfg, err)
	}

	err = gCfg.Write()
	if err != nil {
		t.Fatalf("error updating global config.Error: %s", err)
	}

	tenant := cfg.Tenants[0]
	err = CreateTenant(fakeDriver, &tenant)
	if err != nil {
		t.Fatalf("error '%s' creating tenant\n", err)
	}

	err = CreateNetworks(fakeDriver, &tenant)
	if shouldFail {

		var expError string
		network := tenant.Networks[0]
		if network.PktTagType == "vlan" {
			expError = "requested vlan not available"
		} else {
			expError = "requested vxlan not available"
		}

		if err == nil {
			t.Fatalf("CreateNetworks did not return error\n")
		} else if !strings.Contains(err.Error(), expError) {
			t.Fatalf("CreateNetworks did not return error for OutOfRange\n")
		}
	} else if err != nil {
		t.Fatalf("error '%s' while creating network\n", err)
	}

}

// TestNetworkDeleteWithEPs
// This test creates a network and adds endpoints to it.
// It then tries to delete the network, while it has
// active endpoints and expects them to fail
func TestNetworkDeleteWithEPs(t *testing.T) {
	cfgBytes := []byte(`{
    "Tenants" : [{
        "Name"                  : "tenant-one",
        "Networks"  : [{
            "Name"              : "orange",
            "SubnetCIDR"        : "10.1.1.1/24",
            "Gateway"           : "10.1.1.254",
            "Endpoints" : [
            {
                "Container"     : "myContainer1"
            },
            {
                "Container"     : "myContainer3"
            }
            ]
        }]
    }]}`)

	initFakeStateDriver(t)
	defer deinitFakeStateDriver()

	applyConfig(t, cfgBytes)

	epBindings := []intent.ConfigEP{{
		Container: "myContainer1",
		Host:      "host1",
	}, {
		Container: "myContainer2",
		Host:      "host1",
	}, {
		Container: "myContainer3",
		Host:      "host2",
	}, {
		Container: "myContainer4",
		Host:      "host2",
	}}

	err := CreateEpBindings(&epBindings)
	if err != nil {
		t.Fatalf("error '%s' creating tenant\n", err)
	}

	cfg := &intent.Config{}
	err = json.Unmarshal(cfgBytes, cfg)
	if err != nil {
		t.Fatalf("error '%s' parsing config '%s'\n", err, cfgBytes)
	}

	_, err = resources.NewStateResourceManager(fakeDriver)
	if err != nil {
		log.Fatalf("state store initialization failed. Error: %s", err)
	}
	defer func() { resources.ReleaseStateResourceManager() }()

	for _, tenant := range cfg.Tenants {
		err = DeleteNetworks(fakeDriver, &tenant)
		if err == nil || (err != nil && !strings.Contains(err.Error(), "Error: Network has active endpoints")) {
			t.Fatalf("Network delete did not Fail with Active EPs.")
		}
	}
}

func TestGetAllocatedIPs(t *testing.T) {
	cfgBytes := []byte(`{
    "Tenants" : [{
        "Name"                      : "teaone",
        "Networks"  : [{
            "Name"                : "orange",
			"SubnetCIDR"			: "10.1.1.0/24",
			"Gateway"				: "10.1.1.254",
            "Endpoints" : [{
                "Container"       : "myContainer1"
            },
			{
                "Container"       : "myContainer2"
            },
            {
                "Container"       : "myContainer3"
            }]
        },
		{
            "Name"                : "blue",
			"SubnetCIDR"			: "10.1.1.0/24",
            "Endpoints" : [{
                "Container"       : "myContainer1"
            },
			{
                "Container"       : "myContainer2"
            },
			{
                "Container"       : "myContainer3"
            },
            {
                "Container"       : "myContainer4"
            }]
		},
		{
            "Name"                : "green",
			"SubnetCIDR"			: "10.1.1.0/24",
			"Gateway"				: "10.1.1.254"
        },
		{
            "Name"                : "red",
			"SubnetCIDR"			: "10.1.1.0/24"
        },
		{
            "Name"                : "yellow",
			"SubnetCIDR"			: "10.1.1.0/24",
			"Gateway"				: "10.1.1.254",
            "Endpoints" : [{
                "Container"       : "myContainer1"
            },
			{
                "Container"       : "myContainer2"
            },
			{
                "Container"       : "myContainer3"
            },
			{
                "Container"       : "myContainer4"
            },
			{
                "Container"       : "myContainer5"
            },
			{
                "Container"       : "myContainer6"
            },
            {
                "Container"       : "myContainer7"
            }]
		}]
    }]}`)
	initFakeStateDriver(t)
	defer deinitFakeStateDriver()

	applyConfig(t, cfgBytes)
	nwCfg := &mastercfg.CfgNetworkState{}
	nwCfg.StateDriver = fakeDriver

	// default-gw with endpoints
	expectedAllocedIPs := "10.1.1.1-10.1.1.3, 10.1.1.254"
	networkID := "orange.teaone"
	if err := nwCfg.Read(networkID); err != nil {
		log.Fatalf("unable to locate network: %s", networkID)
	}

	networks := ListAllocatedIPs(nwCfg)
	if networks != expectedAllocedIPs {
		log.Fatalf("got networks '%s' expected '%s'", networks, expectedAllocedIPs)
	}

	// endpoints without default-gw
	expectedAllocedIPs = "10.1.1.1-10.1.1.4"
	networkID = "blue.teaone"
	if err := nwCfg.Read(networkID); err != nil {
		log.Fatalf("unable to locate network: %s", networkID)
	}

	networks = ListAllocatedIPs(nwCfg)
	if networks != expectedAllocedIPs {
		log.Fatalf("got networks '%s' expected '%s'", networks, expectedAllocedIPs)
	}

	// default-gw without endpoints
	expectedAllocedIPs = "10.1.1.254"
	networkID = "green.teaone"
	if err := nwCfg.Read(networkID); err != nil {
		log.Fatalf("unable to locate network: %s", networkID)
	}

	networks = ListAllocatedIPs(nwCfg)
	if networks != expectedAllocedIPs {
		log.Fatalf("got networks '%s' expected '%s'", networks, expectedAllocedIPs)
	}

	// no default-gw, no endpionts
	expectedAllocedIPs = ""
	networkID = "red.teaone"
	if err := nwCfg.Read(networkID); err != nil {
		log.Fatalf("unable to locate network: %s", networkID)
	}

	networks = ListAllocatedIPs(nwCfg)
	if networks != expectedAllocedIPs {
		log.Fatalf("got networks '%s' expected '%s'", networks, expectedAllocedIPs)
	}

	// default-gw, scattered endpionts, multiple ranges
	networkID = "yellow.teaone"
	epID := getEpName(networkID, &intent.ConfigEP{Container: "myContainer3"})
	if _, err := DeleteEndpointID(fakeDriver, epID); err != nil {
		log.Errorf("error deleting endpoint, %s", err)
	}
	epID = getEpName(networkID, &intent.ConfigEP{Container: "myContainer4"})
	if _, err := DeleteEndpointID(fakeDriver, epID); err != nil {
		log.Errorf("error deleting endpoint, %s", err)
	}
	epID = getEpName(networkID, &intent.ConfigEP{Container: "myContainer6"})
	if _, err := DeleteEndpointID(fakeDriver, epID); err != nil {
		log.Errorf("error deleting endpoint, %s", err)
	}
	expectedAllocedIPs = "10.1.1.1-10.1.1.2, 10.1.1.5, 10.1.1.7, 10.1.1.254"
	if err := nwCfg.Read(networkID); err != nil {
		log.Fatalf("unable to locate network: %s", networkID)
	}

	networks = ListAllocatedIPs(nwCfg)
	if networks != expectedAllocedIPs {
		log.Fatalf("got networks '%s' expected '%s'", networks, expectedAllocedIPs)
	}
}

func assertOnTrue(t *testing.T, c bool, msg string) {
	if c {
		t.Fatalf("%s", msg)
	}
}

func TestGetNwAndEpgFromAddrReq(t *testing.T) {
	testData := []struct {
		allocID string
		nwName  string
		epgName string
	}{
		{"abc:xyz.def", "abc.def", "xyz:def"},
		{"net1:epg1.default", "net1.default", "epg1:default"},
		{"net1.default", "net1.default", ""},
		{"abcd", "abcd", ""},
		{"abcd.1234.12", "abcd.1234.12", ""},
		{"abcd:xyz.zyz.abc", "abcd:xyz.zyz.abc", ""},
	}

	for _, d := range testData {
		n, e := getNwAndEpgFromAddrReq(d.allocID)
		assertOnTrue(t, n != d.nwName, fmt.Sprintf("networkname mismatch [%s] != [%s]", n, d.nwName))
		assertOnTrue(t, e != d.epgName, fmt.Sprintf("epgname mismatch [%s] != [%s]", e, d.epgName))
	}
}
