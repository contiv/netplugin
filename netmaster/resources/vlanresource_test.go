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

var vlanRsrcStateDriver = &testVlanRsrcStateDriver{}

type vlanRsrcValidator struct {
	// slice (stack) of expected config and oper states.
	// nextState modifies this slice after every state validate (write)
	// or copy (read)
	expCfg  []AutoVLANCfgResource
	expOper []AutoVLANOperResource
}

func (vt *vlanRsrcValidator) nextCfgState() {
	vt.expCfg = vt.expCfg[1:]
	if len(vt.expCfg) > 0 {
		log.Debugf("after pop cfg is: %+v\n", vt.expCfg[0])
	} else {
		log.Debugf("cfg becomes empty.\n")
	}
}

func (vt *vlanRsrcValidator) nextOperState() {
	vt.expOper = vt.expOper[1:]
	if len(vt.expOper) > 0 {
		log.Debugf("after pop oper is: %+v\n", vt.expOper[0])
	} else {
		log.Debugf("oper becomes empty.\n")
	}
}

func (vt *vlanRsrcValidator) ValidateState(state core.State) error {
	rcvdCfg, okCfg := state.(*AutoVLANCfgResource)
	if okCfg {
		log.Debugf("cfg length: %d", len(vt.expCfg))
		if rcvdCfg.ID != vt.expCfg[0].ID ||
			!rcvdCfg.VLANs.Equal(vt.expCfg[0].VLANs) {
			errStr := fmt.Sprintf("cfg mismatch. Expctd: %+v, Rcvd: %+v",
				vt.expCfg[0], rcvdCfg)
			//panic so we can catch the exact backtrace
			panic(errStr)
		}
		vt.nextCfgState()
		return nil
	}

	rcvdOper, okOper := state.(*AutoVLANOperResource)
	if okOper {
		log.Debugf("oper length: %d", len(vt.expOper))
		if rcvdOper.ID != vt.expOper[0].ID ||
			!rcvdOper.FreeVLANs.Equal(vt.expOper[0].FreeVLANs) {
			fmt.Printf("rcvdOper.ID = %s expOperId = %s \n", rcvdOper.ID, vt.expOper[0].ID)
			fmt.Printf("RcvdFreeVlans = %s, ExpFreeVlans = %s\n", rcvdOper.FreeVLANs.DumpAsBits(),
				vt.expOper[0].FreeVLANs.DumpAsBits())
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

func (vt *vlanRsrcValidator) CopyState(state core.State) error {
	rcvdCfg, okCfg := state.(*AutoVLANCfgResource)
	if okCfg {
		rcvdCfg.ID = vt.expCfg[0].ID
		rcvdCfg.VLANs = vt.expCfg[0].VLANs.Clone()
		vt.nextCfgState()
		return nil
	}

	rcvdOper, okOper := state.(*AutoVLANOperResource)
	if okOper {
		rcvdOper.ID = vt.expOper[0].ID
		rcvdOper.FreeVLANs = vt.expOper[0].FreeVLANs.Clone()
		vt.nextOperState()
		return nil
	}

	return core.Errorf("unknown state object type!")
}

type vlanRsrcValidateOp int

const (
	VlanRsrcValidInitID       = "VlanRsrcValidInitID"
	VlanRsrcValidDeinitID     = "VlanRsrcValidDeinitID"
	VlanRsrcAllocateID        = "VlanRsrcAllocateID"
	VlanRsrcAllocateExhaustID = "VlanRsrcAllocateExhaustID"
	VlanRsrcDeallocateID      = "VlanRsrcDeallocateID"
	VlanRsrcGetListID         = "VlanRsrcGetListID"

	vLANResourceOperWrite = iota
	vLANResourceOperRead
	vLANResourceOperClear
)

var vlanRsrcValidationStateMap = map[string]*vlanRsrcValidator{
	VlanRsrcValidInitID: {
		expCfg: []AutoVLANCfgResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcValidInitID},
				VLANs:       bitset.New(1).Set(1),
			},
		},
		expOper: []AutoVLANOperResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcValidInitID},
				FreeVLANs:   bitset.New(1).Set(1),
			},
		},
	},
	VlanRsrcValidDeinitID: {
		expCfg: []AutoVLANCfgResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcValidDeinitID},
				VLANs:       bitset.New(1).Set(0),
			},
		},
		expOper: []AutoVLANOperResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcValidDeinitID},
				FreeVLANs:   bitset.New(1).Set(0),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcValidDeinitID},
				FreeVLANs:   bitset.New(1).Set(0),
			},
		},
	},
	VlanRsrcAllocateID: {
		expCfg: []AutoVLANCfgResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcAllocateID},
				VLANs:       bitset.New(1).Set(0),
			},
		},
		expOper: []AutoVLANOperResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcAllocateID},
				FreeVLANs:   bitset.New(1).Set(0),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcAllocateID},
				FreeVLANs:   bitset.New(1).Set(0),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcAllocateID},
				FreeVLANs:   bitset.New(1).Clear(0),
			},
		},
	},
	VlanRsrcAllocateExhaustID: {
		expCfg: []AutoVLANCfgResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcAllocateExhaustID},
				VLANs:       bitset.New(1).Clear(0),
			},
		},
		expOper: []AutoVLANOperResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcAllocateExhaustID},
				FreeVLANs:   bitset.New(1).Clear(0),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcAllocateExhaustID},
				FreeVLANs:   bitset.New(1).Clear(0),
			},
		},
	},
	VlanRsrcDeallocateID: {
		expCfg: []AutoVLANCfgResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcDeallocateID},
				VLANs:       bitset.New(1).Set(0),
			},
		},
		expOper: []AutoVLANOperResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcDeallocateID},
				FreeVLANs:   bitset.New(1).Set(0),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcDeallocateID},
				FreeVLANs:   bitset.New(1).Set(0),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcDeallocateID},
				FreeVLANs:   bitset.New(1).Clear(0),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcDeallocateID},
				FreeVLANs:   bitset.New(1).Clear(0),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcDeallocateID},
				FreeVLANs:   bitset.New(1).Set(0),
			},
		},
	},
	VlanRsrcGetListID: {
		expCfg: []AutoVLANCfgResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcGetListID},
				VLANs:       bitset.New(20).Complement().Clear(0),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcGetListID},
				VLANs:       bitset.New(20).Complement().Clear(0),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcGetListID},
				VLANs:       bitset.New(20).Complement().Clear(0),
			},
		},
		expOper: []AutoVLANOperResource{
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcGetListID},
				FreeVLANs:   bitset.New(20).Complement().Clear(0),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcGetListID},
				FreeVLANs:   bitset.New(20).Complement().Clear(0),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcGetListID},
				FreeVLANs:   bitset.New(20).Complement().Clear(0).Clear(1),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcGetListID},
				FreeVLANs:   bitset.New(20).Complement().Clear(0).Clear(1),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcGetListID},
				FreeVLANs:   bitset.New(20).Complement().Clear(0).Clear(1).Clear(2),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcGetListID},
				FreeVLANs:   bitset.New(20).Complement().Clear(0).Clear(1).Clear(2),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcGetListID},
				FreeVLANs:   bitset.New(20).Complement().Clear(0).Clear(1).Clear(2).Clear(19),
			},
			{
				CommonState: core.CommonState{StateDriver: nil, ID: VlanRsrcGetListID},
				FreeVLANs:   bitset.New(20).Complement().Clear(0).Clear(1).Clear(2).Clear(19),
			},
		},
	},
}

