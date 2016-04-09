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
	"io/ioutil"
	"net/http"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/netplugin/cluster"
	"github.com/docker/libnetwork/ipams/remote/api"
)

// getIpamCapability
func getIpamCapability(w http.ResponseWriter, r *http.Request) {
	logEvent("getIpamCapability")

	content, err := json.Marshal(api.GetCapabilityResponse{RequiresMACAddress: true})
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
	if tenant, ok := preq.Options["tenant"]; ok {
		PoolID = preq.Pool + ":" + tenant
	}
	presp := api.RequestPoolResponse{
		PoolID: PoolID,
		Pool:   preq.Pool,
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

	// Build an alloc request to be sent to master
	allocReq := master.AddressAllocRequest{
		AddressPool:          areq.PoolID,
		PreferredIPv4Address: areq.Address,
	}

	//If docker enginer is 1.10+version PoolID will also have
	subnetLen := strings.Split(areq.PoolID, "/")[1]
	if strings.Contains(areq.PoolID, ":") {
		subnetLen = strings.Split(subnetLen, ":")[0]
	}

	var addr string
	if areq.Address != "" {
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
