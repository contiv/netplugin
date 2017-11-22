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
	"strings"
	"time"

	"github.com/contiv/netplugin/core"
	"github.com/hashicorp/consul/api"

	log "github.com/Sirupsen/logrus"
)

// Max times to retry in case of failure
const maxConsulRetries = 10

// ConsulStateDriverConfig encapsulates the configuration parameters to
// initialize consul client
type ConsulStateDriverConfig struct {
	Consul api.Config
}

// ConsulStateDriver implements the StateDriver interface for a consul based distributed
// key-value store used to store config and runtime state for the netplugin.
type ConsulStateDriver struct {
	Client *api.Client
}

// Init the driver with a core.Config.
func (d *ConsulStateDriver) Init(instInfo *core.InstanceInfo) error {
	var err error
	var endpoint *url.URL

	if instInfo == nil || instInfo.DbURL == "" {
		return errors.New("no consul config found")
	}
	endpoint, err = url.Parse(instInfo.DbURL)
	if err != nil {
		return err
	}
	if endpoint.Scheme == "consul" {
		endpoint.Scheme = "http"
	} else if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		return fmt.Errorf("invalid consul URL scheme %q", endpoint.Scheme)
	}
	cfg := api.Config{
		Address: endpoint.Host,
	}

	d.Client, err = api.NewClient(&cfg)

	return err
}

// Deinit is currently a no-op.
func (d *ConsulStateDriver) Deinit() {
}

func processKey(inKey string) string {
	//consul doesn't accepts keys starting with a '/', so trim the leading slash
	return strings.TrimPrefix(inKey, "/")
}

// Write state to key with value.
func (d *ConsulStateDriver) Write(key string, value []byte) error {
	key = processKey(key)

	var err error

	for i := 0; i < maxConsulRetries; i++ {
		_, err = d.Client.KV().Put(&api.KVPair{Key: key, Value: value}, nil)
		if err != nil && (api.IsServerError(err) || strings.Contains(err.Error(), "EOF") || strings.Contains(err.Error(), "connection refused")) {
			// Retry after a delay
			time.Sleep(time.Second)
			continue
		}

		// when err == nil or anything other than connection refused
		return err
	}

	// err could be anything other than connection refused
	return err
}

// Read state from key.
func (d *ConsulStateDriver) Read(key string) ([]byte, error) {
	key = processKey(key)

	var err error
	var kv *api.KVPair

	for i := 0; i < maxConsulRetries; i++ {
		kv, _, err = d.Client.KV().Get(key, nil)
		if err != nil {
			if api.IsServerError(err) || strings.Contains(err.Error(), "EOF") || strings.Contains(err.Error(), "connection refused") {
				time.Sleep(time.Second)
				continue
			}

			return []byte{}, err
		}

		// err == nil
		if kv == nil {
			return []byte{}, core.Errorf("key not found")
		}

		return kv.Value, err
	}

	return []byte{}, err
}

// ReadAll state from baseKey.
func (d *ConsulStateDriver) ReadAll(baseKey string) ([][]byte, error) {
	baseKey = processKey(baseKey)

	var err error
	var kvs api.KVPairs

	for i := 0; i < maxConsulRetries; i++ {
		kvs, _, err = d.Client.KV().List(baseKey, nil)
		if err != nil {
			if api.IsServerError(err) || strings.Contains(err.Error(), "EOF") || strings.Contains(err.Error(), "connection refused") {
				time.Sleep(time.Second)
				continue
			}

			return [][]byte{}, err
		}

		// err == nil
		if kvs == nil {
			// Consul returns success and a nil kv when a key is not found,
			// translate it to 'key not found' error
			return nil, core.Errorf("key not found")
		}

		values := [][]byte{}
		for _, kv := range kvs {
			values = append(values, kv.Value)
		}
		return values, nil

	}

	return [][]byte{}, err
}

