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

package agent

import (
	"net"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/mgmtfn/dockplugin"
	"github.com/contiv/netplugin/mgmtfn/k8splugin"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/netplugin/cluster"
	"github.com/contiv/netplugin/netplugin/plugin"
	"github.com/contiv/netplugin/netplugin/svcplugin"
	"github.com/gorilla/mux"
	"github.com/samalba/dockerclient"

	// import and init skydns libraries
	_ "github.com/contiv/netplugin/netplugin/svcplugin/consulextension"
	_ "github.com/contiv/netplugin/netplugin/svcplugin/skydns2extension"
)

// Agent holds the netplugin agent state
type Agent struct {
	netPlugin    *plugin.NetPlugin      // driver plugin
	pluginConfig *plugin.Config         // plugin configuration
	svcPlugin    svcplugin.SvcregPlugin // svc plugin
	svcQuitCh    chan struct{}          // channel to stop svc plugin
}

// NewAgent creates a new netplugin agent
func NewAgent(pluginConfig *plugin.Config) *Agent {
	opts := pluginConfig.Instance
	netPlugin := &plugin.NetPlugin{}

	// Initialize service registry plugin
	svcPlugin, quitCh, err := svcplugin.NewSvcregPlugin(opts.DbURL, nil)
	if err != nil {
		log.Fatalf("Error initializing service registry plugin")
	}

	// init cluster state
	err = cluster.Init(opts.DbURL)
	if err != nil {
		log.Fatalf("Error initializing cluster. Err: %v", err)
	}

	// Initialize appropriate plugin
	switch opts.PluginMode {
	case "docker":
		dockplugin.InitDockPlugin(netPlugin, svcPlugin)

	case "kubernetes":
		k8splugin.InitCNIServer(netPlugin)

	case "test":
		// nothing to do. internal mode for testing
	default:
		log.Fatalf("Unknown plugin mode -- should be docker | kubernetes")
	}

	// Init the driver plugins..
	err = netPlugin.Init(*pluginConfig)
	if err != nil {
		log.Fatalf("Failed to initialize the plugin. Error: %s", err)
	}

	// create a new agent
	agent := &Agent{
		netPlugin:    netPlugin,
		pluginConfig: pluginConfig,
		svcPlugin:    svcPlugin,
		svcQuitCh:    quitCh,
	}

	return agent
}

// Plugin returns the netplugin instance
func (ag *Agent) Plugin() *plugin.NetPlugin {
	return ag.netPlugin
}

// ProcessCurrentState processes current state as read from stateStore
func (ag *Agent) ProcessCurrentState() error {
	opts := ag.pluginConfig.Instance
	readNet := &mastercfg.CfgNetworkState{}
	readNet.StateDriver = ag.netPlugin.StateDriver
	netCfgs, err := readNet.ReadAll()
	if err == nil {
		for idx, netCfg := range netCfgs {
			net := netCfg.(*mastercfg.CfgNetworkState)
			log.Debugf("read net key[%d] %s, populating state \n", idx, net.ID)
			processNetEvent(ag.netPlugin, net, false)
			if net.NwType == "infra" {
				processInfraNwCreate(ag.netPlugin, net, opts)
			}
		}
	}

	readEp := &mastercfg.CfgEndpointState{}
	readEp.StateDriver = ag.netPlugin.StateDriver
	epCfgs, err := readEp.ReadAll()
	if err == nil {
		for idx, epCfg := range epCfgs {
			ep := epCfg.(*mastercfg.CfgEndpointState)
			log.Debugf("read ep key[%d] %s, populating state \n", idx, ep.ID)
			processEpState(ag.netPlugin, opts, ep.ID)
		}
	}

	readBgp := &mastercfg.CfgBgpState{}
	readBgp.StateDriver = ag.netPlugin.StateDriver
	bgpCfgs, err := readBgp.ReadAll()
	if err == nil {
		for idx, bgpCfg := range bgpCfgs {
			bgp := bgpCfg.(*mastercfg.CfgBgpState)
			log.Debugf("read bgp key[%d] %s, populating state \n", idx, bgp.Hostname)
			processBgpEvent(ag.netPlugin, opts, bgp.Hostname, false)
		}
	}

	readServiceLb := &mastercfg.CfgServiceLBState{}
	readServiceLb.StateDriver = ag.netPlugin.StateDriver
	serviceLbCfgs, err := readServiceLb.ReadAll()
	if err == nil {
		for idx, serviceLbCfg := range serviceLbCfgs {
			serviceLb := serviceLbCfg.(*mastercfg.CfgServiceLBState)
			log.Debugf("read svc key[%d] %s for tenant %s, populating state \n", idx,
				serviceLb.ServiceName, serviceLb.Tenant)
			processServiceLBEvent(ag.netPlugin, serviceLb, false)
		}
	}

	readSvcProviders := &mastercfg.SvcProvider{}
	readSvcProviders.StateDriver = ag.netPlugin.StateDriver
	svcProviders, err := readSvcProviders.ReadAll()
	if err == nil {
		for idx, providers := range svcProviders {
			svcProvider := providers.(*mastercfg.SvcProvider)
			log.Infof("read svc provider[%d] %s , populating state \n", idx,
				svcProvider.ServiceName)
			processSvcProviderUpdEvent(ag.netPlugin, svcProvider, false)
		}
	}

	return nil
}

