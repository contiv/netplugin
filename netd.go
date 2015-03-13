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
	"github.com/contiv/go-etcd/etcd"
	"github.com/samalba/dockerclient"
	"log"
	"os"
	"strings"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/crt"
	"github.com/contiv/netplugin/crtclient"
	"github.com/contiv/netplugin/crtclient/docker"
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/gstate"
	"github.com/contiv/netplugin/netutils"
	"github.com/contiv/netplugin/plugin"
)

// a daemon based on etcd client's Watch interface to trigger plugin's
// network provisioning interfaces

const (
	RECURSIVE = true
)

type cliOpts struct {
	hostLabel   string
	publishVtep bool
}

func getVtepName(netId, hostLabel string) string {
	return netId + "-" + hostLabel
}

func createDeleteVtep(netPlugin *plugin.NetPlugin, netId, preValue string,
	opts cliOpts) error {
	var err error

	cfgNet := &drivers.OvsCfgNetworkState{StateDriver: netPlugin.StateDriver}
	err = cfgNet.Read(netId)
	if err != nil {
		return err
	}

	gOper := &gstate.Oper{}
	err = gOper.Read(netPlugin.StateDriver, cfgNet.Tenant)
	if err != nil {
		return err
	}
	if gOper.DefaultNetType != "vxlan" {
		return nil
	}

	epCfg := &drivers.OvsCfgEndpointState{StateDriver: netPlugin.StateDriver}
	epCfg.Id = getVtepName(netId, opts.hostLabel)
	if preValue != "" {
		err = epCfg.Clear()
		if err != nil {
			log.Printf("error '%s' deleting ep %s \n", err, epCfg.Id)
		}
	} else {
		err = epCfg.Read(epCfg.Id)
		if err != nil {
			epCfg.HomingHost = opts.hostLabel
			epCfg.VtepIp, err = netutils.GetLocalIp()
			if err != nil {
				log.Printf("error '%s' getting local IP \n", err)
			} else {
				epCfg.NetId = netId
				err = epCfg.Write()
				if err != nil {
					log.Printf("error '%s' adding epCfg %v \n", epCfg)
				}
			}
		}
	}

	return nil
}

func skipHost(vtepIp, homingHost, myHostLabel string) bool {
	return (vtepIp == "" && homingHost != myHostLabel ||
		vtepIp != "" && homingHost == myHostLabel)
}

func processCurrentState(netPlugin *plugin.NetPlugin, crt *crt.Crt,
	opts cliOpts) error {
	keys, err := netPlugin.StateDriver.ReadRecursive(gstate.CFG_GLOBAL_PREFIX)
	if err != nil {
		return err
	}
	for idx, key := range keys {
		log.Printf("read global key[%d] %s, populating state \n", idx, key)
		processGlobalEvent(netPlugin, key, "")
	}

	keys, err = netPlugin.StateDriver.ReadRecursive(drivers.NW_CFG_PATH_PREFIX)
	if err != nil {
		return err
	}
	for idx, key := range keys {
		log.Printf("read net key[%d] %s, populating state \n", idx, key)
		processNetEvent(netPlugin, key, "", opts)
	}

	keys, err = netPlugin.StateDriver.ReadRecursive(drivers.EP_CFG_PATH_PREFIX)
	if err != nil {
		return err
	}
	for idx, key := range keys {
		log.Printf("read ep key[%d] %s, populating state \n", idx, key)
		processEpEvent(netPlugin, crt, key, "", opts)
	}

	return nil
}

func processGlobalEvent(netPlugin *plugin.NetPlugin, key, preValue string) (err error) {
	var gOper *gstate.Oper

	tenant := strings.TrimPrefix(key, gstate.CFG_GLOBAL_PREFIX)
	if preValue != "" {
		gOper := &gstate.Oper{}
		err = gOper.Read(netPlugin.StateDriver, tenant)
		if err != nil {
			// already deleted
			log.Printf("Tenant '%s' already deleted \n", tenant)
			err = nil
		} else {
			err = gOper.Clear(netPlugin.StateDriver)
		}

		return
	}

	gOper = &gstate.Oper{}
	err = gOper.Read(netPlugin.StateDriver, tenant)
	if err == nil {
		// already created
		return
	}

	gCfg := &gstate.Cfg{}
	err = gCfg.Read(netPlugin.StateDriver, tenant)
	if err != nil {
		log.Printf("Error '%s' reading tenant %s \n", err, tenant)
		return
	}

	gOper, err = gCfg.Process()
	if err != nil {
		log.Printf("Error '%s' updating the config %v \n", err, gCfg)
		return
	}

	err = gOper.Update(netPlugin.StateDriver)
	if err != nil {
		log.Printf("error '%s' updating goper state %v \n", err, gOper)
	}

	return
}

