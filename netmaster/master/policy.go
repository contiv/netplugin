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
	"strconv"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/objmodel/contivModel"
	"github.com/contiv/ofnet"

	log "github.com/Sirupsen/logrus"
)

// Create the netmaster
var ofnetMaster *ofnet.OfnetMaster

// EpgPolicyExists is a well known exported error
var EpgPolicyExists = core.Errorf("Epg policy exists")

// InitPolicyMgr initializes the policy manager
func InitPolicyMgr() error {
	ofnetMaster = ofnet.NewOfnetMaster(ofnet.OFNET_MASTER_PORT)
	if ofnetMaster == nil {
		log.Fatalf("Error creating ofnet master")
	}

	return nil
}

// PolicyAttach attaches a policy to an endpoint and adds associated rules to policyDB
func PolicyAttach(epg *contivModel.EndpointGroup, policy *contivModel.Policy) error {
	epgpKey := epg.Key + ":" + policy.Key

	// See if it already exists
	gp := FindEpgPolicy(epgpKey)
	if gp != nil {
		log.Errorf("EPG policy %s already exists", epgpKey)
		return EpgPolicyExists
	}

	// Create the epg policy
	gp, err := NewEpgPolicy(epgpKey, epg.EndpointGroupID, policy)
	if err != nil {
		log.Errorf("Error creating EPG policy. Err: %v", err)
		return err
	}

	return nil
}

// PolicyDetach detaches policy from an endpoint and removes associated rules from policyDB
func PolicyDetach(epg *contivModel.EndpointGroup, policy *contivModel.Policy) error {
	epgpKey := epg.Key + ":" + policy.Key

	// find the policy
	gp := FindEpgPolicy(epgpKey)
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
	// Walk all associated endpoint groups
	for epgKey := range policy.LinkSets.EndpointGroups {
		gpKey := epgKey + ":" + policy.Key

		// Find the epg policy
		gp := FindEpgPolicy(gpKey)
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
	}

	return nil
}

// PolicyDelRule removes a rule from existing policy
func PolicyDelRule(policy *contivModel.Policy, rule *contivModel.Rule) error {
	// Walk all associated endpoint groups
	for epgKey := range policy.LinkSets.EndpointGroups {
		gpKey := epgKey + ":" + policy.Key

		// Find the epg policy
		gp := FindEpgPolicy(gpKey)
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
	}

	return nil
}

// RuleMap maps a policy rule to list of ofnet rules
type RuleMap struct {
	Rule       *contivModel.Rule                 // policy rule
	ofnetRules map[string]*ofnet.OfnetPolicyRule // Ofnet rules associated with this policy rule
}

// EpgPolicy has an instance of policy attached to an endpoint group
type EpgPolicy struct {
	EpgPolicyKey    string              // Key for this epg policy
	EndpointGroupID int                 // Endpoint group where this policy is attached to
	RuleMaps        map[string]*RuleMap // rules associated with this policy
}

// Epg policy database
// FIXME: do we need to persist this?
var epgPolicyDb = make(map[string]*EpgPolicy)

// NewEpgPolicy creates a new policy instance attached to an endpoint group
func NewEpgPolicy(epgpKey string, epgID int, policy *contivModel.Policy) (*EpgPolicy, error) {
	gp := new(EpgPolicy)
	gp.EpgPolicyKey = epgpKey
	gp.EndpointGroupID = epgID

	log.Infof("Creating new epg policy: %s", epgpKey)

	// init the dbs
	gp.RuleMaps = make(map[string]*RuleMap)

	// Install all rules within the policy
	for ruleKey := range policy.LinkSets.Rules {
		// find the rule
		rule := contivModel.FindRule(ruleKey)
		if rule == nil {
			log.Errorf("Error finding the rule %s", ruleKey)
			return nil, core.Errorf("rule not found")
		}

		log.Infof("Adding Rule %s to epgp policy %s", ruleKey, epgpKey)

		// Add the rule to epg Policy
		err := gp.AddRule(rule)
		if err != nil {
			log.Errorf("Error adding rule %s to epg polict %s. Err: %v", ruleKey, epgpKey, err)
			return nil, err
		}
	}

	// Save it in DB
	epgPolicyDb[epgpKey] = gp

	log.Info("Created epg policy {%+v}", gp)

	return gp, nil
}

// FindEpgPolicy finds an epg policy
func FindEpgPolicy(epgpKey string) *EpgPolicy {
	return epgPolicyDb[epgpKey]
}

// Delete deletes the epg policy
func (gp *EpgPolicy) Delete() error {
	// delete from the DB
	delete(epgPolicyDb, gp.EpgPolicyKey)

	return nil
}

