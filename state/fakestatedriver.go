package state

import (
	"strings"

	"github.com/contiv/netplugin/core"

	log "github.com/Sirupsen/logrus"
)

// The FakeStateDriver implements core.StateDriver interface for use with
// unit-tests

type ValueData struct {
	value []byte
}

type FakeStateDriverConfig struct {
}

type FakeStateDriver struct {
	TestState map[string]ValueData
}

func (d *FakeStateDriver) Init(config *core.Config) error {
	d.TestState = make(map[string]ValueData)

	return nil
}

func (d *FakeStateDriver) Deinit() {
	d.TestState = nil
}

func (d *FakeStateDriver) Write(key string, value []byte) error {
	val := ValueData{value: value}
	d.TestState[key] = val

	return nil
}

func (d *FakeStateDriver) Read(key string) ([]byte, error) {
	if val, ok := d.TestState[key]; ok {
		return val.value, nil
	}

	return []byte{}, core.Errorf("Key not found!")
}

func (d *FakeStateDriver) ReadAll(baseKey string) ([][]byte, error) {
	values := [][]byte{}

	for key, val := range d.TestState {
		if strings.Contains(key, baseKey) {
			values = append(values, val.value)
		}
	}
	return values, nil
}

func (d *FakeStateDriver) WatchAll(baseKey string, rsps chan [2][]byte) error {
	return core.Errorf("not supported")
}

func (d *FakeStateDriver) ClearState(key string) error {
	if _, ok := d.TestState[key]; ok {
		delete(d.TestState, key)
	}
	return nil
}

func (d *FakeStateDriver) ReadState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) error {
	encodedState, err := d.Read(key)
	if err != nil {
		return err
	}

	err = unmarshal(encodedState, value)
	if err != nil {
		return err
	}

	return nil
}

func (d *FakeStateDriver) ReadAllState(baseKey string, sType core.State,
	unmarshal func([]byte, interface{}) error) ([]core.State, error) {
	return ReadAllStateCommon(d, baseKey, sType, unmarshal)
}

func (d *FakeStateDriver) WatchAllState(baseKey string, sType core.State,
	unmarshal func([]byte, interface{}) error, rsps chan core.WatchState) error {
	return core.Errorf("not supported")
}

func (d *FakeStateDriver) WriteState(key string, value core.State,
	marshal func(interface{}) ([]byte, error)) error {
	encodedState, err := marshal(value)
	if err != nil {
		return err
	}

	err = d.Write(key, encodedState)
	if err != nil {
		return err
	}

	return nil
}

func (d *FakeStateDriver) DumpState() {
	for key, _ := range d.TestState {
		log.Printf("key: %q\n", key)
	}
}
