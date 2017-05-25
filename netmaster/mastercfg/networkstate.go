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
	"github.com/jainvipin/bitset"
)

const (
	// StateBasePath is the base path for all state operations.
	StateBasePath = "/contiv.io/"
	// StateConfigPath is the path to the root of the configuration state
	StateConfigPath = StateBasePath + "state/"
	// StateOperPath is the path for operational/runtime state
	StateOperPath = StateBasePath + "oper/"

	networkConfigPathPrefix  = StateConfigPath + "nets/"
	networkConfigPath        = networkConfigPathPrefix + "%s"
	endpointConfigPathPrefix = StateConfigPath + "eps/"
	endpointConfigPath       = endpointConfigPathPrefix + "%s"
	epGroupConfigPathPrefix  = StateConfigPath + "endpointGroups/"
	epGroupConfigPath        = epGroupConfigPathPrefix + "%s"
)

// CfgNetworkState implements the State interface for a network implemented using
// vlans with ovs. The state is stored as Json objects.
type CfgNetworkState struct {
	core.CommonState
	Tenant        string          `json:"tenant"`
	NetworkName   string          `json:"networkName"`
	NwType        string          `json:"nwType"`
	PktTagType    string          `json:"pktTagType"`
	PktTag        int             `json:"pktTag"`
	ExtPktTag     int             `json:"extPktTag"`
	SubnetIP      string          `json:"subnetIP"`
	SubnetLen     uint            `json:"subnetLen"`
	Gateway       string          `json:"gateway"`
	IPAddrRange   string          `json:"ipAddrRange"`
	EpAddrCount   int             `json:"epAddrCount"`
	EpCount       int             `json:"epCount"`
	IPAllocMap    bitset.BitSet   `json:"ipAllocMap"`
	IPv6Subnet    string          `json:"ipv6SubnetIP"`
	IPv6SubnetLen uint            `json:"ipv6SubnetLen"`
	IPv6Gateway   string          `json:"ipv6Gateway"`
	IPv6AllocMap  map[string]bool `json:"ipv6AllocMap"`
	IPv6LastHost  string          `json:"ipv6LastHost"`
	NetworkTag    string          `json:"networkTag"`
}

// Write the state.
func (s *CfgNetworkState) Write() error {
	key := fmt.Sprintf(networkConfigPath, s.ID)
	return s.StateDriver.WriteState(key, s, json.Marshal)
}

// Read the state for a given identifier
func (s *CfgNetworkState) Read(id string) error {
	key := fmt.Sprintf(networkConfigPath, id)
	return s.StateDriver.ReadState(key, s, json.Unmarshal)
}

// ReadAll state and return the collection.
func (s *CfgNetworkState) ReadAll() ([]core.State, error) {
	return s.StateDriver.ReadAllState(networkConfigPathPrefix, s, json.Unmarshal)
}

// WatchAll state transitions and send them through the channel.
func (s *CfgNetworkState) WatchAll(rsps chan core.WatchState) error {
	return s.StateDriver.WatchAllState(networkConfigPathPrefix, s, json.Unmarshal,
		rsps)
}

// Clear removes the state.
func (s *CfgNetworkState) Clear() error {
	key := fmt.Sprintf(networkConfigPath, s.ID)
	return s.StateDriver.ClearState(key)
}

// IncrEpCount Increments endpoint count
func (s *CfgNetworkState) IncrEpCount() error {
	s.EpCount++
	return s.Write()
}

// DecrEpCount decrements endpoint count
func (s *CfgNetworkState) DecrEpCount() error {
	s.EpCount--
	return s.Write()
}

//GetNwCfgKey returns the key for network state
func GetNwCfgKey(network, tenant string) string {
	return network + "." + tenant
}
