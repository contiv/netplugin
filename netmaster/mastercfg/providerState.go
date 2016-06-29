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
	svcProviderPathPrefix = StateConfigPath + "provider/"
	svcProviderPath       = svcProviderPathPrefix + "%s"
)

//SvcProvider holds service information
type SvcProvider struct {
	core.CommonState
	ServiceName string
	Providers   []string
}

//ProviderDb is map of providers for a service keyed by provider ip
var ProviderDb = make(map[string]*Provider)

//Provider has providers info
type Provider struct {
	IPAddress   string            // provider IP
	ContainerID string            // container id
	Labels      map[string]string // lables
	Tenant      string
	Network     string
	Services    []string
	Container   string //container endpoint id
	EpIDKey     string
}

// Write the state
func (s *SvcProvider) Write() error {
	key := fmt.Sprintf(svcProviderPath, s.ID)
	err := s.StateDriver.WriteState(key, s, json.Marshal)
	return err
}

// Read the state in for a given ID.
func (s *SvcProvider) Read(id string) error {
	key := fmt.Sprintf(svcProviderPath, id)
	err := s.StateDriver.ReadState(key, s, json.Unmarshal)
	return err
}

// ReadAll reads all the state for master bgp configurations and returns it.
func (s *SvcProvider) ReadAll() ([]core.State, error) {
	return s.StateDriver.ReadAllState(svcProviderPathPrefix, s, json.Unmarshal)
}

// Clear removes the configuration from the state store.
func (s *SvcProvider) Clear() error {
	key := fmt.Sprintf(svcProviderPath, s.ID)
	err := s.StateDriver.ClearState(key)
	return err
}

// WatchAll state transitions and send them through the channel.
func (s *SvcProvider) WatchAll(rsps chan core.WatchState) error {
	return s.StateDriver.WatchAllState(svcProviderPathPrefix, s, json.Unmarshal,
		rsps)
}
