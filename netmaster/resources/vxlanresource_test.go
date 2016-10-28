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

var vxlanRsrcStateDriver = &testVXLANRsrcStateDriver{}

type vxlanRsrcValidator struct {
	// slice (stack) of expected config and oper states.
	// nextState modifies this slice after every state validate (write)
	// or copy (read)
	expCfg  []AutoVXLANCfgResource
	expOper []AutoVXLANOperResource
}

func (vt *vxlanRsrcValidator) nextCfgState() {
	vt.expCfg = vt.expCfg[1:]
	if len(vt.expCfg) > 0 {
		log.Debugf("after pop cfg is: %+v\n", vt.expCfg)
	} else {
		log.Debugf("cfg becomes empty.\n")
	}
}

func (vt *vxlanRsrcValidator) nextOperState() {
	vt.expOper = vt.expOper[1:]
	if len(vt.expOper) > 0 {
		log.Debugf("after pop oper is: %+v\n", vt.expOper)
	} else {
		log.Debugf("oper becomes empty.\n")
	}
}

func (vt *vxlanRsrcValidator) ValidateState(state core.State) error {
	rcvdCfg, okCfg := state.(*AutoVXLANCfgResource)
	if okCfg {
		log.Debugf("cfg length: %d", len(vt.expCfg))
		if rcvdCfg.ID != vt.expCfg[0].ID ||
			!rcvdCfg.VXLANs.Equal(vt.expCfg[0].VXLANs) ||
			!rcvdCfg.LocalVLANs.Equal(vt.expCfg[0].LocalVLANs) {
			errStr := fmt.Sprintf("cfg mismatch. Expctd: %+v, Rcvd: %+v",
				vt.expCfg[0], rcvdCfg)
			//panic so we can catch the exact backtrace
			panic(errStr)
		}
		vt.nextCfgState()
		return nil
	}

	rcvdOper, okOper := state.(*AutoVXLANOperResource)
	if okOper {
		log.Debugf("oper length: %d", len(vt.expOper))
		if rcvdOper.ID != vt.expOper[0].ID ||
			!rcvdOper.FreeVXLANs.Equal(vt.expOper[0].FreeVXLANs) ||
			!rcvdOper.FreeLocalVLANs.Equal(vt.expOper[0].FreeLocalVLANs) {
			fmt.Printf("rcvdOper.ID = %s expOperId = %s \n", rcvdOper.ID, vt.expOper[0].ID)
			fmt.Printf("RcvdFreeVXLANs = %s, ExpFreeVXLANs = %s\n", rcvdOper.FreeVXLANs.DumpAsBits(), vt.expOper[0].FreeVXLANs.DumpAsBits())
			fmt.Printf("RcvdFreeLocalVLANs = %s, ExpFreeLocalVLANs = %s\n", rcvdOper.FreeLocalVLANs.DumpAsBits(), vt.expOper[0].FreeLocalVLANs.DumpAsBits())
			errStr := fmt.Sprintf("oper mismatch. Expctd: %+v, Rcvd: %+v",
				vt.expOper[0], rcvdOper)
			//panic so we can catch the exact backtrace
			panic(errStr)
		}
		vt.nextOperState()
		return nil
	}

	return core.Errorf("unknown state object type!")
}

func (vt *vxlanRsrcValidator) CopyState(state core.State) error {
	rcvdCfg, okCfg := state.(*AutoVXLANCfgResource)
	if okCfg {
		rcvdCfg.ID = vt.expCfg[0].ID
		rcvdCfg.VXLANs = vt.expCfg[0].VXLANs.Clone()
		rcvdCfg.LocalVLANs = vt.expCfg[0].LocalVLANs.Clone()
		vt.nextCfgState()
		return nil
	}

	rcvdOper, okOper := state.(*AutoVXLANOperResource)
	if okOper {
		rcvdOper.ID = vt.expOper[0].ID
		rcvdOper.FreeVXLANs = vt.expOper[0].FreeVXLANs.Clone()
		rcvdOper.FreeLocalVLANs = vt.expOper[0].FreeLocalVLANs.Clone()
		vt.nextOperState()
		return nil
	}

	return core.Errorf("unknown state object type!")
}

