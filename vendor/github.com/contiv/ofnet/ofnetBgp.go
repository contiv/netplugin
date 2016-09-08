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

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	bgp "github.com/osrg/gobgp/packet/bgp"
	table "github.com/osrg/gobgp/table"

	api "github.com/osrg/gobgp/api"
	bgpconf "github.com/osrg/gobgp/config"
	gobgp "github.com/osrg/gobgp/server"
	"github.com/shaleman/libOpenflow/openflow13"
	"github.com/shaleman/libOpenflow/protocol"
	"github.com/vishvananda/netlink"
	"google.golang.org/grpc"
)

type OfnetBgp struct {
	sync.Mutex
	routerIP string      // virtual interface ip for bgp
	vlanIntf string      // uplink port name
	agent    *OfnetAgent // Pointer back to ofnet agent that owns this

	//bgp resources
	bgpServer  *gobgp.BgpServer // bgp server instance
	grpcServer *api.Server      // grpc server to talk to gobgp

	myRouterMac   net.HardwareAddr //Router mac used for external proxy
	myBgpPeer     string           // bgp neighbor
	myBgpAs       uint32
	cc            *grpc.ClientConn //grpc client connection
	stopWatch     chan bool
	start         chan bool
	stopArp       chan bool
	intfName      string //loopback intf to run bgp
	oldState      string
	oldAdminState string
}

type OfnetBgpInspect struct {
	Peers []*bgpconf.Neighbor
	Rib   map[string][]*table.Path
}

// Create a new vlrouter instance
func NewOfnetBgp(agent *OfnetAgent, routerInfo []string) *OfnetBgp {
	//Sanity checks
	if agent == nil || agent.datapath == nil {
		log.Errorf("Invilid OfnetAgent")
		return nil
	}
	ofnetBgp := new(OfnetBgp)
	// Keep a reference to the agent
	ofnetBgp.agent = agent

	if len(routerInfo) > 0 {
		ofnetBgp.vlanIntf = routerInfo[0]
	} else {
		log.Errorf("Error creating ofnetBgp. Missing uplink port")
		return nil
	}

	ofnetBgp.bgpServer, ofnetBgp.grpcServer = createBgpServer()

	if ofnetBgp.bgpServer == nil || ofnetBgp.grpcServer == nil {
		log.Errorf("Error instantiating Bgp server")
		return nil
	}
	ofnetBgp.stopWatch = make(chan bool, 1)
	ofnetBgp.intfName = "inb01"
	ofnetBgp.start = make(chan bool, 1)
	ofnetBgp.stopArp = make(chan bool, 1)
	return ofnetBgp
}

/*
Bgp serve routine does the following:
1) Creates inb01 router port
2) Add MyBgp endpoint
3) Kicks off routines to monitor route updates and peer state
*/
func (self *OfnetBgp) StartProtoServer(routerInfo *OfnetProtoRouterInfo) error {

	log.Infof("Starting the Bgp Server with %v", routerInfo)
	//go routine to start gobgp server
	var len uint
	var err error
	self.routerIP, len, err = ParseCIDR(routerInfo.RouterIP)
	as, _ := strconv.Atoi(routerInfo.As)
	self.myBgpAs = uint32(as)

	timeout := grpc.WithTimeout(time.Second)
	conn, err := grpc.Dial("127.0.0.1:50051", timeout, grpc.WithBlock(), grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}
	self.cc = conn
	log.Debugf("Creating the loopback port ")
	err = self.agent.ovsDriver.CreatePort(self.intfName, "internal", 1)
	if err != nil {
		log.Errorf("Error creating the port: %v", err)
	}

	intfIP := fmt.Sprintf("%s/%d", self.routerIP, len)
	log.Debugf("Creating inb01 with ", intfIP)
	ofPortno, _ := self.agent.ovsDriver.GetOfpPortNo(self.intfName)

	link, err := netlink.LinkByName(self.intfName)
	if err != nil {
		log.Errorf("error finding link by name %v", self.intfName)
		return err
	}
	linkIP, err := netlink.ParseAddr(intfIP)
	if err != nil {
		log.Errorf("invalid ip: %s", intfIP)
		return err
	}
	netlink.AddrAdd(link, linkIP)
	netlink.LinkSetUp(link)
	if link == nil || ofPortno == 0 {
		log.Errorf("Error fetching %v/%v/%v information", self.intfName, link, ofPortno)
		return errors.New("Unable to fetch inb01 info")
	}

	intf, _ := net.InterfaceByName(self.intfName)
	vrf := "default"
	epid := self.agent.getEndpointIdByIpVrf(net.ParseIP(self.routerIP), vrf)
	default_vlan := uint16(1)
	_, ok := self.agent.createVrf(vrf)
	if !ok {
		log.Errorf("Error Creating default vrf for Bgp")
		return errors.New("Error creating default vrf")
	}
	self.agent.vlanVrf[default_vlan] = &vrf

	ep := &OfnetEndpoint{
		EndpointID:   epid,
		EndpointType: "internal-bgp",
		IpAddr:       net.ParseIP(self.routerIP),
		IpMask:       net.ParseIP("255.255.255.255"),
		Vrf:          "default",                  // FIXME set VRF correctly
		MacAddrStr:   intf.HardwareAddr.String(), //link.Attrs().HardwareAddr.String(),
		Vlan:         default_vlan,
		PortNo:       ofPortno,
		Timestamp:    time.Now(),
	}

	// Add the endpoint to local routing table

	err = self.agent.datapath.AddLocalEndpoint(*ep)
	if err != nil {
		log.Errorf("Error Adding Local Bgp Endpoint for endpoint %+v,err: %v", ep, err)
		return err
	}
	self.agent.endpointDb.Set(epid, ep)
	self.agent.localEndpointDb.Set(string(ep.PortNo), ep)

	// global configuration
	global := &bgpconf.Global{
		Config: bgpconf.GlobalConfig{
			As:       self.myBgpAs,
			RouterId: self.routerIP,
			Port:     179,
		},
	}

	if err := self.bgpServer.Start(global); err != nil {
		return err
	}

	//monitor route updates from peer, peer state
	go self.watch()
	// register for link ups on uplink and inb01 intf
	self.start <- true
	return nil
}

