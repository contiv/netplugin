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

const (
	bgpConfigPathPrefix = StateConfigPath + "bgp/"
	bgpConfigPath       = bgpConfigPathPrefix + "%s"
)

// CfgBgpState is the router Bgp configuration for the host
type CfgBgpState struct {
	core.CommonState
	Hostname   string `json:"hostname"`
	RouterIP   string `json:"router-ip"`
	As         string `json:"as"`
	NeighborAs string `json:"neighbor-as"`
	Neighbor   string `json:"neighbor"`
}

// Write the state
func (s *CfgBgpState) Write() error {
	key := fmt.Sprintf(bgpConfigPath, s.Hostname)
	return s.StateDriver.WriteState(key, s, json.Marshal)
}

// Read the state in for a given ID.
func (s *CfgBgpState) Read(id string) error {
	key := fmt.Sprintf(bgpConfigPath, id)
	return s.StateDriver.ReadState(key, s, json.Unmarshal)
}

// ReadAll reads all the state for master bgp configurations and returns it.
func (s *CfgBgpState) ReadAll() ([]core.State, error) {
	return s.StateDriver.ReadAllState(bgpConfigPathPrefix, s, json.Unmarshal)
}

// Clear removes the configuration from the state store.
func (s *CfgBgpState) Clear() error {
	key := fmt.Sprintf(bgpConfigPath, s.Hostname)
	return s.StateDriver.ClearState(key)
}

// WatchAll state transitions and send them through the channel.
func (s *CfgBgpState) WatchAll(rsps chan core.WatchState) error {
	return s.StateDriver.WatchAllState(bgpConfigPathPrefix, s, json.Unmarshal,
		rsps)
}
