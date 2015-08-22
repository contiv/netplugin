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

	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/objmodel/contivModel"
	"github.com/contiv/objmodel/objdb/modeldb"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
)

// APIController stores the api controller state
type APIController struct {
	router *mux.Router
}

var apiCtrler *APIController

// NewAPIController creates a new controller
func NewAPIController(router *mux.Router) *APIController {
	ctrler := new(APIController)
	ctrler.router = router

	// initialize the model objects
	contivModel.Init()

	// Register Callbacks
	contivModel.RegisterAppCallbacks(ctrler)
	contivModel.RegisterEndpointGroupCallbacks(ctrler)
	contivModel.RegisterNetworkCallbacks(ctrler)
	contivModel.RegisterPolicyCallbacks(ctrler)
	contivModel.RegisterRuleCallbacks(ctrler)
	contivModel.RegisterServiceCallbacks(ctrler)
	contivModel.RegisterServiceInstanceCallbacks(ctrler)
	contivModel.RegisterTenantCallbacks(ctrler)
	contivModel.RegisterVolumeCallbacks(ctrler)
	contivModel.RegisterVolumeProfileCallbacks(ctrler)

	// Register routes
	contivModel.AddRoutes(router)

	// Add default tenant if it doesnt exist
	tenant := contivModel.FindTenant("default")
	if tenant == nil {
		log.Infof("Creating default tenant")
		err := contivModel.CreateTenant(&contivModel.Tenant{
			Key:        "default",
			TenantName: "default",
			SubnetPool: "10.1.1.1/16",
			SubnetLen:  24,
			Vlans:      "1-2000",
			Vxlans:     "10001-12000",
		})
		if err != nil {
			log.Fatalf("Error creating default tenant. Err: %v", err)
		}
	}

	return ctrler
}

// AppCreate creates app state
func (ac *APIController) AppCreate(app *contivModel.App) error {
	log.Infof("Received AppCreate: %+v", app)

	// Make sure tenant exists
	if app.TenantName == "" {
		return errors.New("Invalid tenant name")
	}

	tenant := contivModel.FindTenant(app.TenantName)
	if tenant == nil {
		return errors.New("Tenant not found")
	}

	// Setup links
	modeldb.AddLink(&app.Links.Tenant, tenant)
	modeldb.AddLinkSet(&tenant.LinkSets.Apps, app)

	// Save the tenant too since we added the links
	err := tenant.Write()
	if err != nil {
		log.Errorf("Error updating tenant state(%+v). Err: %v", tenant, err)
		return err
	}

	return nil
}

// AppUpdate updates app
func (ac *APIController) AppUpdate(app, params *contivModel.App) error {
	log.Infof("Received AppUpdate: %+v, params: %+v", app, params)

	return nil
}

// AppDelete delete the app
func (ac *APIController) AppDelete(app *contivModel.App) error {
	log.Infof("Received AppDelete: %+v", app)
	return nil
}

// EndpointGroupCreate creates end point group
func (ac *APIController) EndpointGroupCreate(endpointGroup *contivModel.EndpointGroup) error {
	log.Infof("Received EndpointGroupCreate: %+v", endpointGroup)
	return nil
}

// EndpointGroupUpdate updates endpoint group
func (ac *APIController) EndpointGroupUpdate(endpointGroup, params *contivModel.EndpointGroup) error {
	log.Infof("Received EndpointGroupUpdate: %+v, params: %+v", endpointGroup, params)
	return nil
}

// EndpointGroupDelete deletes end point group
func (ac *APIController) EndpointGroupDelete(endpointGroup *contivModel.EndpointGroup) error {
	log.Infof("Received EndpointGroupDelete: %+v", endpointGroup)
	return nil
}

// NetworkCreate creates network
func (ac *APIController) NetworkCreate(network *contivModel.Network) error {
	log.Infof("Received NetworkCreate: %+v", network)

	// Make sure tenant exists
	if network.TenantName == "" {
		return errors.New("Invalid tenant name")
	}

	tenant := contivModel.FindTenant(network.TenantName)
	if tenant == nil {
		return errors.New("Tenant not found")
	}

	// Setup links
	modeldb.AddLink(&network.Links.Tenant, tenant)
	modeldb.AddLinkSet(&tenant.LinkSets.Networks, network)

	// Save the tenant too since we added the links
	err := tenant.Write()
	if err != nil {
		log.Errorf("Error updating tenant state(%+v). Err: %v", tenant, err)
		return err
	}

	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	// Build networ config
	networkCfg := intent.ConfigNetwork{
		Name:       network.NetworkName,
		PktTagType: network.Encap,
		PktTag:     "",
		SubnetCIDR: network.Subnet,
		DefaultGw:  network.DefaultGw,
	}

	// Create the network
	err = master.CreateNetwork(networkCfg, stateDriver, network.TenantName)
	if err != nil {
		log.Errorf("Error creating network {%+v}. Err: %v", network, err)
		return err
	}

	return nil
}