type vxlanRsrcValidateOp int

const (
	VXLANRsrcValidInitID            = "VXLANRsrcValidInitID"
	VXLANRsrcValidDeinitID          = "VXLANRsrcValidDeinitID"
	VXLANRsrcAllocateID             = "VXLANRsrcAllocateID"
	VXLANRsrcAllocateExhaustVXLANID = "VXLANRsrcAllocateExhaustVXLANID"
	VXLANRsrcAllocateExhaustVLANID  = "VXLANRsrcAllocateExhaustVLANID"
	VXLANRsrcDeallocateID           = "VXLANRsrcDeallocateID"
	VXLANRsrcGetListID              = "VXLANRsrcGetListID"

	vXLANResourceOpWrite = iota
	vXLANResourceOpRead
	vXLANResourceOpClear
)

var vxlanRsrcValidationStateMap = map[string]*vxlanRsrcValidator{
	VXLANRsrcValidInitID: {
		expCfg: []AutoVXLANCfgResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VXLANRsrcValidInitID},
				VXLANs:      bitset.New(1).Set(0),
				LocalVLANs:  bitset.New(1).Set(0),
			},
		},
		expOper: []AutoVXLANOperResource{
			{
				CommonState:    core.CommonState{StateDriver: nil, ID: VXLANRsrcValidInitID},
				FreeVXLANs:     bitset.New(1).Set(0),
				FreeLocalVLANs: bitset.New(1).Set(0),
			},
		},
	},
	VXLANRsrcValidDeinitID: {
		expCfg: []AutoVXLANCfgResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VXLANRsrcValidDeinitID},
				VXLANs:      bitset.New(1).Set(0),
				LocalVLANs:  bitset.New(1).Set(0),
			},
		},
		expOper: []AutoVXLANOperResource{
			{
				CommonState:    core.CommonState{StateDriver: nil, ID: VXLANRsrcValidDeinitID},
				FreeVXLANs:     bitset.New(1).Set(0),
				FreeLocalVLANs: bitset.New(1).Set(0),
			},
			{
				CommonState:    core.CommonState{StateDriver: nil, ID: VXLANRsrcValidDeinitID},
				FreeVXLANs:     bitset.New(1).Set(0),
				FreeLocalVLANs: bitset.New(1).Set(0),
			},
		},
	},
	VXLANRsrcAllocateID: {
		expCfg: []AutoVXLANCfgResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VXLANRsrcAllocateID},
				VXLANs:      bitset.New(1).Set(0),
				LocalVLANs:  bitset.New(1).Set(0),
			},
		},
		expOper: []AutoVXLANOperResource{
			{
				CommonState:    core.CommonState{StateDriver: nil, ID: VXLANRsrcAllocateID},
				FreeVXLANs:     bitset.New(1).Set(0),
				FreeLocalVLANs: bitset.New(1).Set(0),
			},
			{
				CommonState:    core.CommonState{StateDriver: nil, ID: VXLANRsrcAllocateID},
				FreeVXLANs:     bitset.New(1).Set(0),
				FreeLocalVLANs: bitset.New(1).Set(0),
			},
			{
				CommonState:    core.CommonState{StateDriver: nil, ID: VXLANRsrcAllocateID},
				FreeVXLANs:     bitset.New(1).Clear(0),
				FreeLocalVLANs: bitset.New(1).Clear(0),
			},
		},
	},
	VXLANRsrcAllocateExhaustVXLANID: {
		expCfg: []AutoVXLANCfgResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VXLANRsrcAllocateExhaustVXLANID},
				VXLANs:      bitset.New(1).Clear(0),
				LocalVLANs:  bitset.New(1).Clear(0),
			},
		},
		expOper: []AutoVXLANOperResource{
			{
				CommonState:    core.CommonState{StateDriver: nil, ID: VXLANRsrcAllocateExhaustVXLANID},
				FreeVXLANs:     bitset.New(1).Clear(0),
				FreeLocalVLANs: bitset.New(1).Clear(0),
			},
			{
				CommonState:    core.CommonState{StateDriver: nil, ID: VXLANRsrcAllocateExhaustVXLANID},
				FreeVXLANs:     bitset.New(1).Clear(0),
				FreeLocalVLANs: bitset.New(1).Clear(0),
			},
		},
	},
	VXLANRsrcAllocateExhaustVLANID: {
		expCfg: []AutoVXLANCfgResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VXLANRsrcAllocateExhaustVLANID},
				VXLANs:      bitset.New(1).Set(0),
				LocalVLANs:  bitset.New(1).Clear(0),
			},
		},
		expOper: []AutoVXLANOperResource{
			{
				CommonState:    core.CommonState{StateDriver: nil, ID: VXLANRsrcAllocateExhaustVLANID},
				FreeVXLANs:     bitset.New(1).Set(0),
				FreeLocalVLANs: bitset.New(1).Clear(0),
			},
			{
				CommonState:    core.CommonState{StateDriver: nil, ID: VXLANRsrcAllocateExhaustVLANID},
				FreeVXLANs:     bitset.New(1).Set(0),
				FreeLocalVLANs: bitset.New(1).Clear(0),
			},
		},
	},
	VXLANRsrcDeallocateID: {
		expCfg: []AutoVXLANCfgResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VXLANRsrcDeallocateID},
				VXLANs:      bitset.New(1).Set(0),
				LocalVLANs:  bitset.New(1).Set(0),
			},
		},
		expOper: []AutoVXLANOperResource{
			{
				CommonState:    core.CommonState{StateDriver: nil, ID: VXLANRsrcDeallocateID},
				FreeVXLANs:     bitset.New(1).Set(0),
				FreeLocalVLANs: bitset.New(1).Set(0),
			},
			{
				CommonState:    core.CommonState{StateDriver: nil, ID: VXLANRsrcDeallocateID},
				FreeVXLANs:     bitset.New(1).Set(0),
				FreeLocalVLANs: bitset.New(1).Set(0),
			},
			{
				CommonState:    core.CommonState{StateDriver: nil, ID: VXLANRsrcDeallocateID},
				FreeVXLANs:     bitset.New(1).Clear(0),
				FreeLocalVLANs: bitset.New(1).Clear(0),
			},
			{
				CommonState:    core.CommonState{StateDriver: nil, ID: VXLANRsrcDeallocateID},
				FreeVXLANs:     bitset.New(1).Clear(0),
				FreeLocalVLANs: bitset.New(1).Clear(0),
			},
			{
				CommonState:    core.CommonState{StateDriver: nil, ID: VXLANRsrcDeallocateID},
				FreeVXLANs:     bitset.New(1).Set(0),
				FreeLocalVLANs: bitset.New(1).Set(0),
			},
		},
	},
	VXLANRsrcGetListID: {
		expCfg: []AutoVXLANCfgResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VXLANRsrcGetListID},
				VXLANs:      bitset.New(150).Complement().Clear(0).Clear(149),
				LocalVLANs:  bitset.New(20).Complement().Clear(0).Clear(19),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VXLANRsrcGetListID},
				VXLANs:      bitset.New(150).Complement().Clear(0).Clear(149),
				LocalVLANs:  bitset.New(20).Complement().Clear(0).Clear(19),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VXLANRsrcGetListID},
				VXLANs:      bitset.New(150).Complement().Clear(0).Clear(149),
				LocalVLANs:  bitset.New(20).Complement().Clear(0).Clear(19),
			},
		},
		expOper: []AutoVXLANOperResource{
			{
				CommonState:    core.CommonState{StateDriver: nil, ID: VXLANRsrcGetListID},
				FreeVXLANs:     bitset.New(150).Complement().Clear(0).Clear(149),
				FreeLocalVLANs: bitset.New(20).Complement().Clear(0).Clear(19),
			},
			{
				CommonState:    core.CommonState{StateDriver: nil, ID: VXLANRsrcGetListID},
				FreeVXLANs:     bitset.New(150).Complement().Clear(0).Clear(149),
				FreeLocalVLANs: bitset.New(20).Complement().Clear(0).Clear(19),
			},
			{
				CommonState:    core.CommonState{StateDriver: nil, ID: VXLANRsrcGetListID},
				FreeVXLANs:     bitset.New(150).Complement().Clear(0).Clear(1).Clear(149),
				FreeLocalVLANs: bitset.New(20).Complement().Clear(0).Clear(1).Clear(19),
			},
			{
				CommonState:    core.CommonState{StateDriver: nil, ID: VXLANRsrcGetListID},
				FreeVXLANs:     bitset.New(150).Complement().Clear(0).Clear(1).Clear(149),
				FreeLocalVLANs: bitset.New(20).Complement().Clear(0).Clear(1).Clear(19),
			},
			{
				CommonState:    core.CommonState{StateDriver: nil, ID: VXLANRsrcGetListID},
				FreeVXLANs:     bitset.New(150).Complement().Clear(0).Clear(1).Clear(100).Clear(149),
				FreeLocalVLANs: bitset.New(20).Complement().Clear(0).Clear(1).Clear(2).Clear(19),
			},
			{
				CommonState:    core.CommonState{StateDriver: nil, ID: VXLANRsrcGetListID},
				FreeVXLANs:     bitset.New(150).Complement().Clear(0).Clear(1).Clear(100).Clear(149),
				FreeLocalVLANs: bitset.New(20).Complement().Clear(0).Clear(1).Clear(2).Clear(19),
			},
			{
				CommonState:    core.CommonState{StateDriver: nil, ID: VXLANRsrcGetListID},
				FreeVXLANs:     bitset.New(150).Complement().Clear(0).Clear(1).Clear(100).Clear(101).Clear(149),
				FreeLocalVLANs: bitset.New(20).Complement().Clear(0).Clear(1).Clear(2).Clear(3).Clear(19),
			},
			{
				CommonState:    core.CommonState{StateDriver: nil, ID: VXLANRsrcGetListID},
				FreeVXLANs:     bitset.New(150).Complement().Clear(0).Clear(1).Clear(100).Clear(101).Clear(149),
				FreeLocalVLANs: bitset.New(20).Complement().Clear(0).Clear(1).Clear(2).Clear(3).Clear(19),
			},
			{
				CommonState:    core.CommonState{StateDriver: nil, ID: VXLANRsrcGetListID},
				FreeVXLANs:     bitset.New(150).Complement().Clear(0).Clear(1).Clear(100).Clear(101).Clear(149),
				FreeLocalVLANs: bitset.New(20).Complement().Clear(0).Clear(1).Clear(2).Clear(3).Clear(19),
			},
			{
				CommonState:    core.CommonState{StateDriver: nil, ID: VXLANRsrcGetListID},
				FreeVXLANs:     bitset.New(150).Complement().Clear(0).Clear(1).Clear(100).Clear(101).Clear(149),
				FreeLocalVLANs: bitset.New(20).Complement().Clear(0).Clear(1).Clear(2).Clear(3).Clear(19),
			},
		},
	},
}

