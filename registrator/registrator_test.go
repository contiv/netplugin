package registrator

import (
	"github.com/contiv/netplugin/registrator/bridge"
	skydns "github.com/contiv/netplugin/registrator/skydns2registrator"
	"os"
	"testing"
)

func TestRegistratorInit(t *testing.T) {
	reg, err := InitRegistrator("skydns2:")
	if err != nil || reg == nil {
		t.Fatalf("Registrator init failed. Error: %s", err)
	}
}

func TestRegistratorInitWithDefaultConfig(t *testing.T) {
	bConfig := bridge.DefaultBridgeConfig()
	reg, err := InitRegistrator("skydns2:", bConfig)
	if err != nil || reg == nil {
		t.Fatalf("Registrator init failed. Error: %s", err)
	}
}

func TestRegistratorInitWithInvalidRefreshDetails(t *testing.T) {
	bConfig := bridge.DefaultBridgeConfig()

	bConfig.RefreshTTL = 0
	bConfig.RefreshInterval = 30
	reg, err := InitRegistrator("skydns2:", bConfig)
	if err != nil || reg == nil {
		t.Fatalf("Registrator init failed for values {%v}. Error: %s", bConfig, err)
	}

	bConfig.RefreshTTL = 30
	bConfig.RefreshInterval = 0
	reg, err = InitRegistrator("skydns2:", bConfig)
	if err != nil || reg == nil {
		t.Fatalf("Registrator init failed for values {%v}. Error: %s", bConfig, err)
	}

	bConfig.RefreshTTL = 30
	bConfig.RefreshInterval = 20
	reg, err = InitRegistrator("skydns2:", bConfig)
	if err != nil || reg == nil {
		t.Fatalf("Registrator init failed for values {%v}. Error: %s", bConfig, err)
	}

	bConfig.RefreshTTL = -30
	bConfig.RefreshInterval = -20
	reg, err = InitRegistrator("skydns2:", bConfig)
	if err != nil || reg == nil {
		t.Fatalf("Registrator init failed for values {%v}. Error: %s", bConfig, err)
	}
}

func TestRegistratorInitWithInvalidDeregisterDetails(t *testing.T) {
	bConfig := bridge.DefaultBridgeConfig()

	bConfig.DeregisterCheck = "random"
	reg, err := InitRegistrator("skydns2:", bConfig)
	if err != nil || reg == nil {
		t.Fatalf("Registrator init failed for values {%v}. Error: %s", bConfig, err)
	}
}

func TestMain(m *testing.M) {
	bridge.Register(new(skydns.Factory), "skydns2")
	os.Exit(m.Run())
}
