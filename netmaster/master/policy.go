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

package master

import (
	"github.com/contiv/netplugin/contivmodel"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/utils"

	log "github.com/Sirupsen/logrus"
)

// EpgPolicyExists is a well known exported error
var EpgPolicyExists = core.Errorf("Epg policy exists")

// isPolicyEnabled checks if policies needs to be installed in hosts
func isPolicyEnabled() bool {
	// Dont install policies in ACI mode
	isAci, err := IsAciConfigured()
	if err != nil {
		return true
	}

	return !isAci
}

// PolicyAttach attaches a policy to an endpoint and adds associated rules to policyDB
func PolicyAttach(epg *contivModel.EndpointGroup, policy *contivModel.Policy) error {
	// Dont install policies in ACI mode
	if !isPolicyEnabled() {
		return nil
	}

	epgpKey := epg.Key + ":" + policy.Key

	// See if it already exists
	gp := mastercfg.FindEpgPolicy(epgpKey)
	if gp != nil {
		log.Errorf("EPG policy %s already exists", epgpKey)
		return EpgPolicyExists
	}

	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		log.Errorf("Could not get StateDriver while attaching policy %+v", policy)
		return err
	}

	epgID, err := mastercfg.GetEndpointGroupID(stateDriver, epg.GroupName, epg.TenantName)
	if err != nil {
		log.Errorf("Error getting epgID for %s. Err: %v", epgpKey, err)
		return err
	}

	// Create the epg policy
	gp, err = mastercfg.NewEpgPolicy(epgpKey, epgID, policy)
	if err != nil {
		log.Errorf("Error creating EPG policy. Err: %v", err)
		return err
	}

	return nil
}

// PolicyDetach detaches policy from an endpoint and removes associated rules from policyDB
func PolicyDetach(epg *contivModel.EndpointGroup, policy *contivModel.Policy) error {
	// Dont install policies in ACI mode
	if !isPolicyEnabled() {
		return nil
	}

	epgpKey := epg.Key + ":" + policy.Key

	// find the policy
	gp := mastercfg.FindEpgPolicy(epgpKey)
	if gp == nil {
		log.Errorf("Epg policy %s does not exist", epgpKey)
		return core.Errorf("epg policy does not exist")
	}

	// Delete all rules within the policy
	for ruleKey := range policy.LinkSets.Rules {
		// find the rule
		rule := contivModel.FindRule(ruleKey)
		if rule == nil {
			log.Errorf("Error finding the rule %s", ruleKey)
			continue
		}

		log.Infof("Deleting Rule %s from epgp policy %s", ruleKey, epgpKey)

		// Add the rule to epg Policy
		err := gp.DelRule(rule)
		if err != nil {
			log.Errorf("Error deleting rule %s from epg polict %s. Err: %v", ruleKey, epgpKey, err)
		}
	}

	// delete it
	return gp.Delete()
}

// PolicyAddRule adds a rule to existing policy
func PolicyAddRule(policy *contivModel.Policy, rule *contivModel.Rule) error {
	// Dont install policies in ACI mode
	if !isPolicyEnabled() {
		return nil
	}

	// Walk all associated endpoint groups
	for epgKey := range policy.LinkSets.EndpointGroups {
		gpKey := epgKey + ":" + policy.Key

		// Find the epg policy
		gp := mastercfg.FindEpgPolicy(gpKey)
		if gp == nil {
			log.Errorf("Failed to find the epg policy %s", gpKey)
			return core.Errorf("epg policy not found")
		}

		// Add the Rule
		err := gp.AddRule(rule)
		if err != nil {
			log.Errorf("Error adding the rule %s to epg policy %s. Err: %v", rule.Key, gpKey, err)
			return err
		}

		// Save the policy state
		err = gp.Write()
		if err != nil {
			return err
		}
	}

	return nil
}

// PolicyDelRule removes a rule from existing policy
func PolicyDelRule(policy *contivModel.Policy, rule *contivModel.Rule) error {
	// Dont install policies in ACI mode
	if !isPolicyEnabled() {
		return nil
	}

	// Walk all associated endpoint groups
	for epgKey := range policy.LinkSets.EndpointGroups {
		gpKey := epgKey + ":" + policy.Key

		// Find the epg policy
		gp := mastercfg.FindEpgPolicy(gpKey)
		if gp == nil {
			log.Errorf("Failed to find the epg policy %s", gpKey)
			return core.Errorf("epg policy not found")
		}

		// delete the Rule
		err := gp.DelRule(rule)
		if err != nil {
			log.Errorf("Error deleting the rule %s from epg policy %s. Err: %v", rule.Key, gpKey, err)
			return err
		}

		// Save the policy state
		err = gp.Write()
		if err != nil {
			log.Errorf("Error writing polify %s to state store. Err: %v", gp.EpgPolicyKey, err)
		}
	}

	return nil
}
