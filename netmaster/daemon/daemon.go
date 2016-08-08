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

package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/netmaster/objApi"
	"github.com/contiv/netplugin/netmaster/resources"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/objdb"
	"github.com/contiv/ofnet"
	"github.com/gorilla/mux"

	log "github.com/Sirupsen/logrus"
)

const leaderLockTTL = 30

// MasterDaemon runs the daemon FSM
type MasterDaemon struct {
	// Public state
	ListenURL    string // URL where netmaster needs to listen
	ClusterStore string // state store URL
	ClusterMode  string // cluster scheduler used docker/kubernetes/mesos etc
	DNSEnabled   bool   // Contiv skydns enabled?

	// Private state
	currState        string                          // Current state of the daemon
	apiController    *objApi.APIController           // API controller for contiv model
	stateDriver      core.StateDriver                // KV store
	resmgr           *resources.StateResourceManager // state resource manager
	objdbClient      objdb.API                       // Objdb client
	ofnetMaster      *ofnet.OfnetMaster              // Ofnet master instance
	listenerMutex    sync.Mutex                      // Mutex for HTTP listener
	stopLeaderChan   chan bool                       // Channel to stop the leader listener
	stopFollowerChan chan bool                       // Channel to stop the follower listener
}

var leaderLock objdb.LockInterface // leader lock

// Init initializes the master daemon
func (d *MasterDaemon) Init() {
	// set cluster mode
	err := master.SetClusterMode(d.ClusterMode)
	if err != nil {
		log.Fatalf("Failed to set cluster-mode. Error: %s", err)
	}

	// save dns flag
	err = master.SetDNSEnabled(d.DNSEnabled)
	if err != nil {
		log.Fatalf("Failed to set dns-enable. Error: %s", err)
	}

	// initialize state driver
	d.stateDriver, err = initStateDriver(d.ClusterStore)
	if err != nil {
		log.Fatalf("Failed to init state-store. Error: %s", err)
	}

	// Initialize resource manager
	d.resmgr, err = resources.NewStateResourceManager(d.stateDriver)
	if err != nil {
		log.Fatalf("Failed to init resource manager. Error: %s", err)
	}
}

