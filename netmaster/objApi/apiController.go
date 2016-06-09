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

package objApi

import (
	"errors"
	"github.com/contiv/contivmodel"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/objdb/modeldb"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
)

// APIController stores the api controller state
type APIController struct {
	router *mux.Router
}

var apiCtrler *APIController

// NewAPIController creates a new controller
func NewAPIController(router *mux.Router, storeURL string) *APIController {
	ctrler := new(APIController)
	ctrler.router = router

	// init modeldb
	modeldb.Init(storeURL)

	// initialize the model objects
	contivModel.Init()

	// Register Callbacks
	contivModel.RegisterGlobalCallbacks(ctrler)
	contivModel.RegisterAppProfileCallbacks(ctrler)
	contivModel.RegisterEndpointGroupCallbacks(ctrler)
	contivModel.RegisterNetworkCallbacks(ctrler)
	contivModel.RegisterPolicyCallbacks(ctrler)
	contivModel.RegisterRuleCallbacks(ctrler)
	contivModel.RegisterTenantCallbacks(ctrler)
	contivModel.RegisterBgpCallbacks(ctrler)
	contivModel.RegisterServiceLBCallbacks(ctrler)
	contivModel.RegisterExtContractsGroupCallbacks(ctrler)
	// Register routes
	contivModel.AddRoutes(router)

	// Init global state
	gc := contivModel.FindGlobal("global")
	if gc == nil {
		log.Infof("Creating default global config")
		err := contivModel.CreateGlobal(&contivModel.Global{
			Key:              "global",
			Name:             "global",
			NetworkInfraType: "default",
			Vlans:            "1-4094",
			Vxlans:           "1-10000",
		})
		if err != nil {
			log.Fatalf("Error creating global state. Err: %v", err)
		}
	}

	// Add default tenant if it doesnt exist
	tenant := contivModel.FindTenant("default")
	if tenant == nil {
		log.Infof("Creating default tenant")
		err := contivModel.CreateTenant(&contivModel.Tenant{
			Key:        "default",
			TenantName: "default",
		})
		if err != nil {
			log.Fatalf("Error creating default tenant. Err: %v", err)
		}
	}

	return ctrler
}

// Utility function to check if string exists in a slice
func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// GlobalCreate creates global state
func (ac *APIController) GlobalCreate(global *contivModel.Global) error {
	log.Infof("Received GlobalCreate: %+v", global)

	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	// Build global config
	gCfg := intent.ConfigGlobal{
		NwInfraType: global.NetworkInfraType,
		VLANs:       global.Vlans,
		VXLANs:      global.Vxlans,
	}

	// Create the object
	err = master.CreateGlobal(stateDriver, &gCfg)
	if err != nil {
		log.Errorf("Error creating global config {%+v}. Err: %v", global, err)
		return err
	}

	return nil
}

// GlobalUpdate updates global state
func (ac *APIController) GlobalUpdate(global, params *contivModel.Global) error {
	log.Infof("Received GlobalUpdate: %+v. Old: %+v", params, global)

	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	// Build global config
	gCfg := intent.ConfigGlobal{
		NwInfraType: params.NetworkInfraType,
		VLANs:       params.Vlans,
		VXLANs:      params.Vxlans,
	}

	// Create the object
	err = master.CreateGlobal(stateDriver, &gCfg)
	if err != nil {
		log.Errorf("Error creating global config {%+v}. Err: %v", global, err)
		return err
	}

	global.NetworkInfraType = params.NetworkInfraType
	global.Vlans = params.Vlans
	global.Vxlans = params.Vxlans

	return nil
}

// GlobalDelete is not supported
func (ac *APIController) GlobalDelete(global *contivModel.Global) error {
	log.Infof("Received GlobalDelete: %+v", global)

	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	// Delete global state
	err = master.DeleteGlobal(stateDriver)
	if err != nil {
		log.Errorf("Error deleting global config. Err: %v", err)
		return err
	}
	return nil
}

