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

package drivers

import (
	"fmt"
	"testing"

	"github.com/contiv/netplugin/core"
)

const (
	testEpId  = "testEp"
	epCfgKey  = EP_CFG_PATH_PREFIX + testEpId
	epOperKey = EP_OPER_PATH_PREFIX + testEpId
)

var epStateDriver *testEpStateDriver = &testEpStateDriver{}

type testEpStateDriver struct {
}

func (d *testEpStateDriver) Init(config *core.Config) error {
	return &core.Error{Desc: "Shouldn't be called!"}
}

func (d *testEpStateDriver) Deinit() {
}

func (d *testEpStateDriver) Write(key string, value []byte) error {
	return &core.Error{Desc: "Shouldn't be called!"}
}

func (d *testEpStateDriver) Read(key string) ([]byte, error) {
	return []byte{}, &core.Error{Desc: "Shouldn't be called!"}
}

func (d *testEpStateDriver) ReadAll(baseKey string) ([][]byte, error) {
	return [][]byte{}, &core.Error{Desc: "Shouldn't be called!"}
}

func (d *testEpStateDriver) validateKey(key string) error {
	if key != epCfgKey && key != epOperKey {
		return &core.Error{Desc: fmt.Sprintf("Unexpected key. recvd: %s expected: %s or %s ",
			key, epCfgKey, epOperKey)}
	} else {
		return nil
	}
}

func (d *testEpStateDriver) ClearState(key string) error {
	return d.validateKey(key)
}

func (d *testEpStateDriver) ReadState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) error {
	return d.validateKey(key)
}

func (d *testEpStateDriver) WriteState(key string, value core.State,
	marshal func(interface{}) ([]byte, error)) error {
	return d.validateKey(key)
}

func TestOvsCfgEndpointStateRead(t *testing.T) {
	epCfg := &OvsCfgEndpointState{StateDriver: epStateDriver}

	err := epCfg.Read(testEpId)
	if err != nil {
		t.Fatalf("read config state failed. Error: %s", err)
	}
}

func TestOvsCfgEndpointStateWrite(t *testing.T) {
	epCfg := &OvsCfgEndpointState{StateDriver: epStateDriver, Id: testEpId}

	err := epCfg.Write()
	if err != nil {
		t.Fatalf("write config state failed. Error: %s", err)
	}
}

func TestOvsCfgEndpointStateClear(t *testing.T) {
	epCfg := &OvsCfgEndpointState{StateDriver: epStateDriver, Id: testEpId}

	err := epCfg.Clear()
	if err != nil {
		t.Fatalf("clear config state failed. Error: %s", err)
	}
}

func TestOvsOperEndpointStateRead(t *testing.T) {
	epOper := &OvsOperEndpointState{StateDriver: epStateDriver}

	err := epOper.Read(testEpId)
	if err != nil {
		t.Fatalf("read oper state failed. Error: %s", err)
	}
}

func TestOvsOperEndpointStateWrite(t *testing.T) {
	epOper := &OvsOperEndpointState{StateDriver: epStateDriver, Id: testEpId}

	err := epOper.Write()
	if err != nil {
		t.Fatalf("write oper state failed. Error: %s", err)
	}
}

func TestOvsOperEndpointStateClear(t *testing.T) {
	epOper := &OvsOperEndpointState{StateDriver: epStateDriver, Id: testEpId}

	err := epOper.Clear()
	if err != nil {
		t.Fatalf("clear oper state failed. Error: %s", err)
	}
}
