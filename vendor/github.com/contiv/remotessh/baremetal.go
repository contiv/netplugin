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
	"fmt"

	log "github.com/Sirupsen/logrus"
)

func unexpectedSetupArgError(exptdTypes string, rcvdArgs ...interface{}) error {
	rcvdTypes := ""
	for _, arg := range rcvdArgs {
		rcvdTypes = fmt.Sprintf("%s, %T", rcvdTypes, arg)
	}
	return fmt.Errorf("unexpected args to Setup(). Expected: %s, Received: %s", rcvdTypes, exptdTypes)
}

// HostInfo contains host specific connectivity info for setting up testbed node
type HostInfo struct {
	Name        string
	SSHAddr     string
	SSHPort     string
	User        string
	PrivKeyFile string
	Env         []string
}

// Baremetal implements a host based testbed
type Baremetal struct {
	nodes map[string]TestbedNode
}

// setup implement baremetal testbed specific initilization. It takes a list of
// connectivity infos about the hosts and initilizes ssh client state used by tests.
func (b *Baremetal) setup(hosts []HostInfo) error {
	b.nodes = make(map[string]TestbedNode)

	for _, h := range hosts {
		log.Infof("Adding node: %q(%s:%s:%s:%s)", h.Name, h.SSHAddr, h.SSHPort, h.User, h.PrivKeyFile)
		var (
			node *SSHNode
			err  error
		)
		if node, err = NewSSHNode(h.Name, h.User, h.Env, h.SSHAddr, h.SSHPort, h.PrivKeyFile); err != nil {
			return err
		}
		b.nodes[node.GetName()] = TestbedNode(node)
	}

	return nil
}

// Setup initializes a baremetal testbed.
func (b *Baremetal) Setup(args ...interface{}) error {
	if _, ok := args[0].([]HostInfo); !ok {
		return unexpectedSetupArgError("[]remotessh.HostInfo", args...)
	}
	return b.setup(args[0].([]HostInfo))
}

// Teardown cleans up a baremetal testbed.
func (b *Baremetal) Teardown() {
	for _, node := range b.nodes {
		vnode, ok := node.(*SSHNode)
		if !ok {
			log.Errorf("invalid node type encountered: %T", vnode)
			continue
		}
		vnode.Cleanup()
	}
	b.nodes = nil
}

// GetNode obtains a node by name. The name is the name of the host provided at
// the time of testbed Setup.
func (b *Baremetal) GetNode(name string) TestbedNode {
	return b.nodes[name]
}

// GetNodes returns the nodes in a baremetal setup, returned sequentially.
func (b *Baremetal) GetNodes() []TestbedNode {
	var ret []TestbedNode
	for _, value := range b.nodes {
		ret = append(ret, value)
	}

	return ret
}

// IterateNodes walks each host and executes the function supplied. On error,
// it waits for all hosts to complete before returning the error, if any.
func (b *Baremetal) IterateNodes(fn func(TestbedNode) error) error {
	return iterateNodes(b, fn)
}

// SSHExecAllNodes will ssh into each host and run the specified command.
func (b *Baremetal) SSHExecAllNodes(cmd string) error {
	return sshExecAllNodes(b, cmd)
}
