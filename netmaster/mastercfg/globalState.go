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

// netmaster  - implements the network intent translation to plugin
// events; uses state distribution to achieve intent realization
// netmaster runs as a logically centralized unit on in the cluster

package mastercfg

import (
	"encoding/json"

	"github.com/contiv/netplugin/core"
)

const (
	gBasePath              = StateBasePath + "master/"
	gConfigPath            = gBasePath + "config/"
	globalConfigPathPrefix = gConfigPath
	globalConfigPath       = globalConfigPathPrefix + "global"
)

// GlobConfig is the global configuration applicable to everything
type GlobConfig struct {
	core.CommonState
	NwInfraType string `json:"nw-infra-type"`
	FwdMode     string `json:"fwd-mode"`
	ArpMode     string `json:"arp-mode"`
	PvtSubnet   string `json:"pvt-subnet"`
}

//OldResState is used for global resource update
type OldResState struct {
	VLANs  string
	VXLANs string
}

// Write the state
func (s *GlobConfig) Write() error {
	key := globalConfigPath
	return s.StateDriver.WriteState(key, s, json.Marshal)
}

// Read the state in for a given ID.
func (s *GlobConfig) Read(id string) error {
	key := globalConfigPath
	return s.StateDriver.ReadState(key, s, json.Unmarshal)
}

// ReadAll reads all the state for master global configurations and returns it.
func (s *GlobConfig) ReadAll() ([]core.State, error) {
	return s.StateDriver.ReadAllState(globalConfigPathPrefix, s, json.Unmarshal)
}

// Clear removes the configuration from the state store.
func (s *GlobConfig) Clear() error {
	key := globalConfigPath
	return s.StateDriver.ClearState(key)
}

// WatchAll state transitions and send them through the channel.
func (s *GlobConfig) WatchAll(rsps chan core.WatchState) error {
	return s.StateDriver.WatchAllState(globalConfigPathPrefix, s, json.Unmarshal,
		rsps)
}
