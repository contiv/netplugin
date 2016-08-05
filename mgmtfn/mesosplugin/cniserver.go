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

package mesosplugin

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/mgmtfn/mesosplugin/cniapi"
	"github.com/contiv/netplugin/netplugin/plugin"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net"
	"net/http"
	"os"
)

type cniServer struct {
	networkID      string
	endpointID     string
	endPointLabels map[string]string
	ipv4Addr       string
	pluginArgs     cniapi.CniCmdReqAttr
	cniSuccessResp cniapi.CniCmdSuccessResp
}

type httpAPIFunc func(httpBody []byte) ([]byte, error)

var cniLog *log.Entry

// HTTP wrapper
func httpWrapper(handlerFunc httpAPIFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if httpBody, err := ioutil.ReadAll(r.Body); err == nil {
			cniLog.Infof("==== received new http req %s ======", string(httpBody))
			jsonResp, hErr := handlerFunc(httpBody)
			if hErr != nil {
				cniLog.Infof("sending http Error response %s", string(jsonResp))
				w.WriteHeader(http.StatusInternalServerError)
			} else {
				cniLog.Infof("sending http OK response %s", string(jsonResp))
				w.WriteHeader(http.StatusOK)
			}
			w.Write(jsonResp)

		} else {
			errResp, _ := createCniErrorResponse(fmt.Errorf("failed to read http body: %s", err))
			w.WriteHeader(http.StatusInternalServerError)
			w.Write(errResp)
		}
		w.Header().Set("Content-Type", "application/json")
	}
}

func createCniErrorResponse(errmsg error) ([]byte, error) {
	// CNI_ERROR_UNSUPPORTED is sent for all errors
	cniResp := cniapi.CniCmdErrorResp{CniVersion: cniapi.CniDefaultVersion,
		ErrCode: cniapi.CniStatusErrorUnsupportedField,
		ErrMsg:  fmt.Sprintf("Contiv: %s", errmsg.Error())}

	cniLog.Infof("sending error response: %s", cniResp.ErrMsg)
	jsonResp, err := json.Marshal(&cniResp)
	if err != nil {
		cniLog.Errorf("failed to convert error response to JSON: %s ", err.Error())
		return nil, err
	}
	return jsonResp, errmsg

}

func (cniReq *cniServer) createCniSuccessResponse() ([]byte, error) {
	jsonResp, err := json.Marshal(&cniReq.cniSuccessResp)
	if err != nil {
		cniLog.Errorf("failed to convert success response to JSON: %s ", err.Error())
		return nil, err
	}
	return jsonResp, nil
}

// parse & save labels
func (cniReq *cniServer) parseCniArgs(httpBody []byte) error {

	if err := json.Unmarshal(httpBody, &cniReq.pluginArgs); err != nil {
		return fmt.Errorf("failed to parse JSON req: %s", err.Error())
	}

	cniLog.Debugf("parsed ifname: %s, netns: %s, container-id: %s,"+
		"tenant: %s, network-name: %s, network-group: %s",
		cniReq.pluginArgs.CniIfname, cniReq.pluginArgs.CniNetns,
		cniReq.pluginArgs.CniContainerid,
		cniReq.pluginArgs.Labels.TenantName,
		cniReq.pluginArgs.Labels.NetworkName,
		cniReq.pluginArgs.Labels.NetworkGroup)

	// set defaults
	cniReq.endPointLabels = map[string]string{cniapi.LabelNetworkName: "default-net",
		cniapi.LabelTenantName: "default"}

	for _, label := range []string{cniapi.LabelTenantName, cniapi.LabelNetworkName,
		cniapi.LabelNetworkGroup} {

		switch label {
		case cniapi.LabelTenantName:
			if len(cniReq.pluginArgs.Labels.TenantName) > 0 {
				cniReq.endPointLabels[label] = cniReq.pluginArgs.Labels.TenantName
			}
			cniLog.Infof("netplugin label %s = %s", cniapi.LabelTenantName,
				cniReq.endPointLabels[label])

		case cniapi.LabelNetworkName:
			if len(cniReq.pluginArgs.Labels.NetworkName) > 0 {
				cniReq.endPointLabels[label] = cniReq.pluginArgs.Labels.NetworkName
			}
			cniLog.Infof("netplugin label %s = %s", cniapi.LabelNetworkName,
				cniReq.endPointLabels[label])

		case cniapi.LabelNetworkGroup:
			if len(cniReq.pluginArgs.Labels.NetworkGroup) > 0 {
				cniReq.endPointLabels[label] = cniReq.pluginArgs.Labels.NetworkGroup
			}
			cniLog.Infof("netplugin label %s = %s", cniapi.LabelNetworkGroup,
				cniReq.endPointLabels[label])
		}
	}

	cniReq.networkID = cniReq.endPointLabels[cniapi.LabelNetworkName] + "." +
		cniReq.endPointLabels[cniapi.LabelTenantName]
	cniLog.Infof("network fdn  %s", cniReq.networkID)
	cniReq.endpointID = cniReq.networkID + "-" +
		cniReq.pluginArgs.CniContainerid
	cniLog.Infof("endpoint fdn %s", cniReq.endpointID)
	return nil
}

