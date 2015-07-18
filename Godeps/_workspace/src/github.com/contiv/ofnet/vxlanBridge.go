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
// This file implements the vxlan bridging datapath

import (
    //"fmt"
    "net"
    "net/rpc"
    "time"
    "errors"

    //"github.com/shaleman/libOpenflow/openflow13"
    //"github.com/shaleman/libOpenflow/protocol"
    "github.com/contiv/ofnet/ofctrl"
    "github.com/contiv/ofnet/rpcHub"

    log "github.com/Sirupsen/logrus"
)

// VXLAN tables are structured as follows
//
// +-------+
// | Valid |
// | Pkts  +-->+-------+
// +-------+   | Vlan  |
//             | Table +-------+          +---------+
//             +-------+       +--------->| Mac Dst |      +--------------+
//                                        | Lookup  +--+-->| Ucast Output |
//                                        +---------+  |   +--------------+
//                                                     |
//                                                     |
//                                     +---------------+----------+
//                                     V                          V
//                            +------------------+    +----------------------+
//                            | Local Only Flood |    | Local + Remote Flood |
//                            +------------------+    +----------------------+
//

// Vxlan state.
type Vxlan struct {
    agent       *OfnetAgent             // Pointer back to ofnet agent that owns this
    ofSwitch    *ofctrl.OFSwitch        // openflow switch we are talking to

    vlanDb      map[uint16]*Vlan        // Database of known vlans

    // Mac route table
    macRouteDb      map[string]*MacRoute

    // Fgraph tables
    inputTable      *ofctrl.Table       // Packet lookup starts here
    vlanTable       *ofctrl.Table       // Vlan Table. map port or VNI to vlan
    macDestTable    *ofctrl.Table       // Destination mac lookup

    // Flow Database
    macFlowDb       map[string]*ofctrl.Flow // Database of flow entries
    portVlanFlowDb  map[uint32]*ofctrl.Flow // Database of flow entries
}

// Vlan info
type Vlan struct {
    Vni             uint32                  // Vxlan VNI
    localPortList   map[uint32]*uint32      // List of local ports only
    allPortList     map[uint32]*uint32      // List of local + remote(vtep) ports
    localFlood      *ofctrl.Flood           // local only flood list
    allFlood        *ofctrl.Flood           // local + remote flood list
}

// Mac address info
type MacRoute struct {
    MacAddrStr      string          // Mac address of the end point(in string format)
    Vni             uint32          // Vxlan VNI
    OriginatorIp    net.IP          // Originating switch
    PortNo          uint32          // Port number on originating switch
    Timestamp       time.Time       // Timestamp of the last event
}

const METADATA_RX_VTEP = 0x1

// Create a new vxlan instance
func NewVxlan(agent *OfnetAgent, rpcServ *rpc.Server) *Vxlan {
    vxlan := new(Vxlan)

    // Keep a reference to the agent
    vxlan.agent = agent

    // init DBs
    vxlan.macRouteDb = make(map[string]*MacRoute)
    vxlan.vlanDb     = make(map[uint16]*Vlan)
    vxlan.macFlowDb  = make(map[string]*ofctrl.Flow)
    vxlan.portVlanFlowDb  = make(map[uint32]*ofctrl.Flow)

    log.Infof("Registering vxlan RPC calls")

    // Register for Route rpc callbacks
    err := rpcServ.Register(vxlan)
    if err != nil {
        log.Fatalf("Error registering vxlan RPC")
    }

    return vxlan
}

// Handle new master added event
func (self *Vxlan) MasterAdded(master *OfnetNode) error {
    // Send all local routes to new master.
    for _, macRoute := range self.macRouteDb {
        if macRoute.OriginatorIp.String() == self.agent.localIp.String() {
            var resp bool

            log.Infof("Sending macRoute %+v to master %+v", macRoute, master)

            // Make the RPC call to add the route to master
            client := rpcHub.Client(master.HostAddr, master.HostPort)
            err := client.Call("OfnetMaster.MacRouteAdd", macRoute, &resp)
            if (err != nil) {
                log.Errorf("Failed to add route %+v to master %+v. Err: %v", macRoute, master, err)
                return err
            }
        }
    }

    return nil
}

// Handle switch connected notification
func (self *Vxlan) SwitchConnected(sw *ofctrl.OFSwitch) {
    // Keep a reference to the switch
    self.ofSwitch = sw
    // Init the Fgraph
    self.initFgraph()

    log.Infof("Switch connected(vxlan)")
}
// Handle switch disconnected notification
func (self *Vxlan) SwitchDisconnected(sw *ofctrl.OFSwitch) {
    // FIXME: ??
}

