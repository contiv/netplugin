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

package state

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/contiv/netplugin/core"
)

const (
	waitTimeout = 5 * time.Second
)

func setupEtcdDriver(t *testing.T) *EtcdStateDriver {
	instInfo := core.InstanceInfo{DbURL: "etcd://127.0.0.1:2379"}

	driver := &EtcdStateDriver{}

	err := driver.Init(&instInfo)
	if err != nil {
		t.Fatalf("driver init failed. Error: %s", err)
		return nil
	}

	return driver
}

func TestEtcdStateDriverInit(t *testing.T) {
	setupEtcdDriver(t)
}

func commonTestStateDriverInitInvalidConfig(t *testing.T, d core.StateDriver) {
	instInfo := core.InstanceInfo{DbURL: "xyz://127.0.0.1:2379"}
	err := d.Init(&instInfo)
	if err == nil {
		t.Fatalf("d init succeeded, should have failed.")
	}

	err = d.Init(nil)
	if err == nil {
		t.Fatalf("d init succeeded, should have failed.")
	}
}

func TestEtcdStateDriverInitInvalidConfig(t *testing.T) {
	driver := &EtcdStateDriver{}
	commonTestStateDriverInitInvalidConfig(t, driver)

}

func commonTestStateDriverWrite(t *testing.T, d core.StateDriver) {
	testBytes := []byte{0xb, 0xa, 0xd, 0xb, 0xa, 0xb, 0xe}
	key := "TestKeyRawWrite"

	err := d.Write(key, testBytes)
	if err != nil {
		t.Fatalf("failed to write bytes. Error: %s", err)
	}
}

func TestEtcdStateDriverWrite(t *testing.T) {
	driver := setupEtcdDriver(t)
	commonTestStateDriverWrite(t, driver)
}

func commonTestStateDriverRead(t *testing.T, d core.StateDriver) {
	testBytes := []byte{0xb, 0xa, 0xd, 0xb, 0xa, 0xb, 0xe}
	key := "TestKeyRawRead"

	err := d.Write(key, testBytes)
	if err != nil {
		t.Fatalf("failed to write bytes. Error: %s", err)
	}

	readBytes, err := d.Read(key)
	if err != nil {
		t.Fatalf("failed to read bytes. Error: %s", err)
	}

	if !bytes.Equal(testBytes, readBytes) {
		t.Fatalf("read bytes don't match written bytes. Wrote: %v Read: %v",
			testBytes, readBytes)
	}
}

func TestEtcdStateDriverRead(t *testing.T) {
	driver := setupEtcdDriver(t)
	commonTestStateDriverRead(t, driver)
}

type testState struct {
	core.CommonState
	IgnoredField core.StateDriver `json:"-"`
	IntField     int              `json:"intField"`
	StrField     string           `json:"strField"`
}

func (s *testState) Write() error {
	return core.Errorf("Should not be called!!")
}

func (s *testState) Read(id string) error {
	return core.Errorf("Should not be called!!")
}

func (s *testState) ReadAll() ([]core.State, error) {
	return nil, core.Errorf("Should not be called!!")
}

func (s *testState) Clear() error {
	return core.Errorf("Should not be called!!")
}

func commonTestStateDriverWriteState(t *testing.T, d core.StateDriver) {
	state := &testState{IgnoredField: d, IntField: 1234,
		StrField: "testString"}
	key := "testKey"

	err := d.WriteState(key, state, json.Marshal)
	if err != nil {
		t.Fatalf("failed to write state. Error: %s", err)
	}
}

func TestEtcdStateDriverWriteState(t *testing.T) {
	driver := setupEtcdDriver(t)
	commonTestStateDriverWriteState(t, driver)
}

func commonTestStateDriverWriteStateForUpdate(t *testing.T, d core.StateDriver) {
	state := &testState{IgnoredField: d, IntField: 1234,
		StrField: "testString"}
	key := "testKeyForUpdate"

	err := d.WriteState(key, state, json.Marshal)
	if err != nil {
		t.Fatalf("failed to write state. Error: %s", err)
	}

	state.StrField = "testString-update"
	err = d.WriteState(key, state, json.Marshal)
	if err != nil {
		t.Fatalf("failed to update state. Error: %s", err)
	}
}

func TestEtcdStateDriverWriteStateForUpdate(t *testing.T) {
	driver := setupEtcdDriver(t)
	commonTestStateDriverWriteStateForUpdate(t, driver)
}

