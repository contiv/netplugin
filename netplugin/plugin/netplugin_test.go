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

package plugin

import (
	"encoding/json"
	"fmt"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/state"
	"github.com/contiv/netplugin/utils"
	"testing"
)

var fakeStateDriver *state.FakeStateDriver

func initFakeStateDriver(t *testing.T) {
	// init fake state driver
	instInfo := core.InstanceInfo{}
	d, err := utils.NewStateDriver("fakedriver", &instInfo)
	if err != nil {
		t.Fatalf("failed to init statedriver. Error: %s", err)
	}

	fakeStateDriver = d.(*state.FakeStateDriver)
}

func deinitFakeStateDriver() {
	// release fake state driver
	utils.ReleaseStateDriver()
}

func TestNetPluginInit(t *testing.T) {
	// Testing init NetPlugin
	initFakeStateDriver(t)
	defer deinitFakeStateDriver()
	gCfg := mastercfg.GlobConfig{
		FwdMode:   "bridge",
		PvtSubnet: "172.19.0.0/16"}
	gCfg.StateDriver = fakeStateDriver
	gCfg.Write()

	configStr := `{
					"drivers" : {
						"network": "ovs",
						"endpoint": "ovs",
						"state": "fakedriver"
					},
					"plugin-instance": {
						"host-label": "testHost",
						"fwd-mode":"bridge"
					}
				}`

	// Parse the config
	pluginConfig := Config{}
	err := json.Unmarshal([]byte(configStr), &pluginConfig)
	if err != nil {
		t.Fatalf("Error parsing config. Err: %v", err)
	}

	fmt.Printf("plugin config: %+v", pluginConfig)

	plugin := NetPlugin{}
	err = plugin.Init(pluginConfig)
	if err != nil {
		t.Fatalf("plugin init failed: Error: %s", err)
	}
	defer func() { plugin.Deinit() }()
}

func TestNetPluginInitInvalidConfigEmptyString(t *testing.T) {
	// Test NetPlugin init failure when no config provided
	pluginConfig := Config{}

	plugin := NetPlugin{}
	err := plugin.Init(pluginConfig)
	if err == nil {
		t.Fatalf("plugin init succeeded, should have failed!")
	}
}

func TestNetPluginInitInvalidConfigMissingInstance(t *testing.T) {
	// Test NetPlugin init failure when missing instance config
	configStr := `{
					"drivers" : {
						"network": "ovs",
						"endpoint": "ovs",
						"state": "fakedriver"
					}
				}`

	// Parse the config
	pluginConfig := Config{}
	err := json.Unmarshal([]byte(configStr), &pluginConfig)

	plugin := NetPlugin{}
	err = plugin.Init(pluginConfig)
	if err == nil {
		t.Fatalf("plugin init succeeded, should have failed!")
	}
}

func TestNetPluginInitInvalidConfigEmptyHostLabel(t *testing.T) {
	// Test NetPlugin init failure when empty HostLabel provided
	configStr := `{
					"drivers" : {
						"network": "ovs",
						"endpoint": "ovs",
						"state": "fakedriver"
					},
					"plugin-instance": {
						"host-label": "",
						"fwd-mode":"bridge",
						"db-url": "etcd://127.0.0.1:4001"
					}
				}`

	// Parse the config
	pluginConfig := Config{}
	err := json.Unmarshal([]byte(configStr), &pluginConfig)
	if err != nil {
		t.Fatalf("Error parsing config. Err: %v", err)
	}

	plugin := NetPlugin{}
	err = plugin.Init(pluginConfig)
	if err == nil {
		t.Fatalf("plugin init succeeded, should have failed!")
	}
}

func TestNetPluginInitInvalidConfigMissingStateDriverName(t *testing.T) {
	// Test NetPlugin init failure when missing state driver name
	configStr := `{
					"drivers" : {
						"network": "ovs",
						"endpoint": "ovs"
					},
					"plugin-instance": {
						"host-label": "testHost",
						"fwd-mode":"bridge",
						"db-url": "etcd://127.0.0.1:4001"
					}
				}`

	// Parse the config
	pluginConfig := Config{}
	err := json.Unmarshal([]byte(configStr), &pluginConfig)
	if err != nil {
		t.Fatalf("Error parsing config. Err: %v", err)
	}

	plugin := NetPlugin{}
	err = plugin.Init(pluginConfig)
	if err == nil {
		t.Fatalf("plugin init succeeded, should have failed!")
	}
}

func TestNetPluginInitInvalidConfigMissingStateDriverURL(t *testing.T) {
	// Test NetPlugin init failure when missing state driver url
	configStr := `{
					"drivers" : {
						"network": "ovs",
						"endpoint": "ovs",
						"state": "etcd"
					},
					"plugin-instance": {
						"host-label": "testHost",
						"fwd-mode":"bridge"
					}
				}`

	// Parse the config
	pluginConfig := Config{}
	err := json.Unmarshal([]byte(configStr), &pluginConfig)
	if err != nil {
		t.Fatalf("Error parsing config. Err: %v", err)
	}

	plugin := NetPlugin{}
	err = plugin.Init(pluginConfig)
	if err == nil {
		t.Fatalf("plugin init succeeded, should have failed!")
	}
	defer func() { plugin.Deinit() }()
}

func TestNetPluginInitInvalidConfigMissingNetworkDriverName(t *testing.T) {
	// Test NetPlugin init failure when missing network driver name
	initFakeStateDriver(t)
	defer deinitFakeStateDriver()
	gCfg := mastercfg.GlobConfig{
		FwdMode:   "bridge",
		PvtSubnet: "172.19.0.0/16"}
	gCfg.StateDriver = fakeStateDriver
	gCfg.Write()
	configStr := `{
					"drivers" : {
						"endpoint": "ovs",
						"state": "fakedriver",
						"container": "docker"
					},
					"plugin-instance": {
						"host-label": "testHost",
						"fwd-mode":"bridge",
						"db-url": "etcd://127.0.0.1:4001"
					}
				}`

	// Parse the config
	pluginConfig := Config{}
	err := json.Unmarshal([]byte(configStr), &pluginConfig)
	if err != nil {
		t.Fatalf("Error parsing config. Err: %v", err)
	}

	plugin := NetPlugin{}
	err = plugin.Init(pluginConfig)
	if err == nil {
		t.Fatalf("plugin init succeeded, should have failed!")
	}
}

func TestNetPluginInitInvalidConfigInvalidPrivateSubnet(t *testing.T) {
	// Test NetPlugin init failure when private subnet is not valid
	initFakeStateDriver(t)
	defer deinitFakeStateDriver()
	gCfg := mastercfg.GlobConfig{
		FwdMode:   "routing",
		PvtSubnet: "172.19.0.0"}
	gCfg.StateDriver = fakeStateDriver
	gCfg.Write()
	configStr := `{
					"drivers" : {
						"network": "ovs",
						"endpoint": "ovs",
						"state": "fakedriver",
						"container": "docker",
					},
					"plugin-instance": {
						"host-label": "testHost",
						"db-url": "etcd://127.0.0.1:4001",
						"fwd-mode":"routing",
					}
				}`

	// Parse the config
	pluginConfig := Config{}
	err := json.Unmarshal([]byte(configStr), &pluginConfig)

	plugin := NetPlugin{}
	err = plugin.Init(pluginConfig)
	if err == nil {
		t.Fatalf("plugin init succeeded, should have failed!")
	}
}
