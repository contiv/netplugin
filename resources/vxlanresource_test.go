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
	"log"
	"strings"
	"testing"

	"github.com/contiv/netplugin/core"
	"github.com/jainvipin/bitset"
)

var vxlanRsrcStateDriver *testVxlanRsrcStateDriver = &testVxlanRsrcStateDriver{}

type vxlanRsrcValidator struct {
	// slice (stack) of expected config and oper states.
	// nextState modifies this slice after every state validate (write)
	// or copy (read)
	expCfg  []AutoVxlanCfgResource
	expOper []AutoVxlanOperResource
}

func (vt *vxlanRsrcValidator) nextCfgState() {
	vt.expCfg = vt.expCfg[1:]
	if len(vt.expCfg) > 0 {
		log.Printf("after pop cfg is: %+v\n", vt.expCfg[0])
	} else {
		log.Printf("cfg becomes empty.\n")
	}
}

func (vt *vxlanRsrcValidator) nextOperState() {
	vt.expOper = vt.expOper[1:]
	if len(vt.expOper) > 0 {
		log.Printf("after pop oper is: %+v\n", vt.expOper[0])
	} else {
		log.Printf("oper becomes empty.\n")
	}
}

func (vt *vxlanRsrcValidator) ValidateState(state core.State) error {
	rcvdCfg, okCfg := state.(*AutoVxlanCfgResource)
	if okCfg {
		log.Printf("cfg length: %d", len(vt.expCfg))
		if rcvdCfg.Id() != vt.expCfg[0].Id() ||
			!rcvdCfg.Vxlans.Equal(vt.expCfg[0].Vxlans) ||
			!rcvdCfg.LocalVlans.Equal(vt.expCfg[0].LocalVlans) {
			errStr := fmt.Sprintf("cfg mismatch. Expctd: %+v, Rcvd: %+v",
				vt.expCfg[0], rcvdCfg)
			//panic so we can catch the exact backtrace
			panic(errStr)
		}
		vt.nextCfgState()
		return nil
	}

	rcvdOper, okOper := state.(*AutoVxlanOperResource)
	if okOper {
		log.Printf("oper length: %d", len(vt.expOper))
		if rcvdOper.Id != vt.expOper[0].Id ||
			!rcvdOper.FreeVxlans.Equal(vt.expOper[0].FreeVxlans) ||
			!rcvdOper.FreeLocalVlans.Equal(vt.expOper[0].FreeLocalVlans) {
			errStr := fmt.Sprintf("oper mismatch. Expctd: %+v, Rcvd: %+v",
				vt.expOper[0], rcvdOper)
			//panic so we can catch the exact backtrace
			panic(errStr)
		}
		vt.nextOperState()
		return nil
	}

	return &core.Error{Desc: "unknown state object type!"}
}

func (vt *vxlanRsrcValidator) CopyState(state core.State) error {
	rcvdCfg, okCfg := state.(*AutoVxlanCfgResource)
	if okCfg {
		rcvdCfg.SetId(vt.expCfg[0].Id())
		rcvdCfg.Vxlans = vt.expCfg[0].Vxlans.Clone()
		rcvdCfg.LocalVlans = vt.expCfg[0].LocalVlans.Clone()
		vt.nextCfgState()
		return nil
	}

	rcvdOper, okOper := state.(*AutoVxlanOperResource)
	if okOper {
		rcvdOper.Id = vt.expOper[0].Id
		rcvdOper.FreeVxlans = vt.expOper[0].FreeVxlans.Clone()
		rcvdOper.FreeLocalVlans = vt.expOper[0].FreeLocalVlans.Clone()
		vt.nextOperState()
		return nil
	}

	return &core.Error{Desc: "unknown state object type!"}
}

type vxlanRsrcValidateOp int

