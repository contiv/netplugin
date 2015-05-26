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

package crt

import (
	"encoding/json"
	"reflect"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/crtclient"
	"github.com/contiv/netplugin/crtclient/docker"
)

type Crt struct {
	ContainerIf crtclient.ContainerIf
}

type CrtConfig struct {
	Crt struct {
		Type string
	}
}

type ContainerIfTypes struct {
	CrtType       reflect.Type
	CrtConfigType reflect.Type
}

var ContainerIfRegistry = map[string]ContainerIfTypes{
	"docker": ContainerIfTypes{
		CrtType:       reflect.TypeOf(docker.Docker{}),
		CrtConfigType: reflect.TypeOf(docker.DockerConfig{}),
	},
}

func (c *Crt) AttachEndpoint(
	contEpContext *crtclient.ContainerEPContext) error {
	return c.ContainerIf.AttachEndpoint(contEpContext)
}

func (c *Crt) DetachEndpoint(contEpContext *crtclient.ContainerEPContext) error {
	return c.ContainerIf.DetachEndpoint(contEpContext)
}

func (c *Crt) GetContainerID(contName string) string {
	return c.ContainerIf.GetContainerID(contName)
}

func (c *Crt) GetContainerName(contID string) (string, error) {
	return c.ContainerIf.GetContainerName(contID)
}

func (c *Crt) Deinit() {
	c.ContainerIf.Deinit()
}

func (c *Crt) Init(configStr string) error {

	cfg := &CrtConfig{}
	err := json.Unmarshal([]byte(configStr), cfg)
	if err != nil {
		return err
	}

	if _, ok := ContainerIfRegistry[cfg.Crt.Type]; !ok {
		return core.Errorf("unregistered container run time")
	}

	crtConfigType := ContainerIfRegistry[cfg.Crt.Type].CrtConfigType
	crtConfig := reflect.New(crtConfigType).Interface()
	err = json.Unmarshal([]byte(configStr), crtConfig)
	if err != nil {
		return err
	}

	crtType := ContainerIfRegistry[cfg.Crt.Type].CrtType
	crtif := reflect.New(crtType).Interface()
	c.ContainerIf = crtif.(crtclient.ContainerIf)
	err = c.ContainerIf.Init(&crtclient.Config{V: crtConfig})
	if err != nil {
		return err
	}

	return nil
}