type testVXLANRsrcStateDriver struct {
}

func (d *testVXLANRsrcStateDriver) Init(instInfo *core.InstanceInfo) error {
	return core.Errorf("Shouldn't be called!")
}

func (d *testVXLANRsrcStateDriver) Deinit() {
}

func (d *testVXLANRsrcStateDriver) Write(key string, value []byte) error {
	return core.Errorf("Shouldn't be called!")
}

func (d *testVXLANRsrcStateDriver) Read(key string) ([]byte, error) {
	return nil, core.Errorf("Shouldn't be called!")
}

func (d *testVXLANRsrcStateDriver) ReadAll(baseKey string) ([][]byte, error) {
	return nil, core.Errorf("Shouldn't be called!")
}

func (d *testVXLANRsrcStateDriver) WatchAll(baseKey string, rsps chan [2][]byte) error {
	return core.Errorf("not supported")
}

func (d *testVXLANRsrcStateDriver) validate(key string, state core.State,
	op vxlanRsrcValidateOp) error {
	strs := strings.Split(key, "/")
	id := strs[len(strs)-1]
	v, ok := vxlanRsrcValidationStateMap[id]
	if !ok {
		errStr := fmt.Sprintf("No matching validation entry for id: %s", id)
		log.Errorf("%s\n", errStr)
		return core.Errorf(errStr)
	}

	switch op {
	case vXLANResourceOpWrite:
		err := v.ValidateState(state)
		if err != nil {
			return err
		}
		return nil
	case vXLANResourceOpRead:
		return v.CopyState(state)
	case vXLANResourceOpClear:
		fallthrough
	default:
		return nil
	}
}

