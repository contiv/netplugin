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
)

type ConfigEpJson struct {
	Container string
	Host      string
}

type ConfigNetworkJson struct {
	Name      string
	Endpoints []ConfigEpJson
}

type ConfigJson struct {
	DefaultNetType string
	SubnetPool     string
	AllocSubnetLen uint
	Vlans          string
	Vxlans         string

	Networks []ConfigNetworkJson
}

func executeJsonCfg(defOpts *cliOpts) error {
	data, err := ioutil.ReadFile(opts.idStr)
	if err != nil {
		return err
	}

	cfg := &ConfigJson{}
	err = json.Unmarshal(data, cfg)
	if err != nil {
		log.Printf("unmarshal error '%s', cfg %v \n", err, cfg)
		return err
	}
	log.Printf("parsed config %v \n", cfg)

	opts := *defOpts
	opts.construct.Set(CLI_CONSTRUCT_GLOBAL)
	opts.oper.Set(CLI_OPER_CREATE)
	opts.pktTagType = cfg.DefaultNetType
	opts.subnetCidr = cfg.SubnetPool
	opts.allocSubnetLen = cfg.AllocSubnetLen
	opts.vlans = cfg.Vlans
	opts.vxlans = cfg.Vxlans

	err = executeOpts(&opts)
	if err != nil {
		log.Printf("error pushing global config state: %s \n", err)
		return err
	}

	for _, net := range cfg.Networks {
		opts = *defOpts

		opts.construct.Set(CLI_CONSTRUCT_NW)
		opts.oper.Set(CLI_OPER_CREATE)
		opts.idStr = net.Name
		err = executeOpts(&opts)
		if err != nil {
			log.Printf("error pushing network config state: %s \n", err)
			return err
		}

		for _, ep := range net.Endpoints {
			opts = *defOpts
			opts.construct.Set(CLI_CONSTRUCT_EP)
			opts.oper.Set(CLI_OPER_CREATE)
			opts.idStr = net.Name + "-" + ep.Container
			opts.netId = net.Name
			opts.contName = ep.Container
			opts.homingHost = ep.Host
			err = executeOpts(&opts)
			if err != nil {
				log.Printf("error pushing ep config state: %s \n", err)
				return err
			}
		}
	}

	return err
}
