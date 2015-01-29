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
	testHostId = "testHost"
	hostCfgKey = HOST_CFG_PATH_PREFIX + testHostId
)

var hostStateDriver *testHostStateDriver = &testHostStateDriver{}

type testHostStateDriver struct {
}

func (d *testHostStateDriver) Init(config *core.Config) error {
	return &core.Error{Desc: "Shouldn't be called!"}
}

func (d *testHostStateDriver) Deinit() {
}

func (d *testHostStateDriver) Write(key string, value []byte) error {
	return &core.Error{Desc: "Shouldn't be called!"}
}

func (d *testHostStateDriver) Read(key string) ([]byte, error) {
	return []byte{}, &core.Error{Desc: "Shouldn't be called!"}
}

func (d *testHostStateDriver) ReadRecursive(baseKey string) ([]string, error) {
	return []string{}, &core.Error{Desc: "Shouldn't be called!"}
}

func (d *testHostStateDriver) validateKey(key string) error {
	if key != hostCfgKey {
		return &core.Error{Desc: fmt.Sprintf("Unexpected key. recvd: %s expected: %s",
			key, hostCfgKey)}
	} else {
		return nil
	}
}

func (d *testHostStateDriver) ClearState(key string) error {
	return d.validateKey(key)
}

func (d *testHostStateDriver) ReadState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) error {
	return d.validateKey(key)
}

func (d *testHostStateDriver) WriteState(key string, value core.State,
	marshal func(interface{}) ([]byte, error)) error {
	return d.validateKey(key)
}

func TestOvsCfgHostStateRead(t *testing.T) {
	hostCfg := &OvsCfgHostState{StateDriver: hostStateDriver}

	err := hostCfg.Read(testHostId)
	if err != nil {
		t.Fatalf("read config state failed. Error: %s", err)
	}
}

func TestOvsCfgHostStateWrite(t *testing.T) {
	hostCfg := &OvsCfgHostState{StateDriver: hostStateDriver, Id: testHostId}

	err := hostCfg.Write()
	if err != nil {
		t.Fatalf("write config state failed. Error: %s", err)
	}
}

func TestOvsCfgHostStateClear(t *testing.T) {
	hostCfg := &OvsCfgHostState{StateDriver: hostStateDriver, Id: testHostId}

	err := hostCfg.Clear()
	if err != nil {
		t.Fatalf("clear config state failed. Error: %s", err)
	}
}
