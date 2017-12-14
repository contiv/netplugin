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
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/netplugin/utils/netutils"
	"sync"
	"time"
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

// NetPlugin is the configuration struct for the plugin bus. Network and
// Endpoint drivers are all present in `drivers/` and state drivers are present
// in `state/`.
type NetPlugin struct {
	sync.Mutex
	ConfigFile    string
	NetworkDriver core.NetworkDriver
	StateDriver   core.StateDriver
	PluginConfig  Config
}

// Init initializes the NetPlugin instance via the configuration string passed.
func (p *NetPlugin) Init(pluginConfig Config) error {
	var err error
	if pluginConfig.Instance.HostLabel == "" {
		return core.Errorf("empty host-label passed")
	}

	// initialize state driver
	p.StateDriver, err = utils.GetStateDriver()
	if err != nil {
		p.StateDriver, err = utils.NewStateDriver(pluginConfig.Drivers.State, &pluginConfig.Instance)
		if err != nil {
			return err
		}
	}
	defer func() {
		if err != nil {
			utils.ReleaseStateDriver()
		}
	}()

	// set state driver in instance info
	pluginConfig.Instance.StateDriver = p.StateDriver
	err = InitGlobalSettings(p.StateDriver, &pluginConfig.Instance)
	if err != nil {
		return err
	}

	// initialize network driver
	p.NetworkDriver, err = utils.NewNetworkDriver(pluginConfig.Drivers.Network, &pluginConfig.Instance)
	if err != nil {
		return err
	}
	p.PluginConfig = pluginConfig

	defer func() {
		if err != nil {
			p.NetworkDriver.Deinit()
		}
	}()

	return nil
}

// Deinit is a destructor for the NetPlugin configuration.
func (p *NetPlugin) Deinit() {
	p.Lock()
	defer p.Unlock()

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
	p.Lock()
	defer p.Unlock()
	return p.NetworkDriver.CreateNetwork(id)
}

// DeleteNetwork deletes a network provided by the ID.
func (p *NetPlugin) DeleteNetwork(id, subnet, nwType, encap string, pktTag, extPktTag int, Gw string, tenant string) error {
	p.Lock()
	defer p.Unlock()
	return p.NetworkDriver.DeleteNetwork(id, subnet, nwType, encap, pktTag, extPktTag, Gw, tenant)
}

// FetchNetwork retrieves a network's state given an ID.
func (p *NetPlugin) FetchNetwork(id string) (core.State, error) {
	return nil, core.Errorf("Not implemented")
}

// CreateEndpoint creates an endpoint for a given ID.
func (p *NetPlugin) CreateEndpoint(id string) error {
	p.Lock()
	defer p.Unlock()
	return p.NetworkDriver.CreateEndpoint(id)
}

//UpdateEndpointGroup updates the endpoint with the new endpointgroup specification for the given ID.
func (p *NetPlugin) UpdateEndpointGroup(id string) error {
	p.Lock()
	defer p.Unlock()
	return p.NetworkDriver.UpdateEndpointGroup(id)
}

// DeleteEndpoint destroys an endpoint for an ID.
func (p *NetPlugin) DeleteEndpoint(id string) error {
	p.Lock()
	defer p.Unlock()
	return p.NetworkDriver.DeleteEndpoint(id)
}

// CreateRemoteEndpoint creates an endpoint for a given ID.
func (p *NetPlugin) CreateRemoteEndpoint(id string) error {
	p.Lock()
	defer p.Unlock()
	return p.NetworkDriver.CreateRemoteEndpoint(id)
}

// DeleteRemoteEndpoint destroys an endpoint for an ID.
func (p *NetPlugin) DeleteRemoteEndpoint(id string) error {
	p.Lock()
	defer p.Unlock()
	return p.NetworkDriver.DeleteRemoteEndpoint(id)
}

