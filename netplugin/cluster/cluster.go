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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netplugin/plugin"
	"github.com/contiv/netplugin/utils/netutils"
	"github.com/contiv/objdb"

	log "github.com/Sirupsen/logrus"
)

// This file implements netplugin <-> netmaster clustering

const (
	netmasterRPCPort = 9001
	netpluginRPCPort = 9002
	vxlanUDPPort     = 4789
)

// objdb client
var objdbClient objdb.API

// Database of master nodes
var masterDB = make(map[string]*objdb.ServiceInfo)

func masterKey(srvInfo objdb.ServiceInfo) string {
	return srvInfo.HostAddr + ":" + fmt.Sprintf("%d", srvInfo.Port)
}

// Add a master node
func addMaster(netplugin *plugin.NetPlugin, srvInfo objdb.ServiceInfo) error {
	// save it in db
	masterDB[masterKey(srvInfo)] = &srvInfo

	// tell the plugin about the master
	return netplugin.AddMaster(core.ServiceInfo{
		HostAddr: srvInfo.HostAddr,
		Port:     netmasterRPCPort,
	})
}

// delete master node
func deleteMaster(netplugin *plugin.NetPlugin, srvInfo objdb.ServiceInfo) error {
	// delete from the db
	delete(masterDB, masterKey(srvInfo))

	// tel plugin about it
	return netplugin.DeleteMaster(core.ServiceInfo{
		HostAddr: srvInfo.HostAddr,
		Port:     srvInfo.Port,
	})
}

// httpPost performs http POST operation
func httpPost(url string, req interface{}, resp interface{}) error {
	// Convert the req to json
	jsonStr, err := json.Marshal(req)
	if err != nil {
		log.Errorf("Error converting request data(%#v) to Json. Err: %v", req, err)
		return err
	}

	// Perform HTTP POST operation
	res, err := http.Post(url, "application/json", strings.NewReader(string(jsonStr)))
	if err != nil {
		log.Errorf("Error during http get. Err: %v", err)
		return err
	}

	// Check the response code
	if res.StatusCode != http.StatusOK {
		log.Errorf("HTTP error response. Status: %s, StatusCode: %d", res.Status, res.StatusCode)
		return errors.New("HTTP Error response")
	}

	// Read the entire response
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Errorf("Error during ioutil readall. Err: %v", err)
		return err
	}

	// Convert response json to struct
	err = json.Unmarshal(body, resp)
	if err != nil {
		log.Errorf("Error during json unmarshall. Err: %v", err)
		return err
	}

	log.Infof("Results for (%s): %+v\n", url, resp)

	return nil
}

// getMasterLockHolder returns the IP of current master lock hoder
func getMasterLockHolder() (string, error) {
	// Create the lock
	leaderLock, err := objdbClient.NewLock("netmaster/leader", "", 0)
	if err != nil {
		log.Fatalf("Could not create leader lock. Err: %v", err)
	}

	// get current holder of leader lock
	masterNode := leaderLock.GetHolder()
	if masterNode == "" {
		log.Errorf("No leader node found")
		return "", errors.New("No leader node")
	}

	return masterNode, nil
}

// MasterPostReq makes a POST request to master node
func MasterPostReq(path string, req interface{}, resp interface{}) error {
	// first find the holder of master lock
	masterNode, err := getMasterLockHolder()
	if err == nil {
		url := "http://" + masterNode + ":9999" + path
		log.Infof("Making REST request to url: %s", url)

		// Make the REST call to master
		err := httpPost(url, req, resp)
		if err != nil {
			log.Errorf("Error making POST request: Err: %v", err)
		}

		return err
	}

	// Walk all netmasters and see if any of them respond
	for _, master := range masterDB {
		url := "http://" + master.HostAddr + ":9999" + path

		log.Infof("Making REST request to url: %s", url)

		err := httpPost(url, req, resp)
		if err != nil {
			log.Warnf("Error making POST request: Err: %v", err)
			// continue and try making POST call to next master
		} else {
			return nil
		}
	}

	log.Errorf("Error making POST request. All master failed")
	return errors.New("POST request failed")
}

// Register netplugin with service registry
func registerService(objClient objdb.API, ctrlIP, vtepIP string) error {
	// netplugin service info
	srvInfo := objdb.ServiceInfo{
		ServiceName: "netplugin",
		TTL:         10,
		HostAddr:    ctrlIP,
		Port:        netpluginRPCPort,
	}

	// Register the node with service registry
	err := objClient.RegisterService(srvInfo)
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
					Port:     vxlanUDPPort,
				})
				if err != nil {
					log.Errorf("Error adding node {%+v}. Err: %v", nodeInfo, err)
				}
			} else if srvEvent.EventType == objdb.WatchServiceEventDel {
				log.Infof("Node delete event for {%+v}", nodeInfo)

				// remove the node
				err := netplugin.DeletePeerHost(core.ServiceInfo{
					HostAddr: nodeInfo.HostAddr,
					Port:     vxlanUDPPort,
				})
				if err != nil {
					log.Errorf("Error adding node {%+v}. Err: %v", nodeInfo, err)
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
	}
}

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

// Init initializes the cluster module
func Init(netplugin *plugin.NetPlugin, ctrlIP, vtepIP, storeURL string) error {
	var err error

	// Create an objdb client
	objdbClient, err = objdb.NewClient(storeURL)
	if err != nil {
		return err
	}

	// Register ourselves
	registerService(objdbClient, ctrlIP, vtepIP)

	// Start peer discovery loop
	go peerDiscoveryLoop(netplugin, objdbClient, ctrlIP, vtepIP)

	return nil
}
