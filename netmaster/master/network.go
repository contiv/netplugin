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
	"strings"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/docknet"
	"github.com/contiv/netplugin/netmaster/gstate"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/utils/netutils"

	log "github.com/Sirupsen/logrus"
)

func checkPktTagType(pktTagType string) error {
	if pktTagType != "" && pktTagType != "vlan" && pktTagType != "vxlan" {
		return core.Errorf("invalid pktTagType")
	}

	return nil
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

		if network.SubnetCIDR != "" {
			_, _, err = netutils.ParseCIDR(network.SubnetCIDR)
			if err != nil {
				return err
			}
		}

		if network.Gateway != "" {
			if net.ParseIP(network.Gateway) == nil {
				return core.Errorf("invalid IP")
			}
		}
	}

	return err
}

// CreateNetwork creates a network from intent
func CreateNetwork(network intent.ConfigNetwork, stateDriver core.StateDriver, tenantName string) error {
	var extPktTag, pktTag uint

	gstate.GlobalMutex.Lock()
	defer gstate.GlobalMutex.Unlock()
	gCfg := gstate.Cfg{}
	gCfg.StateDriver = stateDriver
	err := gCfg.Read("")
	if err != nil {
		log.Errorf("error reading tenant cfg state. Error: %s", err)
		return err
	}

	// Create network state
	networkID := network.Name + "." + tenantName
	nwCfg := &mastercfg.CfgNetworkState{}
	nwCfg.StateDriver = stateDriver
	if nwCfg.Read(networkID) == nil {
		// TODO: check if parameters changed and apply an update if needed
		return nil
	}

	subnetIP, subnetLen, _ := netutils.ParseCIDR(network.SubnetCIDR)
	err = netutils.ValidateNetworkRangeParams(subnetIP, subnetLen)
	if err != nil {
		return err
	}

	ipv6Subnet, ipv6SubnetLen, _ := netutils.ParseCIDR(network.IPv6SubnetCIDR)

	// if there is no label given generate one for the network
	nwTag := network.CfgdTag
	if nwTag == "" {
		nwTag = network.Name + "." + tenantName
	}

	// construct and update network state
	nwCfg = &mastercfg.CfgNetworkState{
		Tenant:        tenantName,
		NetworkName:   network.Name,
		NwType:        network.NwType,
		PktTagType:    network.PktTagType,
		SubnetIP:      subnetIP,
		SubnetLen:     subnetLen,
		IPv6Subnet:    ipv6Subnet,
		IPv6SubnetLen: ipv6SubnetLen,
		NetworkTag:    nwTag,
	}

	nwCfg.ID = networkID
	nwCfg.StateDriver = stateDriver

	netutils.InitSubnetBitset(&nwCfg.IPAllocMap, nwCfg.SubnetLen)
	subnetAddr := netutils.GetSubnetAddr(nwCfg.SubnetIP, nwCfg.SubnetLen)
	nwCfg.SubnetIP = subnetAddr
	nwCfg.IPAddrRange = netutils.GetIPAddrRange(subnetIP, subnetLen)

	if network.Gateway != "" {
		nwCfg.Gateway = network.Gateway

		// Reserve gateway IP address if gateway is specified
		ipAddrValue, err := netutils.GetIPNumber(subnetAddr, nwCfg.SubnetLen, 32, nwCfg.Gateway)
		if err != nil {
			log.Errorf("Error parsing gateway address %s. Err: %v", nwCfg.Gateway, err)
			return err
		}
		nwCfg.IPAllocMap.Set(ipAddrValue)
	}

	if strings.Contains(subnetIP, "-") {
		netutils.SetBitsOutsideRange(&nwCfg.IPAllocMap, subnetIP, subnetLen)
	}

	if network.IPv6Gateway != "" {
		nwCfg.IPv6Gateway = network.IPv6Gateway

		// Reserve gateway IPv6 address if gateway is specified
		hostID, err := netutils.GetIPv6HostID(nwCfg.IPv6Subnet, nwCfg.IPv6SubnetLen, nwCfg.IPv6Gateway)
		if err != nil {
			log.Errorf("Error parsing gateway address %s. Err: %v", nwCfg.IPv6Gateway, err)
			return err
		}
		netutils.ReserveIPv6HostID(hostID, &nwCfg.IPv6AllocMap)
	}

	// Allocate pkt tags
	reqPktTag := uint(network.PktTag)
	if nwCfg.PktTagType == "vlan" {
		pktTag, err = gCfg.AllocVLAN(reqPktTag)
		if err != nil {
			return err
		}
	} else if nwCfg.PktTagType == "vxlan" {
		extPktTag, pktTag, err = gCfg.AllocVXLAN(reqPktTag)
		if err != nil {
			return err
		}
	}

	nwCfg.ExtPktTag = int(extPktTag)
	nwCfg.PktTag = int(pktTag)

	err = nwCfg.Write()
	if err != nil {
		return err
	}

	// Skip docker and service container configs for infra nw
	if network.NwType == "infra" {
		return nil
	}

	aci, _ := IsAciConfigured()
	if aci {
		// Skip docker network creation for ACI fabric mode.
		return nil
	}

	if GetClusterMode() == core.Docker {
		// Create the network in docker
		err = docknet.CreateDockNet(tenantName, network.Name, "", nwCfg)
		if err != nil {
			log.Errorf("Error creating network %s in docker. Err: %v", nwCfg.ID, err)
			return err
		}
	}

	return nil
}

