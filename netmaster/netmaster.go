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

package netmaster

import (
	"net"
	"strconv"
	"strings"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/gstate"
	"github.com/contiv/netplugin/netutils"
	"github.com/contiv/netplugin/resources"

	log "github.com/Sirupsen/logrus"
)

const (
	defaultInfraNetName = "infra"
)

// interface that cluster manager implements; this is external interface to
// the cluster manager
type netmasterIf interface {
	Init(cfg *Config) error
	CreateNetwork(net *ConfigNetwork) error
	DeleteNetwork(netid string) error
	CreateEndpoint(ep *ConfigEP) error
	DeleteEndpoint(epid string) error
}

// Init is an implementation of the netmasterIf interface
// TODO remove reference to statedriver being passed as an argument,
// instead create and maintain the state
func Init(stateDriver *core.StateDriver, cfg *Config) error {
	return nil
}

func checkPktTagType(pktTagType string) error {
	if pktTagType != "" && pktTagType != "vlan" && pktTagType != "vxlan" {
		return core.Errorf("invalid pktTagType")
	}

	return nil
}

func validateTenantConfig(tenant *ConfigTenant) error {
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

	if tenant.Vlans != "" {
		_, err = netutils.ParseTagRanges(tenant.Vlans, "vlan")
		if err != nil {
			log.Printf("error '%s' parsing vlan range '%s' \n",
				err, tenant.Vlans)
			return err
		}
	}

	if tenant.Vxlans != "" {
		_, err = netutils.ParseTagRanges(tenant.Vxlans, "vxlan")
		if err != nil {
			log.Printf("error '%s' parsing vxlan range '%s' \n",
				err, tenant.Vxlans)
			return err
		}
	}

	return nil
}

// CreateTenant sets the tenant's state according to the passed ConfigTenant.
func CreateTenant(stateDriver core.StateDriver, tenant *ConfigTenant) error {
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
	gCfg.Auto.Vlans = tenant.Vlans
	gCfg.Auto.Vxlans = tenant.Vxlans
	gCfg.Auto.AllocSubnetLen = tenant.AllocSubnetLen
	err = gCfg.Write()
	if err != nil {
		log.Printf("error '%s' updating tenant '%s' \n", err, tenant.Name)
	}

	// XXX: instead of initing resource-manager always, just init and
	// store it once. Also the type of resource-manager should be picked up
	// based on configuration.
	ra := &resources.EtcdResourceManager{Etcd: stateDriver}
	err = ra.Init()
	if err != nil {
		return err
	}

	err = gCfg.Process(core.ResourceManager(ra))
	if err != nil {
		log.Printf("Error '%s' updating the config %v \n", err, gCfg)
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
		log.Printf("error '%s' reading tenant info '%s' \n", err, tenantID)
		return err
	}

	gCfg := &gstate.Cfg{}
	gCfg.StateDriver = stateDriver
	gCfg.Version = gstate.VersionBeta1
	gCfg.Tenant = tenantID
	err = gCfg.Clear()
	if err != nil {
		log.Printf("error '%s' deleting cfg for tenant '%s' \n", err, tenantID)
	}

	err = gOper.Clear()
	if err != nil {
		log.Printf("error '%s' deleting oper for tenant '%s' \n", err, tenantID)
	}

	return err
}

// DeleteTenant deletes a tenant from the state store based on its ConfigTenant.
func DeleteTenant(stateDriver core.StateDriver, tenant *ConfigTenant) error {
	err := validateTenantConfig(tenant)
	if err != nil {
		return err
	}

	if len(tenant.Networks) == 0 {
		return DeleteTenantID(stateDriver, tenant.Name)
	}

	return nil
}

