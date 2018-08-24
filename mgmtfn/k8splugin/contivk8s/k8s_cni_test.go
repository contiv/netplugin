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
	"github.com/sw4iot/ns-utils"
	"github.com/vishvananda/netlink"
)

const (
	utPodIP        = "44.55.66.77/22"
	utCNIARG1      = "K8S_POD_NAMESPACE=utK8sNS"
	utCNIARG2      = "K8S_POD_NAME=utPod"
	utCNIARG3      = "K8S_POD_INFRA_CONTAINER_ID=8ec72deca647bfa60a4b815aa735c87de859b47e872828586749b9d852af1f49"
	utCNINETNSPid  = "/proc/98765/ns/net"
	utCNINETNSPath = "/var/run/netns/netns_test"
	utPodIface     = "testlinkfoo"
)

type restAPIFunc func(r *http.Request) (interface{}, error)

func SetUpTest() (string, int, error) {
	la := netlink.NewLinkAttrs()
	la.Name = utPodIface
	n, err := ns.NewNS()
	if err != nil {
		return "", -1, err
	}

	ipPath, err := osexec.LookPath("ip")
	if err != nil {
		return "", -1, err
	}

	elements := strings.Split(n.Path(), "/")
	cmd := osexec.Command(ipPath, "exec", elements[4], "sleep", "infinity")
	if err = cmd.Start(); err != nil {
		logger.Fatalf("failed to start the 'sleep 9999' process: %v", err)
		return "", -1, err
	}
	pid := cmd.Process.Pid

	dummy := &netlink.Dummy{LinkAttrs: la}
	if err := netlink.LinkAdd(dummy); err != nil {
		logger.Fatalf("failed to add dummy interface: %v", err)
		return "", -1, err
	}

	return n.Path(), pid, nil
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
			resp.Attr.IPAddress = utPodIP
			resp.EndpointID = pInfo.InfraContainerID
			return resp, nil
		}
	}
	logger.Errorf("Failed pod %v", pInfo)
	return resp, fmt.Errorf("failed to add pod")
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
	return resp, fmt.Errorf("failed to delete pod")
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

func TestSetIfAttrs(t *testing.T) {
	newName := "testlinknewname"
	address := "192.168.68.68/24"
	ipv6Address := "2001::100/100"
	nspath := "/proc/1/ns/net"
	ifName := "testlinkfoo"
	if err := setIfAttrs(nspath, ifName, address, ipv6Address, newName); err != nil {
		t.Errorf("setIfAttrs failed: %v", err)
	}
}

func setupTestEnv() {
	testCNIARGS := utCNIARG1 + ";" + utCNIARG2 + ";" + utCNIARG3
	os.Setenv("CNI_ARGS", testCNIARGS)
	os.Setenv("CNI_IFNAME", "eth0")
	return
}

// TestAddpodPid tests the AddPod interface using the pid
func TestAddpodPid(t *testing.T) {
	_, pid, err := SetUpTest()
	if err != nil {
		t.Errorf("TestAddpodPid failed: %v", err)
	}
	setupTestEnv()
	os.Setenv("CNI_NETNS", "/proc/"+strconv.Itoa(pid)+"/ns/net")
	os.Setenv("CNI_COMMAND", "ADD")
	mainfunc()
}

// TestAddpodPath tests the AddPod interface using the path
func TestAddpodPath(t *testing.T) {
	nspath, _, err := SetUpTest()
	if err != nil {
		t.Errorf("TestAddpodPid failed: %v", err)
	}
	setupTestEnv()
	os.Setenv("CNI_NETNS", nspath)
	os.Setenv("CNI_COMMAND", "ADD")
	mainfunc()
}

// TestAddpodPid tests the DeletePod interface using the pid
func TestDelpodPid(m *testing.T) {
	setupTestEnv()
	os.Setenv("CNI_COMMAND", "DEL")
	mainfunc()
}

// TestAddpodPath tests the DeletePod interface using the path
func TestDelpodPath(m *testing.T) {
	setupTestEnv()
	os.Setenv("CNI_COMMAND", "DEL")
	mainfunc()
}
