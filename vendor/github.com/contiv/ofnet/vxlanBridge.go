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
	"errors"
	"fmt"
	"net"
	"net/rpc"
	"strings"

	"github.com/contiv/ofnet/ofctrl"
	"github.com/shaleman/libOpenflow/openflow13"
	"github.com/shaleman/libOpenflow/protocol"

	log "github.com/Sirupsen/logrus"
	cmap "github.com/streamrail/concurrent-map"
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
	agent       *OfnetAgent      // Pointer back to ofnet agent that owns this
	ofSwitch    *ofctrl.OFSwitch // openflow switch we are talking to
	policyAgent *PolicyAgent     // Policy agent
	svcProxy    *ServiceProxy    // Service proxy

	vlanDb map[uint16]*Vlan // Database of known vlans

	// Fgraph tables
	inputTable   *ofctrl.Table // Packet lookup starts here
	vlanTable    *ofctrl.Table // Vlan Table. map port or VNI to vlan
	macDestTable *ofctrl.Table // Destination mac lookup

	// Flow Database
	macFlowDb      map[string]*ofctrl.Flow   // Database of flow entries
	portVlanFlowDb map[uint32]*ofctrl.Flow   // Database of flow entries
	portDnsFlowDb  cmap.ConcurrentMap        // Database of flow entries
	dscpFlowDb     map[uint32][]*ofctrl.Flow // Database of flow entries

	// Arp Flow
	arpRedirectFlow *ofctrl.Flow // ARP redirect flow entry
}

// Vlan info
type Vlan struct {
	Vni            uint32                  // Vxlan VNI
	localPortList  map[uint32]*uint32      // List of local ports only
	allPortList    map[uint32]*uint32      // List of local + remote(vtep) ports
	vtepVlanFlowDb map[uint32]*ofctrl.Flow // VTEP vlan mapping flows
	localFlood     *ofctrl.Flood           // local only flood list
	allFlood       *ofctrl.Flood           // local + remote flood list
	localMacMiss   *ofctrl.Flow            // mac lookup miss entry for locally originated traffic
	vtepMacMiss    *ofctrl.Flow            // mac lookup miss for traffic coming from vtep
}

const METADATA_RX_VTEP = 0x1
const VXLAN_GARP_SUPPORTED = false

// Create a new vxlan instance
func NewVxlan(agent *OfnetAgent, rpcServ *rpc.Server) *Vxlan {
	vxlan := new(Vxlan)

	// Keep a reference to the agent
	vxlan.agent = agent

	vxlan.svcProxy = NewServiceProxy(agent)

	// Create policy agent
	vxlan.policyAgent = NewPolicyAgent(agent, rpcServ)

	// init DBs
	vxlan.vlanDb = make(map[uint16]*Vlan)
	vxlan.macFlowDb = make(map[string]*ofctrl.Flow)
	vxlan.portVlanFlowDb = make(map[uint32]*ofctrl.Flow)
	vxlan.portDnsFlowDb = cmap.New()
	vxlan.dscpFlowDb = make(map[uint32][]*ofctrl.Flow)

	return vxlan
}

// Handle new master added event
func (self *Vxlan) MasterAdded(master *OfnetNode) error {

	return nil
}

// Handle switch connected notification
func (self *Vxlan) SwitchConnected(sw *ofctrl.OFSwitch) {
	// Keep a reference to the switch
	self.ofSwitch = sw

	self.svcProxy.SwitchConnected(sw)
	// Tell the policy agent about the switch
	self.policyAgent.SwitchConnected(sw)

	// Init the Fgraph
	self.initFgraph()

	log.Infof("Switch connected(vxlan)")
}

// Handle switch disconnected notification
func (self *Vxlan) SwitchDisconnected(sw *ofctrl.OFSwitch) {

	self.policyAgent.SwitchDisconnected(sw)
	self.ofSwitch = nil

}

// Handle incoming packet
func (self *Vxlan) PacketRcvd(sw *ofctrl.OFSwitch, pkt *ofctrl.PacketIn) {
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
				isVtepPort := self.isVtepPort(inPort)
				if isVtepPort {
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
	}
}

// InjectGARPs not implemented
func (self *Vxlan) InjectGARPs(epgID int) {
}

// Update global config
func (self *Vxlan) GlobalConfigUpdate(cfg OfnetGlobalConfig) error {
	if self.agent.arpMode == cfg.ArpMode {
		log.Warnf("no change in ARP mode %s", self.agent.arpMode)
	} else {
		self.agent.arpMode = cfg.ArpMode
		self.updateArpRedirectFlow(self.agent.arpMode)
	}
	return nil
}