func validateHostConfig(host *ConfigHost) error {
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

func createVtep(stateDriver core.StateDriver, hostCfg *MasterHostConfig, tenantNet string) error {

	epCfg := &drivers.OvsCfgEndpointState{}
	epCfg.StateDriver = stateDriver
	epCfg.ID = getVtepName(tenantNet, hostCfg.Name)
	epCfg.HomingHost = hostCfg.Name
	epCfg.VtepIP = hostCfg.VtepIP
	epCfg.NetID = tenantNet
	err := createInfraNetwork(epCfg)
	if err != nil {
		log.Printf("error '%s' creating infra vlan \n", err)
		return err
	}

	err = epCfg.Write()
	if err != nil {
		log.Printf("error '%s' adding vtep ep \n", err)
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
		log.Printf("error '%s' deleting vtep ep \n", err)
		return err
	}

	return nil
}

func getVlanIfName(hostLabel string) string {
	return hostLabel + "-native-intf"
}

func createVlanIf(stateDriver core.StateDriver, host *ConfigHost) error {

	epCfg := &drivers.OvsCfgEndpointState{}
	epCfg.StateDriver = stateDriver
	epCfg.ID = getVlanIfName(host.Name)
	epCfg.HomingHost = host.Name
	epCfg.IntfName = host.Intf
	epCfg.NetID = host.NetID
	err := createInfraNetwork(epCfg)
	if err != nil {
		log.Printf("error '%s' creating infra vlan \n", err)
		return err
	}

	err = epCfg.Write()
	if err != nil {
		log.Printf("error '%s' adding vtep ep \n", err)
		return err
	}

	return nil
}

func deleteVlanIf(stateDriver core.StateDriver, hostName string) error {

	epCfg := &drivers.OvsCfgEndpointState{}
	epCfg.StateDriver = stateDriver
	epCfg.ID = getVlanIfName(hostName)
	epCfg.HomingHost = hostName

	err := epCfg.Clear()
	if err != nil {
		log.Printf("error '%s' deleting vtep ep \n", err)
		return err
	}

	return nil
}

// CreateHost creates a host in the state configuration.
func CreateHost(stateDriver core.StateDriver, host *ConfigHost) error {
	err := validateHostConfig(host)
	if err != nil {
		log.Printf("error '%s' validating host config \n", err)
		return err
	}

	// construct and update master host state
	hostCfg := &MasterHostConfig{}
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
				log.Printf("error '%s' eading keys during host create\n", err)
			}
		}
		for _, tenantNet := range tenantNets {
			nw := tenantNet.(*drivers.OvsCfgNetworkState)
			err = createVtep(stateDriver, hostCfg, nw.ID)
			if err != nil {
				log.Printf("error '%s' creating vtep \n", err)
			}
		}
	}
	if host.Intf != "" {
		err = createVlanIf(stateDriver, host)
		if err != nil {
			log.Printf("error '%s' creating infra if %s on host %s \n",
				err, host.Name, host.Intf)
		}
	}

	err = hostCfg.Write()
	if err != nil {
		log.Printf("error '%s' when writing host config \n", err)
		return err
	}

	return nil
}

// DeleteHostID deletes a host by ID.
func DeleteHostID(stateDriver core.StateDriver, hostName string) error {
	hostCfg := &MasterHostConfig{}
	hostCfg.StateDriver = stateDriver
	hostCfg.Name = hostName

	err := hostCfg.Read(hostName)
	if err != nil {
		log.Printf("error '%s' reading master host config name %s \n",
			err, hostName)
		return err
	}

	if hostCfg.VtepIP != "" {
		// walk through all nets and delete vtep eps as necessary
		readNet := &drivers.OvsCfgNetworkState{}
		readNet.StateDriver = stateDriver
		tenantNets, err := readNet.ReadAll()
		if err != nil {
			if !strings.Contains(err.Error(), "Key not found") {
				log.Printf("error '%s' eading keys during host create\n", err)
			}
		}
		for _, tenantNet := range tenantNets {
			nw := tenantNet.(*drivers.OvsCfgNetworkState)
			err = deleteVtep(stateDriver, nw.ID, hostName)
			if err != nil {
				log.Printf("error '%s' deleting vtep \n", err)
			}
		}
	}
	if hostCfg.Intf != "" {
		err = deleteVlanIf(stateDriver, hostName)
		if err != nil {
			log.Printf("error '%s' deleting infra if %s on host %s \n",
				err, hostName)
		}
	}

	err = hostCfg.Clear()
	if err != nil {
		log.Printf("error '%s' when deleting host config \n", err)
		return err
	}

	return err
}

// DeleteHost deletes a host by its ConfigHost state
func DeleteHost(stateDriver core.StateDriver, host *ConfigHost) error {
	return DeleteHostID(stateDriver, host.Name)
}

