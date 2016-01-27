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
	"errors"
	"fmt"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/jainvipin/bitset"
)

const (
	// AutoVLANResource is the name of the resource, for storing state.
	AutoVLANResource = "auto-vlan"
)

const (
	vLANResourceConfigPathPrefix = mastercfg.StateConfigPath + AutoVLANResource + "/"
	vLANResourceConfigPath       = vLANResourceConfigPathPrefix + "%s"
	vLANResourceOperPathPrefix   = mastercfg.StateOperPath + AutoVLANResource + "/"
	vLANResourceOperPath         = vLANResourceOperPathPrefix + "%s"
)

// AutoVLANCfgResource implements the Resource interface for an 'auto-vlan' resource.
// 'auto-vlan' resource allocates a vlan from a range of vlan encaps specified
// at time of resource instantiation
type AutoVLANCfgResource struct {
	core.CommonState
	VLANs *bitset.BitSet `json:"vlans"`
}

// Write the state.
func (r *AutoVLANCfgResource) Write() error {
	key := fmt.Sprintf(vLANResourceConfigPath, r.ID)
	return r.StateDriver.WriteState(key, r, json.Marshal)
}

// Read the state.
func (r *AutoVLANCfgResource) Read(id string) error {
	key := fmt.Sprintf(vLANResourceConfigPath, id)
	return r.StateDriver.ReadState(key, r, json.Unmarshal)
}

// Clear the state.
func (r *AutoVLANCfgResource) Clear() error {
	key := fmt.Sprintf(vLANResourceConfigPath, r.ID)
	return r.StateDriver.ClearState(key)
}

// ReadAll the state for this resource.
func (r *AutoVLANCfgResource) ReadAll() ([]core.State, error) {
	return r.StateDriver.ReadAllState(vLANResourceConfigPathPrefix, r,
		json.Unmarshal)
}

// Init the Resource. Requires a *bitset.BitSet.
func (r *AutoVLANCfgResource) Init(rsrcCfg interface{}) error {
	var ok bool
	r.VLANs, ok = rsrcCfg.(*bitset.BitSet)
	if !ok {
		return core.Errorf("Invalid type for vlan resource config")
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

	oper := &AutoVLANOperResource{FreeVLANs: r.VLANs}
	oper.StateDriver = r.StateDriver
	oper.ID = r.ID
	err = oper.Write()
	if err != nil {
		return err
	}

	return nil
}

// Deinit the resource.
func (r *AutoVLANCfgResource) Deinit() {
	oper := &AutoVLANOperResource{}
	oper.StateDriver = r.StateDriver
	err := oper.Read(r.ID)
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

// Description is a description of this resource. returns AutoVLANResource.
func (r *AutoVLANCfgResource) Description() string {
	return AutoVLANResource
}

// Allocate a resource.
func (r *AutoVLANCfgResource) Allocate(reqVal interface{}) (interface{}, error) {
	oper := &AutoVLANOperResource{}
	oper.StateDriver = r.StateDriver
	err := oper.Read(r.ID)
	if err != nil {
		return nil, err
	}

	var vlan uint
	if (reqVal != nil) && (reqVal.(uint) != 0) {
		vlan = reqVal.(uint)
		if !oper.FreeVLANs.Test(vlan) {
			return nil, errors.New("requested vlan not available")
		}
	} else {
		ok := false
		vlan, ok = oper.FreeVLANs.NextSet(0)
		if !ok {
			return nil, errors.New("no vlans available")
		}
	}
	oper.FreeVLANs.Clear(vlan)

	err = oper.Write()
	if err != nil {
		return nil, err
	}
	return vlan, nil
}

// Deallocate the resource.
func (r *AutoVLANCfgResource) Deallocate(value interface{}) error {
	oper := &AutoVLANOperResource{}
	oper.StateDriver = r.StateDriver
	err := oper.Read(r.ID)
	if err != nil {
		return err
	}

	vlan, ok := value.(uint)
	if !ok {
		return core.Errorf("Invalid type for vlan value")
	}
	if oper.FreeVLANs.Test(vlan) {
		return nil
	}
	oper.FreeVLANs.Set(vlan)

	err = oper.Write()
	if err != nil {
		return err
	}
	return nil
}

// AutoVLANOperResource is an implementation of core.State.
type AutoVLANOperResource struct {
	core.CommonState
	FreeVLANs *bitset.BitSet `json:"freeVLANs"`
}

// Write the state.
func (r *AutoVLANOperResource) Write() error {
	key := fmt.Sprintf(vLANResourceOperPath, r.ID)
	return r.StateDriver.WriteState(key, r, json.Marshal)
}

// Read the state.
func (r *AutoVLANOperResource) Read(id string) error {
	key := fmt.Sprintf(vLANResourceOperPath, id)
	return r.StateDriver.ReadState(key, r, json.Unmarshal)
}

// ReadAll state for this path.
func (r *AutoVLANOperResource) ReadAll() ([]core.State, error) {
	return r.StateDriver.ReadAllState(vLANResourceOperPathPrefix, r,
		json.Unmarshal)
}

// Clear the state.
func (r *AutoVLANOperResource) Clear() error {
	key := fmt.Sprintf(vLANResourceOperPath, r.ID)
	return r.StateDriver.ClearState(key)
}
