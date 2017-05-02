/***
Copyright 2015 Cisco Systems Inc. All rights reserved.

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

package mastercfg

import (
	"encoding/json"
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/core"
	"github.com/jainvipin/bitset"
)

// EndpointGroupState implements the State interface for endpoint group implemented using
// vlans with ovs. The state is stored as Json objects.
type EndpointGroupState struct {
	core.CommonState
	GroupName       string        `json:"groupName"`
	TenantName      string        `json:"tenantName"`
	NetworkName     string        `json:"networkName"`
	EndpointGroupID int           `json:"endpointGroupId"`
	PktTagType      string        `json:"pktTagType"`
	PktTag          int           `json:"pktTag"`
	ExtPktTag       int           `json:"extPktTag"`
	EpCount         int           `json:"epCount"` // To store endpoint Count
	DSCP            int           `json:"DSCP"`
	Bandwidth       string        `json:"Bandwidth"`
	Burst           int           `json:"Burst"`
	IPPool          string        `json:"IPPool"`
	EPGIPAllocMap   bitset.BitSet `json:"epgIpAllocMap"`
	GroupTag        string        `json:"groupTag"`
}

// Write the state.
func (s *EndpointGroupState) Write() error {
	key := fmt.Sprintf(epGroupConfigPath, s.ID)
	return s.StateDriver.WriteState(key, s, json.Marshal)
}

// Read the state for a given identifier
func (s *EndpointGroupState) Read(id string) error {
	key := fmt.Sprintf(epGroupConfigPath, id)
	return s.StateDriver.ReadState(key, s, json.Unmarshal)
}

// ReadAll state and return the collection.
func (s *EndpointGroupState) ReadAll() ([]core.State, error) {
	return s.StateDriver.ReadAllState(epGroupConfigPathPrefix, s, json.Unmarshal)
}

// WatchAll state transitions and send them through the channel.
func (s *EndpointGroupState) WatchAll(rsps chan core.WatchState) error {
	return s.StateDriver.WatchAllState(epGroupConfigPathPrefix, s, json.Unmarshal,
		rsps)
}

// Clear removes the state.
func (s *EndpointGroupState) Clear() error {
	key := fmt.Sprintf(epGroupConfigPath, s.ID)
	return s.StateDriver.ClearState(key)
}

// GetEndpointGroupKey returns endpoint group key
func GetEndpointGroupKey(groupName, tenantName string) string {
	if groupName == "" || tenantName == "" {
		return ""
	}

	return groupName + ":" + tenantName
}

// GetEndpointGroupID returns endpoint group Id for a service
// It autocreates the endpoint group if it doesnt exist
func GetEndpointGroupID(stateDriver core.StateDriver, groupName, tenantName string) (int, error) {
	// If service name is not specified, we are done
	if groupName == "" {
		return 0, nil
	}

	epgKey := GetEndpointGroupKey(groupName, tenantName)
	cfgEpGroup := &EndpointGroupState{}
	cfgEpGroup.StateDriver = stateDriver
	err := cfgEpGroup.Read(epgKey)
	if err != nil {
		log.Errorf("Error finding epg: %s. Err: %v", epgKey, err)
		return 0, core.Errorf("EPG not found")
	}

	// return endpoint group id
	return cfgEpGroup.EndpointGroupID, nil
}
