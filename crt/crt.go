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

// CRT is an abstraction for Container Runtimes.
type CRT struct {
	ContainerIf crtclient.ContainerIf
}

// Config is the configuration for the container runtype. The type is
// polymorphic to allow for multiple runtimes to be supported.
type Config struct {
	CRT struct {
		Type string
	}
}

type containerIfTypes struct {
	CRTType    reflect.Type
	ConfigType reflect.Type
}

var containerIfRegistry = map[string]containerIfTypes{
	"docker": containerIfTypes{
		CRTType:    reflect.TypeOf(docker.Docker{}),
		ConfigType: reflect.TypeOf(docker.Config{}),
	},
}

// AttachEndpoint attaches an endpoint to a container.
func (c *CRT) AttachEndpoint(
	contEpContext *crtclient.ContainerEPContext) error {
	return c.ContainerIf.AttachEndpoint(contEpContext)
}

// DetachEndpoint detaches an endpoint from a container.
func (c *CRT) DetachEndpoint(contEpContext *crtclient.ContainerEPContext) error {
	return c.ContainerIf.DetachEndpoint(contEpContext)
}

// GetContainerID obtains the container identifier for the given name.
func (c *CRT) GetContainerID(contName string) string {
	return c.ContainerIf.GetContainerID(contName)
}

// GetContainerName obtains the container name from the identifier.
func (c *CRT) GetContainerName(contID string) (string, error) {
	return c.ContainerIf.GetContainerName(contID)
}

// ExecContainer executes a specified in the container's namespace
func (c *CRT) ExecContainer(contName string, cmdParams ...string) ([]byte, error) {
	return c.ContainerIf.ExecContainer(contName, cmdParams...)
}

// Deinit deinitializes the container interface.
func (c *CRT) Deinit() {
	c.ContainerIf.Deinit()
}

// Init initializes the container runtime given a JSON configuration that
// conforms to the Config set type.
func (c *CRT) Init(configStr string) error {
	cfg := &Config{}
	err := json.Unmarshal([]byte(configStr), cfg)
	if err != nil {
		return err
	}

	if _, ok := containerIfRegistry[cfg.CRT.Type]; !ok {
		return core.Errorf("unregistered container run time")
	}

	crtConfigType := containerIfRegistry[cfg.CRT.Type].ConfigType
	crtConfig := reflect.New(crtConfigType).Interface()
	err = json.Unmarshal([]byte(configStr), crtConfig)
	if err != nil {
		return err
	}

	crtType := containerIfRegistry[cfg.CRT.Type].CRTType
	crtif := reflect.New(crtType).Interface()
	c.ContainerIf = crtif.(crtclient.ContainerIf)
	err = c.ContainerIf.Init(&crtclient.Config{V: crtConfig})
	if err != nil {
		return err
	}

	return nil
}
