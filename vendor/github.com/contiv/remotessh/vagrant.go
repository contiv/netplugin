/*
Package remotessh provides host connectivity in go for system/integration
testing in a multi host environment. It supports two testbed environments viz.
baremetal and vagrant

Use this library to do remote testing with baremetal or vagrant nodes.

For example, To setup a baremetal setup with a host node with ssh reachability '1.2.3.4' and
port '22' for user 'foo', you can initialize the setup as:
	hosts := []HostInfo{
		{
      Name: "mynode",
		  SSHAddr: "1.2.3.4",
		  SSHPort: "22"
		  User: "foo",
		  PrivKey: "path/to/foo's/privkey/file",
      Env: []string{},
    },
	}

  tb := &Baremetal{}
	tb.Setup(hosts)

Or to auto connect to a vagrant based setup you can initialize the setup as:
    tb := &Vagrant{}
    tb.Setup(false, "", 3) // 3 node cluster, do not run `vagrant up`.

Once you have your favorite setup initialized, this will select the "mynode" node
and run "ls" on it.

    out, err := tb.GetNode("mynode").RunCommandWithOutput("ls")
    if err != nil {
      // exit status != 0
      panic(err)
    }

    fmt.Println(out) // already a string

If you want to walk nodes, you have a few options:

Sequentially:

    for _, node := range tb.GetNodes() {
      node.RunCommand("something")
    }

In Parallel:

    err := tb.IterateNodes(func (node remotessh.TestbedNode) error {
      return node.RunCommand("docker ps -aq | xargs docker rm")
    })

    if err != nil {
      // one or more nodes failed
      panic(err)
    }

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
	"fmt"
	"regexp"
	"strings"

	log "github.com/Sirupsen/logrus"
)

var (
	identityFileRegexp = regexp.MustCompile(`IdentityFile "*(.+?)"*$`)
	portRegexp         = regexp.MustCompile(`Port (.+?)$`)
)

// Vagrant implements a vagrant based testbed
type Vagrant struct {
	expectedNodes int
	nodes         map[string]TestbedNode
}

// setup brings up a vagrant testbed. `start` means to run `vagrant up`. env is
// a string of values to prefix before each command run on each SSHNode.
// numNodes is the number of nodes you want to track: these will be scanned
// from the vagrant file sequentially.
func (v *Vagrant) setup(start bool, env []string, numNodes int) error {
	v.nodes = map[string]TestbedNode{}

	vCmd := &VagrantCommand{ContivNodes: numNodes}
	vCmd.Env = append(vCmd.Env, env...)

	if start {
		output, err := vCmd.RunWithOutput("up")
		if err != nil {
			log.Errorf("Vagrant up failed. Error: %s Output: \n%s\n",
				err, output)
			return err
		}

		defer func() {
			if err != nil {
				v.Teardown()
			}
		}()
	}

	v.expectedNodes = numNodes

	output, err := vCmd.RunWithOutput("status")
	if err != nil {
		log.Errorf("Vagrant status failed. Error: %s Output: \n%s\n",
			err, output)
		return err
	}

	// now some hardwork of finding the names of the running nodes from status output
	re, err := regexp.Compile("[a-zA-Z0-9_\\- ]*running ")
	if err != nil {
		return err
	}
	nodeNamesBytes := re.FindAll(output, -1)
	if nodeNamesBytes == nil {
		err = fmt.Errorf("no running nodes found in vagrant status output: %s",
			output)
		return err
	}
	nodeNames := []string{}
	for _, nodeNameByte := range nodeNamesBytes {
		nodeName := strings.Fields(string(nodeNameByte))[0]
		nodeNames = append(nodeNames, nodeName)
	}

	if len(nodeNames) != numNodes {
		err = fmt.Errorf("number of running node(s) (%d) is not equal to number of expected node(s) (%d) in vagrant status output: %s",
			len(nodeNames), numNodes, output)
		return err
	}

	// some more work to figure the ssh port and private key details
	// XXX: vagrant ssh-config --host <> seems to be broken as-in it doesn't
	// correctly filter the output based on passed host-name. So filtering
	// the output ourselves below.
	if output, err = vCmd.RunWithOutput("ssh-config"); err != nil {
		return fmt.Errorf("error running vagrant ssh-config. Error: %s. Output: %s", err, output)
	}

	if re, err = regexp.Compile("Host [a-zA-Z0-9_-]+|HostName.*|Port [0-9]+|IdentityFile .*"); err != nil {
		return err
	}

	nodeInfosBytes := re.FindAll(output, -1)
	if nodeInfosBytes == nil {
		return fmt.Errorf("failed to find node info in vagrant ssh-config output: %s", output)
	}

	// got the names, now fill up the vagrant-nodes structure
	for _, nodeName := range nodeNames {
		nodeInfoPos := -1
		// nodeInfos is a slice of node info orgranised as nodename, it's port and identity-file in that order per node
		for j := range nodeInfosBytes {
			if string(nodeInfosBytes[j]) == fmt.Sprintf("Host %s", nodeName) {
				nodeInfoPos = j
				break
			}
		}
		if nodeInfoPos == -1 {
			return fmt.Errorf("failed to find %q info in vagrant ssh-config output: %s", nodeName, output)
		}

		hostnameInfo := string(nodeInfosBytes[nodeInfoPos+1])
		portInfo := string(nodeInfosBytes[nodeInfoPos+2])
		idInfo := string(nodeInfosBytes[nodeInfoPos+3])

		hostname := strings.Split(hostnameInfo, " ")
		if len(hostname) != 2 {
			return fmt.Errorf("failed to find hostname in vagrant ssh-config output: %s", nodeName)
		}

		port := portRegexp.FindStringSubmatch(portInfo)
		if port == nil || len(port) < 2 {
			return fmt.Errorf("failed to find %q port info in vagrant ssh-config output: %s",
				nodeName, portInfo)
		}

		privKeyFile := identityFileRegexp.FindStringSubmatch(idInfo)
		if privKeyFile == nil || len(port) < 2 {
			return fmt.Errorf("failed to find %q identity file info in vagrant ssh-config output: %s", nodeName, idInfo)
		}

		log.Infof("Adding node: %q(%s:%s)", nodeName, port[1], privKeyFile[1])
		var node *SSHNode
		if node, err = NewSSHNode(nodeName, "vagrant", env, hostname[1], port[1], privKeyFile[1]); err != nil {
			return err
		}

		v.nodes[node.GetName()] = TestbedNode(node)
	}

	return nil
}

// Setup initializes a vagrant testbed.
func (v *Vagrant) Setup(args ...interface{}) error {
	if _, ok := args[0].(bool); !ok {
		return unexpectedSetupArgError("bool, string, int", args...)
	}
	if _, ok := args[1].([]string); !ok {
		return unexpectedSetupArgError("bool, string, int", args...)
	}
	if _, ok := args[2].(int); !ok {
		return unexpectedSetupArgError("bool, string, int", args...)
	}

	return v.setup(args[0].(bool), args[1].([]string), args[2].(int))
}

// Teardown cleans up a vagrant testbed. It performs `vagrant destroy -f` to
// tear down the environment. While this method can be useful, the notion of
// VMs that clean up after themselves (with an appropriate Makefile to control
// vm availability) will be considerably faster than a method that uses this in
// a suite teardown.
func (v *Vagrant) Teardown() {
	for _, node := range v.nodes {
		vnode, ok := node.(*SSHNode)
		if !ok {
			log.Errorf("invalid node type encountered: %T", vnode)
			continue
		}
		vnode.Cleanup()
	}
	vCmd := &VagrantCommand{ContivNodes: v.expectedNodes}
	output, err := vCmd.RunWithOutput("destroy", "-f")
	if err != nil {
		log.Errorf("Vagrant destroy failed. Error: %s Output: \n%s\n",
			err, output)
	}

	v.nodes = map[string]TestbedNode{}
	v.expectedNodes = 0
}

// GetNode obtains a node by name. The name is the name of the VM provided at
// `config.vm.define` time in Vagrantfiles. It is *not* the hostname of the
// machine, which is `vagrant` for all VMs by default.
func (v *Vagrant) GetNode(name string) TestbedNode {
	return v.nodes[name]
}

// GetNodes returns the nodes in a vagrant setup, returned sequentially.
func (v *Vagrant) GetNodes() []TestbedNode {
	var ret []TestbedNode
	for _, value := range v.nodes {
		ret = append(ret, value)
	}

	return ret
}

// IterateNodes walks each host and executes the function supplied. On error,
// it waits for all hosts to complete before returning the error, if any.
func (v *Vagrant) IterateNodes(fn func(TestbedNode) error) error {
	return iterateNodes(v, fn)
}

// SSHExecAllNodes will ssh into each host and run the specified command.
func (v *Vagrant) SSHExecAllNodes(cmd string) error {
	return sshExecAllNodes(v, cmd)
}
