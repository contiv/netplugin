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
	"log/syslog"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/netplugin/agent"
	"github.com/contiv/netplugin/netplugin/cluster"
	"github.com/contiv/netplugin/netplugin/plugin"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/netplugin/version"

	"github.com/Sirupsen/logrus"
	"github.com/Sirupsen/logrus/hooks/syslog"
	"github.com/urfave/cli"
)

func configureSyslog(syslogParam string) {
	var err error
	var hook logrus.Hook

	// disable colors if we're writing to syslog *and* we're the default text
	// formatter, because the tty detection is useless here.
	if tf, ok := logrus.StandardLogger().Formatter.(*logrus.TextFormatter); ok {
		tf.DisableColors = true
	}

	u, err := url.Parse(syslogParam)
	if err != nil {
		logrus.Fatalf("Could not parse syslog spec: %v", err)
	}

	hook, err = logrus_syslog.NewSyslogHook(u.Scheme, u.Host, syslog.LOG_INFO, "netplugin")
	if err != nil {
		logrus.Fatalf("Could not connect to syslog: %v", err)
	}

	logrus.AddHook(hook)
}

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

func initNetPluginConifg(ctx *cli.Context) (plugin.Config, error) {

	// 1. validate and set up log
	if ctx.Bool("use-syslog") {
		syslogURL := ctx.String("syslog-url")
		configureSyslog(syslogURL)
		logrus.Infof("Using netplugin syslog config: %v", syslogURL)
	} else {
		logrus.Info("Using netplugin syslog config: nil")
	}

	logLevel, err := logrus.ParseLevel(ctx.String("log-level"))
	if err != nil {
		return plugin.Config{}, err
	}
	logrus.SetLevel(logLevel)
	logrus.Infof("Using netplugin log level: %v", logLevel)

	if ctx.Bool("use-json-log") {
		logrus.SetFormatter(&logrus.JSONFormatter{})
		logrus.Info("Using netplugin log format: json")
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true, TimestampFormat: time.StampNano})
		logrus.Info("Using netplugin log format: text")
	}

	// 2. validate and set plugin mode
	pluginMode := strings.ToLower(ctx.String("mode"))
	switch pluginMode {
	case master.Docker, master.Kubernetes, master.SwarmMode, master.Test:
		logrus.Infof("Using netplugin mode: %v", pluginMode)
	case "":
		return plugin.Config{}, fmt.Errorf("netplugin mode is not set")
	default:
		return plugin.Config{}, fmt.Errorf("unknown netplugin mode: %v", pluginMode)
	}

	// 3. validate and set network mode
	networkMode := strings.ToLower(ctx.String("netmode"))
	switch networkMode {
	case "vlan", "vxlan":
		logrus.Infof("Using netplugin network mode: %v", networkMode)
	case "":
		return plugin.Config{}, fmt.Errorf("netplugin network mode is not set")
	default:
		return plugin.Config{}, fmt.Errorf("unknown netplugin network mode: %v", networkMode)
	}

	// 4. validate forward mode
	forwardMode := strings.ToLower(ctx.String("fwdmode"))
	if forwardMode == "" {
		return plugin.Config{}, fmt.Errorf("unknown netplugin forwarding mode: %v", forwardMode)
	} else if forwardMode != "bridge" && forwardMode != "routing" {
		return plugin.Config{}, fmt.Errorf("netplugin forwarding mode is not set")
	} else if networkMode == "vxlan" && forwardMode == "bridge" {
		return plugin.Config{}, fmt.Errorf("invalid netplugin forwarding mode: %q (network mode: %q)", forwardMode, networkMode)
	}
	// vxlan/vlan+routing, vlan+bridge are valid combinations
	logrus.Infof("Using netplugin forwarding mode: %v", forwardMode)

	// 5. validate and set other optional configs
	hostLabel := ctx.String("host")
	logrus.Infof("Using netplugin host: %v", hostLabel)
	controlIP := ctx.String("ctrl-ip")
	logrus.Infof("Using netplugin control IP: %v", controlIP)
	vtepIP := ctx.String("vtep-ip")
	if networkMode == "vxlan" && vtepIP == "" {
		return plugin.Config{}, fmt.Errorf("vtep-ip should be set when using VXLAN mode")
	}
	logrus.Infof("Using netplugin VTEP IP: %v", vtepIP)
	vlanUpLinks := strings.Split(ctx.String("vlan-uplinks"), ",")
	if networkMode == "vlan" && len(vlanUpLinks) == 0 {
		return plugin.Config{}, fmt.Errorf("vlan-uplinks should be set when using VLAN mode")
	}
	logrus.Infof("Using netplugin vlan uplinks: %v", vlanUpLinks)

	var stateStore string
	var stateStoreURL string

	for _, kvStore := range []string{"etcd", "consul"} {
		for _, endpoint := range strings.Split(ctx.String(kvStore), ",") {
			_, err := url.Parse(endpoint)
			if err != nil {
				return plugin.Config{}, fmt.Errorf("invalid netplugin %v endpoint: %v", kvStore, endpoint)
			}
			// TODO: support multi-endpoints
			stateStore = kvStore
			stateStoreURL = endpoint
			logrus.Infof("Using netplugin state storage endpoints: %v: %v", stateStore, stateStoreURL)
			break
		}
		if stateStore != "" && stateStoreURL != "" {
			break
		}
	}
	if stateStore == "" || stateStoreURL == "" {
		logrus.Error("unknown netplugin storage endpoints")
		return plugin.Config{}, fmt.Errorf("unknown netplugin endpoints")
	}

	vxlanPort := ctx.Int("vxlan-port")
	logrus.Infof("Using netplugin vxlan port: %v", vxlanPort)

	// initialize the config
	pluginConfig := plugin.Config{
		Drivers: plugin.Drivers{
			Network: utils.OvsNameStr,
			State:   stateStore,
		},
		Instance: core.InstanceInfo{
			HostLabel:    hostLabel,
			CtrlIP:       controlIP,
			VtepIP:       vtepIP,
			UplinkIntf:   vlanUpLinks,
			DbURL:        stateStoreURL,
			PluginMode:   pluginMode,
			VxlanUDPPort: vxlanPort,
			// TODO: pass in network mode
			FwdMode: forwardMode,
		},
	}

	return pluginConfig, nil
}

