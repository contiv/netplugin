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
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	log "github.com/Sirupsen/logrus"

	"github.com/contiv/netplugin/contivmodel"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/ofnet"
)

const (
	policyConfigPathPrefix = StateConfigPath + "policy/"
	policyConfigPath       = policyConfigPathPrefix + "%s"
)

// RuleMap maps a policy rule to list of ofnet rules
type RuleMap struct {
	Rule       *contivModel.Rule                 // policy rule
	OfnetRules map[string]*ofnet.OfnetPolicyRule // Ofnet rules associated with this policy rule
}

// EpgPolicy has an instance of policy attached to an endpoint group
type EpgPolicy struct {
	core.CommonState
	EpgPolicyKey    string              // Key for this epg policy
	EndpointGroupID int                 // Endpoint group where this policy is attached to
	RuleMaps        map[string]*RuleMap // rules associated with this policy
}

// Epg policy database
var epgPolicyDb = make(map[string]*EpgPolicy)

// Create the netmaster
var ofnetMaster *ofnet.OfnetMaster

// state store
var stateStore core.StateDriver

// InitPolicyMgr initializes the policy manager
func InitPolicyMgr(stateDriver core.StateDriver, ofm *ofnet.OfnetMaster) error {
	// save statestore and ofnet masters
	stateStore = stateDriver
	ofnetMaster = ofm

	// restore all existing epg policies
	err := restoreEpgPolicies(stateDriver)
	if err != nil {
		log.Errorf("Error restoring EPG policies. ")
	}
	return nil
}

// NewEpgPolicy creates a new policy instance attached to an endpoint group
func NewEpgPolicy(epgpKey string, epgID int, policy *contivModel.Policy) (*EpgPolicy, error) {
	gp := new(EpgPolicy)
	gp.EpgPolicyKey = epgpKey
	gp.ID = epgpKey
	gp.EndpointGroupID = epgID
	gp.StateDriver = stateStore

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

	// Save the policy state
	err := gp.Write()
	if err != nil {
		return nil, err
	}

	// Save it in local cache
	epgPolicyDb[epgpKey] = gp

	log.Info("Created epg policy {%+v}", gp)

	return gp, nil
}

// restoreEpgPolicies restores all EPG policies from state store
func restoreEpgPolicies(stateDriver core.StateDriver) error {
	// read all epg policies
	gp := new(EpgPolicy)
	gp.StateDriver = stateDriver
	gpCfgs, err := gp.ReadAll()
	if err == nil {
		for _, gpCfg := range gpCfgs {
			epgp := gpCfg.(*EpgPolicy)
			log.Infof("Restoring EpgPolicy: %+v", epgp)

			// save it in cache
			epgPolicyDb[epgp.EpgPolicyKey] = epgp

			// Restore all rules within the policy
			for ruleKey, ruleMap := range epgp.RuleMaps {
				log.Infof("Restoring Rule %s, Rule: %+v", ruleKey, ruleMap.Rule)

				// delete the entry from the map so that we can add it back
				delete(epgp.RuleMaps, ruleKey)

				// Add the rule to epg Policy
				err := epgp.AddRule(ruleMap.Rule)
				if err != nil {
					log.Errorf("Error restoring rule %s. Err: %v", ruleKey, err)
					return err
				}
			}
		}
	}

	return nil
}

// FindEpgPolicy finds an epg policy
func FindEpgPolicy(epgpKey string) *EpgPolicy {
	return epgPolicyDb[epgpKey]
}

// Delete deletes the epg policy
func (gp *EpgPolicy) Delete() error {
	// delete from the DB
	delete(epgPolicyDb, gp.EpgPolicyKey)

	return gp.Clear()
}

