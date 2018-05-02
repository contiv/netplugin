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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/netplugin/utils/netutils"
)

// AddressAllocRequest is the address request from netplugin
type AddressAllocRequest struct {
	NetworkID            string // Unique identifier for the network
	AddressPool          string // Address pool from which to allocate the address
	PreferredIPv4Address string // Preferred address
}

// AddressAllocResponse is the response from netmaster
type AddressAllocResponse struct {
	NetworkID   string // Unique identifier for the network
	IPv4Address string // Allocated address
}

// AddressReleaseRequest is the release request from netplugin
type AddressReleaseRequest struct {
	NetworkID   string // Unique identifier for the network
	IPv4Address string // Allocated address
}
type AddressReleaseResponse struct {
	Status string
}

// CreateEndpointRequest has the endpoint create request from netplugin
type CreateEndpointRequest struct {
	TenantName   string          // tenant name
	NetworkName  string          // network name
	ServiceName  string          // service name
	EndpointID   string          // Unique identifier for the endpoint
	EPCommonName string          // Common name for the endpoint
	ConfigEP     intent.ConfigEP // Endpoint configuration
}

// CreateEndpointResponse has the endpoint create response from netmaster
type CreateEndpointResponse struct {
	EndpointConfig mastercfg.CfgEndpointState // Endpoint config
}

// DeleteEndpointRequest is the delete endpoint request from netplugin
type DeleteEndpointRequest struct {
	TenantName  string // tenant name
	NetworkName string // network name
	ServiceName string // service name
	EndpointID  string // Unique identifier for the endpoint
	IPv4Address string // Allocated IPv4 address for the endpoint
}

//UpdateEndpointRequest has the update endpoint request from netplugin
type UpdateEndpointRequest struct {
	IPAddress    string            // provider IP
	ContainerID  string            // container id
	Labels       map[string]string // lables
	Tenant       string
	Network      string
	Event        string
	EndpointID   string
	EPCommonName string // Common name for the endpoint
}

//UpdateEndpointResponse is service provider update request from netplugin
type UpdateEndpointResponse struct {
	IPAddress string // provider IP
}

// DeleteEndpointResponse is the delete endpoint response from netmaster
type DeleteEndpointResponse struct {
	EndpointConfig mastercfg.CfgEndpointState // Endpoint config
}

// Global mutex for address allocation
var addrMutex sync.Mutex

// getNwEpgFromAddrReq returns network/epg from addralloc request
func getNwAndEpgFromAddrReq(allocID string) (string, string) {
	nwList := strings.Split(allocID, ":")
	if len(nwList) == 2 { // allocReqId is nw:epg.tenant
		networkID := nwList[0]
		epgList := strings.Split(nwList[1], ".")
		if len(epgList) == 2 {
			epgName := epgList[0] + ":" + epgList[1]
			networkID = networkID + "." + epgList[1]
			return networkID, epgName
		}
	}
	return allocID, ""
}

