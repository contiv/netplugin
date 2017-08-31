/***
Copyright 2016 Cisco Systems Inc. All rights reserved.

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

package cniapi

// cni api definitions shared between the cni executable and netplugin

// PluginPath is the path to the listen socket directory for netplugin
const PluginPath = "/run/contiv"

// ContivCniSocket is the full path to the listen socket for netplugin
const ContivCniSocket = "/run/contiv/contiv-cni.sock"

// EPAddURL is the rest point for adding an endpoint
const EPAddURL = "/ContivCNI.AddPod"

// EPDelURL is the rest point for deleting an endpoint
const EPDelURL = "/ContivCNI.DelPod"

// CNIPodAttr holds attributes of the pod to be attached or detached
type CNIPodAttr struct {
	Name             string `json:"K8S_POD_NAME,omitempty"`
	K8sNameSpace     string `json:"K8S_POD_NAMESPACE,omitempty"`
	InfraContainerID string `json:"K8S_POD_INFRA_CONTAINER_ID,omitempty"`
	NwNameSpace      string `json:"CNI_NETNS,omitempty"`
	IntfName         string `json:"CNI_IFNAME,omitempty"`
}

// RspAddPod contains the response to the AddPod
type RspAddPod struct {
	Result      uint   `json:"result,omitempty"`
	EndpointID  string `json:"endpointid,omitempty"`
	IPAddress   string `json:"ipaddress,omitempty"`
	IPv6Address string `json:"ipv6address,omitempty"`
	ErrMsg      string `json:"errmsg,omitempty"`
	ErrInfo     string `json:"errinfo,omitempty"`
}
