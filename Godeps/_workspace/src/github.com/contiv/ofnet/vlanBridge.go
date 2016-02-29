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

// This file implements the vlan bridging datapath

import (
	"net"
	"net/rpc"
        "fmt"
	"github.com/contiv/ofnet/ofctrl"
	"github.com/shaleman/libOpenflow/openflow13"
	"github.com/shaleman/libOpenflow/protocol"

	log "github.com/Sirupsen/logrus"
)

// Vlan bridging currently uses native OVS bridging.
// This is mostly stub code.

// VlanBridge has Vlan state.
type VlanBridge struct {
	agent       *OfnetAgent      // Pointer back to ofnet agent that owns this
	ofSwitch    *ofctrl.OFSwitch // openflow switch we are talking to
	policyAgent *PolicyAgent     // Policy agent

	// Fgraph tables
	inputTable *ofctrl.Table // Packet lookup starts here
	vlanTable  *ofctrl.Table // Vlan Table. map port or VNI to vlan
	nmlTable   *ofctrl.Table // OVS normal lookup table

	portVlanFlowDb map[uint32]*ofctrl.Flow // Database of flow entries
	uplinkDb       map[uint32]uint32       // Database of uplink ports
}

// NewVlanBridge Create a new vlan instance
func NewVlanBridge(agent *OfnetAgent, rpcServ *rpc.Server) *VlanBridge {
	vlan := new(VlanBridge)

	// Keep a reference to the agent
	vlan.agent = agent

	// init maps
	vlan.portVlanFlowDb = make(map[uint32]*ofctrl.Flow)
	vlan.uplinkDb = make(map[uint32]uint32)

	// Create policy agent
	vlan.policyAgent = NewPolicyAgent(agent, rpcServ)

	return vlan
}

// MasterAdded Handle new master added event
func (vl *VlanBridge) MasterAdded(master *OfnetNode) error {

	return nil
}

// SwitchConnected Handle switch connected notification
func (vl *VlanBridge) SwitchConnected(sw *ofctrl.OFSwitch) {
	// Keep a reference to the switch
	vl.ofSwitch = sw

	// Tell the policy agent about the switch
	vl.policyAgent.SwitchConnected(sw)

	// Init the Fgraph
	vl.initFgraph()

	log.Infof("Switch connected(vlan)")
}

// SwitchDisconnected Handle switch disconnected notification
func (vl *VlanBridge) SwitchDisconnected(sw *ofctrl.OFSwitch) {
	// FIXME: ??
}

// PacketRcvd Handle incoming packet
func (vl *VlanBridge) PacketRcvd(sw *ofctrl.OFSwitch, pkt *ofctrl.PacketIn) {
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
	}
}

// AddLocalEndpoint Add a local endpoint and install associated local route
func (vl *VlanBridge) AddLocalEndpoint(endpoint OfnetEndpoint) error {
	log.Infof("Adding local endpoint: %+v", endpoint)

	// Install a flow entry for vlan mapping and point it to Mac table
	portVlanFlow, err := vl.vlanTable.NewFlow(ofctrl.FlowMatch{
		Priority:  FLOW_MATCH_PRIORITY,
		InputPort: endpoint.PortNo,
	})
	if err != nil {
		log.Errorf("Error creating portvlan entry. Err: %v", err)
		return err
	}

	vrfid := vl.agent.vrfNameIdMap[endpoint.Vrf]
	//set vrf id as METADATA
	metadata, metadataMask := Vrfmetadata(*vrfid)

	if endpoint.EndpointGroup != 0 {
		srcMetadata, srcMetadataMask := SrcGroupMetadata(endpoint.EndpointGroup)
		metadata = metadata | srcMetadata
		metadataMask = metadataMask | srcMetadataMask

	}
	portVlanFlow.SetMetadata(metadata, metadataMask)

	// Set the vlan and install it
	// FIXME: portVlanFlow.SetVlan(endpoint.Vlan)
	dstGrpTbl := vl.ofSwitch.GetTable(DST_GRP_TBL_ID)
	err = portVlanFlow.Next(dstGrpTbl)
	if err != nil {
		log.Errorf("Error installing portvlan entry. Err: %v", err)
		return err
	}

	// save the flow entry
	vl.portVlanFlowDb[endpoint.PortNo] = portVlanFlow

	// Install dst group entry for the endpoint
	err = vl.policyAgent.AddEndpoint(&endpoint)
	if err != nil {
		log.Errorf("Error adding endpoint to policy agent{%+v}. Err: %v", endpoint, err)
		return err
	}

	// Send GARP
	mac, _ := net.ParseMAC(endpoint.MacAddrStr)
	err = vl.sendGARP(endpoint.IpAddr, mac, endpoint.Vlan)
	if err != nil {
		log.Warnf("Error in sending GARP packet for (%s,%s) in vlan %d. Err: %+v",
			endpoint.IpAddr.String(), endpoint.MacAddrStr, endpoint.Vlan, err)
	}

	return nil
}

