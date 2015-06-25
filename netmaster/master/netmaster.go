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
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/gstate"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/netutils"
	"github.com/contiv/netplugin/resources"
	"github.com/contiv/netplugin/utils"

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

	err := checkPktTagType(tenant.DefaultNetType)
	if err != nil {
		return err
	}

	if tenant.SubnetPool != "" {
		_, _, err = net.ParseCIDR(tenant.SubnetPool)
		if err != nil {
			return err
		}
	}

	if tenant.VLANs != "" {
		_, err = netutils.ParseTagRanges(tenant.VLANs, "vlan")
		if err != nil {
			log.Errorf("error parsing vlan range '%s'. Error: %s", tenant.VLANs, err)
			return err
		}
	}

	if tenant.VXLANs != "" {
		_, err = netutils.ParseTagRanges(tenant.VXLANs, "vxlan")
		if err != nil {
			log.Errorf("error parsing vxlan range '%s'.Error: %s", tenant.VXLANs, err)
			return err
		}
	}

	return nil
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

func createInfraNetwork(epCfg *drivers.OvsCfgEndpointState) error {
	if epCfg.NetID != "" {
		return nil
	}

	nwCfg := &drivers.OvsCfgNetworkState{}
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

	epCfg := &drivers.OvsCfgEndpointState{}
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

	epCfg := &drivers.OvsCfgEndpointState{}
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

	epCfg := &drivers.OvsCfgEndpointState{}
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

	epCfg := &drivers.OvsCfgEndpointState{}
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

		readNet := &drivers.OvsCfgNetworkState{}
		readNet.StateDriver = stateDriver
		tenantNets, err := readNet.ReadAll()
		if err != nil {
			if !strings.Contains(err.Error(), "Key not found") {
				log.Errorf("error reading keys during host create. Error: %s", err)
			}
		}
		for _, tenantNet := range tenantNets {
			nw := tenantNet.(*drivers.OvsCfgNetworkState)
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
		readNet := &drivers.OvsCfgNetworkState{}
		readNet.StateDriver = stateDriver
		tenantNets, err := readNet.ReadAll()
		if err != nil {
			if !strings.Contains(err.Error(), "Key not found") {
				log.Errorf("error reading keys during host create. Error: %s", err)
			}
		}
		for _, tenantNet := range tenantNets {
			nw := tenantNet.(*drivers.OvsCfgNetworkState)
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

func validateNetworkConfig(tenant *intent.ConfigTenant) error {
	var err error

	if tenant.Name == "" {
		return core.Errorf("null tenant name")
	}

	for _, network := range tenant.Networks {
		if network.Name == "" {
			core.Errorf("null network name")
		}

		err = checkPktTagType(network.PktTagType)
		if err != nil {
			return err
		}

		if network.PktTag != "" {
			_, err = strconv.Atoi(network.PktTag)
			if err != nil {
				return err
			}
		}

		if network.SubnetCIDR != "" {
			_, _, err = netutils.ParseCIDR(network.SubnetCIDR)
			if err != nil {
				return err
			}
		}

		if network.DefaultGw != "" {
			if net.ParseIP(network.DefaultGw) == nil {
				return core.Errorf("invalid IP")
			}
		}
	}

	return err
}

// CreateNetworks creates the necessary virtual networks for the tenant
// provided by ConfigTenant.
func CreateNetworks(stateDriver core.StateDriver, tenant *intent.ConfigTenant) error {
	var extPktTag, pktTag uint

	gCfg := gstate.Cfg{}
	gCfg.StateDriver = stateDriver
	err := gCfg.Read(tenant.Name)
	if err != nil {
		log.Errorf("error reading tenant cfg state. Error: %s", err)
		return err
	}

	tempRm, err := resources.GetStateResourceManager()
	if err != nil {
		return err
	}
	rm := core.ResourceManager(tempRm)

	err = validateNetworkConfig(tenant)
	if err != nil {
		log.Errorf("error validating network config. Error: %s", err)
		return err
	}

	for _, network := range tenant.Networks {
		nwCfg := &drivers.OvsCfgNetworkState{}
		nwCfg.StateDriver = stateDriver
		if nwCfg.Read(network.Name) == nil {
			// TODO: check if parameters changed and apply an update if needed
			continue
		}

		// construct and update network state
		nwMasterCfg := &NwConfig{}
		nwMasterCfg.StateDriver = stateDriver
		nwMasterCfg.Tenant = tenant.Name
		nwMasterCfg.ID = network.Name
		nwMasterCfg.PktTagType = network.PktTagType
		nwMasterCfg.PktTag = network.PktTag
		nwMasterCfg.SubnetIP, nwMasterCfg.SubnetLen, _ = netutils.ParseCIDR(network.SubnetCIDR)
		nwMasterCfg.DefaultGw = network.DefaultGw

		nwCfg = &drivers.OvsCfgNetworkState{Tenant: nwMasterCfg.Tenant,
			PktTagType: nwMasterCfg.PktTagType,
			SubnetIP:   nwMasterCfg.SubnetIP, SubnetLen: nwMasterCfg.SubnetLen}
		nwCfg.StateDriver = stateDriver
		nwCfg.ID = nwMasterCfg.ID

		if nwMasterCfg.PktTagType == "" {
			nwCfg.PktTagType = gCfg.Deploy.DefaultNetType
		}
		if nwMasterCfg.PktTag == "" {
			if nwCfg.PktTagType == "vlan" {
				pktTag, err = gCfg.AllocVLAN(rm)
				if err != nil {
					return err
				}
			} else if nwCfg.PktTagType == "vxlan" {
				extPktTag, pktTag, err = gCfg.AllocVXLAN(rm)
				if err != nil {
					return err
				}
			}

			nwCfg.ExtPktTag = int(extPktTag)
			nwCfg.PktTag = int(pktTag)
		} else if nwMasterCfg.PktTagType == "vxlan" {
			// XXX: take local vlan as config, instead of allocating it
			// independently. Return erro for now, if user tries this config
			return core.Errorf("Not handled. Need to introduce local-vlan config")
		} else if nwMasterCfg.PktTagType == "vlan" {
			nwCfg.PktTag, _ = strconv.Atoi(nwMasterCfg.PktTag)
			// XXX: do configuration check, to make sure it is allowed
		}

		if nwCfg.SubnetIP == "" {
			nwCfg.SubnetLen = gCfg.Auto.AllocSubnetLen
			nwCfg.SubnetIP, err = gCfg.AllocSubnet(rm)
			if err != nil {
				return err
			}
		}

		nwCfg.DefaultGw = network.DefaultGw
		if nwCfg.DefaultGw == "" {
			// TBD: allocate per global policy
		}

		netutils.InitSubnetBitset(&nwCfg.IPAllocMap, nwCfg.SubnetLen)
		err = nwCfg.Write()
		if err != nil {
			return err
		}

		err = nwMasterCfg.Write()
		if err != nil {
			log.Errorf("error writing nw config. Error: %s", err)
			return err
		}

		if nwCfg.PktTagType == "vxlan" {

			readHost := &HostConfig{}
			readHost.StateDriver = stateDriver
			hostCfgs, err := readHost.ReadAll()
			if err != nil {
				if !strings.Contains(err.Error(), "Key not found") {
					log.Errorf("error reading hosts during net add. Error: %s", err)
				}
			}
			for _, hostCfg := range hostCfgs {
				host := hostCfg.(*HostConfig)
				err = createVtep(stateDriver, host, nwCfg.ID)
				if err != nil {
					log.Errorf("error creating vtep. Error: %s", err)
				}
			}
		}
	}

	return err
}

func freeNetworkResources(stateDriver core.StateDriver, nwMasterCfg *NwConfig,
	nwCfg *drivers.OvsCfgNetworkState, gCfg *gstate.Cfg) (err error) {

	tempRm, err := resources.GetStateResourceManager()
	if err != nil {
		return err
	}
	rm := core.ResourceManager(tempRm)

	if nwCfg.PktTagType == "vlan" {
		err = gCfg.FreeVLAN(rm, uint(nwCfg.PktTag))
		if err != nil {
			return err
		}
	} else if nwCfg.PktTagType == "vxlan" {
		log.Infof("freeing vlan %d vxlan %d", nwCfg.PktTag, nwCfg.ExtPktTag)
		err = gCfg.FreeVXLAN(rm, uint(nwCfg.ExtPktTag), uint(nwCfg.PktTag))
		if err != nil {
			return err
		}
	}

	if nwMasterCfg.SubnetIP == "" {
		log.Infof("freeing subnet %s/%s", nwCfg.SubnetIP, nwCfg.SubnetLen)
		err = gCfg.FreeSubnet(rm, nwCfg.SubnetIP)
		if err != nil {
			return err
		}
	}

	return err
}

// DeleteNetworkID removes a network by ID.
func DeleteNetworkID(stateDriver core.StateDriver, netID string) error {
	nwMasterCfg := &NwConfig{}
	nwMasterCfg.StateDriver = stateDriver
	err := nwMasterCfg.Read(netID)
	if err != nil {
		log.Errorf("network %q is not configured", netID)
		return err
	}

	nwCfg := &drivers.OvsCfgNetworkState{}
	nwCfg.StateDriver = stateDriver
	err = nwCfg.Read(netID)
	if err != nil {
		log.Errorf("network %s is not operational", netID)
		return err
	}

	gCfg := &gstate.Cfg{}
	gCfg.StateDriver = stateDriver
	err = gCfg.Read(nwMasterCfg.Tenant)
	if err != nil {
		log.Errorf("error reading tenant info for %q. Error: %s", nwMasterCfg.Tenant, err)
		return err
	}

	err = freeNetworkResources(stateDriver, nwMasterCfg, nwCfg, gCfg)
	if err != nil {
		return err
	}

	err = nwCfg.Clear()
	if err != nil {
		log.Errorf("error writing nw config. Error: %s", err)
		return err
	}

	return err
}

// DeleteNetworks removes all the virtual networks for a given tenant.
func DeleteNetworks(stateDriver core.StateDriver, tenant *intent.ConfigTenant) error {
	gCfg := &gstate.Cfg{}
	gCfg.StateDriver = stateDriver

	err := gCfg.Read(tenant.Name)
	if err != nil {
		log.Errorf("error reading tenant state. Error: %s", err)
		return err
	}

	err = validateNetworkConfig(tenant)
	if err != nil {
		log.Errorf("error validating network config. Error: %s", err)
		return err
	}

	for _, network := range tenant.Networks {
		if len(network.Endpoints) > 0 {
			continue
		}
		nwMasterCfg := &NwConfig{}
		nwMasterCfg.StateDriver = stateDriver
		err = nwMasterCfg.Read(network.Name)
		if err != nil {
			log.Infof("network %q is not configured", network.Name)
			continue
		}

		nwCfg := &drivers.OvsCfgNetworkState{}
		nwCfg.StateDriver = stateDriver
		err = nwCfg.Read(network.Name)
		if err != nil {
			log.Infof("network %q is not operational", network.Name)
			continue
		}

		err = freeNetworkResources(stateDriver, nwMasterCfg, nwCfg, gCfg)
		if err != nil {
			return err
		}

		err = nwCfg.Clear()
		if err != nil {
			log.Errorf("error when writing nw config. Error: %s", err)
			return err
		}
	}

	return err
}

func validateEndpointConfig(stateDriver core.StateDriver, tenant *intent.ConfigTenant) error {
	var err error

	if tenant.Name == "" {
		return core.Errorf("null tenant name")
	}

	for _, network := range tenant.Networks {
		if network.Name == "" {
			core.Errorf("null network name")
		}

		for _, ep := range network.Endpoints {
			if ep.Container == "" {
				return core.Errorf("invalid container name for the endpoint")
			}
			if ep.IPAddress != "" {
				nwMasterCfg := &NwConfig{}
				nwMasterCfg.StateDriver = stateDriver
				err = nwMasterCfg.Read(network.Name)
				if err != nil {
					log.Errorf("error reading network state. Error: %s", err)
					return err
				}
				if nwMasterCfg.SubnetIP != "" {
					log.Errorf("found ep with ip for auto-allocated net")
					return core.Errorf("found ep with ip for auto-allocated net")
				}
				if net.ParseIP(ep.IPAddress) == nil {
					return core.Errorf("invalid ep IP")
				}
			}
		}
	}

	return err
}

func getEpName(net *ConfigNetwork, ep *ConfigEP) string {
	if ep.Container != "" {
		return net.Name + "-" + ep.Container
	}

	return ep.Host + "-native-intf"
}

func allocSetEpAddress(ep *ConfigEP, epCfg *drivers.OvsCfgEndpointState,
	nwCfg *drivers.OvsCfgNetworkState) (err error) {

	var ipAddrValue uint
	var found bool

	ipAddress := ep.IPAddress
	if ipAddress == "" {
		if ipAddrValue, found = nwCfg.IPAllocMap.NextClear(0); !found {
			log.Errorf("auto allocation failed - address exhaustion in subnet %s/%d",
				nwCfg.SubnetIP, nwCfg.SubnetLen)
			err = core.Errorf("auto allocation failed - address exhaustion in subnet %s/%d",
				nwCfg.SubnetIP, nwCfg.SubnetLen)
			return
		}
		ipAddress, err = netutils.GetSubnetIP(
			nwCfg.SubnetIP, nwCfg.SubnetLen, 32, ipAddrValue)
		if err != nil {
			log.Errorf("create eps: error acquiring subnet ip. Error: %s", err)
			return
		}
	} else if ipAddress != "" && nwCfg.SubnetIP != "" {
		ipAddrValue, err = netutils.GetIPNumber(
			nwCfg.SubnetIP, nwCfg.SubnetLen, 32, ipAddress)
		if err != nil {
			log.Errorf("create eps: error getting host id from hostIP %s Subnet %s/%d. Error: %s",
				ipAddress, nwCfg.SubnetIP, nwCfg.SubnetLen, err)
			return
		}
	}
	epCfg.IPAddress = ipAddress
	nwCfg.IPAllocMap.Set(ipAddrValue)

	// Set mac address which is derived from IP address
	ipAddr := net.ParseIP(ipAddress)
	macAddr := fmt.Sprintf("02:02:%02x:%02x:%02x:%02x", ipAddr[12], ipAddr[13], ipAddr[14], ipAddr[15])
	epCfg.MacAddress = macAddr

	return
}

// CreateEndpoints creates the endpoints for a given tenant.
func CreateEndpoints(stateDriver core.StateDriver, tenant *intent.ConfigTenant) error {
	err := validateEndpointConfig(stateDriver, tenant)
	if err != nil {
		log.Errorf("error validating endpoint config. Error: %s", err)
		return err
	}

	for _, network := range tenant.Networks {
		nwMasterCfg := NwConfig{}
		nwMasterCfg.StateDriver = stateDriver
		err = nwMasterCfg.Read(network.Name)
		if err != nil {
			log.Errorf("error reading cfg network %s. Error: %s", network.Name, err)
			return err
		}

		nwCfg := &drivers.OvsCfgNetworkState{}
		nwCfg.StateDriver = stateDriver
		err = nwCfg.Read(network.Name)
		if err != nil {
			log.Errorf("error reading oper network %s. Error: %s", network.Name, err)
			return err
		}

		for _, ep := range network.Endpoints {
			epCfg := &drivers.OvsCfgEndpointState{}
			epCfg.StateDriver = stateDriver
			epCfg.ID = getEpName(&network, &ep)
			err = epCfg.Read(epCfg.ID)
			if err == nil {
				// TODO: check for diffs and possible updates
				continue
			}

			epCfg.NetID = network.Name
			epCfg.ContName = ep.Container
			epCfg.AttachUUID = ep.AttachUUID
			epCfg.HomingHost = ep.Host

			err = allocSetEpAddress(&ep, epCfg, nwCfg)
			if err != nil {
				log.Errorf("error allocating and/or reserving IP. Error: %s", err)
				return err
			}

			err = epCfg.Write()
			if err != nil {
				log.Errorf("error writing nw config. Error: %s", err)
				return err
			}
			nwCfg.EpCount++
		}

		err = nwCfg.Write()
		if err != nil {
			log.Errorf("error writing nw config. Error: %s", err)
			return err
		}
	}

	return err
}

func freeEndpointResources(epCfg *drivers.OvsCfgEndpointState,
	nwCfg *drivers.OvsCfgNetworkState) error {
	if epCfg.IPAddress == "" {
		return nil
	}

	ipAddrValue, err := netutils.GetIPNumber(
		nwCfg.SubnetIP, nwCfg.SubnetLen, 32, epCfg.IPAddress)
	if err != nil {
		log.Errorf("error getting host id from hostIP %s Subnet %s/%d. Error: %s",
			epCfg.IPAddress, nwCfg.SubnetIP, nwCfg.SubnetLen, err)
		return err
	}
	nwCfg.IPAllocMap.Clear(ipAddrValue)
	nwCfg.EpCount--

	return nil
}

// DeleteEndpointID deletes an endpoint by ID.
func DeleteEndpointID(stateDriver core.StateDriver, epID string) error {
	epCfg := &drivers.OvsCfgEndpointState{}
	epCfg.StateDriver = stateDriver
	err := epCfg.Read(epID)
	if err != nil {
		return err
	}

	nwCfg := &drivers.OvsCfgNetworkState{}
	nwCfg.StateDriver = stateDriver
	err = nwCfg.Read(epCfg.NetID)
	if err != nil {
		return err
	}

	err = freeEndpointResources(epCfg, nwCfg)
	if err != nil {
		return err
	}

	err = epCfg.Clear()
	if err != nil {
		log.Errorf("error writing nw config. Error: %s", err)
		return err
	}

	err = nwCfg.Write()
	if err != nil {
		log.Errorf("error writing nw config. Error: %s", err)
		return err
	}

	return err
}

// DeleteEndpoints deletes the endpoints for the tenant.
func DeleteEndpoints(stateDriver core.StateDriver, tenant *intent.ConfigTenant) error {

	err := validateEndpointConfig(stateDriver, tenant)
	if err != nil {
		log.Errorf("error validating endpoint config: Error: %s", err)
		return err
	}

	for _, network := range tenant.Networks {
		nwMasterCfg := &NwConfig{}
		nwMasterCfg.StateDriver = stateDriver
		err = nwMasterCfg.Read(network.Name)
		if err != nil {
			log.Errorf("error reading network state. Error: %s", err)
			return err
		}

		nwCfg := &drivers.OvsCfgNetworkState{}
		nwCfg.StateDriver = stateDriver
		err = nwCfg.Read(network.Name)
		if err != nil {
			log.Errorf("error reading network state. Error: %s", err)
			return err
		}

		for _, ep := range network.Endpoints {
			epCfg := &drivers.OvsCfgEndpointState{}
			epCfg.StateDriver = stateDriver
			epCfg.ID = getEpName(&network, &ep)
			err = epCfg.Read(epCfg.ID)
			if err != nil {
				log.Errorf("error getting cfg state of ep %s, Error: %s", epCfg.ID, err)
				continue
			}

			err = freeEndpointResources(epCfg, nwCfg)
			if err != nil {
				continue
			}

			err = epCfg.Clear()
			if err != nil {
				log.Errorf("error writing ep config. Error: %s", err)
				return err
			}
		}

		err = nwCfg.Write()
		if err != nil {
			log.Errorf("error writing nw config. Error: %s", err)
			return err
		}
	}

	return err
}

func validateEpBindings(epBindings *[]intent.ConfigEP) error {
	for _, ep := range *epBindings {
		if ep.Host == "" {
			return core.Errorf("invalid host name for the endpoint")
		}
		if ep.Container == "" {
			return core.Errorf("invalid container name for the endpoint")
		}
	}

	return nil
}

// CreateEpBindings binds an endpoint to a host by updating host-label info
// in driver's endpoint configuration.
func CreateEpBindings(epBindings *[]intent.ConfigEP) error {
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	err = validateEpBindings(epBindings)
	if err != nil {
		log.Errorf("error validating the ep bindings. Error: %s", err)
		return err
	}

	readEp := &drivers.OvsCfgEndpointState{}
	readEp.StateDriver = stateDriver
	epCfgs, err := readEp.ReadAll()
	if err != nil {
		log.Errorf("error fetching eps. Error: %s", err)
		return err
	}
	for _, ep := range *epBindings {
		log.Infof("creating binding between container '%s' and host '%s'",
			ep.Container, ep.Host)

		for _, epCfg := range epCfgs {
			cfg := epCfg.(*drivers.OvsCfgEndpointState)
			if cfg.ContName != ep.Container {
				continue
			}
			cfg.HomingHost = ep.Host
			cfg.AttachUUID = ep.AttachUUID
			err = cfg.Write()
			if err != nil {
				log.Errorf("error updating epCfg. Error: %s", err)
				return err
			}
		}
	}

	return nil
}
