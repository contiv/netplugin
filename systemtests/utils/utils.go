package utils

import (
	"fmt"
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

func ConfigSetupCommon(t *testing.T, jsonCfg string, nodes []VagrantNode) {
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

	node := nodes[0]
	// replace newlines with space and "(quote) with \"(escaped quote) for
	// echo to consume and produce desired json config
	jsonCfg = strings.Replace(
		strings.Replace(jsonCfg, "\n", " ", -1),
		"\"", "\\\"", -1)
	cmdStr := fmt.Sprintf("echo %s > /tmp/netdcli.cfg", jsonCfg)
	output, err := node.RunCommandWithOutput(cmdStr)
	if err != nil {
		t.Fatalf("Failed to create netdcli.cfg file. Error: %s\nCmd: %q\nOutput:\n%s\n",
			err, cmdStr, output)
	}

	cmdStr = "netdcli -cfg /tmp/netdcli.cfg 2>&1"
	output, err = node.RunCommandWithOutput(cmdStr)
	if err != nil {
		t.Fatalf("Failed to issue netdcli. Error: %s\nCmd: %q\nOutput:\n%s\n",
			err, cmdStr, output)
	}
}
