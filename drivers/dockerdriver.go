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

package drivers

import (
	"fmt"
    "github.com/samalba/dockerclient"

	"github.com/contiv/netplugin/core"
)

// implements the StateDriver interface for an etcd based distributed
// key-value store used to store config and runtime state for the netplugin.

type DockerDriverConfig struct {
    Docker struct {
        Socket string
    }
}

type DockerDriver struct {
	Client *dockerclient.DockerClient
}

func (d *DockerDriver) Init(config *core.Config) error {
	if config == nil {
		return &core.Error{Desc: fmt.Sprintf("Invalid arguments. cfg: %v", config)}
	}

	cfg, ok := config.V.(*DockerDriverConfig)

	if !ok {
		return &core.Error{Desc: "Invalid config type passed!"}
	}

    // TODO: ADD TLS support
	d.Client, _ = dockerclient.NewDockerClient(cfg.Docker.Socket, nil)

	return nil
}

func (d *DockerDriver) Deinit() {
}

