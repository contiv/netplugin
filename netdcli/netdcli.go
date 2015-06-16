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
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/gstate"
	"github.com/contiv/netplugin/netutils"
	"github.com/contiv/netplugin/resources"
	"github.com/contiv/netplugin/state"
	"github.com/contiv/netplugin/utils"
	"github.com/hashicorp/consul/api"

	log "github.com/Sirupsen/logrus"
)

const (
	cliConstructGlobal         = "global"
	cliConstructNetwork        = "network"
	cliConstructEndpoint       = "endpoint"
	cliConstructVLANResource   = "vlan-rsrc"
	cliConstructVXLANResource  = "vxlan-rsrc"
	cliConstructSubnetResource = "subnet-rsrc"
	cliOperGet                 = "get"
	cliOperCreate              = "create"
	cliOperDelete              = "delete"
	cliOperAttach              = "attach"
	cliOperDetach              = "detach"
)

var constructs = []string{
	cliConstructGlobal,
	cliConstructNetwork,
	cliConstructEndpoint,
	cliConstructVLANResource,
	cliConstructVXLANResource,
	cliConstructSubnetResource,
}

var validOperList = []string{cliOperGet, cliOperCreate, cliOperDelete, cliOperAttach, cliOperDetach}

type cliError struct {
	Desc string
}

func (e *cliError) Error() string {
	return e.Desc
}

type operation struct {
	val string
}

func (o *operation) isValid(val string) bool {
	for _, str := range validOperList {
		if str == val {
			return true
		}
	}
	return false
}

func (o *operation) String() string {
	return fmt.Sprintf("%s ", validOperList)
}

func (o *operation) Set(val string) error {
	if !o.isValid(val) {
		return &cliError{
			Desc: fmt.Sprintf("invalid value for construct (%s). Allowed values: %s",
				val, o.String())}
	}
	o.val = val
	return nil
}

func (o *operation) Get() interface{} {
	return o.val
}

type construct struct {
	val string
}

func (c *construct) String() string {
	return fmt.Sprintf("%s", constructs)
}

func (c *construct) Set(val string) error {
	for _, str := range constructs {
		if str == val {
			c.val = val
			return nil
		}
	}
	return &cliError{Desc: fmt.Sprintf("invalid value for construct (%s). Allowed values: %s",
		val, c.String())}
}

func (c *construct) Get() interface{} {
	return c.val
}