type testVlanRsrcStateDriver struct {
}

func (d *testVlanRsrcStateDriver) Init(instInfo *core.InstanceInfo) error {
	return core.Errorf("Shouldn't be called!")
}

func (d *testVlanRsrcStateDriver) Deinit() {
}

func (d *testVlanRsrcStateDriver) Write(key string, value []byte) error {
	return core.Errorf("Shouldn't be called!")
}

func (d *testVlanRsrcStateDriver) Read(key string) ([]byte, error) {
	return nil, core.Errorf("Shouldn't be called!")
}

func (d *testVlanRsrcStateDriver) ReadAll(baseKey string) ([][]byte, error) {
	return nil, core.Errorf("Shouldn't be called!")
}

func (d *testVlanRsrcStateDriver) WatchAll(baseKey string, rsps chan [2][]byte) error {
	return core.Errorf("not supported")
}

func (d *testVlanRsrcStateDriver) validate(key string, state core.State,
	op vlanRsrcValidateOp) error {
	strs := strings.Split(key, "/")
	id := strs[len(strs)-1]
	v, ok := vlanRsrcValidationStateMap[id]
	if !ok {
		errStr := fmt.Sprintf("No matching validation entry for id: %s", id)
		log.Errorf("%s\n", errStr)
		return core.Errorf(errStr)
	}

	switch op {
	case vLANResourceOperWrite:
		err := v.ValidateState(state)
		if err != nil {
			return err
		}
		return nil
	case vLANResourceOperRead:
		return v.CopyState(state)
	case vLANResourceOperClear:
		fallthrough
	default:
		return nil
	}
}

func (d *testVlanRsrcStateDriver) ClearState(key string) error {
	return d.validate(key, nil, vLANResourceOperClear)
}

func (d *testVlanRsrcStateDriver) ReadState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) error {
	return d.validate(key, value, vLANResourceOperRead)
}

func (d *testVlanRsrcStateDriver) ReadAllState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) ([]core.State, error) {
	return nil, core.Errorf("Shouldn't be called!")
}

func (d *testVlanRsrcStateDriver) WatchAllState(baseKey string, sType core.State,
	unmarshal func([]byte, interface{}) error, rsps chan core.WatchState) error {
	return core.Errorf("not supported")
}

