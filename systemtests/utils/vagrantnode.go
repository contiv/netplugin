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

// VagrantNode implements a node in vagrant testbed
type VagrantNode struct {
	Name    string
	NodeNum int
}

// RunCommand runs a shell command in a vagrant node and returns it's exit status
func (n VagrantNode) RunCommand(cmd string) error {
	vCmd := &VagrantCommand{ContivNodes: n.NodeNum}
	return vCmd.Run("ssh", n.Name, "-c", cmd)
}

// RunCommandWithOutput runs a shell command in a vagrant node and returns it's
// exit status and output
func (n VagrantNode) RunCommandWithOutput(cmd string) (string, error) {
	vCmd := &VagrantCommand{ContivNodes: n.NodeNum}
	output, err := vCmd.RunWithOutput("ssh", n.Name, "-c", cmd)
	return string(output), err
}

// RunCommandBackground runs a background command in a vagrant node
func (n VagrantNode) RunCommandBackground(cmd string) (string, error) {
	vCmd := &VagrantCommand{ContivNodes: n.NodeNum}
	output, err := vCmd.RunWithOutput("ssh", n.Name, "-c", cmd)
	return string(output), err
}

// GetName returns vagrant node's name
func (n VagrantNode) GetName() string {
	return n.Name
}
