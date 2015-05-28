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

package crtclient

// ContainerEPContext is used to manage the parameters for all
// container+endpoint management operations.
type ContainerEPContext struct {
	NewContName    string
	NewAttachUUID  string
	CurrContName   string
	CurrAttachUUID string
	InterfaceID    string
	IPAddress      string
	SubnetLen      uint
	DefaultGw      string
}

// Config should be replaced by core.Config FIXME
type Config struct {
	V interface{}
}

// ContainerIf implementations are used to configure and manage container
// interfaces.
type ContainerIf interface {
	Init(config *Config) error
	Deinit()
	AttachEndpoint(ctx *ContainerEPContext) error
	DetachEndpoint(ctx *ContainerEPContext) error
	GetContainerID(contName string) string
	GetContainerName(contName string) (string, error)
}
