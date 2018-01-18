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
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/core"
	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
	"strconv"
)

const (
	nsURL = "/api/v1/namespaces/"
)

// APIClient defines information needed for the k8s api client
type APIClient struct {
	apiServerPort uint16
	baseURL       string
	watchBase     string
	client        *http.Client
	podCache      podInfo
	authToken     string
}

// SvcWatchResp is the response to a service watch
type SvcWatchResp struct {
	opcode  string
	errStr  string
	svcName string
	svcSpec core.ServiceSpec
}

// EpWatchResp is the response to service endpoints watch
type EpWatchResp struct {
	opcode    string
	errStr    string
	svcName   string
	providers []string
}

type watchSvcStatus struct {
	// The type of watch update contained in the message
	Type string `json:"type"`
	// Pod details
	Object Service `json:"object"`
}

type watchSvcEpStatus struct {
	// The type of watch update contained in the message
	Type string `json:"type"`
	// Pod details
	Object Endpoints `json:"object"`
}

type podInfo struct {
	nameSpace   string
	name        string
	labels      map[string]string
	labelsMutex sync.Mutex
}

// NewAPIClient creates an instance of the k8s api client
func NewAPIClient(serverURL, caFile, keyFile, certFile, authToken string) *APIClient {
	useClientCerts := true
	c := APIClient{}

	c.apiServerPort = 6443 // default
	port := strings.Split(serverURL, ":")
	if len(port) > 0 {
		if v, err := strconv.ParseUint(port[len(port)-1], 10, 16); err == nil {
			c.apiServerPort = uint16(v)
		} else {
			log.Warnf("parse failed: %s, use default api server port: %d", err, c.apiServerPort)
		}
	}

	c.baseURL = serverURL + "/api/v1/namespaces/"
	c.watchBase = serverURL + "/api/v1/watch/"

	// Read CA cert
	ca, err := ioutil.ReadFile(caFile)
	if err != nil {
		log.Fatal(err)
		return nil
	}

	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(ca)

	// Read client cert
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	// We need either the client certs or a non-empty authToken to proceed
	if err != nil {
		// We cannot use client certs now
		useClientCerts = false
		// Check for a non-empty token
		if len(strings.TrimSpace(authToken)) == 0 {
			log.Fatalf("Error %s loading the client certificates and missing auth token", err)
			return nil
		}
	}
	// Setup HTTPS client
	tlsCfg := &tls.Config{
		RootCAs: caPool,
	}
	// Setup client cert based authentication
	if useClientCerts {
		tlsCfg.Certificates = []tls.Certificate{cert}
		tlsCfg.BuildNameToCertificate()
	}
	transport := &http.Transport{TLSClientConfig: tlsCfg}
	c.client = &http.Client{Transport: transport}
	c.authToken = authToken

	p := &c.podCache
	p.labels = make(map[string]string)
	p.nameSpace = ""
	p.name = ""

	return &c
}

func (p *podInfo) setDefaults(ns, name string) {
	p.nameSpace = ns
	p.name = name
	p.labels["io.contiv.tenant"] = "default"
	p.labels["io.contiv.network"] = "default-net"
	p.labels["io.contiv.net-group"] = ""
}

// fetchPodLabels retrieves the labels from the podspec metadata
func (c *APIClient) fetchPodLabels(ns, name string) error {
	var data interface{}

	// initiate a get request to the api server
	podURL := c.baseURL + ns + "/pods/" + name
	req, err := http.NewRequest("GET", podURL, nil)
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(c.authToken)) > 0 {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.authToken))
	}
	r, err := c.client.Do(req)
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
	p.labelsMutex.Lock()
	defer p.labelsMutex.Unlock()
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

	c.podCache.labelsMutex.Lock()
	defer c.podCache.labelsMutex.Unlock()

	res, found := c.podCache.labels[label]

	if found {
		return res, nil
	}

	log.Infof("label %s not found in podSpec for %s.%s", label, ns, name)
	return "", nil
}

