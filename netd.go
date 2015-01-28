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
	"github.com/coreos/go-etcd/etcd"
    "github.com/samalba/dockerclient"
	"log"
	"os"
	"strings"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/plugin"
	"github.com/contiv/netplugin/gstate"
)

// a daemon based on etcd client's Watch interface to trigger plugin's
// network provisioning interfaces

const (
	RECURSIVE = true
)

func handleEtcdEvents(netPlugin *plugin.NetPlugin, rsps chan *etcd.Response,
	stop chan bool, retErr chan error) {
	for {
		// block on change notifications
		rsp := <-rsps

		// determine if the Node is interesting
		// XXX: how does etcd notifies deletes.
		isDelete := false
		operStr := ""
		node := rsp.Node
		if rsp.Node.Value == "" {
			isDelete = true
		}
		var err error = nil
		log.Printf("Received event for key: %s", node.Key)
		switch key := node.Key; {
		case strings.HasPrefix(key, gstate.CFG_GLOBAL):
            var gOper *gstate.Oper

            gCfg := &gstate.Cfg{}
            gCfg.Read(netPlugin.StateDriver)
            gOper, err = gCfg.Process()
            if err != nil {
                log.Printf("Error '%s' updating the config %v \n", err, gCfg)
            } else {
                err = gOper.Update(netPlugin.StateDriver)
                if err != nil {
                    log.Printf("error '%s' updating goper state %v \n", 
                        err, gOper)
                } else {
                    log.Printf("Successfully updated global oper state: %v \n",
                        gOper)
                }
            }

		case strings.HasPrefix(key, drivers.NW_CFG_PATH_PREFIX):
			netId := strings.TrimPrefix(key, drivers.NW_CFG_PATH_PREFIX)
			if isDelete {
				err = netPlugin.DeleteNetwork(netId)
				operStr = "delete"
			} else {
				err = netPlugin.CreateNetwork(netId)
				operStr = "create"
			}
			if err != nil {
				log.Printf("Network operation %s failed. Error: %s",
					operStr, err)
			} else {
				log.Printf("Network operation %s succeeded", operStr)
			}

		case strings.HasPrefix(key, drivers.EP_CFG_PATH_PREFIX):
			epId := strings.TrimPrefix(key, drivers.EP_CFG_PATH_PREFIX)

            // read the context before to be compared with what changed after
            contEpContext, err := netPlugin.GetEndpointContainerContext(epId)
            if err != nil {
                log.Printf("Failed to obtain the container context for ep '%s' \n", epId)
                continue
            }
            log.Printf("read endpoint context: %s \n", contEpContext)

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
                continue
			} 
            log.Printf("Endpoint operation %s succeeded", operStr)

            // attach or detach an endpoint to a container
            if isDelete || 
               (contEpContext.NewContName == "" && contEpContext.CurrContName != "") {
                err = netPlugin.DetachEndpoint(contEpContext)
                if err != nil {
                    log.Printf("Endpoint detach container '%s' from ep '%s' failed . " +
                               "Error: %s", contEpContext.CurrContName, epId, err)
                } else {
                    log.Printf("Endpoint detach container '%s' from ep '%s' succeeded",
                               contEpContext.CurrContName, epId)
                }
            } 
            if !isDelete && contEpContext.NewContName != "" {
                // re-read post ep updated state
                newContEpContext, err1 := netPlugin.GetEndpointContainerContext(epId)
                if err1 != nil {
                    log.Printf("Failed to obtain the container context for ep '%s' \n", epId)
                    continue
                }
                contEpContext.InterfaceId = newContEpContext.InterfaceId
                contEpContext.IpAddress = newContEpContext.IpAddress
                contEpContext.SubnetLen = newContEpContext.SubnetLen

                err = netPlugin.AttachEndpoint(contEpContext)
                if err != nil {
                    log.Printf("Endpoint attach container '%s' to ep '%s' failed . " +
                               "Error: %s", contEpContext.NewContName, epId, err)
                } else {
                    log.Printf("Endpoint attach container '%s' to ep '%s' succeeded",
                               contEpContext.NewContName, epId)
                }
                contId := netPlugin.GetContainerId(contEpContext.NewContName)
                if contId != "" {
                    err = netPlugin.UpdateContainerId(epId, contId)
                    if err != nil {
                        log.Printf("Cont id update err '%s' to ep '%s' - contid %s\n",
                            contEpContext.NewContName, epId, contId)
                    }
                } 
            }
		}
	}

	// shall never come here
	retErr <- nil
}

func handleContainerStart(netPlugin *plugin.NetPlugin, contId string) error {
    var err error
    var epContexts []core.ContainerEpContext

    contName, err := netPlugin.GetContainerName(contId)
    if err != nil {
        log.Printf("Could not find container name from container id %s \n", contId)
        return err
    }

    epContexts, err = netPlugin.GetContainerEpContextByContName(contName)
    if err != nil {
        log.Printf("Error '%s' getting Ep context for container %s \n",
                   err, contName)
        return err
    }

    for _, epCtx := range epContexts {
        log.Printf("## trying attach on epctx %v \n", epCtx)
        err = netPlugin.AttachEndpoint(&epCtx)
        if err != nil {
            log.Printf("Error '%s' attaching container to the network \n", err)
            return err
        }
    }

    return nil
}

func handleDockerEvents(event *dockerclient.Event, args ...interface{}) {
    var err error

    netPlugin, ok := args[0].(*plugin.NetPlugin)
    if !ok {
        log.Printf("error decoding netplugin in handleDocker \n");
    }

    retErr, ok := args[1].(chan error)
    if !ok {
        log.Printf("error decoding netplugin in handleDocker \n");
    }

    log.Printf("Received event: %#v, for netPlugin %v \n", *event, netPlugin)

    // XXX: with plugin (in a lib) this code will handle these events
    // this cod will need to go away then
    switch event.Status {
		case "start":
            err = handleContainerStart(netPlugin, event.Id)
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

func run(netPlugin *plugin.NetPlugin) error {
	// watch the etcd changes and call the respective plugin APIs
	rsps := make(chan *etcd.Response)
	recvErr := make(chan error, 1)
	stop := make(chan bool, 1)
	etcdDriver := netPlugin.StateDriver.(*drivers.EtcdStateDriver)
	etcdClient := etcdDriver.Client

	go handleEtcdEvents(netPlugin, rsps, stop, recvErr)

    // start docker client and handle docker events 
    // wait on error chan for problems handling the docker events
    dockerDriver := netPlugin.ContainerDriver.(*drivers.DockerDriver)
    dockerDriver.Client.StartMonitorEvents(handleDockerEvents, netPlugin, recvErr)

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
	configStr := `{
                    "drivers" : {
                       "network": "ovs",
                       "endpoint": "ovs",
                       "state": "etcd",
                       "container": "docker"
                    },
                    "ovs" : {
                       "dbip": "127.0.0.1",
                       "dbport": 6640
                    },
                    "etcd" : {
                        "machines": ["http://127.0.0.1:4001"]
                    },
                    "docker" : {
                        "socket" : "unix:///var/run/docker.sock"
                    }
                  }`
	netPlugin := &plugin.NetPlugin{}

	err := netPlugin.Init(configStr)
	if err != nil {
		log.Printf("Failed to initialize the plugin. Error: %s", err)
		os.Exit(1)
	}

	//logger := log.New(os.Stdout, "go-etcd: ", log.LstdFlags)
	//etcd.SetLogger(logger)

	err = run(netPlugin)
	if err != nil {
		os.Exit(1)
	} else {
		os.Exit(0)
	}
}