// RemoveLocalEndpoint Remove local endpoint
func (vl *VlanBridge) RemoveLocalEndpoint(endpoint OfnetEndpoint) error {
	// Remove the port vlan flow.
	portVlanFlow := vl.portVlanFlowDb[endpoint.PortNo]
	if portVlanFlow != nil {
		err := portVlanFlow.Delete()
		if err != nil {
			log.Errorf("Error deleting portvlan flow. Err: %v", err)
		}
	}

	// Remove the endpoint from policy tables
	err := vl.policyAgent.DelEndpoint(&endpoint)
	if err != nil {
		log.Errorf("Error deleting endpoint to policy agent{%+v}. Err: %v", endpoint, err)
		return err
	}

	return nil
}

// AddVtepPort Add virtual tunnel end point.
func (vl *VlanBridge) AddVtepPort(portNo uint32, remoteIP net.IP) error {
	return nil
}

// RemoveVtepPort Remove a VTEP port
func (vl *VlanBridge) RemoveVtepPort(portNo uint32, remoteIP net.IP) error {
	return nil
}

// AddVlan Add a vlan.
func (vl *VlanBridge) AddVlan(vlanID uint16, vni uint32, vrf string) error {
	vl.agent.vlanVrf[vlanID] = &vrf
	vl.agent.createVrf(vrf)
	return nil
}

// RemoveVlan Remove a vlan
func (vl *VlanBridge) RemoveVlan(vlanID uint16, vni uint32, vrf string) error {
	delete(vl.agent.vlanVrf, vlanID)
	vl.agent.deleteVrf(vrf)
	return nil
}

// AddEndpoint Add an endpoint to the datapath
func (vl *VlanBridge) AddEndpoint(endpoint *OfnetEndpoint) error {

	if endpoint.Vni != 0 {
		return nil
	}

	log.Infof("Received endpoint: %+v", endpoint)

	// Install dst group entry for the endpoint
	err := vl.policyAgent.AddEndpoint(endpoint)
	if err != nil {
		log.Errorf("Error adding endpoint to policy agent{%+v}. Err: %v", endpoint, err)
		return err
	}

	return nil
}

// RemoveEndpoint removes an endpoint from the datapath
func (vl *VlanBridge) RemoveEndpoint(endpoint *OfnetEndpoint) error {
	log.Infof("Received DELETE endpoint: %+v", endpoint)

	if endpoint.Vni != 0 {
		return nil
	}

	// Remove the endpoint from policy tables
	err := vl.policyAgent.DelEndpoint(endpoint)
	if err != nil {
		log.Errorf("Error deleting endpoint to policy agent{%+v}. Err: %v", endpoint, err)
		return err
	}

	return nil
}

