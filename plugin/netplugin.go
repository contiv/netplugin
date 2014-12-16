package plugin

import (
	"code.google.com/p/gcfg"
	"fmt"
	"netplugin/core"
	"netplugin/drivers"
	"reflect"
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

func (p *NetPlugin) initDriver(driverRegistry map[string]DriverConfigTypes,
	name string) (core.Driver, *core.Config, error) {
	if _, ok := driverRegistry[name]; ok {
		configType := driverRegistry[name].ConfigType
		driverType := driverRegistry[name].DriverType

		driverConfig := reflect.New(configType)
		err := gcfg.ReadFileInto(driverConfig, p.ConfigFile)
		if err != nil {
			return nil, nil, err
		}

		config := core.Config{V: driverConfig}
		driver := reflect.New(driverType)
		return driver, &config, nil
	} else {
		return nil, nil,
			&core.Error{Desc: fmt.Sprintf("Failed to find a registered driver for: %s", name)}
	}

}

func (p *NetPlugin) Init(config *core.Config) error {
	pluginConfig := config.V.(PluginConfig)

	// initialize state driver
	driver, drvConfig, err := p.initDriver(StateDriverRegistry,
		pluginConfig.Drivers.State)
	if err != nil {
		return err
	}
	p.StateDriver = driver.(core.StateDriver)
	err = p.StateDriver.Init(drvConfig)
	if err != nil {
		return err
	}

	// initialize network driver
	driver, drvConfig, err = p.initDriver(NetworkDriverRegistry,
		pluginConfig.Drivers.Network)
	if err != nil {
		return err
	}
	p.NetworkDriver = driver.(core.NetworkDriver)
	err = p.NetworkDriver.Init(drvConfig, p.StateDriver)
	if err != nil {
		return err
	}

	// initialize endpoint driver
	driver, drvConfig, err = p.initDriver(EndpointDriverRegistry,
		pluginConfig.Drivers.Endpoint)
	if err != nil {
		return err
	}
	p.EndpointDriver = driver.(core.EndpointDriver)
	err = p.EndpointDriver.Init(drvConfig, p.StateDriver)
	if err != nil {
		return err
	}

	return nil
}

func (p *NetPlugin) Deinit() {
}

func (p *NetPlugin) CreateNetwork(id string) error {
	return p.NetworkDriver.CreateNetwork(id)
}

func (p *NetPlugin) DeleteNetwork(id string) error {
	return p.NetworkDriver.DeleteNetwork(id)
}

func (p *NetPlugin) FetchNetwork(id string) (core.State, error) {
	return nil, &core.Error{Desc: "Not implemented"}
}

func (p *NetPlugin) CreateEndpoint(id string) error {
	return p.EndpointDriver.CreateEndpoint(id)
}

func (p *NetPlugin) DeleteEndpoint(id string) error {
	return p.EndpointDriver.DeleteEndpoint(id)
}

func (p *NetPlugin) FetchEndpoint(id string) (core.State, error) {
	return nil, &core.Error{Desc: "Not implemented"}
}
