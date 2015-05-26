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

package netmaster

import (
	"encoding/json"
	"fmt"

	"github.com/contiv/netplugin/core"
)

const (
	hostConfigPathPrefix = configPath + "hosts/"
	hostConfigPath       = hostConfigPathPrefix + "%s"
)

// MasterHostConfig is the state carried for network + host maps.
type MasterHostConfig struct {
	core.CommonState
	Name   string `json:"name"`
	Intf   string `json:"intf"`
	VtepIP string `json:"vtepIp"`
	NetID  string `json:"netId"`
}

// Write the master host configuration to the state system.
func (s *MasterHostConfig) Write() error {
	key := fmt.Sprintf(hostConfigPath, s.Name)
	return s.StateDriver.WriteState(key, s, json.Marshal)
}

// Read the host state from the state system, provided a hostname.
func (s *MasterHostConfig) Read(hostname string) error {
	key := fmt.Sprintf(hostConfigPath, hostname)
	return s.StateDriver.ReadState(key, s, json.Unmarshal)
}

// ReadAll implements reading the state for all hosts.
func (s *MasterHostConfig) ReadAll() ([]core.State, error) {
	return s.StateDriver.ReadAllState(hostConfigPathPrefix, s, json.Unmarshal)
}

// Clear purges the state for this host.
func (s *MasterHostConfig) Clear() error {
	key := fmt.Sprintf(hostConfigPath, s.Name)
	return s.StateDriver.ClearState(key)
}
