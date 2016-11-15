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
	"flag"
	"fmt"
	logger "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/mgmtfn/mesosplugin/cniapi"
	"github.com/contiv/netplugin/version"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	logfileName = "/var/log/netcontiv.log"
	logfileSize = 50 * (1 << 20)

	// cni env. variables
	envVarCniCommand     = "CNI_COMMAND"
	envVarCniIfname      = "CNI_IFNAME"
	envVarCniNetns       = "CNI_NETNS"
	envVarCniContainerID = "CNI_CONTAINERID"
)

var cniEnvList = []string{envVarCniCommand, envVarCniIfname, envVarCniNetns, envVarCniContainerID}

type cniAppInfo struct {
	serverURL    string
	httpClient   *http.Client
	pluginName   string
	cniCmd       string
	netcfgFile   string
	logFields    logger.Fields
	cniMesosAttr cniapi.CniCmdReqAttr
}

// plugin log with additional fields
var cniLog *logger.Entry

// parse and save env. variables
func (cniApp *cniAppInfo) parseEnv() error {

	cniApp.logFields = make(logger.Fields)
	envVal := ""

	for _, envName := range cniEnvList {

		if envVal = os.Getenv(envName); envVal == "" {
			return fmt.Errorf("failed to get env variable %s", envName)
		}
		logger.Infof("parsed env variable %s = [%s]", envName, envVal)

		switch envName {

		case envVarCniCommand:
			cniApp.cniCmd = strings.ToUpper(envVal)
			cniApp.logFields["CMD"] = envVal
			if _, ok := map[string]int{cniapi.CniCmdAdd: 1,
				cniapi.CniCmdDel: 2}[cniApp.cniCmd]; ok == false {
				return fmt.Errorf("unknown CNI command %s", envName)
			}

		case envVarCniIfname:
			cniApp.cniMesosAttr.CniIfname = envVal

		case envVarCniNetns:
			cniApp.cniMesosAttr.CniNetns = envVal
			nsDir := filepath.Dir(envVal)
			cniApp.netcfgFile = strings.Join([]string{nsDir, "netcontiv", "network.conf"}, "/")
			logger.Infof("cni network config file location : %s", cniApp.netcfgFile)

		case envVarCniContainerID:
			cniApp.cniMesosAttr.CniContainerid = envVal
			cniApp.logFields["CID"] = strings.Split(envVal, "-")[0]
			logger.Debugf("added fields in logger CID: %s", cniApp.logFields["CID"])

		default:
			cniLog.Errorf("unknown CNI variable %s", envName)
		}
	}
	// update logger
	cniLog = logger.WithFields(cniApp.logFields)

	return nil
}

// parse labels from network_info
func (cniApp *cniAppInfo) parseNwInfoLabels() {

	var cniNetInfo struct {
		Args struct {
			Mesos struct {
				NetworkInfo struct {
					Labels struct {
						NwLabel []cniapi.NetworkLabel `json:"labels"`
					} `json:"labels"`
				} `json:"network_info"`
			} `json:"org.apache.mesos"`
		} `json:"args"`
	}

	cniLog.Infof("parse config file %s ", cniApp.netcfgFile)
	cfgFile, err := ioutil.ReadFile(cniApp.netcfgFile)

	if err != nil {
		cniLog.Warnf("%s", err)
		return
	}

	if err := json.Unmarshal(cfgFile, &cniNetInfo); err != nil {
		cniLog.Errorf("failed to parse %s, %s", cniApp.netcfgFile, err)
		return
	}

	for idx, elem := range cniNetInfo.Args.Mesos.NetworkInfo.Labels.NwLabel {
		cniLog.Infof("configured labels [%d] {key: %s, val: %s}", idx,
			elem.Key, elem.Value)

		// copy netplugin related labels
		switch elem.Key {
		case cniapi.LabelTenantName:
			cniApp.cniMesosAttr.Labels.TenantName = elem.Value

		case cniapi.LabelNetworkName:
			cniApp.cniMesosAttr.Labels.NetworkName = elem.Value

		case cniapi.LabelNetworkGroup:
			cniApp.cniMesosAttr.Labels.NetworkGroup = elem.Value
		}
	}
}

// send response received from netplugin to stdout
func (cniApp *cniAppInfo) sendCniResp(cniResp []byte, retCode int) int {
	cniLog.Infof("sent CNI response: %s ", cniResp)
	fmt.Printf("%s\n", string(cniResp))
	return retCode
}

