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
	"fmt"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/mgmtfn/mesosplugin/cniapi"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/netplugin/cluster"
	"github.com/contiv/netplugin/netplugin/plugin"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/netplugin/utils/netutils"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

const netNsDir = "/var/run/netns/"

var netPlugin *plugin.NetPlugin
var stateDriver core.StateDriver
var hostName string

// execute name space commands
func (cniReq *cniServer) ipnsExecute(namespace string, args []string) ([]byte, error) {
	ipCmd := "ip"
	ipArgs := []string{"netns", "exec", namespace}

	ipArgs = append(ipArgs, args...)
	cniLog.Infof("processing cmd: %v", ipArgs)
	return exec.Command(ipCmd, ipArgs...).CombinedOutput()
}

// execute a batch of namespace commands
func (cniReq *cniServer) ipnsBatchExecute(namespace string, args [][]string) ([]byte, error) {

	for idx, arg1 := range args {
		if out, err := cniReq.ipnsExecute(namespace, arg1); err != nil {
			cniLog.Errorf("failed to execute [%d] %v %s, %s", idx, err, arg1, string(out))
			return out, err
		}
	}

	return nil, nil
}

func parseMesosAgentIPAddr(pidList []byte) (string, error) {
	agentIPaddr := ""
	cniLog.Infof("check pids %v for mesos agent ip addr",
		strings.Split(string(pidList), "\n"))

	for _, pid := range strings.Split(string(pidList), "\n") {
		envVarList, err := ioutil.ReadFile(fmt.Sprintf("/proc/%s/environ", pid))
		if err != nil {
			return agentIPaddr, err
		}
		cniLog.Infof("parsing pid [%s] for mesos agent ip addr", pid)
		for _, envVar := range strings.Split(string(envVarList), "\x00") {
			cniLog.Debugf("parsing Mesos env %s", envVar)
			if strings.HasPrefix(envVar, cniapi.EnvVarMesosAgent) {
				cniLog.Infof("parsing Mesos agent endpoint %s", envVar)
				agentIPaddr = strings.Split(strings.Split(envVar, "=")[1], ":")[0]
				cniLog.Infof("mesos agent ip address is %s", agentIPaddr)
				return agentIPaddr, nil
			}
		}
	}

	cniLog.Errorf("failed to find agent ip addr")
	return agentIPaddr, fmt.Errorf("failed to find the ip address of mesos agent")
}

func (cniReq *cniServer) createHostBrIntf(ovsEpDriver *drivers.OperEndpointState) error {

	hostBrIfName := netutils.GetHostIntfName(ovsEpDriver.PortName)

	// find executor info
	pidList, err := exec.Command("ip", "netns", "pids",
		cniReq.pluginArgs.CniContainerid).CombinedOutput()
	if err != nil {
		cniLog.Errorf("failed to get pid-list for namespace %s: %s",
			cniReq.pluginArgs.CniContainerid, err)
		return err
	}

	agentIPAddr, err := parseMesosAgentIPAddr(pidList)
	if err != nil {
		return err
	}

	// add host interface
	cniLog.Infof("create host-br interface %s", hostBrIfName)
	hostBrIfIPaddr, err := netPlugin.CreateHostAccPort(hostBrIfName, cniReq.ipv4Addr)
	if err != nil {
		cniLog.Errorf("failed to create [%s] in host-br: %s",
			hostBrIfName, err.Error())
		return err
	}

	// move host-br interface to new namespace
	if _, err := exec.Command("ip", "link", "set", hostBrIfName, "netns",
		cniReq.pluginArgs.CniContainerid).CombinedOutput(); err != nil {
		cniLog.Errorf("failed to move %s to namespace %s: %s",
			ovsEpDriver.PortName, cniReq.pluginArgs.CniContainerid, err.Error())
		cniReq.deleteHostBrIntf()
		return err
	}

	nsHostIfCmds := [][]string{
		{"ip", "address", "add", hostBrIfIPaddr, "dev", hostBrIfName},
		{"ip", "link", "set", hostBrIfName, "up"},
		{"ip", "route", "add", fmt.Sprintf("%s/32", agentIPAddr), "dev", hostBrIfName},
	}

	if _, err := cniReq.ipnsBatchExecute(cniReq.pluginArgs.CniContainerid, nsHostIfCmds); err != nil {
		cniReq.deleteHostBrIntf()
		return fmt.Errorf("failed to execute host-br commands in namespace %s: %s ",
			cniReq.pluginArgs.CniContainerid, err.Error())
	}

	return nil
}

