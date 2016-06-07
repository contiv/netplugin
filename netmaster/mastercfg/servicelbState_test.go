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

package mastercfg

import (
	"testing"

	"github.com/contiv/netplugin/core"
)

const (
	testSvcID       = "testServiceLB"
	serviceLBCfgKey = serviceLBConfigPathPrefix + testSvcID
)

type testServiceLBStateDriver struct{}

var serviceLBStateDriver = &testServiceLBStateDriver{}

func (d *testServiceLBStateDriver) Init(instInfo *core.InstanceInfo) error {
	return core.Errorf("Shouldn't be called!")
}

func (d *testServiceLBStateDriver) Deinit() {
}

func (d *testServiceLBStateDriver) Write(key string, value []byte) error {
	return core.Errorf("Shouldn't be called!")
}

func (d *testServiceLBStateDriver) Read(key string) ([]byte, error) {
	return []byte{}, core.Errorf("Shouldn't be called!")
}

func (d *testServiceLBStateDriver) ReadAll(baseKey string) ([][]byte, error) {
	return [][]byte{}, core.Errorf("Shouldn't be called!")
}

func (d *testServiceLBStateDriver) WatchAll(baseKey string, rsps chan [2][]byte) error {
	return core.Errorf("not supported")
}

func (d *testServiceLBStateDriver) validateKey(key string) error {
	if key != serviceLBCfgKey {
		return core.Errorf("Unexpected key. recvd: %s expected: %s ",
			key, serviceLBCfgKey)
	}

	return nil
}

func (d *testServiceLBStateDriver) ClearState(key string) error {
	return d.validateKey(key)
}

func (d *testServiceLBStateDriver) ReadState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) error {
	return d.validateKey(key)
}

func (d *testServiceLBStateDriver) ReadAllState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) ([]core.State, error) {
	return nil, core.Errorf("Shouldn't be called!")
}

func (d *testServiceLBStateDriver) WatchAllState(baseKey string, sType core.State,
	unmarshal func([]byte, interface{}) error, rsps chan core.WatchState) error {
	return core.Errorf("not supported")
}

func (d *testServiceLBStateDriver) WriteState(key string, value core.State,
	marshal func(interface{}) ([]byte, error)) error {
	return d.validateKey(key)
}

func TestCfgServiceLBStateRead(t *testing.T) {
	serviceLBCfg := &CfgServiceLBState{}
	serviceLBCfg.StateDriver = serviceLBStateDriver

	err := serviceLBCfg.Read(testSvcID)
	if err != nil {
		t.Fatalf("read config state failed. Error: %s", err)
	}
}

func TestServiceLBStateWrite(t *testing.T) {
	serviceLBCfg := &CfgServiceLBState{}
	serviceLBCfg.StateDriver = serviceLBStateDriver
	serviceLBCfg.ID = testSvcID

	err := serviceLBCfg.Write()
	if err != nil {
		t.Fatalf("write config state failed. Error: %s", err)
	}
}

func TestServiceLBStateClear(t *testing.T) {
	serviceLBCfg := &CfgServiceLBState{}
	serviceLBCfg.StateDriver = serviceLBStateDriver
	serviceLBCfg.ID = testSvcID

	err := serviceLBCfg.Clear()
	if err != nil {
		t.Fatalf("clear config state failed. Error: %s", err)
	}
}