// NetworkUpdate updates network
func (ac *APIController) NetworkUpdate(network, params *contivModel.Network) error {
	log.Infof("Received NetworkUpdate: %+v, params: %+v", network, params)
	return errors.New("Cant change network parameters after its created")
}

// NetworkDelete deletes network
func (ac *APIController) NetworkDelete(network *contivModel.Network) error {
	log.Infof("Received NetworkDelete: %+v", network)

	// Find the tenant
	tenant := contivModel.FindTenant(network.TenantName)
	if tenant == nil {
		return errors.New("Tenant not found")
	}

	// Remove link
	modeldb.RemoveLinkSet(&tenant.LinkSets.Networks, network)

	// Save the tenant too since we added the links
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
	err = master.DeleteNetworkID(stateDriver, network.NetworkName)
	if err != nil {
		log.Errorf("Error deleting network %s. Err: %v", network.NetworkName, err)
	}

	return nil
}

// PolicyCreate creates policy
func (ac *APIController) PolicyCreate(policy *contivModel.Policy) error {
	log.Infof("Received PolicyCreate: %+v", policy)
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
	return nil
}

// RuleCreate Creates the rule witin a policy
func (ac *APIController) RuleCreate(rule *contivModel.Rule) error {
	log.Infof("Received RuleCreate: %+v", rule)
	return nil
}

// RuleUpdate updates the rule within a policy
func (ac *APIController) RuleUpdate(rule, params *contivModel.Rule) error {
	log.Infof("Received RuleUpdate: %+v, params: %+v", rule, params)
	return nil
}

// RuleDelete deletes the rule within a policy
func (ac *APIController) RuleDelete(rule *contivModel.Rule) error {
	log.Infof("Received RuleDelete: %+v", rule)
	return nil
}

// ServiceCreate creates service
func (ac *APIController) ServiceCreate(service *contivModel.Service) error {
	log.Infof("Received ServiceCreate: %+v", service)

	// check params
	if (service.TenantName == "") || (service.AppName == "") {
		return errors.New("Invalid parameters")
	}

	// Make sure tenant exists
	tenant := contivModel.FindTenant(service.TenantName)
	if tenant == nil {
		return errors.New("Tenant not found")
	}

	// Find the app this service belongs to
	app := contivModel.FindApp(service.TenantName + ":" + service.AppName)
	if app == nil {
		return errors.New("App not found")
	}

	// Setup links
	modeldb.AddLink(&service.Links.App, app)
	modeldb.AddLinkSet(&app.LinkSets.Services, service)

	// Save the app too since we added the links
	err := app.Write()
	if err != nil {
		return err
	}

	// Check if user specified any networks
	if len(service.Networks) == 0 {
		service.Networks = append(service.Networks, "privateNet")
	}

	// link service with network
	for _, netName := range service.Networks {
		netKey := service.TenantName + ":" + netName
		network := contivModel.FindNetwork(netKey)
		if network == nil {
			log.Errorf("Service: %s could not find network %s", service.Key, netKey)
			return errors.New("Network not found")
		}

		// Link the network
		modeldb.AddLinkSet(&service.LinkSets.Networks, network)
		modeldb.AddLinkSet(&network.LinkSets.Services, service)

		// save the network
		err := network.Write()
		if err != nil {
			return err
		}
	}

	// Check if user specified any endpoint group for the service
	if len(service.EndpointGroups) == 0 {
		// Create one default endpointGroup per network
		for _, netName := range service.Networks {
			// params for default endpoint group
			dfltEpgName := service.AppName + "." + service.ServiceName + "." + netName
			endpointGroup := contivModel.EndpointGroup{
				Key:         service.TenantName + ":" + dfltEpgName,
				TenantName:  service.TenantName,
				NetworkName: netName,
				GroupName:   dfltEpgName,
			}

			// Create default endpoint group for the service
			err = contivModel.CreateEndpointGroup(&endpointGroup)
			if err != nil {
				log.Errorf("Error creating endpoint group: %+v, Err: %v", endpointGroup, err)
				return err
			}

			// Add the endpoint group to the list
			service.EndpointGroups = append(service.EndpointGroups, dfltEpgName)
		}
	}

	// Link the service and endpoint group
	for _, epgName := range service.EndpointGroups {
		endpointGroup := contivModel.FindEndpointGroup(service.TenantName + ":" + epgName)
		if endpointGroup == nil {
			log.Errorf("Error: could not find endpoint group: %s", epgName)
			return errors.New("could not find endpointGroup")
		}

		// setup links
		modeldb.AddLinkSet(&service.LinkSets.EndpointGroups, endpointGroup)
		modeldb.AddLinkSet(&endpointGroup.LinkSets.Services, service)

		// save the endpointGroup
		err = endpointGroup.Write()
		if err != nil {
			return err
		}
	}

	// Check if user specified any volume profile
	if service.VolumeProfile == "" {
		service.VolumeProfile = "default"
	}

	volProfKey := service.TenantName + ":" + service.VolumeProfile
	volProfile := contivModel.FindVolumeProfile(volProfKey)
	if volProfile == nil {
		log.Errorf("Could not find the volume profile: %s", service.VolumeProfile)
		return errors.New("VolumeProfile not found")
	}

	// fixup default values
	if service.Scale == 0 {
		service.Scale = 1
	}

	// Create service instances
	for idx := int64(0); idx < service.Scale; idx++ {
		instID := fmt.Sprintf("%d", idx+1)
		var volumes []string

		// Create a volume for each instance based on the profile
		if volProfile.DatastoreType != "none" {
			instVolName := service.AppName + "." + service.ServiceName + "." + instID
			err = contivModel.CreateVolume(&contivModel.Volume{
				Key:           service.TenantName + ":" + instVolName,
				VolumeName:    instVolName,
				TenantName:    service.TenantName,
				DatastoreType: volProfile.DatastoreType,
				PoolName:      volProfile.PoolName,
				Size:          volProfile.Size,
				MountPoint:    volProfile.MountPoint,
			})
			if err != nil {
				log.Errorf("Error creating volume %s. Err: %v", instVolName, err)
				return err
			}
			volumes = []string{instVolName}
		}

		// build instance params
		instKey := service.TenantName + ":" + service.AppName + ":" + service.ServiceName + ":" + instID
		inst := contivModel.ServiceInstance{
			Key:         instKey,
			InstanceID:  instID,
			TenantName:  service.TenantName,
			AppName:     service.AppName,
			ServiceName: service.ServiceName,
			Volumes:     volumes,
		}

		// create the instance
		err := contivModel.CreateServiceInstance(&inst)
		if err != nil {
			log.Errorf("Error creating service instance: %+v. Err: %v", inst, err)
			return err
		}
	}

	return nil
}

