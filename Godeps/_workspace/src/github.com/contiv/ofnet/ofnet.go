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

// This package implements openflow network manager

import (
	"net"
	"time"

	"github.com/contiv/ofnet/ofctrl"
)

// Interface implemented by each datapath
type OfnetDatapath interface {
	// New master was added.
	MasterAdded(master *OfnetNode) error

	// Switch connected notification
	SwitchConnected(sw *ofctrl.OFSwitch)

	// Switch disconnected notification
	SwitchDisconnected(sw *ofctrl.OFSwitch)

	// Process Incoming packet
	PacketRcvd(sw *ofctrl.OFSwitch, pkt *ofctrl.PacketIn)

	// Add a local endpoint to forwarding DB
	AddLocalEndpoint(endpoint OfnetEndpoint) error

	// Remove a local endpoint from forwarding DB
	RemoveLocalEndpoint(endpoint OfnetEndpoint) error

	// Add a remote endpoint to forwarding DB
	AddEndpoint(endpoint *OfnetEndpoint) error

	// Remove a remote endpoint from forwarding DB
	RemoveEndpoint(endpoint *OfnetEndpoint) error

	// Add an remote VTEP
	AddVtepPort(portNo uint32, remoteIp net.IP) error

	// Remove remote VTEP
	RemoveVtepPort(portNo uint32, remoteIp net.IP) error

	// Add a vlan
	AddVlan(vlanId uint16, vni uint32) error

	// Remove a vlan
	RemoveVlan(vlanId uint16, vni uint32) error

	//Add uplink port
	AddUplink(portNo uint32) error

	//Delete uplink port
	RemoveUplink(portNo uint32) error

	// Add a service spec to proxy
	AddSvcSpec(svcName string, spec *ServiceSpec) error

	// Remove a service spec from proxy
	DelSvcSpec(svcName string, spec *ServiceSpec) error

	// Service Proxy Back End update
	SvcProviderUpdate(svcName string, providers []string)
}

// Interface implemented by each control protocol.
type OfnetProto interface {

	//Create a protocol server
	StartProtoServer(routerInfo *OfnetProtoRouterInfo) error

	StopProtoServer() error

	//Add a Protocol Neighbor
	AddProtoNeighbor(neighborInfo *OfnetProtoNeighborInfo) error

	//Delete a Protocol Neighbor
	DeleteProtoNeighbor() error

	//Get Protocol router info
	GetRouterInfo() *OfnetProtoRouterInfo

	//Add Local Route
	AddLocalProtoRoute(path *OfnetProtoRouteInfo) error

	//Delete Local Route
	DeleteLocalProtoRoute(path *OfnetProtoRouteInfo) error

	//Modify protocol Rib (Could be used for testing)
	ModifyProtoRib(path interface{})
}

// Default port numbers
const OFNET_MASTER_PORT = 9001
const OFNET_AGENT_VXLAN_PORT = 9002
const OFNET_AGENT_VLAN_PORT = 9010

// Information about each node
type OfnetNode struct {
	HostAddr string
	HostPort uint16
}

// OfnetEndpoint has info about an endpoint
type OfnetEndpoint struct {
	EndpointID    string    // Unique identifier for the endpoint
	EndpointType  string    // Type of the endpoint "internal", "external" or "externalRoute"
	EndpointGroup int       // Endpoint group identifier for policies.
	IpAddr        net.IP    // IP address of the end point
	IpMask        net.IP    // IP mask for the end point
	VrfId         uint16    // IP address namespace
	MacAddrStr    string    // Mac address of the end point(in string format)
	Vlan          uint16    // Vlan Id for the endpoint
	Vni           uint32    // Vxlan VNI
	OriginatorIp  net.IP    // Originating switch
	PortNo        uint32    // Port number on originating switch
	Timestamp     time.Time // Timestamp of the last event
}

// OfnetPolicyRule has security rule to be installed
type OfnetPolicyRule struct {
	RuleId           string // Unique identifier for the rule
	Priority         int    // Priority for the rule (1..100. 100 is highest)
	SrcEndpointGroup int    // Source endpoint group
	DstEndpointGroup int    // Destination endpoint group
	SrcIpAddr        string // source IP addrss and mask
	DstIpAddr        string // Destination IP address and mask
	IpProtocol       uint8  // IP protocol number
	SrcPort          uint16 // Source port
	DstPort          uint16 // destination port
	TcpFlags         string // TCP flags to match: syn || syn,ack || ack || syn,!ack || !syn,ack;
	Action           string // rule action: 'accept' or 'deny'
}

type OfnetProtoNeighborInfo struct {
	ProtocolType string // type of protocol
	NeighborIP   string // ip address of the neighbor
	As           string // As of neighbor if applicable
}

type OfnetProtoRouterInfo struct {
	ProtocolType string // type of protocol
	RouterIP     string // ip address of the router
	VlanIntf     string // uplink L2 intf
	As           string // As for Bgp protocol
}

type OfnetProtoRouteInfo struct {
	ProtocolType string // type of protocol
	localEpIP    string
	nextHopIP    string
}
