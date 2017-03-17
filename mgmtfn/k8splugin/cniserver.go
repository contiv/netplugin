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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/mgmtfn/k8splugin/cniapi"
	"github.com/contiv/netplugin/netplugin/plugin"
	"github.com/gorilla/mux"
)

// contivKubeCfgFile holds credentials to access k8s api server
const (
	contivKubeCfgFile = "/opt/contiv/config/contiv.json"
	defSvcSubnet      = "10.254.0.0/16"
	tokenFile         = "/var/run/secrets/kubernetes.io/serviceaccount/token"
)

// ContivConfig holds information passed via config file during cluster set up
type ContivConfig struct {
	K8sAPIServer string `json:"K8S_API_SERVER,omitempty"`
	K8sCa        string `json:"K8S_CA,omitempty"`
	K8sKey       string `json:"K8S_KEY,omitempty"`
	K8sCert      string `json:"K8S_CERT,omitempty"`
	K8sToken     string `json:"K8S_TOKEN,omitempty"`
	SvcSubnet    string `json:"SVC_SUBNET,omitempty"`
}

type restAPIFunc func(r *http.Request) (interface{}, error)

var netPlugin *plugin.NetPlugin
var kubeAPIClient *APIClient
var pluginHost string
var contivK8Config ContivConfig

// getConfig reads and parses the contivKubeCfgFile
func getConfig(cfgFile string, pCfg *ContivConfig) error {
	bytes, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		return err
	}

	pCfg.SvcSubnet = defSvcSubnet
	err = json.Unmarshal(bytes, pCfg)
	if err != nil {
		return fmt.Errorf("Error parsing config file: %s", err)
	}

	return nil
}

// getDefaultToken gets the token to access kubernetes API Server
// from the secrets loaded on the container
func getDefaultToken() (string, error) {
	bytes, err := ioutil.ReadFile(tokenFile)
	if err != nil {
		log.Errorf("Failed: %v", err)
		return "", err
	}
	return string(bytes), nil
}

// setUpAPIClient sets up an instance of the k8s api server
func setUpAPIClient() *APIClient {
	// Read config
	err := getConfig(contivKubeCfgFile, &contivK8Config)
	if err != nil {
		log.Errorf("Failed: %v", err)
		return nil
	}
	// If no client certs or token is specified, get the default token
	if len(strings.TrimSpace(contivK8Config.K8sCert)) == 0 && len(strings.TrimSpace(contivK8Config.K8sToken)) == 0 {
		contivK8Config.K8sToken, err = getDefaultToken()
		if err != nil {
			log.Errorf("Failed: %v", err)
			return nil
		}
	}

	return NewAPIClient(contivK8Config.K8sAPIServer, contivK8Config.K8sCa,
		contivK8Config.K8sKey, contivK8Config.K8sCert, contivK8Config.K8sToken)

}

// Simple Wrapper for http handlers
func makeHTTPHandler(handlerFunc restAPIFunc) http.HandlerFunc {
	// Create a closure and return an anonymous function
	return func(w http.ResponseWriter, r *http.Request) {
		// Call the handler
		resp, err := handlerFunc(r)
		if err != nil {
			// Log error
			log.Errorf("Handler for %s %s returned error: %s", r.Method, r.URL, err)

			if resp == nil {
				// Send HTTP response
				http.Error(w, err.Error(), http.StatusInternalServerError)
			} else {
				// Send HTTP response as Json
				content, err1 := json.Marshal(resp)
				if err1 != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				w.WriteHeader(http.StatusInternalServerError)
				w.Write(content)
			}
		} else {
			// Send HTTP response as Json
			content, err := json.Marshal(resp)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Write(content)
		}
	}
}

// InitKubServiceWatch initializes the k8s service watch
func InitKubServiceWatch(np *plugin.NetPlugin) {

	watchClient := setUpAPIClient()
	if watchClient == nil {
		log.Fatalf("Could not init kubernetes API client")
	}

	svcCh := make(chan SvcWatchResp, 1)
	epCh := make(chan EpWatchResp, 1)
	go func() {
		for {
			select {
			case svcEvent := <-svcCh:
				switch svcEvent.opcode {
				case "WARN":
					log.Debugf("svcWatch : %s", svcEvent.errStr)
					break
				case "FATAL":
					log.Errorf("svcWatch : %s", svcEvent.errStr)
					break
				case "ERROR":
					log.Warnf("svcWatch : %s", svcEvent.errStr)
					watchClient.WatchServices(svcCh)
					break

				case "DELETED":
					np.DelSvcSpec(svcEvent.svcName, &svcEvent.svcSpec)
					break
				default:
					np.AddSvcSpec(svcEvent.svcName, &svcEvent.svcSpec)
				}
			case epEvent := <-epCh:
				switch epEvent.opcode {
				case "WARN":
					log.Debugf("epWatch : %s", epEvent.errStr)
					break
				case "FATAL":
					log.Errorf("epWatch : %s", epEvent.errStr)
					break
				case "ERROR":
					log.Warnf("epWatch : %s", epEvent.errStr)
					watchClient.WatchSvcEps(epCh)
					break

				default:
					np.SvcProviderUpdate(epEvent.svcName, epEvent.providers)
				}
			}
		}
	}()

	watchClient.WatchServices(svcCh)
	watchClient.WatchSvcEps(epCh)
}

// InitCNIServer initializes the k8s cni server
func InitCNIServer(netplugin *plugin.NetPlugin) error {

	netPlugin = netplugin
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Could not retrieve hostname: %v", err)
	}

	pluginHost = hostname

	// Set up the api client instance
	kubeAPIClient = setUpAPIClient()
	if kubeAPIClient == nil {
		log.Fatalf("Could not init kubernetes API client")
	}

	log.Debugf("Configuring router")

	router := mux.NewRouter()

	// register handlers for cni
	t := router.Headers("Content-Type", "application/json").Methods("POST").Subrouter()
	t.HandleFunc(cniapi.EPAddURL, makeHTTPHandler(addPod))
	t.HandleFunc(cniapi.EPDelURL, makeHTTPHandler(deletePod))
	t.HandleFunc("/ContivCNI.{*}", unknownAction)

	driverPath := cniapi.ContivCniSocket
	os.Remove(driverPath)
	os.MkdirAll(cniapi.PluginPath, 0700)

	go func() {
		l, err := net.ListenUnix("unix", &net.UnixAddr{Name: driverPath, Net: "unix"})
		if err != nil {
			panic(err)
		}

		log.Infof("k8s plugin listening on %s", driverPath)
		http.Serve(l, router)
		l.Close()
		log.Infof("k8s plugin closing %s", driverPath)
	}()

	//InitKubServiceWatch(netplugin)
	return nil
}

func logEvent(ev string) {
	log.Infof("Handling %q event", ev)
}

// Catchall for additional driver functions.
func unknownAction(w http.ResponseWriter, r *http.Request) {
	log.Infof("Unknown networkdriver action at %q", r.URL.Path)
	content, _ := ioutil.ReadAll(r.Body)
	log.Infof("Body content: %s", string(content))
	w.WriteHeader(503)
}
