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

package state

import (
	"fmt"
	"reflect"

	"github.com/contiv/go-etcd/etcd"
	"github.com/contiv/netplugin/core"

	log "github.com/Sirupsen/logrus"
)

// implements the StateDriver interface for an etcd based distributed
// key-value store used to store config and runtime state for the netplugin.

const (
	RECURSIVE = true
)

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
		return core.Errorf("Invalid arguments. cfg: %v", config)
	}

	cfg, ok := config.V.(*EtcdStateDriverConfig)

	if !ok {
		return core.Errorf("Invalid config type passed!")
	}

	d.Client = etcd.NewClient(cfg.Etcd.Machines)

	return nil
}

func (d *EtcdStateDriver) Deinit() {
}

func (d *EtcdStateDriver) Write(key string, value []byte) error {
	_, err := d.Client.Set(key, string(value[:]), 0)

	return err
}

func (d *EtcdStateDriver) Read(key string) ([]byte, error) {
	resp, err := d.Client.Get(key, false, false)
	if err != nil {
		return []byte{}, err
	}

	return []byte(resp.Node.Value), err
}

func (d *EtcdStateDriver) ReadAll(baseKey string) ([][]byte, error) {
	resp, err := d.Client.Get(baseKey, true, false)
	if err != nil {
		return nil, err
	}

	values := [][]byte{}
	for _, node := range resp.Node.Nodes {
		values = append(values, []byte(node.Value))
	}

	return values, nil
}

func (d *EtcdStateDriver) channelEtcdEvents(etcdRsps chan *etcd.Response,
	rsps chan [2][]byte, retErr chan error) {
	for {
		// block on change notifications
		etcdRsp := <-etcdRsps

		node := etcdRsp.Node
		log.Printf("Received event for key: %s", node.Key)

		// XXX: The logic below assumes that the node returned is always a node
		// of interest. Eg: If we set a watch on /a/b/c, then we are mostly
		// interested in changes in that directory i.e. changes to /a/b/c/d1..d2
		// This works for now as the constructs like network and endpoints that
		// need to be watched are organized as above. Need to revisit when
		// this assumption changes.
		rsp := [2][]byte{nil, nil}
		if etcdRsp.Node.Value != "" {
			rsp[0] = []byte(etcdRsp.Node.Value)
		}
		if etcdRsp.PrevNode != nil && etcdRsp.PrevNode.Value != "" {
			rsp[1] = []byte(etcdRsp.PrevNode.Value)
		}

		//channel the translated response
		rsps <- rsp
	}

	// shall never come here
	retErr <- nil
}

func (d *EtcdStateDriver) WatchAll(baseKey string, rsps chan [2][]byte) error {
	etcdRsps := make(chan *etcd.Response)
	stop := make(chan bool, 1)
	recvErr := make(chan error, 1)

	go d.channelEtcdEvents(etcdRsps, rsps, recvErr)

	_, err := d.Client.Watch(baseKey, 0, RECURSIVE, etcdRsps, stop)
	if err != nil && err != etcd.ErrWatchStoppedByUser {
		log.Printf("etcd watch failed. Error: %s", err)
		return err
	}

	err = <-recvErr
	return err
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

// XXX: move this to some common file
func ReadAllStateCommon(d core.StateDriver, baseKey string, sType core.State,
	unmarshal func([]byte, interface{}) error) ([]core.State, error) {
	stateType := reflect.TypeOf(sType)
	sliceType := reflect.SliceOf(stateType)
	values := reflect.MakeSlice(sliceType, 0, 1)

	byteValues, err := d.ReadAll(baseKey)
	if err != nil {
		return nil, err
	}
	for _, byteValue := range byteValues {
		value := reflect.New(stateType)
		err = unmarshal(byteValue, value.Interface())
		if err != nil {
			return nil, err
		}
		values = reflect.Append(values, value.Elem())
	}

	stateValues := []core.State{}
	for i := 0; i < values.Len(); i++ {
		// sanity checks
		if !values.Index(i).Elem().FieldByName("CommonState").IsValid() {
			panic(fmt.Sprintf("The state structure %v is missing core.CommonState",
				stateType))
			return nil, core.Errorf("The state structure %v is missing core.CommonState",
				stateType)
		}
		//the following works as every core.State is expected to embed core.CommonState struct
		values.Index(i).Elem().FieldByName("CommonState").FieldByName("StateDriver").Set(reflect.ValueOf(d))
		stateValue := values.Index(i).Interface().(core.State)
		stateValues = append(stateValues, stateValue)
	}
	return stateValues, nil
}

func (d *EtcdStateDriver) ReadAllState(baseKey string, sType core.State,
	unmarshal func([]byte, interface{}) error) ([]core.State, error) {
	return ReadAllStateCommon(d, baseKey, sType, unmarshal)
}

func (d *EtcdStateDriver) channelStateEvents(sType core.State,
	unmarshal func([]byte, interface{}) error,
	byteRsps chan [2][]byte, rsps chan core.WatchState, retErr chan error) {
	for {
		// block on change notifications
		byteRsp := <-byteRsps

		rsp := core.WatchState{nil, nil}
		for i := 0; i < 2; i++ {
			if byteRsp[i] == nil {
				continue
			}
			stateType := reflect.TypeOf(sType)
			value := reflect.New(stateType)
			err := unmarshal(byteRsp[i], value.Interface())
			if err != nil {
				retErr <- err
				return
			}
			if !value.Elem().Elem().FieldByName("CommonState").IsValid() {
				panic(fmt.Sprintf("The state structure %v is missing core.CommonState",
					stateType))
				retErr <- core.Errorf("The state structure %v is missing core.CommonState",
					stateType)
				return
			}
			//the following works as every core.State is expected to embed core.CommonState struct
			value.Elem().Elem().FieldByName("CommonState").FieldByName("StateDriver").Set(reflect.ValueOf(d))
			switch i {
			case 0:
				rsp.Curr = value.Elem().Interface().(core.State)
			case 1:
				rsp.Prev = value.Elem().Interface().(core.State)
			}
		}

		//channel the translated response
		rsps <- rsp
	}

	// shall never come here
	retErr <- nil
}

func (d *EtcdStateDriver) WatchAllState(baseKey string, sType core.State,
	unmarshal func([]byte, interface{}) error, rsps chan core.WatchState) error {
	byteRsps := make(chan [2][]byte, 1)
	recvErr := make(chan error, 1)

	go d.channelStateEvents(sType, unmarshal, byteRsps, rsps, recvErr)

	err := d.WatchAll(baseKey, byteRsps)
	if err != nil {
		return err
	}

	err = <-recvErr
	return err

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
