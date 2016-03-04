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
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/netmaster/resources"
	"github.com/contiv/netplugin/state"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/netplugin/version"
	"github.com/hashicorp/consul/api"
)

type cliOpts struct {
	help        bool
	debug       bool
	stateStore  string
	storeURL    string
	listenURL   string
	clusterMode string
	dnsEnabled  bool
	version     bool
}

var flagSet *flag.FlagSet

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTION]...\n", os.Args[0])
	flagSet.PrintDefaults()
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

func parseOpts(opts *cliOpts) error {
	flagSet = flag.NewFlagSet("netm", flag.ExitOnError)
	flagSet.BoolVar(&opts.help,
		"help",
		false,
		"prints this message")
	flagSet.BoolVar(&opts.debug,
		"debug",
		false,
		"Turn on debugging information")
	flagSet.StringVar(&opts.stateStore,
		"state-store",
		utils.EtcdNameStr,
		"State store to use")
	flagSet.StringVar(&opts.storeURL,
		"store-url",
		"",
		"Etcd or Consul cluster url. Empty string resolves to respective state-store's default URL.")
	flagSet.StringVar(&opts.listenURL,
		"listen-url",
		":9999",
		"Url to listen http requests on")
	flagSet.StringVar(&opts.clusterMode,
		"cluster-mode",
		"docker",
		"{docker, kubernetes}")
	flagSet.BoolVar(&opts.dnsEnabled,
		"dns-enable",
		true,
		"Turn on DNS {true, false}")
	flagSet.BoolVar(&opts.version,
		"version",
		false,
		"prints current version")

	if err := flagSet.Parse(os.Args[1:]); err != nil {
		return err
	}

	return nil
}

func execOpts(opts *cliOpts) core.StateDriver {

	if opts.help {
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTION]...\n", os.Args[0])
		flagSet.PrintDefaults()
		os.Exit(0)
	}

	if opts.version {
		fmt.Printf(version.String())
		os.Exit(0)
	}

	log.SetFormatter(&log.TextFormatter{FullTimestamp: true, TimestampFormat: time.StampNano})

	if opts.debug {
		log.SetLevel(log.DebugLevel)
	}

	if err := master.SetClusterMode(opts.clusterMode); err != nil {
		log.Fatalf("Failed to set cluster-mode. Error: %s", err)
	}

	if err := master.SetDNSEnabled(opts.dnsEnabled); err != nil {
		log.Fatalf("Failed to set dns-enable. Error: %s", err)
	}

	sd, err := initStateDriver(opts)
	if err != nil {
		log.Fatalf("Failed to init state-store. Error: %s", err)
	}

	if _, err = resources.NewStateResourceManager(sd); err != nil {
		log.Fatalf("Failed to init resource manager. Error: %s", err)
	}

	return sd
}

func main() {
	d := &daemon{}
	opts := cliOpts{}

	if err := parseOpts(&opts); err != nil {
		log.Fatalf("Failed to parse cli options. Error: %s", err)
	}

	// execute options
	d.stateDriver = execOpts(&opts)

	// store the listen URL
	d.listenURL = opts.listenURL

	// Run daemon FSM
	d.runMasterFsm()
}
