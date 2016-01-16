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
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/netmaster/objApi"
	"github.com/contiv/netplugin/resources"
	"github.com/contiv/netplugin/state"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/objdb"
	"github.com/contiv/objdb/client"
	"github.com/gorilla/mux"
	"github.com/hashicorp/consul/api"
)

type cliOpts struct {
	help       bool
	debug      bool
	stateStore string
	storeURL   string
	listenURL  string
}

type httpAPIFunc func(w http.ResponseWriter, r *http.Request, vars map[string]string) (interface{}, error)

var flagSet *flag.FlagSet

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTION]...\n", os.Args[0])
	flagSet.PrintDefaults()
}

type daemon struct {
	opts          cliOpts
	apiController *objApi.APIController
	stateDriver   core.StateDriver
}

func initStateDriver(opts *cliOpts) (core.StateDriver, error) {
	var cfg *core.Config

	switch opts.stateStore {
	case utils.EtcdNameStr:
		url := "http://127.0.0.1:4001"
		if opts.storeURL != "" {
			url = opts.storeURL
		}
		etcdCfg := &state.EtcdStateDriverConfig{}
		etcdCfg.Etcd.Machines = []string{url}
		cfg = &core.Config{V: etcdCfg}
	case utils.ConsulNameStr:
		url := "http://127.0.0.1:8500"
		if opts.storeURL != "" {
			url = opts.storeURL
		}
		consulCfg := &state.ConsulStateDriverConfig{}
		consulCfg.Consul = api.Config{Address: url}
		cfg = &core.Config{V: consulCfg}
	default:
		return nil, core.Errorf("Unsupported state-store %q", opts.stateStore)
	}

	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	return utils.NewStateDriver(opts.stateStore, string(cfgBytes))
}

func (d *daemon) parseOpts() error {
	flagSet = flag.NewFlagSet("netm", flag.ExitOnError)
	flagSet.BoolVar(&d.opts.help,
		"help",
		false,
		"prints this message")
	flagSet.BoolVar(&d.opts.debug,
		"debug",
		false,
		"Turn on debugging information")
	flagSet.StringVar(&d.opts.stateStore,
		"state-store",
		utils.EtcdNameStr,
		"State store to use")
	flagSet.StringVar(&d.opts.storeURL,
		"store-url",
		"",
		"Etcd or Consul cluster url. Empty string resolves to respective state-store's default URL.")
	flagSet.StringVar(&d.opts.listenURL,
		"listen-url",
		":9999",
		"Url to listen http requests on")

	if err := flagSet.Parse(os.Args[1:]); err != nil {
		return err
	}

	return nil
}

func (d *daemon) registerService() {
	// Create an objdb client
	objdbClient := client.NewClient()

	// Get the address to be used for local communication
	localIP, err := objdbClient.GetLocalAddr()
	if err != nil {
		log.Fatalf("Error getting locla IP address. Err: %v", err)
	}

	// service info
	srvInfo := objdb.ServiceInfo{
		ServiceName: "netmaster",
		HostAddr:    localIP,
		Port:        9999,
	}

	// Register the node with service registry
	err = objdbClient.RegisterService(srvInfo)
	if err != nil {
		log.Fatalf("Error registering service. Err: %v", err)
	}

	log.Infof("Registered netmaster service with registry")
}

func (d *daemon) execOpts() {
	if err := d.parseOpts(); err != nil {
		log.Fatalf("Failed to parse cli options. Error: %s", err)
	}

	if d.opts.help {
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTION]...\n", os.Args[0])
		flagSet.PrintDefaults()
		os.Exit(0)
	}

	if d.opts.debug {
		log.SetLevel(log.DebugLevel)
	}

	sd, err := initStateDriver(&d.opts)
	if err != nil {
		log.Fatalf("Failed to init state-store. Error: %s", err)
	}

	if _, err = resources.NewStateResourceManager(sd); err != nil {
		log.Fatalf("Failed to init resource manager. Error: %s", err)
	}

	d.stateDriver = sd
}

