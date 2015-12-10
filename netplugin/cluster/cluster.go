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
	"github.com/contiv/objmodel/objdb"
	"github.com/contiv/objmodel/objdb/client"
	"github.com/contiv/ofnet"

	log "github.com/Sirupsen/logrus"
)

// This file implements netplugin <-> netmaster clustering

// Database of master nodes
var masterDB = make(map[string]*core.ServiceInfo)

func masterKey(srvInfo core.ServiceInfo) string {
	return srvInfo.HostAddr + ":" + fmt.Sprintf("%d", srvInfo.Port)
}

// Add a master node
func addMaster(netplugin *plugin.NetPlugin, srvInfo core.ServiceInfo) error {
	// save it in db
	masterDB[masterKey(srvInfo)] = &srvInfo

	// tell the plugin about the master
	return netplugin.AddMaster(srvInfo)
}

// delete master node
func deleteMaster(netplugin *plugin.NetPlugin, srvInfo core.ServiceInfo) error {
	// delete from the db
	delete(masterDB, masterKey(srvInfo))

	// tel plugin about it
	return netplugin.DeleteMaster(srvInfo)
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

// MasterPostReq makes a POST request to master node
func MasterPostReq(path string, req interface{}, resp interface{}) error {
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
func registerService(objdbClient objdb.ObjdbApi, localIP string) error {
	// service info
	srvInfo := objdb.ServiceInfo{
		ServiceName: "netplugin",
		HostAddr:    localIP,
		Port:        ofnet.OFNET_AGENT_PORT,
	}

	// Register the node with service registry
	err := objdbClient.RegisterService(srvInfo)
	if err != nil {
		log.Fatalf("Error registering service. Err: %v", err)
		return err
	}

	log.Infof("Registered netplugin service with registry")
	return nil
}

// Main loop to discover peer hosts and masters
func peerDiscoveryLoop(netplugin *plugin.NetPlugin, objdbClient objdb.ObjdbApi, localIP string) {
	// Create channels for watch thread
	nodeEventCh := make(chan objdb.WatchServiceEvent, 1)
	watchStopCh := make(chan bool, 1)
	masterEventCh := make(chan objdb.WatchServiceEvent, 1)
	masterWatchStopCh := make(chan bool, 1)

	// Start a watch on netplugin service so that we dont miss any
	err := objdbClient.WatchService("netplugin", nodeEventCh, watchStopCh)
	if err != nil {
		log.Fatalf("Could not start a watch on netplugin service. Err: %v", err)
	}

	// Start a watch on netmaster too
	err = objdbClient.WatchService("netmaster", masterEventCh, masterWatchStopCh)
	if err != nil {
		log.Fatalf("Could not start a watch on netmaster service. Err: %v", err)
	}

	// Get a list of all existing netplugin nodes
	nodeList, err := objdbClient.GetService("netplugin")
	if err != nil {
		log.Errorf("Error getting node list from objdb. Err: %v", err)
	}

	log.Infof("Got netplugin service list: %+v", nodeList)

	// walk each node and add it as a PeerHost
	for _, node := range nodeList {
		// Ignore if its our own info
		if node.HostAddr == localIP {
			continue
		}
		// add the node
		err := netplugin.AddPeerHost(core.ServiceInfo{
			HostAddr: node.HostAddr,
			Port:     ofnet.OFNET_AGENT_PORT,
		})
		if err != nil {
			log.Errorf("Error adding node {%+v}. Err: %v", node, err)
		}
	}

	// Get a list of all existing netmasters
	masterList, err := objdbClient.GetService("netmaster")
	if err != nil {
		log.Errorf("Error getting master list from objdb. Err: %v", err)
	}

	log.Infof("Got netmaster service list: %+v", masterList)

	// Walk each master and add it
	for _, master := range masterList {
		// Add the master
		err := addMaster(netplugin, core.ServiceInfo{
			HostAddr: master.HostAddr,
			Port:     ofnet.OFNET_MASTER_PORT,
		})
		if err != nil {
			log.Errorf("Error adding master {%+v}. Err: %v", master, err)
		}
	}

	for {
		select {
		case srvEvent := <-nodeEventCh:
			log.Infof("Received netplugin service watch event: %+v", srvEvent)

			// collect the info about the node
			nodeInfo := srvEvent.ServiceInfo

			// check if its our own info coming back to us
			if nodeInfo.HostAddr == localIP {
				break
			}

			// Handle based on event type
			if srvEvent.EventType == objdb.WatchServiceEventAdd {
				log.Infof("Node add event for {%+v}", nodeInfo)

				// add the node
				err := netplugin.AddPeerHost(core.ServiceInfo{
					HostAddr: nodeInfo.HostAddr,
					Port:     ofnet.OFNET_AGENT_PORT,
				})
				if err != nil {
					log.Errorf("Error adding node {%+v}. Err: %v", nodeInfo, err)
				}
			} else if srvEvent.EventType == objdb.WatchServiceEventDel {
				log.Infof("Node delete event for {%+v}", nodeInfo)

				// remove the node
				err := netplugin.DeletePeerHost(core.ServiceInfo{
					HostAddr: nodeInfo.HostAddr,
					Port:     ofnet.OFNET_AGENT_PORT,
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
				err := addMaster(netplugin, core.ServiceInfo{
					HostAddr: nodeInfo.HostAddr,
					Port:     ofnet.OFNET_MASTER_PORT,
				})
				if err != nil {
					log.Errorf("Error adding master {%+v}. Err: %v", nodeInfo, err)
				}
			} else if srvEvent.EventType == objdb.WatchServiceEventDel {
				log.Infof("Master delete event for {%+v}", nodeInfo)

				// Delete the master
				err := deleteMaster(netplugin, core.ServiceInfo{
					HostAddr: nodeInfo.HostAddr,
					Port:     nodeInfo.Port,
				})
				if err != nil {
					log.Errorf("Error deleting master {%+v}. Err: %v", nodeInfo, err)
				}
			}
		}
	}
}

// GetLocalAddr gets local address to be used
func GetLocalAddr() (string, error) {
	// Get objdb's client IP
	clientIP, err := client.NewClient().GetLocalAddr()
	if err != nil {
		log.Warnf("Error getting local address from objdb. Returning first local address. Err: %v", err)

		return netutils.GetFirstLocalAddr()
	}

	// Make sure the ip address is local
	if netutils.IsAddrLocal(clientIP) {
		return clientIP, nil
	}

	// Return first available address if client IP is not local
	return netutils.GetFirstLocalAddr()
}

// Init initializes the cluster module
func Init(netplugin *plugin.NetPlugin, localIP string) error {
	// Create an objdb client
	objdbClient := client.NewClient()

	// Register ourselves
	registerService(objdbClient, localIP)

	// Start peer discovery loop
	go peerDiscoveryLoop(netplugin, objdbClient, localIP)

	return nil
}
