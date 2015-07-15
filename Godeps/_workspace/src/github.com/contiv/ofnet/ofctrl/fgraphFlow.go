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

// This file implements the forwarding graph API for the flow

import (
    "net"
    "encoding/json"

    "github.com/shaleman/libOpenflow/openflow13"
    log "github.com/Sirupsen/logrus"
)

// Small subset of openflow fields we currently support
type FlowMatch struct {
    Priority        uint16              // Priority of the flow
    InputPort       uint32
    MacDa           *net.HardwareAddr
    MacDaMask       *net.HardwareAddr
    MacSa           *net.HardwareAddr
    MacSaMask       *net.HardwareAddr
    Ethertype       uint16
    VlanId          uint16
    IpSa            *net.IP
    IpSaMask        *net.IP
    IpDa            *net.IP
    IpDaMask        *net.IP
    Metadata        *uint64
    MetadataMask    *uint64
    TunnelId        uint64
}

// additional actions in flow's instruction set
type FlowAction struct {
    actionType      string      // Type of action "setVlan", "setMetadata"
    vlanId          uint16      // Vlan Id in case of "setVlan"
    macAddr         net.HardwareAddr    // Mac address to set
    tunnelId        uint64      // Tunnel Id (used for setting VNI)
    metadata        uint64      // Metadata in case of "setMetadata"
    metadataMask    uint64      // Metadata mask
}

// State of a flow entry
type Flow struct {
    Table           *Table          // Table where this flow resides
    Match           FlowMatch       // Fields to be matched
    NextElem        FgraphElem      // Next fw graph element
    isInstalled     bool            // Is the flow installed in the switch
    flowId          uint64          // Unique ID for the flow
    flowActions     []*FlowAction   // List of flow actions
}

// string key for the flow
// FIXME: simple json conversion for now. This needs to be smarter
func (self *Flow) flowKey() string {
    jsonVal, err := json.Marshal(self.Match)
    if (err != nil) {
        log.Errorf("Error forming flowkey for %+v. Err: %v", err)
        return ""
    }

    return string(jsonVal)
}


// Fgraph element type for the flow
func (self *Flow) Type() string {
    return "flow"
}

// instruction set for flow element
func (self *Flow) GetFlowInstr() openflow13.Instruction {
    log.Fatalf("Unexpected call to get flow's instruction set")
    return nil
}

// Translate our match fields into openflow 1.3 match fields
func (self *Flow) xlateMatch() openflow13.Match {
    ofMatch := openflow13.NewMatch()

    if (self.Match.InputPort != 0) {
        inportField := openflow13.NewInPortField(self.Match.InputPort)
        ofMatch.AddField(*inportField)
    }

    if (self.Match.MacDa != nil) {
        if (self.Match.MacDaMask != nil) {
            macDaField := openflow13.NewEthDstField(*self.Match.MacDa, self.Match.MacDaMask)
            ofMatch.AddField(*macDaField)
        } else {
            macDaField := openflow13.NewEthDstField(*self.Match.MacDa, nil)
            ofMatch.AddField(*macDaField)
        }
    }

    if (self.Match.MacSa != nil) {
        if (self.Match.MacSaMask != nil) {
            macSaField := openflow13.NewEthSrcField(*self.Match.MacSa, self.Match.MacSaMask)
            ofMatch.AddField(*macSaField)
        } else {
            macSaField := openflow13.NewEthSrcField(*self.Match.MacSa, nil)
            ofMatch.AddField(*macSaField)
        }
    }

    if (self.Match.Ethertype != 0) {
        etypeField := openflow13.NewEthTypeField(self.Match.Ethertype)
        ofMatch.AddField(*etypeField)
    }

    if (self.Match.VlanId != 0) {
        vidField := openflow13.NewVlanIdField(self.Match.VlanId)
        ofMatch.AddField(*vidField)
    }

    if (self.Match.IpDa != nil) {
        if (self.Match.IpDaMask != nil) {
            ipDaField := openflow13.NewIpv4DstField(*self.Match.IpDa, self.Match.IpDaMask)
            ofMatch.AddField(*ipDaField)
        } else {
            ipDaField := openflow13.NewIpv4DstField(*self.Match.IpDa, nil)
            ofMatch.AddField(*ipDaField)
        }
    }

    if (self.Match.IpSa != nil) {
        if (self.Match.IpSaMask != nil) {
            ipSaField := openflow13.NewIpv4SrcField(*self.Match.IpSa, self.Match.IpSaMask)
            ofMatch.AddField(*ipSaField)
        } else {
            ipSaField := openflow13.NewIpv4SrcField(*self.Match.IpSa, nil)
            ofMatch.AddField(*ipSaField)
        }
    }

    if (self.Match.Metadata != nil) {
        if (self.Match.MetadataMask != nil) {
            metadataField := openflow13.NewMetadataField(*self.Match.Metadata, self.Match.MetadataMask)
            ofMatch.AddField(*metadataField)
        } else {
            metadataField := openflow13.NewMetadataField(*self.Match.Metadata, nil)
            ofMatch.AddField(*metadataField)
        }
    }

    if (self.Match.TunnelId != 0) {
            tunnelIdField := openflow13.NewTunnelIdField(self.Match.TunnelId)
            ofMatch.AddField(*tunnelIdField)
    }

    return *ofMatch
}

