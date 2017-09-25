/***
Copyright 2015 Cisco Systems Inc. All rights reserved.

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

package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/mgmtfn/k8splugin/cniapi"
)

const (
	nwURL = "http://localhost"
)

// NWClient defines informatio needed for the k8s api client
type NWClient struct {
	baseURL string
	client  *http.Client
}

func unixDial(proto, addr string) (conn net.Conn, err error) {
	sock := cniapi.ContivCniSocket
	return net.Dial("unix", sock)
}

// NewNWClient creates an instance of the network driver client
func NewNWClient() *NWClient {
	c := NWClient{}
	c.baseURL = nwURL

	transport := &http.Transport{Dial: unixDial}
	c.client = &http.Client{Transport: transport}

	return &c
}

// AddPod adds a pod to contiv using the cni api
func (c *NWClient) AddPod(podInfo interface{}) (*cniapi.RspAddPod, error) {

	data := cniapi.RspAddPod{}
	buf, err := json.Marshal(podInfo)
	if err != nil {
		return nil, err
	}

	body := bytes.NewBuffer(buf)
	url := c.baseURL + cniapi.EPAddURL
	r, err := c.client.Post(url, "application/json", body)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	switch {
	case r.StatusCode == int(404):
		return nil, fmt.Errorf("page not found")

	case r.StatusCode == int(403):
		return nil, fmt.Errorf("access denied")

	case r.StatusCode == int(500):
		info, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(info, &data)
		if err != nil {
			return nil, err
		}
		return &data, fmt.Errorf("internal server error")

	case r.StatusCode != int(200):
		log.Errorf("POST Status '%s' status code %d \n", r.Status, r.StatusCode)
		return nil, fmt.Errorf("%s", r.Status)
	}

	response, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(response, &data)
	if err != nil {
		return nil, err
	}

	return &data, nil
}

// DelPod deletes a pod from contiv using the cni api
func (c *NWClient) DelPod(podInfo interface{}) error {

	buf, err := json.Marshal(podInfo)
	if err != nil {
		return err
	}

	body := bytes.NewBuffer(buf)
	url := c.baseURL + cniapi.EPDelURL
	r, err := c.client.Post(url, "application/json", body)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	switch {
	case r.StatusCode == int(404):
		return fmt.Errorf("page not found")
	case r.StatusCode == int(403):
		return fmt.Errorf("access denied")
	case r.StatusCode != int(200):
		log.Errorf("GET Status '%s' status code %d \n", r.Status, r.StatusCode)
		return fmt.Errorf("%s", r.Status)
	}

	return nil
}