func processNetEvent(netPlugin *plugin.NetPlugin, key, preValue string,
	opts cliOpts) (err error) {

	netId := strings.TrimPrefix(key, drivers.NW_CFG_PATH_PREFIX)

	operStr := ""
	if preValue != "" {
		err = netPlugin.DeleteNetwork(preValue)
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
	if opts.publishVtep {
		createDeleteVtep(netPlugin, netId, preValue, opts)
	}

	return
}

func getEndpointContainerContext(state *core.StateDriver, epId string) (
	*crtclient.ContainerEpContext, error) {
	var epCtx crtclient.ContainerEpContext
	var err error

	epCfg := &drivers.OvsCfgEndpointState{StateDriver: *state}
	err = epCfg.Read(epId)
	if err != nil {
		return &epCtx, nil
	}
	epCtx.NewContName = epCfg.ContName

	cfgNet := &drivers.OvsCfgNetworkState{StateDriver: *state}
	err = cfgNet.Read(epCfg.NetId)
	if err != nil {
		return &epCtx, err
	}
	epCtx.DefaultGw = cfgNet.DefaultGw
	epCtx.SubnetLen = cfgNet.SubnetLen

	operEp := &drivers.OvsOperEndpointState{StateDriver: *state}
	err = operEp.Read(epId)
	if err != nil {
		return &epCtx, nil
	}
	epCtx.CurrContName = operEp.ContName
	epCtx.InterfaceId = operEp.PortName
	epCtx.IpAddress = operEp.IpAddress

	return &epCtx, err
}

func getContainerEpContextByContName(state *core.StateDriver, contName string) (
	epCtxs []crtclient.ContainerEpContext, err error) {
	var epCtx *crtclient.ContainerEpContext

	contName = strings.TrimPrefix(contName, "/")
	epCfgs, err := drivers.ReadAllEpsCfg(state)
	if err != nil {
		return
	}

	epCtxs = make([]crtclient.ContainerEpContext, len(epCfgs))
	idx := 0
	for _, epCfg := range epCfgs {
		if epCfg.ContName != contName {
			continue
		}

		epCtx, err = getEndpointContainerContext(state, epCfg.Id)
		if err != nil {
			log.Printf("error '%s' getting epCfgState for ep %s \n",
				err, epCfg.Id)
			return epCtxs[:idx], nil
		}
		epCtxs[idx] = *epCtx
		idx = idx + 1
	}

	return epCtxs[:idx], nil
}

func processEpEvent(netPlugin *plugin.NetPlugin, crt *crt.Crt,
	key string, preValue string, opts cliOpts) (err error) {
	epId := strings.TrimPrefix(key, drivers.EP_CFG_PATH_PREFIX)

	homingHost := ""
	vtepIp := ""
	epCfg := &drivers.OvsCfgEndpointState{StateDriver: netPlugin.StateDriver}
	if preValue == "" {
		err = epCfg.Read(epId)
		if err != nil {
			log.Printf("Failed to read config for ep '%s' \n", epId)
			return
		}
		homingHost = epCfg.HomingHost
		vtepIp = epCfg.VtepIp
	} else {
		err = epCfg.Unmarshal(preValue)
		if err != nil {
			log.Printf("Failed to unmarshal epcfg, err '%s' \n", err)
			return
		}
		homingHost = epCfg.HomingHost
		vtepIp = epCfg.VtepIp
	}
	if skipHost(vtepIp, homingHost, opts.hostLabel) {
		log.Printf("skipping mismatching host for ep %s. EP's host %s (my host: %s)",
			epId, homingHost, opts.hostLabel)
		return
	}

	// read the context before to be compared with what changed after
	contEpContext, err := getEndpointContainerContext(
		&netPlugin.StateDriver, epId)
	if err != nil {
		log.Printf("Failed to obtain the container context for ep '%s' \n",
			epId)
		return
	}
	// log.Printf("read endpoint context: %s \n", contEpContext)

	operStr := ""
	if preValue != "" {
		err = netPlugin.DeleteEndpoint(preValue)
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
	if preValue != "" ||
		(contEpContext.NewContName == "" && contEpContext.CurrContName != "") {
		err = crt.ContainerIf.DetachEndpoint(contEpContext)
		if err != nil {
			log.Printf("Endpoint detach container '%s' from ep '%s' failed . "+
				"Error: %s", contEpContext.CurrContName, epId, err)
		} else {
			log.Printf("Endpoint detach container '%s' from ep '%s' succeeded",
				contEpContext.CurrContName, epId)
		}
	}
	if preValue == "" && contEpContext.NewContName != "" {
		// re-read post ep updated state
		newContEpContext, err1 := getEndpointContainerContext(
			&netPlugin.StateDriver, epId)
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

func handleEtcdEvents(netPlugin *plugin.NetPlugin, crt *crt.Crt,
	rsps chan *etcd.Response, stop chan bool, retErr chan error,
	opts cliOpts) {
	for {
		// block on change notifications
		rsp := <-rsps

		node := rsp.Node
		preValue := ""
		if rsp.Node.Value == "" {
			preValue = rsp.PrevNode.Value
		}

		log.Printf("Received event for key: %s", node.Key)
		switch key := node.Key; {
		case strings.HasPrefix(key, gstate.CFG_GLOBAL_PREFIX):
			processGlobalEvent(netPlugin, key, preValue)

		case strings.HasPrefix(key, drivers.NW_CFG_PATH_PREFIX):
			processNetEvent(netPlugin, key, preValue, opts)

		case strings.HasPrefix(key, drivers.EP_CFG_PATH_PREFIX):
			processEpEvent(netPlugin, crt, key, preValue, opts)
		}
	}

	// shall never come here
	retErr <- nil
}

func handleContainerStart(netPlugin *plugin.NetPlugin, crt *crt.Crt,
	contId string) error {
	var err error
	var epContexts []crtclient.ContainerEpContext

	contName, err := crt.GetContainerName(contId)
	if err != nil {
		log.Printf("Could not find container name from container id %s \n", contId)
		return err
	}

	epContexts, err = getContainerEpContextByContName(&netPlugin.StateDriver,
		contName)
	if err != nil {
		log.Printf("Error '%s' getting Ep context for container %s \n",
			err, contName)
		return err
	}

	for _, epCtx := range epContexts {
		log.Printf("## trying attach on epctx %v \n", epCtx)
		err = crt.AttachEndpoint(&epCtx)
		if err != nil {
			log.Printf("Error '%s' attaching container to the network \n", err)
			return err
		}
	}

	return nil
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

func handleEvents(netPlugin *plugin.NetPlugin, crt *crt.Crt,
	opts cliOpts) error {

	// watch the etcd changes and call the respective plugin APIs
	rsps := make(chan *etcd.Response)
	recvErr := make(chan error, 1)
	stop := make(chan bool, 1)
	etcdDriver := netPlugin.StateDriver.(*drivers.EtcdStateDriver)
	etcdClient := etcdDriver.Client

	go handleEtcdEvents(netPlugin, crt, rsps, stop, recvErr, opts)

	// start docker client and handle docker events
	// wait on error chan for problems handling the docker events
	dockerCrt := crt.ContainerIf.(*docker.Docker)
	dockerCrt.Client.StartMonitorEvents(handleDockerEvents, recvErr,
		netPlugin, crt)

	// XXX: todo, restore any config that might have been created till this
	// point
	_, err := etcdClient.Watch(drivers.CFG_PATH, 0, RECURSIVE, rsps, stop)
	if err != nil && err != etcd.ErrWatchStoppedByUser {
		log.Printf("etcd watch failed. Error: %s", err)
		return err
	}

	err = <-recvErr
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
	flagSet.BoolVar(&opts.publishVtep,
		"publish-vtep",
		false,
		"publish the vtep when allowed by global policy")

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