// CreateHostAccPort creates a host access port
func (p *NetPlugin) CreateHostAccPort(portName, globalIP string) (string, error) {
	p.Lock()
	defer p.Unlock()
	return p.NetworkDriver.CreateHostAccPort(portName, globalIP, p.PluginConfig.Instance.HostPvtNW)
}

// DeleteHostAccPort creates a host access port
func (p *NetPlugin) DeleteHostAccPort(portName string) error {
	p.Lock()
	defer p.Unlock()
	return p.NetworkDriver.DeleteHostAccPort(portName)
}

// FetchEndpoint retrieves an endpoint's state for a given ID
func (p *NetPlugin) FetchEndpoint(id string) (core.State, error) {
	return nil, core.Errorf("Not implemented")
}

// AddPeerHost adds an peer host.
func (p *NetPlugin) AddPeerHost(node core.ServiceInfo) error {
	p.Lock()
	defer p.Unlock()
	return p.NetworkDriver.AddPeerHost(node)
}

// DeletePeerHost removes a peer host.
func (p *NetPlugin) DeletePeerHost(node core.ServiceInfo) error {
	p.Lock()
	defer p.Unlock()
	return p.NetworkDriver.DeletePeerHost(node)
}

// AddMaster adds a master node.
func (p *NetPlugin) AddMaster(node core.ServiceInfo) error {
	p.Lock()
	defer p.Unlock()
	return p.NetworkDriver.AddMaster(node)
}

// DeleteMaster removes a master node
func (p *NetPlugin) DeleteMaster(node core.ServiceInfo) error {
	p.Lock()
	defer p.Unlock()
	return p.NetworkDriver.DeleteMaster(node)
}

//AddBgp adds bgp configs
func (p *NetPlugin) AddBgp(id string) error {
	p.Lock()
	defer p.Unlock()
	return p.NetworkDriver.AddBgp(id)
}

//DeleteBgp deletes bgp configs
func (p *NetPlugin) DeleteBgp(id string) error {
	p.Lock()
	defer p.Unlock()
	return p.NetworkDriver.DeleteBgp(id)
}

//AddServiceLB adds service
func (p *NetPlugin) AddServiceLB(servicename string, spec *core.ServiceSpec) error {
	p.Lock()
	defer p.Unlock()
	return p.NetworkDriver.AddSvcSpec(servicename, spec)
}

//DeleteServiceLB deletes service
func (p *NetPlugin) DeleteServiceLB(servicename string, spec *core.ServiceSpec) error {
	p.Lock()
	defer p.Unlock()
	return p.NetworkDriver.DelSvcSpec(servicename, spec)
}

//SvcProviderUpdate function
func (p *NetPlugin) SvcProviderUpdate(servicename string, providers []string) {
	p.Lock()
	defer p.Unlock()
	p.NetworkDriver.SvcProviderUpdate(servicename, providers)
}

// GetEndpointStats returns all endpoint stats
func (p *NetPlugin) GetEndpointStats() ([]byte, error) {
	p.Lock()
	defer p.Unlock()
	return p.NetworkDriver.GetEndpointStats()
}

// InspectState returns current state of the plugin
func (p *NetPlugin) InspectState() ([]byte, error) {
	p.Lock()
	defer p.Unlock()
	return p.NetworkDriver.InspectState()
}

// InspectBgp returns current state of the plugin
func (p *NetPlugin) InspectBgp() ([]byte, error) {
	p.Lock()
	defer p.Unlock()
	return p.NetworkDriver.InspectBgp()
}

// InspectNameserver returns current state of the nameserver
func (p *NetPlugin) InspectNameserver() ([]byte, error) {
	p.Lock()
	defer p.Unlock()
	return p.NetworkDriver.InspectNameserver()
}

//GlobalConfigUpdate update global config
func (p *NetPlugin) GlobalConfigUpdate(cfg Config) error {
	p.Lock()
	defer p.Unlock()
	return p.NetworkDriver.GlobalConfigUpdate(cfg.Instance)
}

