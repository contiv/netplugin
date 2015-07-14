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

// This file implements ofnet agent API which runs on each host alongside OVS.
// This assumes:
//      - ofnet agent is running on each host
//      - There is single OVS switch instance(aka bridge instance)
//      - OVS switch's forwarding is fully controller by ofnet agent
//
// It also assumes OVS is configured for openflow1.3 version and configured
// to connect to controller on specified port

import (
    "fmt"
    "net"
    "net/rpc"
    "time"

    "github.com/contiv/ofnet/ofctrl"
    "github.com/contiv/ofnet/rpcHub"

    log "github.com/Sirupsen/logrus"
)

// OfnetAgent state
type OfnetAgent struct {
    ctrler      *ofctrl.Controller      // Controller instance
    ofSwitch    *ofctrl.OFSwitch        // Switch instance. Assumes single switch per agent
    localIp     net.IP                  // Local IP to be used for tunnel end points
    MyPort      uint16                  // Port where the agent's RPC server is listening
    MyAddr      string                  // RPC server addr. same as localIp. different in testing environments
    isConnected bool                    // Is the switch connected

    rpcServ     *rpc.Server             // jsonrpc server
    rpcListener net.Listener           // Listener
    datapath    OfnetDatapath           // Configured datapath

    masterDb    map[string]*OfnetNode      // list of Masters

    // Port and VNI to vlan mapping table
    portVlanMap map[uint32]*uint16       // Map port number to vlan
    vniVlanMap  map[uint32]*uint16       // Map VNI to vlan
    vlanVniMap  map[uint16]*uint32       // Map vlan to VNI

    // VTEP database
    vtepTable   map[string]*uint32      // Map vtep IP to OVS port number
}

// local End point information
type EndpointInfo struct {
    PortNo      uint32
    MacAddr     net.HardwareAddr
    Vlan        uint16
    IpAddr      net.IP
}

const FLOW_MATCH_PRIORITY = 100     // Priority for all match flows
const FLOW_FLOOD_PRIORITY = 10      // Priority for flood entries
const FLOW_MISS_PRIORITY = 1        // priority for table miss flow


// Create a new Ofnet agent and initialize it
func NewOfnetAgent(dpName string, localIp net.IP, rpcPort uint16, ovsPort uint16) (*OfnetAgent, error) {
    agent := new(OfnetAgent)

    // Init params
    agent.localIp = localIp
    agent.MyPort = rpcPort
    agent.MyAddr = localIp.String()

    agent.masterDb = make(map[string]*OfnetNode)
    agent.portVlanMap = make(map[uint32]*uint16)
    agent.vniVlanMap = make(map[uint32]*uint16)
    agent.vlanVniMap = make(map[uint16]*uint32)

    agent.vtepTable = make(map[string]*uint32)

    // Create an openflow controller
    agent.ctrler = ofctrl.NewController(agent)

    // Start listening to controller port
    go agent.ctrler.Listen(fmt.Sprintf(":%d", ovsPort))

    // Create rpc server
    // FIXME: Figure out how to handle multiple OVS bridges.
    rpcServ, listener := rpcHub.NewRpcServer(rpcPort)
    agent.rpcServ = rpcServ
    agent.rpcListener = listener

    // Register for Master add/remove events
    rpcServ.Register(agent)

    // Create the datapath
    switch dpName {
    case "vrouter":
        agent.datapath = NewVrouter(agent, rpcServ)
    case "vxlan":
        agent.datapath = NewVxlan(agent, rpcServ)
    default:
        log.Fatalf("Unknown Datapath %s", dpName)
    }

    // Return it
    return agent, nil
}

// Delete cleans up an ofnet agent
func (self *OfnetAgent) Delete() error {
    // Disconnect from the switch
    if self.ofSwitch != nil {
        self.ofSwitch.Disconnect()
    }

    // Cleanup the controller
    self.ctrler.Delete()

    // close listeners
    self.rpcListener.Close()

    time.Sleep(100 * time.Millisecond)

    return nil
}

// Handle switch connected event
func (self *OfnetAgent) SwitchConnected(sw *ofctrl.OFSwitch) {
    log.Infof("Switch %v connected", sw.DPID())

    // store it for future use.
    self.ofSwitch = sw

    // Inform the datapath
    self.datapath.SwitchConnected(sw)

    // add default vlan
    //self.AddVlan(1, 1)

    self.isConnected = true
}

// Handle switch disconnect event
func (self *OfnetAgent) SwitchDisconnected(sw *ofctrl.OFSwitch) {
    log.Infof("Switch %v disconnected", sw.DPID())

    // Inform the datapath
    self.datapath.SwitchDisconnected(sw)

    self.ofSwitch = nil
    self.isConnected = false
}

