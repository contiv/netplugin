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

package remotessh

import (
	"os"
	"os/exec"
)

// VagrantCommand is a command that is run on a vagrant node
type VagrantCommand struct {
	ContivNodes int
	Env         []string
}

func (c *VagrantCommand) getCmd(cmd string, args ...string) *exec.Cmd {
	newArgs := append([]string{cmd}, args...)
	osCmd := exec.Command("vagrant", newArgs...)
	osCmd.Env = os.Environ()
	osCmd.Env = append(osCmd.Env, c.Env...)
	return osCmd
}

// Run runs a command and return its exit status
func (c *VagrantCommand) Run(cmd string, args ...string) error {
	return c.getCmd(cmd, args...).Run()
}

// RunWithOutput runs a command and return its exit status and output
func (c *VagrantCommand) RunWithOutput(cmd string, args ...string) ([]byte, error) {
	return c.getCmd(cmd, args...).CombinedOutput()
}
