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

package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/netmaster/intent"
	u "github.com/contiv/netplugin/utils"

	log "github.com/Sirupsen/logrus"
)

const (
	examplesDir = "/src/github.com/contiv/netplugin/examples/"
)

// OkToCleanup tests if a testbed cleanup should be performed.
func OkToCleanup(testFailed bool) bool {
	// don't cleanup if stop-on-error is set
	if os.Getenv("CONTIV_SOE") != "" && testFailed {
		return false
	}
	return true
}

// StopOnError stops the test and panics if CONTIV_SOE env-variable is set and test has failed
func StopOnError(testFailed bool) {
	if os.Getenv("CONTIV_SOE") != "" && testFailed {
		panic("Stopping tests as stop on error was set. Please check test logs to determine the actual failure. The system is left in same state for debugging.")
	}
}

// ConfigCleanupCommon performs common cleanup after each test
func ConfigCleanupCommon(t *testing.T, nodes []TestbedNode) {

	if !OkToCleanup(t.Failed()) {
		return
	}

	for _, node := range nodes {
		cmdStr := "sh -c 'sudo $GOSRC/github.com/contiv/netplugin/scripts/cleanup'"
		output, err := node.RunCommandWithOutput(cmdStr)
		if err != nil {
			t.Errorf("Failed to cleanup the left over test case state. Error: %s\nCmd: %q\nOutput:\n%s\n",
				err, cmdStr, output)
		}
	}
	//XXX: remove this once netplugin is capable of handling cleanup
	StopNetPlugin(t, nodes)
	StopNetmaster(t, nodes[0])
}

// StopNetPlugin stops the netplugin on specified testbed nodes
func StopNetPlugin(t *testing.T, nodes []TestbedNode) {
	for _, node := range nodes {
		cmdStr := "sudo pkill netplugin"
		node.RunCommand(cmdStr)
	}
}

// StopNetmaster stops the netmaster on specified testbed node
func StopNetmaster(t *testing.T, node TestbedNode) {
	cmdStr := "sudo pkill netmaster"
	node.RunCommand(cmdStr)
}

// StartNetPluginWithConfig starts netplugin on specified testbed nodes with specified config
func StartNetPluginWithConfig(t *testing.T, nodes []TestbedNode, nativeInteg bool,
	configStr string) {
	nativeIntegStr := ""
	if nativeInteg {
		nativeIntegStr = "-native-integration"
	}

	for i, node := range nodes {
		//start the netplugin
		var (
			cmdStr   string
			flagsStr string
		)
		if configStr != "" {
			cfgFile := fmt.Sprintf("/tmp/plugin-%d.cfg", i+1)
			//fill up the host-label in the passed config string format
			jsonCfg := fmt.Sprintf(configStr, i+1)
			jsonCfg = getEchoCompatibleStr(jsonCfg)
			cmdStr := fmt.Sprintf("echo \"%s\" > %s", jsonCfg, cfgFile)
			output, err := node.RunCommandWithOutput("sh -c '" + cmdStr + "'")
			if err != nil {
				t.Fatalf("Error '%s' creating config file\nCmd: %q\n Output : %s \n",
					err, cmdStr, output)
			}
			flagsStr = fmt.Sprintf("-config %s %s", cfgFile, nativeIntegStr)
		} else {
			flagsStr = fmt.Sprintf("-host-label host%d %s", i+1, nativeIntegStr)
		}

		if os.Getenv("CONTIV_TESTBED") == "DIND" {
			cmdStr = fmt.Sprintf("netplugin %s 0<&- &>/tmp/netplugin-%d.log ", flagsStr, i+1)
		} else {
			cmdStr = fmt.Sprintf("sudo PATH=$PATH nohup netplugin %s 0<&- &>/tmp/netplugin.log &", flagsStr)
		}
		output, err := node.RunCommandBackground(cmdStr)
		if err != nil {
			t.Fatalf("Failed to launch netplugin. Error: %s\nCmd:%q\n Output : %s\n",
				err, cmdStr, output)
		}
	}
}

// StartNetPlugin starts netplugin on  specified testbed nodes
func StartNetPlugin(t *testing.T, nodes []TestbedNode, nativeInteg bool) {
	StartNetPluginWithConfig(t, nodes, nativeInteg, "")
}

