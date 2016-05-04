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
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"os"
	osexec "os/exec"
	"strings"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netplugin/plugin"
	"github.com/gorilla/mux"
)

const (
	testPodURL       = "/api/v1/namespaces/default/pods/test-pod"
	svcWatchURL      = "/api/v1/watch/services"
	epWatchURL       = "/api/v1/watch/endpoints"
	contivKubeCfgDir = "/opt/contiv/config"
	testCfgFile      = "/tmp/certs/contiv.json"
	testServerURL    = "0.0.0.0:443"
	testSvcPort      = 4639
	testTgtPort      = 9364
	testClusterIP    = "10.254.111.222"
	tstSvcName       = "LipService"
	testMaxSvcs      = 3
	testEPIPAddr1    = "123.45.67.89"
	testEPIPAddr2    = "123.45.67.88"
)

var totalSvcResp int
var totalEPResp int
var maxSvcResp int
var maxEPResp int

type podStruct struct {
	ObjectMeta `json:"metadata,omitempty"`
}

// KubeTestNetDrvConfig represents the configuration of the fakedriver,
// which is an empty struct.
type KubeTestNetDrvConfig struct{}

// KubeTestNetDrv implements core.NetworkDriver interface
// for use with unit-tests
type KubeTestNetDrv struct {
	numAddSvc  int
	numDelSvc  int
	numProvUpd int
	services   map[string]*core.ServiceSpec
	providers  map[string][]string
}
type restFunc func(r *http.Request, iter int) (interface{}, bool, error)

// Init is not implemented.
func (d *KubeTestNetDrv) Init(nfo *core.InstanceInfo) error {
	d.services = make(map[string]*core.ServiceSpec)
	d.providers = make(map[string][]string)
	return nil
}

// Deinit is not implemented.
func (d *KubeTestNetDrv) Deinit() {
}

// CreateNetwork is not implemented.
func (d *KubeTestNetDrv) CreateNetwork(id string) error {
	return nil
}

// DeleteNetwork is not implemented.
func (d *KubeTestNetDrv) DeleteNetwork(id, nwType, encap string, pktTag, extPktTag int, gateway string, tenant string) error {
	return nil
}

// CreateEndpoint is not implemented.
func (d *KubeTestNetDrv) CreateEndpoint(id string) error {
	return nil
}

// DeleteEndpoint is not implemented.
func (d *KubeTestNetDrv) DeleteEndpoint(id string) (err error) {
	return nil
}

// AddPeerHost is not implemented.
func (d *KubeTestNetDrv) AddPeerHost(node core.ServiceInfo) error {
	return nil
}

// DeletePeerHost is not implemented.
func (d *KubeTestNetDrv) DeletePeerHost(node core.ServiceInfo) error {
	return nil
}

// AddMaster is not implemented
func (d *KubeTestNetDrv) AddMaster(node core.ServiceInfo) error {
	return nil
}

// DeleteMaster is not implemented
func (d *KubeTestNetDrv) DeleteMaster(node core.ServiceInfo) error {
	return nil
}

// AddBgp is not implemented.
func (d *KubeTestNetDrv) AddBgp(id string) error {
	return nil
}

// DeleteBgp is not implemented.
func (d *KubeTestNetDrv) DeleteBgp(id string) error {
	return nil
}

// AddSvcSpec is implemented.
func (d *KubeTestNetDrv) AddSvcSpec(svcName string, spec *core.ServiceSpec) error {
	d.services[svcName] = spec
	d.numAddSvc++
	return nil
}

// DelSvcSpec is implemented.
func (d *KubeTestNetDrv) DelSvcSpec(svcName string, spec *core.ServiceSpec) error {
	delete(d.services, svcName)
	d.numDelSvc++
	return nil
}

// SvcProviderUpdate is implemented.
func (d *KubeTestNetDrv) SvcProviderUpdate(svcName string, provs []string) {
	d.providers[svcName] = provs
	d.numProvUpd++
}

