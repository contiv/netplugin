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
	"fmt"
	"strings"
	"testing"

	"github.com/contiv/netplugin/core"
	"github.com/jainvipin/bitset"

	log "github.com/Sirupsen/logrus"
)

var epgRsrcStateDriver = &testEpgRsrcStateDriver{}

type epgRsrcValidator struct {
	// slice (stack) of expected config and oper states.
	// nextState modifies this slice after every state validate (write)
	// or copy (read)
	expCfg  []AutoEPGCfgResource
	expOper []AutoEPGOperResource
}

func (et *epgRsrcValidator) nextCfgState() {
	et.expCfg = et.expCfg[1:]
	if len(et.expCfg) > 0 {
		log.Debugf("after pop cfg is: %+v\n", et.expCfg[0])
	} else {
		log.Debugf("cfg becomes empty.\n")
	}
}

func (et *epgRsrcValidator) nextOperState() {
	et.expOper = et.expOper[1:]
	if len(et.expOper) > 0 {
		log.Debugf("after pop oper is: %+v\n", et.expOper[0])
	} else {
		log.Debugf("oper becomes empty.\n")
	}
}

func (et *epgRsrcValidator) ValidateState(state core.State) error {
	rcvdCfg, okCfg := state.(*AutoEPGCfgResource)
	if okCfg {
		log.Debugf("cfg length: %d", len(et.expCfg))
		if rcvdCfg.ID != et.expCfg[0].ID ||
			!rcvdCfg.EPGs.Equal(et.expCfg[0].EPGs) {
			errStr := fmt.Sprintf("cfg mismatch. Expctd: %+v, Rcvd: %+v",
				et.expCfg[0], rcvdCfg)
			//panic so we can catch the exact backtrace
			panic(errStr)
		}
		et.nextCfgState()
		return nil
	}

	rcvdOper, okOper := state.(*AutoEPGOperResource)
	if okOper {
		log.Debugf("oper length: %d", len(et.expOper))
		if rcvdOper.ID != et.expOper[0].ID ||
			!rcvdOper.FreeEPGs.Equal(et.expOper[0].FreeEPGs) {
			fmt.Printf("rcvdOper.ID = %s expOperId = %s \n", rcvdOper.ID, et.expOper[0].ID)
			fmt.Printf("RcvdFreeEpgs = %s, ExpFreeEpgs = %s\n", rcvdOper.FreeEPGs.DumpAsBits(),
				et.expOper[0].FreeEPGs.DumpAsBits())
			errStr := fmt.Sprintf("oper mismatch. Expctd: %+v, Rcvd: %+v",
				et.expOper[0], rcvdOper)
			//panic so we can catch the exact backtrace
			panic(errStr)
		}
		et.nextOperState()
		return nil
	}

	return core.Errorf("unknown state object type!")
}

func (et *epgRsrcValidator) CopyState(state core.State) error {
	rcvdCfg, okCfg := state.(*AutoEPGCfgResource)
	if okCfg {
		rcvdCfg.ID = et.expCfg[0].ID
		rcvdCfg.EPGs = et.expCfg[0].EPGs.Clone()
		et.nextCfgState()
		return nil
	}

	rcvdOper, okOper := state.(*AutoEPGOperResource)
	if okOper {
		rcvdOper.ID = et.expOper[0].ID
		rcvdOper.FreeEPGs = et.expOper[0].FreeEPGs.Clone()
		et.nextOperState()
		return nil
	}

	return core.Errorf("unknown state object type!")
}

type epgRsrcValidateOp int

const (
	EpgRsrcValidInitID       = "EpgRsrcValidInitID"
	EpgRsrcValidDeinitID     = "EpgRsrcValidDeinitID"
	EpgRsrcAllocateID        = "EpgRsrcAllocateID"
	EpgRsrcAllocateExhaustID = "EpgRsrcAllocateExhaustID"
	EpgRsrcDeallocateID      = "EpgRsrcDeallocateID"
	EpgRsrcGetListID         = "EpgRsrcGetListID"

	ePGResourceOperWrite = iota
	ePGResourceOperRead
	ePGResourceOperClear
)

