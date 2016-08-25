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

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/contiv/contivmodel/client"
	"github.com/contiv/netplugin/mgmtfn/mesosplugin/cniapi"
	"github.com/samalba/dockerclient"
	. "gopkg.in/check.v1"
	"io/ioutil"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// track containers
var containerInfo = map[string]cniapi.CniCmdReqAttr{}
var intLog = log.New()
var docker *dockerclient.DockerClient

type testConfigData struct {
	name          string
	result        bool   // expected test result
	containerName string // create container with name
	operation     string // add or del
	tenantName    string
	networkName   string
	networkGroup  string
	subnet        string
	cleanup       bool // cleanup network/tenant/epg
	agentIPAddr   string
	reqAttr       cniapi.CniCmdReqAttr
	successResp   cniapi.CniCmdSuccessResp
	errResp       cniapi.CniCmdErrorResp
}

func assertOnTrue(c *C, val bool, msg string) {
	if val == true {
		c.Fatalf("Error %s", msg)
	}
	// else continue
}

func (cfg1 *testConfigData) createTenant(its *integTestSuite, c *C) {

	if (len(cfg1.tenantName) == 0) || (cfg1.tenantName == "default") {
		return
	}

	if _, err := its.client.TenantGet(cfg1.tenantName); err != nil {
		intLog.Infof("create tenant %s", cfg1.tenantName)
		err := its.client.TenantPost(&client.Tenant{TenantName: cfg1.tenantName})
		assertNoErr(err, c, "create tenant")
	} else {
		intLog.Infof("tenant %s exists", cfg1.tenantName)
	}
}

func (cfg1 *testConfigData) deleteTenant(its *integTestSuite, c *C) {

	if (len(cfg1.tenantName) == 0) || (cfg1.tenantName == "default") {
		return
	}

	if _, err := its.client.TenantGet(cfg1.tenantName); err == nil {
		intLog.Infof("delete tenant %s", cfg1.tenantName)
		err := its.client.TenantDelete(cfg1.tenantName)
		assertNoErr(err, c, "creating tenant")
	} else {
		intLog.Warnf("no tenant %s", cfg1.tenantName)
	}
}

func (cfg1 *testConfigData) createNetwork(its *integTestSuite, c *C) {

	if len(cfg1.networkName) == 0 {
		return
	}

	if _, err := its.client.NetworkGet(cfg1.tenantName, cfg1.networkName); err != nil {
		intLog.Infof("creating network %s", cfg1.networkName)
		if len(cfg1.tenantName) > 0 {
			err := its.client.NetworkPost(&client.Network{
				TenantName:  cfg1.tenantName,
				NetworkName: cfg1.networkName,
				Subnet:      cfg1.subnet,
				Encap:       its.encap,
			})
			assertNoErr(err, c, "creating network")
		} else {
			err := its.client.NetworkPost(&client.Network{
				TenantName:  "default",
				NetworkName: cfg1.networkName,
				Subnet:      cfg1.subnet,
				Encap:       its.encap,
			})
			assertNoErr(err, c, "creating network")
		}

	} else {
		intLog.Infof("network %s exists", cfg1.networkName)
	}
}

func (cfg1 *testConfigData) deleteNetwork(its *integTestSuite, c *C) {

	if len(cfg1.networkName) == 0 {
		return
	}

	if _, err := its.client.NetworkGet(cfg1.tenantName, cfg1.networkName); err == nil {
		intLog.Infof("delete network %s", cfg1.networkName)
		if len(cfg1.tenantName) > 0 {
			err := its.client.NetworkDelete(cfg1.tenantName, cfg1.networkName)
			assertNoErr(err, c, "delete network")
		} else {
			err := its.client.NetworkDelete("default", cfg1.networkName)
			assertNoErr(err, c, "delete network")
		}

	} else {
		intLog.Warnf("no network %s", cfg1.networkName)
	}
}

