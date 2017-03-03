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
	"reflect"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/utils"
)

//CreateServiceLB adds to the etcd state
func CreateServiceLB(stateDriver core.StateDriver, serviceLbCfg *intent.ConfigServiceLB) error {

	var providersPresent bool
	serviceIP := serviceLbCfg.IPAddress

	log.Infof("Recevied Create Service Load Balancer config {%v}", serviceLbCfg)

	//Check if service already exists.
	svcID := GetServiceID(serviceLbCfg.ServiceName, serviceLbCfg.Tenant)

	mastercfg.SvcMutex.RLock()
	oldServiceInfo := mastercfg.ServiceLBDb[svcID]
	mastercfg.SvcMutex.RUnlock()

	if oldServiceInfo != nil {
		//ServiceInfo Exists
		if reflect.DeepEqual(oldServiceInfo.Ports, serviceLbCfg.Ports) &&
			reflect.DeepEqual(oldServiceInfo.Selectors, serviceLbCfg.Selectors) &&
			serviceLbCfg.Tenant == oldServiceInfo.Tenant {
			return nil
		}
		serviceIP = oldServiceInfo.IPAddress
		DeleteServiceLB(stateDriver, oldServiceInfo.ServiceName, oldServiceInfo.Tenant)
	}

	//New Service
	serviceLbState := &mastercfg.CfgServiceLBState{}
	serviceLbState.ServiceName = serviceLbCfg.ServiceName
	serviceLbState.Tenant = serviceLbCfg.Tenant
	serviceLbState.Network = serviceLbCfg.Network
	serviceLbState.StateDriver = stateDriver
	serviceLbState.ID = GetServiceID(serviceLbCfg.ServiceName, serviceLbCfg.Tenant)
	serviceLbState.Ports = append(serviceLbState.Ports, serviceLbCfg.Ports...)
	serviceLbState.Selectors = make(map[string]string)
	serviceLbState.Providers = make(map[string]*mastercfg.Provider)
	for k, v := range serviceLbCfg.Selectors {
		serviceLbState.Selectors[k] = v
	}

	// find the network from network id
	networkID := serviceLbState.Network + "." + serviceLbState.Tenant
	nwCfg := &mastercfg.CfgNetworkState{}
	nwCfg.StateDriver = stateDriver
	err := nwCfg.Read(networkID)
	if err != nil {
		log.Errorf("network %s on tenant %s is not created %s", serviceLbState.Network, serviceLbCfg.Tenant, networkID)
		return err
	}

	// Alloc addresses
	addr, err := networkAllocAddress(nwCfg, nil, serviceIP, false)
	if err != nil {
		log.Errorf("Failed to allocate address. Err: %v", err)
		return err
	}
	serviceLbState.IPAddress = addr
	mastercfg.SvcMutex.Lock()
	err = serviceLbState.Write()

	if err != nil {
		mastercfg.SvcMutex.Unlock()
		return err
	}

	serviceID := GetServiceID(serviceLbState.ServiceName, serviceLbState.Tenant)

	mastercfg.ServiceLBDb[serviceID] = &mastercfg.ServiceLBInfo{
		IPAddress:   serviceLbState.IPAddress,
		Tenant:      serviceLbState.Tenant,
		ServiceName: serviceLbState.ServiceName,
		Network:     serviceLbState.Network,
	}
	mastercfg.ServiceLBDb[serviceID].Ports = append(mastercfg.ServiceLBDb[serviceID].Ports, serviceLbState.Ports...)
	mastercfg.ServiceLBDb[serviceID].Selectors = make(map[string]string)
	mastercfg.ServiceLBDb[serviceID].Providers = make(map[string]*mastercfg.Provider)

	for k, v := range serviceLbCfg.Selectors {
		mastercfg.ServiceLBDb[serviceID].Selectors[k] = v
	}

	//Check for containers in the tenant matching service selectors
	for _, providerInfo := range mastercfg.ProviderDb {
		if providerInfo.Tenant == serviceLbState.Tenant {
			if eq := reflect.DeepEqual(providerInfo.Labels, mastercfg.ServiceLBDb[serviceID].Selectors); eq {
				//provider matches service selectors
				providerID := getProviderID(providerInfo)
				providerDbID := getProviderDbID(providerInfo)
				providerInfo.Services = append(providerInfo.Services, serviceID)
				mastercfg.ServiceLBDb[serviceID].Providers[providerID] = providerInfo
				mastercfg.ProviderDb[providerDbID] = providerInfo
				serviceLbState.Providers[providerID] = providerInfo
				providersPresent = true
			}
		}
	}

	//Write into cluster store
	err = serviceLbState.Write()

	if err != nil {
		mastercfg.SvcMutex.Unlock()
		return err
	}

	if providersPresent {
		err = SvcProviderUpdate(serviceID, false)
		if err != nil {
			log.Errorf("Error updating Provider for service %s : %s", serviceID, err)
			return err
		}
	}
	mastercfg.SvcMutex.Unlock()

	return nil
}