// AppProfileCreate creates app profile state
func (ac *APIController) AppProfileCreate(prof *contivModel.AppProfile) error {
	log.Infof("Received AppProfileCreate: %+v", prof)

	// Make sure tenant exists
	if prof.TenantName == "" {
		return core.Errorf("Invalid tenant name")
	}

	tenant := contivModel.FindTenant(prof.TenantName)
	if tenant == nil {
		return core.Errorf("Tenant %s not found", prof.TenantName)
	}

	// Setup links
	modeldb.AddLink(&prof.Links.Tenant, tenant)
	modeldb.AddLinkSet(&tenant.LinkSets.AppProfiles, prof)

	for _, epg := range prof.EndpointGroups {
		epgKey := prof.TenantName + ":" + epg
		epgObj := contivModel.FindEndpointGroup(epgKey)
		if epgObj == nil {
			return core.Errorf("EndpointGroup %s not found", epgKey)
		}
		modeldb.AddLinkSet(&prof.LinkSets.EndpointGroups, epgObj)
		modeldb.AddLink(&epgObj.Links.AppProfile, prof)
		err := epgObj.Write()
		if err != nil {
			log.Errorf("Error updating epg state(%+v). Err: %v", epgObj, err)
			return err
		}
	}

	err := tenant.Write()
	if err != nil {
		log.Errorf("Error updating tenant state(%+v). Err: %v", tenant, err)
		return err
	}

	CreateAppNw(prof)
	return nil
}

// AppProfileUpdate updates app
func (ac *APIController) AppProfileUpdate(oldProf, newProf *contivModel.AppProfile) error {
	log.Infof("Received AppProfileUpdate: %+v, newProf: %+v", oldProf, newProf)

	// handle any epg addition
	for _, epg := range newProf.EndpointGroups {
		epgKey := newProf.TenantName + ":" + epg
		log.Infof("Add %s to %s", epgKey, newProf.AppProfileName)
		epgObj := contivModel.FindEndpointGroup(epgKey)
		if epgObj == nil {
			return core.Errorf("EndpointGroup %s not found", epgKey)
		}
		modeldb.AddLinkSet(&newProf.LinkSets.EndpointGroups, epgObj)

		// workaround for objdb update problem
		modeldb.AddLinkSet(&oldProf.LinkSets.EndpointGroups, epgObj)

		modeldb.AddLink(&epgObj.Links.AppProfile, newProf)
		err := epgObj.Write()
		if err != nil {
			log.Errorf("Error updating epg state(%+v). Err: %v", epgObj, err)
			return err
		}
	}

	// handle any epg removal
	for _, epg := range oldProf.EndpointGroups {
		if !stringInSlice(epg, newProf.EndpointGroups) {
			epgKey := newProf.TenantName + ":" + epg
			log.Infof("Remove %s from %s", epgKey, newProf.AppProfileName)
			epgObj := contivModel.FindEndpointGroup(epgKey)
			if epgObj == nil {
				return core.Errorf("EndpointGroup %s not found", epgKey)
			}
			modeldb.RemoveLink(&epgObj.Links.AppProfile, oldProf)
			err := epgObj.Write()
			if err != nil {
				log.Errorf("Error updating epg state(%+v). Err: %v",
					epgObj, err)
				return err
			}

			// workaround for objdb update problem
			modeldb.RemoveLinkSet(&oldProf.LinkSets.EndpointGroups, epgObj)
		}
	}

	// workaround for objdb update problem -- should fix model
	oldProf.EndpointGroups = newProf.EndpointGroups

	// update the app nw
	DeleteAppNw(oldProf)
	CreateAppNw(oldProf)
	return nil
}

// AppProfileDelete delete the app
func (ac *APIController) AppProfileDelete(prof *contivModel.AppProfile) error {
	log.Infof("Received AppProfileDelete: %+v", prof)

	tenant := contivModel.FindTenant(prof.TenantName)
	if tenant == nil {
		return core.Errorf("Tenant %s not found", prof.TenantName)
	}

	DeleteAppNw(prof)

	// remove all links
	for _, epg := range prof.EndpointGroups {
		epgKey := prof.TenantName + ":" + epg
		epgObj := contivModel.FindEndpointGroup(epgKey)
		if epgObj == nil {
			log.Errorf("EndpointGroup %s not found", epgKey)
			continue
		}
		modeldb.RemoveLink(&epgObj.Links.AppProfile, prof)
		err := epgObj.Write()
		if err != nil {
			log.Errorf("Error updating epg state(%+v). Err: %v", epgObj, err)
		}
	}

	modeldb.RemoveLinkSet(&tenant.LinkSets.AppProfiles, prof)
	tenant.Write()
	return nil
}

