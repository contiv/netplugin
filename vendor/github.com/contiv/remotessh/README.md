## remotessh: Test your remote files through SSH with a simple library.

godoc is [here](https://godoc.org/github.com/contiv/remotessh)

Use this library to do remote testing of vagrant nodes.

For example, this will select the "mynode" node and run "ls" on it.

```go
    vagrant := &Vagrant{}
    vagrant.Setup(false, "", 3) // 3 node cluster, do not run `vagrant up`.
    out, err := vagrant.GetNode("mynode").RunCommandWithOutput("ls")
    if err != nil {
      // exit status != 0
      panic(err)
    }

    fmt.Println(out) // already a string
```

Similarly for a Baremetal node:

```go
hosts := []HostInfo{
		{
			Name:        "self",
			SSHAddr:     "127.0.0.1",
			SSHPort:     "22",
			User:        "vagrant",
			PrivKeyFile: "/vagrant/testdata/insecure_private_key",
		},
		{
			Name:        "self1",
			SSHAddr:     "127.0.0.1",
			SSHPort:     "22",
			User:        "vagrant",
			PrivKeyFile: "/vagrant/testdata/insecure_private_key",
		},
	}
	bm := &Baremetal{}
	c.Assert(bm.Setup(hosts), IsNil)
    out, err := bm.GetNode("mynode").RunCommandWithOutput("ls")
    if err != nil {
      // exit status != 0
      panic(err)
    }

    fmt.Println(out) // already a string
```

If you want to walk nodes, you have a few options:

Sequentially:

```go
    vagrant := &remotessh.Vagrant{}
    vagrant.Setup(false, "", 3)
    for _, node := range vagrant.GetNodes() {
      node.RunCommand("something")
    }
```

In Parallel:

```go
    vagrant := &remotessh.Vagrant{}
    vagrant.Setup(false, "", 3)
    err := vagrant.IterateNodes(func (node remotessh.TestbedNode) error {
      return node.RunCommand("docker ps -aq | xargs docker rm")
    })

    if err != nil {
      // one or more nodes failed
      panic(err)
    }
```

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