//DeleteServiceLB deletes from etcd state
func DeleteServiceLB(stateDriver core.StateDriver, serviceName string, tenantName string) error {

	log.Infof("Received Delete Service Load Balancer %s on %s", serviceName, tenantName)
	serviceLBState := &mastercfg.CfgServiceLBState{}
	serviceLBState.StateDriver = stateDriver
	serviceLBState.ID = GetServiceID(serviceName, tenantName)

	mastercfg.SvcMutex.RLock()
	err := serviceLBState.Read(serviceLBState.ID)
	if err != nil {
		mastercfg.SvcMutex.RUnlock()
		log.Errorf("Error reading service lb config for service %s in tenant %s", serviceName, tenantName)
		return err
	}
	mastercfg.SvcMutex.RUnlock()

	// find the network from network id
	nwCfg := &mastercfg.CfgNetworkState{}
	nwCfg.StateDriver = stateDriver
	networkID := serviceLBState.Network + "." + serviceLBState.Tenant
	err = nwCfg.Read(networkID)
	if err != nil {
		log.Errorf("network %s is not operational. Service object deletion failed", networkID)
		return err
	}
	err = networkReleaseAddress(nwCfg, nil, serviceLBState.IPAddress)
	if err != nil {
		log.Errorf("Network release address  failed %s", err)
	}

	serviceID := GetServiceID(serviceLBState.ServiceName, serviceLBState.Tenant)

	mastercfg.SvcMutex.Lock()
	//Remove the service ID from the provider cache
	for _, providerInfo := range mastercfg.ServiceLBDb[serviceID].Providers {
		containerID := providerInfo.ContainerID
		for i, service := range mastercfg.ProviderDb[containerID].Services {
			if service == serviceID {
				mastercfg.ProviderDb[containerID].Services =
					append(mastercfg.ProviderDb[containerID].Services[:i],
						mastercfg.ProviderDb[containerID].Services[i+1:]...)
			}
		}
	}
	//Remove the service from the service cache
	delete(mastercfg.ServiceLBDb, serviceID)

	SvcProviderUpdate(serviceID, true)

	err = serviceLBState.Clear()
	if err != nil {
		mastercfg.SvcMutex.Unlock()

		log.Errorf("Error deleing service lb config for service %s in tenant %s", serviceName, tenantName)
		return err
	}
	mastercfg.SvcMutex.Unlock()

	return nil

}

//RestoreServiceProviderLBDb restores provider and servicelb db
func RestoreServiceProviderLBDb() {

	log.Infof("Restoring ProviderDb and ServiceDB cache")

	svcLBState := &mastercfg.CfgServiceLBState{}
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		log.Errorf("Error Restoring Service and ProviderDb Err:%s", err)
		return
	}
	svcLBState.StateDriver = stateDriver
	svcLBCfgs, err := svcLBState.ReadAll()

	if err == nil {
		mastercfg.SvcMutex.Lock()
		for _, svcLBCfg := range svcLBCfgs {
			svcLB := svcLBCfg.(*mastercfg.CfgServiceLBState)
			//mastercfg.ServiceLBDb = make(map[string]*mastercfg.ServiceLBInfo)
			serviceID := GetServiceID(svcLB.ServiceName, svcLB.Tenant)
			mastercfg.ServiceLBDb[serviceID] = &mastercfg.ServiceLBInfo{
				IPAddress:   svcLB.IPAddress,
				Tenant:      svcLB.Tenant,
				ServiceName: svcLB.ServiceName,
				Network:     svcLB.Network,
			}
			mastercfg.ServiceLBDb[serviceID].Ports = append(mastercfg.ServiceLBDb[serviceID].Ports, svcLB.Ports...)

			mastercfg.ServiceLBDb[serviceID].Selectors = make(map[string]string)
			mastercfg.ServiceLBDb[serviceID].Providers = make(map[string]*mastercfg.Provider)

			for k, v := range svcLB.Selectors {
				mastercfg.ServiceLBDb[serviceID].Selectors[k] = v
			}

			for providerID, providerInfo := range svcLB.Providers {
				mastercfg.ServiceLBDb[serviceID].Providers[providerID] = providerInfo
				providerDBId := providerInfo.ContainerID
				mastercfg.ProviderDb[providerDBId] = providerInfo
			}
		}
		mastercfg.SvcMutex.Unlock()
	}

	//Recover from endpoint state as well .
	epCfgState := mastercfg.CfgEndpointState{}
	epCfgState.StateDriver = stateDriver
	epCfgs, err := epCfgState.ReadAll()
	if err == nil {
		for _, epCfg := range epCfgs {
			ep := epCfg.(*mastercfg.CfgEndpointState)
			providerDBId := ep.ContainerID
			if ep.Labels != nil && mastercfg.ProviderDb[providerDBId] == nil {
				//Create provider info and store it in provider db
				providerInfo := &mastercfg.Provider{}
				providerInfo.ContainerID = ep.ContainerID
				providerInfo.Network = strings.Split(ep.NetID, ".")[0]
				providerInfo.Tenant = strings.Split(ep.NetID, ".")[1]
				providerInfo.Labels = make(map[string]string)
				providerInfo.IPAddress = ep.IPAddress

				for k, v := range ep.Labels {
					providerInfo.Labels[k] = v
				}
				mastercfg.SvcMutex.Lock()
				mastercfg.ProviderDb[providerDBId] = providerInfo
				mastercfg.SvcMutex.Unlock()
			}
		}
	}
}

//GetServiceID returns service id for etcd lookup
func GetServiceID(servicename string, tenantname string) string {
	return servicename + ":" + tenantname
}
