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
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	api "github.com/osrg/gobgp/api"
	bgpconf "github.com/osrg/gobgp/config"
	"github.com/osrg/gobgp/packet"
	bgpserver "github.com/osrg/gobgp/server"
	"github.com/shaleman/libOpenflow/openflow13"
	"github.com/shaleman/libOpenflow/protocol"
	"github.com/vishvananda/netlink"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type OfnetBgp struct {
	sync.Mutex
	routerIP string      // virtual interface ip for bgp
	vlanIntf string      // uplink port name
	agent    *OfnetAgent // Pointer back to ofnet agent that owns this

	//bgp resources
	modRibCh   chan *api.Path //channel for route change notif
	advPathCh  chan *api.Path
	bgpServer  *bgpserver.BgpServer // bgp server instance
	grpcServer *bgpserver.Server    // grpc server to talk to gobgp

	myRouterMac net.HardwareAddr //Router mac used for external proxy
	myBgpPeer   string           // bgp neighbor
	myBgpAs     uint32
	cc          *grpc.ClientConn //grpc client connection
	stop        chan bool
	start       chan bool
	intfName    string //loopback intf to run bgp
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
	ofnetBgp.stop = make(chan bool, 1)
	ofnetBgp.intfName = "inb01"
	ofnetBgp.start = make(chan bool, 1)
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

	self.modRibCh = make(chan *api.Path, 16)
	self.advPathCh = make(chan *api.Path, 16)

	timeout := grpc.WithTimeout(time.Second)
	conn, err := grpc.Dial("127.0.0.1:8080", timeout, grpc.WithBlock(), grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}
	self.cc = conn
	defer self.cc.Close()

	client := api.NewGobgpApiClient(self.cc)
	if client == nil {
		log.Errorf("Invalid Gobgpapi client")
		return errors.New("Error creating Gobgpapiclient")
	}
	path := &api.Path{
		Pattrs: make([][]byte, 0),
	}

	path.Nlri, _ = bgp.NewIPAddrPrefix(uint8(32), self.routerIP).Serialize()
	n, _ := bgp.NewPathAttributeNextHop("0.0.0.0").Serialize()
	path.Pattrs = append(path.Pattrs, n)
	origin, _ := bgp.NewPathAttributeOrigin(bgp.BGP_ORIGIN_ATTR_TYPE_INCOMPLETE).Serialize()
	path.Pattrs = append(path.Pattrs, origin)

	log.Debugf("Creating the loopback port ")
	err = self.agent.ovsDriver.CreatePort(self.intfName, "internal", 1)
	if err != nil {
		log.Errorf("Error creating the port", err)
	}
	defer self.agent.ovsDriver.DeletePort(self.intfName)

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
		log.Errorf("invalid ip ", intfIP)
                return err
	}
	netlink.AddrAdd(link, linkIP)
        netlink.LinkSetUp(link)
	if link == nil || ofPortno == 0 {
		log.Errorf("Error fetching %v information", self.intfName, link, ofPortno)
		return errors.New("Unable to fetch inb01 info")
	}

	intf, _ := net.InterfaceByName(self.intfName)

	epreg := &OfnetEndpoint{
		EndpointID:   self.routerIP,
		EndpointType: "internal-bgp",
		IpAddr:       net.ParseIP(self.routerIP),
		IpMask:       net.ParseIP("255.255.255.255"),
		VrfId:        0,                          // FIXME set VRF correctly
		MacAddrStr:   intf.HardwareAddr.String(), //link.Attrs().HardwareAddr.String(),
		Vlan:         1,
		PortNo:       ofPortno,
		Timestamp:    time.Now(),
	}

	// Add the endpoint to local routing table
	self.agent.endpointDb[self.routerIP] = epreg
	self.agent.localEndpointDb[epreg.PortNo] = epreg
	fmt.Println(epreg)
	err = self.agent.datapath.AddLocalEndpoint(*epreg)

	//Add bgp router id as well
	bgpGlobalCfg := &bgpconf.Global{}
	setDefaultGlobalConfigValues(bgpGlobalCfg)
	bgpGlobalCfg.GlobalConfig.RouterId = net.ParseIP(self.routerIP)
	bgpGlobalCfg.GlobalConfig.As = self.myBgpAs
	self.bgpServer.SetGlobalType(*bgpGlobalCfg)

	self.advPathCh <- path

	//monitor route updates from peer
	go self.monitorBest()
	//monitor peer state
	go self.monitorPeer()
	self.start <- true
	for {
		select {
		case p := <-self.modRibCh:
			err = self.modRib(p)
			if err != nil {
				log.Error("failed to mod rib: ", err)
			}
		case <-self.stop:
			return nil
		}
	}
	return nil
}
func (self *OfnetBgp) StopProtoServer() error {

	log.Info("Received stop bgp server")
	err := self.agent.ovsDriver.DeletePort(self.intfName)
	if err != nil {
		log.Errorf("Error deleting the port", err)
		return err
	}

	// Delete the endpoint from local routing table
	epreg := self.agent.getEndpointByIp(net.ParseIP(self.routerIP))
	if epreg != nil {
		delete(self.agent.endpointDb, self.routerIP)
		delete(self.agent.localEndpointDb, epreg.PortNo)
		err = self.agent.datapath.RemoveLocalEndpoint(*epreg)
	}
	self.routerIP = ""
	self.myBgpAs = 0
	self.stop <- true
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
	client := api.NewGobgpApiClient(self.cc)
	if client == nil {
		log.Errorf("Invalid Gobgpapi client")
		return errors.New("Error creating Gobgpapiclient")
	}
	arg := &api.Arguments{Name: self.myBgpPeer}

	peer, err := client.GetNeighbor(context.Background(), arg)
	if err != nil {
		log.Errorf("GetNeighbor failed ", err)
		return err
	}
	log.Infof("Deleteing Bgp peer from Bgp server")
	p := bgpconf.Neighbor{}
	setNeighborConfigValues(&p)

	p.NeighborAddress = net.ParseIP(peer.Conf.NeighborAddress)
	p.NeighborConfig.NeighborAddress = net.ParseIP(peer.Conf.NeighborAddress)
	p.NeighborConfig.PeerAs = uint32(peer.Conf.PeerAs)
	//FIX ME set ipv6 depending on peerip (for v6 BGP)
	p.AfiSafis.AfiSafiList = []bgpconf.AfiSafi{
		bgpconf.AfiSafi{AfiSafiName: "ipv4-unicast"}}
	self.bgpServer.SetBmpConfig(bgpconf.BmpServers{
		BmpServerList: []bgpconf.BmpServer{},
	})

	self.bgpServer.PeerDelete(p)

	bgpEndpoint := self.agent.getEndpointByIp(net.ParseIP(self.myBgpPeer))
	self.agent.datapath.RemoveEndpoint(bgpEndpoint)
	delete(self.agent.endpointDb, self.myBgpPeer)
	self.myBgpPeer = ""

	uplink, _ := self.agent.ovsDriver.GetOfpPortNo(self.vlanIntf)

	for _, endpoint := range self.agent.endpointDb {
		if endpoint.PortNo == uplink {
			self.agent.datapath.RemoveEndpoint(endpoint)
			if endpoint.EndpointType == "internal" {
				endpoint.PortNo = 0
				self.agent.endpointDb[endpoint.EndpointID] = endpoint
				//We readd unresolved endpoints that were learnt via
				//etcd
				self.agent.datapath.AddEndpoint(endpoint)
			} else if endpoint.EndpointType == "external" {
				delete(self.agent.endpointDb, endpoint.EndpointID)
			}
		}
	}
	return nil
}