func (self *OfnetBgp) StopProtoServer() error {

	log.Info("Stopping bgp server")
	err := self.agent.ovsDriver.DeletePort(self.intfName)
	if err != nil {
		return err
	}
	if self.myBgpPeer != "" {
		self.DeleteProtoNeighbor()
	}

	// Delete the endpoint from local routing table
	epreg := self.agent.getEndpointByIpVrf(net.ParseIP(self.routerIP), "default")
	if epreg != nil {
		self.agent.endpointDb.Remove(epreg.EndpointID)
		self.agent.localEndpointDb.Remove(string(epreg.PortNo))
		err := self.agent.datapath.RemoveLocalEndpoint(*epreg)
		if err != nil {
			return err
		}
	}
	self.routerIP = ""
	self.myBgpAs = 0
	self.cc.Close()
	self.agent.deleteVrf("default")

	self.stopWatch <- true
	self.bgpServer.Stop()
	return nil
}

//DeleteProtoNeighbor deletes bgp neighbor for the host
func (self *OfnetBgp) DeleteProtoNeighbor() error {

	/*As a part of delete bgp neighbors
	1) Search for BGP peer and remove from Bgp.
	2) Delete endpoint info for peer
	3) Finally delete all routes learnt on the nexthop bgp port.
	4) Mark the routes learn via json rpc as unresolved
	*/
	log.Infof("Received DeleteProtoNeighbor to delete bgp neighbor %v", self.myBgpPeer)
	n := &bgpconf.Neighbor{
		Config: bgpconf.NeighborConfig{
			NeighborAddress: self.myBgpPeer,
		},
	}
	self.bgpServer.DeleteNeighbor(n)
	self.stopArp <- true
	bgpEndpoint := self.agent.getEndpointByIpVrf(net.ParseIP(self.myBgpPeer), "default")

	self.agent.datapath.RemoveEndpoint(bgpEndpoint)
	self.agent.endpointDb.Remove(bgpEndpoint.EndpointID)
	self.myBgpPeer = ""

	uplink, _ := self.agent.ovsDriver.GetOfpPortNo(self.vlanIntf)
	var ep *OfnetEndpoint
	for endpoint := range self.agent.endpointDb.IterBuffered() {
		ep = endpoint.Val.(*OfnetEndpoint)
		if ep.PortNo == uplink {
			self.agent.datapath.RemoveEndpoint(ep)
			if ep.EndpointType == "internal" {
				ep.PortNo = 0
				self.agent.endpointDb.Set(ep.EndpointID, ep)
				//We readd unresolved endpoints that were learnt via
				//etcd
				self.agent.datapath.AddEndpoint(ep)
			} else if ep.EndpointType == "external" {
				self.agent.endpointDb.Remove(ep.EndpointID)
			}
		}
	}
	return nil
}

