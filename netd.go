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
	"github.com/samalba/dockerclient"
	"log"
	"os"
	"strings"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/crt"
	"github.com/contiv/netplugin/crtclient"
	"github.com/contiv/netplugin/crtclient/docker"
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/plugin"
)

// a daemon based on etcd client's Watch interface to trigger plugin's
// network provisioning interfaces

const (
	RECURSIVE = true
)

type cliOpts struct {
	hostLabel   string
	nativeInteg bool
}

func skipHost(vtepIp, homingHost, myHostLabel string) bool {
	return (vtepIp == "" && homingHost != myHostLabel ||
		vtepIp != "" && homingHost == myHostLabel)
}

func processCurrentState(netPlugin *plugin.NetPlugin, crt *crt.Crt,
	opts cliOpts) error {
	readNet := &drivers.OvsCfgEndpointState{}
	readNet.StateDriver = netPlugin.StateDriver
	netCfgs, err := readNet.ReadAll()
	if err != nil {
		return err
	}
	for idx, netCfg := range netCfgs {
		net := netCfg.(*drivers.OvsCfgNetworkState)
		log.Printf("read net key[%d] %s, populating state \n", idx, net.Id)
		processNetEvent(netPlugin, net.Id, false)
	}

	readEp := &drivers.OvsCfgEndpointState{}
	readEp.StateDriver = netPlugin.StateDriver
	epCfgs, err := readEp.ReadAll()
	if err != nil {
		return err
	}
	for idx, epCfg := range epCfgs {
		ep := epCfg.(*drivers.OvsCfgEndpointState)
		log.Printf("read ep key[%d] %s, populating state \n", idx, ep.Id)
		processEpEvent(netPlugin, crt, opts, ep.Id, false)
	}

	return nil
}

func processNetEvent(netPlugin *plugin.NetPlugin, netId string,
	isDelete bool) (err error) {
	// take a lock to ensure we are programming one event at a time.
	// Also network create events need to be processed before endpoint creates
	// and reverse shall happen for deletes. That order is ensured by netmaster,
	// so we don't need to worry about that here
	netPlugin.Lock()
	defer func() { netPlugin.Unlock() }()

	operStr := ""
	if isDelete {
		err = netPlugin.DeleteNetwork(netId)
		operStr = "delete"
	} else {
		err = netPlugin.CreateNetwork(netId)
		operStr = "create"
	}
	if err != nil {
		log.Printf("Network operation %s failed. Error: %s", operStr, err)
	} else {
		log.Printf("Network operation %s succeeded", operStr)
	}

	return
}

func getEndpointContainerContext(state core.StateDriver, epId string) (
	*crtclient.ContainerEpContext, error) {
	var epCtx crtclient.ContainerEpContext
	var err error

	epCfg := &drivers.OvsCfgEndpointState{}
	epCfg.StateDriver = state
	err = epCfg.Read(epId)
	if err != nil {
		return &epCtx, nil
	}
	epCtx.NewContName = epCfg.ContName
	epCtx.NewAttachUUID = epCfg.AttachUUID

	cfgNet := &drivers.OvsCfgNetworkState{}
	cfgNet.StateDriver = state
	err = cfgNet.Read(epCfg.NetId)
	if err != nil {
		return &epCtx, err
	}
	epCtx.DefaultGw = cfgNet.DefaultGw
	epCtx.SubnetLen = cfgNet.SubnetLen

	operEp := &drivers.OvsOperEndpointState{}
	operEp.StateDriver = state
	err = operEp.Read(epId)
	if err != nil {
		return &epCtx, nil
	}
	epCtx.CurrContName = operEp.ContName
	epCtx.InterfaceId = operEp.PortName
	epCtx.IpAddress = operEp.IpAddress
	epCtx.CurrAttachUUID = operEp.AttachUUID

	return &epCtx, err
}