//AddProtoNeighbor adds bgp neighbor
func (self *OfnetBgp) AddProtoNeighbor(neighborInfo *OfnetProtoNeighborInfo) error {

	<-self.start

	log.Infof("Received AddProtoNeighbor to Add bgp neighbor %v", neighborInfo.NeighborIP)

	var policyConfig bgpconf.RoutingPolicy

	peerAs, _ := strconv.Atoi(neighborInfo.As)
	p := &bgpconf.Neighbor{}
	setNeighborConfigValues(p)
	p.NeighborAddress = net.ParseIP(neighborInfo.NeighborIP)
	p.NeighborConfig.NeighborAddress = net.ParseIP(neighborInfo.NeighborIP)
	p.NeighborConfig.PeerAs = uint32(peerAs)
	//FIX ME set ipv6 depending on peerip (for v6 BGP)
	p.AfiSafis.AfiSafiList = []bgpconf.AfiSafi{
		bgpconf.AfiSafi{AfiSafiName: "ipv4-unicast"}}
	self.bgpServer.SetBmpConfig(bgpconf.BmpServers{
		BmpServerList: []bgpconf.BmpServer{},
	})

	self.bgpServer.PeerAdd(*p)
	self.bgpServer.SetPolicy(policyConfig)

	log.Infof("Peer %v is added", p.NeighborConfig.NeighborAddress)

	epreg := &OfnetEndpoint{
		EndpointID:   neighborInfo.NeighborIP,
		EndpointType: "external-bgp",
		IpAddr:       net.ParseIP(neighborInfo.NeighborIP),
		IpMask:       net.ParseIP("255.255.255.255"),
		VrfId:        0, // FIXME set VRF correctly
		Vlan:         1,
		Timestamp:    time.Now(),
	}

	// Install the endpoint in datapath
	// First, add the endpoint to local routing table
	self.agent.endpointDb[epreg.EndpointID] = epreg
	err := self.agent.datapath.AddEndpoint(epreg)

	if err != nil {
		log.Errorf("Error adding endpoint: {%+v}. Err: %v", epreg, err)
		return err
	}
	self.myBgpPeer = neighborInfo.NeighborIP
	go self.sendArp()

	//Walk through all the localEndpointDb and them to protocol rib
	for _, endpoint := range self.agent.localEndpointDb {
		path := &OfnetProtoRouteInfo{
			ProtocolType: "bgp",
			localEpIP:    endpoint.IpAddr.String(),
			nextHopIP:    self.routerIP,
		}
		self.AddLocalProtoRoute(path)
	}
	return nil
}

