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
	"strings"

	"github.com/contiv/netplugin/core"
	"github.com/hashicorp/consul/api"

	log "github.com/Sirupsen/logrus"
)

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
func (d *ConsulStateDriver) Init(config *core.Config) error {
	var err error

	if config == nil {
		return core.Errorf("Invalid arguments. cfg: %v", config)
	}

	cfg, ok := config.V.(*ConsulStateDriverConfig)

	if !ok {
		return core.Errorf("Invalid config type passed!")
	}

	d.Client, err = api.NewClient(&cfg.Consul)
	if err != nil {
		return err
	}

	return nil
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
	_, err := d.Client.KV().Put(&api.KVPair{Key: key, Value: value}, nil)

	return err
}

// Read state from key.
func (d *ConsulStateDriver) Read(key string) ([]byte, error) {
	key = processKey(key)
	kv, _, err := d.Client.KV().Get(key, nil)
	if err != nil {
		return []byte{}, err
	}
	// Consul returns success and a nil kv when a key is not found,
	// translate it to 'Key not found' error
	if kv == nil {
		return []byte{}, core.Errorf("Key not found")
	}

	return kv.Value, err
}

// ReadAll state from baseKey.
func (d *ConsulStateDriver) ReadAll(baseKey string) ([][]byte, error) {
	baseKey = processKey(baseKey)
	kvs, _, err := d.Client.KV().List(baseKey, nil)
	if err != nil {
		return nil, err
	}
	// Consul returns success and a nil kv when a key is not found,
	// translate it to 'Key not found' error
	if kvs == nil {
		return nil, core.Errorf("Key not found")
	}

	values := [][]byte{}
	for _, kv := range kvs {
		values = append(values, kv.Value)
	}

	return values, nil
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
					log.Infof("Received create for key: %q, kv: %+v", kv.Key, kv)
				} else if kvSeen.ModifyIndex != kv.ModifyIndex {
					log.Infof("Received modify for key: %q, kv: %+v", kv.Key, kv)
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
				log.Errorf("consul watch failed for key %q. Error: %s", baseKey, err)
				stop <- true
				return err
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

// ReadState reads key into a core.State with the unmarshalling function.
func (d *ConsulStateDriver) ReadState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) error {
	key = processKey(key)
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

// WriteState writes a value of core.State into a key with a given marshalling function.
func (d *ConsulStateDriver) WriteState(key string, value core.State,
	marshal func(interface{}) ([]byte, error)) error {
	key = processKey(key)
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
