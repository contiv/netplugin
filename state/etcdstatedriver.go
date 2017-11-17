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
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/contiv/netplugin/core"
	client "github.com/coreos/etcd/clientv3"

	log "github.com/Sirupsen/logrus"
)

const (
	ctxTimeout     = 20 * time.Second // etcd timeout
	maxEtcdRetries = 10               // Max times to retry in case of failure
)

// EtcdStateDriverConfig encapsulates the etcd endpoints used to communicate
// with it.
type EtcdStateDriverConfig struct {
	Etcd struct {
		Machines []string
	}
}

// EtcdStateDriver implements the StateDriver interface for an etcd based distributed
// key-value store used to store config and runtime state for the netplugin.
type EtcdStateDriver struct {
	Client *client.Client
}

// Init the driver with a core.Config.
func (d *EtcdStateDriver) Init(instInfo *core.InstanceInfo) error {
	var err error

	if instInfo == nil || !strings.Contains(instInfo.DbURL, "etcd://") {
		return errors.New("invalid etcd config")
	}

	etcdURL := strings.Replace(instInfo.DbURL, "etcd://", "http://", 1)
	etcdConfig := client.Config{
		Endpoints: []string{etcdURL},
	}

	d.Client, err = client.New(etcdConfig)
	if err != nil {
		log.Fatalf("Error creating etcd client. Err: %v", err)
	}

	return nil
}

// Deinit closes the etcd client connection.
func (d *EtcdStateDriver) Deinit() {
	d.Client.Close()
}

// Write state to key with value.
func (d *EtcdStateDriver) Write(key string, value []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	var err error

	for i := 0; i < maxEtcdRetries; i++ {
		_, err = d.Client.KV.Put(ctx, key, string(value[:]))
		if err != nil && err.Error() == client.ErrNoAvailableEndpoints.Error() {
			// Retry after a delay
			time.Sleep(time.Second)
			continue
		}

		// when err == nil or anything other than connection refused
		return err
	}

	return err
}

// Read state from key.
func (d *EtcdStateDriver) Read(key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	var err error
	var resp *client.GetResponse

	for i := 0; i < maxEtcdRetries; i++ {
		// etcd3 uses quorum for reads by default
		resp, err = d.Client.KV.Get(ctx, key)
		if err != nil {
			if err.Error() == client.ErrNoAvailableEndpoints.Error() {
				// Retry after a delay
				time.Sleep(time.Second)
				continue
			}

			if resp != nil && len(resp.Kvs) != 0 {
				return []byte(resp.Kvs[0].Value), nil
			}

			return []byte{}, fmt.Errorf("error reading from etcd")
		}

		if resp.Count == 0 {
			return []byte{}, core.Errorf("key not found")
		}

		// TODO: make sure there's only one key in the response?

		return resp.Kvs[0].Value, err
	}

	return []byte{}, err
}

// ReadAll state from baseKey.
func (d *EtcdStateDriver) ReadAll(baseKey string) ([][]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	var err error
	var resp *client.GetResponse

	for i := 0; i < maxEtcdRetries; i++ {
		// etcd uses quorum for reads by default
		resp, err = d.Client.KV.Get(ctx, baseKey, client.WithPrefix(), client.WithSort(client.SortByKey, client.SortAscend))
		if err != nil {

			if err.Error() == client.ErrNoAvailableEndpoints.Error() {
				// Retry after a delay
				time.Sleep(time.Second)
				continue
			}
		}

		if resp.Count == 0 {
			return [][]byte{}, core.Errorf("key not found")
		}

		values := [][]byte{}
		for _, node := range resp.Kvs {
			values = append(values, []byte(node.Value))
		}
		return values, nil
	}

	return [][]byte{}, err
}

