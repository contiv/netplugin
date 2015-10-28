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
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/gstate"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/netutils"
	"github.com/contiv/netplugin/resources"

	log "github.com/Sirupsen/logrus"
)

const (
	defaultInfraNetName = "infra"
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

	masterGc := &GlobConfig{}
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
	err = gCfg.Write()
	if err != nil {
		log.Errorf("error updating tenant '%s'.Error: %s", tenant.Name, err)
		return err
	}

	tempRm, err := resources.GetStateResourceManager()
	if err != nil {
		return err
	}

	err = gCfg.Process(core.ResourceManager(tempRm))
	if err != nil {
		log.Errorf("Error updating the config %+v. Error: %s", gCfg, err)
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

	gCfg := &gstate.Cfg{}
	gCfg.StateDriver = stateDriver
	gCfg.Version = gstate.VersionBeta1
	gCfg.Tenant = tenantID
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

func validateHostConfig(host *intent.ConfigHost) error {
	if host.Name == "" {
		return core.Errorf("null host name")
	}
	if host.VtepIP == "" && host.Intf == "" {
		return core.Errorf("either vtep or intf needed for the host")
	}

	return nil
}

func getVtepName(netID, hostLabel string) string {
	return netID + "-" + hostLabel
}

func createInfraNetwork(epCfg *mastercfg.CfgEndpointState) error {
	if epCfg.NetID != "" {
		return nil
	}

	nwCfg := &mastercfg.CfgNetworkState{}
	nwCfg.StateDriver = epCfg.StateDriver
	if nwCfg.Read(epCfg.NetID) == nil {
		return nil
	}

	nwCfg.ID = defaultInfraNetName
	err := nwCfg.Write()
	if err != nil {
		return err
	}

	epCfg.NetID = defaultInfraNetName
	return nil
}

func createVtep(stateDriver core.StateDriver, hostCfg *HostConfig, tenantNet string) error {

	epCfg := &mastercfg.CfgEndpointState{}
	epCfg.StateDriver = stateDriver
	epCfg.ID = getVtepName(tenantNet, hostCfg.Name)
	epCfg.HomingHost = hostCfg.Name
	epCfg.VtepIP = hostCfg.VtepIP
	epCfg.NetID = tenantNet
	err := createInfraNetwork(epCfg)
	if err != nil {
		log.Errorf("error creating infra vlan. Error: %s", err)
		return err
	}

	err = epCfg.Write()
	if err != nil {
		log.Errorf("error adding vtep ep. Error: %s", err)
		return err
	}

	return nil
}

func deleteVtep(stateDriver core.StateDriver, netID, hostName string) error {

	epCfg := &mastercfg.CfgEndpointState{}
	epCfg.StateDriver = stateDriver
	epCfg.ID = getVtepName(netID, hostName)
	epCfg.HomingHost = hostName
	epCfg.NetID = netID

	err := epCfg.Clear()
	if err != nil {
		log.Errorf("error deleting vtep ep. Error: %s", err)
		return err
	}

	return nil
}

func getVLANIfName(hostLabel string) string {
	return hostLabel + "-native-intf"
}

func createVLANIf(stateDriver core.StateDriver, host *intent.ConfigHost) error {

	epCfg := &mastercfg.CfgEndpointState{}
	epCfg.StateDriver = stateDriver
	epCfg.ID = getVLANIfName(host.Name)
	epCfg.HomingHost = host.Name
	epCfg.IntfName = host.Intf
	epCfg.NetID = host.NetID
	err := createInfraNetwork(epCfg)
	if err != nil {
		log.Errorf("error creating infra vlan. Error: %s", err)
		return err
	}

	err = epCfg.Write()
	if err != nil {
		log.Errorf("error adding vtep ep. Error: %s", err)
		return err
	}

	return nil
}

func deleteVLANIf(stateDriver core.StateDriver, hostName string) error {

	epCfg := &mastercfg.CfgEndpointState{}
	epCfg.StateDriver = stateDriver
	epCfg.ID = getVLANIfName(hostName)
	epCfg.HomingHost = hostName

	err := epCfg.Clear()
	if err != nil {
		log.Errorf("error deleting vtep ep. Error: %s", err)
		return err
	}

	return nil
}

// CreateHost creates a host in the state configuration.
func CreateHost(stateDriver core.StateDriver, host *intent.ConfigHost) error {
	err := validateHostConfig(host)
	if err != nil {
		log.Errorf("error validating host config. Error: %s", err)
		return err
	}

	// construct and update master host state
	hostCfg := &HostConfig{}
	hostCfg.StateDriver = stateDriver
	hostCfg.Name = host.Name
	hostCfg.Intf = host.Intf
	hostCfg.VtepIP = host.VtepIP
	hostCfg.NetID = host.NetID

	if host.VtepIP != "" {
		// walk through all nets and create vtep eps as necessary

		readNet := &mastercfg.CfgNetworkState{}
		readNet.StateDriver = stateDriver
		tenantNets, err := readNet.ReadAll()
		if err != nil {
			if !strings.Contains(err.Error(), "Key not found") {
				log.Errorf("error reading keys during host create. Error: %s", err)
			}
		}
		for _, tenantNet := range tenantNets {
			nw := tenantNet.(*mastercfg.CfgNetworkState)
			err = createVtep(stateDriver, hostCfg, nw.ID)
			if err != nil {
				log.Errorf("error creating vtep. Error: %s", err)
			}
		}
	}
	if host.Intf != "" {
		err = createVLANIf(stateDriver, host)
		if err != nil {
			log.Errorf("error creating infra if %s on host %s, Error: %s",
				host.Name, host.Intf, err)
		}
	}

	err = hostCfg.Write()
	if err != nil {
		log.Errorf("error writing host config. Error: %s", err)
		return err
	}

	return nil
}

// DeleteHostID deletes a host by ID.
func DeleteHostID(stateDriver core.StateDriver, hostName string) error {
	hostCfg := &HostConfig{}
	hostCfg.StateDriver = stateDriver
	hostCfg.Name = hostName

	err := hostCfg.Read(hostName)
	if err != nil {
		log.Errorf("error reading master host config name %s. Error: %s",
			hostName, err)
		return err
	}

	if hostCfg.VtepIP != "" {
		// walk through all nets and delete vtep eps as necessary
		readNet := &mastercfg.CfgNetworkState{}
		readNet.StateDriver = stateDriver
		tenantNets, err := readNet.ReadAll()
		if err != nil {
			if !strings.Contains(err.Error(), "Key not found") {
				log.Errorf("error reading keys during host create. Error: %s", err)
			}
		}
		for _, tenantNet := range tenantNets {
			nw := tenantNet.(*mastercfg.CfgNetworkState)
			err = deleteVtep(stateDriver, nw.ID, hostName)
			if err != nil {
				log.Errorf("error deleting vtep. Error: %s", err)
			}
		}
	}
	if hostCfg.Intf != "" {
		err = deleteVLANIf(stateDriver, hostName)
		if err != nil {
			log.Errorf("error deleting infra if %s on host %s. Error: %s",
				hostCfg.Intf, hostName, err)
		}
	}

	err = hostCfg.Clear()
	if err != nil {
		log.Errorf("error deleting host config. Error: %s", err)
		return err
	}

	return err
}

// DeleteHost deletes a host by its ConfigHost state
func DeleteHost(stateDriver core.StateDriver, host *intent.ConfigHost) error {
	return DeleteHostID(stateDriver, host.Name)
}