const (
	VxlanRsrcValidInitId            = "VxlanRsrcValidInitId"
	VxlanRsrcValidDeinitId          = "VxlanRsrcValidDeinitId"
	VxlanRsrcAllocateId             = "VxlanRsrcAllocateId"
	VxlanRsrcAllocateExhaustVxlanId = "VxlanRsrcAllocateExhaustVxlanId"
	VxlanRsrcAllocateExhaustVlanId  = "VxlanRsrcAllocateExhaustVlanId"
	VxlanRsrcDeallocateId           = "VxlanRsrcDeallocateId"

	VXLAN_RSRC_OP_WRITE = iota
	VXLAN_RSRC_OP_READ
	VXLAN_RSRC_OP_CLEAR
)

var vxlanRsrcValidationStateMap map[string]*vxlanRsrcValidator = map[string]*vxlanRsrcValidator{
	VxlanRsrcValidInitId: &vxlanRsrcValidator{
		expCfg: []AutoVxlanCfgResource{
			{
				ResId:      VxlanRsrcValidInitId,
				Vxlans:     bitset.New(1).Set(0),
				LocalVlans: bitset.New(1).Set(0),
			},
		},
		expOper: []AutoVxlanOperResource{
			{
				Id:             VxlanRsrcValidInitId,
				FreeVxlans:     bitset.New(1).Set(0),
				FreeLocalVlans: bitset.New(1).Set(0),
			},
		},
	},
	VxlanRsrcValidDeinitId: &vxlanRsrcValidator{
		expCfg: []AutoVxlanCfgResource{
			{
				ResId:      VxlanRsrcValidDeinitId,
				Vxlans:     bitset.New(1).Set(0),
				LocalVlans: bitset.New(1).Set(0),
			},
		},
		expOper: []AutoVxlanOperResource{
			{
				Id:             VxlanRsrcValidDeinitId,
				FreeVxlans:     bitset.New(1).Set(0),
				FreeLocalVlans: bitset.New(1).Set(0),
			},
			{
				Id:             VxlanRsrcValidDeinitId,
				FreeVxlans:     bitset.New(1).Set(0),
				FreeLocalVlans: bitset.New(1).Set(0),
			},
		},
	},
	VxlanRsrcAllocateId: &vxlanRsrcValidator{
		expCfg: []AutoVxlanCfgResource{
			{
				ResId:      VxlanRsrcAllocateId,
				Vxlans:     bitset.New(1).Set(0),
				LocalVlans: bitset.New(1).Set(0),
			},
		},
		expOper: []AutoVxlanOperResource{
			{
				Id:             VxlanRsrcAllocateId,
				FreeVxlans:     bitset.New(1).Set(0),
				FreeLocalVlans: bitset.New(1).Set(0),
			},
			{
				Id:             VxlanRsrcAllocateId,
				FreeVxlans:     bitset.New(1).Set(0),
				FreeLocalVlans: bitset.New(1).Set(0),
			},
			{
				Id:             VxlanRsrcAllocateId,
				FreeVxlans:     bitset.New(1).Clear(0),
				FreeLocalVlans: bitset.New(1).Clear(0),
			},
		},
	},
	VxlanRsrcAllocateExhaustVxlanId: &vxlanRsrcValidator{
		expCfg: []AutoVxlanCfgResource{
			{
				ResId:      VxlanRsrcAllocateExhaustVxlanId,
				Vxlans:     bitset.New(1).Clear(0),
				LocalVlans: bitset.New(1).Clear(0),
			},
		},
		expOper: []AutoVxlanOperResource{
			{
				Id:             VxlanRsrcAllocateExhaustVxlanId,
				FreeVxlans:     bitset.New(1).Clear(0),
				FreeLocalVlans: bitset.New(1).Clear(0),
			},
			{
				Id:             VxlanRsrcAllocateExhaustVxlanId,
				FreeVxlans:     bitset.New(1).Clear(0),
				FreeLocalVlans: bitset.New(1).Clear(0),
			},
		},
	},
	VxlanRsrcAllocateExhaustVlanId: &vxlanRsrcValidator{
		expCfg: []AutoVxlanCfgResource{
			{
				ResId:      VxlanRsrcAllocateExhaustVlanId,
				Vxlans:     bitset.New(1).Set(0),
				LocalVlans: bitset.New(1).Clear(0),
			},
		},
		expOper: []AutoVxlanOperResource{
			{
				Id:             VxlanRsrcAllocateExhaustVlanId,
				FreeVxlans:     bitset.New(1).Set(0),
				FreeLocalVlans: bitset.New(1).Clear(0),
			},
			{
				Id:             VxlanRsrcAllocateExhaustVlanId,
				FreeVxlans:     bitset.New(1).Set(0),
				FreeLocalVlans: bitset.New(1).Clear(0),
			},
		},
	},
	VxlanRsrcDeallocateId: &vxlanRsrcValidator{
		expCfg: []AutoVxlanCfgResource{
			{
				ResId:      VxlanRsrcDeallocateId,
				Vxlans:     bitset.New(1).Set(0),
				LocalVlans: bitset.New(1).Set(0),
			},
		},
		expOper: []AutoVxlanOperResource{
			{
				Id:             VxlanRsrcDeallocateId,
				FreeVxlans:     bitset.New(1).Set(0),
				FreeLocalVlans: bitset.New(1).Set(0),
			},
			{
				Id:             VxlanRsrcDeallocateId,
				FreeVxlans:     bitset.New(1).Set(0),
				FreeLocalVlans: bitset.New(1).Set(0),
			},
			{
				Id:             VxlanRsrcDeallocateId,
				FreeVxlans:     bitset.New(1).Clear(0),
				FreeLocalVlans: bitset.New(1).Clear(0),
			},
			{
				Id:             VxlanRsrcDeallocateId,
				FreeVxlans:     bitset.New(1).Clear(0),
				FreeLocalVlans: bitset.New(1).Clear(0),
			},
			{
				Id:             VxlanRsrcDeallocateId,
				FreeVxlans:     bitset.New(1).Set(0),
				FreeLocalVlans: bitset.New(1).Set(0),
			},
		},
	},
}

