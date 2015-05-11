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

package utils

import (
	"fmt"
	"log"
	"os"
)

type Dind struct {
	expectedNodes int
	nodes         []TestbedNode
}

func (v *Dind) Setup(env string, numNodes int) error {

	err := os.Chdir(os.Getenv("CONTIV_HOST_GOPATH") + "/src/github.com/contiv/netplugin")
	if err != nil {
		log.Printf("chDir failed. Error: %s ",
			err)
	}
	cmd := &TestCommand{ContivNodes: numNodes, ContivEnv: env}
	output, err := cmd.RunWithOutput("scripts/dockerhost/start-dockerhosts")
	if err != nil {
		log.Printf("start-dockerhosts failed. Error: %s Output: \n%s\n",
			err, output)
		return err
	}
	v.expectedNodes = numNodes
	defer func() {
		if err != nil {
			v.Teardown()
		}
	}()

	// TODO : Check if expected nodes docker containers are running
	// TODO : Find names of docker hosts using Docker API
	// For now assume the node names
	for i := 0; i < numNodes; i++ {
		nodeName := fmt.Sprintf("%s%d", "netplugin-node", i+1)
		log.Printf("Adding node: %q", nodeName)
		node := TestbedNode(DindNode{Name: nodeName, NodeNum: i + 1})
		v.nodes = append(v.nodes, node)
	}

	return nil
}

func (v *Dind) Teardown() {
	cmd := &TestCommand{ContivNodes: v.expectedNodes}
	output, err := cmd.RunWithOutput("scripts/dockerhost/cleanup-dockerhosts")
	if err != nil {
		log.Printf("cleanup-dockerhosts failed. Error: %s Output:\n%s\n",
			err, output)
	}

	v.nodes = []TestbedNode{}
	v.expectedNodes = 0
}

func (v *Dind) GetNodes() []TestbedNode {
	return v.nodes
}