// Cleans up state off endpointGroup and related objects.
func endpointGroupCleanup(endpointGroup *contivModel.EndpointGroup) {
	// delete the endpoint group state
	err := master.DeleteEndpointGroup(endpointGroup.TenantName, endpointGroup.GroupName)
	if err != nil {
		log.Errorf("Error deleting endpoint group %+v. Err: %v", endpointGroup, err)
	}

	// Detach the endpoint group from the Policies
	for _, policyName := range endpointGroup.Policies {
		policyKey := endpointGroup.TenantName + ":" + policyName

		// find the policy
		policy := contivModel.FindPolicy(policyKey)
		if policy == nil {
			log.Errorf("Could not find policy %s", policyName)
			continue
		}

		// detach policy to epg
		err := master.PolicyDetach(endpointGroup, policy)
		if err != nil && err != master.EpgPolicyExists {
			log.Errorf("Error detaching policy %s from epg %s", policyName, endpointGroup.Key)
		}

		// Remove links
		modeldb.RemoveLinkSet(&policy.LinkSets.EndpointGroups, endpointGroup)
		modeldb.RemoveLinkSet(&endpointGroup.LinkSets.Policies, policy)
		policy.Write()
	}

	// Cleanup any external contracts
	err = cleanupExternalContracts(endpointGroup)
	if err != nil {
		log.Errorf("Error cleaning up external contracts for epg %s", endpointGroup.Key)
	}

	// Remove the endpoint group from network and tenant link sets.
	nwObjKey := endpointGroup.TenantName + ":" + endpointGroup.NetworkName
	network := contivModel.FindNetwork(nwObjKey)
	if network != nil {
		modeldb.RemoveLinkSet(&network.LinkSets.EndpointGroups, endpointGroup)
		network.Write()
	}
	tenant := contivModel.FindTenant(endpointGroup.TenantName)
	if tenant != nil {
		modeldb.RemoveLinkSet(&tenant.LinkSets.EndpointGroups, endpointGroup)
		tenant.Write()
	}
}

// FIXME: hack to allocate unique endpoint group ids
var globalEpgID = 1

// EndpointGroupCreate creates end point group
func (ac *APIController) EndpointGroupCreate(endpointGroup *contivModel.EndpointGroup) error {
	log.Infof("Received EndpointGroupCreate: %+v", endpointGroup)

	// Find the tenant
	tenant := contivModel.FindTenant(endpointGroup.TenantName)
	if tenant == nil {
		return core.Errorf("Tenant not found")
	}

	// Find the network
	nwObjKey := endpointGroup.TenantName + ":" + endpointGroup.NetworkName
	network := contivModel.FindNetwork(nwObjKey)
	if network == nil {
		return core.Errorf("Network %s not found", endpointGroup.NetworkName)
	}

	// If there is a Network with the same name as this endpointGroup, reject.
	nameClash := contivModel.FindNetwork(endpointGroup.Key)
	if nameClash != nil {
		return core.Errorf("Network %s conflicts with the endpointGroup name",
			nameClash.NetworkName)
	}

	// create the endpoint group state
	err := master.CreateEndpointGroup(endpointGroup.TenantName, endpointGroup.NetworkName, endpointGroup.GroupName)
	if err != nil {
		log.Errorf("Error creating endpoing group %+v. Err: %v", endpointGroup, err)
		return err
	}

	// for each policy create an epg policy Instance
	for _, policyName := range endpointGroup.Policies {
		policyKey := endpointGroup.TenantName + ":" + policyName
		// find the policy
		policy := contivModel.FindPolicy(policyKey)
		if policy == nil {
			log.Errorf("Could not find policy %s", policyName)
			endpointGroupCleanup(endpointGroup)
			return core.Errorf("Policy not found")
		}

		// attach policy to epg
		err = master.PolicyAttach(endpointGroup, policy)
		if err != nil {
			log.Errorf("Error attaching policy %s to epg %s", policyName, endpointGroup.Key)
			endpointGroupCleanup(endpointGroup)
			return err
		}

		// establish Links
		modeldb.AddLinkSet(&policy.LinkSets.EndpointGroups, endpointGroup)
		modeldb.AddLinkSet(&endpointGroup.LinkSets.Policies, policy)

		// Write the policy
		err = policy.Write()
		if err != nil {
			endpointGroupCleanup(endpointGroup)
			return err
		}
	}

	// Setup external contracts this EPG might have.
	err = setupExternalContracts(endpointGroup, endpointGroup.ExtContractsGrps)
	if err != nil {
		log.Errorf("Error setting up external contracts for epg %s", endpointGroup.Key)
		endpointGroupCleanup(endpointGroup)
		return err
	}

	// Setup links
	modeldb.AddLink(&endpointGroup.Links.Network, network)
	modeldb.AddLink(&endpointGroup.Links.Tenant, tenant)
	modeldb.AddLinkSet(&network.LinkSets.EndpointGroups, endpointGroup)
	modeldb.AddLinkSet(&tenant.LinkSets.EndpointGroups, endpointGroup)

	// Save the tenant and network since we added the links
	err = network.Write()
	if err != nil {
		endpointGroupCleanup(endpointGroup)
		return err
	}

	err = tenant.Write()
	if err != nil {
		endpointGroupCleanup(endpointGroup)
		return err
	}

	return nil
}