type cliOpts struct {
	help            bool
	debug           bool
	cfgDesired      bool
	cfgAdditions    bool
	cfgDeletions    bool
	cfgHostBindings bool
	oper            operation
	construct       construct
	stateStore      string
	storeURL        string
	tenant          string
	netID           string
	pktTag          string
	pktTagType      string
	subnetCidr      string
	ipAddr          string
	contName        string
	attachUUID      string
	subnetIP        string
	subnetLen       uint
	allocSubnetLen  uint
	defaultGw       string
	idStr           string
	vlans           string
	vxlans          string
	homingHost      string
	vtepIP          string
	intfName        string
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
	flagSet.BoolVar(&opts.debug,
		"debug",
		false,
		"Turn on debugging information",
	)
	flagSet.BoolVar(&opts.cfgHostBindings,
		"host-bindings-cfg",
		false,
		"JSON file describing container to host bindings. Use '-' to read configuration from stdin")
	flagSet.BoolVar(&opts.cfgAdditions,
		"add-cfg",
		false,
		"JSON file describing addition to global and network intent. Use '-' to read configuration from stdin")
	flagSet.BoolVar(&opts.cfgDeletions,
		"del-cfg",
		false,
		"JSON file describing deletion from global and network intent. Use '-' to read configuration from stdin")
	flagSet.BoolVar(&opts.cfgDesired,
		"cfg",
		false,
		"JSON file describing the global and network intent. Use '-' to read configuration from stdin")
	flagSet.StringVar(&opts.stateStore,
		"state-store",
		utils.EtcdNameStr,
		"State store to use")
	flagSet.StringVar(&opts.storeURL,
		"store-url",
		"",
		"Etcd or Consul cluster url. Empty string resolves to respective state-store's default URL.")
	flagSet.StringVar(&opts.tenant,
		"tenant",
		"default",
		"tenant id associated with the construct (global/network/ep)")
	flagSet.StringVar(&opts.netID,
		"net-id",
		"",
		"Network id of the endpoint")
	flagSet.StringVar(&opts.pktTag,
		"tag",
		"auto",
		"VLAN/VXLAN tag of the network")
	flagSet.StringVar(&opts.pktTagType,
		"tag-type",
		"vlan",
		"VLAN/VXLAN tag of the network")
	flagSet.StringVar(&opts.subnetCidr,
		"subnet",
		"",
		"Network Subnet IP with mask e.g. 11.0.1.1/24, or 0/24 to specify only mask")
	flagSet.StringVar(&opts.defaultGw,
		"gw",
		"",
		"Default Gateway Address of the network e.g. 11.0.1.1")
	flagSet.StringVar(&opts.ipAddr,
		"ip-address",
		"auto",
		"IP address associated with the endpoint")
	flagSet.StringVar(&opts.contName,
		"container-id",
		"",
		"Container ID to identify a runningcontainer")
	flagSet.StringVar(&opts.attachUUID,
		"attach-uuid",
		"",
		"Container UUID to attach to if different from contName")
	flagSet.StringVar(&opts.vlans,
		"vlans",
		"",
		"Allowed vlan ranges for auto-allocating vlans e.g. '10-100, 150-200")
	flagSet.UintVar(&opts.allocSubnetLen,
		"alloc-subnet-len",
		24,
		"Subnet length of auto allocated subnets from the subnet pool")
	flagSet.StringVar(&opts.homingHost,
		"host",
		"",
		"Host name/label on which an ep needs to be created. Default is the local host ")
	flagSet.StringVar(&opts.vxlans,
		"vxlans",
		"",
		"Allowed vlan ranges for auto-allocating vxlans e.g. '10000-20000, 30000-35000")
	flagSet.StringVar(&opts.vtepIP,
		"vtep-ip",
		"",
		"Endpoint's Vtep IP address if the endpoint is of vtep type")
	flagSet.StringVar(&opts.intfName,
		"intf-name",
		"",
		"Name of an existing linux device to use as endpoint's interface. This can be used for adding the host interface to the bridge for vlan based networks.")

	flagSet.BoolVar(&opts.help, "help", false, "prints this message")
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTION]...\n", os.Args[0])
	flagSet.PrintDefaults()
}

func logFatalSubnetAndMaskFormatError() {
	log.Fatalf("gateway IP and mask must be specified e.g. 11.0.1.1/24 or " +
		"if gateway is not required to be specified then 0/24")
}

func validateOpts(opts *cliOpts) error {
	var err error

	if flagSet.NArg() != 1 || opts.help {
		usage()
		return nil
	}

	if opts.debug {
		log.SetLevel(log.DebugLevel)
	}

	if opts.oper.Get() == "" {
		log.Fatalf("An operation must be specified")
	}

	if opts.construct.Get() == "" {
		log.Fatalf("A construct must be specified")
	}

	if opts.pktTagType != "vxlan" && opts.pktTagType != "vlan" {
		log.Fatalf("error '%s' packet tag type not supported", opts.pktTagType)
	}

	// global create params validation
	if opts.oper.Get() == cliOperCreate &&
		opts.construct.Get() == cliConstructGlobal {
		if opts.vlans != "" {
			_, err = netutils.ParseTagRanges(opts.vlans, "vlan")
			if err != nil {
				log.Fatalf("error '%s' parsing vlan range '%s' \n", err, opts.vlans)
			}
		}

		if opts.vxlans != "" {
			_, err = netutils.ParseTagRanges(opts.vxlans, "vxlan")
			if err != nil {
				log.Fatalf("error '%s' parsing vxlan range '%s' \n", err, opts.vxlans)
			}
		}
	}

	if opts.pktTag == "auto" {
		if opts.oper.Get() == cliOperCreate &&
			opts.construct.Get() == cliConstructNetwork {
			log.Infof("  auto allocating network subnet from global pool")
		}
	} else if opts.pktTag != "" {
		_, err = strconv.Atoi(opts.pktTag)
		if err != nil {
			log.Fatalf("Error convertinng tag %s to integer \n", opts.pktTag)
		}
	}

	// network create params validation
	if opts.oper.Get() == cliOperCreate &&
		opts.construct.Get() == cliConstructNetwork {
	}

	if opts.homingHost == "" {
		opts.homingHost, err = os.Hostname()
		if err != nil {
			log.Fatalf("error obtaining the hostname, error %s \n", err)
		}
	}

	// default gw and mask parsing
	if opts.subnetCidr == "" {
		opts.subnetLen = 0
		opts.subnetIP = "auto"
	} else {
		_, _, err = net.ParseCIDR(opts.subnetCidr)
		if err != nil {
			log.Fatalf("error '%s' parsing cidr ip %s \n", err, opts.subnetCidr)
		}

		opts.subnetIP, opts.subnetLen, err = netutils.ParseCIDR(opts.subnetCidr)
		if err != nil {
			logFatalSubnetAndMaskFormatError()
		}
	}

	if opts.vtepIP != "" && net.ParseIP(opts.vtepIP) == nil {
		log.Fatalf("error '%s' parsing vtep ip %s \n", err, opts.vtepIP)
	}

	// endpoint parameters validation
	if opts.oper.Get() == cliOperCreate &&
		opts.construct.Get() == cliConstructEndpoint &&
		opts.vtepIP != "" &&
		(opts.netID == "" || opts.ipAddr == "") {
		if opts.ipAddr == "auto" {
			log.Debugf("doing auto ip address assignemt for the ep... \n")
		} else {
			log.Fatalf("Endpoint creation requires a valid net-id, vlan tag, " +
				"and ip address")
		}
	}

	// attach detach parameters validation
	if (opts.oper.Get() == cliOperAttach || opts.oper.Get() == cliOperDetach) &&
		opts.construct.Get() == cliConstructEndpoint && opts.contName == "" {
		log.Fatalf("A valid container-id is needed to attach/detach a container to an ep")
	}

	return err
}

