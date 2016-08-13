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
	"errors"
	"fmt"
	"net"
	"net/rpc"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/ofnet/ofctrl"
	"github.com/contiv/ofnet/ovsdbDriver"
	"github.com/contiv/ofnet/rpcHub"
	"github.com/jainvipin/bitset"
	"github.com/shaleman/libOpenflow/openflow13"
	cmap "github.com/streamrail/concurrent-map"
)

// OfnetAgent state
type OfnetAgent struct {
	ctrler      *ofctrl.Controller // Controller instance
	ofSwitch    *ofctrl.OFSwitch   // Switch instance. Assumes single switch per agent
	localIp     net.IP             // Local IP to be used for tunnel end points
	MyPort      uint16             // Port where the agent's RPC server is listening
	MyAddr      string             // RPC server addr. same as localIp. different in testing environments
	isConnected bool               // Is the switch connected
	rpcServ     *rpc.Server        // jsonrpc server
	rpcListener net.Listener       // Listener
	dpName      string             // Datapath type
	datapath    OfnetDatapath      // Configured datapath
	protopath   OfnetProto         // Configured protopath

	masterDb      map[string]*OfnetNode // list of Masters
	masterDbMutex sync.Mutex            // Sync mutex for masterDb

	// Port to vlan mapping table
	portVlanMap      map[uint32]*uint16 // Map port number to vlan
	portVlanMapMutex sync.RWMutex

	//vni to vlan mapping table
	vniVlanMap   map[uint32]*uint16 // Map VNI to vlan
	vlanVniMap   map[uint16]*uint32 // Map vlan to VNI
	vlanVniMutex sync.RWMutex       // Sync mutex for vlan-vni and vni-vlan maps

	// VTEP database
	vtepTable      map[string]*uint32 // Map vtep IP to OVS port number
	vtepTableMutex sync.RWMutex       // Sync mutex for vtep table

	// Endpoint database
	endpointDb      cmap.ConcurrentMap // all known endpoints
	localEndpointDb cmap.ConcurrentMap // local port to endpoint map

	ovsDriver *ovsdbDriver.OvsDriver

	//Vrf information
	vrfIdBmp     *bitset.BitSet           // bit map to generate a vrf id
	vrfNameIdMap map[string]*uint16       // Map vrf name to vrf Id
	vrfIdNameMap map[uint16]*string       // Map vrf id to vrf Name
	vrfDb        map[string]*OfnetVrfInfo // Db of all the global vrfs
	vrfMutex     sync.RWMutex             // Sync mutex for all vrf tables

	vlanVrf      map[uint16]*string //vlan to vrf mapping
	vlanVrfMutex sync.RWMutex       // Sync mutex for vlan-vrf table

	fwdMode   string         ///forwarding mode routing or bridge
	GARPStats map[int]uint32 // per EPG garp stats.

	mutex sync.RWMutex
	// stats
	stats      map[string]uint64 // arbitrary stats
	errStats   map[string]uint64 // error stats
	statsMutex sync.Mutex        // Sync mutext for modifying stats
}

// local End point information
type EndpointInfo struct {
	PortNo            uint32
	EndpointGroup     int
	MacAddr           net.HardwareAddr
	Vlan              uint16
	IpAddr            net.IP
	Ipv6Addr          net.IP
	Vrf               string
	EndpointGroupVlan uint16
}

const FLOW_MATCH_PRIORITY = 100        // Priority for all match flows
const FLOW_FLOOD_PRIORITY = 10         // Priority for flood entries
const FLOW_MISS_PRIORITY = 1           // priority for table miss flow
const FLOW_POLICY_PRIORITY_OFFSET = 10 // Priority offset for policy rules

const (
	VLAN_TBL_ID           = 1
	SRV_PROXY_DNAT_TBL_ID = 2
	DST_GRP_TBL_ID        = 3
	POLICY_TBL_ID         = 4
	SRV_PROXY_SNAT_TBL_ID = 5
	IP_TBL_ID             = 6
	MAC_DEST_TBL_ID       = 7
)

// Create a new Ofnet agent and initialize it
/*  routerInfo[0] -> Uplink nexthop interface
 */
