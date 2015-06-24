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
	"os/exec"
	"strings"
	"time"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/crt"
	"github.com/contiv/netplugin/crtclient"
	"github.com/contiv/netplugin/crtclient/docker"
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/netutils"
	"github.com/contiv/netplugin/plugin"
	"github.com/contiv/netplugin/utils"
	"github.com/samalba/dockerclient"

	log "github.com/Sirupsen/logrus"
	"github.com/Sirupsen/logrus/hooks/syslog"
)

// a daemon based on etcd client's Watch interface to trigger plugin's
// network provisioning interfaces

type cliOpts struct {
	hostLabel   string
	nativeInteg bool
	cfgFile     string
	debug       bool
	syslog      string
	jsonLog     bool
	vtepIP      string
	vlanIntf    string
}

func skipHost(vtepIP, homingHost, myHostLabel string) bool {
	return (vtepIP == "" && homingHost != myHostLabel ||
		vtepIP != "" && homingHost == myHostLabel)
}

func processCurrentState(netPlugin *plugin.NetPlugin, crt *crt.CRT,
	opts cliOpts) error {
	readNet := &drivers.OvsCfgNetworkState{}
	readNet.StateDriver = netPlugin.StateDriver
	netCfgs, err := readNet.ReadAll()
	if err == nil {
		for idx, netCfg := range netCfgs {
			net := netCfg.(*drivers.OvsCfgNetworkState)
			log.Debugf("read net key[%d] %s, populating state \n", idx, net.ID)
			processNetEvent(netPlugin, net.ID, false)
		}
	}

	readEp := &drivers.OvsCfgEndpointState{}
	readEp.StateDriver = netPlugin.StateDriver
	epCfgs, err := readEp.ReadAll()
	if err == nil {
		for idx, epCfg := range epCfgs {
			ep := epCfg.(*drivers.OvsCfgEndpointState)
			log.Debugf("read ep key[%d] %s, populating state \n", idx, ep.ID)
			processEpEvent(netPlugin, crt, opts, ep.ID, false)
		}
	}

	peer := &drivers.PeerHostState{}
	peer.StateDriver = netPlugin.StateDriver
	peerList, err := peer.ReadAll()
	if err == nil {
		for idx, peerState := range peerList {
			peerInfo := peerState.(*drivers.PeerHostState)
			log.Debugf("read peer key[%d] %s, populating state \n", idx, peerInfo.ID)
			processPeerEvent(netPlugin, opts, peerInfo.ID, false)
		}
	}

	return nil
}

func processNetEvent(netPlugin *plugin.NetPlugin, netID string,
	isDelete bool) (err error) {
	// take a lock to ensure we are programming one event at a time.
	// Also network create events need to be processed before endpoint creates
	// and reverse shall happen for deletes. That order is ensured by netmaster,
	// so we don't need to worry about that here
	netPlugin.Lock()
	defer func() { netPlugin.Unlock() }()

	operStr := ""
	if isDelete {
		err = netPlugin.DeleteNetwork(netID)
		operStr = "delete"
	} else {
		err = netPlugin.CreateNetwork(netID)
		operStr = "create"
	}
	if err != nil {
		log.Errorf("Network operation %s failed. Error: %s", operStr, err)
	} else {
		log.Infof("Network operation %s succeeded", operStr)
	}

	return
}

func processPeerEvent(netPlugin *plugin.NetPlugin, opts cliOpts, peerID string, isDelete bool) (err error) {
	// if this is our own peer info coming back to us, ignore it
	if peerID == opts.hostLabel {
		return nil
	}

	// take a lock to ensure we are programming one event at a time.
	netPlugin.Lock()
	defer func() { netPlugin.Unlock() }()

	operStr := ""
	if isDelete {
		err = netPlugin.DeletePeerHost(peerID)
		operStr = "delete"
	} else {
		err = netPlugin.CreatePeerHost(peerID)
		operStr = "create"
	}
	if err != nil {
		log.Errorf("PeerHost operation %s failed. Error: %s", operStr, err)
	} else {
		log.Infof("PeerHost operation %s succeeded", operStr)
	}

	return
}

