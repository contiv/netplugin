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
	"os"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/netmaster/resources"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/netplugin/version"
)

type cliOpts struct {
	help        bool
	debug       bool
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
	// parse the state store URL
	parts := strings.Split(opts.storeURL, "://")
	if len(parts) < 2 {
		return nil, core.Errorf("Invalid state-store URL %q", opts.storeURL)
	}
	stateStore := parts[0]

	// Make sure we support the statestore type
	switch stateStore {
	case utils.EtcdNameStr:
	case utils.ConsulNameStr:
	default:
		return nil, core.Errorf("Unsupported state-store %q", stateStore)
	}

	// Setup instance info
	instInfo := core.InstanceInfo{
		DbURL: opts.storeURL,
	}

	return utils.NewStateDriver(stateStore, &instInfo)
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
	flagSet.StringVar(&opts.storeURL,
		"store-url",
		"etcd://127.0.0.1:2379",
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

	// store the URLs
	d.listenURL = opts.listenURL
	d.storeURL = opts.storeURL

	// Run daemon FSM
	d.runMasterFsm()
}