func (d *MasterDaemon) registerService() {
	// Get the address to be used for local communication
	localIP, err := getLocalAddr()
	if err != nil {
		log.Fatalf("Error getting local IP address. Err: %v", err)
	}

	// service info
	srvInfo := objdb.ServiceInfo{
		ServiceName: "netmaster",
		TTL:         10,
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
		TTL:         10,
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

// Find all netplugin nodes and register them
func (d *MasterDaemon) registerNetpluginNodes() error {
	// Get all netplugin services
	srvList, err := d.objdbClient.GetService("netplugin")
	if err != nil {
		log.Errorf("Error getting netplugin nodes. Err: %v", err)
		return err
	}

	// Add each node
	for _, srv := range srvList {
		// build host info
		nodeInfo := ofnet.OfnetNode{
			HostAddr: srv.HostAddr,
			HostPort: uint16(srv.Port),
		}

		// Add the node
		err = d.ofnetMaster.AddNode(nodeInfo)
		if err != nil {
			log.Errorf("Error adding node %v. Err: %v", srv, err)
		}
	}

	return nil
}

// registerRoutes registers HTTP route handlers
func (d *MasterDaemon) registerRoutes(router *mux.Router) {
	// Add REST routes
	s := router.Headers("Content-Type", "application/json").Methods("Post").Subrouter()

	s.HandleFunc("/plugin/allocAddress", makeHTTPHandler(master.AllocAddressHandler))
	s.HandleFunc("/plugin/releaseAddress", makeHTTPHandler(master.ReleaseAddressHandler))
	s.HandleFunc("/plugin/createEndpoint", makeHTTPHandler(master.CreateEndpointHandler))
	s.HandleFunc("/plugin/deleteEndpoint", makeHTTPHandler(master.DeleteEndpointHandler))
	s.HandleFunc("/plugin/updateEndpoint", makeHTTPHandler(master.UpdateEndpointHandler))

	s = router.Methods("Get").Subrouter()

	// return netmaster version
	s.HandleFunc(fmt.Sprintf("/%s", master.GetVersionRESTEndpoint), getVersion)
	// Print info about the cluster
	s.HandleFunc(fmt.Sprintf("/%s", master.GetInfoRESTEndpoint), func(w http.ResponseWriter, r *http.Request) {
		info, err := d.getMasterInfo()
		if err != nil {
			log.Errorf("Error getting master state. Err: %v", err)
			http.Error(w, "Error getting master state", http.StatusInternalServerError)
			return
		}

		// convert to json
		resp, err := json.Marshal(info)
		if err != nil {
			http.Error(w,
				core.Errorf("marshalling json failed. Error: %s", err).Error(),
				http.StatusInternalServerError)
			return
		}
		w.Write(resp)
	})

	// services REST endpoints
	// FIXME: we need to remove once service inspect is added
	s.HandleFunc(fmt.Sprintf("/%s/%s", master.GetServiceRESTEndpoint, "{id}"),
		get(false, d.services))
	s.HandleFunc(fmt.Sprintf("/%s", master.GetServicesRESTEndpoint),
		get(true, d.services))

	// Debug REST endpoint for inspecting ofnet state
	s.HandleFunc("/debug/ofnet", func(w http.ResponseWriter, r *http.Request) {
		ofnetMasterState, err := d.ofnetMaster.InspectState()
		if err != nil {
			log.Errorf("Error fetching ofnet state. Err: %v", err)
			http.Error(w, "Error fetching ofnet state", http.StatusInternalServerError)
			return
		}
		w.Write(ofnetMasterState)
	})

}

// runLeader runs leader loop
func (d *MasterDaemon) runLeader() {
	router := mux.NewRouter()

	// acquire listener mutex
	d.listenerMutex.Lock()
	defer d.listenerMutex.Unlock()

	// Create a new api controller
	d.apiController = objApi.NewAPIController(router, d.ClusterStore)

	//Restore state from clusterStore
	d.restoreCache()

	// Register netmaster service
	d.registerService()

	// initialize policy manager
	mastercfg.InitPolicyMgr(d.stateDriver, d.ofnetMaster)

	// setup HTTP routes
	d.registerRoutes(router)

	// Create HTTP server and listener
	server := &http.Server{Handler: router}
	server.SetKeepAlivesEnabled(false)
	listener, err := net.Listen("tcp", d.ListenURL)
	if nil != err {
		log.Fatalln(err)
	}

	log.Infof("Netmaster listening on %s", d.ListenURL)

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
func (d *MasterDaemon) runFollower() {
	router := mux.NewRouter()
	router.PathPrefix("/").HandlerFunc(slaveProxyHandler)

	// acquire listener mutex
	d.listenerMutex.Lock()
	defer d.listenerMutex.Unlock()

	// start server
	server := &http.Server{Handler: router}
	server.SetKeepAlivesEnabled(false)
	listener, err := net.Listen("tcp", d.ListenURL)
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
func (d *MasterDaemon) becomeLeader() {
	// ask listener to stop
	d.stopFollowerChan <- true

	// set current state
	d.currState = "leader"

	// Run the HTTP listener
	go d.runLeader()
}

// becomeFollower changes FSM state to follower
func (d *MasterDaemon) becomeFollower() {
	// ask listener to stop
	d.stopLeaderChan <- true
	time.Sleep(time.Second)

	// set current state
	d.currState = "follower"

	// run follower loop
	go d.runFollower()
}

// RunMasterFsm runs netmaster FSM
func (d *MasterDaemon) RunMasterFsm() {
	var err error

	// Get the address to be used for local communication
	localIP, err := getLocalAddr()
	if err != nil {
		log.Fatalf("Error getting local IP address. Err: %v", err)
	}

	// create new ofnet master
	d.ofnetMaster = ofnet.NewOfnetMaster(localIP, ofnet.OFNET_MASTER_PORT)
	if d.ofnetMaster == nil {
		log.Fatalf("Error creating ofnet master")
	}

	// Create an objdb client
	d.objdbClient, err = objdb.NewClient(d.ClusterStore)
	if err != nil {
		log.Fatalf("Error connecting to state store: %v. Err: %v", d.ClusterStore, err)
	}

	// Register all existing netplugins in the background
	go d.registerNetpluginNodes()

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

func (d *MasterDaemon) restoreCache() {

	//Restore ServiceLBDb and ProviderDb
	master.RestoreServiceProviderLBDb()

}

// getMasterInfo returns information about cluster
func (d *MasterDaemon) getMasterInfo() (map[string]interface{}, error) {
	info := make(map[string]interface{})

	// get local ip
	localIP, err := getLocalAddr()
	if err != nil {
		return nil, errors.New("Error getting local IP address")
	}

	// get current holder of master lock
	leader := leaderLock.GetHolder()
	if leader == "" {
		return nil, errors.New("Leader not found")
	}

	// Get all netplugin services
	srvList, err := d.objdbClient.GetService("netplugin")
	if err != nil {
		log.Errorf("Error getting netplugin nodes. Err: %v", err)
		return nil, err
	}

	// Add each node
	pluginNodes := []string{}
	for _, srv := range srvList {
		pluginNodes = append(pluginNodes, srv.HostAddr)
	}

	// Get all netmaster services
	srvList, err = d.objdbClient.GetService("netmaster")
	if err != nil {
		log.Errorf("Error getting netmaster nodes. Err: %v", err)
		return nil, err
	}

	// Add each node
	masterNodes := []string{}
	for _, srv := range srvList {
		masterNodes = append(masterNodes, srv.HostAddr)
	}

	// setup info map
	info["local-ip"] = localIP
	info["leader-ip"] = leader
	info["current-state"] = d.currState
	info["netplugin-nodes"] = pluginNodes
	info["netmaster-nodes"] = masterNodes

	return info, nil
}
