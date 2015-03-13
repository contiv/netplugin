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
)

// implements the State interface for an endpoint implemented using
// vlans with ovs. The state is stored as Json objects.

type OvsCfgEndpointState struct {
	StateDriver core.StateDriver `json:"-"`
	Id          string           `json:"id"`
	NetId       string           `json:"netId"`
	ContName    string           `json:"contName"`
	IpAddress   string           `json:"ipAddress"`
	HomingHost  string           `json:"homingHost"`
	IntfName    string           `json:"intfName"`
	VtepIp      string           `json:'vtepIP"`
}

func (s *OvsCfgEndpointState) Write() error {
	key := fmt.Sprintf(EP_CFG_PATH, s.Id)
	return s.StateDriver.WriteState(key, s, json.Marshal)
}

func (s *OvsCfgEndpointState) Read(id string) error {
	key := fmt.Sprintf(EP_CFG_PATH, id)
	return s.StateDriver.ReadState(key, s, json.Unmarshal)
}

func (s *OvsCfgEndpointState) Clear() error {
	key := fmt.Sprintf(EP_CFG_PATH, s.Id)
	return s.StateDriver.ClearState(key)
}

func ReadAllOvsCfgEndpoints(d core.StateDriver) ([]*OvsCfgEndpointState, error) {
	values := []*OvsCfgEndpointState{}
	byteValues, err := ReadAll(d, EP_CFG_PATH_PREFIX)
	if err != nil {
		return nil, err
	}
	for _, byteValue := range byteValues {
		value := &OvsCfgEndpointState{StateDriver: d}
		err = json.Unmarshal(byteValue, value)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

type OvsOperEndpointState struct {
	StateDriver core.StateDriver `json:"-"`
	Id          string           `json:"id"`
	NetId       string           `json:"netId"`
	ContName    string           `json:"contName"`
	IpAddress   string           `json:"ipAddress"`
	PortName    string           `json:"portName"`
	HomingHost  string           `json:"homingHost"`
	IntfName    string           `json:"intfName"`
	VtepIp      string           `json:'vtepIP"`
}

func (s *OvsOperEndpointState) Write() error {
	key := fmt.Sprintf(EP_OPER_PATH, s.Id)
	return s.StateDriver.WriteState(key, s, json.Marshal)
}

func (s *OvsOperEndpointState) Read(id string) error {
	key := fmt.Sprintf(EP_OPER_PATH, id)
	return s.StateDriver.ReadState(key, s, json.Unmarshal)
}

func (s *OvsOperEndpointState) Clear() error {
	key := fmt.Sprintf(EP_OPER_PATH, s.Id)
	return s.StateDriver.ClearState(key)
}
