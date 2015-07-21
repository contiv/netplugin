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
// This file implements the virtual router functionality using Vxlan overlay

import (
    //"fmt"
    "net"
    "net/rpc"
    "time"
    "errors"

    "github.com/shaleman/libOpenflow/openflow13"
    "github.com/shaleman/libOpenflow/protocol"
    "github.com/contiv/ofnet/ofctrl"
    "github.com/contiv/ofnet/rpcHub"

    log "github.com/Sirupsen/logrus"
)

// Vrouter state.
// One Vrouter instance exists on each host
type Vrouter struct {
    agent       *OfnetAgent             // Pointer back to ofnet agent that owns this
    ofSwitch    *ofctrl.OFSwitch        // openflow switch we are talking to

    // Fgraph tables
    inputTable  *ofctrl.Table           // Packet lookup starts here
    vlanTable   *ofctrl.Table           // Vlan Table. map port or VNI to vlan
    ipTable     *ofctrl.Table           // IP lookup table

    // Routing table
    routeTable  map[string]*OfnetRoute  // routes indexed by ip addr

    // Flow Database
    flowDb      map[string]*ofctrl.Flow // Database of flow entries
    portVlanFlowDb  map[uint32]*ofctrl.Flow // Database of flow entries

    // Router Mac to be used
    myRouterMac net.HardwareAddr
}

// IP Route information
type OfnetRoute struct {
    IpAddr          net.IP      // IP address of the end point
    VrfId           uint16      // IP address namespace
    OriginatorIp    net.IP      // Originating switch
    PortNo          uint32      // Port number on originating switch
    Timestamp       time.Time   // Timestamp of the last event
}

// Create a new vrouter instance
func NewVrouter(agent *OfnetAgent, rpcServ *rpc.Server) *Vrouter {
    vrouter := new(Vrouter)

    // Keep a reference to the agent
    vrouter.agent = agent

    // Create a route table and my router mac
    vrouter.routeTable = make(map[string]*OfnetRoute)
    vrouter.flowDb = make(map[string]*ofctrl.Flow)
    vrouter.portVlanFlowDb  = make(map[uint32]*ofctrl.Flow)
    vrouter.myRouterMac, _ = net.ParseMAC("00:00:11:11:11:11")

    // Register for Route rpc callbacks
    rpcServ.Register(vrouter)

    return vrouter
}

// Handle new master added event
func (self *Vrouter) MasterAdded(master *OfnetNode) error {
    // Send all local routes to new master.
    for _, route := range self.routeTable {
        if route.OriginatorIp.String() == self.agent.localIp.String() {
            var resp bool

            log.Infof("Sending route %+v to master %+v", route, master)

            // Make the RPC call to add the route to master
            client := rpcHub.Client(master.HostAddr, master.HostPort)
            err := client.Call("OfnetMaster.RouteAdd", route, &resp)
            if (err != nil) {
                log.Errorf("Failed to add route %+v to master %+v. Err: %v", route, master, err)
                return err
            }
        }
    }

    return nil
}


// Handle switch connected notification
func (self *Vrouter) SwitchConnected(sw *ofctrl.OFSwitch) {
    // Keep a reference to the switch
    self.ofSwitch = sw

    log.Infof("Switch connected(vrouter). installing flows")

    // Init the Fgraph
    self.initFgraph()
}

// Handle switch disconnected notification
func (self *Vrouter) SwitchDisconnected(sw *ofctrl.OFSwitch) {
    // FIXME: ??
}

// Handle incoming packet
func (self *Vrouter) PacketRcvd(sw *ofctrl.OFSwitch, pkt *ofctrl.PacketIn) {
    switch(pkt.Data.Ethertype) {
    case 0x0806:
        if ((pkt.Match.Type == openflow13.MatchType_OXM) &&
            (pkt.Match.Fields[0].Class == openflow13.OXM_CLASS_OPENFLOW_BASIC) &&
            (pkt.Match.Fields[0].Field == openflow13.OXM_FIELD_IN_PORT)) {
            // Get the input port number
            switch t := pkt.Match.Fields[0].Value.(type) {
            case *openflow13.InPortField:
                var inPortFld openflow13.InPortField
                inPortFld = *t

                self.processArp(pkt.Data, inPortFld.InPort)
            }

        }

    case 0x0800:
    default:
        log.Errorf("Received unknown ethertype: %x", pkt.Data.Ethertype)
    }
}

