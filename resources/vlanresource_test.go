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

var vlanRsrcStateDriver *testVlanRsrcStateDriver = &testVlanRsrcStateDriver{}

type vlanRsrcValidator struct {
	// slice (stack) of expected config and oper states.
	// nextState modifies this slice after every state validate (write)
	// or copy (read)
	expCfg  []AutoVlanCfgResource
	expOper []AutoVlanOperResource
}

func (vt *vlanRsrcValidator) nextCfgState() {
	vt.expCfg = vt.expCfg[1:]
	if len(vt.expCfg) > 0 {
		log.Printf("after pop cfg is: %+v\n", vt.expCfg[0])
	} else {
		log.Printf("cfg becomes empty.\n")
	}
}

func (vt *vlanRsrcValidator) nextOperState() {
	vt.expOper = vt.expOper[1:]
	if len(vt.expOper) > 0 {
		log.Printf("after pop oper is: %+v\n", vt.expOper[0])
	} else {
		log.Printf("oper becomes empty.\n")
	}
}

func (vt *vlanRsrcValidator) ValidateState(state core.State) error {
	rcvdCfg, okCfg := state.(*AutoVlanCfgResource)
	if okCfg {
		log.Printf("cfg length: %d", len(vt.expCfg))
		if rcvdCfg.Id != vt.expCfg[0].Id ||
			!rcvdCfg.Vlans.Equal(vt.expCfg[0].Vlans) {
			errStr := fmt.Sprintf("cfg mismatch. Expctd: %+v, Rcvd: %+v",
				vt.expCfg[0], rcvdCfg)
			//panic so we can catch the exact backtrace
			panic(errStr)
		}
		vt.nextCfgState()
		return nil
	}

	rcvdOper, okOper := state.(*AutoVlanOperResource)
	if okOper {
		log.Printf("oper length: %d", len(vt.expOper))
		if rcvdOper.Id != vt.expOper[0].Id ||
			!rcvdOper.FreeVlans.Equal(vt.expOper[0].FreeVlans) {
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
	rcvdCfg, okCfg := state.(*AutoVlanCfgResource)
	if okCfg {
		rcvdCfg.Id = vt.expCfg[0].Id
		rcvdCfg.Vlans = vt.expCfg[0].Vlans.Clone()
		vt.nextCfgState()
		return nil
	}

	rcvdOper, okOper := state.(*AutoVlanOperResource)
	if okOper {
		rcvdOper.Id = vt.expOper[0].Id
		rcvdOper.FreeVlans = vt.expOper[0].FreeVlans.Clone()
		vt.nextOperState()
		return nil
	}

	return core.Errorf("unknown state object type!")
}

type vlanRsrcValidateOp int

const (
	VlanRsrcValidInitId       = "VlanRsrcValidInitId"
	VlanRsrcValidDeinitId     = "VlanRsrcValidDeinitId"
	VlanRsrcAllocateId        = "VlanRsrcAllocateId"
	VlanRsrcAllocateExhaustId = "VlanRsrcAllocateExhaustId"
	VlanRsrcDeallocateId      = "VlanRsrcDeallocateId"

	VLAN_RSRC_OP_WRITE = iota
	VLAN_RSRC_OP_READ
	VLAN_RSRC_OP_CLEAR
)

var vlanRsrcValidationStateMap map[string]*vlanRsrcValidator = map[string]*vlanRsrcValidator{
	VlanRsrcValidInitId: &vlanRsrcValidator{
		expCfg: []AutoVlanCfgResource{
			{
				CommonState: core.CommonState{nil, VlanRsrcValidInitId},
				Vlans:       bitset.New(1).Set(1),
			},
		},
		expOper: []AutoVlanOperResource{
			{
				CommonState: core.CommonState{nil, VlanRsrcValidInitId},
				FreeVlans:   bitset.New(1).Set(1),
			},
		},
	},
	VlanRsrcValidDeinitId: &vlanRsrcValidator{
		expCfg: []AutoVlanCfgResource{
			{
				CommonState: core.CommonState{nil, VlanRsrcValidDeinitId},
				Vlans:       bitset.New(1).Set(0),
			},
		},
		expOper: []AutoVlanOperResource{
			{
				CommonState: core.CommonState{nil, VlanRsrcValidDeinitId},
				FreeVlans:   bitset.New(1).Set(0),
			},
			{
				CommonState: core.CommonState{nil, VlanRsrcValidDeinitId},
				FreeVlans:   bitset.New(1).Set(0),
			},
		},
	},
	VlanRsrcAllocateId: &vlanRsrcValidator{
		expCfg: []AutoVlanCfgResource{
			{
				CommonState: core.CommonState{nil, VlanRsrcAllocateId},
				Vlans:       bitset.New(1).Set(0),
			},
		},
		expOper: []AutoVlanOperResource{
			{
				CommonState: core.CommonState{nil, VlanRsrcAllocateId},
				FreeVlans:   bitset.New(1).Set(0),
			},
			{
				CommonState: core.CommonState{nil, VlanRsrcAllocateId},
				FreeVlans:   bitset.New(1).Set(0),
			},
			{
				CommonState: core.CommonState{nil, VlanRsrcAllocateId},
				FreeVlans:   bitset.New(1).Clear(0),
			},
		},
	},
	VlanRsrcAllocateExhaustId: &vlanRsrcValidator{
		expCfg: []AutoVlanCfgResource{
			{
				CommonState: core.CommonState{nil, VlanRsrcAllocateExhaustId},
				Vlans:       bitset.New(1).Clear(0),
			},
		},
		expOper: []AutoVlanOperResource{
			{
				CommonState: core.CommonState{nil, VlanRsrcAllocateExhaustId},
				FreeVlans:   bitset.New(1).Clear(0),
			},
			{
				CommonState: core.CommonState{nil, VlanRsrcAllocateExhaustId},
				FreeVlans:   bitset.New(1).Clear(0),
			},
		},
	},
	VlanRsrcDeallocateId: &vlanRsrcValidator{
		expCfg: []AutoVlanCfgResource{
			{
				CommonState: core.CommonState{nil, VlanRsrcDeallocateId},
				Vlans:       bitset.New(1).Set(0),
			},
		},
		expOper: []AutoVlanOperResource{
			{
				CommonState: core.CommonState{nil, VlanRsrcDeallocateId},
				FreeVlans:   bitset.New(1).Set(0),
			},
			{
				CommonState: core.CommonState{nil, VlanRsrcDeallocateId},
				FreeVlans:   bitset.New(1).Set(0),
			},
			{
				CommonState: core.CommonState{nil, VlanRsrcDeallocateId},
				FreeVlans:   bitset.New(1).Clear(0),
			},
			{
				CommonState: core.CommonState{nil, VlanRsrcDeallocateId},
				FreeVlans:   bitset.New(1).Clear(0),
			},
			{
				CommonState: core.CommonState{nil, VlanRsrcDeallocateId},
				FreeVlans:   bitset.New(1).Set(0),
			},
		},
	},
}

type testVlanRsrcStateDriver struct {
}

func (d *testVlanRsrcStateDriver) Init(config *core.Config) error {
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
		log.Printf("%s\n", errStr)
		return core.Errorf(errStr)
	}

	switch op {
	case VLAN_RSRC_OP_WRITE:
		err := v.ValidateState(state)
		if err != nil {
			return err
		}
		return nil
	case VLAN_RSRC_OP_READ:
		return v.CopyState(state)
	case VLAN_RSRC_OP_CLEAR:
		fallthrough
	default:
		return nil
	}
}

func (d *testVlanRsrcStateDriver) ClearState(key string) error {
	return d.validate(key, nil, VLAN_RSRC_OP_CLEAR)
}

func (d *testVlanRsrcStateDriver) ReadState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) error {
	return d.validate(key, value, VLAN_RSRC_OP_READ)
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
	return d.validate(key, value, VLAN_RSRC_OP_WRITE)
}

