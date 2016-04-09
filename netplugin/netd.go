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
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log/syslog"
	"net/url"
	"os"
	"os/user"
	"time"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/mgmtfn/dockplugin"
	"github.com/contiv/netplugin/mgmtfn/k8splugin"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/netplugin/cluster"
	"github.com/contiv/netplugin/netplugin/plugin"
	"github.com/contiv/netplugin/svcplugin"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/netplugin/version"

	log "github.com/Sirupsen/logrus"
	"github.com/Sirupsen/logrus/hooks/syslog"
)

// a daemon based on etcd client's Watch interface to trigger plugin's
// network provisioning interfaces

type cliOpts struct {
	hostLabel  string
	pluginMode string // plugin could be docker | kubernetes
	cfgFile    string
	debug      bool
	syslog     string
	jsonLog    bool
	ctrlIP     string // IP address to be used by control protocols
	vtepIP     string // IP address to be used by the VTEP
	vlanIntf   string // Uplink interface for VLAN switching
	version    bool
	routerIP   string // myrouter ip to start a protocol like Bgp
	fwdMode    string // default "bridge". Values: "routing" , "bridge"
}

func skipHost(vtepIP, homingHost, myHostLabel string) bool {
	return (vtepIP == "" && homingHost != myHostLabel ||
		vtepIP != "" && homingHost == myHostLabel)
}

func processCurrentState(netPlugin *plugin.NetPlugin, opts cliOpts) error {
	readNet := &mastercfg.CfgNetworkState{}
	readNet.StateDriver = netPlugin.StateDriver
	netCfgs, err := readNet.ReadAll()
	if err == nil {
		for idx, netCfg := range netCfgs {
			net := netCfg.(*mastercfg.CfgNetworkState)
			log.Debugf("read net key[%d] %s, populating state \n", idx, net.ID)
			processNetEvent(netPlugin, net, false)
		}
	}

	readEp := &mastercfg.CfgEndpointState{}
	readEp.StateDriver = netPlugin.StateDriver
	epCfgs, err := readEp.ReadAll()
	if err == nil {
		for idx, epCfg := range epCfgs {
			ep := epCfg.(*mastercfg.CfgEndpointState)
			log.Debugf("read ep key[%d] %s, populating state \n", idx, ep.ID)
			processEpState(netPlugin, opts, ep.ID)
		}
	}

	readBgp := &mastercfg.CfgBgpState{}
	readBgp.StateDriver = netPlugin.StateDriver
	bgpCfgs, err := readBgp.ReadAll()
	if err == nil {
		for idx, bgpCfg := range bgpCfgs {
			bgp := bgpCfg.(*mastercfg.CfgBgpState)
			log.Debugf("read bgp key[%d] %s, populating state \n", idx, bgp.Hostname)
			processBgpEvent(netPlugin, opts, bgp.Hostname, false)
		}
	}

	return nil
}

func processNetEvent(netPlugin *plugin.NetPlugin, nwCfg *mastercfg.CfgNetworkState,
	isDelete bool) (err error) {
	// take a lock to ensure we are programming one event at a time.
	// Also network create events need to be processed before endpoint creates
	// and reverse shall happen for deletes. That order is ensured by netmaster,
	// so we don't need to worry about that here
	netPlugin.Lock()
	defer func() { netPlugin.Unlock() }()

	operStr := ""
	if isDelete {
		err = netPlugin.DeleteNetwork(nwCfg.ID, nwCfg.PktTagType, nwCfg.PktTag, nwCfg.ExtPktTag,
			nwCfg.Gateway, nwCfg.Tenant)
		operStr = "delete"
	} else {
		err = netPlugin.CreateNetwork(nwCfg.ID)
		operStr = "create"
	}
	if err != nil {
		log.Errorf("Network operation %s failed. Error: %s", operStr, err)
	} else {
		log.Infof("Network operation %s succeeded", operStr)
	}

	return
}

