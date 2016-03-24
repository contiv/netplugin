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

// VXLAN tables are structured as follows
//
// +-------+										 +-------------------+
// | Valid +---------------------------------------->| ARP to Controller |
// | Pkts  +-->+-------+                             +-------------------+
// +-------+   | Vlan  |        +---------+
//             | Table +------->| IP Dst  |          +--------------+
//             +-------+        | Lookup  +--------->| Ucast Output |
//                              +----------          +--------------+
//
//

import (
	//"fmt"
	"errors"
	"net"
	"net/rpc"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/ofnet/ofctrl"
	"github.com/shaleman/libOpenflow/openflow13"
	"github.com/shaleman/libOpenflow/protocol"
)

// Vlrouter state.
// One Vlrouter instance exists on each host
type Vlrouter struct {
	agent       *OfnetAgent      // Pointer back to ofnet agent that owns this
	ofSwitch    *ofctrl.OFSwitch // openflow switch we are talking to
	policyAgent *PolicyAgent     // Policy agent

	// Fgraph tables
	inputTable *ofctrl.Table // Packet lookup starts here
	vlanTable  *ofctrl.Table // Vlan Table. map port or VNI to vlan
	ipTable    *ofctrl.Table // IP lookup table

	// Flow Database
	flowDb         map[string]*ofctrl.Flow // Database of flow entries
	portVlanFlowDb map[uint32]*ofctrl.Flow // Database of flow entries

	myRouterMac   net.HardwareAddr  //Router mac used for external proxy
	myBgpPeer     string            // bgp neighbor
	unresolvedEPs map[string]string // unresolved endpoint map
}

// Create a new vlrouter instance
func NewVlrouter(agent *OfnetAgent, rpcServ *rpc.Server) *Vlrouter {
	vlrouter := new(Vlrouter)

	// Keep a reference to the agent
	vlrouter.agent = agent

	// Create policy agent
	vlrouter.policyAgent = NewPolicyAgent(agent, rpcServ)

	// Create a flow dbs and my router mac
	vlrouter.flowDb = make(map[string]*ofctrl.Flow)
	vlrouter.portVlanFlowDb = make(map[uint32]*ofctrl.Flow)
	vlrouter.myRouterMac, _ = net.ParseMAC("00:00:11:11:11:11")
	vlrouter.unresolvedEPs = make(map[string]string)

	return vlrouter
}

// Handle new master added event
func (self *Vlrouter) MasterAdded(master *OfnetNode) error {

	return nil
}

// Handle switch connected notification
func (self *Vlrouter) SwitchConnected(sw *ofctrl.OFSwitch) {
	// Keep a reference to the switch
	self.ofSwitch = sw

	log.Infof("Switch connected(vlrouter). installing flows")

	// Tell the policy agent about the switch
	self.policyAgent.SwitchConnected(sw)

	// Init the Fgraph
	self.initFgraph()
}

// Handle switch disconnected notification
func (self *Vlrouter) SwitchDisconnected(sw *ofctrl.OFSwitch) {
	// FIXME
}

// Handle incoming packet
func (self *Vlrouter) PacketRcvd(sw *ofctrl.OFSwitch, pkt *ofctrl.PacketIn) {
	switch pkt.Data.Ethertype {
	case 0x0806:
		if (pkt.Match.Type == openflow13.MatchType_OXM) &&
			(pkt.Match.Fields[0].Class == openflow13.OXM_CLASS_OPENFLOW_BASIC) &&
			(pkt.Match.Fields[0].Field == openflow13.OXM_FIELD_IN_PORT) {
			// Get the input port number
			switch t := pkt.Match.Fields[0].Value.(type) {
			case *openflow13.InPortField:
				var inPortFld openflow13.InPortField
				inPortFld = *t

				self.processArp(pkt.Data, inPortFld.InPort)
			}
		}

	case 0x0800:
		// FIXME: We dont expect IP packets. Use this for statefull policies.
	default:
		log.Errorf("Received unknown ethertype: %x", pkt.Data.Ethertype)
	}
}

/*AddLocalEndpoint does the following:
1) Adds endpoint to the OVS and the associated flows
2) Populates BGP RIB with local route to be propogated to neighbor
*/

