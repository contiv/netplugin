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
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/mgmtfn/dockplugin/libnetClient"
	"github.com/contiv/netplugin/netmaster/client"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/libnetwork/drivers/remote/api"
	"github.com/gorilla/mux"
)

const pluginPath = "/run/docker/plugins"
const driverName = "netplugin"

// InitDockPlugin initializes the docker plugin
func InitDockPlugin() error {
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Could not retrieve hostname: %v", err)
	}

	var (
		tenantName = "default"
	)

	log.Debugf("Configuring router")

	router := mux.NewRouter()
	s := router.Headers("Accept", "application/vnd.docker.plugins.v1+json").
		Methods("POST").Subrouter()

	dispatchMap := map[string]func(http.ResponseWriter, *http.Request){
		"/Plugin.Activate":                activate(hostname),
		"/Plugin.Deactivate":              deactivate(hostname),
		"/NetworkDriver.CreateNetwork":    createNetwork(),
		"/NetworkDriver.DeleteNetwork":    deleteNetwork(),
		"/NetworkDriver.CreateEndpoint":   createEndpoint(tenantName, hostname),
		"/NetworkDriver.DeleteEndpoint":   deleteEndpoint(tenantName, hostname),
		"/NetworkDriver.EndpointOperInfo": endpointInfo,
		"/NetworkDriver.Join":             join(),
		"/NetworkDriver.Leave":            leave(),
	}

	for dispatchPath, dispatchFunc := range dispatchMap {
		s.HandleFunc(dispatchPath, logHandler(dispatchPath, dispatchFunc))
	}

	driverPath := path.Join(pluginPath, driverName) + ".sock"
	os.Remove(driverPath)
	os.MkdirAll(pluginPath, 0700)

	go func() {
		l, err := net.ListenUnix("unix", &net.UnixAddr{Name: driverPath, Net: "unix"})
		if err != nil {
			panic(err)
		}

		log.Infof("docker plugin listening on %s", driverPath)
		http.Serve(l, router)
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
		log.Warnf("Error received marshalling error response: %v, original error: %s", errc, fullError)
		return
	}

	log.Warnf("Returning HTTP error handling plugin negotiation: %s", fullError)
	http.Error(w, string(content), http.StatusInternalServerError)
}

func logEvent(typ string) {
	log.Infof("Handling %q event", typ)
}

// activate the plugin and register it as a network driver.
func deactivate(hostname string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logEvent("deactivate")
	}
}

// activate the plugin and register it as a network driver.
func activate(hostname string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logEvent("activate")

		content, err := json.Marshal(plugins.Manifest{Implements: []string{"NetworkDriver"}})
		if err != nil {
			httpError(w, "Could not generate bootstrap response", err)
			return
		}

		w.Write(content)
	}
}

func deleteNetwork() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logEvent("delete network")

		content, err := ioutil.ReadAll(r.Body)
		if err != nil {
			httpError(w, "Could not read delete network request", err)
			return
		}

		dnreq := api.DeleteNetworkRequest{}
		if err := json.Unmarshal(content, &dnreq); err != nil {
			httpError(w, "Could not read delete network request", err)
			return
		}

		dnresp := api.DeleteNetworkResponse{}
		content, err = json.Marshal(dnresp)
		if err != nil {
			httpError(w, "Could not generate delete network response", err)
			return
		}
		w.Write(content)
	}
}

func createNetwork() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logEvent("create network")

		content, err := ioutil.ReadAll(r.Body)
		if err != nil {
			httpError(w, "Could not read create network request", err)
			return
		}

		log.Infoln(string(content))

		cnreq := api.CreateNetworkRequest{}
		if err := json.Unmarshal(content, &cnreq); err != nil {
			httpError(w, "Could not read create network request", err)
			return
		}

		log.Infof("CreateNetworkRequest: %+v", cnreq)

		cnresp := api.CreateNetworkResponse{}
		content, err = json.Marshal(cnresp)
		if err != nil {
			httpError(w, "Could not generate create network response", err)
			return
		}

		w.Write(content)
	}
}

func generateEndpoint(containerID, tenantName, networkName, hostname string) *intent.Config {
	return &intent.Config{
		Tenants: []intent.ConfigTenant{
			{
				Name: tenantName,
				Networks: []intent.ConfigNetwork{
					{
						Name: networkName,
						Endpoints: []intent.ConfigEP{
							{
								Container: containerID,
								Host:      hostname,
							},
						},
					},
				},
			},
		},
	}
}

