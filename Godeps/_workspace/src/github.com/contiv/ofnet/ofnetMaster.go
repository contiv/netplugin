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
	"errors"
	"fmt"
	"net"
	"net/rpc"
	"sync"
	"time"

	"github.com/contiv/ofnet/rpcHub"

	log "github.com/Sirupsen/logrus"
)

// Ofnet master state
type OfnetMaster struct {
	myAddr      string       // Address where we are listening
	myPort      uint16       // port where we are listening
	rpcServer   *rpc.Server  // json-rpc server
	rpcListener net.Listener // Listener
	masterMutex sync.Mutex   // Mutex to lock master datastructures

	// Database of agent nodes
	agentDb map[string]*OfnetNode

	// Endpoint database
	endpointDb map[string]*OfnetEndpoint

	// Policy database
	policyDb map[string]*OfnetPolicyRule
}

// Create new Ofnet master
func NewOfnetMaster(myAddr string, portNo uint16) *OfnetMaster {
	// Create the master
	master := new(OfnetMaster)

	// Init params
	master.myAddr = myAddr
	master.myPort = portNo
	master.agentDb = make(map[string]*OfnetNode)
	master.endpointDb = make(map[string]*OfnetEndpoint)
	master.policyDb = make(map[string]*OfnetPolicyRule)

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

// AddNode adds a node by calling MasterAdd rpc call on the node
func (self *OfnetMaster) AddNode(hostInfo OfnetNode) error {
	var resp bool

	// my info
	myInfo := new(OfnetNode)
	myInfo.HostAddr = self.myAddr
	myInfo.HostPort = self.myPort

	client := rpcHub.Client(hostInfo.HostAddr, hostInfo.HostPort)
	err := client.Call("OfnetAgent.AddMaster", myInfo, &resp)
	if err != nil {
		log.Errorf("Error calling AddMaster rpc call on node %v. Err: %v", hostInfo, err)
		return err
	}

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
	self.masterMutex.Lock()
	self.agentDb[hostKey] = node
	self.masterMutex.Unlock()

	log.Infof("Registered node: %+v", node)

	// Send all existing endpoints to the new node
	for _, endpoint := range self.endpointDb {
		if node.HostAddr != endpoint.OriginatorIp.String() {
			var resp bool

			log.Infof("Sending endpoint: %+v to node %s:%d", endpoint, node.HostAddr, node.HostPort)

			client := rpcHub.Client(node.HostAddr, node.HostPort)
			err := client.Call("OfnetAgent.EndpointAdd", endpoint, &resp)
			if err != nil {
				log.Errorf("Error adding endpoint to %s. Err: %v", node.HostAddr, err)
				// continue sending other endpoints
			}
		}
	}

	// Send all existing policy rules to the new node
	for _, rule := range self.policyDb {
		var resp bool

		log.Infof("Sending rule: %+v to node %s:%d", rule, node.HostAddr, node.HostPort)

		client := rpcHub.Client(node.HostAddr, node.HostPort)
		err := client.Call("PolicyAgent.AddRule", rule, &resp)
		if err != nil {
			log.Errorf("Error adding rule to %s. Err: %v", node.HostAddr, err)
			// continue sending other rules
		}
	}

	return nil
}

// Add an Endpoint
func (self *OfnetMaster) EndpointAdd(ep *OfnetEndpoint, ret *bool) error {

	log.Infof("Received Endpoint CReate from Remote netplugin")
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

			log.Infof("Sending endpoint: %+v to node %s:%d", ep, node.HostAddr, node.HostPort)

			client := rpcHub.Client(node.HostAddr, node.HostPort)
			err := client.Call("OfnetAgent.EndpointAdd", ep, &resp)
			if err != nil {
				log.Errorf("Error adding endpoint to %s. Err: %v", node.HostAddr, err)
				// Continue sending the message to other nodes
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
		log.Errorf("Received endpoint DELETE on a non existing endpoint %+v", ep)
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

			log.Infof("Sending DELETE endpoint: %+v to node %s:%d", ep, node.HostAddr, node.HostPort)

			client := rpcHub.Client(node.HostAddr, node.HostPort)
			err := client.Call("OfnetAgent.EndpointDel", ep, &resp)
			if err != nil {
				log.Errorf("Error sending DELERE endpoint to %s. Err: %v", node.HostAddr, err)
				// Continue sending the message to other nodes
			}
		}
	}

	*ret = true
	return nil
}

// AddRule adds a new rule to the policyDB
func (self *OfnetMaster) AddRule(rule *OfnetPolicyRule) error {
	// Check if we have the rule already
	if self.policyDb[rule.RuleId] != nil {
		return errors.New("Rule already exists")
	}

	// Save the rule in DB
	self.policyDb[rule.RuleId] = rule

	// Publish it to all agents except where it came from
	for _, node := range self.agentDb {
		var resp bool

		log.Infof("Sending rule: %+v to node %s:%d", rule, node.HostAddr, node.HostPort)

		client := rpcHub.Client(node.HostAddr, node.HostPort)
		err := client.Call("PolicyAgent.AddRule", rule, &resp)
		if err != nil {
			log.Errorf("Error adding rule to %s. Err: %v", node.HostAddr, err)
			// Continue sending the message to other nodes
		}
	}

	return nil
}

// DelRule removes a rule from policy DB
func (self *OfnetMaster) DelRule(rule *OfnetPolicyRule) error {
	// Check if we have the rule
	if self.policyDb[rule.RuleId] == nil {
		return errors.New("Rule does not exist")
	}

	// Remove the rule from DB
	delete(self.policyDb, rule.RuleId)

	// Publish it to all agents except where it came from
	for _, node := range self.agentDb {
		var resp bool

		log.Infof("Sending DELETE rule: %+v to node %s", rule, node.HostAddr)

		client := rpcHub.Client(node.HostAddr, node.HostPort)
		err := client.Call("PolicyAgent.DelRule", rule, &resp)
		if err != nil {
			log.Errorf("Error adding rule to %s. Err: %v", node.HostAddr, err)
			// Continue sending the message to other nodes
		}
	}

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
			// Continue sending the message to other nodes
		}
	}

	return nil
}
