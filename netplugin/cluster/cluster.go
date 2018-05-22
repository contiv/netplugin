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

package cluster

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netplugin/plugin"
	"github.com/contiv/netplugin/objdb"
	"github.com/contiv/netplugin/utils"

	log "github.com/Sirupsen/logrus"
)

// This file implements netplugin <-> netmaster clustering

const (
	netmasterRPCPort  = 9001
	netpluginRPCPort1 = 9002
	netpluginRPCPort2 = 9003
)

// ObjdbClient client
var ObjdbClient objdb.API

// MasterDB is Database of Master nodes
var MasterDB = make(map[string]*objdb.ServiceInfo)

func masterKey(srvInfo objdb.ServiceInfo) string {
	return srvInfo.HostAddr + ":" + fmt.Sprintf("%d", srvInfo.Port)
}

// Add a master node
func addMaster(netplugin *plugin.NetPlugin, srvInfo objdb.ServiceInfo) error {
	// save it in db
	MasterDB[masterKey(srvInfo)] = &srvInfo
	// tell the plugin about the master
	return netplugin.AddMaster(core.ServiceInfo{
		HostAddr: srvInfo.HostAddr,
		Port:     netmasterRPCPort,
	})
}

// delete master node
func deleteMaster(netplugin *plugin.NetPlugin, srvInfo objdb.ServiceInfo) error {
	// delete from the db
	delete(MasterDB, masterKey(srvInfo))

	// tel plugin about it
	return netplugin.DeleteMaster(core.ServiceInfo{
		HostAddr: srvInfo.HostAddr,
		Port:     netmasterRPCPort,
	})
}

// getMasterLockHolder returns the IP of current master lock hoder
func getMasterLockHolder() (string, error) {
	// Create the lock
	leaderLock, err := ObjdbClient.NewLock("netmaster/leader", "", 0)
	if err != nil {
		log.Fatalf("Could not create leader lock. Err: %v", err)
	}

	// get current holder of leader lock
	masterNode := leaderLock.GetHolder()
	if masterNode == "" {
		log.Errorf("No leader node found")
		return "", errors.New("no leader node")
	}

	return masterNode, nil
}

// GetLeaderNetmaster returns the IP of current leader netmaster
func GetLeaderNetmaster() (string, error) {
	return getMasterLockHolder()
}

// masterReq makes a POST/DELETE request to master node
func masterReq(path string, req interface{}, resp interface{}, isDel bool) error {
	const retryCount = 3

	reqType := "POST"
	if isDel {
		reqType = "DELETE"
	}
	// first find the holder of master lock
	masterNode, err := getMasterLockHolder()
	if err == nil {
		url := "http://" + masterNode + path
		log.Infof("Making REST request to url: %s", url)

		// Make the REST call to master
		for i := 0; i < retryCount; i++ {

			if isDel {
				err = utils.HTTPDel(url)
			} else {
				err = utils.HTTPPost(url, req, resp)
			}
			if err != nil && strings.Contains(err.Error(), "connection refused") {
				log.Warnf("Error making POST request. Retrying...: Err: %v", err)
				// Wait a little before retrying
				time.Sleep(time.Second)
				continue
			} else if err != nil {
				log.Errorf("Error making %s request: Err: %v"+
					"with resp:%+v", reqType, err, resp)
				return err
			}

			return err
		}

		return err
	}

	// Walk all netmasters and see if any of them respond
	for _, master := range MasterDB {
		masterPort := strconv.Itoa(master.Port)
		url := "http://" + master.HostAddr + ":" + masterPort + path

		log.Infof("Making REST request to url: %s", url)

		if isDel {
			err = utils.HTTPDel(url)
		} else {
			err = utils.HTTPPost(url, req, resp)
		}
		if err != nil {
			log.Warnf("Error making %s request: Err: %v", reqType, err)
			// continue and try making POST call to next master
		} else {
			return nil
		}
	}

	log.Errorf("error making %s request. all masters failed", reqType)
	return fmt.Errorf("the %s request failed", reqType)
}

// MasterPostReq makes a POST request to master node
func MasterPostReq(path string, req interface{}, resp interface{}) error {
	return masterReq(path, req, resp, false)
}

// MasterDelReq makes a DELETE request to master node
func MasterDelReq(path string) error {
	return masterReq(path, nil, nil, true)
}

