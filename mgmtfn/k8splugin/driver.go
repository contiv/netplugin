/***
Copyright 2016 Cisco Systems Inc. All rights reserved.

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

package k8splugin

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	osexec "os/exec"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/mgmtfn/k8splugin/cniapi"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/netplugin/cluster"
	"github.com/contiv/netplugin/utils"
	"github.com/vishvananda/netlink"
)

// epSpec contains the spec of the Endpoint to be created
type epSpec struct {
	Tenant     string `json:"tenant,omitempty"`
	Network    string `json:"network,omitempty"`
	Group      string `json:"group,omitempty"`
	EndpointID string `json:"endpointid,omitempty"`
	Name       string `json:"name,omitempty"`
}

// epAttr contains the assigned attributes of the created ep
type epAttr struct {
	IPAddress   string
	PortName    string
	Gateway     string
	IPv6Address string
	IPv6Gateway string
}

// epCleanUp deletes the ep from netplugin and netmaster
func epCleanUp(req *epSpec) error {
	// first delete from netplugin
	// ignore any errors as this is best effort
	netID := req.Network + "." + req.Tenant
	pluginErr := netPlugin.DeleteEndpoint(netID + "-" + req.EndpointID)

	// now delete from master
	delReq := master.DeleteEndpointRequest{
		TenantName:  req.Tenant,
		NetworkName: req.Network,
		ServiceName: req.Group,
		EndpointID:  req.EndpointID,
	}

	var delResp master.DeleteEndpointResponse
	masterErr := cluster.MasterPostReq("/plugin/deleteEndpoint", &delReq, &delResp)

	if pluginErr != nil {
		log.Errorf("failed to delete endpoint: %s from netplugin %s",
			netID+"-"+req.EndpointID, pluginErr)
		return pluginErr
	}

	if masterErr != nil {
		log.Errorf("failed to delete endpoint %+v from netmaster, %s", delReq, masterErr)
	}

	return masterErr
}

// createEP creates the specified EP in contiv
func createEP(req *epSpec) (*epAttr, error) {

	// if the ep already exists, treat as error for now.
	netID := req.Network + "." + req.Tenant
	ep, err := utils.GetEndpoint(netID + "-" + req.EndpointID)
	if err == nil {
		return nil, fmt.Errorf("the EP %s already exists", req.EndpointID)
	}

	// Build endpoint request
	mreq := master.CreateEndpointRequest{
		TenantName:   req.Tenant,
		NetworkName:  req.Network,
		ServiceName:  req.Group,
		EndpointID:   req.EndpointID,
		EPCommonName: req.Name,
		ConfigEP: intent.ConfigEP{
			Container:   req.EndpointID,
			Host:        pluginHost,
			ServiceName: req.Group,
		},
	}

	var mresp master.CreateEndpointResponse
	err = cluster.MasterPostReq("/plugin/createEndpoint", &mreq, &mresp)
	if err != nil {
		epCleanUp(req)
		return nil, err
	}

	// this response should contain IPv6 if the underlying network is configured with IPv6
	log.Infof("Got endpoint create resp from master: %+v", mresp)

	// Ask netplugin to create the endpoint
	err = netPlugin.CreateEndpoint(netID + "-" + req.EndpointID)
	if err != nil {
		log.Errorf("Endpoint creation failed. Error: %s", err)
		epCleanUp(req)
		return nil, err
	}

	ep, err = utils.GetEndpoint(netID + "-" + req.EndpointID)
	if err != nil {
		epCleanUp(req)
		return nil, err
	}

	log.Debug(ep)
	// need to get the subnetlen from nw state.
	nw, err := utils.GetNetwork(netID)
	if err != nil {
		epCleanUp(req)
		return nil, err
	}

	epResponse := epAttr{}
	epResponse.PortName = ep.PortName
	epResponse.IPAddress = ep.IPAddress + "/" + strconv.Itoa(int(nw.SubnetLen))
	epResponse.Gateway = nw.Gateway

	if ep.IPv6Address != "" {
		epResponse.IPv6Address = ep.IPv6Address + "/" + strconv.Itoa(int(nw.IPv6SubnetLen))
		epResponse.IPv6Gateway = nw.IPv6Gateway
	}

	return &epResponse, nil
}

// getEPSpec gets the EP spec using the pod attributes
func getEPSpec(pInfo *cniapi.CNIPodAttr) (*epSpec, error) {
	resp := epSpec{}

	// Get labels from the kube api server
	epg, err := kubeAPIClient.GetPodLabel(pInfo.K8sNameSpace, pInfo.Name,
		"io.contiv.net-group")
	if err != nil {
		log.Errorf("Error getting epg. Err: %v", err)
		return &resp, err
	}

	// Safe to ignore the error return for subsequent invocations of GetPodLabel
	netw, _ := kubeAPIClient.GetPodLabel(pInfo.K8sNameSpace, pInfo.Name,
		"io.contiv.network")
	tenant, _ := kubeAPIClient.GetPodLabel(pInfo.K8sNameSpace, pInfo.Name,
		"io.contiv.tenant")
	log.Infof("labels is %s/%s/%s for pod %s\n", tenant, netw, epg, pInfo.Name)
	resp.Tenant = tenant
	resp.Network = netw
	resp.Group = epg
	resp.EndpointID = pInfo.InfraContainerID
	resp.Name = pInfo.Name

	return &resp, nil
}

func setErrorResp(resp *cniapi.RspAddPod, msg string, err error) {
	resp.Result = 1
	resp.ErrMsg = msg
	resp.ErrInfo = fmt.Sprintf("Err: %v", err)
}

// addPod is the handler for pod additions
func addPod(w http.ResponseWriter, r *http.Request, vars map[string]string) (interface{}, error) {

	resp := cniapi.RspAddPod{}

	logEvent("add pod")

	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Errorf("Failed to read request: %v", err)
		return resp, err
	}

	pInfo := cniapi.CNIPodAttr{}
	if err := json.Unmarshal(content, &pInfo); err != nil {
		return resp, err
	}

	// Get labels from the kube api server
	epReq, err := getEPSpec(&pInfo)
	if err != nil {
		log.Errorf("Error getting labels. Err: %v", err)
		setErrorResp(&resp, "Error getting labels", err)
		return resp, err
	}

	ep, err := createEP(epReq)
	if err != nil {
		log.Errorf("Error creating ep. Err: %v", err)
		setErrorResp(&resp, "Error creating EP", err)
		return resp, err
	}

	resp.Result = 0

	resp.EndpointID = pInfo.InfraContainerID

	resp.Attr = &cniapi.Attr{IPAddress: ep.IPAddress, PortName: ep.PortName,
		Gateway: ep.Gateway, IPv6Address: ep.IPv6Address, IPv6Gateway: ep.IPv6Gateway}

	return resp, nil
}

// deletePod is the handler for pod deletes
func deletePod(w http.ResponseWriter, r *http.Request, vars map[string]string) (interface{}, error) {

	resp := cniapi.RspAddPod{}

	logEvent("del pod")

	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Errorf("Failed to read request: %v", err)
		return resp, err
	}

	pInfo := cniapi.CNIPodAttr{}
	if err := json.Unmarshal(content, &pInfo); err != nil {
		return resp, err
	}

	// Get labels from the kube api server
	epReq, err := getEPSpec(&pInfo)
	if err != nil {
		log.Errorf("Error getting labels. Err: %v", err)
		setErrorResp(&resp, "Error getting labels", err)
		return resp, err
	}

	netPlugin.DeleteHostAccPort(epReq.EndpointID)
	if err = epCleanUp(epReq); err != nil {
		log.Errorf("failed to delete pod, error: %s", err)
	}
	resp.Result = 0
	resp.EndpointID = pInfo.InfraContainerID
	return resp, nil
}
