package core

import (
	"code.google.com/p/gcfg"
	"reflect"
)

// implements the generic Plugin interface

type DriverConfigTypes struct {
	DriverType reflect.Type
	ConfigType reflect.Type
}

var NetworkDriverRegistry = map[string]DriverConfigTypes{
	"ovs": DriverConfigTypes{
		DriverType: OvsDriver,
		ConfigType: OvsDriverConfig,
	},
}

var EndpointDriverRegistry = map[string]DriverConfigTypes{
	"ovs": DriverConfigTypes{
		DriverType: OvsDriver,
		ConfigType: OvsDriverConfig,
	},
}

var StateDriverRegistry = map[string]DriverConfigTypes{
	"etcd": DriverConfigTypes{
		DriverType: EtcdStateDriver,
		ConfigType: EtcdStateDriverConfig,
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
	configFile     string
	networkDriver  *NetworkDriver
	endpointDriver *EndpointDriver
	stateDriver    *StateDriver
}

func (p *NetPlugin) initDriver(driverRegistry map[string]DriverConfigTypes,
	name string) (*interface{}, *Config, error) {
	if str, ok := driverRegistry[name]; ok {
		configType := driverRegistry[name].ConfigType
		driverType := driverRegistry[name].DriverType

		driverConfig := reflect.New(configType)
		err := gcfg.ReadFileInto(driverConfig, p.configFile)
		if err != nil {
			return err
		}

		config = Config{v: driverConfig}
		driver = reflect.New(driverType)
		return driver, &config, nil
	} else {
		return nil, nil,
			error{fmt.Sprintf("Failed to find a registered driver for: ", name)}
	}

}

func (p *NetPlugin) Init(config *Config) {
	pluginConfig = *PluginConfig(config.v)
	config := nil

	// initialize state driver
	driver, config, err := initDriver(StateDriverRegistry,
		pluginConfig.Drivers.State)
	if err != nil {
		return err
	}
	err = p.StateDriver.Init(config)
	if err != nil {
		return err
	}

	// initialize network driver
	driver, config, err := initDriver(NetworkDriverRegistry,
		pluginConfig.Drivers.Network)
	if err != nil {
		return err
	}
	err = p.NetworkDriver.Init(config, p.stateDriver)
	if err != nil {
		return err
	}

	// initialize endpoint driver
	driver, config, err := initDriver(EndpointDriverRegistry,
		pluginConfig.Drivers.Endpoint)
	if err != nil {
		return err
	}
	err = p.EndpointDriver.Init(config, p.stateDriver)
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

func (p *NetPlugin) FetchNetwork(id string) (*State, error) {
}

func (p *NetPlugin) CreateEndpoint(id string) error {
	return p.EndpointDriver.CreateEndpoint(id)
}

func (p *Netplugin) DeleteEndpoint(id string) error {
	return p.EndpointDriver.DeleteEndpoint(id)
}

func (p *NetPlugin) FetchEndpoint(id string) (*State, error) {
}
