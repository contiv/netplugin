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
	"net"
	"strings"
	"testing"

	"github.com/contiv/netplugin/core"
	"github.com/jainvipin/bitset"

	log "github.com/Sirupsen/logrus"
)

var subnetRsrcStateDriver *testSubnetRsrcStateDriver = &testSubnetRsrcStateDriver{}

type subnetRsrcValidator struct {
	// slice (stack) of expected config and oper states.
	// nextState modifies this slice after every state validate (write)
	// or copy (read)
	expCfg  []AutoSubnetCfgResource
	expOper []AutoSubnetOperResource
}

func (vt *subnetRsrcValidator) nextCfgState() {
	vt.expCfg = vt.expCfg[1:]
	if len(vt.expCfg) > 0 {
		log.Printf("after pop cfg is: %+v\n", vt.expCfg[0])
	} else {
		log.Printf("cfg becomes empty.\n")
	}
}

func (vt *subnetRsrcValidator) nextOperState() {
	vt.expOper = vt.expOper[1:]
	if len(vt.expOper) > 0 {
		log.Printf("after pop oper is: %+v\n", vt.expOper[0])
	} else {
		log.Printf("oper becomes empty.\n")
	}
}

func (vt *subnetRsrcValidator) ValidateState(state core.State) error {
	rcvdCfg, okCfg := state.(*AutoSubnetCfgResource)
	if okCfg {
		log.Printf("cfg length: %d", len(vt.expCfg))
		if rcvdCfg.Id != vt.expCfg[0].Id ||
			!rcvdCfg.SubnetPool.Equal(vt.expCfg[0].SubnetPool) ||
			rcvdCfg.SubnetPoolLen != vt.expCfg[0].SubnetPoolLen ||
			rcvdCfg.AllocSubnetLen != vt.expCfg[0].AllocSubnetLen {
			errStr := fmt.Sprintf("cfg mismatch. Expctd: %+v, Rcvd: %+v",
				vt.expCfg[0], rcvdCfg)
			//panic so we can catch the exact backtrace
			panic(errStr)
		}
		vt.nextCfgState()
		return nil
	}

	rcvdOper, okOper := state.(*AutoSubnetOperResource)
	if okOper {
		log.Printf("oper length: %d", len(vt.expOper))
		if rcvdOper.Id != vt.expOper[0].Id ||
			!rcvdOper.FreeSubnets.Equal(vt.expOper[0].FreeSubnets) {
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

func (vt *subnetRsrcValidator) CopyState(state core.State) error {
	rcvdCfg, okCfg := state.(*AutoSubnetCfgResource)
	if okCfg {
		rcvdCfg.Id = vt.expCfg[0].Id
		rcvdCfg.SubnetPool = vt.expCfg[0].SubnetPool
		rcvdCfg.SubnetPoolLen = vt.expCfg[0].SubnetPoolLen
		rcvdCfg.AllocSubnetLen = vt.expCfg[0].AllocSubnetLen
		vt.nextCfgState()
		return nil
	}

	rcvdOper, okOper := state.(*AutoSubnetOperResource)
	if okOper {
		rcvdOper.Id = vt.expOper[0].Id
		rcvdOper.FreeSubnets = vt.expOper[0].FreeSubnets.Clone()
		vt.nextOperState()
		return nil
	}

	return core.Errorf("unknown state object type!")
}

type subnetRsrcValidateOp int

const (
	SubnetRsrcValidInitId       = "SubnetRsrcValidInitId"
	SubnetRsrcValidDeinitId     = "SubnetRsrcValidDeinitId"
	SubnetRsrcAllocateId        = "SubnetRsrcAllocateId"
	SubnetRsrcAllocateExhaustId = "SubnetRsrcAllocateExhaustId"
	SubnetRsrcDeallocateId      = "SubnetRsrcDeallocateId"

	SUBNET_RSRC_OP_WRITE = iota
	SUBNET_RSRC_OP_READ
	SUBNET_RSRC_OP_CLEAR
)

var subnetRsrcValidationStateMap map[string]*subnetRsrcValidator = map[string]*subnetRsrcValidator{
	SubnetRsrcValidInitId: &subnetRsrcValidator{
		expCfg: []AutoSubnetCfgResource{
			{
				CommonState:    core.CommonState{nil, SubnetRsrcValidInitId},
				SubnetPool:     net.ParseIP("1.2.3.4"),
				SubnetPoolLen:  24,
				AllocSubnetLen: 24,
			},
		},
		expOper: []AutoSubnetOperResource{
			{
				CommonState: core.CommonState{nil, SubnetRsrcValidInitId},
				FreeSubnets: bitset.New(1).Set(0),
			},
		},
	},
	SubnetRsrcValidDeinitId: &subnetRsrcValidator{
		expCfg: []AutoSubnetCfgResource{
			{
				CommonState:    core.CommonState{nil, SubnetRsrcValidDeinitId},
				SubnetPool:     net.ParseIP("1.2.3.4"),
				SubnetPoolLen:  24,
				AllocSubnetLen: 24,
			},
		},
		expOper: []AutoSubnetOperResource{
			{
				CommonState: core.CommonState{nil, SubnetRsrcValidDeinitId},
				FreeSubnets: bitset.New(1).Set(0),
			},
			{
				CommonState: core.CommonState{nil, SubnetRsrcValidDeinitId},
				FreeSubnets: bitset.New(1).Set(0),
			},
		},
	},
	SubnetRsrcAllocateId: &subnetRsrcValidator{
		expCfg: []AutoSubnetCfgResource{
			{
				CommonState:    core.CommonState{nil, SubnetRsrcAllocateId},
				SubnetPool:     net.ParseIP("1.2.3.4"),
				SubnetPoolLen:  24,
				AllocSubnetLen: 24,
			},
		},
		expOper: []AutoSubnetOperResource{
			{
				CommonState: core.CommonState{nil, SubnetRsrcAllocateId},
				FreeSubnets: bitset.New(1).Set(0),
			},
			{
				CommonState: core.CommonState{nil, SubnetRsrcAllocateId},
				FreeSubnets: bitset.New(1).Set(0),
			},
			{
				CommonState: core.CommonState{nil, SubnetRsrcAllocateId},
				FreeSubnets: bitset.New(1).Clear(0),
			},
		},
	},
	SubnetRsrcAllocateExhaustId: &subnetRsrcValidator{
		expCfg: []AutoSubnetCfgResource{
			{
				CommonState:    core.CommonState{nil, SubnetRsrcAllocateExhaustId},
				SubnetPool:     net.ParseIP("1.2.3.4"),
				SubnetPoolLen:  24,
				AllocSubnetLen: 24,
			},
		},
		expOper: []AutoSubnetOperResource{
			{
				CommonState: core.CommonState{nil, SubnetRsrcAllocateExhaustId},
				FreeSubnets: bitset.New(1).Set(0),
			},
			{
				CommonState: core.CommonState{nil, SubnetRsrcAllocateExhaustId},
				FreeSubnets: bitset.New(1).Set(0),
			},
			{
				CommonState: core.CommonState{nil, SubnetRsrcAllocateExhaustId},
				FreeSubnets: bitset.New(1).Clear(0),
			},
			{
				CommonState: core.CommonState{nil, SubnetRsrcAllocateExhaustId},
				FreeSubnets: bitset.New(1).Clear(0),
			},
		},
	},
	SubnetRsrcDeallocateId: &subnetRsrcValidator{
		expCfg: []AutoSubnetCfgResource{
			{
				CommonState:    core.CommonState{nil, SubnetRsrcDeallocateId},
				SubnetPool:     net.ParseIP("1.2.3.4"),
				SubnetPoolLen:  24,
				AllocSubnetLen: 24,
			},
		},
		expOper: []AutoSubnetOperResource{
			{
				CommonState: core.CommonState{nil, SubnetRsrcDeallocateId},
				FreeSubnets: bitset.New(1).Set(0),
			},
			{
				CommonState: core.CommonState{nil, SubnetRsrcDeallocateId},
				FreeSubnets: bitset.New(1).Set(0),
			},
			{
				CommonState: core.CommonState{nil, SubnetRsrcDeallocateId},
				FreeSubnets: bitset.New(1).Clear(0),
			},
			{
				CommonState: core.CommonState{nil, SubnetRsrcDeallocateId},
				FreeSubnets: bitset.New(1).Set(0),
			},
			{
				CommonState: core.CommonState{nil, SubnetRsrcDeallocateId},
				FreeSubnets: bitset.New(1).Set(0),
			},
		},
	},
}

type testSubnetRsrcStateDriver struct {
}

func (d *testSubnetRsrcStateDriver) Init(config *core.Config) error {
	return core.Errorf("Shouldn't be called!")
}

func (d *testSubnetRsrcStateDriver) Deinit() {
}

func (d *testSubnetRsrcStateDriver) Write(key string, value []byte) error {
	return core.Errorf("Shouldn't be called!")
}

func (d *testSubnetRsrcStateDriver) Read(key string) ([]byte, error) {
	return nil, core.Errorf("Shouldn't be called!")
}

func (d *testSubnetRsrcStateDriver) ReadAll(baseKey string) ([][]byte, error) {
	return nil, core.Errorf("Shouldn't be called!")
}

func (d *testSubnetRsrcStateDriver) WatchAll(baseKey string, rsps chan [2][]byte) error {
	return core.Errorf("not supported")
}

func (d *testSubnetRsrcStateDriver) validate(key string, state core.State,
	op subnetRsrcValidateOp) error {
	strs := strings.Split(key, "/")
	id := strs[len(strs)-1]
	v, ok := subnetRsrcValidationStateMap[id]
	if !ok {
		errStr := fmt.Sprintf("No matching validation entry for id: %s", id)
		log.Printf("%s\n", errStr)
		return core.Errorf(errStr)
	}

	switch op {
	case SUBNET_RSRC_OP_WRITE:
		err := v.ValidateState(state)
		if err != nil {
			return err
		}
		return nil
	case SUBNET_RSRC_OP_READ:
		return v.CopyState(state)
	case SUBNET_RSRC_OP_CLEAR:
		fallthrough
	default:
		return nil
	}
}

func (d *testSubnetRsrcStateDriver) ClearState(key string) error {
	return d.validate(key, nil, SUBNET_RSRC_OP_CLEAR)
}

func (d *testSubnetRsrcStateDriver) ReadState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) error {
	return d.validate(key, value, SUBNET_RSRC_OP_READ)
}

func (d *testSubnetRsrcStateDriver) ReadAllState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) ([]core.State, error) {
	return nil, core.Errorf("Shouldn't be called!")
}

func (d *testSubnetRsrcStateDriver) WatchAllState(baseKey string, sType core.State,
	unmarshal func([]byte, interface{}) error, rsps chan core.WatchState) error {
	return core.Errorf("not supported")
}

func (d *testSubnetRsrcStateDriver) WriteState(key string, value core.State,
	marshal func(interface{}) ([]byte, error)) error {
	return d.validate(key, value, SUBNET_RSRC_OP_WRITE)
}

func TestAutoSubnetCfgResourceInit(t *testing.T) {
	rsrc := &AutoSubnetCfgResource{}
	rsrc.StateDriver = subnetRsrcStateDriver
	rsrc.Id = SubnetRsrcValidInitId
	err := rsrc.Init(&subnetRsrcValidationStateMap[rsrc.Id].expCfg[0])
	if err != nil {
		t.Fatalf("Subnet resource init failed. Error: %s", err)
	}
}

func TestAutoSubnetCfgResourceDeInit(t *testing.T) {
	rsrc := &AutoSubnetCfgResource{}
	rsrc.StateDriver = subnetRsrcStateDriver
	rsrc.Id = SubnetRsrcValidDeinitId
	err := rsrc.Init(&subnetRsrcValidationStateMap[rsrc.Id].expCfg[0])
	if err != nil {
		t.Fatalf("Subnet resource init failed. Error: %s", err)
	}

	rsrc.Deinit()
}

func TestAutoSubnetCfgResourceAllocate(t *testing.T) {
	rsrc := &AutoSubnetCfgResource{}
	rsrc.StateDriver = subnetRsrcStateDriver
	rsrc.Id = SubnetRsrcAllocateId
	err := rsrc.Init(&subnetRsrcValidationStateMap[rsrc.Id].expCfg[0])
	if err != nil {
		t.Fatalf("Subnet resource init failed. Error: %s", err)
	}

	p, err1 := rsrc.Allocate()
	if err1 != nil {
		t.Fatalf("Subnet resource allocation failed. Error: %s", err1)
	}
	pair := p.(SubnetIpLenPair)
	if !pair.Ip.Equal(rsrc.SubnetPool) || pair.Len != rsrc.AllocSubnetLen {
		t.Fatalf("Allocated subnet mismatch. expected: %s/%d, rcvd: %+v",
			rsrc.SubnetPool, rsrc.AllocSubnetLen, pair)
	}
}

func TestAutoSubnetCfgResourceAllocateExhaustion(t *testing.T) {
	rsrc := &AutoSubnetCfgResource{}
	rsrc.StateDriver = subnetRsrcStateDriver
	rsrc.Id = SubnetRsrcAllocateExhaustId
	err := rsrc.Init(&subnetRsrcValidationStateMap[rsrc.Id].expCfg[0])
	if err != nil {
		t.Fatalf("Subnet resource init failed. Error: %s", err)
	}

	_, err = rsrc.Allocate()
	if err != nil {
		t.Fatalf("Subnet resource allocation failed. Error: %s", err)
	}
	_, err = rsrc.Allocate()
	if err == nil {
		t.Fatalf("Subnet resource allocation succeeded, expected to fail!")
	}
	if !strings.Contains(err.Error(),
		"no subnets available.") {
		t.Fatalf("Subnet resource allocation failure reason mismatch. Expected: %s, rcvd: %s",
			"no subnets available.", err)
	}
}

func TestAutoSubnetCfgResourceDeAllocate(t *testing.T) {
	rsrc := &AutoSubnetCfgResource{}
	rsrc.StateDriver = subnetRsrcStateDriver
	rsrc.Id = SubnetRsrcDeallocateId
	err := rsrc.Init(&subnetRsrcValidationStateMap[rsrc.Id].expCfg[0])
	if err != nil {
		t.Fatalf("Subnet resource init failed. Error: %s", err)
	}

	pair, err1 := rsrc.Allocate()
	if err1 != nil {
		t.Fatalf("Subnet resource allocation failed. Error: %s", err1)
	}

	err = rsrc.Deallocate(pair)
	if err != nil {
		t.Fatalf("Subnet resource deallocation failed. Error: %s", err)
	}
}
