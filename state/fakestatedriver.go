package state

import (
	"strings"

	"github.com/contiv/netplugin/core"

	log "github.com/Sirupsen/logrus"
)

type valueData struct {
	value []byte
}

// FakeStateDriverConfig represents the configuration of the fake statedriver,
// which is an empty struct.
type FakeStateDriverConfig struct{}

// FakeStateDriver implements core.StateDriver interface for use with
// unit-tests
type FakeStateDriver struct {
	TestState map[string]valueData
}

// Init the driver
func (d *FakeStateDriver) Init(instInfo *core.InstanceInfo) error {
	d.TestState = make(map[string]valueData)

	return nil
}

// Deinit the driver
func (d *FakeStateDriver) Deinit() {
	d.TestState = nil
}

// Write value to key
func (d *FakeStateDriver) Write(key string, value []byte) error {
	val := valueData{value: value}
	d.TestState[key] = val

	return nil
}

// Read value from key
func (d *FakeStateDriver) Read(key string) ([]byte, error) {
	if val, ok := d.TestState[key]; ok {
		return val.value, nil
	}

	return []byte{}, core.Errorf("key not found! key: %v", key)
}

// ReadAll values from baseKey
func (d *FakeStateDriver) ReadAll(baseKey string) ([][]byte, error) {
	values := [][]byte{}

	for key, val := range d.TestState {
		if strings.Contains(key, baseKey) {
			values = append(values, val.value)
		}
	}
	return values, nil
}

// WatchAll values from baseKey
func (d *FakeStateDriver) WatchAll(baseKey string, rsps chan [2][]byte) error {
	log.Warnf("watchall not supported")
	select {} // block forever
}

// ClearState clears key
func (d *FakeStateDriver) ClearState(key string) error {
	if _, ok := d.TestState[key]; ok {
		delete(d.TestState, key)
	}
	return nil
}

// ReadState unmarshals state into a core.State
func (d *FakeStateDriver) ReadState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) error {
	encodedState, err := d.Read(key)
	if err != nil {
		return err
	}

	return unmarshal(encodedState, value)
}

// ReadAllState reads all state from baseKey of a given type
func (d *FakeStateDriver) ReadAllState(baseKey string, sType core.State,
	unmarshal func([]byte, interface{}) error) ([]core.State, error) {
	return readAllStateCommon(d, baseKey, sType, unmarshal)
}

// WatchAllState reads all state from baseKey of a given type
func (d *FakeStateDriver) WatchAllState(baseKey string, sType core.State,
	unmarshal func([]byte, interface{}) error, rsps chan core.WatchState) error {
	return core.Errorf("not supported")
}

// WriteState writes a core.State to key.
func (d *FakeStateDriver) WriteState(key string, value core.State,
	marshal func(interface{}) ([]byte, error)) error {
	encodedState, err := marshal(value)
	if err != nil {
		return err
	}

	return d.Write(key, encodedState)
}

// DumpState is a debugging tool.
func (d *FakeStateDriver) DumpState() {
	for key := range d.TestState {
		log.Debugf("key: %q\n", key)
	}
}
