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
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/netplugin/cluster"
	"github.com/contiv/netplugin/utils"
	"github.com/docker/libnetwork/ipams/remote/api"
	"github.com/docker/libnetwork/netlabel"
)

// getIpamCapability
func getIpamCapability(w http.ResponseWriter, r *http.Request) {
	logEvent("getIpamCapability")

	content, err := json.Marshal(api.GetCapabilityResponse{})
	if err != nil {
		httpError(w, "Could not generate getCapability response", err)
		return
	}

	w.Write(content)
}

// getDefaultAddressSpaces
func getDefaultAddressSpaces(w http.ResponseWriter, r *http.Request) {
	logEvent("getDefaultAddressSpaces")

	rcvd, _ := ioutil.ReadAll(r.Body)
	log.Infof("Body content: %s", string(rcvd))

	content, err := json.Marshal(api.GetAddressSpacesResponse{})
	if err != nil {
		httpError(w, "Could not generate getDefaultAddressSpaces response", err)
		return
	}

	w.Write(content)
}

// requestPool
func requestPool(w http.ResponseWriter, r *http.Request) {
	var (
		content []byte
		err     error
		decoder = json.NewDecoder(r.Body)
		preq    = api.RequestPoolRequest{}
	)

	logEvent("requestPool")

	// Decode the JSON request
	err = decoder.Decode(&preq)
	if err != nil {
		httpError(w, "Could not read and parse requestPool request", err)
		return
	}

	log.Infof("Received RequestPoolRequest: %+v", preq)

	// build response
	PoolID := preq.Pool
	Pool := ""

	tag, tagOk := preq.Options["contiv-tag"]
	if tagOk {
		log.Infof("Received contiv-tag: %v", tag)

		// if the PoolRequest has a tag instead of the network or group,
		//  match the tag to a group (else to network)
		epgCfg, err := FindGroupFromTag(tag)
		if err == nil {
			log.Infof("Found matching epg ")

			if len(epgCfg.IPPool) > 0 {
				PoolID = epgCfg.NetworkName + ":" + epgCfg.GroupName + "." + epgCfg.TenantName + "|" + epgCfg.IPPool
				Pool = epgCfg.IPPool
			} else {
				// Get the pool from the Network
				nwCfg, err := utils.GetNetwork(epgCfg.NetworkName + "." + epgCfg.TenantName)
				if err != nil {
					httpError(w, "failed to lookup network. ", err)
					return
				}
				Pool = fmt.Sprintf("%s/%d", nwCfg.SubnetIP, nwCfg.SubnetLen)
				PoolID = epgCfg.NetworkName + "." + epgCfg.TenantName + "|" + Pool
			}
		} else {
			nwCfg, err := FindNetworkFromTag(tag)
			if err != nil {
				httpError(w, "failed to lookup tag. ", err)
				return
			}
			log.Infof("Found matching network ")
			Pool = fmt.Sprintf("%s/%d", nwCfg.SubnetIP, nwCfg.SubnetLen)
			PoolID = nwCfg.NetworkName + "." + nwCfg.Tenant + "|" + Pool
		}
		log.Infof("PoolID: %v", PoolID)
	} else {
		// Docker 1.10+ supports IPAM options. so, we pass the network id as pool-id
		// In docker 1.9, we pass the address pool back as pool id
		// HACK alert: This is very fragile. SImplify this when we stop supporting docker 1.9
		tenant, okt := preq.Options["tenant"]
		network, okn := preq.Options["network"]
		epg, oke := preq.Options["group"]
		if okt && okn {
			if oke {
				PoolID = network + ":" + epg + "." + tenant + "|" + preq.Pool
			} else {
				PoolID = network + "." + tenant + "|" + preq.Pool
			}
		}
		Pool = preq.Pool
		log.Infof("No tag, PoolID: %v", PoolID)
	}
	presp := api.RequestPoolResponse{
		PoolID: PoolID,
		Pool:   Pool,
	}

	log.Infof("Sending RequestPoolResponse: %+v", presp)

	// build json
	content, err = json.Marshal(presp)
	if err != nil {
		httpError(w, "Could not generate requestPool response", err)
		return
	}
	w.Write(content)
}