func (d *daemon) ListenAndServe() {
	router := mux.NewRouter()

	// Create a new api controller
	d.apiController = objApi.NewAPIController(router)

	// initialize policy manager
	mastercfg.InitPolicyMgr(d.stateDriver)

	// Register netmaster service
	d.registerService()

	// register web ui handlers
	registerWebuiHandler(router)

	// Add REST routes
	s := router.Headers("Content-Type", "application/json").Methods("Post").Subrouter()
	s.HandleFunc(fmt.Sprintf("/%s", master.DesiredConfigRESTEndpoint),
		post(d.desiredConfig))
	s.HandleFunc(fmt.Sprintf("/%s", master.AddConfigRESTEndpoint),
		post(d.addConfig))
	s.HandleFunc(fmt.Sprintf("/%s", master.DelConfigRESTEndpoint),
		post(d.delConfig))
	s.HandleFunc(fmt.Sprintf("/%s", master.HostBindingConfigRESTEndpoint),
		post(d.hostBindingsConfig))

	s.HandleFunc("/plugin/allocAddress", makeHTTPHandler(master.AllocAddressHandler))
	s.HandleFunc("/plugin/releaseAddress", makeHTTPHandler(master.ReleaseAddressHandler))
	s.HandleFunc("/plugin/createEndpoint", makeHTTPHandler(master.CreateEndpointHandler))
	s.HandleFunc("/plugin/deleteEndpoint", makeHTTPHandler(master.DeleteEndpointHandler))

	s = router.Methods("Get").Subrouter()
	s.HandleFunc(fmt.Sprintf("/%s/%s", master.GetEndpointRESTEndpoint, "{id}"),
		get(false, d.endpoints))
	s.HandleFunc(fmt.Sprintf("/%s", master.GetEndpointsRESTEndpoint),
		get(true, d.endpoints))
	s.HandleFunc(fmt.Sprintf("/%s/%s", master.GetNetworkRESTEndpoint, "{id}"),
		get(false, d.networks))
	s.HandleFunc(fmt.Sprintf("/%s", master.GetNetworksRESTEndpoint),
		get(true, d.networks))

	log.Infof("Netmaster listening on %s", d.opts.listenURL)

	go objApi.CreateDefaultTenant()

	if err := http.ListenAndServe(d.opts.listenURL, router); err != nil {
		log.Fatalf("Error listening for http requests. Error: %s", err)
	}

}

// registerWebuiHandler registers handlers for serving web UI
func registerWebuiHandler(router *mux.Router) {
	// Setup the router to serve the web UI
	goPath := os.Getenv("GOPATH")
	if goPath != "" {
		webPath := goPath + "/src/github.com/contiv/contivmodel/www/"

		// Make sure we have the web UI files
		_, err := os.Stat(webPath)
		if err != nil {
			webPath = goPath + "/src/github.com/contiv/netplugin/" +
				"Godeps/_workspace/src/github.com/contiv/contivmodel/www/"
			_, err := os.Stat(webPath)
			if err != nil {
				log.Errorf("Can not find the web UI directory")
			}
		}

		log.Infof("Using webPath: %s", webPath)

		// serve static files
		router.PathPrefix("/web/").Handler(http.StripPrefix("/web/", http.FileServer(http.Dir(webPath))))

		// Special case to serve main index.html
		router.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
			http.ServeFile(rw, req, webPath+"index.html")
		})
	}

	// proxy Handler
	router.PathPrefix("/proxy/").HandlerFunc(proxyHandler)
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

func post(hook func(cfg *intent.Config) error) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := &intent.Config{}
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(cfg); err != nil {
			http.Error(w,
				core.Errorf("parsing json failed. Error: %s", err).Error(),
				http.StatusInternalServerError)
			return
		}

		if err := hook(cfg); err != nil {
			http.Error(w,
				err.Error(),
				http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		return
	}
}

func (d *daemon) desiredConfig(cfg *intent.Config) error {
	if err := master.DeleteDelta(cfg); err != nil {
		return err
	}

	if err := master.ProcessAdditions(cfg); err != nil {
		return err
	}
	return nil
}

func (d *daemon) addConfig(cfg *intent.Config) error {
	if err := master.ProcessAdditions(cfg); err != nil {
		return err
	}
	return nil
}

func (d *daemon) delConfig(cfg *intent.Config) error {
	if err := master.ProcessDeletions(cfg); err != nil {
		return err
	}
	return nil
}

func (d *daemon) hostBindingsConfig(cfg *intent.Config) error {
	if err := master.CreateEpBindings(&cfg.HostBindings); err != nil {
		return err
	}
	return nil
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

// XXX: This function should be returning logical state instead of driver state
func (d *daemon) endpoints(id string) ([]core.State, error) {
	var (
		err error
		ep  *drivers.OvsOperEndpointState
	)

	ep = &drivers.OvsOperEndpointState{}
	if ep.StateDriver, err = utils.GetStateDriver(); err != nil {
		return nil, err
	}

	if id == "all" {
		eps, err := ep.ReadAll()
		if err != nil {
			return []core.State{}, nil
		}
		return eps, nil
	}

	err = ep.Read(id)
	if err == nil {
		return []core.State{core.State(ep)}, nil
	}

	return nil, core.Errorf("Unexpected code path. Recieved error during read: %v", err)
}

// XXX: This function should be returning logical state instead of driver state
func (d *daemon) networks(id string) ([]core.State, error) {
	var (
		err error
		nw  *mastercfg.CfgNetworkState
	)

	nw = &mastercfg.CfgNetworkState{}
	if nw.StateDriver, err = utils.GetStateDriver(); err != nil {
		return nil, err
	}

	if id == "all" {
		return nw.ReadAll()
	} else if err := nw.Read(id); err == nil {
		return []core.State{core.State(nw)}, nil
	}

	return nil, core.Errorf("Unexpected code path")
}

func main() {
	d := &daemon{}
	d.execOpts()
	d.ListenAndServe()
}
