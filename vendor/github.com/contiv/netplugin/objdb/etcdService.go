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
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/client"
)

// Service state
type etcdServiceState struct {
	ServiceName string        // Name of the service
	KeyName     string        // Service key name
	TTL         time.Duration // TTL for the service
	HostAddr    string        // Host name or IP address where its running
	Port        int           // Port number where its listening
	Hostname    string        // Host name where its running

	// Channel to stop ttl refresh
	stopChan chan bool
}

// RegisterService Register a service
// Service is registered with a ttl for 60sec and a goroutine is created
// to refresh the ttl.
func (ep *EtcdClient) RegisterService(serviceInfo ServiceInfo) error {
	keyName := "/contiv.io/service/" + serviceInfo.ServiceName + "/" +
		serviceInfo.HostAddr + ":" + strconv.Itoa(serviceInfo.Port)
	ttl := time.Duration(serviceInfo.TTL) * time.Second

	log.Infof("Registering service key: %s, value: %+v", keyName, serviceInfo)

	// if there is a previously registered service, stop refreshing it
	if ep.serviceDb[keyName] != nil {
		ep.serviceDb[keyName].stopChan <- true
	}

	// JSON format the object
	jsonVal, err := json.Marshal(serviceInfo)
	if err != nil {
		log.Errorf("Json conversion error. Err %v", err)
		return err
	}

	// create service state
	srvState := etcdServiceState{
		ServiceName: serviceInfo.ServiceName,
		KeyName:     keyName,
		TTL:         ttl,
		HostAddr:    serviceInfo.HostAddr,
		Port:        serviceInfo.Port,
		stopChan:    make(chan bool, 1),
		Hostname:    serviceInfo.Hostname,
	}

	// Run refresh in background
	go ep.refreshService(&srvState, string(jsonVal[:]))

	// Store it in DB
	ep.serviceDb[keyName] = &srvState

	return nil
}

// GetService lists all end points for a service
func (ep *EtcdClient) GetService(name string) ([]ServiceInfo, error) {
	keyName := "/contiv.io/service/" + name + "/"

	_, srvcList, err := ep.getServiceState(keyName)
	return srvcList, err
}

func (ep *EtcdClient) getServiceState(key string) (uint64, []ServiceInfo, error) {
	var srvcList []ServiceInfo
	retryCount := 0

	// Get the object from etcd client
	resp, err := ep.kapi.Get(context.Background(), key, &client.GetOptions{Recursive: true, Sort: true})
	for err != nil && err.Error() == client.ErrClusterUnavailable.Error() {
		// Retry after a delay
		retryCount++
		if retryCount%16 == 0 {
			log.Warnf("%v -- Retrying...", err)
		}

		time.Sleep(time.Second)
		resp, err = ep.kapi.Get(context.Background(), key,
			&client.GetOptions{Recursive: true, Sort: true})
	}

	if err != nil {
		if strings.Contains(err.Error(), "Key not found") {
			return 0, nil, nil
		}

		log.Errorf("Error getting key %s. Err: %v", key, err)
		return 0, nil, err
	}

	if !resp.Node.Dir {
		log.Errorf("Err. Response is not a directory: %+v", resp.Node)
		return 0, nil, errors.New("Invalid Response from etcd")
	}

	// Parse each node in the directory
	for _, node := range resp.Node.Nodes {
		var respSrvc ServiceInfo
		// Parse JSON response
		err = json.Unmarshal([]byte(node.Value), &respSrvc)
		if err != nil {
			log.Errorf("Error parsing object %s, Err %v", node.Value, err)
			return 0, nil, err
		}

		srvcList = append(srvcList, respSrvc)
	}

	watchIndex := resp.Index
	return watchIndex, srvcList, nil
}

// initServiceState reads the current state and injects it to the channel
// additionally, it returns the next index to watch
func (ep *EtcdClient) initServiceState(key string, eventCh chan WatchServiceEvent) (uint64, error) {
	mIndex, srvcList, err := ep.getServiceState(key)
	if err != nil {
		return mIndex, err
	}

	// walk each service and inject it as an add event
	for _, srvInfo := range srvcList {
		log.Debugf("Sending service add event: %+v", srvInfo)
		// Send Add event
		eventCh <- WatchServiceEvent{
			EventType:   WatchServiceEventAdd,
			ServiceInfo: srvInfo,
		}
	}

	return mIndex, nil
}

