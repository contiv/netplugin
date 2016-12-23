/***
Copyright 2017 Cisco Systems Inc. All rights reserved.

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
	"encoding/json"
	"fmt"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/ofnet"
)

const (
	policyRuleConfigPathPrefix = StateConfigPath + "policyRule/"
	policyRuleConfigPath       = policyRuleConfigPathPrefix + "%s"
)

// CfgPolicyRule implements the State interface for policy rules
type CfgPolicyRule struct {
	core.CommonState
	ofnet.OfnetPolicyRule
}

// Write the state.
func (s *CfgPolicyRule) Write() error {
	key := fmt.Sprintf(policyRuleConfigPath, s.RuleId)
	return s.StateDriver.WriteState(key, s, json.Marshal)
}

// Read the state for a given identifier.
func (s *CfgPolicyRule) Read(id string) error {
	key := fmt.Sprintf(policyRuleConfigPath, id)
	return s.StateDriver.ReadState(key, s, json.Unmarshal)
}

// ReadAll reads all state objects for the policy rules.
func (s *CfgPolicyRule) ReadAll() ([]core.State, error) {
	return s.StateDriver.ReadAllState(policyRuleConfigPathPrefix, s, json.Unmarshal)
}

// WatchAll fills a channel on each state event related to policy rules.
func (s *CfgPolicyRule) WatchAll(rsps chan core.WatchState) error {
	return s.StateDriver.WatchAllState(policyRuleConfigPathPrefix, s, json.Unmarshal,
		rsps)
}

// Clear removes the state.
func (s *CfgPolicyRule) Clear() error {
	key := fmt.Sprintf(policyRuleConfigPath, s.RuleId)
	return s.StateDriver.ClearState(key)
}

// addPolicyRuleState adds policy rule to state store
func addPolicyRuleState(ofnetRule *ofnet.OfnetPolicyRule) error {
	ruleCfg := &CfgPolicyRule{}
	ruleCfg.StateDriver = stateStore
	ruleCfg.OfnetPolicyRule = (*ofnetRule)

	// Save the rule
	return ruleCfg.Write()
}

// delPolicyRuleState deletes policy rule from state store
func delPolicyRuleState(ofnetRule *ofnet.OfnetPolicyRule) error {
	ruleCfg := &CfgPolicyRule{}
	ruleCfg.StateDriver = stateStore
	ruleCfg.OfnetPolicyRule = (*ofnetRule)

	// Delete the rule
	return ruleCfg.Clear()
}
