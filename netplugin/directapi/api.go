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

package directapi

// implements the direct api definitions

// PluginPath is the path to the listen socket directory for netplugin
const PluginPath = "/run/docker/plugins"

// DriverName is the name of the listen socket for netplugin
const DriverName = "netplugin"

// NetPluginSocket is the full path to the listen socket for netplugin
const NetPluginSocket = "/run/docker/plugins/netplugin.sock"

// ReqCreateEP contains the spec of the Endpoint to be created
type ReqCreateEP struct {
	Tenant     string `json:"tenant,omitempty"`
	Network    string `json:"network,omitempty"`
	Group      string `json:"group,omitempty"`
	EndpointID string `json:"endpointid,omitempty"`
}

// RspCreateEP contains the response to the ReqCreateEP
type RspCreateEP struct {
	EndpointID string `json:"endpointid,omitempty"`
	IntfName   string `json:"intfname,omitempty"`
	IPAddress  string `json:"ipaddress,omitempty"`
}
