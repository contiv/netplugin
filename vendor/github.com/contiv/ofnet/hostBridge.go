/***
Copyright 2016 Cisco Systems Inc. All rights reserved.

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

import (
	"errors"
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/libOpenflow/openflow13"
	"github.com/contiv/libOpenflow/protocol"
	"github.com/contiv/ofnet/ofctrl"
	"github.com/contiv/ofnet/ovsdbDriver"
)

// hostAgent state
type HostBridge struct {
	ctrler      *ofctrl.Controller // Controller instance
	ofSwitch    *ofctrl.OFSwitch   // Switch instance. Assumes single switch per agent
	isConnected bool               // Is the switch connected
	dpName      string             // Datapath type
	ovsDriver   *ovsdbDriver.OvsDriver

	// Fgraph tables
	inputTable *ofctrl.Table // Packet lookup starts here
	macTable   *ofctrl.Table // mac lookup table

	gwInFlow  *ofctrl.Flow // Incoming flows from the GW
	gwOutFlow *ofctrl.Flow // Outgoing flows to the GW
	gwARPFlow *ofctrl.Flow // Outgoing ARP requests to GW
}

// NewHostBridge Create a new Host agent and initialize it
func NewHostBridge(bridgeName, dpName string, ovsPort uint16) (*HostBridge, error) {
	agent := new(HostBridge)

	// Init params
	agent.dpName = dpName

	// Create an openflow controller
	agent.ctrler = ofctrl.NewController(agent)

	// Start listening to controller port
	go agent.ctrler.Listen(fmt.Sprintf(":%d", ovsPort))

	// Return it
	return agent, nil
}

// Delete cleans up an ofnet agent
func (self *HostBridge) Delete() error {
	// Disconnect from the switch
	if self.ofSwitch != nil {
		self.ofSwitch.Disconnect()
	}

	// Cleanup the controller
	self.ctrler.Delete()

	time.Sleep(100 * time.Millisecond)

	return nil
}

// Handle switch connected event
func (self *HostBridge) SwitchConnected(sw *ofctrl.OFSwitch) {
	log.Infof("Switch %v connected", sw.DPID())

	// store it for future use.
	self.ofSwitch = sw

	self.isConnected = true
	// Init the Fgraph
	self.initFgraph()
}

// Handle switch disconnect event
func (self *HostBridge) SwitchDisconnected(sw *ofctrl.OFSwitch) {
	log.Infof("Switch %v disconnected", sw.DPID())

	// Ignore if this error was not for current switch
	if sw.DPID().String() != self.ofSwitch.DPID().String() {
		return
	}

	self.ofSwitch = nil
	self.isConnected = false
}

// IsSwitchConnected returns true if switch is connected
func (self *HostBridge) IsSwitchConnected() bool {
	return self.isConnected
}

// WaitForSwitchConnection wait till switch connects
func (self *HostBridge) WaitForSwitchConnection() {
	// Wait for a while for OVS switch to connect to ofnet agent
	for i := 0; i < 20; i++ {
		time.Sleep(1 * time.Second)
		if self.IsSwitchConnected() {
			return
		}
	}

	log.Fatalf("OVS switch %s Failed to connect", self.dpName)
}

// Receive a packet from the switch.
func (self *HostBridge) PacketRcvd(sw *ofctrl.OFSwitch, pkt *ofctrl.PacketIn) {
	log.Debugf("Packet received from switch %v. Packet: %+v", sw.DPID(), pkt)
}

// MultipartReply Receives a multi-part reply from the switch.
func (self *HostBridge) MultipartReply(sw *ofctrl.OFSwitch, reply *openflow13.MultipartReply) {
	log.Debugf("Multi-part reply received from switch: %+v", reply)
}

// AddHostPort Add a host port for access to host network.
func (self *HostBridge) AddHostPort(endpoint EndpointInfo) error {

	log.Infof("Adding local endpoint: %+v", endpoint)
	if self.gwInFlow != nil {
		log.Errorf("GW flow already exists")
		return errors.New("GW flow already exists")
	}

	// Gw is allowed to reach any port
	gwInFlow, err := self.macTable.NewFlow(ofctrl.FlowMatch{
		Priority:  FLOW_MATCH_PRIORITY,
		InputPort: endpoint.PortNo,
	})
	if err != nil {
		log.Errorf("host bridge - err: %v", err)
		return err
	}
	gwInFlow.Next(self.ofSwitch.NormalLookup())

	// Any port is allowed to reach GW
	gwMAC := endpoint.MacAddr
	gwOutFlow, err := self.macTable.NewFlow(ofctrl.FlowMatch{
		Priority: FLOW_MATCH_PRIORITY,
		MacDa:    &gwMAC,
	})

	if err != nil {
		log.Errorf("host bridge - err: %v", err)
		gwInFlow.Delete()
		return err
	}
	gwOutFlow.Next(self.ofSwitch.NormalLookup())

	// Any port is allowed to ARP for GW
	// TODO: match targetIP
	gwARPFlow, err := self.macTable.NewFlow(ofctrl.FlowMatch{
		Priority:  FLOW_MATCH_PRIORITY,
		Ethertype: 0x806,
		ArpOper:   protocol.Type_Request,
	})
	if err != nil {
		log.Errorf("host bridge - err: %v", err)
		gwInFlow.Delete()
		gwOutFlow.Delete()
		return err
	}
	gwARPFlow.Next(self.ofSwitch.NormalLookup())
	self.gwInFlow = gwInFlow
	self.gwOutFlow = gwOutFlow
	self.gwARPFlow = gwARPFlow

	return nil
}

// DelHostPort Remove host port
func (self *HostBridge) DelHostPort(portNo uint32) error {
	if self.gwInFlow != nil {
		self.gwInFlow.Delete()
		self.gwInFlow = nil
	}
	if self.gwOutFlow != nil {
		self.gwOutFlow.Delete()
		self.gwOutFlow = nil
	}
	if self.gwARPFlow != nil {
		self.gwARPFlow.Delete()
		self.gwARPFlow = nil
	}

	return nil
}

// initialize Fgraph on the switch
func (self *HostBridge) initFgraph() error {
	sw := self.ofSwitch

	log.Infof("Installing initial flow entries")

	// Create all tables
	self.inputTable = sw.DefaultTable()
	self.macTable, _ = sw.NewTable(MAC_DEST_TBL_ID)

	// Send all IP/ARP packets to mac lookup
	ipPktFlow, _ := self.inputTable.NewFlow(ofctrl.FlowMatch{
		Priority:  FLOW_MATCH_PRIORITY,
		Ethertype: 0x0800,
	})
	ipPktFlow.Next(self.macTable)

	arpPktFlow, _ := self.inputTable.NewFlow(ofctrl.FlowMatch{
		Priority:  FLOW_MATCH_PRIORITY,
		Ethertype: 0x0806,
	})
	arpPktFlow.Next(self.macTable)

	// Drop everything else
	invalidFlow, _ := self.inputTable.NewFlow(ofctrl.FlowMatch{
		Priority: FLOW_MISS_PRIORITY,
	})
	invalidFlow.Next(sw.DropAction())

	// Add default rule to drop misses in mac lookup.
	macMissFlow, _ := self.macTable.NewFlow(ofctrl.FlowMatch{
		Priority: FLOW_MISS_PRIORITY,
	})
	macMissFlow.Next(sw.DropAction())

	return nil
}