// Add a local endpoint and install associated local route
func (self *Vrouter) AddLocalEndpoint(endpoint EndpointInfo) error {
    // Install a flow entry for vlan mapping and point it to IP table
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
    err = portVlanFlow.Next(self.ipTable)
    if err != nil {
        log.Errorf("Error installing portvlan entry. Err: %v", err)
        return err
    }

    // save the flow entry
    self.portVlanFlowDb[endpoint.PortNo] = portVlanFlow

    // build the route to add
    route := OfnetRoute{
                IpAddr: endpoint.IpAddr,
                VrfId: 0,       // FIXME: get a VRF
                OriginatorIp: self.agent.localIp,
                PortNo: endpoint.PortNo,
                Timestamp:  time.Now(),
            }

    // Add the route to local and master's routing table
    self.localRouteAdd(&route)

    // Create the output port
    outPort, err := self.ofSwitch.OutputPort(endpoint.PortNo)
    if (err != nil) {
        log.Errorf("Error creating output port %d. Err: %v", endpoint.PortNo, err)
        return err
    }

    // Install the IP address
    ipFlow, err := self.ipTable.NewFlow(ofctrl.FlowMatch{
                            Priority: FLOW_MATCH_PRIORITY,
                            Ethertype: 0x0800,
                            IpDa: &endpoint.IpAddr,
                        })
    if (err != nil) {
        log.Errorf("Error creating flow for route: %+v. Err: %v", route, err)
        return err
    }

    // Set Mac addresses
    ipFlow.SetMacDa(endpoint.MacAddr)
    ipFlow.SetMacSa(self.myRouterMac)

    // Point the route at output port
    err = ipFlow.Next(outPort)
    if (err != nil) {
        log.Errorf("Error installing flow for route: %+v. Err: %v", route, err)
        return err
    }

    // Store the flow
    self.flowDb[endpoint.IpAddr.String()] = ipFlow

    return nil
}

// Find a route by output port number
// Note: Works only for local ports
func (self *Vrouter) findLocalRouteByPortno(portNo uint32) *OfnetRoute {
    for _, route := range self.routeTable {
        if (route.OriginatorIp.String() == self.agent.localIp.String()) &&
            (route.PortNo == portNo) {
            return route
        }
    }

    return nil
}

// Remove local endpoint
func (self *Vrouter) RemoveLocalEndpoint(portNo uint32) error {
    // Find the route from local port number
    route := self.findLocalRouteByPortno(portNo)
    if route == nil {
        log.Errorf("Could not find local route ")
    }

    // Remove the port vlan flow.
    portVlanFlow := self.portVlanFlowDb[portNo]
    if portVlanFlow != nil {
        err := portVlanFlow.Delete()
        if err != nil {
            log.Errorf("Error deleting portvlan flow. Err: %v", err)
        }
    }
    
    // Uninstall the route
    err := self.uninstallRoute(route)
    if err != nil {
        log.Errorf("Error uninstalling route. Err: %v", err)
    }

    // Remove it from route table
    delete(self.routeTable, route.IpAddr.String())

    // update timestamp so that this takes priority
    route.Timestamp = time.Now()

    // Send the route delete to all known masters
    for _, master := range self.agent.masterDb {
        var resp bool

        log.Infof("Sending DELETE route %+v to master %+v", route, master)

        // Make the RPC call to delete the route on master
        client := rpcHub.Client(master.HostAddr, master.HostPort)
        err := client.Call("OfnetMaster.RouteDel", route, &resp)
        if (err != nil) {
            log.Errorf("Failed to send DELETE route %+v to master %+v. Err: %v", route, master, err)
        }
    }

    return nil
}

// Add virtual tunnel end point. This is mainly used for mapping remote vtep IP
// to ofp port number.
func (self *Vrouter) AddVtepPort(portNo uint32, remoteIp net.IP) error {
    // Install a flow entry for default VNI/vlan and point it to IP table
    portVlanFlow, _ := self.vlanTable.NewFlow(ofctrl.FlowMatch{
                            Priority: FLOW_MATCH_PRIORITY,
                            InputPort: portNo,
                        })
    // FIXME: Need to match on tunnelId and set vlan-id per VRF
    portVlanFlow.SetVlan(1)
    portVlanFlow.Next(self.ipTable)

    // FIXME: walk all the routes and see if we can install it
    //        This could happen if a route made it to us before VTEP


    return nil
}

