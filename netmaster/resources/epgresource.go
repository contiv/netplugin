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
	// AutoEPGResource is the name of the resource, for storing state.
	AutoEPGResource = "auto-epg"
)

const (
	epgResourceConfigPathPrefix = mastercfg.StateConfigPath + AutoEPGResource + "/"
	epgResourceConfigPath       = epgResourceConfigPathPrefix + "%s"
	epgResourceOperPathPrefix   = mastercfg.StateOperPath + AutoEPGResource + "/"
	epgResourceOperPath         = epgResourceOperPathPrefix + "%s"
)

// AutoEPGCfgResource implements the Resource interface for an 'auto-epg' resource.
// 'auto-epg' resource allocates an EPG ID from a range specified
// at time of resource instantiation
type AutoEPGCfgResource struct {
	core.CommonState
	EPGs *bitset.BitSet `json:"epgs"`
}

// Write the state.
func (r *AutoEPGCfgResource) Write() error {
	key := fmt.Sprintf(epgResourceConfigPath, r.ID)
	return r.StateDriver.WriteState(key, r, json.Marshal)
}

// Read the state.
func (r *AutoEPGCfgResource) Read(id string) error {
	key := fmt.Sprintf(epgResourceConfigPath, id)
	return r.StateDriver.ReadState(key, r, json.Unmarshal)
}

// Clear the state.
func (r *AutoEPGCfgResource) Clear() error {
	key := fmt.Sprintf(epgResourceConfigPath, r.ID)
	return r.StateDriver.ClearState(key)
}

// ReadAll the state for this resource.
func (r *AutoEPGCfgResource) ReadAll() ([]core.State, error) {
	return r.StateDriver.ReadAllState(epgResourceConfigPathPrefix, r,
		json.Unmarshal)
}

// Init the Resource. Requires a *bitset.BitSet.
func (r *AutoEPGCfgResource) Init(rsrcCfg interface{}) error {
	var ok bool
	r.EPGs, ok = rsrcCfg.(*bitset.BitSet)
	if !ok {
		return core.Errorf("Invalid type for EPG resource config")
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

	oper := &AutoEPGOperResource{FreeEPGs: r.EPGs}
	oper.StateDriver = r.StateDriver
	oper.ID = r.ID
	err = oper.Write()
	if err != nil {
		return err
	}

	return nil
}

// Deinit the resource.
func (r *AutoEPGCfgResource) Deinit() {
	oper := &AutoEPGOperResource{}
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

// Reinit the Resource. Requires a *bitset.BitSet.
func (r *AutoEPGCfgResource) Reinit(rsrcCfg interface{}) error {
	var ok bool
	prevEPGs := r.EPGs
	r.EPGs, ok = rsrcCfg.(*bitset.BitSet)
	if !ok {
		return core.Errorf("Invalid type for EPG resource config")
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

	oper := &AutoEPGOperResource{}
	oper.StateDriver = r.StateDriver
	oper.ID = r.ID
	err = oper.Read(r.ID)
	if err != nil {
		return err
	}

	prevEPGs.InPlaceSymmetricDifference(oper.FreeEPGs)
	oper.FreeEPGs = r.EPGs
	for i, e := prevEPGs.NextSet(0); e; i, e = prevEPGs.NextSet(i + 1) {
		oper.FreeEPGs.Clear(i)
	}

	err = oper.Write()
	if err != nil {
		return err
	}

	return nil
}

// Description is a description of this resource. returns AutoEPGResource.
func (r *AutoEPGCfgResource) Description() string {
	return AutoEPGResource
}

// GetList returns number of epgs and stringified list of epgs in use.
func (r *AutoEPGCfgResource) GetList() (uint, string) {
	cfg := &AutoEPGCfgResource{}
	cfg.StateDriver = r.StateDriver
	if err := cfg.Read(r.ID); err != nil {
		log.Errorf("Error reading resource %s: %s", r.ID, err)
		return 0, ""
	}

	oper := &AutoEPGOperResource{}
	oper.StateDriver = r.StateDriver
	if err := oper.Read(r.ID); err != nil {
		log.Errorf("error fetching the epg resource: id %s", r.ID)
		return 0, ""
	}
	oper.FreeEPGs.InPlaceSymmetricDifference(cfg.EPGs)

	numEPGs := uint(0)
	idx := uint(0)
	startIdx := idx
	list := []string{}
	inRange := false

	for {
		foundValue, found := oper.FreeEPGs.NextSet(idx)
		if !found {
			break
		}
		numEPGs++

		if !inRange { // begin of range
			startIdx = foundValue
			inRange = true
		} else if foundValue > idx { // end of range
			thisRange := rangePrint(startIdx, idx-1)
			list = append(list, thisRange)
			startIdx = foundValue
		}
		idx = foundValue + 1
	}

	// list end with allocated value
	if inRange {
		thisRange := rangePrint(startIdx, idx-1)
		list = append(list, thisRange)
	}

	return numEPGs, strings.Join(list, ", ")
}

// Allocate a resource.
func (r *AutoEPGCfgResource) Allocate(reqVal interface{}) (interface{}, error) {
	oper := &AutoEPGOperResource{}
	oper.StateDriver = r.StateDriver
	err := oper.Read(r.ID)
	if err != nil {
		return nil, err
	}

	var epgID uint
	if (reqVal != nil) && (reqVal.(uint) != 0) {
		epgID = reqVal.(uint)
		if !oper.FreeEPGs.Test(epgID) {
			return nil, fmt.Errorf("requested epg id not available - epg_id:%d", epgID)
		}
	} else {
		ok := false
		epgID, ok = oper.FreeEPGs.NextSet(0)
		if !ok {
			return nil, errors.New("no epgs available")
		}
	}
	oper.FreeEPGs.Clear(epgID)

	err = oper.Write()
	if err != nil {
		return nil, err
	}
	return epgID, nil
}

// Deallocate the resource.
func (r *AutoEPGCfgResource) Deallocate(value interface{}) error {
	oper := &AutoEPGOperResource{}
	oper.StateDriver = r.StateDriver
	err := oper.Read(r.ID)
	if err != nil {
		return err
	}

	epgID, ok := value.(uint)
	if !ok {
		return core.Errorf("Invalid type for epg ID value")
	}
	if oper.FreeEPGs.Test(epgID) {
		return nil
	}
	oper.FreeEPGs.Set(epgID)

	err = oper.Write()
	if err != nil {
		return err
	}
	return nil
}

// AutoEPGOperResource is an implementation of core.State.
type AutoEPGOperResource struct {
	core.CommonState
	FreeEPGs *bitset.BitSet `json:"freeEPGs"`
}

// Write the state.
func (r *AutoEPGOperResource) Write() error {
	key := fmt.Sprintf(epgResourceOperPath, r.ID)
	return r.StateDriver.WriteState(key, r, json.Marshal)
}

// Read the state.
func (r *AutoEPGOperResource) Read(id string) error {
	key := fmt.Sprintf(epgResourceOperPath, id)
	return r.StateDriver.ReadState(key, r, json.Unmarshal)
}

// ReadAll state for this path.
func (r *AutoEPGOperResource) ReadAll() ([]core.State, error) {
	return r.StateDriver.ReadAllState(epgResourceOperPathPrefix, r,
		json.Unmarshal)
}

// Clear the state.
func (r *AutoEPGOperResource) Clear() error {
	key := fmt.Sprintf(epgResourceOperPath, r.ID)
	return r.StateDriver.ClearState(key)
}
