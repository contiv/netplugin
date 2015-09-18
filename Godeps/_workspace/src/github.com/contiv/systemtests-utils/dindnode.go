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

// DindNode implements a node in docker-in-docker (dind) testbed
type DindNode struct {
	Name    string
	NodeNum int
}

// RunCommand runs a shell command in a dind node and returns it's exit status
func (n DindNode) RunCommand(cmd string) error {
	tcmd := &TestCommand{ContivNodes: n.NodeNum}
	return tcmd.Run("sudo", "docker", "exec", n.Name, "sh", "-c", cmd)
}

// RunCommandWithOutput runs a shell command in a dind node and returns it's
// exit status and output
func (n DindNode) RunCommandWithOutput(cmd string) (string, error) {
	tcmd := &TestCommand{ContivNodes: n.NodeNum}
	output, err := tcmd.RunWithOutput("sudo", "docker", "exec", n.Name, "sh", "-c", cmd)
	return string(output), err
}

// RunCommandBackground runs a background command in a dind node
func (n DindNode) RunCommandBackground(cmd string) (string, error) {
	tcmd := &TestCommand{ContivNodes: n.NodeNum}
	output, err := tcmd.RunWithOutput("sudo", "docker", "exec", "-d", n.Name, "sh", "-c", cmd)
	return string(output), err
}

// GetName returns dind node's name
func (n DindNode) GetName() string {
	return n.Name
}
