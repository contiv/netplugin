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
	"reflect"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/contiv/netplugin/core"
	"github.com/coreos/etcd/client"

	log "github.com/Sirupsen/logrus"
)

const (
	recursive  = true
	ctxTimeout = 20 * time.Second
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
	Client  client.Client
	KeysAPI client.KeysAPI
}

// Init the driver with a core.Config.
func (d *EtcdStateDriver) Init(instInfo *core.InstanceInfo) error {
	var err error

	if instInfo == nil || !strings.Contains(instInfo.DbURL, "etcd://") {
		return errors.New("Invalid etcd config")
	}

	etcdURL := strings.Replace(instInfo.DbURL, "etcd://", "http://", 1)
	etcdConfig := client.Config{
		Endpoints: []string{etcdURL},
	}

	d.Client, err = client.New(etcdConfig)
	if err != nil {
		log.Fatalf("Error creating etcd client. Err: %v", err)
	}

	// Create keys api
	d.KeysAPI = client.NewKeysAPI(d.Client)

	return nil
}

// Deinit is currently a no-op.
func (d *EtcdStateDriver) Deinit() {}

// Write state to key with value.
func (d *EtcdStateDriver) Write(key string, value []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	_, err := d.KeysAPI.Set(ctx, key, string(value[:]), nil)

	return err
}

// Read state from key.
func (d *EtcdStateDriver) Read(key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	resp, err := d.KeysAPI.Get(ctx, key, &client.GetOptions{Quorum: true})
	if err != nil {
		return []byte{}, err
	}

	return []byte(resp.Node.Value), err
}

// ReadAll state from baseKey.
func (d *EtcdStateDriver) ReadAll(baseKey string) ([][]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	resp, err := d.KeysAPI.Get(ctx, baseKey, &client.GetOptions{Recursive: true, Quorum: true})
	if err != nil {
		return nil, err
	}

	values := [][]byte{}
	for _, node := range resp.Node.Nodes {
		values = append(values, []byte(node.Value))
	}

	return values, nil
}

func (d *EtcdStateDriver) channelEtcdEvents(watcher client.Watcher, rsps chan [2][]byte) {
	for {
		// block on change notifications
		etcdRsp, err := watcher.Next(context.Background())
		if err != nil {
			log.Errorf("Error %v during watch", err)
		}

		// XXX: The logic below assumes that the node returned is always a node
		// of interest. Eg: If we set a watch on /a/b/c, then we are mostly
		// interested in changes in that directory i.e. changes to /a/b/c/d1..d2
		// This works for now as the constructs like network and endpoints that
		// need to be watched are organized as above. Need to revisit when
		// this assumption changes.
		rsp := [2][]byte{nil, nil}
		eventStr := "create"
		if etcdRsp.Node.Value != "" {
			rsp[0] = []byte(etcdRsp.Node.Value)
		}
		if etcdRsp.PrevNode != nil && etcdRsp.PrevNode.Value != "" {
			rsp[1] = []byte(etcdRsp.PrevNode.Value)
			if etcdRsp.Node.Value != "" {
				eventStr = "modify"
			} else {
				eventStr = "delete"
			}
		}

		log.Infof("Received %q for key: %s", eventStr, etcdRsp.Node.Key)
		//channel the translated response
		rsps <- rsp
	}
}

// WatchAll state transitions from baseKey
func (d *EtcdStateDriver) WatchAll(baseKey string, rsps chan [2][]byte) error {
	watcher := d.KeysAPI.Watcher(baseKey, &client.WatcherOptions{Recursive: recursive})
	if watcher == nil {
		log.Errorf("etcd watch failed.")
		return errors.New("Etcd watch failed")
	}

	go d.channelEtcdEvents(watcher, rsps)

	return nil
}

// ClearState removes key from etcd
func (d *EtcdStateDriver) ClearState(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	_, err := d.KeysAPI.Delete(ctx, key, nil)
	return err
}

// ReadState reads key into a core.State with the unmarshalling function.
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

	go channelStateEvents(d, sType, unmarshal, byteRsps, rsps, recvErr)

	err := d.WatchAll(baseKey, byteRsps)
	if err != nil {
		return err
	}

	err = <-recvErr
	return err

}

// WriteState writes a value of core.State into a key with a given marshalling function.
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
