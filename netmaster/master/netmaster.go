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
	"net"

	"github.com/cenkalti/backoff"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/gstate"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/resources"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/netplugin/utils/netutils"

	log "github.com/Sirupsen/logrus"
	"github.com/samalba/dockerclient"
)

const (
	defaultInfraNetName = "infra"
	defaultSkyDNSImage  = "skynetservices/skydns:latest"
)

func checkPktTagType(pktTagType string) error {
	if pktTagType != "" && pktTagType != "vlan" && pktTagType != "vxlan" {
		return core.Errorf("invalid pktTagType")
	}

	return nil
}

func validateTenantConfig(tenant *intent.ConfigTenant) error {
	if tenant.Name == "" {
		return core.Errorf("invalid tenant name")
	}

	if err := checkPktTagType(tenant.DefaultNetType); err != nil {
		return err
	}

	if tenant.SubnetPool != "" {
		if _, _, err := net.ParseCIDR(tenant.SubnetPool); err != nil {
			return err
		}
	}

	if tenant.VLANs != "" {
		if _, err := netutils.ParseTagRanges(tenant.VLANs, "vlan"); err != nil {
			log.Errorf("error parsing vlan range '%s'. Error: %s", tenant.VLANs, err)
			return err
		}
	}

	if tenant.VXLANs != "" {
		if _, err := netutils.ParseTagRanges(tenant.VXLANs, "vxlan"); err != nil {
			log.Errorf("error parsing vxlan range '%s'.Error: %s", tenant.VXLANs, err)
			return err
		}
	}

	return nil
}

// CreateGlobal sets the global state
func CreateGlobal(stateDriver core.StateDriver, gc *intent.ConfigGlobal) error {

	masterGc := &mastercfg.GlobConfig{}
	masterGc.StateDriver = stateDriver
	masterGc.NwInfraType = gc.NwInfraType
	err := masterGc.Write()
	return err
}

// CreateTenant sets the tenant's state according to the passed ConfigTenant.
func CreateTenant(stateDriver core.StateDriver, tenant *intent.ConfigTenant) error {
	gOper := &gstate.Oper{}
	gOper.StateDriver = stateDriver
	err := gOper.Read(tenant.Name)
	if err == nil {
		return err
	}

	err = validateTenantConfig(tenant)
	if err != nil {
		return err
	}

	gCfg := &gstate.Cfg{}
	gCfg.StateDriver = stateDriver
	gCfg.Version = gstate.VersionBeta1
	gCfg.Tenant = tenant.Name
	gCfg.Deploy.DefaultNetType = tenant.DefaultNetType
	gCfg.Deploy.DefaultNetwork = tenant.DefaultNetwork
	gCfg.Auto.SubnetPool, gCfg.Auto.SubnetLen, _ = netutils.ParseCIDR(tenant.SubnetPool)
	gCfg.Auto.VLANs = tenant.VLANs
	gCfg.Auto.VXLANs = tenant.VXLANs
	gCfg.Auto.AllocSubnetLen = tenant.AllocSubnetLen

	tempRm, err := resources.GetStateResourceManager()
	if err != nil {
		return err
	}

	// setup resources
	err = gCfg.Process(core.ResourceManager(tempRm))
	if err != nil {
		log.Errorf("Error updating the config %+v. Error: %s", gCfg, err)
		return err
	}

	// start skydns container
	err = startServiceContainer(tenant.Name)
	if err != nil {
		log.Errorf("Error starting service container. Err: %v", err)
		return err
	}

	err = gCfg.Write()
	if err != nil {
		log.Errorf("error updating tenant '%s'.Error: %s", tenant.Name, err)
		return err
	}

	return nil
}