// processEpState restores endpoint state
func processEpState(netPlugin *plugin.NetPlugin, opts cliOpts, epID string) error {
	// take a lock to ensure we are programming one event at a time.
	// Also network create events need to be processed before endpoint creates
	// and reverse shall happen for deletes. That order is ensured by netmaster,
	// so we don't need to worry about that here
	netPlugin.Lock()
	defer func() { netPlugin.Unlock() }()

	// read endpoint config
	epCfg := &mastercfg.CfgEndpointState{}
	epCfg.StateDriver = netPlugin.StateDriver
	err := epCfg.Read(epID)

	if err != nil {
		log.Errorf("Failed to read config for ep '%s' \n", epID)
		return err
	}
	// if the endpoint is not for this host, ignore it
	if skipHost(epCfg.VtepIP, epCfg.HomingHost, opts.hostLabel) {
		log.Infof("skipping mismatching host for ep %s. EP's host %s (my host: %s)",
			epID, epCfg.HomingHost, opts.hostLabel)
		return nil
	}

	// Create the endpoint
	err = netPlugin.CreateEndpoint(epID)
	if err != nil {
		log.Errorf("Endpoint operation create failed. Error: %s", err)
		return err
	}

	log.Infof("Endpoint operation create succeeded")

	return err
}

func processStateEvent(netPlugin *plugin.NetPlugin, opts cliOpts, rsps chan core.WatchState) {
	for {
		// block on change notifications
		rsp := <-rsps

		// For now we deal with only create and delete events
		currentState := rsp.Curr
		isDelete := false
		eventStr := "create"
		if rsp.Curr == nil {
			currentState = rsp.Prev
			isDelete = true
			eventStr = "delete"
		} else if rsp.Prev != nil {
			log.Infof("Received a modify event, ignoring it")
			if bgpCfg, ok := currentState.(*mastercfg.CfgBgpState); ok {
				log.Infof("Received %q for Bgp: %q", eventStr, bgpCfg.Hostname)
				processBgpEvent(netPlugin, opts, bgpCfg.Hostname, isDelete)
				continue
			}
			log.Infof("Received a modify event, ignoring it")
			continue

		}
		if nwCfg, ok := currentState.(*mastercfg.CfgNetworkState); ok {
			log.Infof("Received %q for network: %q", eventStr, nwCfg.ID)
			processNetEvent(netPlugin, nwCfg, isDelete)
		}
		if bgpCfg, ok := currentState.(*mastercfg.CfgBgpState); ok {
			log.Infof("Received %q for Bgp: %q", eventStr, bgpCfg.Hostname)
			processBgpEvent(netPlugin, opts, bgpCfg.Hostname, isDelete)
		}
	}
}

func handleNetworkEvents(netPlugin *plugin.NetPlugin, opts cliOpts, retErr chan error) {
	rsps := make(chan core.WatchState)
	go processStateEvent(netPlugin, opts, rsps)
	cfg := mastercfg.CfgNetworkState{}
	cfg.StateDriver = netPlugin.StateDriver
	retErr <- cfg.WatchAll(rsps)
	return
}

func handleBgpEvents(netPlugin *plugin.NetPlugin, opts cliOpts, recvErr chan error) {

	rsps := make(chan core.WatchState)
	go processStateEvent(netPlugin, opts, rsps)
	cfg := mastercfg.CfgBgpState{}
	cfg.StateDriver = netPlugin.StateDriver
	recvErr <- cfg.WatchAll(rsps)
	return
}

func handleEvents(netPlugin *plugin.NetPlugin, opts cliOpts) error {

	recvErr := make(chan error, 1)
	go handleNetworkEvents(netPlugin, opts, recvErr)

	go handleBgpEvents(netPlugin, opts, recvErr)

	err := <-recvErr
	if err != nil {
		log.Errorf("Failure occured. Error: %s", err)
		return err
	}

	return nil
}

func configureSyslog(syslogParam string) {
	var err error
	var hook log.Hook

	// disable colors if we're writing to syslog *and* we're the default text
	// formatter, because the tty detection is useless here.
	if tf, ok := log.StandardLogger().Formatter.(*log.TextFormatter); ok {
		tf.DisableColors = true
	}

	if syslogParam == "kernel" {
		hook, err = logrus_syslog.NewSyslogHook("", "", syslog.LOG_INFO, "netplugin")
		if err != nil {
			log.Fatalf("Could not connect to kernel syslog")
		}
	} else {
		u, err := url.Parse(syslogParam)
		if err != nil {
			log.Fatalf("Could not parse syslog spec: %v", err)
		}

		hook, err = logrus_syslog.NewSyslogHook(u.Scheme, u.Host, syslog.LOG_INFO, "netplugin")
		if err != nil {
			log.Fatalf("Could not connect to syslog: %v", err)
		}
	}

	log.AddHook(hook)
}

