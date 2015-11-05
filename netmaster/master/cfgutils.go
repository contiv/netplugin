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
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/gstate"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/utils"

	log "github.com/Sirupsen/logrus"
)

func getEpName(networkName string, ep *intent.ConfigEP) string {
	if ep.Container != "" {
		return networkName + "-" + ep.Container
	}

	return ep.Host + "-native-intf"
}

func hostPresent(allCfg *intent.Config, hostName string) bool {
	for _, host := range allCfg.Hosts {
		if hostName == host.Name {
			return true
		}
	}

	return false
}

func tenantPresent(allCfg *intent.Config, tenantID string) bool {
	for _, tenant := range allCfg.Tenants {
		if tenantID == tenant.Name {
			return true
		}
	}

	return false
}

func netPresent(allCfg *intent.Config, netID string) bool {
	for _, tenant := range allCfg.Tenants {
		for _, net := range tenant.Networks {
			if netID == (net.Name + "." + tenant.Name) {
				return true
			}
		}
	}

	return false
}

func epPresent(allCfg *intent.Config, epID string) bool {
	for _, tenant := range allCfg.Tenants {
		for _, net := range tenant.Networks {
			for _, ep := range net.Endpoints {
				if epID == getEpName(net.Name, &ep) {
					return true
				}
			}
		}
	}

	return false
}

// DeleteDelta deletes any existing configuration from netmaster's statestore
// that is not present in the configuration passed. This may result in
// generating Delete triggers for the netplugin
func DeleteDelta(allCfg *intent.Config) error {
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	readEp := &mastercfg.CfgEndpointState{}
	readEp.StateDriver = stateDriver
	epCfgs, err := readEp.ReadAll()
	if core.ErrIfKeyExists(err) != nil {
		return err
	} else if err != nil {
		err = nil
		epCfgs = []core.State{}
	}
	for _, epCfg := range epCfgs {
		cfg := epCfg.(*mastercfg.CfgEndpointState)
		if !epPresent(allCfg, cfg.ID) {
			_, err1 := DeleteEndpointID(stateDriver, cfg.ID)
			if err1 != nil {
				log.Errorf("error '%s' deleting epid %s \n", err1, cfg.ID)
				err = err1
				continue
			}
		}
	}

	readNet := &mastercfg.CfgNetworkState{}
	readNet.StateDriver = stateDriver
	nwCfgs, err := readNet.ReadAll()
	if core.ErrIfKeyExists(err) != nil {
		return err
	} else if err != nil {
		err = nil
		nwCfgs = []core.State{}
	}
	for _, nwCfg := range nwCfgs {
		cfg := nwCfg.(*mastercfg.CfgNetworkState)
		if !netPresent(allCfg, cfg.ID) {
			err1 := DeleteNetworkID(stateDriver, cfg.ID)
			if err1 != nil {
				log.Errorf("error '%s' deleting net %s \n", err1, cfg.ID)
				err = err1
				continue
			}
		}
	}

	readGlbl := &gstate.Cfg{}
	readGlbl.StateDriver = stateDriver
	gCfgs, err := readGlbl.ReadAll()
	if core.ErrIfKeyExists(err) != nil {
		return err
	} else if err != nil {
		err = nil
		gCfgs = []core.State{}
	}
	for _, gCfg := range gCfgs {
		cfg := gCfg.(*gstate.Cfg)
		if !tenantPresent(allCfg, cfg.Tenant) {
			err1 := DeleteTenantID(stateDriver, cfg.Tenant)
			if err1 != nil {
				log.Errorf("error '%s' deleting tenant %s \n", err1, cfg.Tenant)
				err = err1
				continue
			}
		}
	}

	return err
}

// ProcessAdditions adds the configuration passed to netmaster's statestore.
// This may result in generating Add/Create triggers for the netplugin.
func ProcessAdditions(allCfg *intent.Config) (err error) {
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	for _, tenant := range allCfg.Tenants {
		err1 := CreateTenant(stateDriver, &tenant)
		if err1 != nil {
			log.Errorf("error adding tenant '%s' \n", err1)
			err = err1
			continue
		}

		err1 = CreateNetworks(stateDriver, &tenant)
		if err1 != nil {
			log.Errorf("error adding networks '%s' \n", err1)
			err = err1
			continue
		}

		err1 = CreateEndpoints(stateDriver, &tenant)
		if err1 != nil {
			log.Errorf("error adding endpoints '%s' \n", err1)
			err = err1
			continue
		}
	}

	return
}

// ProcessDeletions deletes the configuration passed from netmaster's statestore.
// This may result in generating Delete triggers for the netplugin.
func ProcessDeletions(allCfg *intent.Config) (err error) {
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	for _, tenant := range allCfg.Tenants {
		err1 := DeleteEndpoints(stateDriver, &tenant)
		if err1 != nil {
			log.Errorf("error deleting endpoints '%s' \n", err1)
			err = err1
			continue
		}

		err1 = DeleteNetworks(stateDriver, &tenant)
		if err1 != nil {
			log.Errorf("error deleting networks '%s' \n", err1)
			err = err1
			continue
		}

		err1 = DeleteTenant(stateDriver, &tenant)
		if err1 != nil {
			log.Errorf("error deleting tenant '%s' \n", err1)
			err = err1
			continue
		}
	}

	return
}