func getEndpointContainerContext(stateDriver core.StateDriver, epID string) (
	*crtclient.ContainerEPContext, error) {
	var epCtx crtclient.ContainerEPContext
	var err error

	epCfg := &drivers.OvsCfgEndpointState{}
	epCfg.StateDriver = stateDriver
	err = epCfg.Read(epID)
	if err != nil {
		return &epCtx, nil
	}
	epCtx.NewContName = epCfg.ContName
	epCtx.NewAttachUUID = epCfg.AttachUUID

	cfgNet := &drivers.OvsCfgNetworkState{}
	cfgNet.StateDriver = stateDriver
	err = cfgNet.Read(epCfg.NetID)
	if err != nil {
		return &epCtx, err
	}
	epCtx.DefaultGw = cfgNet.DefaultGw
	epCtx.SubnetLen = cfgNet.SubnetLen

	operEp := &drivers.OvsOperEndpointState{}
	operEp.StateDriver = stateDriver
	err = operEp.Read(epID)
	if err != nil {
		return &epCtx, nil
	}
	epCtx.CurrContName = operEp.ContName
	epCtx.InterfaceID = operEp.PortName
	epCtx.IPAddress = operEp.IPAddress
	epCtx.CurrAttachUUID = operEp.AttachUUID

	return &epCtx, err
}

func getContainerEPContextByContName(stateDriver core.StateDriver, contName string) (
	epCtxs []crtclient.ContainerEPContext, err error) {
	var epCtx *crtclient.ContainerEPContext

	contName = strings.TrimPrefix(contName, "/")
	readEp := &drivers.OvsCfgEndpointState{}
	readEp.StateDriver = stateDriver
	epCfgs, err := readEp.ReadAll()
	if err != nil {
		return
	}

	epCtxs = make([]crtclient.ContainerEPContext, len(epCfgs))
	idx := 0
	for _, epCfg := range epCfgs {
		cfg := epCfg.(*drivers.OvsCfgEndpointState)
		if cfg.ContName != contName {
			continue
		}

		epCtx, err = getEndpointContainerContext(stateDriver, cfg.ID)
		if err != nil {
			log.Errorf("error '%s' getting epCfgState for ep %s \n",
				err, cfg.ID)
			return epCtxs[:idx], nil
		}
		epCtxs[idx] = *epCtx
		idx = idx + 1
	}

	return epCtxs[:idx], nil
}

func contAttachPointAdded(epCtx *crtclient.ContainerEPContext) bool {
	if epCtx.CurrAttachUUID == "" && epCtx.NewAttachUUID != "" {
		return true
	}
	if epCtx.CurrContName == "" && epCtx.NewContName != "" {
		return true
	}
	return false
}

func contAttachPointDeleted(epCtx *crtclient.ContainerEPContext) bool {
	if epCtx.CurrAttachUUID != "" && epCtx.NewAttachUUID == "" {
		return true
	}
	if epCtx.CurrContName != "" && epCtx.NewContName == "" {
		return true
	}
	return false
}