func (cniReq *cniServer) deleteHostBrIntf() error {
	cniLog.Infof("delete host-br interface for %s", cniReq.endpointID)
	if err := netPlugin.DeleteHostAccPort(cniReq.endpointID); err != nil {
		cniLog.Errorf("failed to delete host-br interface, %s", err)
		return err
	}
	return nil
}

// configure cni namespace
func (cniReq *cniServer) configureNetNs(ovsEpDriver *drivers.OperEndpointState,
	mResp *master.CreateEndpointResponse,
	nwState *mastercfg.CfgNetworkState) error {

	os.MkdirAll(netNsDir, 644)

	// link new name space
	if _, err := os.Stat(netNsDir + cniReq.pluginArgs.CniContainerid); err != nil {
		if err := os.Symlink(cniReq.pluginArgs.CniNetns, netNsDir+cniReq.pluginArgs.CniContainerid); err != nil {
			cniLog.Errorf("failed to link pid to namespace : %s", err.Error())
			return err
		}
	}
	cniLog.Infof("linked %s --> %s ", cniReq.pluginArgs.CniNetns, netNsDir+cniReq.pluginArgs.CniContainerid)

	// move interface to new namespace
	if _, err := exec.Command("ip", "link", "set", ovsEpDriver.PortName, "netns",
		cniReq.pluginArgs.CniContainerid).CombinedOutput(); err != nil {
		cniLog.Errorf("failed to move intf %s to namespace %s: %s",
			ovsEpDriver.PortName, cniReq.pluginArgs.CniContainerid, err.Error())
		cniReq.unlinkNetNs()
		return err
	}

	nsCmds := [][]string{
		{"ip", "link", "set", ovsEpDriver.PortName, "name", cniReq.pluginArgs.CniIfname, "up"},
		{"ip", "address", "add", fmt.Sprintf("%s/%d", ovsEpDriver.IPAddress, nwState.SubnetLen), "dev",
			cniReq.pluginArgs.CniIfname},
	}

	cniReq.ipv4Addr = ovsEpDriver.IPAddress
	cniReq.cniSuccessResp.IP4.IPAddress = fmt.Sprintf("%s/%d", ovsEpDriver.IPAddress, nwState.SubnetLen)

	// gateway
	if len(nwState.Gateway) > 0 {
		gwCmd := []string{"ip", "route", "add", "default", "via", fmt.Sprintf("%s", nwState.Gateway)}
		nsCmds = append(nsCmds, gwCmd)
		cniReq.cniSuccessResp.IP4.Gateway = nwState.Gateway
		cniLog.Infof("ipv4 default gateway of endpoint %s", nwState.Gateway)
	}

	// ipv6
	if len(mResp.EndpointConfig.IPv6Address) > 0 {
		ipv6Cmd := []string{"ip", "-6", "addr", "add",
			fmt.Sprintf("%s/%d", mResp.EndpointConfig.IPv6Address, nwState.IPv6SubnetLen),
			"dev", cniReq.pluginArgs.CniIfname}
		nsCmds = append(nsCmds, ipv6Cmd)
		cniReq.cniSuccessResp.IP6.IPAddress = fmt.Sprintf("%s/%d", mResp.EndpointConfig.IPv6Address, nwState.IPv6SubnetLen)
		cniLog.Infof("ipv6 address of endpoint %s", mResp.EndpointConfig.IPv6Address)

	}

	if len(nwState.IPv6Gateway) > 0 {
		ipv6gwCmd := []string{"ip", "-6", "route", "add", "default", "via",
			fmt.Sprintf("%s", nwState.IPv6Gateway)}
		nsCmds = append(nsCmds, ipv6gwCmd)
		cniReq.cniSuccessResp.IP6.Gateway = nwState.IPv6Gateway
		cniLog.Infof("ipv6 gateway of endpoint %s", nwState.IPv6Gateway)
	}

	if _, err := cniReq.ipnsBatchExecute(cniReq.pluginArgs.CniContainerid, nsCmds); err != nil {
		cniLog.Errorf("failed to execute commands in namespace %s: %s",
			cniReq.pluginArgs.CniNetns, err.Error())
		cniReq.unlinkNetNs()
		return err
	}

	return nil
}

func (cniReq *cniServer) unlinkNetNs() error {
	if err := os.Remove(netNsDir + cniReq.pluginArgs.CniContainerid); err != nil {
		cniLog.Errorf("failed to unlink namespace %s, %s",
			cniReq.pluginArgs.CniNetns, err)
		return err
	}
	return nil
}

