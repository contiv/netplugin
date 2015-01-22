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
    "strings"
    "strconv"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/drivers"
)

const (
	CLI_CONSTRUCT_NW = "network"
	CLI_CONSTRUCT_EP = "endpoint"
	CLI_OPER_GET     = "get"
	CLI_OPER_CREATE  = "create"
	CLI_OPER_DELETE  = "delete"
	CLI_OPER_ATTACH  = "attach"
	CLI_OPER_DETACH  = "detach"
)

var validOperList = []string { CLI_OPER_GET, CLI_OPER_CREATE, CLI_OPER_DELETE, CLI_OPER_ATTACH, CLI_OPER_DETACH }

type CliError struct {
	Desc string
}

func (e *CliError) Error() string {
	return e.Desc
}

type Operation struct {
	val string
}

func (o *Operation) isValid(val string) bool {
    for _, str := range validOperList {
        if str == val {
            return true
        }
    }
    return false
}

func (o *Operation) String() string {
	return fmt.Sprintf("%s ", validOperList)
}

func (o *Operation) Set(val string) error {
	if ! o.isValid(val) {
		return &CliError{
            Desc: fmt.Sprintf("invalid value for construct (%s). Allowed values: %s",
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

type cliOpts struct {
    help        bool
    oper        Operation
    etcdUrl     string
    construct   Construct
    netId       string
    pktTag      string
    pktTagType  string
    gwAndMask   string
    ipAddr      string
    contId      string
    idStr       string
    defaultGw   string
    subnetMask  string
}

var opts cliOpts
var flagSet *flag.FlagSet

func init() {
	flagSet = flag.NewFlagSet("netdcli", flag.ExitOnError)
	flagSet.Var(&opts.oper,
        "oper",
        "Operation to perform")
	flagSet.Var(&opts.construct,
        "construct",
        "Construct to operate on i.e network or endpoint")
	flagSet.StringVar(&opts.etcdUrl,
        "etcd-url",
        "http://127.0.0.1:4001",
        "Etcd cluster url")
	flagSet.StringVar(&opts.netId,
        "net-id",
        "",
        "Network id of the endpoint")
	flagSet.StringVar(&opts.pktTag,
        "tag",
        "auto",
        "Vlan/Vxlan tag of the network")
	flagSet.StringVar(&opts.pktTagType,
        "tag-type",
        "vlan",
        "Vlan/Vxlan tag of the network")
	flagSet.StringVar(&opts.gwAndMask,
        "gw",
        "",
        "Default Gateway IP and mask e.g. 11.0.1.1/24")
	flagSet.StringVar(&opts.ipAddr,
        "ip-address",
        "auto",
        "IP address associated with the endpoint")
	flagSet.StringVar(&opts.contId,
        "container-id",
        "",
        "Container Id to identify a runningcontainer")
    flagSet.BoolVar(&opts.help, "help", false, "prints this message")
}

func usage() {
    fmt.Fprintf(os.Stderr, "Usage: %s [OPTION]...\n", os.Args[0])
    flagSet.PrintDefaults()
}

func logFatalGwAndMaskFormatError() {
    log.Fatalf("gateway IP and mask must be specified e.g. 11.0.1.1/24 or " +
        "if gateway is not required to be specified then 0/24")
}

func validateOpts() error {
    var err error

	if flagSet.NArg() != 1 || opts.help {
        usage()
        os.Exit(0)
	}

	if opts.oper.Get() == "" {
		log.Fatalf("An operation must be specified")
	}

	if opts.construct.Get() == "" {
		log.Fatalf("A construct must be specified")
	}

    // network create params validation
	if opts.oper.Get() == CLI_OPER_CREATE &&
       opts.construct.Get() == CLI_CONSTRUCT_NW &&
       (opts.pktTag == "auto" || opts.pktTagType != "vlan" || opts.gwAndMask == "") {
        if opts.pktTag == "auto" {
            log.Fatalf("vxlan tunneling and auto allocation of vlan/vxlan is coming soon...")
        } else if opts.pktTagType != "vlan" {
            log.Fatalf("vxlan and other packet tag support is coming soon...")
        } else {
            logFatalGwAndMaskFormatError()
        }
	}

    // default gw and mask parsing
    if (opts.oper.Get() == CLI_OPER_CREATE &&
        opts.construct.Get() == CLI_CONSTRUCT_NW) {
        strs := strings.Split(opts.gwAndMask, "/")
        if len(strs) != 2 {
            logFatalGwAndMaskFormatError()
        }

        // TODO: validate ipv4/v6 gateway IP
        if strs[0] != "0" {
            opts.defaultGw = strs[0]
        }
        if intMask, _ := strconv.Atoi(strs[1]); intMask > 32 {
            log.Printf("invalid mask in gateway/mask specification ")
            logFatalGwAndMaskFormatError()
        }
        opts.subnetMask = strs[1]
    }

	// endpoint parameters validation
	if opts.oper.Get() == CLI_OPER_CREATE &&
       opts.construct.Get() == CLI_CONSTRUCT_EP &&
       (opts.netId == "" || opts.ipAddr == "" || opts.ipAddr == "auto") {
        if opts.ipAddr == "auto" {
            log.Fatalf("auto ip address assignemt is coming soon... for now " +
                "please specify an IP address associated with an endpoint\n")
        } else {
            log.Fatalf("Endpoint creation requires a valid net-id, vlan tag, " +
                "and ip address")
        }
	}

    // attach detach parameters validation
	if (opts.oper.Get() == CLI_OPER_ATTACH || opts.oper.Get() == CLI_OPER_DETACH) &&
        opts.construct.Get() == CLI_CONSTRUCT_EP && opts.contId == "" {
		log.Fatalf("A valid container-id is needed to attach/detach a container to an ep")
	}

    return err
}

func main() {
    var err error
	var state core.State = nil

	err = flagSet.Parse(os.Args[1:])
	if err != nil {
		log.Fatalf("Failed to parse command. Error: %s", err)
	}
    opts.idStr = flagSet.Arg(0)
    err = validateOpts()
    if err != nil {
        os.Exit(1)
    }
    log.Printf("parsed all valuees = %v \n", opts)

    // initialize drivers
	etcdDriver := &drivers.EtcdStateDriver{}
	driverConfig := &drivers.EtcdStateDriverConfig{}
	driverConfig.Etcd.Machines = []string{opts.etcdUrl}
	config := &core.Config{V: driverConfig}
	err = etcdDriver.Init(config)
	if err != nil {
		log.Fatalf("Failed to init etcd driver. Error: %s", err)
	}

	switch opts.construct.Get() {
	case CLI_CONSTRUCT_EP:
		if opts.oper.Get() == CLI_OPER_GET {
			epOper := &drivers.OvsOperEndpointState{StateDriver: etcdDriver}
			state = epOper
        } else if opts.oper.Get() == CLI_OPER_ATTACH || opts.oper.Get() == CLI_OPER_DETACH {
            epCfg := &drivers.OvsCfgEndpointState{StateDriver: etcdDriver}
		    err = epCfg.Read(opts.idStr)
            if err != nil {
                log.Fatalf("Failed to read ep %s. Error: %s", opts.construct.Get(), err)
            }
            log.Printf("read ep state as %v for container %s \n", epCfg, opts.contId)
            if (opts.oper.Get() == CLI_OPER_ATTACH) {
                epCfg.ContId = opts.contId
            } else {
                if epCfg.ContId != opts.contId {
                    log.Fatalf("Can not detach container '%s' from endpoint '%s' - " +
                               "container not attached \n", opts.contId, opts.idStr)
                }
                epCfg.ContId = ""
            }
            state = epCfg
        } else {
            epCfg := &drivers.OvsCfgEndpointState{StateDriver: etcdDriver}
            epCfg.Id = opts.idStr
            epCfg.NetId = opts.netId
            epCfg.IpAddress = opts.ipAddr
            epCfg.ContId = opts.contId
            state = epCfg
		}
	case CLI_CONSTRUCT_NW:
		if opts.oper.Get() == CLI_OPER_GET {
			nwOper := &drivers.OvsOperNetworkState{StateDriver: etcdDriver}
			state = nwOper
		} else {
			nwCfg := &drivers.OvsCfgNetworkState{StateDriver: etcdDriver}
            nwCfg.PktTag, _ = strconv.Atoi(opts.pktTag)
            nwCfg.PktTagType = opts.pktTagType
            nwCfg.DefaultGw = opts.defaultGw
            nwCfg.SubnetMask = opts.subnetMask
			nwCfg.Id = opts.idStr
			state = nwCfg
		}
	}

	switch opts.oper.Get() {
	case CLI_OPER_GET:
		err = state.Read(opts.idStr)
		if err != nil {
			log.Fatalf("Failed to read %s. Error: %s", opts.construct.Get(), err)
		} else {
			log.Fatalf("%s State: %v", opts.construct.Get(), state)
		}
    case CLI_OPER_ATTACH, CLI_OPER_DETACH, CLI_OPER_CREATE:
		err = state.Write()
		if err != nil {
			log.Fatalf("Failed to create %s. Error: %s", opts.construct.Get(), err)
		}
	case CLI_OPER_DELETE:
		err = state.Clear()
		if err != nil {
			log.Fatalf("Failed to delete %s. Error: %s", opts.construct.Get(), err)
		}
	}

	os.Exit(0)
}