func (d *testVlanRsrcStateDriver) WriteState(key string, value core.State,
	marshal func(interface{}) ([]byte, error)) error {
	return d.validate(key, value, vLANResourceOperWrite)
}

func TestAutoVLANCfgResourceInit(t *testing.T) {
	rsrc := &AutoVLANCfgResource{}
	rsrc.StateDriver = vlanRsrcStateDriver
	rsrc.ID = VlanRsrcValidInitID
	vlans := vlanRsrcValidationStateMap[rsrc.ID].expCfg[0].VLANs.Clone()
	err := rsrc.Init(vlans)
	if err != nil {
		t.Fatalf("Vlan resource init failed. Error: %s", err)
	}
}

func TestAutoVLANCfgResourceDeInit(t *testing.T) {
	rsrc := &AutoVLANCfgResource{}
	rsrc.StateDriver = vlanRsrcStateDriver
	rsrc.ID = VlanRsrcValidDeinitID
	vlans := vlanRsrcValidationStateMap[rsrc.ID].expCfg[0].VLANs.Clone()
	err := rsrc.Init(vlans)
	if err != nil {
		t.Fatalf("Vlan resource init failed. Error: %s", err)
	}

	rsrc.Deinit()
}

func TestAutoVLANCfgResourceAllocate(t *testing.T) {
	rsrc := &AutoVLANCfgResource{}
	rsrc.StateDriver = vlanRsrcStateDriver
	rsrc.ID = VlanRsrcAllocateID
	vlans := vlanRsrcValidationStateMap[rsrc.ID].expCfg[0].VLANs.Clone()
	err := rsrc.Init(vlans)
	if err != nil {
		t.Fatalf("Vlan resource init failed. Error: %s", err)
	}

	vlan, err1 := rsrc.Allocate(uint(0))
	if err1 != nil {
		t.Fatalf("Vlan resource allocation failed. Error: %s", err1)
	}
	if vlan.(uint) != 0 {
		t.Fatalf("Allocated vlan mismatch. expected: 0, rcvd: %d", vlan)
	}
}

func TestAutoVLANCfgResourceAllocateExhaustion(t *testing.T) {
	rsrc := &AutoVLANCfgResource{}
	rsrc.StateDriver = vlanRsrcStateDriver
	rsrc.ID = VlanRsrcAllocateExhaustID
	vlans := vlanRsrcValidationStateMap[rsrc.ID].expCfg[0].VLANs.Clone()
	err := rsrc.Init(vlans)
	if err != nil {
		t.Fatalf("Vlan resource init failed. Error: %s", err)
	}

	_, err = rsrc.Allocate(uint(0))
	if err == nil {
		t.Fatalf("Vlan resource allocation succeeded, expected to fail!")
	}
	if !strings.Contains(err.Error(), "no vlans available") {
		t.Fatalf("Vlan resource allocation failure reason mismatch. Expected: %s, rcvd: %s",
			"no vlans available", err)
	}
}

func TestAutoVLANCfgResourceDeAllocate(t *testing.T) {
	rsrc := &AutoVLANCfgResource{}
	rsrc.StateDriver = vlanRsrcStateDriver
	rsrc.ID = VlanRsrcDeallocateID
	vlans := vlanRsrcValidationStateMap[rsrc.ID].expCfg[0].VLANs.Clone()
	err := rsrc.Init(vlans)
	if err != nil {
		t.Fatalf("Vlan resource init failed. Error: %s", err)
	}

	vlan, err1 := rsrc.Allocate(uint(0))
	if err1 != nil {
		t.Fatalf("Vlan resource allocation failed. Error: %s", err1)
	}

	err = rsrc.Deallocate(vlan)
	if err != nil {
		t.Fatalf("Vlan resource deallocation failed. Error: %s", err)
	}
}

func TestAutoVLANCfgResourceGetList(t *testing.T) {
	rsrc := &AutoVLANCfgResource{}
	rsrc.StateDriver = vlanRsrcStateDriver
	rsrc.ID = VlanRsrcGetListID
	vlans := vlanRsrcValidationStateMap[rsrc.ID].expCfg[0].VLANs.Clone()
	err := rsrc.Init(vlans)
	if err != nil {
		t.Fatalf("Vlan resource init failed. Error: %s", err)
	}

	if _, err1 := rsrc.Allocate(uint(1)); err1 != nil {
		t.Fatalf("Vlan resource allocation failed. Error: %s", err1)
	}
	if _, err1 := rsrc.Allocate(uint(2)); err1 != nil {
		t.Fatalf("Vlan resource allocation failed. Error: %s", err1)
	}
	if _, err1 := rsrc.Allocate(uint(19)); err1 != nil {
		t.Fatalf("Vlan resource allocation failed. Error: %s", err1)
	}
	expectedList := "1-2, 19"
	numVlans, vlansInUse := rsrc.GetList()
	if numVlans != 3 || vlansInUse != expectedList {
		t.Fatalf("GetList failure, got %s vlanlist (%d vlans), expected %s", vlansInUse, numVlans, expectedList)
	}
}
