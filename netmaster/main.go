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
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/netmaster/daemon"
	"github.com/contiv/netplugin/version"
)

type cliOpts struct {
	help         bool
	debug        bool
	clusterStore string
	listenURL    string
	clusterMode  string
	dnsEnabled   bool
	version      bool
}

var flagSet *flag.FlagSet

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTION]...\n", os.Args[0])
	flagSet.PrintDefaults()
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
	flagSet.StringVar(&opts.clusterStore,
		"cluster-store",
		"etcd://127.0.0.1:2379",
		"Etcd or Consul cluster store url.")
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

func execOpts(opts *cliOpts) {

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
}

func main() {
	opts := cliOpts{}

	if err := parseOpts(&opts); err != nil {
		log.Fatalf("Failed to parse cli options. Error: %s", err)
	}

	// execute options
	execOpts(&opts)

	// create master daemon
	d := &daemon.MasterDaemon{
		ListenURL:    opts.listenURL,
		ClusterStore: opts.clusterStore,
		ClusterMode:  opts.clusterMode,
		DNSEnabled:   opts.dnsEnabled,
	}

	// initialize master daemon
	d.Init()

	// Run daemon FSM
	d.RunMasterFsm()
}
