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
	"net"
	"strings"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/docknet"
	"github.com/contiv/netplugin/netmaster/gstate"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/utils"
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

	// construct and update network state
	nwCfg = &mastercfg.CfgNetworkState{
		Tenant:      tenantName,
		NetworkName: network.Name,
		NwType:      network.NwType,
		PktTagType:  network.PktTagType,
		SubnetIP:    subnetIP,
		SubnetLen:   subnetLen,
	}

	nwCfg.ID = networkID
	nwCfg.StateDriver = stateDriver

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

	netutils.InitSubnetBitset(&nwCfg.IPAllocMap, nwCfg.SubnetLen)
	subnetAddr := netutils.GetSubnetAddr(nwCfg.SubnetIP, nwCfg.SubnetLen)

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
	nwCfg.SubnetIP = subnetAddr

	err = nwCfg.Write()
	if err != nil {
		return err
	}

	// Skip docker and service container configs for infra nw
	if network.NwType == "infra" {
		return nil
	}

	if GetClusterMode() == "docker" {
		// Create the network in docker
		err = docknet.CreateDockNet(tenantName, network.Name, "", nwCfg)
		if err != nil {
			log.Errorf("Error creating network %s in docker. Err: %v", nwCfg.ID, err)
			return err
		}
	}

	if IsDNSEnabled() {
		// Attach service container endpoint to the network
		err = attachServiceContainer(tenantName, network.Name, stateDriver)
		if err != nil {
			log.Errorf("Error attaching service container to network: %s. Err: %v",
				networkID, err)
			return err
		}
	}

	return nil
}

func attachServiceContainer(tenantName, networkName string, stateDriver core.StateDriver) error {
	contName := getDNSName(tenantName)
	docker, err := utils.GetDockerClient()
	if err != nil {
		log.Errorf("Unable to connect to docker. Error %v", err)
		return err
	}

	cinfo, err := docker.InspectContainer(contName)
	if err != nil {
		if strings.Contains(err.Error(), "no such id") {
			// DNS container not started for this tenant. Start skydns container
			err = startServiceContainer(tenantName)
			if err != nil {
				log.Warnf("Error starting service container. "+
					"Continuing without DNS provider. Error: %v", err)
				return nil
			}
			cinfo, err = docker.InspectContainer(contName)
			if err != nil {
				log.Warnf("Error fetching container info after starting %s"+
					"Continuing without DNS provider. Error: %s", contName, err)
				return nil
			}
		}
	}

	// If it's not in running state, restart the container.
	// This case can occur if the host is reloaded
	if !cinfo.State.Running {
		log.Debugf("Container %s not running. Restarting the container", contName)
		err = docker.RestartContainer(contName, 0)
		if err != nil {
			log.Warnf("Error restarting service container %s. "+
				"Continuing without DNS provider. Error: %v",
				contName, err)
			return nil
		}

		// Refetch container info after restart
		cinfo, err = docker.InspectContainer(contName)
		if err != nil {
			log.Warnf("Error fetching container info after restarting %s"+
				"Continuing without DNS provider. Error: %s", contName, err)
			return nil
		}
	}

	log.Debugf("Container info: %+v\n Hostconfig: %+v", cinfo, cinfo.HostConfig)

	// Trim default tenant
	dnetName := docknet.GetDocknetName(tenantName, networkName, "")

	err = docker.ConnectNetwork(dnetName, contName)
	if err != nil {
		log.Warnf("Could not attach container(%s) to network %s. "+
			"Continuing without DNS provider. Error: %s",
			contName, dnetName, err)
		return nil
	}

	ninfo, err := docker.InspectNetwork(dnetName)
	if err != nil {
		log.Errorf("Error getting network info for %s. Err: %v", dnetName, err)
		return err
	}

	log.Debugf("Network info: %+v", ninfo)

	// find the container in network info
	epInfo, ok := ninfo.Containers[cinfo.Id]
	if !ok {
		log.Errorf("Could not find container %s in network info", cinfo.Id)
		return errors.New("Endpoint not found")
	}

	// read network Config
	nwCfg := &mastercfg.CfgNetworkState{}
	networkID := networkName + "." + tenantName
	nwCfg.StateDriver = stateDriver
	err = nwCfg.Read(networkID)
	if err != nil {
		return err
	}

	// set the dns server Info
	nwCfg.DNSServer = strings.Split(epInfo.IPv4Address, "/")[0]
	log.Infof("Dns server for network %s: %s", networkName, nwCfg.DNSServer)

	// write the network config
	err = nwCfg.Write()
	if err != nil {
		return err
	}

	return nil
}