func (d *ConsulStateDriver) channelConsulEvents(baseKey string, kvCache map[string]*api.KVPair,
	consulRsps chan api.KVPairs, rsps chan [2][]byte, retErr chan error, stop chan bool) {
	for {
		select {
		// block on change notifications
		case kvs := <-consulRsps:
			kvsRcvd := map[string]*api.KVPair{}
			// Generate Create/Modifiy events for the keys recvd
			for _, kv := range kvs {
				// XXX: The logic below assumes that the node returned is always a node
				// of interest. Eg: If we set a watch on /a/b/c, then we are mostly
				// interested in changes in that directory i.e. changes to /a/b/c/d1..d2
				// This works for now as the constructs like network and endpoints that
				// need to be watched are organized as above. Need to revisit when
				// this assumption changes.
				kvsRcvd[kv.Key] = kv
				rsp := [2][]byte{nil, nil}
				rsp[0] = kv.Value
				if kvSeen, ok := kvCache[kv.Key]; !ok {
					log.Debugf("Received create for key: %q, kv: %+v", kv.Key, kv)
				} else if kvSeen.ModifyIndex != kv.ModifyIndex {
					log.Debugf("Received modify for key: %q, kv: %+v", kv.Key, kv)
					rsp[1] = kvSeen.Value
				} else {
					// no changes to the key, skipping
					log.Debugf("Skipping key with no changes: %s", kv.Key)
					continue
				}
				//update the map of seen keys
				kvCache[kv.Key] = kv

				//channel the translated response
				rsps <- rsp
			}

			// Generate Delete events for missing keys
			for key, kv := range kvCache {
				if _, ok := kvsRcvd[key]; !ok {
					log.Infof("Received delete for key: %q, Pair: %+v", kv.Key, kv)
					rsps <- [2][]byte{nil, kv.Value}
					// remove this key from the map of seen keys
					delete(kvCache, key)
				}
			}

		case <-stop:
			log.Infof("Stop request received")
			return
		}
	}
}

// WatchAll state transitions from baseKey
func (d *ConsulStateDriver) WatchAll(baseKey string, rsps chan [2][]byte) error {
	baseKey = processKey(baseKey)
	consulRsps := make(chan api.KVPairs, 1)
	stop := make(chan bool, 1)
	recvErr := make(chan error, 2)

	// Consul returns all the keys as return value of List(). The following maps helps
	// track the state that has been seen and used to appropriately generate
	// create, modify and delete events
	kvCache := map[string]*api.KVPair{}
	// read with index=0 to fetch all existing keys
	var waitIndex uint64
	kvs, qm, err := d.Client.KV().List(baseKey, &api.QueryOptions{WaitIndex: waitIndex})
	if err != nil {
		log.Errorf("consul read failed for key %q. Error: %s", baseKey, err)
		return err
	}
	// Consul returns success and a nil kv when a key is not found.
	// Treat this as starting with no state.
	// XXX: shall we fail the watch in this case?
	if kvs == nil {
		kvs = api.KVPairs{}
	}
	for _, kv := range kvs {
		kvCache[kv.Key] = kv
	}
	waitIndex = qm.LastIndex

	go d.channelConsulEvents(baseKey, kvCache, consulRsps, rsps, recvErr, stop)

	for {
		select {
		case err := <-recvErr:
			return err
		default:
			kvs, qm, err := d.Client.KV().List(baseKey, &api.QueryOptions{WaitIndex: waitIndex})
			if err != nil {
				if api.IsServerError(err) || strings.Contains(err.Error(), "EOF") || strings.Contains(err.Error(), "connection refused") {
					log.Warnf("Consul watch: server error: %v for %s. Retrying..", err, baseKey)
					time.Sleep(5 * time.Second)
					continue
				} else {
					log.Errorf("consul watch failed for key %q. Error: %s. stopping watch..", baseKey, err)
					stop <- true
					return err
				}
			}
			// Consul returns success and a nil kv when a key is not found.
			// This shall translate into appropriate 'Delete' events or
			// no events (depending on whether some keys were seen before)
			// XXX: shall we stop the watch in this case?
			if kvs == nil {
				kvs = api.KVPairs{}
			}

			waitIndex = qm.LastIndex
			consulRsps <- kvs
		}
	}
}

// ClearState removes key from etcd.
func (d *ConsulStateDriver) ClearState(key string) error {
	key = processKey(key)
	_, err := d.Client.KV().Delete(key, nil)
	return err
}

// ReadState reads key into a core.State with the unmarshaling function.
func (d *ConsulStateDriver) ReadState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) error {
	key = processKey(key)
	encodedState, err := d.Read(key)
	if err != nil {
		return err
	}

	return unmarshal(encodedState, value)
}

// ReadAllState Reads all the state from baseKey and returns a list of core.State.
func (d *ConsulStateDriver) ReadAllState(baseKey string, sType core.State,
	unmarshal func([]byte, interface{}) error) ([]core.State, error) {
	baseKey = processKey(baseKey)
	return readAllStateCommon(d, baseKey, sType, unmarshal)
}

// WatchAllState watches all state from the baseKey.
func (d *ConsulStateDriver) WatchAllState(baseKey string, sType core.State,
	unmarshal func([]byte, interface{}) error, rsps chan core.WatchState) error {
	baseKey = processKey(baseKey)
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

// WriteState writes a value of core.State into a key with a given marshaling function.
func (d *ConsulStateDriver) WriteState(key string, value core.State,
	marshal func(interface{}) ([]byte, error)) error {
	key = processKey(key)
	encodedState, err := marshal(value)
	if err != nil {
		return err
	}

	return d.Write(key, encodedState)
}
