package drivers

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/contiv/netplugin/core"
)

// setup a etcd cluster, run tests and then cleanup the cluster
// XXX: enabled once I upgrade to golang 1.4
//func TestMain(m *testing.M) {
//
//	// start etcd
//	proc, err := os.StartProcess("etcd", []string{}, nil)
//	if err != nil {
//		log.Printf("failed to start etcd. Error: %s", err)
//		os.Exit(-1)
//	}
//
//	//run the tests
//	exitC := m.Run()
//
//	// stop etcd
//	proc.Kill()
//
//	os.Exit(exitC)
//}

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
	key := "contiv.io/dir1/testKeyRead"

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
