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
	baseURL  string
	client   *http.Client
	podCache podInfo
}

type podInfo struct {
	nameSpace string
	name      string
	labels    map[string]string
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

	p := &c.podCache
	p.labels = make(map[string]string)
	p.nameSpace = ""
	p.name = ""

	return &c
}

func (p *podInfo) setDefaults(ns, name string) {
	p.nameSpace = ns
	p.name = name
	p.labels["tenant"] = "default"
	p.labels["network"] = "default-net"
	p.labels["net-group"] = ""
}

// fetchPodLabels retrieves the labels from the podspec metadata
func (c *APIClient) fetchPodLabels(ns, name string) error {
	var data interface{}

	// initiate a get request to the api server
	podURL := c.baseURL + ns + "/pods/" + name
	r, err := c.client.Get(podURL)
	if err != nil {
		return err
	}

	defer r.Body.Close()
	switch {
	case r.StatusCode == int(404):
		return fmt.Errorf("Page not found!")
	case r.StatusCode == int(403):
		return fmt.Errorf("Access denied!")
	case r.StatusCode != int(200):
		log.Errorf("GET Status '%s' status code %d \n", r.Status, r.StatusCode)
		return fmt.Errorf("%s", r.Status)
	}

	response, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(response, &data)
	if err != nil {
		return err
	}

	podSpec := data.(map[string]interface{})
	m, ok := podSpec["metadata"]
	// Treat missing metadata as a fatal error
	if !ok {
		return fmt.Errorf("metadata not found in podSpec")
	}

	p := &c.podCache
	p.setDefaults(ns, name)

	meta := m.(map[string]interface{})
	l, ok := meta["labels"]
	if ok {
		labels := l.(map[string]interface{})
		for key, val := range labels {
			switch valType := val.(type) {

			case string:
				p.labels[key] = val.(string)

			default:
				log.Infof("Label %s type %v in pod %s.%s ignored",
					key, valType, ns, name)
			}
		}
	} else {
		log.Infof("labels not found in podSpec metadata, using defaults")
	}

	return nil
}

// GetPodLabel retrieves the specified label
func (c *APIClient) GetPodLabel(ns, name, label string) (string, error) {

	// If cache does not match, fetch
	if c.podCache.nameSpace != ns || c.podCache.name != name {
		err := c.fetchPodLabels(ns, name)
		if err != nil {
			return "", err
		}
	}

	res, found := c.podCache.labels[label]
	if found {
		return res, nil
	}

	log.Infof("label %s not found in podSpec for %s.%s", label, ns, name)
	return "", nil
}
