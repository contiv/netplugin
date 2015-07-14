package utils

import (
	"encoding/json"
	"reflect"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/state"
)

// implement utilities for instantiating the supported core.Driver
// (state, network and endpoint) instances

type driverConfigTypes struct {
	DriverType reflect.Type
	ConfigType reflect.Type
}

var networkDriverRegistry = map[string]driverConfigTypes{
	OvsNameStr: driverConfigTypes{
		DriverType: reflect.TypeOf(drivers.OvsDriver{}),
		ConfigType: reflect.TypeOf(drivers.OvsDriverConfig{}),
	},
	// fakedriver is used for tests, so not exposing a public name for it.
	"fakedriver": driverConfigTypes{
		DriverType: reflect.TypeOf(drivers.FakeNetEpDriver{}),
		ConfigType: reflect.TypeOf(drivers.FakeNetEpDriverConfig{}),
	},
}

var stateDriverRegistry = map[string]driverConfigTypes{
	EtcdNameStr: driverConfigTypes{
		DriverType: reflect.TypeOf(state.EtcdStateDriver{}),
		ConfigType: reflect.TypeOf(state.EtcdStateDriverConfig{}),
	},
	ConsulNameStr: driverConfigTypes{
		DriverType: reflect.TypeOf(state.ConsulStateDriver{}),
		ConfigType: reflect.TypeOf(state.ConsulStateDriverConfig{}),
	},
	// fakestate-driver is used for tests, so not exposing a public name for it.
	"fakedriver": driverConfigTypes{
		DriverType: reflect.TypeOf(state.FakeStateDriver{}),
		ConfigType: reflect.TypeOf(state.FakeStateDriverConfig{}),
	},
}

const (
	// EtcdNameStr is a string constant for etcd state-store
	EtcdNameStr = "etcd"
	// ConsulNameStr is a string constant for consul state-store
	ConsulNameStr = "consul"
	// OvsNameStr is a string constant for ovs driver
	OvsNameStr = "ovs"
)

var (
	gStateDriver core.StateDriver
)

// initHelper initializes the NetPlugin by mapping driver names to
// configuration, then it imports the configuration.
func initHelper(driverRegistry map[string]driverConfigTypes,
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

// NewStateDriver instantiates a 'named' state-driver with specified configuration
func NewStateDriver(name, configStr string) (core.StateDriver, error) {
	if name == "" || configStr == "" {
		return nil, core.Errorf("invalid driver name or configuration passed.")
	}

	if gStateDriver != nil {
		return nil, core.Errorf("statedriver instance already exists.")
	}

	driver, drvConfig, err := initHelper(stateDriverRegistry, name, configStr)
	if err != nil {
		return nil, err
	}

	d := driver.(core.StateDriver)
	err = d.Init(drvConfig)
	if err != nil {
		return nil, err
	}

	gStateDriver = d
	return d, nil
}

// GetStateDriver returns the singleton instance of the state-driver
func GetStateDriver() (core.StateDriver, error) {
	if gStateDriver == nil {
		return nil, core.Errorf("statedriver has not been not created.")
	}

	return gStateDriver, nil
}

// ReleaseStateDriver releases the singleton instance of the state-driver
func ReleaseStateDriver() {
	if gStateDriver != nil {
		gStateDriver.Deinit()
	}
	gStateDriver = nil
}

// NewNetworkDriver instantiates a 'named' network-driver with specified configuration
func NewNetworkDriver(name, configStr string, instInfo *core.InstanceInfo) (core.NetworkDriver, error) {
	if name == "" || configStr == "" {
		return nil, core.Errorf("invalid driver name or configuration passed.")
	}

	driver, drvConfig, err := initHelper(networkDriverRegistry, name, configStr)
	if err != nil {
		return nil, err
	}

	d := driver.(core.NetworkDriver)
	err = d.Init(drvConfig, instInfo)
	if err != nil {
		return nil, err
	}

	return d, nil
}