func NewOfnetAgent(bridgeName string, dpName string, localIp net.IP, rpcPort uint16,
	ovsPort uint16, routerInfo ...string) (*OfnetAgent, error) {
	log.Infof("Creating new ofnet agent for %s,%s,%d,%d,%d,%v \n", bridgeName, dpName, localIp, rpcPort, ovsPort, routerInfo)
	agent := new(OfnetAgent)

	// Init params
	agent.localIp = localIp
	agent.MyPort = rpcPort
	agent.MyAddr = localIp.String()
	agent.dpName = dpName

	agent.masterDb = make(map[string]*OfnetNode)
	agent.portVlanMap = make(map[uint32]*uint16)
	agent.vniVlanMap = make(map[uint32]*uint16)
	agent.vlanVniMap = make(map[uint16]*uint32)

	// Initialize vtep database
	agent.vtepTable = make(map[string]*uint32)

	// Initialize endpoint database
	agent.endpointDb = cmap.New()
	agent.localEndpointDb = cmap.New()

	// Initialize vrf database
	agent.vrfDb = make(map[string]*OfnetVrfInfo)
	agent.vrfIdNameMap = make(map[uint16]*string)
	agent.vrfNameIdMap = make(map[string]*uint16)
	agent.vrfIdBmp = bitset.New(256)
	agent.vlanVrf = make(map[uint16]*string)

	// stats db
	agent.stats = make(map[string]uint64)
	agent.errStats = make(map[string]uint64)

	// Create an openflow controller
	agent.ctrler = ofctrl.NewController(agent)

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
		agent.fwdMode = "routing"
	case "vxlan":
		agent.datapath = NewVxlan(agent, rpcServ)
		agent.fwdMode = "bridge"
	case "vlan":
		agent.datapath = NewVlanBridge(agent, rpcServ)
		agent.fwdMode = "bridge"
	case "vlrouter":
		agent.datapath = NewVlrouter(agent, rpcServ)
		agent.fwdMode = "routing"
		agent.ovsDriver = ovsdbDriver.NewOvsDriver(bridgeName)
		agent.protopath = NewOfnetBgp(agent, routerInfo)
	default:
		log.Fatalf("Unknown Datapath %s", dpName)
	}

	// Start listening to controller port
	go agent.ctrler.Listen(fmt.Sprintf(":%d", ovsPort))

	// Return it
	return agent, nil
}

// incrStats increment a stats counter by name
func (self *OfnetAgent) incrStats(statName string) {
	self.statsMutex.Lock()
	defer self.statsMutex.Unlock()

	currStats := self.stats[statName]
	currStats++
	self.stats[statName] = currStats
}

// getStats return current stats by name
func (self *OfnetAgent) getStats(statName string) uint64 {
	self.statsMutex.Lock()
	defer self.statsMutex.Unlock()

	return self.stats[statName]
}

// incrStats increment a stats counter by name
func (self *OfnetAgent) incrErrStats(errName string) {
	self.statsMutex.Lock()
	defer self.statsMutex.Unlock()

	currStats := self.stats[errName+"-ERROR"]
	currStats++
	self.stats[errName+"-ERROR"] = currStats
}

// getEndpointId Get a unique identifier for the endpoint.
func (self *OfnetAgent) getEndpointId(endpoint EndpointInfo) string {
	self.vlanVrfMutex.RLock()
	defer self.vlanVrfMutex.RUnlock()
	if vrf, ok := self.vlanVrf[endpoint.Vlan]; ok {
		return endpoint.IpAddr.String() + ":" + *vrf
	}
	return ""
}

func (self *OfnetAgent) getEndpointIdByIpVlan(ipAddr net.IP, vlan uint16) string {
	self.vlanVrfMutex.RLock()
	defer self.vlanVrfMutex.RUnlock()
	if vrf, ok := self.vlanVrf[vlan]; ok {
		return ipAddr.String() + ":" + *vrf
	}
	return ""
}

func (self *OfnetAgent) getEndpointByID(id string) *OfnetEndpoint {
	if key, ok := self.endpointDb.Get(id); ok {
		return key.(*OfnetEndpoint)
	}
	return nil

}

func (self *OfnetAgent) getEndpointIdByIpVrf(ipAddr net.IP, vrf string) string {
	return ipAddr.String() + ":" + vrf
}

func (self *OfnetAgent) getEndpointByIpVlan(ipAddr net.IP, vlan uint16) *OfnetEndpoint {
	self.vlanVrfMutex.RLock()
	defer self.vlanVrfMutex.RUnlock()

	if vrf, ok := self.vlanVrf[vlan]; ok {
		if key, ok := self.endpointDb.Get(ipAddr.String() + ":" + *vrf); ok {
			return key.(*OfnetEndpoint)
		}
	}
	return nil
}

func (self *OfnetAgent) getEndpointByIpVrf(ipAddr net.IP, vrf string) *OfnetEndpoint {

	if self.endpointDb != nil && vrf != "" {
		if key, ok := self.endpointDb.Get(ipAddr.String() + ":" + vrf); ok {
			return key.(*OfnetEndpoint)
		}
	}
	return nil
}

// GetLocalEndpoint finds the endpoint based on the port number
func (self *OfnetAgent) getLocalEndpoint(portNo uint32) *OfnetEndpoint {
	ep, found := self.localEndpointDb.Get(string(portNo))
	if found {
		return ep.(*OfnetEndpoint)
	}

	return nil
}

