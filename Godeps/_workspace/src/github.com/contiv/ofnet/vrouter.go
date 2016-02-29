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

	"github.com/contiv/ofnet/ofctrl"
	"github.com/shaleman/libOpenflow/openflow13"
	"github.com/shaleman/libOpenflow/protocol"

	log "github.com/Sirupsen/logrus"
)

// Vrouter state.
// One Vrouter instance exists on each host
type Vrouter struct {
	agent       *OfnetAgent      // Pointer back to ofnet agent that owns this
	ofSwitch    *ofctrl.OFSwitch // openflow switch we are talking to
	policyAgent *PolicyAgent     // Policy agent
	svcProxy    *ServiceProxy    // Service proxy

	// Fgraph tables
	inputTable *ofctrl.Table // Packet lookup starts here
	vlanTable  *ofctrl.Table // Vlan Table. map port or VNI to vlan
	ipTable    *ofctrl.Table // IP lookup table

	// Flow Database
	flowDb         map[string]*ofctrl.Flow // Database of flow entries
	portVlanFlowDb map[uint32]*ofctrl.Flow // Database of flow entries
	vlanDb         map[uint16]*Vlan        // Database of known vlans

	myRouterMac net.HardwareAddr // Router Mac to be used
}

// Create a new vrouter instance
func NewVrouter(agent *OfnetAgent, rpcServ *rpc.Server) *Vrouter {
	vrouter := new(Vrouter)

	// Keep a reference to the agent
	vrouter.agent = agent

	// Create policy agent
	vrouter.policyAgent = NewPolicyAgent(agent, rpcServ)
	vrouter.svcProxy = NewServiceProxy()

	// Create a flow dbs and my router mac
	vrouter.flowDb = make(map[string]*ofctrl.Flow)
	vrouter.portVlanFlowDb = make(map[uint32]*ofctrl.Flow)
	vrouter.myRouterMac, _ = net.ParseMAC("00:00:11:11:11:11")
	vrouter.vlanDb = make(map[uint16]*Vlan)

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
	// FIXME: ??
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

	case 0x0800:
		// FIXME: We dont expect IP packets. Use this for statefull policies.
	default:
		log.Errorf("Received unknown ethertype: %x", pkt.Data.Ethertype)
	}
}

// Add a local endpoint and install associated local route
func (self *Vrouter) AddLocalEndpoint(endpoint OfnetEndpoint) error {

	vni := self.agent.vlanVniMap[endpoint.Vlan]
	if vni == nil {
		log.Errorf("VNI for vlan %d is not known", endpoint.Vlan)
		return errors.New("Unknown Vlan")
	}

	// Install a flow entry for vlan mapping and point it to IP table

	portVlanFlow, err := self.vlanTable.NewFlow(ofctrl.FlowMatch{
		Priority:  FLOW_MATCH_PRIORITY,
		InputPort: endpoint.PortNo,
	})

	if err != nil {
		log.Errorf("Error creating portvlan entry. Err: %v", err)
		return err
	}

	vrfid := self.agent.vrfNameIdMap[endpoint.Vrf]

	if *vrfid == 0 {
		log.Errorf("Invalid vrf name:%v", endpoint.Vrf)
		return errors.New("Invalid vrf name")
	}
	//set vrf id as METADATA
	metadata, metadataMask := Vrfmetadata(*vrfid)

	// Set source endpoint group if specified
	if endpoint.EndpointGroup != 0 {
		srcMetadata, srcMetadataMask := SrcGroupMetadata(endpoint.EndpointGroup)
		metadata = metadata | srcMetadata
		metadataMask = metadataMask | srcMetadataMask
	}
	portVlanFlow.SetMetadata(metadata, metadataMask)

	// Point it to dnat table
	dNATTbl := self.ofSwitch.GetTable(SRV_PROXY_DNAT_TBL_ID)
	err = portVlanFlow.Next(dNATTbl)
	if err != nil {
		log.Errorf("Error installing portvlan entry. Err: %v", err)
		return err
	}

	// save the flow entry
	self.portVlanFlowDb[endpoint.PortNo] = portVlanFlow

	// Create the output port
	outPort, err := self.ofSwitch.OutputPort(endpoint.PortNo)
	if err != nil {
		log.Errorf("Error creating output port %d. Err: %v", endpoint.PortNo, err)
		return err
	}

	//Ip table look up will be vrf,ip
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

	return nil
}