// AddUplink adds an uplink to the switch
func (vl *VlanBridge) AddUplink(portNo uint32) error {
	log.Infof("Adding uplink port: %+v", portNo)

	// Install a flow entry for vlan mapping and point it to Mac table
	portVlanFlow, err := vl.vlanTable.NewFlow(ofctrl.FlowMatch{
		Priority:  FLOW_MATCH_PRIORITY,
		InputPort: portNo,
	})
	if err != nil {
		log.Errorf("Error creating portvlan entry. Err: %v", err)
		return err
	}

	// Packets coming from uplink go thru normal lookup(bypass policy)
	err = portVlanFlow.Next(vl.ofSwitch.NormalLookup())
	if err != nil {
		log.Errorf("Error installing portvlan entry. Err: %v", err)
		return err
	}

	// save the flow entry
	vl.portVlanFlowDb[portNo] = portVlanFlow
	vl.uplinkDb[portNo] = portNo

	return nil
}

// RemoveUplink remove an uplink to the switch
func (vl *VlanBridge) RemoveUplink(portNo uint32) error {
	delete(vl.uplinkDb, portNo)
	return nil
}

// AddSvcSpec adds a service spec to proxy
func (vl *VlanBridge) AddSvcSpec(svcName string, spec *ServiceSpec) error {
	return nil
}

// DelSvcSpec removes a service spec from proxy
func (vl *VlanBridge) DelSvcSpec(svcName string, spec *ServiceSpec) error {
	return nil
}

// SvcProviderUpdate Service Proxy Back End update
func (vl *VlanBridge) SvcProviderUpdate(svcName string, providers []string) {
}

// initialize Fgraph on the switch
func (vl *VlanBridge) initFgraph() error {
	sw := vl.ofSwitch

	log.Infof("Installing initial flow entries")

	// Create all tables
	vl.inputTable = sw.DefaultTable()
	vl.vlanTable, _ = sw.NewTable(VLAN_TBL_ID)
	vl.nmlTable, _ = sw.NewTable(MAC_DEST_TBL_ID)

	// Init policy tables
	err := vl.policyAgent.InitTables(MAC_DEST_TBL_ID)
	if err != nil {
		log.Fatalf("Error installing policy table. Err: %v", err)
		return err
	}

	// Send all packets to vlan lookup
	validPktFlow, _ := vl.inputTable.NewFlow(ofctrl.FlowMatch{
		Priority: FLOW_MISS_PRIORITY,
	})
	validPktFlow.Next(vl.vlanTable)

	// Drop all packets that miss Vlan lookup
	vlanMissFlow, _ := vl.vlanTable.NewFlow(ofctrl.FlowMatch{
		Priority: FLOW_MISS_PRIORITY,
	})
	vlanMissFlow.Next(sw.DropAction())

	// Redirect ARP Request packets to controller
	arpFlow, _ := vl.inputTable.NewFlow(ofctrl.FlowMatch{
		Priority:  FLOW_MATCH_PRIORITY,
		Ethertype: 0x0806,
		ArpOper:   protocol.Type_Request,
	})
	arpFlow.Next(sw.SendToController())

	// All packets that have gone thru policy lookup go thru normal OVS switching
	normalLookupFlow, _ := vl.nmlTable.NewFlow(ofctrl.FlowMatch{
		Priority: FLOW_MISS_PRIORITY,
	})
	normalLookupFlow.Next(sw.NormalLookup())

	// Drop all
	return nil
}

/*
 * Process incoming ARP packets
 * ARP request handling in various scenarios:
 * Src and Dest EP known:
 *      - Proxy ARP if Dest EP is present locally on the host
 * Src EP known, Dest EP not known:
 *      - ARP Request to a router/VM scenario. Reinject ARP request to uplinks
 * Src EP not known, Dest EP known:
 *      - Proxy ARP if Dest EP is present locally on the host
 * Src and Dest EP not known:
 *      - Ignore processing the request
 */
