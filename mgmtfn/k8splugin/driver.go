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
	"net"
	"net/http"
	"runtime"
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
	"github.com/contiv/netplugin/utils/netutils"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
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
	IPAddress string
	PortName  string
	Gateway   string
}

// netdGetEndpoint is a utility that reads the EP oper state
func netdGetEndpoint(epID string) (*drivers.OperEndpointState, error) {
	// Get hold of the state driver
	stateDriver, err := utils.GetStateDriver()
	if err != nil {
		return nil, err
	}

	operEp := &drivers.OperEndpointState{}
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
	ep, err := netdGetEndpoint(netID + "-" + req.EndpointID)
	if err == nil {
		return nil, fmt.Errorf("EP %s already exists", req.EndpointID)
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
		epCleanUp(req)
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

func setLinkAddress(link netlink.Link, address string) error {
	addr, err := netlink.ParseAddr(address)
	if err != nil {
		return fmt.Errorf("failed to parse address %s: %v", address, err)
	}

	return netlink.AddrAdd(link, addr)
}

func enterPIDNetNS(pid int) error {
	netNS, err := netns.GetFromPid(pid)
	if err != nil {
		return fmt.Errorf("failed to get the netns of pid %v: %v", pid, err)
	}
	defer netNS.Close()

	if err = netns.Set(netNS); err != nil {
		return fmt.Errorf("failed to enter network namespace of pid %v: %v", pid, err)
	}

	return nil
}

// setIfAttrs sets the required attributes for the container interface
func setIfAttrs(pid int, ifname, cidr, newname string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	globalNS, err := netns.Get()
	if err != nil {
		return fmt.Errorf("failed to get the global network namespace: %v", err)
	}

	defer func() {
		netns.Set(globalNS)
		globalNS.Close()
	}()

	// find the link
	link, err := getLink(ifname)
	if err != nil {
		return fmt.Errorf("unable to find link %q. Error %q", ifname, err)
	}

	// move to the desired netns
	if err = netlink.LinkSetNsPid(link, pid); err != nil {
		return fmt.Errorf("unable to move interface %s to pid %d. Error: %s",
			ifname, pid, err)
	}

	netNS, err := netns.GetFromPid(pid)
	if err != nil {
		return fmt.Errorf("failed to get the netns of pid %v: %v", pid, err)
	}

	if err = netns.Set(netNS); err != nil {
		return fmt.Errorf("failed to enter network namespace of pid %v: %v", pid, err)
	}

	link, err = getLink(ifname)
	if err != nil {
		return fmt.Errorf("unable to find link %q. Error %q", ifname, err)
	}

	// rename the interface from ifname to newname
	if err := netlink.LinkSetName(link, newname); err != nil {
		return fmt.Errorf("failed to rename interface %v to %v: %v",
			link, newname, err)
	}

	// set the ip address
	if err = setLinkAddress(link, cidr); err != nil {
		return fmt.Errorf("failed to bring up link %v: %v", newname, err)
	}

	// set the link up
	if err = netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("unable to bring up interface %v with address %v: %v",
			newname, cidr, err)
	}

	return nil
}

func addStaticRoute(pid int, subnet, intfName string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	globalNS, err := netns.Get()
	if err != nil {
		return fmt.Errorf("failed to get the global network namespace: %v", err)
	}

	defer func() {
		netns.Set(globalNS)
		globalNS.Close()
	}()

	if err = enterPIDNetNS(pid); err != nil {
		return fmt.Errorf("failed to enter network namespace of pid %v: %v", pid, err)
	}

	parsedSubnet, err := netlink.ParseIPNet(subnet)
	if err != nil {
		return fmt.Errorf("failed to parse subnet %v", subnet)
	}

	// find the link
	link, err := getLink(intfName)
	if err != nil {
		return fmt.Errorf("unable to find link %q. Error %q", intfName, err)
	}

	r := netlink.Route{LinkIndex: link.Attrs().Index,
		Dst: parsedSubnet,
	}

	// set static route
	if err = netlink.RouteAdd(&r); err != nil {
		return fmt.Errorf("failed to set default gw %v. Error: %v", parsedSubnet, err)
	}

	return nil
}

// setDefGw sets the default gateway for the container namespace
func setDefGw(pid int, gw, intfName string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	globalNS, err := netns.Get()
	if err != nil {
		return fmt.Errorf("failed to get the global network namespace: %v", err)
	}

	defer func() {
		netns.Set(globalNS)
		globalNS.Close()
	}()

	if err = enterPIDNetNS(pid); err != nil {
		return fmt.Errorf("failed to enter network namespace of pid %v: %v", pid, err)
	}

	addr := net.ParseIP(gw)
	if addr == nil {
		return fmt.Errorf("failed to parse address %v", gw)
	}

	// find the link
	link, err := getLink(intfName)
	if err != nil {
		return fmt.Errorf("unable to find link %q. Error %q", intfName, err)
	}

	r := netlink.Route{LinkIndex: link.Attrs().Index,
		Gw: addr,
	}

	// set default gw
	if err = netlink.RouteAdd(&r); err != nil {
		return fmt.Errorf("failed to set default gw %v. Error: %v", gw, err)
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

	var epErr error

	defer func() {
		if epErr != nil {
			log.Errorf("error %s, remove endpoint", epErr)
			netPlugin.DeleteHostAccPort(epReq.EndpointID)
			epCleanUp(epReq)
		}
	}()

	// convert netns to pid that netlink needs
	pid, epErr := nsToPID(pInfo.NwNameSpace)
	if epErr != nil {
		log.Errorf("Error moving to netns. Err: %v", epErr)
		setErrorResp(&resp, "Error moving to netns", epErr)
		return resp, epErr
	}

	// Set interface attributes for the new port
	epErr = setIfAttrs(pid, ep.PortName, ep.IPAddress, pInfo.IntfName)
	if epErr != nil {
		log.Errorf("Error setting interface attributes. Err: %v", epErr)
		setErrorResp(&resp, "Error setting interface attributes", epErr)
		return resp, epErr
	}

	// if Gateway is not specified on the nw, use the host gateway
	gwIntf := pInfo.IntfName
	gw := ep.Gateway
	if gw == "" {
		hostIf := netutils.GetHostIntfName(ep.PortName)
		hostIP, err := netPlugin.CreateHostAccPort(hostIf, ep.IPAddress)
		if err != nil {
			log.Errorf("Error setting host access. Err: %v", err)
		} else {
			err = setIfAttrs(pid, hostIf, hostIP, "host1")
			if err != nil {
				log.Errorf("Move to pid %d failed", pid)
			} else {
				gw, err = netutils.HostIPToGateway(hostIP)
				if err != nil {
					log.Errorf("Error getting host GW ip: %s, err: %v", hostIP, err)
				} else {
					gwIntf = "host1"
					// make sure service subnet points to eth0
					svcSubnet := contivK8Config.SvcSubnet
					addStaticRoute(pid, svcSubnet, pInfo.IntfName)
				}
			}
		}

	}

	// Set default gateway
	epErr = setDefGw(pid, gw, gwIntf)
	if epErr != nil {
		log.Errorf("Error setting default gateway. Err: %v", epErr)
		setErrorResp(&resp, "Error setting default gateway", epErr)
		return resp, epErr
	}

	resp.Result = 0
	resp.IPAddress = ep.IPAddress
	resp.EndpointID = pInfo.InfraContainerID
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