//GetRouterInfo returns the configured RouterInfo
func (self *OfnetBgp) GetRouterInfo() *OfnetProtoRouterInfo {
	routerInfo := &OfnetProtoRouterInfo{
		ProtocolType: "bgp",
		RouterIP:     self.routerIP,
		VlanIntf:     self.vlanIntf,
	}
	return routerInfo
}

//AddLocalProtoRoute is used to add local endpoint to the protocol RIB
func (self *OfnetBgp) AddLocalProtoRoute(pathInfo *OfnetProtoRouteInfo) error {

	if self.routerIP == "" {
		//ignoring populating to the bgp rib because
		//Bgp is not configured.
		return nil
	}

	log.Infof("Received AddLocalProtoRoute to add local endpoint to protocol RIB: %v", pathInfo)

	path := &api.Path{
		Pattrs: make([][]byte, 0),
	}

	// form the path structure with appropriate path attributes
	nlri := bgp.NewIPAddrPrefix(32, pathInfo.localEpIP)
	path.Nlri, _ = nlri.Serialize()
	origin, _ := bgp.NewPathAttributeOrigin(bgp.BGP_ORIGIN_ATTR_TYPE_EGP).Serialize()
	path.Pattrs = append(path.Pattrs, origin)
	aspathParam := []bgp.AsPathParamInterface{bgp.NewAs4PathParam(2, []uint32{self.myBgpAs})}
	aspath, _ := bgp.NewPathAttributeAsPath(aspathParam).Serialize()
	path.Pattrs = append(path.Pattrs, aspath)
	n, _ := bgp.NewPathAttributeNextHop(pathInfo.nextHopIP).Serialize()
	path.Pattrs = append(path.Pattrs, n)

	name := ""

	arg := &api.ModPathArguments{
		Resource: api.Resource_GLOBAL,
		Name:     name,
		Paths:    []*api.Path{path},
	}

	//send arguement stream
	client := api.NewGobgpApiClient(self.cc)
	if client == nil {
		log.Errorf("Gobgpapi stream invalid")
		return nil
	}

	stream, err := client.ModPath(context.Background())
	if err != nil {
		log.Errorf("Fail to enforce Modpath", err)
		return err
	}
	err = stream.Send(arg)
	if err != nil {
		log.Errorf("Failed to send strean", err)
		return err
	}
	stream.CloseSend()
	res, e := stream.CloseAndRecv()
	if e != nil {
		log.Errorf("Falied toclose stream ")
		return e
	}
	if res.Code != api.Error_SUCCESS {
		return fmt.Errorf("error: code: %d, msg: %s", res.Code, res.Msg)
	}
	return nil
}