func validateNetworkConfig(tenant *ConfigTenant) error {
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
func CreateNetworks(stateDriver core.StateDriver, tenant *ConfigTenant) error {
	var extPktTag, pktTag uint

	gCfg := gstate.Cfg{}
	gCfg.StateDriver = stateDriver
	err := gCfg.Read(tenant.Name)
	if err != nil {
		log.Printf("error '%s' reading tenant cfg state \n", err)
		return err
	}

	// XXX: instead of initing resource-manager always, just init and
	// store it once. Also the type of resource-manager should be picked up
	// based on configuration.
	tempRa := &resources.EtcdResourceManager{Etcd: stateDriver}
	err = tempRa.Init()
	if err != nil {
		return err
	}
	ra := core.ResourceManager(tempRa)

	err = validateNetworkConfig(tenant)
	if err != nil {
		log.Printf("error '%s' validating network config \n", err)
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
		nwMasterCfg := &MasterNwConfig{}
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
				pktTag, err = gCfg.AllocVlan(ra)
				if err != nil {
					return err
				}
			} else if nwCfg.PktTagType == "vxlan" {
				extPktTag, pktTag, err = gCfg.AllocVxlan(ra)
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
			nwCfg.PktTag = int(pktTag)
			nwCfg.ExtPktTag, _ = strconv.Atoi(nwMasterCfg.PktTag)
		} else if nwMasterCfg.PktTagType == "vlan" {
			nwCfg.PktTag, _ = strconv.Atoi(nwMasterCfg.PktTag)
			// XXX: do configuration check, to make sure it is allowed
		}

		if nwCfg.SubnetIP == "" {
			nwCfg.SubnetLen = gCfg.Auto.AllocSubnetLen
			nwCfg.SubnetIP, err = gCfg.AllocSubnet(ra)
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
			log.Printf("error '%s' when writing nw config \n", err)
			return err
		}

		if nwCfg.PktTagType == "vxlan" {

			readHost := &MasterHostConfig{}
			readHost.StateDriver = stateDriver
			hostCfgs, err := readHost.ReadAll()
			if err != nil {
				if !strings.Contains(err.Error(), "Key not found") {
					log.Printf("error '%s' reading hosts during net add\n", err)
				}
			}
			for _, hostCfg := range hostCfgs {
				host := hostCfg.(*MasterHostConfig)
				err = createVtep(stateDriver, host, nwCfg.ID)
				if err != nil {
					log.Printf("error '%s' creating vtep \n", err)
				}
			}
		}
	}

	return err
}

func freeNetworkResources(stateDriver core.StateDriver, nwMasterCfg *MasterNwConfig,
	nwCfg *drivers.OvsCfgNetworkState, gCfg *gstate.Cfg) (err error) {

	// XXX: instead of initing resource-manager always, just init and
	// store it once. Also the type of resource-manager should be picked up
	// based on configuration.
	tempRa := &resources.EtcdResourceManager{Etcd: stateDriver}
	err = tempRa.Init()
	if err != nil {
		return err
	}
	ra := core.ResourceManager(tempRa)

	if nwCfg.PktTagType == "vlan" {
		err = gCfg.FreeVlan(ra, uint(nwCfg.PktTag))
		if err != nil {
			return err
		}
	} else if nwCfg.PktTagType == "vxlan" {
		log.Printf("freeing vlan %d vxlan %d \n", nwCfg.PktTag,
			nwCfg.ExtPktTag)
		err = gCfg.FreeVxlan(ra, uint(nwCfg.ExtPktTag), uint(nwCfg.PktTag))
		if err != nil {
			return err
		}
	}

	if nwMasterCfg.SubnetIP == "" {
		log.Printf("freeing subnet %s/%s \n", nwCfg.SubnetIP,
			nwCfg.SubnetLen)
		err = gCfg.FreeSubnet(ra, nwCfg.SubnetIP)
		if err != nil {
			return err
		}
	}

	return err
}

