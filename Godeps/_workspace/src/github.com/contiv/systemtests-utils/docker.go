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
	"fmt"
	"strings"
	"testing"
)

// DockerCleanupWithEnv kills and removes a container on a specified testbed node
// and with specified env-variables
func DockerCleanupWithEnv(t *testing.T, node TestbedNode, contName string, env []string) {
	if !OkToCleanup(t.Failed()) {
		return
	}
	cmdStr := fmt.Sprintf("sudo %s docker kill %s", strings.Join(env, " "), contName)
	node.RunCommand(cmdStr)
	cmdStr = fmt.Sprintf("sudo %s docker rm %s", strings.Join(env, " "), contName)
	node.RunCommand(cmdStr)
}

// DockerCleanup kills and removes a container on a specified testbed node
func DockerCleanup(t *testing.T, node TestbedNode, contName string) {
	DockerCleanupWithEnv(t, node, contName, []string{})
}

// StartServerWithEnvAndArgs starts a server container with specified env-variables
func StartServerWithEnvAndArgs(t *testing.T, node TestbedNode, contName string,
	env, dockerArgs []string) {
	cmdStr := "sudo %s docker run -d %s --name=" + contName +
		" ubuntu /bin/bash -c \"mkfifo foo && < foo\""
	cmdStr = fmt.Sprintf(cmdStr, strings.Join(env, " "),
		strings.Join(dockerArgs, " "))
	output, err := node.RunCommandWithOutput(cmdStr)
	if err != nil {
		OvsDumpInfo(node)
		t.Fatalf("Error '%s' launching container '%s', Output: \n%s\n",
			err, contName, output)
	}
}

// StartServer starts a server container
func StartServer(t *testing.T, node TestbedNode, contName string) {
	StartServerWithEnvAndArgs(t, node, contName, []string{}, []string{})
}

// StartClientWithEnvAndArgs starts a client container with specified env-variables.
// It expects ping to server container to succeed
func StartClientWithEnvAndArgs(t *testing.T, node TestbedNode, contName, ipAddress string,
	env, dockerArgs []string) {
	cmdStr := "sudo %s docker run %s --name=" + contName +
		" ubuntu /bin/bash -c \"ping -c5 " + ipAddress + "\""
	cmdStr = fmt.Sprintf(cmdStr, strings.Join(env, " "),
		strings.Join(dockerArgs, " "))
	output, err := node.RunCommandWithOutput(cmdStr)
	if err != nil {
		OvsDumpInfo(node)
		t.Fatalf("Error '%s' launching container '%s', Output: \n%s\n",
			err, contName, output)
	}

	cmdStr = fmt.Sprintf("sudo docker logs %s", contName)
	output, err = node.RunCommandWithOutput(cmdStr)
	if err != nil {
		t.Fatalf("Error '%s' fetching container '%s' logs, Output: \n%s\n",
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

// StartClient starts a client container. It expects ping to server container to succeed
func StartClient(t *testing.T, node TestbedNode, contName, ipAddress string) {
	StartClientWithEnvAndArgs(t, node, contName, ipAddress, []string{}, []string{})
}

// StartClientFailureWithEnvAndArgs starts a client container with specified env-variables.
// It expects ping to server container to failure
func StartClientFailureWithEnvAndArgs(t *testing.T, node TestbedNode, contName, ipAddress string,
	env, dockerArgs []string) {
	cmdStr := "sudo %s docker run %s --name=" + contName +
		" ubuntu /bin/bash -c \"ping -c5 " + ipAddress + "\""
	cmdStr = fmt.Sprintf(cmdStr, strings.Join(env, " "),
		strings.Join(dockerArgs, " "))
	output, err := node.RunCommandWithOutput(cmdStr)
	if err == nil {
		t.Fatalf("Ping did not fail as expected, err '%s' container '%s', "+
			"Output: \n%s\n", err, contName, output)
	}

	cmdStr = fmt.Sprintf("sudo docker logs %s", contName)
	output, err = node.RunCommandWithOutput(cmdStr)
	if err != nil || !strings.Contains(string(output), ", 100% packet loss,") {
		t.Fatalf("Ping did not fail as expected, err '%s' container '%s', "+
			"Output: \n%s\n", err, contName, output)
	}
}

// StartClientFailure starts a client container. It expects ping to server container to fail
func StartClientFailure(t *testing.T, node TestbedNode, contName, ipAddress string) {
	StartClientFailureWithEnvAndArgs(t, node, contName, ipAddress, []string{}, []string{})
}

func getContainerUUID(node TestbedNode, contName string) (string, error) {
	cmdStr := "sudo docker inspect --format='{{.Id}}' " + contName
	output, err := node.RunCommandWithOutput(cmdStr)
	if err != nil {
		output = ""
	}
	return strings.TrimSpace(output), err
}