//DeleteLocalProtoRoute withdraws local endpoints from protocol RIB
func (self *OfnetBgp) DeleteLocalProtoRoute(pathInfo *OfnetProtoRouteInfo) error {

	log.Infof("Received DeleteLocalProtoRoute to withdraw local endpoint to protocol RIB: %v", pathInfo)

	path := &api.Path{
		Pattrs: make([][]byte, 0),
	}

	//form appropraite path attributes for path to be withdrawn
	nlri := bgp.NewIPAddrPrefix(32, pathInfo.localEpIP)
	path.Nlri, _ = nlri.Serialize()
	origin, _ := bgp.NewPathAttributeOrigin(bgp.BGP_ORIGIN_ATTR_TYPE_EGP).Serialize()
	path.Pattrs = append(path.Pattrs, origin)
	aspathParam := []bgp.AsPathParamInterface{bgp.NewAs4PathParam(2, []uint32{self.myBgpAs})}
	aspath, _ := bgp.NewPathAttributeAsPath(aspathParam).Serialize()
	path.Pattrs = append(path.Pattrs, aspath)
	n, _ := bgp.NewPathAttributeNextHop(pathInfo.nextHopIP).Serialize()
	path.Pattrs = append(path.Pattrs, n)
	path.IsWithdraw = true
	name := ""

	arg := &api.ModPathArguments{
		Resource: api.Resource_GLOBAL,
		Name:     name,
		Paths:    []*api.Path{path},
	}

	//send arguement stream
	client := api.NewGobgpApiClient(self.cc)
	if client == nil {
		log.Errorf("Gobgpapi stream invalid")
		return nil
	}

	stream, err := client.ModPath(context.Background())
	if err != nil {
		log.Errorf("Fail to enforce Modpathi", err)
		return err
	}

	err = stream.Send(arg)
	if err != nil {
		log.Errorf("Failed to send strean", err)
		return err
	}
	stream.CloseSend()

	res, e := stream.CloseAndRecv()
	if e != nil {
		log.Errorf("Falied toclose stream ")
		return e
	}

	if res.Code != api.Error_SUCCESS {
		return fmt.Errorf("error: code: %d, msg: %s", res.Code, res.Msg)
	}
	return nil
}

//monitorBest monitors for route updates/changes form peer
func (self *OfnetBgp) monitorBest() {

	client := api.NewGobgpApiClient(self.cc)
	if client == nil {
		log.Errorf("Invalid Gobgpapi client")
		return
	}
	arg := &api.Arguments{
		Resource: api.Resource_GLOBAL,
		Rf:       uint32(bgp.RF_IPv4_UC),
	}

	stream, err := client.MonitorBestChanged(context.Background(), arg)
	if err != nil {
		return
	}

	for {
		dst, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Infof("monitorBest stream ended")
			return
		}
		self.modRibCh <- dst.Paths[0]
	}
	return
}

// monitorPeer is used to monitor the bgp peer state
func (self *OfnetBgp) monitorPeer() {

	var oldAdminState, oldState string

	client := api.NewGobgpApiClient(self.cc)
	if client == nil {
		log.Errorf("Invalid Gobgpapi client")
		return
	}
	arg := &api.Arguments{}

	stream, err := client.MonitorPeerState(context.Background(), arg)
	if err != nil {
		log.Errorf("MonitorPeerState failed ", err)
		return
	}
	for {
		s, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Warnf("MonitorPeerState stream ended :")
			break
		}
		fmt.Printf("[NEIGH] %s fsm: %s admin: %s\n", s.Conf.NeighborAddress,
			s.Info.BgpState, s.Info.AdminState)
		if oldState == "BGP_FSM_ESTABLISHED" && oldAdminState == "ADMIN_STATE_UP" {
			uplink, _ := self.agent.ovsDriver.GetOfpPortNo(self.vlanIntf)
			/*If the state changed from being established to idle or active:
			   1) delete all endpoints learnt via bgp Peer
				 2) mark routes pointing to the bgp nexthop as unresolved
				 3) mark the bgp peer reachbility as unresolved
			*/
			endpoint := self.agent.getEndpointByIp(net.ParseIP(self.myBgpPeer))
			self.agent.datapath.RemoveEndpoint(endpoint)
			endpoint.PortNo = 0
			self.agent.endpointDb[endpoint.EndpointID] = endpoint
			self.agent.datapath.AddEndpoint(endpoint)

			for _, endpoint = range self.agent.endpointDb {
				if endpoint.PortNo == uplink {
					self.agent.datapath.RemoveEndpoint(endpoint)
					if endpoint.EndpointType == "internal" {
						endpoint.PortNo = 0
						self.agent.endpointDb[endpoint.EndpointID] = endpoint
						//We readd unresolved endpoints that were learnt via
						//json rpc
						self.agent.datapath.AddEndpoint(endpoint)
					} else if endpoint.EndpointType == "external" {
						delete(self.agent.endpointDb, endpoint.EndpointID)
					}
				}
			}
		}
		oldState = s.Info.BgpState
		oldAdminState = s.Info.AdminState
	}
	return
}

