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
	"log"
	"regexp"
	"strings"

	"github.com/contiv/netplugin/core"
)

type Vagrant struct {
	expectedNodes int
	nodes         []TestbedNode
}

func (v *Vagrant) Setup(env string, numNodes int) error {
	vCmd := &VagrantCommand{ContivNodes: numNodes, ContivEnv: env}
	output, err := vCmd.RunWithOutput("up")
	if err != nil {
		log.Printf("Vagrant up failed. Error: %s Output: \n%s\n",
			err, output)
		return err
	}
	v.expectedNodes = numNodes
	defer func() {
		if err != nil {
			v.Teardown()
		}
	}()

	output, err = vCmd.RunWithOutput("status")
	if err != nil {
		log.Printf("Vagrant status failed. Error: %s Output: \n%s\n",
			err, output)
		return err
	}

	// now some hardwork of finding the names of the running nodes from status output
	re, err := regexp.Compile("[a-zA-Z0-9_\\- ]*running \\(virtualbox\\)")
	if err != nil {
		return err
	}
	nodeNamesBytes := re.FindAll(output, -1)
	if nodeNamesBytes == nil {
		err = &core.Error{Desc: fmt.Sprintf("No running nodes found in vagrant status output: \n%s\n", output)}
		return err
	}
	nodeNames := []string{}
	for _, nodeNameByte := range nodeNamesBytes {
		nodeName := strings.Fields(string(nodeNameByte))[0]
		nodeNames = append(nodeNames, nodeName)
	}
	if len(nodeNames) != numNodes {
		err = &core.Error{Desc: fmt.Sprintf("Number of running node(s) (%d) is not equal to number of expected node(s) (%d) in vagrant status output: \n%s\n",
			len(nodeNames), numNodes, output)}
		return err
	}

	// got the names, now fill up the vagrant-nodes structure
	for i, nodeName := range nodeNames {
		log.Printf("Adding node: %q", nodeName)
		node := TestbedNode(VagrantNode{Name: nodeName, NodeNum: i + 1})
		v.nodes = append(v.nodes, node)
	}

	return nil
}

func (v *Vagrant) Teardown() {
	vCmd := &VagrantCommand{ContivNodes: v.expectedNodes}
	output, err := vCmd.RunWithOutput("destroy", "-f")
	if err != nil {
		log.Printf("Vagrant destroy failed. Error: %s Output: \n%s\n",
			err, output)
	}

	v.nodes = []TestbedNode{}
	v.expectedNodes = 0
}

func (v *Vagrant) GetNodes() []TestbedNode {
	return v.nodes
}