//Reinit reinitialize the network driver
func (p *NetPlugin) Reinit(cfg Config) {
	var err error

	p.Lock()
	defer p.Unlock()
	if p.NetworkDriver != nil {
		logrus.Infof("Reinit de-initializing NetworkDriver")
		p.NetworkDriver.Deinit()
		p.NetworkDriver = nil
	}

	cfg.Instance.StateDriver, _ = utils.GetStateDriver()
	p.NetworkDriver, err = utils.NewNetworkDriver(cfg.Drivers.Network, &cfg.Instance)
	logrus.Infof("Reinit Initializing NetworkDriver")

	if err != nil {
		logrus.Errorf("Reinit De-initializing due to error: %v", err)
		p.NetworkDriver.Deinit()
	}
}

//InitGlobalSettings initializes cluster-wide settings (e.g. fwd-mode)
func InitGlobalSettings(stateDriver core.StateDriver, inst *core.InstanceInfo) error {

	/*
		Query global settings from state store
		1. if forward mode or private net is empty, retry after 1 seccond
		2. if forward mode doesn't match, return error
		3. if stored private net is wrong, return error
	*/

	gCfg := mastercfg.GlobConfig{}
	gCfg.StateDriver = stateDriver

	// wait until able to get fwd mode and private subnet
	for {
		if err := gCfg.Read(""); err != nil {
			logrus.Warnf("Error reading global settings from cluster store, error: %v", err.Error())
		} else {
			if gCfg.FwdMode == "" || gCfg.PvtSubnet == "" {
				if gCfg.FwdMode == "" {
					logrus.Warnf("No forwarding mode found from cluster store")
				}
				if gCfg.PvtSubnet == "" {
					logrus.Warnf("No private subnet found from cluster store")
				}

			} else {
				logrus.Infof("Got global forwarding mode: %v", gCfg.FwdMode)
				logrus.Infof("Got global private subnet: %v", gCfg.PvtSubnet)
				break
			}
		}
		logrus.Warnf("Sleep 1 second and retry pulling global settings")
		time.Sleep(1 * time.Second)
	}

	// make sure local config matches netmaster config
	if inst.FwdMode != "" && inst.FwdMode != gCfg.FwdMode {
		err := fmt.Errorf("netplugin's local forward mode %q doesn't match global settings %q", inst.FwdMode, gCfg.FwdMode)
		logrus.Errorf(err.Error())
		return err
	}
	inst.FwdMode = gCfg.FwdMode

	logrus.Infof("Using forwarding mode: %v", inst.FwdMode)
	net, err := netutils.CIDRToMask(gCfg.PvtSubnet)
	if err != nil {
		err := fmt.Errorf("error convert private subnet %v from CIDR to mask, error %v", gCfg.PvtSubnet, err.Error())
		logrus.Errorf(err.Error())
		return err
	}
	inst.HostPvtNW = net
	logrus.Infof("Using host private subnet: %v", gCfg.PvtSubnet)
	return nil
}

//AddSvcSpec adds k8 service spec
func (p *NetPlugin) AddSvcSpec(svcName string, spec *core.ServiceSpec) {
	p.Lock()
	defer p.Unlock()
	p.NetworkDriver.AddSvcSpec(svcName, spec)
}

//DelSvcSpec deletes k8 service spec
func (p *NetPlugin) DelSvcSpec(svcName string, spec *core.ServiceSpec) {
	p.Lock()
	defer p.Unlock()
	p.NetworkDriver.DelSvcSpec(svcName, spec)
}

// AddPolicyRule creates a policy rule
func (p *NetPlugin) AddPolicyRule(id string) error {
	p.Lock()
	defer p.Unlock()
	return p.NetworkDriver.AddPolicyRule(id)
}

// DelPolicyRule creates a policy rule
func (p *NetPlugin) DelPolicyRule(id string) error {
	p.Lock()
	defer p.Unlock()
	return p.NetworkDriver.DelPolicyRule(id)
}