// EndpointGroupUpdate updates endpoint group
func (ac *APIController) EndpointGroupUpdate(endpointGroup, params *contivModel.EndpointGroup) error {
	log.Infof("Received EndpointGroupUpdate: %+v, params: %+v", endpointGroup, params)

	// if the network association was changed, reject the update.
	if endpointGroup.NetworkName != params.NetworkName {
		return core.Errorf("Cannot change network association after epg is created.")
	}

	// Only update policy attachments

	// Look for policy adds
	for _, policyName := range params.Policies {
		if !stringInSlice(policyName, endpointGroup.Policies) {
			policyKey := endpointGroup.TenantName + ":" + policyName

			// find the policy
			policy := contivModel.FindPolicy(policyKey)
			if policy == nil {
				log.Errorf("Could not find policy %s", policyName)
				return core.Errorf("Policy not found")
			}

			// attach policy to epg
			err := master.PolicyAttach(endpointGroup, policy)
			if err != nil && err != master.EpgPolicyExists {
				log.Errorf("Error attaching policy %s to epg %s", policyName, endpointGroup.Key)
				return err
			}

			// Setup links
			modeldb.AddLinkSet(&policy.LinkSets.EndpointGroups, endpointGroup)
			modeldb.AddLinkSet(&endpointGroup.LinkSets.Policies, policy)
			err = policy.Write()
			if err != nil {
				return err
			}
		}
	}

	// now look for policy removals
	for _, policyName := range endpointGroup.Policies {
		if !stringInSlice(policyName, params.Policies) {
			policyKey := endpointGroup.TenantName + ":" + policyName

			// find the policy
			policy := contivModel.FindPolicy(policyKey)
			if policy == nil {
				log.Errorf("Could not find policy %s", policyName)
				return core.Errorf("Policy not found")
			}

			// detach policy to epg
			err := master.PolicyDetach(endpointGroup, policy)
			if err != nil && err != master.EpgPolicyExists {
				log.Errorf("Error detaching policy %s from epg %s", policyName, endpointGroup.Key)
				return err
			}

			// Remove links
			modeldb.RemoveLinkSet(&policy.LinkSets.EndpointGroups, endpointGroup)
			modeldb.RemoveLinkSet(&endpointGroup.LinkSets.Policies, policy)
			err = policy.Write()
			if err != nil {
				return err
			}
		}
	}
	// Update the policy list
	endpointGroup.Policies = params.Policies

	// For the external contracts, we can keep the update simple. Remove
	// all that we have now, and update the epg with the new list.
	// Step 1: Cleanup existing external contracts.
	err := cleanupExternalContracts(endpointGroup)
	if err != nil {
		return err
	}
	// Step 2: Add contracts from the update.
	// Consumed contracts
	err = setupExternalContracts(endpointGroup, params.ExtContractsGrps)
	if err != nil {
		return err
	}

	// Update the epg itself with the new contracts groups.
	endpointGroup.ExtContractsGrps = params.ExtContractsGrps

	// if there is an associated app profiles, update that as well
	profKey := endpointGroup.Links.AppProfile.ObjKey
	profObj := contivModel.FindAppProfile(profKey)
	if profObj == nil {
		log.Warnf("EndpointGroupUpdate prof %s not found", profKey)
	} else {
		log.Infof("EndpointGroupUpdate sync prof %s", profKey)
		DeleteAppNw(profObj)
		CreateAppNw(profObj)
	}

	return nil
}

// EndpointGroupDelete deletes end point group
func (ac *APIController) EndpointGroupDelete(endpointGroup *contivModel.EndpointGroup) error {
	log.Infof("Received EndpointGroupDelete: %+v", endpointGroup)

	// if this is associated with an app profile, reject the delete
	if endpointGroup.Links.AppProfile.ObjKey != "" {
		return core.Errorf("Cannot delete %s, associated to appProfile %s",
			endpointGroup.GroupName, endpointGroup.Links.AppProfile.ObjKey)
	}

	endpointGroupCleanup(endpointGroup)
	return nil
}