func executeOpts(opts *cliOpts) error {
	var coreState core.State

	err := validateOpts(opts)
	if err != nil {
		return err
	}

	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return err
	}

	switch opts.construct.Get() {
	case cliConstructEndpoint:
		if opts.oper.Get() == cliOperGet {
			epOper := &drivers.OvsOperEndpointState{}
			epOper.StateDriver = stateDriver
			coreState = epOper
		} else if opts.oper.Get() == cliOperAttach || opts.oper.Get() == cliOperDetach {
			epCfg := &drivers.OvsCfgEndpointState{}
			epCfg.StateDriver = stateDriver
			err = epCfg.Read(opts.idStr)
			if err != nil {
				log.Errorf("Failed to read ep %s. Error: %s", opts.construct.Get(), err)
				return err
			}
			log.Debugf("read ep state as %v for container %s", epCfg, opts.contName)
			if opts.oper.Get() == cliOperAttach {
				epCfg.ContName = opts.contName
				epCfg.AttachUUID = opts.attachUUID
			} else {
				if epCfg.ContName != opts.contName {
					return core.Errorf("Can not detach container '%s' from endpoint '%s' - "+
						"container not attached", opts.contName, opts.idStr)
				}
				epCfg.ContName = ""
			}
			coreState = epCfg
		} else {
			epCfg := &drivers.OvsCfgEndpointState{}
			epCfg.StateDriver = stateDriver
			epCfg.ID = opts.idStr
			epCfg.NetID = opts.netID
			epCfg.IPAddress = opts.ipAddr
			epCfg.ContName = opts.contName
			epCfg.AttachUUID = opts.attachUUID
			epCfg.HomingHost = opts.homingHost
			epCfg.VtepIP = opts.vtepIP
			epCfg.IntfName = opts.intfName
			coreState = epCfg
		}
	case cliConstructNetwork:
		if opts.oper.Get() == cliOperGet {
			nwCfg := &drivers.OvsCfgNetworkState{}
			nwCfg.StateDriver = stateDriver
			coreState = nwCfg
		} else {
			nwCfg := &drivers.OvsCfgNetworkState{}
			nwCfg.StateDriver = stateDriver
			nwCfg.PktTag, _ = strconv.Atoi(opts.pktTag)
			nwCfg.Tenant = opts.tenant
			nwCfg.PktTagType = opts.pktTagType
			nwCfg.SubnetIP = opts.subnetIP
			nwCfg.SubnetLen = opts.subnetLen
			nwCfg.DefaultGw = opts.defaultGw
			nwCfg.ID = opts.idStr
			coreState = nwCfg
		}
	case cliConstructGlobal:
		gcfg := &gstate.Cfg{}
		gcfg.StateDriver = stateDriver
		if opts.oper.Get() == cliOperGet {
			err = gcfg.Read(opts.tenant)
			log.Debugf("State: %v \n", gcfg)
		} else if opts.oper.Get() == cliOperDelete {
			gcfg.Version = gstate.VersionBeta1
			gcfg.Tenant = opts.tenant
			err = gcfg.Clear()
			if err != nil {
				log.Errorf("Failed to delete %s. Error: %s", opts.construct.Get(), err)
				return err
			}
		} else {
			gcfg.Version = gstate.VersionBeta1
			gcfg.Tenant = opts.tenant
			gcfg.Deploy.DefaultNetType = opts.pktTagType
			gcfg.Auto.SubnetPool = opts.subnetIP
			gcfg.Auto.SubnetLen = opts.subnetLen
			gcfg.Auto.VLANs = opts.vlans
			gcfg.Auto.VXLANs = opts.vxlans
			gcfg.Auto.AllocSubnetLen = opts.allocSubnetLen
			err = gcfg.Write()
		}
		if err != nil {
			return err
		}
		return err
	case cliConstructVLANResource:
		fallthrough
	case cliConstructVXLANResource:
		fallthrough
	case cliConstructSubnetResource:
		if opts.oper.Get() == cliOperGet {
			if cliConstructVLANResource == opts.construct.Get() {
				rsrc := &resources.AutoVLANCfgResource{}
				rsrc.StateDriver = stateDriver
				coreState = rsrc
			}
			if cliConstructVXLANResource == opts.construct.Get() {
				rsrc := &resources.AutoVXLANCfgResource{}
				rsrc.StateDriver = stateDriver
				coreState = rsrc
			}
			if cliConstructSubnetResource == opts.construct.Get() {
				rsrc := &resources.AutoSubnetCfgResource{}
				rsrc.StateDriver = stateDriver
				coreState = rsrc
			}
		} else {
			return core.Errorf("Only get operation is supported for resources")
		}
	}

	switch opts.oper.Get() {
	case cliOperGet:
		err = coreState.Read(opts.idStr)
		if err != nil {
			log.Errorf("Failed to read %s. Error: %s", opts.construct.Get(), err)
			return err
		}
		content, err := json.MarshalIndent(coreState, "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal corestate %+v", coreState)
		}
		fmt.Println(string(content))
	case cliOperAttach, cliOperDetach, cliOperCreate:
		err = coreState.Write()
		if err != nil {
			log.Errorf("Failed to create %s. Error: %s", opts.construct.Get(), err)
			return err
		}
	case cliOperDelete:
		err = coreState.Clear()
		if err != nil {
			log.Errorf("Failed to delete %s. Error: %s", opts.construct.Get(), err)
			return err
		}
	}

	return nil
}