//AddProtoNeighbor adds bgp neighbor
func (self *OfnetBgp) AddProtoNeighbor(neighborInfo *OfnetProtoNeighborInfo) error {

	<-self.start
	log.Infof("Received AddProtoNeighbor to add bgp neighbor %v", neighborInfo.NeighborIP)

	peerAs, _ := strconv.Atoi(neighborInfo.As)

	n := &bgpconf.Neighbor{
		Config: bgpconf.NeighborConfig{
			NeighborAddress: neighborInfo.NeighborIP,
			PeerAs:          uint32(peerAs),
		},
		Timers: bgpconf.Timers{
			Config: bgpconf.TimersConfig{
				ConnectRetry: 60,
			},
		},
	}

	err := self.bgpServer.AddNeighbor(n)
	if err != nil {
		return err
	}

	epid := self.agent.getEndpointIdByIpVrf(net.ParseIP(neighborInfo.NeighborIP), "default")
	epreg := &OfnetEndpoint{
		EndpointID:   epid,
		EndpointType: "external-bgp",
		IpAddr:       net.ParseIP(neighborInfo.NeighborIP),
		IpMask:       net.ParseIP("255.255.255.255"),
		Vrf:          "default", // FIXME set VRF correctly
		Vlan:         1,
		Timestamp:    time.Now(),
	}

	// Install the endpoint in datapath
	// First, add the endpoint to local routing table

	err = self.agent.datapath.AddEndpoint(epreg)
	if err != nil {
		log.Errorf("Error adding endpoint: {%+v}. Err: %v", epreg, err)
		return err
	}
	self.agent.endpointDb.Set(epreg.EndpointID, epreg)

	self.myBgpPeer = neighborInfo.NeighborIP
	go self.sendArp(self.stopArp)

	paths := []*OfnetProtoRouteInfo{}
	//Walk through all the localEndpointDb and them to protocol rib
	for endpoint := range self.agent.localEndpointDb.IterBuffered() {
		ep := endpoint.Val.(*OfnetEndpoint)
		path := &OfnetProtoRouteInfo{
			ProtocolType: "bgp",
			localEpIP:    ep.IpAddr.String(),
			nextHopIP:    self.routerIP,
		}
		paths = append(paths, path)
	}
	self.AddLocalProtoRoute(paths)
	return nil
}

//GetRouterInfo returns the configured RouterInfo
func (self *OfnetBgp) GetRouterInfo() *OfnetProtoRouterInfo {
	if self.routerIP == "" {
		return nil
	}
	routerInfo := &OfnetProtoRouterInfo{
		ProtocolType: "bgp",
		RouterIP:     self.routerIP,
		VlanIntf:     self.vlanIntf,
	}
	return routerInfo
}

//AddLocalProtoRoute is used to add local endpoint to the protocol RIB
func (self *OfnetBgp) AddLocalProtoRoute(pathInfo []*OfnetProtoRouteInfo) error {

	if self.routerIP == "" {
		//ignoring populating to the bgp rib because
		//Bgp is not configured.
		return nil
	}
	log.Infof("Received AddLocalProtoRoute to add local endpoint to protocol RIB: %+v", pathInfo)

	// add routes
	attrs := []bgp.PathAttributeInterface{
		bgp.NewPathAttributeOrigin(1),
		bgp.NewPathAttributeNextHop(pathInfo[0].nextHopIP),
		bgp.NewPathAttributeAsPath([]bgp.AsPathParamInterface{bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, []uint32{self.myBgpAs})}),
	}
	paths := []*table.Path{}
	for _, path := range pathInfo {
		paths = append(paths, table.NewPath(nil, bgp.NewIPAddrPrefix(32, path.localEpIP), false, attrs, time.Now(), false))
	}

	_, err := self.bgpServer.AddPath("", paths)
	if err != nil {
		return err
	}

	return nil
}

//DeleteLocalProtoRoute withdraws local endpoints from protocol RIB
func (self *OfnetBgp) DeleteLocalProtoRoute(pathInfo []*OfnetProtoRouteInfo) error {

	log.Infof("Received DeleteLocalProtoRoute to withdraw local endpoint to protocol RIB: %v", pathInfo)

	attrs := []bgp.PathAttributeInterface{
		bgp.NewPathAttributeOrigin(1),
		bgp.NewPathAttributeNextHop(pathInfo[0].nextHopIP),
		bgp.NewPathAttributeAsPath([]bgp.AsPathParamInterface{bgp.NewAs4PathParam(bgp.BGP_ASPATH_ATTR_TYPE_SEQ, []uint32{self.myBgpAs})}),
	}
	paths := []*table.Path{}
	for _, path := range pathInfo {
		paths = append(paths, table.NewPath(nil, bgp.NewIPAddrPrefix(32, path.localEpIP), true, attrs, time.Now(), false))
	}
	if err := self.bgpServer.DeletePath(nil, bgp.RF_IPv4_UC, "", paths); err != nil {
		return err
	}

	return nil
}

