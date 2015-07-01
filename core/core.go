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

// Address is a string represenation of a network address (mac, ip, dns-name, url etc)
type Address struct {
	addr string
}

// Config object parsed from a json styled config
type Config struct {
	V interface{}
}

// Network identifies a group of (addressable) endpoints that can
// comunicate.
type Network interface {
	CreateNetwork(id string) error
	DeleteNetwork(id string) error
	FetchNetwork(id string) (State, error)
}

// Endpoint identifies an addressable entity in a network. An endpoint
// belongs to a single network.
type Endpoint interface {
	CreateEndpoint(id string) error
	DeleteEndpoint(id string) error
	FetchEndpoint(id string) (State, error)
}

// PeerHost identifies a peer which this node can communicate to
// Generally this info is used for network wide operations like setting up
// VTEP tunnels, synchronizing routes etc.
type PeerHost interface {
	CreatePeerHost(id string) error
	DeletePeerHost(id string) error
}

// Plugin brings together an implementation of a network, endpoint and
// state drivers. Along with implementing north-bound interfaces for
// network and endpoint operations
type Plugin interface {
	Init(configStr string) error
	Deinit()
	Network
	Endpoint
	PeerHost
}

// InstanceInfo encapsulates data that is specific to a running instance of
// netplugin like label of host on which it is started.
type InstanceInfo struct {
	StateDriver StateDriver `json:"-"`
	HostLabel   string      `json:"host-label"`
	VtepIP      string      `json:"vtep-ip"`
	VlanIntf    string      `json:"vlan-if"`
}

// Driver implements the programming logic
type Driver interface{}

// NetworkDriver implements the programming logic for network and endpoints
type NetworkDriver interface {
	Driver
	Init(config *Config, info *InstanceInfo) error
	Deinit()
	CreateNetwork(id string) error
	DeleteNetwork(id string) error
	CreateEndpoint(id string) error
	DeleteEndpoint(id string) error
	CreatePeerHost(id string) error
	DeletePeerHost(id string) error
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
	Init(config *Config) error
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
	Description() string
	Allocate() (interface{}, error)
	Deallocate(interface{}) error
}

// ResourceManager provides mechanism to manage (define/undefine,
// allocate/deallocate) resources. Example, it may provide management in
// logically centralized manner in a distributed system
type ResourceManager interface {
	Init() error
	Deinit()
	DefineResource(id, desc string, rsrcCfg interface{}) error
	UndefineResource(id, desc string) error
	AllocateResourceVal(id, desc string) (interface{}, error)
	DeallocateResourceVal(id, desc string, value interface{}) error
}
