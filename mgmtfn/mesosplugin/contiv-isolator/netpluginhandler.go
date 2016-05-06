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

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/contiv/netplugin/mgmtfn/mesosplugin/api"
)

const driverPath = "/run/mesos-netmodules-netplugin.sock"

var client *http.Client

func init() {
	tr := &http.Transport{
		Dial: fakeDial,
	}
	client = &http.Client{Transport: tr}
}

func fakeDial(proto, addr string) (conn net.Conn, err error) {
	return net.DialTimeout("unix", driverPath, time.Second*5)
}

func makeGenericRequest(data []byte, endpoint string) (*http.Response, error) {
	buf := bytes.NewBuffer(data)
	hostname, err := os.Hostname()

	reqURL := fmt.Sprintf("http://%v/%v", hostname, endpoint)
	resp, err := client.Post(reqURL, "application/contiv.io.netplugin.netmodules.v0.1+json", buf)

	if err != nil {
		return nil, fmt.Errorf("encountered error %v", err)
	}

	return resp, nil
}

func makeIPAMRequest(req *api.IPAMRequest, endpoint string) (*api.IPAMResponse, error) {
	var ipamResponse api.IPAMResponse
	reqData, err := json.Marshal(&req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Allocate request data: %v", err)
	}

	resp, err := makeGenericRequest(reqData, endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)

	if err := dec.Decode(&ipamResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal IPAMResponse: %v", err)
	}

	return &ipamResponse, nil

}

func makeVirtualizerRequest(req *api.VirtualizerRequest, endpoint string) (*api.VirtualizerResponse, error) {
	var virtualizerResponse api.VirtualizerResponse
	reqData, err := json.Marshal(&req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Allocate request data: %v", err)
	}

	resp, err := makeGenericRequest(reqData, endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)

	if err := dec.Decode(&virtualizerResponse); err != nil {
		return nil, err
	}

	return &virtualizerResponse, nil
}

type NetpluginHandler struct{}

func (h *NetpluginHandler) Allocate(req *api.IPAMRequest) ([]byte, error) {
	response, err := makeIPAMRequest(req, "Plugin.Allocate")
	if err != nil {
		return nil, err
	}

	data, err := api.EncodeIPAMResponse(response)
	if err != nil {
		errorMessage := fmt.Sprintf("encountered error while encoding response: %v", err)
		data, _ = api.BuildErrorResponse(errorMessage)
		return data, err
	}

	return data, nil
}

func (h *NetpluginHandler) Release(req *api.IPAMRequest) ([]byte, error) {
	response, err := makeIPAMRequest(req, "Plugin.Release")
	if err != nil {
		return nil, err
	}
	data, err := api.EncodeIPAMResponse(response)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (h *NetpluginHandler) Isolate(req *api.VirtualizerRequest) ([]byte, error) {
	response, err := makeVirtualizerRequest(req, "Plugin.Isolate")
	if err != nil {
		return nil, err
	}

	data, err := api.EncodeVirtualizerResponse(response)
	if err != nil {
		errorMessage := fmt.Sprintf("encountered error while encoding response: %v", err)
		data, _ = api.BuildErrorResponse(errorMessage)
		return data, err
	}

	return data, nil
}

func (h *NetpluginHandler) Cleanup(req *api.VirtualizerRequest) ([]byte, error) {
	response, err := makeVirtualizerRequest(req, "Plugin.Cleanup")
	if err != nil {
		return nil, err
	}

	data, err := api.EncodeVirtualizerResponse(response)
	if err != nil {
		errorMessage := fmt.Sprintf("encountered error while encoding response: %v", err)
		data, _ = api.BuildErrorResponse(errorMessage)
		return data, err
	}

	return data, nil
}
