package utils

import (
	"testing"

	"github.com/contiv/netplugin/core"
)

func TestNewStateDriverValidConfig(t *testing.T) {
	drv, err := NewStateDriver("fakedriver", &core.InstanceInfo{})
	defer func() { ReleaseStateDriver() }()
	if err != nil {
		t.Fatalf("failed to instantiate state driver. Error: %s", err)
	}
	if drv == nil {
		t.Fatalf("nil driver instance was returned")
	}
}

func TestNewStateDriverInvalidConfig(t *testing.T) {
	_, err := NewStateDriver("fakedriver", nil)
	if err == nil {
		t.Fatalf("state driver instantiation succeeded, expected to fail")
	}
}

func TestNewStateDriverInvalidDriverName(t *testing.T) {
	_, err := NewStateDriver("non-existent-name", &core.InstanceInfo{})
	if err == nil {
		t.Fatalf("state driver instantiation succeeded, expected to fail")
	}
}

func TestNewStateDriverSecondCreate(t *testing.T) {
	_, err := NewStateDriver("fakedriver", &core.InstanceInfo{})
	defer func() { ReleaseStateDriver() }()
	if err != nil {
		t.Fatalf("failed to instantiate state driver. Error: %s", err)
	}

	_, err = NewStateDriver("fakedriver", &core.InstanceInfo{})
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
	instInfo := &core.InstanceInfo{}
	drv, err := NewNetworkDriver("fakedriver", instInfo)
	if err != nil {
		t.Fatalf("failed to instantiate network driver. Error: %s", err)
	}
	if drv == nil {
		t.Fatalf("nil driver instance was returned")
	}
}

func TestNewNetworkDriverInvalidConfig(t *testing.T) {
	_, err := NewNetworkDriver("fakedriver", nil)
	if err == nil {
		t.Fatalf("network driver instantiation succeeded, expected to fail")
	}
}

func TestNewNetworkDriverInvalidDriverName(t *testing.T) {
	instInfo := &core.InstanceInfo{}
	_, err := NewNetworkDriver("non-existent-name", instInfo)
	if err == nil {
		t.Fatalf("network driver instantiation succeeded, expected to fail")
	}
}
