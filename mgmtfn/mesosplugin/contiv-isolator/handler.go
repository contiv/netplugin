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
	"io"
	"io/ioutil"

	"github.com/contiv/netplugin/mgmtfn/mesosplugin/api"
)

type Handler interface {
	Allocate(req *api.IPAMRequest) ([]byte, error)
	Release(req *api.IPAMRequest) ([]byte, error)
	Isolate(req *api.VirtualizerRequest) ([]byte, error)
	Cleanup(req *api.VirtualizerRequest) ([]byte, error)
}

func ProcessRequest(data []byte, handler Handler) ([]byte, error) {
	var response []byte
	cmd, reqData, err := api.GetRequestType(data)
	if err != nil {
		return nil, err
	}

	switch cmd {
	case "allocate":
		req := &api.IPAMRequest{}
		req, err = api.DecodeIPAMRequest(reqData)
		if err != nil {
			return nil, err
		}
		response, err = handler.Allocate(req)
	case "release":
		req := &api.IPAMRequest{}
		req, err = api.DecodeIPAMRequest(reqData)
		if err != nil {
			return nil, err
		}
		response, err = handler.Release(req)
	case "isolate":
		req := &api.VirtualizerRequest{}
		req, err = api.DecodeVirtualizerRequest(reqData)
		if err != nil {
			return nil, err
		}
		response, err = handler.Isolate(req)
	case "cleanup":
		req := &api.VirtualizerRequest{}
		req, err = api.DecodeVirtualizerRequest(reqData)
		if err != nil {
			return nil, err
		}
		response, err = handler.Cleanup(req)
	}
	return response, err
}

func WriteError(msg string, out io.Writer) error {
	response, err := api.BuildErrorResponse(msg)
	if err != nil {
		return err
	}

	_, err = out.Write(response)
	if err != nil {
		return err
	}
	return nil
}

func HandleRequest(input io.Reader, output io.Writer, h Handler) error {
	data, err := ioutil.ReadAll(input)
	if err != nil {
		_ = WriteError(fmt.Sprintf("failed to read data: %v", err), output)
		return err
	}
	response, err := ProcessRequest(data, h)
	if err != nil {
		_ = WriteError(fmt.Sprintf("failed to handle request: %v", err), output)
		return err
	}

	_, err = output.Write(response)
	if err != nil {
		// no point to attempt to write anything here
		return err
	}

	return nil
}
