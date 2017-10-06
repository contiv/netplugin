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
	"io/ioutil"
	"net"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// MaxSSHRetries is the number of times we'll retry SSH connection
var MaxSSHRetries = 3

// SSHRetryDelay is the delay between SSH connection retries
var SSHRetryDelay = time.Second

// SSHNode implements a node with ssh connectivity in a testbed
type SSHNode struct {
	Name      string
	env       []string
	primaryIP net.IP
	sshAddr   string
	sshPort   string
	config    *ssh.ClientConfig
}

// NewSSHNode intializes a ssh-client based node in a testbed
func NewSSHNode(name, user string, env []string, sshAddr, sshPort, privKeyFile string) (*SSHNode, error) {
	var (
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
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
	}

	return &SSHNode{Name: name, env: env, sshAddr: sshAddr, sshPort: sshPort, config: config}, nil
}

func (n *SSHNode) dial() (*ssh.Client, error) {
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", n.sshAddr, n.sshPort), n.config)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// Cleanup does nothing
func (n *SSHNode) Cleanup() {}

func newCmdStrWithSource(cmd string, env []string) string {
	// we need to source the environment manually as the ssh package client
	// doesn't do it automatically (I guess something to do with non interative
	// mode)

	// this unwind is so that we can quote environment variables before passing
	// them back to the shell.
	var envstr string
	for _, envvar := range env {
		kv := strings.SplitN(envvar, "=", 2)

		if len(kv) == 2 {
			envstr += fmt.Sprintf("%s=%q ", kv[0], kv[1])
		} else if len(kv) == 1  && len(envvar) > 0 {
			envstr += fmt.Sprintf("%s= ", kv[0])
		}
	}

	command := fmt.Sprintf("%s bash -lc '%s'", envstr, cmd)
	log.Debugf("remotessh: Running: %q", command)
	return command
}

func (n *SSHNode) getClientAndSession() (*ssh.Client, *ssh.Session, error) {
	var client *ssh.Client
	var s *ssh.Session
	var err error

	// Retry few times if ssh connection fails
	for i := 0; i < MaxSSHRetries; i++ {
		client, err = n.dial()
		if err != nil {
			time.Sleep(SSHRetryDelay)
			continue
		}

		s, err = client.NewSession()
		if err != nil {
			client.Close()
			time.Sleep(SSHRetryDelay)
			continue
		}
		modes := ssh.TerminalModes{
			ssh.ECHO:          0,
			ssh.TTY_OP_ISPEED: 14400,
			ssh.TTY_OP_OSPEED: 14400,
		}
		// Request pseudo terminal
		if err := s.RequestPty("xterm", 40, 80, modes); err != nil {
			return nil, nil, fmt.Errorf("failed to get pseudo-terminal: %v", err)
		}

		return client, s, nil
	}

	return nil, nil, err
}

// RunCommand runs a shell command in a vagrant node and returns it's exit status
func (n *SSHNode) RunCommand(cmd string) error {
	client, s, err := n.getClientAndSession()
	if err != nil {
		return err
	}

	defer client.Close()
	defer s.Close()

	return s.Run(newCmdStrWithSource(cmd, n.env))
}

// RunCommandWithOutput runs a shell command in a vagrant node and returns it's
// exit status and output
func (n *SSHNode) RunCommandWithOutput(cmd string) (string, error) {
	client, s, err := n.getClientAndSession()
	if err != nil {
		return "", err
	}

	defer client.Close()
	defer s.Close()

	output, err := s.CombinedOutput(newCmdStrWithSource(cmd, n.env))
	return string(output), err
}

// RunCommandBackground runs a background command in a vagrant node.
func (n *SSHNode) RunCommandBackground(cmd string) error {
	// XXX we leak a connection here so we can keep the session alive. While this
	// is less than ideal it allows us to "fire and forget" from our perspective,
	// and give system tests the ability to manage the background processes themselves.
	_, s, err := n.getClientAndSession()
	if err != nil {
		return err
	}

	// start and forget about the command as user asked to run in background.
	// The limitation is we/ won't know if it fails though. Not a worry right
	// now as the test will fail anyways, but might be good to find a better way.
	return s.Start(newCmdStrWithSource(cmd, n.env))
}

// GetName returns vagrant node's name
func (n *SSHNode) GetName() string {
	return n.Name
}
