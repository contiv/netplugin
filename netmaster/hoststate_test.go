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

package netmaster

import (
	"errors"
	"fmt"
	"testing"

	"github.com/contiv/netplugin/core"
)

const (
	testHostId = "testHost"
	hostCfgKey = HOST_CFG_PATH_PREFIX + testHostId
)

var hostState *testHostStateDriver = &testHostStateDriver{}

type testHostStateDriver struct {
}

func (d *testHostStateDriver) Init(config *core.Config) error {
	return errors.New("Shouldn't be called!")
}

func (d *testHostStateDriver) Deinit() {
}

func (d *testHostStateDriver) Write(key string, value []byte) error {
	return errors.New("Shouldn't be called!")
}

func (d *testHostStateDriver) Read(key string) ([]byte, error) {
	return []byte{}, errors.New("Shouldn't be called!")
}

func (d *testHostStateDriver) ReadAll(baseKey string) ([][]byte, error) {
	return [][]byte{}, &core.Error{Desc: "Shouldn't be called!"}
}

func (d *testHostStateDriver) validateKey(key string) error {
	if key != hostCfgKey {
		return errors.New(fmt.Sprintf("Unexpected key. recvd: %s "+
			"expected: %s", key, hostCfgKey))
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

func TestMasterHostConfigRead(t *testing.T) {
	hostCfg := &MasterHostConfig{StateDriver: hostState}

	err := hostCfg.Read(testHostId)
	if err != nil {
		t.Fatalf("read config state failed. Error: %s", err)
	}
}

func TestMasterHostConfigWrite(t *testing.T) {
	hostCfg := &MasterHostConfig{StateDriver: hostState, Name: testHostId}

	err := hostCfg.Write()
	if err != nil {
		t.Fatalf("write config state failed. Error: %s", err)
	}
}

func TestMasterHostConfigClear(t *testing.T) {
	hostCfg := &MasterHostConfig{StateDriver: hostState, Name: testHostId}

	err := hostCfg.Clear()
	if err != nil {
		t.Fatalf("clear config state failed. Error: %s", err)
	}
}