// Simple Wrapper for http handlers
func restWrapper(handlerFunc restFunc) http.HandlerFunc {
	// Create a closure and return an anonymous function
	return func(w1 http.ResponseWriter, r *http.Request) {
		w := httputil.NewChunkedWriter(w1)
		flusher, ok := w1.(http.Flusher)
		if !ok {
			log.Errorf("Could not get flusher")
			http.NotFound(w1, r)
			return
		}
		w1.Header().Set("Transfer-Encoding", "chunked")
		w1.WriteHeader(http.StatusOK)
		flusher.Flush()

		// Call the handler
		count := 0
		for {
			resp, done, err := handlerFunc(r, count)
			if err != nil {
				// Send HTTP response
				http.Error(w1, err.Error(), http.StatusInternalServerError)
			} else {
				// Send HTTP response as Json
				content, err := json.Marshal(resp)
				if err != nil {
					log.Errorf("Marshal failed: %v", err)
					http.Error(w1, err.Error(), http.StatusInternalServerError)
					return
				}
				_, err = w.Write(content)
				if err != nil {
					log.Errorf("Write failed: %v", err)
				}
				flusher.Flush()
			}
			if done {
				break
			}
			count++
		}
	}
}

func testPodGet(r *http.Request, iter int) (interface{}, bool, error) {

	lMap := make(map[string]string)
	lMap["io.contiv.network"] = "ut-net"
	meta := ObjectMeta{Name: "test-pod",
		Namespace: "default",
		Labels:    lMap,
	}

	resp := podStruct{ObjectMeta: meta}

	return resp, true, nil

}

func serviceWatch(r *http.Request, iter int) (interface{}, bool, error) {
	for totalSvcResp >= maxSvcResp {
		time.Sleep(time.Second)
	}

	totalSvcResp++

	sPort := ServicePort{
		Protocol:   ProtocolTCP,
		Port:       testSvcPort,
		TargetPort: testTgtPort,
	}
	ports := make([]ServicePort, 1)
	ports[0] = sPort
	sSpec := ServiceSpec{
		ClusterIP: testClusterIP,
		Ports:     ports,
	}
	meta := ObjectMeta{Name: tstSvcName}
	svc := Service{
		ObjectMeta: meta,
		Spec:       sSpec,
	}
	typeStr := ""
	if iter == 1 {
		typeStr = "DELETED"
	} else {
		typeStr = "ADDED"
	}
	resp := watchSvcStatus{
		Type:   typeStr,
		Object: svc,
	}

	log.Debugf("### inside service watch req: %+v", r)
	if iter == testMaxSvcs {
		return resp, true, nil
	}
	time.Sleep(time.Second)
	return resp, false, nil
}

func epWatch(r *http.Request, iter int) (interface{}, bool, error) {

	for totalEPResp >= maxEPResp {
		time.Sleep(time.Second)
	}

	totalEPResp++

	epPort := EndpointPort{
		Port:     testTgtPort,
		Protocol: ProtocolTCP,
	}
	ports := make([]EndpointPort, 1)
	ports[0] = epPort

	epAddr1 := EndpointAddress{
		IP: testEPIPAddr1,
	}
	epAddr2 := EndpointAddress{
		IP: testEPIPAddr2,
	}
	addrs := make([]EndpointAddress, 2)
	addrs[0] = epAddr1
	addrs[1] = epAddr2
	var availAddr []EndpointAddress
	switch iter {
	case 0:
		availAddr = addrs
	case 1:
		availAddr = addrs[1:1]
	case 2:
		availAddr = addrs[0:0]
	}
	epSubset := EndpointSubset{
		Addresses: availAddr,
		Ports:     ports,
	}

	epSS := make([]EndpointSubset, 1)
	epSS[0] = epSubset
	meta := ObjectMeta{Name: tstSvcName}
	eps := Endpoints{
		ObjectMeta: meta,
		Subsets:    epSS,
	}
	resp := watchSvcEpStatus{
		Type:   "ADDED",
		Object: eps,
	}

	if iter == testMaxSvcs {
		return resp, true, nil
	}
	time.Sleep(time.Second)
	return resp, false, nil
}

