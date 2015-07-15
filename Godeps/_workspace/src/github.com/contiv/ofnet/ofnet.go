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
    AddLocalEndpoint(endpoint EndpointInfo) error

    // Remove a local endpoint from forwarding DB
    RemoveLocalEndpoint(portNo uint32) error

    // Add an remote VTEP
    AddVtepPort(portNo uint32, remoteIp net.IP) error

    // Remove remote VTEP
    RemoveVtepPort(portNo uint32, remoteIp net.IP) error

    // Add a vlan
    AddVlan(vlanId uint16, vni uint32) error

    // Remove a vlan
    RemoveVlan(vlanId uint16, vni uint32) error
}

// Default port numbers
const OFNET_MASTER_PORT = 9001
const OFNET_AGENT_PORT  = 9002

// Information about each node
type OfnetNode struct {
    HostAddr    string
    HostPort    uint16
}