// Add a local endpoint and install associated local route
func (self *Vxlan) AddLocalEndpoint(endpoint OfnetEndpoint) error {
	log.Infof("Adding local endpoint: %+v", endpoint)

	vni := self.agent.getvlanVniMap(endpoint.Vlan)
	if vni == nil {
		log.Errorf("VNI for vlan %d is not known", endpoint.Vlan)
		return errors.New("Unknown Vlan")
	}

	dNATTbl := self.ofSwitch.GetTable(SRV_PROXY_DNAT_TBL_ID)

	// Install a flow entry for vlan mapping and point it to Mac table
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

	// Add the port to local and remote flood list
	output, err := self.ofSwitch.OutputPort(endpoint.PortNo)
	if err != nil {
		return err
	}
	vlan := self.vlanDb[endpoint.Vlan]
	if vlan != nil {
		vlan.localFlood.AddOutput(output)
		vlan.allFlood.AddOutput(output)
	}

	macAddr, err := net.ParseMAC(endpoint.MacAddrStr)
	if err != nil {
		return err
	}

	// Finally install the mac address
	macFlow, err := self.macDestTable.NewFlow(ofctrl.FlowMatch{
		Priority: FLOW_MATCH_PRIORITY,
		VlanId:   endpoint.Vlan,
		MacDa:    &macAddr,
	})
	if err != nil {
		log.Errorf("Error creating mac flow for endpoint %+v. Err: %v", endpoint, err)
		return err
	}

	// Remove vlan tag and point it to local port
	macFlow.PopVlan()
	macFlow.Next(output)

	// Install dst group entry for the endpoint
	err = self.policyAgent.AddEndpoint(&endpoint)
	if err != nil {
		log.Errorf("Error adding endpoint to policy agent{%+v}. Err: %v", endpoint, err)
		return err
	}

	// Install dst group entry for IPv6 endpoint
	if endpoint.Ipv6Addr != nil {
		err = self.policyAgent.AddIpv6Endpoint(&endpoint)
		if err != nil {
			log.Errorf("Error adding IPv6 endpoint to policy agent{%+v}. Err: %v", endpoint, err)
			return err
		}
	}

	// Save the flow in DB
	self.macFlowDb[endpoint.MacAddrStr] = macFlow

	// Send GARP
	err = self.sendGARP(endpoint.IpAddr, macAddr, uint64(endpoint.Vni))
	if err != nil {
		log.Warnf("Error in sending GARP packet for (%s,%s) in vlan %d. Err: %+v",
			endpoint.IpAddr.String(), endpoint.MacAddrStr, endpoint.Vlan, err)
	}

	return nil
}

