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

package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/contiv/objmodel/contivModel"
	"github.com/contiv/objmodel/objdb/modeldb"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
)

type ApiController struct {
	router *mux.Router
}

var apiCtrler *ApiController

// Create a new controller
func NewApiController(router *mux.Router) *ApiController {
	ctrler := new(ApiController)
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

func (self *ApiController) AppCreate(app *contivModel.App) error {
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
func (self *ApiController) AppUpdate(app, params *contivModel.App) error {
	log.Infof("Received AppUpdate: %+v, params: %+v", app, params)

	return nil
}

func (self *ApiController) AppDelete(app *contivModel.App) error {
	log.Infof("Received AppDelete: %+v", app)
	return nil
}

func (self *ApiController) EndpointGroupCreate(endpointGroup *contivModel.EndpointGroup) error {
	log.Infof("Received EndpointGroupCreate: %+v", endpointGroup)
	return nil
}

func (self *ApiController) EndpointGroupUpdate(endpointGroup, params *contivModel.EndpointGroup) error {
	log.Infof("Received EndpointGroupUpdate: %+v, params: %+v", endpointGroup, params)
	return nil
}

func (self *ApiController) EndpointGroupDelete(endpointGroup *contivModel.EndpointGroup) error {
	log.Infof("Received EndpointGroupDelete: %+v", endpointGroup)
	return nil
}

func (self *ApiController) NetworkCreate(network *contivModel.Network) error {
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

	return nil
}

func (self *ApiController) NetworkUpdate(network, params *contivModel.Network) error {
	log.Infof("Received NetworkUpdate: %+v, params: %+v", network, params)
	return nil
}

func (self *ApiController) NetworkDelete(network *contivModel.Network) error {
	log.Infof("Received NetworkDelete: %+v", network)
	return nil
}
func (self *ApiController) PolicyCreate(policy *contivModel.Policy) error {
	log.Infof("Received PolicyCreate: %+v", policy)
	return nil
}
func (self *ApiController) PolicyUpdate(policy, params *contivModel.Policy) error {
	log.Infof("Received PolicyUpdate: %+v, params: %+v", policy, params)
	return nil
}
func (self *ApiController) PolicyDelete(policy *contivModel.Policy) error {
	log.Infof("Received PolicyDelete: %+v", policy)
	return nil
}

func (self *ApiController) RuleCreate(rule *contivModel.Rule) error {
	log.Infof("Received RuleCreate: %+v", rule)
	return nil
}
func (self *ApiController) RuleUpdate(rule, params *contivModel.Rule) error {
	log.Infof("Received RuleUpdate: %+v, params: %+v", rule, params)
	return nil
}
func (self *ApiController) RuleDelete(rule *contivModel.Rule) error {
	log.Infof("Received RuleDelete: %+v", rule)
	return nil
}

func (self *ApiController) ServiceCreate(service *contivModel.Service) error {
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
	for idx := 0; idx < service.Scale; idx++ {
		instId := fmt.Sprintf("%d", idx+1)
		var volumes []string

		// Create a volume for each instance based on the profile
		if volProfile.DatastoreType != "none" {
			instVolName := service.AppName + "." + service.ServiceName + "." + instId
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
		instKey := service.TenantName + ":" + service.AppName + ":" + service.ServiceName + ":" + instId
		inst := contivModel.ServiceInstance{
			Key:         instKey,
			InstanceID:  instId,
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

func (self *ApiController) ServiceUpdate(service, params *contivModel.Service) error {
	log.Infof("Received ServiceUpdate: %+v, params: %+v", service, params)
	return nil
}

func (self *ApiController) ServiceDelete(service *contivModel.Service) error {
	log.Infof("Received ServiceDelete: %+v", service)
	return nil
}

func (self *ApiController) ServiceInstanceCreate(serviceInstance *contivModel.ServiceInstance) error {
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

func (self *ApiController) ServiceInstanceUpdate(serviceInstance, params *contivModel.ServiceInstance) error {
	log.Infof("Received ServiceInstanceUpdate: %+v, params: %+v", serviceInstance, params)
	return nil
}

func (self *ApiController) ServiceInstanceDelete(serviceInstance *contivModel.ServiceInstance) error {
	log.Infof("Received ServiceInstanceDelete: %+v", serviceInstance)
	return nil
}
func (self *ApiController) TenantCreate(tenant *contivModel.Tenant) error {
	log.Infof("Received TenantCreate: %+v", tenant)

	if tenant.TenantName == "" {
		return errors.New("Invalid tenant name")
	}

	// Create private network for the tenant
	err := contivModel.CreateNetwork(&contivModel.Network{
		Key:         tenant.TenantName + ":" + "privateNet",
		IsPublic:    false,
		IsPrivate:   true,
		Encap:       "vxlan",
		Subnet:      "10.1.0.0/16",
		NetworkName: "privateNet",
		TenantName:  tenant.TenantName,
	})
	if err != nil {
		log.Errorf("Error creating privateNet for tenant: %+v. Err: %v", tenant, err)
		return err
	}

	// Create public network for the tenant
	err = contivModel.CreateNetwork(&contivModel.Network{
		Key:         tenant.TenantName + ":" + "publicNet",
		IsPublic:    true,
		IsPrivate:   false,
		Encap:       "vlan",
		Subnet:      "192.168.1.0/24",
		NetworkName: "publicNet",
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

func (self *ApiController) TenantUpdate(tenant, params *contivModel.Tenant) error {
	log.Infof("Received TenantUpdate: %+v, params: %+v", tenant, params)
	return nil
}

func (self *ApiController) TenantDelete(tenant *contivModel.Tenant) error {
	log.Infof("Received TenantDelete: %+v", tenant)
	return nil
}

func (self *ApiController) VolumeCreate(volume *contivModel.Volume) error {
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

func (self *ApiController) VolumeUpdate(volume, params *contivModel.Volume) error {
	log.Infof("Received VolumeUpdate: %+v, params: %+v", volume, params)
	return nil
}

func (self *ApiController) VolumeDelete(volume *contivModel.Volume) error {
	log.Infof("Received VolumeDelete: %+v", volume)
	return nil
}

func (self *ApiController) VolumeProfileCreate(volumeProfile *contivModel.VolumeProfile) error {
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

func (self *ApiController) VolumeProfileUpdate(volumeProfile, params *contivModel.VolumeProfile) error {
	log.Infof("Received VolumeProfileUpdate: %+v, params: %+v", volumeProfile, params)
	return nil
}

func (self *ApiController) VolumeProfileDelete(volumeProfile *contivModel.VolumeProfile) error {
	return nil
}

// Create a HTTP Server and initialize the router
func CreateServer(port int) {
	listenAddr := ":" + strconv.Itoa(port)

	// Create a router
	router := mux.NewRouter()

	// Create the API controller
	apiCtrler = NewApiController(router)

	log.Infof("HTTP server listening on %s", listenAddr)

	// Start the HTTP server
	log.Fatal(http.ListenAndServe(listenAddr, router))
}

func main() {
	CreateServer(8000)
}
