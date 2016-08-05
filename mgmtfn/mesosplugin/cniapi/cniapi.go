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
// CNI API definitions shared between the Mesos plugin and netplugin

package cniapi

// PluginPath : netplugin socket dir
const PluginPath = "/run/contiv"

// ContivMesosSocket : netplugin socket file
const ContivMesosSocket = "/run/contiv/contiv-mesos.sock"

// MesosNwIntfAdd : endpoint handling network interface add
const MesosNwIntfAdd = "/MesosNwIntfAdd"

// MesosNwIntfDel : endpoint handling network interface delete
const MesosNwIntfDel = "/MesosNwIntfDel"

// NetworkLabel : network labels from Mesos
type NetworkLabel struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

// NetpluginLabel : netplugin labels
type NetpluginLabel struct {
	TenantName   string `json:"io.contiv.tenant,omitempty"`
	NetworkName  string `json:"io.contiv.network,omitempty"`
	NetworkGroup string `json:"io.contiv.net-group,omitempty"`
}

// CniCmdReqAttr contains CNI attributes passed to netplugin
type CniCmdReqAttr struct {
	CniIfname      string         `json:"cni_ifname,omitempty"`
	CniNetns       string         `json:"cni_netns,omitempty"`
	CniContainerid string         `json:"cni_containerid,omitempty"`
	Labels         NetpluginLabel `json:"labels,omitempty"`
}

/*
 *  based on CNI Spec https://github.com/containernetworking/cni/blob/master/SPEC.md
 *  version:
 *   @tomdee
 *        SPEC: introduce "args" field and new error code
 *         tomdee committed on Jun 13
 */

const (
	// CniStatusSuccess : success return code
	CniStatusSuccess = 0
	// CniStatusErrorIncompatibleVersion : error return code
	CniStatusErrorIncompatibleVersion = 1
	// CniStatusErrorUnsupportedField : error return code
	CniStatusErrorUnsupportedField = 2

	// LabelTenantName : contiv tenant label
	LabelTenantName = "io.contiv.tenant"
	// LabelNetworkName : contiv network label
	LabelNetworkName = "io.contiv.network"
	// LabelNetworkGroup : contiv group label
	LabelNetworkGroup = "io.contiv.net-group"

	// CniDefaultVersion : CNI version
	CniDefaultVersion = "0.2"

	// EnvVarMesosAgent : MESOS env. variable
	EnvVarMesosAgent = "MESOS_AGENT_ENDPOINT"

	// CniCmdAdd : CNI commands
	CniCmdAdd = "ADD"
	// CniCmdDel : CNI commands
	CniCmdDel = "DEL"
)

// CniIpaddr contains ip/gwy/route from netplugin
type CniIpaddr struct {
	IPAddress string   `json:"ip"`
	Gateway   string   `json:"gateway,omitempty"`
	Routes    []string `json:"routes,omitempty"`
}

// CniDNS contains DNS information from netplugin
type CniDNS struct {
	NameServers []string `json:"nameservers,omitempty"`
	Domain      string   `json:"domain,omitempty"`
	Search      []string `json:"search,omitempty"`
	Options     []string `json:"options,omitempty"`
}

// CniCmdErrorResp contains error message from netplugin
type CniCmdErrorResp struct {
	CniVersion string `json:"cniVersion,omitempty"`
	ErrCode    int32  `json:"code,omitempty"`
	ErrMsg     string `json:"msg,omitempty"`
	ErrDetails string `json:"details,omitempty"`
}

// CniCmdSuccessResp contains the response from netplugin
type CniCmdSuccessResp struct {
	CniVersion string    `json:"cniVersion,omitempty"`
	IP4        CniIpaddr `json:"ip4"`
	IP6        CniIpaddr `json:"ip6"`
	DNS        CniDNS    `json:"dns,omitempty"`
}