func getContainerEpContextByContName(state core.StateDriver, contName string) (
	epCtxs []crtclient.ContainerEpContext, err error) {
	var epCtx *crtclient.ContainerEpContext

	contName = strings.TrimPrefix(contName, "/")
	readEp := &drivers.OvsCfgEndpointState{}
	readEp.StateDriver = state
	epCfgs, err := readEp.ReadAll()
	if err != nil {
		return
	}

	epCtxs = make([]crtclient.ContainerEpContext, len(epCfgs))
	idx := 0
	for _, epCfg := range epCfgs {
		cfg := epCfg.(*drivers.OvsCfgEndpointState)
		if cfg.ContName != contName {
			continue
		}

		epCtx, err = getEndpointContainerContext(state, cfg.Id)
		if err != nil {
			log.Printf("error '%s' getting epCfgState for ep %s \n",
				err, cfg.Id)
			return epCtxs[:idx], nil
		}
		epCtxs[idx] = *epCtx
		idx = idx + 1
	}

	return epCtxs[:idx], nil
}

func contAttachPointAdded(epCtx *crtclient.ContainerEpContext) bool {
	if epCtx.CurrAttachUUID == "" && epCtx.NewAttachUUID != "" {
		return true
	}
	if epCtx.CurrContName == "" && epCtx.NewContName != "" {
		return true
	}
	return false
}

func contAttachPointDeleted(epCtx *crtclient.ContainerEpContext) bool {
	if epCtx.CurrAttachUUID != "" && epCtx.NewAttachUUID == "" {
		return true
	}
	if epCtx.CurrContName != "" && epCtx.NewContName == "" {
		return true
	}
	return false
}

func processEpEvent(netPlugin *plugin.NetPlugin, crt *crt.Crt, opts cliOpts,
	epId string, isDelete bool) (err error) {
	// take a lock to ensure we are programming one event at a time.
	// Also network create events need to be processed before endpoint creates
	// and reverse shall happen for deletes. That order is ensured by netmaster,
	// so we don't need to worry about that here
	netPlugin.Lock()
	defer func() { netPlugin.Unlock() }()

	homingHost := ""
	vtepIp := ""

	if !isDelete {
		epCfg := &drivers.OvsCfgEndpointState{}
		epCfg.StateDriver = netPlugin.StateDriver
		err = epCfg.Read(epId)
		if err != nil {
			log.Printf("Failed to read config for ep '%s' \n", epId)
			return
		}
		homingHost = epCfg.HomingHost
		vtepIp = epCfg.VtepIp
	} else {
		epOper := &drivers.OvsOperEndpointState{}
		epOper.StateDriver = netPlugin.StateDriver
		err = epOper.Read(epId)
		if err != nil {
			log.Printf("Failed to read oper for ep %s, err '%s' \n", epId, err)
			return
		}
		homingHost = epOper.HomingHost
		vtepIp = epOper.VtepIp
	}
	if skipHost(vtepIp, homingHost, opts.hostLabel) {
		log.Printf("skipping mismatching host for ep %s. EP's host %s (my host: %s)",
			epId, homingHost, opts.hostLabel)
		return
	}

	// read the context before to be compared with what changed after
	contEpContext, err := getEndpointContainerContext(
		netPlugin.StateDriver, epId)
	if err != nil {
		log.Printf("Failed to obtain the container context for ep '%s' \n",
			epId)
		return
	}
	// log.Printf("read endpoint context: %s \n", contEpContext)

	operStr := ""
	if isDelete {
		err = netPlugin.DeleteEndpoint(epId)
		operStr = "delete"
	} else {
		err = netPlugin.CreateEndpoint(epId)
		operStr = "create"
	}
	if err != nil {
		log.Printf("Endpoint operation %s failed. Error: %s",
			operStr, err)
		return
	}
	log.Printf("Endpoint operation %s succeeded", operStr)

	// attach or detach an endpoint to a container
	if isDelete || contAttachPointDeleted(contEpContext) {
		err = crt.ContainerIf.DetachEndpoint(contEpContext)
		if err != nil {
			log.Printf("Endpoint detach container '%s' from ep '%s' failed . "+
				"Error: %s", contEpContext.CurrContName, epId, err)
		} else {
			log.Printf("Endpoint detach container '%s' from ep '%s' succeeded",
				contEpContext.CurrContName, epId)
		}
	}
	if !isDelete && contAttachPointAdded(contEpContext) {
		// re-read post ep updated state
		newContEpContext, err1 := getEndpointContainerContext(
			netPlugin.StateDriver, epId)
		if err1 != nil {
			log.Printf("Failed to obtain the container context for ep '%s' \n", epId)
			return
		}
		contEpContext.InterfaceId = newContEpContext.InterfaceId
		contEpContext.IpAddress = newContEpContext.IpAddress
		contEpContext.SubnetLen = newContEpContext.SubnetLen

		err = crt.ContainerIf.AttachEndpoint(contEpContext)
		if err != nil {
			log.Printf("Endpoint attach container '%s' to ep '%s' failed . "+
				"Error: %s", contEpContext.NewContName, epId, err)
		} else {
			log.Printf("Endpoint attach container '%s' to ep '%s' succeeded",
				contEpContext.NewContName, epId)
		}
		contId := crt.ContainerIf.GetContainerId(contEpContext.NewContName)
		if contId != "" {
		}
	}

	return
}