// NetworkCreate creates network
func (ac *APIController) NetworkCreate(network *contivModel.Network) error {
	log.Infof("Received NetworkCreate: %+v", network)

	// Make sure tenant exists
	if network.TenantName == "" {
		return core.Errorf("Invalid tenant name")
	}

	tenant := contivModel.FindTenant(network.TenantName)
	if tenant == nil {
		return core.Errorf("Tenant not found")
	}

	// If there is an EndpointGroup with the same name as this network, reject.
	nameClash := contivModel.FindEndpointGroup(network.Key)
	if nameClash != nil {
		return core.Errorf("EndpointGroup %s conflicts with the network name",
			nameClash.GroupName)
	}

	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	// Build network config
	networkCfg := intent.ConfigNetwork{
		Name:       network.NetworkName,
		NwType:     network.NwType,
		PktTagType: network.Encap,
		PktTag:     network.PktTag,
		SubnetCIDR: network.Subnet,
		Gateway:    network.Gateway,
	}

	// Create the network
	err = master.CreateNetwork(networkCfg, stateDriver, network.TenantName)
	if err != nil {
		log.Errorf("Error creating network {%+v}. Err: %v", network, err)
		return err
	}

	// Setup links
	modeldb.AddLink(&network.Links.Tenant, tenant)
	modeldb.AddLinkSet(&tenant.LinkSets.Networks, network)

	// Save the tenant too since we added the links
	err = tenant.Write()
	if err != nil {
		log.Errorf("Error updating tenant state(%+v). Err: %v", tenant, err)
		return err
	}

	return nil
}

// NetworkUpdate updates network
func (ac *APIController) NetworkUpdate(network, params *contivModel.Network) error {
	log.Infof("Received NetworkUpdate: %+v, params: %+v", network, params)
	return core.Errorf("Cant change network parameters after its created")
}

// NetworkDelete deletes network
func (ac *APIController) NetworkDelete(network *contivModel.Network) error {
	log.Infof("Received NetworkDelete: %+v", network)

	// Find the tenant
	tenant := contivModel.FindTenant(network.TenantName)
	if tenant == nil {
		return core.Errorf("Tenant not found")
	}

	// if the network has associated epgs, fail the delete
	epgCount := len(network.LinkSets.EndpointGroups)
	if epgCount != 0 {
		return core.Errorf("cannot delete %s has %d endpoint groups",
			network.NetworkName, epgCount)
	}

	// Remove link
	modeldb.RemoveLinkSet(&tenant.LinkSets.Networks, network)

	// Save the tenant too since we removed the links
	err := tenant.Write()
	if err != nil {
		return err
	}

	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	// Delete the network
	networkID := network.NetworkName + "." + network.TenantName
	err = master.DeleteNetworkID(stateDriver, networkID)
	if err != nil {
		log.Errorf("Error deleting network %s. Err: %v", network.NetworkName, err)
		return err
	}

	return nil
}

// PolicyCreate creates policy
func (ac *APIController) PolicyCreate(policy *contivModel.Policy) error {
	log.Infof("Received PolicyCreate: %+v", policy)

	// Make sure tenant exists
	if policy.TenantName == "" {
		return core.Errorf("Invalid tenant name")
	}

	tenant := contivModel.FindTenant(policy.TenantName)
	if tenant == nil {
		return core.Errorf("Tenant not found")
	}

	// Setup links
	modeldb.AddLink(&policy.Links.Tenant, tenant)
	modeldb.AddLinkSet(&tenant.LinkSets.Policies, policy)

	// Save the tenant too since we added the links
	err := tenant.Write()
	if err != nil {
		log.Errorf("Error updating tenant state(%+v). Err: %v", tenant, err)
		return err
	}

	return nil
}

// PolicyUpdate updates policy
func (ac *APIController) PolicyUpdate(policy, params *contivModel.Policy) error {
	log.Infof("Received PolicyUpdate: %+v, params: %+v", policy, params)
	return nil
}

