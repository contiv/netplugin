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

package dockplugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/contivmodel/client"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/docknet"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/netplugin/cluster"
	"github.com/contiv/netplugin/utils"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/drivers/remote/api"
	"golang.org/x/net/context"
)

const defaultTenantName = "default"

func getCapability(w http.ResponseWriter, r *http.Request) {
	logEvent("getCapability")

	content, err := json.Marshal(api.GetCapabilityResponse{Scope: "global"})
	if err != nil {
		httpError(w, "Could not generate getCapability response", err)
		return
	}

	w.Write(content)
}

func deleteNetwork(w http.ResponseWriter, r *http.Request) {
	var (
		content []byte
		err     error
		decoder = json.NewDecoder(r.Body)
		dnreq   = api.DeleteNetworkRequest{}
	)

	logEvent("delete network")

	err = decoder.Decode(&dnreq)
	if err != nil {
		httpError(w, "Could not read and parse the delete network request", err)
		return
	}
	dnresp := api.DeleteNetworkResponse{}
	content, err = json.Marshal(dnresp)
	if err != nil {
		httpError(w, "Could not generate delete network response", err)
		return
	}
	w.Write(content)
}

func createNetwork(w http.ResponseWriter, r *http.Request) {
	var (
		content []byte
		err     error
		decoder = json.NewDecoder(r.Body)
		cnreq   = api.CreateNetworkRequest{}
	)

	logEvent("create network")

	err = decoder.Decode(&cnreq)
	if err != nil {
		httpError(w, "Could not read and parse the create network request", err)
		return
	}

	log.Infof("CreateNetworkRequest: %+v", cnreq)

	cnresp := api.CreateNetworkResponse{}
	content, err = json.Marshal(cnresp)
	if err != nil {
		httpError(w, "Could not generate create network response", err)
		return
	}

	w.Write(content)
}

func deleteEndpoint(hostname string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			content []byte
			err     error
			decoder = json.NewDecoder(r.Body)
			dereq   = api.DeleteEndpointRequest{}
		)

		logEvent("delete endpoint")

		err = decoder.Decode(&dereq)
		if err != nil {
			httpError(w, "Could not read and parse the delete endpoint request", err)
			return
		}

		log.Infof("Received DeleteEndpointRequest: %+v", dereq)

		tenantName, netName, serviceName, err := GetDockerNetworkName(dereq.NetworkID)
		if err != nil {
			log.Errorf("Error getting network name for UUID: %s. Err: %v", dereq.NetworkID, err)
			httpError(w, "Could not get network name", err)
			return
		}

		// Build endpoint delete request
		delreq := master.DeleteEndpointRequest{
			TenantName:  tenantName,
			NetworkName: netName,
			ServiceName: serviceName,
			EndpointID:  dereq.EndpointID,
		}

		var delResp master.DeleteEndpointResponse
		err = cluster.MasterPostReq("/plugin/deleteEndpoint", &delreq, &delResp)
		if err != nil {
			httpError(w, "master failed to delete endpoint", err)
			return
		}

		netID := netName + "." + tenantName
		_, err = utils.GetEndpoint(netID + "-" + delreq.EndpointID)
		if err != nil {
			httpError(w, "Could not find endpoint", err)
			return
		}

		// delete the endpoint
		err = netPlugin.DeleteEndpoint(netID + "-" + delreq.EndpointID)
		if err != nil {
			log.Errorf("Error deleting endpoint %s. Err: %v", delreq.EndpointID, err)
			httpError(w, "failed to delete endpoint", err)
			return
		}

		// build response
		content, err = json.Marshal(api.DeleteEndpointResponse{})
		if err != nil {
			httpError(w, "Could not generate delete endpoint response", err)
			return
		}

		w.Write(content)
	}
}

