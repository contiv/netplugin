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

	"github.com/contiv/netplugin/core"
)

// EndpointGroupState implements the State interface for endpoint group implemented using
// vlans with ovs. The state is stored as Json objects.
type EndpointGroupState struct {
	core.CommonState
	Name        string `json:"name"`
	Tenant      string `json:"tenant"`
	NetworkName string `json:"networkName"`
	PktTagType  string `json:"pktTagType"`
	PktTag      int    `json:"pktTag"`
	ExtPktTag   int    `json:"extPktTag"`
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
	key := fmt.Sprintf(networkConfigPath, s.ID)
	return s.StateDriver.ClearState(key)
}