// PolicyDelete deletes policy
func (ac *APIController) PolicyDelete(policy *contivModel.Policy) error {
	log.Infof("Received PolicyDelete: %+v", policy)

	// Check if any endpoint group is using the Policy
	if len(policy.LinkSets.EndpointGroups) != 0 {
		return core.Errorf("Policy is being used")
	}

	// Delete all associated Rules
	for key := range policy.LinkSets.Rules {
		// delete the rule
		err := contivModel.DeleteRule(key)
		if err != nil {
			log.Errorf("Error deleting the rule: %s. Err: %v", key, err)
		}
	}

	return nil
}

func syncAppProfile(policy *contivModel.Policy) {
	// find all appProfiles that have an association
	profMap := make(map[string]bool)

	for epg := range policy.LinkSets.EndpointGroups {
		epgObj := contivModel.FindEndpointGroup(epg)
		if epgObj == nil {
			log.Warnf("syncAppProfile epg %s not found", epg)
		} else {
			prof := epgObj.Links.AppProfile.ObjKey
			profMap[prof] = true
			log.Infof("syncAppProfile epg %s ==> prof %s", epg, prof)
		}
	}

	for ap := range profMap {
		profObj := contivModel.FindAppProfile(ap)
		if profObj == nil {
			log.Warnf("syncAppProfile prof %s not found", ap)
		} else {
			log.Infof("syncAppProfile sync prof %s", ap)
			DeleteAppNw(profObj)
			CreateAppNw(profObj)
		}
	}
}

// RuleCreate Creates the rule within a policy
func (ac *APIController) RuleCreate(rule *contivModel.Rule) error {
	log.Infof("Received RuleCreate: %+v", rule)

	// verify parameter values
	if rule.Direction == "in" {
		if rule.ToNetwork != "" || rule.ToEndpointGroup != "" || rule.ToIpAddress != "" {
			return errors.New("Can not specify 'to' parameters in incoming rule")
		}
		if rule.FromNetwork != "" && rule.FromIpAddress != "" {
			return errors.New("Can not specify both from network and from ip address")
		}

		if rule.FromNetwork != "" && rule.FromEndpointGroup != "" {
			return errors.New("Can not specify both from network and from EndpointGroup")
		}
	} else if rule.Direction == "out" {
		if rule.FromNetwork != "" || rule.FromEndpointGroup != "" || rule.FromIpAddress != "" {
			return errors.New("Can not specify 'from' parameters in outgoing rule")
		}
		if rule.ToNetwork != "" && rule.ToIpAddress != "" {
			return errors.New("Can not specify both to-network and to-ip address")
		}
		if rule.ToNetwork != "" && rule.ToEndpointGroup != "" {
			return errors.New("Can not specify both to-network and to-EndpointGroup")
		}
	} else {
		return errors.New("Invalid direction for the rule")
	}

	// Make sure endpoint groups and networks referred exists.
	if rule.FromEndpointGroup != "" {
		epgKey := rule.TenantName + ":" + rule.FromEndpointGroup

		// find the endpoint group
		epg := contivModel.FindEndpointGroup(epgKey)
		if epg == nil {
			log.Errorf("Error finding endpoint group %s", epgKey)
			return errors.New("endpoint group not found")
		}
	} else if rule.ToEndpointGroup != "" {
		epgKey := rule.TenantName + ":" + rule.ToEndpointGroup

		// find the endpoint group
		epg := contivModel.FindEndpointGroup(epgKey)
		if epg == nil {
			log.Errorf("Error finding endpoint group %s", epgKey)
			return errors.New("endpoint group not found")
		}
	} else if rule.FromNetwork != "" {
		netKey := rule.TenantName + ":" + rule.FromNetwork

		net := contivModel.FindNetwork(netKey)
		if net == nil {
			log.Errorf("Network %s not found", netKey)
			return errors.New("FromNetwork not found")
		}
	} else if rule.ToNetwork != "" {
		netKey := rule.TenantName + ":" + rule.ToNetwork

		net := contivModel.FindNetwork(netKey)
		if net == nil {
			log.Errorf("Network %s not found", netKey)
			return errors.New("ToNetwork not found")
		}
	}

	policyKey := rule.TenantName + ":" + rule.PolicyName

	// find the policy
	policy := contivModel.FindPolicy(policyKey)
	if policy == nil {
		log.Errorf("Error finding policy %s", policyKey)
		return core.Errorf("Policy not found")
	}

	// Trigger policyDB Update
	err := master.PolicyAddRule(policy, rule)
	if err != nil {
		log.Errorf("Error adding rule %s to policy %s. Err: %v", rule.Key, policy.Key, err)
		return err
	}

	// link the rule to policy
	modeldb.AddLinkSet(&rule.LinkSets.Policies, policy)
	modeldb.AddLinkSet(&policy.LinkSets.Rules, rule)
	err = policy.Write()
	if err != nil {
		return err
	}

	// Update any affected app profiles
	syncAppProfile(policy)

	return nil
}

