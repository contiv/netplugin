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

package drivers

import (
	"encoding/json"
	"fmt"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"reflect"
)

// OvsOperBgpState is the necessary data used to perform operations on endpoints.
type OvsOperBgpState struct {
	core.CommonState
	Name     string   `json:"name"`
	As       string   `json:"As"`
	Neighbor []string `json:"neighbor"`
}

// Matches matches the fields updated from configuration state
func (s *OvsOperBgpState) Matches(c *mastercfg.CfgBgpState) bool {
	return s.Name == c.Name &&
		s.As == c.As &&
		reflect.DeepEqual(s.Neighbor, c.Neighbor)
}

// Write the state.
func (s *OvsOperBgpState) Write() error {
	key := fmt.Sprintf(bgpOperPath, s.ID)
	return s.StateDriver.WriteState(key, s, json.Marshal)
}

// Read the state for a given identifier.
func (s *OvsOperBgpState) Read(id string) error {
	key := fmt.Sprintf(bgpOperPath, id)
	return s.StateDriver.ReadState(key, s, json.Unmarshal)
}

// ReadAll reads all state into separate objects.
func (s *OvsOperBgpState) ReadAll() ([]core.State, error) {
	return s.StateDriver.ReadAllState(bgpOperPathPrefix, s, json.Unmarshal)
}

// Clear removes the state.
func (s *OvsOperBgpState) Clear() error {
	key := fmt.Sprintf(bgpOperPath, s.ID)
	return s.StateDriver.ClearState(key)
}
