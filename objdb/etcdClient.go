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

package objdb

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
	client "github.com/coreos/etcd/clientv3"
)

type etcdPlugin struct {
	mutex *sync.Mutex
}

// EtcdClient has etcd client state
type EtcdClient struct {
	client *client.Client // etcd client

	serviceDb map[string]*etcdServiceState
}

// Max retry count
const maxEtcdRetries = 10

// Register the plugin
func init() {
	RegisterPlugin("etcd", &etcdPlugin{mutex: new(sync.Mutex)})
}

// Initialize the etcd client
func (ep *etcdPlugin) NewClient(endpoints []string) (API, error) {
	var err error
	var ec = new(EtcdClient)

	ep.mutex.Lock()
	defer ep.mutex.Unlock()

	// Setup default url
	if len(endpoints) == 0 {
		endpoints = []string{"http://127.0.0.1:2379"}
	}

	etcdConfig := client.Config{
		Endpoints: endpoints,
	}

	// Create a new client
	ec.client, err = client.New(etcdConfig)
	if err != nil {
		log.Fatalf("Error creating etcd client. Err: %v", err)
		return nil, err
	}

	// Initialize service DB
	ec.serviceDb = make(map[string]*etcdServiceState)

	// Make sure we can read from etcd
	_, err = ec.client.KV.Get(context.Background(), "test")
	if err != nil {
		log.Errorf("Failed to connect to etcd. Err: %v", err)
		return nil, err
	}

	return ec, nil
}

// GetObj Get an object
func (ep *EtcdClient) GetObj(key string, retVal interface{}) error {
	log.Infof(`objdb: getting "%s"`, key)
	keyName := "/contiv.io/obj/" + key

	// Get the object from etcd client
	// etcd uses quorum for reads by default
	resp, err := ep.client.KV.Get(context.Background(), keyName)
	if err != nil {
		// Retry few times if cluster is unavailable
		if err.Error() == client.ErrNoAvailableEndpoints.Error() {
			for i := 0; i < maxEtcdRetries; i++ {
				resp, err = ep.client.KV.Get(context.Background(), keyName)
				if err == nil {
					break
				}

				// Retry after a delay
				time.Sleep(time.Second)
			}
		}
		if err != nil {
			log.Errorf("Error getting key %s. Err: %v", keyName, err)
			return err
		}
	}

	if resp.Count == 0 {
		msg := "Key " + keyName + " not found"
		log.Errorf(msg)
		return errors.New(msg)
	}

	//	log.Infof("Got object response:\n\n%s\n\n", string(resp.Kvs[0].Value))

	// Parse JSON response
	if err := json.Unmarshal(resp.Kvs[0].Value, retVal); err != nil {
		log.Errorf("Error parsing object %s, Err %v", resp.Kvs[0].Value, err)
		return err
	}

	return nil
}

// ListDir Get a list of objects in a directory
func (ep *EtcdClient) ListDir(key string) ([]string, error) {
	keyName := "/contiv.io/obj/" + key

	// Get the object from etcd client
	resp, err := ep.client.KV.Get(context.Background(), keyName, client.WithPrefix(), client.WithSort(client.SortByKey, client.SortAscend))
	if err != nil {
		// Retry few times if cluster is unavailable
		if err.Error() == client.ErrNoAvailableEndpoints.Error() {
			for i := 0; i < maxEtcdRetries; i++ {
				resp, err = ep.client.KV.Get(context.Background(), keyName, client.WithPrefix(), client.WithSort(client.SortByKey, client.SortAscend))
				if err == nil {
					break
				}

				// Retry after a delay
				time.Sleep(time.Second)
			}
		}
		if err != nil {
			return nil, err
		}
	}

	var retList []string

	// convert all the keys into strings (etcd3 doesn't have directories)
	for _, kv := range resp.Kvs {
		retList = append(retList, string(kv.Key))
	}

	return retList, nil
}

// SetObj Save an object, create if it doesnt exist
func (ep *EtcdClient) SetObj(key string, value interface{}) error {
	log.Infof(`objdb: setting "%s"`, key)
	keyName := "/contiv.io/obj/" + key

	// JSON format the object
	jsonVal, err := json.Marshal(value)
	if err != nil {
		log.Errorf("Json conversion error. Err %v", err)
		return err
	}

	// Set it via etcd client
	_, err = ep.client.KV.Put(context.Background(), keyName, string(jsonVal[:]))
	if err != nil {
		// Retry few times if cluster is unavailable
		if err.Error() == client.ErrNoAvailableEndpoints.Error() {
			for i := 0; i < maxEtcdRetries; i++ {
				_, err = ep.client.KV.Put(context.Background(), keyName, string(jsonVal[:]))
				if err == nil {
					break
				}

				// Retry after a delay
				time.Sleep(time.Second)
			}
		}
		if err != nil {
			log.Errorf("Error setting key %s, Err: %v", keyName, err)
			return err
		}
	}

	return nil
}

// DelObj Remove an object
func (ep *EtcdClient) DelObj(key string) error {
	log.Infof(`objdb: deleting "%s"`, key)
	keyName := "/contiv.io/obj/" + key

	// Remove it via etcd client
	_, err := ep.client.KV.Delete(context.Background(), keyName)
	if err != nil {
		// Retry few times if cluster is unavailable
		if err.Error() == client.ErrNoAvailableEndpoints.Error() {
			for i := 0; i < maxEtcdRetries; i++ {
				_, err = ep.client.KV.Delete(context.Background(), keyName)
				if err == nil {
					break
				}

				// Retry after a delay
				time.Sleep(time.Second)
			}
		}
		if err != nil {
			log.Errorf("Error removing key %s, Err: %v", keyName, err)
			return err
		}
	}

	return nil
}