func createEndpoint(hostname string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			content []byte
			err     error
			decoder = json.NewDecoder(r.Body)
			cereq   = api.CreateEndpointRequest{}
		)

		logEvent("create endpoint")

		err = decoder.Decode(&cereq)
		if err != nil {
			httpError(w, "Could not read and parse the create endpoint request", err)
			return
		}

		tenantName, netName, serviceName, err := GetDockerNetworkName(cereq.NetworkID)
		if err != nil {
			log.Errorf("Error getting network name for UUID: %s. Err: %v", cereq.NetworkID, err)
			httpError(w, "Could not get network name", err)
			return
		}

		// Build endpoint request
		mreq := master.CreateEndpointRequest{
			TenantName:  tenantName,
			NetworkName: netName,
			ServiceName: serviceName,
			EndpointID:  cereq.EndpointID,
			ConfigEP: intent.ConfigEP{
				Container:   cereq.EndpointID,
				Host:        hostname,
				IPAddress:   strings.Split(cereq.Interface.Address, "/")[0],
				IPv6Address: strings.Split(cereq.Interface.AddressIPv6, "/")[0],
				ServiceName: serviceName,
			},
		}

		log.Infof("CreateEndpointRequest: %+v. Interface: %+v", mreq, cereq.Interface)

		var mresp master.CreateEndpointResponse
		err = cluster.MasterPostReq("/plugin/createEndpoint", &mreq, &mresp)
		if err != nil {
			httpError(w, "master failed to create endpoint", err)
			return
		}

		log.Infof("Got endpoint create resp from master: %+v", mresp)
		netID := netName + "." + tenantName

		// Ask netplugin to create the endpoint
		err = netPlugin.CreateEndpoint(netID + "-" + cereq.EndpointID)
		if err != nil {
			log.Errorf("Endpoint creation failed. Error: %s", err)
			httpError(w, "Could not create endpoint", err)
			return
		}

		ep, err := utils.GetEndpoint(netID + "-" + cereq.EndpointID)
		if err != nil {
			httpError(w, "Could not find created endpoint", err)
			return
		}

		log.Debug(ep)

		epResponse := api.CreateEndpointResponse{
			Interface: &api.EndpointInterface{
				MacAddress: mresp.EndpointConfig.MacAddress,
			},
		}

		log.Infof("Sending CreateEndpointResponse: {%+v}, IP Addr: %v", epResponse, ep.IPAddress)

		content, err = json.Marshal(epResponse)
		if err != nil {
			httpError(w, "Could not generate create endpoint response", err)
			return
		}

		w.Write(content)
	}
}

func endpointInfo(w http.ResponseWriter, r *http.Request) {
	var (
		err     error
		decoder = json.NewDecoder(r.Body)
		epireq  = api.EndpointInfoRequest{}
	)

	logEvent("endpoint info")

	err = decoder.Decode(&epireq)
	if err != nil {
		httpError(w, "Could not read and parse the endpoint info request", err)
		return
	}

	log.Infof("EndpointInfoRequest: %+v", epireq)

	resp, err := json.Marshal(api.EndpointInfoResponse{})
	if err != nil {
		httpError(w, "Could not generate endpoint info response", err)
		return
	}

	w.Write(resp)
}

func join(w http.ResponseWriter, r *http.Request) {
	var (
		content []byte
		err     error
		decoder = json.NewDecoder(r.Body)
		jr      = api.JoinRequest{}
	)

	logEvent("join")

	err = decoder.Decode(&jr)
	if err != nil {
		httpError(w, "Could not read and parse the join request", err)
		return
	}

	log.Infof("JoinRequest: %+v", jr)

	tenantName, netName, _, err := GetDockerNetworkName(jr.NetworkID)
	if err != nil {
		log.Errorf("Error getting network name for UUID: %s. Err: %v", jr.NetworkID, err)
		httpError(w, "Could not get network name", err)
		return
	}

	netID := netName + "." + tenantName
	ep, err := utils.GetEndpoint(netID + "-" + jr.EndpointID)
	if err != nil {
		httpError(w, "Could not find created endpoint", err)
		return
	}

	nw, err := utils.GetNetwork(netID)
	if err != nil {
		httpError(w, "Could not get network", err)
		return
	}

	joinResp := api.JoinResponse{
		InterfaceName: &api.InterfaceName{
			SrcName:   ep.PortName,
			DstPrefix: "eth",
		},
		Gateway: nw.Gateway,
	}

	log.Infof("Sending JoinResponse: {%+v}, InterfaceName: %s", joinResp, ep.PortName)

	content, err = json.Marshal(joinResp)
	if err != nil {
		httpError(w, "Could not generate join response", err)
		return
	}

	w.Write(content)
}