func (cfg1 *testConfigData) verifyEndpoint(its *integTestSuite, c *C, resp cniapi.CniCmdSuccessResp) {

	endpoint, epErr := its.client.EndpointInspect(cfg1.reqAttr.CniContainerid)

	if (cfg1.result != false) && (cfg1.operation == cniapi.CniCmdAdd) {
		assertNoErr(epErr, c, fmt.Sprintf("epg not found for  %s", cfg1.reqAttr.CniContainerid))
		assertOnTrue(c, endpoint.Oper.IpAddress[0] != strings.Split(resp.IP4.IPAddress, "/")[0],
			fmt.Sprintf("expected %s, got %s", endpoint.Oper.IpAddress, resp.IP4.IPAddress))
		out, err := exec.Command("ip", "netns", "exec", cfg1.reqAttr.CniContainerid,
			"ip", "route", "get", cfg1.agentIPAddr).CombinedOutput()
		assertNoErr(err, c, fmt.Sprintf("failed to get host route"))
		rlist := strings.Fields(string(out))
		assertOnTrue(c, rlist[0] != cfg1.agentIPAddr, fmt.Sprintf("invalid route %v", rlist))
		assertOnTrue(c, strings.Contains(rlist[2], "hport") != true,
			fmt.Sprintf("invalid host route %v", rlist))
	} else {
		assertOnTrue(c, epErr == nil, fmt.Sprintf("epg still exist, error: %s, %+v", epErr, endpoint))
		out, err := exec.Command("ip", "netns", "exec", cfg1.reqAttr.CniContainerid,
			"ip", "route", "get", cfg1.agentIPAddr).CombinedOutput()
		assertOnTrue(c, err == nil, fmt.Sprintf("namespace %s still exist %s", cfg1.reqAttr.CniContainerid,
			string(out)))
		out, err1 := exec.Command("ip", "netns", "list").CombinedOutput()
		assertNoErr(err1, c, "ip netns list")
		if len(cfg1.reqAttr.CniContainerid) > 0 && strings.Contains(string(out), cfg1.reqAttr.CniContainerid) {
			exec.Command("ip", "netns", "delete", cfg1.reqAttr.CniContainerid).CombinedOutput()
			assertOnTrue(c, true, fmt.Sprintf("name space exists %s", string(out)))
		}
	}
}

func (cfg1 *testConfigData) createEPG(its *integTestSuite, c *C) {

	if len(cfg1.networkGroup) == 0 {
		return
	}

	if _, err := its.client.EndpointGroupGet(cfg1.tenantName, cfg1.networkGroup); err != nil {
		intLog.Infof("creating endpoint group %s", cfg1.networkGroup)
		err := its.client.EndpointGroupPost(&client.EndpointGroup{
			TenantName:  cfg1.tenantName,
			GroupName:   cfg1.networkGroup,
			NetworkName: cfg1.networkName,
		})
		assertNoErr(err, c, "creating EPG")
	} else {
		intLog.Infof("EPG %s exists", cfg1.networkGroup)
	}
}

func (cfg1 *testConfigData) deleteEPG(its *integTestSuite, c *C) {

	if len(cfg1.networkGroup) == 0 {
		return
	}

	if _, err := its.client.EndpointGroupGet(cfg1.tenantName, cfg1.networkGroup); err == nil {
		intLog.Infof("delete endpoint group %s", cfg1.networkGroup)
		err := its.client.EndpointGroupDelete(cfg1.tenantName, cfg1.networkGroup)
		assertNoErr(err, c, "delete EPG")
	} else {
		intLog.Warnf("no EPG %s", cfg1.networkGroup)
	}
}

