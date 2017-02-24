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

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/docknet"
	"github.com/contiv/netplugin/netmaster/gstate"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/utils"

	log "github.com/Sirupsen/logrus"
)

// CreateEndpointGroup handles creation of endpoint group
func CreateEndpointGroup(tenantName, networkName, groupName string) error {

	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	// Read global config
	gstate.GlobalMutex.Lock()
	defer gstate.GlobalMutex.Unlock()
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

	epgID, err := gCfg.AllocNextEPG()
	if err != nil {
		log.Errorf("Error allocating EPG ID")
		return err
	}
	log.Debugf("Allocated EPG ID: %d for EPG: %s", epgID, groupName)

	// Create epGroup state
	epgCfg := &mastercfg.EndpointGroupState{
		GroupName:       groupName,
		TenantName:      tenantName,
		NetworkName:     networkName,
		EndpointGroupID: int(epgID),
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
	return epgCfg.Write()
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

	if epgCfg.EpCount != 0 {
		return core.Errorf("Error: EPG %s has active endpoints", groupName)
	}

	// Delete the endpoint group state
	gstate.GlobalMutex.Lock()
	defer gstate.GlobalMutex.Unlock()
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

	err = gCfg.FreeEPG(uint(epgCfg.EndpointGroupID))
	if err != nil {
		log.Errorf("Could not free EPG ID: %d. Err: %+v", epgCfg.EndpointGroupID, err)
	}
	log.Debugf("Freeing EPG ID: %d", epgCfg.EndpointGroupID)

	// Delete endpoint group
	err = epgCfg.Clear()
	if err != nil {
		log.Errorf("error writing epGroup config. Error: %v", err)
		return err
	}

	if GetClusterMode() == "docker" {
		return docknet.DeleteDockNet(epgCfg.TenantName, epgCfg.NetworkName, epgCfg.GroupName)
	}
	return nil
}

//UpdateEndpointGroup updates the endpointgroups
func UpdateEndpointGroup(bandwidth, groupName, tenantName string, Dscp, burst int) error {

	// Get the state driver - get the etcd driver state
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	key := mastercfg.GetEndpointGroupKey(groupName, tenantName)
	if key == "" {
		return errors.New("Error finding endpointGroup key ")
	}

	// Read etcd driver
	epCfg := mastercfg.EndpointGroupState{}
	epCfg.StateDriver = stateDriver

	err = epCfg.Read(key)
	if err != nil {
		log.Errorf("Error finding endpointgroup %s. Err: %v", key, err)
		return err
	}

	//update the epGroup state
	epCfg.DSCP = Dscp
	epCfg.Bandwidth = bandwidth
	epCfg.Burst = burst

	//Write to etcd
	return epCfg.Write()
}