// RuleUpdate updates the rule within a policy
func (ac *APIController) RuleUpdate(rule, params *contivModel.Rule) error {
	log.Infof("Received RuleUpdate: %+v, params: %+v", rule, params)
	return errors.New("Can not update a rule after its created")
}

// RuleDelete deletes the rule within a policy
func (ac *APIController) RuleDelete(rule *contivModel.Rule) error {
	log.Infof("Received RuleDelete: %+v", rule)

	policyKey := rule.TenantName + ":" + rule.PolicyName

	// find the policy
	policy := contivModel.FindPolicy(policyKey)
	if policy == nil {
		log.Errorf("Error finding policy %s", policyKey)
		return core.Errorf("Policy not found")
	}

	// unlink the rule from policy
	modeldb.RemoveLinkSet(&policy.LinkSets.Rules, rule)
	err := policy.Write()
	if err != nil {
		return err
	}

	// Trigger policyDB Update
	err = master.PolicyDelRule(policy, rule)
	if err != nil {
		log.Errorf("Error deleting rule %s to policy %s. Err: %v", rule.Key, policy.Key, err)
		return err
	}

	// Update any affected app profiles
	syncAppProfile(policy)

	return nil
}

// TenantCreate creates a tenant
func (ac *APIController) TenantCreate(tenant *contivModel.Tenant) error {
	log.Infof("Received TenantCreate: %+v", tenant)

	if tenant.TenantName == "" {
		return core.Errorf("Invalid tenant name")
	}

	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	// Build tenant config
	tenantCfg := intent.ConfigTenant{
		Name:           tenant.TenantName,
		DefaultNetwork: tenant.DefaultNetwork,
	}

	// Create the tenant
	err = master.CreateTenant(stateDriver, &tenantCfg)
	if err != nil {
		log.Errorf("Error creating tenant {%+v}. Err: %v", tenant, err)
		return err
	}

	return nil
}

// TenantUpdate updates a tenant
func (ac *APIController) TenantUpdate(tenant, params *contivModel.Tenant) error {
	log.Infof("Received TenantUpdate: %+v, params: %+v", tenant, params)

	return core.Errorf("Cant change tenant parameters after its created")
}

// TenantDelete deletes a tenant
func (ac *APIController) TenantDelete(tenant *contivModel.Tenant) error {
	log.Infof("Received TenantDelete: %+v", tenant)

	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	// FIXME: Should we walk all objects under the tenant and delete it?

	// Delete the tenant
	err = master.DeleteTenantID(stateDriver, tenant.TenantName)
	if err != nil {
		log.Errorf("Error deleting tenant %s. Err: %v", tenant.TenantName, err)
	}

	return nil
}

//BgpCreate add bgp neighbor
func (ac *APIController) BgpCreate(bgpCfg *contivModel.Bgp) error {
	log.Infof("Received BgpCreate: %+v", bgpCfg)

	if bgpCfg.Hostname == "" {
		return core.Errorf("Invalid host name")
	}

	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	// Build bgp config
	bgpIntentCfg := intent.ConfigBgp{
		Hostname:   bgpCfg.Hostname,
		RouterIP:   bgpCfg.Routerip,
		As:         bgpCfg.As,
		NeighborAs: bgpCfg.NeighborAs,
		Neighbor:   bgpCfg.Neighbor,
	}

	// Add the Bgp neighbor
	err = master.AddBgp(stateDriver, &bgpIntentCfg)
	if err != nil {
		log.Errorf("Error creating Bgp neighbor {%+v}. Err: %v", bgpCfg.Neighbor, err)
		return err
	}
	return nil
}

//BgpDelete deletes bgp neighbor
func (ac *APIController) BgpDelete(bgpCfg *contivModel.Bgp) error {

	log.Infof("Received delete for Bgp config on {%+v} ", bgpCfg.Hostname)
	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	err = master.DeleteBgp(stateDriver, bgpCfg.Hostname)
	if err != nil {
		log.Errorf("Error Deleting Bgp neighbor. Err: %v", err)
		return err
	}
	return nil
}

