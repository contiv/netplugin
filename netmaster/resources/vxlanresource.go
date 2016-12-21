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
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/jainvipin/bitset"
)

const (
	// AutoVXLANResource is a string description of the type of resource.
	AutoVXLANResource = "auto-vxlan"
)

const (
	vXLANResourceConfigPathPrefix = mastercfg.StateConfigPath + AutoVXLANResource + "/"
	vXLANResourceConfigPath       = vXLANResourceConfigPathPrefix + "%s"
	vXLANResourceOperPathPrefix   = mastercfg.StateOperPath + AutoVXLANResource + "/"
	vXLANResourceOperPath         = vXLANResourceOperPathPrefix + "%s"
)

// AutoVXLANCfgResource implements the Resource interface for an 'auto-vxlan' resource.
// 'auto-vxlan' resource allocates a vxlan from a range of vxlan encaps specified
// at time of resource instantiation
type AutoVXLANCfgResource struct {
	core.CommonState
	VXLANs          *bitset.BitSet `json:"vxlans"`
	LocalVLANs      *bitset.BitSet `json:"LocalVLANs"`
	FreeVXLANsStart uint           `json:"FreeVXLANsStart"`
}

// VXLANVLANPair Pairs a VXLAN tag with a VLAN tag.
type VXLANVLANPair struct {
	VXLAN uint
	VLAN  uint
}

// Write the state.
func (r *AutoVXLANCfgResource) Write() error {
	key := fmt.Sprintf(vXLANResourceConfigPath, r.ID)
	return r.StateDriver.WriteState(key, r, json.Marshal)
}

// Read the state.
func (r *AutoVXLANCfgResource) Read(id string) error {
	key := fmt.Sprintf(vXLANResourceConfigPath, id)
	return r.StateDriver.ReadState(key, r, json.Unmarshal)
}

// Clear the state.
func (r *AutoVXLANCfgResource) Clear() error {
	key := fmt.Sprintf(vXLANResourceConfigPath, r.ID)
	return r.StateDriver.ClearState(key)
}

// ReadAll reads all the state from the resource.
func (r *AutoVXLANCfgResource) ReadAll() ([]core.State, error) {
	return r.StateDriver.ReadAllState(vXLANResourceConfigPathPrefix, r,
		json.Unmarshal)
}

// Init the resource.
func (r *AutoVXLANCfgResource) Init(rsrcCfg interface{}) error {
	cfg, ok := rsrcCfg.(*AutoVXLANCfgResource)
	if !ok {
		return core.Errorf("Invalid vxlan resource config.")
	}
	r.VXLANs = cfg.VXLANs
	r.LocalVLANs = cfg.LocalVLANs
	err := r.Write()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			r.Clear()
		}
	}()

	oper := &AutoVXLANOperResource{FreeVXLANs: r.VXLANs, FreeLocalVLANs: r.LocalVLANs}
	oper.StateDriver = r.StateDriver
	oper.ID = r.ID
	return oper.Write()
}

// Deinit the resource.
func (r *AutoVXLANCfgResource) Deinit() {
	oper := &AutoVXLANOperResource{}
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

// Reinit the resource.
func (r *AutoVXLANCfgResource) Reinit(rsrcCfg interface{}) error {

	cfg, ok := rsrcCfg.(*AutoVXLANCfgResource)
	if !ok {
		return core.Errorf("Invalid vxlan resource config.")
	}

	prevVXLANs := r.VXLANs
	prevFreeStart := r.FreeVXLANsStart

	r.VXLANs = cfg.VXLANs
	r.LocalVLANs = cfg.LocalVLANs
	r.FreeVXLANsStart = cfg.FreeVXLANsStart

	err := r.Write()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			r.Clear()
		}
	}()

	oper := &AutoVXLANOperResource{}
	oper.StateDriver = r.StateDriver
	oper.ID = r.ID

	err = oper.Read(r.ID)
	if err != nil {
		return err
	}

	prevVXLANs.InPlaceSymmetricDifference(oper.FreeVXLANs)

	oper.FreeVXLANs = r.VXLANs
	for i, e := prevVXLANs.NextSet(0); e; i, e = prevVXLANs.NextSet(i + 1) {
		vxlan := i + prevFreeStart
		oper.FreeVXLANs.Clear(vxlan - r.FreeVXLANsStart)
	}

	oper.FreeLocalVLANs = oper.FreeLocalVLANs.Intersection(r.LocalVLANs)

	return oper.Write()
}

// Description is a string description of the resource. Returns AutoVXLANResource.
func (r *AutoVXLANCfgResource) Description() string {
	return AutoVXLANResource
}

