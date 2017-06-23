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

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/libOpenflow/openflow13"
	"github.com/contiv/libOpenflow/protocol"
	"github.com/contiv/ofnet/ofctrl"
	cmap "github.com/streamrail/concurrent-map"
)

// Vlrouter state.
// One Vlrouter instance exists on each host
type Vlrouter struct {
	agent       *OfnetAgent      // Pointer back to ofnet agent that owns this
	ofSwitch    *ofctrl.OFSwitch // openflow switch we are talking to
	policyAgent *PolicyAgent     // Policy agent
	svcProxy    *ServiceProxy    // Service proxy

	// Fgraph tables
	inputTable *ofctrl.Table // Packet lookup starts here
	vlanTable  *ofctrl.Table // Vlan Table. map port or VNI to vlan
	ipTable    *ofctrl.Table // IP lookup table

	// Flow Database
	flowDb         map[string]*ofctrl.Flow   // Database of flow entries
	portVlanFlowDb map[uint32]*ofctrl.Flow   // Database of flow entries
	dscpFlowDb     map[uint32][]*ofctrl.Flow // Database of flow entries
	portDnsFlowDb  cmap.ConcurrentMap        // Database of flow entries

	uplinkPortDb  cmap.ConcurrentMap // Database of uplink ports
	myRouterMac   net.HardwareAddr   //Router mac used for external proxy
	anycastMac    net.HardwareAddr   //Anycast mac used for local endpoints
	myBgpPeer     string             // bgp neighbor
	unresolvedEPs cmap.ConcurrentMap // unresolved endpoint map
	uplinkOfp     uint32             // uplink intf portno mapping

}

// GetUplink API gets the uplink port with uplinkID from uplink DB
func (vl *Vlrouter) GetUplink(uplinkID string) *PortInfo {
	uplink, ok := vl.uplinkPortDb.Get(uplinkID)
	if !ok {
		return nil
	}
	return uplink.(*PortInfo)
}

// NewVlrouter creates a new vlrouter instance
func NewVlrouter(agent *OfnetAgent, rpcServ *rpc.Server) *Vlrouter {
	vlrouter := new(Vlrouter)

	// Keep a reference to the agent
	vlrouter.agent = agent

	// Create policy agent
	vlrouter.policyAgent = NewPolicyAgent(agent, rpcServ)
	vlrouter.svcProxy = NewServiceProxy(agent)

	// Create a flow dbs and my router mac
	vlrouter.flowDb = make(map[string]*ofctrl.Flow)
	vlrouter.portVlanFlowDb = make(map[uint32]*ofctrl.Flow)
	vlrouter.dscpFlowDb = make(map[uint32][]*ofctrl.Flow)
	vlrouter.portDnsFlowDb = cmap.New()
	vlrouter.anycastMac, _ = net.ParseMAC("00:00:11:11:11:11")
	vlrouter.unresolvedEPs = cmap.New()

	vlrouter.uplinkPortDb = cmap.New()

	return vlrouter
}

// MasterAdded handles new master added event
func (vl *Vlrouter) MasterAdded(master *OfnetNode) error {

	return nil
}

// Handle switch connected notification
func (vl *Vlrouter) SwitchConnected(sw *ofctrl.OFSwitch) {
	// Keep a reference to the switch
	vl.ofSwitch = sw

	log.Infof("Switch connected(vlrouter). installing flows")

	vl.svcProxy.SwitchConnected(sw)
	// Tell the policy agent about the switch
	vl.policyAgent.SwitchConnected(sw)

	// Init the Fgraph
	vl.initFgraph()
}

// Handle switch disconnected notification
func (vl *Vlrouter) SwitchDisconnected(sw *ofctrl.OFSwitch) {
	vl.policyAgent.SwitchDisconnected(sw)
	vl.ofSwitch = nil

}

// Handle incoming packet
func (vl *Vlrouter) PacketRcvd(sw *ofctrl.OFSwitch, pkt *ofctrl.PacketIn) {
	if pkt.TableId == SRV_PROXY_SNAT_TBL_ID || pkt.TableId == SRV_PROXY_DNAT_TBL_ID {
		// these are destined to service proxy
		vl.svcProxy.HandlePkt(pkt)
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

				vl.processArp(pkt.Data, inPortFld.InPort)
			}
		}

	case protocol.IPv4_MSG:
		var inPort uint32
		if (pkt.TableId == 0) && (pkt.Match.Type == openflow13.MatchType_OXM) &&
			(pkt.Match.Fields[0].Class == openflow13.OXM_CLASS_OPENFLOW_BASIC) &&
			(pkt.Match.Fields[0].Field == openflow13.OXM_FIELD_IN_PORT) {
			// Get the input port number
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
				if pkt.Data.VLANID.VID != 0 {
					vl.agent.incrErrStats("dnsPktUplink")
					return
				}

				if dnsResp, err := processDNSPkt(vl.agent, inPort, udpPkt.Data); err == nil {
					if respPkt, err := buildUDPRespPkt(&pkt.Data, dnsResp); err == nil {
						vl.agent.incrStats("dnsPktReply")
						pktOut := openflow13.NewPacketOut()
						pktOut.Data = respPkt
						pktOut.AddAction(openflow13.NewActionOutput(inPort))
						vl.ofSwitch.Send(pktOut)
						return
					}
				}

				// re-inject DNS packet
				ethPkt := buildDnsForwardPkt(&pkt.Data)
				pktOut := openflow13.NewPacketOut()
				pktOut.Data = ethPkt
				pktOut.InPort = inPort

				pktOut.AddAction(openflow13.NewActionOutput(openflow13.P_TABLE))
				vl.agent.incrStats("dnsPktForward")
				vl.ofSwitch.Send(pktOut)
				return
			}
		}
	default:
		log.Errorf("Received unknown ethertype: %x", pkt.Data.Ethertype)
	}
}

