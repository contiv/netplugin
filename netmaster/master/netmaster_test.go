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
	"log"
	"strings"
	"testing"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/resources"
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
	config := &core.Config{V: &state.FakeStateDriverConfig{}}
	cfgBytes, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshalling configuration failed. Error: %s", err)
	}

	d, err := utils.NewStateDriver("fakedriver", string(cfgBytes))
	if err != nil {
		t.Fatalf("failed to init statedriver. Error: %s", err)
	}

	fakeDriver = d.(*state.FakeStateDriver)
	testMode = true
}

func deinitFakeStateDriver() {
	utils.ReleaseStateDriver()
	testMode = false
}

func TestVlanConfig(t *testing.T) {
	cfgBytes := []byte(`{
    "Tenants" : [{
        "Name"                      : "tenant-one",
        "DefaultNetType"            : "vlan",
        "SubnetPool"                : "11.1.0.0/16",
        "AllocSubnetLen"            : 24,
        "Vlans"                     : "11-28",
        "Networks"  : [{
            "Name"                  : "orange",
            "Endpoints" : [{
                "Container"         : "myContainer1"
            },
            {
                "Container"         : "myContainer2"
            }]
        },
        {
            "Name"                  : "purple",
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
        "DefaultNetType"          : "vlan",
        "SubnetPool"              : "11.1.0.0/16",
        "AllocSubnetLen"          : 24,
        "Vlans"                   : "11-48",
        "Networks"  : [{
            "Name"                : "orange",
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
        "DefaultNetType"        : "vxlan",
        "SubnetPool"            : "11.1.0.0/16",
        "AllocSubnetLen"        : 24,
        "Vxlans"                : "10001-14000",
        "Networks"  : [{
            "Name"              : "orange",
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
        "DefaultNetType"        : "vxlan",
        "SubnetPool"            : "11.1.0.0/16",
        "AllocSubnetLen"        : 24,
        "Vxlans"                : "10001-14000",
        "Networks"  : [{
            "Name"              : "orange",
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
func TestVxlanConfigPktTagOutOfRange(t *testing.T) {
	cfgBytes := []byte(`{
    "Tenants" : [{
        "Name"                  : "tenant-one",
        "DefaultNetType"        : "vxlan",
        "SubnetPool"            : "11.1.0.0/16",
        "AllocSubnetLen"        : 24,
        "Vxlans"                : "2001-3000",
        "Networks"  : [{
            "Name"              : "orange",
            "PktTag"            : 1300
        }]
    }]}`)

	initFakeStateDriver(t)
	defer deinitFakeStateDriver()

	// Create the Tenant and Network
	applyConfig(t, cfgBytes)
	fakeDriver.DumpState()

	keys := []string{"tenant-one"}
	verifyKeys(t, keys)

	keys = []string{"nets/orange"}
	fakeDriver.DumpState()
	verifyKeysDoNotExist(t, keys)
}