//modrib receives route updates from BGP server and adds the endpoint
func (self *OfnetBgp) modRib(path *api.Path) error {
	var nlri bgp.AddrPrefixInterface
	var nextHop string
	var macAddrStr string
	var portNo uint32
	if len(path.Nlri) > 0 {
		nlri = &bgp.IPAddrPrefix{}
		err := nlri.DecodeFromBytes(path.Nlri)
		if err != nil {
			return err
		}
	}

	for _, attr := range path.Pattrs {
		p, err := bgp.GetPathAttribute(attr)
		if err != nil {
			return err
		}

		err = p.DecodeFromBytes(attr)
		if err != nil {
			return err
		}

		if p.GetType() == bgp.BGP_ATTR_TYPE_NEXT_HOP {
			nextHop = p.(*bgp.PathAttributeNextHop).Value.String()
			break
		}
	}
	if nextHop == "0.0.0.0" {
		return nil
	}

	if nlri == nil {
		return fmt.Errorf("no nlri")
	}

	endpointIPNet, _ := netlink.ParseIPNet(nlri.String())
	log.Infof("Bgp Rib Received endpoint update for %v , with nexthop %v",
		endpointIPNet, nextHop)

	//check if bgp published a route local to the host
	epid := endpointIPNet.IP.Mask(endpointIPNet.Mask).String()

	//Check if the route is local
	if nextHop == self.routerIP {
		log.Info("This is a local route skipping endpoint create! ")
		return nil
	} else if self.agent.endpointDb[epid] != nil {
		if self.agent.endpointDb[epid].EndpointType != "external" {
			log.Info("Endpoint was learnt via internal protocol. skipping update! ")
			return nil
		}
	}

	if self.agent.endpointDb[nextHop] == nil {
		//the nexthop is not the directly connected eBgp peer
		macAddrStr = ""
		portNo = 0
	} else {
		macAddrStr = self.agent.endpointDb[nextHop].MacAddrStr
		portNo = self.agent.endpointDb[nextHop].PortNo
	}

	ipmask := net.ParseIP("255.255.255.255").Mask(endpointIPNet.Mask)

	if path.IsWithdraw != true {
		epreg := &OfnetEndpoint{
			EndpointID:   epid,
			EndpointType: "external",
			IpAddr:       endpointIPNet.IP,
			IpMask:       ipmask,
			VrfId:        0, // FIXME set VRF correctly
			MacAddrStr:   macAddrStr,
			Vlan:         1,
			OriginatorIp: self.agent.localIp,
			PortNo:       portNo,
			Timestamp:    time.Now(),
		}

		// Install the endpoint in datapath
		// First, add the endpoint to local routing table
		self.agent.endpointDb[epreg.EndpointID] = epreg
		err := self.agent.datapath.AddEndpoint(epreg)
		if err != nil {
			log.Errorf("Error adding endpoint: {%+v}. Err: %v", epreg, err)
			return err
		}
	} else {
		log.Info("Received route withdraw from BGP for ", endpointIPNet)
		endpoint := self.agent.getEndpointByIp(endpointIPNet.IP)
		if endpoint != nil {
			self.agent.datapath.RemoveEndpoint(endpoint)
			delete(self.agent.endpointDb, endpoint.EndpointID)
		}
	}
	return nil
}