func commonTestStateDriverClearState(t *testing.T, d core.StateDriver) {
	state := &testState{IntField: 1234, StrField: "testString"}
	key := "testKeyClear"

	err := d.WriteState(key, state, json.Marshal)
	if err != nil {
		t.Fatalf("failed to write state. Error: %s", err)
	}

	err = d.ClearState(key)
	if err != nil {
		t.Fatalf("failed to clear state. Error: %s", err)
	}
}

func TestEtcdStateDriverClearState(t *testing.T) {
	driver := setupEtcdDriver(t)
	commonTestStateDriverClearState(t, driver)
}

func commonTestStateDriverReadState(t *testing.T, d core.StateDriver) {
	state := &testState{IgnoredField: d, IntField: 1234,
		StrField: "testString"}
	key := "contiv/dir1/testKeyRead"

	err := d.WriteState(key, state, json.Marshal)
	if err != nil {
		t.Fatalf("failed to write state. Error: %s", err)
	}

	readState := &testState{}
	err = d.ReadState(key, readState, json.Unmarshal)
	if err != nil {
		t.Fatalf("failed to read state. Error: %s", err)
	}

	if readState.IntField != state.IntField || readState.StrField != state.StrField {
		t.Fatalf("Read state didn't match state written. Wrote: %v Read: %v",
			state, readState)
	}
}

func TestEtcdStateDriverReadState(t *testing.T) {
	driver := setupEtcdDriver(t)
	commonTestStateDriverReadState(t, driver)
}

func commonTestStateDriverReadStateAfterUpdate(t *testing.T, d core.StateDriver) {
	state := &testState{IntField: 1234, StrField: "testString"}
	key := "testKeyReadUpdate"

	err := d.WriteState(key, state, json.Marshal)
	if err != nil {
		t.Fatalf("failed to write state. Error: %s", err)
	}

	state.StrField = "testStringUpdated"
	err = d.WriteState(key, state, json.Marshal)
	if err != nil {
		t.Fatalf("failed to update state. Error: %s", err)
	}

	readState := &testState{}
	err = d.ReadState(key, readState, json.Unmarshal)
	if err != nil {
		t.Fatalf("failed to read state. Error: %s", err)
	}

	if readState.IntField != state.IntField || readState.StrField != state.StrField {
		t.Fatalf("Read state didn't match state written. Wrote: %v Read: %v",
			state, readState)
	}
}

func TestEtcdStateDriverReadStateAfterUpdate(t *testing.T) {
	driver := setupEtcdDriver(t)
	commonTestStateDriverReadStateAfterUpdate(t, driver)
}

func commonTestStateDriverReadStateAfterClear(t *testing.T, d core.StateDriver) {
	state := &testState{IntField: 1234, StrField: "testString"}
	key := "testKeyReadClear"

	err := d.WriteState(key, state, json.Marshal)
	if err != nil {
		t.Fatalf("failed to write state. Error: %s", err)
	}

	err = d.ClearState(key)
	if err != nil {
		t.Fatalf("failed to clear state. Error: %s", err)
	}

	readState := &testState{}
	err = d.ReadState(key, readState, json.Unmarshal)
	if err == nil {
		t.Fatalf("Able to read cleared state!. Key: %s, Value: %v",
			key, readState)
	}
}

func TestEtcdStateDriverReadStateAfterClear(t *testing.T) {
	driver := setupEtcdDriver(t)
	commonTestStateDriverReadStateAfterClear(t, driver)
}

func commonTestStateDriverWatchAllStateCreate(t *testing.T, d core.StateDriver) {
	state := &testState{IntField: 1234, StrField: "testString"}
	baseKey := "create"
	key := baseKey + "/testKeyWatchAll"

	recvErr := make(chan error, 1)
	stateCh := make(chan core.WatchState, 1)
	timer := time.After(waitTimeout)

	go func(rsps chan core.WatchState, retErr chan error) {
		err := d.WatchAllState(baseKey, state, json.Unmarshal, stateCh)
		if err != nil {
			retErr <- err
			return
		}
	}(stateCh, recvErr)

	// trigger create after a slight pause to ensure that events are not missed
	time.Sleep(time.Second)
	err := d.WriteState(key, state, json.Marshal)
	if err != nil {
		t.Fatalf("failed to write state. Error: %s", err)
	}
	defer func() {
		d.ClearState(key)
	}()

for_label:
	for {
		select {
		case watchState := <-stateCh:
			s := watchState.Curr.(*testState)
			if s.IntField != state.IntField || s.StrField != state.StrField {
				t.Fatalf("Watch state mismatch. Expctd: %+v, Rcvd: %+v", state, s)
			}
			if watchState.Prev != nil {
				t.Fatalf("Watch state as prev state set %+v, expected to be nil", watchState.Prev)
			}
			break for_label
		case err := <-recvErr:
			t.Fatalf("Watch failed. Error: %s", err)
			break for_label
		case <-timer:
			t.Fatalf("timed out waiting for events")
			break for_label
		}
	}
}