// send error response & return code
func (cniApp *cniAppInfo) sendCniErrorResp(errorMsg string) int {
	cniLog.Infof("prepare CNI error response: %s ", errorMsg)
	// CNI_ERROR_UNSUPPORTED is sent for all errors
	cniResp := cniapi.CniCmdErrorResp{CniVersion: cniapi.CniDefaultVersion,
		ErrCode: cniapi.CniStatusErrorUnsupportedField,
		ErrMsg:  "contiv: " + errorMsg}

	jsonResp, err := json.Marshal(cniResp)
	if err == nil {
		fmt.Printf("%s\n", string(jsonResp))
		cniLog.Infof("CNI error response: %s", string(jsonResp))
	} else {
		cniLog.Errorf("failed to convert CNI error response to JSON, %s ", err)
		// send minimal response to stdout
		fmt.Printf("{ \n")
		fmt.Printf("\"cniVersion\": \"%s\" \n", cniResp.CniVersion)
		fmt.Printf("\"code\": \"%d\" \n", cniResp.ErrCode)
		fmt.Printf("\"msg\": \"%s %s\" \n", "contiv", cniResp.ErrMsg)
		fmt.Printf("} \n")
	}
	return int(cniResp.ErrCode)
}

// handle http req & response to netplugin
func (cniApp *cniAppInfo) handleHTTP(url string, jsonReq *bytes.Buffer) int {

	cniLog.Infof("http POST url: %s data: %v", url, jsonReq)
	httpResp, err := cniApp.httpClient.Post(url, "application/json", jsonReq)
	if err != nil {
		return cniApp.sendCniErrorResp("failed to get response from netplugin :" + err.Error())
	}
	defer httpResp.Body.Close()

	switch httpResp.StatusCode {

	case http.StatusOK:
		cniLog.Infof("received http OK response from netplugin")
		info, err := ioutil.ReadAll(httpResp.Body)
		if err != nil {
			return cniApp.sendCniErrorResp("failed to read success response from netplugin :" + err.Error())
		}
		return cniApp.sendCniResp(info, cniapi.CniStatusSuccess)

	case http.StatusInternalServerError:
		cniLog.Infof("received http error response from netplugin")
		info, err := ioutil.ReadAll(httpResp.Body)
		if err != nil {
			return cniApp.sendCniErrorResp("failed to read error response from netplugin :" + err.Error())
		}
		return cniApp.sendCniResp(info, cniapi.CniStatusErrorUnsupportedField)

	default:
		cniLog.Infof("received unknown error from netplugin")
		return cniApp.sendCniErrorResp("error response from netplugin: " + http.StatusText(httpResp.StatusCode))
	}
}

// process CNI ADD/DEL commands
func (cniApp *cniAppInfo) processCmd() int {
	urlDict := map[string]string{cniapi.CniCmdAdd: cniApp.serverURL + cniapi.MesosNwIntfAdd,
		cniapi.CniCmdDel: cniApp.serverURL + cniapi.MesosNwIntfDel}

	jsonReq, err := json.Marshal(cniApp.cniMesosAttr)
	if err != nil {
		return cniApp.sendCniErrorResp("failed to convert Mesos attr. to JSON format :" + err.Error())
	}

	jsonBuf := bytes.NewBuffer(jsonReq)
	url := urlDict[cniApp.cniCmd]
	cniLog.Infof("process cni cmd: %s url: %s", cniApp.cniCmd, url)
	return cniApp.handleHTTP(url, jsonBuf)
}

// initialize netplugin client
func (cniApp *cniAppInfo) init() {
	cniApp.serverURL = "http://localhost"

	trans := &http.Transport{Dial: func(network, addr string) (net.Conn,
		error) {
		return net.Dial("unix", cniapi.ContivMesosSocket)
	}}
	cniApp.httpClient = &http.Client{Transport: trans}
}

func main() {
	var pluginVersion bool
	cniApp := &cniAppInfo{}

	if fileStat, err := os.Stat(logfileName); err == nil {
		if fileStat.Size() >= logfileSize {
			os.Rename(logfileName, fmt.Sprintf("%s%s", logfileName, ".old"))
		}
	}

	logFd, err := os.OpenFile(logfileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		logger.Fatalf("%s", err)
	}

	defer logFd.Close()
	logger.SetOutput(logFd)
	cniApp.init()

	logger.Infof("===[%s] starting Contiv CNI plugin %s for Mesos   ===",
		time.Now(), os.Args[0])
	logger.Infof("%s", version.String())
	logger.Infof("CNI version : %s", cniapi.CniDefaultVersion)

	flagSet := flag.NewFlagSet("netcontiv", flag.ExitOnError)
	flagSet.BoolVar(&pluginVersion,
		"version",
		false,
		"show version")

	if err := flagSet.Parse(os.Args[1:]); err != nil {
		fmt.Printf("failed to parse command, Error: %s", err)
	}

	if pluginVersion {
		fmt.Printf("Contiv netplugin version : %s", version.String())
		logger.Infof("CNI version : %s", cniapi.CniDefaultVersion)
		os.Exit(0)
	}

	if err := cniApp.parseEnv(); err != nil {
		logger.Errorf("%s", err)
		os.Exit(1)
	}
	cniApp.parseNwInfoLabels()
	retCode := cniApp.processCmd()
	cniLog.Infof("cni return code: %d", retCode)
	os.Exit(retCode)
}
