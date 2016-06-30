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

package vagrantssh

import (
	"fmt"
	"sync"
)

// Testbed is a collection of test nodes
type Testbed interface {
	Setup(args ...interface{}) error
	Teardown()
	GetNodes() []TestbedNode
	GetNode(name string) TestbedNode
	IterateNodes(fn func(TestbedNode) error) error
}

func iterateNodes(tb Testbed, fn func(TestbedNode) error) error {
	wg := sync.WaitGroup{}
	nodes := tb.GetNodes()
	errChan := make(chan error, len(nodes))

	for _, node := range nodes {
		wg.Add(1)

		go func(node TestbedNode) {
			if err := fn(node); err != nil {
				errChan <- fmt.Errorf(`Error: "%v" on host: %q"`, err, node.GetName())
			}
			wg.Done()
		}(node)
	}

	wg.Wait()

	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
}

func sshExecAllNodes(tb Testbed, cmd string) error {
	return tb.IterateNodes(func(node TestbedNode) error {
		return node.RunCommand(cmd)
	})
}