type testVxlanRsrcStateDriver struct {
}

func (d *testVxlanRsrcStateDriver) Init(config *core.Config) error {
	return &core.Error{Desc: "Shouldn't be called!"}
}

func (d *testVxlanRsrcStateDriver) Deinit() {
}

func (d *testVxlanRsrcStateDriver) Write(key string, value []byte) error {
	return &core.Error{Desc: "Shouldn't be called!"}
}

func (d *testVxlanRsrcStateDriver) Read(key string) ([]byte, error) {
	return nil, &core.Error{Desc: "Shouldn't be called!"}
}

func (d *testVxlanRsrcStateDriver) ReadAll(baseKey string) ([][]byte, error) {
	return nil, &core.Error{Desc: "Shouldn't be called!"}
}

func (d *testVxlanRsrcStateDriver) validate(key string, state core.State,
	op vxlanRsrcValidateOp) error {
	strs := strings.Split(key, "/")
	id := strs[len(strs)-1]
	v, ok := vxlanRsrcValidationStateMap[id]
	if !ok {
		errStr := fmt.Sprintf("No matching validation entry for id: %s", id)
		log.Printf("%s\n", errStr)
		return &core.Error{Desc: errStr}
	}

	switch op {
	case VXLAN_RSRC_OP_WRITE:
		err := v.ValidateState(state)
		if err != nil {
			return err
		}
		return nil
	case VXLAN_RSRC_OP_READ:
		return v.CopyState(state)
	case VXLAN_RSRC_OP_CLEAR:
		fallthrough
	default:
		return nil
	}
}

func (d *testVxlanRsrcStateDriver) ClearState(key string) error {
	return d.validate(key, nil, VXLAN_RSRC_OP_CLEAR)
}

func (d *testVxlanRsrcStateDriver) ReadState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) error {
	return d.validate(key, value, VXLAN_RSRC_OP_READ)
}

func (d *testVxlanRsrcStateDriver) WriteState(key string, value core.State,
	marshal func(interface{}) ([]byte, error)) error {
	return d.validate(key, value, VXLAN_RSRC_OP_WRITE)
}

func TestAutoVxlanCfgResourceInit(t *testing.T) {
	rsrc := &AutoVxlanCfgResource{}
	rsrc.SetStateDriver(vxlanRsrcStateDriver)
	rsrc.SetId(VxlanRsrcValidInitId)
	err := rsrc.Init(&vxlanRsrcValidationStateMap[rsrc.Id()].expCfg[0])
	if err != nil {
		t.Fatalf("Vxlan resource init failed. Error: %s", err)
	}
}