func attachContainer(stateDriver core.StateDriver, crt *crt.Crt, contName string) error {

	epContexts, err := getContainerEpContextByContName(stateDriver, contName)
	if err != nil {
		log.Printf("Error '%s' getting Ep context for container %s \n",
			err, contName)
		return err
	}

	for _, epCtx := range epContexts {
		if epCtx.NewAttachUUID != "" || epCtx.InterfaceId == "" {
			log.Printf("## skipping attach on epctx %v \n", epCtx)
			continue
		} else {
			log.Printf("## trying attach on epctx %v \n", epCtx)
		}
		err = crt.AttachEndpoint(&epCtx)
		if err != nil {
			log.Printf("Error '%s' attaching container to the network \n", err)
			return err
		}
	}

	return nil
}

func handleContainerStart(netPlugin *plugin.NetPlugin, crt *crt.Crt,
	contId string) error {
	// var epContexts []crtclient.ContainerEpContext

	contName, err := crt.GetContainerName(contId)
	if err != nil {
		log.Printf("Could not find container name from container id %s \n", contId)
		return err
	}

	err = attachContainer(netPlugin.StateDriver, crt, contName)
	if err != nil {
		log.Printf("error attaching container err \n", err)
	}

	return err
}

func handleDockerEvents(event *dockerclient.Event, retErr chan error,
	args ...interface{}) {
	var err error

	netPlugin, ok := args[0].(*plugin.NetPlugin)
	if !ok {
		log.Printf("error decoding netplugin in handleDocker \n")
	}

	crt, ok := args[1].(*crt.Crt)
	if !ok {
		log.Printf("error decoding netplugin in handleDocker \n")
	}

	log.Printf("Received event: %#v, for netPlugin %v \n", *event, netPlugin)

	// XXX: with plugin (in a lib) this code will handle these events
	// this cod will need to go away then
	switch event.Status {
	case "start":
		err = handleContainerStart(netPlugin, crt, event.Id)
		if err != nil {
			log.Printf("error '%s' handling container %s \n", err, event.Id)
		}

	case "die":
		log.Printf("received die event for container \n")
		// decide if we should remove the container network policy or leave
		// it until ep configuration is removed
		// ep configuration as instantiated can be applied to another container
		// or reincarnation of the same container

	}

	if err != nil {
		retErr <- err
	}
}

func processStateEvent(netPlugin *plugin.NetPlugin, crt *crt.Crt, opts cliOpts,
	rsps chan core.WatchState, retErr chan error) {
	for {
		// block on change notifications
		rsp := <-rsps

		// For now we deal with only create and delete events
		state := rsp.Curr
		isDelete := false
		eventStr := "create"
		if rsp.Curr == nil {
			state = rsp.Prev
			isDelete = true
			eventStr = "delete"
		} else if rsp.Prev != nil {
			// XXX: late host binding modifies the ep-cfg state to update the host-label.
			// Need to treat it as Create, revisit to see if we can prevent this
			// by just triggering create once instead.
			log.Printf("Received a modify event, treating it as a 'create'")
		}

		if nwCfg, ok := state.(*drivers.OvsCfgNetworkState); ok {
			log.Printf("Received %q for network: %q", eventStr, nwCfg.Id)
			processNetEvent(netPlugin, nwCfg.Id, isDelete)
		}
		if epCfg, ok := state.(*drivers.OvsCfgEndpointState); ok {
			log.Printf("Received %q for endpoint: %q", eventStr, epCfg.Id)
			processEpEvent(netPlugin, crt, opts, epCfg.Id, isDelete)
		}
	}

	// shall never come here
	retErr <- nil
}

