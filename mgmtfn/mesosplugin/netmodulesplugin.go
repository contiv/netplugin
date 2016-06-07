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

package mesosplugin

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
	"github.com/contiv/netplugin/mgmtfn/mesosplugin/api"
	"github.com/contiv/netplugin/netplugin/plugin"
	"github.com/gorilla/mux"
)

const pluginPath = "/run/"
const driverName = "mesos-netmodules-netplugin"

var netPlugin *plugin.NetPlugin

// InitMesosPlugin initializes the Mesos net-modules plugin
func InitMesosPlugin(netplugin *plugin.NetPlugin) error {
	netPlugin = netplugin
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Could not retrieve hostname: %v", err)
	}

	// TODO: remove this
	log.Debugf("hostname is: %v", hostname)

	log.Debugf("Configuring router")

	router := mux.NewRouter()
	s := router.Methods("POST").Subrouter()

	dispatchMap := map[string]func(http.ResponseWriter, *http.Request){
		"/Plugin.Allocate": requestAddress,
		"/Plugin.Release":  releaseAddress,
		"/Plugin.Isolate":  join,
		"/Plugin.Cleanup":  leave,
	}

	for dispatchPath, dispatchFunc := range dispatchMap {
		s.HandleFunc(dispatchPath, logHandler(dispatchPath, dispatchFunc))
	}

	s.HandleFunc("/Plugin.{*}", unknownAction)

	driverPath := path.Join(pluginPath, driverName) + ".sock"
	os.Remove(driverPath)
	os.MkdirAll(pluginPath, 0700)

	go func() {
		l, err := net.ListenUnix("unix", &net.UnixAddr{Name: driverPath, Net: "unix"})
		if err != nil {
			panic(err)
		}

		log.Infof("mesos net-modules netplugin API listening on %s", driverPath)
		http.Serve(l, router)
		l.Close()
		log.Infof("mesos net-modules netplugin API listener closing %s", driverPath)
	}()
	go func() {
		srv := &http.Server{
			Handler: router,
		}

		srv.ListenAndServe()

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

	content, errc := json.Marshal(api.VirtualizerResponse{Error: fullError})
	if errc != nil {
		log.Warnf("Error received marshalling error response: %v, original error: %s", errc, fullError)
		return
	}

	log.Errorf("Returning HTTP error handling plugin negotiation: %s", fullError)
	http.Error(w, string(content), http.StatusInternalServerError)
}

func logEvent(typ string) {
	log.Infof("Handling %q event", typ)
}

// Catch all for additional driver functions.
func unknownAction(w http.ResponseWriter, r *http.Request) {
	log.Infof("Unknown networkdriver action at %q", r.URL.Path)
	content, _ := ioutil.ReadAll(r.Body)
	log.Infof("Body content: %s", string(content))
	w.WriteHeader(503)
}
