package plugin

import (
	"testing"
)

func TestNetPluginInit(t *testing.T) {
	configStr := `{
                    "drivers" : {
                       "network": "ovs",
                       "endpoint": "ovs",
                       "state": "etcd"
                    },
                    "ovs" : {
                       "dbip": "127.0.0.1",
                       "dbport": 6640
                    },
                    "etcd" : {
                        "machines": ["http://1.0.0.1:4001"]
                    }
                  }`
	plugin := NetPlugin{}
	err := plugin.Init(configStr)
	if err != nil {
		t.Fatalf("plugin init failed: Error: %s", err)
	}
	defer func() { plugin.Deinit() }()
}

func TestNetPluginInitInvalidConfigEmptyString(t *testing.T) {
	configStr := ""
	plugin := NetPlugin{}
	err := plugin.Init(configStr)
	if err == nil {
		t.Fatalf("plugin init succeeded, should have failed!")
	}
}

func TestNetPluginInitInvalidConfigMissingStateDriverName(t *testing.T) {
	configStr := `{
                    "drivers" : {
                       "network": "ovs",
                       "endpoint": "ovs"
                    },
                    "ovs" : {
                       "dbip": "127.0.0.1",
                       "dbport": 6640
                    },
                    "etcd" : {
                        "machines": ["http://1.0.0.1:4001"]
                    }
                  }`
	plugin := NetPlugin{}
	err := plugin.Init(configStr)
	if err == nil {
		t.Fatalf("plugin init succeeded, should have failed!")
	}
}

func TestNetPluginInitInvalidConfigMissingStateDriver(t *testing.T) {
	configStr := `{
                    "drivers" : {
                       "network": "ovs",
                       "endpoint": "ovs",
                       "state": "etcd"
                    },
                    "ovs" : {
                       "dbip": "127.0.0.1",
                       "dbport": 6640
                    }
                  }`
	plugin := NetPlugin{}
	err := plugin.Init(configStr)
	if err != nil {
		t.Fatalf("plugin init failed: Error: %s", err)
	}
	defer func() { plugin.Deinit() }()
}

func TestNetPluginInitInvalidConfigMissingNetworkDriverName(t *testing.T) {
	configStr := `{
                    "drivers" : {
                       "endpoint": "ovs",
                       "state": "etcd"
                    },
                    "ovs" : {
                       "dbip": "127.0.0.1",
                       "dbport": 6640
                    },
                    "etcd" : {
                        "machines": ["http://1.0.0.1:4001"]
                    }
                  }`
	plugin := NetPlugin{}
	err := plugin.Init(configStr)
	if err == nil {
		t.Fatalf("plugin init succeeded, should have failed!")
	}
}

func TestNetPluginInitInvalidConfigMissingEndpointDriverName(t *testing.T) {
	configStr := `{
                    "drivers" : {
                       "network": "ovs",
                       "state": "etcd"
                    },
                    "ovs" : {
                       "dbip": "127.0.0.1",
                       "dbport": 6640
                    },
                    "etcd" : {
                        "machines": ["http://1.0.0.1:4001"]
                    }
                  }`
	plugin := NetPlugin{}
	err := plugin.Init(configStr)
	if err == nil {
		t.Fatalf("plugin init succeeded, should have failed!")
	}
}

func TestNetPluginInitInvalidConfigMissingNetworkDriver(t *testing.T) {
	configStr := `{
                    "drivers" : {
                       "network": "ovs",
                       "endpoint": "ovs",
                       "state": "etcd"
                    },
                    "etcd" : {
                        "machines": ["http://1.0.0.1:4001"]
                    }
                  }`
	plugin := NetPlugin{}
	err := plugin.Init(configStr)
	if err != nil {
		t.Fatalf("plugin init failed: Error: %s", err)
	}
	defer func() { plugin.Deinit() }()
}
