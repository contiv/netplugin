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
	"testing"

	"github.com/contiv/netplugin/state"
)

var fakeStateDriver *state.FakeStateDriver

func TestNetPluginInit(t *testing.T) {
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
	pluginConfig := Config{}

	plugin := NetPlugin{}
	err := plugin.Init(pluginConfig)
	if err == nil {
		t.Fatalf("plugin init succeeded, should have failed!")
	}
}

func TestNetPluginInitInvalidConfigMissingInstance(t *testing.T) {
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

func TestNetPluginInitInvalidConfigMissingFwdMode(t *testing.T) {
	configStr := `{
                    "drivers" : {
                       "network": "ovs",
                       "endpoint": "ovs",
                       "state": "fakedriver",
                       "container": "docker",
                    },
					"plugin-instance": {
						 "host-label": "testHost",
						 "db-url": "etcd://127.0.0.1:4001"
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
