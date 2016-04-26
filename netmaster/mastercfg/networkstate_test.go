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
	testNwID = "testNw"
	nwCfgKey = networkConfigPathPrefix + testNwID
)

type testNwStateDriver struct{}

var nwStateDriver = &testNwStateDriver{}

func (d *testNwStateDriver) Init(instInfo *core.InstanceInfo) error {
	return core.Errorf("Shouldn't be called!")
}

func (d *testNwStateDriver) Deinit() {
}

func (d *testNwStateDriver) Write(key string, value []byte) error {
	return core.Errorf("Shouldn't be called!")
}

func (d *testNwStateDriver) Read(key string) ([]byte, error) {
	return []byte{}, core.Errorf("Shouldn't be called!")
}

func (d *testNwStateDriver) ReadAll(baseKey string) ([][]byte, error) {
	return [][]byte{}, core.Errorf("Shouldn't be called!")
}

func (d *testNwStateDriver) WatchAll(baseKey string, rsps chan [2][]byte) error {
	return core.Errorf("not supported")
}

func (d *testNwStateDriver) validateKey(key string) error {
	if key != nwCfgKey {
		return core.Errorf("Unexpected key. recvd: %s expected: %s ",
			key, nwCfgKey)
	}

	return nil
}

func (d *testNwStateDriver) ClearState(key string) error {
	return d.validateKey(key)
}

func (d *testNwStateDriver) ReadState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) error {
	return d.validateKey(key)
}

func (d *testNwStateDriver) ReadAllState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) ([]core.State, error) {
	return nil, core.Errorf("Shouldn't be called!")
}

func (d *testNwStateDriver) WatchAllState(baseKey string, sType core.State,
	unmarshal func([]byte, interface{}) error, rsps chan core.WatchState) error {
	return core.Errorf("not supported")
}

func (d *testNwStateDriver) WriteState(key string, value core.State,
	marshal func(interface{}) ([]byte, error)) error {
	return d.validateKey(key)
}

func TestCfgNetworkStateRead(t *testing.T) {
	nwCfg := &CfgNetworkState{}
	nwCfg.StateDriver = nwStateDriver

	err := nwCfg.Read(testNwID)
	if err != nil {
		t.Fatalf("read config state failed. Error: %s", err)
	}
}

func TestCfgNetworkStateWrite(t *testing.T) {
	nwCfg := &CfgNetworkState{}
	nwCfg.StateDriver = nwStateDriver
	nwCfg.ID = testNwID

	err := nwCfg.Write()
	if err != nil {
		t.Fatalf("write config state failed. Error: %s", err)
	}
}

func TestCfgNetworkStateClear(t *testing.T) {
	nwCfg := &CfgNetworkState{}
	nwCfg.StateDriver = nwStateDriver
	nwCfg.ID = testNwID

	err := nwCfg.Clear()
	if err != nil {
		t.Fatalf("clear config state failed. Error: %s", err)
	}
}