func deleteEndpoint(tenantName, hostname string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logEvent("delete endpoint")

		content, err := ioutil.ReadAll(r.Body)
		if err != nil {
			httpError(w, "Could not read delete endpoint request", err)
			return
		}

		der := api.DeleteEndpointRequest{}
		if err := json.Unmarshal(content, &der); err != nil {
			httpError(w, "Could not read delete endpoint request", err)
			return
		}

		networkName, err := GetNetworkName(der.NetworkID)
		if err != nil {
			log.Errorf("Error getting network name for UUID: %s. Err: %v", der.NetworkID, err)
			httpError(w, "Could not get network name", err)
			return
		}

		if err := netdcliDel(generateEndpoint(der.EndpointID, tenantName, networkName, hostname)); err != nil {
			httpError(w, "Could not create the endpoint", err)
			return
		}

		content, err = json.Marshal(api.DeleteEndpointResponse{})
		if err != nil {
			httpError(w, "Could not generate delete endpoint response", err)
			return
		}

		w.Write(content)
	}
}

func createEndpoint(tenantName, hostname string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// assumptions on options passed as of early v1:
		// io.docker.network.endpoint.exposedports: docker's notion of exposed ports:
		//   * array of struct of Port, Proto (I presume 6 is ipv6 and 4 is ipv4)
		// io.docker.network.endpoint.portmap: map of exposed ports to the host
		//   * structure follows:
		//     {
		//       "Proto": 6,
		//       "IP": "",
		//       "Port": 1234,
		//       "HostIP": "",
		//       "HostPort": 1234
		//     }
		//

		logEvent("create endpoint")

		content, err := ioutil.ReadAll(r.Body)
		if err != nil {
			httpError(w, "Could not read endpoint create request", err)
			return
		}

		cereq := api.CreateEndpointRequest{}

		if err := json.Unmarshal(content, &cereq); err != nil {
			httpError(w, "Could not read endpoint create request", err)
			return
		}

		log.Infof("CreateEndpointRequest: %+v", cereq)

		networkName, err := GetNetworkName(cereq.NetworkID)
		if err != nil {
			log.Errorf("Error getting network name for UUID: %s. Err: %v", cereq.NetworkID, err)
			httpError(w, "Could not get network name", err)
			return
		}

		// FIXME:
		GetEndPointName(cereq.NetworkID, cereq.EndpointID)

		if err := netdcliAdd(generateEndpoint(cereq.EndpointID, tenantName, networkName, hostname)); err != nil {
			httpError(w, "Could not create endpoint", err)
			return
		}

		time.Sleep(1 * time.Second)

		ep, err := netdcliGetEndpoint(networkName + "-" + cereq.EndpointID)
		if err != nil {
			httpError(w, "Could not find created endpoint", err)
			return
		}

		log.Debug(ep)

		nw, err := netdcliGetNetwork(networkName)
		if err != nil {
			httpError(w, "Could not find created endpoint", err)
			return
		}

		epResponse := api.CreateEndpointResponse{
			Interfaces: []*api.EndpointInterface{
				&api.EndpointInterface{
					ID:      1,
					Address: fmt.Sprintf("%s/%d", ep[0].IPAddress, nw[0].SubnetLen),
				},
			},
		}

		content, err = json.Marshal(epResponse)
		if err != nil {
			httpError(w, "Could not generate create endpoint response", err)
			return
		}

		w.Write(content)
	}
}

func endpointInfo(w http.ResponseWriter, r *http.Request) {
	logEvent("endpoint info")

	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httpError(w, "Could not read endpoint create request", err)
		return
	}

	epireq := api.EndpointInfoRequest{}

	if err := json.Unmarshal(content, &epireq); err != nil {
		httpError(w, "Could not read endpoint create request", err)
		return
	}

	log.Infof("EndpointInfoRequest: %+v", epireq)

	resp, err := json.Marshal(api.EndpointInfoResponse{})
	if err != nil {
		httpError(w, "Could not generate endpoint info response", err)
		return
	}

	w.Write(resp)
}

