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
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/mgmtfn/mesosplugin/api"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/netplugin/cluster"
	"github.com/contiv/netplugin/utils/netutils"
)

type endpointParams struct {
	tenant  string
	network string
	group   string
}

func getEndpointParameters(netgroups []string, labels []map[string]string) (*endpointParams, error) {
	var (
		epg     string
		network = "default-net"
		tenant  = "default"
	)

	if len(netgroups) > 1 {
		return nil, fmt.Errorf("only one netgroup can be specified")
	}

	if len(netgroups) == 1 {
		epg = netgroups[0]
	}

	for _, label := range labels {
		k := label["key"]
		v := label["value"]
		if k == "io.contiv.network" {
			network = v
		}

		if k == "io.contiv.tenant" {
			tenant = v
		}

	}

	ep := &endpointParams{
		tenant:  tenant,
		network: network,
		group:   epg,
	}
	return ep, nil
}

// requestAddress
func requestAddress(w http.ResponseWriter, r *http.Request) {
	var (
		content []byte
		err     error
		areq    = api.IPAMRequest{}
		decoder = json.NewDecoder(r.Body)
		ip      string
	)

	logEvent("requestAddress")

	// Decode the JSON message
	err = decoder.Decode(&areq)
	if err != nil {
		httpError(w, "Could not read and parse requestAddress request", err)
		return
	}

	log.Infof("Received RequestAddressRequest: %+v", areq)

	if len(areq.Labels) == 0 {
		httpError(w, "Labels are missing", fmt.Errorf("Labels length is 0"))
	}

	epParams, err := getEndpointParameters(areq.Netgroups, areq.Labels)
	if err != nil {
		httpError(w, "Failed to getEndpointParameters", err)
	}

	if len(areq.IPs) == 1 {
		ip = areq.IPs[0]
	}
	network := fmt.Sprintf("%v.%v", epParams.network, epParams.tenant)
	// Build an alloc request to be sent to master
	allocReq := master.AddressAllocRequest{
		NetworkID:            network,
		PreferredIPv4Address: ip,
	}

	var addr string
	// Make a REST call to master
	var allocResp master.AddressAllocResponse
	err = cluster.MasterPostReq("/plugin/allocAddress", &allocReq, &allocResp)
	if err != nil {
		httpError(w, "master failed to allocate address", err)
		return
	}

	addr = strings.Split(allocResp.IPv4Address, "/")[0]

	allocatedEndpoints.Lock()
	ipAddrValue, found := allocatedEndpoints.ipMap.NextClear(0)
	if !found {
		log.Errorf("failed to allocate bridge IP: %v", nil)
		httpError(w, "failed to created endpoint: couldn't allocate bridge IP address", nil)
	}
	vethIP, err := netutils.GetSubnetIP("10.112.95.0", 24, 32, ipAddrValue)
	allocatedEndpoints.ipMap.Set(ipAddrValue)
	if err != nil {
		log.Errorf("failed to allocate bridge IP: %v", err)
		httpError(w, "failed to created endpoint: couldn't get IP from host id", nil)
	}
	allocatedEndpoints.Unlock()

	// build response
	aresp := api.IPAMResponse{
		IPV4: []string{vethIP, addr},
	}

	log.Infof("Sending RequestAddressResponse: %+v, %+v", aresp, addr)

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
		areq    = api.IPAMRequest{}
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
	relResp := api.IPAMResponse{}

	log.Infof("Sending ReleaseAddressResponse: {%+v}", relResp)

	content, err = json.Marshal(relResp)
	if err != nil {
		httpError(w, "Could not generate release addr response", err)
		return
	}

	// Send response
	w.Write(content)
}
