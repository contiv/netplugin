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
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"

	log "github.com/Sirupsen/logrus"
)

// consulPlugin contains consul plugin specific state
type consulPlugin struct {
	mutex *sync.Mutex
}

// ConsulClient has consul client state
type ConsulClient struct {
	client       *api.Client // consul client
	consulConfig api.Config

	serviceDb map[string]*consulServiceState
}

// Max times to retry
const maxConsulRetries = 10

// init Register the plugin
func init() {
	RegisterPlugin("consul", &consulPlugin{mutex: new(sync.Mutex)})
}

// Init initializes the consul client
func (cp *consulPlugin) NewClient(endpoints []string, config *Config) (API, error) {
	cc := new(ConsulClient)

	if len(endpoints) == 0 {
		endpoints = []string{"127.0.0.1:8500"}
	}

	// default consul config
	cc.consulConfig = api.Config{Address: strings.TrimPrefix(endpoints[0], "http://")}

	// Initialize service DB
	cc.serviceDb = make(map[string]*consulServiceState)

	// Init consul client
	client, err := api.NewClient(&cc.consulConfig)
	if err != nil {
		log.Fatalf("Error initializing consul client")
		return nil, err
	}

	cc.client = client

	// verify we can reach the consul
	_, _, err = client.KV().List("/", nil)
	if err != nil {
		if api.IsServerError(err) || strings.Contains(err.Error(), "EOF") ||
			strings.Contains(err.Error(), "connection refused") {
			for i := 0; i < maxConsulRetries; i++ {
				_, _, err = client.KV().List("/", nil)
				if err == nil {
					break
				}

				// Retry after a delay
				time.Sleep(time.Second)
			}
		}

		// return error if it failed after retries
		if err != nil {
			log.Errorf("Error connecting to consul. Err: %v", err)
			return nil, err
		}
	}

	return cc, nil
}

func processKey(inKey string) string {
	//consul doesn't accepts keys starting with a '/', so trim the leading slash
	return strings.TrimPrefix(inKey, "/")
}

// GetObj reads the object
func (cp *ConsulClient) GetObj(key string, retVal interface{}) error {
	key = processKey("/contiv.io/obj/" + processKey(key))

	resp, _, err := cp.client.KV().Get(key, &api.QueryOptions{RequireConsistent: true})
	if err != nil {
		if api.IsServerError(err) || strings.Contains(err.Error(), "EOF") ||
			strings.Contains(err.Error(), "connection refused") {
			for i := 0; i < maxConsulRetries; i++ {
				resp, _, err = cp.client.KV().Get(key, &api.QueryOptions{RequireConsistent: true})
				if err == nil {
					break
				}

				// Retry after a delay
				time.Sleep(time.Second)
			}
		}

		// return error if it failed after retries
		if err != nil {
			return err
		}
	}
	// Consul returns success and a nil kv when a key is not found,
	// translate it to 'Key not found' error
	if resp == nil {
		return errors.New("Key not found")
	}

	// Parse JSON response
	if err := json.Unmarshal(resp.Value, retVal); err != nil {
		log.Errorf("Error parsing object %v, Err %v", resp.Value, err)
		return err
	}

	return nil
}

// ListDir returns a list of keys in a directory
func (cp *ConsulClient) ListDir(key string) ([]string, error) {
	key = processKey("/contiv.io/obj/" + processKey(key))

	kvs, _, err := cp.client.KV().List(key, nil)
	if err != nil {
		if api.IsServerError(err) || strings.Contains(err.Error(), "EOF") ||
			strings.Contains(err.Error(), "connection refused") {
			for i := 0; i < maxConsulRetries; i++ {
				kvs, _, err = cp.client.KV().List(key, nil)
				if err == nil {
					break
				}

				// Retry after a delay
				time.Sleep(time.Second)
			}
		}

		// return error if it failed after retries
		if err != nil {
			return nil, err
		}
	}
	// Consul returns success and a nil kv when a key is not found,
	// translate it to 'Key not found' error
	if kvs == nil {
		return []string{}, nil
	}

	var keys []string
	for _, kv := range kvs {
		keys = append(keys, string(kv.Value))
	}

	return keys, nil
}

// SetObj writes an object
func (cp *ConsulClient) SetObj(key string, value interface{}) error {
	key = processKey("/contiv.io/obj/" + processKey(key))

	// JSON format the object
	jsonVal, err := json.Marshal(value)
	if err != nil {
		log.Errorf("Json conversion error. Err %v", err)
		return err
	}

	_, err = cp.client.KV().Put(&api.KVPair{Key: key, Value: jsonVal}, nil)
	if err != nil {
		if api.IsServerError(err) || strings.Contains(err.Error(), "EOF") ||
			strings.Contains(err.Error(), "connection refused") {
			for i := 0; i < maxConsulRetries; i++ {
				_, err = cp.client.KV().Put(&api.KVPair{Key: key, Value: jsonVal}, nil)
				if err == nil {
					break
				}

				// Retry after a delay
				time.Sleep(time.Second)
			}
		}
	}

	return err
}

// DelObj deletes an object
func (cp *ConsulClient) DelObj(key string) error {
	key = processKey("/contiv.io/obj/" + processKey(key))
	_, err := cp.client.KV().Delete(key, nil)
	if err != nil {
		if api.IsServerError(err) || strings.Contains(err.Error(), "EOF") ||
			strings.Contains(err.Error(), "connection refused") {
			for i := 0; i < maxConsulRetries; i++ {
				_, err = cp.client.KV().Delete(key, nil)
				if err == nil {
					break
				}

				// Retry after a delay
				time.Sleep(time.Second)
			}
		}
	}

	return err
}
