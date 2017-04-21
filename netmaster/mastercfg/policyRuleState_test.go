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
	testRuleID = "testPolicyRule"
	ruleCfgKey = policyRuleConfigPathPrefix + testRuleID
)

type testRuleStateDriver struct{}

var policyRuleStateDriver = &testRuleStateDriver{}

func (d *testRuleStateDriver) Init(instInfo *core.InstanceInfo) error {
	return core.Errorf("Shouldn't be called!")
}

func (d *testRuleStateDriver) Deinit() {
}

func (d *testRuleStateDriver) Write(key string, value []byte) error {
	return core.Errorf("Shouldn't be called!")
}

func (d *testRuleStateDriver) Read(key string) ([]byte, error) {
	return []byte{}, core.Errorf("Shouldn't be called!")
}

func (d *testRuleStateDriver) ReadAll(baseKey string) ([][]byte, error) {
	return [][]byte{}, core.Errorf("Shouldn't be called!")
}

func (d *testRuleStateDriver) WatchAll(baseKey string, rsps chan [2][]byte) error {
	return core.Errorf("not supported")
}

func (d *testRuleStateDriver) validateKey(key string) error {
	if key != ruleCfgKey {
		return core.Errorf("Unexpected key. recvd: %s expected: %s ",
			key, ruleCfgKey)
	}

	return nil
}

func (d *testRuleStateDriver) ClearState(key string) error {
	return d.validateKey(key)
}

func (d *testRuleStateDriver) ReadState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) error {
	return d.validateKey(key)
}

func (d *testRuleStateDriver) ReadAllState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) ([]core.State, error) {
	return nil, core.Errorf("Shouldn't be called!")
}

func (d *testRuleStateDriver) WatchAllState(baseKey string, sType core.State,
	unmarshal func([]byte, interface{}) error, rsps chan core.WatchState) error {
	return core.Errorf("not supported")
}

func (d *testRuleStateDriver) WriteState(key string, value core.State,
	marshal func(interface{}) ([]byte, error)) error {
	return d.validateKey(key)
}

func TestCfgPolicyRuleRead(t *testing.T) {
	ruleCfg := &CfgPolicyRule{}
	ruleCfg.StateDriver = policyRuleStateDriver

	err := ruleCfg.Read(testRuleID)
	if err != nil {
		t.Fatalf("read config state failed. Error: %s", err)
	}
}

func TestCfgPolicyRuleWrite(t *testing.T) {
	ruleCfg := &CfgPolicyRule{}
	ruleCfg.StateDriver = policyRuleStateDriver
	ruleCfg.RuleId = testRuleID

	err := ruleCfg.Write()
	if err != nil {
		t.Fatalf("write config state failed. Error: %s", err)
	}
}

func TestCfgPolicyRuleClear(t *testing.T) {
	ruleCfg := &CfgPolicyRule{}
	ruleCfg.StateDriver = policyRuleStateDriver
	ruleCfg.RuleId = testRuleID

	err := ruleCfg.Clear()
	if err != nil {
		t.Fatalf("clear config state failed. Error: %s", err)
	}
}
