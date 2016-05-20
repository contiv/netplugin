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

package docknet

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/utils"
	"github.com/samalba/dockerclient"

	log "github.com/Sirupsen/logrus"
)

const (
	defaultTenantName = "default"
	docknetOperPrefix = mastercfg.StateOperPath + "docknet/"
	docknetOperPath   = docknetOperPrefix + "%s"
)

var netDriverName = "netplugin"
var ipamDriverName = "netplugin"

// OperState has oper state of docker network
type OperState struct {
	core.CommonState
	TenantName  string `json:"tenantName"`
	NetworkName string `json:"networkName"`
	ServiceName string `json:"serviceName"`
	DocknetUUID string `json:"docknetUUID"`
}

// Write the state.
func (s *OperState) Write() error {
	key := fmt.Sprintf(docknetOperPath, s.ID)
	return s.StateDriver.WriteState(key, s, json.Marshal)
}

// Read the state for a given identifier
func (s *OperState) Read(id string) error {
	key := fmt.Sprintf(docknetOperPath, id)
	return s.StateDriver.ReadState(key, s, json.Unmarshal)
}

// ReadAll state and return the collection.
func (s *OperState) ReadAll() ([]core.State, error) {
	return s.StateDriver.ReadAllState(docknetOperPrefix, s, json.Unmarshal)
}

// WatchAll state transitions and send them through the channel.
func (s *OperState) WatchAll(rsps chan core.WatchState) error {
	return s.StateDriver.WatchAllState(docknetOperPrefix, s, json.Unmarshal,
		rsps)
}

// Clear removes the state.
func (s *OperState) Clear() error {
	key := fmt.Sprintf(docknetOperPath, s.ID)
	return s.StateDriver.ClearState(key)
}

// GetDocknetName trims default tenant from network name
func GetDocknetName(tenantName, networkName, epgName string) string {

	netName := ""
	// if epg is specified, always use that, else use nw
	if epgName == "" {
		netName = networkName
	} else {
		netName = epgName
	}

	// add tenant suffix if not the default tenant
	if tenantName != defaultTenantName {
		netName = netName + "/" + tenantName
	}

	return netName
}

// CreateDockNet Creates a network in docker daemon
func CreateDockNet(tenantName, networkName, serviceName string, nwCfg *mastercfg.CfgNetworkState) error {
	var nwID string

	// Trim default tenant name
	docknetName := GetDocknetName(tenantName, networkName, serviceName)

	// connect to docker
	docker, err := dockerclient.NewDockerClient("unix:///var/run/docker.sock", nil)
	if err != nil {
		log.Errorf("Unable to connect to docker. Error %v", err)
		return errors.New("Unable to connect to docker")
	}

	// Check if the network already exists
	nw, err := docker.InspectNetwork(docknetName)
	if err == nil && nw.Driver == netDriverName {
		log.Infof("docker network: %s already exists", docknetName)
		nwID = nw.ID
	} else if err == nil && nw.Driver != netDriverName {
		log.Errorf("Network name %s used by another driver %s", docknetName, nw.Driver)
		return errors.New("Network name used by another driver")
	} else if err != nil {
		// plugin options to be sent to docker
		netPluginOptions := make(map[string]string)
		netPluginOptions["tenant"] = nwCfg.Tenant
		netPluginOptions["encap"] = nwCfg.PktTagType
		if nwCfg.PktTagType == "vxlan" {
			netPluginOptions["pkt-tag"] = strconv.Itoa(nwCfg.ExtPktTag)
		} else {
			netPluginOptions["pkt-tag"] = strconv.Itoa(nwCfg.PktTag)
		}

		subnetCIDR := fmt.Sprintf("%s/%d", nwCfg.SubnetIP, nwCfg.SubnetLen)

		ipamOptions := make(map[string]string)
		ipamOptions["tenant"] = nwCfg.Tenant
		ipamOptions["network"] = nwCfg.NetworkName

		// Build network parameters
		nwCreate := dockerclient.NetworkCreate{
			Name:           docknetName,
			CheckDuplicate: true,
			Driver:         netDriverName,
			IPAM: dockerclient.IPAM{
				Driver: ipamDriverName,
				Config: []dockerclient.IPAMConfig{
					dockerclient.IPAMConfig{
						Subnet:  subnetCIDR,
						Gateway: nwCfg.Gateway,
					},
				},
				Options: ipamOptions,
			},
			Options: netPluginOptions,
		}

		log.Infof("Creating docker network: %+v", nwCreate)

		// Create network
		resp, err := docker.CreateNetwork(&nwCreate)
		if err != nil {
			log.Errorf("Error creating network %s. Err: %v", docknetName, err)
			return err
		}

		nwID = resp.ID
	}

	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		log.Warnf("Couldn't read global config %v", err)
		return err
	}

	// save docknet oper state
	dnetOper := OperState{
		TenantName:  tenantName,
		NetworkName: networkName,
		ServiceName: serviceName,
		DocknetUUID: nwID,
	}
	dnetOper.ID = fmt.Sprintf("%s.%s.%s", tenantName, networkName, serviceName)
	dnetOper.StateDriver = stateDriver

	// write the dnet oper state
	err = dnetOper.Write()
	if err != nil {
		return err
	}

	return nil
}

// DeleteDockNet deletes a network in docker daemon
func DeleteDockNet(tenantName, networkName, serviceName string) error {
	// Trim default tenant name
	docknetName := GetDocknetName(tenantName, networkName, serviceName)

	// connect to docker
	docker, err := dockerclient.NewDockerClient("unix:///var/run/docker.sock", nil)
	if err != nil {
		log.Errorf("Unable to connect to docker. Error %v", err)
		return errors.New("Unable to connect to docker")
	}

	log.Infof("Deleting docker network: %+v", docknetName)

	// Delete network
	err = docker.RemoveNetwork(docknetName)
	if err != nil {
		log.Errorf("Error deleting network %s. Err: %v", docknetName, err)
		return err
	}

	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		log.Warnf("Couldn't read global config %v", err)
		return err
	}

	// save docknet oper state
	dnetOper := OperState{}
	dnetOper.ID = fmt.Sprintf("%s.%s.%s", tenantName, networkName, serviceName)
	dnetOper.StateDriver = stateDriver

	// write the dnet oper state
	err = dnetOper.Clear()
	if err != nil {
		return err
	}

	return nil
}

// FindDocknetByUUID find the docknet by UUID
func FindDocknetByUUID(dnetID string) (*OperState, error) {
	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		log.Warnf("Couldn't read global config %v", err)
		return nil, err
	}

	tmpDnet := OperState{}
	tmpDnet.StateDriver = stateDriver
	dnetOperList, err := tmpDnet.ReadAll()
	if err != nil {
		log.Errorf("Error getting docknet list. Err: %v", err)
		return nil, err
	}

	// Walk all dnets and find the matching UUID
	for _, dnet := range dnetOperList {
		if dnet.(*OperState).DocknetUUID == dnetID {
			return dnet.(*OperState), nil
		}
	}

	return nil, errors.New("docknet UUID not found")
}
