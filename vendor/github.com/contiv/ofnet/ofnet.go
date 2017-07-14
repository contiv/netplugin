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

	"github.com/contiv/libOpenflow/openflow13"
	"github.com/contiv/ofnet/ofctrl"
)

const (
	OFNET_INTERNAL     = 1 //Internal contiv cluster ep type
	OFNET_INTERNAL_BGP = 2 //Internal contiv bgp intf ep
	OFNET_EXTERNAL_BGP = 3 //External contiv bgp neighbor ep
	OFNET_EXTERNAL     = 4 //External eps (non contiv eps)
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

	// Update a local endpoint state
	UpdateLocalEndpoint(ep *OfnetEndpoint, epInfo EndpointInfo) error

	// Add a remote endpoint to forwarding DB
	AddEndpoint(endpoint *OfnetEndpoint) error

	// Remove a remote endpoint from forwarding DB
	RemoveEndpoint(endpoint *OfnetEndpoint) error

	// Add an remote VTEP
	AddVtepPort(portNo uint32, remoteIp net.IP) error

	// Remove remote VTEP
	RemoveVtepPort(portNo uint32, remoteIp net.IP) error

	// Add a vlan
	AddVlan(vlanId uint16, vni uint32, vrf string) error

	// Remove a vlan
	RemoveVlan(vlanId uint16, vni uint32, vrf string) error

	//Add uplink port
	AddUplink(uplinkPort *PortInfo) error

	//Update uplink port
	UpdateUplink(uplinkName string, update PortUpdates) error

	//Delete uplink port
	RemoveUplink(uplinkName string) error

	// AddHostPort
	AddHostPort(hp HostPortInfo) error

	// RemoveHostPort
	RemoveHostPort(portNo uint32) error

	//Inject GARPs
	InjectGARPs(epgID int)

	// Add a service spec to proxy
	AddSvcSpec(svcName string, spec *ServiceSpec) error

	// Remove a service spec from proxy
	DelSvcSpec(svcName string, spec *ServiceSpec) error

	// Service Proxy Back End update
	SvcProviderUpdate(svcName string, providers []string)

	// Handle multipart replies from OVS
	MultipartReply(sw *ofctrl.OFSwitch, reply *openflow13.MultipartReply)

	// Get endpoint stats
	GetEndpointStats() (map[string]*OfnetEndpointStats, error)

	// Return the datapath state
	InspectState() (interface{}, error)

	// Set global config
	GlobalConfigUpdate(cfg OfnetGlobalConfig) error

	// flush the endpoints
	FlushEndpoints(endpointType int)
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

	//Set Protocol router info
	SetRouterInfo(uplinkPort *PortInfo) error

	//Add Local Route
	AddLocalProtoRoute(path []*OfnetProtoRouteInfo) error

	//Delete Local Route
	DeleteLocalProtoRoute(path []*OfnetProtoRouteInfo) error

	//Modify protocol Rib (Could be used for testing)
	ModifyProtoRib(path interface{})

	//Inspect bgp
	InspectProto() (interface{}, error)
}

// Default port numbers
const OFNET_MASTER_PORT = 9001
const OFNET_AGENT_VXLAN_PORT = 9002
const OFNET_AGENT_VLAN_PORT = 9010

// internal vlan id
const nameServerInternalVlanId = 4093

// Information about each node
type OfnetNode struct {
	HostAddr string
	HostPort uint16
}

