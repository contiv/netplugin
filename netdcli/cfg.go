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

package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"time"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/gstate"
	"github.com/contiv/netplugin/netmaster"
)

func getEpName(net *netmaster.ConfigNetwork, ep *netmaster.ConfigEp) string {
	if ep.Container != "" {
		return net.Name + "-" + ep.Container
	} else {
		return ep.Host + "-native-intf"
	}
}

func postProcessing() {
	time.Sleep(1 * time.Second)
}

func hostPresent(allCfg *netmaster.Config, hostName string) bool {
	for _, host := range allCfg.Hosts {
		if hostName == host.Name {
			return true
		}
	}

	return false
}

func tenantPresent(allCfg *netmaster.Config, tenantId string) bool {
	for _, tenant := range allCfg.Tenants {
		if tenantId == tenant.Name {
			return true
		}
	}

	return false
}

func netPresent(allCfg *netmaster.Config, netId string) bool {
	for _, tenant := range allCfg.Tenants {
		for _, net := range tenant.Networks {
			if netId == net.Name {
				return true
			}
		}
	}

	return false
}

func epPresent(allCfg *netmaster.Config, epId string) bool {
	for _, tenant := range allCfg.Tenants {
		for _, net := range tenant.Networks {
			for _, ep := range net.Endpoints {
				if epId == getEpName(&net, &ep) {
					return true
				}
			}
		}
	}

	return false
}

func deleteDelta(stateDriver core.StateDriver, allCfg *netmaster.Config) error {

	epCfgs, err := drivers.ReadAllOvsCfgEndpoints(stateDriver)
	if err != nil {
		return core.ErrIfKeyExists(err)
	}
	for _, epCfg := range epCfgs {
		if !epPresent(allCfg, epCfg.Id) {
			err = netmaster.DeleteEndpointId(stateDriver, epCfg.Id)
			if err != nil {
				log.Printf("error '%s' deleting epid %s \n", err, epCfg.Id)
				continue
			}
		}
	}

	nwCfgs := []*drivers.OvsCfgNetworkState{}
	nwCfgs, err = drivers.ReadAllOvsCfgNetworks(stateDriver)
	if err != nil {
		return nil
	}
	for _, nwCfg := range nwCfgs {
		if !netPresent(allCfg, nwCfg.Id) {
			err = netmaster.DeleteNetworkId(stateDriver, nwCfg.Id)
			if err != nil {
				log.Printf("error '%s' deleting net %s \n", err, nwCfg.Id)
				continue
			}
		}
	}

	gCfgs := []*gstate.Cfg{}
	gCfgs, err = gstate.ReadAllGlobalCfg(stateDriver)
	if err != nil {
		return nil
	}
	for _, gCfg := range gCfgs {
		if !tenantPresent(allCfg, gCfg.Tenant) {
			err = netmaster.DeleteTenantId(stateDriver, gCfg.Tenant)
			if err != nil {
				log.Printf("error '%s' deleting tenant %s \n", err, gCfg.Tenant)
				continue
			}
		}
	}

	hostCfgs, err := netmaster.ReadAllMasterHostCfg(stateDriver)
	if err != nil {
		return err
	}
	for _, hostCfg := range hostCfgs {
		hostName := hostCfg.Name
		if !hostPresent(allCfg, hostName) {
			err = netmaster.DeleteHostId(stateDriver, hostName)
			if err != nil {
				log.Printf("error '%s' deleting host %s \n", err, hostName)
				continue
			}
		}
	}

	return nil
}

func processAdditions(stateDriver core.StateDriver, allCfg *netmaster.Config) (err error) {
	for _, host := range allCfg.Hosts {
		err := netmaster.CreateHost(stateDriver, &host)
		if err != nil {
			log.Printf("error '%s' adding host %s \n", err, host.Name)
			continue
		}
	}

	for _, tenant := range allCfg.Tenants {
		err := netmaster.CreateTenant(stateDriver, &tenant)
		if err != nil {
			log.Printf("error adding tenant '%s' \n", err)
			continue
		}

		err = netmaster.CreateNetworks(stateDriver, &tenant)
		if err != nil {
			log.Printf("error adding networks '%s' \n", err)
			continue
		}

		err = netmaster.CreateEndpoints(stateDriver, &tenant)
		if err != nil {
			log.Printf("error adding endpoints '%s' \n", err)
			continue
		}
	}

	return
}

func processDeletions(stateDriver core.StateDriver, allCfg *netmaster.Config) (err error) {
	for _, host := range allCfg.Hosts {
		err := netmaster.DeleteHost(stateDriver, &host)
		if err != nil {
			log.Printf("error '%s' deleting host %s \n", err, host.Name)
			continue
		}
	}

	for _, tenant := range allCfg.Tenants {
		err = netmaster.DeleteEndpoints(stateDriver, &tenant)
		if err != nil {
			log.Printf("error deleting endpoints '%s' \n", err)
			continue
		}

		err = netmaster.DeleteNetworks(stateDriver, &tenant)
		if err != nil {
			log.Printf("error deleting networks '%s' \n", err)
			continue
		}

		err = netmaster.DeleteTenant(stateDriver, &tenant)
		if err != nil {
			log.Printf("error deleting tenant '%s' \n", err)
			continue
		}
	}

	return
}

func initEtcd(defOpts *cliOpts) (core.StateDriver, error) {
	driverConfig := &drivers.EtcdStateDriverConfig{}
	driverConfig.Etcd.Machines = []string{defOpts.etcdUrl}
	config := &core.Config{V: driverConfig}

	etcdDriver := &drivers.EtcdStateDriver{}
	err := etcdDriver.Init(config)
	if err != nil {
		log.Printf("error '%s' initializing etcd \n", err)
	}

	return etcdDriver, err
}

func executeJsonCfg(defOpts *cliOpts) (err error) {
	data, err := ioutil.ReadFile(opts.idStr)
	if err != nil {
		return err
	}

	stateDriver, err := initEtcd(defOpts)
	if err != nil {
		log.Fatalf("Failed to init etcd driver. Error: %s", err)
	}

	if opts.cfgHostBindings {
		epBindings := []netmaster.ConfigEp{}
		err = json.Unmarshal(data, &epBindings)
		if err != nil {
			log.Printf("error '%s' unmarshing host bindings, data %s \n",
				err, data)
			return
		}

		err = netmaster.CreateEpBindings(stateDriver, &epBindings)
		if err != nil {
			log.Printf("error '%s' creating host bindings \n", err)
		}
		return
	}

	allCfg := &netmaster.Config{}
	err = json.Unmarshal(data, allCfg)
	if err != nil {
		log.Printf("error '%s' unmarshaling tenant cfg, data %s \n", err, data)
		return
	}
	// log.Printf("parsed config %v \n", allCfg)

	if defOpts.cfgDesired {
		err = deleteDelta(stateDriver, allCfg)
	}
	if err != nil {
		log.Printf("error deleting delta '%s' \n", err)
		return
	}

	if defOpts.cfgDeletions {
		err = processDeletions(stateDriver, allCfg)
	} else if defOpts.cfgAdditions || defOpts.cfgDesired {
		err = processAdditions(stateDriver, allCfg)
	} else {
		log.Fatalf("invalid json config file type\n")
		return
	}
	if err != nil {
		log.Printf("error processing cfg '%s' \n", err)
		return
	}

	return
}