// Install all flow actions
func (self *Flow) installFlowActions(flowMod *openflow13.FlowMod,
                                    instr openflow13.Instruction) error {
    var actInstr openflow13.Instruction
    var addActn bool = false

    // Create a apply_action instruction to be used if its not already created
    switch instr.(type) {
    case *openflow13.InstrActions:
        actInstr = instr
    default:
        actInstr = openflow13.NewInstrApplyActions()
    }


    // Loop thru all actions
    for _, flowAction := range self.flowActions {
        switch(flowAction.actionType) {
        case "setVlan":
            // Push Vlan Tag action
            pushVlanAction := openflow13.NewActionPushVlan(0x8100)

            // Set Outer vlan tag field
            vlanField := openflow13.NewVlanIdField(flowAction.vlanId)
            setVlanAction := openflow13.NewActionSetField(*vlanField)


            // Prepend push vlan & setvlan actions to existing instruction
            actInstr.AddAction(setVlanAction, true)
            actInstr.AddAction(pushVlanAction, true)
            addActn = true

            log.Debugf("flow install. Added pushvlan action: %+v, setVlan actions: %+v",
                            pushVlanAction, setVlanAction)

        case "popVlan":
            // Create pop vln action
            popVlan := openflow13.NewActionPopVlan()

            // Add it to instruction
            actInstr.AddAction(popVlan, true)
            addActn = true

            log.Debugf("flow install. Added popVlan action: %+v", popVlan)

        case "setMacDa":
            // Set Outer MacDA field
            macDaField := openflow13.NewEthDstField(flowAction.macAddr, nil)
            setMacDaAction := openflow13.NewActionSetField(*macDaField)


            // Add set macDa action to the instruction
            actInstr.AddAction(setMacDaAction, true)
            addActn = true

            log.Debugf("flow install. Added setMacDa action: %+v", setMacDaAction)


        case "setMacSa":
            // Set Outer MacSA field
            macSaField := openflow13.NewEthSrcField(flowAction.macAddr, nil)
            setMacSaAction := openflow13.NewActionSetField(*macSaField)

            // Add set macDa action to the instruction
            actInstr.AddAction(setMacSaAction, true)
            addActn = true

            log.Debugf("flow install. Added setMacSa Action: %+v", setMacSaAction)

        case "setTunnelId":
            // Set tunnelId field
            tunnelIdField := openflow13.NewTunnelIdField(flowAction.tunnelId)
            setTunnelAction := openflow13.NewActionSetField(*tunnelIdField)

            // Add set tunnel action to the instruction
            actInstr.AddAction(setTunnelAction, true)
            addActn = true

            log.Debugf("flow install. Added setTunnelId Action: %+v", setTunnelAction)

        case "setMetadata":
            // Set Metadata instruction
            metadataInstr := openflow13.NewInstrWriteMetadata(flowAction.metadata, flowAction.metadataMask)

            // Add the instruction to flowmod
            flowMod.AddInstruction(metadataInstr)

        default:
            log.Fatalf("Unknown action type %s", flowAction.actionType)
        }
    }

    // Add the instruction to flow if its not already added
    if ((addActn) && (actInstr != instr)) {
        // Add the instrction to flowmod
        flowMod.AddInstruction(actInstr)
    }

    return nil
}

