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
)

// CfgEndpointState implements the State interface for an endpoint implemented using
// vlans with ovs. The state is stored as Json objects.
type CfgEndpointState struct {
	core.CommonState
	NetID      string `json:"netID"`
	ContName   string `json:"contName"`
	AttachUUID string `json:"attachUUID"`
	IPAddress  string `json:"ipAddress"`
	HomingHost string `json:"homingHost"`
	IntfName   string `json:"intfName"`
	VtepIP     string `json:"vtepIP"`
}

// Write the state.
func (s *CfgEndpointState) Write() error {
	key := fmt.Sprintf(drivers.EndpointConfigPath, s.ID)
	return s.StateDriver.WriteState(key, s, json.Marshal)
}

// Read the state for a given identifier.
func (s *CfgEndpointState) Read(id string) error {
	key := fmt.Sprintf(drivers.EndpointConfigPath, id)
	return s.StateDriver.ReadState(key, s, json.Unmarshal)
}

// ReadAll reads all state objects for the endpoints.
func (s *CfgEndpointState) ReadAll() ([]core.State, error) {
	return s.StateDriver.ReadAllState(drivers.EndpointConfigPathPrefix, s, json.Unmarshal)
}

// WatchAll fills a channel on each state event related to endpoints.
func (s *CfgEndpointState) WatchAll(rsps chan core.WatchState) error {
	return s.StateDriver.WatchAllState(drivers.EndpointConfigPathPrefix, s, json.Unmarshal, rsps)
}

// Clear removes the state.
func (s *CfgEndpointState) Clear() error {
	key := fmt.Sprintf(drivers.EndpointConfigPath, s.ID)
	return s.StateDriver.ClearState(key)
}

// OperEndpointState is the necessary data used to perform operations on endpoints.
type OperEndpointState struct {
	core.CommonState
	NetID      string `json:"netID"`
	ContName   string `json:"contName"`
	AttachUUID string `json:"attachUUID"`
	IPAddress  string `json:"ipAddress"`
	PortName   string `json:"portName"`
	HomingHost string `json:"homingHost"`
	IntfName   string `json:"intfName"`
	VtepIP     string `json:"vtepIP"`
}

// Matches matches the fields updated from configuration state
func (s *OperEndpointState) Matches(c *CfgEndpointState) bool {
	return s.NetID == c.NetID &&
		s.ContName == c.ContName &&
		s.AttachUUID == c.AttachUUID &&
		s.IPAddress == c.IPAddress &&
		s.HomingHost == c.HomingHost &&
		s.IntfName == c.IntfName &&
		s.VtepIP == c.VtepIP
}

// Write the state.
func (s *OperEndpointState) Write() error {
	key := fmt.Sprintf(drivers.EndpointOperPath, s.ID)
	return s.StateDriver.WriteState(key, s, json.Marshal)
}

// Read the state for a given identifier.
func (s *OperEndpointState) Read(id string) error {
	key := fmt.Sprintf(drivers.EndpointOperPath, id)
	return s.StateDriver.ReadState(key, s, json.Unmarshal)
}

// ReadAll reads all state into separate objects.
func (s *OperEndpointState) ReadAll() ([]core.State, error) {
	return s.StateDriver.ReadAllState(drivers.EndpointOperPathPrefix, s, json.Unmarshal)
}

// Clear removes the state.
func (s *OperEndpointState) Clear() error {
	key := fmt.Sprintf(drivers.EndpointOperPath, s.ID)
	return s.StateDriver.ClearState(key)
}