// ServiceUpdate updates service
func (ac *APIController) ServiceUpdate(service, params *contivModel.Service) error {
	log.Infof("Received ServiceUpdate: %+v, params: %+v", service, params)
	return nil
}

// ServiceDelete deletes service
func (ac *APIController) ServiceDelete(service *contivModel.Service) error {
	log.Infof("Received ServiceDelete: %+v", service)
	return nil
}

// ServiceInstanceCreate creates a service instance
func (ac *APIController) ServiceInstanceCreate(serviceInstance *contivModel.ServiceInstance) error {
	log.Infof("Received ServiceInstanceCreate: %+v", serviceInstance)
	inst := serviceInstance

	// Find the service
	serviceKey := inst.TenantName + ":" + inst.AppName + ":" + inst.ServiceName
	service := contivModel.FindService(serviceKey)
	if service == nil {
		log.Errorf("Service %s not found for instance: %+v", serviceKey, inst)
		return errors.New("Service not found")
	}

	// Add links
	modeldb.AddLinkSet(&service.LinkSets.Instances, inst)
	modeldb.AddLink(&inst.Links.Service, service)

	// setup links with volumes
	for _, volumeName := range inst.Volumes {
		// find the volume
		volume := contivModel.FindVolume(inst.TenantName + ":" + volumeName)
		if volume == nil {
			log.Errorf("Could not find colume %s for service: %s", volumeName, inst.Key)
			return errors.New("Could not find the volume")
		}

		// add Links
		modeldb.AddLinkSet(&inst.LinkSets.Volumes, volume)
		modeldb.AddLinkSet(&volume.LinkSets.ServiceInstances, inst)
	}

	return nil
}

// ServiceInstanceUpdate updates a service instance
func (ac *APIController) ServiceInstanceUpdate(serviceInstance, params *contivModel.ServiceInstance) error {
	log.Infof("Received ServiceInstanceUpdate: %+v, params: %+v", serviceInstance, params)
	return nil
}

// ServiceInstanceDelete deletes a service instance
func (ac *APIController) ServiceInstanceDelete(serviceInstance *contivModel.ServiceInstance) error {
	log.Infof("Received ServiceInstanceDelete: %+v", serviceInstance)
	return nil
}

