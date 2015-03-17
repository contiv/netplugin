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

// implements the Resource interface for an 'auto-vxlan' resource.
// 'auto-vxlan' resource allocates a vxlan from a range of vxlan encaps specified
// at time of resource instantiation

const (
	AUTO_VXLAN_RSRC = "auto-vxlan"
)

const (
	VXLAN_RSRC_CFG_PATH_PREFIX  = drivers.CFG_PATH + AUTO_VXLAN_RSRC + "/"
	VXLAN_RSRC_CFG_PATH         = VXLAN_RSRC_CFG_PATH_PREFIX + "%s"
	VXLAN_RSRC_OPER_PATH_PREFIX = drivers.OPER_PATH + AUTO_VXLAN_RSRC + "/"
	VXLAN_RSRC_OPER_PATH        = VXLAN_RSRC_OPER_PATH_PREFIX + "%s"
)

type AutoVxlanCfgResource struct {
	stateDriver core.StateDriver `json:"-"`
	id          string           `json:"id"`
	Vxlans      *bitset.BitSet   `json:"vxlans"`
	LocalVlans  *bitset.BitSet   `json:"LocalVlans"`
}

type VxlanVlanPair struct {
	Vxlan uint
	Vlan  uint
}

func (r *AutoVxlanCfgResource) Write() error {
	key := fmt.Sprintf(VXLAN_RSRC_CFG_PATH, r.Id())
	return r.stateDriver.WriteState(key, r, json.Marshal)
}

func (r *AutoVxlanCfgResource) Read(id string) error {
	key := fmt.Sprintf(VXLAN_RSRC_CFG_PATH, id)
	return r.stateDriver.ReadState(key, r, json.Unmarshal)
}

func (r *AutoVxlanCfgResource) Clear() error {
	key := fmt.Sprintf(VXLAN_RSRC_CFG_PATH, r.Id())
	return r.stateDriver.ClearState(key)
}

func (r *AutoVxlanCfgResource) ReadAll() ([]core.State, error) {
	values := []*AutoVxlanCfgResource{}
	byteValues, err := r.stateDriver.ReadAll(VXLAN_RSRC_CFG_PATH_PREFIX)
	if err != nil {
		return nil, err
	}
	for _, byteValue := range byteValues {
		value := &AutoVxlanCfgResource{}
		value.SetStateDriver(r.StateDriver())
		err = json.Unmarshal(byteValue, value)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}

	stateValues := []core.State{}
	for _, val := range values {
		stateValues = append(stateValues, core.State(val))
	}
	return stateValues, nil
}

func (r *AutoVxlanCfgResource) Id() string {
	return r.id
}

func (r *AutoVxlanCfgResource) SetId(id string) {
	r.id = id
}

func (r *AutoVxlanCfgResource) StateDriver() core.StateDriver {
	return r.stateDriver
}

func (r *AutoVxlanCfgResource) SetStateDriver(stateDriver core.StateDriver) {
	r.stateDriver = stateDriver
}

func (r *AutoVxlanCfgResource) Init(rsrcCfg interface{}) error {
	cfg, ok := rsrcCfg.(*AutoVxlanCfgResource)
	if !ok {
		return &core.Error{Desc: "Invalid vxlan resource config."}
	}
	r.Vxlans = cfg.Vxlans
	r.LocalVlans = cfg.LocalVlans
	err := r.Write()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			r.Clear()
		}
	}()

	oper := AutoVxlanOperResource{StateDriver: r.StateDriver(), Id: r.Id(),
		FreeVxlans: r.Vxlans, FreeLocalVlans: r.LocalVlans}
	err = oper.Write()
	if err != nil {
		return err
	}

	return nil
}

func (r *AutoVxlanCfgResource) Deinit() {
	oper := AutoVxlanOperResource{StateDriver: r.StateDriver()}
	err := oper.Read(r.Id())
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

func (r *AutoVxlanCfgResource) Description() string {
	return AUTO_VXLAN_RSRC
}

func (r *AutoVxlanCfgResource) Allocate() (interface{}, error) {
	oper := &AutoVxlanOperResource{StateDriver: r.StateDriver()}
	err := oper.Read(r.Id())
	if err != nil {
		return nil, err
	}

	vxlan, ok := oper.FreeVxlans.NextSet(0)
	if !ok {
		return nil, &core.Error{Desc: "no vxlans available."}
	}

	vlan, ok := oper.FreeLocalVlans.NextSet(0)
	if !ok {
		return nil, &core.Error{Desc: "no local vlans available."}
	}

	oper.FreeVxlans.Clear(vxlan)
	oper.FreeLocalVlans.Clear(vlan)

	err = oper.Write()
	if err != nil {
		return nil, err
	}
	return VxlanVlanPair{Vxlan: vxlan, Vlan: vlan}, nil
}

func (r *AutoVxlanCfgResource) Deallocate(value interface{}) error {
	oper := &AutoVxlanOperResource{StateDriver: r.StateDriver()}
	err := oper.Read(r.Id())
	if err != nil {
		return err
	}

	pair, ok := value.(VxlanVlanPair)
	if !ok {
		return &core.Error{Desc: "Invalid type for vxlan-vlan pair"}
	}
	vxlan := pair.Vxlan
	oper.FreeVxlans.Set(vxlan)
	vlan := pair.Vlan
	oper.FreeLocalVlans.Set(vlan)

	err = oper.Write()
	if err != nil {
		return err
	}
	return nil
}

type AutoVxlanOperResource struct {
	StateDriver    core.StateDriver `json:"-"`
	Id             string           `json:"id"`
	FreeVxlans     *bitset.BitSet   `json:"freeVxlans"`
	FreeLocalVlans *bitset.BitSet   `json:"freeLocalVlans"`
}

func (r *AutoVxlanOperResource) Write() error {
	key := fmt.Sprintf(VXLAN_RSRC_OPER_PATH, r.Id)
	return r.StateDriver.WriteState(key, r, json.Marshal)
}

func (r *AutoVxlanOperResource) Read(id string) error {
	key := fmt.Sprintf(VXLAN_RSRC_OPER_PATH, id)
	return r.StateDriver.ReadState(key, r, json.Unmarshal)
}

func (r *AutoVxlanOperResource) Clear() error {
	key := fmt.Sprintf(VXLAN_RSRC_OPER_PATH, r.Id)
	return r.StateDriver.ClearState(key)
}