func (vl *VlanBridge) processArp(pkt protocol.Ethernet, inPort uint32) {
	switch t := pkt.Data.(type) {
	case *protocol.ARP:
		log.Debugf("Processing ARP packet on port %d: %+v", inPort, *t)
		var arpIn protocol.ARP = *t
             
		switch arpIn.Operation {
		case protocol.Type_Request:
			// If it's a GARP packet, ignore processing
			if arpIn.IPSrc.String() == arpIn.IPDst.String() {
				log.Debugf("Ignoring GARP packet")
				return
			}

			// Lookup the Source and Dest IP in the endpoint table
			//Vrf derivation logic :
                        var vlan uint16
			if vl.uplinkDb[inPort] != 0 {
				//arp packet came in from uplink hence tagged
				fmt.Println("the vlan id is ", pkt.VLANID.VID)
				vlan = pkt.VLANID.VID
			} else {
				//arp packet came from local endpoints - derive vrf from inport
                                if vl.agent.portVlanMap[inPort] != nil {
				   vlan = *(vl.agent.portVlanMap[inPort])
                                }else {
                                   log.Debugf("Invalid port vlan mapping. Ignoring arp packet")
                                   return
                                } 
			}
			srcEp := vl.agent.getEndpointByIpVlan(arpIn.IPSrc, vlan)
			dstEp := vl.agent.getEndpointByIpVlan(arpIn.IPDst, vlan)

			fmt.Println("The src and des ep are", srcEp, dstEp)

			// No information about the src or dest EP. Ignore processing.
			if srcEp == nil && dstEp == nil {
				log.Debugf("No information on source/destination. Ignoring ARP request.")
				return
			}
			// If we know the dstEp to be present locally, send the Proxy ARP response
			if dstEp != nil {
				// Container to Container communication. Send proxy ARP response.
				// Unknown node to Container communication
				//   -> Send proxy ARP response only if Endpoint is local.
				//   -> This is to avoid sending ARP responses from ofnet agent on multiple hosts
				if srcEp != nil ||
					(srcEp == nil && dstEp.OriginatorIp.String() == vl.agent.localIp.String()) {
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
					vl.ofSwitch.Send(pktOut)

					return
				}
			}
			if srcEp != nil && dstEp == nil {
				// If the ARP request was received from uplink
				// Ignore processing the packet
				for _, portNo := range vl.uplinkDb {
					if portNo == inPort {
						log.Debugf("Ignore processing ARP packet from uplink")
						return
					}
				}

				// ARP request from local container to unknown IP
				// Reinject ARP to uplinks
				ethPkt := protocol.NewEthernet()
				ethPkt.VLANID.VID = srcEp.Vlan
				ethPkt.HWDst = pkt.HWDst
				ethPkt.HWSrc = pkt.HWSrc
				ethPkt.Ethertype = 0x0806
				ethPkt.Data = &arpIn

				log.Infof("Received ARP request for unknown IP: %v. "+
					"Reinjecting ARP request Ethernet to uplinks: %+v", arpIn.IPDst, ethPkt)

				// Packet out
				pktOut := openflow13.NewPacketOut()
				pktOut.InPort = inPort
				pktOut.Data = ethPkt
				for _, portNo := range vl.uplinkDb {
					log.Debugf("Sending to uplink: %+v", portNo)
					pktOut.AddAction(openflow13.NewActionOutput(portNo))
				}

				// Send the packet out
				vl.ofSwitch.Send(pktOut)
			}

		case protocol.Type_Reply:
			log.Debugf("Received ARP response packet: %+v from port %d", arpIn, inPort)

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
			vl.ofSwitch.Send(pktOut)
		}
	}
}

// sendGARP sends GARP for the specified IP, MAC
func (vl *VlanBridge) sendGARP(ip net.IP, mac net.HardwareAddr, vlanID uint16) error {
    pktOut := BuildGarpPkt(ip, mac, vlanID)

	for _, portNo := range vl.uplinkDb {
		log.Debugf("Sending to uplink: %+v", portNo)
		pktOut.AddAction(openflow13.NewActionOutput(portNo))

		// NOTE: Sending it on only one uplink to avoid loops
		// Once MAC pinning mode is supported, this logic has to change
		break
	}

	// Send it out
	vl.ofSwitch.Send(pktOut)
	return nil
}