// Delete cleans up an ofnet agent
func (self *OfnetAgent) Delete() error {
	var resp bool
	// Disconnect from the switch
	log.Infof("Received Delete for switch %s", self.ofSwitch.DPID().String)
	if self.GetRouterInfo() != nil {
		err := self.DeleteBgp()
		if err != nil {
			log.Errorf("Error clearing Bgp state,err:%s", err)
			return err
		}
	}
	if self.ofSwitch != nil {
		self.ofSwitch.Disconnect()
	}

	// Cleanup the controller
	self.ctrler.Delete()

	// close listeners
	self.rpcListener.Close()

	time.Sleep(100 * time.Millisecond)

	// My info to send to master
	myInfo := new(OfnetNode)
	myInfo.HostAddr = self.MyAddr
	myInfo.HostPort = self.MyPort
	self.masterDbMutex.Lock()
	defer self.masterDbMutex.Unlock()
	for _, node := range self.masterDb {
		err := rpcHub.Client(node.HostAddr, node.HostPort).Call("OfnetMaster.UnRegisterNode", &myInfo, &resp)
		if err != nil {
			log.Errorf("Failed to register with the master %+v. Err: %v", node, err)
			return err
		}
	}
	return nil
}

// Handle switch connected event
func (self *OfnetAgent) SwitchConnected(sw *ofctrl.OFSwitch) {
	log.Infof("Switch %v connected", sw.DPID())

	// store it for future use.
	self.ofSwitch = sw

	// Inform the datapath
	self.datapath.SwitchConnected(sw)

	self.mutex.Lock()
	self.isConnected = true
	self.mutex.Unlock()
}

// Handle switch disconnect event
func (self *OfnetAgent) SwitchDisconnected(sw *ofctrl.OFSwitch) {
	log.Infof("Switch %v disconnected", sw.DPID())

	// Ignore if this error was not for current switch
	if sw.DPID().String() != self.ofSwitch.DPID().String() {
		return
	}

	// Inform the datapath
	self.datapath.SwitchDisconnected(sw)

	self.mutex.Lock()
	self.ofSwitch = nil
	self.isConnected = false
	self.mutex.Unlock()
}

// IsSwitchConnected returns true if switch is connected
func (self *OfnetAgent) IsSwitchConnected() bool {
	self.mutex.RLock()
	defer self.mutex.RUnlock()
	return self.isConnected
}

