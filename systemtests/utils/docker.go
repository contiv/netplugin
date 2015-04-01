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
	"strings"
	"testing"
)

func DockerCleanup(t *testing.T, node VagrantNode, contName string) {
	if !OkToCleanup(t.Failed()) {
		return
	}
	cmdStr := "sudo docker kill " + contName
	node.RunCommand(cmdStr)
	cmdStr = "sudo docker rm " + contName
	node.RunCommand(cmdStr)
}

func StartServer(t *testing.T, node VagrantNode, contName string) {
	cmdStr := "sudo docker run -d --name=" + contName +
		" ubuntu /bin/bash -c 'mkfifo foo && < foo'"
	output, err := node.RunCommandWithOutput(cmdStr)
	if err != nil {
		OvsDumpInfo(node)
		t.Fatalf("Error '%s' launching container '%s', Output: \n%s\n",
			err, contName, output)
	}
}

func StartClient(t *testing.T, node VagrantNode, contName, ipAddress string) {
	cmdStr := "sudo docker run --name=" + contName +
		" ubuntu /bin/bash -c 'ping -c5 " + ipAddress + "'"
	output, err := node.RunCommandWithOutput(cmdStr)
	if err != nil {
		OvsDumpInfo(node)
		t.Fatalf("Error '%s' launching container '%s', Output: \n%s\n",
			err, contName, output)
	}

	//verify that the output indicates <100% loss (some loss is expected due to
	// timing of interface creation and starting ping)
	if strings.Contains(string(output), ", 100% packet loss,") {
		OvsDumpInfo(node)
		t.Fatalf("Ping test failed for container '%s', Output: \n%s\n",
			contName, output)
	}
}

func StartClientFailure(t *testing.T, node VagrantNode, contName, ipAddress string) {
	cmdStr := "sudo docker run --name=" + contName +
		" ubuntu /bin/bash -c 'ping -c5 " + ipAddress + "'"
	output, err := node.RunCommandWithOutput(cmdStr)
	if err == nil || !strings.Contains(string(output), ", 100% packet loss,") {
		t.Fatalf("Ping did not fail as expected, err '%s' container '%s', "+
			"Output: \n%s\n", err, contName, output)
	}
}

func getUUID(node VagrantNode, contName string) (string, error) {
	cmdStr := "sudo docker inspect --format='{{.Id}}' " + contName
	output, err := node.RunCommandWithOutput(cmdStr)
	if err != nil {
		output = ""
	}
	return strings.TrimSpace(output), err
}