// Handle incoming packet
func (self *Vxlan) PacketRcvd(sw *ofctrl.OFSwitch, pkt *ofctrl.PacketIn) {
    // Ignore all incoming packets for now
}

// Add a local endpoint and install associated local route
func (self *Vxlan) AddLocalEndpoint(endpoint EndpointInfo) error {
    log.Infof("Adding local endpoint: %+v", endpoint)

    vni := self.agent.vlanVniMap[endpoint.Vlan]
    if vni == nil {
        log.Errorf("VNI for vlan %d is not known", endpoint.Vlan)
        return errors.New("Unknown Vlan")
    }

    // Install a flow entry for vlan mapping and point it to Mac table
    portVlanFlow, err := self.vlanTable.NewFlow(ofctrl.FlowMatch{
                            Priority: FLOW_MATCH_PRIORITY,
                            InputPort: endpoint.PortNo,
                        })
    if err != nil {
        log.Errorf("Error creating portvlan entry. Err: %v", err)
        return err
    }

    // Set the vlan and install it
    portVlanFlow.SetVlan(endpoint.Vlan)
    err = portVlanFlow.Next(self.macDestTable)
    if err != nil {
        log.Errorf("Error installing portvlan entry. Err: %v", err)
        return err
    }

    // save the flow entry
    self.portVlanFlowDb[endpoint.PortNo] = portVlanFlow

    // Add the port to local and remote flood list
    output, _ := self.ofSwitch.OutputPort(endpoint.PortNo)
    vlan := self.vlanDb[endpoint.Vlan]
    if vlan != nil {
        vlan.localFlood.AddOutput(output)
        vlan.allFlood.AddOutput(output)
    }

    // Finally install the mac address
    macFlow, err := self.macDestTable.NewFlow(ofctrl.FlowMatch{
                            Priority: FLOW_MATCH_PRIORITY,
                            VlanId: endpoint.Vlan,
                            MacDa: &endpoint.MacAddr,
                        })
    if err != nil {
        log.Errorf("Error creating mac flow for endpoint %+v. Err: %v", endpoint, err)
        return err
    }

    // Remove vlan tag and point it to local port
    macFlow.PopVlan()
    macFlow.Next(output)

    // Save the flow in DB
    self.macFlowDb[endpoint.MacAddr.String()] = macFlow

    // Build the mac route
    macRoute := MacRoute{
                    MacAddrStr: endpoint.MacAddr.String(),
                    Vni: *vni,
                    OriginatorIp: self.agent.localIp,
                    PortNo: endpoint.PortNo,
                    Timestamp: time.Now(),
                }

    // Advertize the route to master
    err = self.localMacRouteAdd(&macRoute)
    if (err != nil) {
        log.Errorf("Failed to add route %+v to master. Err: %v", macRoute, err)
        return err
    }

    return nil
}

// Find a mac by output port number
// Note: Works only for local ports
// FIXME: remove this function and add a mapping between local portNo and macRoute
func (self *Vxlan) findLocalMacRouteByPortno(portNo uint32) *MacRoute {
    for _, macRoute := range self.macRouteDb {
        if (macRoute.OriginatorIp.String() == self.agent.localIp.String()) &&
            (macRoute.PortNo == portNo) {
            return macRoute
        }
    }

    return nil
}

// Remove local endpoint
func (self *Vxlan) RemoveLocalEndpoint(portNo uint32) error {
    // find the mac route
    macRoute := self.findLocalMacRouteByPortno(portNo)
    if macRoute == nil {
        log.Errorf("Local mac route not found for port: %d", portNo)
        return errors.New("Local mac route not found")
    }

    // Remove the port from flood lists
    vlanId := self.agent.vniVlanMap[macRoute.Vni]
    vlan := self.vlanDb[*vlanId]
    output, _ := self.ofSwitch.OutputPort(portNo)
    vlan.localFlood.RemoveOutput(output)
    vlan.allFlood.RemoveOutput(output)

    // Remove the port vlan flow.
    portVlanFlow := self.portVlanFlowDb[portNo]
    if portVlanFlow != nil {
        err := portVlanFlow.Delete()
        if err != nil {
            log.Errorf("Error deleting portvlan flow. Err: %v", err)
        }
    }
    
    // Uninstall the flow
    err := self.uninstallMacRoute(macRoute)
    if err != nil {
        log.Errorf("Error Uninstalling mac route: %+v. Err: %v", macRoute, err)
    }

    // Remove the route from local DB and Advertize delete
    return self.localMacRouteDel(macRoute)
}