//monitorBest monitors for route updates/changes form peer
func (self *OfnetBgp) watch() {
	w := self.bgpServer.Watch(gobgp.WatchBestPath(), gobgp.WatchPeerState(true))
	for {
		select {
		case ev := <-w.Event():
			switch msg := ev.(type) {
			case *gobgp.WatchEventBestPath:
				for _, path := range msg.PathList {
					self.modRib(path)
				}
			case *gobgp.WatchEventPeerState:
				self.peerUpdate(msg)

			}
		case <-self.stopWatch:
			return
		}
	}
}

// monitorPeer is used to monitor the bgp peer state
func (self *OfnetBgp) peerUpdate(s *gobgp.WatchEventPeerState) {

	fmt.Printf("[NEIGH] %s fsm: %s admin: %v\n", s.PeerAddress,
		s.State, s.AdminState.String())
	if self.oldState == "BGP_FSM_ESTABLISHED" && self.oldAdminState == "ADMIN_STATE_UP" {
		uplink, _ := self.agent.ovsDriver.GetOfpPortNo(self.vlanIntf)
		/*If the state changed from being established to idle or active:
		   1) delete all endpoints learnt via bgp Peer
			 2) mark routes pointing to the bgp nexthop as unresolved
			 3) mark the bgp peer reachbility as unresolved
		*/
		endpoint := self.agent.getEndpointByIpVrf(net.ParseIP(self.myBgpPeer), "default")
		self.agent.datapath.RemoveEndpoint(endpoint)
		endpoint.PortNo = 0

		err := self.agent.datapath.AddEndpoint(endpoint)
		if err != nil {
			log.Errorf("Error unresolving bgp peer %s ", self.myBgpPeer)
		}
		self.agent.endpointDb.Set(endpoint.EndpointID, endpoint)

		var ep *OfnetEndpoint
		for endpoint := range self.agent.endpointDb.IterBuffered() {
			ep = endpoint.Val.(*OfnetEndpoint)
			if ep.PortNo == uplink {
				self.agent.datapath.RemoveEndpoint(ep)
				if ep.EndpointType == "internal" {
					ep.PortNo = 0
					self.agent.endpointDb.Set(ep.EndpointID, ep)
					//We readd unresolved endpoints that were learnt via
					//json rpc
					self.agent.datapath.AddEndpoint(ep)
				} else if ep.EndpointType == "external" {
					self.agent.endpointDb.Remove(ep.EndpointID)
				}
			}
		}
	}
	self.oldState = s.State.String()
	self.oldAdminState = s.AdminState.String()

	return
}

//modrib receives route updates from BGP server and adds the endpoint
func (self *OfnetBgp) modRib(path *table.Path) error {
	var nlri bgp.AddrPrefixInterface
	var nextHop string
	var macAddrStr string
	var portNo uint32

	nlri = path.GetNlri()
	nextHop = path.GetNexthop().String()

	if nextHop == "0.0.0.0" {
		return nil
	}

	if nlri == nil {
		return fmt.Errorf("no nlri")
	}

	endpointIPNet, _ := netlink.ParseIPNet(nlri.String())
	log.Infof("Bgp Rib Received endpoint update for path %s", path.String())

	//check if bgp published a route local to the host
	epid := self.agent.getEndpointIdByIpVrf(endpointIPNet.IP.Mask(endpointIPNet.Mask), "default")

	//Check if the route is local
	if nextHop == self.routerIP {
		log.Debugf("This is a local route skipping endpoint create! ")
		return nil
	} else if ep := self.agent.getEndpointByID(epid); ep != nil {
		if ep.EndpointType != "external" {
			log.Debugf("Endpoint was learnt via internal protocol. skipping update! ")
			return nil
		}
	}

	nhEpid := self.agent.getEndpointIdByIpVrf(net.ParseIP(nextHop), "default")

	if ep := self.agent.getEndpointByID(nhEpid); ep == nil {
		//the nexthop is not the directly connected eBgp peer
		macAddrStr = ""
		portNo = 0
	} else {
		macAddrStr = ep.MacAddrStr
		portNo = ep.PortNo
	}

	ipmask := net.ParseIP("255.255.255.255").Mask(endpointIPNet.Mask)

	if path.IsWithdraw != true {
		epreg := &OfnetEndpoint{
			EndpointID:   epid,
			EndpointType: "external",
			IpAddr:       endpointIPNet.IP,
			IpMask:       ipmask,
			Vrf:          "default", // FIXME set VRF correctly
			MacAddrStr:   macAddrStr,
			Vlan:         1,
			OriginatorIp: self.agent.localIp,
			PortNo:       portNo,
			Timestamp:    time.Now(),
		}

		// Install the endpoint in datapath
		// First, add the endpoint to local routing table

		self.agent.endpointDb.Set(epreg.EndpointID, epreg)
		err := self.agent.datapath.AddEndpoint(epreg)
		if err != nil {
			log.Errorf("Error adding endpoint: {%+v}. Err: %v", epreg, err)
			return err
		}
	} else {
		log.Info("Received route withdraw from BGP for ", endpointIPNet)
		endpoint := self.agent.getEndpointByIpVrf(endpointIPNet.IP, "default")
		if endpoint != nil {
			self.agent.datapath.RemoveEndpoint(endpoint)
			self.agent.endpointDb.Remove(endpoint.EndpointID)
		}
	}
	return nil
}

