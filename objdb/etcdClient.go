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
	"github.com/coreos/etcd/client"
	"net"
	"net/http"
)

type etcdPlugin struct {
	mutex *sync.Mutex
}

// EtcdClient has etcd client state
type EtcdClient struct {
	client client.Client // etcd client
	kapi   client.KeysAPI

	serviceDb map[string]*etcdServiceState
}

// Max retry count
const maxEtcdRetries = 10

// Register the plugin
func init() {
	RegisterPlugin("etcd", &etcdPlugin{mutex: new(sync.Mutex)})
}

// Initialize the etcd client
func (ep *etcdPlugin) NewClient(endpoints []string, config *Config) (API, error) {
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
		Transport: client.DefaultTransport,
	}

	if config != nil && config.TLS != nil {
		// Set transport
		etcdConfig.Transport = &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 10 * time.Second,
			TLSClientConfig:     config.TLS,
		}
	}

	// Create a new client
	ec.client, err = client.New(etcdConfig)
	if err != nil {
		log.Fatalf("Error creating etcd client. Err: %v", err)
		return nil, err
	}

	// create keys api
	ec.kapi = client.NewKeysAPI(ec.client)

	// Initialize service DB
	ec.serviceDb = make(map[string]*etcdServiceState)

	// Make sure we can read from etcd
	_, err = ec.kapi.Get(context.Background(), "/", &client.GetOptions{Recursive: true, Sort: true})
	if err != nil {
		log.Errorf("Failed to connect to etcd. Err: %v", err)
		return nil, err
	}

	return ec, nil
}

// GetObj Get an object
func (ep *EtcdClient) GetObj(key string, retVal interface{}) error {
	keyName := "/contiv.io/obj/" + key

	// Get the object from etcd client
	resp, err := ep.kapi.Get(context.Background(), keyName, &client.GetOptions{Quorum: true})
	if err != nil {
		// Retry few times if cluster is unavailable
		if err.Error() == client.ErrClusterUnavailable.Error() {
			for i := 0; i < maxEtcdRetries; i++ {
				resp, err = ep.kapi.Get(context.Background(), keyName, &client.GetOptions{Quorum: true})
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

	// Parse JSON response
	if err := json.Unmarshal([]byte(resp.Node.Value), retVal); err != nil {
		log.Errorf("Error parsing object %s, Err %v", resp.Node.Value, err)
		return err
	}

	return nil
}

// Recursive function to look thru each directory and get the files
func recursAddNode(node *client.Node, list []string) []string {
	for _, innerNode := range node.Nodes {
		// add only the files.
		if !innerNode.Dir {
			list = append(list, innerNode.Value)
		} else {
			list = recursAddNode(innerNode, list)
		}
	}

	return list
}

// ListDir Get a list of objects in a directory
func (ep *EtcdClient) ListDir(key string) ([]string, error) {
	keyName := "/contiv.io/obj/" + key

	getOpts := client.GetOptions{
		Recursive: true,
		Sort:      true,
		Quorum:    true,
	}

	// Get the object from etcd client
	resp, err := ep.kapi.Get(context.Background(), keyName, &getOpts)
	if err != nil {
		// Retry few times if cluster is unavailable
		if err.Error() == client.ErrClusterUnavailable.Error() {
			for i := 0; i < maxEtcdRetries; i++ {
				resp, err = ep.kapi.Get(context.Background(), keyName, &getOpts)
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

	if !resp.Node.Dir {
		log.Errorf("ListDir response is not a directory")
		return nil, errors.New("Response is not directory")
	}

	var retList []string
	// Call a recursive function to recurse thru each directory and get all files
	// Warning: assumes directory itep is not interesting to the caller
	// Warning2: there is also an assumption that keynames are not required
	//           Which means, caller has to derive the key from value :(
	retList = recursAddNode(resp.Node, retList)

	return retList, nil
}

// SetObj Save an object, create if it doesnt exist
func (ep *EtcdClient) SetObj(key string, value interface{}) error {
	keyName := "/contiv.io/obj/" + key

	// JSON format the object
	jsonVal, err := json.Marshal(value)
	if err != nil {
		log.Errorf("Json conversion error. Err %v", err)
		return err
	}

	// Set it via etcd client
	_, err = ep.kapi.Set(context.Background(), keyName, string(jsonVal[:]), nil)
	if err != nil {
		// Retry few times if cluster is unavailable
		if err.Error() == client.ErrClusterUnavailable.Error() {
			for i := 0; i < maxEtcdRetries; i++ {
				_, err = ep.kapi.Set(context.Background(), keyName, string(jsonVal[:]), nil)
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
	keyName := "/contiv.io/obj/" + key

	// Remove it via etcd client
	_, err := ep.kapi.Delete(context.Background(), keyName, nil)
	if err != nil {
		// Retry few times if cluster is unavailable
		if err.Error() == client.ErrClusterUnavailable.Error() {
			for i := 0; i < maxEtcdRetries; i++ {
				_, err = ep.kapi.Delete(context.Background(), keyName, nil)
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
