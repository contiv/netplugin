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
	"fmt"
	"net"
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/mgmtfn/dockplugin"
	"github.com/contiv/netplugin/mgmtfn/k8splugin"
	"github.com/contiv/netplugin/mgmtfn/mesosplugin"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/netplugin/cluster"
	"github.com/contiv/netplugin/netplugin/plugin"
	"github.com/contiv/netplugin/utils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
	"github.com/gorilla/mux"
	"golang.org/x/net/context"
)

// Agent holds the netplugin agent state
type Agent struct {
	netPlugin    *plugin.NetPlugin // driver plugin
	pluginConfig *plugin.Config    // plugin configuration
}

// NewAgent creates a new netplugin agent
func NewAgent(pluginConfig *plugin.Config) *Agent {
	opts := pluginConfig.Instance
	netPlugin := &plugin.NetPlugin{}

	// init cluster state
	err := cluster.Init(pluginConfig.Drivers.State, []string{opts.DbURL})
	if err != nil {
		log.Fatalf("Error initializing cluster. Err: %v", err)
	}

	// Init the driver plugins..
	err = netPlugin.Init(*pluginConfig)
	if err != nil {
		log.Fatalf("Failed to initialize the plugin. Error: %s", err)
	}

	// Initialize appropriate plugin
	switch opts.PluginMode {
	case core.SwarmMode:
		fallthrough
	case core.Docker:
		dockplugin.InitDockPlugin(netPlugin, opts.PluginMode)

	case core.Kubernetes:
		k8splugin.InitCNIServer(netPlugin)

	case core.Test:
		// nothing to do. internal mode for testing
	default:
		log.Fatalf("Unknown plugin mode -- should be %s | %s | %s",
			core.Docker, core.SwarmMode, core.Kubernetes)
	}
	// init mesos plugin
	mesosplugin.InitPlugin(netPlugin)

	// create a new agent
	agent := &Agent{
		netPlugin:    netPlugin,
		pluginConfig: pluginConfig,
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
			processNetEvent(ag.netPlugin, net, false, opts)
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

	readEpg := mastercfg.EndpointGroupState{}
	readEpg.StateDriver = ag.netPlugin.StateDriver
	epgCfgs, err := readEpg.ReadAll()
	if err == nil {
		for idx, epgCfg := range epgCfgs {
			epg := epgCfg.(*mastercfg.EndpointGroupState)
			log.Infof("Read epg key[%d] %s, populating state \n", idx, epg.GroupName)
			processEpgEvent(ag.netPlugin, opts, epg.ID, false)
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
	err := cluster.RunLoop(ag.netPlugin, opts.CtrlIP, opts.VtepIP, opts.HostLabel)
	if err != nil {
		log.Errorf("Error starting cluster run loop")
	}

	// start service REST requests
	ag.serveRequests()

	return nil
}

func (ag *Agent) monitorDockerEvents(de chan error) {
	// watch for docker events
	docker, err := dockerclient.NewClient("unix:///var/run/docker.sock", "", nil, nil)
	if err != nil {
		log.Errorf("Error connecting to docker - %v", err)
		de <- err
		return
	}

	contFilter := filters.NewArgs()
	contFilter.Add("type", events.ContainerEventType)

	events, errs := docker.Events(context.Background(), types.EventsOptions{Filters: contFilter})
	go ag.handleDockerEvents(events, errs)
}

// HandleEvents handles events
func (ag *Agent) HandleEvents() error {
	opts := ag.pluginConfig.Instance
	recvErr := make(chan error, 1)

	go handleNetworkEvents(ag.netPlugin, opts, recvErr)

	go handleBgpEvents(ag.netPlugin, opts, recvErr)

	go handleEndpointEvents(ag.netPlugin, opts, recvErr)

	go handleEpgEvents(ag.netPlugin, opts, recvErr)

	go handleServiceLBEvents(ag.netPlugin, opts, recvErr)

	go handleSvcProviderUpdEvents(ag.netPlugin, opts, recvErr)

	go handleGlobalCfgEvents(ag.netPlugin, opts, recvErr)

	go handlePolicyRuleEvents(ag.netPlugin, opts, recvErr)

	if ag.pluginConfig.Instance.PluginMode == core.Docker ||
		ag.pluginConfig.Instance.PluginMode == core.SwarmMode {
		go ag.monitorDockerEvents(recvErr)
	} else if ag.pluginConfig.Instance.PluginMode == core.Kubernetes {
		// start watching kubernetes events
		k8splugin.InitKubServiceWatch(ag.netPlugin)
	}
	err := <-recvErr
	if err != nil {
		time.Sleep(1 * time.Second)
		log.Errorf("Failure occurred. Error: %s", err)
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
	s.HandleFunc("/inspect/bgp", func(w http.ResponseWriter, r *http.Request) {
		bgpState, err := ag.netPlugin.InspectBgp()
		if err != nil {
			log.Errorf("Error fetching bgp. Err: %v", err)
			http.Error(w, "Error fetching bgp", http.StatusInternalServerError)
			return
		}
		w.Write(bgpState)
	})

	s.HandleFunc("/inspect/nameserver", func(w http.ResponseWriter, r *http.Request) {
		ns, err := ag.netPlugin.NetworkDriver.InspectNameserver()
		if err != nil {
			log.Errorf("Error fetching nameserver state. Err: %v", err)
			http.Error(w, "Error fetching nameserver state", http.StatusInternalServerError)
			return
		}
		w.Write(ns)
	})

	s = router.Methods("Delete").Subrouter()
	s.HandleFunc("/debug/reclaimEndpoint/{id}", utils.MakeHTTPHandler(ag.ReclaimEndpointHandler))

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

// ReclaimEndpointHandler reclaims endpoint
func (ag *Agent) ReclaimEndpointHandler(w http.ResponseWriter, r *http.Request, vars map[string]string) (interface{}, error) {
	epID := vars["id"]
	// delete the endpoint
	err := ag.netPlugin.DeleteEndpoint(epID)
	if err != nil {
		log.Errorf("Error deleting endpoint %v. Err: %v", epID, err)
		http.Error(w, fmt.Sprintf("failed to delete endpoint: %+v", epID), 0)
		return nil, err
	}

	return nil, nil
}
