package consulClient

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/hashicorp/consul/api"
	"github.com/netplugin.orig/core"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/objdb"
)

type ConsulPlugin struct {
	client       *api.Client // consul client
	consulConfig api.Config

	mutex *sync.Mutex
}

var consulPlugin = &ConsulPlugin{mutex: new(sync.Mutex)}

// InitPlugin Register the plugin
func InitPlugin() {
	objdb.RegisterPlugin("consul", consulPlugin)
}

// Init initializes the consul client
func (self *ConsulPlugin) Init(machines []string) error {
	// default consul config
	self.consulConfig = api.Config{Address: "127.0.0.1:8500"}

	// Init consul client
	client, err := api.NewClient(&self.consulConfig)
	if err != nil {
		log.Fatalf("Error initializing consul client")
		return err
	}

	self.client = client

	return nil
}

func processKey(inKey string) string {
	//consul doesn't accepts keys starting with a '/', so trim the leading slash
	return strings.TrimPrefix(inKey, "/")
}

func (self *ConsulPlugin) GetObj(key string, retVal interface{}) error {
	key = processKey("/contiv.io/obj/" + processKey(key))

	resp, _, err := self.client.KV().Get(key, nil)
	if err != nil {
		return err
	}
	// Consul returns success and a nil kv when a key is not found,
	// translate it to 'Key not found' error
	if resp == nil {
		return core.Errorf("Key not found")
	}

	// Parse JSON response
	if err := json.Unmarshal([]byte(resp.Value), retVal); err != nil {
		log.Errorf("Error parsing object %s, Err %v", resp.Value, err)
		return err
	}

	return nil
}

// ListDir returns a list of keys in a directory
func (self *ConsulPlugin) ListDir(key string) ([]string, error) {
	key = processKey("/contiv.io/obj/" + processKey(key))

	kvs, _, err := self.client.KV().List(key, nil)
	if err != nil {
		return nil, err
	}
	// Consul returns success and a nil kv when a key is not found,
	// translate it to 'Key not found' error
	if kvs == nil {
		return nil, core.Errorf("Key not found")
	}

	var keys []string
	for _, kv := range kvs {
		keys = append(keys, kv.Key)
	}

	return keys, nil
}

// SetObj writes an object
func (self *ConsulPlugin) SetObj(key string, value interface{}) error {
	key = processKey("/contiv.io/obj/" + processKey(key))

	// JSON format the object
	jsonVal, err := json.Marshal(value)
	if err != nil {
		log.Errorf("Json conversion error. Err %v", err)
		return err
	}

	_, err = self.client.KV().Put(&api.KVPair{Key: key, Value: jsonVal}, nil)

	return err
}

// DelObj deletes an object
func (self *ConsulPlugin) DelObj(key string) error {
	key = processKey("/contiv.io/obj/" + processKey(key))
	_, err := self.client.KV().Delete(key, nil)
	return err
}

func (self *ConsulPlugin) GetLocalAddr() (string, error) {
	return "", nil
}
func (self *ConsulPlugin) NewLock(name string, myId string, ttl uint64) (objdb.LockInterface, error) {
	return nil, nil
}
func (self *ConsulPlugin) RegisterService(serviceInfo objdb.ServiceInfo) error {
	return nil
}
func (self *ConsulPlugin) GetService(name string) ([]objdb.ServiceInfo, error) {
	return nil, nil
}
func (self *ConsulPlugin) WatchService(name string, eventCh chan objdb.WatchServiceEvent, stopCh chan bool) error {
	return nil
}
func (self *ConsulPlugin) DeregisterService(serviceInfo objdb.ServiceInfo) error {
	return nil
}