func processEpEvent(netPlugin *plugin.NetPlugin, crt *crt.CRT, opts cliOpts,
	epID string, isDelete bool) (err error) {
	// take a lock to ensure we are programming one event at a time.
	// Also network create events need to be processed before endpoint creates
	// and reverse shall happen for deletes. That order is ensured by netmaster,
	// so we don't need to worry about that here
	netPlugin.Lock()
	defer func() { netPlugin.Unlock() }()

	homingHost := ""
	vtepIP := ""

	if !isDelete {
		epCfg := &drivers.OvsCfgEndpointState{}
		epCfg.StateDriver = netPlugin.StateDriver
		err = epCfg.Read(epID)
		if err != nil {
			log.Errorf("Failed to read config for ep '%s' \n", epID)
			return
		}
		homingHost = epCfg.HomingHost
		vtepIP = epCfg.VtepIP
	} else {
		epOper := &drivers.OvsOperEndpointState{}
		epOper.StateDriver = netPlugin.StateDriver
		err = epOper.Read(epID)
		if err != nil {
			log.Errorf("Failed to read oper for ep %s, err '%s' \n", epID, err)
			return
		}
		homingHost = epOper.HomingHost
		vtepIP = epOper.VtepIP
	}
	if skipHost(vtepIP, homingHost, opts.hostLabel) {
		log.Infof("skipping mismatching host for ep %s. EP's host %s (my host: %s)",
			epID, homingHost, opts.hostLabel)
		return
	}

	// read the context before to be compared with what changed after
	contEpContext, err := getEndpointContainerContext(
		netPlugin.StateDriver, epID)
	if err != nil {
		log.Errorf("Failed to obtain the container context for ep '%s' \n",
			epID)
		return
	}

	log.Debugf("read endpoint context: %s \n", contEpContext)

	operStr := ""
	if isDelete {
		err = netPlugin.DeleteEndpoint(epID)
		operStr = "delete"
	} else {
		err = netPlugin.CreateEndpoint(epID)
		operStr = "create"
	}
	if err != nil {
		log.Errorf("Endpoint operation %s failed. Error: %s",
			operStr, err)
		return
	}

	log.Infof("Endpoint operation %s succeeded", operStr)

	// attach or detach an endpoint to a container
	if isDelete || contAttachPointDeleted(contEpContext) {
		err = crt.ContainerIf.DetachEndpoint(contEpContext)
		if err != nil {
			log.Errorf("Endpoint detach container '%s' from ep '%s' failed . "+
				"Error: %s", contEpContext.CurrContName, epID, err)
		} else {
			log.Infof("Endpoint detach container '%s' from ep '%s' succeeded",
				contEpContext.CurrContName, epID)
		}
	}
	if !isDelete && contAttachPointAdded(contEpContext) {
		// re-read post ep updated state
		newContEpContext, err1 := getEndpointContainerContext(
			netPlugin.StateDriver, epID)
		if err1 != nil {
			log.Errorf("Failed to obtain the container context for ep '%s' \n", epID)
			return
		}
		contEpContext.InterfaceID = newContEpContext.InterfaceID
		contEpContext.IPAddress = newContEpContext.IPAddress
		contEpContext.SubnetLen = newContEpContext.SubnetLen

		err = crt.ContainerIf.AttachEndpoint(contEpContext)
		if err != nil {
			log.Errorf("Endpoint attach container '%s' to ep '%s' failed . "+
				"Error: %s", contEpContext.NewContName, epID, err)
		} else {
			log.Infof("Endpoint attach container '%s' to ep '%s' succeeded",
				contEpContext.NewContName, epID)
		}
		contID := crt.ContainerIf.GetContainerID(contEpContext.NewContName)
		if contID != "" {
		}
	}

	return
}

func attachContainer(stateDriver core.StateDriver, crt *crt.CRT, contName string) error {

	epContexts, err := getContainerEPContextByContName(stateDriver, contName)
	if err != nil {
		log.Errorf("Error '%s' getting Ep context for container %s \n",
			err, contName)
		return err
	}

	for _, epCtx := range epContexts {
		if epCtx.NewAttachUUID != "" || epCtx.InterfaceID == "" {
			log.Debugf("## skipping attach on epctx %v \n", epCtx)
			continue
		} else {
			log.Debugf("## trying attach on epctx %v \n", epCtx)
		}
		err = crt.AttachEndpoint(&epCtx)
		if err != nil {
			log.Errorf("Error '%s' attaching container to the network \n", err)
			return err
		}
	}

	return nil
}

func handleContainerStart(netPlugin *plugin.NetPlugin, crt *crt.CRT,
	contID string) error {
	// var epContexts []crtclient.ContainerEPContext

	contName, err := crt.GetContainerName(contID)
	if err != nil {
		log.Errorf("Could not find container name from container id %s \n", contID)
		return err
	}

	err = attachContainer(netPlugin.StateDriver, crt, contName)
	if err != nil {
		log.Errorf("error attaching container: %v\n", err)
	}

	return err
}

func handleContainerStop(netPlugin *plugin.NetPlugin, crt *crt.CRT,
	contID string) error {
	// If CONTIV_DIND_HOST_GOPATH env variable is set we can assume we are in docker in docker testbed
	// Here we need to set the network namespace of the ports created by netplugin back to NS of the docker host
	hostGoPath := os.Getenv("CONTIV_DIND_HOST_GOPATH")
	if hostGoPath != "" {
		osCmd := exec.Command("github.com/contiv/netplugin/scripts/dockerhost/setlocalns.sh")
		osCmd.Dir = os.Getenv("GOSRC")
		output, err := osCmd.Output()
		if err != nil {
			log.Errorf("setlocalns failed. Error: %s Output: \n%s\n",
				err, output)
			return err
		}
		return err
	}
	return nil
}