// StartNetmasterWithFlags starts netplugin on specified testbed nodes with specified flags
func StartNetmasterWithFlags(t *testing.T, node TestbedNode, flags map[string]string) {
	var (
		cmdStr   string
		flagsStr string
	)

	for k, v := range flags {
		flagsStr += fmt.Sprintf("%s=%s", k, v)
	}

	if os.Getenv("CONTIV_TESTBED") == "DIND" {
		cmdStr = fmt.Sprintf("netmaster %s 0<&- &>/tmp/netmaster.log ", flagsStr)
	} else {
		cmdStr = fmt.Sprintf("nohup netmaster %s 0<&- &>/tmp/netmaster.log &", flagsStr)
	}
	output, err := node.RunCommandBackground(cmdStr)
	if err != nil {
		t.Fatalf("Failed to launch netplugin. Error: %s\nCmd:%q\n Output : %s\n",
			err, cmdStr, output)
	}

}

// StartNetmaster starts netplugin on specified testbed node
func StartNetmaster(t *testing.T, node TestbedNode) {
	StartNetmasterWithFlags(t, node, map[string]string{})
}

// StartPowerStripAdapter starts the powerstrip adapter on specified testbed nodes
func StartPowerStripAdapter(t *testing.T, nodes []TestbedNode) {
	for i, node := range nodes {
		// hardcoding eth1 as it is the control interface in demo setup
		cmdStr := `ip addr show eth1 | grep "\<inet\>" | \
                 awk '{split($2,a, "/"); print a[1]}'`
		output, err := node.RunCommandWithOutput(cmdStr)
		if err != nil {
			t.Fatalf("Error '%s' starting powerstrip adapter, Output: \n%s\n",
				err, output)
		}
		nodeIPAddr := strings.Trim(string(output), " \n")

		// host-label needs to match between netplugin and powerstrip adapter
		cmdStr = `cd $GOSRC/github.com/contiv/netplugin && \
              sudo docker build -t netplugin/pslibnet mgmtfn/pslibnet && \
              sudo docker run -d --name pslibnet --expose 80 netplugin/pslibnet \
                --host-label=host%d --etcd-url=http://%s:2379`
		cmdStr = fmt.Sprintf(cmdStr, i+1, nodeIPAddr)
		output, err = node.RunCommandWithOutput(cmdStr)
		if err != nil {
			t.Fatalf("Error '%s' starting powerstrip adapter, Output: \n%s\n",
				err, output)
		}

		cmdStr = `cat > /tmp/adapters.yml <<EOF
version: 1
endpoints:
  "POST /*/containers/create":
    pre: [pslibnet]
    post: [pslibnet]
  "POST /*/containers/*/start":
    pre: [pslibnet]
    post: [pslibnet]
  "POST /*/containers/*/stop":
    pre: [pslibnet]
  "POST /*/containers/*/delete":
    pre: [pslibnet]
adapters:
  pslibnet: http://pslibnet/adapter/
EOF`
		output, err = node.RunCommandWithOutput(cmdStr)
		if err != nil {
			t.Fatalf("Error '%s' starting powerstrip adapter, Output: \n%s\n",
				err, output)
		}

		cmdStr = `sudo docker run -d --name powerstrip \
                -v /var/run/docker.sock:/var/run/docker.sock \
                -v /tmp/adapters.yml:/etc/powerstrip/adapters.yml \
                --link pslibnet:pslibnet -p 2375:2375 clusterhq/powerstrip:v0.0.1`
		output, err = node.RunCommandWithOutput(cmdStr)
		if err != nil {
			t.Fatalf("Error '%s' starting powerstrip adapter, Output: \n%s\n",
				err, output)
		}
	}
}

func getEchoCompatibleStr(inStr string) string {
	// replace newlines with space and "(quote) with \"(escaped quote) for
	// echo to consume and produce desired json config
	return strings.Replace(strings.Replace(inStr, "\n", " ", -1), "\"", "\\\"", -1)
}

func applyConfig(t *testing.T, cfgType, jsonCfg string, node TestbedNode, stateStore string) {
	// replace newlines with space and "(quote) with \"(escaped quote) for
	// echo to consume and produce desired json config
	jsonCfg = getEchoCompatibleStr(jsonCfg)
	cmdStr := fmt.Sprintf("echo \"%s\" > /tmp/netdcli.cfg", jsonCfg)
	output, err := node.RunCommandWithOutput("sh -c '" + cmdStr + "'")
	if err != nil {
		t.Fatalf("Error '%s' creating config file\nCmd: %q\n Output : %s \n",
			err, cmdStr, output)
	}
	cmdStr = "netdcli -" + cfgType + " /tmp/netdcli.cfg 2>&1"
	if stateStore != "" {
		cmdStr = "netdcli -state-store " + stateStore + " -" + cfgType + " /tmp/netdcli.cfg 2>&1"
	}
	output, err = node.RunCommandWithOutput(cmdStr)
	if err != nil {
		t.Fatalf("Failed to apply config. Error: %s\nCmd: %q\n Output : %s\n",
			err, cmdStr, output)
	}
}