// releasePool
func releasePool(w http.ResponseWriter, r *http.Request) {
	var (
		content []byte
		err     error
		decoder = json.NewDecoder(r.Body)
		preq    = api.ReleasePoolRequest{}
	)

	logEvent("releasePool")

	// Decode the JSON message
	err = decoder.Decode(&preq)
	if err != nil {
		httpError(w, "Could not read and parse releasePool request", err)
		return
	}

	log.Infof("Received ReleasePoolRequest: %+v", preq)

	// response
	relResp := api.ReleasePoolResponse{}

	log.Infof("Sending ReleasePoolResponse: {%+v}", relResp)

	content, err = json.Marshal(relResp)
	if err != nil {
		httpError(w, "Could not generate release pool response", err)
		return
	}

	// Send response
	w.Write(content)
}

// requestAddress
func requestAddress(w http.ResponseWriter, r *http.Request) {
	var (
		content []byte
		err     error
		areq    = api.RequestAddressRequest{}
		decoder = json.NewDecoder(r.Body)
	)

	logEvent("requestAddress")

	// Decode the JSON message
	err = decoder.Decode(&areq)
	if err != nil {
		httpError(w, "Could not read and parse requestAddress request", err)
		return
	}

	log.Infof("Received RequestAddressRequest: %+v", areq)

	networkID := ""
	addrPool := areq.PoolID
	subnetLen := strings.Split(areq.PoolID, "/")[1]

	// check if pool id contains address pool or network id
	// HACK alert: This is very fragile. Simplify this when we stop supporting docker 1.9
	if strings.Contains(areq.PoolID, "|") {
		addrPool = strings.Split(areq.PoolID, "|")[1]
		networkID = strings.Split(areq.PoolID, "|")[0]
	}

	// Build an alloc request to be sent to master
	allocReq := master.AddressAllocRequest{
		AddressPool:          addrPool,
		NetworkID:            networkID,
		PreferredIPv4Address: areq.Address,
	}

	var addr string

	// check if this request is for gateway
	reqType, ok := areq.Options["RequestAddressType"]
	if ok && reqType == netlabel.Gateway {
		if areq.Address != "" {
			addr = areq.Address + "/" + subnetLen
		} else {
			// simply return a dummy address
			addr = addrPool
		}
	} else if areq.Address != "" {
		// This is a special case for docker 1.9 gateway request which does not
		// come with 'RequestAddressType' label
		// FIXME: Remove this hack when we stop supporting docker 1.9
		addr = areq.Address + "/" + subnetLen
	} else {
		// Make a REST call to master
		var allocResp master.AddressAllocResponse
		err = cluster.MasterPostReq("/plugin/allocAddress", &allocReq, &allocResp)
		if err != nil {
			httpError(w, "master failed to allocate address", err)
			return
		}

		addr = allocResp.IPv4Address
	}

	// build response
	aresp := api.RequestAddressResponse{
		Address: addr,
	}

	log.Infof("Sending RequestAddressResponse: %+v", aresp)

	// build json
	content, err = json.Marshal(aresp)
	if err != nil {
		httpError(w, "Could not generate requestAddress response", err)
		return
	}

	w.Write(content)
}

// releaseAddress
func releaseAddress(w http.ResponseWriter, r *http.Request) {
	var (
		content []byte
		err     error
		areq    = api.ReleaseAddressRequest{}
		decoder = json.NewDecoder(r.Body)
	)

	logEvent("releaseAddress")

	// Decode the JSON message
	err = decoder.Decode(&areq)
	if err != nil {
		httpError(w, "Could not read and parse releaseAddress request", err)
		return
	}

	log.Infof("Received ReleaseAddressRequest: %+v", areq)

	//Build an release request to be sent to master
	releaseReq := master.AddressReleaseRequest{
		NetworkID:   areq.PoolID,
		IPv4Address: areq.Address,
	}
	var releaseResp master.AddressReleaseResponse
	if err = cluster.MasterPostReq("/plugin/releaseAddress",
		&releaseReq, &releaseResp); err != nil {
		httpError(w, "master failed to release request", err)
		return
	}
	// response
	relResp := api.ReleaseAddressResponse{}

	log.Infof("Sending ReleaseAddressResponse: {%+v}", relResp)

	content, err = json.Marshal(relResp)
	if err != nil {
		httpError(w, "Could not generate release addr response", err)
		return
	}

	// Send response
	w.Write(content)
}