// CreateNetworks creates the necessary virtual networks for the tenant
// provided by ConfigTenant.
func CreateNetworks(stateDriver core.StateDriver, tenant *intent.ConfigTenant) error {
	// Validate the config
	err := validateNetworkConfig(tenant)
	if err != nil {
		log.Errorf("error validating network config. Error: %s", err)
		return err
	}

	for _, network := range tenant.Networks {
		err = CreateNetwork(network, stateDriver, tenant.Name)
		if err != nil {
			log.Errorf("Error creating network {%+v}. Err: %v", network, err)
			return err
		}
	}

	return err
}

func freeNetworkResources(stateDriver core.StateDriver, nwCfg *mastercfg.CfgNetworkState, gCfg *gstate.Cfg) (err error) {
	if nwCfg.PktTagType == "vlan" {
		err = gCfg.FreeVLAN(uint(nwCfg.PktTag))
		if err != nil {
			return err
		}
	} else if nwCfg.PktTagType == "vxlan" {
		log.Infof("freeing vlan %d vxlan %d", nwCfg.PktTag, nwCfg.ExtPktTag)
		err = gCfg.FreeVXLAN(uint(nwCfg.ExtPktTag), uint(nwCfg.PktTag))
		if err != nil {
			return err
		}
	}

	if err := gCfg.UnassignNetwork(nwCfg.ID); err != nil {
		return err
	}

	return err
}

// DeleteNetworkID removes a network by ID.
func DeleteNetworkID(stateDriver core.StateDriver, netID string) error {
	nwCfg := &mastercfg.CfgNetworkState{}
	nwCfg.StateDriver = stateDriver
	err := nwCfg.Read(netID)
	if err != nil {
		log.Errorf("network %s is not operational", netID)
		return err
	}

	// Will Skip docker network deletion for ACI fabric mode.
	aci, _ := IsAciConfigured()

	if nwCfg.NwType != "infra" {
		// For Infra nw, endpoint delete initiated by netplugin
		// Check if there are any active endpoints
		if hasActiveEndpoints(nwCfg) {
			return core.Errorf("Error: Network has active endpoints")
		}

		if GetClusterMode() == core.Docker && aci == false {
			// Delete the docker network
			err = docknet.DeleteDockNet(nwCfg.Tenant, nwCfg.NetworkName, "")
			if err != nil {
				log.Errorf("Error deleting network %s. Err: %v", netID, err)
				// DeleteDockNet will fail when network has active endpoints.
				// No damage is done yet. It is safe to fail.
				return err
			}
		}
	}

	gstate.GlobalMutex.Lock()
	defer gstate.GlobalMutex.Unlock()
	gCfg := &gstate.Cfg{}
	gCfg.StateDriver = stateDriver
	err = gCfg.Read("")
	if err != nil {
		log.Errorf("error reading tenant info for %q. Error: %s", nwCfg.Tenant, err)
		return err
	}

	// Free resource associated with the network
	err = freeNetworkResources(stateDriver, nwCfg, gCfg)
	if err != nil {
		// Error while freeing up vlan/vxlan/subnet/gateway resources
		// This can only happen because of defects in code
		// No need of any corrective handling here
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

	err := gCfg.Read("")
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
		networkID := network.Name + "." + tenant.Name
		err = DeleteNetworkID(stateDriver, networkID)
		if err != nil {
			return err
		}
	}

	return err
}

