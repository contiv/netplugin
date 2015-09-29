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
	"io/ioutil"

	"golang.org/x/crypto/ssh"
)

// VagrantNode implements a node in vagrant testbed
type VagrantNode struct {
	Name   string
	client *ssh.Client
}

//NewVagrantNode intializes a node in vagrant testbed
func NewVagrantNode(name, port, privKeyFile string) (*VagrantNode, error) {
	var (
		vnode      *VagrantNode
		err        error
		signer     ssh.Signer
		privateKey []byte
	)

	if privateKey, err = ioutil.ReadFile(privKeyFile); err != nil {
		return nil, err
	}

	if signer, err = ssh.ParsePrivateKey(privateKey); err != nil {
		return nil, err
	}

	config := &ssh.ClientConfig{
		User: "vagrant",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
	}

	vnode = &VagrantNode{Name: name}
	if vnode.client, err = ssh.Dial("tcp", fmt.Sprintf("127.0.0.1:%s", port), config); err != nil {
		return nil, err
	}

	return vnode, nil
}

//Cleanup clears the ssh client resources
func (n *VagrantNode) Cleanup() {
	n.client.Close()
}

func newCmdStrWithSource(cmd string) string {
	// we need to source the environment manually as the ssh package client
	// doesn't do it automatically (I guess something to do with non interative
	// mode)
	return fmt.Sprintf("bash -lc '%s'", cmd)
}

// RunCommand runs a shell command in a vagrant node and returns it's exit status
func (n *VagrantNode) RunCommand(cmd string) error {
	var (
		s   *ssh.Session
		err error
	)

	if s, err = n.client.NewSession(); err != nil {
		return err
	}
	defer s.Close()

	if err := s.RequestPty("vt100", 80, 25, ssh.TerminalModes{}); err != nil {
		fmt.Println(err)
		return err
	}

	return s.Run(newCmdStrWithSource(cmd))
}

// RunCommandWithOutput runs a shell command in a vagrant node and returns it's
// exit status and output
func (n *VagrantNode) RunCommandWithOutput(cmd string) (string, error) {
	var (
		s   *ssh.Session
		err error
	)

	if s, err = n.client.NewSession(); err != nil {
		return "", err
	}
	defer s.Close()

	output, err := s.CombinedOutput(newCmdStrWithSource(cmd))
	return string(output), err
}

// RunCommandBackground runs a background command in a vagrant node
func (n *VagrantNode) RunCommandBackground(cmd string) (string, error) {
	var (
		s   *ssh.Session
		err error
	)

	if s, err = n.client.NewSession(); err != nil {
		return "", err
	}
	defer s.Close()

	// start and forget about the command as user asked to run in background.
	// The limitation is we/ won't know if it fails though. Not a worry right
	// now as the test will fail anyways, but might be good to find a better way.
	return "", s.Start(newCmdStrWithSource(cmd))
}

// GetName returns vagrant node's name
func (n *VagrantNode) GetName() string {
	return n.Name
}