// Remove local endpoint
func (self *Vxlan) RemoveLocalEndpoint(endpoint OfnetEndpoint) error {
	log.Infof("Received Remove local endpont {%v}", endpoint)
	// Remove the port from flood lists
	var vlanId *uint16

	if vlanId = self.agent.getvniVlanMap(endpoint.Vni); vlanId == nil {
		log.Errorf("Invalid vni to vlan mapping for vni:%v", endpoint.Vni)
		return errors.New("Invalid vni to vlan mapping")
	}

	vlan := self.vlanDb[*vlanId]
	output, err := self.ofSwitch.OutputPort(endpoint.PortNo)
	if err == nil {
		vlan.localFlood.RemoveOutput(output)
		vlan.allFlood.RemoveOutput(output)
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

	// find the flow
	macFlow := self.macFlowDb[endpoint.MacAddrStr]
	if macFlow == nil {
		log.Errorf("Could not find the flow for endpoint: %+v", endpoint)
		return errors.New("Mac flow not found")
	}

	// Delete the flow
	err = macFlow.Delete()
	if err != nil {
		log.Errorf("Error deleting mac flow: %+v. Err: %v", macFlow, err)
	}

	self.svcProxy.DelEndpoint(&endpoint)

	// Remove the endpoint from policy tables
	err = self.policyAgent.DelEndpoint(&endpoint)
	if err != nil {
		log.Errorf("Error deleting endpoint to policy agent{%+v}. Err: %v", endpoint, err)
		return err
	}

	// Remove IPv6 endpoint from policy tables
	if endpoint.Ipv6Addr != nil {
		err = self.policyAgent.DelIpv6Endpoint(&endpoint)
		if err != nil {
			log.Errorf("Error deleting IPv6 endpoint from policy agent{%+v}. Err: %v", endpoint, err)
			return err
		}
	}

	return nil
}

// UpdateLocalEndpoint update local endpoint state
func (self *Vxlan) UpdateLocalEndpoint(endpoint *OfnetEndpoint, epInfo EndpointInfo) error {
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

// Add virtual tunnel end point. This is mainly used for mapping remote vtep IP
// to ofp port number.
func (self *Vxlan) AddVtepPort(portNo uint32, remoteIp net.IP) error {

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

	// Install VNI to vlan mapping for each vni
	sNATTbl := self.ofSwitch.GetTable(SRV_PROXY_SNAT_TBL_ID)
	self.agent.vlanVniMutex.RLock()
	for vni, vlan := range self.agent.vniVlanMap {
		// Install a flow entry for  VNI/vlan and point it to macDest table
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
			log.Errorf("Error adding Flow for VNI %d. Err: %v", vni, err)
			self.agent.vlanVniMutex.RUnlock()
			return err
		}
		portVlanFlow.SetVlan(*vlan)
		// Set the metadata to indicate packet came in from VTEP port

		var vrfid *uint16
		if vrf := self.agent.getvlanVrf(*vlan); vrf != nil {
			vrfid = self.agent.getvrfId(*vrf)
			if vrfid == nil {
				self.agent.vlanVniMutex.RUnlock()
				return fmt.Errorf("Invalid vrf id for vrf:%s", *vrf)
			}
		} else {
			self.agent.vlanVniMutex.RUnlock()
			return fmt.Errorf("Unable to find vrf for vlan %v", *vlan)
		}
		//set vrf id as METADATA
		vrfmetadata, vrfmetadataMask := Vrfmetadata(*vrfid)

		metadata := METADATA_RX_VTEP | vrfmetadata
		metadataMask := METADATA_RX_VTEP | vrfmetadataMask

		portVlanFlow.SetMetadata(metadata, metadataMask)

		// Point to next table
		// Note that we bypass policy lookup on dest host.
		portVlanFlow.Next(sNATTbl)

		// save the port vlan flow for cleaning up later
		self.vlanDb[*vlan].vtepVlanFlowDb[portNo] = portVlanFlow
	}
	self.agent.vlanVniMutex.RUnlock()

	// Walk all vlans and add vtep port to the vlan
	for vlanId, vlan := range self.vlanDb {
		vni := self.agent.getvlanVniMap(vlanId)
		if vni == nil {
			log.Errorf("Can not find vni for vlan: %d", vlanId)
		}
		output, err := self.ofSwitch.OutputPort(portNo)
		if err != nil {
			return err
		}
		vlan.allFlood.AddTunnelOutput(output, uint64(*vni))
	}

	// walk all routes and see if we need to install it
	var ep *OfnetEndpoint
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
func (self *Vxlan) RemoveVtepPort(portNo uint32, remoteIp net.IP) error {
	if f, ok := self.portDnsFlowDb.Get(fmt.Sprintf("%d", portNo)); ok {
		if dnsVtepFlow, ok := f.(*ofctrl.Flow); ok {
			if err := dnsVtepFlow.Delete(); err != nil {
				log.Errorf("Error deleting nameserver flow. Err: %v", err)
			}
		}
	}
	self.portDnsFlowDb.Remove(fmt.Sprintf("%d", portNo))

	// Remove the VTEP from flood lists
	output, _ := self.ofSwitch.OutputPort(portNo)
	for _, vlan := range self.vlanDb {
		// Walk all vlans and remove from flood lists
		vlan.allFlood.RemoveOutput(output)

		portVlanFlow := vlan.vtepVlanFlowDb[portNo]
		portVlanFlow.Delete()
		delete(vlan.vtepVlanFlowDb, portNo)
	}
	return nil
}

// Add a vlan.
func (self *Vxlan) AddVlan(vlanId uint16, vni uint32, vrf string) error {
	var err error
	self.agent.vlanVrfMutex.Lock()
	self.agent.vlanVrf[vlanId] = &vrf
	self.agent.vlanVrfMutex.Unlock()

	self.agent.createVrf(vrf)
	// check if the vlan already exists. if it does, we are done
	if self.vlanDb[vlanId] != nil {
		return nil
	}

	// create new vlan object
	vlan := new(Vlan)
	vlan.Vni = vni
	vlan.localPortList = make(map[uint32]*uint32)
	vlan.allPortList = make(map[uint32]*uint32)
	vlan.vtepVlanFlowDb = make(map[uint32]*ofctrl.Flow)

	// Create flood entries
	vlan.localFlood, err = self.ofSwitch.NewFlood()
	if err != nil {
		return err
	}
	vlan.allFlood, err = self.ofSwitch.NewFlood()
	if err != nil {
		return err
	}

	// Walk all VTEP ports and add vni-vlan mapping for new VNI
	self.agent.vtepTableMutex.RLock()
	for _, vtepPort := range self.agent.vtepTable {
		// Install a flow entry for  VNI/vlan and point it to macDest table
		portVlanFlow, err := self.vlanTable.NewFlow(ofctrl.FlowMatch{
			Priority:  FLOW_MATCH_PRIORITY,
			InputPort: *vtepPort,
			TunnelId:  uint64(vni),
		})
		if err != nil {
			log.Errorf("Error creating port vlan flow for vlan %d. Err: %v", vlanId, err)
			self.agent.vtepTableMutex.RUnlock()
			return err
		}

		// Set vlan id
		portVlanFlow.SetVlan(vlanId)

		// Set the metadata to indicate packet came in from VTEP port
		var vrfid *uint16
		if vrf := self.agent.getvlanVrf(vlanId); vrf != nil {
			vrfid = self.agent.getvrfId(*vrf)
			if vrfid == nil {
				self.agent.vtepTableMutex.RUnlock()
				return fmt.Errorf("Invalid vrf id for vrf:%s", *vrf)
			}
		} else {
			self.agent.vtepTableMutex.RUnlock()
			return fmt.Errorf("Unable to find vrf for vlan %v", *vlan)
		}
		//set vrf id as METADATA
		vrfmetadata, vrfmetadataMask := Vrfmetadata(*vrfid)

		metadata := METADATA_RX_VTEP | vrfmetadata
		metadataMask := METADATA_RX_VTEP | vrfmetadataMask

		portVlanFlow.SetMetadata(metadata, metadataMask)

		// Point to next table
		// Note that we pypass policy lookup on dest host
		portVlanFlow.Next(self.macDestTable)

		// save it in cache
		vlan.vtepVlanFlowDb[*vtepPort] = portVlanFlow
	}

	// Walk all VTEP ports and add it to the allFlood list
	for _, vtepPort := range self.agent.vtepTable {
		output, err := self.ofSwitch.OutputPort(*vtepPort)
		if err != nil {
			self.agent.vtepTableMutex.RUnlock()
			return err
		}
		vlan.allFlood.AddTunnelOutput(output, uint64(vni))
	}
	self.agent.vtepTableMutex.RUnlock()

	log.Infof("Installing vlan flood entry for vlan: %d", vlanId)

	// Install local mac miss entries in macDestTable
	var metadataLclRx uint64 = 0
	var metadataVtepRx uint64 = METADATA_RX_VTEP
	localMacMiss, err := self.macDestTable.NewFlow(ofctrl.FlowMatch{
		Priority:     FLOW_FLOOD_PRIORITY,
		VlanId:       vlanId,
		Metadata:     &metadataLclRx,
		MetadataMask: &metadataVtepRx,
	})
	if err != nil {
		log.Errorf("Error creating local+remote flood. Err: %v", err)
		return err
	}

	localMacMiss.Next(vlan.allFlood)
	vlan.localMacMiss = localMacMiss

	// Setup Mac miss flow for vtep traffic
	vtepMacMiss, err := self.macDestTable.NewFlow(ofctrl.FlowMatch{
		Priority:     FLOW_FLOOD_PRIORITY,
		VlanId:       vlanId,
		Metadata:     &metadataVtepRx,
		MetadataMask: &metadataVtepRx,
	})

	if err != nil {
		log.Errorf("Error creating local flood. Err: %v", err)
		return err
	}

	vtepMacMiss.Next(vlan.localFlood)
	vlan.vtepMacMiss = vtepMacMiss

	// store it in DB
	self.vlanDb[vlanId] = vlan
	self.agent.vrfMutex.Lock()
	self.agent.vlanVrf[vlanId] = &vrf
	self.agent.vrfMutex.Unlock()

	self.agent.createVrf(vrf)
	return nil
}

// Remove a vlan
func (self *Vxlan) RemoveVlan(vlanId uint16, vni uint32, vrf string) error {

	vlan := self.vlanDb[vlanId]
	if vlan == nil {
		log.Fatalf("Could not find the vlan %d", vlanId)
	}

	// Make sure the flood lists are empty
	if vlan.localFlood.NumOutput() != 0 {
		log.Fatalf("VLAN flood list is not empty")
	}

	log.Infof("Deleting vxlan: %d, vlan: %d", vni, vlanId)

	// Uninstall the flood lists
	vlan.allFlood.Delete()
	vlan.localFlood.Delete()

	// Uninstall mac miss Entries
	vlan.localMacMiss.Delete()
	vlan.vtepMacMiss.Delete()

	// uninstall vtep vlan mapping flows
	for _, portVlanFlow := range vlan.vtepVlanFlowDb {
		portVlanFlow.Delete()
	}

	// Remove it from DB
	delete(self.vlanDb, vlanId)
	self.agent.vlanVrfMutex.Lock()
	delete(self.agent.vlanVrf, vlanId)
	self.agent.vlanVrfMutex.Unlock()
	self.agent.deleteVrf(vrf)
	return nil
}

// AddEndpoint Add an endpoint to the datapath
func (self *Vxlan) AddEndpoint(endpoint *OfnetEndpoint) error {
	// Ignore non-vxlan endpoints
	if endpoint.Vni == 0 {
		return nil
	}

	log.Infof("Received endpoint: %+v", endpoint)

	// Lookup the VTEP for the endpoint
	vtepPort := self.agent.getvtepTablePort(endpoint.OriginatorIp.String())
	if vtepPort == nil {
		log.Warnf("Could not find the VTEP for endpoint: %+v", endpoint)

		// Return since remote host is not known.
		// When VTEP gets added, we'll re-install the routes
		return nil
	}

	// map VNI to vlan Id
	vlanId := self.agent.getvniVlanMap(endpoint.Vni)
	if vlanId == nil {
		log.Errorf("Endpoint %+v on unknown VNI: %d", endpoint, endpoint.Vni)
		return errors.New("Unknown VNI")
	}

	macAddr, err := net.ParseMAC(endpoint.MacAddrStr)
	if err != nil {
		return err
	}

	// Install the endpoint in OVS
	// Create an output port for the vtep
	outPort, err := self.ofSwitch.OutputPort(*vtepPort)
	if err != nil {
		log.Errorf("Error creating output port %d. Err: %v", *vtepPort, err)
		return err
	}

	// Finally install the mac address
	macFlow, err := self.macDestTable.NewFlow(ofctrl.FlowMatch{
		Priority: FLOW_MATCH_PRIORITY,
		VlanId:   *vlanId,
		MacDa:    &macAddr,
	})
	if err != nil {
		log.Errorf("Error creating mac flow {%+v}. Err: %v", macFlow, err)
		return err
	}

	macFlow.PopVlan()
	macFlow.SetTunnelId(uint64(endpoint.Vni))
	macFlow.Next(outPort)

	// Install dst group entry for the endpoint
	err = self.policyAgent.AddEndpoint(endpoint)
	if err != nil {
		log.Errorf("Error adding endpoint to policy agent{%+v}. Err: %v", endpoint, err)
		return err
	}

	// Install dst group entry for IPv6 endpoint
	if endpoint.Ipv6Addr != nil {
		err = self.policyAgent.AddIpv6Endpoint(endpoint)
		if err != nil {
			log.Errorf("Error adding IPv6 endpoint to policy agent{%+v}. Err: %v", endpoint, err)
			return err
		}
	}

	// Save the flow in DB
	self.macFlowDb[endpoint.MacAddrStr] = macFlow
	return nil
}

// RemoveEndpoint removes an endpoint from the datapath
func (self *Vxlan) RemoveEndpoint(endpoint *OfnetEndpoint) error {
	// Ignore non-vxlan endpoints
	if endpoint.Vni == 0 {
		return nil
	}

	log.Infof("Received DELETE endpoint: %+v", endpoint)

	// find the flow
	macFlow := self.macFlowDb[endpoint.MacAddrStr]
	if macFlow == nil {
		log.Errorf("Could not find the flow for endpoint: %+v", endpoint)
		return errors.New("Mac flow not found")
	}

	// Delete the flow
	err := macFlow.Delete()
	if err != nil {
		log.Errorf("Error deleting mac flow: %+v. Err: %v", macFlow, err)
	}

	// Remove the endpoint from policy tables
	err = self.policyAgent.DelEndpoint(endpoint)
	if err != nil {
		log.Errorf("Error deleting endpoint to policy agent{%+v}. Err: %v", endpoint, err)
		return err
	}

	// Remove the endpoint from policy tables
	if endpoint.Ipv6Addr != nil {
		err = self.policyAgent.DelIpv6Endpoint(endpoint)
		if err != nil {
			log.Errorf("Error deleting IPv6 endpoint from policy agent{%+v}. Err: %v", endpoint, err)
			return err
		}
	}

	return nil
}

// AddUplink adds an uplink to the switch
func (vx *Vxlan) AddUplink(uplinkPort *PortInfo) error {
	return nil
}

// UpdateUplink updates uplink info
func (vx *Vxlan) UpdateUplink(uplinkName string, updates PortUpdates) error {
	return nil
}

// RemoveUplink remove an uplink to the switch
func (vx *Vxlan) RemoveUplink(uplinkName string) error {
	return nil
}

// AddSvcSpec adds a service spec to proxy
func (vx *Vxlan) AddSvcSpec(svcName string, spec *ServiceSpec) error {
	return vx.svcProxy.AddSvcSpec(svcName, spec)
}

// DelSvcSpec removes a service spec from proxy
func (vx *Vxlan) DelSvcSpec(svcName string, spec *ServiceSpec) error {
	return vx.svcProxy.DelSvcSpec(svcName, spec)
}

// SvcProviderUpdate Service Proxy Back End update
func (vx *Vxlan) SvcProviderUpdate(svcName string, providers []string) {
	vx.svcProxy.ProviderUpdate(svcName, providers)
}

// GetEndpointStats fetches ep stats
func (vx *Vxlan) GetEndpointStats() (map[string]*OfnetEndpointStats, error) {
	return vx.svcProxy.GetEndpointStats()
}

// MultipartReply handles stats reply
func (vx *Vxlan) MultipartReply(sw *ofctrl.OFSwitch, reply *openflow13.MultipartReply) {
	if reply.Type == openflow13.MultipartType_Flow {
		vx.svcProxy.FlowStats(reply)
	}
}

// InspectState returns current state
func (vx *Vxlan) InspectState() (interface{}, error) {
	vxExport := struct {
		PolicyAgent *PolicyAgent // Policy agent
		SvcProxy    interface{}  // Service proxy
		// VlanDb      map[uint16]*Vlan // Database of known vlans
	}{
		vx.policyAgent,
		vx.svcProxy.InspectState(),
		// vr.vlanDb,
	}
	return vxExport, nil
}

// initialize Fgraph on the switch
func (self *Vxlan) initFgraph() error {
	sw := self.ofSwitch

	log.Infof("Installing initial flow entries")

	// Create all tables
	self.inputTable = sw.DefaultTable()
	self.vlanTable, _ = sw.NewTable(VLAN_TBL_ID)
	self.macDestTable, _ = sw.NewTable(MAC_DEST_TBL_ID)

	// setup SNAT table
	// Matches in SNAT table (i.e. incoming) go to mac dest
	self.svcProxy.InitSNATTable(MAC_DEST_TBL_ID)

	// Init policy tables
	err := self.policyAgent.InitTables(SRV_PROXY_SNAT_TBL_ID)
	if err != nil {
		log.Fatalf("Error installing policy table. Err: %v", err)
		return err
	}

	// Next table for DNAT is Policy
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

	// if arp-mode is ArpProxy, redirect ARP packets to controller
	// In ArpFlood mode, ARP packets are flooded in datapath and
	// there is no proxy-arp functionality
	if self.agent.arpMode == ArpProxy {
		self.updateArpRedirectFlow(self.agent.arpMode)
	}

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

	// Drop all packets that miss mac dest lookup AND vlan flood lookup
	floodMissFlow, _ := self.macDestTable.NewFlow(ofctrl.FlowMatch{
		Priority: FLOW_MISS_PRIORITY,
	})
	floodMissFlow.Next(sw.DropAction())

	// Drop all
	return nil
}

// isVtepPort returns true if the port is a vtep port
func (self *Vxlan) isVtepPort(inPort uint32) bool {
	self.agent.vtepTableMutex.RLock()
	defer self.agent.vtepTableMutex.RUnlock()
	for _, vtepPort := range self.agent.vtepTable {
		if *vtepPort == inPort {
			return true
		}
	}

	return false
}

// add a flow to redirect ARP packet to controller for arp-proxy
func (self *Vxlan) updateArpRedirectFlow(newArpMode ArpModeT) {
	sw := self.ofSwitch

	add := (newArpMode == ArpProxy)
	if add {
		// Redirect ARP Request packets to controller
		arpFlow, _ := self.inputTable.NewFlow(ofctrl.FlowMatch{
			Priority:  FLOW_MATCH_PRIORITY,
			Ethertype: 0x0806,
			ArpOper:   protocol.Type_Request,
		})
		arpFlow.Next(sw.SendToController())
		self.arpRedirectFlow = arpFlow
	} else {
		if self.arpRedirectFlow != nil {
			self.arpRedirectFlow.Delete()
		}
	}
}

/*
 * Process incoming ARP packets
 * ARP request handling in various scenarios:
 * Src and Dest EP known:
 *      - Proxy ARP if Dest EP is present locally on the host
 * Src EP known, Dest EP not known:
 *      - ARP Request to a router/VM scenario. Reinject ARP request to VTEPs
 * Src EP not known, Dest EP known:
 *      - Proxy ARP if Dest EP is present locally on the host
 * Src and Dest EP not known:
 *      - Ignore processing the request
 */
func (self *Vxlan) processArp(pkt protocol.Ethernet, inPort uint32) {
	switch t := pkt.Data.(type) {
	case *protocol.ARP:
		log.Debugf("Processing ARP packet on port %d: %+v", inPort, *t)
		var arpIn protocol.ARP = *t

		self.agent.incrStats("ArpPktRcvd")

		switch arpIn.Operation {
		case protocol.Type_Request:
			// If it's a GARP packet, ignore processing
			if arpIn.IPSrc.String() == arpIn.IPDst.String() {
				log.Debugf("Ignoring GARP packet")
				return
			}

			self.agent.incrStats("ArpReqRcvd")

			var vlan uint16
			if self.isVtepPort(inPort) {
				vlan = pkt.VLANID.VID
			} else {
				vlan_ := self.agent.getPortVlanMap(inPort)
				if vlan_ == nil {
					log.Debugf("Invalid port vlan mapping. Ignoring arp packet")
					self.agent.incrStats("ArpReqInvalidPortVlan")
					return
				}
				vlan = *vlan_
			}

			// Lookup the Source and Dest IP in the endpoint table
			srcEp := self.agent.getEndpointByIpVlan(arpIn.IPSrc, vlan)
			dstEp := self.agent.getEndpointByIpVlan(arpIn.IPDst, vlan)

			// No information about the src or dest EP. Ignore processing.
			if srcEp == nil && dstEp == nil {
				log.Debugf("No information on source/destination. Ignoring ARP request.")
				self.agent.incrStats("ArpRequestUnknownSrcDst")

				return
			}

			// Handle packets from vtep ports
			if self.isVtepPort(inPort) {
				if dstEp == nil {
					self.agent.incrStats("ArpReqUnknownDestFromVtep")
					return
				}

				if dstEp.OriginatorIp.String() != self.agent.localIp.String() {
					self.agent.incrStats("ArpReqNonLocalDestFromVtep")
					return
				}
			}

			// If we know the dstEp to be present locally, send the Proxy ARP response
			if dstEp != nil {
				// Container to Container communication. Send proxy ARP response.
				// Unknown node to Container communication
				//   -> Send proxy ARP response only if Endpoint is local.
				//   -> This is to avoid sending ARP responses from ofnet agent on multiple hosts
				if srcEp != nil ||
					(srcEp == nil && dstEp.OriginatorIp.String() == self.agent.localIp.String()) {
					// Form an ARP response
					arpPkt, _ := protocol.NewARP(protocol.Type_Reply)
					arpPkt.HWSrc, _ = net.ParseMAC(dstEp.MacAddrStr)
					arpPkt.IPSrc = arpIn.IPDst
					arpPkt.HWDst = arpIn.HWSrc
					arpPkt.IPDst = arpIn.IPSrc
					log.Debugf("Sending Proxy ARP response: %+v", arpPkt)

					// Build the ethernet packet
					ethPkt := protocol.NewEthernet()
					ethPkt.VLANID.VID = pkt.VLANID.VID
					ethPkt.HWDst = arpPkt.HWDst
					ethPkt.HWSrc = arpPkt.HWSrc
					ethPkt.Ethertype = 0x0806
					ethPkt.Data = arpPkt
					log.Debugf("Sending Proxy ARP response Ethernet: %+v", ethPkt)

					// Construct Packet out
					pktOut := openflow13.NewPacketOut()
					pktOut.Data = ethPkt
					pktOut.AddAction(openflow13.NewActionOutput(inPort))

					// Send the packet out
					self.ofSwitch.Send(pktOut)

					self.agent.incrStats("ArpReqRespSent")

					return
				}
			}

			proxyMac := self.svcProxy.GetSvcProxyMAC(arpIn.IPDst)
			if proxyMac != "" {
				pktOut := getProxyARPResp(&arpIn, proxyMac,
					pkt.VLANID.VID, inPort)
				self.ofSwitch.Send(pktOut)
				self.agent.incrStats("ArpReqRespSent")
				return
			}

			if srcEp != nil && dstEp == nil {
				// If the ARP request was received from VTEP port
				// Ignore processing the packet
				self.agent.vtepTableMutex.RLock()
				for _, vtepPort := range self.agent.vtepTable {
					if *vtepPort == inPort {
						log.Debugf("Received packet from VTEP port. Ignore processing")
						self.agent.incrStats("ArpReqUnknownDestFromVtep")
						self.agent.vtepTableMutex.RUnlock()
						return
					}
				}
				self.agent.vtepTableMutex.RUnlock()

				// ARP request from local container to unknown IP
				// Reinject ARP to VTEP ports
				ethPkt := protocol.NewEthernet()
				ethPkt.HWDst = pkt.HWDst
				ethPkt.HWSrc = pkt.HWSrc
				ethPkt.Ethertype = 0x0806
				ethPkt.Data = &arpIn

				log.Infof("Received ARP request for unknown IP: %v. "+
					"Reinjecting ARP request Ethernet to VTEP ports: %+v", arpIn.IPDst, ethPkt)

				// Packet out
				pktOut := openflow13.NewPacketOut()
				pktOut.InPort = inPort
				pktOut.Data = ethPkt

				tunnelIdField := openflow13.NewTunnelIdField(uint64(srcEp.Vni))
				setTunnelAction := openflow13.NewActionSetField(*tunnelIdField)

				// Add set tunnel action to the instruction
				pktOut.AddAction(setTunnelAction)
				self.agent.vtepTableMutex.RLock()
				for _, vtepPort := range self.agent.vtepTable {
					log.Debugf("Sending to VTEP port: %+v", *vtepPort)
					pktOut.AddAction(openflow13.NewActionOutput(*vtepPort))
				}
				self.agent.vtepTableMutex.RUnlock()
				// Send the packet out
				self.ofSwitch.Send(pktOut)
				self.agent.incrStats("ArpReqReinject")
			}

		case protocol.Type_Reply:
			log.Debugf("Received ARP response packet: %+v from port %d", arpIn, inPort)
			self.agent.incrStats("ArpRespRcvd")

			ethPkt := protocol.NewEthernet()
			ethPkt.VLANID = pkt.VLANID
			ethPkt.HWDst = pkt.HWDst
			ethPkt.HWSrc = pkt.HWSrc
			ethPkt.Ethertype = 0x0806
			ethPkt.Data = &arpIn
			log.Debugf("Sending ARP response Ethernet: %+v", ethPkt)

			// Packet out
			pktOut := openflow13.NewPacketOut()
			pktOut.InPort = inPort
			pktOut.Data = ethPkt
			pktOut.AddAction(openflow13.NewActionOutput(openflow13.P_NORMAL))

			log.Debugf("Reinjecting ARP reply packet: %+v", pktOut)
			// Send it out
			self.ofSwitch.Send(pktOut)
		}
	}
}

// sendGARP sends GARP for the specified IP, MAC
func (self *Vxlan) sendGARP(ip net.IP, mac net.HardwareAddr, vni uint64) error {

	// NOTE: Enable this when EVPN support is added.
	if !VXLAN_GARP_SUPPORTED {
		return nil
	}

	pktOut := BuildGarpPkt(ip, mac, 0)

	tunnelIdField := openflow13.NewTunnelIdField(vni)
	setTunnelAction := openflow13.NewActionSetField(*tunnelIdField)

	// Add set tunnel action to the instruction
	pktOut.AddAction(setTunnelAction)
	self.agent.vtepTableMutex.RLock()
	for _, vtepPort := range self.agent.vtepTable {
		log.Debugf("Sending to Vtep port: %+v", *vtepPort)
		pktOut.AddAction(openflow13.NewActionOutput(*vtepPort))
	}
	self.agent.vtepTableMutex.RUnlock()
	// Send it out
	self.ofSwitch.Send(pktOut)
	self.agent.incrStats("GarpPktSent")

	return nil
}
