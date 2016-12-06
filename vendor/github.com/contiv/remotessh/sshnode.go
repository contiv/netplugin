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
	"os"
	"io"
	"io/ioutil"
	"net"
	"strings"
	"time"
	"sync"

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
		} else if len(kv) == 1 {
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

// ScpFromRemoteToLocal scp remote file to local dir
func (n *SSHNode) ScpFromRemoteToLocal(remoteFilename string, localFilename string) error {
	client, s, err := n.getClientAndSession()
	if err != nil {
		return err
	}
	defer client.Close()
	defer s.Close()

	w, err := s.StdinPipe()
	if err != nil {
		return err
	}
	defer w.Close()

	r, err := s.StdoutPipe()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		// write a null byte to remote to initiate
		nullByte := []byte{0}
		w.Write(nullByte)

		// read commands from remote 
		data := make ([]byte, 100) 
		bytesRead, err := r.Read(data)
		if err != nil && err != io.EOF {
			log.Errorf("scp command failed to start. err=%s", err.Error())
			return
		}
		cmds := strings.Split(string(data[:bytesRead]), " ")
		filesize := "0"
		if len(cmds) > 2 {
			filesize = cmds[1]
		}
		log.Debugf("remotessh: scp %s %s : filesize %s, Cmd %d bytes.. Cmd data %s", 
			remoteFilename, localFilename, filesize,
			bytesRead, string(data[:bytesRead]))

		f,err := os.Create(localFilename)
		if err != nil {
			log.Errorf("failed to create local file %s", localFilename)
			return
		}
		defer f.Close()

		fileContents := make ([]byte, 100) 
		more := true
		for more {
			// write a null byte to remote to initiate
			w.Write(nullByte)

			bytesRead, err = r.Read(fileContents)
			if err == io.EOF {
				more = false
			} else if err != nil {
				log.Errorf("could not scp file %s completely. err=%s", 
					remoteFilename, err.Error())
			}
			f.Write(fileContents[:bytesRead])
		}
		f.Sync()
	} ()

	s.Run("/usr/bin/scp -f " + remoteFilename)
	wg.Wait()
	return nil
}

// ScpFromLocalToRemote scp file to remote node
func (n *SSHNode) ScpFromLocalToRemote(localFilename string, remoteFilename string) error {
	client, s, err := n.getClientAndSession()
	if err != nil {
		return err
	}
	defer client.Close()
	defer s.Close()

	w, err := s.StdinPipe()
	if err != nil {
		return err
	}
	defer w.Close()

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		fileContents, err := ioutil.ReadFile(localFilename)
		if err != nil {
			return 
		}
		log.Debugf("Local file (%d bytes) contents: %s\n", len(string(fileContents)), string(fileContents))

		cmds := []byte(fmt.Sprintf("C0664 %d %s\n", len(fileContents), remoteFilename))
		w.Write(cmds)
		log.Debugf("remotessh: scp %s %s : filesize %d ", 
			localFilename, remoteFilename, len(fileContents))

		w.Write(fileContents)
		fmt.Fprintln(w, "\x00")
	} ()

	s.Run("/usr/bin/scp -t " + remoteFilename)
	wg.Wait()
	return nil
}
