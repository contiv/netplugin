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
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/mgmtfn/dockplugin"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/netplugin/cluster"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/samalba/dockerclient"
	"golang.org/x/net/context"
)

// Handles docker events monitored by dockerclient. Currently we only handle
// container start and die event*/
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
			if len(labelMap) == 0 {
				//Ignore container without labels
				return
			}
			containerTenant := getTenantFromContainerInspect(&containerInfo)
			network, ipAddress, err := getEpNetworkInfoFromContainerInspect(&containerInfo)
			if err != nil {
				log.Errorf("Error getting container network info for %v.Err:%s", event.ID, err)
			}
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

			var svcProvResp master.SvcProvUpdateResponse

			log.Infof("Sending Provider create request to master: {%+v}", providerUpdReq)

			err = cluster.MasterPostReq("/plugin/svcProviderUpdate", providerUpdReq, &svcProvResp)
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
