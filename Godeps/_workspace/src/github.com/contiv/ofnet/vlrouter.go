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
	"fmt"
	"net"
	"net/rpc"

	"github.com/contiv/ofnet/ofctrl"
	api "github.com/osrg/gobgp/api"
	"github.com/osrg/gobgp/packet"
	//"github.com/osrg/gobgp/server"
	"github.com/shaleman/libOpenflow/openflow13"
	"github.com/shaleman/libOpenflow/protocol"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	log "github.com/Sirupsen/logrus"
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

	// Router Mac to be used
	myRouterMac net.HardwareAddr
	myRouterIP  net.IP
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
	// FIXME: ??
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

// Add a local endpoint and install associated local route
func (self *Vlrouter) AddLocalEndpoint(endpoint OfnetEndpoint) error {
	// Install a flow entry for vlan mapping and point it to IP table
	portVlanFlow, err := self.vlanTable.NewFlow(ofctrl.FlowMatch{
		Priority:  FLOW_MATCH_PRIORITY,
		InputPort: endpoint.PortNo,
	})
	if err != nil {
		log.Errorf("Error creating portvlan entry. Err: %v", err)
		return err
	}
	err = portVlanFlow.Next(self.ipTable)
	if err != nil {
		log.Errorf("Error installing portvlan entry. Err: %v", err)
		return err
	}

	// save the flow entry
	self.portVlanFlowDb[endpoint.PortNo] = portVlanFlow

	// Set the vlan and install it
	// FIXME: Dont set the vlan till multi-vrf support. We cant pop vlan unless flow matches on vlan
	// portVlanFlow.SetVlan(endpoint.Vlan)
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

	log.Infof("ADDING TO BGP !!! LOCAL ROUTE")
	//dial grpc server
	conn, err := grpc.Dial("127.0.0.1:8080", grpc.WithBlock(), grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	path := &api.Path{
		Pattrs: make([][]byte, 0),
	}

	nlri := bgp.NewIPAddrPrefix(32, endpoint.IpAddr.String())
	path.Nlri, _ = nlri.Serialize()
	origin, _ := bgp.NewPathAttributeOrigin(bgp.BGP_ORIGIN_ATTR_TYPE_INCOMPLETE).Serialize()
	path.Pattrs = append(path.Pattrs, origin)
	n, _ := bgp.NewPathAttributeNextHop("50.1.1.1").Serialize()
	path.Pattrs = append(path.Pattrs, n)

	name := ""

	arg := &api.ModPathArguments{
		Resource: api.Resource_GLOBAL,
		Name:     name,
		Paths:    []*api.Path{path},
	}

	//send arguement stream
	client := api.NewGobgpApiClient(conn)
	log.Infof("The NewGobgpApiClient is ", client)
	stream, err := client.ModPath(context.Background())
	log.Infof("The stream is ", stream)
	if err != nil {
		log.Errorf("Fail to enforce Modpathi", err)
		return err
	}
	log.Infof("Sending msg")
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

// Remove local endpoint
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
	err = self.policyAgent.DelEndpoint(&endpoint)
	if err != nil {
		log.Errorf("Error deleting endpoint to policy agent{%+v}. Err: %v", endpoint, err)
		return err
	}

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

// AddEndpoint Add an endpoint to the datapath
func (self *Vlrouter) AddEndpoint(endpoint *OfnetEndpoint) error {
	log.Infof("AddEndpoint call for endpoint: %+v", endpoint)
	//Install a flow entry for vlan mapping and point it to IP table
	// Remove this. This has to be added as a part of Add vlan .
	portVlanFlow, err := self.vlanTable.NewFlow(ofctrl.FlowMatch{
		Priority:  FLOW_MATCH_PRIORITY,
		InputPort: endpoint.PortNo,
	})

	// Set the vlan and install it
	// FIXME: Dont set the vlan till multi-vrf support. We cant pop vlan unless flow matches on vlan
	// portVlanFlow.SetVlan(endpoint.Vlan)
	if err == nil {
		portVlanFlow.Next(self.ipTable)
		// save the flow entry
		self.portVlanFlowDb[endpoint.PortNo] = portVlanFlow
	}
	fmt.Println("TRYING TO ADD PORT FOR ENDPOINT PORT NO ")
	fmt.Println(endpoint.PortNo)
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
	// This is strictly not required at the source OVS. Source mac will be
	// overwritten by the dest OVS anyway. We keep the source mac for debugging purposes..
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

	// Store it in flow db
	self.flowDb[endpoint.IpAddr.String()] = ipFlow

	return nil
}

// RemoveEndpoint removes an endpoint from the datapath
func (self *Vlrouter) RemoveEndpoint(endpoint *OfnetEndpoint) error {
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
	//	err = self.policyAgent.DelEndpoint(endpoint)
	//	if err != nil {
	//		log.Errorf("Error deleting endpoint to policy agent{%+v}. Err: %v", endpoint, err)
	//		return err
	//	}

	return nil
}

// initialize Fgraph on the switch
func (self *Vlrouter) initFgraph() error {
	sw := self.ofSwitch

	// Create all tables
	self.inputTable = sw.DefaultTable()
	self.vlanTable, _ = sw.NewTable(VLAN_TBL_ID)
	self.ipTable, _ = sw.NewTable(IP_TBL_ID)

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

// Process incoming ARP packets
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
				log.Infof("Received ARP request for unknown IP: %v but will respond", arpHdr.IPDst)
				srcMac = self.myRouterMac
				//srcMac, _ = net.ParseMAC(endpoint.MacAddrStr)
				//return
			} else {
				if endpoint.EndpointType == "internal" || endpoint.EndpointType == "internal-bgp" {
					//srcMac, _ = net.ParseMAC(endpoint.MacAddrStr)
					intf, _ = net.InterfaceByName("eth7")
					srcMac = intf.HardwareAddr
				} else if endpoint.EndpointType == "external" {
					srcMac = self.myRouterMac
				}
			}

			//Check if source endpoint is learnt.
			endpoint = self.agent.getEndpointByIp(arpHdr.IPSrc)
			if endpoint != nil {
				//endpoint exists from where the arp is received.
				if endpoint.MacAddrStr == "" && endpoint.PortNo == 0 {

					//learn the mac address and portno for the endpoint
					self.RemoveEndpoint(endpoint)
					endpoint.PortNo = inPort
					endpoint.MacAddrStr = arpHdr.HWSrc.String()
					self.agent.endpointDb[endpoint.EndpointID] = endpoint
					self.AddEndpoint(endpoint)
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