func main() {
	var opts cliOpts
	var flagSet *flag.FlagSet

	defHostLabel, err := os.Hostname()
	if err != nil {
		log.Fatalf("Failed to fetch hostname. Error: %s", err)
	}

	// parse rest of the args that require creating state
	flagSet = flag.NewFlagSet("netd", flag.ExitOnError)
	flagSet.BoolVar(&opts.debug,
		"debug",
		false,
		"Show debugging information generated by netplugin")
	flagSet.StringVar(&opts.syslog,
		"syslog",
		"",
		"Log to syslog at proto://ip:port -- use 'kernel' to log via kernel syslog")
	flagSet.BoolVar(&opts.jsonLog,
		"json-log",
		false,
		"Format logs as JSON")
	flagSet.StringVar(&opts.hostLabel,
		"host-label",
		defHostLabel,
		"label used to identify endpoints homed for this host, default is host name. If -config flag is used then host-label must be specified in the the configuration passed.")
	flagSet.StringVar(&opts.pluginMode,
		"plugin-mode",
		"docker",
		"plugin mode docker|kubernetes")
	flagSet.StringVar(&opts.cfgFile,
		"config",
		"",
		"plugin configuration. Use '-' to read configuration from stdin")
	flagSet.StringVar(&opts.vtepIP,
		"vtep-ip",
		"",
		"My VTEP ip address")
	flagSet.StringVar(&opts.ctrlIP,
		"ctrl-ip",
		"",
		"Local ip address to be used for control communication")
	flagSet.StringVar(&opts.vlanIntf,
		"vlan-if",
		"",
		"My VTEP ip address")
	flagSet.BoolVar(&opts.version,
		"version",
		false,
		"Show version")
	flagSet.StringVar(&opts.routerIP,
		"router-ip",
		"",
		"My Router ip address")
	flagSet.StringVar(&opts.fwdMode,
		"fwd-mode",
		"bridge",
		"Forwarding Mode")

	err = flagSet.Parse(os.Args[1:])
	if err != nil {
		log.Fatalf("Failed to parse command. Error: %s", err)
	}

	if opts.version {
		fmt.Printf(version.String())
		os.Exit(0)
	}

	// Make sure we are running as root
	usr, err := user.Current()
	if (err != nil) || (usr.Username != "root") {
		log.Fatalf("This process can only be run as root")
	}

	if opts.debug {
		log.SetLevel(log.DebugLevel)
		os.Setenv("CONTIV_TRACE", "1")
	}

	if opts.jsonLog {
		log.SetFormatter(&log.JSONFormatter{})
	} else {
		log.SetFormatter(&log.TextFormatter{FullTimestamp: true, TimestampFormat: time.StampNano})
	}

	if opts.syslog != "" {
		configureSyslog(opts.syslog)
	}

	if opts.fwdMode != "bridge" && opts.fwdMode != "routing" {
		log.Infof("Invalid forwarding mode. Setting the mode to bridge ")
		opts.fwdMode = "bridge"
	}

	if flagSet.NFlag() < 1 {
		log.Infof("host-label not specified, using default (%s)", opts.hostLabel)
	}

	// default to using local IP addr
	localIP, err := cluster.GetLocalAddr()
	if err != nil {
		log.Fatalf("Error getting local address. Err: %v", err)
	}
	if opts.vtepIP == "" {
		opts.vtepIP = localIP
	}
	if opts.ctrlIP == "" {
		opts.ctrlIP = localIP
	}

	defConfigStr := fmt.Sprintf(`{
                    "drivers" : {
                       "network": %q,
                       "state": "etcd"
                    },
                    "plugin-instance": {
                       "host-label": %q,
						"vtep-ip": %q,
						"vlan-if": %q,
						"router-ip":%q,
						"fwdMode":%q
                    },
                    %q : {
                       "dbip": "127.0.0.1",
                       "dbport": 6640
                    },
                    "etcd" : {
                        "machines": ["http://127.0.0.1:4001"]
                    },
                    "docker" : {
                        "socket" : "unix:///var/run/docker.sock"
                    }
                  }`, utils.OvsNameStr, opts.hostLabel, opts.vtepIP,
		opts.vlanIntf, opts.routerIP, opts.fwdMode, utils.OvsNameStr)

	netPlugin := &plugin.NetPlugin{}

	config := []byte{}
	if opts.cfgFile == "" {
		log.Infof("config not specified, using default config")
		config = []byte(defConfigStr)
	} else if opts.cfgFile == "-" {
		reader := bufio.NewReader(os.Stdin)
		config, err = ioutil.ReadAll(reader)
		if err != nil {
			log.Fatalf("reading config from stdin failed. Error: %s", err)
		}
	} else {
		config, err = ioutil.ReadFile(opts.cfgFile)
		if err != nil {
			log.Fatalf("reading config from file failed. Error: %s", err)
		}
	}

	// Parse the config
	pluginConfig := plugin.Config{}
	err = json.Unmarshal([]byte(config), &pluginConfig)
	if err != nil {
		log.Fatalf("Error parsing config. Err: %v", err)
	}

	// extract host-label from the configuration
	if pluginConfig.Instance.HostLabel == "" {
		log.Fatalf("Empty host-label passed in configuration")
	}
	opts.hostLabel = pluginConfig.Instance.HostLabel

	// Use default values when config options are not specified
	if pluginConfig.Instance.VtepIP == "" {
		pluginConfig.Instance.VtepIP = opts.vtepIP
	}
	if pluginConfig.Instance.VlanIntf == "" {
		pluginConfig.Instance.VlanIntf = opts.vlanIntf
	}
	if pluginConfig.Instance.RouterIP == "" {
		pluginConfig.Instance.RouterIP = opts.routerIP
	}
	if pluginConfig.Instance.FwdMode == "" {
		pluginConfig.Instance.FwdMode = opts.fwdMode
	}

	svcplugin.QuitCh = make(chan struct{})
	defer close(svcplugin.QuitCh)

	// Initialize appropriate plugin
	switch opts.pluginMode {
	case "docker":
		dockplugin.InitDockPlugin(netPlugin)

	case "kubernetes":
		k8splugin.InitCNIServer(netPlugin)

	default:
		log.Fatalf("Unknown plugin mode -- should be docker | kubernetes")
	}

	// Init the driver plugins..
	err = netPlugin.Init(pluginConfig, string(config))
	if err != nil {
		log.Fatalf("Failed to initialize the plugin. Error: %s", err)
	}

	// Process all current state
	processCurrentState(netPlugin, opts)

	// Initialize clustering
	cluster.Init(netPlugin, opts.ctrlIP)

	//logger := log.New(os.Stdout, "go-etcd: ", log.LstdFlags)
	//etcd.SetLogger(logger)

	if opts.pluginMode == "kubernetes" {
		k8splugin.InitKubServiceWatch(netPlugin)
	}

	if err := handleEvents(netPlugin, opts); err != nil {
		os.Exit(1)
	}
}

//processBgpEvent processes Bgp neighbor add/delete events
func processBgpEvent(netPlugin *plugin.NetPlugin, opts cliOpts, hostID string,
	isDelete bool) (err error) {

	if opts.hostLabel != hostID {
		log.Errorf("Ignoring Bgp Event on this host")
		return
	}
	netPlugin.Lock()
	defer func() { netPlugin.Unlock() }()

	operStr := ""
	if isDelete {
		err = netPlugin.DeleteBgp(hostID)
		operStr = "delete"
	} else {
		err = netPlugin.AddBgp(hostID)
		operStr = "create"
	}
	if err != nil {
		log.Errorf("Bgp operation %s failed. Error: %s", operStr, err)
	} else {
		log.Infof("Bgp operation %s succeeded", operStr)
	}
	return
}
