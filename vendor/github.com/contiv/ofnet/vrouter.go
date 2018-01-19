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
	"errors"
	"fmt"
	"net"
	"net/rpc"
	"strings"

	"github.com/contiv/libOpenflow/openflow13"
	"github.com/contiv/libOpenflow/protocol"
	"github.com/contiv/ofnet/ofctrl"
	"github.com/spf13/pflag"

	log "github.com/Sirupsen/logrus"
	cmap "github.com/streamrail/concurrent-map"
)

// Vrouter state.
// One Vrouter instance exists on each host
type Vrouter struct {
	agent       *OfnetAgent      // Pointer back to ofnet agent that owns this
	ofSwitch    *ofctrl.OFSwitch // openflow switch we are talking to
	policyAgent *PolicyAgent     // Policy agent
	svcProxy    *ServiceProxy    // Service proxy

	// Fgraph tables
	inputTable     *ofctrl.Table // Packet lookup starts here
	vlanTable      *ofctrl.Table // Vlan Table. map port or VNI to vlan
	ipTable        *ofctrl.Table // IP lookup table
	proxySNATTable *ofctrl.Table // Svc proxy SNAT table
	hostSNATTable  *ofctrl.Table // Egress via host nat port
	hostDNATTable  *ofctrl.Table // Ingress via host nat port

	// Flow Database
	flowDb         map[string]*ofctrl.Flow   // Database of flow entries
	portVlanFlowDb map[uint32]*ofctrl.Flow   // Database of flow entries
	dscpFlowDb     map[uint32][]*ofctrl.Flow // Database of flow entries
	portDnsFlowDb  cmap.ConcurrentMap        // Database of flow entries
	vlanDb         map[uint16]*Vlan          // Database of known vlans

	myRouterMac   net.HardwareAddr   // Router Mac to be used
	hostNATInfo   HostPortInfo       // Information for host NAT access
	hostNATFlowDB cmap.ConcurrentMap // Database of host NAT flows
}

// Create a new vrouter instance
func NewVrouter(agent *OfnetAgent, rpcServ *rpc.Server) *Vrouter {
	vrouter := new(Vrouter)

	// Keep a reference to the agent
	vrouter.agent = agent

	// Create policy agent
	vrouter.policyAgent = NewPolicyAgent(agent, rpcServ)
	vrouter.svcProxy = NewServiceProxy(agent)

	// Create a flow dbs and my router mac
	vrouter.flowDb = make(map[string]*ofctrl.Flow)
	vrouter.portVlanFlowDb = make(map[uint32]*ofctrl.Flow)
	vrouter.dscpFlowDb = make(map[uint32][]*ofctrl.Flow)
	vrouter.portDnsFlowDb = cmap.New()
	vrouter.myRouterMac, _ = net.ParseMAC("00:00:11:11:11:11")
	vrouter.vlanDb = make(map[uint16]*Vlan)
	vrouter.hostNATFlowDB = cmap.New()

	return vrouter
}

// Handle new master added event
func (self *Vrouter) MasterAdded(master *OfnetNode) error {

	return nil
}

// Handle switch connected notification
func (self *Vrouter) SwitchConnected(sw *ofctrl.OFSwitch) {
	// Keep a reference to the switch
	self.ofSwitch = sw

	log.Infof("Switch connected(vrouter). installing flows")

	self.svcProxy.SwitchConnected(sw)
	// Tell the policy agent about the switch
	self.policyAgent.SwitchConnected(sw)

	// Init the Fgraph
	self.initFgraph()
}

// Handle switch disconnected notification
func (self *Vrouter) SwitchDisconnected(sw *ofctrl.OFSwitch) {

	self.svcProxy.SwitchDisconnected(sw)
	// Tell the policy agent about the switch disconnected
	self.policyAgent.SwitchDisconnected(sw)

	self.ofSwitch = nil

}

// Handle incoming packet
func (self *Vrouter) PacketRcvd(sw *ofctrl.OFSwitch, pkt *ofctrl.PacketIn) {
	if pkt.TableId == SRV_PROXY_SNAT_TBL_ID || pkt.TableId == SRV_PROXY_DNAT_TBL_ID {
		// these are destined to service proxy
		self.svcProxy.HandlePkt(pkt)
		return
	}

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

	case protocol.IPv4_MSG:
		var inPort uint32
		if (pkt.TableId == 0) && (pkt.Match.Type == openflow13.MatchType_OXM) &&
			(pkt.Match.Fields[0].Class == openflow13.OXM_CLASS_OPENFLOW_BASIC) &&
			(pkt.Match.Fields[0].Field == openflow13.OXM_FIELD_IN_PORT) {

			// get the input port number
			switch t := pkt.Match.Fields[0].Value.(type) {
			case *openflow13.InPortField:
				inPort = t.InPort
			default:
				log.Debugf("unknown match type %v for ipv4 pkt", t)
				return
			}
		}

		ipPkt := pkt.Data.Data.(*protocol.IPv4)
		switch ipPkt.Protocol {
		case protocol.Type_UDP:
			udpPkt := ipPkt.Data.(*protocol.UDP)
			switch udpPkt.PortDst {
			case 53:
				if (pkt.TableId == 0) && (pkt.Match.Type == openflow13.MatchType_OXM) &&
					len(pkt.Match.Fields) >= 2 &&
					(pkt.Match.Fields[1].Class == openflow13.OXM_CLASS_OPENFLOW_BASIC) &&
					(pkt.Match.Fields[1].Field == openflow13.OXM_FIELD_TUNNEL_ID) {
					self.agent.incrErrStats("dnsPktVtep")
					return
				}

				if dnsResp, err := processDNSPkt(self.agent, inPort, udpPkt.Data); err == nil {
					if respPkt, err := buildUDPRespPkt(&pkt.Data, dnsResp); err == nil {
						self.agent.incrStats("dnsPktReply")
						pktOut := openflow13.NewPacketOut()
						pktOut.Data = respPkt
						pktOut.AddAction(openflow13.NewActionOutput(inPort))
						self.ofSwitch.Send(pktOut)
						return
					}
				}

				// re-inject DNS packet
				ethPkt := buildDnsForwardPkt(&pkt.Data)
				pktOut := openflow13.NewPacketOut()
				pktOut.Data = ethPkt
				pktOut.InPort = inPort

				pktOut.AddAction(openflow13.NewActionOutput(openflow13.P_TABLE))
				self.agent.incrStats("dnsPktForward")
				self.ofSwitch.Send(pktOut)
				return
			}
		}
	default:
		log.Errorf("Received unknown ethertype: %x", pkt.Data.Ethertype)
	}
}

