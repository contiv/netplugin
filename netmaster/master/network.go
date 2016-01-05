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
	"github.com/contiv/netplugin/resources"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/netplugin/utils/netutils"
	"github.com/samalba/dockerclient"

	log "github.com/Sirupsen/logrus"
)

const (
	driverName        = "netplugin"
	defaultTenantName = "default"
)

var testMode = false

// Trim default tenant from network name
func getDocknetName(tenantName, networkName, serviceName string) string {
	if tenantName == defaultTenantName {
		if serviceName == "" {
			return fmt.Sprintf("%s", networkName)
		}

		return fmt.Sprintf("%s.%s", serviceName, networkName)
	}

	if serviceName == "" {
		return fmt.Sprintf("%s/%s", networkName, tenantName)
	}

	return fmt.Sprintf("%s.%s/%s", serviceName, networkName, tenantName)
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

// createDockNet Creates a network in docker daemon
func createDockNet(tenantName, networkName, serviceName, subnetCIDR, gateway string) error {
	// do nothing in test mode
	if testMode {
		return nil
	}

	// Trim default tenant name
	docknetName := getDocknetName(tenantName, networkName, serviceName)

	// connect to docker
	docker, err := dockerclient.NewDockerClient("unix:///var/run/docker.sock", nil)
	if err != nil {
		log.Errorf("Unable to connect to docker. Error %v", err)
		return errors.New("Unable to connect to docker")
	}

	// Check if the network already exists
	nw, err := docker.InspectNetwork(docknetName)
	if err == nil && nw.Driver == driverName {
		return nil
	} else if err == nil && nw.Driver != driverName {
		log.Errorf("Network name %s used by another driver %s", docknetName, nw.Driver)
		return errors.New("Network name used by another driver")
	}

	// Build network parameters
	nwCreate := dockerclient.NetworkCreate{
		Name:           docknetName,
		CheckDuplicate: true,
		Driver:         driverName,
		IPAM: dockerclient.IPAM{
			Driver: driverName,
			Config: []dockerclient.IPAMConfig{
				dockerclient.IPAMConfig{
					Subnet:  subnetCIDR,
					Gateway: gateway,
				},
			},
		},
	}

	log.Infof("Creating docker network: %+v", nwCreate)

	// Create network
	_, err = docker.CreateNetwork(&nwCreate)
	if err != nil {
		log.Errorf("Error creating network %s. Err: %v", docknetName, err)
		return err
	}

	return nil
}

// deleteDockNet deletes a network in docker daemon
func deleteDockNet(tenantName, networkName, serviceName string) error {
	// do nothing in test mode
	if testMode {
		return nil
	}

	// Trim default tenant name
	docknetName := getDocknetName(tenantName, networkName, serviceName)

	// connect to docker
	docker, err := dockerclient.NewDockerClient("unix:///var/run/docker.sock", nil)
	if err != nil {
		log.Errorf("Unable to connect to docker. Error %v", err)
		return errors.New("Unable to connect to docker")
	}

	log.Infof("Deleting docker network: %+v", docknetName)

	// Delete network
	err = docker.RemoveNetwork(docknetName)
	if err != nil {
		log.Errorf("Error deleting network %s. Err: %v", docknetName, err)
		// FIXME: Ignore errors till we fully move to docker 1.9
		return nil
	}

	return nil
}

// CreateNetwork creates a network from intent
func CreateNetwork(network intent.ConfigNetwork, stateDriver core.StateDriver, tenantName string) error {
	var extPktTag, pktTag uint

	gCfg := gstate.Cfg{}
	gCfg.StateDriver = stateDriver
	err := gCfg.Read(tenantName)
	if err != nil {
		log.Errorf("error reading tenant cfg state. Error: %s", err)
		return err
	}

	tempRm, err := resources.GetStateResourceManager()
	if err != nil {
		return err
	}
	rm := core.ResourceManager(tempRm)

	// Create network state
	networkID := network.Name + "." + tenantName
	nwCfg := &mastercfg.CfgNetworkState{}
	nwCfg.StateDriver = stateDriver
	if nwCfg.Read(networkID) == nil {
		// TODO: check if parameters changed and apply an update if needed
		return nil
	}

	subnetIP, subnetLen, _ := netutils.ParseCIDR(network.SubnetCIDR)

	// construct and update network state
	nwCfg = &mastercfg.CfgNetworkState{
		Tenant:      tenantName,
		NetworkName: network.Name,
		PktTagType:  network.PktTagType,
		SubnetIP:    subnetIP,
		SubnetLen:   subnetLen,
		Gateway:     network.Gateway,
	}

	nwCfg.ID = networkID
	nwCfg.StateDriver = stateDriver

	if network.PktTagType == "" {
		nwCfg.PktTagType = gCfg.Deploy.DefaultNetType
	}
	if network.PktTag == 0 {
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
	} else if network.PktTagType == "vxlan" {
		nwCfg.ExtPktTag = network.PktTag
		nwCfg.PktTag = network.PktTag
	} else if network.PktTagType == "vlan" {
		nwCfg.PktTag = network.PktTag
		// XXX: do configuration check, to make sure it is allowed
	}

	if nwCfg.SubnetIP == "" {
		nwCfg.SubnetLen = gCfg.Auto.AllocSubnetLen
		nwCfg.SubnetIP, err = gCfg.AllocSubnet(rm)
		if err != nil {
			return err
		}
		nwCfg.SubnetIsAllocated = true
	}

	defaultNwName, err := gCfg.AssignDefaultNetwork(network.Name)
	if err != nil {
		log.Errorf("error assigning the default network. Error: %s", err)
		return err
	}

	if network.Name == defaultNwName {
		// For auto derived subnets assign gateway ip be the last valid unicast ip the subnet
		if nwCfg.Gateway == "" && nwCfg.SubnetIsAllocated {
			var ipAddrValue uint
			ipAddrValue = (1 << (32 - nwCfg.SubnetLen)) - 2
			nwCfg.Gateway, err = netutils.GetSubnetIP(nwCfg.SubnetIP, nwCfg.SubnetLen, 32, ipAddrValue)
			if err != nil {
				return err
			}
			nwCfg.IPAllocMap.Set(ipAddrValue)
		}
	}

	netutils.InitSubnetBitset(&nwCfg.IPAllocMap, nwCfg.SubnetLen)
	err = nwCfg.Write()
	if err != nil {
		return err
	}

	// Create the network in docker
	//	subnetCIDR := fmt.Sprintf("%s/%d", nwCfg.SubnetIP, nwCfg.SubnetLen)
	//	err = createDockNet(tenantName, network.Name, "", subnetCIDR, nwCfg.Gateway)
	//	if err != nil {
	//		log.Errorf("Error creating network %s in docker. Err: %v", nwCfg.ID, err)
	//		return err
	//	}

	// Attach service container endpoint to the network
	//	err = attachServiceContainer(tenantName, network.Name, stateDriver)
	//	if err != nil {
	//		log.Errorf("Error attaching service container to network: %s. Err: %v",
	//			networkID, err)
	//		return err
	//	}

	return nil
}

func attachServiceContainer(tenantName, networkName string, stateDriver core.StateDriver) error {
	// do nothing in test mode
	if testMode {
		return nil
	}

	contName := tenantName + "dns"
	docker, err := utils.GetDockerClient()
	if err != nil {
		log.Errorf("Unable to connect to docker. Error %v", err)
		return err
	}

	// Trim default tenant
	dnetName := getDocknetName(tenantName, networkName, "")

	err = docker.ConnectNetwork(dnetName, contName)
	if err != nil {
		log.Errorf("Could not attach container(%s) to network %s. Error: %s",
			contName, dnetName, err)
		return err
	}

	// inspect the container
	cinfo, err := docker.InspectContainer(contName)
	if err != nil {
		log.Errorf("Error inspecting the container %s. Err: %v", contName, err)
		return err
	}

	log.Debugf("Container info: %+v\n Hostconfig: %+v", cinfo, cinfo.HostConfig)

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

	dnsContName := tenantName + "dns"
	cinfo, err := docker.InspectContainer(dnsContName)
	if err != nil {
		log.Errorf("Error inspecting the container %s. Err: %v", dnsContName, err)
		return err
	}

	// Trim default tenant
	dnetName := getDocknetName(tenantName, networkName, "")

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
		err = docker.DisconnectNetwork(dnetName, dnsContName)
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

	if nwCfg.SubnetIsAllocated {
		log.Infof("freeing subnet %s/%d", nwCfg.SubnetIP, nwCfg.SubnetLen)
		err = gCfg.FreeSubnet(rm, nwCfg.SubnetIP)
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

	// detach Dns container
	//	err = detachServiceContainer(nwCfg.Tenant, nwCfg.NetworkName)
	//	if err != nil {
	//		log.Errorf("Error detaching service container. Err: %v", err)
	//	}

	// Delete the docker network
	//	err = deleteDockNet(nwCfg.Tenant, nwCfg.NetworkName, "")
	//	if err != nil {
	//		log.Errorf("Error deleting network %s. Err: %v", netID, err)
	//	}

	gCfg := &gstate.Cfg{}
	gCfg.StateDriver = stateDriver
	err = gCfg.Read(nwCfg.Tenant)
	if err != nil {
		log.Errorf("error reading tenant info for %q. Error: %s", nwCfg.Tenant, err)
		return err
	}

	// Free resource associated with the network
	err = freeNetworkResources(stateDriver, nwCfg, gCfg)
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

		networkID := network.Name + "." + tenant.Name
		nwCfg := &mastercfg.CfgNetworkState{}
		nwCfg.StateDriver = stateDriver
		err = nwCfg.Read(networkID)
		if err != nil {
			log.Infof("network %q is not operational", network.Name)
			continue
		}

		err = freeNetworkResources(stateDriver, nwCfg, gCfg)
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

	nwCfg.IPAllocMap.Clear(ipAddrValue)
	nwCfg.EpCount--

	return nil
}
