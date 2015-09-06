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
package ofnet

// This file contains the ofnet master implementation

import (
	"fmt"
	"net"
	"net/rpc"
	"time"

	"github.com/contiv/ofnet/rpcHub"

	log "github.com/Sirupsen/logrus"
)

// Ofnet master state
type OfnetMaster struct {
	rpcServer   *rpc.Server  // json-rpc server
	rpcListener net.Listener // Listener

	// Database of agent nodes
	agentDb map[string]*OfnetNode

	// Endpoint database
	endpointDb map[string]*OfnetEndpoint
}

// Create new Ofnet master
func NewOfnetMaster(portNo uint16) *OfnetMaster {
	// Create the master
	master := new(OfnetMaster)

	// Init params
	master.agentDb = make(map[string]*OfnetNode)
	master.endpointDb = make(map[string]*OfnetEndpoint)

	// Create a new RPC server
	master.rpcServer, master.rpcListener = rpcHub.NewRpcServer(portNo)

	// Register RPC handler
	err := master.rpcServer.Register(master)
	if err != nil {
		log.Fatalf("Error Registering RPC callbacks. Err: %v", err)
		return nil
	}

	return master
}

// Delete closes rpc listener
func (self *OfnetMaster) Delete() error {
	self.rpcListener.Close()
	time.Sleep(100 * time.Millisecond)

	return nil
}

// Register an agent
func (self *OfnetMaster) RegisterNode(hostInfo *OfnetNode, ret *bool) error {
	// Create a node
	node := new(OfnetNode)
	node.HostAddr = hostInfo.HostAddr
	node.HostPort = hostInfo.HostPort

	hostKey := fmt.Sprintf("%s:%d", hostInfo.HostAddr, hostInfo.HostPort)

	// Add it to DB
	self.agentDb[hostKey] = node

	log.Infof("Registered node: %+v", node)

	// Send all existing endpoints to the new node
	for _, endpoint := range self.endpointDb {
		if node.HostAddr != endpoint.OriginatorIp.String() {
			var resp bool

			log.Infof("Sending endpoint: %+v to node %s", endpoint, node.HostAddr)

			client := rpcHub.Client(node.HostAddr, node.HostPort)
			err := client.Call("OfnetAgent.EndpointAdd", endpoint, &resp)
			if err != nil {
				log.Errorf("Error adding endpoint to %s. Err: %v", node.HostAddr, err)
			}
		}
	}

	return nil
}

// Add an Endpoint
func (self *OfnetMaster) EndpointAdd(ep *OfnetEndpoint, ret *bool) error {
	// Check if we have the endpoint already and which is more recent
	oldEp := self.endpointDb[ep.EndpointID]
	if oldEp != nil {
		// If old endpoint has more recent timestamp, nothing to do
		if !ep.Timestamp.After(oldEp.Timestamp) {
			return nil
		}
	}

	// Save the endpoint in DB
	self.endpointDb[ep.EndpointID] = ep

	// Publish it to all agents except where it came from
	for _, node := range self.agentDb {
		if node.HostAddr != ep.OriginatorIp.String() {
			var resp bool

			log.Infof("Sending endpoint: %+v to node %s", ep, node.HostAddr)

			client := rpcHub.Client(node.HostAddr, node.HostPort)
			err := client.Call("OfnetAgent.EndpointAdd", ep, &resp)
			if err != nil {
				log.Errorf("Error adding endpoint to %s. Err: %v", node.HostAddr, err)
				return err
			}
		}
	}

	*ret = true
	return nil
}

// Delete an Endpoint
func (self *OfnetMaster) EndpointDel(ep *OfnetEndpoint, ret *bool) error {
	// Check if we have the endpoint, if we dont have the endpoint, nothing to do
	oldEp := self.endpointDb[ep.EndpointID]
	if oldEp == nil {
		return nil
	}

	// If existing endpoint has more recent timestamp, nothing to do
	if oldEp.Timestamp.After(ep.Timestamp) {
		return nil
	}

	// Delete the endpoint from DB
	delete(self.endpointDb, ep.EndpointID)

	// Publish it to all agents except where it came from
	for _, node := range self.agentDb {
		if node.HostAddr != ep.OriginatorIp.String() {
			var resp bool

			log.Infof("Sending DELETE endpoint: %+v to node %s", ep, node.HostAddr)

			client := rpcHub.Client(node.HostAddr, node.HostPort)
			err := client.Call("OfnetAgent.EndpointDel", ep, &resp)
			if err != nil {
				log.Errorf("Error sending DELERE endpoint to %s. Err: %v", node.HostAddr, err)
				return err
			}
		}
	}

	*ret = true
	return nil
}

// Make a dummy RPC call to all agents. for testing purposes..
func (self *OfnetMaster) MakeDummyRpcCall() error {
	// Publish it to all agents except where it came from
	for _, node := range self.agentDb {
		var resp bool
		dummyArg := "dummy string"

		log.Infof("Making dummy rpc call to node %+v", node)

		client := rpcHub.Client(node.HostAddr, node.HostPort)
		err := client.Call("OfnetAgent.DummyRpc", &dummyArg, &resp)
		if err != nil {
			log.Errorf("Error making dummy rpc call to %+v. Err: %v", node, err)
			return err
		}
	}

	return nil
}
