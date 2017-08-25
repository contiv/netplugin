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
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/rpc"
	"sync"
	"time"

	"github.com/contiv/ofnet/rpcHub"

	log "github.com/Sirupsen/logrus"
)

// per agent stats
type ofnetAgentStats struct {
	Stats map[string]uint64
}

// Ofnet master state
type OfnetMaster struct {
	myAddr      string       // Address where we are listening
	myPort      uint16       // port where we are listening
	rpcServer   *rpc.Server  // json-rpc server
	rpcListener net.Listener // Listener
	masterMutex sync.RWMutex // Mutex to lock master datastructures
	statsMutex  sync.Mutex   // Mutex to protect stats update

	// Database of agent nodes
	agentDb map[string]*OfnetNode

	// Endpoint database
	endpointDb map[string]*OfnetEndpoint

	// Policy database
	policyDb map[string]*OfnetPolicyRule

	// agent stats
	agentStats map[string]*ofnetAgentStats
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

// incrAgentStats increments an agent key
func (self *OfnetMaster) incrAgentStats(hostKey, statName string) {
	self.statsMutex.Lock()
	defer self.statsMutex.Unlock()

	// lookup the agent
	agentStats := self.agentStats[hostKey]
	if agentStats == nil {
		return
	}

	// increment the stats
	currStats := agentStats.Stats[statName]
	currStats++
	agentStats.Stats[statName] = currStats
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

// ClearNode clears up the resources on the node for GC
func (self *OfnetMaster) ClearNode(host OfnetNode) error {
	self.masterMutex.Lock()
	// Remove existing endpoints from the node
	for _, ep := range self.endpointDb {
		if host.HostAddr == ep.OriginatorIp.String() {
			var resp bool
			for _, node := range self.agentDb {
				if host.HostAddr == node.HostAddr {
					continue
				}
				log.Infof("Removing endpoint: %+v from node %s:%d", ep, node.HostAddr, node.HostPort)

				client := rpcHub.Client(node.HostAddr, node.HostPort)
				err := client.Call("OfnetAgent.EndpointDel", ep, &resp)
				if err != nil {
					log.Errorf("Error removing endpoint from %s. Err: %v", node.HostAddr, err)
					// continue sending other endpoint delete
				}
				delete(self.endpointDb, ep.EndpointID)
			}
		}
	}
	self.masterMutex.Unlock()

	return nil
}

// RegisterNode registers an agent
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

	// take a read lock for accessing db
	self.masterMutex.RLock()
	defer self.masterMutex.RUnlock()

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

	// increment stats
	self.incrAgentStats(hostKey, "registered")

	return nil
}

// UnRegisterNode unregisters an agent
func (self *OfnetMaster) UnRegisterNode(hostInfo *OfnetNode, ret *bool) error {
	hostKey := fmt.Sprintf("%s:%d", hostInfo.HostAddr, hostInfo.HostPort)

	// Add it to DB
	self.masterMutex.Lock()
	delete(self.agentDb, hostKey)
	self.masterMutex.Unlock()
	rpcHub.DisconnectClient(hostInfo.HostPort, hostInfo.HostAddr)

	// increment stats
	self.incrAgentStats(hostKey, "unregistered")

	return nil
}

// Add an Endpoint
func (self *OfnetMaster) EndpointAdd(ep *OfnetEndpoint, ret *bool) error {

	log.Infof("Received Endpoint CReate from Remote netplugin")
	// Check if we have the endpoint already and which is more recent
	self.masterMutex.RLock()
	oldEp := self.endpointDb[ep.EndpointID]
	self.masterMutex.RUnlock()
	if oldEp != nil {
		// If old endpoint has more recent timestamp, nothing to do
		if !ep.Timestamp.After(oldEp.Timestamp) {
			return nil
		}
	}

	// Save the endpoint in DB
	self.masterMutex.Lock()
	self.endpointDb[ep.EndpointID] = ep
	self.masterMutex.Unlock()

	// take a read lock for accessing db
	self.masterMutex.RLock()
	defer self.masterMutex.RUnlock()

	// Publish it to all agents except where it came from
	for nodeKey, node := range self.agentDb {
		if node.HostAddr != ep.OriginatorIp.String() {
			var resp bool

			log.Infof("Sending endpoint: %+v to node %s:%d", ep, node.HostAddr, node.HostPort)

			client := rpcHub.Client(node.HostAddr, node.HostPort)
			err := client.Call("OfnetAgent.EndpointAdd", ep, &resp)
			if err != nil {
				log.Errorf("Error adding endpoint to %s. Err: %v", node.HostAddr, err)
				// Continue sending the message to other nodes

				// increment stats
				self.incrAgentStats(nodeKey, "EndpointAddFailure")
			} else {
				// increment stats
				self.incrAgentStats(nodeKey, "EndpointAddSent")
			}
		}
	}

	*ret = true
	return nil
}

// Delete an Endpoint
func (self *OfnetMaster) EndpointDel(ep *OfnetEndpoint, ret *bool) error {
	// Check if we have the endpoint, if we dont have the endpoint, nothing to do
	self.masterMutex.RLock()
	oldEp := self.endpointDb[ep.EndpointID]
	self.masterMutex.RUnlock()
	if oldEp == nil {
		log.Errorf("Received endpoint DELETE on a non existing endpoint %+v", ep)
		return nil
	}

	// If existing endpoint has more recent timestamp, nothing to do
	if oldEp.Timestamp.After(ep.Timestamp) {
		return nil
	}

	// Delete the endpoint from DB
	self.masterMutex.Lock()
	delete(self.endpointDb, ep.EndpointID)
	self.masterMutex.Unlock()

	// take a read lock for accessing db
	self.masterMutex.RLock()
	defer self.masterMutex.RUnlock()

	// Publish it to all agents except where it came from
	for nodeKey, node := range self.agentDb {
		if node.HostAddr != ep.OriginatorIp.String() {
			var resp bool

			log.Infof("Sending DELETE endpoint: %+v to node %s:%d", ep, node.HostAddr, node.HostPort)

			client := rpcHub.Client(node.HostAddr, node.HostPort)
			err := client.Call("OfnetAgent.EndpointDel", ep, &resp)
			if err != nil {
				log.Errorf("Error sending DELERE endpoint to %s. Err: %v", node.HostAddr, err)
				// Continue sending the message to other nodes

				// increment stats
				self.incrAgentStats(nodeKey, "EndpointDelFailure")
			} else {
				// increment stats
				self.incrAgentStats(nodeKey, "EndpointDelSent")
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
	self.masterMutex.Lock()
	self.policyDb[rule.RuleId] = rule
	self.masterMutex.Unlock()

	// take a read lock for accessing db
	self.masterMutex.RLock()
	defer self.masterMutex.RUnlock()

	// Publish it to all agents
	for nodeKey, node := range self.agentDb {
		var resp bool

		log.Infof("Sending rule: %+v to node %s:%d", rule, node.HostAddr, node.HostPort)

		client := rpcHub.Client(node.HostAddr, node.HostPort)
		err := client.Call("PolicyAgent.AddRule", rule, &resp)
		if err != nil {
			log.Errorf("Error adding rule to %s. Err: %v", node.HostAddr, err)
			// Continue sending the message to other nodes

			// increment stats
			self.incrAgentStats(nodeKey, "AddRuleFailure")
		} else {
			// increment stats
			self.incrAgentStats(nodeKey, "AddRuleSent")
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
	self.masterMutex.Lock()
	delete(self.policyDb, rule.RuleId)
	self.masterMutex.Unlock()

	// take a read lock for accessing db
	self.masterMutex.RLock()
	defer self.masterMutex.RUnlock()

	// Publish it to all agents
	for nodeKey, node := range self.agentDb {
		var resp bool

		log.Infof("Sending DELETE rule: %+v to node %s", rule, node.HostAddr)

		client := rpcHub.Client(node.HostAddr, node.HostPort)
		err := client.Call("PolicyAgent.DelRule", rule, &resp)
		if err != nil {
			log.Errorf("Error adding rule to %s. Err: %v", node.HostAddr, err)
			// Continue sending the message to other nodes

			// increment stats
			self.incrAgentStats(nodeKey, "DelRuleFailure")
		} else {
			// increment stats
			self.incrAgentStats(nodeKey, "DelRuleSent")
		}
	}

	return nil
}

// Make a dummy RPC call to all agents. for testing purposes..
func (self *OfnetMaster) MakeDummyRpcCall() error {
	// Call all agents
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

// InjectGARPs triggers GARPS in the datapath on the specified epg
func (self *OfnetMaster) InjectGARPs(epgID int) {
	// take a read lock for accessing db
	self.masterMutex.RLock()
	defer self.masterMutex.RUnlock()

	// Send to all agents
	for nodeKey, node := range self.agentDb {
		var resp bool

		client := rpcHub.Client(node.HostAddr, node.HostPort)
		err := client.Call("OfnetAgent.InjectGARPs", epgID, &resp)
		if err != nil {
			log.Errorf("Error triggering GARP on %s. Err: %v", node.HostAddr, err)

			// increment stats
			self.incrAgentStats(nodeKey, "InjectGARPsFailure")
		} else {
			// increment stats
			self.incrAgentStats(nodeKey, "InjectGARPsSent")
		}
	}
}

// InspectState returns current state as json
func (self *OfnetMaster) InspectState() ([]byte, error) {
	// convert ofnet struct to an exported struct for json marshaling
	ofnetExport := struct {
		MyAddr     string                      // Address where we are listening
		MyPort     uint16                      // port where we are listening
		AgentDb    map[string]*OfnetNode       // Database of agent nodes
		EndpointDb map[string]*OfnetEndpoint   // Endpoint database
		PolicyDb   map[string]*OfnetPolicyRule // Policy database
		AgentStats map[string]*ofnetAgentStats // Agent stats
	}{
		self.myAddr,
		self.myPort,
		self.agentDb,
		self.endpointDb,
		self.policyDb,
		self.agentStats,
	}

	// convert struct to json
	jsonStats, err := json.Marshal(ofnetExport)
	if err != nil {
		log.Errorf("Error encoding ofnet master state. Err: %v", err)
		return []byte{}, err
	}

	return jsonStats, nil
}
