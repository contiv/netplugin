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

package master

import (
	"fmt"
	"net"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/utils"

	log "github.com/Sirupsen/logrus"
)

func validateEndpointConfig(stateDriver core.StateDriver, tenant *intent.ConfigTenant) error {
	var err error

	if tenant.Name == "" {
		return core.Errorf("null tenant name")
	}

	for _, network := range tenant.Networks {
		if network.Name == "" {
			core.Errorf("null network name")
		}

		for _, ep := range network.Endpoints {
			if ep.Container == "" {
				return core.Errorf("invalid container name for the endpoint")
			}
			if ep.IPAddress != "" {
				if network.SubnetCIDR != "" {
					log.Errorf("found ep with ip for auto-allocated net")
					return core.Errorf("found ep with ip for auto-allocated net")
				}
				if net.ParseIP(ep.IPAddress) == nil {
					return core.Errorf("invalid ep IP")
				}
			}
		}
	}

	return err
}

func allocSetEpAddress(ep *intent.ConfigEP, epCfg *mastercfg.CfgEndpointState,
	nwCfg *mastercfg.CfgNetworkState) (err error) {

	ipAddress, err := networkAllocAddress(nwCfg, ep.IPAddress)
	if err != nil {
		log.Errorf("Error allocating IP address. Err: %v", err)
		return
	}

	epCfg.IPAddress = ipAddress

	// Set mac address which is derived from IP address
	ipAddr := net.ParseIP(ipAddress)
	macAddr := fmt.Sprintf("02:02:%02x:%02x:%02x:%02x", ipAddr[12], ipAddr[13], ipAddr[14], ipAddr[15])
	epCfg.MacAddress = macAddr

	return
}

// CreateEndpoint creates an endpoint
func CreateEndpoint(stateDriver core.StateDriver, nwCfg *mastercfg.CfgNetworkState,
	ep *intent.ConfigEP) (*mastercfg.CfgEndpointState, error) {
	epCfg := &mastercfg.CfgEndpointState{}
	epCfg.StateDriver = stateDriver
	epCfg.ID = getEpName(nwCfg.ID, ep)
	err := epCfg.Read(epCfg.ID)
	if err == nil {
		// TODO: check for diffs and possible updates
		return epCfg, nil
	}

	epCfg.NetID = nwCfg.ID
	epCfg.ContName = ep.Container
	epCfg.AttachUUID = ep.AttachUUID
	epCfg.HomingHost = ep.Host
	epCfg.ServiceName = ep.ServiceName

	// Allocate addresses
	err = allocSetEpAddress(ep, epCfg, nwCfg)
	if err != nil {
		log.Errorf("error allocating and/or reserving IP. Error: %s", err)
		return nil, err
	}

	// Set endpoint group
	// Skip for infra nw
	if nwCfg.NwType != "infra" {
		epCfg.EndpointGroupKey = mastercfg.GetEndpointGroupKey(ep.ServiceName, nwCfg.Tenant)
		epCfg.EndpointGroupID, err = mastercfg.GetEndpointGroupID(stateDriver, ep.ServiceName, nwCfg.Tenant)
		if err != nil {
			log.Errorf("Error getting endpoint group for %s.%s. Err: %v", ep.ServiceName, nwCfg.ID, err)
			return nil, err
		}
	}

	err = nwCfg.IncrEpCount()
	if err != nil {
		log.Errorf("Error incrementing ep count. Err: %v", err)
		return nil, err
	}

	err = epCfg.Write()
	if err != nil {
		log.Errorf("error writing ep config. Error: %s", err)
		return nil, err
	}

	return epCfg, nil
}

// CreateEndpoints creates the endpoints for a given tenant.
func CreateEndpoints(stateDriver core.StateDriver, tenant *intent.ConfigTenant) error {
	err := validateEndpointConfig(stateDriver, tenant)
	if err != nil {
		log.Errorf("error validating endpoint config. Error: %s", err)
		return err
	}

	for _, network := range tenant.Networks {
		nwCfg := &mastercfg.CfgNetworkState{}
		nwCfg.StateDriver = stateDriver
		networkID := network.Name + "." + tenant.Name
		err = nwCfg.Read(networkID)
		if err != nil {
			log.Errorf("error reading oper network %s. Error: %s", network.Name, err)
			return err
		}

		for _, ep := range network.Endpoints {
			_, err = CreateEndpoint(stateDriver, nwCfg, &ep)
			if err != nil {
				log.Errorf("Error creating endpoint %+v. Err: %v", ep, err)
				return err
			}
		}

		err = nwCfg.Write()
		if err != nil {
			log.Errorf("error writing nw config. Error: %s", err)
			return err
		}
	}

	return err
}

// DeleteEndpointID deletes an endpoint by ID.
func DeleteEndpointID(stateDriver core.StateDriver, epID string) (*mastercfg.CfgEndpointState, error) {
	epCfg := &mastercfg.CfgEndpointState{}
	epCfg.StateDriver = stateDriver
	err := epCfg.Read(epID)
	if err != nil {
		return nil, err
	}

	nwCfg := &mastercfg.CfgNetworkState{}
	nwCfg.StateDriver = stateDriver
	err = nwCfg.Read(epCfg.NetID)

	// Network may already be deleted if infra nw
	// If network present, free up nw resources
	if err == nil && epCfg.IPAddress != "" {
		err = networkReleaseAddress(nwCfg, epCfg.IPAddress)
		if err != nil {
			log.Errorf("Error releasing endpoint state for: %s. Err: %v", epCfg.IPAddress, err)
		}

		// decrement ep count
		nwCfg.EpCount--

		// write modified nw state
		err = nwCfg.Write()
		if err != nil {
			log.Errorf("error writing nw config. Error: %s", err)
		}
	}

	// Even if network not present (already deleted), cleanup ep cfg
	err = epCfg.Clear()
	if err != nil {
		log.Errorf("error writing ep config. Error: %s", err)
		return nil, err
	}

	return epCfg, err
}

func validateEpBindings(epBindings *[]intent.ConfigEP) error {
	for _, ep := range *epBindings {
		if ep.Host == "" {
			return core.Errorf("invalid host name for the endpoint")
		}
		if ep.Container == "" {
			return core.Errorf("invalid container name for the endpoint")
		}
	}

	return nil
}

// CreateEpBindings binds an endpoint to a host by updating host-label info
// in driver's endpoint configuration.
func CreateEpBindings(epBindings *[]intent.ConfigEP) error {
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	err = validateEpBindings(epBindings)
	if err != nil {
		log.Errorf("error validating the ep bindings. Error: %s", err)
		return err
	}

	readEp := &mastercfg.CfgEndpointState{}
	readEp.StateDriver = stateDriver
	epCfgs, err := readEp.ReadAll()
	if err != nil {
		log.Errorf("error fetching eps. Error: %s", err)
		return err
	}
	for _, ep := range *epBindings {
		log.Infof("creating binding between container '%s' and host '%s'",
			ep.Container, ep.Host)

		for _, epCfg := range epCfgs {
			cfg := epCfg.(*mastercfg.CfgEndpointState)
			if cfg.ContName != ep.Container {
				continue
			}
			cfg.HomingHost = ep.Host
			cfg.AttachUUID = ep.AttachUUID
			err = cfg.Write()
			if err != nil {
				log.Errorf("error updating epCfg. Error: %s", err)
				return err
			}
		}
	}

	return nil
}
