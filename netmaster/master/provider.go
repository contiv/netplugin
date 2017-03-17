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
	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/utils"
)

//SvcProviderUpdate propagates service provider updates to netplugins
func SvcProviderUpdate(serviceID string, isDelete bool) error {
	providerList := []string{}

	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	svcProvider := &mastercfg.SvcProvider{}
	svcProvider.StateDriver = stateDriver

	if _, present := mastercfg.ServiceLBDb[serviceID]; !present {
		svcProvider.ID = serviceID
		return svcProvider.Clear()
	}

	for _, provider := range mastercfg.ServiceLBDb[serviceID].Providers {
		providerList = append(providerList, provider.IPAddress)
	}

	//empty the current provider list
	svcProvider.Providers = nil
	//update to the latest provider list
	svcProvider.Providers = append(svcProvider.Providers, providerList...)
	svcProvider.ServiceName = serviceID
	svcProvider.ID = serviceID

	log.Infof("Updating service providers with {%v} on service %s", svcProvider.Providers, serviceID)

	return svcProvider.Write()
}

func getProviderID(provider *mastercfg.Provider) string {
	return provider.IPAddress + ":" + provider.Tenant
}

func getProviderDbID(provider *mastercfg.Provider) string {
	return provider.ContainerID
}
