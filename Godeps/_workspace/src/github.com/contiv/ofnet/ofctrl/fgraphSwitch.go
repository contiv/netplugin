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

// This file implements the forwarding graph API for the switch

import (
    "errors"

    "github.com/shaleman/libOpenflow/openflow13"

    // log "github.com/Sirupsen/logrus"
)

// Initialize the fgraph elements on the switch
func (self *OFSwitch) initFgraph() error {
    // Create the DBs
    self.tableDb = make(map[uint8]*Table)
    self.outputPorts = make(map[uint32]*Output)

    // Create the table 0
    table := new(Table)
    table.Switch = self
    table.TableId = 0
    table.flowDb = make(map[string]*Flow)
    self.tableDb[0] = table

    // Create drop action
    dropAction := new(Output)
    dropAction.outputType = "drop"
    dropAction.portNo = openflow13.P_ANY
    self.dropAction = dropAction

    // create send to controller action
    sendToCtrler := new(Output)
    sendToCtrler.outputType = "toController"
    sendToCtrler.portNo = openflow13.P_CONTROLLER
    self.sendToCtrler = sendToCtrler

    // Clear all existing flood lists
    groupMod := openflow13.NewGroupMod()
    groupMod.GroupId = openflow13.OFPG_ALL
    groupMod.Command = openflow13.OFPGC_DELETE
    groupMod.Type = openflow13.OFPGT_ALL
    self.Send(groupMod)

    return nil
}

// Create a new table. return an error if it already exists
func (self *OFSwitch) NewTable(tableId uint8) (*Table, error) {
    // Check the parameters
    if (tableId == 0) {
        return nil, errors.New("Table 0 already exists")
    }

    // check if the table already exists
    if (self.tableDb[tableId] != nil) {
        return nil, errors.New("Table already exists")
    }

    // Create a new table
    table := new(Table)
    table.Switch = self
    table.TableId = tableId
    table.flowDb = make(map[string]*Flow)

    // Save it in the DB
    self.tableDb[tableId] = table

    return table, nil
}

// Delete a table.
// Return an error if there are fgraph nodes pointing at it
func (self *OFSwitch) DeleteTable(tableId uint8) error {
    // FIXME: to be implemented
    return nil
}


// Return table 0 which is the starting table for all packets
func (self *OFSwitch) DefaultTable() *Table {
    return self.tableDb[0]
}

// Return a output graph element for the port
func (self *OFSwitch) OutputPort(portNo uint32) (*Output, error) {
    if self.outputPorts[portNo] != nil {
        return self.outputPorts[portNo], nil
    }

    // Create a new output element
    output := new(Output)
    output.outputType = "port"
    output.portNo = portNo

    // store all outputs in a DB
    self.outputPorts[portNo] = output

    return output, nil
}

// Return the drop graph element
func (self *OFSwitch) DropAction() *Output {
    return self.dropAction
}

// Return send to controller graph element
func (self *OFSwitch) SendToController() *Output {
    return self.sendToCtrler
}

// FIXME: Unique group id for the flood entries
var uniqueGroupId uint32 = 1

// Create a new flood list
func (self *OFSwitch) NewFlood() (*Flood, error) {
    flood := new(Flood)

    flood.Switch = self
    flood.GroupId = uniqueGroupId
    uniqueGroupId += 1

    // Install it in HW right away
    flood.install()

    return flood, nil
}
