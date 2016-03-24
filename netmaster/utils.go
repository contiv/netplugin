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

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/contiv/netplugin/core"
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

// proxyHandler acts as a simple reverse proxy to access containers via http
func proxyHandler(w http.ResponseWriter, r *http.Request) {
	proxyURL := strings.TrimPrefix(r.URL.Path, "/proxy/")
	log.Infof("proxy handler for %q : %s", r.URL.Path, proxyURL)

	// build the proxy url
	url, _ := url.Parse("http://" + proxyURL)

	// Create a proxy for the URL
	proxy := httputil.NewSingleHostReverseProxy(url)

	// modify the request url
	newReq := *r
	newReq.URL = url

	log.Debugf("Proxying request(%v): %+v", url, newReq)

	// Serve http
	proxy.ServeHTTP(w, &newReq)
}

// slaveProxyHandler redirects to current master
func slaveProxyHandler(w http.ResponseWriter, r *http.Request) {
	log.Infof("proxy handler for %q ", r.URL.Path)

	// get current holder of master lock
	masterNode := leaderLock.GetHolder()

	// build the proxy url
	url, _ := url.Parse(fmt.Sprintf("http://%s:9999", masterNode))

	// Create a proxy for the URL
	proxy := httputil.NewSingleHostReverseProxy(url)

	// modify the request url
	newReq := *r
	// newReq.URL = url

	log.Infof("Proxying request(%v): %+v", url, newReq)

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