// AddConfig issues netdcli with -add-cfg flag
func AddConfig(t *testing.T, jsonCfg string, node TestbedNode) {
	applyConfig(t, "add-cfg", jsonCfg, node, "")
}

// AddConfigConsul issues netdcli with -add-cfg flag and uses consul state-store
func AddConfigConsul(t *testing.T, jsonCfg string, node TestbedNode) {
	applyConfig(t, "add-cfg", jsonCfg, node, u.ConsulNameStr)
}

// DelConfig issues netdcli with -del-cfg flag
func DelConfig(t *testing.T, jsonCfg string, node TestbedNode) {
	applyConfig(t, "del-cfg", jsonCfg, node, "")
}

// DelConfigConsul issues netdcli with -del-cfg flag and uses consul state-store
func DelConfigConsul(t *testing.T, jsonCfg string, node TestbedNode) {
	applyConfig(t, "del-cfg", jsonCfg, node, u.ConsulNameStr)
}

// ApplyDesiredConfig issues netdcli with -cfg flag
func ApplyDesiredConfig(t *testing.T, jsonCfg string, node TestbedNode) {
	applyConfig(t, "cfg", jsonCfg, node, "")
}

// ApplyDesiredConfigConsul issues netdcli with -cfg flag and uses consul state-store
func ApplyDesiredConfigConsul(t *testing.T, jsonCfg string, node TestbedNode) {
	applyConfig(t, "cfg", jsonCfg, node, u.ConsulNameStr)
}

// ApplyHostBindingsConfig issues netdcli with -host-bindings-cfg flag
func ApplyHostBindingsConfig(t *testing.T, jsonCfg string, node TestbedNode) {
	applyConfig(t, "host-bindings-cfg", jsonCfg, node, "")
}

// FixUpContainerUUIDs fills up UUID information in passed jsonCfg and returns host-binding configuration
func FixUpContainerUUIDs(t *testing.T, nodes []TestbedNode, jsonCfg string) (string, error) {
	epBindings := []intent.ConfigEP{}
	err := json.Unmarshal([]byte(jsonCfg), &epBindings)
	if err != nil {
		t.Fatalf("error '%s' unmarshing host bindings, data %s \n", err,
			jsonCfg)
		return "", err
	}

	// fill in as much as possible for this host; assume that the
	// container name is unique across hosts
	for _, node := range nodes {
		for idx := range epBindings {
			ep := &epBindings[idx]
			if ep.AttachUUID != "" {
				continue
			}
			attachUUID, _ := getContainerUUID(node, ep.Container)
			if attachUUID != "" {
				ep.AttachUUID = attachUUID
			}
		}
	}

	bytes, err := json.Marshal(epBindings)
	if err != nil {
		t.Fatalf("error '%s' marshaling host bindings, data %v \n",
			err, epBindings)
		return "", err
	}

	return string(bytes[:]), err
}

// FixUpInfraContainerUUIDs fills up UUID information in passed jsonCfg and returns host-binding configuration
func FixUpInfraContainerUUIDs(t *testing.T, nodes []TestbedNode, jsonCfg, infraContCfg string) (string, error) {

	epBindings := []intent.ConfigEP{}
	err := json.Unmarshal([]byte(jsonCfg), &epBindings)
	if err != nil {
		t.Fatalf("error '%s' unmarshing host bindings, data %s \n", err,
			jsonCfg)
		return "", err
	}

	infraContMap := make(map[string]string)
	infraContCfg = strings.TrimSpace(infraContCfg)
	infraContRecords := strings.Split(infraContCfg, "\n")
	for _, infraContRecord := range infraContRecords {
		fields := strings.Split(infraContRecord, ":")
		if len(fields) != 2 {
			t.Fatalf("error parsing the container mappings cfg '%s' rec '%s'\n",
				infraContCfg, infraContRecord)
		}
		infraContMap[fields[0]] = fields[1]
	}

	// fill in as much as possible for this host; assume that the
	// container name is unique across hosts
	for _, node := range nodes {
		for idx := range epBindings {
			ep := &epBindings[idx]
			if ep.AttachUUID != "" {
				continue
			}

			infraContName, ok := infraContMap[ep.Container]
			if !ok {
				continue
			}

			attachUUID, _ := getContainerUUID(node, infraContName)
			if attachUUID != "" {
				ep.AttachUUID = attachUUID
			}
		}
	}

	bytes, err := json.Marshal(epBindings)
	if err != nil {
		t.Fatalf("error '%s' marshaling host bindings, data %v \n",
			err, epBindings)
		return "", err
	}

	return string(bytes[:]), err
}

