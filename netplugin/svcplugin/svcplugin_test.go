package svcplugin

import (
	"os"
	"testing"

	"github.com/contiv/netplugin/netplugin/svcplugin/bridge"
	skydns "github.com/contiv/netplugin/netplugin/svcplugin/skydns2extension"
)

func TestServicePluginInit(t *testing.T) {
	reg, err := InitServicePlugin("skydns2:")
	if err != nil || reg == nil {
		t.Fatalf("Service plugin init failed. Error: %s", err)
	}
}

func TestServicePluginInitWithDefaultConfig(t *testing.T) {
	bConfig := bridge.DefaultBridgeConfig()
	reg, err := InitServicePlugin("skydns2:", bConfig)
	if err != nil || reg == nil {
		t.Fatalf("Service plugin init failed. Error: %s", err)
	}
}

func TestServicePluginInitWithInvalidRefreshDetails(t *testing.T) {
	bConfig := bridge.DefaultBridgeConfig()

	bConfig.RefreshTTL = 0
	bConfig.RefreshInterval = 30
	reg, err := InitServicePlugin("skydns2:", bConfig)
	if err != nil || reg == nil {
		t.Fatalf("Service plugin init failed for values {%v}. Error: %s", bConfig, err)
	}

	bConfig.RefreshTTL = 30
	bConfig.RefreshInterval = 0
	reg, err = InitServicePlugin("skydns2:", bConfig)
	if err != nil || reg == nil {
		t.Fatalf("Service plugin init failed for values {%v}. Error: %s", bConfig, err)
	}

	bConfig.RefreshTTL = 30
	bConfig.RefreshInterval = 20
	reg, err = InitServicePlugin("skydns2:", bConfig)
	if err != nil || reg == nil {
		t.Fatalf("Service plugin init failed for values {%v}. Error: %s", bConfig, err)
	}

	bConfig.RefreshTTL = -30
	bConfig.RefreshInterval = -20
	reg, err = InitServicePlugin("skydns2:", bConfig)
	if err != nil || reg == nil {
		t.Fatalf("Service plugin init failed for values {%v}. Error: %s", bConfig, err)
	}
}

func TestServicePluginInitWithInvalidDeregisterDetails(t *testing.T) {
	bConfig := bridge.DefaultBridgeConfig()

	bConfig.DeregisterCheck = "random"
	reg, err := InitServicePlugin("skydns2:", bConfig)
	if err != nil || reg == nil {
		t.Fatalf("Service plugin init failed for values {%v}. Error: %s", bConfig, err)
	}
}

func TestMain(m *testing.M) {
	bridge.Register(new(skydns.Factory), "skydns2")
	os.Exit(m.Run())
}
