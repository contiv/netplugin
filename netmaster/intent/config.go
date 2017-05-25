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

// ConfigGlobal keeps track of settings that are globally applicable
type ConfigGlobal struct {
	NwInfraType string
	VLANs       string
	VXLANs      string
	FwdMode     string
	ArpMode     string
	PvtSubnet   string
}

// ConfigEP encapulsates an endpoint: a leg into a network
type ConfigEP struct {
	Container   string
	Host        string
	IPAddress   string
	IPv6Address string
	ServiceName string
}

// ConfigNetwork is a multi-destination isolated containment of endpoints
// or it is an endpoint group
type ConfigNetwork struct {
	Name string

	// overrides for various functions when auto allocation is not desired
	NwType         string
	PktTagType     string
	PktTag         int
	SubnetCIDR     string
	Gateway        string
	IPv6SubnetCIDR string
	IPv6Gateway    string
	Vrf            string
	CfgdTag        string

	// eps associated with the network
	Endpoints []ConfigEP
}

// ConfigTenant keeps the global tenant specific policy and networks within
type ConfigTenant struct {
	Name           string
	DefaultNetwork string
	VLANs          string
	VXLANs         string
	Networks       []ConfigNetwork
}

//ConfigBgp keeps bgp specific configs
type ConfigBgp struct {
	Hostname   string
	RouterIP   string
	As         string
	NeighborAs string
	Neighbor   string
}

//ConfigServiceLB keeps servicelb specific configs
type ConfigServiceLB struct {
	ServiceName string
	Tenant      string
	Selectors   map[string]string
	Network     string
	Ports       []string
	IPAddress   string
}

// Config is the top level configuration
type Config struct {
	Tenants []ConfigTenant
	// (optional) host bindings
	HostBindings []ConfigEP
	RouterInfo   []ConfigBgp
}