func leave(w http.ResponseWriter, r *http.Request) {
	var (
		content []byte
		err     error
		decoder = json.NewDecoder(r.Body)
		lr      = api.LeaveRequest{}
	)

	logEvent("leave")

	err = decoder.Decode(&lr)
	if err != nil {
		httpError(w, "Could not read and parse the leave request", err)
		return
	}

	log.Infof("LeaveRequest: %+v", lr)

	// Send response
	leaveResp := api.LeaveResponse{}

	log.Infof("Sending LeaveResponse: {%+v}", leaveResp)

	content, err = json.Marshal(leaveResp)
	if err != nil {
		httpError(w, "Could not generate leave response", err)
		return
	}

	w.Write(content)
}

func allocateNetwork(w http.ResponseWriter, r *http.Request) {

	var (
		content []byte
		err     error
		decoder = json.NewDecoder(r.Body)
		anreq   = api.AllocateNetworkRequest{}
	)

	logEvent("allocateNetwork")

	err = decoder.Decode(&anreq)
	if err != nil {
		httpError(w, "Could not read and parse the allocateNetwork request", err)
		return
	}

	log.Infof("AllocateNetworkRequest: %+v", anreq)

	// If we are in swarm-mode:
	//     - if tag is given, it indicates the epg or network
	//     - else if subnet given create a contiv network
	// In swarm-mode libnetwork api for allocateNetwork is called
	// once in the cluster and createNetwork is called in every
	// node where a container in the network is instantiated.
	// We process only allocateNetwork in this case.
	//
	if pluginMode == core.SwarmMode {
		tag := ""
		log.Infof("Options: %+v", anreq.Options)
		if _, tagOk := anreq.Options["contiv-tag"]; tagOk {
			log.Infof("contiv-tag %+v ", anreq.Options["contiv-tag"])
			tag = anreq.Options["contiv-tag"]
		}
		err = createNetworkHelper(anreq.NetworkID, tag, anreq.IPv4Data, anreq.IPv6Data)
		if err != nil {
			httpError(w, "createNetwork failed! ", err)
		}
	} else {
		log.Infof("ClusterMode is %s", master.GetClusterMode())
	}

	resp := api.AllocateNetworkResponse{}
	resp.Options = anreq.Options
	content, err = json.Marshal(resp)
	if err != nil {
		httpError(w, "failed to marshal JSON for AllocateNetwork response", err)
		return
	}

	w.Write(content)
}

func freeNetwork(w http.ResponseWriter, r *http.Request) {
	var (
		content []byte
		err     error
		decoder = json.NewDecoder(r.Body)
	)

	logEvent("freeNetwork")

	req := api.FreeNetworkRequest{}
	err = decoder.Decode(&req)
	if err != nil {
		httpError(w, "Could not read and parse the freeNetwork request", err)
		return
	}

	if pluginMode == core.SwarmMode {
		err = deleteNetworkHelper(req.NetworkID)
		if err != nil {
			httpError(w, "Could not delete network", err)
			return
		}
	}

	resp := api.FreeNetworkResponse{}
	content, err = json.Marshal(resp)
	if err != nil {
		httpError(w, "failed to marshal JSON for freeNetwork response", err)
		return
	}

	w.Write(content)
}

func programExternalConnectivity(w http.ResponseWriter, r *http.Request) {
	resp := api.ProgramExternalConnectivityResponse{}

	logEvent("program externalConnectivity")

	content, err := json.Marshal(resp)
	if err != nil {
		httpError(w, "failed to marshal JSON for externalConnectivity request", err)
		return
	}

	w.Write(content)
}

func revokeExternalConnectivity(w http.ResponseWriter, r *http.Request) {
	resp := api.RevokeExternalConnectivityResponse{}

	logEvent("revoke externalConnectivity")

	content, err := json.Marshal(resp)
	if err != nil {
		httpError(w, "failed to marshal JSON for externalConnectivity response", err)
		return
	}

	w.Write(content)
}

func discoverNew(w http.ResponseWriter, r *http.Request) {
	resp := api.DiscoveryResponse{}

	logEvent("discoverNew")

	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httpError(w, "Could not read contents of discoverNew", err)
		return
	}
	log.Infof("DiscoverNew content: %s", string(content))

	content, err = json.Marshal(resp)
	if err != nil {
		httpError(w, "failed to marshall JSON for discoverNew", err)
		return
	}

	w.Write(content)
}