func (self *Vlrouter) AddLocalEndpoint(endpoint OfnetEndpoint) error {
	// Install a flow entry for vlan mapping and point it to IP table
	if self.agent.ctrler == nil {
		return nil
	}

	portVlanFlow, err := self.vlanTable.NewFlow(ofctrl.FlowMatch{
		Priority:  FLOW_MATCH_PRIORITY,
		InputPort: endpoint.PortNo,
	})
	if err != nil {
		log.Errorf("Error creating portvlan entry. Err: %v", err)
		return err
	}

	// Set source endpoint group if specified
	if endpoint.EndpointGroup != 0 {
		metadata, metadataMask := SrcGroupMetadata(endpoint.EndpointGroup)
		portVlanFlow.SetMetadata(metadata, metadataMask)
	}

	// Point it to dst group table for policy lookups
	dstGrpTbl := self.ofSwitch.GetTable(DST_GRP_TBL_ID)
	err = portVlanFlow.Next(dstGrpTbl)
	if err != nil {
		log.Errorf("Error installing portvlan entry. Err: %v", err)
		return err
	}

	// save the flow entry
	self.portVlanFlowDb[endpoint.PortNo] = portVlanFlow

	outPort, err := self.ofSwitch.OutputPort(endpoint.PortNo)
	if err != nil {
		log.Errorf("Error creating output port %d. Err: %v", endpoint.PortNo, err)
		return err
	}

	// Install the IP address
	ipFlow, err := self.ipTable.NewFlow(ofctrl.FlowMatch{
		Priority:  FLOW_MATCH_PRIORITY,
		Ethertype: 0x0800,
		IpDa:      &endpoint.IpAddr,
	})

	if err != nil {
		log.Errorf("Error creating flow for endpoint: %+v. Err: %v", endpoint, err)
		return err
	}

	destMacAddr, _ := net.ParseMAC(endpoint.MacAddrStr)

	// Set Mac addresses
	ipFlow.SetMacDa(destMacAddr)
	ipFlow.SetMacSa(self.myRouterMac)

	// Point the route at output port
	err = ipFlow.Next(outPort)
	if err != nil {
		log.Errorf("Error installing flow for endpoint: %+v. Err: %v", endpoint, err)
		return err
	}

	// Store the flow
	self.flowDb[endpoint.IpAddr.String()] = ipFlow

	if endpoint.EndpointType == "internal-bgp" {
		return nil
	}

	// Install dst group entry for the endpoint
	err = self.policyAgent.AddEndpoint(&endpoint)
	if err != nil {
		log.Errorf("Error adding endpoint to policy agent{%+v}. Err: %v", endpoint, err)
		return err
	}

	path := &OfnetProtoRouteInfo{
		ProtocolType: "bgp",
		localEpIP:    endpoint.IpAddr.String(),
		nextHopIP:    self.agent.GetRouterInfo().RouterIP,
	}
	self.agent.AddLocalProtoRoute(path)

	return nil
}

/* RemoveLocalEndpoint does the following
1) Removes the local endpoint and associated flows from OVS
2) Withdraws the route from BGP RIB
*/
func (self *Vlrouter) RemoveLocalEndpoint(endpoint OfnetEndpoint) error {

	// Remove the port vlan flow.
	portVlanFlow := self.portVlanFlowDb[endpoint.PortNo]
	if portVlanFlow != nil {
		err := portVlanFlow.Delete()
		if err != nil {
			log.Errorf("Error deleting portvlan flow. Err: %v", err)
		}
	}

	// Find the flow entry
	ipFlow := self.flowDb[endpoint.IpAddr.String()]
	if ipFlow == nil {
		log.Errorf("Error finding the flow for endpoint: %+v", endpoint)
		return errors.New("Flow not found")
	}

	// Delete the Fgraph entry
	err := ipFlow.Delete()
	if err != nil {
		log.Errorf("Error deleting the endpoint: %+v. Err: %v", endpoint, err)
	}

	// Remove the endpoint from policy tables
	if endpoint.EndpointType != "internal-bgp" {
		err = self.policyAgent.DelEndpoint(&endpoint)
		if err != nil {
			log.Errorf("Error deleting endpoint to policy agent{%+v}. Err: %v", endpoint, err)
			return err
		}
	}
	path := &OfnetProtoRouteInfo{
		ProtocolType: "bgp",
		localEpIP:    endpoint.IpAddr.String(),
		nextHopIP:    self.agent.GetRouterInfo().RouterIP,
	}

	self.agent.DeleteLocalProtoRoute(path)

	return nil
}

