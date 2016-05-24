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
	"flag"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/Sirupsen/logrus/hooks/syslog"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/mgmtfn/dockplugin"
	"github.com/contiv/netplugin/mgmtfn/k8splugin"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/netplugin/cluster"
	"github.com/contiv/netplugin/netplugin/plugin"
	"github.com/contiv/netplugin/svcplugin"
	"github.com/contiv/netplugin/utils/netutils"
	"github.com/contiv/netplugin/version"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/samalba/dockerclient"
	"golang.org/x/net/context"
	"log/syslog"
	"net/url"
	"os"
	"os/user"
	"strconv"
	"strings"
	"time"
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
	dbURL      string // state store URL
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
			if net.NwType == "infra" {
				processInfraNwCreate(netPlugin, net, opts)
			}
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

	readServiceLb := &mastercfg.CfgServiceLBState{}
	readServiceLb.StateDriver = netPlugin.StateDriver
	serviceLbCfgs, err := readServiceLb.ReadAll()
	if err == nil {
		for idx, serviceLbCfg := range serviceLbCfgs {
			serviceLb := serviceLbCfg.(*mastercfg.CfgServiceLBState)
			log.Debugf("read svc key[%d] %s for tenant %s, populating state \n", idx,
				serviceLb.ServiceName, serviceLb.Tenant)
			processServiceLBEvent(netPlugin, opts, serviceLb, false)
		}
	}

	readSvcProviders := &mastercfg.SvcProvider{}
	readSvcProviders.StateDriver = netPlugin.StateDriver
	svcProviders, err := readSvcProviders.ReadAll()
	if err == nil {
		for idx, providers := range svcProviders {
			svcProvider := providers.(*mastercfg.SvcProvider)
			log.Infof("read svc provider[%d] %s , populating state \n", idx,
				svcProvider.ServiceName)
			processSvcProviderUpdEvent(netPlugin, opts, svcProvider, false)
		}
	}

	return nil
}

// Process Infra Nw Create
// Auto allocate an endpoint for this node
func processInfraNwCreate(netPlugin *plugin.NetPlugin, nwCfg *mastercfg.CfgNetworkState, opts cliOpts) (err error) {
	pluginHost := opts.hostLabel

	// Build endpoint request
	mreq := master.CreateEndpointRequest{
		TenantName:  nwCfg.Tenant,
		NetworkName: nwCfg.NetworkName,
		EndpointID:  pluginHost,
		ConfigEP: intent.ConfigEP{
			Container: pluginHost,
			Host:      pluginHost,
		},
	}

	var mresp master.CreateEndpointResponse
	err = cluster.MasterPostReq("/plugin/createEndpoint", &mreq, &mresp)
	if err != nil {
		log.Errorf("master failed to create endpoint %s", err)
		return err
	}

	log.Infof("Got endpoint create resp from master: %+v", mresp)

	// Take lock to ensure netPlugin processes only one cmd at a time
	netPlugin.Lock()
	defer func() { netPlugin.Unlock() }()

	// Ask netplugin to create the endpoint
	netID := nwCfg.NetworkName + "." + nwCfg.Tenant
	err = netPlugin.CreateEndpoint(netID + "-" + pluginHost)
	if err != nil {
		log.Errorf("Endpoint creation failed. Error: %s", err)
		return err
	}

	// Assign IP to interface
	ipCIDR := fmt.Sprintf("%s/%d", mresp.EndpointConfig.IPAddress, nwCfg.SubnetLen)
	err = netutils.SetInterfaceIP(nwCfg.NetworkName, ipCIDR)
	if err != nil {
		log.Errorf("Could not assign ip: %s", err)
		return err
	}

	return nil
}

