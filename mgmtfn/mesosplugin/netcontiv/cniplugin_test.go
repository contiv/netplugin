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
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/contiv/netplugin/mgmtfn/mesosplugin/cniapi"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

// assert on true & print error message
func cniAssert(t *testing.T, val bool, msg string) {
	if val == true {
		cniLog.Errorf("%s", msg)
		t.FailNow()
	}
	// else continue
}

// test CNI env. variables
func TestEnvVariables(t *testing.T) {
	envTestVars := []map[string]string{
		{envVarCniCommand: "ADD", envVarCniIfname: "eth0", envVarCniNetns: "/abc/1234",
			envVarCniContainerID: "567890", "status": "success"},
		{envVarCniCommand: "DEL", envVarCniIfname: "eth0", envVarCniNetns: "/abc/1234",
			envVarCniContainerID: "567890", "status": "success"},
		{envVarCniCommand: "ABC", envVarCniIfname: "eth0", envVarCniNetns: "/abc/1234",
			envVarCniContainerID: "567890", "status": "failure"},
		{envVarCniIfname: "eth0", envVarCniNetns: "/abc/1234",
			envVarCniContainerID: "567890", "status": "failure"},
		{envVarCniCommand: "", envVarCniIfname: "eth0", envVarCniNetns: "/abc/1234",
			envVarCniContainerID: "567890", "status": "failure"},
	}

	for _, testCaseEnv := range envTestVars {
		testApp := cniAppInfo{}

		for _, var1 := range []string{envVarCniCommand,
			envVarCniIfname, envVarCniNetns, envVarCniContainerID} {
			os.Unsetenv(var1)
			os.Setenv(var1, testCaseEnv[var1])
		}

		err := testApp.parseEnv()
		if testCaseEnv["status"] == "failure" {
			cniAssert(t, err == nil, "test was expected to fail")
		} else {
			cniAssert(t, err != nil, fmt.Sprintf("test function failed %s", err))
			cniAssert(t, testApp.cniCmd != testCaseEnv[envVarCniCommand],
				fmt.Sprintf("expected cmd %s, got %s", testCaseEnv[envVarCniCommand],
					testApp.cniCmd))
			cniAssert(t, testApp.cniMesosAttr.CniIfname != testCaseEnv[envVarCniIfname],
				fmt.Sprintf("expected ifname %s, got %s", testApp.cniMesosAttr.CniIfname,
					testCaseEnv[envVarCniIfname]))
			cniAssert(t, testApp.cniMesosAttr.CniNetns != testCaseEnv[envVarCniNetns],
				fmt.Sprintf("expected netns %s, got %s", testApp.cniMesosAttr.CniNetns,
					testCaseEnv[envVarCniNetns]))
			cniAssert(t, testApp.cniMesosAttr.CniContainerid != testCaseEnv[envVarCniContainerID],
				fmt.Sprintf("expected conId %s, got %s", testApp.cniMesosAttr.CniContainerid,
					testCaseEnv[envVarCniContainerID]))
		}
	}
}

// test cni labels
func TestParseANwInfo(t *testing.T) {
	cniNetInfo := struct {
		Args struct {
			Mesos struct {
				NetworkInfo struct {
					Labels struct {
						NwLabel []cniapi.NetworkLabel `json:"labels"`
					} `json:"labels"`
				} `json:"network_info"`
			} `json:"org.apache.mesos"`
		} `json:"args"`
	}{}

	testlbls := [][]cniapi.NetworkLabel{
		{{Key: "abc", Value: "test"}, {Key: "cde", Value: "fgh"}},
		{{Key: "tenant_name", Value: "web"}, {Key: "network_name", Value: "subnet100"},
			{Key: "network_group", Value: "acl1"}},
		{{Key: "network_name", Value: "subnet100"},
			{Key: "network_group", Value: "acl1"}},
		{{Key: cniapi.LabelTenantName, Value: "tenant101"},
			{Key: cniapi.LabelNetworkName, Value: "network101"},
			{Key: cniapi.LabelNetworkGroup, Value: "epg101"}},
		{{Key: cniapi.LabelTenantName, Value: ""},
			{Key: cniapi.LabelNetworkName, Value: ""},
			{Key: cniapi.LabelNetworkGroup, Value: ""}},
		{{Key: cniapi.LabelTenantName, Value: "ABCNET"},
			{Key: cniapi.LabelNetworkGroup, Value: "EPG"}},
	}

	testfile := "/tmp/jsonlbl007.json"
	defer os.Remove(testfile)

	cniTestApp := cniAppInfo{}

	for _, testlbl1 := range testlbls {
		cniNetInfo.Args.Mesos.NetworkInfo.Labels.NwLabel = testlbl1
		jreq, err := json.Marshal(&cniNetInfo)
		cniAssert(t, err != nil, fmt.Sprintf("json conversion error for %+v %s", testlbl1, err))
		os.Remove(testfile)
		err = ioutil.WriteFile(testfile, jreq, 0644)
		cniAssert(t, err != nil, fmt.Sprintf("failed to write to file %s, %s", testlbl1, err))
		cniTestApp.netcfgFile = testfile
		cniTestApp.parseNwInfoLabels()

		for _, lbl := range testlbl1 {
			switch lbl.Key {
			case cniapi.LabelTenantName:
				cniAssert(t, cniTestApp.cniMesosAttr.Labels.TenantName != lbl.Value,
					fmt.Sprintf("%s : expected %s got %s", lbl.Key,
						lbl.Value, cniTestApp.cniMesosAttr.Labels.TenantName))

			case cniapi.LabelNetworkName:
				cniAssert(t, cniTestApp.cniMesosAttr.Labels.NetworkName != lbl.Value,
					fmt.Sprintf("%s : expected %s got %s", lbl.Key,
						lbl.Value, cniTestApp.cniMesosAttr.Labels.NetworkName))

			case cniapi.LabelNetworkGroup:
				cniAssert(t, cniTestApp.cniMesosAttr.Labels.NetworkGroup != lbl.Value,
					fmt.Sprintf("%s : expected %s got %s", lbl.Key,
						lbl.Value, cniTestApp.cniMesosAttr.Labels.NetworkGroup))
			}
		}
	}
}

