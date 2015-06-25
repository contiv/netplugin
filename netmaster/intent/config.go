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

// netmaster intent specification

package intent

// ConfigHost keeps track of the host's properties; A host is a node where
// containers are deployed
type ConfigHost struct {
	Name   string
	Intf   string
	VtepIP string
	NetID  string
}

// ConfigInfraNetwork holds information about the name and LAN tagging
// configuration.
type ConfigInfraNetwork struct {
	Name       string
	PktTagType string
	PktTag     string
}

// ConfigEP encapulsates an endpoint: a leg into a network
type ConfigEP struct {
	Container  string
	Host       string
	AttachUUID string
	IPAddress  string
}

// ConfigNetwork is a multi-destination isolated containment of endpoints
// or it is an endpoint group
type ConfigNetwork struct {
	Name string

	// overrides for various functions when auto allocation is not desired
	PktTagType string
	PktTag     string
	SubnetCIDR string
	DefaultGw  string

	// eps associated with the network
	Endpoints []ConfigEP
}

// ConfigTenant keeps the global tenant specific policy and networks within
type ConfigTenant struct {
	Name           string
	DefaultNetType string
	SubnetPool     string
	AllocSubnetLen uint
	VLANs          string
	VXLANs         string

	Networks []ConfigNetwork
}

// Config is the top level configuration
type Config struct {
	InfraNetworks []ConfigInfraNetwork
	Hosts         []ConfigHost
	Tenants       []ConfigTenant
	// (optional) host bindings
	HostBindings []ConfigEP
}