func handleDockerEvents(event *dockerclient.Event, retErr chan error,
	args ...interface{}) {
	var err error

	netPlugin, ok := args[0].(*plugin.NetPlugin)
	if !ok {
		log.Errorf("error decoding netplugin in handleDocker \n")
	}

	crt, ok := args[1].(*crt.CRT)
	if !ok {
		log.Errorf("error decoding netplugin in handleDocker \n")
	}

	log.Infof("Received event: %#v, for netPlugin %v \n", *event, netPlugin)

	// XXX: with plugin (in a lib) this code will handle these events
	// this cod will need to go away then
	switch event.Status {
	case "start":
		err = handleContainerStart(netPlugin, crt, event.Id)
		if err != nil {
			log.Errorf("error '%s' handling container %s \n", err, event.Id)
		}

	case "die":
		log.Debugf("received die event for container \n")
		err = handleContainerStop(netPlugin, crt, event.Id)
		if err != nil {
			log.Errorf("error '%s' handling container %s \n", err, event.Id)
		}
		// decide if we should remove the container network policy or leave
		// it until ep configuration is removed
		// ep configuration as instantiated can be applied to another container
		// or reincarnation of the same container

	}

	if err != nil {
		retErr <- err
	}
}

func processStateEvent(netPlugin *plugin.NetPlugin, crt *crt.CRT, opts cliOpts,
	rsps chan core.WatchState) {
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
			// XXX: late host binding modifies the ep-cfg state to update the host-label.
			// Need to treat it as Create, revisit to see if we can prevent this
			// by just triggering create once instead.
			log.Debugf("Received a modify event, treating it as a 'create'")
		}

		if nwCfg, ok := currentState.(*drivers.OvsCfgNetworkState); ok {
			log.Infof("Received %q for network: %q", eventStr, nwCfg.ID)
			processNetEvent(netPlugin, nwCfg.ID, isDelete)
		}
		if epCfg, ok := currentState.(*drivers.OvsCfgEndpointState); ok {
			log.Infof("Received %q for endpoint: %q", eventStr, epCfg.ID)
			processEpEvent(netPlugin, crt, opts, epCfg.ID, isDelete)
		}
		if peerInfo, ok := currentState.(*drivers.PeerHostState); ok {
			log.Infof("Received %q for peer host: %q", eventStr, peerInfo.ID)
			processPeerEvent(netPlugin, opts, peerInfo.ID, isDelete)
		}
	}
}

func handleNetworkEvents(netPlugin *plugin.NetPlugin, crt *crt.CRT,
	opts cliOpts, retErr chan error) {
	rsps := make(chan core.WatchState)
	go processStateEvent(netPlugin, crt, opts, rsps)
	cfg := drivers.OvsCfgNetworkState{}
	cfg.StateDriver = netPlugin.StateDriver
	retErr <- cfg.WatchAll(rsps)
	return
}

func handleEndpointEvents(netPlugin *plugin.NetPlugin, crt *crt.CRT,
	opts cliOpts, retErr chan error) {
	rsps := make(chan core.WatchState)
	go processStateEvent(netPlugin, crt, opts, rsps)
	cfg := drivers.OvsCfgEndpointState{}
	cfg.StateDriver = netPlugin.StateDriver
	retErr <- cfg.WatchAll(rsps)
	return
}

func handlePeerEvents(netPlugin *plugin.NetPlugin, crt *crt.CRT,
	opts cliOpts, retErr chan error) {
	rsps := make(chan core.WatchState)
	go processStateEvent(netPlugin, crt, opts, rsps)
	peer := drivers.PeerHostState{}
	peer.StateDriver = netPlugin.StateDriver
	retErr <- peer.WatchAll(rsps)
	return
}

func handleStateEvents(netPlugin *plugin.NetPlugin, crt *crt.CRT, opts cliOpts,
	retErr chan error) {
	// monitor network events
	go handleNetworkEvents(netPlugin, crt, opts, retErr)

	// monitor endpoint events
	go handleEndpointEvents(netPlugin, crt, opts, retErr)

	// monitor peer host events
	go handlePeerEvents(netPlugin, crt, opts, retErr)
}

func handleEvents(netPlugin *plugin.NetPlugin, crt *crt.CRT, opts cliOpts) error {
	recvErr := make(chan error, 1)
	recvEventErr := make(chan error)

	//monitor and process state change events
	go handleStateEvents(netPlugin, crt, opts, recvErr)
	startDockerEventPoll(netPlugin, crt, recvEventErr, opts)

	go func() {
		for {
			err := <-recvEventErr
			if err != nil {
				log.Warnf("Failure occured talking to docker events. Sleeping for a second and retrying. Error: %s", err)
			}

			time.Sleep(1 * time.Second)
			// FIXME note that we are assuming this goroutine crashed, which if not
			// might generate double events. Synchronize this with a channel.
			startDockerEventPoll(netPlugin, crt, recvEventErr, opts)
		}
	}()

	err := <-recvErr
	if err != nil {
		log.Errorf("Failure occured. Error: %s", err)
		return err
	}

	return nil
}