func startServiceContainer(tenantName string) error {
	// do nothing in test mode
	if testMode {
		return nil
	}

	var err error
	docker, err := utils.GetDockerClient()
	if err != nil {
		log.Errorf("Unable to connect to docker. Error %v", err)
		return err
	}

	// pull the skydns image if it does not exist
	imageName := defaultSkyDNSImage
	_, err = docker.InspectImage(imageName)
	if err != nil {
		pullOperation := func() error {
			err := docker.PullImage(imageName, nil)
			if err != nil {
				log.Errorf("Retrying to pull image: %s", imageName)
				return err
			}
			return nil
		}

		err = backoff.Retry(pullOperation, backoff.NewExponentialBackOff())
		if err != nil {
			log.Errorf("Unable to pull image: %s", imageName)
			return err
		}
	}

	containerConfig := &dockerclient.ContainerConfig{
		Image: imageName,
		Env: []string{"ETCD_MACHINES=http://172.17.0.1:4001",
			"SKYDNS_NAMESERVERS=8.8.8.8:53",
			"SKYDNS_ADDR=0.0.0.0:53",
			"SKYDNS_DOMAIN=" + tenantName}}

	containerID, err := docker.CreateContainer(containerConfig, tenantName+"dns")
	if err != nil {
		log.Errorf("Error creating DNS container for tenant: %s. Error: %s", tenantName, err)
	}

	// Start the container
	err = docker.StartContainer(containerID, nil)
	if err != nil {
		log.Errorf("Error starting DNS container for tenant: %s. Error: %s", tenantName, err)
	}

	return err
}

func stopAndRemoveServiceContainer(tenantName string) error {
	var err error
	docker, err := utils.GetDockerClient()
	if err != nil {
		log.Errorf("Unable to connect to docker. Error %v", err)
		return err
	}

	dnsContName := tenantName + "dns"
	// Stop the container
	err = docker.StopContainer(dnsContName, 10)
	if err != nil {
		log.Errorf("Error stopping DNS container for tenant: %s. Error: %s", tenantName, err)
		return err
	}

	err = docker.RemoveContainer(dnsContName, true, true)
	if err != nil {
		log.Errorf("Error removing DNS container for tenant: %s. Error: %s", tenantName, err)
		return err
	}
	return err
}

// DeleteTenantID deletes a tenant from the state store, by ID.
func DeleteTenantID(stateDriver core.StateDriver, tenantID string) error {
	gOper := &gstate.Oper{}
	gOper.StateDriver = stateDriver
	err := gOper.Read(tenantID)
	if err != nil {
		log.Errorf("error reading tenant info '%s'. Error: %s", tenantID, err)
		return err
	}

	err = stopAndRemoveServiceContainer(tenantID)
	if err != nil {
		log.Errorf("Error in stopping service container for tenant: %+v", tenantID)
		return err
	}

	gCfg := &gstate.Cfg{}
	gCfg.StateDriver = stateDriver
	err = gCfg.Read(tenantID)
	if err != nil {
		log.Errorf("error reading tenant config '%s'. Error: %s", tenantID, err)
		return err
	}

	tempRm, err := resources.GetStateResourceManager()
	if err == nil {
		err = gCfg.DeleteResources(core.ResourceManager(tempRm))
		if err != nil {
			log.Errorf("Error deleting the config %+v. Error: %s", gCfg, err)
		}
	}

	// delete tenant config
	err = gCfg.Clear()
	if err != nil {
		log.Errorf("error deleting cfg for tenant %q: Error: %s", tenantID, err)
	}

	err = gOper.Clear()
	if err != nil {
		log.Errorf("error deleting oper for tenant %q: Error: %s", tenantID, err)
	}

	return err
}

// DeleteTenant deletes a tenant from the state store based on its ConfigTenant.
func DeleteTenant(stateDriver core.StateDriver, tenant *intent.ConfigTenant) error {
	err := validateTenantConfig(tenant)
	if err != nil {
		return err
	}

	if len(tenant.Networks) == 0 {
		return DeleteTenantID(stateDriver, tenant.Name)
	}

	return nil
}

// IsAciConfigured returns true if aci is configured on netmaster.
func IsAciConfigured() (res bool, err error) {
	// Get the state driver
	stateDriver, uErr := utils.GetStateDriver()
	if uErr != nil {
		log.Warnf("Couldn't read global config %v", uErr)
		return false, uErr
	}

	// read global config
	masterGc := &mastercfg.GlobConfig{}
	masterGc.StateDriver = stateDriver
	uErr = masterGc.Read("config")
	if core.ErrIfKeyExists(uErr) != nil {
		log.Errorf("Couldn't read global config %v", uErr)
		return false, uErr
	}

	if uErr != nil {
		log.Warnf("Couldn't read global config %v", uErr)
		return false, nil
	}

	if masterGc.NwInfraType != "aci" {
		log.Debugf("NwInfra type is %v, no ACI", masterGc.NwInfraType)
		return false, nil
	}

	return true, nil
}