// TenantCreate creates a tenant
func (ac *APIController) TenantCreate(tenant *contivModel.Tenant) error {
	log.Infof("Received TenantCreate: %+v", tenant)

	if tenant.TenantName == "" {
		return errors.New("Invalid tenant name")
	}

	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	// Build tenant config
	tenantCfg := intent.ConfigTenant{
		Name:           tenant.TenantName,
		DefaultNetType: "vlan",
		SubnetPool:     tenant.SubnetPool,
		AllocSubnetLen: uint(tenant.SubnetLen),
		VLANs:          tenant.Vlans,
		VXLANs:         tenant.Vxlans,
	}

	// Create the tenant
	err = master.CreateTenant(stateDriver, &tenantCfg)
	if err != nil {
		log.Errorf("Error creating tenant {%+v}. Err: %v", tenant, err)
		return err
	}

	// Create private network for the tenant
	err = contivModel.CreateNetwork(&contivModel.Network{
		Key:         tenant.TenantName + ":" + "private",
		IsPublic:    false,
		IsPrivate:   true,
		Encap:       "vxlan",
		Subnet:      "10.1.0.0/16",
		DefaultGw:   "10.1.0.1",
		NetworkName: "private",
		TenantName:  tenant.TenantName,
	})
	if err != nil {
		log.Errorf("Error creating privateNet for tenant: %+v. Err: %v", tenant, err)
		return err
	}

	// Create public network for the tenant
	err = contivModel.CreateNetwork(&contivModel.Network{
		Key:         tenant.TenantName + ":" + "public",
		IsPublic:    true,
		IsPrivate:   false,
		Encap:       "vlan",
		Subnet:      "192.168.1.0/24",
		DefaultGw:   "192.168.1.1",
		NetworkName: "public",
		TenantName:  tenant.TenantName,
	})
	if err != nil {
		log.Errorf("Error creating publicNet for tenant: %+v. Err: %v", tenant, err)
		return err
	}

	// Create a default volume profile for the tenant
	err = contivModel.CreateVolumeProfile(&contivModel.VolumeProfile{
		Key:               tenant.TenantName + ":" + "default",
		VolumeProfileName: "default",
		TenantName:        tenant.TenantName,
		DatastoreType:     "none",
		PoolName:          "",
		Size:              "",
		MountPoint:        "",
	})
	if err != nil {
		log.Errorf("Error creating default volume profile. Err: %v", err)
		return err
	}

	return nil
}

// TenantUpdate updates a tenant
func (ac *APIController) TenantUpdate(tenant, params *contivModel.Tenant) error {
	log.Infof("Received TenantUpdate: %+v, params: %+v", tenant, params)

	return errors.New("Cant change tenant parameters after its created")
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

// VolumeCreate creates a volume
func (ac *APIController) VolumeCreate(volume *contivModel.Volume) error {
	log.Infof("Received VolumeCreate: %+v", volume)

	// Make sure tenant exists
	if volume.TenantName == "" {
		return errors.New("Invalid tenant name")
	}

	tenant := contivModel.FindTenant(volume.TenantName)
	if tenant == nil {
		return errors.New("Tenant not found")
	}

	// Setup links
	modeldb.AddLink(&volume.Links.Tenant, tenant)
	modeldb.AddLinkSet(&tenant.LinkSets.Volumes, volume)

	// Save the tenant too since we added the links
	err := tenant.Write()
	if err != nil {
		return err
	}

	return nil
}

// VolumeUpdate updates a volume
func (ac *APIController) VolumeUpdate(volume, params *contivModel.Volume) error {
	log.Infof("Received VolumeUpdate: %+v, params: %+v", volume, params)
	return nil
}

// VolumeDelete deletes a volume
func (ac *APIController) VolumeDelete(volume *contivModel.Volume) error {
	log.Infof("Received VolumeDelete: %+v", volume)
	return nil
}

// VolumeProfileCreate create a volume profile
func (ac *APIController) VolumeProfileCreate(volumeProfile *contivModel.VolumeProfile) error {
	log.Infof("Received VolumeProfileCreate: %+v", volumeProfile)

	// Make sure tenant exists
	if volumeProfile.TenantName == "" {
		return errors.New("Invalid tenant name")
	}
	tenant := contivModel.FindTenant(volumeProfile.TenantName)
	if tenant == nil {
		return errors.New("Tenant not found")
	}

	// Setup links
	modeldb.AddLink(&volumeProfile.Links.Tenant, tenant)
	modeldb.AddLinkSet(&tenant.LinkSets.VolumeProfiles, volumeProfile)

	// Save the tenant too since we added the links
	err := tenant.Write()
	if err != nil {
		return err
	}

	return nil
}

// VolumeProfileUpdate updates a volume profile
func (ac *APIController) VolumeProfileUpdate(volumeProfile, params *contivModel.VolumeProfile) error {
	log.Infof("Received VolumeProfileUpdate: %+v, params: %+v", volumeProfile, params)
	return nil
}

// VolumeProfileDelete delete a volume profile
func (ac *APIController) VolumeProfileDelete(volumeProfile *contivModel.VolumeProfile) error {
	return nil
}