// PostInit post initialization
func (ag *Agent) PostInit() error {
	opts := ag.pluginConfig.Instance

	// Initialize clustering
	err := cluster.RunLoop(ag.netPlugin, opts.CtrlIP, opts.VtepIP)
	if err != nil {
		log.Errorf("Error starting cluster run loop")
	}

	// start service REST requests
	ag.serveRequests()

	return nil
}

// HandleEvents handles events
func (ag *Agent) HandleEvents() error {
	opts := ag.pluginConfig.Instance
	recvErr := make(chan error, 1)

	go handleNetworkEvents(ag.netPlugin, opts, recvErr)

	go handleBgpEvents(ag.netPlugin, opts, recvErr)

	go handleServiceLBEvents(ag.netPlugin, opts, recvErr)

	go handleSvcProviderUpdEvents(ag.netPlugin, opts, recvErr)

	if ag.pluginConfig.Instance.PluginMode == "docker" {
		// watch for docker events
		docker, _ := dockerclient.NewDockerClient("unix:///var/run/docker.sock", nil)
		go docker.StartMonitorEvents(handleDockerEvents, recvErr, ag.netPlugin, recvErr)
	} else if ag.pluginConfig.Instance.PluginMode == "kubernetes" {
		// start watching kubernetes events
		k8splugin.InitKubServiceWatch(ag.netPlugin)
	}
	err := <-recvErr
	if err != nil {
		log.Errorf("Failure occured. Error: %s", err)
		return err
	}

	return nil
}

// serveRequests serve REST api requests
func (ag *Agent) serveRequests() {
	listenURL := ":9090"
	router := mux.NewRouter()

	// Add REST routes
	s := router.Methods("GET").Subrouter()
	s.HandleFunc("/svcstats", func(w http.ResponseWriter, r *http.Request) {
		stats, err := ag.netPlugin.GetEndpointStats()
		if err != nil {
			log.Errorf("Error fetching stats from driver. Err: %v", err)
			http.Error(w, "Error fetching stats from driver", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(stats)
	})
	s.HandleFunc("/inspect/driver", func(w http.ResponseWriter, r *http.Request) {
		driverState, err := ag.netPlugin.InspectState()
		if err != nil {
			log.Errorf("Error fetching driver state. Err: %v", err)
			http.Error(w, "Error fetching driver state", http.StatusInternalServerError)
			return
		}
		w.Write(driverState)
	})

	// Create HTTP server and listener
	server := &http.Server{Handler: router}
	listener, err := net.Listen("tcp", listenURL)
	if nil != err {
		log.Fatalln(err)
	}

	log.Infof("Netplugin listening on %s", listenURL)

	// start server
	go server.Serve(listener)
}