//createBgpServer creates and starts a bgp server and correspoinding grpc server
func createBgpServer() (bgpServer *bgpserver.BgpServer, grpcServer *bgpserver.Server) {
	bgpServer = bgpserver.NewBgpServer(bgp.BGP_PORT)
	if bgpServer == nil {
		log.Errorf("Error creating bgp server")
		return
	} else {
		go bgpServer.Serve()
	}
	// start grpc Server
	grpcServer = bgpserver.NewGrpcServer(bgpserver.GRPC_PORT, bgpServer.GrpcReqCh)
	if grpcServer == nil {
		log.Errorf("Error creating bgp server")
		return
	} else {
		go grpcServer.Serve()
	}
	return
}

//setDefaultGlobalConfigValues sets the default global configs for bgp
func setDefaultGlobalConfigValues(bt *bgpconf.Global) error {

	bt.AfiSafis.AfiSafiList = []bgpconf.AfiSafi{
		bgpconf.AfiSafi{AfiSafiName: "ipv4-unicast"},
		bgpconf.AfiSafi{AfiSafiName: "ipv6-unicast"},
		bgpconf.AfiSafi{AfiSafiName: "l3vpn-ipv4-unicast"},
		bgpconf.AfiSafi{AfiSafiName: "l3vpn-ipv6-unicast"},
		bgpconf.AfiSafi{AfiSafiName: "l2vpn-evpn"},
		bgpconf.AfiSafi{AfiSafiName: "encap"},
		bgpconf.AfiSafi{AfiSafiName: "rtc"},
		bgpconf.AfiSafi{AfiSafiName: "ipv4-flowspec"},
		bgpconf.AfiSafi{AfiSafiName: "l3vpn-ipv4-flowspec"},
		bgpconf.AfiSafi{AfiSafiName: "ipv6-flowspec"},
		bgpconf.AfiSafi{AfiSafiName: "l3vpn-ipv6-flowspec"},
	}
	bt.MplsLabelRange.MinLabel = bgpconf.DEFAULT_MPLS_LABEL_MIN
	bt.MplsLabelRange.MaxLabel = bgpconf.DEFAULT_MPLS_LABEL_MAX

	return nil
}

//setNeighborConfigValues sets the default neighbor configs for bgp
func setNeighborConfigValues(neighbor *bgpconf.Neighbor) error {

	neighbor.Timers.TimersConfig.ConnectRetry = float64(bgpconf.DEFAULT_CONNECT_RETRY)
	neighbor.Timers.TimersConfig.HoldTime = float64(bgpconf.DEFAULT_HOLDTIME)
	neighbor.Timers.TimersConfig.KeepaliveInterval = float64(bgpconf.DEFAULT_HOLDTIME / 3)
	neighbor.Timers.TimersConfig.IdleHoldTimeAfterReset =
		float64(bgpconf.DEFAULT_IDLE_HOLDTIME_AFTER_RESET)
	//FIX ME need to check with global peer to set internal or external
	neighbor.NeighborConfig.PeerType = bgpconf.PEER_TYPE_EXTERNAL
	neighbor.Transport.TransportConfig.PassiveMode = false
	return nil
}

func (self *OfnetBgp) sendArp() {

	//Get the Mac of the vlan intf
	//Get the portno of the uplink
	//Build an arp packet and send on portno of uplink
	time.Sleep(5 * time.Second)
	for {
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

		log.Infof("Sending ARP Request: %+v", arpReq)

		// build the ethernet packet
		ethPkt := protocol.NewEthernet()
		ethPkt.HWDst = bMac
		ethPkt.HWSrc = arpReq.HWSrc
		ethPkt.Ethertype = 0x0806
		ethPkt.Data = arpReq

		log.Infof("Sending ARP Request Ethernet: %+v", ethPkt)

		// Packet out
		pktOut := openflow13.NewPacketOut()
		pktOut.Data = ethPkt
		pktOut.AddAction(openflow13.NewActionOutput(ofPortno))

		log.Infof("Sending ARP Request packet: %+v", pktOut)

		// Send it out
		self.agent.ofSwitch.Send(pktOut)
		time.Sleep(1800 * time.Second)
	}
}

func (self *OfnetBgp) ModifyProtoRib(path interface{}) {
	self.modRibCh <- path.(*api.Path)
}
