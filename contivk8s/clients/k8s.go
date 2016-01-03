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
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"io/ioutil"
	"net/http"
)

const (
	nsURL = "/api/v1/namespaces/"
)

// APIClient defines informatio needed for the k8s api client
type APIClient struct {
	baseURL string
	client  *http.Client
}

// NewAPIClient creates an instance of the k8s api client
func NewAPIClient(serverURL, caFile, keyFile, certFile string) *APIClient {
	c := APIClient{}
	c.baseURL = serverURL + "/api/v1/namespaces/"

	// Read client cert
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Fatalf("%s", err)
		return nil
	}

	// Read CA cert
	ca, err := ioutil.ReadFile(caFile)
	if err != nil {
		log.Fatal(err)
		return nil
	}

	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(ca)

	// Setup HTTPS client
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caPool,
	}
	tlsCfg.BuildNameToCertificate()
	transport := &http.Transport{TLSClientConfig: tlsCfg}
	c.client = &http.Client{Transport: transport}

	return &c
}

// GetPodLabel retrieves the value of the specified label from the pod
func (c *APIClient) GetPodLabel(ns, name, label string) (string, error) {
	var data interface{}

	// initiate a get request to the api server
	podURL := c.baseURL + ns + "/pods/" + name
	r, err := c.client.Get(podURL)
	if err != nil {
		return "", err
	}

	defer r.Body.Close()
	switch {
	case r.StatusCode == int(404):
		return "", fmt.Errorf("Page not found!")
	case r.StatusCode == int(403):
		return "", fmt.Errorf("Access denied!")
	case r.StatusCode != int(200):
		log.Errorf("GET Status '%s' status code %d \n", r.Status, r.StatusCode)
		return "", fmt.Errorf("%s", r.Status)
	}

	response, err1 := ioutil.ReadAll(r.Body)
	if err1 != nil {
		return "", err1
	}

	err1 = json.Unmarshal(response, &data)
	if err1 != nil {
		return "", err1
	}

	podSpec := data.(map[string]interface{})
	m, ok := podSpec["metadata"]
	if !ok {
		return "", fmt.Errorf("metadata not found in podSpec")
	}

	meta := m.(map[string]interface{})
	m, ok = meta["labels"]
	if !ok {
		return "", fmt.Errorf("labels not found in podSpec metadata")
	}

	labels := m.(map[string]interface{})
	lval, found := labels[label]
	if !found {
		// this means net-group is not specified
		return "", nil
	}

	res := lval.(string)
	return res, nil
}