// ListAllocatedIPs returns a string of allocated IPs in a network
func ListAllocatedIPs(nwCfg *mastercfg.CfgNetworkState) string {
	return netutils.ListAllocatedIPs(nwCfg.IPAllocMap, nwCfg.IPAddrRange, nwCfg.SubnetIP, nwCfg.SubnetLen)
}

// ListAvailableIPs returns a string of available IPs in a network
func ListAvailableIPs(nwCfg *mastercfg.CfgNetworkState) string {
	return netutils.ListAvailableIPs(nwCfg.IPAllocMap, nwCfg.SubnetIP, nwCfg.SubnetLen)
}

// Allocate an address from the network
func networkAllocAddress(nwCfg *mastercfg.CfgNetworkState, epgCfg *mastercfg.EndpointGroupState,
	reqAddr string, isIPv6 bool) (string, error) {
	var ipAddress string
	var ipAddrValue uint
	var found bool
	var err error
	var hostID string

	// alloc address
	if reqAddr == "" {
		if isIPv6 {
			// Get the next available IPv6 address
			hostID, err = netutils.GetNextIPv6HostID(nwCfg.IPv6LastHost, nwCfg.IPv6Subnet, nwCfg.IPv6SubnetLen, nwCfg.IPv6AllocMap)
			if err != nil {
				log.Errorf("create eps: error allocating ip. Error: %s", err)
				return "", err
			}
			ipAddress, err = netutils.GetSubnetIPv6(nwCfg.IPv6Subnet, nwCfg.IPv6SubnetLen, hostID)
			if err != nil {
				log.Errorf("create eps: error acquiring subnet ip. Error: %s", err)
				return "", err
			}
			nwCfg.IPv6LastHost = hostID
			netutils.ReserveIPv6HostID(hostID, &nwCfg.IPv6AllocMap)
		} else {
			if epgCfg != nil && len(epgCfg.IPPool) > 0 { // allocate from epg network
				log.Infof("allocating ip address from epg pool %s", epgCfg.IPPool)
				ipAddrValue, found = netutils.NextClear(epgCfg.EPGIPAllocMap, 0, nwCfg.SubnetLen)
				if !found {
					log.Errorf("auto allocation failed - address exhaustion in pool %s",
						epgCfg.IPPool)
					err = core.Errorf("auto allocation failed - address exhaustion in pool %s",
						epgCfg.IPPool)
					return "", err
				}
				ipAddress, err = netutils.GetSubnetIP(nwCfg.SubnetIP, nwCfg.SubnetLen, 32, ipAddrValue)
				if err != nil {
					log.Errorf("create eps: error acquiring subnet ip. Error: %s", err)
					return "", err
				}
				epgCfg.EPGIPAllocMap.Set(ipAddrValue)
			} else {
				ipAddrValue, found = netutils.NextClear(nwCfg.IPAllocMap, 0, nwCfg.SubnetLen)
				if !found {
					log.Errorf("auto allocation failed - address exhaustion in subnet %s/%d",
						nwCfg.SubnetIP, nwCfg.SubnetLen)
					err = core.Errorf("auto allocation failed - address exhaustion in subnet %s/%d",
						nwCfg.SubnetIP, nwCfg.SubnetLen)
					return "", err
				}
				ipAddress, err = netutils.GetSubnetIP(nwCfg.SubnetIP, nwCfg.SubnetLen, 32, ipAddrValue)
				if err != nil {
					log.Errorf("create eps: error acquiring subnet ip. Error: %s", err)
					return "", err
				}
				nwCfg.IPAllocMap.Set(ipAddrValue)
			}
		}

		// Docker, Mesos issue a Alloc Address first, followed by a CreateEndpoint
		// Kubernetes issues a create endpoint directly
		// since networkAllocAddress is called from both AllocAddressHandler and CreateEndpointHandler,
		// we need to make sure that the EpCount is incremented only when we are allocating
		// a new IP. In case of Docker, Mesos CreateEndPoint will already request a IP that
		// allocateAddress had allocated in the earlier call.
		nwCfg.EpAddrCount++

	} else if reqAddr != "" && nwCfg.SubnetIP != "" {
		if isIPv6 {
			hostID, err = netutils.GetIPv6HostID(nwCfg.IPv6Subnet, nwCfg.IPv6SubnetLen, reqAddr)
			if err != nil {
				log.Errorf("create eps: error getting host id from hostIP %s Subnet %s/%d. Error: %s",
					reqAddr, nwCfg.IPv6Subnet, nwCfg.IPv6SubnetLen, err)
				return "", err
			}
			netutils.ReserveIPv6HostID(hostID, &nwCfg.IPv6AllocMap)
		} else {

			if epgCfg != nil && len(epgCfg.IPPool) > 0 { // allocate from epg network
				ipAddrValue, err = netutils.GetIPNumber(nwCfg.SubnetIP, nwCfg.SubnetLen, 32, reqAddr)
				if err != nil {
					log.Errorf("create eps: error getting host id from hostIP %s pool %s. Error: %s",
						reqAddr, epgCfg.IPPool, err)
					return "", err
				}
				epgCfg.EPGIPAllocMap.Set(ipAddrValue)
			} else {
				ipAddrValue, err = netutils.GetIPNumber(nwCfg.SubnetIP, nwCfg.SubnetLen, 32, reqAddr)
				if err != nil {
					log.Errorf("create eps: error getting host id from hostIP %s Subnet %s/%d. Error: %s",
						reqAddr, nwCfg.SubnetIP, nwCfg.SubnetLen, err)
					return "", err
				}
				nwCfg.IPAllocMap.Set(ipAddrValue)
			}
		}

		ipAddress = reqAddr
	}

	if epgCfg != nil && len(epgCfg.IPPool) > 0 {
		err = epgCfg.Write()
		if err != nil {
			log.Errorf("error writing epg config. Error: %s", err)
			return "", err
		}
	}

	err = nwCfg.Write()
	if err != nil {
		log.Errorf("error writing nw config. Error: %s", err)
		return "", err
	}

	return ipAddress, nil
}