// WatchService Watch for a service
func (ep *EtcdClient) WatchService(name string, eventCh chan WatchServiceEvent, stopCh chan bool) error {
	keyName := "/contiv.io/service/" + name + "/"

	// Create channels
	watchCh := make(chan *client.Response, 1)

	// Create watch context
	watchCtx, watchCancel := context.WithCancel(context.Background())

	// Start the watch thread
	go func() {
		// Get current state and etcd index to watch
		watchIndex, err := ep.initServiceState(keyName, eventCh)
		if err != nil {
			log.Fatalf("Unable to watch service key: %s - %v", keyName,
				err)
		}

		log.Infof("Watching for service: %s at index %v", keyName, watchIndex)
		// Start the watch
		watcher := ep.kapi.Watcher(keyName, &client.WatcherOptions{AfterIndex: watchIndex, Recursive: true})
		if watcher == nil {
			log.Errorf("Error watching service %s. Etcd returned invalid watcher", keyName)

			// Emit the event
			eventCh <- WatchServiceEvent{EventType: WatchServiceEventError}
		}

		// Keep getting next event
		for {
			// Block till next watch event
			etcdRsp, err := watcher.Next(watchCtx)
			if err != nil && err.Error() == client.ErrClusterUnavailable.Error() {
				log.Infof("Stopping watch on key %s", keyName)
				return
			} else if err != nil {
				log.Errorf("Error %v during watch. Watch thread exiting", err)
				return
			}

			// Send it to watch channel
			watchCh <- etcdRsp
		}
	}()

	// handle messages from watch service
	go func() {
		var srvMap = make(map[string]ServiceInfo)
		for {
			select {
			case watchResp := <-watchCh:
				var srvInfo ServiceInfo

				log.Debugf("Received event {%#v}\n Node: {%#v}\n PrevNade: {%#v}", watchResp, watchResp.Node, watchResp.PrevNode)

				// derive service info from key
				srvKey := strings.TrimPrefix(watchResp.Node.Key, "/contiv.io/service/")

				// We ignore all events except Set/Delete/Expire
				// Note that Set event doesnt exactly mean new service end point.
				// If a service restarts and re-registers before it expired, we'll
				// receive set again. receivers need to handle this case
				if _, ok := srvMap[srvKey]; !ok && watchResp.Action == "set" {
					// Parse JSON response
					err := json.Unmarshal([]byte(watchResp.Node.Value), &srvInfo)
					if err != nil {
						log.Errorf("Error parsing object %s, Err %v", watchResp.Node.Value, err)
						break
					}

					log.Infof("Sending service add event: %+v", srvInfo)
					// Send Add event
					eventCh <- WatchServiceEvent{
						EventType:   WatchServiceEventAdd,
						ServiceInfo: srvInfo,
					}

					// save it in cache
					srvMap[srvKey] = srvInfo
				} else if (watchResp.Action == "delete") || (watchResp.Action == "expire") {
					// Parse JSON response
					err := json.Unmarshal([]byte(watchResp.PrevNode.Value), &srvInfo)
					if err != nil {
						log.Errorf("Error parsing object %s, Err %v", watchResp.Node.Value, err)
						break
					}

					log.Infof("Sending service del event: %+v", srvInfo)

					// Send Delete event
					eventCh <- WatchServiceEvent{
						EventType:   WatchServiceEventDel,
						ServiceInfo: srvInfo,
					}

					// remove it from cache
					delete(srvMap, srvKey)
				}
			case stopReq := <-stopCh:
				if stopReq {
					// Stop watch and return
					log.Infof("Stopping watch on %s", keyName)
					watchCancel()
					return
				}
			}
		}
	}()

	return nil
}

// DeregisterService Deregister a service
// This removes the service from the registry and stops the refresh groutine
func (ep *EtcdClient) DeregisterService(serviceInfo ServiceInfo) error {
	keyName := "/contiv.io/service/" + serviceInfo.ServiceName + "/" +
		serviceInfo.HostAddr + ":" + strconv.Itoa(serviceInfo.Port)

	// Find it in the database
	srvState := ep.serviceDb[keyName]
	if srvState == nil {
		log.Errorf("Could not find the service in db %s", keyName)
		return errors.New("Service not found")
	}

	// stop the refresh thread and delete service
	srvState.stopChan <- true
	delete(ep.serviceDb, keyName)

	// Delete the service instance
	_, err := ep.kapi.Delete(context.Background(), keyName, nil)
	if err != nil {
		log.Errorf("Error deleting key %s. Err: %v", keyName, err)
		return err
	}

	return nil
}

// Keep refreshing the service every 30sec
func (ep *EtcdClient) refreshService(srvState *etcdServiceState, keyVal string) {
	// Set it via etcd client
	_, err := ep.kapi.Set(context.Background(), srvState.KeyName, keyVal, &client.SetOptions{TTL: srvState.TTL})
	if err != nil {
		log.Errorf("Error setting key %s, Err: %v", srvState.KeyName, err)
	}

	// Loop forever
	for {
		select {
		case <-time.After(srvState.TTL / 3):
			log.Debugf("Refreshing key: %s", srvState.KeyName)

			_, err := ep.kapi.Set(context.Background(), srvState.KeyName, keyVal, &client.SetOptions{TTL: srvState.TTL})
			if err != nil {
				log.Errorf("Error setting key %s, Err: %v", srvState.KeyName, err)
			}

		case <-srvState.stopChan:
			log.Infof("Stop refreshing key: %s", srvState.KeyName)
			return
		}
	}
}