// createOfnetRule creates a directional ofnet rule
func (gp *EpgPolicy) createOfnetRule(rule *contivModel.Rule, dir string) (*ofnet.OfnetPolicyRule, error) {
	ruleID := gp.EpgPolicyKey + ":" + rule.Key + ":" + dir

	// Create an ofnet rule
	ofnetRule := new(ofnet.OfnetPolicyRule)
	ofnetRule.RuleId = ruleID
	ofnetRule.Priority = rule.Priority
	ofnetRule.Action = rule.Action

	remoteEpgID := 0
	// See if user specified an endpoint Group in the rule
	if rule.EndpointGroup != "" {
		epgKey := rule.TenantName + ":" + rule.Network + ":" + rule.EndpointGroup

		// find the endpoint group
		epg := contivModel.FindEndpointGroup(epgKey)
		if epg == nil {
			log.Errorf("Error finding endpoint group %s", epgKey)
			return nil, core.Errorf("endpoint group not found")
		}

		remoteEpgID = epg.EndpointGroupID
	}

	// Set protocol
	switch rule.Protocol {
	case "tcp":
		ofnetRule.IpProtocol = 6
	case "udp":
		ofnetRule.IpProtocol = 17
	case "icmp":
		ofnetRule.IpProtocol = 1
	case "igmp":
		ofnetRule.IpProtocol = 2
	case "":
		ofnetRule.IpProtocol = 0
	default:
		proto, err := strconv.Atoi(rule.Protocol)
		if err == nil && proto < 256 {
			ofnetRule.IpProtocol = uint8(proto)
		}
	}

	// Set directional parameters
	switch dir {
	case "inRx":
		// Set src/dest endpoint group
		ofnetRule.DstEndpointGroup = gp.EndpointGroupID
		ofnetRule.SrcEndpointGroup = remoteEpgID

		// Set src/dest IP Address
		ofnetRule.SrcIpAddr = rule.IpAddress

		// set port numbers
		ofnetRule.DstPort = uint16(rule.Port)

		// set tcp flags
		if rule.Protocol == "tcp" && rule.Port == 0 {
			ofnetRule.TcpFlags = "syn,!ack"
		}
	case "inTx":
		// Set src/dest endpoint group
		ofnetRule.SrcEndpointGroup = gp.EndpointGroupID
		ofnetRule.DstEndpointGroup = remoteEpgID

		// Set src/dest IP Address
		ofnetRule.DstIpAddr = rule.IpAddress

		// set port numbers
		ofnetRule.SrcPort = uint16(rule.Port)
	case "outRx":
		// Set src/dest endpoint group
		ofnetRule.DstEndpointGroup = gp.EndpointGroupID
		ofnetRule.SrcEndpointGroup = remoteEpgID

		// Set src/dest IP Address
		ofnetRule.SrcIpAddr = rule.IpAddress

		// set port numbers
		ofnetRule.SrcPort = uint16(rule.Port)
	case "outTx":
		// Set src/dest endpoint group
		ofnetRule.SrcEndpointGroup = gp.EndpointGroupID
		ofnetRule.DstEndpointGroup = remoteEpgID

		// Set src/dest IP Address
		ofnetRule.DstIpAddr = rule.IpAddress

		// set port numbers
		ofnetRule.DstPort = uint16(rule.Port)

		// set tcp flags
		if rule.Protocol == "tcp" && rule.Port == 0 {
			ofnetRule.TcpFlags = "syn,!ack"
		}
	default:
		log.Fatalf("Unknown rule direction %s", dir)
	}

	// Add the Rule to policyDB
	err := ofnetMaster.AddRule(ofnetRule)
	if err != nil {
		log.Errorf("Error creating rule {%+v}. Err: %v", ofnetRule, err)
		return nil, err
	}

	log.Infof("Added rule {%+v} to policyDB", ofnetRule)

	return ofnetRule, nil
}

// AddRule adds a rule to epg policy
func (gp *EpgPolicy) AddRule(rule *contivModel.Rule) error {
	var dirs []string

	// check if the rule exists already
	if gp.RuleMaps[rule.Key] != nil {
		// FIXME: see if we can update the rule
		return core.Errorf("Rule already exists")
	}

	// Figure out all the directional rules we need to install
	switch rule.Direction {
	case "in":
		if (rule.Protocol == "udp" || rule.Protocol == "tcp") && rule.Port != 0 {
			dirs = []string{"inRx", "inTx"}
		} else {
			dirs = []string{"inRx"}
		}
	case "out":
		if (rule.Protocol == "udp" || rule.Protocol == "tcp") && rule.Port != 0 {
			dirs = []string{"outRx", "outTx"}
		} else {
			dirs = []string{"outTx"}
		}
	case "both":
		if (rule.Protocol == "udp" || rule.Protocol == "tcp") && rule.Port != 0 {
			dirs = []string{"inRx", "inTx", "outRx", "outTx"}
		} else {
			dirs = []string{"inRx", "outTx"}
		}

	}

	// create a ruleMap
	ruleMap := new(RuleMap)
	ruleMap.ofnetRules = make(map[string]*ofnet.OfnetPolicyRule)
	ruleMap.Rule = rule

	// Create ofnet rules
	for _, dir := range dirs {
		ofnetRule, err := gp.createOfnetRule(rule, dir)
		if err != nil {
			log.Errorf("Error creating %s ofnet rule for {%+v}. Err: %v", dir, rule, err)
			return err
		}

		// add it to the rule map
		ruleMap.ofnetRules[ofnetRule.RuleId] = ofnetRule
	}

	// save the rulemap
	gp.RuleMaps[rule.Key] = ruleMap

	return nil
}

// DelRule removes a rule from epg policy
func (gp *EpgPolicy) DelRule(rule *contivModel.Rule) error {
	// check if the rule exists
	ruleMap := gp.RuleMaps[rule.Key]
	if ruleMap == nil {
		return core.Errorf("Rule does not exists")
	}

	// Delete each ofnet rule under this policy rule
	for _, ofnetRule := range ruleMap.ofnetRules {
		log.Infof("Deleting rule {%+v} from policyDB", ofnetRule)

		// Delete the rule from policyDB
		err := ofnetMaster.DelRule(ofnetRule)
		if err != nil {
			log.Errorf("Error deleting the ofnet rule {%+v}. Err: %v", ofnetRule, err)
		}
	}

	// delete the cache
	delete(gp.RuleMaps, rule.Key)

	return nil
}