func handleNetworkEvents(netPlugin *plugin.NetPlugin, crt *crt.Crt,
	opts cliOpts, retErr chan error) {
	rsps := make(chan core.WatchState)
	go processStateEvent(netPlugin, crt, opts, rsps, retErr)
	cfg := drivers.OvsCfgNetworkState{}
	cfg.StateDriver = netPlugin.StateDriver
	retErr <- cfg.WatchAll(rsps)
	return
}

func handleEndpointEvents(netPlugin *plugin.NetPlugin, crt *crt.Crt,
	opts cliOpts, retErr chan error) {
	rsps := make(chan core.WatchState)
	go processStateEvent(netPlugin, crt, opts, rsps, retErr)
	cfg := drivers.OvsCfgEndpointState{}
	cfg.StateDriver = netPlugin.StateDriver
	retErr <- cfg.WatchAll(rsps)
	return
}

func handleStateEvents(netPlugin *plugin.NetPlugin, crt *crt.Crt, opts cliOpts,
	retErr chan error) {
	// monitor network events
	go handleNetworkEvents(netPlugin, crt, opts, retErr)

	// monitor endpoint events
	go handleEndpointEvents(netPlugin, crt, opts, retErr)
}

func handleEvents(netPlugin *plugin.NetPlugin, crt *crt.Crt, opts cliOpts) error {

	recvErr := make(chan error, 1)

	//monitor and process state change events
	go handleStateEvents(netPlugin, crt, opts, recvErr)

	if !opts.nativeInteg {
		// start docker client and handle docker events
		// wait on error chan for problems handling the docker events
		dockerCrt := crt.ContainerIf.(*docker.Docker)
		dockerCrt.Client.StartMonitorEvents(handleDockerEvents, recvErr,
			netPlugin, crt)
	}

	err := <-recvErr
	if err != nil {
		log.Printf("Failure occured. Error: %s", err)
		return err
	}

	return nil
}

func main() {
	var opts cliOpts
	var flagSet *flag.FlagSet

	defHostLabel, err := os.Hostname()
	if err != nil {
		log.Printf("Failed to fetch hostname. Error: %s", err)
		os.Exit(1)
	}

	flagSet = flag.NewFlagSet("netd", flag.ExitOnError)
	flagSet.StringVar(&opts.hostLabel,
		"host-label",
		defHostLabel,
		"label used to identify endpoints homed for this host, default is host name")
	flagSet.BoolVar(&opts.nativeInteg,
		"native-integration",
		false,
		"do not listen to container runtime events, because the events are natively integrated into their call sequence and external integration is not required")

	err = flagSet.Parse(os.Args[1:])
	if err != nil {
		log.Fatalf("Failed to parse command. Error: %s", err)
	}

	if flagSet.NFlag() < 1 {
		log.Printf("host-label not specified, using default (%s)", opts.hostLabel)
	}

	configStr := `{
                    "drivers" : {
                       "network": "ovs",
                       "endpoint": "ovs",
                       "state": "etcd"
                    },
                    "ovs" : {
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
                  }`
	netPlugin := &plugin.NetPlugin{}

	err = netPlugin.Init(configStr)
	if err != nil {
		log.Printf("Failed to initialize the plugin. Error: %s", err)
		os.Exit(1)
	}

	crt := &crt.Crt{}
	err = crt.Init(configStr)
	if err != nil {
		log.Printf("Failed to initialize container run time, err %s \n", err)
		os.Exit(1)
	}

	processCurrentState(netPlugin, crt, opts)

	//logger := log.New(os.Stdout, "go-etcd: ", log.LstdFlags)
	//etcd.SetLogger(logger)

	err = handleEvents(netPlugin, crt, opts)
	if err != nil {
		os.Exit(1)
	} else {
		os.Exit(0)
	}
}