var epgRsrcValidationStateMap = map[string]*epgRsrcValidator{
	EpgRsrcValidInitID: {
		expCfg: []AutoEPGCfgResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcValidInitID},
				EPGs:        bitset.New(1).Set(1),
			},
		},
		expOper: []AutoEPGOperResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcValidInitID},
				FreeEPGs:    bitset.New(1).Set(1),
			},
		},
	},
	EpgRsrcValidDeinitID: {
		expCfg: []AutoEPGCfgResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcValidDeinitID},
				EPGs:        bitset.New(1).Set(0),
			},
		},
		expOper: []AutoEPGOperResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcValidDeinitID},
				FreeEPGs:    bitset.New(1).Set(0),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcValidDeinitID},
				FreeEPGs:    bitset.New(1).Set(0),
			},
		},
	},
	EpgRsrcAllocateID: {
		expCfg: []AutoEPGCfgResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcAllocateID},
				EPGs:        bitset.New(1).Set(0),
			},
		},
		expOper: []AutoEPGOperResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcAllocateID},
				FreeEPGs:    bitset.New(1).Set(0),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcAllocateID},
				FreeEPGs:    bitset.New(1).Set(0),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcAllocateID},
				FreeEPGs:    bitset.New(1).Clear(0),
			},
		},
	},
	EpgRsrcAllocateExhaustID: {
		expCfg: []AutoEPGCfgResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcAllocateExhaustID},
				EPGs:        bitset.New(1).Clear(0),
			},
		},
		expOper: []AutoEPGOperResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcAllocateExhaustID},
				FreeEPGs:    bitset.New(1).Clear(0),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcAllocateExhaustID},
				FreeEPGs:    bitset.New(1).Clear(0),
			},
		},
	},
	EpgRsrcDeallocateID: {
		expCfg: []AutoEPGCfgResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcDeallocateID},
				EPGs:        bitset.New(1).Set(0),
			},
		},
		expOper: []AutoEPGOperResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcDeallocateID},
				FreeEPGs:    bitset.New(1).Set(0),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcDeallocateID},
				FreeEPGs:    bitset.New(1).Set(0),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcDeallocateID},
				FreeEPGs:    bitset.New(1).Clear(0),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcDeallocateID},
				FreeEPGs:    bitset.New(1).Clear(0),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcDeallocateID},
				FreeEPGs:    bitset.New(1).Set(0),
			},
		},
	},
	EpgRsrcGetListID: {
		expCfg: []AutoEPGCfgResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcGetListID},
				EPGs:        bitset.New(20).Complement().Clear(0),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcGetListID},
				EPGs:        bitset.New(20).Complement().Clear(0),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcGetListID},
				EPGs:        bitset.New(20).Complement().Clear(0),
			},
		},
		expOper: []AutoEPGOperResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcGetListID},
				FreeEPGs:    bitset.New(20).Complement().Clear(0),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcGetListID},
				FreeEPGs:    bitset.New(20).Complement().Clear(0),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcGetListID},
				FreeEPGs:    bitset.New(20).Complement().Clear(0).Clear(1),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcGetListID},
				FreeEPGs:    bitset.New(20).Complement().Clear(0).Clear(1),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcGetListID},
				FreeEPGs:    bitset.New(20).Complement().Clear(0).Clear(1).Clear(2),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcGetListID},
				FreeEPGs:    bitset.New(20).Complement().Clear(0).Clear(1).Clear(2),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcGetListID},
				FreeEPGs:    bitset.New(20).Complement().Clear(0).Clear(1).Clear(2).Clear(19),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: EpgRsrcGetListID},
				FreeEPGs:    bitset.New(20).Complement().Clear(0).Clear(1).Clear(2).Clear(19),
			},
		},
	},
}

type testEpgRsrcStateDriver struct {
}

func (d *testEpgRsrcStateDriver) Init(instInfo *core.InstanceInfo) error {
	return core.Errorf("Shouldn't be called!")
}

func (d *testEpgRsrcStateDriver) Deinit() {
}

func (d *testEpgRsrcStateDriver) Write(key string, value []byte) error {
	return core.Errorf("Shouldn't be called!")
}

func (d *testEpgRsrcStateDriver) Read(key string) ([]byte, error) {
	return nil, core.Errorf("Shouldn't be called!")
}

func (d *testEpgRsrcStateDriver) ReadAll(baseKey string) ([][]byte, error) {
	return nil, core.Errorf("Shouldn't be called!")
}

func (d *testEpgRsrcStateDriver) WatchAll(baseKey string, rsps chan [2][]byte) error {
	return core.Errorf("not supported")
}

func (d *testEpgRsrcStateDriver) validate(key string, state core.State,
	op epgRsrcValidateOp) error {
	strs := strings.Split(key, "/")
	id := strs[len(strs)-1]
	v, ok := epgRsrcValidationStateMap[id]
	if !ok {
		errStr := fmt.Sprintf("No matching validation entry for id: %s", id)
		log.Errorf("%s\n", errStr)
		return core.Errorf(errStr)
	}

	switch op {
	case ePGResourceOperWrite:
		err := v.ValidateState(state)
		if err != nil {
			return err
		}
		return nil
	case ePGResourceOperRead:
		return v.CopyState(state)
	case ePGResourceOperClear:
		fallthrough
	default:
		return nil
	}
}

func (d *testEpgRsrcStateDriver) ClearState(key string) error {
	return d.validate(key, nil, ePGResourceOperClear)
}

func (d *testEpgRsrcStateDriver) ReadState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) error {
	return d.validate(key, value, ePGResourceOperRead)
}

func (d *testEpgRsrcStateDriver) ReadAllState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) ([]core.State, error) {
	return nil, core.Errorf("Shouldn't be called!")
}

func (d *testEpgRsrcStateDriver) WatchAllState(baseKey string, sType core.State,
	unmarshal func([]byte, interface{}) error, rsps chan core.WatchState) error {
	return core.Errorf("not supported")
}

