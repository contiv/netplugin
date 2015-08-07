package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/netmaster/client"
	"github.com/contiv/netplugin/netmaster/intent"

	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/libnetwork/drivers/remote/api"
	"github.com/gorilla/mux"

	log "github.com/Sirupsen/logrus"
)

var (
	optListen = flag.String("listen", "localhost:4545", "host:port to listen on")
	optDebug  = flag.Bool("debug", false, "Get debug logging")
)

func init() {
	flag.Usage = usage
}

func usage() {
	fmt.Printf("Usage: " + os.Args[0] + " <interface-name> <tenant> <network>\n")
	flag.PrintDefaults()
	os.Exit(1)
}

func main() {
	flag.Parse()
	if len(flag.Args()) != 3 {
		flag.Usage()
	}

	if *optDebug {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug level logging enabled")
	}

	var (
		interfaceName = flag.Arg(0)
		tenantName    = flag.Arg(1)
		networkName   = flag.Arg(2)
	)

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Could not retrieve hostname: %v", err)
	}

	log.Debugf("Configuring router")

	router := mux.NewRouter()
	s := router.Headers("Accept", "application/vnd.docker.plugins.v1+json").
		Methods("POST").Subrouter()

	dispatchMap := map[string]func(http.ResponseWriter, *http.Request){
		"/Plugin.Activate":                activate(hostname, interfaceName),
		"/Plugin.Deactivate":              deactivate(hostname, interfaceName),
		"/NetworkDriver.CreateNetwork":    createNetwork(tenantName, networkName),
		"/NetworkDriver.DeleteNetwork":    deleteNetwork(tenantName, networkName),
		"/NetworkDriver.CreateEndpoint":   createEndpoint(tenantName, networkName, hostname),
		"/NetworkDriver.DeleteEndpoint":   deleteEndpoint(tenantName, networkName, hostname),
		"/NetworkDriver.EndpointOperInfo": endpointInfo,
		"/NetworkDriver.Join":             join(networkName),
		"/NetworkDriver.Leave":            leave(networkName),
	}

	for dispatchPath, dispatchFunc := range dispatchMap {
		s.HandleFunc(dispatchPath, logHandler(dispatchPath, *optDebug, dispatchFunc))
	}

	if *optDebug {
		log.Debug("Enabling catchall networkdriver interface")
		s.HandleFunc("/NetworkDriver.{action:.*}", action)
	}

	http.ListenAndServe(*optListen, router)
}

func logHandler(name string, debug bool, actionFunc func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if debug {
			buf := new(bytes.Buffer)
			io.Copy(buf, r.Body)
			log.Debugf("Dispatching %s with %v", name, strings.TrimSpace(string(buf.Bytes())))
			var writer *io.PipeWriter
			r.Body, writer = io.Pipe()
			go func() {
				io.Copy(writer, buf)
				writer.Close()
			}()
		}

		actionFunc(w, r)
	}
}

// Catchall for additional driver functions.
func action(w http.ResponseWriter, r *http.Request) {
	log.Debugf("Unknown networkdriver action at %q", r.URL.Path)
	content, _ := ioutil.ReadAll(r.Body)
	log.Debug("Body content:", string(content))
	w.WriteHeader(503)
}

// generate the appropriate Hosts structure for this host.
func generateHosts(hostname, interfaceName string) *intent.Config {
	return &intent.Config{
		Hosts: []intent.ConfigHost{
			{
				Name: hostname,
				Intf: interfaceName,
			},
		},
	}
}

// activate the plugin and register it as a network driver.
func deactivate(hostname, interfaceName string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logEvent("deactivate")
	}
}

// activate the plugin and register it as a network driver.
func activate(hostname, interfaceName string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logEvent("activate")

		if err := netdcliAdd(generateHosts(hostname, interfaceName)); err != nil {
			httpError(w, "Could not bootstrap netplugin", err)
			return
		}

		content, err := json.Marshal(plugins.Manifest{Implements: []string{"NetworkDriver"}})
		if err != nil {
			httpError(w, "Could not generate bootstrap response", err)
			return
		}

		w.Write(content)
	}
}

func generateNetwork(tenantName, networkName string) *intent.Config {
	return &intent.Config{
		Tenants: []intent.ConfigTenant{
			{
				Name: tenantName,
				Networks: []intent.ConfigNetwork{
					{
						Name: networkName,
					},
				},
			},
		},
	}
}

func deleteNetwork(tenantName, networkName string) func(http.ResponseWriter, *http.Request) {
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

		// FIXME Note that we don't actually have anything from the request we can
		// use just yet.

		if err := netdcliDel(generateNetwork(tenantName, networkName)); err != nil {
			httpError(w, "Could not delete network", err)
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

func createNetwork(tenantName, networkName string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logEvent("create network")

		content, err := ioutil.ReadAll(r.Body)
		if err != nil {
			httpError(w, "Could not read create network request", err)
			return
		}

		log.Debugln(string(content))

		cnreq := api.CreateNetworkRequest{}
		if err := json.Unmarshal(content, &cnreq); err != nil {
			httpError(w, "Could not read create network request", err)
			return
		}

		// FIXME Note that we don't actually have anything from the request we can
		// use just yet.

		if err := netdcliAdd(generateNetwork(tenantName, networkName)); err != nil {
			httpError(w, "Could not create network", err)
			return
		}

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

func deleteEndpoint(tenantName, networkName, hostname string) func(http.ResponseWriter, *http.Request) {
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

func createEndpoint(tenantName, networkName, hostname string) func(http.ResponseWriter, *http.Request) {
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

		fmt.Println(cereq)

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

	content, err := json.Marshal(api.EndpointInfoResponse{})
	if err != nil {
		httpError(w, "Could not generate endpoint info response", err)
		return
	}

	w.Write(content)
}

func join(networkName string) func(http.ResponseWriter, *http.Request) {
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

		content, err = json.Marshal(api.JoinResponse{
			InterfaceNames: []*api.InterfaceName{
				&api.InterfaceName{},
				&api.InterfaceName{
					SrcName: ep[0].PortName,
					DstName: "eth",
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

func leave(networkName string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logEvent("leave")
		w.WriteHeader(200)
	}
}

func netdcliAdd(payload *intent.Config) error {
	c := client.New("localhost:9999")
	fmt.Println(payload)
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