// Remove local endpoint
func (self *Vrouter) RemoveLocalEndpoint(endpoint OfnetEndpoint) error {

	// Remove the port vlan flow.
	portVlanFlow := self.portVlanFlowDb[endpoint.PortNo]
	if portVlanFlow != nil {
		err := portVlanFlow.Delete()
		if err != nil {
			log.Errorf("Error deleting portvlan flow. Err: %v", err)
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

	return nil
}

// Add virtual tunnel end point. This is mainly used for mapping remote vtep IP
// to ofp port number.
func (self *Vrouter) AddVtepPort(portNo uint32, remoteIp net.IP) error {
	// Install VNI to vlan mapping for each vni
	log.Infof("Adding VTEP for portno %v , remote IP : %v", portNo, remoteIp)
	for vni, vlan := range self.agent.vniVlanMap {
		// Install a flow entry for  VNI/vlan and point it to Ip table
		portVlanFlow, err := self.vlanTable.NewFlow(ofctrl.FlowMatch{
			Priority:  FLOW_MATCH_PRIORITY,
			InputPort: portNo,
			TunnelId:  uint64(vni),
		})

		if err != nil && strings.Contains(err.Error(), "Flow already exists") {
			log.Infof("VTEP %s already exists", remoteIp.String())
			return nil
		} else if err != nil {
			log.Errorf("Error adding Flow for VTEP %v. Err: %v", remoteIp, err)
			return err
		}

		// Point it to next table.
		// Note that we bypass policy lookup on dest host.
		portVlanFlow.Next(self.ipTable)
		// Set the metadata to indicate packet came in from VTEP port

		vrf := self.agent.vlanVrf[*vlan]
		if vrf == nil {
			log.Errorf("Invalid vlan to Vrf mapping for %v", *vlan)
			return errors.New("Invalid vlan to Vrf mapping")
		}
		vrfid := self.agent.vrfNameIdMap[*vrf]
		if *vrfid == 0 {
			log.Errorf("Invalid vrf name:%v", *vrf)
			return errors.New("Invalid vrf name")
		}

		// Point it to next table.
		// Note that we bypass policy lookup on dest host.
		sNATTbl := self.ofSwitch.GetTable(SRV_PROXY_SNAT_TBL_ID)
		portVlanFlow.Next(sNATTbl)
		//set vrf id as METADATA
		vrfmetadata, vrfmetadataMask := Vrfmetadata(*vrfid)

		metadata := METADATA_RX_VTEP | vrfmetadata
		metadataMask := METADATA_RX_VTEP | vrfmetadataMask

		portVlanFlow.SetMetadata(metadata, metadataMask)
	}
	// walk all routes and see if we need to install it
	for _, endpoint := range self.agent.endpointDb {
		if endpoint.OriginatorIp.String() == remoteIp.String() {
			err := self.AddEndpoint(endpoint)
			if err != nil {
				log.Errorf("Error installing endpoint during vtep add(%v) EP: %+v. Err: %v", remoteIp, endpoint, err)
				return err
			}
		}
	}
	return nil
}

// Remove a VTEP port
func (self *Vrouter) RemoveVtepPort(portNo uint32, remoteIp net.IP) error {
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
	self.agent.vlanVrf[vlanId] = &vrf

	// Walk all VTEP ports and add vni-vlan mapping for new VNI
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

		// Set the metadata to indicate packet came in from VTEP port
		vrf := self.agent.vlanVrf[vlanId]
		if vrf == nil {
			log.Errorf("Invalid vlan to Vrf mapping for %v", *vlan)
			return errors.New("Invalid vlan to Vrf mapping")
		}

		vrfid := self.agent.vrfNameIdMap[*vrf]

		if *vrfid == 0 {
			log.Errorf("Invalid vrf name:%v", *vrf)
			return errors.New("Invalid vrf name")
		}
		//set vrf id as METADATA
		vrfmetadata, vrfmetadataMask := Vrfmetadata(*vrfid)

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
	delete(self.agent.vlanVrf, vlanId)
	return nil
}

// AddEndpoint Add an endpoint to the datapath
func (self *Vrouter) AddEndpoint(endpoint *OfnetEndpoint) error {

	if endpoint.Vni == 0 {
		return nil
	}

	// Lookup the VTEP for the endpoint
	vtepPort := self.agent.vtepTable[endpoint.OriginatorIp.String()]
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

	vrfid := self.agent.vrfNameIdMap[endpoint.Vrf]
	if *vrfid == 0 {
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

	return nil
}

// AddUplink adds an uplink to the switch
func (vr *Vrouter) AddUplink(portNo uint32) error {
	// Nothing to do
	return nil
}

// RemoveUplink remove an uplink to the switch
func (vr *Vrouter) RemoveUplink(portNo uint32) error {
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

// initialize Fgraph on the switch
func (self *Vrouter) initFgraph() error {
	sw := self.ofSwitch

	// Create all tables
	self.inputTable = sw.DefaultTable()
	self.vlanTable, _ = sw.NewTable(VLAN_TBL_ID)
	self.ipTable, _ = sw.NewTable(IP_TBL_ID)

	// setup SNAT table
	// Matches in SNAT table (i.e. incoming) go to IP look up
	self.svcProxy.InitSNATTable(IP_TBL_ID)

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
			// Lookup the Dest IP in the endpoint table
			vlan := self.agent.portVlanMap[inPort]
			endpointId := self.agent.getEndpointIdByIpVlan(arpHdr.IPDst, *vlan)
			endpoint := self.agent.endpointDb[endpointId]
			if endpoint == nil {
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

func Vrfmetadata(vrfid uint16) (uint64, uint64) {
	metadata := uint64(vrfid) << 32
	metadataMask := uint64(0xFF00000000)
	metadata = metadata & metadataMask

	return metadata, metadataMask
}
