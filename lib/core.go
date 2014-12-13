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
	Clear(id string) error
}

type Config struct {
	// Config object parsed from a git-config like config
	v interface{}
}

type Network interface {
	// A network identifies a group of (addressable) endpoints that can
	// comunicate.
	CreateNetwork(id string) error
	DeleteNetwork(id string) error
}

type Endpoint interface {
	// An endpoint identifies an addressable entity in a network. An endpoint
	// belongs to a single network.
	CreateEndpoint(id string) error
	DeleteEndpoint(id string) error
}

type Plugin interface {
	// A plugin brings together an implementation of a network, endpoint and
	// state drivers. Along with implementing north-bound interfaces for
	// network and endpoint operations
	Init(config *Config)
	Deinit()
	Network
	Endpoint
}

type Driver interface {
	// A driver implements the programming logic
	Init(config *Config, stateDriver *StateDriver) error
	Deinit()
}

type NetworkDriver interface {
	// A network driver implements the programming logic for network
	Driver
	CreateNetwork(id string) error
	DeleteNetwork(id string) error
}

type EndpointDriver interface {
	// An endpoint driver implements the programming logic for endpoints
	Driver
	CreateEndpoint(id string) error
	DeleteEndpoint(id string) error
	GetEndpointAddress(id string) (Address, error) //determine the endpoint's address
}

type StateDriver interface {
	// A state driver provides mechanism for reading/writing state for networks,
	// endpoints and meta-data managed by the core. The state is assumed to be
	// stored as key-value pairs with keys of type 'string' and value to be an
	// opaque binary string, encoded/decoded by the logic specific to the
	// high-level(consumer) interface.
	Init(config *Config) error
	Deinit()
	Write(key string, value []byte) error
	Read(key string) ([]byte, error)
	WriteState(key string, value *State,
		marshal func(interface{}) ([]byte, error)) error
	ReadState(key string, value *State,
		unmarshal func([]byte, interface{}) error)
	ClearState(key string) error
}
