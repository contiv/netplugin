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
	"reflect"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/drivers"
)

// implements the generic Plugin interface

type DriverConfigTypes struct {
	DriverType reflect.Type
	ConfigType reflect.Type
}

var NetworkDriverRegistry = map[string]DriverConfigTypes{
	"ovs": DriverConfigTypes{
		DriverType: reflect.TypeOf(drivers.OvsDriver{}),
		ConfigType: reflect.TypeOf(drivers.OvsDriverConfig{}),
	},
}

var EndpointDriverRegistry = map[string]DriverConfigTypes{
	"ovs": DriverConfigTypes{
		DriverType: reflect.TypeOf(drivers.OvsDriver{}),
		ConfigType: reflect.TypeOf(drivers.OvsDriverConfig{}),
	},
}

var StateDriverRegistry = map[string]DriverConfigTypes{
	"etcd": DriverConfigTypes{
		DriverType: reflect.TypeOf(drivers.EtcdStateDriver{}),
		ConfigType: reflect.TypeOf(drivers.EtcdStateDriverConfig{}),
	},
}

type PluginConfig struct {
	Drivers struct {
		Network  string
		Endpoint string
		State    string
	}
}

type NetPlugin struct {
	ConfigFile     string
	NetworkDriver  core.NetworkDriver
	EndpointDriver core.EndpointDriver
	StateDriver    core.StateDriver
}

func (p *NetPlugin) InitHelper(driverRegistry map[string]DriverConfigTypes,
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
	} else {
		return nil, nil,
			&core.Error{Desc: fmt.Sprintf("Failed to find a registered driver for: %s",
				driverName)}
	}

}

func (p *NetPlugin) Init(configStr string) error {
	if configStr == "" {
		return &core.Error{Desc: "empty config passed"}
	}

	var driver core.Driver = nil
	drvConfig := &core.Config{}
	pluginConfig := &PluginConfig{}
	err := json.Unmarshal([]byte(configStr), pluginConfig)
	if err != nil {
		return err
	}

	// initialize state driver
	driver, drvConfig, err = p.InitHelper(StateDriverRegistry,
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

	// initialize network driver
	driver, drvConfig, err = p.InitHelper(NetworkDriverRegistry,
		pluginConfig.Drivers.Network, configStr)
	if err != nil {
		return err
	}
	p.NetworkDriver = driver.(core.NetworkDriver)
	err = p.NetworkDriver.Init(drvConfig, p.StateDriver)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			p.NetworkDriver.Deinit()
		}
	}()

	// initialize endpoint driver
	driver, drvConfig, err = p.InitHelper(EndpointDriverRegistry,
		pluginConfig.Drivers.Endpoint, configStr)
	if err != nil {
		return err
	}
	p.EndpointDriver = driver.(core.EndpointDriver)
	err = p.EndpointDriver.Init(drvConfig, p.StateDriver)
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

func (p *NetPlugin) CreateNetwork(id string) error {
	return p.NetworkDriver.CreateNetwork(id)
}

func (p *NetPlugin) DeleteNetwork(value string) error {
	return p.NetworkDriver.DeleteNetwork(value)
}

func (p *NetPlugin) FetchNetwork(id string) (core.State, error) {
	return nil, &core.Error{Desc: "Not implemented"}
}

func (p *NetPlugin) CreateEndpoint(id string) error {
	return p.EndpointDriver.CreateEndpoint(id)
}

func (p *NetPlugin) DeleteEndpoint(value string) error {
	return p.EndpointDriver.DeleteEndpoint(value)
}

func (p *NetPlugin) UpdateContainerId(id string, contId string) error {
	return p.EndpointDriver.UpdateContainerId(id, contId)
}

func (p *NetPlugin) FetchEndpoint(id string) (core.State, error) {
	return nil, &core.Error{Desc: "Not implemented"}
}
