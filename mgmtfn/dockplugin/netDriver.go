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
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/netplugin/cluster"
	"github.com/contiv/netplugin/netplugin/directapi"
	"github.com/contiv/netplugin/utils"
	"github.com/docker/libnetwork/drivers/remote/api"
	"github.com/samalba/dockerclient"
)

const defaultTenantName = "default"

func getCapability() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logEvent("getCapability")

		content, err := json.Marshal(api.GetCapabilityResponse{Scope: "global"})
		if err != nil {
			httpError(w, "Could not generate getCapability response", err)
			return
		}

		w.Write(content)
	}
}

func deleteNetwork() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logEvent("delete network")

		content, err := ioutil.ReadAll(r.Body)
		if err != nil {
			httpError(w, "Could not read delete network request", err)
			return
		}

		dnreq := api.DeleteNetworkRequest{}
		if err := json.Unmarshal(content, &dnreq); err != nil {
			httpError(w, "Could not read delete network request", err)
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
}

func createNetwork() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logEvent("create network")

		content, err := ioutil.ReadAll(r.Body)
		if err != nil {
			httpError(w, "Could not read create network request", err)
			return
		}

		log.Infoln(string(content))

		cnreq := api.CreateNetworkRequest{}
		if err := json.Unmarshal(content, &cnreq); err != nil {
			httpError(w, "Could not read create network request", err)
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
}