func (d *EtcdStateDriver) channelEtcdEvents(watcher client.WatchChan, rsps chan [2][]byte) {
	for resp := range watcher {

		for _, ev := range resp.Events {
			//			fmt.Printf("%s %q : %q\n", ev.Type, ev.Kv.Key, ev.Kv.Value)

			rsp := [2][]byte{nil, nil}
			eventStr := "create"
			if string(ev.Kv.Value) != "" {
				rsp[0] = ev.Kv.Value
			}

			if ev.PrevKv != nil && string(ev.PrevKv.Value) != "" {
				rsp[1] = ev.PrevKv.Value
				if string(ev.Kv.Value) != "" {
					eventStr = "modify"
				} else {
					eventStr = "delete"
				}
			}

			log.Debugf("Received %q for key: %s", eventStr, ev.Kv.Key)
			//channel the translated response
			rsps <- rsp
		}
	}
}

// WatchAll state transitions from baseKey
func (d *EtcdStateDriver) WatchAll(baseKey string, rsps chan [2][]byte) error {
	watcher := d.Client.Watch(context.Background(), baseKey, client.WithPrefix())

	go d.channelEtcdEvents(watcher, rsps)

	return nil
}

// ClearState removes key from etcd
func (d *EtcdStateDriver) ClearState(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	_, err := d.Client.KV.Delete(ctx, key)
	return err
}

// ReadState reads key into a core.State with the unmarshaling function.
func (d *EtcdStateDriver) ReadState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) error {
	encodedState, err := d.Read(key)
	if err != nil {
		return err
	}

	return unmarshal(encodedState, value)
}

// readAllStateCommon reads and unmarshals (given a function) all state into a
// list of core.State objects.
// XXX: move this to some common file
func readAllStateCommon(d core.StateDriver, baseKey string, sType core.State,
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

// ReadAllState Reads all the state from baseKey and returns a list of core.State.
func (d *EtcdStateDriver) ReadAllState(baseKey string, sType core.State,
	unmarshal func([]byte, interface{}) error) ([]core.State, error) {
	return readAllStateCommon(d, baseKey, sType, unmarshal)
}

// channelStateEvents watches for updates(created, modify, delete) to a state of
// specified type and unmarshals (given a function) all changes and puts then on
// channel of core.WatchState objects.
// XXX: move this to some common file
func channelStateEvents(d core.StateDriver, sType core.State,
	unmarshal func([]byte, interface{}) error,
	byteRsps chan [2][]byte, rsps chan core.WatchState, retErr chan error) {
	for {
		// block on change notifications
		byteRsp := <-byteRsps

		rsp := core.WatchState{Curr: nil, Prev: nil}
		for i := 0; i < 2; i++ {
			if byteRsp[i] == nil {
				continue
			}
			stateType := reflect.TypeOf(sType)
			value := reflect.New(stateType)
			err := unmarshal(byteRsp[i], value.Interface())
			if err != nil {
				log.Errorf("unmarshal error: %v", err)
				retErr <- err
				return
			}
			if !value.Elem().Elem().FieldByName("CommonState").IsValid() {
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
}

// WatchAllState watches all state from the baseKey.
func (d *EtcdStateDriver) WatchAllState(baseKey string, sType core.State,
	unmarshal func([]byte, interface{}) error, rsps chan core.WatchState) error {
	byteRsps := make(chan [2][]byte, 1)
	recvErr := make(chan error, 1)

	err := d.WatchAll(baseKey, byteRsps)
	if err != nil {
		log.Errorf("WatchAll returned %v", err)
		return err
	}

	for {
		go channelStateEvents(d, sType, unmarshal, byteRsps, rsps, recvErr)

		err = <-recvErr
		log.Errorf("Err from channelStateEvents %v", err)
		time.Sleep(time.Second)
	}
}

// WriteState writes a value of core.State into a key with a given marshaling function.
func (d *EtcdStateDriver) WriteState(key string, value core.State,
	marshal func(interface{}) ([]byte, error)) error {
	encodedState, err := marshal(value)
	if err != nil {
		return err
	}

	return d.Write(key, encodedState)
}