func initStateDriver(opts *cliOpts) (core.StateDriver, error) {
	var cfg *core.Config

	switch opts.stateStore {
	case utils.EtcdNameStr:
		url := "http://127.0.0.1:4001"
		if opts.storeURL != "" {
			url = opts.storeURL
		}
		etcdCfg := &state.EtcdStateDriverConfig{}
		etcdCfg.Etcd.Machines = []string{url}
		cfg = &core.Config{V: etcdCfg}
	case utils.ConsulNameStr:
		url := "http://127.0.0.1:8500"
		if opts.storeURL != "" {
			url = opts.storeURL
		}
		consulCfg := &state.ConsulStateDriverConfig{}
		consulCfg.Consul = api.Config{Address: url}
		cfg = &core.Config{V: consulCfg}
	default:
		return nil, core.Errorf("Unsupported state-store %q", opts.stateStore)
	}

	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	return utils.NewStateDriver(opts.stateStore, string(cfgBytes))
}

func main() {
	err := flagSet.Parse(os.Args[1:])
	if err != nil {
		log.Fatalf("Failed to parse command. Error: %s", err)
	}
	opts.idStr = flagSet.Arg(0)

	sd, err := initStateDriver(&opts)
	if err != nil {
		log.Fatalf("state store initialization failed. Error: %s", err)
	}

	_, err = resources.NewStateResourceManager(sd)
	if err != nil {
		log.Fatalf("state store initialization failed. Error: %s", err)
	}

	if opts.cfgDesired || opts.cfgDeletions || opts.cfgAdditions || opts.cfgHostBindings {
		err = executeJSONCfg(&opts)
	} else {
		err = executeOpts(&opts)
	}
	if err != nil {
		log.Fatalf("error %s executing the config opts %v \n", err, opts)
	}

	os.Exit(0)
}
