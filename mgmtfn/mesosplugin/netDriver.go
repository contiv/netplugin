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

package mesosplugin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/mgmtfn/mesosplugin/api"
	"github.com/contiv/netplugin/netlib"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/netplugin/cluster"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/netplugin/utils/netutils"
	"github.com/jainvipin/bitset"
	"github.com/vishvananda/netlink"
)

const defaultTenantName = "default"

type endpointState struct {
	endpointRequest master.CreateEndpointRequest
	endpoint        *drivers.OvsOperEndpointState
	network         *mastercfg.CfgNetworkState
	vethMasterName  string
	vethIP          string
}

type endpoints struct {
	epStates map[string]endpointState
	ipMap    bitset.BitSet
	sync.Mutex
}

var allocatedEndpoints *endpoints

func init() {
	allocatedEndpoints = &endpoints{
		epStates: make(map[string]endpointState),
	}
	netutils.InitSubnetBitset(&allocatedEndpoints.ipMap, 24)
	ipAddrValue, _ := allocatedEndpoints.ipMap.NextClear(0)
	allocatedEndpoints.ipMap.Set(ipAddrValue)
}

func join(w http.ResponseWriter, r *http.Request) {
	var (
		content []byte
		err     error
		decoder = json.NewDecoder(r.Body)
		cereq   = api.VirtualizerRequest{}
	)

	logEvent("create endpoint")

	err = decoder.Decode(&cereq)
	if err != nil {
		httpError(w, "Could not read and parse the create endpoint request", err)
		return
	}

	log.Infof("CreateEndpointRequest: %+v", cereq)

	epParams, err := getEndpointParameters(cereq.Netgroups, cereq.Labels)
	if err != nil {
		log.Errorf("Error getting endpoint parameters: %v. Err: %v", cereq, err)
		httpError(w, "Could not get endpoint parameters", err)
		return
	}

	netID := fmt.Sprintf("%v.%v", epParams.network, epParams.tenant)
	endpointID := fmt.Sprintf("%v-%v", netID, cereq.ContainerID)
	hostname, _ := os.Hostname()
	// Build endpoint request
	mreq := master.CreateEndpointRequest{
		TenantName:  epParams.tenant,
		NetworkName: epParams.network,
		ServiceName: epParams.group,
		EndpointID:  cereq.ContainerID,
		ConfigEP: intent.ConfigEP{
			Container:   cereq.ContainerID,
			Host:        hostname,
			IPAddress:   cereq.IPV4Addresses[1],
			ServiceName: epParams.group,
		},
	}

	var mresp master.CreateEndpointResponse
	err = cluster.MasterPostReq("/plugin/createEndpoint", &mreq, &mresp)
	if err != nil {
		httpError(w, "master failed to create endpoint", err)
		return
	}

	log.Infof("Got endpoint create resp from master: %+v", mresp)

	// Ask netplugin to create the endpoint
	err = netPlugin.CreateEndpoint(endpointID)
	if err != nil {
		log.Errorf("Endpoint creation failed. Error: %s", err)
		httpError(w, "Could not create endpoint", err)
		return
	}

	ep, err := netdGetEndpoint(endpointID)
	if err != nil {
		httpError(w, "Could not find created endpoint", err)
		return
	}

	log.Infof("ep %+v", ep)

	net, err := netdGetNetwork(ep.NetID)
	if err != nil {
		httpError(w, "Could not find network by NetID", err)
		return
	}

	ipAddress := ep.IPAddress + "/" + strconv.Itoa(int(net.SubnetLen))

	if1, if2 := netlib.GenerateVethNames()
	log.Infof("generated veth names: %v, %v", if1, if2)

	allocatedEndpoints.Lock()
	allocatedEndpoints.epStates[cereq.ContainerID] = endpointState{
		endpointRequest: mreq,
		endpoint:        ep,
		network:         net,
		vethMasterName:  if1,
		vethIP:          cereq.IPV4Addresses[0],
	}
	allocatedEndpoints.Unlock()

	bridgeIf := "contivmesos0"
	// run on host
	// FIXME: handle error
	netlib.AddBridge("10.112.95.1/24", bridgeIf)

	// FIXME: handle error
	netlib.CreateVethPair(cereq.PID, bridgeIf, if1, if2)

	finalVethIP := fmt.Sprintf("%s/24", cereq.IPV4Addresses[0])

	nics := []netlib.NICConfig{
		{IPAddress: ipAddress,
			Name:     ep.PortName,
			NewName:  "eth1",
			MoveToNS: true,
		},
		{IPAddress: finalVethIP,
			Name:      if2,
			NewName:   "eth0",
			GatewayIP: "10.112.95.1",
		},
		{
			Name: "lo",
		},
	}

	log.Infof("nics: %+v", nics)
	/*
		out, err := exec.Command("/sbin/ifconfig").CombinedOutput()
		log.Infof(string(out))
		log.Infof(fmt.Sprintf("error:%v\n", err))
		out, err = exec.Command("/sbin/ip", "r").CombinedOutput()
		log.Infof(string(out))
	*/
	if err := netlib.MoveAndConfigureNICs(cereq.PID, nics); err != nil {
		log.Infof("failed to set up interfaces: %v", err)
		httpError(w, "failed to set up interfaces", err)
	}

	//log.Infof("net %+v", net)

	epResponse := api.VirtualizerResponse{}

	// Add the service information using Service plugin
	/*		if serviceName != "" {
				log.Infof("Calling AddService with: ID: %s, Name: %s, Network: %s, Tenant: %s, IP: %s", cereq.EndpointID[len(cereq.EndpointID)-12:], serviceName, netName, tenantName, ep.IPAddress)
				dnsBridge.AddService(cereq.EndpointID[len(cereq.EndpointID)-12:], serviceName, netName, tenantName, ep.IPAddress)
			}
	*/

	// log.Infof("Sending CreateEndpointResponse: {%+v}, IP Addr: %v", epResponse, ep.IPAddress)
	content, err = json.Marshal(epResponse)
	if err != nil {
		httpError(w, "Could not generate create endpoint response", err)
		return
	}

	w.Write(content)
}

