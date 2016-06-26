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

package master

const (
	//DesiredConfigRESTEndpoint is the REST endpoint to post desired configuration
	DesiredConfigRESTEndpoint = "desired-config"
	//AddConfigRESTEndpoint is the REST endpoint to post configuration additions
	AddConfigRESTEndpoint = "add-config"
	// DelConfigRESTEndpoint is the REST endpoint to post configuration deletions
	DelConfigRESTEndpoint = "del-config"
	//HostBindingConfigRESTEndpoint is the REST endpoint to post host binding configuration
	HostBindingConfigRESTEndpoint = "host-bindings-config"
	//GetEndpointRESTEndpoint is the REST endpoint to request info of an endpoint
	GetEndpointRESTEndpoint = "endpoint"
	//GetEndpointsRESTEndpoint is the REST endpoint to request info of all endpoints
	GetEndpointsRESTEndpoint = "endpoints"
	//GetNetworkRESTEndpoint is the REST endpoint to request info of a network
	GetNetworkRESTEndpoint = "network"
	//GetNetworksRESTEndpoint is the REST endpoint to request info of all networks
	GetNetworksRESTEndpoint = "networks"
	//GetVersionRESTEndpoint is the REST endpoint to get version info
	GetVersionRESTEndpoint = "version"
	//GetServiceRESTEndpoint is the REST endpoint to get service info of a service
	GetServiceRESTEndpoint = "service"
	//GetServicesRESTEndpoint is the REST endpoint to request info of all services
	GetServicesRESTEndpoint = "services"
)
