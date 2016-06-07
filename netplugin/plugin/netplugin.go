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

package plugin

import (
	"sync"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/utils"
)

// implements the generic Plugin interface

// Drivers has driver config
type Drivers struct {
	Network  string `json:"network"`
	Endpoint string `json:"endpoint"`
	State    string `json:"state"`
}

// Config has the configuration for the plugin
type Config struct {
	Drivers  Drivers           `json:"drivers"`
	Instance core.InstanceInfo `json:"plugin-instance"`
}

/*
//ProviderInfo has providers info
type ProviderInfo struct {
	IPAddress   string            // provider IP
	ContainerID string            // container id
	Labels      map[string]string // lables
	Tenant      string
	Network     string
}
*/
/*
//ServiceLBInfo holds service information
type ServiceLBInfo struct {
	ServiceName string //Service name
	IPAddress   string //Service IP
	Tenant      string //Tenant name of the service
	SrvPort     uint16 //Service port
	ProvPort    uint16
	Labels      map[string]string        // Labels associated with a service
	Providers   map[string]*ProviderInfo //map of providers for a service keyed by provider ip
}
*/
//ServiceLBInfo is map of all services
//var ServiceLBDb map[string]*ServiceLBInfo //DB for all services keyed by servicename.tenant
//ProviderDb is map of all providers
//var ProviderDb map[string]*ProviderInfo

//

// NetPlugin is the configuration struct for the plugin bus. Network and
// Endpoint drivers are all present in `drivers/` and state drivers are present
// in `state/`.
type NetPlugin struct {
	sync.Mutex
	ConfigFile    string
	NetworkDriver core.NetworkDriver
	StateDriver   core.StateDriver
}

// Init initializes the NetPlugin instance via the configuration string passed.
func (p *NetPlugin) Init(pluginConfig Config) error {
	var err error
	if pluginConfig.Instance.HostLabel == "" {
		return core.Errorf("empty host-label passed")
	}

	// initialize state driver
	p.StateDriver, err = utils.NewStateDriver(pluginConfig.Drivers.State, &pluginConfig.Instance)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			utils.ReleaseStateDriver()
		}
	}()

	// set state driver in instance info
	pluginConfig.Instance.StateDriver = p.StateDriver

	// initialize network driver
	p.NetworkDriver, err = utils.NewNetworkDriver(pluginConfig.Drivers.Network, &pluginConfig.Instance)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			p.NetworkDriver.Deinit()
		}
	}()
	/*
		ServiceLBDb = make(map[string]*ServiceLBInfo)
		ProviderDb = make(map[string]*ProviderInfo)
	*/
	return nil
}

// Deinit is a destructor for the NetPlugin configuration.
func (p *NetPlugin) Deinit() {
	if p.NetworkDriver != nil {
		p.NetworkDriver.Deinit()
		p.NetworkDriver = nil
	}
	if p.StateDriver != nil {
		utils.ReleaseStateDriver()
		p.StateDriver = nil
	}
}

// CreateNetwork creates a network for a given ID.
func (p *NetPlugin) CreateNetwork(id string) error {
	return p.NetworkDriver.CreateNetwork(id)
}

// DeleteNetwork deletes a network provided by the ID.
func (p *NetPlugin) DeleteNetwork(id, nwType, encap string, pktTag, extPktTag int, Gw string, tenant string) error {
	return p.NetworkDriver.DeleteNetwork(id, nwType, encap, pktTag, extPktTag, Gw, tenant)
}

// FetchNetwork retrieves a network's state given an ID.
func (p *NetPlugin) FetchNetwork(id string) (core.State, error) {
	return nil, core.Errorf("Not implemented")
}

// CreateEndpoint creates an endpoint for a given ID.
func (p *NetPlugin) CreateEndpoint(id string) error {
	return p.NetworkDriver.CreateEndpoint(id)
}

// DeleteEndpoint destroys an endpoint for an ID.
func (p *NetPlugin) DeleteEndpoint(id string) error {
	return p.NetworkDriver.DeleteEndpoint(id)
}

// FetchEndpoint retrieves an endpoint's state for a given ID
func (p *NetPlugin) FetchEndpoint(id string) (core.State, error) {
	return nil, core.Errorf("Not implemented")
}

// AddPeerHost adds an peer host.
func (p *NetPlugin) AddPeerHost(node core.ServiceInfo) error {
	return p.NetworkDriver.AddPeerHost(node)
}

// DeletePeerHost removes a peer host.
func (p *NetPlugin) DeletePeerHost(node core.ServiceInfo) error {
	return p.NetworkDriver.DeletePeerHost(node)
}

// AddMaster adds a master node.
func (p *NetPlugin) AddMaster(node core.ServiceInfo) error {
	return p.NetworkDriver.AddMaster(node)
}

// DeleteMaster removes a master node
func (p *NetPlugin) DeleteMaster(node core.ServiceInfo) error {
	return p.NetworkDriver.DeleteMaster(node)
}

//AddBgp adds bgp configs
func (p *NetPlugin) AddBgp(id string) error {
	return p.NetworkDriver.AddBgp(id)
}

//DeleteBgp deletes bgp configs
func (p *NetPlugin) DeleteBgp(id string) error {
	return p.NetworkDriver.DeleteBgp(id)
}

//AddServiceLB adds service
func (p *NetPlugin) AddServiceLB(servicename string, spec *core.ServiceSpec) error {
	return p.NetworkDriver.AddSvcSpec(servicename, spec)
}

//DeleteServiceLB deletes service
func (p *NetPlugin) DeleteServiceLB(servicename string, spec *core.ServiceSpec) error {
	return p.NetworkDriver.DelSvcSpec(servicename, spec)
}

//SvcProviderUpdate hhhh
func (p *NetPlugin) SvcProviderUpdate(servicename string, providers []string) {
	p.NetworkDriver.SvcProviderUpdate(servicename, providers)
}