// Add a vlan.
// This is mainly used for mapping vlan id to Vxlan VNI
func (self *Vlrouter) AddVlan(vlanId uint16, vni uint32) error {
	// FIXME: Add this for multiple VRF support
	return nil
}

// Remove a vlan
func (self *Vlrouter) RemoveVlan(vlanId uint16, vni uint32) error {
	// FIXME: Add this for multiple VRF support
	return nil
}

/* AddEndpoint does the following :
1)Adds a remote endpoint and associated flows to OVS
2)The remotes routes can be 3 endpoint types :
  a) internal - json rpc based learning from peer netplugins/ofnetagents in the cluster
	b) external - remote endpoint learn via BGP
	c) external-bgp - endpoint of BGP peer
*/
func (self *Vlrouter) AddEndpoint(endpoint *OfnetEndpoint) error {

	nexthopEp := self.agent.getEndpointByIp(net.ParseIP(self.myBgpPeer))
	if nexthopEp != nil && nexthopEp.PortNo != 0 {
		endpoint.MacAddrStr = nexthopEp.MacAddrStr
		endpoint.PortNo = nexthopEp.PortNo
	} else {
		endpoint.PortNo = 0
		endpoint.MacAddrStr = " "
		if endpoint.EndpointType != "external-bgp" {
			//for the remote endpoints maintain a cache of
			//routes that need to be resolved to next hop.
			// bgp peer resolution happens via ARP and hence not
			//maintainer in cache.
			log.Info("Storing endpoint info in cache")
			self.unresolvedEPs[endpoint.EndpointID] = endpoint.EndpointID
		}
	}
	if endpoint.EndpointType == "external-bgp" {
		self.myBgpPeer = endpoint.IpAddr.String()
	}
	log.Infof("AddEndpoint call for endpoint: %+v", endpoint)

	outPort, err := self.ofSwitch.OutputPort(endpoint.PortNo)
	if err != nil {
		log.Errorf("Error creating output port %d. Err: %v", endpoint.PortNo, err)
		return err
	}

	// Install the IP address
	ipFlow, err := self.ipTable.NewFlow(ofctrl.FlowMatch{
		Priority:  FLOW_MATCH_PRIORITY,
		Ethertype: 0x0800,
		IpDa:      &endpoint.IpAddr,
		IpDaMask:  &endpoint.IpMask,
	})
	if err != nil {
		log.Errorf("Error creating flow for endpoint: %+v. Err: %v", endpoint, err)
		return err
	}

	// Set Mac addresses
	DAMac, _ := net.ParseMAC(endpoint.MacAddrStr)
	ipFlow.SetMacDa(DAMac)
	ipFlow.SetMacSa(self.myRouterMac)

	// Set VNI
	// FIXME: hardcode VNI for default VRF.
	// FIXME: We need to use fabric VNI per VRF
	// FIXME: Cant pop vlan tag till the flow matches on vlan.

	// Point it to output port
	err = ipFlow.Next(outPort)
	if err != nil {
		log.Errorf("Error installing flow for endpoint: %+v. Err: %v", endpoint, err)
		return err
	}

	// Install dst group entry for the endpoint
	if endpoint.EndpointType == "internal" {
		err = self.policyAgent.AddEndpoint(endpoint)
		if err != nil {
			log.Errorf("Error adding endpoint to policy agent{%+v}. Err: %v", endpoint, err)
			return err
		}
	}
	// Store it in flow db
	self.flowDb[endpoint.IpAddr.String()] = ipFlow

	return nil
}

