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

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/docknet"
	"github.com/contiv/netplugin/netmaster/gstate"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/utils"

	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/utils/netutils"
	"strings"
)

const maxEpgID = 65535

// FIXME: hack to allocate unique endpoint group ids
var globalEpgID = 1

// CreateEndpointGroup handles creation of endpoint group
func CreateEndpointGroup(tenantName, networkName, groupName, ipPool, cfgdTag string) error {
	var epgID int

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

	// check epg range is with in network
	if len(ipPool) > 0 {
		if netutils.IsIPv6(ipPool) == true {
			return fmt.Errorf("ipv6 address pool is not supported for Endpoint Groups")
		}

		if err = netutils.ValidateNetworkRangeParams(ipPool, nwCfg.SubnetLen); err != nil {
			return fmt.Errorf("invalid ip-pool %s", ipPool)
		}

		addrRangeList := strings.Split(ipPool, "-")
		if _, err := netutils.GetIPNumber(nwCfg.SubnetIP, nwCfg.SubnetLen, 32, addrRangeList[0]); err != nil {
			return fmt.Errorf("bad ip-pool %s, EPG ip-pool must be a subset of network %s/%d", ipPool, nwCfg.SubnetIP,
				nwCfg.SubnetLen)
		}
		if _, err := netutils.GetIPNumber(nwCfg.SubnetIP, nwCfg.SubnetLen, 32, addrRangeList[1]); err != nil {
			return fmt.Errorf("bad ip-pool %s, EPG ip-pool must be a subset of network %s/%d", ipPool, nwCfg.SubnetIP,
				nwCfg.SubnetLen)
		}

		if err := netutils.TestIPAddrRange(&nwCfg.IPAllocMap, ipPool, nwCfg.SubnetIP,
			nwCfg.SubnetLen); err != nil {
			return err
		}
	}

	// if there is no label given generate one for the epg
	epgTag := cfgdTag
	if epgTag == "" {
		epgTag = groupName + "." + tenantName
	}

	// params for docker network
	if GetClusterMode() == core.Docker {
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
		IPPool:          ipPool,
		EndpointGroupID: epgID,
		PktTagType:      nwCfg.PktTagType,
		PktTag:          nwCfg.PktTag,
		ExtPktTag:       nwCfg.ExtPktTag,
		GroupTag:        epgTag,
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
			return errors.New("network type must be VLAN for ACI mode")
		}

		pktTag, err := gCfg.AllocVLAN(0)
		if err != nil {
			return err
		}
		epgCfg.PktTag = int(pktTag)
		log.Debugf("ACI -- Allocated vlan %v for epg %v", pktTag, groupName)

	}

	if len(ipPool) > 0 {
		// mark range as used
		netutils.SetIPAddrRange(&nwCfg.IPAllocMap, ipPool, nwCfg.SubnetIP, nwCfg.SubnetLen)

		if err := nwCfg.Write(); err != nil {
			return fmt.Errorf("updating epg ipaddress in network failed: %s", err)
		}
		netutils.InitSubnetBitset(&epgCfg.EPGIPAllocMap, nwCfg.SubnetLen)
		netutils.SetBitsOutsideRange(&epgCfg.EPGIPAllocMap, ipPool, nwCfg.SubnetLen)
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

	networkID := epgCfg.NetworkName + "." + epgCfg.TenantName
	nwCfg := &mastercfg.CfgNetworkState{}
	nwCfg.StateDriver = stateDriver
	err = nwCfg.Read(networkID)
	if err != nil {
		log.Errorf("Could not find network %s. Err: %v", networkID, err)
		return err
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

	// mark it as unused
	if len(epgCfg.IPPool) > 0 {
		netutils.ClearIPAddrRange(&nwCfg.IPAllocMap, epgCfg.IPPool, nwCfg.SubnetIP, nwCfg.SubnetLen)
		if err = nwCfg.Write(); err != nil {
			log.Errorf("error writing nw config after releasing subnet. Error: %v", err)
			return err
		}
	}

	// Delete endpoint group
	err = epgCfg.Clear()
	if err != nil {
		log.Errorf("error writing epGroup config. Error: %v", err)
		return err
	}

	if GetClusterMode() == core.Docker {
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
		return errors.New("error finding endpointGroup key ")
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
