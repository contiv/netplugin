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
	"errors"
	"strconv"

	"github.com/contiv/contivmodel"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/docknet"
	"github.com/contiv/netplugin/netmaster/gstate"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/utils"

	log "github.com/Sirupsen/logrus"
)

// getEndpointGroupID returns endpoint group Id for a service
// It autocreates the endpoint group if it doesnt exist
func getEndpointGroupID(serviceName, networkName, tenantName string) (int, error) {
	// If service name is not specified, we are done
	if serviceName == "" {
		// FIXME: Need a better way to handle default epg for the network
		return 0, nil
	}

	// form the key based on network and service name.
	epgKey := tenantName + ":" + networkName + ":" + serviceName

	// See if the epg exists
	epg := contivModel.FindEndpointGroup(epgKey)
	if epg == nil {
		return 0, core.Errorf("EPG not created")
	}

	// return endpoint group id
	return epg.EndpointGroupID, nil
}

// CreateEndpointGroup handles creation of endpoint group
func CreateEndpointGroup(tenantName, networkName, groupName string, epgID int) error {
	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	// Read global config
	gCfg := gstate.Cfg{}
	gCfg.StateDriver = stateDriver
	err = gCfg.Read(tenantName)
	if err != nil {
		log.Errorf("error reading tenant cfg state. Error: %s", err)
		return err
	}

	// read the network config
	networkID := networkName + "." + tenantName
	nwCfg := &mastercfg.CfgNetworkState{}
	nwCfg.StateDriver = stateDriver
	err = nwCfg.Read(networkID)
	if err != nil {
		log.Errorf("Could not find network %s. Err: %v", networkID, err)
		return err
	}

	// params for docker network
	if GetClusterMode() == "docker" {
		// Create each EPG as a docker network
		err = docknet.CreateDockNet(tenantName, networkName, groupName, nwCfg)
		if err != nil {
			log.Errorf("Error creating docker network for group %s.%s. Err: %v", networkName, groupName, err)
			return err
		}
	}

	// Create epGroup state
	epgCfg := &mastercfg.EndpointGroupState{
		Name:        groupName,
		Tenant:      tenantName,
		NetworkName: networkName,
		PktTagType:  nwCfg.PktTagType,
		PktTag:      nwCfg.PktTag,
		ExtPktTag:   nwCfg.ExtPktTag,
	}

	epgCfg.StateDriver = stateDriver
	epgCfg.ID = strconv.Itoa(epgID)
	log.Debugf("##Create EpGroup %v network %v tagtype %v", groupName, networkName, nwCfg.PktTagType)

	// if aci mode allocate per-epg vlan. otherwise, stick to per-network vlan
	aciMode, rErr := IsAciConfigured()
	if rErr != nil {
		return rErr
	}

	// Special handling for ACI mode
	if aciMode {
		if epgCfg.PktTagType != "vlan" {
			log.Errorf("Network type must be VLAN for ACI mode")
			return errors.New("Network type must be VLAN for ACI mode")
		}

		pktTag, err := gCfg.AllocVLAN(0)
		if err != nil {
			return err
		}
		epgCfg.PktTag = int(pktTag)
		log.Debugf("ACI -- Allocated vlan %v for epg %v", pktTag, groupName)

	}

	err = epgCfg.Write()
	if err != nil {
		return err
	}

	return nil
}

// DeleteEndpointGroup handles endpoint group deletes
func DeleteEndpointGroup(epgID int) error {
	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	epgCfg := &mastercfg.EndpointGroupState{}
	epgCfg.StateDriver = stateDriver
	err = epgCfg.Read(strconv.Itoa(epgID))
	if err != nil {
		log.Errorf("EpGroup %v is not configured", epgID)
		return err
	}

	gCfg := gstate.Cfg{}
	gCfg.StateDriver = stateDriver
	err = gCfg.Read(epgCfg.Tenant)
	if err != nil {
		log.Errorf("error reading tenant cfg state. Error: %s", err)
		return err
	}

	// if aci mode we allocate per-epg vlan. free it here.
	aciMode, aErr := IsAciConfigured()
	if aErr != nil {
		return aErr
	}

	if aciMode {
		if epgCfg.PktTagType == "vlan" {
			err = gCfg.FreeVLAN(uint(epgCfg.PktTag))
			if err != nil {
				return err
			}
			log.Debugf("Freed vlan %v\n", epgCfg.PktTag)
		}
	}

	// Delete endpoint group
	err = epgCfg.Clear()
	if err != nil {
		log.Errorf("error writing epGroup config. Error: %v", err)
		return err
	}

	return docknet.DeleteDockNet(epgCfg.Tenant, epgCfg.NetworkName, epgCfg.Name)
}
