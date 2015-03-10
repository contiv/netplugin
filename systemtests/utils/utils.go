package utils

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"testing"
)

func ConfigCleanupCommon(t *testing.T, nodes []VagrantNode) {
	for _, node := range nodes {
		cmdStr := "sudo $GOSRC/github.com/contiv/netplugin/scripts/cleanup"
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

func startNetPlugin(t *testing.T, nodes []VagrantNode) {
	for i, node := range nodes {
		//start the netplugin
		cmdStr := fmt.Sprintf("sudo PATH=$PATH nohup netplugin -host-label host%d 0<&- &>/tmp/netplugin.log &",
			i+1)
		output, err := node.RunCommandWithOutput(cmdStr)
		if err != nil {
			t.Fatalf("Failed to launch netplugin. Error: %s\nCmd:%q\nOutput:\n%s\n",
				err, cmdStr, output)
		}
	}

}

func applyConfig(t *testing.T, cfgType, jsonCfg string, node VagrantNode) {
	// replace newlines with space and "(quote) with \"(escaped quote) for
	// echo to consume and produce desired json config
	jsonCfg = strings.Replace(
		strings.Replace(jsonCfg, "\n", " ", -1),
		"\"", "\\\"", -1)
	cmdStr := fmt.Sprintf("echo %s > /tmp/netdcli.cfg", jsonCfg)
	output, err := node.RunCommandWithOutput(cmdStr)
	if err != nil {
		t.Fatalf("Error '%s' creating config file\nCmd: %q\nOutput:\n%s\n",
			err, cmdStr, output)
	}

	cmdStr = "netdcli -" + cfgType + " /tmp/netdcli.cfg 2>&1"
	output, err = node.RunCommandWithOutput(cmdStr)
	if err != nil {
		t.Fatalf("Failed to apply config. Error: %s\nCmd: %q\nOutput:\n%s\n",
			err, cmdStr, output)
	}
}

func AddConfig(t *testing.T, jsonCfg string, node VagrantNode) {
	applyConfig(t, "add-cfg", jsonCfg, node)
}

func DelConfig(t *testing.T, jsonCfg string, node VagrantNode) {
	applyConfig(t, "del-cfg", jsonCfg, node)
}

func ApplyDesiredConfig(t *testing.T, jsonCfg string, node VagrantNode) {
	applyConfig(t, "cfg", jsonCfg, node)
}

func ConfigSetupCommon(t *testing.T, jsonCfg string, nodes []VagrantNode) {
	startNetPlugin(t, nodes)

	ApplyDesiredConfig(t, jsonCfg, nodes[0])
}

func GetIpAddress(t *testing.T, node VagrantNode, ep string) string {
	cmdStr := "netdcli -oper get -construct endpoint " + ep +
		" 2>&1 | grep IpAddress | awk -F : '{gsub(\"[,}{]\",\"\", $2); print $2}'"
	output, err := node.RunCommandWithOutput(cmdStr)

	if err != nil || string(output) == "" {
		t.Fatalf("Error '%s' getting ip for ep %s, Output: \n%s\n",
			err, ep, output)
	}
	return string(output)
}

func NetworkStateExists(node VagrantNode, network string) error {
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

func DumpNetpluginLogs(node VagrantNode) {
	cmdStr := fmt.Sprintf("sudo cat /tmp/netplugin.log")
	output, err := node.RunCommandWithOutput(cmdStr)
	if err == nil {
		log.Printf("logs on node %s: \n%s\n", node.Name, output)
	}
}