// createOfnetRule creates a directional ofnet rule
func (gp *EpgPolicy) createOfnetRule(rule *contivModel.Rule, dir string) (*ofnet.OfnetPolicyRule, error) {
	var remoteEpgID int
	var err error

	ruleID := gp.EpgPolicyKey + ":" + rule.Key + ":" + dir

	// Create an ofnet rule
	ofnetRule := new(ofnet.OfnetPolicyRule)
	ofnetRule.RuleId = ruleID
	ofnetRule.Priority = rule.Priority
	ofnetRule.Action = rule.Action

	// See if user specified an endpoint Group in the rule
	if rule.FromEndpointGroup != "" {
		remoteEpgID, err = GetEndpointGroupID(stateStore, rule.FromEndpointGroup, rule.TenantName)
		if err != nil {
			log.Errorf("Error finding endpoint group %s/%s/%s. Err: %v",
				rule.FromEndpointGroup, rule.FromNetwork, rule.TenantName, err)
		}
	} else if rule.ToEndpointGroup != "" {
		remoteEpgID, err = GetEndpointGroupID(stateStore, rule.ToEndpointGroup, rule.TenantName)
		if err != nil {
			log.Errorf("Error finding endpoint group %s/%s/%s. Err: %v",
				rule.ToEndpointGroup, rule.ToNetwork, rule.TenantName, err)
		}
	} else if rule.FromNetwork != "" {
		netKey := rule.TenantName + ":" + rule.FromNetwork

		net := contivModel.FindNetwork(netKey)
		if net == nil {
			log.Errorf("Network %s not found", netKey)
			return nil, errors.New("the FromNetwork key wasn't found")
		}

		rule.FromIpAddress = net.Subnet
	} else if rule.ToNetwork != "" {
		netKey := rule.TenantName + ":" + rule.ToNetwork

		net := contivModel.FindNetwork(netKey)
		if net == nil {
			log.Errorf("Network %s not found", netKey)
			return nil, errors.New("the ToNetwork key wasn't found")
		}

		rule.ToIpAddress = net.Subnet
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
		ofnetRule.SrcIpAddr = rule.FromIpAddress
		if len(rule.ToIpAddress) > 0 {
			ofnetRule.DstIpAddr = rule.ToIpAddress
		}

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
		ofnetRule.DstIpAddr = rule.FromIpAddress
		if len(rule.ToIpAddress) > 0 {
			ofnetRule.SrcIpAddr = rule.ToIpAddress
		}

		// set port numbers
		ofnetRule.SrcPort = uint16(rule.Port)
	case "outRx":
		// Set src/dest endpoint group
		ofnetRule.DstEndpointGroup = gp.EndpointGroupID
		ofnetRule.SrcEndpointGroup = remoteEpgID

		// Set src/dest IP Address
		ofnetRule.SrcIpAddr = rule.ToIpAddress

		// set port numbers
		ofnetRule.SrcPort = uint16(rule.Port)
	case "outTx":
		// Set src/dest endpoint group
		ofnetRule.SrcEndpointGroup = gp.EndpointGroupID
		ofnetRule.DstEndpointGroup = remoteEpgID

		// Set src/dest IP Address
		ofnetRule.DstIpAddr = rule.ToIpAddress

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
	err = ofnetMaster.AddRule(ofnetRule)
	if err != nil {
		log.Errorf("Error creating rule {%+v}. Err: %v", ofnetRule, err)
		return nil, err
	}

	// Send AddRule to netplugin agents
	err = addPolicyRuleState(ofnetRule)
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
	ruleMap.OfnetRules = make(map[string]*ofnet.OfnetPolicyRule)
	ruleMap.Rule = rule

	// Create ofnet rules
	for _, dir := range dirs {
		ofnetRule, err := gp.createOfnetRule(rule, dir)
		if err != nil {
			log.Errorf("Error creating %s ofnet rule for {%+v}. Err: %v", dir, rule, err)
			return err
		}

		// add it to the rule map
		ruleMap.OfnetRules[ofnetRule.RuleId] = ofnetRule
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
	for _, ofnetRule := range ruleMap.OfnetRules {
		log.Infof("Deleting rule {%+v} from policyDB", ofnetRule)

		// Delete the rule from policyDB
		err := ofnetMaster.DelRule(ofnetRule)
		if err != nil {
			log.Errorf("Error deleting the ofnet rule {%+v}. Err: %v", ofnetRule, err)
		}

		// Send DelRule to netplugin agents
		err = delPolicyRuleState(ofnetRule)
		if err != nil {
			log.Errorf("Error deleting the ofnet rule {%+v}. Err: %v", ofnetRule, err)
		}
	}

	// delete the cache
	delete(gp.RuleMaps, rule.Key)

	return nil
}

// Write the state.
func (gp *EpgPolicy) Write() error {
	key := fmt.Sprintf(policyConfigPath, gp.ID)
	return gp.StateDriver.WriteState(key, gp, json.Marshal)
}

// Read the state for a given identifier
func (gp *EpgPolicy) Read(id string) error {
	key := fmt.Sprintf(policyConfigPath, id)
	return gp.StateDriver.ReadState(key, gp, json.Unmarshal)
}

// ReadAll state and return the collection.
func (gp *EpgPolicy) ReadAll() ([]core.State, error) {
	return gp.StateDriver.ReadAllState(policyConfigPathPrefix, gp, json.Unmarshal)
}

// WatchAll state transitions and send them through the channel.
func (gp *EpgPolicy) WatchAll(rsps chan core.WatchState) error {
	return gp.StateDriver.WatchAllState(policyConfigPathPrefix, gp, json.Unmarshal,
		rsps)
}

// Clear removes the state.
func (gp *EpgPolicy) Clear() error {
	key := fmt.Sprintf(policyConfigPath, gp.ID)
	return gp.StateDriver.ClearState(key)
}

// NotifyEpgChanged triggers GARPs.
func NotifyEpgChanged(epgID int) {
	ofnetMaster.InjectGARPs(epgID)
}