// Remove a VTEP port
func (self *Vrouter) RemoveVtepPort(portNo uint32, remoteIp net.IP) error {
    // walk all the routes and uninstall the ones pointing at remote host
    for _, route := range self.routeTable {
        // Find all the routes pointing at the remote VTEP
        if route.OriginatorIp.String() == remoteIp.String() {
            // Uninstall the route from HW
            err := self.uninstallRoute(route)
            if err != nil {
                log.Errorf("Error uninstalling route %+v. Err: %v", route, err)
            }
        }
    }

    return nil
}

// Add a vlan.
// This is mainly used for mapping vlan id to Vxlan VNI
func (self *Vrouter) AddVlan(vlanId uint16, vni uint32) error {
    // FIXME: Add this for multiple VRF support
    return nil
}

// Remove a vlan
func (self *Vrouter) RemoveVlan(vlanId uint16, vni uint32) error {
    // FIXME: Add this for multiple VRF support
    return nil
}

// Add remote route RPC call from master
func (self *Vrouter) RouteAdd(route *OfnetRoute, ret *bool) error {
    log.Infof("RouteAdd rpc call for route: %+v", route)

    // If this is a local route we are done
    if (route.OriginatorIp.String() == self.agent.localIp.String()) {
        return nil
    }

    // Check if we have the route already and which is more recent
    oldRoute := self.routeTable[route.IpAddr.String()]
    if (oldRoute != nil) {
        // If old route has more recent timestamp, nothing to do
        if (!route.Timestamp.After(oldRoute.Timestamp)) {
            return nil
        }
    }

    // First, add the route to local routing table
    self.routeTable[route.IpAddr.String()] = route

    // Lookup the VTEP for the route
    vtepPort := self.agent.vtepTable[route.OriginatorIp.String()]
    if (vtepPort == nil) {
        log.Errorf("Could not find the VTEP for route: %+v", route)

        return errors.New("VTEP not found")
    }

    // Install the route in OVS
    // Create an output port for the vtep
    outPort, err := self.ofSwitch.OutputPort(*vtepPort)
    if (err != nil) {
        log.Errorf("Error creating output port %d. Err: %v", *vtepPort, err)
        return err
    }

    // Install the IP address
    ipFlow, err := self.ipTable.NewFlow(ofctrl.FlowMatch{
                            Priority: FLOW_MATCH_PRIORITY,
                            Ethertype: 0x0800,
                            IpDa: &route.IpAddr,
                        })
    if (err != nil) {
        log.Errorf("Error creating flow for route: %+v. Err: %v", route, err)
        return err
    }

    // Set Mac addresses
    ipFlow.SetMacDa(self.myRouterMac)
    // This is strictly not required at the source OVS. Source mac will be
    // overwritten by the dest OVS anyway. We keep the source mac for debugging purposes..
    // ipFlow.SetMacSa(self.myRouterMac)

    // Set VNI
    // FIXME: hardcode VNI for default VRF.
    // FIXME: We need to use one fabric VNI per VRF
    ipFlow.SetTunnelId(1)

    // Point it to output port
    err = ipFlow.Next(outPort)
    if (err != nil) {
        log.Errorf("Error installing flow for route: %+v. Err: %v", route, err)
        return err
    }

    // Store it in flow db
    self.flowDb[route.IpAddr.String()] = ipFlow

    return nil
}

// Delete remote route RPC call from master
func (self *Vrouter) RouteDel(route *OfnetRoute, ret *bool) error {
    // If this is a local route we are done
    if (route.OriginatorIp.String() == self.agent.localIp.String()) {
        return nil
    }

    // Ignore duplicate delete requests we might receive from multiple
    // Ofnet masters
    if self.routeTable[route.IpAddr.String()] == nil {
        return nil
    }

    // Uninstall the route
    err := self.uninstallRoute(route)
    if err != nil {
        log.Errorf("Error uninstalling route. Err: %v", err)
    }

    // Remove it from route table
    delete(self.routeTable, route.IpAddr.String())

    return nil
}

