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
	testhostID = "contivhost"
	bgpCfgKey  = bgpConfigPathPrefix + testhostID
)

type testBgpStateDriver struct{}

var bgpStateDriver = &testBgpStateDriver{}

func (d *testBgpStateDriver) Init(instInfo *core.InstanceInfo) error {
	return core.Errorf("Shouldn't be called!")
}

func (d *testBgpStateDriver) Deinit() {
}

func (d *testBgpStateDriver) Write(key string, value []byte) error {
	return core.Errorf("Shouldn't be called!")
}

func (d *testBgpStateDriver) Read(key string) ([]byte, error) {
	return []byte{}, core.Errorf("Shouldn't be called!")
}

func (d *testBgpStateDriver) ReadAll(baseKey string) ([][]byte, error) {
	return [][]byte{}, core.Errorf("Shouldn't be called!")
}

func (d *testBgpStateDriver) WatchAll(baseKey string, rsps chan [2][]byte) error {
	return core.Errorf("not supported")
}

func (d *testBgpStateDriver) validateKey(key string) error {
	if key != bgpCfgKey {
		return core.Errorf("Unexpected key. recvd: %s expected: %s ",
			key, bgpCfgKey)
	}

	return nil
}

func (d *testBgpStateDriver) ClearState(key string) error {
	return d.validateKey(key)
}

func (d *testBgpStateDriver) ReadState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) error {
	return d.validateKey(key)
}

func (d *testBgpStateDriver) ReadAllState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) ([]core.State, error) {
	return nil, core.Errorf("Shouldn't be called!")
}

func (d *testBgpStateDriver) WatchAllState(baseKey string, sType core.State,
	unmarshal func([]byte, interface{}) error, rsps chan core.WatchState) error {
	return core.Errorf("not supported")
}

func (d *testBgpStateDriver) WriteState(key string, value core.State,
	marshal func(interface{}) ([]byte, error)) error {
	return d.validateKey(key)
}

func TestCfgBgpStateRead(t *testing.T) {
	bgpCfg := &CfgBgpState{}
	bgpCfg.StateDriver = bgpStateDriver

	err := bgpCfg.Read(testhostID)
	if err != nil {
		t.Fatalf("read config state failed. Error: %s", err)
	}
}

func TestCfgBgpStateWrite(t *testing.T) {

	bgpCfg := &CfgBgpState{}
	bgpCfg.StateDriver = bgpStateDriver
	bgpCfg.Hostname = testhostID

	err := bgpCfg.Write()
	if err != nil {
		t.Fatalf("write config state failed. Error: %s", err)
	}
}

func TestCfgBgpStateClear(t *testing.T) {
	bgpCfg := &CfgBgpState{}
	bgpCfg.StateDriver = bgpStateDriver
	bgpCfg.Hostname = testhostID

	err := bgpCfg.Clear()
	if err != nil {
		t.Fatalf("clear config state failed. Error: %s", err)
	}
}
