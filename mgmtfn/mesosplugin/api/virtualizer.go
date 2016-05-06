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

type VirtualizerRequestArgs struct {
	Hostname      string              `json:"hostname"`
	ContainerID   string              `json:"container_id"`
	PID           int                 `json:"pid,omitempty"`
	IPV4Addresses []string            `json:"ipv4_addrs,omitempty"`
	IPV6Addresses []string            `json:"ipv6_addrs,omitempty"`
	Netgroups     []string            `json:"netgroups,omitempty"`
	Labels        []map[string]string `json:"labels,omitempty"`
}

type VirtualizerRequest struct {
	VirtualizerRequestArgs
}

type VirtualizerResponse struct {
	Error string `json:"error,omitempty"`
}
