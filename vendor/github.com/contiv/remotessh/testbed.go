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

// Testbed is a collection of test nodes
type Testbed interface {
	Setup(args ...interface{}) error
	Teardown()
	GetNodes() []TestbedNode
	GetNode(name string) TestbedNode
	IterateNodes(fn func(TestbedNode) error) error
}

func iterateNodes(tb Testbed, fn func(TestbedNode) error) error {
	nodes := tb.GetNodes()
	errChan := make(chan error, len(nodes))

	for _, node := range nodes {
		go func(node TestbedNode) {
			errChan <- fn(node)
		}(node)
	}

	var err error

	for range nodes {
		if chanerr := <-errChan; chanerr != nil {
			err = chanerr
		}
	}

	return err
}

func sshExecAllNodes(tb Testbed, cmd string) error {
	return tb.IterateNodes(func(node TestbedNode) error {
		return node.RunCommand(cmd)
	})
}
