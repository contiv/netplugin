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

package agent

import (
	"io"
	"runtime"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/mgmtfn/dockplugin"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/netplugin/cluster"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/dockerversion"
	"golang.org/x/net/context"
)

func (ag *Agent) lbServiceExist() bool {
	lbCfg := &mastercfg.CfgServiceLBState{}
	plugin := ag.Plugin()
	lbCfg.StateDriver = plugin.StateDriver

	if lbList, err := lbCfg.ReadAll(); err == nil {
		return len(lbList) > 0
	}
	return false
}

// Handles docker events monitored by dockerclient. Currently we only handle
// container start and die event
func (ag *Agent) handleDockerEvents(events <-chan events.Message, errs <-chan error) {

	for {
		select {
		case err := <-errs:
			if err != nil && err == io.EOF {
				log.Errorf("Closing the events channel. Err: %+v", err)
				return
			}
			if err != nil && err != io.EOF {
				log.Errorf("Received error from docker events. Err: %+v", err)
			}
		case event := <-events:
			log.Debugf("Received Docker event: {%#v}\n", event)
			// process events only when LB services exist.
			if !ag.lbServiceExist() {
				continue
			}
			switch event.Status {
			case "start":
				endpointUpdReq := &master.UpdateEndpointRequest{}
				defaultHeaders := map[string]string{"User-Agent": "Docker-Client/" + dockerversion.Version + " (" + runtime.GOOS + ")"}
				cli, err := dockerclient.NewClient("unix:///var/run/docker.sock", "v1.21", nil, defaultHeaders)
				if err != nil {
					panic(err)
				}

				containerInfo, err := cli.ContainerInspect(context.Background(), event.ID)

				if err != nil {
					log.Errorf("Container Inspect failed :%s", err)
					break
				}

				if event.ID != "" {
					labelMap := getLabelsFromContainerInspect(&containerInfo)
					containerTenant := getTenantFromContainerInspect(&containerInfo)
					networkName, ipAddress, err := getEpNetworkInfoFromContainerInspect(&containerInfo)
					if err != nil {
						log.Errorf("Error getting container network info for %v.Err:%s", event.ID, err)
					}
					endpoint := getEndpointFromContainerInspect(&containerInfo)

					if ipAddress != "" {
						//Create provider info
						endpointUpdReq.IPAddress = ipAddress
						endpointUpdReq.ContainerID = event.ID
						endpointUpdReq.Tenant = containerTenant
						endpointUpdReq.Network = networkName
						endpointUpdReq.Event = "start"
						endpointUpdReq.EndpointID = endpoint
						endpointUpdReq.EPCommonName = containerInfo.Name
						endpointUpdReq.Labels = make(map[string]string)

						for k, v := range labelMap {
							endpointUpdReq.Labels[k] = v
						}
					}

					var epUpdResp master.UpdateEndpointResponse

					log.Infof("Sending Endpoint update request to master: {%+v}", endpointUpdReq)

					err = cluster.MasterPostReq("/plugin/updateEndpoint", endpointUpdReq, &epUpdResp)
					if err != nil {
						log.Errorf("Event: 'start' , Http error posting endpoint update, Error:%s", err)
					}
				} else {
					log.Errorf("Unable to fetch container labels for container %s ", event.ID)
				}
			case "die":
				endpointUpdReq := &master.UpdateEndpointRequest{}
				endpointUpdReq.ContainerID = event.ID
				endpointUpdReq.Event = "die"
				var epUpdResp master.UpdateEndpointResponse
				log.Infof("Sending Endpoint update request to master: {%+v} on container delete", endpointUpdReq)
				err := cluster.MasterPostReq("/plugin/updateEndpoint", endpointUpdReq, &epUpdResp)
				if err != nil {
					log.Errorf("Event:'die' Http error posting endpoint update, Error:%s", err)
				}
			}
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
func getEpNetworkInfoFromContainerInspect(containerInfo *types.ContainerJSON) (string, string, error) {
	var networkName string
	var IPAddress string
	var networkUUID string
	if containerInfo != nil && containerInfo.NetworkSettings != nil {
		for _, endpoint := range containerInfo.NetworkSettings.Networks {
			IPAddress = endpoint.IPAddress
			networkUUID = endpoint.NetworkID
			_, network, serviceName, err := dockplugin.GetDockerNetworkName(networkUUID)
			if err != nil {
				log.Errorf("Error getting docker networkname for network uuid : %s", networkUUID)
				return "", "", err
			}
			if serviceName != "" {
				networkName = serviceName
			} else {
				networkName = network
			}
		}
	}
	return networkName, IPAddress, nil
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

func getEndpointFromContainerInspect(containerInfo *types.ContainerJSON) string {

	endpointID := ""
	if containerInfo != nil && containerInfo.NetworkSettings != nil {
		for _, endpoint := range containerInfo.NetworkSettings.Networks {
			endpointID = endpoint.EndpointID
		}
	}
	return endpointID

}