func (d *testEpgRsrcStateDriver) WriteState(key string, value core.State,
	marshal func(interface{}) ([]byte, error)) error {
	return d.validate(key, value, ePGResourceOperWrite)
}

func TestAutoEPGCfgResourceInit(t *testing.T) {
	rsrc := &AutoEPGCfgResource{}
	rsrc.StateDriver = epgRsrcStateDriver
	rsrc.ID = EpgRsrcValidInitID
	epgs := epgRsrcValidationStateMap[rsrc.ID].expCfg[0].EPGs.Clone()
	err := rsrc.Init(epgs)
	if err != nil {
		t.Fatalf("Epg resource init failed. Error: %s", err)
	}
}

func TestAutoEPGCfgResourceDeInit(t *testing.T) {
	rsrc := &AutoEPGCfgResource{}
	rsrc.StateDriver = epgRsrcStateDriver
	rsrc.ID = EpgRsrcValidDeinitID
	epgs := epgRsrcValidationStateMap[rsrc.ID].expCfg[0].EPGs.Clone()
	err := rsrc.Init(epgs)
	if err != nil {
		t.Fatalf("Epg resource init failed. Error: %s", err)
	}

	rsrc.Deinit()
}

func TestAutoEPGCfgResourceAllocate(t *testing.T) {
	rsrc := &AutoEPGCfgResource{}
	rsrc.StateDriver = epgRsrcStateDriver
	rsrc.ID = EpgRsrcAllocateID
	epgs := epgRsrcValidationStateMap[rsrc.ID].expCfg[0].EPGs.Clone()
	err := rsrc.Init(epgs)
	if err != nil {
		t.Fatalf("Epg resource init failed. Error: %s", err)
	}

	epg, err1 := rsrc.Allocate(uint(0))
	if err1 != nil {
		t.Fatalf("Epg resource allocation failed. Error: %s", err1)
	}
	if epg.(uint) != 0 {
		t.Fatalf("Allocated epg mismatch. expected: 0, rcvd: %d", epg)
	}
}

func TestAutoEPGCfgResourceAllocateExhaustion(t *testing.T) {
	rsrc := &AutoEPGCfgResource{}
	rsrc.StateDriver = epgRsrcStateDriver
	rsrc.ID = EpgRsrcAllocateExhaustID
	epgs := epgRsrcValidationStateMap[rsrc.ID].expCfg[0].EPGs.Clone()
	err := rsrc.Init(epgs)
	if err != nil {
		t.Fatalf("Epg resource init failed. Error: %s", err)
	}

	_, err = rsrc.Allocate(uint(0))
	if err == nil {
		t.Fatalf("Epg resource allocation succeeded, expected to fail!")
	}
	if !strings.Contains(err.Error(), "no epgs available") {
		t.Fatalf("Epg resource allocation failure reason mismatch. Expected: %s, rcvd: %s",
			"no epgs available", err)
	}
}

func TestAutoEPGCfgResourceDeAllocate(t *testing.T) {
	rsrc := &AutoEPGCfgResource{}
	rsrc.StateDriver = epgRsrcStateDriver
	rsrc.ID = EpgRsrcDeallocateID
	epgs := epgRsrcValidationStateMap[rsrc.ID].expCfg[0].EPGs.Clone()
	err := rsrc.Init(epgs)
	if err != nil {
		t.Fatalf("Epg resource init failed. Error: %s", err)
	}

	epg, err1 := rsrc.Allocate(uint(0))
	if err1 != nil {
		t.Fatalf("Epg resource allocation failed. Error: %s", err1)
	}

	err = rsrc.Deallocate(epg)
	if err != nil {
		t.Fatalf("Epg resource deallocation failed. Error: %s", err)
	}
}

func TestAutoEPGCfgResourceGetList(t *testing.T) {
	rsrc := &AutoEPGCfgResource{}
	rsrc.StateDriver = epgRsrcStateDriver
	rsrc.ID = EpgRsrcGetListID
	epgs := epgRsrcValidationStateMap[rsrc.ID].expCfg[0].EPGs.Clone()
	err := rsrc.Init(epgs)
	if err != nil {
		t.Fatalf("Epg resource init failed. Error: %s", err)
	}

	if _, err1 := rsrc.Allocate(uint(1)); err1 != nil {
		t.Fatalf("Epg resource allocation failed. Error: %s", err1)
	}
	if _, err1 := rsrc.Allocate(uint(2)); err1 != nil {
		t.Fatalf("Epg resource allocation failed. Error: %s", err1)
	}
	if _, err1 := rsrc.Allocate(uint(19)); err1 != nil {
		t.Fatalf("Epg resource allocation failed. Error: %s", err1)
	}
	expectedList := "1-2, 19"
	numEpgs, epgsInUse := rsrc.GetList()
	if numEpgs != 3 || epgsInUse != expectedList {
		t.Fatalf("GetList failure, got %s epglist (%d epgs), expected %s", epgsInUse, numEpgs, expectedList)
	}
}