// GetList returns number of vlans and stringified list of vlans in use.
func (r *AutoVXLANCfgResource) GetList() (uint, string) {
	cfg := &AutoVXLANCfgResource{}
	cfg.StateDriver = r.StateDriver
	if err := cfg.Read(r.ID); err != nil {
		log.Errorf("Error reading resource %s: %s", r.ID, err)
		return 0, ""
	}

	oper := &AutoVXLANOperResource{}
	oper.StateDriver = r.StateDriver
	if err := oper.Read(r.ID); err != nil {
		log.Errorf("Error reading resource %s: %s", r.ID, err)
		return 0, ""
	}
	oper.FreeVXLANs.InPlaceSymmetricDifference(cfg.VXLANs)

	numVlans := uint(0)
	idx := uint(0)
	startIdx := idx
	list := []string{}
	inRange := false

	for {
		foundValue, found := oper.FreeVXLANs.NextSet(idx)
		if !found {
			break
		}
		numVlans++

		if !inRange { // begin of range
			startIdx = foundValue
			inRange = true
		} else if foundValue > idx { // end of range
			thisRange := rangePrint(startIdx+cfg.FreeVXLANsStart, idx-1+cfg.FreeVXLANsStart)
			list = append(list, thisRange)
			startIdx = foundValue
		}
		idx = foundValue + 1
	}

	// list end with allocated value
	if inRange {
		thisRange := rangePrint(startIdx+cfg.FreeVXLANsStart, idx-1+cfg.FreeVXLANsStart)
		list = append(list, thisRange)
	}

	return numVlans, strings.Join(list, ", ")
}

// Allocate allocates a new resource.
func (r *AutoVXLANCfgResource) Allocate(reqVal interface{}) (interface{}, error) {
	oper := &AutoVXLANOperResource{}
	oper.StateDriver = r.StateDriver
	err := oper.Read(r.ID)
	if err != nil {
		return nil, err
	}

	var vxlan uint
	if (reqVal != nil) && (reqVal.(uint) != 0) {
		vxlan = reqVal.(uint)
		if !oper.FreeVXLANs.Test(vxlan) {
			return nil, fmt.Errorf("requested vxlan not available")
		}
	} else {
		ok := false
		vxlan, ok = oper.FreeVXLANs.NextSet(0)
		if !ok {
			return nil, errors.New("no vxlans available")
		}
	}

	vlan, ok := oper.FreeLocalVLANs.NextSet(0)
	if !ok {
		return nil, errors.New("no local vlans available")
	}

	oper.FreeVXLANs.Clear(vxlan)
	oper.FreeLocalVLANs.Clear(vlan)

	err = oper.Write()
	if err != nil {
		return nil, err
	}
	return VXLANVLANPair{VXLAN: vxlan, VLAN: vlan}, nil
}

// Deallocate removes and cleans up a resource.
func (r *AutoVXLANCfgResource) Deallocate(value interface{}) error {
	oper := &AutoVXLANOperResource{}
	oper.StateDriver = r.StateDriver
	err := oper.Read(r.ID)
	if err != nil {
		return err
	}

	pair, ok := value.(VXLANVLANPair)
	if !ok {
		return core.Errorf("Invalid type for vxlan-vlan pair")
	}
	vxlan := pair.VXLAN
	oper.FreeVXLANs.Set(vxlan)
	vlan := pair.VLAN
	oper.FreeLocalVLANs.Set(vlan)

	return oper.Write()
}

// AutoVXLANOperResource is an implementation of core.State
type AutoVXLANOperResource struct {
	core.CommonState
	FreeVXLANs     *bitset.BitSet `json:"freeVXLANs"`
	FreeLocalVLANs *bitset.BitSet `json:"freeLocalVLANs"`
}

// Write the state.
func (r *AutoVXLANOperResource) Write() error {
	key := fmt.Sprintf(vXLANResourceOperPath, r.ID)
	return r.StateDriver.WriteState(key, r, json.Marshal)
}

// Read the state.
func (r *AutoVXLANOperResource) Read(id string) error {
	key := fmt.Sprintf(vXLANResourceOperPath, id)
	return r.StateDriver.ReadState(key, r, json.Unmarshal)
}

// ReadAll the state for the given type.
func (r *AutoVXLANOperResource) ReadAll() ([]core.State, error) {
	return r.StateDriver.ReadAllState(vXLANResourceOperPathPrefix, r,
		json.Unmarshal)
}

// Clear the state.
func (r *AutoVXLANOperResource) Clear() error {
	key := fmt.Sprintf(vXLANResourceOperPath, r.ID)
	return r.StateDriver.ClearState(key)
}
