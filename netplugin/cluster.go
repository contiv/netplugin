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
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/plugin"
	"github.com/contiv/objmodel/objdb"
	"github.com/contiv/objmodel/objdb/client"
	"github.com/contiv/ofnet"

	log "github.com/Sirupsen/logrus"
)

// This file implements netplugin <-> netmaster clustering

// Register netplugin with service registry
func registerService(objdbClient objdb.ObjdbApi, opts cliOpts) error {
	// Get the address to be used for local communication
	localIP, err := objdbClient.GetLocalAddr()
	if err != nil {
		log.Fatalf("Error getting locla IP address. Err: %v", err)
		return err
	}

	// service info
	srvInfo := objdb.ServiceInfo{
		ServiceName: "netplugin",
		HostAddr:    localIP,
		Port:        ofnet.OFNET_AGENT_PORT,
	}

	// Register the node with service registry
	err = objdbClient.RegisterService(srvInfo)
	if err != nil {
		log.Fatalf("Error registering service. Err: %v", err)
		return err
	}

	log.Infof("Registered netplugin service with registry")
	return nil
}

// Main loop to discover peer hosts and masters
func peerDiscoveryLoop(netplugin *plugin.NetPlugin, objdbClient objdb.ObjdbApi) {
	// Create channels for watch thread
	nodeEventCh := make(chan objdb.WatchServiceEvent, 1)
	watchStopCh := make(chan bool, 1)
	masterEventCh := make(chan objdb.WatchServiceEvent, 1)
	masterWatchStopCh := make(chan bool, 1)

	// Get the local address
	localIP, err := objdbClient.GetLocalAddr()
	if err != nil {
		log.Fatalf("Error getting locla IP address. Err: %v", err)
	}

	// Start a watch on netplugin service so that we dont miss any
	err = objdbClient.WatchService("netplugin", nodeEventCh, watchStopCh)
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
		err := netplugin.AddMaster(core.ServiceInfo{
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

			// check if its our on info coming back to us
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
				err := netplugin.AddMaster(core.ServiceInfo{
					HostAddr: nodeInfo.HostAddr,
					Port:     ofnet.OFNET_MASTER_PORT,
				})
				if err != nil {
					log.Errorf("Error adding master {%+v}. Err: %v", nodeInfo, err)
				}
			} else if srvEvent.EventType == objdb.WatchServiceEventDel {
				log.Infof("Master delete event for {%+v}", nodeInfo)

				// Delete the master
				err := netplugin.DeleteMaster(core.ServiceInfo{
					HostAddr: nodeInfo.HostAddr,
					Port:     nodeInfo.Port,
				})
				if err != nil {
					log.Errorf("Error deletin master {%+v}. Err: %v", nodeInfo, err)
				}
			}
		}
	}
}

func clusterInit(netplugin *plugin.NetPlugin, opts cliOpts) error {
	// Create an objdb client
	objdbClient := client.NewClient()

	// Register ourselves
	registerService(objdbClient, opts)

	// Start peer discovery loop
	go peerDiscoveryLoop(netplugin, objdbClient)

	return nil
}