func TestAutoVxlanCfgResourceDeInit(t *testing.T) {
	rsrc := &AutoVxlanCfgResource{}
	rsrc.SetStateDriver(vxlanRsrcStateDriver)
	rsrc.SetId(VxlanRsrcValidDeinitId)
	err := rsrc.Init(&vxlanRsrcValidationStateMap[rsrc.Id()].expCfg[0])
	if err != nil {
		t.Fatalf("Vxlan resource init failed. Error: %s", err)
	}

	rsrc.Deinit()
}

func TestAutoVxlanCfgResourceAllocate(t *testing.T) {
	rsrc := &AutoVxlanCfgResource{}
	rsrc.SetStateDriver(vxlanRsrcStateDriver)
	rsrc.SetId(VxlanRsrcAllocateId)
	err := rsrc.Init(&vxlanRsrcValidationStateMap[rsrc.Id()].expCfg[0])
	if err != nil {
		t.Fatalf("Vxlan resource init failed. Error: %s", err)
	}

	p, err1 := rsrc.Allocate()
	if err1 != nil {
		t.Fatalf("Vxlan resource allocation failed. Error: %s", err1)
	}
	pair := p.(VxlanVlanPair)
	if pair.Vxlan != 0 || pair.Vlan != 0 {
		t.Fatalf("Allocated vxlan mismatch. expected: (0,0), rcvd: %v", pair)
	}
}

func TestAutoVxlanCfgResourceAllocateVxlanExhaustion(t *testing.T) {
	rsrc := &AutoVxlanCfgResource{}
	rsrc.SetStateDriver(vxlanRsrcStateDriver)
	rsrc.SetId(VxlanRsrcAllocateExhaustVxlanId)
	err := rsrc.Init(&vxlanRsrcValidationStateMap[rsrc.Id()].expCfg[0])
	if err != nil {
		t.Fatalf("Vxlan resource init failed. Error: %s", err)
	}

	_, err = rsrc.Allocate()
	if err == nil {
		t.Fatalf("Vxlan resource allocation succeeded, expected to fail!")
	}
	if err.Error() != "no vxlans available." {
		t.Fatalf("Vxlan resource allocation failure reason mismatch. Expected: %s, rcvd: %s",
			"no vxlans available.", err)
	}
}

func TestAutoVxlanCfgResourceAllocateVlanExhaustion(t *testing.T) {
	rsrc := &AutoVxlanCfgResource{}
	rsrc.SetStateDriver(vxlanRsrcStateDriver)
	rsrc.SetId(VxlanRsrcAllocateExhaustVlanId)
	err := rsrc.Init(&vxlanRsrcValidationStateMap[rsrc.Id()].expCfg[0])
	if err != nil {
		t.Fatalf("Vxlan resource init failed. Error: %s", err)
	}

	_, err = rsrc.Allocate()
	if err == nil {
		t.Fatalf("Vxlan resource allocation succeeded, expected to fail!")
	}
	if err.Error() != "no local vlans available." {
		t.Fatalf("Vxlan resource allocation failure reason mismatch. Expected: %s, rcvd: %s",
			"no local vlans available.", err)
	}
}

func TestAutoVxlanCfgResourceDeAllocate(t *testing.T) {
	rsrc := &AutoVxlanCfgResource{}
	rsrc.SetStateDriver(vxlanRsrcStateDriver)
	rsrc.SetId(VxlanRsrcDeallocateId)
	err := rsrc.Init(&vxlanRsrcValidationStateMap[rsrc.Id()].expCfg[0])
	if err != nil {
		t.Fatalf("Vxlan resource init failed. Error: %s", err)
	}

	pair, err1 := rsrc.Allocate()
	if err1 != nil {
		t.Fatalf("Vxlan resource allocation failed. Error: %s", err1)
	}

	err = rsrc.Deallocate(pair)
	if err != nil {
		t.Fatalf("Vxlan resource deallocation failed. Error: %s", err)
	}
}