// Install a flow entry
func (self *Flow) install() error {
    // Create a flowmode entry
    flowMod := openflow13.NewFlowMod()
    flowMod.TableId = self.Table.TableId
    flowMod.Priority = self.Match.Priority
    flowMod.Cookie = self.flowId

    // Add or modify
    if (!self.isInstalled) {
        flowMod.Command = openflow13.FC_ADD
    } else {
        flowMod.Command = openflow13.FC_MODIFY
    }

    // convert match fields to openflow 1.3 format
    flowMod.Match = self.xlateMatch()
    log.Debugf("flow install: Match: %+v", flowMod.Match)


    // Based on the next elem, decide what to install
    switch (self.NextElem.Type()) {
    case "table":
        // Get the instruction set from the element
        instr := self.NextElem.GetFlowInstr()

        // Check if there are any flow actions to perform
        self.installFlowActions(flowMod, instr)

        // Add the instruction to flowmod
        flowMod.AddInstruction(instr)

        log.Debugf("flow install: added goto table instr: %+v", instr)

    case "flood": fallthrough;
    case "output":
        // Get the instruction set from the element
        instr := self.NextElem.GetFlowInstr()

        // Add the instruction to flowmod if its not nil
        // a nil instruction means drop action
        if (instr != nil) {

            // Check if there are any flow actions to perform
            self.installFlowActions(flowMod, instr)

            flowMod.AddInstruction(instr)

            log.Debugf("flow install: added output port instr: %+v", instr)
        }
    default:
        log.Fatalf("Unknown Fgraph element type %s", self.NextElem.Type())
    }

    log.Debugf("Sending flowmod: %+v", flowMod)

    // Send the message
    self.Table.Switch.Send(flowMod)

    // Mark it as installed
    self.isInstalled = true

    return nil
}

// Set Next element in the Fgraph. This determines what actions will be
// part of the flow's instruction set
func (self *Flow) Next(elem FgraphElem) error {
    // Set the next element in the graph
    self.NextElem = elem

    // Install the flow entry
    return self.install()
}

// Special actions on the flow to set vlan id
func (self *Flow) SetVlan(vlanId uint16) error {
    action := new(FlowAction)
    action.actionType = "setVlan"
    action.vlanId   = vlanId

    // Add to the action list
    // FIXME: detect duplicates
    self.flowActions = append(self.flowActions, action)

    // If the flow entry was already installed, re-install it
    if (self.isInstalled) {
        self.install()
    }

    return nil
}

// Special actions on the flow to set vlan id
func (self *Flow) PopVlan() error {
    action := new(FlowAction)
    action.actionType = "popVlan"

    // Add to the action list
    // FIXME: detect duplicates
    self.flowActions = append(self.flowActions, action)

    // If the flow entry was already installed, re-install it
    if (self.isInstalled) {
        self.install()
    }

    return nil
}

// Special actions on the flow to set mac dest addr
func (self *Flow) SetMacDa(macDa net.HardwareAddr) error {
    action := new(FlowAction)
    action.actionType = "setMacDa"
    action.macAddr   = macDa

    // Add to the action list
    // FIXME: detect duplicates
    self.flowActions = append(self.flowActions, action)

    // If the flow entry was already installed, re-install it
    if (self.isInstalled) {
        self.install()
    }

    return nil
}

// Special action on the flow to set mac source addr
func (self *Flow) SetMacSa(macSa net.HardwareAddr) error {
    action := new(FlowAction)
    action.actionType = "setMacSa"
    action.macAddr   = macSa

    // Add to the action list
    // FIXME: detect duplicates
    self.flowActions = append(self.flowActions, action)

    // If the flow entry was already installed, re-install it
    if (self.isInstalled) {
        self.install()
    }

    return nil
}

// Special actions on the flow to set metadata
func (self *Flow) SetMetadata(metadata , metadataMask uint64) error {
    action := new(FlowAction)
    action.actionType   = "setMetadata"
    action.metadata     = metadata
    action.metadataMask = metadataMask

    // Add to the action list
    // FIXME: detect duplicates
    self.flowActions = append(self.flowActions, action)

    // If the flow entry was already installed, re-install it
    if (self.isInstalled) {
        self.install()
    }

    return nil
}

// Special actions on the flow to set vlan id
func (self *Flow) SetTunnelId(tunnelId uint64) error {
    action := new(FlowAction)
    action.actionType = "setTunnelId"
    action.tunnelId   = tunnelId

    // Add to the action list
    // FIXME: detect duplicates
    self.flowActions = append(self.flowActions, action)

    // If the flow entry was already installed, re-install it
    if (self.isInstalled) {
        self.install()
    }

    return nil
}

// Delete the flow
func (self *Flow) Delete() error {
    // Delete from ofswitch
    if (self.isInstalled) {
        // Create a flowmode entry
        flowMod := openflow13.NewFlowMod()
        flowMod.Command = openflow13.FC_DELETE
        flowMod.TableId = self.Table.TableId
        flowMod.Priority = self.Match.Priority
        flowMod.Cookie = self.flowId
        flowMod.CookieMask = 0xffffffffffffffff
        flowMod.OutPort = openflow13.P_ANY
        flowMod.OutGroup = openflow13.OFPG_ANY

        log.Debugf("Sending DELETE flowmod: %+v", flowMod)

        // Send the message
        self.Table.Switch.Send(flowMod)
    }

    // Delete it from the table
    flowKey := self.flowKey()
    self.Table.DeleteFlow(flowKey)

    return nil
}