func deleteEndpoint(hostname string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logEvent("delete endpoint")

		content, err := ioutil.ReadAll(r.Body)
		if err != nil {
			httpError(w, "Could not read delete endpoint request", err)
			return
		}

		der := api.DeleteEndpointRequest{}
		if err := json.Unmarshal(content, &der); err != nil {
			httpError(w, "Could not read delete endpoint request", err)
			return
		}

		log.Infof("Received DeleteEndpointRequest: %+v", der)

		tenantName, netName, serviceName, err := GetDockerNetworkName(der.NetworkID)
		if err != nil {
			log.Errorf("Error getting network name for UUID: %s. Err: %v", der.NetworkID, err)
			httpError(w, "Could not get network name", err)
			return
		}

		// Build endpoint delete request
		delreq := master.DeleteEndpointRequest{
			TenantName:  tenantName,
			NetworkName: netName,
			ServiceName: serviceName,
			EndpointID:  der.EndpointID,
		}

		var delResp master.DeleteEndpointResponse
		err = cluster.MasterPostReq("/plugin/deleteEndpoint", &delreq, &delResp)
		if err != nil {
			httpError(w, "master failed to delete endpoint", err)
			return
		}

		netID := netName + "." + tenantName
		ep, err := netdGetEndpoint(netID + "-" + delreq.EndpointID)
		if err != nil {
			httpError(w, "Could not find endpoint", err)
			return
		}

		// Remove the DNS entry for the service
		if serviceName != "" {
			log.Infof("Calling RemoveService with: ID: %s, Name: %s, Network: %s, Tenant: %s, IP: %s", delreq.EndpointID[len(delreq.EndpointID)-12:], serviceName, netName, tenantName, ep.IPAddress)
			dnsBridge.RemoveService(delreq.EndpointID[len(delreq.EndpointID)-12:], serviceName, netName, tenantName, ep.IPAddress)
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

// epFailCleanUp cleans up if a create fails.
func epFailCleanUp(req directapi.ReqCreateEP) {
	// first delete from netplugin
	// ignore any errors as this is best effort
	netID := req.Network + "." + req.Tenant
	netPlugin.DeleteEndpoint(netID + "-" + req.EndpointID)

	// now delete from master
	delReq := master.DeleteEndpointRequest{
		TenantName:  req.Tenant,
		NetworkName: req.Network,
		ServiceName: req.Group,
		EndpointID:  req.EndpointID,
	}

	var delResp master.DeleteEndpointResponse
	cluster.MasterPostReq("/plugin/deleteEndpoint", &delReq, &delResp)
}

func directEPCreate(hostname string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logEvent("attach endpoint")

		content, err := ioutil.ReadAll(r.Body)
		if err != nil {
			httpError(w, "Could not read endpoint create request", err)
			return
		}

		req := directapi.ReqCreateEP{}
		if err := json.Unmarshal(content, &req); err != nil {
			httpError(w, "Could not read endpoint attach request", err)
			return
		}

		// if the ep already exists, just return with the info
		netID := req.Network + "." + req.Tenant
		ep, err := netdGetEndpoint(netID + "-" + req.EndpointID)
		if err != nil {

			// Build endpoint request
			mreq := master.CreateEndpointRequest{
				TenantName:  req.Tenant,
				NetworkName: req.Network,
				ServiceName: req.Group,
				EndpointID:  req.EndpointID,
				ConfigEP: intent.ConfigEP{
					Container:   req.EndpointID,
					Host:        hostname,
					ServiceName: req.Group,
				},
			}

			var mresp master.CreateEndpointResponse
			err = cluster.MasterPostReq("/plugin/createEndpoint", &mreq, &mresp)
			if err != nil {
				epFailCleanUp(req)
				httpError(w, "master failed to create endpoint", err)
				return
			}

			log.Infof("Got endpoint create resp from master: %+v", mresp)

			// Ask netplugin to create the endpoint
			err = netPlugin.CreateEndpoint(netID + "-" + req.EndpointID)
			if err != nil {
				log.Errorf("Endpoint creation failed. Error: %s", err)
				epFailCleanUp(req)
				httpError(w, "Could not create endpoint", err)
				return
			}

			ep, err = netdGetEndpoint(netID + "-" + req.EndpointID)
			if err != nil {
				epFailCleanUp(req)
				httpError(w, "Could not find created endpoint", err)
				return
			}
		}

		log.Debug(ep)
		// need to get the subnetlen from nw state.
		nw, err := netdGetNetwork(netID)
		if err != nil {
			httpError(w, "Could not read network oper state", err)
			return
		}

		epResponse := directapi.RspCreateEP{}
		epResponse.EndpointID = ep.ContUUID
		epResponse.IntfName = ep.PortName
		epResponse.IPAddress = ep.IPAddress + "/" + strconv.Itoa(int(nw.SubnetLen))

		// Add the service information using Service plugin
		//		if serviceName != "" {
		//			log.Infof("Calling AddService with: ID: %s, Name: %s, Network: %s, Tenant: %s, IP: %s", cereq.EndpointID[len(cereq.EndpointID)-12:], serviceName, netName, tenantName, ep.IPAddress)
		//			dnsBridge.AddService(cereq.EndpointID[len(cereq.EndpointID)-12:], serviceName, netName, tenantName, ep.IPAddress)
		//		}

		log.Infof("Sending CreateEndpointResponse: {%+v}, IP Addr: %v", epResponse, ep.IPAddress)

		content, err = json.Marshal(epResponse)
		if err != nil {
			httpError(w, "Could not generate create endpoint response", err)
			return
		}

		w.Write(content)
	}
}

func createEndpoint(hostname string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logEvent("create endpoint")

		content, err := ioutil.ReadAll(r.Body)
		if err != nil {
			httpError(w, "Could not read endpoint create request", err)
			return
		}

		cereq := api.CreateEndpointRequest{}
		if err := json.Unmarshal(content, &cereq); err != nil {
			httpError(w, "Could not read endpoint create request", err)
			return
		}

		log.Infof("CreateEndpointRequest: %+v. Interface: %+v", cereq, cereq.Interface)

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
				ServiceName: serviceName,
			},
		}

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

		ep, err := netdGetEndpoint(netID + "-" + cereq.EndpointID)
		if err != nil {
			httpError(w, "Could not find created endpoint", err)
			return
		}

		log.Debug(ep)

		epResponse := api.CreateEndpointResponse{
			Interface: &api.EndpointInterface{},
		}

		// Add the service information using Service plugin
		if serviceName != "" {
			log.Infof("Calling AddService with: ID: %s, Name: %s, Network: %s, Tenant: %s, IP: %s", cereq.EndpointID[len(cereq.EndpointID)-12:], serviceName, netName, tenantName, ep.IPAddress)
			dnsBridge.AddService(cereq.EndpointID[len(cereq.EndpointID)-12:], serviceName, netName, tenantName, ep.IPAddress)
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
	logEvent("endpoint info")

	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httpError(w, "Could not read endpoint create request", err)
		return
	}

	epireq := api.EndpointInfoRequest{}

	if err := json.Unmarshal(content, &epireq); err != nil {
		httpError(w, "Could not read endpoint create request", err)
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

func join() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logEvent("join")

		jr := api.JoinRequest{}
		content, err := ioutil.ReadAll(r.Body)
		if err != nil {
			httpError(w, "Could not read join request", err)
			return
		}

		if err := json.Unmarshal(content, &jr); err != nil {
			httpError(w, "Could not parse join request", err)
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
		ep, err := netdGetEndpoint(netID + "-" + jr.EndpointID)
		if err != nil {
			httpError(w, "Could not find created endpoint", err)
			return
		}

		nw, err := netdGetNetwork(netID)
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
}

func leave() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logEvent("leave")

		lr := api.LeaveRequest{}
		content, err := ioutil.ReadAll(r.Body)
		if err != nil {
			httpError(w, "Could not read leave request", err)
			return
		}

		if err := json.Unmarshal(content, &lr); err != nil {
			httpError(w, "Could not parse leave request", err)
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
}

func netdGetEndpoint(epID string) (*drivers.OvsOperEndpointState, error) {
	// Get hold of the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return nil, err
	}

	operEp := &drivers.OvsOperEndpointState{}
	operEp.StateDriver = stateDriver
	err = operEp.Read(epID)
	if err != nil {
		return nil, err
	}

	return operEp, nil
}

func netdGetNetwork(networkID string) (*mastercfg.CfgNetworkState, error) {
	// Get hold of the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return nil, err
	}

	// find the network from network id
	nwCfg := &mastercfg.CfgNetworkState{}
	nwCfg.StateDriver = stateDriver
	err = nwCfg.Read(networkID)
	if err != nil {
		return nil, err
	}

	return nwCfg, nil
}

// GetDockerNetworkName gets network name from network UUID
func GetDockerNetworkName(nwID string) (string, string, string, error) {
	docker, err := dockerclient.NewDockerClient("unix:///var/run/docker.sock", nil)
	if err != nil {
		log.Errorf("Unable to connect to docker. Error %v", err)
		return "", "", "", errors.New("Unable to connect to docker")
	}

	nwList, err := docker.ListNetworks("")
	if err != nil {
		log.Infof("Error: %v", err)
		return "", "", "", err
	}

	log.Debugf("Got networks:")

	// find the network by uuid
	for _, nw := range nwList {
		log.Debugf("%+v", nw)
		if nw.ID == nwID {
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
				// has ser.network in default tenant
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
				return "", "", "", errors.New("Invalid format")
			}

			return tenantName, netName, serviceName, nil

		}
	}

	// UUID was not Found
	return "", "", "", errors.New("Network UUID not found")
}
