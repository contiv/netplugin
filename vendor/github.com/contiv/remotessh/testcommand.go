/***
Copyright 2015 Cisco Systems Inc. All rights reserved.

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
	"fmt"
	"os"
	"os/exec"
)

// TestCommand is a command that is run on a test node
type TestCommand struct {
	ContivNodes int
	ContivEnv   []string
}

func (c *TestCommand) getCmd(cmd string, args ...string) *exec.Cmd {
	osCmd := exec.Command(cmd, args...)
	osCmd.Env = os.Environ()

	if c.ContivNodes != 0 {
		osCmd.Env = append(osCmd.Env, fmt.Sprintf("CONTIV_NODES=%d", c.ContivNodes))
	}

	osCmd.Env = append(osCmd.Env, c.ContivEnv...)

	return osCmd
}

// Run runs a command and return it's exit status
func (c *TestCommand) Run(cmd string, args ...string) error {
	return c.getCmd(cmd, args...).Run()
}

// RunWithOutput runs a command and return it's exit status and output
func (c *TestCommand) RunWithOutput(cmd string, args ...string) ([]byte, error) {
	return c.getCmd(cmd, args...).CombinedOutput()
}
