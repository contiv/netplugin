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

package api

type IPAMRequestArgs struct {
	Hostname  string              `json:"hostname,omitempty"`
	NumIPv4   int                 `json:"num_ipv4,omitempty"`
	NumIPv6   int                 `json:"num_ipv6,omitempty"`
	UID       string              `json:"uid,omitempty"`
	Netgroups []string            `json:"netgroups,omitempty"`
	Labels    []map[string]string `json:"labels,omitempty"`
	IPs       []string            `json:"ips,omitempty"`
}

type IPAMRequest struct {
	IPAMRequestArgs
}

type IPAMResponse struct {
	IPV4  []string `json:"ipv4,omitempty"`
	IPV6  string   `json:"ipv6,omitempty"`
	Error string   `json:"error,omitempty"`
}