func (cfg1 *testConfigData) runContainer(its *integTestSuite, c *C) {

	if len(cfg1.containerName) == 0 {
		return
	}

	// check if container exists
	if stInfo, ok := containerInfo[cfg1.containerName]; ok {
		cfg1.reqAttr = stInfo
		return
	}

	containerName := cfg1.containerName
	intLog.Infof("creating container: %s", containerName)

	err := fmt.Errorf("")
	for c := 0; c < 10 && err != nil; c++ {
		// could fail in docker 1.10
		if err = docker.PullImage("alpine", nil); err != nil {
			intLog.Warnf("failed to pull alpine, %s", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	assertNoErr(err, c, "pull alpine image")

	// Create a container
	containerConfig := &dockerclient.ContainerConfig{
		Image: "alpine",
		// self clean up after a few sec.
		Cmd:             []string{"sleep", "60"},
		AttachStdin:     false,
		NetworkDisabled: true,
		Env: []string{fmt.Sprintf("%s=%s", cniapi.EnvVarMesosAgent,
			cfg1.agentIPAddr)},
		Tty: false}

	containerID, err := docker.CreateContainer(containerConfig,
		containerName, nil)
	assertNoErr(err, c, fmt.Sprintf("create container %s", containerName))

	// Start the container
	hostConfig := &dockerclient.HostConfig{}
	err = docker.StartContainer(containerID, hostConfig)
	assertNoErr(err, c, fmt.Sprintf("start container %s", containerName))

	cfg1.reqAttr.CniContainerid = containerID
	inspect, err := docker.InspectContainer(containerID)
	assertNoErr(err, c, fmt.Sprintf("inspect container %s", containerName))
	cfg1.reqAttr.CniNetns = fmt.Sprintf("/proc/%d/ns/net", inspect.State.Pid)
	containerInfo[containerName] = cfg1.reqAttr
	intLog.Infof("test container %s created %+v", containerID, containerInfo)
	intLog.Infof("containers : %+v", containerInfo)
}

func (cfg1 *testConfigData) destroyContainer(its *integTestSuite, c *C) {
	if len(cfg1.containerName) == 0 {
		return
	}

	// check if container exists
	if _, ok := containerInfo[cfg1.containerName]; !ok {
		intLog.Warnf("no conainer with name %s", cfg1.containerName)
		return
	}

	cinfo := &cfg1.reqAttr
	intLog.Infof("stop container %s", cinfo.CniContainerid)
	err := docker.StopContainer(cinfo.CniContainerid, 0)
	assertNoErr(err, c, fmt.Sprintf("stop container %s", cinfo.CniContainerid))
	err = docker.RemoveContainer(cinfo.CniContainerid, true, true)
	assertNoErr(err, c, fmt.Sprintf("remove container %s", cinfo.CniContainerid))
	delete(containerInfo, cfg1.containerName)
}

func cleanupContainers() {
	intLog.Infof("######### cleaning containers ########")
	for containerName := range containerInfo {
		intLog.Infof("cleanup container %s", containerName)
		if err := docker.StopContainer(containerName, 0); err != nil {
			intLog.Warnf("failed to stop container %s, %s", containerName, err)
		}
		if err := docker.RemoveContainer(containerName, true, true); err != nil {
			intLog.Warnf("failed to remove container %s,  %s", containerName, err)
		}
	}
}

// handle http req & response to netplugin
func processHTTP(c *C, url string, jsonReq *bytes.Buffer) (int, []byte) {

	trans := &http.Transport{Dial: func(network, addr string) (net.Conn,
		error) {
		return net.Dial("unix", cniapi.ContivMesosSocket)
	}}

	httpClient := &http.Client{Transport: trans}

	intLog.Infof("http POST url: %s data: %v", url, jsonReq)
	httpResp, err := httpClient.Post(url, "application/json", jsonReq)
	assertNoErr(err, c, "post http ")

	defer httpResp.Body.Close()

	switch httpResp.StatusCode {

	case http.StatusOK:
		intLog.Infof("received http OK reponse from netplugin")
		info, err := ioutil.ReadAll(httpResp.Body)
		assertNoErr(err, c, "receive http data")
		return httpResp.StatusCode, info

	case http.StatusInternalServerError:
		intLog.Infof("received http error reponse from netplugin")
		info, err := ioutil.ReadAll(httpResp.Body)
		assertNoErr(err, c, "receive http data")
		return httpResp.StatusCode, info

	default:
		intLog.Errorf("received unknown error from netplugin")
		assertNoErr(fmt.Errorf("unknwon error from netplugin"), c, "unknown")
	}

	return 0, nil
}

type intgFmt struct{}

func (t *intgFmt) Format(e *log.Entry) ([]byte, error) {
	e.Message = strings.Join([]string{"[INTG-TEST]", e.Message}, " ")
	nt := log.TextFormatter{}
	return nt.Format(e)
}

// TestMesosCniEndpoints : test cni endpoints
func (its *integTestSuite) TestMesosCniEndpoints(c *C) {
	intLog.Formatter = new(intgFmt)
	defer cleanupContainers()

	defaultCfg := []testConfigData{

		// tenantName= networkName=, epg=
		{
			name:   "default-1",
			result: true, cleanup: false, containerName: "mesos-default-c1", operation: cniapi.CniCmdAdd,
			tenantName: "default", networkName: "default-net", networkGroup: "epg-default",
			subnet: "10.36.1.0/24", agentIPAddr: "172.20.10.101",
			reqAttr: cniapi.CniCmdReqAttr{
				CniIfname: "eth0",
				Labels: cniapi.NetpluginLabel{
					TenantName:   "",
					NetworkName:  "",
					NetworkGroup: "",
				},
			},
		},

		// cleanup tenantName= networkName=, epg=
		{
			name:   "default-2",
			result: true, cleanup: true, containerName: "mesos-default-c1", operation: cniapi.CniCmdDel,
			tenantName: "default", networkName: "default-net", networkGroup: "epg-default",
			subnet: "10.36.1.0/24", agentIPAddr: "172.20.10.101",
			reqAttr: cniapi.CniCmdReqAttr{
				CniIfname: "eth0",
				Labels: cniapi.NetpluginLabel{
					TenantName:   "",
					NetworkName:  "",
					NetworkGroup: "",
				},
			},
		},

		// tenantName=default networkName=, epg=
		{
			name:   "default-3",
			result: true, cleanup: false, containerName: "mesos-default-c1", operation: cniapi.CniCmdAdd,
			tenantName: "default", networkName: "default-net", networkGroup: "epg-default",
			subnet: "10.36.1.0/24", agentIPAddr: "172.20.10.101",
			reqAttr: cniapi.CniCmdReqAttr{
				CniIfname: "eth0",
				Labels: cniapi.NetpluginLabel{
					TenantName:   "default",
					NetworkName:  "",
					NetworkGroup: "",
				},
			},
		},

		// cleanup tenantName=default networkName=, epg=
		{
			name:   "default-4",
			result: true, cleanup: true, containerName: "mesos-default-c1", operation: cniapi.CniCmdDel,
			tenantName: "default", networkName: "default-net", networkGroup: "epg-default",
			subnet: "10.36.1.0/24", agentIPAddr: "172.20.10.101",
			reqAttr: cniapi.CniCmdReqAttr{
				CniIfname: "eth0",
				Labels: cniapi.NetpluginLabel{
					TenantName:   "default",
					NetworkName:  "",
					NetworkGroup: "",
				},
			},
		},

		// tenantName= networkName="default-net", epg=
		{
			name:   "default-5",
			result: true, cleanup: false, containerName: "mesos-default-c1", operation: cniapi.CniCmdAdd,
			tenantName: "default", networkName: "default-net", networkGroup: "epg-default",
			subnet: "10.36.1.0/24", agentIPAddr: "172.20.10.101",
			reqAttr: cniapi.CniCmdReqAttr{
				CniIfname: "eth0",
				Labels: cniapi.NetpluginLabel{
					TenantName:   "",
					NetworkName:  "default-net",
					NetworkGroup: "",
				},
			},
		},

		// cleanup tenantName= networkName="default-net", epg=
		{
			name:   "default-6",
			result: true, cleanup: true, containerName: "mesos-default-c1", operation: cniapi.CniCmdDel,
			tenantName: "default", networkName: "default-net", networkGroup: "epg-default",
			subnet: "10.36.1.0/24", agentIPAddr: "172.20.10.101",
			reqAttr: cniapi.CniCmdReqAttr{
				CniIfname: "eth0",
				Labels: cniapi.NetpluginLabel{
					TenantName:   "",
					NetworkName:  "default-net",
					NetworkGroup: "",
				},
			},
		},

		// tenantName= networkName=, epg=epg-default
		{
			name:   "default-7",
			result: true, cleanup: false, containerName: "mesos-default-c1", operation: cniapi.CniCmdAdd,
			tenantName: "default", networkName: "default-net", networkGroup: "epg-default",
			subnet: "10.36.1.0/24", agentIPAddr: "172.20.10.101",
			reqAttr: cniapi.CniCmdReqAttr{
				CniIfname: "eth0",
				Labels: cniapi.NetpluginLabel{
					TenantName:   "",
					NetworkName:  "",
					NetworkGroup: "epg-default",
				},
			},
		},

		// cleanup tenantName= networkName=, epg=epg-default
		{
			name:   "default-8",
			result: true, cleanup: true, containerName: "mesos-default-c1", operation: cniapi.CniCmdDel,
			tenantName: "default", networkName: "default-net", networkGroup: "epg-default",
			subnet: "10.36.1.0/24", agentIPAddr: "172.20.10.101",
			reqAttr: cniapi.CniCmdReqAttr{
				CniIfname: "eth0",
				Labels: cniapi.NetpluginLabel{
					TenantName:   "",
					NetworkName:  "",
					NetworkGroup: "epg-default",
				},
			},
		},

		// tenantName=default networkName=default-net, epg=
		{
			name:   "default-9",
			result: true, cleanup: false, containerName: "mesos-default-c1", operation: cniapi.CniCmdAdd,
			tenantName: "default", networkName: "default-net", networkGroup: "epg-default",
			subnet: "10.36.1.0/24", agentIPAddr: "172.20.10.101",
			reqAttr: cniapi.CniCmdReqAttr{
				CniIfname: "eth0",
				Labels: cniapi.NetpluginLabel{
					TenantName:   "default",
					NetworkName:  "default-net",
					NetworkGroup: "",
				},
			},
		},

		// cleanup tenantName=default networkName=default-net, epg=
		{
			name:   "default-10",
			result: true, cleanup: true, containerName: "mesos-default-c1", operation: cniapi.CniCmdDel,
			tenantName: "default", networkName: "default-net", networkGroup: "epg-default",
			subnet: "10.36.1.0/24", agentIPAddr: "172.20.10.101",
			reqAttr: cniapi.CniCmdReqAttr{
				CniIfname: "eth0",
				Labels: cniapi.NetpluginLabel{
					TenantName:   "default",
					NetworkName:  "default-net",
					NetworkGroup: "",
				},
			},
		},

		// tenantName=default networkName= epg=epg-default
		{
			name:   "default-11",
			result: true, cleanup: false, containerName: "mesos-default-c1", operation: cniapi.CniCmdAdd,
			tenantName: "default", networkName: "default-net", networkGroup: "epg-default",
			subnet: "10.36.1.0/24", agentIPAddr: "172.20.10.101",
			reqAttr: cniapi.CniCmdReqAttr{
				CniIfname: "eth0",
				Labels: cniapi.NetpluginLabel{
					TenantName:   "",
					NetworkName:  "",
					NetworkGroup: "epg-default",
				},
			},
		},

		// cleanup tenantName=default networkName= epg=epg-default
		{
			name:   "default-12",
			result: true, cleanup: true, containerName: "mesos-default-c1", operation: cniapi.CniCmdDel,
			tenantName: "default", networkName: "default-net", networkGroup: "epg-default",
			subnet: "10.36.1.0/24", agentIPAddr: "172.20.10.101",
			reqAttr: cniapi.CniCmdReqAttr{
				CniIfname: "eth0",
				Labels: cniapi.NetpluginLabel{
					TenantName:   "",
					NetworkName:  "",
					NetworkGroup: "epg-default",
				},
			},
		},

		// tenantName=default networkName=default-net epg=epg-default
		{
			name:   "default-13",
			result: true, cleanup: false, containerName: "mesos-default-c1", operation: cniapi.CniCmdAdd,
			tenantName: "default", networkName: "default-net", networkGroup: "epg-default",
			subnet: "10.36.1.0/24", agentIPAddr: "172.20.10.101",
			reqAttr: cniapi.CniCmdReqAttr{
				CniIfname: "eth0",
				Labels: cniapi.NetpluginLabel{
					TenantName:   "default",
					NetworkName:  "default-net",
					NetworkGroup: "epg-default",
				},
			},
		},

		// cleanup tenantName=default networkName=default-net epg=epg-default
		{
			name:   "default-14",
			result: true, cleanup: true, containerName: "mesos-default-c1", operation: cniapi.CniCmdDel,
			tenantName: "default", networkName: "default-net", networkGroup: "epg-default",
			subnet: "10.36.1.0/24", agentIPAddr: "172.20.10.101",
			reqAttr: cniapi.CniCmdReqAttr{
				CniIfname: "eth0",
				Labels: cniapi.NetpluginLabel{
					TenantName:   "default",
					NetworkName:  "default-net",
					NetworkGroup: "epg-default",
				},
			},
		},
	}

	dkc, err := dockerclient.NewDockerClient("http://localhost:2385", nil)
	assertNoErr(err, c, "new docker client")
	docker = dkc

	for i, cfg1 := range defaultCfg {
		if strings.Split(cfg1.name, "-")[1] != strconv.Itoa(i+1) {
			intLog.Fatalf("invalid test case number %s", cfg1.name)
		}
		executeEndpointTest(its, c, cfg1)
	}

	successCfg := []testConfigData{
		// tenantName=ten1 networkName=net1, epg=
		{
			name:   "eps-1",
			result: true, cleanup: false, containerName: "mesos-eps-c1", operation: cniapi.CniCmdAdd,
			tenantName: "ten1", networkName: "net1", networkGroup: "",
			subnet: "10.36.2.0/24", agentIPAddr: "172.20.10.101",
			reqAttr: cniapi.CniCmdReqAttr{
				CniIfname: "eth0",
				Labels: cniapi.NetpluginLabel{
					TenantName:   "ten1",
					NetworkName:  "net1",
					NetworkGroup: "",
				},
			},
		},

		// cleanup tenantName=ten1 networkName=net1, epg=
		{
			name:   "eps-2",
			result: true, cleanup: true, containerName: "mesos-eps-c1", operation: cniapi.CniCmdDel,
			tenantName: "ten1", networkName: "net1", networkGroup: "",
			subnet: "10.36.2.0/24", agentIPAddr: "172.20.10.101",
			reqAttr: cniapi.CniCmdReqAttr{
				CniIfname: "eth0",
				Labels: cniapi.NetpluginLabel{
					TenantName:   "ten1",
					NetworkName:  "net1",
					NetworkGroup: "",
				},
			},
		},
	}

	for i, cfg1 := range successCfg {
		if strings.Split(cfg1.name, "-")[1] != strconv.Itoa(i+1) {
			intLog.Fatalf("invalid test case number %s", cfg1.name)
		}
		executeEndpointTest(its, c, cfg1)
	}

	failCfg := []testConfigData{
		// tenantName=ten1 networkName=net1, epg=
		{
			name:   "epf-1",
			result: true, cleanup: false, containerName: "mesos-epf-f1", operation: cniapi.CniCmdAdd,
			tenantName: "ten-f1", networkName: "net-f1", networkGroup: "",
			subnet: "10.36.3.0/30", agentIPAddr: "172.20.10.201",
			reqAttr: cniapi.CniCmdReqAttr{
				CniIfname: "eth0",
				Labels: cniapi.NetpluginLabel{
					TenantName:   "ten-f1",
					NetworkName:  "net-f1",
					NetworkGroup: "",
				},
			},
		},
		{
			name:   "epf-2",
			result: true, cleanup: false, containerName: "mesos-epf-f2", operation: cniapi.CniCmdAdd,
			tenantName: "ten-f1", networkName: "net-f1", networkGroup: "",
			subnet: "10.36.3.0/30", agentIPAddr: "172.20.10.201",
			reqAttr: cniapi.CniCmdReqAttr{
				CniIfname: "eth0",
				Labels: cniapi.NetpluginLabel{
					TenantName:   "ten-f1",
					NetworkName:  "net-f1",
					NetworkGroup: "",
				},
			},
		},

		// exhaust ip address
		{
			name:   "epf-3",
			result: false, cleanup: false, containerName: "mesos-epf-f3", operation: cniapi.CniCmdAdd,
			tenantName: "ten-f1", networkName: "net-f1", networkGroup: "",
			subnet: "10.36.3.0/30", agentIPAddr: "172.20.10.201",
			reqAttr: cniapi.CniCmdReqAttr{
				CniIfname: "eth0",
				Labels: cniapi.NetpluginLabel{
					TenantName:   "ten-f1",
					NetworkName:  "net-f1",
					NetworkGroup: "",
				},
			},
		},

		//cleanup
		{
			name:   "epf-4",
			result: true, cleanup: false, containerName: "mesos-epf-f1", operation: cniapi.CniCmdDel,
			tenantName: "ten-f1", networkName: "net-f1", networkGroup: "",
			subnet: "10.36.3.0/30", agentIPAddr: "172.20.10.201",
			reqAttr: cniapi.CniCmdReqAttr{
				CniIfname: "eth0",
				Labels: cniapi.NetpluginLabel{
					TenantName:   "ten-f1",
					NetworkName:  "net-f1",
					NetworkGroup: "",
				},
			},
		},

		{
			name:   "epf-5",
			result: true, cleanup: true, containerName: "mesos-epf-f2", operation: cniapi.CniCmdDel,
			tenantName: "ten-f1", networkName: "net-f1", networkGroup: "",
			subnet: "10.36.3.0/30", agentIPAddr: "172.20.10.201",
			reqAttr: cniapi.CniCmdReqAttr{
				CniIfname: "eth0",
				Labels: cniapi.NetpluginLabel{
					TenantName:   "ten-f1",
					NetworkName:  "net-f1",
					NetworkGroup: "",
				},
			},
		},

		// no container
		{
			name:   "epf-6",
			result: false, cleanup: true, containerName: "", operation: cniapi.CniCmdAdd,
			tenantName: "ten-f2", networkName: "net-f2", networkGroup: "",
			subnet: "10.36.4.0/30", agentIPAddr: "172.20.10.201",
			reqAttr: cniapi.CniCmdReqAttr{
				CniIfname:      "eth0",
				CniContainerid: "notexist",
				Labels: cniapi.NetpluginLabel{
					TenantName:   "ten-f2",
					NetworkName:  "net-f2",
					NetworkGroup: "",
				},
			},
		},

		// no tenant
		{
			name:   "epf-7",
			result: false, cleanup: true, containerName: "mesos-epf-f3", operation: cniapi.CniCmdAdd,
			tenantName: "", networkName: "", networkGroup: "",
			subnet: "10.36.3.0/30", agentIPAddr: "172.20.10.201",
			reqAttr: cniapi.CniCmdReqAttr{
				CniIfname: "eth0",
				Labels: cniapi.NetpluginLabel{
					TenantName:   "ten-f1",
					NetworkName:  "",
					NetworkGroup: "",
				},
			},
		},

		// no network
		{
			name:   "epf-8",
			result: false, cleanup: true, containerName: "mesos-epf-f3", operation: cniapi.CniCmdAdd,
			tenantName: "", networkName: "", networkGroup: "",
			subnet: "10.36.3.0/30", agentIPAddr: "172.20.10.201",
			reqAttr: cniapi.CniCmdReqAttr{
				CniIfname: "eth0",
				Labels: cniapi.NetpluginLabel{
					TenantName:   "",
					NetworkName:  "default-net",
					NetworkGroup: "",
				},
			},
		},
		// no epg
		{
			name:   "epf-9",
			result: false, cleanup: true, containerName: "mesos-epf-f3", operation: cniapi.CniCmdAdd,
			tenantName: "", networkName: "default-net", networkGroup: "",
			subnet: "10.36.3.0/30", agentIPAddr: "172.20.10.201",
			reqAttr: cniapi.CniCmdReqAttr{
				CniIfname: "eth0",
				Labels: cniapi.NetpluginLabel{
					TenantName:   "",
					NetworkName:  "default-net",
					NetworkGroup: "epg1",
				},
			},
		},

		// delete non-existing endpoint
		{
			name:   "epf-10",
			result: true, cleanup: false, containerName: "", operation: cniapi.CniCmdDel,
			tenantName: "", networkName: "", networkGroup: "",
			subnet: "", agentIPAddr: "172.20.10.201",
			reqAttr: cniapi.CniCmdReqAttr{
				CniIfname: "eth0",
				Labels: cniapi.NetpluginLabel{
					TenantName:   "",
					NetworkName:  "default-net",
					NetworkGroup: "epg1",
				},
			},
		},
	}

	for i, cfg1 := range failCfg {
		if strings.Split(cfg1.name, "-")[1] != strconv.Itoa(i+1) {
			intLog.Fatalf("invalid test case number %s", cfg1.name)
		}
		executeEndpointTest(its, c, cfg1)
	}
}

func executeEndpointTest(its *integTestSuite, c *C, cfg1 testConfigData) {
	intLog.Infof("############## test [%s]  %+v #############", cfg1.name, cfg1)

	cfg1.createTenant(its, c)
	cfg1.createNetwork(its, c)
	cfg1.createEPG(its, c)
	cfg1.runContainer(its, c)

	intLog.Infof("container attributes %+v", cfg1.reqAttr)

	jsonReq, err := json.Marshal(cfg1.reqAttr)
	assertNoErr(err, c, "json conversion")
	jsonBuf := bytes.NewBuffer(jsonReq)
	url := "http://localhost"
	if cfg1.operation == cniapi.CniCmdDel {
		url += cniapi.MesosNwIntfDel
	} else {
		url += cniapi.MesosNwIntfAdd
	}
	status, jsonResp := processHTTP(c, url, jsonBuf)

	if cfg1.result != false {
		assertOnTrue(c, status != http.StatusOK,
			fmt.Sprintf("invalid http ret code, expected OK, got %d", status))
	} else {
		assertOnTrue(c, status != http.StatusInternalServerError,
			fmt.Sprintf("invalid http ret code, expected internal error, got %d", status))
	}

	successResp := cniapi.CniCmdSuccessResp{}
	if status == http.StatusOK {
		if cfg1.operation == cniapi.CniCmdAdd {
			err := json.Unmarshal(jsonResp, &successResp)
			assertNoErr(err, c, "json unmarshal")
			intLog.Infof("success response: %+v", successResp)
		}

	} else {
		errResp := cniapi.CniCmdErrorResp{}
		err := json.Unmarshal(jsonResp, &errResp)
		assertNoErr(err, c, "json unmarshal")
		intLog.Infof("error response: %+v", errResp)
		assertOnTrue(c, errResp.ErrCode != cniapi.CniStatusErrorUnsupportedField,
			fmt.Sprintf("expected retcode %d got %d",
				cniapi.CniStatusErrorUnsupportedField, errResp.ErrCode))
	}
	cfg1.verifyEndpoint(its, c, successResp)

	if (cfg1.result == false) || (cfg1.operation == cniapi.CniCmdDel) {
		cfg1.destroyContainer(its, c)
	}
	// cleanup tenant/network/epg
	if cfg1.cleanup != false {
		cfg1.deleteEPG(its, c)
		cfg1.deleteNetwork(its, c)
		cfg1.deleteTenant(its, c)
	}

}