func (d *testVXLANRsrcStateDriver) ClearState(key string) error {
	return d.validate(key, nil, vXLANResourceOpClear)
}

func (d *testVXLANRsrcStateDriver) ReadState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) error {
	return d.validate(key, value, vXLANResourceOpRead)
}

func (d *testVXLANRsrcStateDriver) ReadAllState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) ([]core.State, error) {
	return nil, core.Errorf("Shouldn't be called!")
}

func (d *testVXLANRsrcStateDriver) WatchAllState(baseKey string, sType core.State,
	unmarshal func([]byte, interface{}) error, rsps chan core.WatchState) error {
	return core.Errorf("not supported")
}

func (d *testVXLANRsrcStateDriver) WriteState(key string, value core.State,
	marshal func(interface{}) ([]byte, error)) error {
	return d.validate(key, value, vXLANResourceOpWrite)
}

func TestAutoVXLANCfgResourceInit(t *testing.T) {
	rsrc := &AutoVXLANCfgResource{}
	rsrc.StateDriver = vxlanRsrcStateDriver
	rsrc.ID = VXLANRsrcValidInitID
	err := rsrc.Init(&vxlanRsrcValidationStateMap[rsrc.ID].expCfg[0])
	if err != nil {
		t.Fatalf("VXLAN resource init failed. Error: %s", err)
	}
}

