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

package dockplugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/netplugin/plugin"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/libnetwork/drivers/remote/api"
	"github.com/gorilla/mux"
)

const pluginPath = "/run/docker/plugins"
const driverName = "netplugin"

var netPlugin *plugin.NetPlugin
var pluginMode string

// InitDockPlugin initializes the docker plugin
func InitDockPlugin(np *plugin.NetPlugin, mode string) error {
	// Save state
	netPlugin = np
	pluginMode = mode

	// Get local hostname
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Could not retrieve hostname: %v", err)
	}

	log.Debugf("Configuring router")

	router := mux.NewRouter()
	s := router.Methods("POST").Subrouter()

	dispatchMap := map[string]func(http.ResponseWriter, *http.Request){
		"/Plugin.Activate":                           activate(hostname),
		"/Plugin.Deactivate":                         deactivate(hostname),
		"/NetworkDriver.GetCapabilities":             getCapability,
		"/NetworkDriver.CreateNetwork":               createNetwork,
		"/NetworkDriver.DeleteNetwork":               deleteNetwork,
		"/NetworkDriver.CreateEndpoint":              createEndpoint(hostname),
		"/NetworkDriver.DeleteEndpoint":              deleteEndpoint(hostname),
		"/NetworkDriver.EndpointOperInfo":            endpointInfo,
		"/NetworkDriver.Join":                        join,
		"/NetworkDriver.Leave":                       leave,
		"/NetworkDriver.AllocateNetwork":             allocateNetwork,
		"/NetworkDriver.FreeNetwork":                 freeNetwork,
		"/NetworkDriver.ProgramExternalConnectivity": programExternalConnectivity,
		"/NetworkDriver.RevokeExternalConnectivity":  revokeExternalConnectivity,
		"/NetworkDriver.DiscoverNew":                 discoverNew,
		"/NetworkDriver.DiscoverDelete":              discoverDelete,
		"/IpamDriver.GetDefaultAddressSpaces":        getDefaultAddressSpaces,
		"/IpamDriver.RequestPool":                    requestPool,
		"/IpamDriver.ReleasePool":                    releasePool,
		"/IpamDriver.RequestAddress":                 requestAddress,
		"/IpamDriver.ReleaseAddress":                 releaseAddress,
		"/IpamDriver.GetCapabilities":                getIpamCapability,
	}

	for dispatchPath, dispatchFunc := range dispatchMap {
		s.HandleFunc(dispatchPath, logHandler(dispatchPath, dispatchFunc))
	}

	s.HandleFunc("/NetworkDriver.{*}", unknownAction)
	s.HandleFunc("/IpamDriver.{*}", unknownAction)

	driverPath := path.Join(pluginPath, driverName) + ".sock"
	os.Remove(driverPath)
	os.MkdirAll(pluginPath, 0700)

	go func() {
		l, err := net.ListenUnix("unix", &net.UnixAddr{Name: driverPath, Net: "unix"})
		if err != nil {
			panic(err)
		}

		log.Infof("docker plugin listening on %s", driverPath)
		server := &http.Server{Handler: router}
		server.SetKeepAlivesEnabled(false)
		server.Serve(l)
		l.Close()
		log.Infof("docker plugin closing %s", driverPath)
	}()

	return nil
}

func logHandler(name string, actionFunc func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		// Debug logs
		buf := new(bytes.Buffer)
		io.Copy(buf, r.Body)
		log.Debugf("Dispatching %s with %v", name, strings.TrimSpace(string(buf.Bytes())))
		var writer *io.PipeWriter
		r.Body, writer = io.Pipe()
		go func() {
			io.Copy(writer, buf)
			writer.Close()
		}()

		// Perform the action
		actionFunc(w, r)
	}
}

func httpError(w http.ResponseWriter, message string, err error) {
	fullError := fmt.Sprintf("%s %v", message, err)

	content, errc := json.Marshal(api.Response{Err: fullError})
	if errc != nil {
		log.Warnf("Error received marshaling error response: %v, original error: %s", errc, fullError)
		return
	}
	w.Write(content)

	log.Errorf("Returning HTTP error handling plugin negotiation: %s", fullError)
	http.Error(w, string(content), http.StatusInternalServerError)
}

func logEvent(typ string) {
	log.Infof("Handling %q event from libnetwork", typ)
}

// Catchall for additional driver functions.
func unknownAction(w http.ResponseWriter, r *http.Request) {
	log.Infof("Unknown networkdriver action at %q", r.URL.Path)
	content, _ := ioutil.ReadAll(r.Body)
	log.Infof("Body content: %s", string(content))
	http.NotFound(w, r)
}

// deactivate the plugin
func deactivate(hostname string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logEvent("deactivate")
	}
}

// activate the plugin and register it as a network driver.
func activate(hostname string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logEvent("activate")

		content, err := json.Marshal(plugins.Manifest{Implements: []string{"NetworkDriver", "IpamDriver"}})
		if err != nil {
			httpError(w, "Could not generate bootstrap response", err)
			return
		}

		w.Write(content)
	}
}