// AllocAddressHandler allocates addresses
func AllocAddressHandler(w http.ResponseWriter, r *http.Request, vars map[string]string) (interface{}, error) {
	var allocReq AddressAllocRequest
	var epgCfg *mastercfg.EndpointGroupState
	var networkID string
	var epgName string

	// Get object from the request
	err := json.NewDecoder(r.Body).Decode(&allocReq)
	if err != nil {
		log.Errorf("Error decoding AllocAddressHandler. Err %v", err)
		return nil, err
	}

	log.Infof("Received AddressAllocRequest: %+v", allocReq)

	// Take a global lock for address allocation
	addrMutex.Lock()
	defer addrMutex.Unlock()

	// Get hold of the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return nil, err
	}

	isIPv6 := netutils.IsIPv6(allocReq.AddressPool)

	// Determine the network id to use
	if allocReq.NetworkID != "" {
		networkID, epgName = getNwAndEpgFromAddrReq(allocReq.NetworkID)
	} else {
		// find the network from address pool
		subnetIP := strings.Split(allocReq.AddressPool, "/")[0]
		subnetLen := strings.Split(allocReq.AddressPool, "/")[1]
		tenant := ""
		if strings.Contains(subnetLen, ":") {
			tenant = strings.Split(subnetLen, ":")[1]
			subnetLen = strings.Split(subnetLen, ":")[0]
		}

		// find the network from networkID
		readNet := &mastercfg.CfgNetworkState{}
		readNet.StateDriver = stateDriver
		netList, err := readNet.ReadAll()
		if err != nil {
			if !strings.Contains(err.Error(), "key not found") {
				log.Errorf("error reading keys during host create. Error: %s", err)
				return nil, err
			}
		}

		for _, ncfg := range netList {
			nw := ncfg.(*mastercfg.CfgNetworkState)
			if isIPv6 && nw.IPv6Subnet == subnetIP && fmt.Sprintf("%d", nw.IPv6SubnetLen) == subnetLen {
				if tenant == "" || nw.Tenant == tenant {
					networkID = nw.ID
				}
			} else if nw.SubnetIP == subnetIP && fmt.Sprintf("%d", nw.SubnetLen) == subnetLen {
				if tenant == "" || nw.Tenant == tenant {
					networkID = nw.ID
				}
			}
		}
	}

	if len(epgName) > 0 {
		epgCfg = &mastercfg.EndpointGroupState{}
		epgCfg.StateDriver = stateDriver
		if err = epgCfg.Read(epgName); err != nil {
			log.Errorf("failed to read epg %s, %s", epgName, err)
			return nil, err
		}
		log.Infof("AddressAllocRequest for network: %s epg: %s", networkID, epgName)
	}

	if networkID == "" {
		if GetClusterMode() == core.SwarmMode {
			// If the network was created using docker command,
			// we get allocReq before the network is created. Here,
			// we return the first IP in the subnet as gateway IP.
			// Subsequent createNetwork request will come with
			// a subnet and gateway which will allocate the gateway IP
			var ipAddress string
			var subnetLen uint
			subnetIP := strings.Split(allocReq.AddressPool, "/")[0]
			subnetLen = func() uint {
				p, err := strconv.Atoi(strings.Split(allocReq.AddressPool, "/")[1])
				if err != nil {
					log.Errorf("error acquiring subnet len. Error: %s", err)
				}
				return uint(p)
			}()
			if isIPv6 {
				// Get first available IPv6 address
				ipAddress, err = netutils.GetSubnetIPv6(subnetIP, subnetLen, "")
				if err != nil {
					log.Errorf("error acquiring subnet ip. Error: %s", err)
					return "", err
				}
			} else {
				// Get first available IPv4 address
				subnetAddr := netutils.GetSubnetAddr(subnetIP, subnetLen)
				ipAddress, err = netutils.GetSubnetIP(subnetAddr, subnetLen, 32, 1)
				if err != nil {
					log.Errorf("error acquiring subnet ip. Error: %s", err)
					return "", err
				}
			}
			// Build the response
			aresp := AddressAllocResponse{
				NetworkID:   allocReq.NetworkID,
				IPv4Address: ipAddress + "/" + fmt.Sprintf("%d", subnetLen),
			}

			return aresp, nil
		}
		log.Errorf("Could not find the network for: %s", allocReq.NetworkID)
		return nil, errors.New("network not found")
	}

	// find the network from network id
	nwCfg := &mastercfg.CfgNetworkState{}
	nwCfg.StateDriver = stateDriver
	err = nwCfg.Read(networkID)
	if err != nil {
		log.Errorf("network %s is not operational", networkID)
		return nil, err
	}

	// Alloc addresses
	addr, err := networkAllocAddress(nwCfg, epgCfg, allocReq.PreferredIPv4Address, netutils.IsIPv6(allocReq.AddressPool))
	if err != nil {
		log.Errorf("Failed to allocate address. Err: %v", err)
		return nil, err
	}

	var subnetLen uint
	if isIPv6 {
		subnetLen = nwCfg.IPv6SubnetLen
	} else {
		subnetLen = nwCfg.SubnetLen
	}

	// Build the response
	aresp := AddressAllocResponse{
		NetworkID:   allocReq.NetworkID,
		IPv4Address: addr + "/" + fmt.Sprintf("%d", subnetLen),
	}

	return aresp, nil
}

// ReleaseAddressHandler releases addresses
func ReleaseAddressHandler(w http.ResponseWriter, r *http.Request, vars map[string]string) (interface{}, error) {
	var relReq AddressReleaseRequest
	var networkID string
	var epgName string
	var epgCfg *mastercfg.EndpointGroupState

	// Get object from the request
	err := json.NewDecoder(r.Body).Decode(&relReq)
	if err != nil {
		log.Errorf("Error decoding ReleaseAddressHandler. Err %v", err)
		return nil, err
	}

	log.Infof("Received AddressReleaseRequest: %+v", relReq)

	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return nil, err
	}

	networkID, epgName = getNwAndEpgFromAddrReq(relReq.NetworkID)
	if len(epgName) > 0 {
		epgCfg = &mastercfg.EndpointGroupState{}
		epgCfg.StateDriver = stateDriver
		if err = epgCfg.Read(epgName); err != nil {
			log.Errorf("failed to read epg %s, %s", epgName, err)
			return nil, err
		}
		log.Infof("AddressReleaseRequest for network: %s epg: %s", networkID, epgName)
	}

	// find the network from network id
	nwCfg := &mastercfg.CfgNetworkState{}
	nwCfg.StateDriver = stateDriver
	err = nwCfg.Read(networkID)
	if err != nil {
		log.Errorf("network %s is not operational", relReq.NetworkID)
		return nil, err
	}

	// release addresses
	err = networkReleaseAddress(nwCfg, epgCfg, relReq.IPv4Address)
	if err != nil {
		log.Errorf("Failed to release address. Err: %v", err)
		return nil, err
	}

	return "success", nil
}

