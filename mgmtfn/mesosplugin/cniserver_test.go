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
	"os"
	"os/exec"
	"strconv"
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

func TestParseCniArgs(t *testing.T) {
	cniLog = log.WithField("test", "mesos-test")
	testArgs := []cniapi.CniCmdReqAttr{
		{CniIfname: "eth0", CniNetns: "ns1234", CniContainerid: "123456",
			Labels: cniapi.NetpluginLabel{TenantName: "test-tenant", NetworkName: "test-nw",
				NetworkGroup: "test-epg"}},
		{CniIfname: "eth0", CniNetns: "ns1234", CniContainerid: "123456",
			Labels: cniapi.NetpluginLabel{NetworkName: "test-nw",
				NetworkGroup: "test-epg"}},
		{CniIfname: "eth0", CniNetns: "ns1234", CniContainerid: "123456",
			Labels: cniapi.NetpluginLabel{
				NetworkGroup: "test-epg"}},
		{CniIfname: "eth0", CniNetns: "ns1234", CniContainerid: "123456"},
	}

	for _, arg1 := range testArgs {
		testServer := cniServer{}
		jReq, err := json.Marshal(&arg1)
		cniAssert(t, err != nil, fmt.Sprintf("json conversion failed %s", err))
		err = testServer.parseCniArgs(jReq)
		cniAssert(t, err != nil, fmt.Sprintf("parse failed %s", err))
		cniAssert(t, testServer.pluginArgs.CniIfname != arg1.CniIfname,
			fmt.Sprintf("expected ifname %s, got %s", arg1.CniIfname, testServer.pluginArgs.CniIfname))
		cniAssert(t, testServer.pluginArgs.CniNetns != arg1.CniNetns,
			fmt.Sprintf("expected netns %s, got %s", arg1.CniNetns, testServer.pluginArgs.CniNetns))
		cniAssert(t, testServer.pluginArgs.CniContainerid != arg1.CniContainerid,
			fmt.Sprintf("expected container-id %s, got %s", arg1.CniContainerid,
				testServer.pluginArgs.CniContainerid))
		cniAssert(t, testServer.pluginArgs.Labels.TenantName != arg1.Labels.TenantName,
			fmt.Sprintf("expected tenant name %s, got %s", arg1.Labels.TenantName,
				testServer.pluginArgs.Labels.TenantName))
		cniAssert(t, testServer.pluginArgs.Labels.NetworkName != arg1.Labels.NetworkName,
			fmt.Sprintf("expected network name %s, got %s", arg1.Labels.NetworkName,
				testServer.pluginArgs.Labels.NetworkName))
		cniAssert(t, testServer.pluginArgs.Labels.NetworkGroup != arg1.Labels.NetworkGroup,
			fmt.Sprintf("expected network group %s, got %s", arg1.Labels.NetworkGroup,
				testServer.pluginArgs.Labels.NetworkGroup))
		// check defaults
		if len(arg1.Labels.TenantName) > 0 {
			cniAssert(t, testServer.endPointLabels[cniapi.LabelTenantName] != arg1.Labels.TenantName,
				fmt.Sprintf("[%s] expected %s gpt %s", cniapi.LabelTenantName, arg1.Labels.TenantName,
					testServer.endPointLabels[cniapi.LabelTenantName]))
		} else {
			cniAssert(t, testServer.endPointLabels[cniapi.LabelTenantName] != "default",
				fmt.Sprintf("invalid default tenant %s",
					testServer.endPointLabels[cniapi.LabelTenantName]))
		}
		if len(arg1.Labels.NetworkName) > 0 {
			cniAssert(t, testServer.endPointLabels[cniapi.LabelNetworkName] != arg1.Labels.NetworkName,
				fmt.Sprintf("[%s] expected %s gpt %s", cniapi.LabelNetworkName, arg1.Labels.NetworkName,
					testServer.endPointLabels[cniapi.LabelNetworkName]))

		} else {
			cniAssert(t, testServer.endPointLabels[cniapi.LabelNetworkName] != "default-net",
				fmt.Sprintf("invalid default network name %s",
					testServer.endPointLabels[cniapi.LabelNetworkName]))
		}

		//fdn
		cniAssert(t, testServer.networkID != testServer.endPointLabels[cniapi.LabelNetworkName]+"."+
			testServer.endPointLabels[cniapi.LabelTenantName],
			fmt.Sprintf("invalid networkId id %s", testServer.networkID))

		cniAssert(t, testServer.endpointID != testServer.networkID+"-"+arg1.CniContainerid,
			fmt.Sprintf("invalid endpoint id %s", testServer.endpointID))

	}

}

func TestParseMesosAgentIPAddr(t *testing.T) {
	cniLog = log.WithField("test", "mesos-test")
	cmd := exec.Command("sleep", "0.5")
	ipaddr := "192.168.2.1"
	cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", cniapi.EnvVarMesosAgent, ipaddr))
	err := cmd.Start()
	cniAssert(t, err != nil, fmt.Sprintf("failed to start test process, %s", err))
	pidList := strconv.Itoa(cmd.Process.Pid)
	cniLog.Infof("parse pids %s %d", pidList)
	parsedIP, err1 := parseMesosAgentIPAddr([]byte(pidList))
	cniAssert(t, err1 != nil, fmt.Sprintf("%s", err))
	cniAssert(t, parsedIP != ipaddr, fmt.Sprintf("expected %s got %s", ipaddr, parsedIP))
	err = cmd.Wait()
	cniAssert(t, err != nil, fmt.Sprintf("%s", err))
}
