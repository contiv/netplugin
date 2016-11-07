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

package mastercfg

import (
	"encoding/json"
	"fmt"

	"github.com/contiv/netplugin/core"
)

// CfgEndpointState implements the State interface for an endpoint implemented using
// vlans with ovs. The state is stored as Json objects.
type CfgEndpointState struct {
	core.CommonState
	NetID            string            `json:"netID"`
	EndpointID       string            `json:"endpointID"`
	ServiceName      string            `json:"serviceName"`
	EndpointGroupID  int               `json:"endpointGroupId"`
	EndpointGroupKey string            `json:"endpointGroupKey"`
	IPAddress        string            `json:"ipAddress"`
	IPv6Address      string            `json:"ipv6Address"`
	MacAddress       string            `json:"macAddress"`
	HomingHost       string            `json:"homingHost"`
	IntfName         string            `json:"intfName"`
	VtepIP           string            `json:"vtepIP"`
	Labels           map[string]string `json:"labels"`
	ContainerID      string            `json:"containerId"`
	EPCommonName     string            `json:"epCommonName"`
}

// Write the state.
func (s *CfgEndpointState) Write() error {
	key := fmt.Sprintf(endpointConfigPath, s.ID)
	return s.StateDriver.WriteState(key, s, json.Marshal)
}

// Read the state for a given identifier.
func (s *CfgEndpointState) Read(id string) error {
	key := fmt.Sprintf(endpointConfigPath, id)
	return s.StateDriver.ReadState(key, s, json.Unmarshal)
}

// ReadAll reads all state objects for the endpoints.
func (s *CfgEndpointState) ReadAll() ([]core.State, error) {
	return s.StateDriver.ReadAllState(endpointConfigPathPrefix, s, json.Unmarshal)
}

// WatchAll fills a channel on each state event related to endpoints.
func (s *CfgEndpointState) WatchAll(rsps chan core.WatchState) error {
	return s.StateDriver.WatchAllState(endpointConfigPathPrefix, s, json.Unmarshal,
		rsps)
}

// Clear removes the state.
func (s *CfgEndpointState) Clear() error {
	key := fmt.Sprintf(endpointConfigPath, s.ID)
	return s.StateDriver.ClearState(key)
}