func leave(w http.ResponseWriter, r *http.Request) {
	var (
		content []byte
		err     error
		decoder = json.NewDecoder(r.Body)
		lr      = api.VirtualizerRequest{}
	)

	logEvent("leave")

	err = decoder.Decode(&lr)
	if err != nil {
		httpError(w, "Could not read and parse the leave request", err)
		return
	}

	log.Infof("LeaveRequest: %+v", lr)

	allocatedEndpoints.Lock()
	epState := allocatedEndpoints.epStates[lr.ContainerID]

	netID := fmt.Sprintf("%v.%v", epState.endpointRequest.NetworkName, epState.endpointRequest.TenantName)
	endpointID := fmt.Sprintf("%v-%v", netID, lr.ContainerID)

	err1 := netPlugin.DeleteEndpoint(endpointID)
	if err1 != nil {
		log.Infof("encountered error while deleting the endpoint: %v", err1)
	}

	// now delete from master
	delReq := master.DeleteEndpointRequest{
		TenantName:  epState.endpointRequest.TenantName,
		NetworkName: epState.endpointRequest.NetworkName,
		ServiceName: epState.endpointRequest.ServiceName,
		EndpointID:  lr.ContainerID,
	}

	var delResp master.DeleteEndpointResponse
	err2 := cluster.MasterPostReq("/plugin/deleteEndpoint", &delReq, &delResp)
	if err2 != nil {
		log.Infof("encountered error while deleting the endpoint from the master: %v", err2)
	}

	link, err := netlink.LinkByName(epState.vethMasterName)
	if err != nil {
		log.Infof("encountered error while trying to find eth0: %v", err)
	} else {
		err = netlink.LinkDel(link)
		if err != nil {
			log.Infof("failed to delete link: %v", err)
		}
	}
	ipAddrValue, err := netutils.GetIPNumber("10.112.95.0", 24, 32, epState.vethIP)
	if err == nil {
		allocatedEndpoints.ipMap.Clear(ipAddrValue)
	} else {
		log.Infof("failed to deallocated epState.vethIP")
	}

	delete(allocatedEndpoints.epStates, lr.ContainerID)
	allocatedEndpoints.Unlock()
	// Send response
	leaveResp := api.VirtualizerResponse{}

	log.Infof("Sending LeaveResponse: {%+v}", leaveResp)

	content, err = json.Marshal(leaveResp)
	if err != nil {
		httpError(w, "Could not generate leave response", err)
		return
	}

	w.Write(content)
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