// CreateEndpointHandler handles create endpoint requests
func CreateEndpointHandler(w http.ResponseWriter, r *http.Request, vars map[string]string) (interface{}, error) {
	var epReq CreateEndpointRequest

	// Get object from the request
	err := json.NewDecoder(r.Body).Decode(&epReq)
	if err != nil {
		log.Errorf("Error decoding AllocAddressHandler. Err %v", err)
		return nil, err
	}

	log.Infof("Received CreateEndpointRequest: %+v", epReq)
	// Take a global lock for address allocation
	addrMutex.Lock()
	defer addrMutex.Unlock()

	// Gte the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return nil, err
	}

	// find the network from network id
	netID := epReq.NetworkName + "." + epReq.TenantName
	nwCfg := &mastercfg.CfgNetworkState{}
	nwCfg.StateDriver = stateDriver
	err = nwCfg.Read(netID)
	if err != nil {
		log.Errorf("network %s is not operational", netID)
		return nil, err
	}

	// Create the endpoint
	epCfg, err := CreateEndpoint(stateDriver, nwCfg, &epReq)
	if err != nil {
		log.Errorf("CreateEndpoint failure for ep: %v. Err: %v", epReq.ConfigEP, err)
		return nil, err
	}

	// build ep create response
	epResp := CreateEndpointResponse{
		EndpointConfig: *epCfg,
	}

	// return the response
	return epResp, nil
}

// DeleteEndpointHandler handles delete endpoint requests
func DeleteEndpointHandler(w http.ResponseWriter, r *http.Request, vars map[string]string) (interface{}, error) {
	var epdelReq DeleteEndpointRequest

	// Get object from the request
	err := json.NewDecoder(r.Body).Decode(&epdelReq)
	if err != nil {
		log.Errorf("Error decoding AllocAddressHandler. Err %v", err)
		return nil, err
	}

	log.Infof("Received DeleteEndpointRequest: %+v", epdelReq)

	// Get the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return nil, err
	}

	// Take a global lock for address release
	addrMutex.Lock()
	defer addrMutex.Unlock()

	// build the endpoint ID
	netID := epdelReq.NetworkName + "." + epdelReq.TenantName
	epID := getEpName(netID, &intent.ConfigEP{Container: epdelReq.EndpointID})

	// delete the endpoint
	epCfg, err := DeleteEndpointID(stateDriver, epID)
	if err != nil {
		log.Errorf("Error deleting endpoint: %v", epID)
		return nil, err
	}

	// build the response
	delResp := DeleteEndpointResponse{
		EndpointConfig: *epCfg,
	}

	// done. return resp
	return delResp, nil
}

