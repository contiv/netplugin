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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/netmaster/objApi"
	"github.com/contiv/netplugin/netmaster/resources"
	"github.com/contiv/netplugin/objdb"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/netplugin/utils/netutils"
	"github.com/contiv/ofnet"
	"github.com/gorilla/mux"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/netmaster/k8snetwork"
)

const leaderLockTTL = 30

// MasterDaemon runs the daemon FSM
type MasterDaemon struct {
	// Public state
	ListenURL          string // URL where netmaster listens for ext requests
	ControlURL         string // URL where netmaster listens for ctrl pkts
	ClusterStoreDriver string // state store driver name
	ClusterStoreURL    string // state store endpoint
	ClusterMode        string // cluster scheduler used docker/kubernetes/mesos etc
	NetworkMode        string // network mode (vlan or vxlan)
	NetForwardMode     string // forwarding mode (bridge or routing)
	NetInfraType       string // infra type (aci or default)

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
		log.Fatalf("Failed to set cluster-mode %q. Error: %s", d.ClusterMode, err)
	}

	// initialize state driver
	d.stateDriver, err = utils.NewStateDriver(d.ClusterStoreDriver, &core.InstanceInfo{DbURL: d.ClusterStoreURL})
	if err != nil {
		log.Fatalf("Failed to init state-store: driver %q, URLs %q. Error: %s", d.ClusterStoreDriver, d.ClusterStoreURL, err)
	}

	// Initialize resource manager
	d.resmgr, err = resources.NewStateResourceManager(d.stateDriver)
	if err != nil {
		log.Fatalf("Failed to init resource manager. Error: %s", err)
	}

	// Create an objdb client
	d.objdbClient, err = objdb.InitClient(d.ClusterStoreDriver, []string{d.ClusterStoreURL})
	if err != nil {
		log.Fatalf("Error connecting to state store: driver %q, URLs %q. Err: %v", d.ClusterStoreDriver, d.ClusterStoreURL, err)
	}
}

