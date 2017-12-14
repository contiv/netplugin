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
	"fmt"
	"strconv"
	"strings"

	"encoding/json"
	"io/ioutil"
	"net/http"

	contivModel "github.com/contiv/netplugin/contivmodel"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/netmaster/docknet"
	"github.com/contiv/netplugin/netmaster/gstate"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/objdb"
	"github.com/contiv/netplugin/objdb/modeldb"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/netplugin/utils/netutils"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	bgpconf "github.com/osrg/gobgp/config"
)

const (
	// TODO: all need to come from config
	defaultHostPvtNet = "172.19.0.0/16"
	defaultARPMode    = "proxy"
	defaultVLANRange  = "1-4094"
	defaultVXLANRange = "1-10000"
)

// APIController stores the api controller state
type APIController struct {
	router      *mux.Router
	objdbClient objdb.API // Objdb client
}

// APIControllerConfig cofig options for API controller
type APIControllerConfig struct {
	NetPrivateCIDR string
	NetARPMode     string
	NetVLANRange   string
	NetVXLANRange  string
	NetForwardMode string
	NetInfraType   string
}

// BgpInspect is bgp inspect struct
type BgpInspect struct {
	Peers []*bgpconf.Neighbor
	Dsts  []string
}

var apiCtrler *APIController

