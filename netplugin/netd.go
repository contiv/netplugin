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
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netplugin/agent"
	"github.com/contiv/netplugin/netplugin/plugin"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/netplugin/utils/netutils"
	"github.com/contiv/netplugin/version"

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
)

const binName = "netplugin"

func startNetPlugin(pluginConfig *plugin.Config) {
	// Create a new agent
	ag := agent.NewAgent(pluginConfig)

	// Process all current state
	ag.ProcessCurrentState()

	// post initialization processing
	ag.PostInit()

	// handle events
	if err := ag.HandleEvents(); err != nil {
		logrus.Errorf("Netplugin exiting due to error: %v", err)
		os.Exit(1)
	}
}

func initNetPluginConfig(ctx *cli.Context) (*plugin.Config, error) {
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

	// 4. validate and set other optional configs
	hostLabel := ctx.String("host")
	var configErr error
	if hostLabel == "" {
		hostLabel, configErr = os.Hostname()
		if configErr != nil {
			return nil, fmt.Errorf("Failed to get hostname: %s", configErr.Error())
		}
	}
	logrus.Infof("Using netplugin host: %v", hostLabel)
	controlIP := ctx.String("ctrl-ip")
	if controlIP == "" {
		controlIP, configErr = netutils.GetDefaultAddr()
		if configErr != nil {
			logrus.Fatalf("Failed to get host address: %s", configErr.Error())
		}
	}
	logrus.Infof("Using netplugin control IP: %v", controlIP)
	// TODO: Ignore vtep ip if it's not vxlan mode
	vtepIP := ctx.String("vtep-ip")
	if vtepIP == "" {
		vtepIP, configErr = netutils.GetDefaultAddr()
		if configErr != nil {
			logrus.Fatalf("Failed to get host address: %s", configErr.Error())
		}
	}
	logrus.Infof("Using netplugin VTEP IP: %v", vtepIP)

	vlanUpLinks := utils.FilterEmpty(strings.Split(ctx.String("vlan-uplinks"), ","))
	if netConfigs.NetworkMode == "vlan" && len(vlanUpLinks) == 0 {
		return nil, fmt.Errorf("vlan-uplinks must be set when using VLAN mode")
	}
	logrus.Infof("Using netplugin vlan uplinks: %v", vlanUpLinks)

	vxlanPort := ctx.Int("vxlan-port")
	logrus.Infof("Using netplugin vxlan port: %v", vxlanPort)

	return &plugin.Config{
		Drivers: plugin.Drivers{
			Network: utils.OvsNameStr,
			State:   dbConfigs.StoreDriver,
		},
		Instance: core.InstanceInfo{
			HostLabel:    hostLabel,
			CtrlIP:       controlIP,
			VtepIP:       vtepIP,
			UplinkIntf:   vlanUpLinks,
			DbURL:        dbConfigs.StoreURL,
			PluginMode:   netConfigs.Mode,
			VxlanUDPPort: vxlanPort,
			FwdMode:      netConfigs.ForwardMode, // TODO: pass in network mode
		},
	}, nil
}

func main() {
	app := cli.NewApp()
	app.Version = "\n" + version.String()
	app.Usage = "Contiv netplugin service"
	netpluginFlags := []cli.Flag{
		cli.StringFlag{
			Name:   "host, host-label",
			EnvVar: "CONTIV_NETPLUGIN_HOST",
			Usage:  "set netplugin host to identify itself (default: <host-name-reported-by-the-kernel>)",
		},
		cli.StringFlag{
			Name:   "vtep-ip",
			EnvVar: "CONTIV_NETPLUGIN_VTEP_IP",
			Usage:  "set netplugin vtep ip for vxlan communication (default: <host-ip-from-local-resolver>)",
		},
		cli.StringFlag{
			Name:   "ctrl-ip",
			EnvVar: "CONTIV_NETPLUGIN_CONTROL_IP",
			Usage:  "set netplugin control ip for control plane communication (default: <host-ip-from-local-resolver>)",
		},
		cli.StringFlag{
			Name:   "vlan-uplinks, vlan-if",
			EnvVar: "CONTIV_NETPLUGIN_VLAN_UPLINKS",
			Usage:  "a comma-delimited list of netplugin uplink interfaces",
		},
		cli.IntFlag{
			Name:   "vxlan-port",
			Value:  4789,
			EnvVar: "CONTIV_NETPLUGIN_VXLAN_PORT",
			Usage:  "set netplugin VXLAN port",
		},
	}
	app.Flags = utils.FlattenFlags(netpluginFlags, utils.BuildDBFlags(binName), utils.BuildNetworkFlags(binName), utils.BuildLogFlags(binName))
	sort.Sort(cli.FlagsByName(app.Flags))
	app.Action = func(ctx *cli.Context) error {
		configs, err := initNetPluginConfig(ctx)
		if err != nil {
			errmsg := err.Error()
			logrus.Error(errmsg)
			// use 22 Invalid argument as error return code
			// http://www-numi.fnal.gov/offline_software/srt_public_context/WebDocs/Errors/unix_system_errors.html
			return cli.NewExitError(errmsg, 22)
		}
		startNetPlugin(configs)
		return nil
	}
	app.Run(os.Args)
}