// OfnetEndpoint has info about an endpoint
type OfnetEndpoint struct {
	EndpointID        string    // Unique identifier for the endpoint
	EndpointType      int       // Type of the endpoint , "external" or "externalRoute"
	EndpointGroup     int       // Endpoint group identifier for policies.
	IpAddr            net.IP    // IP address of the end point
	IpMask            net.IP    // IP mask for the end point
	Ipv6Addr          net.IP    // IPv6 address of the end point
	Ipv6Mask          net.IP    // IPv6 mask for the end point
	Vrf               string    // IP address namespace
	MacAddrStr        string    // Mac address of the end point(in string format)
	Vlan              uint16    // Vlan Id for the endpoint
	Vni               uint32    // Vxlan VNI
	EndpointGroupVlan uint16    // EnpointGroup Vlan, needed in non-Standalone mode of netplugin
	OriginatorIp      net.IP    // Originating switch
	OriginatorMac     string    // Mac address of the endpoint host
	PortNo            uint32    `json:"-"` // Port number on originating switch
	Dscp              int       `json:"-"` // DSCP value for the endpoint
	Timestamp         time.Time // Timestamp of the last event
	HostPvtIP         net.IP    `json:"-"` // Private IP
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

// OfnetProtoNeighborInfo has bgp neighbor info
type OfnetProtoNeighborInfo struct {
	ProtocolType string // type of protocol
	NeighborIP   string // ip address of the neighbor
	As           string // As of neighbor if applicable
}

// OfnetProtoRouterInfo has local router info
type OfnetProtoRouterInfo struct {
	ProtocolType string    // type of protocol
	RouterIP     string    // ip address of the router
	UplinkPort   *PortInfo // uplink L2 intf
	As           string    // As for Bgp protocol
}

// OfnetProtoRouteInfo contains a route
type OfnetProtoRouteInfo struct {
	ProtocolType string // type of protocol
	localEpIP    string
	nextHopIP    string
}

type ArpModeT string

const (
	// ArpFlood - ARP packets will be flooded in this mode
	ArpFlood ArpModeT = "flood"
	// ArpProxy - ARP packets will be redirected to controller
	ArpProxy ArpModeT = "proxy"

	// PortType - individual port
	PortType = "individual"
	// BondType - bonded port
	BondType = "bond"

	// LacpUpdate - for port update info
	LacpUpdate = "lacp-upd"
)

// OfnetGlobalConfig has global level configs for ofnet
type OfnetGlobalConfig struct {
	ArpMode ArpModeT // arp mode: proxy or flood
}

// OfnetVrfInfo has info about a VRF
type OfnetVrfInfo struct {
	VrfName     string // vrf name
	VrfId       uint16 // local vrf id
	NumNetworks uint16 // ref count of networks in the vrf
}

// OfnetDatapathStats is generic stats struct
type OfnetDatapathStats struct {
	PacketsIn  uint64
	BytesIn    uint64
	PacketsOut uint64
	BytesOut   uint64
}

// OfnetSvcProviderStats has stats for a provider of a service
type OfnetSvcProviderStats struct {
	ProviderIP         string // Provider IP address
	ServiceIP          string // service ip address
	ServiceVrf         string // Provider VRF name
	OfnetDatapathStats        // stats
}

// OfnetSvcStats per service stats from one client
type OfnetSvcStats struct {
	ServiceIP  string                           // service ip address
	ServiceVRF string                           // service vrf name
	Protocol   string                           // service protocol tcp | udp
	SvcPort    string                           // Service Port
	ProvPort   string                           // Provider port
	SvcStats   OfnetDatapathStats               // aggregate service stats
	ProvStats  map[string]OfnetSvcProviderStats // per provider stats
}

// OfnetEndpointStats has stats for local endpoints
type OfnetEndpointStats struct {
	EndpointIP string                   // Endpoint IP address
	VrfName    string                   // vrf name
	PortStats  OfnetDatapathStats       // Aggregate port stats
	SvcStats   map[string]OfnetSvcStats // Service level stats
}

type linkStatus int

// LinkStatus maintains link up/down information
const (
	linkDown linkStatus = iota
	linkUp
)

// LinkInfo maintains individual link information
type LinkInfo struct {
	Name       string
	Port       *PortInfo
	LinkStatus linkStatus
	OfPort     uint32
}

// PortInfo maintains port information
type PortInfo struct {
	Name        string
	Type        string
	LinkStatus  linkStatus
	MbrLinks    []*LinkInfo
	ActiveLinks []*LinkInfo
}

// PortUpdates maintains multiplae port update info
type PortUpdates struct {
	PortName string
	Updates  []PortUpdate
}

// PortUpdate maintains information about port update
type PortUpdate struct {
	UpdateType string
	UpdateInfo interface{}
}

// LACP update
type LinkUpdateInfo struct {
	LinkName   string
	LacpStatus bool
}