func mesosNwIntfAdd(httpBody []byte) ([]byte, error) {
	addReq := cniServer{}

	cniLog.Infof("process intf add req ")

	if err := addReq.parseCniArgs(httpBody); err != nil {
		return createCniErrorResponse(err)
	}

	if err := addReq.createCniEndPoint(); err != nil {
		return createCniErrorResponse(err)
	}

	// success
	return addReq.createCniSuccessResponse()
}

func mesosNwIntfDel(httpBody []byte) ([]byte, error) {
	retSuccess := []byte("")
	delReq := cniServer{}

	cniLog.Infof("process intf delete req")

	if err := delReq.parseCniArgs(httpBody); err != nil {
		return createCniErrorResponse(err)
	}

	if err := delReq.deleteCniEndPoint(); err != nil {
		return createCniErrorResponse(err)
	}

	return retSuccess, nil
}

func unknownReq(w http.ResponseWriter, r *http.Request) {
	cniLog.Infof("unknown http request at %q", r.URL.Path)
	content, _ := ioutil.ReadAll(r.Body)
	cniLog.Debugf("received http body content: %s", string(content))
	w.WriteHeader(http.StatusServiceUnavailable)
}

// InitPlugin registers REST endpoints to handle requests from Mesos CNI plugins
func InitPlugin(netPlugin *plugin.NetPlugin) {

	cniLog = log.WithField("plugin", "mesos")
	cniLog.Infof("starting Mesos CNI server")
	router := mux.NewRouter()

	// register handlers for cni plugin
	subRtr := router.Headers("Content-Type", "application/json").Subrouter()
	subRtr.HandleFunc(cniapi.MesosNwIntfAdd, httpWrapper(mesosNwIntfAdd)).Methods("POST")
	subRtr.HandleFunc(cniapi.MesosNwIntfDel, httpWrapper(mesosNwIntfDel)).Methods("POST")
	router.HandleFunc("/{*}", unknownReq)

	sockFile := cniapi.ContivMesosSocket
	os.Remove(sockFile)
	os.MkdirAll(cniapi.PluginPath, 0700)

	cniDriverInit(netPlugin)

	go func() {
		lsock, err := net.ListenUnix("unix", &net.UnixAddr{Name: sockFile, Net: "unix"})
		if err != nil {
			cniLog.Errorf("Mesos CNI server failed:  %s", err)
			return
		}

		cniLog.Infof("Mesos CNI server is listening on %s", sockFile)
		http.Serve(lsock, router)
		lsock.Close()
		cniLog.Infof("Mesos CNI server socket %s closed ", sockFile)
	}()
}
