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
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/drivers"
)

const (
	CLI_CONSTRUCT_NW = "network"
	CLI_CONSTRUCT_EP = "endpoint"
	CLI_OPER_GET     = "get"
	CLI_OPER_CREATE  = "create"
	CLI_OPER_DELETE  = "delete"
)

type CliError struct {
	Desc string
}

func (e *CliError) Error() string {
	return e.Desc
}

type Operation struct {
	val string
}

func (o *Operation) String() string {
	return fmt.Sprintf("%s, %s or %s", CLI_OPER_GET, CLI_OPER_CREATE, CLI_OPER_DELETE)
}

func (o *Operation) Set(val string) error {
	if val != CLI_OPER_GET && val != CLI_OPER_CREATE && val != CLI_OPER_DELETE {
		return &CliError{Desc: fmt.Sprintf("invalid value for construct (%s). Allowed values: %s",
			val, o.String())}
	}
	o.val = val
	return nil
}

func (o *Operation) Get() interface{} {
	return o.val
}

type Construct struct {
	val string
}

func (c *Construct) String() string {
	return fmt.Sprintf("%s or %s", CLI_CONSTRUCT_NW, CLI_CONSTRUCT_EP)
}

func (c *Construct) Set(val string) error {
	if val != CLI_CONSTRUCT_NW && val != CLI_CONSTRUCT_EP {
		return &CliError{Desc: fmt.Sprintf("invalid value for construct (%s). Allowed values: %s",
			val, c.String())}
	}
	c.val = val
	return nil
}

func (c *Construct) Get() interface{} {
	return c.val
}

func main() {
	flagSet := flag.NewFlagSet("netd-cli", flag.ExitOnError)
	oper := &Operation{}
	flagSet.Var(oper, "oper", "Operation to perform")
	construct := &Construct{}
	flagSet.Var(construct, "construct", "Construct to operate on i.e network or endpoint")
	etcdUrl := ""
	flagSet.StringVar(&etcdUrl, "etcd-url", "http://127.0.0.1:4001", "Etcd cluster url")
	netId := ""
	flagSet.StringVar(&netId, "net-id", "", "Network id of the endpoint")
	vlanTag := 0
	flagSet.IntVar(&vlanTag, "tag", 0, "Vlan tag of the endpoint")
	idStr := ""

	err := flagSet.Parse(os.Args[1:])
	if err != nil {
		log.Printf("Failed to parse command. Error: %s", err)
		os.Exit(1)
	}

	if oper.Get() == "" {
		log.Printf("An operation must be specified")
		os.Exit(1)
	}

	if construct.Get() == "" {
		log.Printf("A construct must be specified")
		os.Exit(1)
	}

	/* make sure all arguments are specified for endpoint create */
	if oper.Get() == CLI_OPER_CREATE && construct.Get() == CLI_CONSTRUCT_EP && (netId == "" || vlanTag == 0) {
		log.Printf("A valid net-id and vlan tag needs to be specified for endpoint creation")
		os.Exit(1)
	}

	if flagSet.NArg() != 1 {
		log.Printf("One argument is expected")
		os.Exit(1)
	} else {
		idStr = flagSet.Arg(0)
	}

	etcdDriver := &drivers.EtcdStateDriver{}
	driverConfig := &drivers.EtcdStateDriverConfig{}
	driverConfig.Etcd.Machines = []string{etcdUrl}
	config := &core.Config{V: driverConfig}
	err = etcdDriver.Init(config)
	if err != nil {
		log.Printf("Failed to init etcd driver. Error: %s", err)
		os.Exit(1)
	}

	var state core.State = nil
	switch construct.Get() {
	case CLI_CONSTRUCT_EP:
		if oper.Get() == CLI_OPER_GET {
			epOper := &drivers.OvsOperEndpointState{StateDriver: etcdDriver}
			state = epOper
		} else {
			epCfg := &drivers.OvsCfgEndpointState{StateDriver: etcdDriver}
			epCfg.Id = idStr
			epCfg.NetId = netId
			epCfg.VlanTag = vlanTag
			state = epCfg
		}
	case CLI_CONSTRUCT_NW:
		if oper.Get() == CLI_OPER_GET {
			nwOper := &drivers.OvsOperNetworkState{StateDriver: etcdDriver}
			state = nwOper
		} else {
			nwCfg := &drivers.OvsCfgNetworkState{StateDriver: etcdDriver}
			nwCfg.Id = idStr
			state = nwCfg
		}
	}

	switch oper.Get() {
	case CLI_OPER_GET:
		err = state.Read(idStr)
		if err != nil {
			log.Printf("Failed to create %s. Error: %s", construct.Get(), err)
			os.Exit(1)
		} else {
			log.Printf("%s State: %v", construct.Get(), state)
		}
	case CLI_OPER_CREATE:
		err = state.Write()
		if err != nil {
			log.Printf("Failed to create %s. Error: %s", construct.Get(), err)
			os.Exit(1)
		}
	case CLI_OPER_DELETE:
		err = state.Clear()
		if err != nil {
			log.Printf("Failed to delete %s. Error: %s", construct.Get(), err)
			os.Exit(1)
		}
	}

	os.Exit(0)
}
