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

package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/netmaster/master"
)

// Client provides the methods for issuing post and get requests to netmaster
type Client struct {
	url   string
	httpC *http.Client
}

// New instantiates a new netmaster client
func New(url string) *Client {
	return &Client{url: url, httpC: http.DefaultClient}
}

func (c *Client) formURL(rsrc string) string {
	return fmt.Sprintf("http://%s/%s", c.url, rsrc)
}

func (c *Client) doPost(rsrc string, cfg *intent.Config) error {
	var (
		body []byte
		err  error
		resp *http.Response
	)

	if body, err = json.Marshal(cfg); err != nil {
		return core.Errorf("json marshalling failed. Error: %s", err)
	}

	if resp, err = c.httpC.Post(c.formURL(rsrc), "application/json", bytes.NewReader(body)); err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return core.Errorf("Response status: %q. Response body: %+v", resp.Status, resp.Body)
	}

	return nil
}

// XXX: we should have a well defined structure for the info that is resturned
func (c *Client) doGet(rsrc string) ([]byte, error) {
	var (
		body []byte
		err  error
		resp *http.Response
	)

	if resp, err = c.httpC.Get(c.formURL(rsrc)); err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Println(rsrc)
		return nil, core.Errorf("Response status: %q. Response Body: %+v", resp.Status, resp.Body)
	}

	if body, err = ioutil.ReadAll(resp.Body); err != nil {
		return nil, err
	}

	return body, nil
}

// PostDesiredConfig posts the net desired configuration to netmaster
func (c *Client) PostDesiredConfig(cfg *intent.Config) error {
	return c.doPost(master.DesiredConfigRESTEndpoint, cfg)
}

// PostAddConfig posts the configuration additions to netmaster
func (c *Client) PostAddConfig(cfg *intent.Config) error {
	return c.doPost(master.AddConfigRESTEndpoint, cfg)
}

// PostDeleteConfig posts the configuration deletions to netmaster
func (c *Client) PostDeleteConfig(cfg *intent.Config) error {
	return c.doPost(master.DelConfigRESTEndpoint, cfg)
}

// PostHostBindings posts the host binding configuration to netmaster
func (c *Client) PostHostBindings(cfg *intent.Config) error {
	return c.doPost(master.HostBindingConfigRESTEndpoint, cfg)
}

// GetEndpoint requests info of a specified endpoint from netmaster
func (c *Client) GetEndpoint(id string) ([]byte, error) {
	return c.doGet(fmt.Sprintf("/%s/%s", master.GetEndpointRESTEndpoint, id))
}

// GetAllEndpoints requests info of all endpoints from netmaster
func (c *Client) GetAllEndpoints() ([]byte, error) {
	return c.doGet(master.GetEndpointsRESTEndpoint)
}

// GetNetwork requests info of a specified network from netmaster
func (c *Client) GetNetwork(id string) ([]byte, error) {
	return c.doGet(fmt.Sprintf("/%s/%s", master.GetNetworkRESTEndpoint, id))
}

// GetAllNetworks requests info of all networks from netmaster
func (c *Client) GetAllNetworks() ([]byte, error) {
	return c.doGet(master.GetNetworksRESTEndpoint)
}