func discoverDelete(w http.ResponseWriter, r *http.Request) {
	resp := api.DiscoveryResponse{}

	logEvent("discoverDelete")

	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httpError(w, "Could not read contents of discoverDelete", err)
		return
	}
	log.Infof("DiscoverDelete content: %s", string(content))

	content, err = json.Marshal(resp)
	if err != nil {
		httpError(w, "failed to marshall JSON for discoverDelete", err)
		return
	}

	w.Write(content)
}

// GetDockerNetworkName gets network name from network UUID
func GetDockerNetworkName(nwID string) (string, string, string, error) {
	// first see if we can find the network in docknet oper state
	dnetOper, err := docknet.FindDocknetByUUID(nwID)
	if err == nil {
		return dnetOper.TenantName, dnetOper.NetworkName, dnetOper.ServiceName, nil
	}
	if pluginMode == core.SwarmMode {
		log.Errorf("Unable to find docknet info in objstore")
		return "", "", "", err
	}

	// create docker client
	docker, err := dockerclient.NewClient("unix:///var/run/docker.sock", "", nil, nil)
	if err != nil {
		log.Errorf("Unable to connect to docker. Error %v", err)
		return "", "", "", errors.New("unable to connect to docker")
	}

	nwIDFilter := filters.NewArgs()
	nwIDFilter.Add("id", nwID)
	nwList, err := docker.NetworkList(context.Background(), types.NetworkListOptions{Filters: nwIDFilter})
	if err != nil {
		log.Infof("Error: %v", err)
		return "", "", "", err
	}

	if len(nwList) != 1 {
		if len(nwList) == 0 {
			err = errors.New("network UUID not found")
		} else {
			err = errors.New("more than one network found with the same ID")
		}
		return "", "", "", err
	}
	nw := nwList[0]
	log.Infof("Returning network name %s for ID %s", nw.Name, nwID)

	// parse the network name
	var tenantName, netName, serviceName string
	names := strings.Split(nw.Name, "/")
	if len(names) == 2 {
		// has service.network/tenant format.
		tenantName = names[1]

		// parse service and network names
		sNames := strings.Split(names[0], ".")
		if len(sNames) == 2 {
			// has service.network format
			netName = sNames[1]
			serviceName = sNames[0]
		} else {
			netName = sNames[0]
		}
	} else if len(names) == 1 {
		// has service.network in default tenant
		tenantName = defaultTenantName

		// parse service and network names
		sNames := strings.Split(names[0], ".")
		if len(sNames) == 2 {
			// has service.network format
			netName = sNames[1]
			serviceName = sNames[0]
		} else {
			netName = sNames[0]
		}
	} else {
		log.Errorf("Invalid network name format for network %s", nw.Name)
		return "", "", "", errors.New("invalid format")
	}

	return tenantName, netName, serviceName, nil
}

// FindGroupFromTag finds the group that has matching tag
func FindGroupFromTag(epgTag string) (*mastercfg.EndpointGroupState, error) {
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return nil, err
	}

	epgCfg := &mastercfg.EndpointGroupState{}
	epgCfg.StateDriver = stateDriver

	epgList, err := epgCfg.ReadAll()
	if err != nil {
		return nil, err
	}

	var epg *mastercfg.EndpointGroupState
	found := false
	for _, epgP := range epgList {
		epg = epgP.(*mastercfg.EndpointGroupState)
		if epg.GroupTag == epgTag {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("Couldn't find group matching the tag %s", epgTag)
	}
	return epg, nil
}

// FindNetworkFromTag finds the network that has matching tag
func FindNetworkFromTag(nwTag string) (*mastercfg.CfgNetworkState, error) {
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return nil, err
	}
	nwCfg := &mastercfg.CfgNetworkState{}
	nwCfg.StateDriver = stateDriver
	nwList, err := nwCfg.ReadAll()
	if err != nil {
		return nil, err
	}
	var nw *mastercfg.CfgNetworkState
	found := false
	for _, nwP := range nwList {
		nw = nwP.(*mastercfg.CfgNetworkState)
		if nw.NetworkTag == nwTag {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("Couldn't find network matching the tag %s", nwTag)
	}
	return nw, nil
}

