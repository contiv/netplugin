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
	testSvcProviderID = "testSvcProvider"
	svcProviderCfgKey = svcProviderPathPrefix + testSvcProviderID
)

type testSvcProviderStateDriver struct{}

var svcProviderStateDriver = &testSvcProviderStateDriver{}

func (d *testSvcProviderStateDriver) Init(instInfo *core.InstanceInfo) error {
	return core.Errorf("Shouldn't be called!")
}

func (d *testSvcProviderStateDriver) Deinit() {
}

func (d *testSvcProviderStateDriver) Write(key string, value []byte) error {
	return core.Errorf("Shouldn't be called!")
}

func (d *testSvcProviderStateDriver) Read(key string) ([]byte, error) {
	return []byte{}, core.Errorf("Shouldn't be called!")
}

func (d *testSvcProviderStateDriver) ReadAll(baseKey string) ([][]byte, error) {
	return [][]byte{}, core.Errorf("Shouldn't be called!")
}

func (d *testSvcProviderStateDriver) WatchAll(baseKey string, rsps chan [2][]byte) error {
	return core.Errorf("not supported")
}

func (d *testSvcProviderStateDriver) validateKey(key string) error {
	if key != svcProviderCfgKey {
		return core.Errorf("Unexpected key. recvd: %s expected: %s ",
			key, svcProviderCfgKey)
	}

	return nil
}

func (d *testSvcProviderStateDriver) ClearState(key string) error {
	return d.validateKey(key)
}

func (d *testSvcProviderStateDriver) ReadState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) error {
	return d.validateKey(key)
}

func (d *testSvcProviderStateDriver) ReadAllState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) ([]core.State, error) {
	return nil, core.Errorf("Shouldn't be called!")
}

func (d *testSvcProviderStateDriver) WatchAllState(baseKey string, sType core.State,
	unmarshal func([]byte, interface{}) error, rsps chan core.WatchState) error {
	return core.Errorf("not supported")
}

func (d *testSvcProviderStateDriver) WriteState(key string, value core.State,
	marshal func(interface{}) ([]byte, error)) error {
	return d.validateKey(key)
}

func TestSvcProviderRead(t *testing.T) {
	svcProviderCfg := &SvcProvider{}
	svcProviderCfg.StateDriver = svcProviderStateDriver

	err := svcProviderCfg.Read(testSvcProviderID)
	if err != nil {
		t.Fatalf("read config state failed. Error: %s", err)
	}
}

func TestSvcProviderWrite(t *testing.T) {
	svcProviderCfg := &SvcProvider{}
	svcProviderCfg.StateDriver = svcProviderStateDriver
	svcProviderCfg.ID = testSvcProviderID

	err := svcProviderCfg.Write()
	if err != nil {
		t.Fatalf("write config state failed. Error: %s", err)
	}
}

func TestSvcProviderClear(t *testing.T) {
	svcProviderCfg := &SvcProvider{}
	svcProviderCfg.StateDriver = svcProviderStateDriver
	svcProviderCfg.ID = testSvcProviderID

	err := svcProviderCfg.Clear()
	if err != nil {
		t.Fatalf("clear config state failed. Error: %s", err)
	}
}
