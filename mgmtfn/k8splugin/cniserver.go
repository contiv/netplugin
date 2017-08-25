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
	"net"
	"net/http"
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/mgmtfn/k8splugin/cniapi"
	"github.com/contiv/netplugin/netplugin/plugin"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/netplugin/utils/k8sutils"
	"github.com/gorilla/mux"
)

var netPlugin *plugin.NetPlugin
var kubeAPIClient *APIClient
var pluginHost string
var contivK8Config k8sutils.ContivConfig

// GetK8sClusterIPRange returns k8s cluster ip range
func GetK8sClusterIPRange() string {
	return contivK8Config.SvcSubnet
}

// setUpAPIClient sets up an instance of the k8s api server
func setUpAPIClient() *APIClient {
	// Read config
	err := k8sutils.GetK8SConfig(&contivK8Config)
	if err != nil {
		log.Errorf("Failed: %v", err)
		return nil
	}

	return NewAPIClient(contivK8Config.K8sAPIServer, contivK8Config.K8sCa,
		contivK8Config.K8sKey, contivK8Config.K8sCert, contivK8Config.K8sToken)

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
	t.HandleFunc(cniapi.EPAddURL, utils.MakeHTTPHandler(addPod))
	t.HandleFunc(cniapi.EPDelURL, utils.MakeHTTPHandler(deletePod))
	t.HandleFunc("/ContivCNI.{*}", utils.UnknownAction)

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
