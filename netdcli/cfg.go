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
	"bufio"
	"encoding/json"
	"io/ioutil"
	"os"
	"time"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/gstate"
	"github.com/contiv/netplugin/netmaster"
	"github.com/contiv/netplugin/state"

	log "github.com/Sirupsen/logrus"
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

	readEp := &drivers.OvsCfgEndpointState{}
	readEp.StateDriver = stateDriver
	epCfgs, err := readEp.ReadAll()
	if core.ErrIfKeyExists(err) != nil {
		return err
	} else if err != nil {
		err = nil
		epCfgs = []core.State{}
	}
	for _, epCfg := range epCfgs {
		cfg := epCfg.(*drivers.OvsCfgEndpointState)
		if !epPresent(allCfg, cfg.Id) {
			err1 := netmaster.DeleteEndpointId(stateDriver, cfg.Id)
			if err1 != nil {
				log.Printf("error '%s' deleting epid %s \n", err1, cfg.Id)
				err = err1
				continue
			}
		}
	}

	readNet := &drivers.OvsCfgNetworkState{}
	readNet.StateDriver = stateDriver
	nwCfgs, err := readNet.ReadAll()
	if core.ErrIfKeyExists(err) != nil {
		return err
	} else if err != nil {
		err = nil
		nwCfgs = []core.State{}
	}
	for _, nwCfg := range nwCfgs {
		cfg := nwCfg.(*drivers.OvsCfgNetworkState)
		if !netPresent(allCfg, cfg.Id) {
			err1 := netmaster.DeleteNetworkId(stateDriver, cfg.Id)
			if err1 != nil {
				log.Printf("error '%s' deleting net %s \n", err1, cfg.Id)
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
			err1 := netmaster.DeleteTenantId(stateDriver, cfg.Tenant)
			if err1 != nil {
				log.Printf("error '%s' deleting tenant %s \n", err1, cfg.Tenant)
				err = err1
				continue
			}
		}
	}

	readHost := &netmaster.MasterHostConfig{}
	readHost.StateDriver = stateDriver
	hostCfgs, err := readHost.ReadAll()
	if core.ErrIfKeyExists(err) != nil {
		return err
	} else if err != nil {
		err = nil
		hostCfgs = []core.State{}
	}
	for _, hostCfg := range hostCfgs {
		cfg := hostCfg.(*netmaster.MasterHostConfig)
		hostName := cfg.Name
		if !hostPresent(allCfg, hostName) {
			err1 := netmaster.DeleteHostId(stateDriver, hostName)
			if err1 != nil {
				log.Printf("error '%s' deleting host %s \n", err1, hostName)
				err = err1
				continue
			}
		}
	}

	return err
}

func processAdditions(stateDriver core.StateDriver, allCfg *netmaster.Config) (err error) {
	for _, host := range allCfg.Hosts {
		err1 := netmaster.CreateHost(stateDriver, &host)
		if err1 != nil {
			log.Printf("error '%s' adding host %s \n", err1, host.Name)
			err = err1
			continue
		}
	}

	for _, tenant := range allCfg.Tenants {
		err1 := netmaster.CreateTenant(stateDriver, &tenant)
		if err1 != nil {
			log.Printf("error adding tenant '%s' \n", err1)
			err = err1
			continue
		}

		err1 = netmaster.CreateNetworks(stateDriver, &tenant)
		if err1 != nil {
			log.Printf("error adding networks '%s' \n", err1)
			err = err1
			continue
		}

		err1 = netmaster.CreateEndpoints(stateDriver, &tenant)
		if err1 != nil {
			log.Printf("error adding endpoints '%s' \n", err1)
			err = err1
			continue
		}
	}

	return
}

func processDeletions(stateDriver core.StateDriver, allCfg *netmaster.Config) (err error) {
	for _, host := range allCfg.Hosts {
		err1 := netmaster.DeleteHost(stateDriver, &host)
		if err1 != nil {
			log.Printf("error '%s' deleting host %s \n", err1, host.Name)
			err = err1
			continue
		}
	}

	for _, tenant := range allCfg.Tenants {
		err1 := netmaster.DeleteEndpoints(stateDriver, &tenant)
		if err1 != nil {
			log.Printf("error deleting endpoints '%s' \n", err1)
			err = err1
			continue
		}

		err1 = netmaster.DeleteNetworks(stateDriver, &tenant)
		if err1 != nil {
			log.Printf("error deleting networks '%s' \n", err1)
			err = err1
			continue
		}

		err1 = netmaster.DeleteTenant(stateDriver, &tenant)
		if err1 != nil {
			log.Printf("error deleting tenant '%s' \n", err1)
			err = err1
			continue
		}
	}

	return
}

func initEtcd(defOpts *cliOpts) (core.StateDriver, error) {
	driverConfig := &state.EtcdStateDriverConfig{}
	driverConfig.Etcd.Machines = []string{defOpts.etcdUrl}
	config := &core.Config{V: driverConfig}

	etcdDriver := &state.EtcdStateDriver{}
	err := etcdDriver.Init(config)
	if err != nil {
		log.Printf("error '%s' initializing etcd \n", err)
	}

	return etcdDriver, err
}

func executeJsonCfg(defOpts *cliOpts) (err error) {
	data := []byte{}
	if opts.idStr == "-" {
		reader := bufio.NewReader(os.Stdin)
		data, err = ioutil.ReadAll(reader)
		if err != nil {
			return err
		}

	} else {
		data, err = ioutil.ReadFile(opts.idStr)
		if err != nil {
			return err
		}
	}

	stateDriver, err := initEtcd(defOpts)
	if err != nil {
		log.Fatalf("Failed to init etcd driver. Error: %s", err)
	}

	if opts.cfgHostBindings {
		epBindings := []netmaster.ConfigEp{}
		err = json.Unmarshal(data, &epBindings)
		if err != nil {
			log.Printf("error '%s' unmarshing host bindings, data ============\n%s\n=============\n", err, data)
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
