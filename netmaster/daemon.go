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

package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/netmaster/objApi"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/netplugin/utils/netutils"
	"github.com/contiv/objdb"
	"github.com/contiv/ofnet"
	"github.com/gorilla/mux"

	log "github.com/Sirupsen/logrus"
)

const leaderLockTTL = 30

type daemon struct {
	listenURL        string                // URL where netmaster needs to listen
	currState        string                // Current state of the daemon
	storeURL         string                // state store URL
	apiController    *objApi.APIController // API controller for contiv model
	stateDriver      core.StateDriver      // KV store
	objdbClient      objdb.API             // Objdb client
	ofnetMaster      *ofnet.OfnetMaster    // Ofnet master instance
	listenerMutex    sync.Mutex            // Mutex for HTTP listener
	stopLeaderChan   chan bool             // Channel to stop the leader listener
	stopFollowerChan chan bool             // Channel to stop the follower listener
}

var leaderLock objdb.LockInterface // leader lock

// GetLocalAddr gets local address to be used
func GetLocalAddr() (string, error) {
	// get the ip address by local hostname
	localIP, err := netutils.GetMyAddr()
	if err == nil && netutils.IsAddrLocal(localIP) {
		return localIP, nil
	}

	// Return first available address if we could not find by hostname
	return netutils.GetFirstLocalAddr()
}

func (d *daemon) registerService() {
	// Get the address to be used for local communication
	localIP, err := GetLocalAddr()
	if err != nil {
		log.Fatalf("Error getting local IP address. Err: %v", err)
	}

	// service info
	srvInfo := objdb.ServiceInfo{
		ServiceName: "netmaster",
		HostAddr:    localIP,
		Port:        9999,
		Role:        d.currState,
	}

	// Register the node with service registry
	err = d.objdbClient.RegisterService(srvInfo)
	if err != nil {
		log.Fatalf("Error registering service. Err: %v", err)
	}

	// service info
	srvInfo = objdb.ServiceInfo{
		ServiceName: "netmaster.rpc",
		HostAddr:    localIP,
		Port:        ofnet.OFNET_MASTER_PORT,
		Role:        d.currState,
	}

	// Register the node with service registry
	err = d.objdbClient.RegisterService(srvInfo)
	if err != nil {
		log.Fatalf("Error registering service. Err: %v", err)
	}

	log.Infof("Registered netmaster service with registry")
}

// registerWebuiHandler registers handlers for serving web UI
func (d *daemon) registerWebuiHandler(router *mux.Router) {
	// Setup the router to serve the web UI
	goPath := os.Getenv("GOPATH")
	if goPath != "" {
		webPath := goPath + "/src/github.com/contiv/contivmodel/www/"

		// Make sure we have the web UI files
		_, err := os.Stat(webPath)
		if err != nil {
			webPath = goPath + "/src/github.com/contiv/netplugin/" +
				"Godeps/_workspace/src/github.com/contiv/contivmodel/www/"
			_, err := os.Stat(webPath)
			if err != nil {
				log.Errorf("Can not find the web UI directory")
			}
		}

		log.Infof("Using webPath: %s", webPath)

		// serve static files
		router.PathPrefix("/web/").Handler(http.StripPrefix("/web/", http.FileServer(http.Dir(webPath))))

		// Special case to serve main index.html
		router.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
			http.ServeFile(rw, req, webPath+"index.html")
		})
	}

	// proxy Handler
	router.PathPrefix("/proxy/").HandlerFunc(proxyHandler)
}

// registerRoutes registers HTTP route handlers
func (d *daemon) registerRoutes(router *mux.Router) {
	// register web ui handlers
	d.registerWebuiHandler(router)

	// Add REST routes
	s := router.Headers("Content-Type", "application/json").Methods("Post").Subrouter()

	s.HandleFunc("/plugin/allocAddress", makeHTTPHandler(master.AllocAddressHandler))
	s.HandleFunc("/plugin/releaseAddress", makeHTTPHandler(master.ReleaseAddressHandler))
	s.HandleFunc("/plugin/createEndpoint", makeHTTPHandler(master.CreateEndpointHandler))
	s.HandleFunc("/plugin/deleteEndpoint", makeHTTPHandler(master.DeleteEndpointHandler))

	s = router.Methods("Get").Subrouter()
	s.HandleFunc(fmt.Sprintf("/%s/%s", master.GetEndpointRESTEndpoint, "{id}"),
		get(false, d.endpoints))
	s.HandleFunc(fmt.Sprintf("/%s", master.GetEndpointsRESTEndpoint),
		get(true, d.endpoints))
	s.HandleFunc(fmt.Sprintf("/%s/%s", master.GetNetworkRESTEndpoint, "{id}"),
		get(false, d.networks))
	s.HandleFunc(fmt.Sprintf("/%s", master.GetNetworksRESTEndpoint),
		get(true, d.networks))
	s.HandleFunc(fmt.Sprintf("/%s", master.GetVersionRESTEndpoint), getVersion)

	// See if we need to create the default tenant
	go objApi.CreateDefaultTenant()
}