// InjectGARPs not implemented
func (self *Vrouter) InjectGARPs(epgID int) {
}

// GlobalConfigUpdate not implemented
func (self *Vrouter) GlobalConfigUpdate(cfg OfnetGlobalConfig) error {
	return nil
}

// Add a local endpoint and install associated local route
func (self *Vrouter) AddLocalEndpoint(endpoint OfnetEndpoint) error {
	dNATTbl := self.ofSwitch.GetTable(SRV_PROXY_DNAT_TBL_ID)

	// check to make sure we have vlan to vni mapping
	vni := self.agent.getvlanVniMap(endpoint.Vlan)
	if vni == nil {
		log.Errorf("VNI for vlan %d is not known", endpoint.Vlan)
		return errors.New("Unknown Vlan")
	}

	// Install a flow entry for vlan mapping and point it to next table
	portVlanFlow, err := createPortVlanFlow(self.agent, self.vlanTable, dNATTbl, &endpoint)
	if err != nil {
		log.Errorf("Error creating portvlan entry. Err: %v", err)
		return err
	}

	// save the flow entry
	self.portVlanFlowDb[endpoint.PortNo] = portVlanFlow

	// install DSCP flow entries if required
	if endpoint.Dscp != 0 {
		dscpV4Flow, dscpV6Flow, err := createDscpFlow(self.agent, self.vlanTable, dNATTbl, &endpoint)
		if err != nil {
			log.Errorf("Error installing DSCP flows. Err: %v", err)
			return err
		}

		// save it for tracking
		self.dscpFlowDb[endpoint.PortNo] = []*ofctrl.Flow{dscpV4Flow, dscpV6Flow}
	}

	// Create the output port
	outPort, err := self.ofSwitch.OutputPort(endpoint.PortNo)
	if err != nil {
		log.Errorf("Error creating output port %d. Err: %v", endpoint.PortNo, err)
		return err
	}

	//Ip table look up will be vrf,ip
	vrfid := self.agent.getvrfId(endpoint.Vrf)
	if vrfid == nil {
		log.Errorf("Invalid vrf name:%v", endpoint.Vrf)
		return errors.New("Invalid vrf name")
	}

	vrfmetadata, vrfmetadataMask := Vrfmetadata(*vrfid)

	// Install the IP address
	ipFlow, err := self.ipTable.NewFlow(ofctrl.FlowMatch{
		Priority:     FLOW_MATCH_PRIORITY,
		Ethertype:    0x0800,
		IpDa:         &endpoint.IpAddr,
		Metadata:     &vrfmetadata,
		MetadataMask: &vrfmetadataMask,
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

	// Install dst group entry for the endpoint
	err = self.policyAgent.AddEndpoint(&endpoint)
	if err != nil {
		log.Errorf("Error adding endpoint to policy agent{%+v}. Err: %v", endpoint, err)
		return err
	}

	// Store the flow
	//using endpointId as key for flowDb because
	//endpointId is a combination of ip and vrf
	flowId := self.agent.getEndpointIdByIpVlan(endpoint.IpAddr, endpoint.Vlan)
	self.flowDb[flowId] = ipFlow

	// Install the IPv6 address
	if endpoint.Ipv6Addr != nil && endpoint.Ipv6Addr.String() != "" {
		err = self.AddLocalIpv6Flow(endpoint)
		if err != nil {
			return err
		}
	}

	if endpoint.HostPvtIP.String() != "<nil>" && self.hostNATInfo.PortNo != 0 {
		return self.setupHostNAT(endpoint, vrfmetadata, vrfmetadataMask)
	}
	return nil
}

func (self *Vrouter) setupHostNAT(endpoint OfnetEndpoint, vrfmetadata, vrfmetadataMask uint64) error {
	// setup the host NAT flows
	hsNAT, err := self.hostSNATTable.NewFlow(ofctrl.FlowMatch{
		Priority:  FLOW_MATCH_PRIORITY,
		Ethertype: 0x0800,
		InputPort: endpoint.PortNo,
	})

	if err != nil {
		log.Errorf("Error adding endpoint to host SNAT {%+v}. Err: %v", endpoint, err)
		return err
	}

	hsNAT.SetIPField(endpoint.HostPvtIP, "Src")
	hsNAT.SetMacDa(self.hostNATInfo.MacAddr)
	natOut, err := self.ofSwitch.OutputPort(self.hostNATInfo.PortNo)
	if err != nil {
		return err
	}
	// Point the route at output port
	err = hsNAT.Next(natOut)
	if err != nil {
		log.Errorf("Error installing host NAT out flow for endpoint: %+v. Err: %v", endpoint, err)
		return err
	}
	snatFlowKey := endpoint.HostPvtIP.String() + ".snat"
	self.hostNATFlowDB.Set(snatFlowKey, hsNAT)

	hdNAT, err := self.hostDNATTable.NewFlow(ofctrl.FlowMatch{
		Priority:  FLOW_MATCH_PRIORITY,
		InputPort: self.hostNATInfo.PortNo,
		Ethertype: 0x0800,
		IpDa:      &endpoint.HostPvtIP,
	})

	if err != nil {
		log.Errorf("Error adding endpoint to host DNAT {%+v}. Err: %v", endpoint, err)
		return err
	}

	hdNAT.SetIPField(endpoint.IpAddr, "Dst")
	eMac, err := net.ParseMAC(endpoint.MacAddrStr)
	if err != nil {
		log.Errorf("Error parsing ep MAC: %s", endpoint.MacAddrStr)
		return err
	}
	hdNAT.SetMacDa(eMac)
	hdNAT.SetMetadata(vrfmetadata, vrfmetadataMask)
	// Point to SNAT table
	err = hdNAT.Next(self.proxySNATTable)
	if err != nil {
		log.Errorf("Error installing host NAT in flow for endpoint: %+v. Err: %v", endpoint, err)
		return err
	}
	dnatFlowKey := endpoint.HostPvtIP.String() + ".dnat"
	self.hostNATFlowDB.Set(dnatFlowKey, hdNAT)
	return nil
}

// Remove local endpoint
func (self *Vrouter) RemoveLocalEndpoint(endpoint OfnetEndpoint) error {
	// Remove host nat flows
	dnatFlowKey := endpoint.HostPvtIP.String() + ".dnat"
	hdFlow, _ := self.hostNATFlowDB.Get(dnatFlowKey)
	if hdFlow != nil {
		hdFlow.(*ofctrl.Flow).Delete()
		self.hostNATFlowDB.Remove(dnatFlowKey)
	}
	snatFlowKey := endpoint.HostPvtIP.String() + ".snat"
	hsFlow, _ := self.hostNATFlowDB.Get(snatFlowKey)
	if hsFlow != nil {
		hsFlow.(*ofctrl.Flow).Delete()
		self.hostNATFlowDB.Remove(snatFlowKey)
	}

	// Remove the port vlan flow.
	portVlanFlow := self.portVlanFlowDb[endpoint.PortNo]
	if portVlanFlow != nil {
		err := portVlanFlow.Delete()
		if err != nil {
			log.Errorf("Error deleting portvlan flow. Err: %v", err)
		}
	}

	// Remove dscp flows.
	dscpFlows, found := self.dscpFlowDb[endpoint.PortNo]
	if found {
		for _, dflow := range dscpFlows {
			err := dflow.Delete()
			if err != nil {
				log.Errorf("Error deleting dscp flow {%+v}. Err: %v", dflow, err)
			}
		}
	}

	// Find the flow entry
	flowId := self.agent.getEndpointIdByIpVlan(endpoint.IpAddr, endpoint.Vlan)
	ipFlow := self.flowDb[flowId]
	if ipFlow == nil {
		log.Errorf("Error finding the flow for endpoint: %+v", endpoint)
		return errors.New("Flow not found")
	}

	// Delete the Fgraph entry
	err := ipFlow.Delete()
	if err != nil {
		log.Errorf("Error deleting the endpoint: %+v. Err: %v", endpoint, err)
	}

	self.svcProxy.DelEndpoint(&endpoint)
	// Remove the endpoint from policy tables
	err = self.policyAgent.DelEndpoint(&endpoint)
	if err != nil {
		log.Errorf("Error deleting endpoint to policy agent{%+v}. Err: %v", endpoint, err)
		return err
	}

	if endpoint.Ipv6Addr != nil && endpoint.Ipv6Addr.String() != "" {
		err = self.RemoveLocalIpv6Flow(endpoint)
		if err != nil {
			return err
		}
	}

	return nil
}

// UpdateLocalEndpoint update local endpoint state
func (self *Vrouter) UpdateLocalEndpoint(endpoint *OfnetEndpoint, epInfo EndpointInfo) error {
	oldDscp := endpoint.Dscp

	// Remove existing DSCP flows if required
	if epInfo.Dscp == 0 || epInfo.Dscp != endpoint.Dscp {
		// remove old DSCP flows
		dscpFlows, found := self.dscpFlowDb[endpoint.PortNo]
		if found {
			for _, dflow := range dscpFlows {
				err := dflow.Delete()
				if err != nil {
					log.Errorf("Error deleting dscp flow {%+v}. Err: %v", dflow, err)
					return err
				}
			}
		}
	}

	// change DSCP value
	endpoint.Dscp = epInfo.Dscp

	// Add new DSCP flows if required
	if epInfo.Dscp != 0 && epInfo.Dscp != oldDscp {
		dNATTbl := self.ofSwitch.GetTable(SRV_PROXY_DNAT_TBL_ID)

		// add new dscp flows
		dscpV4Flow, dscpV6Flow, err := createDscpFlow(self.agent, self.vlanTable, dNATTbl, endpoint)
		if err != nil {
			log.Errorf("Error installing DSCP flows. Err: %v", err)
			return err
		}

		// save it for tracking
		self.dscpFlowDb[endpoint.PortNo] = []*ofctrl.Flow{dscpV4Flow, dscpV6Flow}
	}

	return nil
}

// AddHostPort sets up host access.
func (self *Vrouter) AddHostPort(hp HostPortInfo) error {
	if hp.Kind == "NAT" && self.hostNATInfo.PortNo != 0 {
		log.Errorf("Host NAT port exists: %+v", self.hostNATInfo)
		return fmt.Errorf("Host NAT port exists")
	}

	ipDa, daMask, err := ParseIPAddrMaskString(hp.IpAddr)
	if err != nil {
		log.Errorf("Bad host route - %v", err)
		return err
	}

	netMask := pflag.ParseIPv4Mask(daMask.String())
	maskedIP := ipDa.Mask(netMask)

	// Save the info
	self.hostNATInfo = hp

	// Set up DNAT for ingress traffic
	inNATFlow, _ := self.inputTable.NewFlow(ofctrl.FlowMatch{
		InputPort: self.hostNATInfo.PortNo,
		// Put at lower priority than ARP rule
		Priority: FLOW_MATCH_PRIORITY - 1,
	})
	inNATFlow.Next(self.hostDNATTable)

	// Packets from hostport that miss in IP table are dropped
	inNATMiss, _ := self.ipTable.NewFlow(ofctrl.FlowMatch{
		InputPort: self.hostNATInfo.PortNo,
		Priority:  FLOW_FLOOD_PRIORITY,
	})
	inNATMiss.Next(self.ofSwitch.DropAction())

	// Deny packets explicitly addressed to the NAT subnet
	denyFlow, _ := self.hostSNATTable.NewFlow(ofctrl.FlowMatch{
		Priority:  HOST_SNAT_DENY_PRIORITY,
		Ethertype: 0x0800,
		IpDa:      &maskedIP,
		IpDaMask:  daMask,
	})
	denyFlow.Next(self.ofSwitch.DropAction())

	self.hostNATFlowDB.Set("inNATFlow", inNATFlow)
	self.hostNATFlowDB.Set("inNATMiss", inNATMiss)
	self.hostNATFlowDB.Set("denyFlow", denyFlow)
	return nil
}

// RemoveHostPort sets up host access.
func (self *Vrouter) RemoveHostPort(hp uint32) error {
	if hp == self.hostNATInfo.PortNo {
		self.hostNATInfo.PortNo = 0
		inNATFlow, _ := self.hostNATFlowDB.Get("inNATFlow")
		inNATMiss, _ := self.hostNATFlowDB.Get("inNATMiss")
		denyFlow, _ := self.hostNATFlowDB.Get("denyFlow")
		if inNATFlow != nil {
			inNATFlow.(*ofctrl.Flow).Delete()
		}
		if inNATMiss != nil {
			inNATMiss.(*ofctrl.Flow).Delete()
		}
		if denyFlow != nil {
			denyFlow.(*ofctrl.Flow).Delete()
		}

		self.hostNATFlowDB.Remove("inNATFlow")
		self.hostNATFlowDB.Remove("inNATMiss")
		self.hostNATFlowDB.Remove("denyFlow")
	}
	return nil
}

// Add a local IPv6 flow
func (self *Vrouter) AddLocalIpv6Flow(endpoint OfnetEndpoint) error {

	vrfid := self.agent.getvrfId(endpoint.Vrf)
	if vrfid == nil {
		log.Errorf("Invalid vrf name:%v", endpoint.Vrf)
		return errors.New("Invalid vrf name")
	}

	// Create the output port
	outPort, err := self.ofSwitch.OutputPort(endpoint.PortNo)
	if err != nil {
		log.Errorf("Error creating output port %d. Err: %v", endpoint.PortNo, err)
		return err
	}

	//Ip table look up will be vrf,ip
	vrfmetadata, vrfmetadataMask := Vrfmetadata(*vrfid)
	// Install the IPv6 address
	ipv6Flow, err := self.ipTable.NewFlow(ofctrl.FlowMatch{
		Priority:     FLOW_MATCH_PRIORITY,
		Ethertype:    0x86DD,
		Ipv6Da:       &endpoint.Ipv6Addr,
		Metadata:     &vrfmetadata,
		MetadataMask: &vrfmetadataMask,
	})
	if err != nil {
		log.Errorf("Error creating flow for endpoint: %+v. Err: %v", endpoint, err)
		return err
	}

	destMacAddr, _ := net.ParseMAC(endpoint.MacAddrStr)

	// Set Mac addresses
	ipv6Flow.SetMacDa(destMacAddr)
	ipv6Flow.SetMacSa(self.myRouterMac)

	// Point the route at output port
	err = ipv6Flow.Next(outPort)
	if err != nil {
		log.Errorf("Error installing flow for endpoint: %+v. Err: %v", endpoint, err)
		return err
	}

	// Install dst group entry for the endpoint
	err = self.policyAgent.AddIpv6Endpoint(&endpoint)
	if err != nil {
		log.Errorf("Error adding IPv6 endpoint to policy agent{%+v}. Err: %v", endpoint, err)
		return err
	}

	// Store the flow
	//using endpointId as key for flowDb because
	//endpointId is a combination of ip and vrf
	flowId := self.agent.getEndpointIdByIpVlan(endpoint.Ipv6Addr, endpoint.Vlan)
	self.flowDb[flowId] = ipv6Flow

	return nil
}

// Remove local IPv6 flow
func (self *Vrouter) RemoveLocalIpv6Flow(endpoint OfnetEndpoint) error {

	// Find the flow entry
	flowId := self.agent.getEndpointIdByIpVlan(endpoint.Ipv6Addr, endpoint.Vlan)
	ipv6Flow := self.flowDb[flowId]
	if ipv6Flow == nil {
		log.Errorf("Error finding the flow for endpoint: %+v", endpoint)
		return errors.New("Flow not found")
	}

	// Delete the Fgraph entry
	err := ipv6Flow.Delete()
	if err != nil {
		log.Errorf("Error deleting the endpoint: %+v. Err: %v", endpoint, err)
	}

	// TODO: where do we add svcProxy endpoint? Do we need it for IPv6?
	//self.svcProxy.DelEndpoint(&endpoint)

	// Remove the endpoint from policy tables
	err = self.policyAgent.DelIpv6Endpoint(&endpoint)
	if err != nil {
		log.Errorf("Error deleting IPv6 endpoint to policy agent{%+v}. Err: %v", endpoint, err)
		return err
	}

	return nil
}

// Add virtual tunnel end point. This is mainly used for mapping remote vtep IP
// to ofp port number.
func (self *Vrouter) AddVtepPort(portNo uint32, remoteIp net.IP) error {
	// Install VNI to vlan mapping for each vni
	log.Infof("Adding VTEP for portno %v , remote IP : %v", portNo, remoteIp)
	dnsVtepFlow, err := self.inputTable.NewFlow(ofctrl.FlowMatch{
		Priority:   DNS_FLOW_MATCH_PRIORITY + 2,
		InputPort:  portNo,
		Ethertype:  protocol.IPv4_MSG,
		IpProto:    protocol.Type_UDP,
		UdpDstPort: 53,
	})
	if err != nil {
		log.Errorf("Error creating nameserver flow entry. Err: %v", err)
		return err
	}
	dnsVtepFlow.Next(self.vlanTable)
	self.portDnsFlowDb.Set(fmt.Sprintf("%d", portNo), dnsVtepFlow)

	self.agent.vlanVniMutex.RLock()
	for vni, vlan := range self.agent.vniVlanMap {
		// Install a flow entry for  VNI/vlan and point it to Ip table
		portVlanFlow, err := self.vlanTable.NewFlow(ofctrl.FlowMatch{
			Priority:  FLOW_MATCH_PRIORITY,
			InputPort: portNo,
			TunnelId:  uint64(vni),
		})

		if err != nil && strings.Contains(err.Error(), "Flow already exists") {
			log.Infof("VTEP %s already exists", remoteIp.String())
			self.agent.vlanVniMutex.RUnlock()
			return nil
		} else if err != nil {
			log.Errorf("Error adding Flow for VTEP %v. Err: %v", remoteIp, err)
			self.agent.vlanVniMutex.RUnlock()
			return err
		}

		// Set the metadata to indicate packet came in from VTEP port

		vrf := self.agent.getvlanVrf(*vlan)

		if vrf == nil {
			log.Errorf("Invalid vlan to Vrf mapping for %v", *vlan)
			self.agent.vlanVniMutex.RUnlock()
			return errors.New("Invalid vlan to Vrf mapping")
		}

		vrfid := self.agent.getvrfId(*vrf)
		if vrfid == nil || *vrfid == 0 {
			log.Errorf("Invalid vrf name:%v", *vrf)
			self.agent.vlanVniMutex.RUnlock()
			return errors.New("Invalid vrf name")
		}

		//set vrf id as METADATA
		vrfmetadata, vrfmetadataMask := Vrfmetadata(*vrfid)

		metadata := METADATA_RX_VTEP | vrfmetadata
		metadataMask := METADATA_RX_VTEP | vrfmetadataMask

		portVlanFlow.SetMetadata(metadata, metadataMask)

		// Point it to next table.
		// Note that we bypass policy lookup on dest host.
		sNATTbl := self.ofSwitch.GetTable(SRV_PROXY_SNAT_TBL_ID)
		portVlanFlow.Next(sNATTbl)

		// save the port vlan flow for cleaning up later
		self.vlanDb[*vlan].vtepVlanFlowDb[portNo] = portVlanFlow
	}
	self.agent.vlanVniMutex.RUnlock()
	var ep *OfnetEndpoint
	// walk all routes and see if we need to install it
	for endpoint := range self.agent.endpointDb.IterBuffered() {
		ep = endpoint.Val.(*OfnetEndpoint)
		if ep.OriginatorIp.String() == remoteIp.String() {
			err := self.AddEndpoint(ep)
			if err != nil {
				log.Errorf("Error installing endpoint during vtep add(%v) EP: %+v. Err: %v", remoteIp, ep, err)
				return err
			}
		}
	}
	return nil
}

// Remove a VTEP port
func (self *Vrouter) RemoveVtepPort(portNo uint32, remoteIp net.IP) error {
	if f, ok := self.portDnsFlowDb.Get(fmt.Sprintf("%d", portNo)); ok {
		if dnsVtepFlow, ok := f.(*ofctrl.Flow); ok {
			if err := dnsVtepFlow.Delete(); err != nil {
				log.Errorf("Error deleting nameserver flow. Err: %v", err)
			}
		}
	}
	self.portDnsFlowDb.Remove(fmt.Sprintf("%d", portNo))

	for _, vlan := range self.vlanDb {
		portVlanFlow := vlan.vtepVlanFlowDb[portNo]
		portVlanFlow.Delete()
		delete(vlan.vtepVlanFlowDb, portNo)
	}

	return nil
}

// Add a vlan.
// This is mainly used for mapping vlan id to Vxlan VNI
func (self *Vrouter) AddVlan(vlanId uint16, vni uint32, vrf string) error {

	// check if the vlan already exists. if it does, we are done
	if self.vlanDb[vlanId] != nil {
		return nil
	}

	// create new vlan object
	vlan := new(Vlan)
	vlan.Vni = vni
	vlan.vtepVlanFlowDb = make(map[uint32]*ofctrl.Flow)

	_, ok := self.agent.createVrf(vrf)
	if !ok {
		return errors.New("Error creating Vrf")
	}

	self.agent.vlanVrfMutex.Lock()
	self.agent.vlanVrf[vlanId] = &vrf
	self.agent.vlanVrfMutex.Unlock()

	// Walk all VTEP ports and add vni-vlan mapping for new VNI
	self.agent.vtepTableMutex.RLock()
	defer self.agent.vtepTableMutex.RUnlock()
	for _, vtepPort := range self.agent.vtepTable {
		// Install a flow entry for  VNI/vlan and point it to macDest table
		portVlanFlow, err := self.vlanTable.NewFlow(ofctrl.FlowMatch{
			Priority:  FLOW_MATCH_PRIORITY,
			InputPort: *vtepPort,
			TunnelId:  uint64(vni),
		})
		if err != nil {
			log.Errorf("Error creating port vlan flow for vlan %d. Err: %v", vlanId, err)
			return err
		}

		var vrfid *uint16
		if vrfid = self.agent.getvrfId(vrf); vrfid == nil {
			log.Errorf("Invalid vrf name:%v", vrf)
			return errors.New("Invalid vrf name")
		}

		//set vrf id as METADATA
		vrfmetadata, vrfmetadataMask := Vrfmetadata(*vrfid)

		// Set the metadata to indicate packet came in from VTEP port
		metadata := METADATA_RX_VTEP | vrfmetadata
		metadataMask := METADATA_RX_VTEP | vrfmetadataMask

		portVlanFlow.SetMetadata(metadata, metadataMask)

		// Point to next table
		sNATTbl := self.ofSwitch.GetTable(SRV_PROXY_SNAT_TBL_ID)
		portVlanFlow.Next(sNATTbl)
		vlan.vtepVlanFlowDb[*vtepPort] = portVlanFlow
	}

	// store it in DB
	self.vlanDb[vlanId] = vlan

	return nil
}

// Remove a vlan
func (self *Vrouter) RemoveVlan(vlanId uint16, vni uint32, vrf string) error {
	vlan := self.vlanDb[vlanId]
	if vlan == nil {
		log.Fatalf("Could not find the vlan %d", vlanId)
	}
	// uninstall vtep vlan mapping flows
	for _, portVlanFlow := range vlan.vtepVlanFlowDb {
		portVlanFlow.Delete()
	}

	// Remove it from DB
	delete(self.vlanDb, vlanId)
	err := self.agent.deleteVrf(vrf)
	if err != nil {
		return err
	}
	self.agent.vlanVrfMutex.Lock()
	delete(self.agent.vlanVrf, vlanId)
	self.agent.vlanVrfMutex.Unlock()
	return nil
}

// AddEndpoint Add an endpoint to the datapath
func (self *Vrouter) AddEndpoint(endpoint *OfnetEndpoint) error {
	if endpoint.Vni == 0 {
		return nil
	}

	// Lookup the VTEP for the endpoint
	vtepPort := self.agent.getvtepTablePort(endpoint.OriginatorIp.String())
	if vtepPort == nil {
		log.Warnf("Could not find the VTEP for endpoint: %+v", endpoint)

		// Return if VTEP is not found. We'll install the route when VTEP is added
		return nil
	}

	// Install the endpoint in OVS
	// Create an output port for the vtep
	outPort, err := self.ofSwitch.OutputPort(*vtepPort)
	if err != nil {
		log.Errorf("Error creating output port %d. Err: %v", *vtepPort, err)
		return err
	}

	vrfid := self.agent.getvrfId(endpoint.Vrf)
	if vrfid == nil || *vrfid == 0 {
		log.Errorf("Invalid vrf name:%v", endpoint.Vrf)
		return errors.New("Invalid vrf name")
	}

	//set vrf id as METADATA
	metadata, metadataMask := Vrfmetadata(*vrfid)

	// Install the IP address
	ipFlow, err := self.ipTable.NewFlow(ofctrl.FlowMatch{
		Priority:     FLOW_MATCH_PRIORITY,
		Ethertype:    0x0800,
		IpDa:         &endpoint.IpAddr,
		Metadata:     &metadata,
		MetadataMask: &metadataMask,
	})
	if err != nil {
		log.Errorf("Error creating flow for endpoint: %+v. Err: %v", endpoint, err)
		return err
	}

	// Set Mac addresses
	ipFlow.SetMacDa(self.myRouterMac)

	// Set VNI
	ipFlow.SetTunnelId(uint64(endpoint.Vni))
	ipFlow.Next(outPort)

	// Point it to output port
	err = ipFlow.Next(outPort)
	if err != nil {
		log.Errorf("Error installing flow for endpoint: %+v. Err: %v", endpoint, err)
		return err
	}

	// Install dst group entry for the endpoint
	err = self.policyAgent.AddEndpoint(endpoint)
	if err != nil {
		log.Errorf("Error adding endpoint to policy agent{%+v}. Err: %v", endpoint, err)
		return err
	}

	// Store it in flow db
	flowId := self.agent.getEndpointIdByIpVlan(endpoint.IpAddr, endpoint.Vlan)
	self.flowDb[flowId] = ipFlow
	// Install dst group entry for IPv6 endpoint
	if endpoint.Ipv6Addr != nil && endpoint.Ipv6Addr.String() != "" {
		err = self.AddRemoteIpv6Flow(endpoint)
		if err != nil {
			log.Errorf("Error adding IPv6 flow {%+v}. Err: %v", endpoint, err)
			return err
		}
	}

	return nil
}

// RemoveEndpoint removes an endpoint from the datapath
func (self *Vrouter) RemoveEndpoint(endpoint *OfnetEndpoint) error {
	// Find the flow entry
	if endpoint.Vni == 0 {
		return nil
	}

	flowId := self.agent.getEndpointIdByIpVlan(endpoint.IpAddr, endpoint.Vlan)
	ipFlow := self.flowDb[flowId]
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
	err = self.policyAgent.DelEndpoint(endpoint)
	if err != nil {
		log.Errorf("Error deleting endpoint to policy agent{%+v}. Err: %v", endpoint, err)
		return err
	}

	// Remove IPv6 endpoint from policy tables
	if endpoint.Ipv6Addr != nil && endpoint.Ipv6Addr.String() != "" {
		err = self.RemoveRemoteIpv6Flow(endpoint)
		if err != nil {
			log.Errorf("Error deleting IPv6 flow for endpoint {%+v}. Err: %v", endpoint, err)
			return err
		}
	}

	return nil
}

// Add IPv6 flow for remote endpoint
func (self *Vrouter) AddRemoteIpv6Flow(endpoint *OfnetEndpoint) error {

	// Lookup the VTEP for the endpoint
	vtepPort := self.agent.getvtepTablePort(endpoint.OriginatorIp.String())
	if vtepPort == nil {
		log.Warnf("Could not find the VTEP for endpoint: %+v", endpoint)

		// Return if VTEP is not found. We'll install the route when VTEP is added
		return nil
	}

	// Install the endpoint in OVS
	// Create an output port for the vtep
	outPort, err := self.ofSwitch.OutputPort(*vtepPort)
	if err != nil {
		log.Errorf("Error creating output port %d. Err: %v", *vtepPort, err)
		return err
	}

	var vrfid *uint16
	if vrfid = self.agent.getvrfId(endpoint.Vrf); vrfid == nil {
		log.Errorf("Invalid vrf name:%v", endpoint.Vrf)
		return errors.New("Invalid vrf name")
	}

	//set vrf id as METADATA
	metadata, metadataMask := Vrfmetadata(*vrfid)

	// Install the IP address
	ipv6Flow, err := self.ipTable.NewFlow(ofctrl.FlowMatch{
		Priority:     FLOW_MATCH_PRIORITY,
		Ethertype:    0x86DD,
		Ipv6Da:       &endpoint.Ipv6Addr,
		Metadata:     &metadata,
		MetadataMask: &metadataMask,
	})

	if err != nil {
		log.Errorf("Error creating flow for endpoint: %+v. Err: %v", endpoint, err)
		return err
	}

	// Set Mac addresses
	ipv6Flow.SetMacDa(self.myRouterMac)

	// Set VNI
	ipv6Flow.SetTunnelId(uint64(endpoint.Vni))
	ipv6Flow.Next(outPort)

	// Point it to output port
	err = ipv6Flow.Next(outPort)
	if err != nil {
		log.Errorf("Error installing flow for endpoint: %+v. Err: %v", endpoint, err)
		return err
	}

	// Install dst group entry for the endpoint
	err = self.policyAgent.AddIpv6Endpoint(endpoint)
	if err != nil {
		log.Errorf("Error adding endpoint to policy agent{%+v}. Err: %v", endpoint, err)
		return err
	}

	// Store it in flow db
	flowId := self.agent.getEndpointIdByIpVlan(endpoint.Ipv6Addr, endpoint.Vlan)
	self.flowDb[flowId] = ipv6Flow

	return nil
}

// Remove IPv6 flow for remote endpoint
func (self *Vrouter) RemoveRemoteIpv6Flow(endpoint *OfnetEndpoint) error {
	// Find the flow entry
	flowId := self.agent.getEndpointIdByIpVlan(endpoint.Ipv6Addr, endpoint.Vlan)
	ipv6Flow := self.flowDb[flowId]
	if ipv6Flow == nil {
		log.Errorf("Error finding the flow for endpoint: %+v", endpoint)
		return errors.New("Flow not found")
	}

	// Delete the Fgraph entry
	err := ipv6Flow.Delete()
	if err != nil {
		log.Errorf("Error deleting the endpoint: %+v. Err: %v", endpoint, err)
	}

	// Remove the endpoint from policy tables
	err = self.policyAgent.DelIpv6Endpoint(endpoint)
	if err != nil {
		log.Errorf("Error deleting endpoint to policy agent{%+v}. Err: %v", endpoint, err)
		return err
	}

	return nil
}

// AddUplink adds an uplink to the switch
func (vr *Vrouter) AddUplink(uplinkPort *PortInfo) error {
	// Nothing to do
	return nil
}

// UpdateUplink updates uplink info
func (vr *Vrouter) UpdateUplink(uplinkName string, updates PortUpdates) error {
	return nil
}

// RemoveUplink remove an uplink to the switch
func (vr *Vrouter) RemoveUplink(uplinkName string) error {
	// Nothing to do
	return nil
}

// AddSvcSpec adds a service spec to proxy
func (vr *Vrouter) AddSvcSpec(svcName string, spec *ServiceSpec) error {
	return vr.svcProxy.AddSvcSpec(svcName, spec)
}

// DelSvcSpec removes a service spec from proxy
func (vr *Vrouter) DelSvcSpec(svcName string, spec *ServiceSpec) error {
	return vr.svcProxy.DelSvcSpec(svcName, spec)
}

// SvcProviderUpdate Service Proxy Back End update
func (vr *Vrouter) SvcProviderUpdate(svcName string, providers []string) {
	vr.svcProxy.ProviderUpdate(svcName, providers)
}

// MultipartReply handles multipart replies
func (vr *Vrouter) MultipartReply(sw *ofctrl.OFSwitch, reply *openflow13.MultipartReply) {
	if reply.Type == openflow13.MultipartType_Flow {
		vr.svcProxy.FlowStats(reply)
	}
}

// GetEndpointStats fetches ep stats
func (vr *Vrouter) GetEndpointStats() (map[string]*OfnetEndpointStats, error) {
	return vr.svcProxy.GetEndpointStats()
}

func (vr *Vrouter) InspectState() (interface{}, error) {
	vrouterExport := struct {
		PolicyAgent *PolicyAgent // Policy agent
		SvcProxy    interface{}  // Service proxy
		// VlanDb      map[uint16]*Vlan // Database of known vlans
		MyRouterMac net.HardwareAddr // Router Mac to be used
	}{
		vr.policyAgent,
		vr.svcProxy.InspectState(),
		// vr.vlanDb,
		vr.myRouterMac,
	}
	return vrouterExport, nil
}

// initialize Fgraph on the switch
func (self *Vrouter) initFgraph() error {
	sw := self.ofSwitch

	// Create all tables
	self.inputTable = sw.DefaultTable()
	self.vlanTable, _ = sw.NewTable(VLAN_TBL_ID)
	self.ipTable, _ = sw.NewTable(IP_TBL_ID)
	self.hostSNATTable, _ = sw.NewTable(HOST_SNAT_TBL_ID)
	self.hostDNATTable, _ = sw.NewTable(HOST_DNAT_TBL_ID)

	// setup SNAT table
	// Matches in SNAT table (i.e. incoming) go to IP look up
	self.svcProxy.InitSNATTable(IP_TBL_ID)
	self.proxySNATTable = sw.GetTable(SRV_PROXY_SNAT_TBL_ID)
	if self.proxySNATTable == nil {
		log.Fatalf("Error creating service proxy table.")
		return fmt.Errorf("Error creating service proxy table.")
	}

	// Init policy tables
	err := self.policyAgent.InitTables(SRV_PROXY_SNAT_TBL_ID)
	if err != nil {
		log.Fatalf("Error installing policy table. Err: %v", err)
		return err
	}

	// Matches in DNAT go to Policy
	self.svcProxy.InitDNATTable(DST_GRP_TBL_ID)

	//Create all drop entries
	// Drop mcast source mac
	bcastMac, _ := net.ParseMAC("01:00:00:00:00:00")
	bcastSrcFlow, _ := self.inputTable.NewFlow(ofctrl.FlowMatch{
		Priority:  FLOW_MATCH_PRIORITY,
		MacSa:     &bcastMac,
		MacSaMask: &bcastMac,
	})
	bcastSrcFlow.Next(sw.DropAction())

	// redirect dns requests from containers (oui 02:02:xx) to controller
	macSaMask := net.HardwareAddr{0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00}
	macSa := net.HardwareAddr{0x02, 0x02, 0x00, 0x00, 0x00, 0x00}
	dnsRedirectFlow, _ := self.inputTable.NewFlow(ofctrl.FlowMatch{
		Priority:   DNS_FLOW_MATCH_PRIORITY,
		MacSa:      &macSa,
		MacSaMask:  &macSaMask,
		Ethertype:  protocol.IPv4_MSG,
		IpProto:    protocol.Type_UDP,
		UdpDstPort: 53,
	})
	dnsRedirectFlow.Next(sw.SendToController())

	// re-inject dns requests
	dnsReinjectFlow, _ := self.inputTable.NewFlow(ofctrl.FlowMatch{
		Priority:   DNS_FLOW_MATCH_PRIORITY + 1,
		MacSa:      &macSa,
		MacSaMask:  &macSaMask,
		VlanId:     nameServerInternalVlanId,
		Ethertype:  protocol.IPv4_MSG,
		IpProto:    protocol.Type_UDP,
		UdpDstPort: 53,
	})
	dnsReinjectFlow.PopVlan()
	dnsReinjectFlow.Next(self.vlanTable)

	// Redirect ARP packets to controller
	arpFlow, _ := self.inputTable.NewFlow(ofctrl.FlowMatch{
		Priority:  FLOW_MATCH_PRIORITY,
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

	// Packets that miss IP lookup go to hostSNAT
	ipMissFlow, _ := self.ipTable.NewFlow(ofctrl.FlowMatch{
		Priority: FLOW_MISS_PRIORITY,
	})
	ipMissFlow.Next(self.hostSNATTable)

	// Misses in host NAT tables are dropped
	snatMissFlow, _ := self.hostSNATTable.NewFlow(ofctrl.FlowMatch{
		Priority: FLOW_MISS_PRIORITY,
	})
	snatMissFlow.Next(sw.DropAction())
	dnatMissFlow, _ := self.hostDNATTable.NewFlow(ofctrl.FlowMatch{
		Priority: FLOW_MISS_PRIORITY,
	})
	dnatMissFlow.Next(sw.DropAction())

	return nil
}

// Process incoming ARP packets
func (self *Vrouter) processArp(pkt protocol.Ethernet, inPort uint32) {
	log.Debugf("processing ARP packet on port %d", inPort)
	switch t := pkt.Data.(type) {
	case *protocol.ARP:
		log.Debugf("ARP packet: %+v", *t)
		var arpHdr protocol.ARP = *t

		self.agent.incrStats("ArpPktRcvd")

		switch arpHdr.Operation {
		case protocol.Type_Request:
			self.agent.incrStats("ArpReqRcvd")

			var tgtMac net.HardwareAddr
			if inPort != 0 && inPort == self.hostNATInfo.PortNo {
				// respond to all arp requests from the NAT port
				tgtMac = self.myRouterMac
			} else {
				var vlan *uint16
				if vlan = self.agent.getPortVlanMap(inPort); vlan == nil {
					self.agent.incrStats("ArpReqInvalidPortVlan")
					return
				}
				tgtMac = self.myRouterMac
				endpointId := self.agent.getEndpointIdByIpVlan(arpHdr.IPDst, *vlan)
				endpoint := self.agent.getEndpointByID(endpointId)
				if endpoint == nil {
					// Look for a service entry for the target IP
					proxyMac := self.svcProxy.GetSvcProxyMAC(arpHdr.IPDst)
					if proxyMac == "" {
						// If we dont know the IP address, dont send an ARP response
						log.Debugf("Received ARP request for unknown IP: %v", arpHdr.IPDst)
						self.agent.incrStats("ArpReqUnknownDest")
						return
					}

					tgtMac, _ = net.ParseMAC(proxyMac)
				}
			}

			// Form an ARP response
			arpResp, _ := protocol.NewARP(protocol.Type_Reply)
			arpResp.HWSrc = tgtMac
			arpResp.IPSrc = arpHdr.IPDst
			arpResp.HWDst = arpHdr.HWSrc
			arpResp.IPDst = arpHdr.IPSrc

			log.Debugf("Sending ARP response: %+v", arpResp)

			// build the ethernet packet
			ethPkt := protocol.NewEthernet()
			ethPkt.HWDst = arpResp.HWDst
			ethPkt.HWSrc = arpResp.HWSrc
			ethPkt.Ethertype = 0x0806
			ethPkt.Data = arpResp

			log.Debugf("Sending ARP response Ethernet: %+v", ethPkt)

			// Packet out
			pktOut := openflow13.NewPacketOut()
			pktOut.Data = ethPkt
			pktOut.AddAction(openflow13.NewActionOutput(inPort))

			log.Debugf("Sending ARP response packet: %+v", pktOut)

			// Send it out
			self.ofSwitch.Send(pktOut)
			self.agent.incrStats("ArpReqRespSent")

		default:
			log.Debugf("Dropping ARP response packet from port %d", inPort)
			self.agent.incrStats("ArpRespRcvd")
		}
	}
}

func Vrfmetadata(vrfid uint16) (uint64, uint64) {
	metadata := uint64(vrfid) << 32
	metadataMask := uint64(0xFF00000000)
	metadata = metadata & metadataMask

	return metadata, metadataMask
}

//FlushEndpoints flushes endpoints from ovs
func (self *Vrouter) FlushEndpoints(endpointType int) {
}