// WaitForSwitchConnection wait till switch connects
func (self *OfnetAgent) WaitForSwitchConnection() {
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
func (self *OfnetAgent) PacketRcvd(sw *ofctrl.OFSwitch, pkt *ofctrl.PacketIn) {
	log.Debugf("Packet received from switch %v. Packet: %+v", sw.DPID(), pkt)

	// Inform the datapath
	self.datapath.PacketRcvd(sw, pkt)

	// increment stats
	self.incrStats("PktRcvd")
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
	self.masterDbMutex.Lock()
	self.masterDb[masterKey] = master
	self.masterDbMutex.Unlock()

	// increment stats
	self.incrStats("MasterAdd")

	// My info to send to master
	myInfo := new(OfnetNode)
	myInfo.HostAddr = self.MyAddr
	myInfo.HostPort = self.MyPort

	// Register the agent with the master
	err := rpcHub.Client(master.HostAddr, master.HostPort).Call("OfnetMaster.RegisterNode", &myInfo, &resp)
	if err != nil {
		log.Errorf("Failed to register with the master %+v. Err: %v", master, err)

		// increment stats
		self.incrErrStats("RegisterNodeFailure")

		return err
	}

	// Perform master added callback so that datapaths can send their FDB to master
	err = self.datapath.MasterAdded(master)
	if err != nil {
		log.Errorf("Error making master added callback for %+v. Err: %v", master, err)
	}
	var ep *OfnetEndpoint
	// Send all local endpoints to new master.
	for endpoint := range self.localEndpointDb.IterBuffered() {
		ep = endpoint.Val.(*OfnetEndpoint)
		if ep.OriginatorIp.String() == self.localIp.String() {
			var resp bool

			log.Infof("Sending endpoint %+v to master %+v", ep, master)

			// Make the RPC call to add the endpoint to master
			client := rpcHub.Client(master.HostAddr, master.HostPort)
			err := client.Call("OfnetMaster.EndpointAdd", ep, &resp)
			if err != nil {
				log.Errorf("Failed to add endpoint %+v to master %+v. Err: %v", endpoint, master, err)

				// increment stats
				self.incrErrStats("MasterAddEndpointAddSendFailure")

				// continue sending other routes
			} else {
				// increment stats
				self.incrStats("MasterAddEndpointAddSent")
			}
		}
	}

	return nil
}

// Remove the master from master DB
func (self *OfnetAgent) RemoveMaster(masterInfo *OfnetNode) error {
	log.Infof("Deleting master: %+v", masterInfo)

	masterKey := fmt.Sprintf("%s:%d", masterInfo.HostAddr, masterInfo.HostPort)

	// Remove it from DB
	self.masterDbMutex.Lock()
	delete(self.masterDb, masterKey)
	self.masterDbMutex.Unlock()

	// increment stats
	self.incrStats("RemoveMaster")

	return nil
}

// InjectGARPs inject garps for all eps on the epg.
func (self *OfnetAgent) InjectGARPs(epgID int, resp *bool) error {
	self.datapath.InjectGARPs(epgID)

	// increment stats
	self.incrStats("InjectGARPs")

	return nil
}

// Add a local endpoint.
// This takes ofp port number, mac address, vlan , VrfId and IP address of the port.
func (self *OfnetAgent) AddLocalEndpoint(endpoint EndpointInfo) error {
	// Add port vlan mapping
	log.Infof("Received local endpoint add for {%+v}", endpoint)
	self.portVlanMapMutex.Lock()
	self.portVlanMap[endpoint.PortNo] = &endpoint.Vlan
	self.portVlanMapMutex.Unlock()

	// increment stats
	self.incrStats("AddLocalEndpoint")

	// Map Vlan to VNI
	self.vlanVniMutex.RLock()
	vni := self.vlanVniMap[endpoint.Vlan]
	self.vlanVniMutex.RUnlock()
	if vni == nil {
		log.Errorf("VNI for vlan %d is not known", endpoint.Vlan)
		return errors.New("Unknown Vlan")
	}

	epId := self.getEndpointIdByIpVlan(endpoint.IpAddr, endpoint.Vlan)
	// ignore duplicate adds
	if ep, _ := self.localEndpointDb.Get(string(endpoint.PortNo)); ep != nil {
		e := ep.(*OfnetEndpoint)
		if e.EndpointID == epId {
			return nil
		}
	}
	self.vlanVrfMutex.RLock()
	vrf := self.vlanVrf[endpoint.Vlan]
	self.vlanVrfMutex.RUnlock()

	if vrf == nil {
		log.Errorf("Invalid vlan to vrf mapping for %v", endpoint.Vlan)
		return errors.New("Invalid vlan to vrf mapping")
	}

	var v6mask net.IP
	if endpoint.Ipv6Addr != nil {
		v6mask = net.ParseIP("ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff")
	}

	// Build endpoint registry info
	epreg := &OfnetEndpoint{
		EndpointID:        epId,
		EndpointType:      "internal",
		EndpointGroup:     endpoint.EndpointGroup,
		IpAddr:            endpoint.IpAddr,
		IpMask:            net.ParseIP("255.255.255.255"),
		Ipv6Addr:          endpoint.Ipv6Addr,
		Ipv6Mask:          v6mask,
		Vrf:               *vrf,
		MacAddrStr:        endpoint.MacAddr.String(),
		Vlan:              endpoint.Vlan,
		Vni:               *vni,
		OriginatorIp:      self.localIp,
		PortNo:            endpoint.PortNo,
		Timestamp:         time.Now(),
		EndpointGroupVlan: endpoint.EndpointGroupVlan,
	}

	// Call the datapath
	err := self.datapath.AddLocalEndpoint(*epreg)
	if err != nil {
		log.Errorf("Adding endpoint (%+v) to datapath. Err: %v", epreg, err)
		return err
	}

	// Add the endpoint to local routing table

	self.endpointDb.Set(epId, epreg)
	self.localEndpointDb.Set(string(endpoint.PortNo), epreg)

	// Send the endpoint to all known masters
	self.masterDbMutex.Lock()
	for _, master := range self.masterDb {
		var resp bool

		log.Infof("Sending endpoint %+v to master %+v", epreg, master)

		// Make the RPC call to add the endpoint to master
		err := rpcHub.Client(master.HostAddr, master.HostPort).Call("OfnetMaster.EndpointAdd", epreg, &resp)
		if err != nil {
			log.Errorf("Failed to add endpoint %+v to master %+v. Err: %v", epreg, master, err)
			// Continue sending the message to other masters.
		} else {
			// increment stats
			self.incrStats("EndpointAddSent")
		}
	}
	self.masterDbMutex.Unlock()
	log.Infof("Local Endpoint added and distributed successfully {%+v}", epreg)
	return nil
}

// Remove local endpoint
func (self *OfnetAgent) RemoveLocalEndpoint(portNo uint32) error {
	// increment stats
	self.incrStats("RemoveLocalEndpoint")

	// find the local copy
	epreg, _ := self.localEndpointDb.Get(string(portNo))
	if epreg == nil {
		log.Errorf("Endpoint not found for port %d", portNo)
		return errors.New("Endpoint not found")
	}
	ep := epreg.(*OfnetEndpoint)
	log.Infof("Received local endpoint remove and withdraw for {%+v}", epreg)
	// Call the datapath
	err := self.datapath.RemoveLocalEndpoint(*ep)
	if err != nil {
		log.Errorf("Error deleting endpoint port %d. Err: %v", portNo, err)
	}

	// delete the endpoint from local endpoint table

	self.endpointDb.Remove(ep.EndpointID)
	self.localEndpointDb.Remove(string(portNo))
	self.portVlanMapMutex.Lock()
	delete(self.portVlanMap, portNo)
	self.portVlanMapMutex.Unlock()

	// Send the DELETE to all known masters
	self.masterDbMutex.Lock()
	for _, master := range self.masterDb {
		var resp bool

		log.Infof("Sending DELETE endpoint %+v to master %+v", ep, master)

		// Make the RPC call to delete the endpoint on master
		client := rpcHub.Client(master.HostAddr, master.HostPort)
		err := client.Call("OfnetMaster.EndpointDel", ep, &resp)
		if err != nil {
			log.Errorf("Failed to DELETE endpoint %+v on master %+v. Err: %v", epreg, master, err)
		} else {
			// increment stats
			self.incrStats("EndpointDelSent")
		}
	}
	self.masterDbMutex.Unlock()
	log.Infof("Local endpoint removed and withdrawn successfully")
	return nil
}

// Add virtual tunnel end point. This is mainly used for mapping remote vtep IP
// to ofp port number.
func (self *OfnetAgent) AddVtepPort(portNo uint32, remoteIp net.IP) error {
	// Ignore duplicate Add vtep messages
	self.vtepTableMutex.Lock()

	oldPort, ok := self.vtepTable[remoteIp.String()]
	if ok && *oldPort == portNo {
		self.vtepTableMutex.Unlock()
		return nil
	}

	log.Infof("Received Add VTEP port(%d), Remote IP: %v", portNo, remoteIp)

	// Dont handle endpointDB operations during this time

	// Store the vtep IP to port number mapping
	self.vtepTable[remoteIp.String()] = &portNo
	self.vtepTableMutex.Unlock()
	// Call the datapath
	return self.datapath.AddVtepPort(portNo, remoteIp)
}

// Remove a VTEP port
func (self *OfnetAgent) RemoveVtepPort(portNo uint32, remoteIp net.IP) error {
	// Clear the vtep IP to port number mapping
	log.Infof("Received Remove VTEP port(%d), Remote IP: %v", portNo, remoteIp)
	self.vtepTableMutex.Lock()
	delete(self.vtepTable, remoteIp.String())
	self.vtepTableMutex.Unlock()

	// walk all the endpoints and uninstall the ones pointing at remote host
	for endpoint := range self.endpointDb.IterBuffered() {
		ep := endpoint.Val.(*OfnetEndpoint)
		// Find all the routes pointing at the remote VTEP
		if ep.OriginatorIp.String() == remoteIp.String() {
			if val, _ := self.endpointDb.Get(ep.EndpointID); val != nil {
				// Uninstall the route from HW
				err := self.datapath.RemoveEndpoint(ep)
				if err != nil {
					log.Errorf("Error uninstalling endpoint %+v. Err: %v", ep, err)
				} else {
					// Remove it from endpoint table
					self.endpointDb.Remove(ep.EndpointID)
				}
			}
		}
	}
	// Call the datapath
	return self.datapath.RemoveVtepPort(portNo, remoteIp)
}

// Add a Network.
// This is mainly used for mapping vlan id to Vxlan VNI and add gateway for network
func (self *OfnetAgent) AddNetwork(vlanId uint16, vni uint32, Gw string, Vrf string) error {

	log.Infof("Received Add Network for  Vlan %d. Vni %d Gw %s Vrf %s", vlanId, vni, Gw, Vrf)
	// if nothing changed, ignore the message
	self.vlanVniMutex.Lock()
	oldVni, ok := self.vlanVniMap[vlanId]
	if ok && *oldVni == vni {
		self.vlanVniMutex.Unlock()
		return nil
	}

	// store it in DB
	self.vlanVniMap[vlanId] = &vni
	self.vniVlanMap[vni] = &vlanId
	self.vlanVniMutex.Unlock()
	// Call the datapath
	err := self.datapath.AddVlan(vlanId, vni, Vrf)
	if err != nil {
		return err
	}

	self.vlanVrfMutex.RLock()
	vrf := self.vlanVrf[vlanId]
	self.vlanVrfMutex.RUnlock()

	gwEpid := self.getEndpointIdByIpVrf(net.ParseIP(Gw), *vrf)

	if Gw != "" && self.fwdMode == "routing" {
		// Call the datapath
		epreg := &OfnetEndpoint{
			EndpointID:   gwEpid,
			EndpointType: "internal",
			IpAddr:       net.ParseIP(Gw),
			IpMask:       net.ParseIP("255.255.255.255"),
			Vrf:          *vrf,
			Vni:          vni,
			Vlan:         vlanId,
			PortNo:       0,
			Timestamp:    time.Now(),
		}
		self.endpointDb.Set(gwEpid, epreg)
		// increment stats
	}
	self.incrStats("AddNetwork")

	return nil
}

// Remove a vlan from datapath
func (self *OfnetAgent) RemoveNetwork(vlanId uint16, vni uint32, Gw string, Vrf string) error {
	// Dont handle endpointDB operations during this time
	log.Infof("Received Remove Network for  Vlan %d. Vni %d Vrf:%s", vlanId, vni, Vrf)
	gwEpid := self.getEndpointIdByIpVlan(net.ParseIP(Gw), vlanId)

	self.endpointDb.Remove(gwEpid)

	// make sure there are no endpoints still installed in this vlan
	for endpoint := range self.endpointDb.IterBuffered() {
		ep := endpoint.Val.(*OfnetEndpoint)
		if (vni != 0) && (ep.Vni == vni) {
			if ep.OriginatorIp.String() == self.localIp.String() {
				log.Fatalf("Vlan %d still has routes. Route: %+v", vlanId, ep)
			} else if ep.EndpointType == "internal" {
				// Network delete arrived before other hosts cleanup endpoint
				log.Warnf("Vlan %d still has routes, cleaning up. Route: %+v", vlanId, ep)
				// Uninstall the endpoint from datapath
				err := self.datapath.RemoveEndpoint(ep)
				if err != nil {
					log.Errorf("Error deleting endpoint: {%+v}. Err: %v", ep, err)
				}

				// Remove it from endpoint table
				self.endpointDb.Remove(ep.EndpointID)
			}
		}
	}
	// Clear the database
	self.vlanVniMutex.Lock()
	delete(self.vlanVniMap, vlanId)
	delete(self.vniVlanMap, vni)
	self.vlanVniMutex.Unlock()
	// increment stats
	self.incrStats("RemoveNetwork")

	// Call the datapath
	return self.datapath.RemoveVlan(vlanId, vni, Vrf)
}

// AddUplink adds an uplink to the switch
func (self *OfnetAgent) AddUplink(portNo uint32, ifname string) error {
	// Call the datapath
	return self.datapath.AddUplink(portNo, ifname)
}

// RemoveUplink remove an uplink to the switch
func (self *OfnetAgent) RemoveUplink(portNo uint32) error {
	// Call the datapath
	return self.datapath.RemoveUplink(portNo)
}

// AddSvcSpec adds a service spec to proxy
func (self *OfnetAgent) AddSvcSpec(svcName string, spec *ServiceSpec) error {
	// increment stats
	self.incrStats("AddSvcSpec")

	return self.datapath.AddSvcSpec(svcName, spec)
}

// DelSvcSpec removes a service spec from proxy
func (self *OfnetAgent) DelSvcSpec(svcName string, spec *ServiceSpec) error {
	// increment stats
	self.incrStats("DelSvcSpec")

	return self.datapath.DelSvcSpec(svcName, spec)
}

// SvcProviderUpdate Service Proxy Back End update
func (self *OfnetAgent) SvcProviderUpdate(svcName string, providers []string) {
	// increment stats
	self.incrStats("SvcProviderUpdate")

	self.datapath.SvcProviderUpdate(svcName, providers)
}

// Add remote endpoint RPC call from master
func (self *OfnetAgent) EndpointAdd(epreg *OfnetEndpoint, ret *bool) error {
	var oldEp *OfnetEndpoint
	log.Infof("EndpointAdd rpc call for endpoint: %+v. localIp: %v", epreg, self.localIp)

	// If this is a local endpoint we are done
	if epreg.OriginatorIp.String() == self.localIp.String() {
		return nil
	}

	// switch connection is not up, return
	if !self.IsSwitchConnected() {
		log.Warnf("Received EndpointAdd for {%+v} before switch connection was up ", epreg)
		return nil
	}

	// increment stats
	self.incrStats("EndpointAddRcvd")

	// Dont handle other endpointDB operations during this time
	// Check if we have the endpoint already and which is more recent
	ep, ok := self.endpointDb.Get(epreg.EndpointID)
	if ok {
		oldEp = ep.(*OfnetEndpoint)
	}

	if oldEp != nil {
		// If old endpoint has more recent timestamp, nothing to do
		if !epreg.Timestamp.After(oldEp.Timestamp) {
			return nil
		} else {
			// Uninstall the old endpoint from datapath
			err := self.datapath.RemoveEndpoint(oldEp)
			if err != nil {
				log.Errorf("Error deleting old endpoint: {%+v}. Err: %v", oldEp, err)
			}
		}
	}

	// First, add the endpoint to local routing table
	self.endpointDb.Set(epreg.EndpointID, epreg)
	// Install the endpoint in datapath
	err := self.datapath.AddEndpoint(epreg)
	if err != nil {
		log.Errorf("Error adding endpoint: {%+v}. Err: %v", epreg, err)
		return err
	}
	log.Infof("Remote endpoint add successful for endpoint : {%+v}", epreg)
	return nil
}

// Delete remote endpoint RPC call from master
func (self *OfnetAgent) EndpointDel(epreg *OfnetEndpoint, ret *bool) error {
	// If this is a local endpoint we are done
	log.Infof("Received remote endpoint delete for endpoint {%+v}", epreg)

	if epreg.OriginatorIp.String() == self.localIp.String() {
		return nil
	}

	// Ignore duplicate delete requests we might receive from multiple
	// Ofnet masters
	if val, _ := self.endpointDb.Get(epreg.EndpointID); val == nil {
		return nil
	}

	// increment stats
	self.incrStats("EndpointDelRcvd")

	// Dont handle endpointDB operations during this time

	// Uninstall the endpoint from datapath
	err := self.datapath.RemoveEndpoint(epreg)
	if err != nil {
		log.Errorf("Error deleting endpoint: {%+v}. Err: %v", epreg, err)
	}

	// Remove it from endpoint table
	self.endpointDb.Remove(epreg.EndpointID)

	return nil
}

func (self *OfnetAgent) DummyRpc(arg *string, ret *bool) error {
	log.Infof("Received dummy route RPC call")
	return nil
}

//AddBgpNeighbors add bgp neighbor
func (self *OfnetAgent) AddBgp(routerIP string, As string, neighborAs string, peer string) error {

	log.Infof("Received BGP config: RouterIp:%s, As:%s, NeighborAs:%s, PeerIP:%s", routerIP, As, neighborAs, peer)

	if self.protopath == nil {
		log.Errorf("Ofnet is not initialized in routing mode")
		return errors.New("Ofnet not in routing mode")
	}
	routerInfo := &OfnetProtoRouterInfo{
		ProtocolType: "bgp",
		RouterIP:     routerIP,
		As:           As,
	}
	neighborInfo := &OfnetProtoNeighborInfo{
		ProtocolType: "bgp",
		NeighborIP:   peer,
		As:           neighborAs,
	}
	rinfo := self.GetRouterInfo()
	if rinfo != nil {
		self.DeleteBgp()
	}

	go self.protopath.StartProtoServer(routerInfo)

	err := self.protopath.AddProtoNeighbor(neighborInfo)
	if err != nil {
		log.Errorf("Error adding protocol neighbor")
		return err
	}
	return nil
}

func (self *OfnetAgent) DeleteBgp() error {
	log.Infof("Received Delete BGP neighbor config")
	if self.protopath == nil {
		log.Errorf("Ofnet is not initialized in routing mode")
		return errors.New("Ofnet not in routing mode")
	}
	if self.GetRouterInfo() != nil {
		err := self.protopath.DeleteProtoNeighbor()
		if err != nil {
			log.Errorf("Error deleting protocol neighbor")
			return err
		}
		err = self.protopath.StopProtoServer()
		if err != nil {
			log.Errorf("Error stopping bgp server,err:%v", err)
		}
		return err
	}
	return nil
}

func (self *OfnetAgent) GetRouterInfo() *OfnetProtoRouterInfo {
	if self.protopath != nil {
		return self.protopath.GetRouterInfo()
	}
	return nil
}

func (self *OfnetAgent) AddLocalProtoRoute(path *OfnetProtoRouteInfo) {
	if self.protopath != nil {
		self.protopath.AddLocalProtoRoute(path)
	}
}

func (self *OfnetAgent) DeleteLocalProtoRoute(path *OfnetProtoRouteInfo) {
	if self.protopath != nil {
		self.protopath.DeleteLocalProtoRoute(path)
	}
}

// MultipartReply Receives a multi-part reply from the switch.
func (self *OfnetAgent) MultipartReply(sw *ofctrl.OFSwitch, reply *openflow13.MultipartReply) {
	log.Debugf("Multi-part reply received from switch: %+v", reply)

	// Inform the datapath
	self.datapath.MultipartReply(sw, reply)
}

// GetEndpointStats fetches all endpoint stats
func (self *OfnetAgent) GetEndpointStats() ([]*OfnetEndpointStats, error) {
	return self.datapath.GetEndpointStats()
}

// InspectBgp returns ofnet bgp state
func (self *OfnetAgent) InspectBgp() (interface{}, error) {
	peer, err := self.protopath.InspectProto()
	return peer, err
}

// InspectState returns ofnet agent state
func (self *OfnetAgent) InspectState() (interface{}, error) {
	dpState, err := self.datapath.InspectState()
	if err != nil {
		log.Errorf("Error getting state from datapath. Err: %v", err)
		return nil, err
	}

	// convert ofnet struct to an exported struct for json marshaling
	ofnetExport := struct {
		LocalIp     net.IP                // Local IP to be used for tunnel end points
		MyPort      uint16                // Port where the agent's RPC server is listening
		MyAddr      string                // RPC server addr. same as localIp. different in testing environments
		IsConnected bool                  // Is the switch connected
		DpName      string                // Datapath type
		Protopath   OfnetProto            // Configured protopath
		MasterDb    map[string]*OfnetNode // list of Masters
		// PortVlanMap     map[uint32]*uint16        // Map port number to vlan
		// VniVlanMap      map[uint32]*uint16        // Map VNI to vlan
		// VlanVniMap      map[uint16]*uint32        // Map vlan to VNI
		VtepTable  map[string]*uint32     // Map vtep IP to OVS port number
		EndpointDb map[string]interface{} // all known endpoints
		// LocalEndpointDb map[uint32]*OfnetEndpoint // local port to endpoint map
		VrfNameIdMap map[string]*uint16 // Map vrf name to vrf Id
		// VrfIdNameMap    map[uint16]*string        // Map vrf id to vrf Name
		VrfDb map[string]*OfnetVrfInfo // Db of all the global vrfs
		// VlanVrf         map[uint16]*string        //vlan to vrf mapping
		FwdMode  string            ///forwarding mode routing or bridge
		Stats    map[string]uint64 // arbitrary stats
		ErrStats map[string]uint64 // error stats
		Datapath interface{}       // datapath state
	}{
		self.localIp,
		self.MyPort,
		self.MyAddr,
		self.isConnected,
		self.dpName,
		self.protopath,
		self.masterDb,
		// self.portVlanMap,
		// self.vniVlanMap,
		// self.vlanVniMap,
		self.vtepTable,
		self.endpointDb.Items(),
		// self.localEndpointDb,
		self.vrfNameIdMap,
		// self.vrfIdNameMap,
		self.vrfDb,
		// self.vlanVrf,
		self.fwdMode,
		self.stats,
		self.errStats,
		dpState,
	}

	return &ofnetExport, nil
}

func (self *OfnetAgent) createVrf(Vrf string) (uint16, bool) {

	log.Infof("Received create vrf for %s \n", Vrf)
	self.vrfMutex.Lock()
	defer self.vrfMutex.Unlock()
	if vrfid, ok := self.vrfNameIdMap[Vrf]; ok {
		self.vrfDb[Vrf].NumNetworks++
		return *vrfid, ok
	}

	vrfid, ok := self.vrfIdBmp.NextClear(1)

	if !ok {
		log.Errorf("Error allocating vrf id for Vrf:%s", Vrf)
		return 0, ok
	}
	self.vrfIdBmp.Set(vrfid)
	vrfInfo := &OfnetVrfInfo{
		NumNetworks: 1,
		VrfName:     Vrf,
		VrfId:       uint16(vrfid),
	}
	self.vrfIdNameMap[vrfInfo.VrfId] = &Vrf
	self.vrfNameIdMap[Vrf] = &vrfInfo.VrfId
	self.vrfDb[Vrf] = vrfInfo
	return vrfInfo.VrfId, ok
}

func (self *OfnetAgent) deleteVrf(Vrf string) error {

	self.vrfMutex.Lock()
	defer self.vrfMutex.Unlock()
	if vrfid, ok := self.vrfNameIdMap[Vrf]; ok {
		self.vrfDb[Vrf].NumNetworks--
		if self.vrfDb[Vrf].NumNetworks == 0 {
			self.vrfIdBmp.Clear(uint(*vrfid))
			delete(self.vrfNameIdMap, Vrf)
			delete(self.vrfIdNameMap, *vrfid)
			delete(self.vrfDb, Vrf)
		}
		return nil
	}
	return errors.New("Unknown Vrf")
}

func (self *OfnetAgent) getPortVlanMap(port uint32) *uint16 {
	self.portVlanMapMutex.RLock()
	defer self.portVlanMapMutex.RUnlock()
	return self.portVlanMap[port]
}

func (self *OfnetAgent) getvniVlanMap(vni uint32) *uint16 {
	self.vlanVniMutex.RLock()
	defer self.vlanVniMutex.RUnlock()
	return self.vniVlanMap[vni]
}

func (self *OfnetAgent) getvlanVniMap(vlan uint16) *uint32 {
	self.vlanVniMutex.RLock()
	defer self.vlanVniMutex.RUnlock()
	return self.vlanVniMap[vlan]
}

func (self *OfnetAgent) getvtepTablePort(ip string) *uint32 {
	self.vtepTableMutex.RLock()
	defer self.vtepTableMutex.RUnlock()
	return self.vtepTable[ip]
}

func (self *OfnetAgent) getvrfId(name string) *uint16 {
	self.vrfMutex.RLock()
	defer self.vrfMutex.RUnlock()
	return self.vrfNameIdMap[name]
}

func (self *OfnetAgent) getvrfName(id uint16) *string {
	self.vrfMutex.RLock()
	defer self.vrfMutex.RUnlock()
	return self.vrfIdNameMap[id]
}

func (self *OfnetAgent) getvlanVrf(vlan uint16) *string {
	self.vlanVrfMutex.Lock()
	defer self.vlanVrfMutex.Unlock()
	return self.vlanVrf[vlan]
}
