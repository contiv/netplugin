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
	"net/url"
	"reflect"
	"time"

	"golang.org/x/net/context"

	"github.com/contiv/netplugin/core"
	"github.com/coreos/etcd/client"

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
	Client  client.Client
	KeysAPI client.KeysAPI
}

// Init the driver with a core.Config.
func (d *EtcdStateDriver) Init(instInfo *core.InstanceInfo) error {
	var err error
	var endpoint *url.URL

	if instInfo == nil || instInfo.DbURL == "" {
		return errors.New("no etcd config found")
	}
	endpoint, err = url.Parse(instInfo.DbURL)
	if err != nil {
		return err
	}
	if endpoint.Scheme == "etcd" {
		endpoint.Scheme = "http"
	} else if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		return core.Errorf("invalid etcd URL scheme %q", endpoint.Scheme)
	}
	// TODO: support multi-endpoints
	etcdConfig := client.Config{
		Endpoints: []string{endpoint.String()},
	}

	d.Client, err = client.New(etcdConfig)
	if err != nil {
		log.Fatalf("error creating etcd client. Err: %v", err)
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

	var err error

	for i := 0; i < maxEtcdRetries; i++ {
		_, err = d.KeysAPI.Set(ctx, key, string(value[:]), nil)
		if err != nil && err.Error() == client.ErrClusterUnavailable.Error() {
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
	var resp *client.Response

	for i := 0; i < maxEtcdRetries; i++ {
		resp, err = d.KeysAPI.Get(ctx, key, &client.GetOptions{Quorum: true})
		if err == nil {
			if resp != nil && resp.Node != nil {
				return []byte(resp.Node.Value), nil
			}

			return []byte{}, fmt.Errorf("error reading from etcd")
		}

		if client.IsKeyNotFound(err) {
			return []byte{}, core.Errorf("key not found")
		}

		if err.Error() == client.ErrClusterUnavailable.Error() {
			// Retry after a delay
			time.Sleep(time.Second)
			continue
		}

		return []byte{}, err
	}

	return []byte{}, err
}

// ReadAll state from baseKey.
func (d *EtcdStateDriver) ReadAll(baseKey string) ([][]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	defer cancel()

	var err error
	var resp *client.Response

	for i := 0; i < maxEtcdRetries; i++ {
		resp, err = d.KeysAPI.Get(ctx, baseKey, &client.GetOptions{Recursive: true, Quorum: true})
		if err == nil {
			values := [][]byte{}
			for _, node := range resp.Node.Nodes {
				values = append(values, []byte(node.Value))
			}
			return values, nil
		}

		if client.IsKeyNotFound(err) {
			return [][]byte{}, core.Errorf("key not found")
		}

		if err.Error() == client.ErrClusterUnavailable.Error() {
			// Retry after a delay
			time.Sleep(time.Second)
			continue
		}

		return [][]byte{}, err
	}

	return [][]byte{}, err
}

func (d *EtcdStateDriver) channelEtcdEvents(watcher client.Watcher, rsps chan [2][]byte) {
	for {
		// block on change notifications
		etcdRsp, err := watcher.Next(context.Background())
		if err != nil {
			log.Errorf("Error %v during watch", err)
			time.Sleep(time.Second)
			continue
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

		log.Debugf("Received %q for key: %s", eventStr, etcdRsp.Node.Key)
		//channel the translated response
		rsps <- rsp
	}
}

// WatchAll state transitions from baseKey
func (d *EtcdStateDriver) WatchAll(baseKey string, rsps chan [2][]byte) error {
	watcher := d.KeysAPI.Watcher(baseKey, &client.WatcherOptions{Recursive: true})
	if watcher == nil {
		log.Errorf("etcd watch failed.")
		return errors.New("etcd watch failed")
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