// setupTestServer creates a listener for the rest requests.
func setupTestServer(m *testing.M) {

	// Read client cert
	cert, err := tls.LoadX509KeyPair("/tmp/certs/server.crt", "/tmp/certs/server.key")
	if err != nil {
		log.Fatalf("%v", err)
		return
	}

	// Read CA cert
	ca, err := ioutil.ReadFile("/tmp/certs/ca.crt")
	if err != nil {
		log.Errorf("%v", err)
		return
	}

	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(ca)

	// Setup HTTPS server
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caPool,
	}
	tlsCfg.BuildNameToCertificate()
	router := mux.NewRouter()

	// register handlers
	t := router.Headers().Methods("GET").Subrouter()
	t.HandleFunc(testPodURL, restWrapper(testPodGet))
	t.HandleFunc(svcWatchURL, restWrapper(serviceWatch))
	t.HandleFunc(epWatchURL, restWrapper(epWatch))

	server := &http.Server{Addr: testServerURL, Handler: router, TLSConfig: tlsCfg}

	go func() {
		err := server.ListenAndServeTLS("", "")
		if err != nil {
			log.Fatalf("server returned %v", err)
		}
	}()

	// make sure the listener is ready before returning
	for count := 0; count < 5; count++ {
		stat, err := osexec.Command("netstat", "-tlpn").CombinedOutput()
		if err != nil {
			log.Fatalf("Unable to check server status: %v", err)
			return
		}

		if strings.Contains(string(stat), ":443") {
			return
		}

		time.Sleep(time.Second)
		log.Infof("stat : %s", stat)
	}

	log.Fatalf("Kube server not ready after 5 sec")
}

func setupCerts(m *testing.M) {
	_, err := osexec.Command("mkdir", "-p", contivKubeCfgDir).CombinedOutput()
	if err != nil {
		log.Fatalf("mkdir failed: %v", err)
		return
	}

	_, err = osexec.Command("cp", testCfgFile, contivKubeCfgDir).CombinedOutput()
	if err != nil {
		log.Fatalf("Failed to copy %s dir :%v", testCfgFile, err)
		return
	}
}

// TestMain sets up a fake kube server to enable testing the client
func TestMain(m *testing.M) {
	setupCerts(m)
	setupTestServer(m)
	os.Exit(m.Run())
}

func verifySvc(m *testing.T, drv *KubeTestNetDrv) {
	ls, ok := drv.services["LipService"]
	if !ok {
		m.Errorf("Service was not correctly updated on client")
	} else {
		m.Logf("service: %+v", ls)
		if ls.IPAddress != testClusterIP {
			m.Errorf("ClusterIP is incorrect")
		}

		if len(ls.Ports) != 1 {
			m.Errorf("Noumber of ports is incorrect")
		}

		if ls.Ports[0].Protocol != "TCP" {
			m.Errorf("Protocol is incorrect")
		}

		if ls.Ports[0].SvcPort != testSvcPort {
			m.Errorf("Svc port is incorrect")
		}

		if ls.Ports[0].ProvPort != testTgtPort {
			m.Errorf("Prov port is incorrect")
		}
	}
}

// TestKubeWatch tests the watch interface
func TestKubeWatch(m *testing.T) {
	drv := &KubeTestNetDrv{}
	np := &plugin.NetPlugin{
		NetworkDriver: drv,
	}
	drv.Init(nil)
	totalSvcResp = 0
	totalEPResp = 0

	InitKubServiceWatch(np)
	m.Logf("--ADD--")
	maxEPResp = 0
	maxSvcResp = 1
	for ix := 0; ix < 2; ix++ {
		time.Sleep(time.Second)
	}
	m.Logf("Drv: %+v", drv)
	if drv.numAddSvc != 1 {
		m.Errorf("Add service was not seen by client")
	} else {
		m.Logf("Add service seen by client, as expected")
	}

	verifySvc(m, drv)

	if len(drv.providers) != 0 {
		m.Errorf("Provider list is incorrect")

	}

	m.Logf("--DEL--")
	maxEPResp = 0
	maxSvcResp = 2
	for ix := 0; ix < 3; ix++ {
		time.Sleep(time.Second)
	}
	m.Logf("Drv: %+v", drv)
	if drv.numDelSvc != 1 {
		m.Errorf("Del service was not seen by client")
	} else {
		m.Logf("Del service seen by client, as expected")
	}

	_, ok := drv.services["LipService"]
	if ok {
		m.Errorf("Service was not deleted on client")
	}

	m.Logf("--CLOSE--")
	maxEPResp = 6
	maxSvcResp = 4
	for ix := 0; ix < 8; ix++ {
		time.Sleep(time.Second)
	}
	m.Logf("Drv: %+v", drv)
	m.Logf("services: %+v", drv.services["LipService"])
	if (drv.numAddSvc != 3) || (drv.numDelSvc != 1) || (drv.numProvUpd != 6) {
		m.Errorf("All updates were not seen by client")
	}

	verifySvc(m, drv)

	provs, ok := drv.providers["LipService"]
	if !ok {
		m.Errorf("Providers were not updated on client")
	} else {
		if len(provs) != 2 {
			m.Errorf("Providers were not updated correctly on client")
		}
	}
}