// Add virtual tunnel end point. This is mainly used for mapping remote vtep IP
// to ofp port number.
func (self *Vxlan) AddVtepPort(portNo uint32, remoteIp net.IP) error {
    // Install VNI to vlan mapping for each vni
    for vni, vlan := range self.agent.vniVlanMap {
        // Install a flow entry for  VNI/vlan and point it to macDest table
        portVlanFlow, _ := self.vlanTable.NewFlow(ofctrl.FlowMatch{
                                Priority: FLOW_MATCH_PRIORITY,
                                InputPort: portNo,
                                TunnelId: uint64(vni),
                            })
        portVlanFlow.SetVlan(*vlan)

        // Set the metadata to indicate packet came in from VTEP port
        portVlanFlow.SetMetadata(METADATA_RX_VTEP, METADATA_RX_VTEP)

        // Point to next table
        portVlanFlow.Next(self.macDestTable)
    }

    // Walk all vlans and add vtep port to the vlan
    for vlanId, vlan := range self.vlanDb {
        vni := self.agent.vlanVniMap[vlanId]
        if vni == nil {
            log.Errorf("Can not find vni for vlan: %d", vlanId)
        }
        output, _ := self.ofSwitch.OutputPort(portNo)
        vlan.allFlood.AddTunnelOutput(output, uint64(*vni))
    }

    return nil
}

// Remove a VTEP port
func (self *Vxlan) RemoveVtepPort(portNo uint32, remoteIp net.IP) error {
    // Remove the VTEP from flood lists
    output, _ := self.ofSwitch.OutputPort(portNo)
    for _, vlan := range self.vlanDb {
        // Walk all vlans and remove from flood lists
        vlan.allFlood.RemoveOutput(output)
    }

    // FIXME: uninstall vlan-vni mapping.

    // Walk all routes and remove anything pointing at this VTEP
    for _, macRoute := range self.macRouteDb {
        // If it originated from this remote host, uninstall the flow
        if macRoute.OriginatorIp.String() == remoteIp.String() {
            err := self.uninstallMacRoute(macRoute)
            if err != nil {
                log.Errorf("Error uninstalling mac route: %+v. Err: %v", macRoute, err)
            }
        }
    }

    return nil
}

// Add a vlan.
func (self *Vxlan) AddVlan(vlanId uint16, vni uint32) error {
    // check if the vlan already exists. if it does, we are done
    if self.vlanDb[vlanId] != nil {
        return nil
    }

    // create new vlan object
    vlan := new(Vlan)
    vlan.Vni = vni
    vlan.localPortList = make(map[uint32]*uint32)
    vlan.allPortList = make(map[uint32]*uint32)

    // Create flood entries
    vlan.localFlood, _ = self.ofSwitch.NewFlood()
    vlan.allFlood, _ = self.ofSwitch.NewFlood()

    // Walk all VTEP ports and add vni-vlan mapping for new VNI
    for _, vtepPort := range self.agent.vtepTable {
        // Install a flow entry for  VNI/vlan and point it to macDest table
        portVlanFlow, err := self.vlanTable.NewFlow(ofctrl.FlowMatch{
                                Priority: FLOW_MATCH_PRIORITY,
                                InputPort: *vtepPort,
                                TunnelId: uint64(vni),
                            })
        if err != nil {
            log.Errorf("Error creating port vlan flow for vlan %d. Err: %v", vlanId, err)
            return err
        }

        // Set vlan id
        portVlanFlow.SetVlan(vlanId)

        // Set the metadata to indicate packet came in from VTEP port
        portVlanFlow.SetMetadata(METADATA_RX_VTEP, METADATA_RX_VTEP)

        // Point to next table
        portVlanFlow.Next(self.macDestTable)
    }

    // Walk all VTEP ports and add it to the allFlood list
    for _, vtepPort := range self.agent.vtepTable {
        output, _ := self.ofSwitch.OutputPort(*vtepPort)
        vlan.allFlood.AddTunnelOutput(output, uint64(vni))
    }

    log.Infof("Installing vlan flood entry for vlan: %d", vlanId)

    // Install local flood and remote flood entries in macDestTable
    var metadataLclRx uint64 = 0
    var metadataVtepRx uint64 = METADATA_RX_VTEP
    vlanFlood, err := self.macDestTable.NewFlow(ofctrl.FlowMatch{
                            Priority: FLOW_FLOOD_PRIORITY,
                            VlanId: vlanId,
                            Metadata: &metadataLclRx,
                            MetadataMask: &metadataVtepRx,
                        })
    if err != nil {
        log.Errorf("Error creating local+remote flood. Err: %v", err)
        return err
    }

    vlanFlood.Next(vlan.allFlood)
    vlanLclFlood, err := self.macDestTable.NewFlow(ofctrl.FlowMatch{
                            Priority: FLOW_FLOOD_PRIORITY,
                            VlanId: vlanId,
                            Metadata: &metadataVtepRx,
                            MetadataMask: &metadataVtepRx,
                        })
    if err != nil {
        log.Errorf("Error creating local flood. Err: %v", err)
        return err
    }
    vlanLclFlood.Next(vlan.localFlood)

    // store it in DB
    self.vlanDb[vlanId] = vlan

    return nil
}

