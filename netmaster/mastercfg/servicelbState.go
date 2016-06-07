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
	"sync"
)

const (
	serviceLBConfigPathPrefix = StateConfigPath + "serviceLB/"
	serviceLBConfigPath       = serviceLBConfigPathPrefix + "%s"
)

//ServiceLBInfo holds service information
type ServiceLBInfo struct {
	ServiceName string               //Service name
	IPAddress   string               //Service IP
	Tenant      string               //Tenant name of the service
	Network     string               // service network
	Ports       []string             //Service_port:Provider_port:protocol
	Selectors   map[string]string    // selector labels associated with a service
	Providers   map[string]*Provider //map of providers for a service keyed by provider ip
}

//ServiceLBDb is map of all services
var ServiceLBDb = make(map[string]*ServiceLBInfo) //DB for all services keyed by servicename.tenant

//SvcMutex is mutex for service transaction
var SvcMutex sync.RWMutex

// CfgServiceLBState is the service object configuration
type CfgServiceLBState struct {
	core.CommonState
	ServiceName string               `json:"servicename"`
	Tenant      string               `json:"tenantname"`
	Network     string               `json:"subnet"`
	Ports       []string             `json:"ports"`
	Selectors   map[string]string    `json:"selectors"`
	IPAddress   string               `json:"ipaddress"`
	Providers   map[string]*Provider `json:"providers"`
}

// Write the state
func (s *CfgServiceLBState) Write() error {
	key := fmt.Sprintf(serviceLBConfigPath, s.ID)
	err := s.StateDriver.WriteState(key, s, json.Marshal)
	return err
}

// Read the state in for a given ID.
func (s *CfgServiceLBState) Read(id string) error {
	key := fmt.Sprintf(serviceLBConfigPath, id)
	err := s.StateDriver.ReadState(key, s, json.Unmarshal)
	return err
}

// ReadAll reads all the state for master bgp configurations and returns it.
func (s *CfgServiceLBState) ReadAll() ([]core.State, error) {
	return s.StateDriver.ReadAllState(serviceLBConfigPathPrefix, s, json.Unmarshal)
}

// Clear removes the configuration from the state store.
func (s *CfgServiceLBState) Clear() error {
	key := fmt.Sprintf(serviceLBConfigPath, s.ID)
	err := s.StateDriver.ClearState(key)
	return err
}

// WatchAll state transitions and send them through the channel.
func (s *CfgServiceLBState) WatchAll(rsps chan core.WatchState) error {
	return s.StateDriver.WatchAllState(serviceLBConfigPathPrefix, s, json.Unmarshal,
		rsps)
}