// WatchServices watches the services object on the api server
func (c *APIClient) WatchServices(respCh chan SvcWatchResp) {
	ctx, _ := context.WithCancel(context.Background())

	go func() {
		// Make request to Kubernetes API
		getURL := c.watchBase + "services"
		req, err := http.NewRequest("GET", getURL, nil)
		if err != nil {
			respCh <- SvcWatchResp{opcode: "FATAL", errStr: fmt.Sprintf("Req %v", err)}
			return
		}
		if len(strings.TrimSpace(c.authToken)) > 0 {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.authToken))
		}
		res, err := ctxhttp.Do(ctx, c.client, req)
		if err != nil {
			log.Errorf("Watch error: %v", err)
			respCh <- SvcWatchResp{opcode: "FATAL", errStr: fmt.Sprintf("Do %v", err)}
			return
		}
		defer res.Body.Close()

		var wss watchSvcStatus
		reader := bufio.NewReader(res.Body)

		// bufio.Reader.ReadBytes is blocking, so we watch for
		// context timeout or cancellation in a goroutine
		// and close the response body when see see it. The
		// response body is also closed via defer when the
		// request is made, but closing twice is OK.
		go func() {
			<-ctx.Done()
			res.Body.Close()
		}()

		for {
			line, err := reader.ReadBytes('\n')
			if ctx.Err() != nil {
				respCh <- SvcWatchResp{opcode: "ERROR", errStr: fmt.Sprintf("ctx %v", err)}
				return
			}
			if err != nil {
				respCh <- SvcWatchResp{opcode: "ERROR", errStr: fmt.Sprintf("read %v", err)}
				return
			}
			if err := json.Unmarshal(line, &wss); err != nil {
				respCh <- SvcWatchResp{opcode: "WARN", errStr: fmt.Sprintf("unmarshal %v", err)}
				continue
			}

			if wss.Object.ObjectMeta.Namespace == "kube-system" && (wss.Object.ObjectMeta.Name == "kube-scheduler" || wss.Object.ObjectMeta.Name == "kube-controller-manager") {
				// Ignoring these frequent updates
				continue
			}

			resp := SvcWatchResp{opcode: wss.Type}
			resp.svcName = wss.Object.ObjectMeta.Name
			sSpec := core.ServiceSpec{}
			sSpec.Ports = make([]core.PortSpec, 0, 1)
			sSpec.IPAddress = wss.Object.Spec.ClusterIP
			sSpec.ExternalIPs = wss.Object.Spec.ExternalIPs
			for _, port := range wss.Object.Spec.Ports {
				ps := core.PortSpec{Protocol: string(port.Protocol),
					SvcPort:  uint16(port.Port),
					NodePort: uint16(port.NodePort),
				}

				// handle 'kubernetes' service
				// Use port from configuration till named ports are supported.
				if resp.svcName == "kubernetes" && (len(wss.Object.ObjectMeta.Namespace) == 0 ||
					wss.Object.ObjectMeta.Namespace == "default") {
					ps.ProvPort = c.apiServerPort
				} else {
					ps.ProvPort = uint16(port.TargetPort)
				}
				sSpec.Ports = append(sSpec.Ports, ps)
			}

			resp.svcSpec = sSpec
			log.Infof("resp: %+v", resp)

			respCh <- resp
		}
	}()
}

// WatchSvcEps watches the service endpoints object
func (c *APIClient) WatchSvcEps(respCh chan EpWatchResp) {
	ctx, _ := context.WithCancel(context.Background())

	go func() {
		// Make request to Kubernetes API
		getURL := c.watchBase + "endpoints"
		req, err := http.NewRequest("GET", getURL, nil)
		if err != nil {
			respCh <- EpWatchResp{opcode: "FATAL", errStr: fmt.Sprintf("Req %v", err)}
			return
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.authToken))
		res, err := ctxhttp.Do(ctx, c.client, req)
		if err != nil {
			log.Errorf("EP Watch error: %v", err)
			respCh <- EpWatchResp{opcode: "FATAL", errStr: fmt.Sprintf("Do %v", err)}
			return
		}
		defer res.Body.Close()

		var weps watchSvcEpStatus
		reader := bufio.NewReader(res.Body)

		// bufio.Reader.ReadBytes is blocking, so we watch for
		// context timeout or cancellation in a goroutine
		// and close the response body when see see it. The
		// response body is also closed via defer when the
		// request is made, but closing twice is OK.
		go func() {
			<-ctx.Done()
			res.Body.Close()
		}()

		for {
			line, err := reader.ReadBytes('\n')
			if ctx.Err() != nil {
				respCh <- EpWatchResp{opcode: "ERROR", errStr: fmt.Sprintf("ctx %v", err)}
				return
			}
			if err != nil {
				respCh <- EpWatchResp{opcode: "ERROR", errStr: fmt.Sprintf("read %v", err)}
				return
			}
			if err := json.Unmarshal(line, &weps); err != nil {
				respCh <- EpWatchResp{opcode: "WARN", errStr: fmt.Sprintf("unmarshal %v", err)}
				continue
			}

			if weps.Object.ObjectMeta.Namespace == "kube-system" && (weps.Object.ObjectMeta.Name == "kube-scheduler" || weps.Object.ObjectMeta.Name == "kube-controller-manager") {
				// Ignoring these frequent updates
				continue
			}

			resp := EpWatchResp{opcode: weps.Type}
			resp.svcName = weps.Object.ObjectMeta.Name
			resp.providers = make([]string, 0, 1)
			for _, subset := range weps.Object.Subsets {
				// TODO: handle partially ready providers
				for _, addr := range subset.Addresses {
					resp.providers = append(resp.providers, addr.IP)
				}
			}

			log.Infof("kube ep watch: %v", resp)
			respCh <- resp
		}
	}()
}
