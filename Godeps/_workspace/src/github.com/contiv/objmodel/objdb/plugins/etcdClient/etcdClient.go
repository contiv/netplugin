package etcdClient

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	"github.com/contiv/objmodel/objdb"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/go-etcd/etcd"
)

type EtcdPlugin struct {
	client *etcd.Client // etcd client

	serviceDb map[string]*serviceState
	mutex     *sync.Mutex
}

type selfData struct {
	Name string `json:"name"`
}

type member struct {
	Name       string   `json:"name"`
	ClientURLs []string `json:"clientURLs"`
}

type memData struct {
	Members []member `json:"members"`
}

// etcd plugin state
var etcdPlugin = &EtcdPlugin{mutex: new(sync.Mutex)}

// Register the plugin
func InitPlugin() {
	objdb.RegisterPlugin("etcd", etcdPlugin)
}

// Initialize the etcd client
func (self *EtcdPlugin) Init(machines []string) error {
	self.mutex.Lock()
	defer self.mutex.Unlock()
	// Create a new client
	self.client = etcd.NewClient(machines)
	if self.client == nil {
		log.Fatal("Error creating etcd client.")
		return errors.New("Error creating etcd client")
	}

	// Set strong consistency
	self.client.SetConsistency(etcd.STRONG_CONSISTENCY)

	// Initialize service DB
	self.serviceDb = make(map[string]*serviceState)

	return nil
}

// Get an object
func (self *EtcdPlugin) GetObj(key string, retVal interface{}) error {
	keyName := "/contiv.io/obj/" + key

	// Get the object from etcd client
	resp, err := self.client.Get(keyName, false, false)
	if err != nil {
		log.Errorf("Error getting key %s. Err: %v", keyName, err)
		return err
	}

	// Parse JSON response
	if err := json.Unmarshal([]byte(resp.Node.Value), retVal); err != nil {
		log.Errorf("Error parsing object %s, Err %v", resp.Node.Value, err)
		return err
	}

	return nil
}

// Recursive function to look thru each directory and get the files
func recursAddNode(node *etcd.Node, list []string) []string {
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

// Get a list of objects in a directory
func (self *EtcdPlugin) ListDir(key string) ([]string, error) {
	keyName := "/contiv.io/obj/" + key

	// Get the object from etcd client
	resp, err := self.client.Get(keyName, true, true)
	if err != nil {
		return nil, nil
	}

	if !resp.Node.Dir {
		log.Errorf("ListDir response is not a directory")
		return nil, errors.New("Response is not directory")
	}

	retList := make([]string, 0)
	// Call a recursive function to recurse thru each directory and get all files
	// Warning: assumes directory itself is not interesting to the caller
	// Warning2: there is also an assumption that keynames are not required
	//           Which means, caller has to derive the key from value :(
	retList = recursAddNode(resp.Node, retList)

	return retList, nil
}

// Save an object, create if it doesnt exist
func (self *EtcdPlugin) SetObj(key string, value interface{}) error {
	keyName := "/contiv.io/obj/" + key

	// JSON format the object
	jsonVal, err := json.Marshal(value)
	if err != nil {
		log.Errorf("Json conversion error. Err %v", err)
		return err
	}

	// Set it via etcd client
	if _, err := self.client.Set(keyName, string(jsonVal[:]), 0); err != nil {
		log.Errorf("Error setting key %s, Err: %v", keyName, err)
		return err
	}

	return nil
}

// Remove an object
func (self *EtcdPlugin) DelObj(key string) error {
	keyName := "/contiv.io/obj/" + key

	// Remove it via etcd client
	if _, err := self.client.Delete(keyName, false); err != nil {
		log.Errorf("Error removing key %s, Err: %v", keyName, err)
		return err
	}

	return nil
}

// Get JSON output from a http request
func httpGetJson(url string, data interface{}) (interface{}, error) {
	res, err := http.Get(url)
	if err != nil {
		log.Errorf("Error during http get. Err: %v", err)
		return nil, err
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Errorf("Error during ioutil readall. Err: %v", err)
		return nil, err
	}

	if err := json.Unmarshal(body, data); err != nil {
		log.Errorf("Error during json unmarshall. Err: %v", err)
		return nil, err
	}

	log.Debugf("Results for (%s): %+v\n", url, data)

	return data, nil
}

// Return the local address where etcd is listening
func (self *EtcdPlugin) GetLocalAddr() (string, error) {
	var selfData selfData
	// Get self state from etcd
	if _, err := httpGetJson("http://localhost:2379/v2/stats/self", &selfData); err != nil {
		log.Errorf("Error getting self state. Err: %v", err)
		return "", errors.New("Error getting self state")
	}

	var memData memData

	// Get member list from etcd
	if _, err := httpGetJson("http://localhost:2379/v2/members", &memData); err != nil {
		log.Errorf("Error getting self state. Err: %v", err)
		return "", errors.New("Error getting self state")
	}

	myName := selfData.Name
	members := memData.Members

	for _, mem := range members {
		if mem.Name == myName {
			for _, clientUrl := range mem.ClientURLs {
				hostStr := strings.TrimPrefix(clientUrl, "http://")
				hostAddr := strings.Split(hostStr, ":")[0]
				log.Infof("Got host addr: %s", hostAddr)
				return hostAddr, nil
			}
		}
	}
	return "", errors.New("Address not found")
}
