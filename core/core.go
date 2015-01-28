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

package core

// The package 'core' provides definition for a generic interface that helps
// provision networking for an endpoint (like a container,
// a vm or a bare-metal host). The interface is invoked (north-bound) by the
// 'daemon' or the extension-plugin (TBD) part of docker. The interface in
// turn invokes (south-bound) a driver-interface that provides
// hardware/kernel/device specific programming implementation, if any.

type Address struct {
	// A string represenation of a network address (mac, ip, dns-name, url etc)
	addr string
}

type State interface {
	Write() error
	Read(id string) error
	Clear() error
}

type Config struct {
	// Config object parsed from a json styled config
	V interface{}
}

type Network interface {
	// A network identifies a group of (addressable) endpoints that can
	// comunicate.
	CreateNetwork(id string) error
	DeleteNetwork(id string) error
	FetchNetwork(id string) (State, error)
}

type Endpoint interface {
	// An endpoint identifies an addressable entity in a network. An endpoint
	// belongs to a single network.
	CreateEndpoint(id string) error
	DeleteEndpoint(id string) error
	FetchEndpoint(id string) (State, error)
}

type Plugin interface {
	// A plugin brings together an implementation of a network, endpoint and
	// state drivers. Along with implementing north-bound interfaces for
	// network and endpoint operations
	Init(configStr string) error
	Deinit()
	Network
	Endpoint
}

type Driver interface {
	// A driver implements the programming logic
}

type NetworkDriver interface {
	// A network driver implements the programming logic for network
	Driver
	Init(config *Config, stateDriver StateDriver) error
	Deinit()
	CreateNetwork(id string) error
	DeleteNetwork(id string) error
}

type EndpointDriver interface {
	// An endpoint driver implements the programming logic for endpoints
	Driver
	Init(config *Config, stateDriver StateDriver) error
	Deinit()
	CreateEndpoint(id string) error
	DeleteEndpoint(id string) error
	MakeEndpointAddress() (*Address, error)
	GetEndpointContainerContext(id string) (*ContainerEpContext, error)
	GetContainerEpContextByContName(contName string) ([]ContainerEpContext, error)
    UpdateContainerId(id string, contId string) error
}

type StateDriver interface {
	// A state driver provides mechanism for reading/writing state for networks,
	// endpoints and meta-data managed by the core. The state is assumed to be
	// stored as key-value pairs with keys of type 'string' and value to be an
	// opaque binary string, encoded/decoded by the logic specific to the
	// high-level(consumer) interface.
	Driver
	Init(config *Config) error
	Deinit()
	Write(key string, value []byte) error
	Read(key string) ([]byte, error)
    ReadRecursive(baseKey string) ([]string, error)
	WriteState(key string, value State,
		marshal func(interface{}) ([]byte, error)) error
	ReadState(key string, value State,
		unmarshal func([]byte, interface{}) error) error
	ClearState(key string) error
}

type ContainerEpContext struct {
    NewContName     string 
    CurrContName    string 
    InterfaceId     string
    IpAddress       string
    SubnetLen       uint
    DefaultGw       string
}

type ContainerDriver interface {
    // Container driver provides a mechanism to interface with container
    // runtime to handle events create, start, die, stop, pause, etc.
    Driver
    Init(config *Config) error
    Deinit()
    AttachEndpoint(ctx *ContainerEpContext) error
    DetachEndpoint(ctx *ContainerEpContext) error
    GetContainerId(contName string) string
    GetContainerName(contName string) (string, error)
}

