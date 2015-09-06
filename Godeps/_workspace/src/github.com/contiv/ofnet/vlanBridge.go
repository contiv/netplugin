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

// This file implements the vlan bridging datapath

import (
	"net"
	"net/rpc"

	"github.com/contiv/ofnet/ofctrl"

	log "github.com/Sirupsen/logrus"
)

// Vlan bridging currently uses native OVS bridging.
// This is mostly stub code.

// Vlan state.
type VlanBridge struct {
	agent    *OfnetAgent      // Pointer back to ofnet agent that owns this
	ofSwitch *ofctrl.OFSwitch // openflow switch we are talking to
}

// Create a new vxlan instance
func NewVlanBridge(agent *OfnetAgent, rpcServ *rpc.Server) *VlanBridge {
	vlan := new(VlanBridge)

	// Keep a reference to the agent
	vlan.agent = agent

	return vlan
}

// Handle new master added event
func (self *VlanBridge) MasterAdded(master *OfnetNode) error {

	return nil
}

// Handle switch connected notification
func (self *VlanBridge) SwitchConnected(sw *ofctrl.OFSwitch) {
	// Keep a reference to the switch
	self.ofSwitch = sw

	log.Infof("Switch connected(vlan)")
}

// Handle switch disconnected notification
func (self *VlanBridge) SwitchDisconnected(sw *ofctrl.OFSwitch) {
	// FIXME: ??
}

// Handle incoming packet
func (self *VlanBridge) PacketRcvd(sw *ofctrl.OFSwitch, pkt *ofctrl.PacketIn) {
	// Ignore all incoming packets for now
}

// Add a local endpoint and install associated local route
func (self *VlanBridge) AddLocalEndpoint(endpoint OfnetEndpoint) error {
	log.Infof("Adding local endpoint: %+v", endpoint)

	// Nothing to do. Let OVS do its thing..

	return nil
}

// Remove local endpoint
func (self *VlanBridge) RemoveLocalEndpoint(endpoint OfnetEndpoint) error {
	// Nothing to do. Let OVS handle switching..

	return nil
}

// Add virtual tunnel end point.
func (self *VlanBridge) AddVtepPort(portNo uint32, remoteIp net.IP) error {
	return nil
}

// Remove a VTEP port
func (self *VlanBridge) RemoveVtepPort(portNo uint32, remoteIp net.IP) error {
	return nil
}

// Add a vlan.
func (self *VlanBridge) AddVlan(vlanId uint16, vni uint32) error {
	return nil
}

// Remove a vlan
func (self *VlanBridge) RemoveVlan(vlanId uint16, vni uint32) error {
	return nil
}

// AddEndpoint Add an endpoint to the datapath
func (self *VlanBridge) AddEndpoint(endpoint *OfnetEndpoint) error {
	log.Infof("Received endpoint: %+v", endpoint)

	// Nothing to do.. let OVS handle forwarding.

	return nil
}

// RemoveEndpoint removes an endpoint from the datapath
func (self *VlanBridge) RemoveEndpoint(endpoint *OfnetEndpoint) error {
	log.Infof("Received DELETE endpoint: %+v", endpoint)

	// Nothing to do. Let OVS handle forwarding..

	return nil
}
