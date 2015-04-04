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
	HOST_CFG_PATH_PREFIX = CFG_PATH + "hosts/"
	HOST_CFG_PATH        = HOST_CFG_PATH_PREFIX + "%s"
)

type MasterHostConfig struct {
	core.CommonState
	Name   string `json:"name"`
	Intf   string `json:"intf"`
	VtepIp string `json:"vtepIp"`
	NetId  string `json:"netId"`
}

func (s *MasterHostConfig) Write() error {
	key := fmt.Sprintf(HOST_CFG_PATH, s.Name)
	return s.StateDriver.WriteState(key, s, json.Marshal)
}

func (s *MasterHostConfig) Read(hostname string) error {
	key := fmt.Sprintf(HOST_CFG_PATH, hostname)
	return s.StateDriver.ReadState(key, s, json.Unmarshal)
}

func (s *MasterHostConfig) ReadAll() ([]core.State, error) {
	return s.StateDriver.ReadAllState(HOST_CFG_PATH_PREFIX, s, json.Unmarshal)
}

func (s *MasterHostConfig) Clear() error {
	key := fmt.Sprintf(HOST_CFG_PATH, s.Name)
	return s.StateDriver.ClearState(key)
}
