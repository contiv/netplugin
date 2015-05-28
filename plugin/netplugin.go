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
	"reflect"
	"sync"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/state"
)

// implements the generic Plugin interface

type driverConfigTypes struct {
	DriverType reflect.Type
	ConfigType reflect.Type
}

var networkDriverRegistry = map[string]driverConfigTypes{
	"ovs": driverConfigTypes{
		DriverType: reflect.TypeOf(drivers.OvsDriver{}),
		ConfigType: reflect.TypeOf(drivers.OvsDriverConfig{}),
	},
}

var endpointDriverRegistry = map[string]driverConfigTypes{
	"ovs": driverConfigTypes{
		DriverType: reflect.TypeOf(drivers.OvsDriver{}),
		ConfigType: reflect.TypeOf(drivers.OvsDriverConfig{}),
	},
}

var stateDriverRegistry = map[string]driverConfigTypes{
	"etcd": driverConfigTypes{
		DriverType: reflect.TypeOf(state.EtcdStateDriver{}),
		ConfigType: reflect.TypeOf(state.EtcdStateDriverConfig{}),
	},
}

type config struct {
	Drivers struct {
		Network  string `json:"network"`
		Endpoint string `json:"endpoint"`
		State    string `json:"state"`
	}
	Instance core.InstanceInfo `json:"plugin-instance"`
}

// NetPlugin is the configuration struct for the plugin bus. Network and
// Endpoint drivers are all present in `drivers/` and state drivers are present
// in `state/`.
type NetPlugin struct {
	sync.Mutex
	ConfigFile     string
	NetworkDriver  core.NetworkDriver
	EndpointDriver core.EndpointDriver
	StateDriver    core.StateDriver
}

// initHelper initializes the NetPlugin by mapping driver names to
// configuration, then it imports the configuration.
func (p *NetPlugin) initHelper(driverRegistry map[string]driverConfigTypes,
	driverName string, configStr string) (core.Driver, *core.Config, error) {
	if _, ok := driverRegistry[driverName]; ok {
		configType := driverRegistry[driverName].ConfigType
		driverType := driverRegistry[driverName].DriverType

		driverConfig := reflect.New(configType).Interface()
		err := json.Unmarshal([]byte(configStr), driverConfig)
		if err != nil {
			return nil, nil, err
		}

		config := &core.Config{V: driverConfig}
		driver := reflect.New(driverType).Interface()
		return driver, config, nil
	}

	return nil, nil, core.Errorf("Failed to find a registered driver for: %s", driverName)
}

// Init initializes the NetPlugin instance via the configuration string passed.
func (p *NetPlugin) Init(configStr string) error {
	if configStr == "" {
		return core.Errorf("empty config passed")
	}

	var driver core.Driver
	drvConfig := &core.Config{}
	pluginConfig := &config{}
	err := json.Unmarshal([]byte(configStr), pluginConfig)
	if err != nil {
		return err
	}

	if pluginConfig.Instance.HostLabel == "" {
		return core.Errorf("empty host-label passed")
	}

	// initialize state driver
	driver, drvConfig, err = p.initHelper(stateDriverRegistry,
		pluginConfig.Drivers.State, configStr)
	if err != nil {
		return err
	}
	p.StateDriver = driver.(core.StateDriver)
	err = p.StateDriver.Init(drvConfig)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			p.StateDriver.Deinit()
		}
	}()

	instanceInfo := &core.InstanceInfo{
		HostLabel:   pluginConfig.Instance.HostLabel,
		StateDriver: p.StateDriver}
	// initialize network driver
	driver, drvConfig, err = p.initHelper(networkDriverRegistry,
		pluginConfig.Drivers.Network, configStr)
	if err != nil {
		return err
	}
	p.NetworkDriver = driver.(core.NetworkDriver)
	err = p.NetworkDriver.Init(drvConfig, instanceInfo)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			p.NetworkDriver.Deinit()
		}
	}()

	// initialize endpoint driver
	driver, drvConfig, err = p.initHelper(endpointDriverRegistry,
		pluginConfig.Drivers.Endpoint, configStr)
	if err != nil {
		return err
	}
	p.EndpointDriver = driver.(core.EndpointDriver)
	err = p.EndpointDriver.Init(drvConfig, instanceInfo)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			p.EndpointDriver.Deinit()
		}
	}()

	return nil
}

// Deinit is a destructor for the NetPlugin configuration.
func (p *NetPlugin) Deinit() {
	if p.EndpointDriver != nil {
		p.EndpointDriver.Deinit()
	}
	if p.NetworkDriver != nil {
		p.NetworkDriver.Deinit()
	}
	if p.StateDriver != nil {
		p.StateDriver.Deinit()
	}
}

// CreateNetwork creates a network for a given ID.
func (p *NetPlugin) CreateNetwork(id string) error {
	return p.NetworkDriver.CreateNetwork(id)
}

// DeleteNetwork deletes a network provided by the ID.
func (p *NetPlugin) DeleteNetwork(id string) error {
	return p.NetworkDriver.DeleteNetwork(id)
}

// FetchNetwork retrieves a network's state given an ID.
func (p *NetPlugin) FetchNetwork(id string) (core.State, error) {
	return nil, core.Errorf("Not implemented")
}

// CreateEndpoint creates an endpoint for a given ID.
func (p *NetPlugin) CreateEndpoint(id string) error {
	return p.EndpointDriver.CreateEndpoint(id)
}

// DeleteEndpoint destroys an endpoint for an ID.
func (p *NetPlugin) DeleteEndpoint(id string) error {
	return p.EndpointDriver.DeleteEndpoint(id)
}

// FetchEndpoint retrieves an endpoint's state for a given ID
func (p *NetPlugin) FetchEndpoint(id string) (core.State, error) {
	return nil, core.Errorf("Not implemented")
}