// networkReleaseAddress release the ip address
func networkReleaseAddress(nwCfg *mastercfg.CfgNetworkState, epgCfg *mastercfg.EndpointGroupState, ipAddress string) error {
	isIPv6 := netutils.IsIPv6(ipAddress)
	if isIPv6 {
		hostID, err := netutils.GetIPv6HostID(nwCfg.SubnetIP, nwCfg.SubnetLen, ipAddress)
		if err != nil {
			log.Errorf("error getting host id from hostIP %s Subnet %s/%d. Error: %s",
				ipAddress, nwCfg.SubnetIP, nwCfg.SubnetLen, err)
			return err
		}
		// networkReleaseAddress is called from multiple places
		// Make sure we decrement the EpCount only if the IPAddress
		// was not already freed earlier
		if _, found := nwCfg.IPv6AllocMap[hostID]; found {
			nwCfg.EpAddrCount--
		}
		delete(nwCfg.IPv6AllocMap, hostID)
	} else {
		if epgCfg != nil && len(epgCfg.IPPool) > 0 {
			log.Infof("releasing epg ip: %s", ipAddress)
			ipAddrValue, err := netutils.GetIPNumber(nwCfg.SubnetIP, nwCfg.SubnetLen, 32, ipAddress)
			if err != nil {
				log.Errorf("error getting host id from hostIP %s pool %s. Error: %s",
					ipAddress, epgCfg.IPPool, err)
				return err
			}
			// networkReleaseAddress is called from multiple places
			// Make sure we decrement the EpCount only if the IPAddress
			// was not already freed earlier
			if epgCfg.EPGIPAllocMap.Test(ipAddrValue) {
				nwCfg.EpAddrCount--
			}
			epgCfg.EPGIPAllocMap.Clear(ipAddrValue)
			if err := epgCfg.Write(); err != nil {
				log.Errorf("error writing epg config. Error: %s", err)
				return err
			}

		} else {
			ipAddrValue, err := netutils.GetIPNumber(nwCfg.SubnetIP, nwCfg.SubnetLen, 32, ipAddress)
			if err != nil {
				log.Errorf("error getting host id from hostIP %s Subnet %s/%d. Error: %s",
					ipAddress, nwCfg.SubnetIP, nwCfg.SubnetLen, err)
				return err
			}
			// networkReleaseAddress is called from multiple places
			// Make sure we decrement the EpCount only if the IPAddress
			// was not already freed earlier
			if nwCfg.IPAllocMap.Test(ipAddrValue) {
				nwCfg.EpAddrCount--
			}
			nwCfg.IPAllocMap.Clear(ipAddrValue)
			log.Infof("Releasing IP Address: %v"+
				"from networkId:%+v", ipAddrValue,
				nwCfg.NetworkName)
		}
	}
	err := nwCfg.Write()
	if err != nil {
		log.Errorf("error writing nw config. Error: %s", err)
		return err
	}

	return nil
}

func hasActiveEndpoints(nwCfg *mastercfg.CfgNetworkState) bool {
	return nwCfg.EpCount > 0
}
