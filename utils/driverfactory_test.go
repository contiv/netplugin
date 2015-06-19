package utils

import (
	"encoding/json"
	"testing"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/state"
)

func TestNewStateDriverValidConfig(t *testing.T) {
	config := &core.Config{V: &state.FakeStateDriverConfig{}}
	cfgBytes, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshalling configuration failed. Error: %s", err)
	}

	drv, err := NewStateDriver("fakedriver", string(cfgBytes))
	defer func() { ReleaseStateDriver() }()
	if err != nil {
		t.Fatalf("failed to instantiate state driver. Error: %s", err)
	}
	if drv == nil {
		t.Fatalf("nil driver instance was returned")
	}
}

func TestNewStateDriverInvalidConfig(t *testing.T) {
	_, err := NewStateDriver("fakedriver", "")
	if err == nil {
		t.Fatalf("state driver instantiation succeeded, expected to fail")
	}
}

func TestNewStateDriverInvalidDriverName(t *testing.T) {
	config := &core.Config{V: &state.FakeStateDriverConfig{}}
	cfgBytes, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshalling configuration failed. Error: %s", err)
	}

	_, err = NewStateDriver("non-existent-name", string(cfgBytes))
	if err == nil {
		t.Fatalf("state driver instantiation succeeded, expected to fail")
	}
}

func TestNewStateDriverSecondCreate(t *testing.T) {
	config := &core.Config{V: &state.FakeStateDriverConfig{}}
	cfgBytes, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshalling configuration failed. Error: %s", err)
	}

	_, err = NewStateDriver("fakedriver", string(cfgBytes))
	defer func() { ReleaseStateDriver() }()
	if err != nil {
		t.Fatalf("failed to instantiate state driver. Error: %s", err)
	}

	_, err = NewStateDriver("fakedriver", string(cfgBytes))
	if err == nil {
		t.Fatalf("second state driver instantiation succeeded, expected to fail")
	}
}

func TestGetStateDriverNonExistentStateDriver(t *testing.T) {
	_, err := GetStateDriver()
	if err == nil {
		t.Fatalf("getting state-driver succeeded, expected to fail")
	}
}

func TestNewNetworkDriverValidConfig(t *testing.T) {
	config := &core.Config{V: &drivers.FakeNetEpDriverConfig{}}
	cfgBytes, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshalling configuration failed. Error: %s", err)
	}

	instInfo := &core.InstanceInfo{}
	drv, err := NewNetworkDriver("fakedriver", string(cfgBytes), instInfo)
	if err != nil {
		t.Fatalf("failed to instantiate network driver. Error: %s", err)
	}
	if drv == nil {
		t.Fatalf("nil driver instance was returned")
	}
}

func TestNewNetworkDriverInvalidConfig(t *testing.T) {
	instInfo := &core.InstanceInfo{}
	_, err := NewNetworkDriver("fakedriver", "", instInfo)
	if err == nil {
		t.Fatalf("network driver instantiation succeeded, expected to fail")
	}
}

func TestNewNetworkDriverInvalidDriverName(t *testing.T) {
	config := &core.Config{V: &drivers.FakeNetEpDriverConfig{}}
	cfgBytes, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshalling configuration failed. Error: %s", err)
	}

	instInfo := &core.InstanceInfo{}
	_, err = NewNetworkDriver("non-existent-name", string(cfgBytes), instInfo)
	if err == nil {
		t.Fatalf("network driver instantiation succeeded, expected to fail")
	}
}