/*
netplugin supported models:
vxlan+routing, vlan+bridge/routing
*/

func main() {
	hostname, _ := os.Hostname()
	localIP, _ := cluster.GetLocalAddr()
	app := cli.NewApp()
	app.Version = "\n" + version.String()
	app.Usage = "Contiv netplugin service"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "log-level",
			Value:  "INFO",
			EnvVar: "CONTIV_NETPLUGIN_LOG_LEVEL",
			Usage:  "set netplugin log level, options: [DEBUG, INFO, WARN, ERROR]",
		},
		cli.BoolFlag{
			Name:   "use-json-log, json-log",
			EnvVar: "CONTIV_NETPLUGIN_USE_JSON_LOG",
			Usage:  "set netplugin log format to json",
		},
		cli.BoolFlag{
			Name:   "use-syslog, syslog",
			EnvVar: "CONTIV_NETPLUGIN_USE_SYSLOG",
			Usage:  "set netplugin send log to syslog or not",
		},
		cli.StringFlag{
			Name:   "syslog-url",
			Value:  "udp://127.0.0.1:514",
			EnvVar: "CONTIV_NETPLUGIN_SYSLOG_URL",
			Usage:  "set netplugin syslog url in format protocol://ip:port",
		},
		cli.StringFlag{
			Name:   "host, host-label",
			Value:  hostname,
			EnvVar: "CONTIV_NETPLUGIN_HOST",
			Usage:  "set netplugin host to identify itself",
		},
		cli.StringFlag{
			Name:   "mode, plugin-mode",
			EnvVar: "CONTIV_NETPLUGIN_MODE",
			Usage:  "set netplugin mode, options: [docker, kubernetes, swarm-mode]",
		},
		cli.StringFlag{
			Name:   "vtep-ip",
			Value:  localIP,
			EnvVar: "CONTIV_NETPLUGIN_VTEP_IP",
			Usage:  "set netplugin vtep ip for vxlan communication",
		},
		cli.StringFlag{
			Name:   "ctrl-ip",
			Value:  localIP,
			EnvVar: "CONTIV_NETPLUGIN_CONTROL_IP",
			Usage:  "set netplugin control ip for control plane communication",
		},
		cli.StringFlag{
			Name:   "vlan-uplinks, vlan-if",
			EnvVar: "CONTIV_NETPLUGIN_VLAN_UPLINKS",
			Usage:  "a comma-delimited list of netplugin uplink interfaces",
		},
		cli.StringFlag{
			Name:   "etcd-endpoints, etcd",
			EnvVar: "CONTIV_NETPLUGIN_ETCD_ENDPOINTS",
			Usage:  "a comma-delimited list of netplugin etcd endpoints",
		},
		cli.StringFlag{
			Name:   "consul-endpoints, consul",
			EnvVar: "CONTIV_NETPLUGIN_CONSUL_ENDPOINTS",
			Usage:  "a comma-delimited list of netplugin consul endpoints, ignored when etcd-endpoints is set",
		},
		cli.IntFlag{
			Name:   "vxlan-port",
			Value:  4789,
			EnvVar: "CONTIV_NETPLUGIN_VXLAN_PORT",
			Usage:  "set netplugin VXLAN port",
		},
		cli.StringFlag{
			Name:   "netmode, network-mode",
			EnvVar: "CONTIV_NETPLUGIN_NET_MODE",
			Usage:  "set netplugin network mode, options: [vlan, vxlan]",
		},
		cli.StringFlag{
			Name:   "fwdmode, forward-mode",
			EnvVar: "CONTIV_NETPLUGIN_FORWARD_MODE",
			Usage:  "set netplugin forwarding network mode, options: [bridge, routing]",
		},
		/*
			// only ovs is supported
			// TODO: turn it on when having more than one backend supported
			cli.StringFlag {
				Name: "driver, net-driver",
				Value: "ovs",
				EnvVar: "CONTIV_NETPLUGIN_DRIVER",
				Usage: "set netplugin key-value store url, options: [ovs, vpp]",
			}
		*/
	}
	sort.Sort(cli.FlagsByName(app.Flags))
	app.Action = func(ctx *cli.Context) error {
		configs, err := initNetPluginConifg(ctx)
		if err != nil {
			errmsg := err.Error()
			logrus.Error(errmsg)
			return cli.NewExitError(errmsg, (len(errmsg)%254 + 1))
		}
		startNetPlugin(&configs)
		return nil
	}
	app.Run(os.Args)
}
