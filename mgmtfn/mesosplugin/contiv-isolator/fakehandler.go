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

package main

import (
	"fmt"

	"github.com/contiv/netplugin/mgmtfn/mesosplugin/api"
)

type FakeHandler struct{}

func (h *FakeHandler) Allocate(req *api.IPAMRequest) ([]byte, error) {
	response := api.IPAMResponse{}
	data, err := api.EncodeIPAMResponse(&response)
	if err != nil {
		return nil, err
	}
	fmt.Println(req)
	return data, nil
}

func (h *FakeHandler) Release(req *api.IPAMRequest) ([]byte, error) {
	response := api.IPAMResponse{}
	data, err := api.EncodeIPAMResponse(&response)
	if err != nil {
		return nil, err
	}
	fmt.Println(req)
	return data, nil
}

func (h *FakeHandler) Isolate(req *api.VirtualizerRequest) ([]byte, error) {
	response := api.VirtualizerResponse{}
	data, err := api.EncodeVirtualizerResponse(&response)
	if err != nil {
		return nil, err
	}
	fmt.Println(req)
	return data, nil
}

func (h *FakeHandler) Cleanup(req *api.VirtualizerRequest) ([]byte, error) {
	response := api.VirtualizerResponse{}
	data, err := api.EncodeVirtualizerResponse(&response)
	if err != nil {
		return nil, err
	}
	fmt.Println(req)
	return data, nil
}