func (d *MasterDaemon) registerService() {
	var err error

	ctrlURL := strings.Split(d.ControlURL, ":")
	masterIP := ctrlURL[0]
	masterPort, _ := strconv.Atoi(ctrlURL[1])

	// service info
	srvInfo := objdb.ServiceInfo{
		ServiceName: "netmaster",
		TTL:         10,
		HostAddr:    masterIP,
		Port:        masterPort,
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
		HostAddr:    masterIP,
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

// Find all netplugin nodes and add them to ofnet master
func (d *MasterDaemon) agentDiscoveryLoop() {

	// Create channels for watch thread
	agentEventCh := make(chan objdb.WatchServiceEvent, 1)
	watchStopCh := make(chan bool, 1)

	// Start a watch on netplugin service
	err := d.objdbClient.WatchService("netplugin", agentEventCh, watchStopCh)
	if err != nil {
		log.Fatalf("Could not start a watch on netplugin service. Err: %v", err)
	}

	for {
		agentEv := <-agentEventCh
		log.Debugf("Received netplugin watch event: %+v", agentEv)
		// build host info
		nodeInfo := ofnet.OfnetNode{
			HostAddr: agentEv.ServiceInfo.HostAddr,
			HostPort: uint16(agentEv.ServiceInfo.Port),
		}

		if agentEv.EventType == objdb.WatchServiceEventAdd {
			err = d.ofnetMaster.AddNode(nodeInfo)
			if err != nil {
				log.Errorf("Error adding node %v. Err: %v", nodeInfo, err)
			}
		} else if agentEv.EventType == objdb.WatchServiceEventDel {
			var res bool
			log.Infof("Unregister node %+v", nodeInfo)
			d.ofnetMaster.UnRegisterNode(&nodeInfo, &res)

			go d.startDeferredCleanup(nodeInfo, agentEv.ServiceInfo.Hostname)
		}

		// Dont process next peer event for another 100ms
		time.Sleep(100 * time.Millisecond)
	}
}

func (d *MasterDaemon) startDeferredCleanup(host ofnet.OfnetNode, hostName string) {
	timeout := make(chan bool, 1)
	timeoutDuration := 24 * time.Hour
	go func() {
		log.Infof("Started GC timer for 1 day for node: %+v", host)
		time.Sleep(timeoutDuration)
		timeout <- true
	}()

	for {
		select {
		case <-time.After(timeoutDuration / 10):
			hostKey := fmt.Sprintf("%s:%d", host.HostAddr, host.HostPort)
			srvList, err := d.objdbClient.GetService("netplugin")
			if err != nil {
				log.Errorf("Error getting netplugin nodes. Err: %v", err)
			}

			for _, srv := range srvList {
				serviceKey := fmt.Sprintf("%s:%d", srv.HostAddr, srv.Port)
				if serviceKey == hostKey {
					log.Infof("Aborting GC on node: %+v", host)
					return
				}
			}
		case <-timeout:
			log.Infof("Started GC for node: %+v", host)
			d.ofnetMaster.ClearNode(host)
			master.DeleteEndpoints(hostName)
			return
		}
	}
}

// getPluginAddress gets the adrress of the netplugin agent given the host name
func (d *MasterDaemon) getPluginAddress(hostName string) (string, error) {
	srvList, err := d.objdbClient.GetService("netplugin.vtep")
	if err != nil {
		log.Errorf("Error getting netplugin nodes. Err: %v", err)
		return "", err
	}

	for _, srv := range srvList {
		if srv.Hostname == hostName {
			return srv.HostAddr, nil
		}
	}

	return "", fmt.Errorf("Could not find plugin instance with name: %s", hostName)
}

// ClearEndpoints clears all the endpoints
func (d *MasterDaemon) ClearEndpoints(stateDriver core.StateDriver, epCfgs *[]core.State, id, matchField string) error {
	for _, epCfg := range *epCfgs {
		ep := epCfg.(*mastercfg.CfgEndpointState)
		if (matchField == "net" && ep.NetID == id) ||
			(matchField == "group" && ep.ServiceName == id) ||
			(matchField == "ep" && strings.Contains(ep.EndpointID, id)) {
			// Delete the endpoint state from netmaster
			_, err := master.DeleteEndpointID(stateDriver, ep.ID)
			if err != nil {
				return fmt.Errorf("Cannot cleanup EP: %s. Err: %+v", ep.EndpointID, err)
			}

			pluginAddress, err := d.getPluginAddress(ep.HomingHost)
			if err != nil {
				return err
			}

			epDelURL := "http://" + pluginAddress + ":9090/debug/reclaimEndpoint/" + ep.ID
			err = utils.HTTPDel(epDelURL)
			if err != nil {
				return fmt.Errorf("Error sending HTTP delete request to %s. Err: %+v", pluginAddress, err)
			}
		}
	}

	return nil
}

// registerRoutes registers HTTP route handlers
func (d *MasterDaemon) registerRoutes(router *mux.Router) {
	// Add REST routes
	s := router.Headers("Content-Type", "application/json").Methods("Post").Subrouter()

	s.HandleFunc("/plugin/allocAddress", utils.MakeHTTPHandler(master.AllocAddressHandler))
	s.HandleFunc("/plugin/releaseAddress", utils.MakeHTTPHandler(master.ReleaseAddressHandler))
	s.HandleFunc("/plugin/createEndpoint", utils.MakeHTTPHandler(master.CreateEndpointHandler))
	s.HandleFunc("/plugin/deleteEndpoint", utils.MakeHTTPHandler(master.DeleteEndpointHandler))
	s.HandleFunc("/plugin/updateEndpoint", utils.MakeHTTPHandler(master.UpdateEndpointHandler))

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
				core.Errorf("marshaling json failed. Error: %s", err).Error(),
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

	s = router.Methods("Delete").Subrouter()
	s.HandleFunc("/debug/epcleanup/tenant/{tenant}/{category}/{id}", func(w http.ResponseWriter, r *http.Request) {
		errStr := ""
		var epCfgs []core.State

		vars := mux.Vars(r)
		tenantName := vars["tenant"]
		category := vars["category"]
		id := vars["id"]

		// Get the state driver
		stateDriver, err := utils.GetStateDriver()
		if err != nil {
			log.Errorf("error getting state drive. Error: %+v", err)
			return
		}

		switch category {
		case "net":
			errStr = fmt.Sprintf("Received request to cleanup Network with ID: %s", id)
			nwKey := mastercfg.GetNwCfgKey(id, tenantName)
			nwCfg := &mastercfg.CfgNetworkState{}
			nwCfg.StateDriver = stateDriver
			err = nwCfg.Read(nwKey)
			if err != nil {
				log.Errorf("error reading network: %s. Error: %s", nwKey, err)
				return
			}

			if nwCfg.EpCount == 0 {
				return
			}
			readEp := &mastercfg.CfgEndpointState{}
			readEp.StateDriver = stateDriver
			epCfgs, err = readEp.ReadAll()
			if err != nil {
				log.Errorf("Could not read eps for network: %s. Err: %v", id, err)
				return
			}

			id = id + "." + tenantName
		case "group":
			errStr = fmt.Sprintf("Received request to cleanup EPG with ID: %s", id)

			epgKey := mastercfg.GetEndpointGroupKey(id, tenantName)
			epgCfg := &mastercfg.EndpointGroupState{}
			epgCfg.StateDriver = stateDriver
			err = epgCfg.Read(epgKey)
			if err != nil {
				log.Errorf("error reading EPG: %s. Error: %s", epgKey, err)
				return
			}

			if epgCfg.EpCount == 0 {
				return
			}
			readEp := &mastercfg.CfgEndpointState{}
			readEp.StateDriver = stateDriver
			epCfgs, err = readEp.ReadAll()
			if err != nil {
				log.Errorf("Could not read eps for group: %s. Err: %v", id, err)
				return
			}
		case "ep":
			errStr = fmt.Sprintf("Received request to cleanup Endpoint with ID: %s", id)
			readEp := &mastercfg.CfgEndpointState{}
			readEp.StateDriver = stateDriver
			epCfgs, err = readEp.ReadAll()
			if err != nil {
				log.Errorf("Could not read eps for group: %s. Err: %v", id, err)
				return
			}
		default:
			errStr = fmt.Sprintf("Unknown category error")
			return
		}
		err = d.ClearEndpoints(stateDriver, &epCfgs, id, category)
		if err != nil {
			log.Errorf("Error during ClearEndpoints. Err: %+v", err)
			return
		}
		http.Error(w, errStr, http.StatusOK)
		return
	})
}

func getEpName(networkName string, ep *intent.ConfigEP) string {
	if ep.Container != "" {
		return networkName + "-" + ep.Container
	}

	return ep.Host + "-native-intf"
}

// runLeader runs leader loop
func (d *MasterDaemon) runLeader() {
	router := mux.NewRouter()

	// Create a new api controller
	apiConfig := &objApi.APIControllerConfig{
		NetForwardMode: d.NetForwardMode,
		NetInfraType:   d.NetInfraType,
	}
	d.apiController = objApi.NewAPIController(router, d.objdbClient, apiConfig)

	//Restore state from clusterStore
	d.restoreCache()

	// Register netmaster service
	d.registerService()

	// initialize policy manager
	mastercfg.InitPolicyMgr(d.stateDriver, d.ofnetMaster)

	// setup HTTP routes
	d.registerRoutes(router)

	d.startListeners(router, d.stopLeaderChan)

	log.Infof("Exiting Leader mode")
}

// runFollower runs the follower FSM loop
func (d *MasterDaemon) runFollower() {
	router := mux.NewRouter()
	router.PathPrefix("/").HandlerFunc(slaveProxyHandler)

	// Register netmaster service
	d.registerService()

	// just wait on stop channel
	log.Infof("Listening in follower mode")
	d.startListeners(router, d.stopFollowerChan)

	log.Info("Exiting follower mode")
}

func (d *MasterDaemon) startListeners(router *mux.Router, stopChan chan bool) {
	// acquire listener mutex
	d.listenerMutex.Lock()
	defer d.listenerMutex.Unlock()

	// Create HTTP server and listener
	server := &http.Server{Handler: router}
	server.SetKeepAlivesEnabled(false)

	// bind on external address
	listener, err := net.Listen("tcp", d.ListenURL)
	if nil != err {
		log.Fatalln(err)
	}
	log.Infof("Netmaster listening on %s", d.ListenURL)
	listener = utils.ListenWrapper(listener)
	defer listener.Close()

	go server.Serve(listener)

	if d.ControlURL != d.ListenURL {
		externalAddr := strings.Split(d.ListenURL, ":")
		internalAddr := strings.Split(d.ControlURL, ":")
		if externalAddr[0] == "0.0.0.0" && externalAddr[1] == internalAddr[1] {
			// ignore internal bind if external and internal are on the same port and external bind on 0.0.0.0
			log.Infof("Ignore creating API listener on %q because %q covers it", d.ControlURL, d.ListenURL)
		} else {
			// it should fail-fast if ControlURL and ListenURL have other overlapping
			ctrlListener, err := net.Listen("tcp", d.ControlURL)
			if nil != err {
				log.Fatalln(err)
			}
			log.Infof("Netmaster listening on %s for control packets", d.ControlURL)
			ctrlListener = utils.ListenWrapper(ctrlListener)
			defer ctrlListener.Close()

			// start server
			go server.Serve(ctrlListener)
		}
	}

	// Wait till we are asked to stop
	<-stopChan
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

// InitServices init watch services
func (d *MasterDaemon) InitServices() {
	if d.ClusterMode == "kubernetes" {
		isLeader := func() bool {
			return d.currState == "leader"
		}
		networkpolicy.InitK8SServiceWatch(d.ControlURL, isLeader)
	}
}

// RunMasterFsm runs netmaster FSM
func (d *MasterDaemon) RunMasterFsm() {
	var err error

	masterURL := strings.Split(d.ControlURL, ":")
	masterIP, masterPort := masterURL[0], masterURL[1]
	if len(masterURL) != 2 {
		log.Fatalf("Invalid netmaster URL")
	}

	// create new ofnet master
	d.ofnetMaster = ofnet.NewOfnetMaster(masterIP, ofnet.OFNET_MASTER_PORT)
	if d.ofnetMaster == nil {
		log.Fatalf("Error creating ofnet master")
	}

	// Register all existing netplugins in the background
	go d.agentDiscoveryLoop()

	// Create the lock
	leaderLock, err = d.objdbClient.NewLock("netmaster/leader", masterIP+":"+masterPort, leaderLockTTL)
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
	localIP, err := netutils.GetDefaultAddr()
	if err != nil {
		return nil, errors.New("error getting local IP address")
	}

	// get current holder of master lock
	leader := leaderLock.GetHolder()
	if leader == "" {
		return nil, errors.New("leader not found")
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
