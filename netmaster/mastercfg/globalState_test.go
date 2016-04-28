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
	gCfgKey = globalConfigPath
)

type testglobalStateDriver struct{}

var gcStateDriver = &testglobalStateDriver{}

func (d *testglobalStateDriver) Init(instInfo *core.InstanceInfo) error {
	return core.Errorf("Shouldn't be called!")
}

func (d *testglobalStateDriver) Deinit() {
}

func (d *testglobalStateDriver) Write(key string, value []byte) error {
	return core.Errorf("Shouldn't be called!")
}

func (d *testglobalStateDriver) Read(key string) ([]byte, error) {
	return []byte{}, core.Errorf("Shouldn't be called!")
}

func (d *testglobalStateDriver) ReadAll(baseKey string) ([][]byte, error) {
	return [][]byte{}, core.Errorf("Shouldn't be called!")
}

func (d *testglobalStateDriver) WatchAll(baseKey string, rsps chan [2][]byte) error {
	return core.Errorf("not supported")
}

func (d *testglobalStateDriver) validateKey(key string) error {
	if key != gCfgKey {
		return core.Errorf("Unexpected key. recvd: %s expected: %s", key, gCfgKey)
	}

	return nil
}

func (d *testglobalStateDriver) ClearState(key string) error {
	return d.validateKey(key)
}

func (d *testglobalStateDriver) ReadState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) error {
	return d.validateKey(key)
}

func (d *testglobalStateDriver) ReadAllState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) ([]core.State, error) {
	return nil, core.Errorf("Shouldn't be called!")
}

func (d *testglobalStateDriver) WatchAllState(baseKey string, sType core.State,
	unmarshal func([]byte, interface{}) error, rsps chan core.WatchState) error {
	return core.Errorf("not supported")
}

func (d *testglobalStateDriver) WriteState(key string, value core.State,
	marshal func(interface{}) ([]byte, error)) error {
	return d.validateKey(key)
}

func TestGlobConfigRead(t *testing.T) {
	gcCfg := &GlobConfig{}
	gcCfg.StateDriver = gcStateDriver

	err := gcCfg.Read(gCfgKey)
	if err != nil {
		t.Fatalf("read config state failed. Error: %s", err)
	}
}

func TestGlobConfigWrite(t *testing.T) {
	gcCfg := &GlobConfig{}
	gcCfg.StateDriver = gcStateDriver
	gcCfg.ID = gCfgKey

	err := gcCfg.Write()
	if err != nil {
		t.Fatalf("write config state failed. Error: %s", err)
	}
}

func TestGlobConfigClear(t *testing.T) {
	gcCfg := &GlobConfig{}
	gcCfg.StateDriver = gcStateDriver
	gcCfg.ID = gCfgKey

	err := gcCfg.Clear()
	if err != nil {
		t.Fatalf("clear config state failed. Error: %s", err)
	}
}