// detachServiceContainer detaches the service container's endpoint during network delete
//      - detach happens only if all other endpoints in the network are already removed
func detachServiceContainer(tenantName, networkName string) error {
	docker, err := utils.GetDockerClient()
	if err != nil {
		log.Errorf("Unable to connect to docker. Error %v", err)
		return errors.New("Unable to connect to docker")
	}

	dnsContName := getDNSName(tenantName)
	cinfo, err := docker.InspectContainer(dnsContName)
	if err != nil {
		log.Errorf("Error inspecting the container %s. Err: %v", dnsContName, err)
		return err
	}

	// Trim default tenant
	dnetName := docknet.GetDocknetName(tenantName, networkName, "")

	// inspect docker network
	nwState, err := docker.InspectNetwork(dnetName)
	if err != nil {
		log.Errorf("Error while inspecting network: %+v", dnetName)
		return err
	}

	log.Infof("Containers in network: %+v are {%+v}", dnetName, nwState.Containers)
	dnsServerIP := strings.Split(nwState.Containers[cinfo.Id].IPv4Address, "/")[0]

	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		log.Errorf("Could not get StateDriver while trying to disconnect dnsContainer from %+v", networkName)
		return err
	}

	// Read network config and get DNSServer information
	nwCfg := &mastercfg.CfgNetworkState{}
	nwCfg.StateDriver = stateDriver
	networkID := networkName + "." + tenantName
	err = nwCfg.Read(networkID)
	if err != nil {
		return err
	}

	log.Infof("dnsServerIP: %+v, nwCfg.dnsip: %+v", dnsServerIP, nwCfg.DNSServer)
	// Remove dns container from network if all other endpoints are withdrawn
	if len(nwState.Containers) == 1 && (dnsServerIP == nwCfg.DNSServer) {
		log.Infof("Disconnecting dns container from network as all other endpoints are removed: %+v", networkName)
		err = docker.DisconnectNetwork(dnetName, dnsContName, false)
		if err != nil {
			log.Errorf("Could not detach container(%s) from network %s. Error: %s",
				dnsContName, dnetName, err)
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

	if nwCfg.NwType != "infra" {
		// For Infra nw, endpoint delete initiated by netplugin
		// Check if there are any active endpoints
		if hasActiveEndpoints(nwCfg) {
			return core.Errorf("Error: Network has active endpoints")
		}

		if IsDNSEnabled() {
			// detach Dns container
			err = detachServiceContainer(nwCfg.Tenant, nwCfg.NetworkName)
			if err != nil {
				log.Errorf("Error detaching service container. Err: %v", err)
			}
		}

		if GetClusterMode() == "docker" {
			// Delete the docker network
			err = docknet.DeleteDockNet(nwCfg.Tenant, nwCfg.NetworkName, "")
			if err != nil {
				log.Errorf("Error deleting network %s. Err: %v", netID, err)
				// DeleteDockNet will fail when network has active endpoints.
				// No damage is done yet. It is safe to fail.
				// We do not have to call attachServiceContainer here,
				// as detachServiceContainer detaches only when there are no
				// endpoints remaining.
				return err
			}
		}
	}

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

// Allocate an address from the network
func networkAllocAddress(nwCfg *mastercfg.CfgNetworkState, reqAddr string) (string, error) {
	var ipAddress string
	var ipAddrValue uint
	var found bool
	var err error

	// alloc address
	if reqAddr == "" {
		ipAddrValue, found = nwCfg.IPAllocMap.NextClear(0)
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

		// Docker, Mesos issue a Alloc Address first, followed by a CreateEndpoint
		// Kubernetes issues a create endpoint directly
		// since networkAllocAddress is called from both AllocAddressHandler and CreateEndpointHandler,
		// we need to make sure that the EpCount is incremented only when we are allocating
		// a new IP. In case of Docker, Mesos CreateEndPoint will already request a IP that
		// allocateAddress had allocated in the earlier call.
		nwCfg.EpAddrCount++

	} else if reqAddr != "" && nwCfg.SubnetIP != "" {
		ipAddrValue, err = netutils.GetIPNumber(nwCfg.SubnetIP, nwCfg.SubnetLen, 32, reqAddr)
		if err != nil {
			log.Errorf("create eps: error getting host id from hostIP %s Subnet %s/%d. Error: %s",
				reqAddr, nwCfg.SubnetIP, nwCfg.SubnetLen, err)
			return "", err
		}

		ipAddress = reqAddr
	}

	// Set the bitmap
	nwCfg.IPAllocMap.Set(ipAddrValue)

	err = nwCfg.Write()
	if err != nil {
		log.Errorf("error writing nw config. Error: %s", err)
		return "", err
	}

	return ipAddress, nil
}

// networkReleaseAddress release the ip address
func networkReleaseAddress(nwCfg *mastercfg.CfgNetworkState, ipAddress string) error {
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

	err = nwCfg.Write()
	if err != nil {
		log.Errorf("error writing nw config. Error: %s", err)
		return err
	}

	return nil
}

func hasActiveEndpoints(nwCfg *mastercfg.CfgNetworkState) bool {
	// We spin a dns container if IsDNSEnabled() == true
	// We need to exlude that from Active EPs check.
	return (IsDNSEnabled() && nwCfg.EpCount > 1) || ((!IsDNSEnabled()) && nwCfg.EpCount > 0)
}