// test cni response
func TestSendCniResp(t *testing.T) {
	testApp := cniAppInfo{}
	cniResp := cniapi.CniCmdErrorResp{CniVersion: cniapi.CniDefaultVersion,
		ErrCode: cniapi.CniStatusErrorUnsupportedField,
		ErrMsg:  "contiv: " + "test message"}

	jsonResp, err := json.Marshal(cniResp)
	cniAssert(t, err != nil, fmt.Sprintf("json converion error %s", err))
	cniAssert(t, testApp.sendCniResp(jsonResp,
		cniapi.CniStatusErrorUnsupportedField) != cniapi.CniStatusErrorUnsupportedField,
		"unknown return code")
}

// test cni error
func TestSendCniErrorResp(t *testing.T) {
	testApp := cniAppInfo{}
	cniAssert(t, testApp.sendCniErrorResp(fmt.Sprintf("test error")) != cniapi.CniStatusErrorUnsupportedField,
		"unknown error code")
}

// test error response from http server
func TestHttpServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpBody, err := ioutil.ReadAll(r.Body)
		cniAssert(t, err != nil, fmt.Sprintf("failed to read http body %s", err))
		cniLog.Infof("http body %s", string(httpBody))
		w.WriteHeader(http.StatusInternalServerError)
		cniResp := cniapi.CniCmdErrorResp{CniVersion: cniapi.CniDefaultVersion,
			ErrCode: cniapi.CniStatusErrorUnsupportedField,
			ErrMsg:  fmt.Sprintf("Contiv: netplugin error")}
		cniLog.Infof("server sending error response: %s \n", cniResp.ErrMsg)
		jsonResp, err := json.Marshal(&cniResp)
		cniAssert(t, err != nil, fmt.Sprintf("json conversion error %s", err))
		w.Write(jsonResp)
	}))
	defer ts.Close()
	testApp := cniAppInfo{}
	testApp.httpClient = &http.Client{}
	cniLog.Infof("http url %s", ts.URL)
	jsonReq, err := json.Marshal(testApp.cniMesosAttr)
	cniAssert(t, err != nil, fmt.Sprintf("json conversion failed %s", err))
	jsonBuf := bytes.NewBuffer(jsonReq)
	cniAssert(t, testApp.handleHTTP(ts.URL, jsonBuf) != cniapi.CniStatusErrorUnsupportedField,
		"unexpected return code from http hamdler")
}

// test OK from http server
func TestHttpServerOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpBody, err := ioutil.ReadAll(r.Body)
		cniAssert(t, err != nil, fmt.Sprintf("failed to read http body %s", err))
		cniLog.Infof("http body %s", string(httpBody))
		w.WriteHeader(http.StatusOK)
		cniResp := cniapi.CniCmdSuccessResp{
			CniVersion: cniapi.CniDefaultVersion,
		}
		cniResp.IP4.IPAddress = "10.36.28.101/24"
		cniResp.IP4.Gateway = "10.36.28.1"
		jsonResp, err := json.Marshal(&cniResp)
		cniAssert(t, err != nil, fmt.Sprintf("json conversion error %s", err))
		w.Write(jsonResp)
	}))
	defer ts.Close()
	testApp := cniAppInfo{}
	testApp.httpClient = &http.Client{}
	cniLog.Infof("http url %s", ts.URL)
	jsonReq, err := json.Marshal(testApp.cniMesosAttr)
	cniAssert(t, err != nil, fmt.Sprintf("json conversion failed %s", err))
	jsonBuf := bytes.NewBuffer(jsonReq)
	cniAssert(t, testApp.handleHTTP(ts.URL, jsonBuf) != cniapi.CniStatusSuccess, "htttp handler failed")
}