func TestEtcdStateDriverWatchAllStateCreate(t *testing.T) {
	driver := setupEtcdDriver(t)
	commonTestStateDriverWatchAllStateCreate(t, driver)
}

func commonTestStateDriverWatchAllStateModify(t *testing.T, d core.StateDriver) {
	state := &testState{IntField: 1234, StrField: "testString"}
	modState := &testState{IntField: 5678, StrField: "modString"}
	baseKey := "modify"
	key := baseKey + "/testKeyWatchAll"

	err := d.WriteState(key, state, json.Marshal)
	if err != nil {
		t.Fatalf("failed to write state. Error: %s", err)
	}
	defer func() {
		d.ClearState(key)
	}()

	recvErr := make(chan error, 1)
	stateCh := make(chan core.WatchState, 1)
	timer := time.After(waitTimeout)

	go func(rsps chan core.WatchState, retErr chan error) {
		err := d.WatchAllState(baseKey, state, json.Unmarshal, stateCh)
		if err != nil {
			retErr <- err
			return
		}
	}(stateCh, recvErr)

	// trigger modify after a slight pause to ensure that events are not missed
	time.Sleep(time.Second)
	err = d.WriteState(key, modState, json.Marshal)
	if err != nil {
		t.Fatalf("failed to write state. Error: %s", err)
	}

for_label:
	for {
		select {
		case watchState := <-stateCh:
			s := watchState.Curr.(*testState)
			if s.IntField != modState.IntField || s.StrField != modState.StrField {
				t.Fatalf("Watch state mismatch. Expctd: %+v, Rcvd: %+v", modState, s)
			}
			if watchState.Prev == nil {
				t.Fatalf("Received a modify event without previous state value set.")
			}
			s = watchState.Prev.(*testState)
			if s.IntField != state.IntField || s.StrField != state.StrField {
				t.Fatalf("Watch state mismatch. Expctd: %+v, Rcvd: %+v", state, s)
			}
			break for_label
		case err := <-recvErr:
			t.Fatalf("Watch failed. Error: %s", err)
			break for_label
		case <-timer:
			t.Fatalf("timed out waiting for events")
			break for_label
		}
	}
}

func TestEtcdStateDriverWatchAllStateModify(t *testing.T) {
	driver := setupEtcdDriver(t)
	commonTestStateDriverWatchAllStateModify(t, driver)
}

func commonTestStateDriverWatchAllStateDelete(t *testing.T, d core.StateDriver) {
	state := &testState{IntField: 1234, StrField: "testString"}
	baseKey := "delete"
	key := baseKey + "/testKeyWatchAll"

	err := d.WriteState(key, state, json.Marshal)
	if err != nil {
		t.Fatalf("failed to write state. Error: %s", err)
	}
	defer func() {
		d.ClearState(key)
	}()

	recvErr := make(chan error, 1)
	stateCh := make(chan core.WatchState, 1)
	timer := time.After(waitTimeout)

	go func(rsps chan core.WatchState, retErr chan error) {
		err := d.WatchAllState(baseKey, state, json.Unmarshal, stateCh)
		if err != nil {
			retErr <- err
			return
		}
	}(stateCh, recvErr)

	// trigger delete after a slight pause to ensure that events are not missed
	time.Sleep(time.Second)
	err = d.ClearState(key)
	if err != nil {
		t.Fatalf("failed to clear state. Error: %s", err)
	}

for_label:
	for {
		select {
		case watchState := <-stateCh:
			if watchState.Curr != nil {
				t.Fatalf("Watch state has current state set %+v, expected to be nil", watchState.Curr)
			}
			if watchState.Prev == nil {
				t.Fatalf("Received a delete event without previous state value set.")
			}
			s := watchState.Prev.(*testState)
			if s.IntField != state.IntField || s.StrField != state.StrField {
				t.Fatalf("Watch state mismatch. Expctd: %+v, Rcvd: %+v", state, s)
			}
			break for_label
		case err := <-recvErr:
			t.Fatalf("Watch failed. Error: %s", err)
			break for_label
		case <-timer:
			t.Fatalf("timed out waiting for events")
			break for_label
		}
	}
}

func TestEtcdStateDriverWatchAllStateDelete(t *testing.T) {
	driver := setupEtcdDriver(t)
	commonTestStateDriverWatchAllStateDelete(t, driver)
}
