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

package daemon

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/netplugin/utils/netutils"
	"github.com/contiv/netplugin/version"
	"github.com/gorilla/mux"

	log "github.com/Sirupsen/logrus"
)

type httpAPIFunc func(w http.ResponseWriter, r *http.Request, vars map[string]string) (interface{}, error)

// get current version
func getVersion(w http.ResponseWriter, r *http.Request) {
	ver := version.Get()

	resp, err := json.Marshal(ver)
	if err != nil {
		http.Error(w,
			core.Errorf("marshalling json failed. Error: %s", err).Error(),
			http.StatusInternalServerError)
		return
	}
	w.Write(resp)
	return
}

// initStateDriver creates a state driver based on the cluster store URL
func initStateDriver(clusterStore string) (core.StateDriver, error) {
	// parse the state store URL
	parts := strings.Split(clusterStore, "://")
	if len(parts) < 2 {
		return nil, core.Errorf("Invalid state-store URL %q", clusterStore)
	}
	stateStore := parts[0]

	// Make sure we support the statestore type
	switch stateStore {
	case utils.EtcdNameStr:
	case utils.ConsulNameStr:
	default:
		return nil, core.Errorf("Unsupported state-store %q", stateStore)
	}

	// Setup instance info
	instInfo := core.InstanceInfo{
		DbURL: clusterStore,
	}

	return utils.NewStateDriver(stateStore, &instInfo)
}

// getLocalAddr gets local address to be used
func getLocalAddr() (string, error) {
	// get the ip address by local hostname
	localIP, err := netutils.GetMyAddr()
	if err == nil && netutils.IsAddrLocal(localIP) {
		return localIP, nil
	}

	// Return first available address if we could not find by hostname
	return netutils.GetFirstLocalAddr()
}

// slaveProxyHandler redirects to current master
func slaveProxyHandler(w http.ResponseWriter, r *http.Request) {
	log.Infof("proxy handler for %q ", r.URL.Path)

	localIP, err := getLocalAddr()
	if err != nil {
		log.Fatalf("Error getting local IP address. Err: %v", err)
	}

	// get current holder of master lock
	masterNode := leaderLock.GetHolder()
	if masterNode == "" {
		http.Error(w, "Leader not found", http.StatusInternalServerError)
		return
	}

	// If we are the master, return
	if localIP == masterNode {
		http.Error(w, "Self proxying error", http.StatusInternalServerError)
		return
	}

	// build the proxy url
	url, _ := url.Parse(fmt.Sprintf("http://%s:9999", masterNode))

	// Create a proxy for the URL
	proxy := httputil.NewSingleHostReverseProxy(url)

	// modify the request url
	newReq := *r
	// newReq.URL = url

	// Serve http
	proxy.ServeHTTP(w, &newReq)
}

// Simple Wrapper for http handlers
func makeHTTPHandler(handlerFunc httpAPIFunc) http.HandlerFunc {
	// Create a closure and return an anonymous function
	return func(w http.ResponseWriter, r *http.Request) {
		// Call the handler
		resp, err := handlerFunc(w, r, mux.Vars(r))
		if err != nil {
			// Log error
			log.Errorf("Handler for %s %s returned error: %s", r.Method, r.URL, err)

			// Send HTTP response
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			// Send HTTP response as Json
			err = writeJSON(w, http.StatusOK, resp)
			if err != nil {
				log.Errorf("Error generating json. Err: %v", err)
			}
		}
	}
}

// writeJSON: writes the value v to the http response stream as json with standard
// json encoding.
func writeJSON(w http.ResponseWriter, code int, v interface{}) error {
	// Set content type as json
	w.Header().Set("Content-Type", "application/json")

	// write the HTTP status code
	w.WriteHeader(code)

	// Write the Json output
	return json.NewEncoder(w).Encode(v)
}

func get(getAll bool, hook func(id string) ([]core.State, error)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			idStr  string
			states []core.State
			resp   []byte
			ok     bool
			err    error
		)

		if getAll {
			idStr = "all"
		} else if idStr, ok = mux.Vars(r)["id"]; !ok {
			http.Error(w,
				core.Errorf("Failed to find the id string in the request.").Error(),
				http.StatusInternalServerError)
		}

		if states, err = hook(idStr); err != nil {
			http.Error(w,
				err.Error(),
				http.StatusInternalServerError)
			return
		}

		if resp, err = json.Marshal(states); err != nil {
			http.Error(w,
				core.Errorf("marshalling json failed. Error: %s", err).Error(),
				http.StatusInternalServerError)
			return
		}

		w.Write(resp)
		return
	}
}

// services: This function should be returning logical state instead of driver state
func (d *MasterDaemon) services(id string) ([]core.State, error) {
	var (
		err error
		svc *mastercfg.CfgServiceLBState
	)

	svc = &mastercfg.CfgServiceLBState{}
	if svc.StateDriver, err = utils.GetStateDriver(); err != nil {
		return nil, err
	}

	if id == "all" {
		return svc.ReadAll()
	} else if err := svc.Read(id); err == nil {
		return []core.State{core.State(svc)}, nil
	}

	return nil, err
}
