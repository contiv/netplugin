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

package resources

import (
	"encoding/json"
	"fmt"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/drivers"
	"github.com/jainvipin/bitset"
)

// implements the Resource interface for an 'auto-vlan' resource.
// 'auto-vlan' resource allocates a vlan from a range of vlan encaps specified
// at time of resource instantiation

const (
	AUTO_VLAN_RSRC = "auto-vlan"
)

const (
	VLAN_RSRC_CFG_PATH_PREFIX  = drivers.CFG_PATH + AUTO_VLAN_RSRC + "/"
	VLAN_RSRC_CFG_PATH         = VLAN_RSRC_CFG_PATH_PREFIX + "%s"
	VLAN_RSRC_OPER_PATH_PREFIX = drivers.OPER_PATH + AUTO_VLAN_RSRC + "/"
	VLAN_RSRC_OPER_PATH        = VLAN_RSRC_OPER_PATH_PREFIX + "%s"
)

type AutoVlanCfgResource struct {
	core.CommonState
	Vlans *bitset.BitSet `json:"vlans"`
}

func (r *AutoVlanCfgResource) Write() error {
	key := fmt.Sprintf(VLAN_RSRC_CFG_PATH, r.Id)
	return r.StateDriver.WriteState(key, r, json.Marshal)
}

func (r *AutoVlanCfgResource) Read(id string) error {
	key := fmt.Sprintf(VLAN_RSRC_CFG_PATH, id)
	return r.StateDriver.ReadState(key, r, json.Unmarshal)
}

func (r *AutoVlanCfgResource) Clear() error {
	key := fmt.Sprintf(VLAN_RSRC_CFG_PATH, r.Id)
	return r.StateDriver.ClearState(key)
}

func (r *AutoVlanCfgResource) ReadAll() ([]core.State, error) {
	return r.StateDriver.ReadAllState(VLAN_RSRC_CFG_PATH_PREFIX, r,
		json.Unmarshal)
}

func (r *AutoVlanCfgResource) Init(rsrcCfg interface{}) error {
	var ok bool
	r.Vlans, ok = rsrcCfg.(*bitset.BitSet)
	if !ok {
		return &core.Error{Desc: "Invalid type for vlan resource config"}
	}
	err := r.Write()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			r.Clear()
		}
	}()

	oper := &AutoVlanOperResource{FreeVlans: r.Vlans}
	oper.StateDriver = r.StateDriver
	oper.Id = r.Id
	err = oper.Write()
	if err != nil {
		return err
	}

	return nil
}

func (r *AutoVlanCfgResource) Deinit() {
	oper := &AutoVlanOperResource{}
	oper.StateDriver = r.StateDriver
	err := oper.Read(r.Id)
	if err != nil {
		// continue cleanup
	} else {
		err = oper.Clear()
		if err != nil {
			// continue cleanup
		}
	}

	r.Clear()
}

func (r *AutoVlanCfgResource) Description() string {
	return AUTO_VLAN_RSRC
}

func (r *AutoVlanCfgResource) Allocate() (interface{}, error) {
	oper := &AutoVlanOperResource{}
	oper.StateDriver = r.StateDriver
	err := oper.Read(r.Id)
	if err != nil {
		return nil, err
	}

	vlan, ok := oper.FreeVlans.NextSet(0)
	if !ok {
		return nil, &core.Error{Desc: "no vlans available."}
	}

	oper.FreeVlans.Clear(vlan)

	err = oper.Write()
	if err != nil {
		return nil, err
	}
	return vlan, nil
}

func (r *AutoVlanCfgResource) Deallocate(value interface{}) error {
	oper := &AutoVlanOperResource{}
	oper.StateDriver = r.StateDriver
	err := oper.Read(r.Id)
	if err != nil {
		return err
	}

	vlan, ok := value.(uint)
	if !ok {
		return &core.Error{Desc: "Invalid type for vlan value"}
	}
	if oper.FreeVlans.Test(vlan) {
		return nil
	}
	oper.FreeVlans.Set(vlan)

	err = oper.Write()
	if err != nil {
		return err
	}
	return nil
}

type AutoVlanOperResource struct {
	core.CommonState
	FreeVlans *bitset.BitSet `json:"freeVlans"`
}

func (r *AutoVlanOperResource) Write() error {
	key := fmt.Sprintf(VLAN_RSRC_OPER_PATH, r.Id)
	return r.StateDriver.WriteState(key, r, json.Marshal)
}

func (r *AutoVlanOperResource) Read(id string) error {
	key := fmt.Sprintf(VLAN_RSRC_OPER_PATH, id)
	return r.StateDriver.ReadState(key, r, json.Unmarshal)
}

func (r *AutoVlanOperResource) ReadAll() ([]core.State, error) {
	return r.StateDriver.ReadAllState(VLAN_RSRC_OPER_PATH_PREFIX, r,
		json.Unmarshal)
}

func (r *AutoVlanOperResource) Clear() error {
	key := fmt.Sprintf(VLAN_RSRC_OPER_PATH, r.Id)
	return r.StateDriver.ClearState(key)
}
