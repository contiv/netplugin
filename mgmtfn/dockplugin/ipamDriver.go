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

// getDefaultAddressSpaces
func getDefaultAddressSpaces() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
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
}

// requestPool
func requestPool() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logEvent("requestPool")

		// Read the message
		content, err := ioutil.ReadAll(r.Body)
		if err != nil {
			httpError(w, "Could not read requestPool request", err)
			return
		}

		// parse the json
		preq := api.RequestPoolRequest{}
		if err := json.Unmarshal(content, &preq); err != nil {
			httpError(w, "Could not parse requestPool request", err)
			return
		}

		log.Infof("Received RequestPoolRequest: %+v", preq)

		// build response
		presp := api.RequestPoolResponse{
			PoolID: preq.Pool,
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
}

// releasePool
func releasePool() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logEvent("releasePool")

		// Read the message
		content, err := ioutil.ReadAll(r.Body)
		if err != nil {
			httpError(w, "Could not read releasePool request", err)
			return
		}

		// parse the json
		preq := api.ReleasePoolRequest{}
		if err := json.Unmarshal(content, &preq); err != nil {
			httpError(w, "Could not parse releasePool request", err)
			return
		}

		log.Infof("Received ReleasePoolRequest: %+v", preq)

		w.WriteHeader(200)
	}
}

// requestAddress
func requestAddress() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logEvent("requestAddress")

		// Read the message
		content, err := ioutil.ReadAll(r.Body)
		if err != nil {
			httpError(w, "Could not read requestAddress request", err)
			return
		}

		// parse the json
		areq := api.RequestAddressRequest{}
		if err := json.Unmarshal(content, &areq); err != nil {
			httpError(w, "Could not parse requestAddress request", err)
			return
		}

		log.Infof("Received RequestAddressRequest: %+v", areq)

		// Build an alloc request to be sent to master
		allocReq := master.AddressAllocRequest{
			NetworkID:            areq.PoolID,
			PreferredIPv4Address: areq.Address,
		}

		subnetLen := strings.Split(areq.PoolID, "/")[1]

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
}

// releaseAddress
func releaseAddress() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logEvent("releaseAddress")

		// Read the message
		content, err := ioutil.ReadAll(r.Body)
		if err != nil {
			httpError(w, "Could not read releaseAddress request", err)
			return
		}

		// parse the json
		preq := api.ReleaseAddressRequest{}
		if err := json.Unmarshal(content, &preq); err != nil {
			httpError(w, "Could not parse releaseAddress request", err)
			return
		}

		log.Infof("Received ReleaseAddressRequest: %+v", preq)

		w.WriteHeader(200)
	}
}
