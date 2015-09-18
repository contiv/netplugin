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
	"os"

	log "github.com/Sirupsen/logrus"
)

// Dind implements docker-in-docker(dind) based testbed
type Dind struct {
	expectedNodes int
	nodes         []TestbedNode
}

// Setup brings up a dind testbed
func (v *Dind) Setup(start bool, env string, numNodes int) error {
	err := os.Chdir(os.Getenv("CONTIV_HOST_GOPATH") + "/src/github.com/contiv/netplugin")
	if err != nil {
		log.Errorf("chDir failed. Error: %s ",
			err)
	}

	if start {
		cmd := &TestCommand{ContivNodes: numNodes, ContivEnv: env}
		output, err := cmd.RunWithOutput("scripts/dockerhost/start-dockerhosts")
		if err != nil {
			log.Errorf("start-dockerhosts failed. Error: %s Output: \n%s\n",
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

	// TODO : Check if expected nodes docker containers are running
	// TODO : Find names of docker hosts using Docker API
	// For now assume the node names
	for i := 0; i < numNodes; i++ {
		nodeName := fmt.Sprintf("%s%d", "netplugin-node", i+1)
		log.Infof("Adding node: %q", nodeName)
		node := TestbedNode(DindNode{Name: nodeName, NodeNum: i + 1})
		v.nodes = append(v.nodes, node)
	}

	return nil
}

// Teardown cleans up a dind testbed
func (v *Dind) Teardown() {
	cmd := &TestCommand{ContivNodes: v.expectedNodes}
	output, err := cmd.RunWithOutput("scripts/dockerhost/cleanup-dockerhosts")
	if err != nil {
		log.Errorf("cleanup-dockerhosts failed. Error: %s Output:\n%s\n",
			err, output)
	}

	v.nodes = []TestbedNode{}
	v.expectedNodes = 0
}

// GetNodes returns the nodes in a dind setup
func (v *Dind) GetNodes() []TestbedNode {
	return v.nodes
}