func TestAutoVXLANCfgResourceDeInit(t *testing.T) {
	rsrc := &AutoVXLANCfgResource{}
	rsrc.StateDriver = vxlanRsrcStateDriver
	rsrc.ID = VXLANRsrcValidDeinitID
	err := rsrc.Init(&vxlanRsrcValidationStateMap[rsrc.ID].expCfg[0])
	if err != nil {
		t.Fatalf("VXLAN resource init failed. Error: %s", err)
	}

	rsrc.Deinit()
}

func TestAutoVXLANCfgResourceAllocate(t *testing.T) {
	rsrc := &AutoVXLANCfgResource{}
	rsrc.StateDriver = vxlanRsrcStateDriver
	rsrc.ID = VXLANRsrcAllocateID
	err := rsrc.Init(&vxlanRsrcValidationStateMap[rsrc.ID].expCfg[0])
	if err != nil {
		t.Fatalf("VXLAN resource init failed. Error: %s", err)
	}

	p, err1 := rsrc.Allocate(uint(0))
	if err1 != nil {
		t.Fatalf("VXLAN resource allocation failed. Error: %s", err1)
	}
	pair := p.(VXLANVLANPair)
	if pair.VXLAN != 0 || pair.VLAN != 0 {
		t.Fatalf("Allocated vxlan mismatch. expected: (0,0), rcvd: %v", pair)
	}
}

