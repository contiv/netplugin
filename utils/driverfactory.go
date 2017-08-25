package utils

import (
	"reflect"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/drivers/ovsd"
	"github.com/contiv/netplugin/drivers/vppd"
	"github.com/contiv/netplugin/state"
)

// implement utilities for instantiating the supported core.Driver
// (state, network and endpoint) instances

type driverConfigTypes struct {
	DriverType reflect.Type
	ConfigType reflect.Type
}

var networkDriverRegistry = map[string]driverConfigTypes{
	OvsNameStr: {
		DriverType: reflect.TypeOf(ovsd.OvsDriver{}),
		ConfigType: reflect.TypeOf(ovsd.OvsDriver{}),
	},
	VppNameStr: {
		DriverType: reflect.TypeOf(vppd.VppDriver{}),
		ConfigType: reflect.TypeOf(vppd.VppDriver{}),
	},
	// fakedriver is used for tests, so not exposing a public name for it.
	"fakedriver": {
		DriverType: reflect.TypeOf(drivers.FakeNetEpDriver{}),
		ConfigType: reflect.TypeOf(drivers.FakeNetEpDriverConfig{}),
	},
}

var stateDriverRegistry = map[string]driverConfigTypes{
	EtcdNameStr: {
		DriverType: reflect.TypeOf(state.EtcdStateDriver{}),
		ConfigType: reflect.TypeOf(state.EtcdStateDriverConfig{}),
	},
	ConsulNameStr: {
		DriverType: reflect.TypeOf(state.ConsulStateDriver{}),
		ConfigType: reflect.TypeOf(state.ConsulStateDriverConfig{}),
	},
	// fakestate-driver is used for tests, so not exposing a public name for it.
	"fakedriver": {
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
	// VppNameStr is a string constant for vpp driver
	VppNameStr = "vpp"
)

var (
	gStateDriver core.StateDriver
)

// initHelper initializes the NetPlugin by mapping driver names to
// configuration, then it imports the configuration.
func initHelper(driverRegistry map[string]driverConfigTypes, driverName string) (core.Driver, error) {
	if _, ok := driverRegistry[driverName]; ok {
		driverType := driverRegistry[driverName].DriverType

		driver := reflect.New(driverType).Interface()
		return driver, nil
	}

	return nil, core.Errorf("Failed to find a registered driver for: %s", driverName)
}

// NewStateDriver instantiates a 'named' state-driver with specified configuration
func NewStateDriver(name string, instInfo *core.InstanceInfo) (core.StateDriver, error) {
	if name == "" || instInfo == nil {
		return nil, core.Errorf("invalid driver name or configuration passed.")
	}

	if gStateDriver != nil {
		return nil, core.Errorf("statedriver instance already exists.")
	}

	driver, err := initHelper(stateDriverRegistry, name)
	if err != nil {
		return nil, err
	}

	d := driver.(core.StateDriver)
	err = d.Init(instInfo)
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
func NewNetworkDriver(name string, instInfo *core.InstanceInfo) (core.NetworkDriver, error) {
	if name == "" || instInfo == nil {
		return nil, core.Errorf("invalid driver name or configuration passed.")
	}

	driver, err := initHelper(networkDriverRegistry, name)
	if err != nil {
		return nil, err
	}

	d := driver.(core.NetworkDriver)
	err = d.Init(instInfo)
	if err != nil {
		return nil, err
	}

	return d, nil
}