func startDockerEventPoll(netPlugin *plugin.NetPlugin, crt *crt.CRT, recvErr chan error, opts cliOpts) {
	if !opts.nativeInteg {
		// start docker client and handle docker events
		// wait on error chan for problems handling the docker events
		dockerCRT := crt.ContainerIf.(*docker.Docker)
		dockerCRT.Client.StartMonitorEvents(handleDockerEvents, recvErr, netPlugin, crt)
	}
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
	// default to using eth1's IP addr
	defVtepIp, _ := netutils.GetInterfaceIP("eth1")
	defVlanIntf := "eth2"

	flagSet = flag.NewFlagSet("netd", flag.ExitOnError)
	flagSet.StringVar(&opts.syslog,
		"syslog",
		"",
		"Log to syslog at proto://ip:port -- use 'kernel' to log via kernel syslog")
	flagSet.BoolVar(&opts.debug,
		"debug",
		false,
		"Show debugging information generated by netplugin")
	flagSet.BoolVar(&opts.jsonLog,
		"json-log",
		false,
		"Format logs as JSON")
	flagSet.StringVar(&opts.hostLabel,
		"host-label",
		defHostLabel,
		"label used to identify endpoints homed for this host, default is host name. If -config flag is used then host-label must be specified in the the configuration passed.")
	flagSet.BoolVar(&opts.nativeInteg,
		"native-integration",
		false,
		"do not listen to container runtime events, because the events are natively integrated into their call sequence and external integration is not required")
	flagSet.StringVar(&opts.cfgFile,
		"config",
		"",
		"plugin configuration. Use '-' to read configuration from stdin")
	flagSet.StringVar(&opts.vtepIP,
		"vtep-ip",
		defVtepIp,
		"My VTEP ip address")
	flagSet.StringVar(&opts.vlanIntf,
		"vlan-if",
		defVlanIntf,
		"My VTEP ip address")

	err = flagSet.Parse(os.Args[1:])
	if err != nil {
		log.Fatalf("Failed to parse command. Error: %s", err)
	}

	if opts.debug {
		log.SetLevel(log.DebugLevel)
		os.Setenv("CONTIV_TRACE", "1")
	}

	if opts.jsonLog {
		log.SetFormatter(&log.JSONFormatter{})
	}

	if opts.syslog != "" {
		configureSyslog(opts.syslog)
	}

	if flagSet.NFlag() < 1 {
		log.Infof("host-label not specified, using default (%s)", opts.hostLabel)
	}

	defConfigStr := fmt.Sprintf(`{
                    "drivers" : {
                       "network": %q,
                       "state": "etcd"
                    },
                    "plugin-instance": {
                       "host-label": %q,
						"vtep-ip": %q,
						"vlan-if": %q
                    },
                    %q : {
                       "dbip": "127.0.0.1",
                       "dbport": 6640
                    },
                    "etcd" : {
                        "machines": ["http://127.0.0.1:4001"]
                    },
                    "crt" : {
                       "type": "docker"
                    },
                    "docker" : {
                        "socket" : "unix:///var/run/docker.sock"
                    }
                  }`, utils.OvsNameStr, opts.hostLabel, opts.vtepIP,
					opts.vlanIntf, utils.OvsNameStr)

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

	// extract host-label from the configuration
	tmpInstInfo := &struct {
		Instance core.InstanceInfo `json:"plugin-instance"`
	}{}
	err = json.Unmarshal(config, tmpInstInfo)
	if err != nil {
		log.Fatalf("Failed to parse configuration. Error: %s", err)
	}
	if tmpInstInfo.Instance.HostLabel == "" {
		log.Fatalf("Empty host-label passed in configuration")
	}
	opts.hostLabel = tmpInstInfo.Instance.HostLabel

	err = netPlugin.Init(string(config))
	if err != nil {
		log.Fatalf("Failed to initialize the plugin. Error: %s", err)
	}

	crt := &crt.CRT{}
	err = crt.Init(string(config))
	if err != nil {
		log.Fatalf("Failed to initialize container run time, err %s \n", err)
	}

	// Process all current state
	processCurrentState(netPlugin, crt, opts)

	//logger := log.New(os.Stdout, "go-etcd: ", log.LstdFlags)
	//etcd.SetLogger(logger)

	if err := handleEvents(netPlugin, crt, opts); err != nil {
		os.Exit(1)
	}
}
