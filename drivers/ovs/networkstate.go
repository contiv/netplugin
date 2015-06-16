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

package ovs

import (
	"encoding/json"
	"fmt"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/drivers"
	"github.com/jainvipin/bitset"
)

// CfgNetworkState implements the State interface for a network implemented using
// vlans with ovs. The state is stored as Json objects.
type CfgNetworkState struct {
	core.CommonState
	Tenant     string        `json:"tenant"`
	PktTagType string        `json:"pktTagType"`
	PktTag     int           `json:"pktTag"`
	ExtPktTag  int           `json:"extPktTag"`
	SubnetIP   string        `json:"subnetIP"`
	SubnetLen  uint          `json:"subnetLen"`
	DefaultGw  string        `json:"defaultGw"`
	EpCount    int           `json:"epCount"`
	IPAllocMap bitset.BitSet `json:"ipAllocMap"`
}

// Write the state.
func (s *CfgNetworkState) Write() error {
	key := fmt.Sprintf(drivers.NetworkConfigPath, s.ID)
	return s.StateDriver.WriteState(key, s, json.Marshal)
}

// Read the state for a given identifier
func (s *CfgNetworkState) Read(id string) error {
	key := fmt.Sprintf(drivers.NetworkConfigPath, id)
	return s.StateDriver.ReadState(key, s, json.Unmarshal)
}

// ReadAll state and return the collection.
func (s *CfgNetworkState) ReadAll() ([]core.State, error) {
	return s.StateDriver.ReadAllState(drivers.NetworkConfigPathPrefix, s, json.Unmarshal)
}

// WatchAll state transitions and send them through the channel.
func (s *CfgNetworkState) WatchAll(rsps chan core.WatchState) error {
	return s.StateDriver.WatchAllState(drivers.NetworkConfigPathPrefix, s, json.Unmarshal,
		rsps)
}

// Clear removes the state.
func (s *CfgNetworkState) Clear() error {
	key := fmt.Sprintf(drivers.NetworkConfigPath, s.ID)
	return s.StateDriver.ClearState(key)
}