//createBgpServer creates and starts a bgp server and correspoinding grpc server
func createBgpServer() (bgpServer *gobgp.BgpServer, grpcServer *api.Server) {
	bgpServer = gobgp.NewBgpServer()
	if bgpServer == nil {
		log.Errorf("Error creating bgp server")
		return
	} else {
		go bgpServer.Serve()
	}
	// start grpc Server
	grpcServer = api.NewGrpcServer(bgpServer, ":50051")
	if grpcServer == nil {
		log.Errorf("Error creating bgp server")
		return
	} else {
		go grpcServer.Serve()
	}
	return
}

func (self *OfnetBgp) sendArp(stopArp chan bool) {

	//Get the Mac of the vlan intf
	//Get the portno of the uplink
	//Build an arp packet and send on portno of uplink
	time.Sleep(2 * time.Second)
	self.sendArpPacketOut()

	for {
		select {
		case <-stopArp:
			return
		case <-time.After(1800 * time.Second):
			self.sendArpPacketOut()
		}
	}
}

func (self *OfnetBgp) ModifyProtoRib(path interface{}) {
	self.modRib(path.(*table.Path))
}

func (self *OfnetBgp) sendArpPacketOut() {
	if self.myBgpPeer == "" {
		return
	}
	intf, _ := net.InterfaceByName(self.vlanIntf)
	ofPortno, _ := self.agent.ovsDriver.GetOfpPortNo(self.vlanIntf)
	bMac, _ := net.ParseMAC("FF:FF:FF:FF:FF:FF")
	zeroMac, _ := net.ParseMAC("00:00:00:00:00:00")

	srcIP := net.ParseIP(self.routerIP)
	dstIP := net.ParseIP(self.myBgpPeer)
	arpReq, _ := protocol.NewARP(protocol.Type_Request)
	arpReq.HWSrc = intf.HardwareAddr
	arpReq.IPSrc = srcIP
	arpReq.HWDst = zeroMac
	arpReq.IPDst = dstIP

	log.Debugf("Sending ARP Request: %+v", arpReq)

	// build the ethernet packet
	ethPkt := protocol.NewEthernet()
	ethPkt.HWDst = bMac
	ethPkt.HWSrc = arpReq.HWSrc
	ethPkt.Ethertype = 0x0806
	ethPkt.Data = arpReq

	log.Debugf("Sending ARP Request Ethernet: %+v", ethPkt)

	// Packet out
	pktOut := openflow13.NewPacketOut()
	pktOut.Data = ethPkt
	pktOut.AddAction(openflow13.NewActionOutput(ofPortno))

	log.Debugf("Sending ARP Request packet: %+v", pktOut)

	// Send it out
	self.agent.ofSwitch.Send(pktOut)
}

func (self *OfnetBgp) InspectProto() (interface{}, error) {
	OfnetBgpInspect := new(OfnetBgpInspect)
	var err error

	if self.bgpServer == nil {
		return nil, nil
	}
	// Get Bgp info
	OfnetBgpInspect.Peers = self.bgpServer.GetNeighbor()

	if OfnetBgpInspect.Peers == nil {
		return nil, nil
	}

	// Get rib info
	_, OfnetBgpInspect.Rib, err = self.bgpServer.GetRib(self.myBgpPeer, bgp.RF_IPv4_UC, nil)
	if err != nil {
		log.Errorf("Bgp Inspect failed: %v", err)
		return nil, err
	}

	return OfnetBgpInspect, nil
}