// Register netplugin with service registry
func registerService(objClient objdb.API, ctrlIP, vtepIP, hostname string, vxlanUDPPort int) error {
	// netplugin service info
	srvInfo := objdb.ServiceInfo{
		ServiceName: "netplugin",
		TTL:         10,
		HostAddr:    ctrlIP,
		Port:        netpluginRPCPort1,
		Hostname:    hostname,
	}

	// Register the node with service registry
	err := objClient.RegisterService(srvInfo)
	if err != nil {
		log.Fatalf("Error registering service. Err: %v", err)
		return err
	}

	srvInfo = objdb.ServiceInfo{
		ServiceName: "netplugin",
		TTL:         10,
		HostAddr:    ctrlIP,
		Port:        netpluginRPCPort2,
		Hostname:    hostname,
	}

	// Register the node with service registry
	err = objClient.RegisterService(srvInfo)
	if err != nil {
		log.Fatalf("Error registering service. Err: %v", err)
		return err
	}

	// netplugn VTEP service info
	srvInfo = objdb.ServiceInfo{
		ServiceName: "netplugin.vtep",
		TTL:         10,
		HostAddr:    vtepIP,
		Port:        vxlanUDPPort,
		Hostname:    hostname,
	}

	// Register the node with service registry
	err = objClient.RegisterService(srvInfo)
	if err != nil {
		log.Fatalf("Error registering service. Err: %v", err)
		return err
	}

	log.Infof("Registered netplugin service with registry")
	return nil
}

// Main loop to discover peer hosts and masters
func peerDiscoveryLoop(netplugin *plugin.NetPlugin, objClient objdb.API, ctrlIP, vtepIP string) {
	// Create channels for watch thread
	nodeEventCh := make(chan objdb.WatchServiceEvent, 1)
	watchStopCh := make(chan bool, 1)
	masterEventCh := make(chan objdb.WatchServiceEvent, 1)
	masterWatchStopCh := make(chan bool, 1)

	// Start a watch on netmaster
	err := objClient.WatchService("netmaster.rpc", masterEventCh, masterWatchStopCh)
	if err != nil {
		log.Fatalf("Could not start a watch on netmaster service. Err: %v", err)
	}

	// Start a watch on netplugin service
	err = objClient.WatchService("netplugin.vtep", nodeEventCh, watchStopCh)
	if err != nil {
		log.Fatalf("Could not start a watch on netplugin service. Err: %v", err)
	}

	for {
		select {
		case srvEvent := <-nodeEventCh:
			log.Debugf("Received netplugin service watch event: %+v", srvEvent)

			// collect the info about the node
			nodeInfo := srvEvent.ServiceInfo

			// check if its our own info coming back to us
			if nodeInfo.HostAddr == vtepIP {
				break
			}

			// Handle based on event type
			if srvEvent.EventType == objdb.WatchServiceEventAdd {
				log.Infof("Node add event for {%+v}", nodeInfo)

				// add the node
				err := netplugin.AddPeerHost(core.ServiceInfo{
					HostAddr: nodeInfo.HostAddr,
					Port:     netplugin.PluginConfig.Instance.VxlanUDPPort,
				})
				if err != nil {
					log.Errorf("Error adding node {%+v}. Err: %v", nodeInfo, err)
				}
			} else if srvEvent.EventType == objdb.WatchServiceEventDel {
				log.Infof("Node delete event for {%+v}", nodeInfo)

				// remove the node
				err := netplugin.DeletePeerHost(core.ServiceInfo{
					HostAddr: nodeInfo.HostAddr,
					Port:     netplugin.PluginConfig.Instance.VxlanUDPPort,
				})
				if err != nil {
					log.Errorf("Error deleting node {%+v}. Err: %v", nodeInfo, err)
				}
			}
		case srvEvent := <-masterEventCh:
			log.Infof("Received netmaster service watch event: %+v", srvEvent)

			// collect the info about the node
			nodeInfo := srvEvent.ServiceInfo

			// Handle based on event type
			if srvEvent.EventType == objdb.WatchServiceEventAdd {
				log.Infof("Master add event for {%+v}", nodeInfo)

				// Add the master
				err := addMaster(netplugin, nodeInfo)
				if err != nil {
					log.Errorf("Error adding master {%+v}. Err: %v", nodeInfo, err)
				}
			} else if srvEvent.EventType == objdb.WatchServiceEventDel {
				log.Infof("Master delete event for {%+v}", nodeInfo)

				// Delete the master
				err := deleteMaster(netplugin, nodeInfo)
				if err != nil {
					log.Errorf("Error deleting master {%+v}. Err: %v", nodeInfo, err)
				}
			}
		}

		// Dont process next peer event for another 100ms
		time.Sleep(100 * time.Millisecond)
	}
}

// Init initializes the cluster module
func Init(storeDriver string, storeURLs []string) error {
	var err error

	// Create an objdb client
	ObjdbClient, err = objdb.InitClient(storeDriver, storeURLs)

	return err
}

// RunLoop registers netplugin service with cluster store and runs peer discovery
func RunLoop(netplugin *plugin.NetPlugin, ctrlIP, vtepIP, hostname string) error {
	// Register ourselves
	err := registerService(ObjdbClient, ctrlIP, vtepIP, hostname, netplugin.PluginConfig.Instance.VxlanUDPPort)

	// Start peer discovery loop
	go peerDiscoveryLoop(netplugin, ObjdbClient, ctrlIP, vtepIP)

	return err
}
