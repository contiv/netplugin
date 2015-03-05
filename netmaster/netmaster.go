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

// netmaster  - implements the network intent translation to plugin
// events; uses state distribution to achieve intent realization
// netmaster runs as a logically centralized unit on in the cluster

package netmaster

import (
	"errors"
	"log"
	"net"
	"strconv"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/gstate"
	"github.com/contiv/netplugin/netutils"
)

// Config structs define the config intent for various network entities
type ConfigEp struct {
	Container string
	Host      string
	IpAddress string

	// XXX: need to think more, if interface name really belongs to logical
	// config. One usecase for having interface name in logical config might be
	// the SRIOV case, where the virtual interfaces could be pre-exist.
	Intf string
}

type ConfigNetwork struct {
	Name string

	// overrides for various functions when auto allocation is not desired
	PktTagType string
	PktTag     string
	SubnetCIDR string
	DefaultGw  string

	// eps associated with the network
	Endpoints []ConfigEp
}

type ConfigTenant struct {
	Name           string
	DefaultNetType string
	SubnetPool     string
	AllocSubnetLen uint
	Vlans          string
	Vxlans         string

	Networks []ConfigNetwork
}

type Config struct {
	Tenants []ConfigTenant
}

// interface that cluster manager implements; this is external interface to
// the cluster manager
type netmasterIf interface {
	Init(cfg *Config) error
	CreateNetwork(net *ConfigNetwork) error
	DeleteNetwork(netid string) error
	CreateEndpoint(ep *ConfigEp) error
	DeleteEndpoint(epid string) error
}

// Implementation of the above interface
// TODO remove reference to statedriver being passed as an argument,
// instead create and maintain the state

func Init(stateDriver *core.StateDriver, cfg *Config) error {
	return nil
}

func checkPktTagType(pktTagType string) error {
	if pktTagType != "" && pktTagType != "vlan" && pktTagType != "vxlan" {
		return errors.New("invalid pktTagType")
	}

	return nil
}

