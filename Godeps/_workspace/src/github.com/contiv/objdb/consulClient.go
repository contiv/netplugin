package objdb

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"github.com/hashicorp/consul/api"

	log "github.com/Sirupsen/logrus"
)

// consulPlugin contains consul plugin specific state
type consulPlugin struct {
	client       *api.Client // consul client
	consulConfig api.Config

	mutex *sync.Mutex
}

// init Register the plugin
func init() {
	RegisterPlugin("consul", &consulPlugin{mutex: new(sync.Mutex)})
}

// Init initializes the consul client
func (cp *consulPlugin) Init(machines []string) error {
	// default consul config
	cp.consulConfig = api.Config{Address: "127.0.0.1:8500"}

	// Init consul client
	client, err := api.NewClient(&cp.consulConfig)
	if err != nil {
		log.Fatalf("Error initializing consul client")
		return err
	}

	cp.client = client

	return nil
}

func processKey(inKey string) string {
	//consul doesn't accepts keys starting with a '/', so trim the leading slash
	return strings.TrimPrefix(inKey, "/")
}

// GetObj reads the object
func (cp *consulPlugin) GetObj(key string, retVal interface{}) error {
	key = processKey("/contiv.io/obj/" + processKey(key))

	resp, _, err := cp.client.KV().Get(key, &api.QueryOptions{RequireConsistent: true})
	if err != nil {
		return err
	}
	// Consul returns success and a nil kv when a key is not found,
	// translate it to 'Key not found' error
	if resp == nil {
		return errors.New("Key not found")
	}

	// Parse JSON response
	if err := json.Unmarshal([]byte(resp.Value), retVal); err != nil {
		log.Errorf("Error parsing object %s, Err %v", resp.Value, err)
		return err
	}

	return nil
}

// ListDir returns a list of keys in a directory
func (cp *consulPlugin) ListDir(key string) ([]string, error) {
	key = processKey("/contiv.io/obj/" + processKey(key))

	kvs, _, err := cp.client.KV().List(key, nil)
	if err != nil {
		return nil, err
	}
	// Consul returns success and a nil kv when a key is not found,
	// translate it to 'Key not found' error
	if kvs == nil {
		return nil, errors.New("Key not found")
	}

	var keys []string
	for _, kv := range kvs {
		keys = append(keys, kv.Key)
	}

	return keys, nil
}

// SetObj writes an object
func (cp *consulPlugin) SetObj(key string, value interface{}) error {
	key = processKey("/contiv.io/obj/" + processKey(key))

	// JSON format the object
	jsonVal, err := json.Marshal(value)
	if err != nil {
		log.Errorf("Json conversion error. Err %v", err)
		return err
	}

	_, err = cp.client.KV().Put(&api.KVPair{Key: key, Value: jsonVal}, nil)

	return err
}

// DelObj deletes an object
func (cp *consulPlugin) DelObj(key string) error {
	key = processKey("/contiv.io/obj/" + processKey(key))
	_, err := cp.client.KV().Delete(key, nil)
	return err
}

// GetLocalAddr gets local address of the host
func (cp *consulPlugin) GetLocalAddr() (string, error) {
	return "", nil
}

// NewLock returns a new lock instance
func (cp *consulPlugin) NewLock(name string, myID string, ttl uint64) (LockInterface, error) {
	return nil, nil
}

// RegisterService registers a service
func (cp *consulPlugin) RegisterService(serviceInfo ServiceInfo) error {
	return nil
}

// GetService gets all instances of a service
func (cp *consulPlugin) GetService(name string) ([]ServiceInfo, error) {
	return nil, nil
}

// WatchService watches for service instance changes
func (cp *consulPlugin) WatchService(name string, eventCh chan WatchServiceEvent, stopCh chan bool) error {
	return nil
}

// DeregisterService deregisters a service instance
func (cp *consulPlugin) DeregisterService(serviceInfo ServiceInfo) error {
	return nil
}
