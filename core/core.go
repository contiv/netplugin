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

// Package core provides definition for a generic interface that helps
// provision networking for an endpoint (like a container,
// a vm or a bare-metal host). The interface is invoked (north-bound) by the
// 'daemon' or the extension-plugin (TBD) part of docker. The interface in
// turn invokes (south-bound) a driver-interface that provides
// hardware/kernel/device specific programming implementation, if any.
package core

// Address is a string representation of a network address (mac, ip, dns-name, url etc)
type Address struct {
	addr string
}

// Config object parsed from a json styled config
type Config struct {
	V interface{}
}

// ServiceInfo has information about a service
type ServiceInfo struct {
	HostAddr string // Host name or IP address where its running
	Port     int    // Port number where its listening
}

// Network identifies a group of (addressable) endpoints that can
// comunicate.
type Network interface {
	CreateNetwork(id string) error
	DeleteNetwork(id, encap string, pktTag, extPktTag int, tenant string) error
	FetchNetwork(id string) (State, error)
}

// Endpoint identifies an addressable entity in a network. An endpoint
// belongs to a single network.
type Endpoint interface {
	CreateEndpoint(id string) error
	DeleteEndpoint(id string) error
	FetchEndpoint(id string) (State, error)
}

// Clustering has functions for discovering peer nodes and masters
type Clustering interface {
	AddPeerHost(node ServiceInfo) error
	DeletePeerHost(node ServiceInfo) error
	AddMaster(node ServiceInfo) error
	DeleteMaster(node ServiceInfo) error
}

// Plugin brings together an implementation of a network, endpoint and
// state drivers. Along with implementing north-bound interfaces for
// network and endpoint operations
type Plugin interface {
	Init(instInfo InstanceInfo) error
	Deinit()
	Network
	Endpoint
	Clustering
}

// InstanceInfo encapsulates data that is specific to a running instance of
// netplugin like label of host on which it is started.
type InstanceInfo struct {
	StateDriver  StateDriver `json:"-"`
	HostLabel    string      `json:"host-label"`
	CtrlIP       string      `json:"ctrl-ip"`
	VtepIP       string      `json:"vtep-ip"`
	UplinkIntf   []string    `json:"uplink-if"`
	RouterIP     string      `json:"router-ip"`
	FwdMode      string      `json:"fwd-mode"`
	ArpMode      string      `json:"arp-mode"`
	DbURL        string      `json:"db-url"`
	PluginMode   string      `json:"plugin-mode"`
	HostPvtNW    int         `json:"host-pvt-nw"`
	VxlanUDPPort int         `json:"vxlan-port"`
}

// PortSpec defines protocol/port info required to host the service
type PortSpec struct {
	Protocol string
	SvcPort  uint16 // advertised port
	ProvPort uint16 // actual port of provider
	NodePort uint16 // port on the node where service is exposed
}

// ServiceSpec defines a service to be proxied
type ServiceSpec struct {
	IPAddress   string
	Ports       []PortSpec
	ExternalIPs []string // externally visible IPs
}

// Driver implements the programming logic
type Driver interface{}

// NetworkDriver implements the programming logic for network and endpoints
type NetworkDriver interface {
	Driver
	Init(instInfo *InstanceInfo) error
	Deinit()
	CreateNetwork(id string) error
	DeleteNetwork(id, subnet, nwType, encap string, pktTag, extPktTag int, gateway string, tenant string) error
	CreateEndpoint(id string) error
	UpdateEndpointGroup(id string) error
	DeleteEndpoint(id string) error
	CreateRemoteEndpoint(id string) error
	DeleteRemoteEndpoint(id string) error
	CreateHostAccPort(portName, globalIP string, nw int) (string, error)
	DeleteHostAccPort(id string) error
	AddPeerHost(node ServiceInfo) error
	DeletePeerHost(node ServiceInfo) error
	AddMaster(node ServiceInfo) error
	DeleteMaster(node ServiceInfo) error
	AddBgp(id string) error
	DeleteBgp(id string) error
	// Add a service spec to proxy
	AddSvcSpec(svcName string, spec *ServiceSpec) error
	// Remove a service spec from proxy
	DelSvcSpec(svcName string, spec *ServiceSpec) error
	// Service Proxy Back End update
	SvcProviderUpdate(svcName string, providers []string)
	// Get endpoint stats
	GetEndpointStats() ([]byte, error)
	// return current state in json form
	InspectState() ([]byte, error)
	// return bgp in json form
	InspectBgp() ([]byte, error)
	// Set global config
	GlobalConfigUpdate(inst InstanceInfo) error
	InspectNameserver() ([]byte, error)
	AddPolicyRule(id string) error
	DelPolicyRule(id string) error
}

// WatchState is used to provide a difference between core.State structs by
// providing both the current and previous state.
type WatchState struct {
	Curr State
	Prev State
}

// StateDriver provides the mechanism for reading/writing state for networks,
// endpoints and meta-data managed by the core. The state is assumed to be
// stored as key-value pairs with keys of type 'string' and value to be an
// opaque binary string, encoded/decoded by the logic specific to the
// high-level(consumer) interface.
type StateDriver interface {
	Driver
	Init(instInfo *InstanceInfo) error
	Deinit()

	// XXX: the following raw versions of Read, Write, ReadAll and WatchAll
	// can perhaps be removed from core API, as no one uses them directly.
	Write(key string, value []byte) error
	Read(key string) ([]byte, error)
	ReadAll(baseKey string) ([][]byte, error)
	WatchAll(baseKey string, rsps chan [2][]byte) error

	WriteState(key string, value State,
		marshal func(interface{}) ([]byte, error)) error
	ReadState(key string, value State,
		unmarshal func([]byte, interface{}) error) error
	ReadAllState(baseKey string, stateType State,
		unmarshal func([]byte, interface{}) error) ([]State, error)
	// WatchAllState returns changes to a state from the point watch is started.
	// It's a blocking call.
	// XXX: This specification introduces a small time window where a few
	// updates might be missed that occurred just before watch was started.
	// May be watch shall return all existing state first and then subsequent
	// updates. Revisit if this enhancement is needed.
	WatchAllState(baseKey string, stateType State,
		unmarshal func([]byte, interface{}) error, rsps chan WatchState) error
	ClearState(key string) error
}

// Resource defines a allocatable unit. A resource is uniquely identified
// by 'ID'. A resource description identifies the nature of the resource.
type Resource interface {
	State
	Init(rsrcCfg interface{}) error
	Deinit()
	Reinit(rsrcCfg interface{}) error
	Description() string
	GetList() (uint, string)
	Allocate(interface{}) (interface{}, error)
	Deallocate(interface{}) error
}

// ResourceManager provides mechanism to manage (define/undefine,
// allocate/deallocate) resources. Example, it may provide management in
// logically centralized manner in a distributed system
type ResourceManager interface {
	Init() error
	Deinit()
	DefineResource(id, desc string, rsrcCfg interface{}) error
	RedefineResource(id, desc string, rsrcCfg interface{}) error
	UndefineResource(id, desc string) error
	GetResourceList(id, desc string) (uint, string)
	AllocateResourceVal(id, desc string, reqValue interface{}) (interface{}, error)
	DeallocateResourceVal(id, desc string, value interface{}) error
}