// NewAPIController creates a new controller
func NewAPIController(router *mux.Router, objdbClient objdb.API, configs *APIControllerConfig) *APIController {
	ctrler := new(APIController)
	ctrler.router = router
	ctrler.objdbClient = objdbClient

	// init modeldb
	modeldb.Init(&objdbClient)

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
	contivModel.RegisterEndpointCallbacks(ctrler)
	contivModel.RegisterNetprofileCallbacks(ctrler)
	contivModel.RegisterAciGwCallbacks(ctrler)
	// Register routes
	contivModel.AddRoutes(router)

	// Init global state from config
	initGlobalConfigs(configs)

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

func initGlobalConfigs(configs *APIControllerConfig) error {
	get := func(candidate, defaultStr string) string {
		if candidate != "" {
			return candidate
		}
		return defaultStr
	}

	globalConfig := contivModel.FindGlobal("global")
	if globalConfig == nil {
		globalConfig = &contivModel.Global{
			Key:  "global",
			Name: "global",
		}
	}
	// set value from config
	globalConfig.NetworkInfraType = configs.NetInfraType
	globalConfig.FwdMode = configs.NetForwardMode
	// set value from config or default value
	// TODO: make them come from config
	globalConfig.ArpMode = get(configs.NetARPMode, defaultARPMode)
	globalConfig.PvtSubnet = get(configs.NetPrivateCIDR, defaultHostPvtNet)
	globalConfig.Vlans = get(configs.NetVLANRange, defaultVLANRange)
	globalConfig.Vxlans = get(configs.NetVXLANRange, defaultVXLANRange)
	// contivModel.CreateGlobal does both creating and updating
	if err := contivModel.CreateGlobal(globalConfig); err != nil {
		return fmt.Errorf("Error creating global state. Err: %v", err.Error())
	}
	log.Infof("Initiated global configs: %+v", globalConfig)
	return nil
}

// GlobalGetOper retrieves glboal operational information
func (ac *APIController) GlobalGetOper(global *contivModel.GlobalInspect) error {
	log.Infof("Received GlobalInspect: %+v", global)

	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	gOper := &gstate.Oper{}
	gOper.StateDriver = stateDriver
	err = gOper.Read("")
	if err != nil {
		log.Errorf("Error obtaining global operational state")
		return err
	}

	global.Oper.DefaultNetwork = gOper.DefaultNetwork
	global.Oper.FreeVXLANsStart = int(gOper.FreeVXLANsStart)

	gCfg := &gstate.Cfg{}
	gCfg.StateDriver = stateDriver
	numVlans, vlansInUse := gCfg.GetVlansInUse()
	numVxlans, vxlansInUse := gCfg.GetVxlansInUse()

	global.Oper.NumNetworks = int(numVlans + numVxlans)
	global.Oper.VlansInUse = vlansInUse
	global.Oper.VxlansInUse = vxlansInUse
	global.Oper.ClusterMode = master.GetClusterMode()
	return nil
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
		FwdMode:     global.FwdMode,
		ArpMode:     global.ArpMode,
		PvtSubnet:   global.PvtSubnet,
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

	gCfg := &gstate.Cfg{}
	gCfg.StateDriver = stateDriver
	numVlans, _ := gCfg.GetVlansInUse()
	numVxlans, _ := gCfg.GetVxlansInUse()

	// Build global config
	globalCfg := intent.ConfigGlobal{}

	// Generate helpful error message when networks exist
	errExistingNetworks := func(optionLabel string) error {
		msgs := []string{}
		if numVlans > 0 {
			msgs = append(msgs, fmt.Sprintf("%d vlans", numVlans))
		}
		if numVxlans > 0 {
			msgs = append(msgs, fmt.Sprintf("%d vxlans", numVxlans))
		}
		msg := fmt.Sprintf("Unable to update %s due to existing %s",
			optionLabel, strings.Join(msgs, " and "))
		log.Errorf(msg)
		return fmt.Errorf(msg)
	}

	//check for change in forwarding mode
	if global.FwdMode != params.FwdMode {
		//check if there exists any non default network and tenants
		if numVlans+numVxlans > 0 {
			return errExistingNetworks("forwarding mode")
		}
		if global.FwdMode == "routing" {
			//check if any bgp configurations exists.
			bgpCfgs := &mastercfg.CfgBgpState{}
			bgpCfgs.StateDriver = stateDriver
			cfgs, _ := bgpCfgs.ReadAll()
			if len(cfgs) != 0 {
				log.Errorf("Unable to change the forwarding mode due to existing bgp configs")
				return fmt.Errorf("please delete existing Bgp configs")
			}
		}
		globalCfg.FwdMode = params.FwdMode
	}
	if global.ArpMode != params.ArpMode {
		globalCfg.ArpMode = params.ArpMode
	}
	if global.Vlans != params.Vlans {
		globalCfg.VLANs = params.Vlans
	}
	if global.Vxlans != params.Vxlans {
		globalCfg.VXLANs = params.Vxlans
	}
	if global.NetworkInfraType != params.NetworkInfraType {
		globalCfg.NwInfraType = params.NetworkInfraType
	}
	if global.PvtSubnet != params.PvtSubnet {
		if (global.PvtSubnet != "" || params.PvtSubnet != defaultHostPvtNet) && numVlans+numVxlans > 0 {
			return errExistingNetworks("private subnet")
		}
		globalCfg.PvtSubnet = params.PvtSubnet
	}

	// Create the object
	err = master.UpdateGlobal(stateDriver, &globalCfg)
	if err != nil {
		log.Errorf("Error creating global config {%+v}. Err: %v", global, err)
		return err
	}

	global.NetworkInfraType = params.NetworkInfraType
	global.Vlans = params.Vlans
	global.Vxlans = params.Vxlans
	global.FwdMode = params.FwdMode
	global.ArpMode = params.ArpMode
	global.PvtSubnet = params.PvtSubnet

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

// AciGwCreate creates aci state
func (ac *APIController) AciGwCreate(aci *contivModel.AciGw) error {
	log.Infof("Received AciGwCreate: %+v", aci)
	// Fail the create if app profiles exist
	profCount := contivModel.GetAppProfileCount()
	if profCount != 0 {
		log.Warnf("AciGwCreate: %d existing App-Profiles found.",
			profCount)
	}

	return nil
}

// AciGwUpdate updates aci state
func (ac *APIController) AciGwUpdate(aci, params *contivModel.AciGw) error {
	log.Infof("Received AciGwUpdate: %+v", params)
	// Fail the update if app profiles exist
	profCount := contivModel.GetAppProfileCount()
	if profCount != 0 {
		log.Warnf("AciGwUpdate: %d existing App-Profiles found.",
			profCount)
	}

	aci.EnforcePolicies = params.EnforcePolicies
	aci.IncludeCommonTenant = params.IncludeCommonTenant
	aci.NodeBindings = params.NodeBindings
	aci.PathBindings = params.PathBindings
	aci.PhysicalDomain = params.PhysicalDomain
	return nil
}

// AciGwDelete deletes aci state
func (ac *APIController) AciGwDelete(aci *contivModel.AciGw) error {
	log.Infof("Received AciGwDelete")
	// Fail the delete if app profiles exist
	profCount := contivModel.GetAppProfileCount()
	if profCount != 0 {
		return core.Errorf("%d App-Profiles found. Delete them first",
			profCount)
	}

	return nil
}

// AciGwGetOper provides operational info for the aci object
func (ac *APIController) AciGwGetOper(op *contivModel.AciGwInspect) error {
	op.Oper.NumAppProfiles = contivModel.GetAppProfileCount()
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

	// Setup links
	modeldb.AddLink(&prof.Links.Tenant, tenant)
	modeldb.AddLinkSet(&tenant.LinkSets.AppProfiles, prof)

	err := tenant.Write()
	if err != nil {
		log.Errorf("Error updating tenant state(%+v). Err: %v", tenant, err)
		return err
	}

	err = CreateAppNw(prof)
	return err
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
	lErr := CreateAppNw(oldProf)
	return lErr
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

// EndpointGetOper retrieves glboal operational information
func (ac *APIController) EndpointGetOper(endpoint *contivModel.EndpointInspect) error {
	log.Infof("Received EndpointInspect: %+v", endpoint)

	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	readEp := &mastercfg.CfgEndpointState{}
	readEp.StateDriver = stateDriver
	// TODO avoid linear read
	epCfgs, err := readEp.ReadAll()
	if err == nil {
		for _, epCfg := range epCfgs {
			ep := epCfg.(*mastercfg.CfgEndpointState)
			if strings.Contains(ep.EndpointID, endpoint.Oper.Key) ||
				strings.Contains(ep.ContainerID, endpoint.Oper.Key) ||
				strings.Contains(ep.EPCommonName, endpoint.Oper.Key) {

				endpoint.Oper.Network = ep.NetID
				endpoint.Oper.EndpointID = ep.EndpointID
				endpoint.Oper.ServiceName = ep.ServiceName
				endpoint.Oper.EndpointGroupID = ep.EndpointGroupID
				endpoint.Oper.EndpointGroupKey = ep.EndpointGroupKey
				endpoint.Oper.IpAddress = []string{ep.IPAddress, ep.IPv6Address}
				endpoint.Oper.MacAddress = ep.MacAddress
				endpoint.Oper.HomingHost = ep.HomingHost
				endpoint.Oper.IntfName = ep.IntfName
				endpoint.Oper.VtepIP = ep.VtepIP
				endpoint.Oper.Labels = fmt.Sprintf("%s", ep.Labels)
				endpoint.Oper.ContainerID = ep.ContainerID
				endpoint.Oper.ContainerName = ep.EPCommonName

				epOper := &drivers.OperEndpointState{}
				epOper.StateDriver = stateDriver
				err := epOper.Read(ep.NetID + "-" + ep.EndpointID)
				if err == nil {
					endpoint.Oper.VirtualPort = "v" + epOper.PortName
				}

				return nil
			}
		}
	}
	return fmt.Errorf("endpoint not found")
}

//GetNetprofileKey gets the netprofile key.
func GetNetprofileKey(tenantName, profileName string) string {
	key := tenantName + ":" + profileName

	return key
}

//GetpolicyKey will return the policy key
func GetpolicyKey(tenantName, policyName string) string {
	key := tenantName + ":" + policyName

	return key
}

// Cleans up state off endpointGroup and related objects.
func endpointGroupCleanup(endpointGroup *contivModel.EndpointGroup) error {
	// delete the endpoint group state
	err := master.DeleteEndpointGroup(endpointGroup.TenantName, endpointGroup.GroupName)
	if err != nil {
		log.Errorf("Error deleting endpoint group %+v. Err: %v", endpointGroup, err)
		return err
	}

	// Detach the endpoint group from the Policies
	for _, policyName := range endpointGroup.Policies {
		policyKey := GetpolicyKey(endpointGroup.TenantName, policyName)

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

	return nil
}

// FIXME: hack to allocate unique endpoint group ids
var globalEpgID = 1

// EndpointGroupCreate creates Endpoint Group
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
	err := master.CreateEndpointGroup(endpointGroup.TenantName, endpointGroup.NetworkName,
		endpointGroup.GroupName, endpointGroup.IpPool, endpointGroup.CfgdTag)
	if err != nil {
		log.Errorf("Error creating endpoint group %+v. Err: %v", endpointGroup, err)
		return err
	}
	// for each policy create an epg policy Instance
	for _, policyName := range endpointGroup.Policies {
		policyKey := GetpolicyKey(endpointGroup.TenantName, policyName)
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

	// If endpoint group is to be attached to any netprofile, then attach the netprofile and create links and linksets.
	if endpointGroup.NetProfile != "" {
		profileKey := GetNetprofileKey(endpointGroup.TenantName, endpointGroup.NetProfile)
		netprofile := contivModel.FindNetprofile(profileKey)
		if netprofile == nil {
			log.Errorf("Error finding netprofile: %s", profileKey)
			return errors.New("netprofile not found")
		}

		// attach NetProfile to epg
		err = master.UpdateEndpointGroup(netprofile.Bandwidth, endpointGroup.GroupName, endpointGroup.TenantName, netprofile.DSCP, netprofile.Burst)
		if err != nil {
			log.Errorf("Error attaching NetProfile %s to epg %s", endpointGroup.NetProfile, endpointGroup.Key)
			endpointGroupCleanup(endpointGroup)
			return err
		}

		//establish links (epg - netprofile)
		modeldb.AddLink(&endpointGroup.Links.NetProfile, netprofile)
		//establish linksets (Netprofile - epg)
		modeldb.AddLinkSet(&netprofile.LinkSets.EndpointGroups, endpointGroup)

		//Write the attached Netprofile to modeldb
		err = netprofile.Write()
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

	if endpointGroup.IpPool != params.IpPool {
		return core.Errorf("Cannot change IP pool after epg is created.")
	}

	// Only update policy attachments

	// Look for policy adds
	for _, policyName := range params.Policies {
		if !stringInSlice(policyName, endpointGroup.Policies) {
			policyKey := GetpolicyKey(endpointGroup.TenantName, policyName)

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

	//for netProfile removal,
	if endpointGroup.NetProfile != "" && params.NetProfile == "" {

		profileKey := GetNetprofileKey(endpointGroup.TenantName, endpointGroup.NetProfile)
		netprofile := contivModel.FindNetprofile(profileKey)
		if netprofile == nil {
			log.Errorf("Error finding netprofile: %s", profileKey)
			return errors.New("netprofile not found")
		}

		// attach NetProfile to epg
		err := master.UpdateEndpointGroup("", endpointGroup.GroupName, endpointGroup.TenantName, 0, 0)
		if err != nil {
			log.Errorf("Error attaching NetProfile %s to epg %s", endpointGroup.NetProfile, endpointGroup.Key)
			endpointGroupCleanup(endpointGroup)
			return err
		}

		modeldb.RemoveLink(&endpointGroup.Links.NetProfile, netprofile)
		modeldb.RemoveLinkSet(&netprofile.LinkSets.EndpointGroups, endpointGroup)

		err = netprofile.Write()
		if err != nil {
			endpointGroupCleanup(endpointGroup)
			return err
		}

		endpointGroup.NetProfile = params.NetProfile

	} else if params.NetProfile != "" {

		paramsKey := GetNetprofileKey(params.TenantName, params.NetProfile)
		netprofile := contivModel.FindNetprofile(paramsKey)
		if netprofile == nil {
			log.Errorf("Error finding netprofile: %s", paramsKey)
			return errors.New("netprofile not found")
		}

		// attach NetProfile to epg
		err := master.UpdateEndpointGroup(netprofile.Bandwidth, endpointGroup.GroupName, endpointGroup.TenantName, netprofile.DSCP, netprofile.Burst)
		if err != nil {
			log.Errorf("Error attaching NetProfile %s to epg %s", params.NetProfile, endpointGroup.Key)
			endpointGroupCleanup(endpointGroup)
			return err
		}

		if endpointGroup.NetProfile != "" && endpointGroup.NetProfile != params.NetProfile {
			// get the epg netprofile object.
			profileKey := GetNetprofileKey(endpointGroup.TenantName, endpointGroup.NetProfile)
			epgnetprofile := contivModel.FindNetprofile(profileKey)
			if epgnetprofile == nil {
				log.Errorf("Error finding netprofile: %s", profileKey)
				return errors.New("netprofile not found")
			}

			//remove links and linksets from the old netprofile.
			modeldb.RemoveLink(&endpointGroup.Links.NetProfile, epgnetprofile)
			modeldb.RemoveLinkSet(&epgnetprofile.LinkSets.EndpointGroups, endpointGroup)

			//add links and linksets to new netprofile.
			modeldb.AddLink(&endpointGroup.Links.NetProfile, netprofile)
			modeldb.AddLinkSet(&netprofile.LinkSets.EndpointGroups, endpointGroup)

		} else if endpointGroup.NetProfile == "" {
			//add links from epg to netprofile and linksets from netprofile to epg
			modeldb.AddLink(&endpointGroup.Links.NetProfile, netprofile)
			modeldb.AddLinkSet(&netprofile.LinkSets.EndpointGroups, endpointGroup)
		}

		err = netprofile.Write()
		if err != nil {
			endpointGroupCleanup(params)
			return err
		}

		endpointGroup.NetProfile = params.NetProfile

	}

	// now look for policy removals
	for _, policyName := range endpointGroup.Policies {
		if !stringInSlice(policyName, params.Policies) {
			policyKey := GetpolicyKey(endpointGroup.TenantName, policyName)

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

			// Remove linksets
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

	if endpointGroup.Links.AppProfile.ObjKey != "" {
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
	}

	return nil
}

// EndpointGroupGetOper inspects endpointGroup
func (ac *APIController) EndpointGroupGetOper(endpointGroup *contivModel.EndpointGroupInspect) error {
	log.Infof("Received EndpointGroupInspect: %+v", endpointGroup)

	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	epgCfg := &mastercfg.EndpointGroupState{}
	epgCfg.StateDriver = stateDriver
	epgID := endpointGroup.Config.GroupName + ":" + endpointGroup.Config.TenantName
	if err := epgCfg.Read(epgID); err != nil {
		log.Errorf("Error fetching endpointGroup from mastercfg: %s", epgID)
		return err
	}

	nwName := epgCfg.NetworkName + "." + epgCfg.TenantName
	nwCfg := &mastercfg.CfgNetworkState{}
	nwCfg.StateDriver = stateDriver
	if err := nwCfg.Read(nwName); err != nil {
		log.Errorf("Error fetching network config %s", nwName)
		return err
	}

	endpointGroup.Oper.ExternalPktTag = epgCfg.ExtPktTag
	endpointGroup.Oper.PktTag = epgCfg.PktTag
	endpointGroup.Oper.NumEndpoints = epgCfg.EpCount
	endpointGroup.Oper.AvailableIPAddresses = netutils.ListAvailableIPs(epgCfg.EPGIPAllocMap,
		nwCfg.SubnetIP, nwCfg.SubnetLen)
	endpointGroup.Oper.AllocatedIPAddresses = netutils.ListAllocatedIPs(epgCfg.EPGIPAllocMap,
		epgCfg.IPPool, nwCfg.SubnetIP, nwCfg.SubnetLen)
	endpointGroup.Oper.GroupTag = epgCfg.GroupTag

	readEp := &mastercfg.CfgEndpointState{}
	readEp.StateDriver = stateDriver
	epCfgs, err := readEp.ReadAll()
	if err == nil {
		for _, epCfg := range epCfgs {
			ep := epCfg.(*mastercfg.CfgEndpointState)
			if ep.EndpointGroupKey == epgID {
				epOper := contivModel.EndpointOper{}
				epOper.Network = ep.NetID
				epOper.EndpointID = ep.EndpointID
				epOper.ServiceName = ep.ServiceName
				epOper.EndpointGroupID = ep.EndpointGroupID
				epOper.EndpointGroupKey = ep.EndpointGroupKey
				epOper.IpAddress = []string{ep.IPAddress, ep.IPv6Address}
				epOper.MacAddress = ep.MacAddress
				epOper.HomingHost = ep.HomingHost
				epOper.IntfName = ep.IntfName
				epOper.VtepIP = ep.VtepIP
				epOper.Labels = fmt.Sprintf("%s", ep.Labels)
				epOper.ContainerID = ep.ContainerID
				epOper.ContainerName = ep.EPCommonName
				endpointGroup.Oper.Endpoints = append(endpointGroup.Oper.Endpoints, epOper)
			}
		}
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

	// In swarm-mode work-flow, if epg is mapped to a docker network, reject the delete
	if master.GetClusterMode() == core.SwarmMode {
		dnet, err := docknet.GetDocknetState(endpointGroup.TenantName, endpointGroup.NetworkName, endpointGroup.GroupName)
		if err == nil {
			return fmt.Errorf("cannot delete group %s mapped to docker network %s",
				endpointGroup.GroupName, dnet.DocknetUUID)
		}
		if !strings.Contains(strings.ToLower(err.Error()), "key not found") {
			log.Errorf("Error getting docknet state for %s.%s. (retval = %s)",
				endpointGroup.TenantName, endpointGroup.GroupName, err.Error())
			return err
		}
		log.Infof("No docknet state for %s.%s. (retval = %s)",
			endpointGroup.TenantName, endpointGroup.GroupName, err.Error())
	}

	// get the netprofile structure by finding the netprofile
	profileKey := GetNetprofileKey(endpointGroup.TenantName, endpointGroup.NetProfile)
	netprofile := contivModel.FindNetprofile(profileKey)

	if netprofile != nil {
		// Remove linksets from netprofile.
		modeldb.RemoveLinkSet(&netprofile.LinkSets.EndpointGroups, endpointGroup)
	}

	err := endpointGroupCleanup(endpointGroup)
	if err != nil {
		log.Errorf("EPG cleanup failed: %+v", err)
	}

	return err

}

// NetworkCreate creates network
func (ac *APIController) NetworkCreate(network *contivModel.Network) error {
	log.Infof("Received NetworkCreate: %+v", network)

	// Make sure global settings is valid
	if err := validateGlobalConfig(network.Encap); err != nil {
		return fmt.Errorf("Global configuration is not ready: %v", err.Error())
	}

	// Make sure tenant exists
	if network.TenantName == "" {
		return core.Errorf("Invalid tenant name")
	}

	tenant := contivModel.FindTenant(network.TenantName)
	if tenant == nil {
		return core.Errorf("Tenant not found")
	}

	for key := range tenant.LinkSets.Networks {
		networkDetail := contivModel.FindNetwork(key)
		if networkDetail == nil {
			log.Errorf("Network key %s not found", key)
			return fmt.Errorf("network key %s not found", key)
		}

		// Check for overlapping subnetv6 if existing and current subnetv6 is non-empty
		if network.Ipv6Subnet != "" && networkDetail.Ipv6Subnet != "" {
			flagv6 := netutils.IsOverlappingSubnetv6(network.Ipv6Subnet, networkDetail.Ipv6Subnet)
			if flagv6 == true {
				log.Errorf("Overlapping of Subnetv6 Networks")
				return errors.New("network " + networkDetail.NetworkName + " conflicts with subnetv6  " + network.Ipv6Subnet)
			}
		}

		// Check for overlapping subnet if existing and current subnet is non-empty
		if network.Subnet != "" && networkDetail.Subnet != "" {
			flag := netutils.IsOverlappingSubnet(network.Subnet, networkDetail.Subnet)
			if flag == true {
				log.Errorf("Overlapping of Networks")
				return errors.New("network " + networkDetail.NetworkName + " conflicts with subnet " + network.Subnet)
			}
		}
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
		Name:           network.NetworkName,
		NwType:         network.NwType,
		PktTagType:     network.Encap,
		PktTag:         network.PktTag,
		SubnetCIDR:     network.Subnet,
		Gateway:        network.Gateway,
		IPv6SubnetCIDR: network.Ipv6Subnet,
		IPv6Gateway:    network.Ipv6Gateway,
		CfgdTag:        network.CfgdTag,
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

// NetworkGetOper inspects network
func (ac *APIController) NetworkGetOper(network *contivModel.NetworkInspect) error {
	log.Infof("Received NetworkInspect: %+v", network)

	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	nwCfg := &mastercfg.CfgNetworkState{}
	nwCfg.StateDriver = stateDriver
	networkID := network.Config.NetworkName + "." + network.Config.TenantName
	if err := nwCfg.Read(networkID); err != nil {
		log.Errorf("Error fetching network from mastercfg: %s", networkID)
		return err
	}

	network.Oper.AllocatedAddressesCount = nwCfg.EpAddrCount
	network.Oper.AvailableIPAddresses = master.ListAvailableIPs(nwCfg)
	network.Oper.AllocatedIPAddresses = master.ListAllocatedIPs(nwCfg)
	network.Oper.ExternalPktTag = nwCfg.ExtPktTag
	network.Oper.NumEndpoints = nwCfg.EpCount
	network.Oper.PktTag = nwCfg.PktTag
	network.Oper.NetworkTag = nwCfg.NetworkTag

	readEp := &mastercfg.CfgEndpointState{}
	readEp.StateDriver = stateDriver
	epCfgs, err := readEp.ReadAll()
	if err == nil {
		for _, epCfg := range epCfgs {
			ep := epCfg.(*mastercfg.CfgEndpointState)
			if ep.NetID == networkID {
				epOper := contivModel.EndpointOper{}
				epOper.Network = ep.NetID
				epOper.EndpointID = ep.EndpointID
				epOper.ServiceName = ep.ServiceName
				epOper.EndpointGroupID = ep.EndpointGroupID
				epOper.EndpointGroupKey = ep.EndpointGroupKey
				epOper.IpAddress = []string{ep.IPAddress, ep.IPv6Address}
				epOper.MacAddress = ep.MacAddress
				epOper.HomingHost = ep.HomingHost
				epOper.IntfName = ep.IntfName
				epOper.VtepIP = ep.VtepIP
				epOper.Labels = fmt.Sprintf("%s", ep.Labels)
				epOper.ContainerID = ep.ContainerID
				epOper.ContainerName = ep.EPCommonName
				network.Oper.Endpoints = append(network.Oper.Endpoints, epOper)
			}
		}
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

	svcCount := len(network.LinkSets.Servicelbs)
	if svcCount != 0 {
		return core.Errorf("cannot delete %s has %d services ",
			network.NetworkName, svcCount)
	}

	// In swarm-mode work-flow, if this is mapped to a docker network, reject delete
	if master.GetClusterMode() == core.SwarmMode {
		docknet, err := docknet.GetDocknetState(network.TenantName, network.NetworkName, "")
		if err == nil {
			return fmt.Errorf("cannot delete network %s mapped to docker network %s",
				network.NetworkName, docknet.DocknetUUID)
		}
		if !strings.Contains(strings.ToLower(err.Error()), "key not found") {
			log.Errorf("Error getting docknet state for %s.%s. (retval = %s)",
				network.TenantName, network.NetworkName, err.Error())
			return err
		}
		log.Infof("No docknet state for %s.%s. (retval = %s)",
			network.TenantName, network.NetworkName, err.Error())
	}

	// Remove link
	modeldb.RemoveLinkSet(&tenant.LinkSets.Networks, network)

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

	// Save the tenant too since we removed the links
	return tenant.Write()
}

// NetprofileCreate creates the network rule
func (ac *APIController) NetprofileCreate(netProfile *contivModel.Netprofile) error {
	log.Infof("Received NetprofileCreate: %+v", netProfile)

	// Check if the tenant exists
	if netProfile.TenantName == "" {
		return core.Errorf("Invalid tenant name")
	}

	if netProfile.Burst > 0 && netProfile.Burst < 2 {
		return core.Errorf("Invalid Burst size. burst size > 1500 bytes")
	}

	tenant := contivModel.FindTenant(netProfile.TenantName)
	if tenant == nil {
		return core.Errorf("Tenant not found")
	}

	// Setup links & Linksets.
	modeldb.AddLink(&netProfile.Links.Tenant, tenant)
	modeldb.AddLinkSet(&tenant.LinkSets.NetProfiles, netProfile)

	// Save the tenant in etcd - This writes to etcd.
	err := tenant.Write()
	if err != nil {
		log.Errorf("Error updating tenant state(%+v). Err: %v", tenant, err)
		return err
	}

	return nil
}

// NetprofileUpdate updates the netprofile
func (ac *APIController) NetprofileUpdate(profile, params *contivModel.Netprofile) error {
	log.Infof("Received NetprofileUpdate: %+v, params: %+v", profile, params)

	if params.Burst > 0 && params.Burst < 2 {
		return core.Errorf("Invalid Burst size. burst size must be > 1500 bytes")
	}
	profile.Bandwidth = params.Bandwidth
	profile.DSCP = params.DSCP
	profile.Burst = params.Burst

	for key := range profile.LinkSets.EndpointGroups {
		// Find the corresponding epg
		epg := contivModel.FindEndpointGroup(key)
		if epg == nil {
			return core.Errorf("EndpointGroups not found")
		}

		err := master.UpdateEndpointGroup(params.Bandwidth, epg.GroupName, epg.TenantName, params.DSCP, params.Burst)
		if err != nil {
			log.Errorf("Error updating the EndpointGroups: %s. Err: %v", epg.GroupName, err)
		}
	}
	return nil
}

// NetprofileDelete deletes netprofile
func (ac *APIController) NetprofileDelete(netProfile *contivModel.Netprofile) error {
	log.Infof("Deleting Netprofile:%s", netProfile.ProfileName)

	// Find Tenant
	tenant := contivModel.FindTenant(netProfile.TenantName)
	if tenant == nil {
		return core.Errorf("Tenant %s not found", netProfile.TenantName)
	}
	// Check if any endpoint group is using the network policy
	if len(netProfile.LinkSets.EndpointGroups) != 0 {
		return core.Errorf("NetProfile is being used")
	}

	modeldb.RemoveLinkSet(&tenant.LinkSets.NetProfiles, netProfile)
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

// PolicyGetOper inspects policy
func (ac *APIController) PolicyGetOper(policy *contivModel.PolicyInspect) error {
	log.Infof("Received PolicyInspect: %+v", policy)

	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	// To hold total number of Endpoint count
	var policyEPCount int

	policyCfg := &mastercfg.EpgPolicy{}
	policyCfg.StateDriver = stateDriver

	// Policy is attached to EPG. So we need to fetch EPGs as well.
	epgCfg := &mastercfg.EndpointGroupState{}
	epgCfg.StateDriver = stateDriver

	// Get all the Endpoints
	readEp := &mastercfg.CfgEndpointState{}
	readEp.StateDriver = stateDriver
	epCfgs, epErr := readEp.ReadAll()

	// Get all the EPGs on which this policy is applied
	epgs := policy.Config.LinkSets.EndpointGroups

	// Scan all the EPGs which are under this policy
	for _, epg := range epgs {
		log.Infof("EPG Object : %+v", epg)

		// Reversing key from TenantName:EPGName to EPGName:TenantName
		sList := strings.Split(epg.ObjKey, ":")
		if sList == nil {
			log.Errorf("EPG key %+v is not in valid format", epg.ObjKey)
			return err
		}
		epgID := sList[1] + ":" + sList[0]

		log.Infof("EPG ID : %s", epgID)

		if err := epgCfg.Read(epgID); err != nil {
			log.Errorf("Error fetching endpointGroup from mastercfg: %s", epgID)
			return err
		}

		policyEPCount = policyEPCount + epgCfg.EpCount

		if epErr == nil {
			for _, epCfg := range epCfgs {
				ep := epCfg.(*mastercfg.CfgEndpointState)
				if ep.EndpointGroupKey == epgID {
					epOper := contivModel.EndpointOper{}
					epOper.Network = ep.NetID
					epOper.EndpointID = ep.EndpointID
					epOper.ServiceName = ep.ServiceName
					epOper.EndpointGroupID = ep.EndpointGroupID
					epOper.EndpointGroupKey = ep.EndpointGroupKey
					epOper.IpAddress = []string{ep.IPAddress, ep.IPv6Address}
					epOper.MacAddress = ep.MacAddress
					epOper.HomingHost = ep.HomingHost
					epOper.IntfName = ep.IntfName
					epOper.VtepIP = ep.VtepIP
					epOper.Labels = fmt.Sprintf("%s", ep.Labels)
					epOper.ContainerID = ep.ContainerID
					epOper.ContainerName = ep.EPCommonName
					policy.Oper.Endpoints = append(policy.Oper.Endpoints, epOper)
				}
			}
		}
	} // End of main for loop

	policy.Oper.NumEndpoints = policyEPCount

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

	// Find Tenant
	tenant := contivModel.FindTenant(policy.TenantName)
	if tenant == nil {
		return core.Errorf("Tenant %s not found", policy.TenantName)
	}

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

	//Remove Links
	modeldb.RemoveLinkSet(&tenant.LinkSets.Policies, policy)

	// Save the tenant too since we added the links
	err := tenant.Write()
	if err != nil {
		log.Errorf("Error updating tenant state(%+v). Err: %v", tenant, err)
		return err
	}

	return nil
}

func getAffectedProfs(policy *contivModel.Policy,
	matchEpg *contivModel.EndpointGroup) map[string]bool {
	profMap := make(map[string]bool)
	// find all appProfiles that have an association via policy
	for epg := range policy.LinkSets.EndpointGroups {
		epgObj := contivModel.FindEndpointGroup(epg)
		if epgObj == nil {
			log.Warnf("syncAppProfile epg %s not found", epg)
		} else {
			prof := epgObj.Links.AppProfile.ObjKey
			if prof != "" {
				profMap[prof] = true
				log.Infof("syncAppProfile epg %s ==> prof %s", epg, prof)
			}
		}
	}

	// add any app-profile associated via a matching epg
	if matchEpg != nil {
		prof := matchEpg.Links.AppProfile.ObjKey
		if prof != "" {
			profMap[prof] = true
			log.Infof("syncAppProfile epg %s ==> prof %s",
				matchEpg, prof)
		}
	}

	return profMap
}

func syncAppProfile(profMap map[string]bool) {
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
	var epg *contivModel.EndpointGroup
	epg = nil

	// verify parameter values
	if rule.Direction == "in" {
		if rule.ToNetwork != "" || rule.ToEndpointGroup != "" {
			return errors.New("can not specify 'to' parameters in incoming rule")
		}
		if rule.FromNetwork != "" && rule.FromIpAddress != "" {
			return errors.New("can not specify both from network and from ip address")
		}

		if rule.FromNetwork != "" && rule.FromEndpointGroup != "" {
			return errors.New("can not specify both from network and from EndpointGroup")
		}
	} else if rule.Direction == "out" {
		if rule.FromNetwork != "" || rule.FromEndpointGroup != "" || rule.FromIpAddress != "" {
			return errors.New("can not specify 'from' parameters in outgoing rule")
		}
		if rule.ToNetwork != "" && rule.ToIpAddress != "" {
			return errors.New("can not specify both to-network and to-ip address")
		}
		if rule.ToNetwork != "" && rule.ToEndpointGroup != "" {
			return errors.New("can not specify both to-network and to-EndpointGroup")
		}
	} else {
		return errors.New("invalid direction for the rule")
	}

	// Make sure endpoint groups and networks referred exists.
	if rule.FromEndpointGroup != "" {
		epgKey := rule.TenantName + ":" + rule.FromEndpointGroup
		// find the endpoint group
		epg = contivModel.FindEndpointGroup(epgKey)
		if epg == nil {
			log.Errorf("Error finding endpoint group %s", epgKey)
			return errors.New("endpoint group not found")
		}
	} else if rule.ToEndpointGroup != "" {
		epgKey := rule.TenantName + ":" + rule.ToEndpointGroup

		// find the endpoint group
		epg = contivModel.FindEndpointGroup(epgKey)
		if epg == nil {
			log.Errorf("Error finding endpoint group %s", epgKey)
			return errors.New("endpoint group not found")
		}
	} else if rule.FromNetwork != "" {
		netKey := rule.TenantName + ":" + rule.FromNetwork

		net := contivModel.FindNetwork(netKey)
		if net == nil {
			log.Errorf("Network %s not found", netKey)
			return errors.New("from Network not found")
		}
	} else if rule.ToNetwork != "" {
		netKey := rule.TenantName + ":" + rule.ToNetwork

		net := contivModel.FindNetwork(netKey)
		if net == nil {
			log.Errorf("Network %s not found", netKey)
			return errors.New("to Network not found")
		}
	}

	policyKey := GetpolicyKey(rule.TenantName, rule.PolicyName)

	// find the policy
	policy := contivModel.FindPolicy(policyKey)
	if policy == nil {
		log.Errorf("Error finding policy %s", policyKey)
		return core.Errorf("Policy not found")
	}

	if rule.Direction == "in" && rule.ToIpAddress != "" {
		// rules from k8s network policy
		// verify 'toIpAddress' is part of epg and subnet
		if len(policy.LinkSets.EndpointGroups) != 1 {
			errMsg := fmt.Errorf("failed to configure %s, %d endpoint groups linked to policy %s ",
				rule.ToIpAddress,
				len(policy.LinkSets.EndpointGroups),
				rule.PolicyName)
			log.Error(errMsg)
			return errMsg
		}
		epgKey := ""
		for key := range policy.LinkSets.EndpointGroups {
			epgKey = strings.Replace(key, rule.TenantName+":", "", 1) + ":" + rule.TenantName
		}

		stateDriver, err := utils.GetStateDriver()
		if err != nil {
			log.Errorf("failed to configure %s, %s", rule.ToIpAddress, err)
			return fmt.Errorf("failed to configure %s, unable to connect to key-value store",
				rule.ToIpAddress)
		}
		readEp := &mastercfg.CfgEndpointState{}
		readEp.StateDriver = stateDriver
		epCfgs, err := readEp.ReadAll()
		if err != nil {
			log.Errorf("failed to configure %s, %s", rule.ToIpAddress, err)
			return fmt.Errorf("failed to configure %s, unable to read endpoint state", rule.ToIpAddress)
		}

		// TODO: cache ip <--> epg for performance
		if func(epgName string) bool {
			for _, epCfg := range epCfgs {
				if ep, ok := epCfg.(*mastercfg.CfgEndpointState); ok {
					if ep.IPAddress == rule.ToIpAddress && ep.EndpointGroupKey == epgName {
						return true
					}
				}
			}
			return false
		}(epgKey) != true {
			errMsg := fmt.Errorf("failed to configure %s, ip address is not in epg %s",
				rule.ToIpAddress,
				epgKey)
			log.Error(errMsg)
			return errMsg
		}
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

	// link the rule to epg and vice versa
	if epg != nil {
		modeldb.AddLinkSet(&epg.LinkSets.MatchRules, rule)
		modeldb.AddLink(&rule.Links.MatchEndpointGroup, epg)
		err = epg.Write()
		if err != nil {
			return err
		}
	}

	// Update any affected app profiles
	pMap := getAffectedProfs(policy, epg)
	syncAppProfile(pMap)

	return nil
}

// RuleUpdate updates the rule within a policy
func (ac *APIController) RuleUpdate(rule, params *contivModel.Rule) error {
	log.Infof("Received RuleUpdate: %+v, params: %+v", rule, params)
	return errors.New("can not update a rule after its created")
}

// RuleDelete deletes the rule within a policy
func (ac *APIController) RuleDelete(rule *contivModel.Rule) error {
	var epg *contivModel.EndpointGroup

	epg = nil
	log.Infof("Received RuleDelete: %+v", rule)

	policyKey := GetpolicyKey(rule.TenantName, rule.PolicyName)

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

	// unlink the rule from matching epg
	epgKey := rule.Links.MatchEndpointGroup.ObjKey
	if epgKey != "" {
		epg = contivModel.FindEndpointGroup(epgKey)
		if epg != nil {
			modeldb.RemoveLinkSet(&epg.LinkSets.MatchRules, rule)
		}
	}

	// Trigger policyDB Update
	err = master.PolicyDelRule(policy, rule)
	if err != nil {
		log.Errorf("Error deleting rule %s to policy %s. Err: %v", rule.Key, policy.Key, err)
		return err
	}

	// Update any affected app profiles
	pMap := getAffectedProfs(policy, epg)
	syncAppProfile(pMap)

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

// Get all the networks inside tenant
func getTenantNetworks(tenant *contivModel.TenantInspect) error {

	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	tenantID := tenant.Config.TenantName
	numEPs := 0
	s := []string{}
	networkID := ""
	for _, net := range tenant.Config.LinkSets.Networks {
		networkID = net.ObjKey
		log.Infof("network has ID %s", networkID)
		s = strings.Split(networkID, ":")
		if s[0] == tenantID {
			networkID = s[1] + "." + s[0]
			nwCfg := &mastercfg.CfgNetworkState{}
			nwCfg.StateDriver = stateDriver
			if err := nwCfg.Read(networkID); err != nil {
				log.Errorf("Error fetching network from mastercfg: %s", networkID)
				return err
			}
			numEPs = numEPs + nwCfg.EpCount
			netOper := contivModel.NetworkOper{}
			netOper.AllocatedAddressesCount = nwCfg.EpAddrCount
			netOper.AvailableIPAddresses = master.ListAvailableIPs(nwCfg)
			netOper.AllocatedIPAddresses = master.ListAllocatedIPs(nwCfg)
			netOper.ExternalPktTag = nwCfg.ExtPktTag
			netOper.PktTag = nwCfg.PktTag
			netOper.NumEndpoints = nwCfg.EpCount
			readEp := &mastercfg.CfgEndpointState{}
			readEp.StateDriver = stateDriver
			epCfgs, err := readEp.ReadAll()
			if err == nil {
				for _, epCfg := range epCfgs {
					ep := epCfg.(*mastercfg.CfgEndpointState)
					if ep.NetID == networkID {
						epOper := contivModel.EndpointOper{}
						epOper.Network = ep.NetID
						epOper.EndpointID = ep.EndpointID
						epOper.ServiceName = ep.ServiceName
						epOper.EndpointGroupID = ep.EndpointGroupID
						epOper.EndpointGroupKey = ep.EndpointGroupKey
						epOper.IpAddress = []string{ep.IPAddress, ep.IPv6Address}
						epOper.MacAddress = ep.MacAddress
						epOper.HomingHost = ep.HomingHost
						epOper.IntfName = ep.IntfName
						epOper.VtepIP = ep.VtepIP
						epOper.Labels = fmt.Sprintf("%s", ep.Labels)
						epOper.ContainerID = ep.ContainerID
						epOper.ContainerName = ep.EPCommonName
						netOper.Endpoints = append(netOper.Endpoints, epOper)
					}
				}
			}
			tenant.Oper.Networks = append(tenant.Oper.Networks, netOper)
		}
	}

	tenant.Oper.TotalEndpoints = numEPs
	return nil
}

// Get all the EPGs inside tenant
func getTenantEPGs(tenant *contivModel.TenantInspect) error {

	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	tenantID := tenant.Config.TenantName
	s := []string{}
	epgID := ""
	for _, epg := range tenant.Config.LinkSets.EndpointGroups {
		epgID = epg.ObjKey
		log.Infof("EPG ID is  %s", epgID)
		s = strings.Split(epgID, ":")
		if s[0] == tenantID {
			epgID = s[1] + ":" + s[0]
			epgCfg := &mastercfg.EndpointGroupState{}
			epgCfg.StateDriver = stateDriver
			if err := epgCfg.Read(epgID); err != nil {
				log.Errorf("Error fetching epg from mastercfg: %s", epgID)
				return err
			}
			epgOper := contivModel.EndpointGroupOper{}
			epgOper.ExternalPktTag = epgCfg.ExtPktTag
			epgOper.PktTag = epgCfg.PktTag
			epgOper.NumEndpoints = epgCfg.EpCount

			readEp := &mastercfg.CfgEndpointState{}
			readEp.StateDriver = stateDriver
			epCfgs, err := readEp.ReadAll()
			if err == nil {
				for _, epCfg := range epCfgs {
					ep := epCfg.(*mastercfg.CfgEndpointState)
					log.Infof("EndpointGroupKey is %s", ep.EndpointGroupKey)
					if ep.EndpointGroupKey == epgID {
						epOper := contivModel.EndpointOper{}
						epOper.Network = ep.NetID
						epOper.EndpointID = ep.EndpointID
						epOper.ServiceName = ep.ServiceName
						epOper.EndpointGroupID = ep.EndpointGroupID
						epOper.EndpointGroupKey = ep.EndpointGroupKey
						epOper.IpAddress = []string{ep.IPAddress, ep.IPv6Address}
						epOper.MacAddress = ep.MacAddress
						epOper.HomingHost = ep.HomingHost
						epOper.IntfName = ep.IntfName
						epOper.VtepIP = ep.VtepIP
						epOper.Labels = fmt.Sprintf("%s", ep.Labels)
						epOper.ContainerID = ep.ContainerID
						epOper.ContainerName = ep.EPCommonName
						epgOper.Endpoints = append(epgOper.Endpoints, epOper)
					}
				}
			}
			tenant.Oper.EndpointGroups = append(tenant.Oper.EndpointGroups, epgOper)
		}
	}

	return nil
}

// TenantGetOper inspects tenant
func (ac *APIController) TenantGetOper(tenant *contivModel.TenantInspect) error {
	log.Infof("Received TenantInspect: %+v", tenant)

	tenant.Oper.TotalNetworks = len(tenant.Config.LinkSets.Networks)
	tenant.Oper.TotalEPGs = len(tenant.Config.LinkSets.EndpointGroups)
	tenant.Oper.TotalNetprofiles = len(tenant.Config.LinkSets.NetProfiles)
	tenant.Oper.TotalPolicies = len(tenant.Config.LinkSets.Policies)
	tenant.Oper.TotalAppProfiles = len(tenant.Config.LinkSets.AppProfiles)
	tenant.Oper.TotalServicelbs = len(tenant.Config.LinkSets.Servicelbs)

	//Get all the networks config and oper parmeters under this tenant
	getTenantNetworks(tenant)

	//Get all the EPGs config and oper parmeters under this tenant
	getTenantEPGs(tenant)

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

	// if the tenant has associated app profiles, fail the delete
	profCount := len(tenant.LinkSets.AppProfiles)
	if profCount != 0 {
		return core.Errorf("cannot delete %s, has %d app profiles",
			tenant.TenantName, profCount)
	}
	// if the tenant has associated epgs, fail the delete
	epgCount := len(tenant.LinkSets.EndpointGroups)
	if epgCount != 0 {
		return core.Errorf("cannot delete %s has %d endpoint groups",
			tenant.TenantName, epgCount)
	}
	// if the tenant has associated policies, fail the delete
	policyCount := len(tenant.LinkSets.Policies)
	if policyCount != 0 {
		return core.Errorf("cannot delete %s has %d policies",
			tenant.TenantName, policyCount)
	}
	npCount := len(tenant.LinkSets.NetProfiles)
	if npCount != 0 {
		return core.Errorf("Cannot delete %s has %d netprofiles", tenant.TenantName, npCount)
	}
	// if the tenant has associated networks, fail the delete
	nwCount := len(tenant.LinkSets.Networks)
	if nwCount != 0 {
		return core.Errorf("cannot delete %s has %d networks",
			tenant.TenantName, nwCount)
	}

	// Build tenant config
	tenantCfg := intent.ConfigTenant{
		Name:           tenant.TenantName,
		DefaultNetwork: tenant.DefaultNetwork,
	}

	// Delete the tenant
	err = master.DeleteTenant(stateDriver, &tenantCfg)
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

	oldbgpCfg.Hostname = NewbgpCfg.Hostname
	oldbgpCfg.Routerip = NewbgpCfg.Routerip
	oldbgpCfg.As = NewbgpCfg.As
	oldbgpCfg.NeighborAs = NewbgpCfg.NeighborAs
	oldbgpCfg.Neighbor = NewbgpCfg.Neighbor

	NewbgpCfg.Write()

	return nil
}

//BgpGetOper inspects the oper state of bgp object
func (ac *APIController) BgpGetOper(bgp *contivModel.BgpInspect) error {
	var obj BgpInspect
	var host string

	srvList, err := ac.objdbClient.GetService("netplugin")
	if err != nil {
		log.Errorf("Error getting netplugin nodes. Err: %v", err)
		return err
	}

	for _, srv := range srvList {
		if srv.Hostname == bgp.Config.Hostname {
			host = srv.HostAddr
		}
	}

	url := "http://" + host + ":9090/inspect/bgp"
	r, err := http.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	switch {
	case r.StatusCode == int(404):
		return errors.New("page not found")
	case r.StatusCode == int(403):
		return errors.New("access denied")
	case r.StatusCode == int(500):
		response, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return err
		}
		return errors.New(string(response))
	case r.StatusCode != int(200):
		log.Debugf("GET Status '%s' status code %d \n", r.Status, r.StatusCode)
		return errors.New(r.Status)
	}

	response, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(response, &obj); err != nil {
		return err
	}
	//Assuming bgp peer state will be only for one neighbor
	if obj.Peers != nil {
		nConf := obj.Peers[0]
		bgp.Oper.NeighborStatus = string(nConf.State.SessionState)
		bgp.Oper.AdminStatus = nConf.State.AdminState
	}

	if obj.Dsts != nil {
		for _, dst := range obj.Dsts {
			bgp.Oper.Routes = append(bgp.Oper.Routes, dst)
		}
		bgp.Oper.NumRoutes = len(bgp.Oper.Routes)
	}

	return nil
}

//ServiceLBCreate creates service object
func (ac *APIController) ServiceLBCreate(serviceCfg *contivModel.ServiceLB) error {

	log.Infof("Received Service Load Balancer create: %+v", serviceCfg)

	if serviceCfg.ServiceName == "" {
		return core.Errorf("Invalid service name")
	}

	if len(serviceCfg.Selectors) == 0 {
		return core.Errorf("Invalid selector options")
	}

	if !validatePorts(serviceCfg.Ports) {
		return core.Errorf("Invalid Port maping . Port format is - Port:TargetPort:Protocol")
	}

	if serviceCfg.TenantName == "" {
		return core.Errorf("Invalid tenant name")
	}

	tenant := contivModel.FindTenant(serviceCfg.TenantName)
	if tenant == nil {
		return core.Errorf("Tenant %s not found", serviceCfg.TenantName)
	}

	network := contivModel.FindNetwork(serviceCfg.TenantName + ":" + serviceCfg.NetworkName)
	if network == nil {
		return core.Errorf("Network %s not found", serviceCfg.NetworkName)
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
	// Setup links
	if tenant != nil {
		modeldb.AddLink(&serviceCfg.Links.Tenant, tenant)
		modeldb.AddLinkSet(&tenant.LinkSets.Servicelbs, serviceCfg)
		tenant.Write()
	}

	// Setup links
	if network != nil {
		modeldb.AddLink(&serviceCfg.Links.Network, network)
		modeldb.AddLinkSet(&network.LinkSets.Servicelbs, serviceCfg)
		network.Write()
	}
	return nil

}

//ServiceLBUpdate updates service object
func (ac *APIController) ServiceLBUpdate(oldServiceCfg *contivModel.ServiceLB, serviceCfg *contivModel.ServiceLB) error {
	log.Infof("Received Service Load Balancer update: %+v", serviceCfg)
	err := ac.ServiceLBCreate(serviceCfg)
	if err != nil {
		return err
	}
	oldServiceCfg.ServiceName = serviceCfg.ServiceName
	oldServiceCfg.TenantName = serviceCfg.TenantName
	oldServiceCfg.NetworkName = serviceCfg.NetworkName
	oldServiceCfg.IpAddress = serviceCfg.IpAddress
	oldServiceCfg.Selectors = nil
	oldServiceCfg.Ports = nil
	oldServiceCfg.Selectors = append(oldServiceCfg.Selectors, serviceCfg.Selectors...)
	oldServiceCfg.Ports = append(oldServiceCfg.Ports, serviceCfg.Ports...)
	return nil
}

//ServiceLBDelete deletes service object
func (ac *APIController) ServiceLBDelete(serviceCfg *contivModel.ServiceLB) error {

	log.Info("Received Service Load Balancer delete : {%+v}", serviceCfg)

	if serviceCfg.ServiceName == "" {
		return core.Errorf("Invalid service name")
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
	// Find the tenant
	tenant := contivModel.FindTenant(serviceCfg.TenantName)
	if tenant == nil {
		return core.Errorf("Tenant %s not found", serviceCfg.TenantName)
	}

	modeldb.RemoveLinkSet(&tenant.LinkSets.Servicelbs, serviceCfg)
	tenant.Write()

	nwKey := serviceCfg.TenantName + ":" + serviceCfg.NetworkName
	network := contivModel.FindNetwork(nwKey)
	if network == nil {
		return core.Errorf("Network %s not found in tenant %s", serviceCfg.NetworkName, serviceCfg.TenantName)
	}
	modeldb.RemoveLinkSet(&network.LinkSets.Servicelbs, serviceCfg)
	network.Write()

	return nil

}

//ServiceLBGetOper inspects the oper state of service lb object
func (ac *APIController) ServiceLBGetOper(serviceLB *contivModel.ServiceLBInspect) error {
	log.Infof("Received Service load balancer inspect : %+v", serviceLB)

	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}
	serviceID := master.GetServiceID(serviceLB.Config.ServiceName, serviceLB.Config.TenantName)
	service := mastercfg.ServiceLBDb[serviceID]
	if service == nil {
		return errors.New("invalid Service name. Oper state does not exist")
	}
	serviceLB.Oper.ServiceVip = service.IPAddress
	count := 0
	for _, provider := range service.Providers {

		epCfg := &mastercfg.CfgEndpointState{}
		epCfg.StateDriver = stateDriver
		err := epCfg.Read(provider.EpIDKey)
		if err != nil {
			continue
		}
		epOper := contivModel.EndpointOper{}
		epOper.Network = epCfg.NetID
		epOper.EndpointID = epCfg.EndpointID
		epOper.ServiceName = service.ServiceName //FIXME:fill in service name in endpoint
		epOper.EndpointGroupID = epCfg.EndpointGroupID
		epOper.EndpointGroupKey = epCfg.EndpointGroupKey
		epOper.IpAddress = []string{epCfg.IPAddress, epCfg.IPv6Address}
		epOper.MacAddress = epCfg.MacAddress
		epOper.HomingHost = epCfg.HomingHost
		epOper.IntfName = epCfg.IntfName
		epOper.VtepIP = epCfg.VtepIP
		epOper.Labels = fmt.Sprintf("%s", epCfg.Labels)
		epOper.ContainerID = epCfg.ContainerID
		epOper.ContainerName = epCfg.EPCommonName
		serviceLB.Oper.Providers = append(serviceLB.Oper.Providers, epOper)
		count++
		epCfg = nil
	}
	serviceLB.Oper.NumProviders = count
	return nil

}

func validateSelectors(selector string) bool {
	return strings.Count(selector, "=") == 1
}

func validatePorts(ports []string) bool {

	if len(ports) == 0 {
		return false
	}
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
		case "TCP", "tcp", "Tcp":
			return true
		case "UDP", "udp", "Udp":
			return true
		default:
			return false
		}
	}
	return true
}

func validateGlobalConfig(netmode string) error {
	globalConfig := contivModel.FindGlobal("global")
	if globalConfig == nil {
		return errors.New("global configuration is not ready")
	}
	if globalConfig.FwdMode == "" {
		return errors.New("global forwarding mode is not set")
	}
	if strings.ToLower(netmode) == "vlan" && globalConfig.Vlans == "" {
		return errors.New("global vlan range is not set")
	}
	if strings.ToLower(netmode) == "vxlan" && globalConfig.Vxlans == "" {
		return errors.New("global vxlan range is not set")
	}
	return nil
}