//BgpUpdate updates bgp config
func (ac *APIController) BgpUpdate(oldbgpCfg *contivModel.Bgp, NewbgpCfg *contivModel.Bgp) error {
	log.Infof("Received BgpUpdate: %+v", NewbgpCfg)

	if NewbgpCfg.Hostname == "" {
		return core.Errorf("Invalid host name")
	}

	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	// Build bgp config
	bgpIntentCfg := intent.ConfigBgp{
		Hostname:   NewbgpCfg.Hostname,
		RouterIP:   NewbgpCfg.Routerip,
		As:         NewbgpCfg.As,
		NeighborAs: NewbgpCfg.NeighborAs,
		Neighbor:   NewbgpCfg.Neighbor,
	}

	// Add the Bgp neighbor
	err = master.AddBgp(stateDriver, &bgpIntentCfg)
	if err != nil {
		log.Errorf("Error creating Bgp neighbor {%+v}. Err: %v", NewbgpCfg.Neighbor, err)
		return err
	}

	return nil
}

//ServiceLBCreate creates service object
func (ac *APIController) ServiceLBCreate(serviceCfg *contivModel.ServiceLB) error {

	log.Infof("Received Service Load Balancer create: %+v", serviceCfg)

	if serviceCfg.ServiceName == "" {
		return core.Errorf("Invalid service name")
	}
	if serviceCfg.TenantName == "" {
		serviceCfg.TenantName = "default"
	}

	if len(serviceCfg.Selectors) == 0 {
		return core.Errorf("Invalid selector options")
	}

	if !validatePorts(serviceCfg.Ports) {
		return core.Errorf("Invalid Port maping . Port format is - Port:TargetPort:Protocol")
	}

	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	// Build service config
	serviceIntentCfg := intent.ConfigServiceLB{
		ServiceName: serviceCfg.ServiceName,
		Tenant:      serviceCfg.TenantName,
		Network:     serviceCfg.NetworkName,
		IPAddress:   serviceCfg.IpAddress,
	}
	serviceIntentCfg.Ports = append(serviceIntentCfg.Ports, serviceCfg.Ports...)

	serviceIntentCfg.Selectors = make(map[string]string)

	for _, selector := range serviceCfg.Selectors {
		if validateSelectors(selector) {
			key := strings.Split(selector, "=")[0]
			value := strings.Split(selector, "=")[1]
			serviceIntentCfg.Selectors[key] = value
		} else {
			return core.Errorf("Invalid selector %s. selector format is key1=value1", selector)
		}
	}
	// Add the service object
	err = master.CreateServiceLB(stateDriver, &serviceIntentCfg)
	if err != nil {
		log.Errorf("Error creating service  {%+v}. Err: %v", serviceIntentCfg.ServiceName, err)
		return err
	}
	return nil

}

//ServiceLBUpdate updates service object
func (ac *APIController) ServiceLBUpdate(oldServiceCfg *contivModel.ServiceLB, serviceCfg *contivModel.ServiceLB) error {
	return ac.ServiceLBCreate(serviceCfg)
}

//ServiceLBDelete deletes service object
func (ac *APIController) ServiceLBDelete(serviceCfg *contivModel.ServiceLB) error {

	log.Info("Received Service Load Balancer delete : {%+v}", serviceCfg)

	if serviceCfg.ServiceName == "" {
		return core.Errorf("Invalid service name")
	}
	if serviceCfg.TenantName == "" {
		serviceCfg.TenantName = "default"
	}
	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	// Add the service object
	err = master.DeleteServiceLB(stateDriver, serviceCfg.ServiceName, serviceCfg.TenantName)
	if err != nil {
		log.Errorf("Error deleting Service Load Balancer object {%+v}. Err: %v", serviceCfg.ServiceName, err)
		return err
	}
	return nil

}

func validateSelectors(selector string) bool {
	if strings.Count(selector, "=") == 1 {
		return true
	}
	return false
}

func validatePorts(ports []string) bool {
	for _, x := range ports {
		svcPort := strings.Split(x, ":")[0]
		provPort := strings.Split(x, ":")[1]
		protocol := strings.Split(x, ":")[2]

		if provPort == "" || protocol == "" || svcPort == "" {
			return false
		}

		_, err := strconv.Atoi(provPort)
		if err != nil {
			return false
		}
		_, err = strconv.Atoi(svcPort)
		if err != nil {
			return false
		}

		switch protocol {
		case "TCP":
			return true
		case "UDP":
			return true
		default:
			return false
		}
	}
	return true
}
