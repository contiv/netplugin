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

	// Add an uplink to the switch
	AddUplink(portNo uint32) error

	// Remove an uplink
	RemoveUplink(portNo uint32) error
}

// Default port numbers
const OFNET_MASTER_PORT = 9001
const OFNET_AGENT_PORT = 9002

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
