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

type DindNode struct {
	Name    string
	NodeNum int
}

func (n DindNode) RunCommand(cmd string) error {
	tcmd := &TestCommand{ContivNodes: n.NodeNum}
	return tcmd.Run("sh", "-c", "sudo docker exec "+n.Name+" "+cmd)
}

func (n DindNode) RunCommandWithOutput(cmd string) (string, error) {
	tcmd := &TestCommand{ContivNodes: n.NodeNum}
	output, err := tcmd.RunWithOutput("sh", "-c", "sudo docker exec "+n.Name+" "+cmd)
	return string(output), err
}

func (n DindNode) RunCommandBackground(cmd string) (string, error) {
	tcmd := &TestCommand{ContivNodes: n.NodeNum}
	output, err := tcmd.RunWithOutput("sh", "-c", "sudo docker exec -d "+n.Name+" "+cmd)
	return string(output), err
}

func (n DindNode) GetName() string {
	return n.Name
}
