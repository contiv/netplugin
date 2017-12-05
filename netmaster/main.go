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
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/daemon"
	"github.com/contiv/netplugin/netmaster/docknet"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/netplugin/utils/netutils"
	"github.com/contiv/netplugin/version"
	"github.com/urfave/cli"
)

type cliOpts struct {
	help         bool
	debug        bool
	pluginName   string
	clusterStore string
	listenURL    string
	controlURL   string
	clusterMode  string
	version      bool
}

const binName = "netmaster"

func initNetMaster(ctx *cli.Context) (*daemon.MasterDaemon, error) {
	// 1. validate and init logging
	if err := utils.InitLogging(binName, ctx); err != nil {
		return nil, err
	}

	// 2. validate network configs
	netConfigs, err := utils.ValidateNetworkOptions(binName, ctx)
	if err != nil {
		return nil, err
	}

	// 3. validate db configs
	dbConfigs, err := utils.ValidateDBOptions(binName, ctx)
	if err != nil {
		return nil, err
	}

	// 4. set v2 plugin name if it's set
	pluginName := ctx.String("name")
	if netConfigs.Mode == core.Docker || netConfigs.Mode == core.SwarmMode {
		logrus.Infof("Using netmaster docker v2 plugin name: %s", pluginName)
		docknet.UpdateDockerV2PluginName(pluginName, pluginName)
	} else {
		logrus.Infof("Ignoring netmaster docker v2 plugin name: %s (netmaster mode: %s)", pluginName, netConfigs.Mode)
	}

	// 5. set plugin listen addresses
	externalAddress := ctx.String("external-address")
	if externalAddress == "" {
		return nil, errors.New("netmaster external-address is not set")
	} else if err := netutils.ValidateBindAddress(externalAddress); err != nil {
		return nil, err
	}
	logrus.Infof("Using netmaster external-address: %s", externalAddress)

	internalAddress := ctx.String("internal-address")
	if internalAddress == "" {
		localIP, err := netutils.GetDefaultAddr()
		if err != nil {
			return nil, fmt.Errorf("Failed to get host address: %s", err.Error())
		}
		internalAddress = localIP + ":" + strings.Split(externalAddress, ":")[1]
	} else if err := netutils.ValidateBindAddress(internalAddress); err != nil {
		return nil, err
	}

	logrus.Infof("Using netmaster internal-address: %s", internalAddress)

	// 6. validate infra type
	infra := strings.ToLower(ctx.String("infra"))
	switch infra {
	case "aci", "default":
		logrus.Infof("Using netmaster infra type: %s", infra)
	default:
		return nil, fmt.Errorf("Unknown netmaster infra type: %s", infra)
	}

	return &daemon.MasterDaemon{
		ListenURL:          externalAddress,
		ControlURL:         internalAddress,
		ClusterStoreDriver: dbConfigs.StoreDriver,
		ClusterStoreURL:    dbConfigs.StoreURL, //TODO: support more than one url
		ClusterMode:        netConfigs.Mode,
		NetworkMode:        netConfigs.NetworkMode,
		NetForwardMode:     netConfigs.ForwardMode,
		NetInfraType:       infra,
	}, nil
}

func startNetMaster(netmaster *daemon.MasterDaemon) {
	// initialize master daemon
	netmaster.Init()
	// start monitoring services
	netmaster.InitServices()
	// Run daemon FSM
	netmaster.RunMasterFsm()
}

func main() {
	app := cli.NewApp()
	app.Version = "\n" + version.String()
	app.Usage = "Contiv netmaster service"
	netmasterFlags := []cli.Flag{
		cli.StringFlag{
			Name:   "infra, infra-type",
			Value:  "default",
			EnvVar: "CONTIV_NETMASTER_INFRA",
			Usage:  "set netmaster infra type, options [aci, default]",
		},
		cli.StringFlag{
			Name:   "name, plugin-name",
			Value:  "netplugin",
			EnvVar: "CONTIV_NETMASTER_PLUGIN_NAME",
			Usage:  "set netmaster plugin name for docker v2 plugin",
		},
		cli.StringFlag{
			Name:   "external-address, listen-url",
			Value:  "0.0.0.0:9999",
			EnvVar: "CONTIV_NETMASTER_EXTERNAL_ADDRESS",
			Usage:  "set netmaster external address to listen on, used for general API service",
		},
		cli.StringFlag{
			Name:   "internal-address, control-url",
			EnvVar: "CONTIV_NETMASTER_INTERNAL_ADDRESS",
			Usage:  "set netmaster internal address to listen on, used for RPC and leader election (default: <host-ip-from-local-resolver>:<port-of-external-address>)",
		},
	}
	app.Flags = utils.FlattenFlags(netmasterFlags, utils.BuildDBFlags(binName), utils.BuildNetworkFlags(binName), utils.BuildLogFlags(binName))
	sort.Sort(cli.FlagsByName(app.Flags))
	app.Action = func(ctx *cli.Context) error {
		netmaster, err := initNetMaster(ctx)
		if err != nil {
			errmsg := err.Error()
			logrus.Error(errmsg)
			// use 22 Invalid argument as error return code
			// http://www-numi.fnal.gov/offline_software/srt_public_context/WebDocs/Errors/unix_system_errors.html
			return cli.NewExitError(errmsg, 22)
		}
		startNetMaster(netmaster)
		return nil
	}
	app.Run(os.Args)
}