//UpdateEndpointHandler handles update event from netplugin
func UpdateEndpointHandler(w http.ResponseWriter, r *http.Request, vars map[string]string) (interface{}, error) {

	var epUpdReq UpdateEndpointRequest

	// Get object from the request
	err := json.NewDecoder(r.Body).Decode(&epUpdReq)

	if err != nil {
		log.Errorf("Error decoding EndpointUpdateRequest. Err %v", err)
		return nil, err
	}

	log.Infof("Received EndpointUpdateRequest {%+v}", epUpdReq)

	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return nil, err
	}

	if epUpdReq.Event == "start" {
		//Received container start event from netplugin. Check if the Provider
		//matches any service and perform service provider update if there is a matching
		//service.

		epCfg := &mastercfg.CfgEndpointState{}
		epCfg.StateDriver = stateDriver

		nwCfg := &mastercfg.CfgNetworkState{}
		nwCfg.StateDriver = stateDriver
		//check if networkname is epg name or network name
		key := mastercfg.GetNwCfgKey(epUpdReq.Network, epUpdReq.Tenant)
		err := nwCfg.Read(key)
		if err != nil {
			if !strings.Contains(err.Error(), "key not found") {
				return nil, err
			}
			//If network is not found then networkname is epg
			epgCfg := &mastercfg.EndpointGroupState{}
			epgCfg.StateDriver = stateDriver
			key = mastercfg.GetEndpointGroupKey(epUpdReq.Network, epUpdReq.Tenant)
			err := epgCfg.Read(key)
			if err != nil {
				return nil, err
			}
			//get the network associated with the endpoint group
			key = mastercfg.GetNwCfgKey(epgCfg.NetworkName, epUpdReq.Tenant)
		}

		epCfg.ID = getEpName(key, &intent.ConfigEP{Container: epUpdReq.EndpointID})

		err = epCfg.Read(epCfg.ID)
		if err != nil {
			return nil, err
		}

		provider := &mastercfg.Provider{}
		provider.IPAddress = epUpdReq.IPAddress
		provider.Tenant = epUpdReq.Tenant
		provider.Network = epUpdReq.Network
		provider.ContainerID = epUpdReq.ContainerID
		provider.Labels = make(map[string]string)

		if epCfg.Labels == nil {
			//endpoint cfg doesnt have labels
			epCfg.Labels = make(map[string]string)
		}

		for k, v := range epUpdReq.Labels {
			provider.Labels[k] = v
			epCfg.Labels[k] = v
		}
		provider.EpIDKey = epCfg.ID
		//maintain the containerId in endpointstat for recovery
		epCfg.ContainerID = epUpdReq.ContainerID
		epCfg.EPCommonName = epUpdReq.EPCommonName

		err = epCfg.Write()
		if err != nil {
			log.Errorf("error writing ep config. Error: %s", err)
			return nil, err
		}

		providerID := getProviderID(provider)
		providerDbID := getProviderDbID(provider)
		if providerID == "" || providerDbID == "" {
			return nil, fmt.Errorf("invalid ProviderID from providerInfo:{%v}", provider)
		}

		//update provider db
		mastercfg.SvcMutex.Lock()
		mastercfg.ProviderDb[providerDbID] = provider

		for serviceID, service := range mastercfg.ServiceLBDb {
			count := 0
			if service.Tenant == epUpdReq.Tenant {
				for key, value := range epUpdReq.Labels {
					if val := service.Selectors[key]; val == value {
						count++
					}

					if count == len(service.Selectors) {
						//Container corresponds to the service since it
						//matches all service Selectors
						mastercfg.ProviderDb[providerDbID].Services =
							append(mastercfg.ProviderDb[providerDbID].Services, serviceID)
							//Update ServiceDB
						mastercfg.ServiceLBDb[serviceID].Providers[providerID] = provider

						serviceLbState := &mastercfg.CfgServiceLBState{}
						serviceLbState.StateDriver = stateDriver
						err = serviceLbState.Read(serviceID)
						if err != nil {
							mastercfg.SvcMutex.Unlock()
							return nil, err
						}
						serviceLbState.Providers[providerID] = provider
						serviceLbState.Write()
						SvcProviderUpdate(serviceID, false)
						break
					}
				}
			}
		}
		mastercfg.SvcMutex.Unlock()

	} else if epUpdReq.Event == "die" {
		//Received a container die event. If it was a service provider -
		//clear the provider db and the service db and change the etcd state

		providerDbID := epUpdReq.ContainerID
		if providerDbID == "" {
			return nil, fmt.Errorf("invalid containerID in UpdateEndpointRequest:(nil)")
		}

		mastercfg.SvcMutex.Lock()
		provider := mastercfg.ProviderDb[providerDbID]
		if provider == nil {
			mastercfg.SvcMutex.Unlock()
			// It is not a provider . Ignore event
			return nil, nil
		}

		for _, serviceID := range provider.Services {
			service := mastercfg.ServiceLBDb[serviceID]
			providerID := getProviderID(provider)
			if providerID == "" {
				mastercfg.SvcMutex.Unlock()
				return nil, fmt.Errorf("invalid ProviderID from providerInfo:{%v}", provider)
			}
			if service.Providers[providerID] != nil {
				delete(service.Providers, providerID)

				serviceLbState := &mastercfg.CfgServiceLBState{}
				serviceLbState.StateDriver = stateDriver
				err = serviceLbState.Read(serviceID)
				if err != nil {
					mastercfg.SvcMutex.Unlock()
					return nil, err
				}
				delete(serviceLbState.Providers, providerID)
				serviceLbState.Write()
				delete(mastercfg.ProviderDb, providerDbID)
				SvcProviderUpdate(serviceID, false)

			}
		}
		mastercfg.SvcMutex.Unlock()

	}

	epUpdResp := &UpdateEndpointResponse{
		IPAddress: epUpdReq.IPAddress,
	}
	return epUpdResp, nil
}