// RemoveEndpoint removes an endpoint from the datapath
func (self *Vlrouter) RemoveEndpoint(endpoint *OfnetEndpoint) error {

	//Delete the endpoint if it is in the cache
	delete(self.unresolvedEPs, endpoint.EndpointID)

	// Find the flow entry
	ipFlow := self.flowDb[endpoint.IpAddr.String()]
	if ipFlow == nil {
		log.Errorf("Error finding the flow for endpoint: %+v", endpoint)
		return errors.New("Flow not found")
	}

	// Delete the Fgraph entry
	err := ipFlow.Delete()
	if err != nil {
		log.Errorf("Error deleting the endpoint: %+v. Err: %v", endpoint, err)
	}

	//Remove the endpoint from policy tables
	if endpoint.EndpointType == "internal" {
		err = self.policyAgent.DelEndpoint(endpoint)
		if err != nil {
			log.Errorf("Error deleting endpoint to policy agent{%+v}. Err: %v", endpoint, err)
			return err
		}
	}

	return nil
}

// initialize Fgraph on the switch
func (self *Vlrouter) initFgraph() error {
	sw := self.ofSwitch

	// Create all tables
	self.inputTable = sw.DefaultTable()
	self.vlanTable, _ = sw.NewTable(VLAN_TBL_ID)
	self.ipTable, _ = sw.NewTable(IP_TBL_ID)

	// Init policy tables
	err := self.policyAgent.InitTables(IP_TBL_ID)
	if err != nil {
		log.Fatalf("Error installing policy table. Err: %v", err)
		return err
	}

	//Create all drop entries
	// Drop mcast source mac
	bcastMac, _ := net.ParseMAC("01:00:00:00:00:00")
	bcastSrcFlow, _ := self.inputTable.NewFlow(ofctrl.FlowMatch{
		Priority:  FLOW_MATCH_PRIORITY,
		MacSa:     &bcastMac,
		MacSaMask: &bcastMac,
	})
	bcastSrcFlow.Next(sw.DropAction())

	// Redirect ARP packets to controller
	arpFlow, _ := self.inputTable.NewFlow(ofctrl.FlowMatch{
		Priority:  FLOW_MATCH_PRIORITY,
		Ethertype: 0x0806,
	})
	arpFlow.Next(sw.SendToController())

	//All ARP replies will need IP table lookup
	Mac, _ := net.ParseMAC("00:00:11:11:11:11")
	arpFlow, _ = self.inputTable.NewFlow(ofctrl.FlowMatch{
		Priority:  300,
		Ethertype: 0x0806,
		MacSa:     &Mac,
	})
	arpFlow.Next(self.ipTable)

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

/*processArp does the following :
1)  Process incoming ARP packets
2)  Proxy with Router mac if arp request is from local internal endpoint
3)  Proxy with interface mac is arp request is from remote endpoint
4) Learn MAC,Port of the source if its not learnt and it is bgp peer endpoint
*/
func (self *Vlrouter) processArp(pkt protocol.Ethernet, inPort uint32) {
	log.Debugf("processing ARP packet on port %d", inPort)
	switch t := pkt.Data.(type) {
	case *protocol.ARP:
		log.Debugf("ARP packet: %+v", *t)
		var arpHdr protocol.ARP = *t
		var srcMac net.HardwareAddr
		var intf *net.Interface

		switch arpHdr.Operation {
		case protocol.Type_Request:
			// Lookup the Dest IP in the endpoint table
			endpoint := self.agent.getEndpointByIp(arpHdr.IPDst)
			if endpoint == nil {
				//If we dont know the IP address, dont send an ARP response
				log.Infof("Received ARP request for unknown IP: %v ", arpHdr.IPDst)
				return
			} else {
				if endpoint.EndpointType == "internal" || endpoint.EndpointType == "internal-bgp" {
					//srcMac, _ = net.ParseMAC(endpoint.MacAddrStr)
					intf, _ = net.InterfaceByName(self.agent.GetRouterInfo().VlanIntf)
					srcMac = intf.HardwareAddr
				} else if endpoint.EndpointType == "external" || endpoint.EndpointType == "external-bgp" {
					endpoint = self.agent.getEndpointByIp(arpHdr.IPSrc)
					if endpoint != nil {
						if endpoint.EndpointType == "internal" || endpoint.EndpointType == "internal-bgp" {
							srcMac = self.myRouterMac
						} else {
							return
						}

					} else {
						return
					}

				}
			}

			//Check if source endpoint is learnt.
			endpoint = self.agent.getEndpointByIp(arpHdr.IPSrc)
			if endpoint != nil && endpoint.EndpointType == "external-bgp" {
				//endpoint exists from where the arp is received.
				if endpoint.PortNo == 0 {
					log.Infof("Received ARP from BGP Peer on %s: Mac: %s", endpoint.PortNo, endpoint.MacAddrStr)
					//learn the mac address and portno for the endpoint
					self.RemoveEndpoint(endpoint)
					endpoint.PortNo = inPort
					endpoint.MacAddrStr = arpHdr.HWSrc.String()
					self.agent.endpointDb[endpoint.EndpointID] = endpoint
					self.AddEndpoint(endpoint)
					self.resolveUnresolvedEPs(endpoint.MacAddrStr, inPort)

				}
			}

			// Form an ARP response
			arpResp, _ := protocol.NewARP(protocol.Type_Reply)
			arpResp.HWSrc = srcMac
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
		case protocol.Type_Reply:
			endpoint := self.agent.getEndpointByIp(arpHdr.IPSrc)
			if endpoint != nil && endpoint.EndpointType == "external-bgp" {
				//endpoint exists from where the arp is received.
				if endpoint.PortNo == 0 {
					log.Infof("Received ARP from BGP Peer on %s: Mac: %s", endpoint.PortNo, endpoint.MacAddrStr)
					//learn the mac address and portno for the endpoint
					self.RemoveEndpoint(endpoint)
					endpoint.PortNo = inPort
					endpoint.MacAddrStr = arpHdr.HWSrc.String()
					self.agent.endpointDb[endpoint.EndpointID] = endpoint
					self.AddEndpoint(endpoint)
					self.resolveUnresolvedEPs(endpoint.MacAddrStr, inPort)

				}
			}

		default:
			log.Infof("Dropping ARP response packet from port %d", inPort)
		}
	}
}
func (self *Vlrouter) AddVtepPort(portNo uint32, remoteIp net.IP) error {
	return nil
}

// Remove a VTEP port
func (self *Vlrouter) RemoveVtepPort(portNo uint32, remoteIp net.IP) error {
	return nil
}

/*resolveUnresolvedEPs walks through the unresolved endpoint list and resolves
over given mac and port*/

func (self *Vlrouter) resolveUnresolvedEPs(MacAddrStr string, portNo uint32) {

	for endpointID, _ := range self.unresolvedEPs {
		endpoint := self.agent.endpointDb[endpointID]
		self.RemoveEndpoint(endpoint)
		endpoint.PortNo = portNo
		endpoint.MacAddrStr = MacAddrStr
		self.agent.endpointDb[endpoint.EndpointID] = endpoint
		self.AddEndpoint(endpoint)
		delete(self.unresolvedEPs, endpointID)
	}

}

// AddUplink adds an uplink to the switch
func (self *Vlrouter) AddUplink(portNo uint32) error {
	log.Infof("Adding uplink port: %+v", portNo)

	// Install a flow entry for vlan mapping and point it to Mac table
	portVlanFlow, err := self.vlanTable.NewFlow(ofctrl.FlowMatch{
		Priority:  FLOW_MATCH_PRIORITY,
		InputPort: portNo,
	})
	if err != nil {
		log.Errorf("Error creating portvlan entry. Err: %v", err)
		if strings.Contains(err.Error(), "Flow already exists") {
			return nil
		}
		return err
	}

	// Packets coming from uplink go thru policy and iptable lookup
	//FIXME: Change next to Policy table
	err = portVlanFlow.Next(self.ipTable)
	if err != nil {
		log.Errorf("Error installing portvlan entry. Err: %v", err)
		return err
	}

	// save the flow entry
	self.portVlanFlowDb[portNo] = portVlanFlow

	return nil
}

func (self *Vlrouter) RemoveUplink(portNo uint32) error {
	return nil
}

// AddSvcSpec adds a service spec to proxy
func (self *Vlrouter) AddSvcSpec(svcName string, spec *ServiceSpec) error {
	return nil
}

// DelSvcSpec removes a service spec from proxy
func (self *Vlrouter) DelSvcSpec(svcName string, spec *ServiceSpec) error {
	return nil
}

// SvcProviderUpdate Service Proxy Back End update
func (self *Vlrouter) SvcProviderUpdate(svcName string, providers []string) {
}