// Add a local route to routing table and distribute it
func (self *Vrouter) localRouteAdd(route *OfnetRoute) error {
    // First, add the route to local routing table
    self.routeTable[route.IpAddr.String()] = route

    // Send the route to all known masters
    for _, master := range self.agent.masterDb {
        var resp bool

        log.Infof("Sending route %+v to master %+v", route, master)

        // Make the RPC call to add the route to master
        err := rpcHub.Client(master.HostAddr, master.HostPort).Call("OfnetMaster.RouteAdd", route, &resp)
        if (err != nil) {
            log.Errorf("Failed to add route %+v to master %+v. Err: %v", route, master, err)
            return err
        }
    }

    return nil
}

// Remove a route from OVS
func (self *Vrouter) uninstallRoute(route *OfnetRoute) error {
    // Find the flow entry
    ipFlow := self.flowDb[route.IpAddr.String()]
    if (ipFlow == nil) {
        log.Errorf("Error finding the flow for route: %+v", route)
        return errors.New("Flow not found")
    }

    // Delete the Fgraph entry
    err := ipFlow.Delete()
    if err != nil {
        log.Errorf("Error deleting the route: %+v. Err: %v", route, err)
    }

    return err
}


const VLAN_TBL_ID = 1
const IP_TBL_ID = 2

// initialize Fgraph on the switch
func (self *Vrouter) initFgraph() error {
    sw := self.ofSwitch

    // Create all tables
    self.inputTable = sw.DefaultTable()
    self.vlanTable, _ = sw.NewTable(VLAN_TBL_ID)
    self.ipTable, _ = sw.NewTable(IP_TBL_ID)

    //Create all drop entries
    // Drop mcast source mac
    bcastMac, _ := net.ParseMAC("01:00:00:00:00:00")
    bcastSrcFlow, _ := self.inputTable.NewFlow(ofctrl.FlowMatch{
                            Priority: FLOW_MATCH_PRIORITY,
                            MacSa: &bcastMac,
                            MacSaMask: &bcastMac,
                        })
    bcastSrcFlow.Next(sw.DropAction())

    // Redirect ARP packets to controller
    arpFlow, _ := self.inputTable.NewFlow(ofctrl.FlowMatch{
                            Priority: FLOW_MATCH_PRIORITY,
                            Ethertype: 0x0806,
                        })
    arpFlow.Next(sw.SendToController())

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

    // Drop all packets that miss IP lookup
    ipMissFlow, _ := self.ipTable.NewFlow(ofctrl.FlowMatch{
                            Priority: FLOW_MISS_PRIORITY,
                        })
    ipMissFlow.Next(sw.DropAction())

    return nil
}

// Process incoming ARP packets
func (self *Vrouter) processArp(pkt protocol.Ethernet, inPort uint32) {
    log.Debugf("processing ARP packet on port %d", inPort)
    switch t := pkt.Data.(type) {
    case *protocol.ARP:
        log.Debugf("ARP packet: %+v", *t)
        var arpHdr protocol.ARP = *t

        switch arpHdr.Operation {
        case protocol.Type_Request:
            // Lookup the Dest IP in the routing table
            route := self.routeTable[arpHdr.IPDst.String()]
            if route == nil {
                // If we dont know the IP address, dont send an ARP response
                log.Infof("Received ARP request for unknown IP: %v", arpHdr.IPDst)
                return
            }

            // Form an ARP response
            arpResp, _ := protocol.NewARP(protocol.Type_Reply)
            arpResp.HWSrc = self.myRouterMac
            arpResp.IPSrc = arpHdr.IPDst
            arpResp.HWDst = arpHdr.HWSrc
            arpResp.IPDst = arpHdr.IPSrc

            log.Infof("Sending ARP response: %+v", arpResp)

            // build the ethernet packet
            ethPkt := protocol.NewEthernet()
            ethPkt.HWDst = arpResp.HWDst
            ethPkt.HWSrc = arpResp.HWSrc
            ethPkt.Ethertype = 0x0806
            ethPkt.Data = arpResp

            log.Infof("Sending ARP response Ethernet: %+v", ethPkt)

            // Packet out
            pktOut := openflow13.NewPacketOut()
            pktOut.Data = ethPkt
            pktOut.AddAction(openflow13.NewActionOutput(inPort))

            log.Infof("Sending ARP response packet: %+v", pktOut)

            // Send it out
            self.ofSwitch.Send(pktOut)
        default:
            log.Infof("Dropping ARP response packet from port %d", inPort)
        }
    }
}
