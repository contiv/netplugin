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

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/netplugin/utils/netutils"
	"github.com/contiv/netplugin/version"
	"github.com/gorilla/mux"

	log "github.com/Sirupsen/logrus"
)

// get current version
func getVersion(w http.ResponseWriter, r *http.Request) {
	ver := version.Get()

	resp, err := json.Marshal(ver)
	if err != nil {
		http.Error(w,
			core.Errorf("marshaling json failed. Error: %s", err).Error(),
			http.StatusInternalServerError)
		return
	}
	w.Write(resp)
	return
}

// slaveProxyHandler redirects to current master
func slaveProxyHandler(w http.ResponseWriter, r *http.Request) {
	log.Infof("proxy handler for %q ", r.URL.Path)

	localIP, err := netutils.GetDefaultAddr()
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
	url, _ := url.Parse(fmt.Sprintf("http://%s", masterNode))

	// Create a proxy for the URL
	proxy := httputil.NewSingleHostReverseProxy(url)

	// modify the request url
	newReq := *r
	// newReq.URL = url

	// Serve http
	proxy.ServeHTTP(w, &newReq)
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
				core.Errorf("marshaling json failed. Error: %s", err).Error(),
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