// Remove a vlan
func (self *Vxlan) RemoveVlan(vlanId uint16, vni uint32) error {
    vlan := self.vlanDb[vlanId]
    if vlan == nil {
        log.Fatalf("Could not find the vlan %d", vlanId)
    }

    // Make sure the flood lists are empty
    if (vlan.allFlood.NumOutput() != 0) || (vlan.localFlood.NumOutput() != 0) {
        log.Fatalf("VLAN flood list is not empty")
    }

    // make sure there are no mac routes still installed in this vlan
    for _, macRoute := range self.macRouteDb {
        if macRoute.Vni == vni {
            log.Fatalf("Vlan %d still has routes. Route: %+v", vlanId, macRoute)
        }
    }

    // Uninstall the flood lists
    vlan.allFlood.Delete()
    vlan.localFlood.Delete()

    // Remove it from DB
    delete(self.vlanDb, vlanId)

    return nil
}

// Mac route add rpc call from master
func (self *Vxlan) MacRouteAdd(macRoute *MacRoute, ret *bool) error {
    log.Infof("Received mac route: %+v", macRoute)

    // If this is a local route we are done
    if (macRoute.OriginatorIp.String() == self.agent.localIp.String()) {
        return nil
    }

    // Check if we have the route already and which is more recent
    oldRoute := self.macRouteDb[macRoute.MacAddrStr]
    if (oldRoute != nil) {
        // If old route has more recent timestamp, nothing to do
        if !macRoute.Timestamp.After(oldRoute.Timestamp) {
            return nil
        }
    }

    // First, add the route to local routing table
    self.macRouteDb[macRoute.MacAddrStr] = macRoute

    // Lookup the VTEP for the route
    vtepPort := self.agent.vtepTable[macRoute.OriginatorIp.String()]
    if (vtepPort == nil) {
        log.Errorf("Could not find the VTEP for mac route: %+v", macRoute)

        return errors.New("VTEP not found")
    }

    // map VNI to vlan Id
    vlanId := self.agent.vniVlanMap[macRoute.Vni]
    if vlanId == nil {
        log.Errorf("Macroute %+v on unknown VNI: %d", macRoute, macRoute.Vni)
        return errors.New("Unknown VNI")
    }

    macAddr, _ := net.ParseMAC(macRoute.MacAddrStr)

    // Install the route in OVS
    // Create an output port for the vtep
    outPort, err := self.ofSwitch.OutputPort(*vtepPort)
    if (err != nil) {
        log.Errorf("Error creating output port %d. Err: %v", *vtepPort, err)
        return err
    }

    // Finally install the mac address
    macFlow, _ := self.macDestTable.NewFlow(ofctrl.FlowMatch{
                            Priority: FLOW_MATCH_PRIORITY,
                            VlanId: *vlanId,
                            MacDa: &macAddr,
                        })
    macFlow.PopVlan()
    macFlow.SetTunnelId(uint64(macRoute.Vni))
    macFlow.Next(outPort)

    // Save the flow in DB
    self.macFlowDb[macRoute.MacAddrStr] = macFlow

    return nil
}

// Mac route delete rpc call from master
func (self *Vxlan) MacRouteDel (macRoute *MacRoute, ret *bool) error {
    log.Infof("Received DELETE mac route: %+v", macRoute)

    // If this is a local route we are done
    if (macRoute.OriginatorIp.String() == self.agent.localIp.String()) {
        return nil
    }

    // Ignore duplicate delete requests we might receive from multiple
    // Ofnet masters
    if self.macRouteDb[macRoute.MacAddrStr] == nil {
        return nil
    }

    // Uninstall the route
    err := self.uninstallMacRoute(macRoute)
    if err != nil {
        log.Errorf("Error uninstalling mac route %+v. Err: %v", macRoute, err)
    }

    // Remove it from route table
    delete(self.macRouteDb, macRoute.MacAddrStr)

    return nil
}