// IsSwitchConnected returns true if switch is connected
func (self *OfnetAgent) IsSwitchConnected() bool {
    return self.isConnected
}

// Receive a packet from the switch.
func (self *OfnetAgent) PacketRcvd(sw *ofctrl.OFSwitch, pkt *ofctrl.PacketIn) {
    log.Infof("Packet received from switch %v. Packet: %+v", sw.DPID(), pkt)
    log.Infof("Input Port: %+v", pkt.Match.Fields[0].Value)

    // Inform the datapath
    self.datapath.PacketRcvd(sw, pkt)
}

// Add a master
// ofnet agent tries to connect to the master and download routes
func (self *OfnetAgent) AddMaster(masterInfo *OfnetNode, ret *bool) error {
    master := new(OfnetNode)
    master.HostAddr = masterInfo.HostAddr
    master.HostPort = masterInfo.HostPort

    var resp bool

    log.Infof("Adding master: %+v", *master)

    masterKey := fmt.Sprintf("%s:%d", masterInfo.HostAddr, masterInfo.HostPort)

    // Save it in DB
    self.masterDb[masterKey] = master

    // My info to send to master
    myInfo := new(OfnetNode)
    myInfo.HostAddr = self.MyAddr
    myInfo.HostPort = self.MyPort

    // Register the agent with the master
    err := rpcHub.Client(master.HostAddr, master.HostPort).Call("OfnetMaster.RegisterNode", &myInfo, &resp)
    if (err != nil) {
        log.Fatalf("Failed to register with the master %+v. Err: %v", master, err)
        return err
    }

    // Perform master added callback so that datapaths can send their FDB to master
    err = self.datapath.MasterAdded(master)
    if err != nil {
        log.Errorf("Error making master added callback for %+v. Err: %v", master, err)
    }

    return nil
}

// Remove the master from master DB
func (self *OfnetAgent) RemoveMaster(masterInfo *OfnetNode) error {
    log.Infof("Deleting master: %+v", masterInfo)

    masterKey := fmt.Sprintf("%s:%d", masterInfo.HostAddr, masterInfo.HostPort)

    // Remove it from DB
    delete(self.masterDb, masterKey)

    return nil
}

// Add a local endpoint.
// This takes ofp port number, mac address, vlan and IP address of the port.
func (self *OfnetAgent) AddLocalEndpoint(endpoint EndpointInfo) error {
    // Add port vlan mapping
    self.portVlanMap[endpoint.PortNo] = &endpoint.Vlan

    // Call the datapath
    return self.datapath.AddLocalEndpoint(endpoint)
}

// Remove local endpoint
func (self *OfnetAgent) RemoveLocalEndpoint(portNo uint32) error {
    // Clear it from DB
    delete(self.portVlanMap, portNo)

    // Call the datapath
    return self.datapath.RemoveLocalEndpoint(portNo)
}

// Add virtual tunnel end point. This is mainly used for mapping remote vtep IP
// to ofp port number.
func (self *OfnetAgent) AddVtepPort(portNo uint32, remoteIp net.IP) error {
    log.Infof("Adding VTEP port(%d), Remote IP: %v", portNo, remoteIp)

    // Store the vtep IP to port number mapping
    self.vtepTable[remoteIp.String()] = &portNo

    // Call the datapath
    return self.datapath.AddVtepPort(portNo, remoteIp)
}

// Remove a VTEP port
func (self *OfnetAgent) RemoveVtepPort(portNo uint32, remoteIp net.IP) error {
    // Clear the vtep IP to port number mapping
    delete(self.vtepTable, remoteIp.String())

    // Call the datapath
    return self.datapath.RemoveVtepPort(portNo, remoteIp)
}

// Add a vlan.
// This is mainly used for mapping vlan id to Vxlan VNI
func (self *OfnetAgent) AddVlan(vlanId uint16, vni uint32) error {
    // store it in DB
    self.vlanVniMap[vlanId] = &vni
    self.vniVlanMap[vni] = &vlanId

    // Call the datapath
    return self.datapath.AddVlan(vlanId, vni)
}

// Remove a vlan from datapath
func (self *OfnetAgent) RemoveVlan(vlanId uint16, vni uint32) error {
    // Clear the database
    delete(self.vlanVniMap, vlanId)
    delete(self.vniVlanMap, vni)

    // Call the datapath
    return self.datapath.RemoveVlan(vlanId, vni)
}

func (self *OfnetAgent) DummyRpc(arg *string, ret *bool) error {
    log.Infof("Received dummy route RPC call")
    return nil
}