// DeleteNetworkID removes a network by ID.
func DeleteNetworkID(stateDriver core.StateDriver, netID string) error {
	nwMasterCfg := &MasterNwConfig{}
	nwMasterCfg.StateDriver = stateDriver
	err := nwMasterCfg.Read(netID)
	if err != nil {
		log.Printf("network not configured \n")
		return err
	}

	nwCfg := &drivers.OvsCfgNetworkState{}
	nwCfg.StateDriver = stateDriver
	err = nwCfg.Read(netID)
	if err != nil {
		log.Printf("network not operational \n")
		return err
	}

	gCfg := &gstate.Cfg{}
	gCfg.StateDriver = stateDriver
	err = gCfg.Read(nwMasterCfg.Tenant)
	if err != nil {
		log.Printf("error reading tenant info \n")
		return err
	}

	err = freeNetworkResources(stateDriver, nwMasterCfg, nwCfg, gCfg)
	if err != nil {
		return err
	}

	err = nwCfg.Clear()
	if err != nil {
		log.Printf("error '%s' when writing nw config \n", err)
		return err
	}

	return err
}

// DeleteNetworks removes all the virtual networks for a given tenant.
func DeleteNetworks(stateDriver core.StateDriver, tenant *ConfigTenant) error {
	gCfg := &gstate.Cfg{}
	gCfg.StateDriver = stateDriver

	err := gCfg.Read(tenant.Name)
	if err != nil {
		log.Printf("error '%s' reading tenant state \n", err)
		return err
	}

	err = validateNetworkConfig(tenant)
	if err != nil {
		log.Printf("error '%s' validating network config \n", err)
		return err
	}

	for _, network := range tenant.Networks {
		if len(network.Endpoints) > 0 {
			continue
		}
		nwMasterCfg := &MasterNwConfig{}
		nwMasterCfg.StateDriver = stateDriver
		err = nwMasterCfg.Read(network.Name)
		if err != nil {
			log.Printf("network not configured \n")
			continue
		}

		nwCfg := &drivers.OvsCfgNetworkState{}
		nwCfg.StateDriver = stateDriver
		err = nwCfg.Read(network.Name)
		if err != nil {
			log.Printf("network not operational \n")
			continue
		}

		err = freeNetworkResources(stateDriver, nwMasterCfg, nwCfg, gCfg)
		if err != nil {
			return err
		}

		err = nwCfg.Clear()
		if err != nil {
			log.Printf("error '%s' when writing nw config \n", err)
			return err
		}
	}

	return err
}