// XXX: This function should be returning logical state instead of driver state
func (d *daemon) endpoints(id string) ([]core.State, error) {
	var (
		err error
		ep  *drivers.OvsOperEndpointState
	)

	ep = &drivers.OvsOperEndpointState{}
	if ep.StateDriver, err = utils.GetStateDriver(); err != nil {
		return nil, err
	}

	if id == "all" {
		eps, err := ep.ReadAll()
		if err != nil {
			return []core.State{}, nil
		}
		return eps, nil
	}

	err = ep.Read(id)
	if err == nil {
		return []core.State{core.State(ep)}, nil
	}

	return nil, core.Errorf("Unexpected code path. Recieved error during read: %v", err)
}

// XXX: This function should be returning logical state instead of driver state
func (d *daemon) networks(id string) ([]core.State, error) {
	var (
		err error
		nw  *mastercfg.CfgNetworkState
	)

	nw = &mastercfg.CfgNetworkState{}
	if nw.StateDriver, err = utils.GetStateDriver(); err != nil {
		return nil, err
	}

	if id == "all" {
		return nw.ReadAll()
	} else if err := nw.Read(id); err == nil {
		return []core.State{core.State(nw)}, nil
	}

	return nil, core.Errorf("Unexpected code path")
}

// runLeader runs leader loop
func (d *daemon) runLeader() {
	router := mux.NewRouter()

	// acquire listener mutex
	d.listenerMutex.Lock()
	defer d.listenerMutex.Unlock()

	// Create a new api controller
	d.apiController = objApi.NewAPIController(router, d.storeURL)

	// initialize policy manager
	mastercfg.InitPolicyMgr(d.stateDriver, d.ofnetMaster)

	// Register netmaster service
	d.registerService()

	// setup HTTP routes
	d.registerRoutes(router)

	// Create HTTP server and listener
	server := &http.Server{Handler: router}
	listener, err := net.Listen("tcp", d.listenURL)
	if nil != err {
		log.Fatalln(err)
	}

	log.Infof("Netmaster listening on %s", d.listenURL)

	listener = utils.ListenWrapper(listener)

	// start server
	go server.Serve(listener)

	// Wait till we are asked to stop
	<-d.stopLeaderChan

	// Close the listener and exit
	listener.Close()
	log.Infof("Exiting Leader mode")
}

// runFollower runs the follower FSM loop
func (d *daemon) runFollower() {
	router := mux.NewRouter()
	router.PathPrefix("/").HandlerFunc(slaveProxyHandler)

	// acquire listener mutex
	d.listenerMutex.Lock()
	defer d.listenerMutex.Unlock()

	// start server
	server := &http.Server{Handler: router}
	listener, err := net.Listen("tcp", d.listenURL)
	if nil != err {
		log.Fatalln(err)
	}

	listener = utils.ListenWrapper(listener)

	// start server
	go server.Serve(listener)

	// Register netmaster service
	d.registerService()

	// just wait on stop channel
	log.Infof("Listening in follower mode")
	<-d.stopFollowerChan

	// Close the listener and exit
	listener.Close()
	log.Info("Exiting follower mode")
}

// becomeLeader changes daemon FSM state to master
func (d *daemon) becomeLeader() {
	// ask listener to stop
	d.stopFollowerChan <- true

	// set current state
	d.currState = "leader"

	// Run the HTTP listener
	go d.runLeader()
}

// becomeFollower changes FSM state to follower
func (d *daemon) becomeFollower() {
	// ask listener to stop
	d.stopLeaderChan <- true

	// set current state
	d.currState = "follower"

	// run follower loop
	go d.runFollower()
}

// runMasterFsm runs netmaster FSM
func (d *daemon) runMasterFsm() {
	var err error

	// create new ofnet master
	d.ofnetMaster = ofnet.NewOfnetMaster(ofnet.OFNET_MASTER_PORT)
	if d.ofnetMaster == nil {
		log.Fatalf("Error creating ofnet master")
	}

	// Create an objdb client
	d.objdbClient, err = objdb.NewClient(d.storeURL)
	if err != nil {
		log.Fatalf("Error connecting to state store: %v. Err: %v", d.storeURL, err)
	}

	// Get the address to be used for local communication
	localIP, err := GetLocalAddr()
	if err != nil {
		log.Fatalf("Error getting local IP address. Err: %v", err)
	}

	// Create the lock
	leaderLock, err = d.objdbClient.NewLock("netmaster/leader", localIP, leaderLockTTL)
	if err != nil {
		log.Fatalf("Could not create leader lock. Err: %v", err)
	}

	// Try to acquire the lock
	err = leaderLock.Acquire(0)
	if err != nil {
		// We dont expect any error during acquire.
		log.Fatalf("Error while acquiring lock. Err: %v", err)
	}

	// Initialize the stop channel
	d.stopLeaderChan = make(chan bool, 1)
	d.stopFollowerChan = make(chan bool, 1)

	// set current state
	d.currState = "follower"

	// Start off being a follower
	go d.runFollower()

	// Main run loop waiting on leader lock
	for {
		// Wait for lock events
		select {
		case event := <-leaderLock.EventChan():
			if event.EventType == objdb.LockAcquired {
				log.Infof("Leader lock acquired")

				d.becomeLeader()
			} else if event.EventType == objdb.LockLost {
				log.Infof("Leader lock lost. Becoming follower")

				d.becomeFollower()
			}
		}
	}
}
