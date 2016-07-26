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

	"github.com/contiv/netplugin/netmaster/docknet"
	"github.com/contiv/netplugin/netmaster/gstate"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/utils"

	log "github.com/Sirupsen/logrus"
)

const maxEpgID = 65535

// FIXME: hack to allocate unique endpoint group ids
var globalEpgID = 1

// CreateEndpointGroup handles creation of endpoint group
func CreateEndpointGroup(tenantName, networkName, groupName string) error {
	var epgID int

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

	// assign unique endpoint group ids
	// FIXME: This is a hack. need to add a epgID resource
	for i := 0; i < maxEpgID; i++ {
		epgID = globalEpgID
		globalEpgID = globalEpgID + 1
		if globalEpgID > maxEpgID {
			globalEpgID = 1
		}
		epgCfg := &mastercfg.EndpointGroupState{}
		epgCfg.StateDriver = stateDriver
		err = epgCfg.Read(strconv.Itoa(epgID))
		if err != nil {
			break
		}
	}

	// Create epGroup state
	epgCfg := &mastercfg.EndpointGroupState{
		GroupName:       groupName,
		TenantName:      tenantName,
		NetworkName:     networkName,
		EndpointGroupID: epgID,
		PktTagType:      nwCfg.PktTagType,
		PktTag:          nwCfg.PktTag,
		ExtPktTag:       nwCfg.ExtPktTag,
	}

	epgCfg.StateDriver = stateDriver
	epgCfg.ID = mastercfg.GetEndpointGroupKey(groupName, tenantName)
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
func DeleteEndpointGroup(tenantName, groupName string) error {
	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	epgKey := mastercfg.GetEndpointGroupKey(groupName, tenantName)
	epgCfg := &mastercfg.EndpointGroupState{}
	epgCfg.StateDriver = stateDriver
	err = epgCfg.Read(epgKey)
	if err != nil {
		log.Errorf("error reading EPG key %s. Error: %s", epgKey, err)
		return err
	}

	// Delete the endpoint group state
	gCfg := gstate.Cfg{}
	gCfg.StateDriver = stateDriver
	err = gCfg.Read(epgCfg.TenantName)
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

	return docknet.DeleteDockNet(epgCfg.TenantName, epgCfg.NetworkName, epgCfg.GroupName)
}