func TestAutoVlanCfgResourceInit(t *testing.T) {
	rsrc := &AutoVlanCfgResource{}
	rsrc.StateDriver = vlanRsrcStateDriver
	rsrc.Id = VlanRsrcValidInitId
	vlans := vlanRsrcValidationStateMap[rsrc.Id].expCfg[0].Vlans.Clone()
	err := rsrc.Init(vlans)
	if err != nil {
		t.Fatalf("Vlan resource init failed. Error: %s", err)
	}
}

func TestAutoVlanCfgResourceDeInit(t *testing.T) {
	rsrc := &AutoVlanCfgResource{}
	rsrc.StateDriver = vlanRsrcStateDriver
	rsrc.Id = VlanRsrcValidDeinitId
	vlans := vlanRsrcValidationStateMap[rsrc.Id].expCfg[0].Vlans.Clone()
	err := rsrc.Init(vlans)
	if err != nil {
		t.Fatalf("Vlan resource init failed. Error: %s", err)
	}

	rsrc.Deinit()
}

func TestAutoVlanCfgResourceAllocate(t *testing.T) {
	rsrc := &AutoVlanCfgResource{}
	rsrc.StateDriver = vlanRsrcStateDriver
	rsrc.Id = VlanRsrcAllocateId
	vlans := vlanRsrcValidationStateMap[rsrc.Id].expCfg[0].Vlans.Clone()
	err := rsrc.Init(vlans)
	if err != nil {
		t.Fatalf("Vlan resource init failed. Error: %s", err)
	}

	vlan, err1 := rsrc.Allocate()
	if err1 != nil {
		t.Fatalf("Vlan resource allocation failed. Error: %s", err1)
	}
	if vlan.(uint) != 0 {
		t.Fatalf("Allocated vlan mismatch. expected: 0, rcvd: %u", vlan)
	}
}

func TestAutoVlanCfgResourceAllocateExhaustion(t *testing.T) {
	rsrc := &AutoVlanCfgResource{}
	rsrc.StateDriver = vlanRsrcStateDriver
	rsrc.Id = VlanRsrcAllocateExhaustId
	vlans := vlanRsrcValidationStateMap[rsrc.Id].expCfg[0].Vlans.Clone()
	err := rsrc.Init(vlans)
	if err != nil {
		t.Fatalf("Vlan resource init failed. Error: %s", err)
	}

	_, err = rsrc.Allocate()
	if err == nil {
		t.Fatalf("Vlan resource allocation succeeded, expected to fail!")
	}
	if !strings.Contains(err.Error(), "no vlans available.") {
		t.Fatalf("Vlan resource allocation failure reason mismatch. Expected: %s, rcvd: %s",
			"no vlans available.", err)
	}
}

func TestAutoVlanCfgResourceDeAllocate(t *testing.T) {
	rsrc := &AutoVlanCfgResource{}
	rsrc.StateDriver = vlanRsrcStateDriver
	rsrc.Id = VlanRsrcDeallocateId
	vlans := vlanRsrcValidationStateMap[rsrc.Id].expCfg[0].Vlans.Clone()
	err := rsrc.Init(vlans)
	if err != nil {
		t.Fatalf("Vlan resource init failed. Error: %s", err)
	}

	vlan, err1 := rsrc.Allocate()
	if err1 != nil {
		t.Fatalf("Vlan resource allocation failed. Error: %s", err1)
	}

	err = rsrc.Deallocate(vlan)
	if err != nil {
		t.Fatalf("Vlan resource deallocation failed. Error: %s", err)
	}
}