func join() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logEvent("join")

		jr := api.JoinRequest{}
		content, err := ioutil.ReadAll(r.Body)
		if err != nil {
			httpError(w, "Could not read join request", err)
			return
		}

		if err := json.Unmarshal(content, &jr); err != nil {
			httpError(w, "Could not parse join request", err)
			return
		}

		log.Infof("JoinRequest: %+v", jr)

		networkName, err := GetNetworkName(jr.NetworkID)
		if err != nil {
			log.Errorf("Error getting network name for UUID: %s. Err: %v", jr.NetworkID, err)
			httpError(w, "Could not get network name", err)
			return
		}

		// FIXME:
		GetEndPointName(jr.NetworkID, jr.EndpointID)

		ep, err := netdcliGetEndpoint(networkName + "-" + jr.EndpointID)
		if err != nil {
			httpError(w, "Could not derive created interface", err)
			return
		}

		nw, err := netdcliGetNetwork(networkName)
		if err != nil {
			httpError(w, "Could not get network", err)
			return
		}

		// Inspect the container
		/*
			sbKey := strings.Split(jr.SandboxKey, "/")
			cntId := sbKey[len(sbKey)-1]
			log.Infof("Executing: docker inspect %s", cntId)
			out, err := exec.Command("docker", "inspect", cntId).CombinedOutput()
			log.Infof("docker ispect %s\nErr: %v\n%s", cntId, err, out)

			out, err := exec.Command("sh", "-c", "curl localhost:4243/v1.20/networks").CombinedOutput()
			log.Infof("curl localhost:4243/v1.20/networks\nErr: %v\n%s", err, out)
		*/

		content, err = json.Marshal(api.JoinResponse{
			InterfaceNames: []*api.InterfaceName{
				&api.InterfaceName{},
				&api.InterfaceName{
					SrcName:   ep[0].PortName,
					DstPrefix: "eth",
				},
			},
			Gateway: nw[0].DefaultGw,
		})

		if err != nil {
			httpError(w, "Could not generate join response", err)
			return
		}

		w.Write(content)
	}
}

func leave() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logEvent("leave")
		w.WriteHeader(200)
	}
}

func netdcliAdd(payload *intent.Config) error {
	c := client.New("localhost:9999")
	log.Infof("netdcliAdd payload: %+v", payload)
	if err := c.PostAddConfig(payload); err != nil {
		println(err)
		return err
	}

	return nil
}

func netdcliDel(payload *intent.Config) error {
	c := client.New("localhost:9999")
	return c.PostDeleteConfig(payload)
}

func netdcliGetEndpoint(name string) ([]drivers.OvsOperEndpointState, error) {
	c := client.New("localhost:9999")
	content, err := c.GetEndpoint(name)
	if err != nil {
		return nil, err
	}

	var endpoint []drivers.OvsOperEndpointState

	if err := json.Unmarshal(content, &endpoint); err != nil {
		return nil, err
	}

	return endpoint, nil
}

func netdcliGetNetwork(name string) ([]drivers.OvsCfgNetworkState, error) {
	var network []drivers.OvsCfgNetworkState

	c := client.New("localhost:9999")
	content, err := c.GetNetwork(name)
	if err != nil {
		return network, err
	}

	if err := json.Unmarshal(content, &network); err != nil {
		return network, err
	}

	return network, nil
}

// GetNetworkName gets network name from network UUID
func GetNetworkName(nwID string) (string, error) {
	api := libnetClient.NewRemoteAPI("")

	nw, err := api.NetworkByID(nwID)
	if err != nil {
		log.Infof("Error: %v", err)
		return "", err
	}

	log.Infof("Returning network name %s for ID %s", nw.Name(), nwID)

	return nw.Name(), nil
}

// GetEndPointName Returns endpoint name from networkId, endpointId
func GetEndPointName(nwID, epID string) (string, error) {
	api := libnetClient.NewRemoteAPI("")

	nw, err := api.NetworkByID(nwID)
	if err != nil {
		log.Infof("Error: %v", err)
		return "", err
	}

	ep, err := nw.EndpointByID(epID)
	if err != nil {
		log.Infof("Error: %v", err)
		return "", err
	}

	log.Infof("Returning endpoint name %s for ID %s/%s", ep.Name(), nwID, epID)

	return ep.Name(), nil
}
