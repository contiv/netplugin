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
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/mgmtfn/k8splugin/cniapi"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/netmaster/mastercfg"
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
}

// epAttr contains the assigned attributes of the created ep
type epAttr struct {
	IPAddress string
	PortName  string
	Gateway   string
}

// netdGetEndpoint is a utility that reads the EP oper state
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

// netdGetNetwork is a utility that reads the n/w oper state
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

// epCleanUp deletes the ep from netplugin and netmaster
func epCleanUp(req *epSpec) error {
	// first delete from netplugin
	// ignore any errors as this is best effort
	netID := req.Network + "." + req.Tenant
	err1 := netPlugin.DeleteEndpoint(netID + "-" + req.EndpointID)

	// now delete from master
	delReq := master.DeleteEndpointRequest{
		TenantName:  req.Tenant,
		NetworkName: req.Network,
		ServiceName: req.Group,
		EndpointID:  req.EndpointID,
	}

	var delResp master.DeleteEndpointResponse
	err2 := cluster.MasterPostReq("/plugin/deleteEndpoint", &delReq, &delResp)

	if err1 != nil {
		return err1
	}

	return err2
}

// createEP creates the specified EP in contiv
func createEP(req *epSpec) (*epAttr, error) {

	// if the ep already exists, treat as error for now.
	netID := req.Network + "." + req.Tenant
	ep, err := netdGetEndpoint(netID + "-" + req.EndpointID)
	if err == nil {
		return nil, fmt.Errorf("EP %s already exists", req.EndpointID)
	}

	// Build endpoint request
	mreq := master.CreateEndpointRequest{
		TenantName:  req.Tenant,
		NetworkName: req.Network,
		ServiceName: req.Group,
		EndpointID:  req.EndpointID,
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

	log.Infof("Got endpoint create resp from master: %+v", mresp)

	// Ask netplugin to create the endpoint
	err = netPlugin.CreateEndpoint(netID + "-" + req.EndpointID)
	if err != nil {
		log.Errorf("Endpoint creation failed. Error: %s", err)
		epCleanUp(req)
		return nil, err
	}

	ep, err = netdGetEndpoint(netID + "-" + req.EndpointID)
	if err != nil {
		epCleanUp(req)
		return nil, err
	}

	log.Debug(ep)
	// need to get the subnetlen from nw state.
	nw, err := netdGetNetwork(netID)
	if err != nil {
		return nil, err
	}

	epResponse := epAttr{}
	epResponse.PortName = ep.PortName
	epResponse.IPAddress = ep.IPAddress + "/" + strconv.Itoa(int(nw.SubnetLen))
	epResponse.Gateway = nw.Gateway

	return &epResponse, nil
}

// getLink is a wrapper that fetches the netlink corresponding to the ifname
func getLink(ifname string) (netlink.Link, error) {
	// find the link
	link, err := netlink.LinkByName(ifname)
	if err != nil {
		if !strings.Contains(err.Error(), "Link not found") {
			log.Errorf("unable to find link %q. Error: %q", ifname, err)
			return link, err
		}
		// try once more as sometimes (somehow) link creation is taking
		// sometime, causing link not found error
		time.Sleep(1 * time.Second)
		link, err = netlink.LinkByName(ifname)
		if err != nil {
			log.Errorf("unable to find link %q. Error %q", ifname, err)
		}
		return link, err
	}
	return link, err
}

// nsToPID is a utility that extracts the PID from the netns
func nsToPID(ns string) (int, error) {
	// Make sure ns is well formed
	ok := strings.HasPrefix(ns, "/proc/")
	if !ok {
		return -1, fmt.Errorf("Invalid nw name space: %v", ns)
	}

	elements := strings.Split(ns, "/")
	return strconv.Atoi(elements[2])
}

// setIfAttrs sets the required attributes for the container interface
func setIfAttrs(pid int, ifname, cidr, newname string) error {

	nsenterPath, err := osexec.LookPath("nsenter")
	if err != nil {
		return err
	}
	ipPath, err := osexec.LookPath("ip")
	if err != nil {
		return err
	}

	// find the link
	link, err := getLink(ifname)
	if err != nil {
		log.Errorf("unable to find link %q. Error %q", ifname, err)
		return err
	}

	// move to the desired netns
	err = netlink.LinkSetNsPid(link, pid)
	if err != nil {
		log.Errorf("unable to move interface %s to pid %d. Error: %s",
			ifname, pid, err)
		return err
	}

	// rename to the desired ifname
	nsPid := fmt.Sprintf("%d", pid)
	rename, err := osexec.Command(nsenterPath, "-t", nsPid, "-n", "-F", "--", ipPath, "link",
		"set", "dev", ifname, "name", newname).CombinedOutput()
	if err != nil {
		log.Errorf("unable to rename interface %s to %s. Error: %s",
			ifname, newname, err)
		return nil
	}
	log.Infof("Output from rename: %v", rename)

	// set the ip address
	assignIP, err := osexec.Command(nsenterPath, "-t", nsPid, "-n", "-F", "--", ipPath,
		"address", "add", cidr, "dev", newname).CombinedOutput()

	if err != nil {
		log.Errorf("unable to assign ip %s to %s. Error: %s",
			cidr, newname, err)
		return nil
	}
	log.Infof("Output from ip assign: %v", assignIP)

	// Finally, mark the link up
	bringUp, err := osexec.Command(nsenterPath, "-t", nsPid, "-n", "-F", "--", ipPath,
		"link", "set", "dev", newname, "up").CombinedOutput()

	if err != nil {
		log.Errorf("unable to assign ip %s to %s. Error: %s",
			cidr, newname, err)
		return nil
	}
	log.Debugf("Output from ip assign: %v", bringUp)
	return nil

}

// setDefGw sets the default gateway for the container namespace
func setDefGw(pid int, gw, intfName string) error {
	nsenterPath, err := osexec.LookPath("nsenter")
	if err != nil {
		return err
	}
	routePath, err := osexec.LookPath("route")
	if err != nil {
		return err
	}
	// set default gw
	nsPid := fmt.Sprintf("%d", pid)
	_, err = osexec.Command(nsenterPath, "-t", nsPid, "-n", "-F", "--", routePath, "add",
		"default", "gw", gw, intfName).CombinedOutput()
	if err != nil {
		log.Errorf("unable to set default gw %s. Error: %s",
			gw, err)
		return nil
	}
	return nil
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

	return &resp, nil
}

// addPod is the handler for pod additions
func addPod(r *http.Request) (interface{}, error) {

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
		return resp, err
	}

	ep, err := createEP(epReq)
	if err != nil {
		log.Errorf("Error creating ep. Err: %v", err)
		return resp, err
	}

	// convert netns to pid that netlink needs
	pid, err := nsToPID(pInfo.NwNameSpace)
	if err != nil {
		return resp, err
	}

	// Set interface attributes for the new port
	err = setIfAttrs(pid, ep.PortName, ep.IPAddress, pInfo.IntfName)
	if err != nil {
		log.Errorf("Error setting interface attributes. Err: %v", err)
		return resp, err
	}

	// Set default gateway
	err = setDefGw(pid, ep.Gateway, pInfo.IntfName)
	if err != nil {
		log.Errorf("Error setting default gateway. Err: %v", err)
		return resp, err
	}

	resp.IPAddress = ep.IPAddress
	resp.EndpointID = pInfo.InfraContainerID
	return resp, nil
}

// deletePod is the handler for pod deletes
func deletePod(r *http.Request) (interface{}, error) {

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
		return resp, err
	}

	err = epCleanUp(epReq)
	resp.EndpointID = pInfo.InfraContainerID
	return resp, err
}
