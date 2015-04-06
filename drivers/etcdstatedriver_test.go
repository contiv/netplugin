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
	"bytes"
	"encoding/json"
	"testing"

	"github.com/contiv/netplugin/core"
)

func setupDriver(t *testing.T) *EtcdStateDriver {
	etcdConfig := &EtcdStateDriverConfig{}
	etcdConfig.Etcd.Machines = []string{"http://127.0.0.1:4001"}
	config := &core.Config{V: etcdConfig}

	driver := &EtcdStateDriver{}

	err := driver.Init(config)
	if err != nil {
		t.Fatalf("driver init failed. Error: %s", err)
		return nil
	}

	return driver
}

func TestEtcdStateDriverInit(t *testing.T) {
	setupDriver(t)
}

func TestEtcdStateDriverInitInvalidConfig(t *testing.T) {
	config := &core.Config{}

	driver := EtcdStateDriver{}

	err := driver.Init(config)
	if err == nil {
		t.Fatalf("driver init succeeded, should have failed.")
	}

	err = driver.Init(nil)
	if err == nil {
		t.Fatalf("driver init succeeded, should have failed.")
	}
}

func TestEtcdStateDriverWrite(t *testing.T) {
	driver := setupDriver(t)
	testBytes := []byte{0xb, 0xa, 0xd, 0xb, 0xa, 0xb, 0xe}
	key := "TestKeyRawWrite"

	err := driver.Write(key, testBytes)
	if err != nil {
		t.Fatalf("failed to write bytes. Error: %s", err)
	}
}

func TestEtcdStateDriverRead(t *testing.T) {
	driver := setupDriver(t)
	testBytes := []byte{0xb, 0xa, 0xd, 0xb, 0xa, 0xb, 0xe}
	key := "TestKeyRawRead"

	err := driver.Write(key, testBytes)
	if err != nil {
		t.Fatalf("failed to write bytes. Error: %s", err)
	}

	readBytes, err := driver.Read(key)
	if err != nil {
		t.Fatalf("failed to read bytes. Error: %s", err)
	}

	if !bytes.Equal(testBytes, readBytes) {
		t.Fatalf("read bytes don't match written bytes. Wrote: %v Read: %v",
			testBytes, readBytes)
	}
}

type testState struct {
	IgnoredField core.StateDriver `json:"-"`
	IntField     int              `json:"intField"`
	StrField     string           `json:"strField"`
}

func (s *testState) Write() error {
	return &core.Error{Desc: "Should not be called!!"}
}

func (s *testState) Read(id string) error {
	return &core.Error{Desc: "Should not be called!!"}
}

func (s *testState) ReadAll() ([]core.State, error) {
	return nil, &core.Error{Desc: "Should not be called!!"}
}

func (s *testState) Clear() error {
	return &core.Error{Desc: "Should not be called!!"}
}

func TestEtcdStateDriverWriteState(t *testing.T) {
	driver := setupDriver(t)
	state := &testState{IgnoredField: driver, IntField: 1234,
		StrField: "testString"}
	key := "testKey"

	err := driver.WriteState(key, state, json.Marshal)
	if err != nil {
		t.Fatalf("failed to write state. Error: %s", err)
	}
}

func TestEtcdStateDriverWriteStateForUpdate(t *testing.T) {
	driver := setupDriver(t)
	state := &testState{IgnoredField: driver, IntField: 1234,
		StrField: "testString"}
	key := "testKeyForUpdate"

	err := driver.WriteState(key, state, json.Marshal)
	if err != nil {
		t.Fatalf("failed to write state. Error: %s", err)
	}

	state.StrField = "testString-update"
	err = driver.WriteState(key, state, json.Marshal)
	if err != nil {
		t.Fatalf("failed to update state. Error: %s", err)
	}
}

func TestEtcdStateDriverClearState(t *testing.T) {
	driver := setupDriver(t)
	state := &testState{IntField: 1234, StrField: "testString"}
	key := "testKeyClear"

	err := driver.WriteState(key, state, json.Marshal)
	if err != nil {
		t.Fatalf("failed to write state. Error: %s", err)
	}

	err = driver.ClearState(key)
	if err != nil {
		t.Fatalf("failed to clear state. Error: %s", err)
	}
}

func TestEtcdStateDriverReadState(t *testing.T) {
	driver := setupDriver(t)
	state := &testState{IgnoredField: driver, IntField: 1234,
		StrField: "testString"}
	key := "/contiv/dir1/testKeyRead"

	err := driver.WriteState(key, state, json.Marshal)
	if err != nil {
		t.Fatalf("failed to write state. Error: %s", err)
	}

	readState := &testState{}
	err = driver.ReadState(key, readState, json.Unmarshal)
	if err != nil {
		t.Fatalf("failed to read state. Error: %s", err)
	}

	if readState.IntField != state.IntField || readState.StrField != state.StrField {
		t.Fatalf("Read state didn't match state written. Wrote: %v Read: %v",
			state, readState)
	}
}

func TestEtcdStateDriverReadStateAfterUpdate(t *testing.T) {
	driver := setupDriver(t)
	state := &testState{IntField: 1234, StrField: "testString"}
	key := "testKeyReadUpdate"

	err := driver.WriteState(key, state, json.Marshal)
	if err != nil {
		t.Fatalf("failed to write state. Error: %s", err)
	}

	state.StrField = "testStringUpdated"
	err = driver.WriteState(key, state, json.Marshal)
	if err != nil {
		t.Fatalf("failed to update state. Error: %s", err)
	}

	readState := &testState{}
	err = driver.ReadState(key, readState, json.Unmarshal)
	if err != nil {
		t.Fatalf("failed to read state. Error: %s", err)
	}

	if readState.IntField != state.IntField || readState.StrField != state.StrField {
		t.Fatalf("Read state didn't match state written. Wrote: %v Read: %v",
			state, readState)
	}
}

func TestEtcdStateDriverReadStateAfterClear(t *testing.T) {
	driver := setupDriver(t)
	state := &testState{IntField: 1234, StrField: "testString"}
	key := "testKeyReadClear"

	err := driver.WriteState(key, state, json.Marshal)
	if err != nil {
		t.Fatalf("failed to write state. Error: %s", err)
	}

	err = driver.ClearState(key)
	if err != nil {
		t.Fatalf("failed to clear state. Error: %s", err)
	}

	readState := &testState{}
	err = driver.ReadState(key, readState, json.Unmarshal)
	if err == nil {
		t.Fatalf("Able to read cleared state!. Key: %s, Value: %v",
			key, readState)
	}
}