func validateEndpointConfig(stateDriver core.StateDriver, tenant *ConfigTenant) error {
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
				nwMasterCfg := &MasterNwConfig{}
				nwMasterCfg.StateDriver = stateDriver
				err = nwMasterCfg.Read(network.Name)
				if err != nil {
					log.Printf("validate: error '%s' reading network state \n",
						err)
					return err
				}
				if nwMasterCfg.SubnetIP != "" {
					log.Printf("validate: found endpoint with ip for " +
						"auto-allocated net \n")
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
func allocSetEpIP(ep *ConfigEP, epCfg *drivers.OvsCfgEndpointState,
	nwCfg *drivers.OvsCfgNetworkState) (err error) {

	var ipAddrValue uint
	var found bool

	ipAddress := ep.IPAddress
	if ipAddress == "" {
		if ipAddrValue, found = nwCfg.IPAllocMap.NextClear(0); !found {
			log.Printf("auto allocation failed - address exhaustion "+
				"in subnet %s/%d \n", nwCfg.SubnetIP, nwCfg.SubnetLen)
			return
		}
		ipAddress, err = netutils.GetSubnetIP(
			nwCfg.SubnetIP, nwCfg.SubnetLen, 32, ipAddrValue)
		if err != nil {
			log.Printf("create eps: error acquiring subnet ip '%s' \n",
				err)
			return
		}
	} else if ipAddress != "" && nwCfg.SubnetIP != "" {
		ipAddrValue, err = netutils.GetIPNumber(
			nwCfg.SubnetIP, nwCfg.SubnetLen, 32, ipAddress)
		if err != nil {
			log.Printf("create eps: error getting host id from hostIP "+
				"%s Subnet %s/%d err '%s'\n",
				ipAddress, nwCfg.SubnetIP, nwCfg.SubnetLen, err)
			return
		}
	}
	epCfg.IPAddress = ipAddress
	nwCfg.IPAllocMap.Set(ipAddrValue)

	return
}

// CreateEndpoints creates the endpoints for a given tenant.
func CreateEndpoints(stateDriver core.StateDriver, tenant *ConfigTenant) error {
	err := validateEndpointConfig(stateDriver, tenant)
	if err != nil {
		log.Printf("error '%s' validating network config \n", err)
		return err
	}

	for _, network := range tenant.Networks {
		nwMasterCfg := MasterNwConfig{}
		nwMasterCfg.StateDriver = stateDriver
		err = nwMasterCfg.Read(network.Name)
		if err != nil {
			log.Printf("create eps: error '%s' reading cfg network %s \n",
				err, network.Name)
			return err
		}

		nwCfg := &drivers.OvsCfgNetworkState{}
		nwCfg.StateDriver = stateDriver
		err = nwCfg.Read(network.Name)
		if err != nil {
			log.Printf("create eps: error '%s' reading oper network %s \n",
				err, network.Name)
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

			err = allocSetEpIP(&ep, epCfg, nwCfg)
			if err != nil {
				log.Printf("error '%s' allocating and/or reserving IP\n", err)
				return err
			}

			err = epCfg.Write()
			if err != nil {
				log.Printf("error '%s' when writing nw config \n", err)
				return err
			}
			nwCfg.EpCount++
		}

		err = nwCfg.Write()
		if err != nil {
			log.Printf("error '%s' when writing nw config \n", err)
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
		log.Printf("error getting host id from hostIP %s "+
			"Subnet %s/%d err '%s'\n",
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
		log.Printf("error '%s' when writing nw config \n", err)
		return err
	}

	err = nwCfg.Write()
	if err != nil {
		log.Printf("error '%s' when writing nw config \n", err)
		return err
	}

	return err
}

// DeleteEndpoints deletes the endpoints for the tenant.
func DeleteEndpoints(stateDriver core.StateDriver, tenant *ConfigTenant) error {

	err := validateEndpointConfig(stateDriver, tenant)
	if err != nil {
		log.Printf("error '%s' validating network config \n", err)
		return err
	}

	for _, network := range tenant.Networks {
		nwMasterCfg := &MasterNwConfig{}
		nwMasterCfg.StateDriver = stateDriver
		err = nwMasterCfg.Read(network.Name)
		if err != nil {
			log.Printf("error '%s' reading network state \n", err)
			return err
		}

		nwCfg := &drivers.OvsCfgNetworkState{}
		nwCfg.StateDriver = stateDriver
		err = nwCfg.Read(network.Name)
		if err != nil {
			log.Printf("error '%s' reading tenant state \n", err)
			return err
		}

		for _, ep := range network.Endpoints {
			epCfg := &drivers.OvsCfgEndpointState{}
			epCfg.StateDriver = stateDriver
			epCfg.ID = getEpName(&network, &ep)
			err = epCfg.Read(epCfg.ID)
			if err != nil {
				log.Printf("error '%s' getting cfg state of ep %s \n",
					err, epCfg.ID)
				continue
			}

			err = freeEndpointResources(epCfg, nwCfg)
			if err != nil {
				continue
			}

			err = epCfg.Clear()
			if err != nil {
				log.Printf("error '%s' when writing nw config \n", err)
				return err
			}
		}

		err = nwCfg.Write()
		if err != nil {
			log.Printf("error '%s' when writing nw config \n", err)
			return err
		}
	}

	return err
}

func validateEpBindings(epBindings *[]ConfigEP) error {
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
func CreateEpBindings(stateDriver core.StateDriver, epBindings *[]ConfigEP) error {

	err := validateEpBindings(epBindings)
	if err != nil {
		log.Printf("error '%s' validating the ep bindings \n", err)
		return err
	}

	readEp := &drivers.OvsCfgEndpointState{}
	readEp.StateDriver = stateDriver
	epCfgs, err := readEp.ReadAll()
	if err != nil {
		log.Printf("error '%s' fetching eps \n", err)
		return err
	}
	for _, ep := range *epBindings {
		log.Printf("creating binding between container '%s' and host '%s' \n",
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
				log.Printf("error '%s' updating epCfg \n", err)
				return err
			}
		}
	}

	return nil
}