func TestAutoVXLANCfgResourceAllocateVXLANExhaustion(t *testing.T) {
	rsrc := &AutoVXLANCfgResource{}
	rsrc.StateDriver = vxlanRsrcStateDriver
	rsrc.ID = VXLANRsrcAllocateExhaustVXLANID
	err := rsrc.Init(&vxlanRsrcValidationStateMap[rsrc.ID].expCfg[0])
	if err != nil {
		t.Fatalf("VXLAN resource init failed. Error: %s", err)
	}

	_, err = rsrc.Allocate(uint(0))
	if err == nil {
		t.Fatalf("VXLAN resource allocation succeeded, expected to fail!")
	}
	if !strings.Contains(err.Error(), "no vxlans available") {
		t.Fatalf("VXLAN resource allocation failure reason mismatch. Expected: %s, rcvd: %s",
			"no vxlans available", err)
	}
}

func TestAutoVXLANCfgResourceAllocateVLANExhaustion(t *testing.T) {
	rsrc := &AutoVXLANCfgResource{}
	rsrc.StateDriver = vxlanRsrcStateDriver
	rsrc.ID = VXLANRsrcAllocateExhaustVLANID
	err := rsrc.Init(&vxlanRsrcValidationStateMap[rsrc.ID].expCfg[0])
	if err != nil {
		t.Fatalf("VXLAN resource init failed. Error: %s", err)
	}

	_, err = rsrc.Allocate(uint(0))
	if err == nil {
		t.Fatalf("VXLAN resource allocation succeeded, expected to fail!")
	}
	if !strings.Contains(err.Error(), "no local vlans available") {
		t.Fatalf("VXLAN resource allocation failure reason mismatch. Expected: %s, rcvd: %s",
			"no local vlans available", err)
	}
}

func TestAutoVXLANCfgResourceDeAllocate(t *testing.T) {
	rsrc := &AutoVXLANCfgResource{}
	rsrc.StateDriver = vxlanRsrcStateDriver
	rsrc.ID = VXLANRsrcDeallocateID
	err := rsrc.Init(&vxlanRsrcValidationStateMap[rsrc.ID].expCfg[0])
	if err != nil {
		t.Fatalf("VXLAN resource init failed. Error: %s", err)
	}

	pair, err1 := rsrc.Allocate(uint(0))
	if err1 != nil {
		t.Fatalf("VXLAN resource allocation failed. Error: %s", err1)
	}

	err = rsrc.Deallocate(pair)
	if err != nil {
		t.Fatalf("VXLAN resource deallocation failed. Error: %s", err)
	}
}

func TestAutoVXLANCfgResourceGetList(t *testing.T) {
	rsrc := &AutoVXLANCfgResource{}
	rsrc.StateDriver = vxlanRsrcStateDriver
	rsrc.ID = VXLANRsrcGetListID
	err := rsrc.Init(&vxlanRsrcValidationStateMap[rsrc.ID].expCfg[0])
	if err != nil {
		t.Fatalf("VXLAN resource init failed. Error: %s", err)
	}

	if _, err1 := rsrc.Allocate(uint(1)); err1 != nil {
		t.Fatalf("VXLAN resource allocation failed. Error: %s", err1)
	}
	if _, err1 := rsrc.Allocate(uint(100)); err1 != nil {
		t.Fatalf("VXLAN resource allocation failed. Error: %s", err1)
	}
	if _, err1 := rsrc.Allocate(uint(101)); err1 != nil {
		t.Fatalf("VXLAN resource allocation failed. Error: %s", err1)
	}

	expectedList := "1, 100-101"
	numVxlans, vxlansInUse := rsrc.GetList()
	if numVxlans != 3 || vxlansInUse != expectedList {
		t.Fatalf("GetList failure, got '%s' vxlanlist (%d vxlans) expected List '%s' ", vxlansInUse, numVxlans, expectedList)
	}
}