func (cniReq *cniServer) createCniEndPoint() error {
	var err error
	ovsEpDriver := &drivers.OperEndpointState{}
	ovsEpDriver.StateDriver = stateDriver

	nwState := &mastercfg.CfgNetworkState{}
	nwState.StateDriver = stateDriver

	// build endpoint request
	epReq := master.CreateEndpointRequest{
		TenantName:  cniReq.endPointLabels[cniapi.LabelTenantName],
		NetworkName: cniReq.endPointLabels[cniapi.LabelNetworkName],
		ServiceName: cniReq.endPointLabels[cniapi.LabelNetworkGroup],
		EndpointID:  cniReq.pluginArgs.CniContainerid,
		ConfigEP: intent.ConfigEP{
			Container:   cniReq.pluginArgs.CniContainerid,
			Host:        hostName,
			ServiceName: cniReq.endPointLabels[cniapi.LabelNetworkGroup],
		},
	}

	cniLog.Infof("endpoint-req: epid:%s cont-id:%s ", epReq.EndpointID, epReq.ConfigEP.Container)

	epResp := master.CreateEndpointResponse{}
	if err = cluster.MasterPostReq("/plugin/createEndpoint", &epReq, &epResp); err != nil {
		cniLog.Errorf("failed to create endpoint in master: %s", err.Error())
		return err
	}

	cniLog.Infof("endpoint created  %+v", epResp)

	// create endpoint in netplugin
	if err = netPlugin.CreateEndpoint(cniReq.endpointID); err != nil {
		cniLog.Errorf("failed to create endpoint in netplugin: %s", err.Error())
		goto cleanupMaster
	}

	// read end point
	if err = ovsEpDriver.Read(cniReq.endpointID); err != nil {
		cniLog.Errorf("failed to read endpoint: %s", err.Error())
		goto cleanupNetplugin
	}

	if err = nwState.Read(cniReq.networkID); err != nil {
		cniLog.Errorf("failed to read network config : %s", err.Error())
		goto cleanupNetplugin
	}

	cniLog.Debugf("read new network config +%v", nwState)

	if err = cniReq.configureNetNs(ovsEpDriver, &epResp, nwState); err != nil {
		goto cleanupNetplugin
	}

	return nil

	// unwind the setup, ignore errors
cleanupNetplugin:
	cniReq.deleteNetpluginEndPoint()
cleanupMaster:
	cniReq.deleteMasterEndPoint()

	return err
}

func (cniReq *cniServer) deleteMasterEndPoint() error {

	// delete from master
	delReq := master.DeleteEndpointRequest{
		TenantName:  cniReq.endPointLabels[cniapi.LabelTenantName],
		NetworkName: cniReq.endPointLabels[cniapi.LabelNetworkName],
		ServiceName: cniReq.endPointLabels[cniapi.LabelNetworkGroup],
		EndpointID:  cniReq.pluginArgs.CniContainerid,
	}

	delResp := master.DeleteEndpointResponse{}
	if err := cluster.MasterPostReq("/plugin/deleteEndpoint", &delReq, &delResp); err != nil {
		cniLog.Errorf("failed to delete endpoint %s from master %s ", cniReq.endpointID, err.Error())
		return err
	}
	return nil
}

func (cniReq *cniServer) deleteNetpluginEndPoint() error {
	if err := netPlugin.DeleteEndpoint(cniReq.endpointID); err != nil {
		cniLog.Errorf("failed to delete endpoint %s from netplugin %s ", cniReq.endpointID, err)
		return err
	}

	return nil
}

func (cniReq *cniServer) deleteCniEndPoint() error {

	// execute once for the container
	if strings.HasSuffix(cniReq.pluginArgs.CniIfname, "0") {
		if err := cniReq.deleteHostBrIntf(); err != nil {
			cniLog.Errorf("failed to delete host-br %s", err)
		}
	}

	if err := cniReq.unlinkNetNs(); err != nil {
		cniLog.Errorf("failed to delete cni endpoint, %s ", err)
	}

	if err := cniReq.deleteNetpluginEndPoint(); err != nil {
		cniLog.Errorf("failed to delete cni endpoint, %s ", err)
	}

	if err := cniReq.deleteMasterEndPoint(); err != nil {
		cniLog.Errorf("failed to delete cni endpoint, %s ", err)
	}

	return nil
}

func cniDriverInit(plugin *plugin.NetPlugin) error {
	var err error

	netPlugin = plugin
	if stateDriver, err = utils.GetStateDriver(); err != nil {
		cniLog.Errorf("failed to get state driver %s", err)
		return err
	}

	if hostName, err = os.Hostname(); err != nil {
		cniLog.Errorf("failed to get hostname %s", err)
		return err
	}

	return nil
}