// createNetworkHelper creates the association between docker network and contiv network
//  if tag is given map docker net to epg or network, else create a contiv network
func createNetworkHelper(networkID string, tag string, IPv4Data, IPv6Data []driverapi.IPAMData) error {
	var tenantName, networkName, serviceName string
	var err error
	if tag != "" {
		// we need to map docker network to policy group or network using the tag
		log.Infof("Received tag %s", tag)
		var nw *mastercfg.CfgNetworkState
		epg, err := FindGroupFromTag(tag)
		if err != nil {
			nw, err = FindNetworkFromTag(tag)
			if err != nil {
				return errors.New("failed to lookup tag")
			}
		}
		if epg != nil {
			tenantName = epg.TenantName
			networkName = epg.NetworkName
			serviceName = epg.GroupName
		} else if nw != nil {
			tenantName = nw.Tenant
			networkName = nw.NetworkName
			serviceName = ""
		}
	} else if len(IPv4Data) > 0 {
		// if subnet is specified in docker command, we create a contiv network
		subnetPool := ""
		gateway := ""
		if IPv4Data[0].Pool != nil {
			subnetPool = IPv4Data[0].Pool.String()
		}
		if IPv4Data[0].Gateway != nil {
			gateway = strings.Split(IPv4Data[0].Gateway.String(), "/")[0]
		}
		subnetv6 := ""
		gatewayv6 := ""
		if len(IPv6Data) > 0 {
			if IPv6Data[0].Pool != nil {
				subnetv6 = IPv6Data[0].Pool.String()
			}
			if IPv6Data[0].Gateway != nil {
				gatewayv6 = strings.Split(IPv6Data[0].Gateway.String(), "/")[0]
			}
		}
		// build key and URL
		keyStr := "default" + ":" + networkID
		url := "/api/v1/networks/" + keyStr + "/"

		tenantName = "default"
		networkName = networkID
		serviceName = ""

		req := client.Network{
			TenantName:  tenantName,
			NetworkName: networkName,
			Subnet:      subnetPool,
			Gateway:     gateway,
			Ipv6Subnet:  subnetv6,
			Ipv6Gateway: gatewayv6,
			Encap:       "vxlan",
		}

		var resp client.Network
		err = cluster.MasterPostReq(url, &req, &resp)
		if err != nil {
			log.Errorf("failed to create network in netmaster: %s", err.Error())
			return errors.New("failed to create network in netmaster")
		}
		log.Infof("Created contiv network %+v", req)
	}
	// Create docknet oper state to map the docker network to contiv network
	// We do not create a network in docker as it is created explicitly by user
	err = docknet.CreateDockNetState(tenantName, networkName, serviceName, networkID)
	if err != nil {
		log.Errorf("Error creating docknet state: %s", err.Error())
		return errors.New("Error creating docknet state")
	}
	return nil
}

// deleteNetworkHelper removes the association between docker network
// and contiv network. We have to remove docker network state before
// remove network in contiv.
func deleteNetworkHelper(networkID string) error {
	dnet, err := docknet.FindDocknetByUUID(networkID)
	if err == nil {
		// delete the dnet oper state
		err = docknet.DeleteDockNetState(dnet.TenantName, dnet.NetworkName, dnet.ServiceName)
		if err != nil {
			msg := fmt.Sprintf("Could not delete docknet for nwID %s: %s", networkID, err.Error())
			log.Errorf(msg)
			return errors.New(msg)
		}
		log.Infof("Deleted docker network mapping for %v", networkID)
	} else {
		msg := fmt.Sprintf("Could not find Docker network %s: %s", networkID, err.Error())
		log.Errorf(msg)
	}

	netID := networkID + ".default"
	_, err = utils.GetNetwork(netID)
	if err == nil {
		// if we find a contiv network with the ID hash, then it must be
		// a docker created network (from the libnetwork create api).
		// build key and URL
		keyStr := "default" + ":" + networkID
		url := "/api/v1/networks/" + keyStr + "/"

		err = cluster.MasterDelReq(url)
		if err != nil {
			msg := fmt.Sprintf("Failed to delete network: %s", err.Error())
			log.Errorf(msg)
			return errors.New(msg)
		}
		log.Infof("Deleted contiv network %v", networkID)
	} else {
		log.Infof("Could not find contiv network %v", networkID)
	}

	return nil
}