// InjectGARPs not implemented
func (vl *Vlrouter) InjectGARPs(epgID int) {
}

// GlobalConfigUpdate not implemented
func (vl *Vlrouter) GlobalConfigUpdate(cfg OfnetGlobalConfig) error {
	return nil
}

/*AddLocalEndpoint does the following:
1) Adds endpoint to the OVS and the associated flows
2) Populates BGP RIB with local route to be propogated to neighbor
*/

func (vl *Vlrouter) AddLocalEndpoint(endpoint OfnetEndpoint) error {
	log.Infof("Received add Local Endpoint for %v", endpoint)
	if vl.agent.ctrler == nil {
		return nil
	}

	dNATTbl := vl.ofSwitch.GetTable(SRV_PROXY_DNAT_TBL_ID)

	// Install a flow entry for vlan mapping and point it to next table
	portVlanFlow, err := createPortVlanFlow(vl.agent, vl.vlanTable, dNATTbl, &endpoint)
	if err != nil {
		log.Errorf("Error creating portvlan entry. Err: %v", err)
		return err
	}

	// save the flow entry
	vl.portVlanFlowDb[endpoint.PortNo] = portVlanFlow

	// install DSCP flow entries if required
	if endpoint.Dscp != 0 {
		dscpV4Flow, dscpV6Flow, err := createDscpFlow(vl.agent, vl.vlanTable, dNATTbl, &endpoint)
		if err != nil {
			log.Errorf("Error installing DSCP flows. Err: %v", err)
			return err
		}

		// save it for tracking
		vl.dscpFlowDb[endpoint.PortNo] = []*ofctrl.Flow{dscpV4Flow, dscpV6Flow}
	}

	// get output flow
	outPort, err := vl.ofSwitch.OutputPort(endpoint.PortNo)
	if err != nil {
		log.Errorf("Error creating output port %d. Err: %v", endpoint.PortNo, err)
		return err
	}

	// Install the IP address
	ipFlow, err := vl.ipTable.NewFlow(ofctrl.FlowMatch{
		Priority:  LOCAL_ENDPOINT_FLOW_TAGGED_PRIORITY,
		Ethertype: 0x0800,
		VlanId:    endpoint.Vlan,
		IpDa:      &endpoint.IpAddr,
	})
	if err != nil {
		log.Errorf("Error creating flow for endpoint: %+v. Err: %v", endpoint, err)
		return err
	}
	ipFlow.PopVlan()

	ipFlow2, err := vl.ipTable.NewFlow(ofctrl.FlowMatch{
		Priority:  LOCAL_ENDPOINT_FLOW_PRIORITY,
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
	ipFlow.SetMacSa(vl.anycastMac)
	ipFlow2.SetMacDa(destMacAddr)
	ipFlow2.SetMacSa(vl.anycastMac)
	// Point the route at output port
	err = ipFlow2.Next(outPort)
	if err != nil {
		log.Errorf("Error installing IP flow for endpoint: %+v. Err: %v", endpoint, err)
		return err
	}
	// Store the flow
	flowId := vl.agent.getEndpointIdByIpVlan(endpoint.IpAddr, endpoint.Vlan)
	vl.flowDb[flowId+"vlan"] = ipFlow2

	// Point the route at output port
	err = ipFlow.Next(outPort)
	if err != nil {
		log.Errorf("Error installing IP flow for endpoint: %+v. Err: %v", endpoint, err)
		return err
	}

	// Store the flow
	flowId = vl.agent.getEndpointIdByIpVlan(endpoint.IpAddr, endpoint.Vlan)
	vl.flowDb[flowId] = ipFlow

	if !vl.agent.isInternalBgp(&endpoint) {
		// Install dst group entry for the endpoint
		err = vl.policyAgent.AddEndpoint(&endpoint)
		if err != nil {
			log.Errorf("Error adding endpoint to policy agent{%+v}. Err: %v", endpoint, err)
			return err
		}
		path := &OfnetProtoRouteInfo{
			ProtocolType: "bgp",
			localEpIP:    endpoint.IpAddr.String(),
			nextHopIP:    "",
		}
		if vl.agent.GetRouterInfo() != nil {
			path.nextHopIP = vl.agent.GetRouterInfo().RouterIP
		}
		vl.agent.AddLocalProtoRoute([]*OfnetProtoRouteInfo{path})
	}
	if endpoint.Ipv6Addr != nil && endpoint.Ipv6Addr.String() != "" {
		err = vl.AddLocalIpv6Flow(endpoint)
		if err != nil {
			return err
		}
	}
	return nil
}

/* RemoveLocalEndpoint does the following
1) Removes the local endpoint and associated flows from OVS
2) Withdraws the route from BGP RIB
*/
func (vl *Vlrouter) RemoveLocalEndpoint(endpoint OfnetEndpoint) error {

	log.Infof("Received Remove Local Endpoint for endpoint:{%+v}", endpoint)
	// Remove the port vlan flow.
	portVlanFlow := vl.portVlanFlowDb[endpoint.PortNo]
	if portVlanFlow != nil {
		err := portVlanFlow.Delete()
		if err != nil {
			log.Errorf("Error deleting portvlan flow. Err: %v", err)
		}
	}

	// Remove dscp flows.
	dscpFlows, found := vl.dscpFlowDb[endpoint.PortNo]
	if found {
		for _, dflow := range dscpFlows {
			err := dflow.Delete()
			if err != nil {
				log.Errorf("Error deleting dscp flow {%+v}. Err: %v", dflow, err)
			}
		}
	}

	// Find the flow entry
	flowId := endpoint.EndpointID
	ipFlow := vl.flowDb[flowId]
	if ipFlow == nil {
		log.Errorf("Error finding the flow for endpoint: %+v", endpoint)
		return errors.New("Flow not found")
	}

	// Delete the Fgraph entry
	err := ipFlow.Delete()
	if err != nil {
		log.Errorf("Error deleting the endpoint: %+v. Err: %v", endpoint, err)
	}

	flowId = endpoint.EndpointID + "vlan"
	ipFlow = vl.flowDb[flowId]
	if ipFlow == nil {
		log.Errorf("Error finding the tagged flow for endpoint: %+v", endpoint)
		return errors.New("Flow not found")
	}

	// Delete the Fgraph entry
	err = ipFlow.Delete()
	if err != nil {
		log.Errorf("Error deleting the endpoint: %+v. Err: %v", endpoint, err)
	}

	vl.svcProxy.DelEndpoint(&endpoint)

	// Remove the endpoint from policy tables
	if !vl.agent.isInternalBgp(&endpoint) {
		err = vl.policyAgent.DelEndpoint(&endpoint)
		if err != nil {
			log.Errorf("Error deleting endpoint to policy agent{%+v}. Err: %v", endpoint, err)
			return err
		}
	}

	path := &OfnetProtoRouteInfo{
		ProtocolType: "bgp",
		localEpIP:    endpoint.IpAddr.String(),
		nextHopIP:    "",
	}
	if vl.agent.GetRouterInfo() != nil {
		path.nextHopIP = vl.agent.GetRouterInfo().RouterIP
	}
	vl.agent.DeleteLocalProtoRoute([]*OfnetProtoRouteInfo{path})

	if endpoint.Ipv6Addr != nil && endpoint.Ipv6Addr.String() != "" {
		err = vl.RemoveLocalIpv6Flow(endpoint)
		if err != nil {
			return err
		}
	}
	return nil
}

// UpdateLocalEndpoint update local endpoint state
func (vl *Vlrouter) UpdateLocalEndpoint(endpoint *OfnetEndpoint, epInfo EndpointInfo) error {
	oldDscp := endpoint.Dscp

	// Remove existing DSCP flows if required
	if epInfo.Dscp == 0 || epInfo.Dscp != endpoint.Dscp {
		// remove old DSCP flows
		dscpFlows, found := vl.dscpFlowDb[endpoint.PortNo]
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
		dNATTbl := vl.ofSwitch.GetTable(SRV_PROXY_DNAT_TBL_ID)

		// add new dscp flows
		dscpV4Flow, dscpV6Flow, err := createDscpFlow(vl.agent, vl.vlanTable, dNATTbl, endpoint)
		if err != nil {
			log.Errorf("Error installing DSCP flows. Err: %v", err)
			return err
		}

		// save it for tracking
		vl.dscpFlowDb[endpoint.PortNo] = []*ofctrl.Flow{dscpV4Flow, dscpV6Flow}
	}

	return nil
}

// Add IPv6 flows
func (vl *Vlrouter) AddLocalIpv6Flow(endpoint OfnetEndpoint) error {

	outPort, err := vl.ofSwitch.OutputPort(endpoint.PortNo)
	if err != nil {
		log.Errorf("Error creating output port %d. Err: %v", endpoint.PortNo, err)
		return err
	}

	// Install the IPv6 address
	ipv6Flow, err := vl.ipTable.NewFlow(ofctrl.FlowMatch{
		Priority:  FLOW_MATCH_PRIORITY,
		Ethertype: 0x86DD,
		Ipv6Da:    &endpoint.Ipv6Addr,
	})

	if err != nil {
		log.Errorf("Error creating IPv6 flow for endpoint: %+v. Err: %v", endpoint, err)
		return err
	}

	destMacAddr, _ := net.ParseMAC(endpoint.MacAddrStr)

	// Set Mac addresses
	ipv6Flow.SetMacDa(destMacAddr)
	ipv6Flow.SetMacSa(vl.anycastMac)

	// Point the route at output port
	err = ipv6Flow.Next(outPort)
	if err != nil {
		log.Errorf("Error installing IPv6 flow for endpoint: %+v. Err: %v", endpoint, err)
		return err
	}

	// Store the flow
	flowId := vl.agent.getEndpointIdByIpVlan(endpoint.Ipv6Addr, endpoint.Vlan)
	vl.flowDb[flowId] = ipv6Flow

	if !vl.agent.isInternalBgp(&endpoint) {
		// Install dst group entry for IPv6 endpoint
		err = vl.policyAgent.AddIpv6Endpoint(&endpoint)
		if err != nil {
			log.Errorf("Error adding IPv6 endpoint to policy agent{%+v}. Err: %v", endpoint, err)
			return err
		}

		// Add IPv6 route in BGP
		path := &OfnetProtoRouteInfo{
			ProtocolType: "bgp",
			localEpIP:    endpoint.Ipv6Addr.String(),
			nextHopIP:    "",
		}
		if vl.agent.GetRouterInfo() != nil {
			path.nextHopIP = vl.agent.GetRouterInfo().RouterIP
		}
		vl.agent.AddLocalProtoRoute([]*OfnetProtoRouteInfo{path})
	}

	return nil
}

// Remove the IPv6 flow
func (vl *Vlrouter) RemoveLocalIpv6Flow(endpoint OfnetEndpoint) error {

	// Find the IPv6 flow entry
	flowId := vl.agent.getEndpointIdByIpVlan(endpoint.Ipv6Addr, endpoint.Vlan)
	ipv6Flow := vl.flowDb[flowId]
	if ipv6Flow == nil {
		log.Errorf("Error finding the flow for endpoint: %+v", endpoint)
		return errors.New("Flow not found")
	}

	// Delete the Fgraph entry
	err := ipv6Flow.Delete()
	if err != nil {
		log.Errorf("Error deleting IPv6 endpoint: %+v. Err: %v", endpoint, err)
	}

	// Remove the endpoint from policy tables
	if !vl.agent.isInternalBgp(&endpoint) {
		err = vl.policyAgent.DelIpv6Endpoint(&endpoint)
		if err != nil {
			log.Errorf("Error deleting IPv6 endpoint from policy agent{%+v}. Err: %v", endpoint, err)
			return err
		}
	}
	path := &OfnetProtoRouteInfo{
		ProtocolType: "bgp",
		localEpIP:    endpoint.Ipv6Addr.String(),
		nextHopIP:    "",
	}
	if vl.agent.GetRouterInfo() != nil {
		path.nextHopIP = vl.agent.GetRouterInfo().RouterIP
	}
	vl.agent.DeleteLocalProtoRoute([]*OfnetProtoRouteInfo{path})

	return nil
}

// Add a vlan.
// This is mainly used for mapping vlan id to Vxlan VNI
func (vl *Vlrouter) AddVlan(vlanId uint16, vni uint32, vrf string) error {
	log.Infof("Received Add Vlan for vlanid :%d,vni %d", vlanId, vni)

	vrf = "default"
	vl.agent.vlanVrfMutex.Lock()
	vl.agent.vlanVrf[vlanId] = &vrf
	vl.agent.vlanVrfMutex.Unlock()
	vl.agent.createVrf(vrf)
	return nil
}

// Remove a vlan
func (vl *Vlrouter) RemoveVlan(vlanId uint16, vni uint32, vrf string) error {
	// FIXME: Add this for multiple VRF support
	vl.agent.vlanVrfMutex.Lock()
	delete(vl.agent.vlanVrf, vlanId)
	vl.agent.vlanVrfMutex.Unlock()
	vl.agent.deleteVrf(vrf)
	return nil
}

/* AddEndpoint does the following :
1)Adds a remote endpoint and associated flows to OVS
2)The remotes routes can be 3 endpoint types :
  a) internal - json rpc based learning from peer netplugins/ofnetagents in the cluster
	b) external - remote endpoint learn via BGP
	c) external-bgp - endpoint of BGP peer
*/
func (vl *Vlrouter) AddEndpoint(endpoint *OfnetEndpoint) error {

	priority := uint16(FLOW_MATCH_PRIORITY)
	log.Infof("Received AddEndpoint for endpoint: %+v", endpoint)
	if endpoint.Vni != 0 {
		return nil
	}

	flowId := vl.agent.getEndpointIdByIpVlan(endpoint.IpAddr, endpoint.Vlan)

	//nexthopEp := vl.agent.getEndpointByIpVrf(net.ParseIP(vl.agent.GetNeighbor()), "default")
	if vl.agent.isExternal(endpoint) {
		endpoint.Vlan = 0
		priority = EXTERNAL_FLOW_PRIORITY
		flowId = flowId + "external"
	} else {
		//All Contiv endpoints will be stamped with originator host mac
		if endpoint.OriginatorMac != "" {
			endpoint.PortNo = vl.uplinkOfp
			endpoint.MacAddrStr = endpoint.OriginatorMac
		}
		if endpoint.PortNo == 0 {
			if !vl.agent.isExternalBgp(endpoint) {
				//for the remote endpoints maintain a cache of
				//routes that need to be resolved to next hop.
				// bgp peer resolution happens via ARP and hence not
				//maintained in cache.
				log.Infof("Storing endpoint info in cache")
				//vl.unresolvedEPs.Set(endpoint.EndpointID, endpoint.EndpointID)
				return nil
			}
		}
	}
	if vl.agent.isExternalBgp(endpoint) {
		endpoint.Vlan = 0
		if endpoint.PortNo == 0 {
			return nil
		}
		flowId = flowId + "external"
	}

	vrfid := vl.agent.getvrfId(endpoint.Vrf)
	if *vrfid == 0 {
		log.Errorf("Invalid vrf name:%v", endpoint.Vrf)
		return errors.New("Invalid vrf name")
	}

	//set vrf id as METADATA
	//metadata, metadataMask := Vrfmetadata(*vrfid)

	outPort, err := vl.ofSwitch.OutputPort(endpoint.PortNo)
	if err != nil {
		log.Errorf("Error creating output port %d. Err: %v", endpoint.PortNo, err)
		return err
	}

	// Install the IP address
	ipFlow, err := vl.ipTable.NewFlow(ofctrl.FlowMatch{
		Priority:  priority,
		Ethertype: 0x0800,
		IpDa:      &endpoint.IpAddr,
		IpDaMask:  &endpoint.IpMask,
	})
	if err != nil {
		log.Errorf("Error creating flow for endpoint: %+v. Err: %v", endpoint, err)
		return err
	}

	// Set Mac addresses
	if endpoint.Vlan != 0 {
		ipFlow.SetVlan(endpoint.Vlan)
	}

	DAMac, _ := net.ParseMAC(endpoint.MacAddrStr)
	ipFlow.SetMacDa(DAMac)
	ipFlow.SetMacSa(vl.myRouterMac)

	// Point it to output port
	err = ipFlow.Next(outPort)
	if err != nil {
		log.Errorf("Error installing flow for endpoint: %+v. Err: %v", endpoint, err)
		return err
	}

	// Install dst group entry for the endpoint
	if vl.agent.isInternal(endpoint) {
		err = vl.policyAgent.AddEndpoint(endpoint)
		if err != nil {
			log.Errorf("Error adding endpoint to policy agent{%+v}. Err: %v", endpoint, err)
			return err
		}
	}
	// Store it in flow db
	vl.flowDb[flowId] = ipFlow
	if endpoint.Ipv6Addr != nil && endpoint.Ipv6Addr.String() != "" {
		err = vl.AddRemoteIpv6Flow(endpoint)
		if err != nil {
			log.Errorf("Error adding IPv6 flow for remote endpoint {%+v}. Err: %v", endpoint, err)
			return err
		}
	}
	return nil
}

// RemoveEndpoint removes an endpoint from the datapath
func (vl *Vlrouter) RemoveEndpoint(endpoint *OfnetEndpoint) error {

	log.Infof("Received Remove endpoint for endpoint: %+v", endpoint)

	if endpoint.Vni != 0 {
		return nil
	}

	flowId := endpoint.EndpointID

	if vl.agent.isExternalBgp(endpoint) {
		flowId = flowId + "external"
		vl.myBgpPeer = ""
	}

	//Delete the endpoint if it is in the cache
	//if _, ok := vl.unresolvedEPs.Get(endpoint.EndpointID); ok {
	//	vl.unresolvedEPs.Remove(endpoint.EndpointID)
	//	return nil
	//}

	// Find the flow entry
	if vl.agent.isExternal(endpoint) {
		//This scenrio occurs when bgp unsets the external endpointtype
		if _, ok := vl.flowDb[flowId+"external"]; ok {
			flowId = flowId + "external"
		}
	}

	ipFlow := vl.flowDb[flowId]
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
	if vl.agent.isInternal(endpoint) {
		err = vl.policyAgent.DelEndpoint(endpoint)
		if err != nil {
			log.Errorf("Error deleting endpoint to policy agent{%+v}. Err: %v", endpoint, err)
			return err
		}
	}
	if endpoint.Ipv6Addr != nil && endpoint.Ipv6Addr.String() != "" {
		err = vl.RemoveRemoteIpv6Flow(endpoint)
		if err != nil {
			log.Errorf("Error deleting IPv6 endpoint from policy agent{%+v}. Err: %v", endpoint, err)
			return err
		}
	}

	return nil
}

// Add IPv6 flow for the remote endpoint
func (vl *Vlrouter) AddRemoteIpv6Flow(endpoint *OfnetEndpoint) error {
	ipv6EpId := vl.agent.getEndpointIdByIpVlan(endpoint.Ipv6Addr, endpoint.Vlan)

	if vl.agent.isExternal(endpoint) { //nexthopEp != nil && nexthopEp.PortNo != 0 {
		//	endpoint.MacAddrStr = nexthopEp.MacAddrStr
		//	endpoint.PortNo = nexthopEp.PortNo
	} else {
		if endpoint.OriginatorMac != "" {
			endpoint.PortNo = vl.uplinkOfp
			endpoint.MacAddrStr = endpoint.OriginatorMac
		} else {
			endpoint.PortNo = 0
			endpoint.MacAddrStr = " "

			//for the remote endpoints maintain a cache of
			//routes that need to be resolved to next hop.
			// bgp peer resolution happens via ARP and hence not
			//maintainer in cache.
			log.Debugf("Storing endpoint info in cache")
			//vl.unresolvedEPs.Set(ipv6EpId, ipv6EpId)
		}
	}

	if vl.agent.isExternalBgp(endpoint) {
		//vl.myBgpPeer = endpoint.IpAddr.String()
	}

	log.Infof("AddRemoteIpv6Flow for endpoint: %+v", endpoint)

	vrfid := vl.agent.getvrfId(endpoint.Vrf)
	if *vrfid == 0 {
		log.Errorf("Invalid vrf name:%v", endpoint.Vrf)
		return errors.New("Invalid vrf name")
	}

	//set vrf id as METADATA
	//metadata, metadataMask := Vrfmetadata(*vrfid)

	outPort, err := vl.ofSwitch.OutputPort(endpoint.PortNo)
	if err != nil {
		log.Errorf("Error creating output port %d. Err: %v", endpoint.PortNo, err)
		return err
	}

	// Install the IP address
	ipv6Flow, err := vl.ipTable.NewFlow(ofctrl.FlowMatch{
		Priority:   FLOW_MATCH_PRIORITY,
		Ethertype:  0x86DD,
		Ipv6Da:     &endpoint.Ipv6Addr,
		Ipv6DaMask: &endpoint.Ipv6Mask,
	})
	if err != nil {
		log.Errorf("Error creating flow for endpoint: %+v. Err: %v", endpoint, err)
		return err
	}

	// Set Mac addresses
	DAMac, _ := net.ParseMAC(endpoint.MacAddrStr)
	ipv6Flow.SetMacDa(DAMac)
	ipv6Flow.SetMacSa(vl.myRouterMac)

	// Point it to output port
	err = ipv6Flow.Next(outPort)
	if err != nil {
		log.Errorf("Error installing flow for endpoint: %+v. Err: %v", endpoint, err)
		return err
	}

	// Install dst group entry for the endpoint
	if vl.agent.isInternal(endpoint) {
		err = vl.policyAgent.AddIpv6Endpoint(endpoint)
		if err != nil {
			log.Errorf("Error adding IPv6 endpoint to policy agent{%+v}. Err: %v", endpoint, err)
			return err
		}
	}
	// Store it in flow db
	vl.flowDb[ipv6EpId] = ipv6Flow

	return nil
}

// Remove IPv6 flow for the remote endpoint
func (vl *Vlrouter) RemoveRemoteIpv6Flow(endpoint *OfnetEndpoint) error {

	//Delete the endpoint if it is in the cache
	ipv6EpId := vl.agent.getEndpointIdByIpVlan(endpoint.Ipv6Addr, endpoint.Vlan)
	//vl.unresolvedEPs.Remove(ipv6EpId)

	// Find the flow entry
	ipv6Flow := vl.flowDb[ipv6EpId]
	if ipv6Flow == nil {
		log.Errorf("Error finding the flow for endpoint: %+v", endpoint)
		return errors.New("Flow not found")
	}

	// Delete the Fgraph entry
	err := ipv6Flow.Delete()
	if err != nil {
		log.Errorf("Error deleting the endpoint: %+v. Err: %v", endpoint, err)
	}

	//Remove the endpoint from policy tables
	if vl.agent.isInternal(endpoint) {
		err = vl.policyAgent.DelIpv6Endpoint(endpoint)
		if err != nil {
			log.Errorf("Error deleting IPv6 endpoint from policy agent{%+v}. Err: %v", endpoint, err)
			return err
		}
	}

	return nil
}

// initialize Fgraph on the switch
func (vl *Vlrouter) initFgraph() error {
	sw := vl.ofSwitch

	// Create all tables
	vl.inputTable = sw.DefaultTable()
	vl.vlanTable, _ = sw.NewTable(VLAN_TBL_ID)
	vl.ipTable, _ = sw.NewTable(IP_TBL_ID)

	// setup SNAT table
	// Matches in SNAT table (i.e. incoming) go to IP look up
	vl.svcProxy.InitSNATTable(IP_TBL_ID)

	// Init policy tables
	err := vl.policyAgent.InitTables(SRV_PROXY_SNAT_TBL_ID)
	if err != nil {
		log.Fatalf("Error installing policy table. Err: %v", err)
		return err
	}

	// Matches in DNAT go to Policy
	vl.svcProxy.InitDNATTable(DST_GRP_TBL_ID)

	//Create all drop entries
	// Drop mcast source mac
	bcastMac, _ := net.ParseMAC("01:00:00:00:00:00")
	bcastSrcFlow, _ := vl.inputTable.NewFlow(ofctrl.FlowMatch{
		Priority:  FLOW_MATCH_PRIORITY,
		MacSa:     &bcastMac,
		MacSaMask: &bcastMac,
	})
	bcastSrcFlow.Next(sw.DropAction())

	// redirect dns requests from containers (oui 02:02:xx) to controller
	macSaMask := net.HardwareAddr{0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00}
	macSa := net.HardwareAddr{0x02, 0x02, 0x00, 0x00, 0x00, 0x00}
	dnsRedirectFlow, _ := vl.inputTable.NewFlow(ofctrl.FlowMatch{
		Priority:   DNS_FLOW_MATCH_PRIORITY,
		MacSa:      &macSa,
		MacSaMask:  &macSaMask,
		Ethertype:  protocol.IPv4_MSG,
		IpProto:    protocol.Type_UDP,
		UdpDstPort: 53,
	})
	dnsRedirectFlow.Next(sw.SendToController())

	// re-inject dns requests
	dnsReinjectFlow, _ := vl.inputTable.NewFlow(ofctrl.FlowMatch{
		Priority:   DNS_FLOW_MATCH_PRIORITY + 1,
		MacSa:      &macSa,
		MacSaMask:  &macSaMask,
		VlanId:     nameServerInternalVlanId,
		Ethertype:  protocol.IPv4_MSG,
		IpProto:    protocol.Type_UDP,
		UdpDstPort: 53,
	})
	dnsReinjectFlow.PopVlan()
	dnsReinjectFlow.Next(vl.vlanTable)

	// Redirect ARP packets to controller
	arpFlow, _ := vl.inputTable.NewFlow(ofctrl.FlowMatch{
		Priority:  FLOW_MATCH_PRIORITY,
		Ethertype: 0x0806,
	})
	arpFlow.Next(sw.SendToController())

	//All ARP replies will need IP table lookup
	Mac, _ := net.ParseMAC("00:00:11:11:11:11")
	arpFlow, _ = vl.inputTable.NewFlow(ofctrl.FlowMatch{
		Priority:  300,
		Ethertype: 0x0806,
		MacSa:     &Mac,
	})
	arpFlow.Next(vl.ipTable)

	// Send all valid packets to vlan table
	// This is installed at lower priority so that all packets that miss above
	// flows will match entry
	validPktFlow, _ := vl.inputTable.NewFlow(ofctrl.FlowMatch{
		Priority: FLOW_MISS_PRIORITY,
	})
	validPktFlow.Next(vl.vlanTable)

	// Drop all packets that miss Vlan lookup
	vlanMissFlow, _ := vl.vlanTable.NewFlow(ofctrl.FlowMatch{
		Priority: FLOW_MISS_PRIORITY,
	})
	vlanMissFlow.Next(sw.DropAction())

	// Drop all packets that miss IP lookup
	ipMissFlow, _ := vl.ipTable.NewFlow(ofctrl.FlowMatch{
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
func (vl *Vlrouter) processArp(pkt protocol.Ethernet, inPort uint32) {
	log.Infof("processing ARP packet on port %d", inPort)
	switch t := pkt.Data.(type) {
	case *protocol.ARP:
		log.Infof("ARP packet: %+v", *t)
		var arpHdr protocol.ARP = *t
		var srcMac net.HardwareAddr
		var intf *net.Interface
		var err error

		vl.agent.incrStats("ArpPktRcvd")

		switch arpHdr.Operation {
		case protocol.Type_Request:
			vl.agent.incrStats("ArpReqRcvd")

			// Lookup the Dest IP in the endpoint table
			endpoint := vl.agent.getEndpointByIpVrf(arpHdr.IPDst, "default")
			if endpoint == nil {
				// Look for a service entry for the target IP
				proxyMac := vl.svcProxy.GetSvcProxyMAC(arpHdr.IPDst)
				if proxyMac == "" {
					// If we dont know the IP address, dont send an ARP response
					log.Debugf("Received ARP request for unknown IP: %v", arpHdr.IPDst)
					vl.agent.incrStats("ArpReqUnknownDest")
					return
				}
				srcMac, _ = net.ParseMAC(proxyMac)
			} else {
				srcEp := vl.agent.getLocalEndpoint(inPort)
				if vl.agent.isInternal(endpoint) || vl.agent.isInternalBgp(endpoint) {
					if srcEp != nil && vl.agent.isInternal(srcEp) {
						srcMac = vl.anycastMac
					} else {
						if vl.agent.GetRouterInfo() != nil {
							uplink := vl.agent.GetRouterInfo().UplinkPort

							if uplink == nil || len(uplink.MbrLinks) == 0 {
								log.Errorf("Error getting interface information. Err: No member links present")
								return
							}

							intf, err = net.InterfaceByName(uplink.MbrLinks[0].Name)
							if err != nil {
								log.Errorf("Error getting interface information. Err: %+v", err)
								return
							}
						} else if vl.uplinkPortDb.Count() > 0 {
							for ul := range vl.uplinkPortDb.IterBuffered() {
								uplink := ul.Val.(*PortInfo)
								if uplink != nil && len(uplink.MbrLinks) > 0 {
									intf, err = net.InterfaceByName(uplink.MbrLinks[0].Name)
								} else {
									log.Infof("Uplink intf not present. Ignoring Arp")
									return
								}
							}
						} else {
							log.Infof("Uplink intf not present. Ignoring Arp")
							return
						}
						srcMac = intf.HardwareAddr
					}
				} else if vl.agent.isExternal(endpoint) || vl.agent.isExternalBgp(endpoint) {
					if endpoint.PortNo != 0 && srcEp != nil {
						if vl.agent.isInternal(srcEp) || vl.agent.isInternalBgp(srcEp) {
							srcMac = vl.anycastMac
						} else {
							return
						}
					} else if srcEp != nil {
						vl.agent.incrStats("ArpReqUnknownEndpoint")
						vl.sendArpPacketOut(arpHdr.IPSrc, arpHdr.IPDst)
						return
					} else {
						vl.agent.incrStats("ArpReqUnknownEndpoint")
						return
					}
				}
			}

			//Check if source endpoint is learnt.
			endpoint = vl.agent.getEndpointByIpVrf(arpHdr.IPSrc, "default")
			if endpoint != nil && vl.agent.isExternalBgp(endpoint) {
				//endpoint exists from where the arp is received.
				if endpoint.PortNo == 0 {
					log.Infof("Received ARP request from BGP Peer on %d: Mac: %s", endpoint.PortNo, arpHdr.HWSrc.String())
					//learn the mac address and portno for the endpoint
					endpoint.PortNo = inPort
					endpoint.MacAddrStr = arpHdr.HWSrc.String()
					vl.agent.endpointDb.Set(endpoint.EndpointID, endpoint)
					vl.AddEndpoint(endpoint)
					//vl.resolveUnresolvedEPs(endpoint.MacAddrStr, inPort)
					vl.agent.incrStats("ArpReqRcvdFromBgpPeer")
				}
			}

			// Form an ARP response
			arpResp, _ := protocol.NewARP(protocol.Type_Reply)
			arpResp.HWSrc = srcMac
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
			vl.ofSwitch.Send(pktOut)
			vl.agent.incrStats("ArpReqRespSent")

		case protocol.Type_Reply:
			vl.agent.incrStats("ArpRespRcvd")

			endpoint := vl.agent.getEndpointByIpVrf(arpHdr.IPSrc, "default")
			if endpoint != nil && vl.agent.isExternalBgp(endpoint) {
				//endpoint exists from where the arp is received.
				if endpoint.PortNo == 0 {
					log.Infof("Received ARP reply from BGP Peer on %d: Mac: %s", endpoint.PortNo, arpHdr.HWSrc.String())
					//learn the mac address and portno for the endpoint
					endpoint.PortNo = inPort
					endpoint.MacAddrStr = arpHdr.HWSrc.String()
					vl.agent.endpointDb.Set(endpoint.EndpointID, endpoint)
					vl.AddEndpoint(endpoint)
					//vl.resolveUnresolvedEPs(endpoint.MacAddrStr, inPort)
				}
			}

		default:
			log.Debugf("Dropping ARP response packet from port %d", inPort)
		}
	}
}

func (vl *Vlrouter) AddVtepPort(portNo uint32, remoteIp net.IP) error {
	return nil
}

// Remove a VTEP port
func (vl *Vlrouter) RemoveVtepPort(portNo uint32, remoteIp net.IP) error {
	return nil
}

/*resolveUnresolvedEPs walks through the unresolved endpoint list and resolves
over given mac and port*/

func (vl *Vlrouter) resolveUnresolvedEPs(MacAddrStr string, portNo uint32) {

	for id := range vl.unresolvedEPs.IterBuffered() {
		endpointID := id.Val.(string)
		endpoint := vl.agent.getEndpointByID(endpointID)
		endpoint.PortNo = portNo
		endpoint.MacAddrStr = MacAddrStr
		vl.agent.endpointDb.Set(endpoint.EndpointID, endpoint)
		vl.AddEndpoint(endpoint)
		vl.unresolvedEPs.Remove(endpointID)
	}
}

// AddUplink adds an uplink to the switch
func (vl *Vlrouter) AddUplink(uplinkPort *PortInfo) error {
	log.Infof("Adding uplink: %+v", uplinkPort)

	if len(uplinkPort.MbrLinks) == 0 {
		err := fmt.Errorf("Atleast one uplink is needed to be configured for routing mode. Num uplinks configured: %d", len(uplinkPort.MbrLinks))
		return err
	}

	uplinkPort.MbrLinks = uplinkPort.MbrLinks[:1]
	linkInfo := uplinkPort.MbrLinks[0]
	vl.uplinkOfp = linkInfo.OfPort
	dnsUplinkFlow, err := vl.inputTable.NewFlow(ofctrl.FlowMatch{
		Priority:   DNS_FLOW_MATCH_PRIORITY + 2,
		InputPort:  linkInfo.OfPort,
		Ethertype:  protocol.IPv4_MSG,
		IpProto:    protocol.Type_UDP,
		UdpDstPort: 53,
	})
	if err != nil {
		log.Errorf("Error creating nameserver flow entry. Err: %v", err)
		return err
	}
	dnsUplinkFlow.Next(vl.vlanTable)

	// Install a flow entry for vlan mapping and point it to Mac table
	portVlanFlow, err := vl.vlanTable.NewFlow(ofctrl.FlowMatch{
		Priority:  FLOW_MATCH_PRIORITY,
		InputPort: linkInfo.OfPort,
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
	sNATTbl := vl.ofSwitch.GetTable(SRV_PROXY_SNAT_TBL_ID)
	portVlanFlow.Next(sNATTbl)
	if err != nil {
		log.Errorf("Error installing portvlan entry. Err: %v", err)
		return err
	}

	// save the flow entry
	vl.portVlanFlowDb[linkInfo.OfPort] = portVlanFlow
	vl.portDnsFlowDb.Set(fmt.Sprintf("%d", linkInfo.OfPort), dnsUplinkFlow)

	intf, err := net.InterfaceByName(linkInfo.Name)
	if err != nil {
		log.Debugf("Unable to update router mac to uplink mac:err", err)
		return err
	}
	vl.myRouterMac = intf.HardwareAddr

	vl.uplinkPortDb.Set(uplinkPort.Name, uplinkPort)
	return nil
}

// UpdateUplink updates uplink info
func (vl *Vlrouter) UpdateUplink(uplinkName string, updates PortUpdates) error {
	return nil
}

func (vl *Vlrouter) RemoveUplink(uplinkName string) error {
	uplinkPort := vl.GetUplink(uplinkName)

	if uplinkPort == nil {
		err := fmt.Errorf("Could not get uplink with name: %s", uplinkName)
		return err
	}

	for _, link := range uplinkPort.MbrLinks {
		// Uninstall the flow entry
		portVlanFlow := vl.portVlanFlowDb[link.OfPort]
		if portVlanFlow != nil {
			portVlanFlow.Delete()
			delete(vl.portVlanFlowDb, link.OfPort)
		}
		if f, ok := vl.portDnsFlowDb.Get(fmt.Sprintf("%d", link.OfPort)); ok {
			if dnsUplinkFlow, ok := f.(*ofctrl.Flow); ok {
				if err := dnsUplinkFlow.Delete(); err != nil {
					log.Errorf("Error deleting nameserver flow. Err: %v", err)
				}
			}
		}
		vl.portDnsFlowDb.Remove(fmt.Sprintf("%d", link.OfPort))
	}

	vl.uplinkPortDb.Remove(uplinkName)
	vl.myRouterMac = nil
	vl.uplinkOfp = 0
	return nil
}

// AddHostPort is not implemented
func (self *Vlrouter) AddHostPort(hp HostPortInfo) error {
	return nil
}

// RemoveHostPort is not implemented
func (self *Vlrouter) RemoveHostPort(hp uint32) error {
	return nil
}

// AddSvcSpec adds a service spec to proxy
func (vl *Vlrouter) AddSvcSpec(svcName string, spec *ServiceSpec) error {
	return vl.svcProxy.AddSvcSpec(svcName, spec)
}

// DelSvcSpec removes a service spec from proxy
func (vl *Vlrouter) DelSvcSpec(svcName string, spec *ServiceSpec) error {
	return vl.svcProxy.DelSvcSpec(svcName, spec)
}

// SvcProviderUpdate Service Proxy Back End update
func (vl *Vlrouter) SvcProviderUpdate(svcName string, providers []string) {
	vl.svcProxy.ProviderUpdate(svcName, providers)
}

// GetEndpointStats fetches ep stats
func (vl *Vlrouter) GetEndpointStats() (map[string]*OfnetEndpointStats, error) {
	return vl.svcProxy.GetEndpointStats()
}

// MultipartReply handles stats reply
func (vl *Vlrouter) MultipartReply(sw *ofctrl.OFSwitch, reply *openflow13.MultipartReply) {
	if reply.Type == openflow13.MultipartType_Flow {
		vl.svcProxy.FlowStats(reply)
	}
}

// InspectState returns current state
func (vl *Vlrouter) InspectState() (interface{}, error) {
	vlrExport := struct {
		PolicyAgent *PolicyAgent // Policy agent
		SvcProxy    interface{}  // Service proxy
	}{
		vl.policyAgent,
		vl.svcProxy.InspectState(),
	}
	return vlrExport, nil
}

// send proxy arp packet
func (vl *Vlrouter) sendArpPacketOut(srcIP, dstIP net.IP) {
	// routing mode supports 1 uplink with 1 active link
	var uplinkMemberLink *LinkInfo
	for uplinkObj := range vl.uplinkPortDb.IterBuffered() {
		uplink := uplinkObj.Val.(*PortInfo)
		uplinkMemberLink = uplink.getActiveLink()
		break
	}
	if uplinkMemberLink == nil {
		log.Debugf("No active interface on uplink. Not sending ARP for IP:%s \n", dstIP.String())
		return
	}

	ofPortno := uplinkMemberLink.OfPort
	intf, _ := net.InterfaceByName(uplinkMemberLink.Name)

	bMac, _ := net.ParseMAC("FF:FF:FF:FF:FF:FF")
	zeroMac, _ := net.ParseMAC("00:00:00:00:00:00")

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
	vl.agent.ofSwitch.Send(pktOut)
}

//Flushendpoints - flushes out endpoints from ovs
func (vl *Vlrouter) FlushEndpoints(endpointType int) {

	if endpointType == OFNET_EXTERNAL || endpointType == OFNET_EXTERNAL_BGP {
		for id, flow := range vl.flowDb {
			if strings.Contains(id, "external") {
				flow.Delete()
			}
		}
	}
}
