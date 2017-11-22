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

package integration

import (
	"os"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/contiv/netplugin/contivmodel"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/daemon"
	"github.com/contiv/netplugin/netplugin/agent"
	"github.com/contiv/netplugin/netplugin/plugin"
	"github.com/contiv/netplugin/utils/netutils"
)

// NPCluster holds a new neplugin/netmaster cluster stats
type NPCluster struct {
	MasterDaemon *daemon.MasterDaemon // master instance
	PluginAgent  *agent.Agent         // netplugin agent
	HostLabel    string               // local host name
	LocalIP      string               // local ip addr
}

// NewNPCluster creates a new cluster of netplugin/netmaster
func NewNPCluster(its *integTestSuite) (*NPCluster, error) {
	// get local host name
	hostLabel, err := os.Hostname()
	if err != nil {
		log.Fatalf("Failed to fetch hostname. Error: %s", err)
	}

	// get local IP addr
	localIP, err := netutils.GetDefaultAddr()
	if err != nil {
		log.Fatalf("Error getting local address. Err: %v", err)
	}

	// create master daemon
	md := &daemon.MasterDaemon{
		ListenURL:          "0.0.0.0:9999",
		ControlURL:         "0.0.0.0:9999",
		ClusterMode:        "test",
		ClusterStoreDriver: its.clusterStoreDriver,
		ClusterStoreURL:    its.clusterStoreURL,
		NetworkMode:        its.encap,
		NetForwardMode:     its.fwdMode,
		NetInfraType:       its.fabricMode,
	}

	pluginConfig := plugin.Config{
		Drivers: plugin.Drivers{
			Network: "ovs",
			State:   its.clusterStoreDriver,
		},
		Instance: core.InstanceInfo{
			HostLabel:  hostLabel,
			CtrlIP:     localIP,
			VtepIP:     localIP,
			UplinkIntf: []string{"eth2"},
			DbURL:      its.clusterStoreURL,
			PluginMode: "test",
		},
	}

	// initialize master daemon
	md.Init()

	// Run daemon FSM
	go md.RunMasterFsm()

	// Wait for a second for master to initialize
	time.Sleep(10 * time.Second)

	err = contivModel.CreateGlobal(&contivModel.Global{
		Key:              "global",
		Name:             "global",
		NetworkInfraType: its.fabricMode,
		Vlans:            "1-4094",
		Vxlans:           "1-10000",
		FwdMode:          its.fwdMode,
		ArpMode:          its.arpMode,
		PvtSubnet:        "172.19.0.0/16",
	})

	if err != nil {
		log.Fatalf("Error creating global state. Err: %v", err)
	}

	// Create a new agent
	ag := agent.NewAgent(&pluginConfig)

	// Process all current state
	ag.ProcessCurrentState()

	// post initialization processing
	ag.PostInit()

	// handle events
	go func() {
		if err := ag.HandleEvents(); err != nil {
			log.Infof("Netplugin exiting due to error: %v", err)
			os.Exit(1)
		}
	}()

	// Wait for a second for things to settle down
	time.Sleep(time.Second)

	cl := &NPCluster{
		MasterDaemon: md,
		PluginAgent:  ag,
		HostLabel:    hostLabel,
		LocalIP:      localIP,
	}

	return cl, nil
}