func validateTenantConfig(tenant *ConfigTenant) error {
	if tenant.Name == "" {
		return errors.New("invalid tenant name")
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

func CreateTenant(stateDriver core.StateDriver, tenant *ConfigTenant) error {

	gOper := &gstate.Oper{}
	err := gOper.Read(stateDriver, tenant.Name)
	if err == nil {
		return err
	}

	err = validateTenantConfig(tenant)
	if err != nil {
		return err
	}

	var gCfg gstate.Cfg
	gCfg.Version = gstate.VersionBeta1
	gCfg.Tenant = tenant.Name
	gCfg.Deploy.DefaultNetType = tenant.DefaultNetType
	gCfg.Auto.SubnetPool, gCfg.Auto.SubnetLen, _ = netutils.ParseCIDR(tenant.SubnetPool)
	gCfg.Auto.Vlans = tenant.Vlans
	gCfg.Auto.Vxlans = tenant.Vxlans
	gCfg.Auto.AllocSubnetLen = tenant.AllocSubnetLen
	err = gCfg.Update(stateDriver)
	if err != nil {
		log.Printf("error '%s' updating tenant '%s' \n", err, tenant.Name)
	}

	gOper, err = gCfg.Process()
	if err != nil {
		log.Printf("Error '%s' updating the config %v \n", err, gCfg)
		return err
	}

	err = gOper.Update(stateDriver)
	if err != nil {
		log.Printf("error '%s' updating goper state %v \n", err, gOper)
	}

	return err
}

func DeleteTenantId(stateDriver core.StateDriver, tenantId string) error {
	gOper := gstate.Oper{}
	err := gOper.Read(stateDriver, tenantId)
	if err != nil {
		log.Printf("error '%s' reading tenant info '%s' \n", err, tenantId)
		return err
	}

	gCfg := gstate.Cfg{}
	gCfg.Version = gstate.VersionBeta1
	gCfg.Tenant = tenantId
	err = gCfg.Clear(stateDriver)
	if err != nil {
		log.Printf("error '%s' deleting cfg for tenant '%s' \n", err, tenantId)
	}

	err = gOper.Clear(stateDriver)
	if err != nil {
		log.Printf("error '%s' deleting oper for tenant '%s' \n", err, tenantId)
	}

	return err
}

func DeleteTenant(stateDriver core.StateDriver, tenant *ConfigTenant) error {
	err := validateTenantConfig(tenant)
	if err != nil {
		return err
	}

	return DeleteTenantId(stateDriver, tenant.Name)
}

func validateNetworkConfig(tenant *ConfigTenant) error {
	var err error

	if tenant.Name == "" {
		errors.New("null tenant name")
	}

	for _, network := range tenant.Networks {
		if network.Name == "" {
			errors.New("null network name")
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
				return errors.New("invalid IP")
			}
		}
	}

	return err
}

func CreateNetworks(stateDriver core.StateDriver, tenant *ConfigTenant) error {
	var extPktTag, pktTag uint

	gCfg := gstate.Cfg{}
	err := gCfg.Read(stateDriver, tenant.Name)
	if err != nil {
		log.Printf("error '%s' reading tenant cfg state \n", err)
		return err
	}

	// TODO: acquire distributed lock before updating gOper update
	gOper := gstate.Oper{}
	err = gOper.Read(stateDriver, tenant.Name)
	if err != nil {
		log.Printf("error '%s' reading tenant oper state \n", err)
		return err
	}

	err = validateNetworkConfig(tenant)
	if err != nil {
		log.Printf("error '%s' validating network config \n", err)
		return err
	}

	for _, network := range tenant.Networks {
		nwCfg := &drivers.OvsCfgNetworkState{StateDriver: stateDriver}
		if nwCfg.Read(network.Name) == nil {
			// TODO: check if parameters changed and apply an update if needed
			continue
		}

		// construct and update network state
		nwMasterCfg := &MasterNwConfig{StateDriver: stateDriver}
		nwMasterCfg.Tenant = tenant.Name
		nwMasterCfg.Id = network.Name
		nwMasterCfg.PktTagType = network.PktTagType
		nwMasterCfg.PktTag = network.PktTag
		nwMasterCfg.SubnetIp, nwMasterCfg.SubnetLen, _ = netutils.ParseCIDR(network.SubnetCIDR)
		nwMasterCfg.DefaultGw = network.DefaultGw

		nwCfg = &drivers.OvsCfgNetworkState{StateDriver: stateDriver,
			Id: nwMasterCfg.Id, Tenant: nwMasterCfg.Tenant,
			PktTagType: nwMasterCfg.PktTagType,
			SubnetIp:   nwMasterCfg.SubnetIp, SubnetLen: nwMasterCfg.SubnetLen}

		if nwMasterCfg.PktTagType == "" {
			nwCfg.PktTagType = gOper.DefaultNetType
		}
		if nwMasterCfg.PktTag == "" {
			if nwCfg.PktTagType == "vlan" {
				pktTag, err = gOper.AllocVlan()
				if err != nil {
					return err
				}
			} else if gOper.DefaultNetType == "vxlan" {
				extPktTag, pktTag, err = gOper.AllocVxlan()
				if err != nil {
					return err
				}
			}

			log.Printf("allocated vlan %d vxlan %d \n", pktTag, extPktTag)
			nwCfg.ExtPktTag = int(extPktTag)
			nwCfg.PktTag = int(pktTag)
		} else if nwCfg.PktTagType == "vxlan" {
			pktTag, err = gOper.AllocLocalVlan()
			if err != nil {
				return err
			}
			nwCfg.PktTag = int(pktTag)
			nwCfg.ExtPktTag, _ = strconv.Atoi(nwMasterCfg.PktTag)
		} else if nwMasterCfg.PktTagType == "vlan" {
			nwCfg.PktTag, _ = strconv.Atoi(nwMasterCfg.PktTag)
			gOper.SetVlan(uint(nwCfg.PktTag))
		}

		if nwCfg.SubnetIp == "" {
			nwCfg.SubnetLen = gCfg.Auto.AllocSubnetLen
			nwCfg.SubnetIp, err = gOper.AllocSubnet()
			if err != nil {
				return err
			}
		}

		nwCfg.DefaultGw = network.DefaultGw
		if nwCfg.DefaultGw == "" {
			// TBD: allocate per global policy
		}

		netutils.InitSubnetBitset(&nwCfg.IpAllocMap, nwCfg.SubnetLen)
		err = nwCfg.Write()
		if err != nil {
			return err
		}

		err = nwMasterCfg.Write()
		if err != nil {
			log.Printf("error '%s' when writing nw config \n", err)
			return err
		}
	}

	err = gOper.Update(stateDriver)
	if err != nil {
		log.Printf("error updating the global state - %s \n", err)
		return err
	}

	return err
}

func freeNetworkResources(nwMasterCfg *MasterNwConfig,
	nwCfg *drivers.OvsCfgNetworkState, gOper *gstate.Oper) (err error) {
	if nwCfg.PktTagType == "vlan" {
		err = gOper.FreeVlan(uint(nwCfg.PktTag))
		if err != nil {
			return err
		}
	} else if gOper.DefaultNetType == "vxlan" {
		log.Printf("freeing vlan %d vxlan %d \n", nwCfg.PktTag,
			nwCfg.ExtPktTag)
		err = gOper.FreeVxlan(uint(nwCfg.ExtPktTag), uint(nwCfg.PktTag))
		if err != nil {
			return err
		}
	}

	if nwMasterCfg.SubnetIp == "" {
		log.Printf("freeing subnet %s/%s \n", nwCfg.SubnetIp,
			nwCfg.SubnetLen)
		err = gOper.FreeSubnet(nwCfg.SubnetIp)
		if err != nil {
			return err
		}
	}

	return err
}

func DeleteNetworkId(stateDriver core.StateDriver, netId string) error {
	nwMasterCfg := &MasterNwConfig{StateDriver: stateDriver}
	err := nwMasterCfg.Read(netId)
	if err != nil {
		log.Printf("network not configured \n")
		return err
	}

	nwCfg := &drivers.OvsCfgNetworkState{StateDriver: stateDriver}
	err = nwCfg.Read(netId)
	if err != nil {
		log.Printf("network not operational \n")
		return err
	}

	gCfg := gstate.Cfg{}
	err = gCfg.Read(stateDriver, nwMasterCfg.Tenant)
	if err != nil {
		log.Printf("error reading tenant info \n")
		return err
	}

	gOper := gstate.Oper{}
	err = gOper.Read(stateDriver, nwMasterCfg.Tenant)
	if err != nil {
		log.Printf("error reading tenant info \n")
		return err
	}

	err = freeNetworkResources(nwMasterCfg, nwCfg, &gOper)
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

func DeleteNetworks(stateDriver core.StateDriver, tenant *ConfigTenant) error {
	var gCfg gstate.Cfg
	var gOper gstate.Oper

	err := gCfg.Read(stateDriver, tenant.Name)
	if err != nil {
		log.Printf("error '%s' reading tenant state \n", err)
		return err
	}

	// TODO: acquire distributed lock before updating gOper update
	err = gOper.Read(stateDriver, tenant.Name)
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
		nwMasterCfg := &MasterNwConfig{StateDriver: stateDriver}
		err = nwMasterCfg.Read(network.Name)
		if err != nil {
			log.Printf("network not configured \n")
			continue
		}

		nwCfg := &drivers.OvsCfgNetworkState{StateDriver: stateDriver}
		err = nwCfg.Read(network.Name)
		if err != nil {
			log.Printf("network not operational \n")
			continue
		}

		err = freeNetworkResources(nwMasterCfg, nwCfg, &gOper)
		if err != nil {
			return err
		}

		err = nwCfg.Clear()
		if err != nil {
			log.Printf("error '%s' when writing nw config \n", err)
			return err
		}
	}

	err = gOper.Update(stateDriver)
	if err != nil {
		log.Printf("error updating the global state - %s \n", err)
		return err
	}

	return err
}

func validateEndpointConfig(stateDriver core.StateDriver, tenant *ConfigTenant) error {
	var err error

	if tenant.Name == "" {
		errors.New("null tenant name")
	}

	for _, network := range tenant.Networks {
		if network.Name == "" {
			errors.New("null network name")
		}

		for _, ep := range network.Endpoints {
			if ep.IpAddress != "" {
				nwMasterCfg := &MasterNwConfig{StateDriver: stateDriver}
				err = nwMasterCfg.Read(network.Name)
				if err != nil {
					log.Printf("validate: error '%s' reading network state \n",
						err)
					return err
				}
				if nwMasterCfg.SubnetIp != "" {
					log.Printf("validate: found endpoint with ip for " +
						"auto-allocated net \n")
					return errors.New("found ep with ip for auto-allocated net")
				}
				if net.ParseIP(ep.IpAddress) == nil {
					return errors.New("invalid ep IP")
				}
			}
		}
	}

	return err
}

func getEpName(net *ConfigNetwork, ep *ConfigEp) string {
	if ep.Container != "" {
		return net.Name + "-" + ep.Container
	} else {
		return ep.Host + "-native-intf"
	}
}

func CreateEndpoints(stateDriver core.StateDriver, tenant *ConfigTenant) error {
	var ipAddrValue uint = 0
	var found bool

	err := validateEndpointConfig(stateDriver, tenant)
	if err != nil {
		log.Printf("error '%s' validating network config \n", err)
		return err
	}

	for _, network := range tenant.Networks {
		nwMasterCfg := MasterNwConfig{StateDriver: stateDriver}
		err = nwMasterCfg.Read(network.Name)
		if err != nil {
			log.Printf("create eps: error '%s' reading cfg network %s \n",
				err, network.Name)
			return err
		}

		nwCfg := &drivers.OvsCfgNetworkState{StateDriver: stateDriver}
		err = nwCfg.Read(network.Name)
		if err != nil {
			log.Printf("create eps: error '%s' reading oper network %s \n",
				err, network.Name)
			return err
		}

		for _, ep := range network.Endpoints {
			epCfg := &drivers.OvsCfgEndpointState{StateDriver: stateDriver}
			epCfg.Id = getEpName(&network, &ep)
			err = epCfg.Read(epCfg.Id)
			if err == nil {
				// TODO: check for diffs and possible updates
				continue
			}

			epCfg.NetId = network.Name
			epCfg.ContName = ep.Container
			epCfg.HomingHost = ep.Host
			epCfg.IntfName = ep.Intf
			// epCfg.VtepIp = ep.vtepIp

			ipAddress := ep.IpAddress
			if ipAddress == "" {
				if ipAddrValue, found = nwCfg.IpAllocMap.NextClear(0); !found {
					log.Printf("auto allocation failed - address exhaustion "+
						"in subnet %s/%d \n", nwCfg.SubnetIp, nwCfg.SubnetLen)
					return err
				}
				ipAddress, err = netutils.GetSubnetIp(
					nwCfg.SubnetIp, nwCfg.SubnetLen, 32, ipAddrValue)
				if err != nil {
					log.Printf("create eps: error acquiring subnet ip '%s' \n",
						err)
					return err
				}
				log.Printf("ep %s was allocated ip address %s \n",
					epCfg.Id, ipAddress)
			} else if ipAddress != "" && nwCfg.SubnetIp != "" {
				ipAddrValue, err = netutils.GetIpNumber(
					nwCfg.SubnetIp, nwCfg.SubnetLen, 32, ipAddress)
				if err != nil {
					log.Printf("create eps: error getting host id from hostIp "+
						"%s Subnet %s/%d err '%s'\n",
						ipAddress, nwCfg.SubnetIp, nwCfg.SubnetLen, err)
					return err
				}
			}
			epCfg.IpAddress = ipAddress
			nwCfg.IpAllocMap.Set(ipAddrValue)

			err = epCfg.Write()
			if err != nil {
				log.Printf("error '%s' when writing nw config \n", err)
				return err
			}
			nwCfg.EpCount += 1
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
	ipAddrValue, err := netutils.GetIpNumber(
		nwCfg.SubnetIp, nwCfg.SubnetLen, 32, epCfg.IpAddress)
	if err != nil {
		log.Printf("error getting host id from hostIp %s "+
			"Subnet %s/%d err '%s'\n",
			epCfg.IpAddress, nwCfg.SubnetIp, nwCfg.SubnetLen, err)
		return err
	}
	nwCfg.IpAllocMap.Clear(ipAddrValue)
	nwCfg.EpCount -= 1

	return nil
}

func DeleteEndpointId(stateDriver core.StateDriver, epId string) error {
	epCfg := &drivers.OvsCfgEndpointState{StateDriver: stateDriver}
	err := epCfg.Read(epId)
	if err != nil {
		return err
	}

	nwCfg := &drivers.OvsCfgNetworkState{StateDriver: stateDriver}
	err = nwCfg.Read(epCfg.NetId)
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

func DeleteEndpoints(stateDriver core.StateDriver, tenant *ConfigTenant) error {

	err := validateEndpointConfig(stateDriver, tenant)
	if err != nil {
		log.Printf("error '%s' validating network config \n", err)
		return err
	}

	for _, network := range tenant.Networks {
		nwMasterCfg := &MasterNwConfig{StateDriver: stateDriver}
		err = nwMasterCfg.Read(network.Name)
		if err != nil {
			log.Printf("error '%s' reading network state \n", err)
			return err
		}

		nwCfg := &drivers.OvsCfgNetworkState{StateDriver: stateDriver}
		err = nwCfg.Read(network.Name)
		if err != nil {
			log.Printf("error '%s' reading tenant state \n", err)
			return err
		}

		for _, ep := range network.Endpoints {
			epCfg := &drivers.OvsCfgEndpointState{StateDriver: stateDriver}
			epCfg.Id = getEpName(&network, &ep)
			err = epCfg.Read(epCfg.Id)
			if err != nil {
				log.Printf("error '%s' getting cfg state of ep %s \n",
					err, epCfg.Id)
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
