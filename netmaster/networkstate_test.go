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

// netmaster  - implements the network intent translation to plugin
// events; uses state distribution to achieve intent realization
// netmaster runs as a logically centralized unit on in the cluster

package netmaster

import (
	"fmt"
	"testing"

	"github.com/contiv/netplugin/core"
)

const (
	testNwId = "testNw"
	nwCfgKey = NW_CFG_PATH_PREFIX + testNwId
)

var nwStateDriver *testNwStateDriver = &testNwStateDriver{}

type testNwStateDriver struct {
}

func (d *testNwStateDriver) Init(config *core.Config) error {
	return &core.Error{Desc: "Shouldn't be called!"}
}

func (d *testNwStateDriver) Deinit() {
}

func (d *testNwStateDriver) Write(key string, value []byte) error {
	return &core.Error{Desc: "Shouldn't be called!"}
}

func (d *testNwStateDriver) Read(key string) ([]byte, error) {
	return []byte{}, &core.Error{Desc: "Shouldn't be called!"}
}

func (d *testNwStateDriver) ReadRecursive(baseKey string) ([]string, error) {
	return []string{}, &core.Error{Desc: "Shouldn't be called!"}
}

func (d *testNwStateDriver) validateKey(key string) error {
	if key != nwCfgKey {
		return &core.Error{Desc: fmt.Sprintf("Unexpected key. recvd: %s "+
			"expected: %s or %s ", key, nwCfgKey)}
	} else {
		return nil
	}
}

func (d *testNwStateDriver) ClearState(key string) error {
	return d.validateKey(key)
}

func (d *testNwStateDriver) ReadState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) error {
	return d.validateKey(key)
}

func (d *testNwStateDriver) WriteState(key string, value core.State,
	marshal func(interface{}) ([]byte, error)) error {
	return d.validateKey(key)
}

func TestMasterNwConfigRead(t *testing.T) {
	nwCfg := &MasterNwConfig{StateDriver: nwStateDriver}

	err := nwCfg.Read(testNwId)
	if err != nil {
		t.Fatalf("read config state failed. Error: %s", err)
	}
}

func TestMasterNwConfigWrite(t *testing.T) {
	nwCfg := &MasterNwConfig{StateDriver: nwStateDriver, Id: testNwId}

	err := nwCfg.Write()
	if err != nil {
		t.Fatalf("write config state failed. Error: %s", err)
	}
}

func TestMasterNwConfigClear(t *testing.T) {
	nwCfg := &MasterNwConfig{StateDriver: nwStateDriver, Id: testNwId}

	err := nwCfg.Clear()
	if err != nil {
		t.Fatalf("clear config state failed. Error: %s", err)
	}
}
