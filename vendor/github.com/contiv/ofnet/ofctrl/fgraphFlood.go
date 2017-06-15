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
package ofctrl

// This file implements the forwarding graph API for the Flood element

import (
	"errors"

	"github.com/contiv/libOpenflow/openflow13"

	log "github.com/Sirupsen/logrus"
)

// Flood Fgraph element
type Flood struct {
	Switch      *OFSwitch // Switch where this flood entry is present
	GroupId     uint32    // Unique id for the openflow group
	isInstalled bool      // Is this installed in the datapath

	FloodList []FloodOutput // List of output ports to flood to
}

type FloodOutput struct {
	outPort  *Output
	isTunnel bool
	tunnelId uint64
}

// Fgraph element type for the output
func (self *Flood) Type() string {
	return "flood"
}

// instruction set for output element
func (self *Flood) GetFlowInstr() openflow13.Instruction {
	// If there are no ports in the flood entry, return
	if !self.isInstalled {
		return nil
	}

	groupInstr := openflow13.NewInstrApplyActions()
	groupAct := openflow13.NewActionGroup(self.GroupId)
	groupInstr.AddAction(groupAct, false)

	return groupInstr
}

// Add a new Output to group element
func (self *Flood) AddOutput(out *Output) error {
	self.FloodList = append(self.FloodList, FloodOutput{out, false, 0})

	// Install in the HW
	return self.install()
}

// Add a new Output to group element
func (self *Flood) AddTunnelOutput(out *Output, tunnelId uint64) error {
	self.FloodList = append(self.FloodList, FloodOutput{out, true, tunnelId})

	// Install in the HW
	return self.install()
}

// Remove a port from flood list
func (self *Flood) RemoveOutput(out *Output) error {
	// walk all flood list entries and see if it matches the output port
	for idx, output := range self.FloodList {
		if output.outPort == out {
			// Remove from the flood list. strange golang syntax to remove an element from slice
			self.FloodList = append(self.FloodList[:idx], self.FloodList[idx+1:]...)

			// Re-install the flood list with removed port
			return self.install()
		}
	}

	return errors.New("Output not found")
}

// Return number of ports in flood list
func (self *Flood) NumOutput() int {
	return len(self.FloodList)
}

// Install a group entry in OF switch
func (self *Flood) install() error {
	groupMod := openflow13.NewGroupMod()
	groupMod.GroupId = self.GroupId

	// Change the OP to modify if it was already installed
	if self.isInstalled {
		groupMod.Command = openflow13.OFPGC_MODIFY
	}

	// OF type for flood list
	groupMod.Type = openflow13.OFPGT_ALL

	// Loop thru all output ports and add it to group bucket
	for _, output := range self.FloodList {
		// Get the output action from output entry
		act := output.outPort.GetOutAction()
		if act != nil {
			// Create a new bucket for each port
			bkt := openflow13.NewBucket()

			// Set tunnel Id if required
			if output.isTunnel {
				tunnelField := openflow13.NewTunnelIdField(output.tunnelId)
				setTunnel := openflow13.NewActionSetField(*tunnelField)
				bkt.AddAction(setTunnel)
			}

			// Always remove vlan tag
			popVlan := openflow13.NewActionPopVlan()
			bkt.AddAction(popVlan)

			// Add the output action to the bucket
			bkt.AddAction(act)

			// Add the bucket to group
			groupMod.AddBucket(*bkt)
		}
	}

	log.Debugf("Installing Group entry: %+v", groupMod)

	// Send it to the switch
	self.Switch.Send(groupMod)

	// Mark it as installed
	self.isInstalled = true

	return nil
}

// Delete a flood list
func (self *Flood) Delete() error {
	// Remove it from OVS if its installed
	if self.isInstalled {
		groupMod := openflow13.NewGroupMod()
		groupMod.GroupId = self.GroupId
		groupMod.Command = openflow13.OFPGC_DELETE

		log.Debugf("Deleting Group entry: %+v", groupMod)

		// Send it to the switch
		self.Switch.Send(groupMod)
	}

	return nil
}
