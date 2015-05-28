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

package netmaster

import (
	"encoding/json"
	"fmt"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/drivers"
)

const (
	basePath                = drivers.StateBasePath + "master/"
	configPath              = basePath + "config/"
	networkConfigPathPrefix = configPath + "nets/"
	networkConfigPath       = networkConfigPathPrefix + "%s"
)

// MasterNwConfig is the network configuration for a given tenant+network
type MasterNwConfig struct {
	core.CommonState
	Tenant     string `json:"tenant"`
	PktTagType string `json:"pktTagType"`
	PktTag     string `json:"pktTag"`
	SubnetIP   string `json:"subnetIP"`
	SubnetLen  uint   `json:"subnetLen"`
	DefaultGw  string `json:"defaultGw"`
}

// Write the state
func (s *MasterNwConfig) Write() error {
	key := fmt.Sprintf(networkConfigPath, s.ID)
	return s.StateDriver.WriteState(key, s, json.Marshal)
}

// Read the state in for a given ID.
func (s *MasterNwConfig) Read(id string) error {
	key := fmt.Sprintf(networkConfigPath, id)
	return s.StateDriver.ReadState(key, s, json.Unmarshal)
}

// ReadAll reads all the state for master network configurations and returns it.
func (s *MasterNwConfig) ReadAll() ([]core.State, error) {
	return s.StateDriver.ReadAllState(networkConfigPathPrefix, s, json.Unmarshal)
}

// Clear removes the configuration from the state store.
func (s *MasterNwConfig) Clear() error {
	key := fmt.Sprintf(networkConfigPath, s.ID)
	return s.StateDriver.ClearState(key)
}