// Add a local route to routing table and distribute it
func (self *Vxlan) localMacRouteAdd(macRoute *MacRoute) error {
    // First, add the route to local routing table
    self.macRouteDb[macRoute.MacAddrStr] = macRoute

    // Send the route to all known masters
    for _, master := range self.agent.masterDb {
        var resp bool

        log.Infof("Sending macRoute %+v to master %+v", macRoute, master)

        // Make the RPC call to add the route to master
        client := rpcHub.Client(master.HostAddr, master.HostPort)
        err := client.Call("OfnetMaster.MacRouteAdd", macRoute, &resp)
        if (err != nil) {
            log.Errorf("Failed to add route %+v to master %+v. Err: %v", macRoute, master, err)
            return err
        }
    }

    return nil
}

// Delete a local route and inform the master
func (self *Vxlan) localMacRouteDel(macRoute *MacRoute) error {
    // delete the route from local routing table
    delete(self.macRouteDb, macRoute.MacAddrStr)

    // Send the DELETE to all known masters
    for _, master := range self.agent.masterDb {
        var resp bool

        log.Infof("Sending DELETE macRoute %+v to master %+v", macRoute, master)

        // Make the RPC call to add the route to master
        client := rpcHub.Client(master.HostAddr, master.HostPort)
        err := client.Call("OfnetMaster.MacRouteDel", macRoute, &resp)
        if (err != nil) {
            log.Errorf("Failed to DELETE route %+v to master %+v. Err: %v", macRoute, master, err)
            return err
        }
    }

    return nil
}

// Uninstall mac route from OVS
func (self *Vxlan) uninstallMacRoute(macRoute *MacRoute) error {
    // find the flow
    macFlow := self.macFlowDb[macRoute.MacAddrStr]
    if macFlow == nil {
        log.Errorf("Could not find the flow for macRoute: %+v", macRoute)
        return errors.New("Mac flow not found")
    }

    // Delete the flow
    err := macFlow.Delete()
    if err != nil {
        log.Errorf("Error deleting mac flow: %+v. Err: %v", macFlow, err)
    }

    return err
}


const MAC_DEST_TBL_ID = 3

// initialize Fgraph on the switch
func (self *Vxlan) initFgraph() error {
    sw := self.ofSwitch

    log.Infof("Installing initial flow entries")

    // Create all tables
    self.inputTable = sw.DefaultTable()
    self.vlanTable, _ = sw.NewTable(VLAN_TBL_ID)
    self.macDestTable, _ = sw.NewTable(MAC_DEST_TBL_ID)

    //Create all drop entries
    // Drop mcast source mac
    bcastMac, _ := net.ParseMAC("01:00:00:00:00:00")
    bcastSrcFlow, _ := self.inputTable.NewFlow(ofctrl.FlowMatch{
                            Priority: FLOW_MATCH_PRIORITY,
                            MacSa: &bcastMac,
                            MacSaMask: &bcastMac,
                        })
    bcastSrcFlow.Next(sw.DropAction())

    // FIXME: Add additional checks on:
    //  Drop STP packets
    //  Send LLDP packets to controller
    //  Send LACP packets to controller
    //  Drop all other reserved mcast packets in 01-80-C2 range.

    // Send all valid packets to vlan table
    // This is installed at lower priority so that all packets that miss above
    // flows will match entry
    validPktFlow, _ := self.inputTable.NewFlow(ofctrl.FlowMatch{
                            Priority: FLOW_MISS_PRIORITY,
                        })
    validPktFlow.Next(self.vlanTable)

    // Drop all packets that miss Vlan lookup
    vlanMissFlow, _ := self.vlanTable.NewFlow(ofctrl.FlowMatch{
                            Priority: FLOW_MISS_PRIORITY,
                        })
    vlanMissFlow.Next(sw.DropAction())

    // Drop all packets that miss mac dest lookup AND vlan flood lookup
    floodMissFlow, _ := self.macDestTable.NewFlow(ofctrl.FlowMatch{
                            Priority: FLOW_MISS_PRIORITY,
                        })
    floodMissFlow.Next(sw.DropAction())

    // Drop all
    return nil
}