// ConfigSetupCommon performs common configuration setup on specified testbed nodes
func ConfigSetupCommon(t *testing.T, jsonCfg string, nodes []TestbedNode) {
	StartNetmaster(t, nodes[0])

	StartNetPlugin(t, nodes, false)

	ApplyDesiredConfig(t, jsonCfg, nodes[0])
}

// GetNetpluginConfigWithConsul returns netplugin config that uses consul state store
func GetNetpluginConfigWithConsul() string {
	return `{
                    "drivers" : {
                       "network": "ovs",
                       "endpoint": "ovs",
                       "state": "consul"
                    },
                    "plugin-instance": {
                       "host-label": "host%d"
                    },
	                "ovs" : {
                       "dbip": "127.0.0.1",
                       "dbport": 6640
                    },
                    "consul" : {
                        "address": "127.0.0.1:8500"
                    },
                    "crt" : {
                       "type": "docker"
                    },
                    "docker" : {
                        "socket" : "unix:///var/run/docker.sock"
                    }
			}`
}

// ConfigSetupCommonWithConsul performs common configuration setup on specified testbed nodes
func ConfigSetupCommonWithConsul(t *testing.T, jsonCfg string, nodes []TestbedNode) {
	StartNetmasterWithFlags(t, nodes[0], map[string]string{
		"--state-store": "consul"})

	StartNetPluginWithConfig(t, nodes, false, GetNetpluginConfigWithConsul())

	ApplyDesiredConfigConsul(t, jsonCfg, nodes[0])
}

// GetIPAddress returns IP-address information for specified endpoint
func GetIPAddress(t *testing.T, node TestbedNode, ep, stateStore string) string {
	cmdStr := "netdcli -oper get -construct endpoint " + ep + " 2>&1"
	if stateStore != "" {
		cmdStr = "netdcli -oper get -state-store " + stateStore + " -construct endpoint " + ep + " 2>&1"
	}
	output, err := node.RunCommandWithOutput(cmdStr)

	if err != nil || string(output) == "" {
		time.Sleep(2 * time.Second)
		output, err = node.RunCommandWithOutput(cmdStr)
		if err != nil || output == "" {
			t.Fatalf("Error getting ip for ep %s. Error: %s, Cmdstr: %s, Output: \n%s\n",
				err, ep, cmdStr, output)
		}
	}

	output = strings.Trim(string(output), "[]")

	epStruct := drivers.OvsOperEndpointState{}

	if err := json.Unmarshal([]byte(output), &epStruct); err != nil {
		t.Fatalf("Error getting ip for ep %s. Error: %s, Cmdstr: %s, Output: \n%s\n",
			err, ep, cmdStr, output)
	}

	return epStruct.IPAddress
}

// GetIPAddressFromNetworkAndContainerName return IP address when network id and
// container name are known
// XXX: used for powerstrip/docker integration testing where ep-name is
// derived by concatanating net-id to container-id
func GetIPAddressFromNetworkAndContainerName(t *testing.T, node TestbedNode,
	netID, contName string) string {
	uuid, err := getContainerUUID(node, contName)
	if err != nil {
		t.Fatalf("failed to get container uuid for container %q on node %q",
			contName, node.GetName())
	}
	epName := fmt.Sprintf("%s-%s", netID, uuid)
	return GetIPAddress(t, node, epName, "")
}

// NetworkStateExists tests if state for specified network exists
func NetworkStateExists(node TestbedNode, network, stateStore string) error {
	cmdStr := "netdcli -oper get -construct network " + network + " 2>&1"
	if stateStore != "" {
		cmdStr = "netdcli -state-store " + stateStore + "-oper get -construct network " + network + " 2>&1"
	}
	output, err := node.RunCommandWithOutput(cmdStr)

	if err != nil {
		return err
	}
	if string(output) == "" {
		return core.Errorf("got null output")
	}
	return nil
}

// DumpNetpluginLogs prints netplugin logs from the specified testbed node
func DumpNetpluginLogs(node TestbedNode) {
	cmdStr := fmt.Sprintf("sudo cat /tmp/netplugin.log")
	output, err := node.RunCommandWithOutput(cmdStr)
	if err == nil {
		log.Debugf("logs on node %s: \n%s\n", node.GetName(), output)
	}
}

// GetCfgFile returns the path string for specified file name in examples directory
func GetCfgFile(fileName string) string {
	cfgDir := os.Getenv("GOPATH")
	cfgDir = cfgDir + examplesDir
	return cfgDir + fileName + ".json"
}
