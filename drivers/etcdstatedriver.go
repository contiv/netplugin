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

package drivers

import (
	"encoding/base64"
	"fmt"
	"github.com/coreos/go-etcd/etcd"

	"github.com/contiv/netplugin/core"
)

// implements the StateDriver interface for an etcd based distributed
// key-value store used to store config and runtime state for the netplugin.

type EtcdStateDriverConfig struct {
	Etcd struct {
		Machines []string
	}
}

type EtcdStateDriver struct {
	Client *etcd.Client
}

func (d *EtcdStateDriver) Init(config *core.Config) error {
	if config == nil {
		return &core.Error{Desc: fmt.Sprintf("Invalid arguments. cfg: %v", config)}
	}

	cfg, ok := config.V.(*EtcdStateDriverConfig)

	if !ok {
		return &core.Error{Desc: "Invalid config type passed!"}
	}

	d.Client = etcd.NewClient(cfg.Etcd.Machines)

	return nil
}

func (d *EtcdStateDriver) Deinit() {
}

func (d *EtcdStateDriver) Write(key string, value []byte) error {
	// XXX: etcd go client right now accepts only string values, so
	// encode the received byte array as base64 string before storing it.
	encodedStr := base64.URLEncoding.EncodeToString(value)
	_, err := d.Client.Set(key, encodedStr, 0)

	return err
}

func (d *EtcdStateDriver) Read(key string) ([]byte, error) {
	resp, err := d.Client.Get(key, false, false)
	if err != nil {
		return []byte{}, err
	}

	// XXX: etcd go client right now accepts only string values, so
	// decode the received data as base64 string.
	decodedStr := []byte{}
	decodedStr, err = base64.URLEncoding.DecodeString(resp.Node.Value)
	if err != nil {
		return []byte{}, err
	}

	return decodedStr, err
}

func (d *EtcdStateDriver) ClearState(key string) error {
	_, err := d.Client.Delete(key, false)
	return err
}

func (d *EtcdStateDriver) ReadState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) error {
	encodedState, err := d.Read(key)
	if err != nil {
		return err
	}

	err = unmarshal(encodedState, value)
	if err != nil {
		return err
	}

	return nil
}

func (d *EtcdStateDriver) WriteState(key string, value core.State,
	marshal func(interface{}) ([]byte, error)) error {
	encodedState, err := marshal(value)
	if err != nil {
		return err
	}

	err = d.Write(key, encodedState)
	if err != nil {
		return err
	}

	return nil
}
