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
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/contiv/netplugin/netmaster"
)

const (
	EXAMPLES_DIR = "/src/github.com/contiv/netplugin/examples/"
)

func OkToCleanup(testFailed bool) bool {
	// don't cleanup if stop-on-error is set
	if os.Getenv("CONTIV_SOE") != "" && testFailed {
		return false
	}
	return true
}

func StopOnError(testFailed bool) {
	if os.Getenv("CONTIV_SOE") != "" && testFailed {
		panic("Stopping tests as stop on error was set. Please check test logs to determine the actual failure. The system is left in same state for debugging.")
	}
}

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
		//XXX: remove this once netplugin is capable of handling cleanup
		cmdStr = "sudo pkill netplugin"
		node.RunCommand(cmdStr)
	}
}

func startNetPlugin(t *testing.T, nodes []TestbedNode, nativeInteg bool) {
	nativeIntegStr := ""
	if nativeInteg {
		nativeIntegStr = "-native-integration"
	}

	for i, node := range nodes {
		//start the netplugin
		var cmdStr string
		if os.Getenv("CONTIV_TESTBED") == "DIND" {
			cmdStr = fmt.Sprintf("netplugin %s -host-label host%d 0<&- &>/tmp/netplugin-%d.log ", nativeIntegStr,
				i+1, i+1)
		} else {
			cmdStr = fmt.Sprintf("sudo PATH=$PATH nohup netplugin %s -host-label host%d 0<&- &>/tmp/netplugin.log &", nativeIntegStr,
				i+1)
		}
		output, err := node.RunCommandBackground(cmdStr)
		if err != nil {
			t.Fatalf("Failed to launch netplugin. Error: %s\nCmd:%q\n Output : %s\n",
				err, cmdStr, output)
		}
	}
}

func applyConfig(t *testing.T, cfgType, jsonCfg string, node TestbedNode) {
	// replace newlines with space and "(quote) with \"(escaped quote) for
	// echo to consume and produce desired json config
	jsonCfg = strings.Replace(
		strings.Replace(jsonCfg, "\n", " ", -1),
		"\"", "\\\"", -1)
	cmdStr := fmt.Sprintf("echo \"%s\" > /tmp/netdcli.cfg", jsonCfg)
	output, err := node.RunCommandWithOutput("sh -c '" + cmdStr + "'")
	if err != nil {
		t.Fatalf("Error '%s' creating config file\nCmd: %q\n Output : %s \n",
			err, cmdStr, output)
	}
	cmdStr = "netdcli -" + cfgType + " /tmp/netdcli.cfg 2>&1"
	output, err = node.RunCommandWithOutput(cmdStr)
	if err != nil {
		t.Fatalf("Failed to apply config. Error: %s\nCmd: %q\n Output : %s\n",
			err, cmdStr, output)
	}
}

func AddConfig(t *testing.T, jsonCfg string, node TestbedNode) {
	applyConfig(t, "add-cfg", jsonCfg, node)
}

func DelConfig(t *testing.T, jsonCfg string, node TestbedNode) {
	applyConfig(t, "del-cfg", jsonCfg, node)
}

func ApplyDesiredConfig(t *testing.T, jsonCfg string, node TestbedNode) {
	applyConfig(t, "cfg", jsonCfg, node)
}

func ApplyHostBindingsConfig(t *testing.T, jsonCfg string, node TestbedNode) {
	applyConfig(t, "host-bindings-cfg", jsonCfg, node)
}

func FixUpContainerUUIDs(t *testing.T, nodes []TestbedNode, jsonCfg string) (string, error) {
	epBindings := []netmaster.ConfigEp{}
	err := json.Unmarshal([]byte(jsonCfg), &epBindings)
	if err != nil {
		t.Fatalf("error '%s' unmarshing host bindings, data %s \n", err,
			jsonCfg)
		return "", err
	}

	// fill in as much as possible for this host; assume that the
	// container name is unique across hosts
	for _, node := range nodes {
		for idx, _ := range epBindings {
			ep := &epBindings[idx]
			if ep.AttachUUID != "" {
				continue
			}
			attachUUID, _ := getUUID(node, ep.Container)
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

func FixUpInfraContainerUUIDs(t *testing.T, nodes []TestbedNode, jsonCfg, infraContCfg string) (string, error) {

	epBindings := []netmaster.ConfigEp{}
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
		for idx, _ := range epBindings {
			ep := &epBindings[idx]
			if ep.AttachUUID != "" {
				continue
			}

			infraContName, ok := infraContMap[ep.Container]
			if !ok {
				continue
			}

			attachUUID, _ := getUUID(node, infraContName)
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

func ConfigSetupCommon(t *testing.T, jsonCfg string, nodes []TestbedNode) {
	startNetPlugin(t, nodes, false)

	ApplyDesiredConfig(t, jsonCfg, nodes[0])
}

func GetIpAddress(t *testing.T, node TestbedNode, ep string) string {
	cmdStr := "netdcli -oper get -construct endpoint " + ep +
		" 2>&1 | grep IpAddress | awk -F : '{gsub(\"[,}{]\",\"\", $2); print $2}'"
	output, err := node.RunCommandWithOutput(cmdStr)

	if err != nil || string(output) == "" {
		time.Sleep(2 * time.Second)
		output, err = node.RunCommandWithOutput(cmdStr)
	}

	if err != nil || string(output) == "" {
		t.Fatalf("Error '%s' getting ip for ep %s, Output: \n%s\n",
			err, ep, output)
	}
	return string(output)
}

func NetworkStateExists(node TestbedNode, network string) error {
	cmdStr := "netdcli -oper get -construct network " + network + " 2>&1"
	output, err := node.RunCommandWithOutput(cmdStr)

	if err != nil {
		return err
	}
	if string(output) == "" {
		return errors.New("got null output")
	}
	return nil
}

func DumpNetpluginLogs(node TestbedNode) {
	cmdStr := fmt.Sprintf("sudo cat /tmp/netplugin.log")
	output, err := node.RunCommandWithOutput(cmdStr)
	if err == nil {
		log.Printf("logs on node %s: \n%s\n", node.GetName(), output)
	}
}

func GetCfgFile(fileName string) string {
	cfgDir := os.Getenv("GOPATH")
	cfgDir = cfgDir + EXAMPLES_DIR
	return cfgDir + fileName + ".json"
}
