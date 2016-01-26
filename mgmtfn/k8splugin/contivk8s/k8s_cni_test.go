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

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	osexec "os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	logger "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/mgmtfn/k8splugin/cniapi"
	"github.com/gorilla/mux"
)

const (
	utPodIP    = "44.55.66.77/22"
	utCNIARG1  = "K8S_POD_NAMESPACE=utK8sNS"
	utCNIARG2  = "K8S_POD_NAME=utPod"
	utCNIARG3  = "K8S_POD_INFRA_CONTAINER_ID=8ec72deca647bfa60a4b815aa735c87de859b47e872828586749b9d852af1f49"
	utCNINETNS = "/proc/98765/ns/net"
)

type restAPIFunc func(r *http.Request) (interface{}, error)

// nsToPID is a utility that extracts the PID from the netns
func nsToPID(ns string) (int, error) {
	// Make sure ns is well formed
	ok := strings.HasPrefix(ns, "/proc/")
	if !ok {
		return -1, fmt.Errorf("Invalid nw name space: %v", ns)
	}

	elements := strings.Split(ns, "/")
	return strconv.Atoi(elements[2])
}

// stubAddPod is the handler for testing pod additions
func stubAddPod(r *http.Request) (interface{}, error) {

	resp := cniapi.RspAddPod{}

	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logger.Errorf("Failed to read request: %v", err)
		return resp, err
	}

	pInfo := cniapi.CNIPodAttr{}
	if err := json.Unmarshal(content, &pInfo); err != nil {
		return resp, err
	}

	// verify pod attributes are as expected.
	if pInfo.Name == "utPod" && pInfo.K8sNameSpace == "utK8sNS" &&
		pInfo.InfraContainerID != "" && pInfo.IntfName != "" {
		_, err := nsToPID(pInfo.NwNameSpace)
		if err != nil {
			logger.Errorf("Failed to fetch pid from netns %s: %v", pInfo.NwNameSpace,
				err)
		} else {
			// respond with success
			resp.IPAddress = utPodIP
			resp.EndpointID = pInfo.InfraContainerID
			return resp, nil
		}
	}
	logger.Errorf("Failed pod %v", pInfo)
	return resp, fmt.Errorf("Failed to add pod")
}

// stubDeletePod is the handler for testing pod additions
func stubDeletePod(r *http.Request) (interface{}, error) {

	resp := cniapi.RspAddPod{}

	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logger.Errorf("Failed to read request: %v", err)
		return resp, err
	}

	pInfo := cniapi.CNIPodAttr{}
	if err := json.Unmarshal(content, &pInfo); err != nil {
		return resp, err
	}

	// verify pod attributes are as expected.
	if pInfo.Name == "utPod" && pInfo.K8sNameSpace == "utK8sNS" &&
		pInfo.InfraContainerID != "" && pInfo.IntfName != "" {
		resp.EndpointID = pInfo.InfraContainerID
		return resp, nil
	}
	logger.Errorf("Failed pod %v", pInfo)
	return resp, fmt.Errorf("Failed to delete pod")
}

// Simple Wrapper for http handlers
func httpWrapper(handlerFunc restAPIFunc) http.HandlerFunc {
	// Create a closure and return an anonymous function
	return func(w http.ResponseWriter, r *http.Request) {
		// Call the handler
		resp, err := handlerFunc(r)
		if err != nil {
			// Log error
			logger.Errorf("Handler for %s %s returned error: %s", r.Method, r.URL, err)

			// Send HTTP response
			http.Error(w, err.Error(), http.StatusInternalServerError)
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

// setupTestServer creates a listener for the rest requests.
func setupTestServer() {

	router := mux.NewRouter()

	// register handlers for cni
	t := router.Headers("Content-Type", "application/json").Methods("POST").Subrouter()
	t.HandleFunc(cniapi.EPAddURL, httpWrapper(stubAddPod))
	t.HandleFunc(cniapi.EPDelURL, httpWrapper(stubDeletePod))

	driverPath := cniapi.ContivCniSocket
	os.Remove(driverPath)
	os.MkdirAll(cniapi.PluginPath, 0700)

	go func() {
		l, err := net.ListenUnix("unix", &net.UnixAddr{Name: driverPath, Net: "unix"})
		if err != nil {
			panic(err)
		}

		logger.Infof("k8s test plugin listening on %s", driverPath)
		http.Serve(l, router)
		l.Close()
		logger.Infof("k8s test plugin closing %s", driverPath)
	}()

	// make sure the listener is ready before returning
	for count := 0; count < 5; count++ {
		_, err := osexec.Command("ls", driverPath).CombinedOutput()
		if err == nil {
			return
		}
		time.Sleep(time.Second)
	}

	logger.Fatalf("Listener not ready after 5 sec")

}

// TestMain sets up an http server for testing k8s plugin REST interface
func TestMain(m *testing.M) {
	setupTestServer()
	os.Exit(m.Run())
}

func setupTestEnv() {
	testCNIARGS := utCNIARG1 + ";" + utCNIARG2 + ";" + utCNIARG3
	os.Setenv("CNI_ARGS", testCNIARGS)
	os.Setenv("CNI_NETNS", utCNINETNS)
	os.Setenv("CNI_IFNAME", "eth0")
}

// TestAddpod tests the AddPod interface
func TestAddpod(m *testing.T) {
	setupTestEnv()
	os.Setenv("CNI_COMMAND", "ADD")
	mainfunc()
}

// TestAddpod tests the DeletePod interface
func TestDelpod(m *testing.T) {
	setupTestEnv()
	os.Setenv("CNI_COMMAND", "DEL")
	mainfunc()
}