// Process Infra Nw Delete
// Delete the auto allocated endpoint
func processInfraNwDelete(netPlugin *plugin.NetPlugin, nwCfg *mastercfg.CfgNetworkState, opts cliOpts) (err error) {
	pluginHost := opts.hostLabel

	// Build endpoint request
	mreq := master.DeleteEndpointRequest{
		TenantName:  nwCfg.Tenant,
		NetworkName: nwCfg.NetworkName,
		EndpointID:  pluginHost,
	}

	var mresp master.DeleteEndpointResponse
	err = cluster.MasterPostReq("/plugin/deleteEndpoint", &mreq, &mresp)
	if err != nil {
		log.Errorf("master failed to delete endpoint %s", err)
		return err
	}

	log.Infof("Got endpoint create resp from master: %+v", mresp)

	// Network delete will take care of infra nw EP delete in plugin

	return
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
		err = netPlugin.DeleteNetwork(nwCfg.ID, nwCfg.NwType, nwCfg.PktTagType, nwCfg.PktTag, nwCfg.ExtPktTag,
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

//processBgpEvent processes Bgp neighbor add/delete events
func processBgpEvent(netPlugin *plugin.NetPlugin, opts cliOpts, hostID string, isDelete bool) error {
	var err error

	if opts.hostLabel != hostID {
		log.Errorf("Ignoring Bgp Event on this host")
		return err
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
			} /*
				if serviceLbCfg, ok := currentState.(*mastercfg.CfgServiceLBState); ok {
					log.Infof("Received %q for Service %s on tenant %s", eventStr,
						serviceLbCfg.ServiceName, serviceLbCfg.Tenant)
					processServiceLBEvent(netPlugin, opts, serviceLbCfg, isDelete)
				}*/

			if svcProvider, ok := currentState.(*mastercfg.SvcProvider); ok {
				log.Infof("Received %q for Service %s , provider:%#v", eventStr,
					svcProvider.ServiceName, svcProvider.Providers)
				processSvcProviderUpdEvent(netPlugin, opts, svcProvider, isDelete)
			}

			log.Infof("Received a modify event, ignoring it")
			continue

		}

		if nwCfg, ok := currentState.(*mastercfg.CfgNetworkState); ok {
			log.Infof("Received %q for network: %q", eventStr, nwCfg.ID)
			if isDelete != true {
				processNetEvent(netPlugin, nwCfg, isDelete)
				if nwCfg.NwType == "infra" {
					processInfraNwCreate(netPlugin, nwCfg, opts)
				}
			} else {
				if nwCfg.NwType == "infra" {
					processInfraNwDelete(netPlugin, nwCfg, opts)
				}
				processNetEvent(netPlugin, nwCfg, isDelete)
			}
		}
		if bgpCfg, ok := currentState.(*mastercfg.CfgBgpState); ok {
			log.Infof("Received %q for Bgp: %q", eventStr, bgpCfg.Hostname)
			processBgpEvent(netPlugin, opts, bgpCfg.Hostname, isDelete)
		}
		if serviceLbCfg, ok := currentState.(*mastercfg.CfgServiceLBState); ok {
			log.Infof("Received %q for Service %s on tenant %s", eventStr,
				serviceLbCfg.ServiceName, serviceLbCfg.Tenant)
			processServiceLBEvent(netPlugin, opts, serviceLbCfg, isDelete)
		}
		if svcProvider, ok := currentState.(*mastercfg.SvcProvider); ok {
			log.Infof("Received %q for Service %s on tenant %s", eventStr,
				svcProvider.ServiceName, svcProvider.Providers)
			processSvcProviderUpdEvent(netPlugin, opts, svcProvider, isDelete)
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

func handleServiceLBEvents(netPlugin *plugin.NetPlugin, opts cliOpts, recvErr chan error) {

	rsps := make(chan core.WatchState)
	go processStateEvent(netPlugin, opts, rsps)
	cfg := mastercfg.CfgServiceLBState{}
	cfg.StateDriver = netPlugin.StateDriver
	recvErr <- cfg.WatchAll(rsps)
	return
}

func handleSvcProviderUpdEvents(netPlugin *plugin.NetPlugin, opts cliOpts, recvErr chan error) {
	rsps := make(chan core.WatchState)
	go processStateEvent(netPlugin, opts, rsps)
	cfg := mastercfg.SvcProvider{}
	cfg.StateDriver = netPlugin.StateDriver
	recvErr <- cfg.WatchAll(rsps)
	return
}

func handleEvents(netPlugin *plugin.NetPlugin, opts cliOpts) error {

	recvErr := make(chan error, 1)

	go handleNetworkEvents(netPlugin, opts, recvErr)

	go handleBgpEvents(netPlugin, opts, recvErr)

	go handleServiceLBEvents(netPlugin, opts, recvErr)

	go handleSvcProviderUpdEvents(netPlugin, opts, recvErr)

	docker, _ := dockerclient.NewDockerClient("unix:///var/run/docker.sock", nil)
	go docker.StartMonitorEvents(handleDockerEvents, recvErr, netPlugin, recvErr)

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
	flagSet.StringVar(&opts.fwdMode,
		"fwd-mode",
		"bridge",
		"Forwarding Mode")
	flagSet.StringVar(&opts.dbURL,
		"cluster-store",
		"etcd://127.0.0.1:2379",
		"state store url")

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
		log.Fatalf("Invalid forwarding mode. Allowed modes are bridge,routing ")
	}

	if flagSet.NFlag() < 1 {
		log.Infof("host-label not specified, using default (%s)", opts.hostLabel)
	}

	// default to using local IP addr
	localIP, err := cluster.GetLocalAddr()
	if err != nil {
		log.Fatalf("Error getting local address. Err: %v", err)
	}
	if opts.ctrlIP == "" {
		opts.ctrlIP = localIP
	}
	if opts.vtepIP == "" {
		opts.vtepIP = opts.ctrlIP
	}

	// parse store URL
	parts := strings.Split(opts.dbURL, "://")
	if len(parts) < 2 {
		log.Fatalf("Invalid cluster-store-url %s", opts.dbURL)
	}
	stateStore := parts[0]

	netPlugin := &plugin.NetPlugin{}

	// initialize the config
	pluginConfig := plugin.Config{
		Drivers: plugin.Drivers{
			Network: "ovs",
			State:   stateStore,
		},
		Instance: core.InstanceInfo{
			HostLabel: opts.hostLabel,
			VtepIP:    opts.vtepIP,
			VlanIntf:  opts.vlanIntf,
			RouterIP:  opts.routerIP,
			FwdMode:   opts.fwdMode,
			DbURL:     opts.dbURL,
		},
	}

	// Initialize service registry plugin
	svcPlugin, quitCh, err := svcplugin.NewSvcregPlugin(opts.dbURL, nil)
	if err != nil {
		log.Fatalf("Error initializing service registry plugin")
	}
	defer close(quitCh)

	// Initialize appropriate plugin
	switch opts.pluginMode {
	case "docker":
		dockplugin.InitDockPlugin(netPlugin, svcPlugin)

	case "kubernetes":
		k8splugin.InitCNIServer(netPlugin)

	default:
		log.Fatalf("Unknown plugin mode -- should be docker | kubernetes")
	}

	// Init the driver plugins..
	err = netPlugin.Init(pluginConfig)
	if err != nil {
		log.Fatalf("Failed to initialize the plugin. Error: %s", err)
	}

	// Process all current state
	processCurrentState(netPlugin, opts)

	// Initialize clustering
	cluster.Init(netPlugin, opts.ctrlIP, opts.vtepIP, opts.dbURL)

	if opts.pluginMode == "kubernetes" {
		k8splugin.InitKubServiceWatch(netPlugin)
	}

	if err := handleEvents(netPlugin, opts); err != nil {
		os.Exit(1)
	}
}

//processServiceLBEvent processes service load balancer object events

func processServiceLBEvent(netPlugin *plugin.NetPlugin, opts cliOpts, svcLBCfg *mastercfg.CfgServiceLBState,
	isDelete bool) error {
	var err error
	portSpecList := []core.PortSpec{}
	portSpec := core.PortSpec{}

	netPlugin.Lock()
	defer func() { netPlugin.Unlock() }()
	serviceID := svcLBCfg.ID

	log.Infof("Recevied Process Service load balancer event {%v}", svcLBCfg)

	//create portspect list from state.
	//Ports format: servicePort:ProviderPort:Protocol
	for _, port := range svcLBCfg.Ports {

		portInfo := strings.Split(port, ":")
		if len(portInfo) != 3 {
			return errors.New("Invalid Port Format")
		}
		svcPort := portInfo[0]
		provPort := portInfo[1]
		portSpec.Protocol = portInfo[2]

		sPort, _ := strconv.ParseUint(svcPort, 10, 16)
		portSpec.SvcPort = uint16(sPort)

		pPort, _ := strconv.ParseUint(provPort, 10, 16)
		portSpec.ProvPort = uint16(pPort)

		portSpecList = append(portSpecList, portSpec)
	}

	spec := &core.ServiceSpec{
		IPAddress: svcLBCfg.IPAddress,
		Ports:     portSpecList,
	}
	operStr := ""
	if isDelete {
		err = netPlugin.DeleteServiceLB(serviceID, spec)
		operStr = "delete"
	} else {
		err = netPlugin.AddServiceLB(serviceID, spec)
		operStr = "create"
	}
	if err != nil {
		log.Errorf("Service Load Balancer %s failed.Error:%s", operStr, err)
		return err
	}
	log.Infof("Service Load Balancer %s succeeded", operStr)

	return nil
}

//processSvcProviderUpdEvent updates service provider events
func processSvcProviderUpdEvent(netPlugin *plugin.NetPlugin, opts cliOpts,
	svcProvider *mastercfg.SvcProvider, isDelete bool) error {
	if isDelete {
		//ignore delete event since servicelb delete will take care of this.
		return nil
	}
	netPlugin.SvcProviderUpdate(svcProvider.ServiceName, svcProvider.Providers)
	return nil
}

/*Handles docker events monitored by dockerclient. Currently we only handle
  container start and die event*/
func handleDockerEvents(event *dockerclient.Event, ec chan error, args ...interface{}) {

	log.Printf("Received Docker event: {%#v}\n", *event)
	providerUpdReq := &master.SvcProvUpdateRequest{}
	switch event.Status {
	case "start":
		defaultHeaders := map[string]string{"User-Agent": "engine-api-cli-1.0"}
		cli, err := client.NewClient("unix:///var/run/docker.sock", "v1.21", nil, defaultHeaders)
		if err != nil {
			panic(err)
		}
		containerInfo, err := cli.ContainerInspect(context.Background(), event.ID)
		if err != nil {
			log.Errorf("Container Inspect failed :%s", err)
			return
		}
		if event.ID != "" {
			labelMap := getLabelsFromContainerInspect(&containerInfo)

			containerTenant := getTenantFromContainerInspect(&containerInfo)
			network, ipAddress := getEpNetworkInfoFromContainerInspect(&containerInfo)
			container := getContainerFromContainerInspect(&containerInfo)
			if ipAddress != "" {
				//Create provider info
				networkname := strings.Split(network, "/")[0]
				providerUpdReq.IPAddress = ipAddress
				providerUpdReq.ContainerID = event.ID
				providerUpdReq.Tenant = containerTenant
				providerUpdReq.Network = networkname
				providerUpdReq.Event = "start"
				providerUpdReq.Container = container
				providerUpdReq.Labels = make(map[string]string)

				for k, v := range labelMap {
					providerUpdReq.Labels[k] = v
				}
			}
			if len(labelMap) == 0 {
				//Ignore container without labels
				return
			}
			var svcProvResp master.SvcProvUpdateResponse

			log.Infof("Sending Provider create request to master: {%+v}", providerUpdReq)

			err := cluster.MasterPostReq("/plugin/svcProviderUpdate", providerUpdReq, &svcProvResp)
			if err != nil {
				log.Errorf("Event: 'start' , Http error posting service provider update, Error:%s", err)
			}
		} else {
			log.Errorf("Unable to fetch container labels for container %s ", event.ID)
		}
	case "die":
		providerUpdReq.ContainerID = event.ID
		providerUpdReq.Event = "die"
		var svcProvResp master.SvcProvUpdateResponse
		log.Infof("Sending Provider delete request to master: {%+v}", providerUpdReq)
		err := cluster.MasterPostReq("/plugin/svcProviderUpdate", providerUpdReq, &svcProvResp)
		if err != nil {
			log.Errorf("Event:'die' Http error posting service provider update, Error:%s", err)
		}
	}
}

//getLabelsFromContainerInspect returns the labels associated with the container
func getLabelsFromContainerInspect(containerInfo *types.ContainerJSON) map[string]string {
	if containerInfo != nil && containerInfo.Config != nil {
		return containerInfo.Config.Labels
	}
	return nil
}

//getTenantFromContainerInspect returns the tenant the container belongs to.
func getTenantFromContainerInspect(containerInfo *types.ContainerJSON) string {
	tenant := "default"
	if containerInfo != nil && containerInfo.NetworkSettings != nil {
		for network := range containerInfo.NetworkSettings.Networks {
			if strings.Contains(network, "/") {
				//network name is of the form networkname/tenantname for non default tenant
				tenant = strings.Split(network, "/")[1]
			}
		}
	}
	return tenant
}

/*getEpNetworkInfoFromContainerInspect inspects the network info from containerinfo returned by dockerclient*/
func getEpNetworkInfoFromContainerInspect(containerInfo *types.ContainerJSON) (string, string) {
	var networkName string
	var IPAddress string

	if containerInfo != nil && containerInfo.NetworkSettings != nil {
		for network, endpoint := range containerInfo.NetworkSettings.Networks {
			networkName = network
			if strings.Contains(network, "/") {
				//network name is of the form networkname/tenantname for non default tenant
				networkName = strings.Split(network, "/")[0]
			}
			IPAddress = endpoint.IPAddress
		}
	}
	return networkName, IPAddress
}

func getContainerFromContainerInspect(containerInfo *types.ContainerJSON) string {

	container := ""
	if containerInfo != nil && containerInfo.NetworkSettings != nil {
		for _, endpoint := range containerInfo.NetworkSettings.Networks {
			container = endpoint.EndpointID
		}
	}
	return container

}
