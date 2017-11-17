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
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
	client "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
)

// Service state
type etcdServiceState struct {
	ServiceName string // Name of the service
	KeyName     string // Service key name
	TTL         int64  // TTL for the service (seconds)
	HostAddr    string // Host name or IP address where its running
	Port        int    // Port number where its listening
	Hostname    string // Host name where its running

	// Channel to stop ttl refresh
	stopChan chan bool

	// Channel notified when ttl refresh is stopped
	stoppedChan chan bool

	// ID of the lease keeping keys alive
	leaseID client.LeaseID
}

// RegisterService Register a service
// Service is registered with a ttl for 60sec and a goroutine is created
// to refresh the ttl.
func (ep *EtcdClient) RegisterService(serviceInfo ServiceInfo) error {
	//	return nil
	keyName := "/contiv.io/service/" + serviceInfo.ServiceName + "/" +
		serviceInfo.HostAddr + ":" + strconv.Itoa(serviceInfo.Port)

	log.Infof("Registering service key: %s, value: %+v", keyName, serviceInfo)

	// if there is a previously registered service, stop refreshing it
	if _, ok := ep.serviceDb[keyName]; ok {
		ep.DeregisterService(serviceInfo)
	}

	// JSON format the object
	jsonVal, err := json.Marshal(serviceInfo)
	if err != nil {
		log.Errorf("Json conversion error. Err %v", err)
		return err
	}

	lease, err := ep.client.Grant(context.TODO(), serviceInfo.TTL)
	if err != nil {
		return errors.New("Failed to create lease: " + err.Error())
	}

	log.Infof("Got lease id %d for service %s with ttl %d", lease.ID, serviceInfo.ServiceName, serviceInfo.TTL)

	// create service state
	srvState := etcdServiceState{
		ServiceName: serviceInfo.ServiceName,
		KeyName:     keyName,
		TTL:         serviceInfo.TTL,
		HostAddr:    serviceInfo.HostAddr,
		Port:        serviceInfo.Port,
		stopChan:    make(chan bool, 1),
		stoppedChan: make(chan bool, 1),
		Hostname:    serviceInfo.Hostname,
		leaseID:     lease.ID,
	}

	// Store it in DB
	ep.serviceDb[keyName] = &srvState

	// Run refresh in background
	go ep.refreshService(&srvState, string(jsonVal[:]))

	return nil
}

// GetService lists all end points for a service
func (ep *EtcdClient) GetService(name string) ([]ServiceInfo, error) {
	// TODO: validate that name is not empty
	keyName := "/contiv.io/service/" + name + "/"

	_, srvcList, err := ep.getServiceState(keyName)
	return srvcList, err
}

func (ep *EtcdClient) getServiceState(key string) (uint64, []ServiceInfo, error) {
	var srvcList []ServiceInfo
	retryCount := 0

	// Get the object from etcd client
	resp, err := ep.client.Get(context.Background(), key, client.WithPrefix(), client.WithSort(client.SortByKey, client.SortAscend))
	for err != nil && err.Error() == client.ErrNoAvailableEndpoints.Error() {
		// Retry after a delay
		retryCount++
		if retryCount%16 == 0 {
			log.Warnf("%v -- Retrying...", err)
		}

		time.Sleep(time.Second)
		resp, err = ep.client.Get(context.Background(), key, client.WithPrefix(), client.WithSort(client.SortByKey, client.SortAscend))
	}

	if err != nil {
		if strings.Contains(err.Error(), "Key not found") {
			return 0, nil, nil
		}

		log.Errorf("Error getting key %s. Err: %v", key, err)
		return 0, nil, err
	}

	// Parse each node in the directory
	for _, node := range resp.Kvs {
		var respSrvc ServiceInfo
		// Parse JSON response
		err = json.Unmarshal(node.Value, &respSrvc)
		if err != nil {
			log.Errorf("Error parsing object %s, Err %v", node.Value, err)
			return 0, nil, err
		}

		srvcList = append(srvcList, respSrvc)
	}

	watchIndex := uint64(resp.Header.Revision)
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
	watchCh := make(chan *client.Event, 1)

	// Create watch context
	watchCtx, watchCancel := context.WithCancel(context.Background())

	// Start the watch thread
	go func() {
		// Get current state and etcd index to watch
		watchIndex, err := ep.initServiceState(keyName, eventCh)
		if err != nil {
			log.Fatalf("Unable to watch service key: %s - %v", keyName, err)
		}

		log.Infof("Watching for service: %s at index %v", keyName, watchIndex)
		// Start the watch
		watcher := ep.client.Watch(watchCtx, keyName, client.WithPrefix(), client.WithRev(int64(watchIndex)))

		if watcher == nil {
			log.Errorf("Error watching service %s. Etcd returned invalid watcher", keyName)

			// Emit the event
			eventCh <- WatchServiceEvent{EventType: WatchServiceEventError}
		}

		// Keep getting next event
		for watchResp := range watcher {

			// TODO: check to see if ev.Canceled and log ev.Err()

			for _, ev := range watchResp.Events {

				if err != nil {
					log.Errorf("Error %v during watch. Watch thread exiting", err)
					return
				}

				// Send it to watch channel
				watchCh <- ev
			}
		}

	}()

	// handle messages from watch service
	go func() {
		var srvMap = make(map[string]ServiceInfo)
		for {
			select {
			case watchResp := <-watchCh:
				var srvInfo ServiceInfo

				log.Debugf("Received event {%#v}\n Node: {%#v}\n PrevNode: {%#v}", watchResp, watchResp.Kv, watchResp.PrevKv)

				// derive service info from key
				srvKey := strings.TrimPrefix(string(watchResp.Kv.Key), "/contiv.io/service/")

				// We ignore all events except Set/Delete/Expire
				// Note that PUT event doesnt exactly mean new service end point.
				// If a service restarts and re-registers before it expired, we'll
				// receive PUT again. receivers need to handle this case
				if _, ok := srvMap[srvKey]; !ok && watchResp.Type == mvccpb.PUT {
					// Parse JSON response
					err := json.Unmarshal(watchResp.Kv.Value, &srvInfo)
					if err != nil {
						log.Errorf("Error parsing object %s, Err %v", watchResp.Kv.Value, err)
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
				} else if watchResp.Type == mvccpb.DELETE {
					// log.Infof("Received watch resp:\n\n%#v\n\n", watchResp)
					// // Parse JSON response
					// err := json.Unmarshal(watchResp.PrevKv.Value, &srvInfo)
					// if err != nil {
					// 	log.Errorf("Error parsing object %s, Err %v", watchResp.PrevKv.Value, err)
					// 	break
					// }

					// log.Infof("Sending service del event: %+v", srvInfo)

					// // Send Delete event
					// eventCh <- WatchServiceEvent{
					// 	EventType:   WatchServiceEventDel,
					// 	ServiceInfo: srvInfo,
					// }

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
	srvState, ok := ep.serviceDb[keyName]
	if !ok {
		log.Errorf("Could not find the service in db %s", keyName)
		return fmt.Errorf("Service %s not found", serviceInfo.ServiceName)
	}

	log.Infof("deregistering service %s with lease %d", serviceInfo.ServiceName, srvState.leaseID)

	// revoke the lease to delete the keys
	if _, err := ep.client.Revoke(context.TODO(), srvState.leaseID); err != nil {
		log.Errorf("Error revoking lease for %s (id: %d), err: %v", keyName, srvState.leaseID, err)
		// TODO: should we continue anyways here?
		// return err
	}

	// stop the refresh thread and delete service
	srvState.stopChan <- true
	<-srvState.stoppedChan

	delete(ep.serviceDb, keyName)

	return nil
}

// Keep refreshing the service every 30sec
func (ep *EtcdClient) refreshService(srvState *etcdServiceState, keyVal string) {
	_, err := ep.client.Put(context.Background(), srvState.KeyName, keyVal, client.WithLease(srvState.leaseID))
	if err != nil {
		log.Errorf("Error setting key %s with lease %d, Err: %v", srvState.KeyName, srvState.leaseID, err)
	}

	refreshInterval := (time.Duration(srvState.TTL) * time.Second) / 3

	log.Infof("Refreshing service %s with TTL of %d every %d seconds", srvState.ServiceName, srvState.TTL, refreshInterval/time.Second)

	// Loop forever
	for {
		select {
		case <-time.After(refreshInterval):
			// log.Debugf("Refreshing key: %s", srvState.KeyName)
			// log.Infof("Refreshing lease %d", srvState.leaseID)

			_, err := ep.client.KeepAliveOnce(context.TODO(), srvState.leaseID)
			if err != nil {
				log.Errorf("Error refreshing lease %d for key %s (service: %s). Err: %v", srvState.leaseID, srvState.KeyName, srvState.ServiceName, err)
			}

		case <-srvState.stopChan:
			log.Infof("Stop refreshing key %s with lease %d", srvState.KeyName, srvState.leaseID)
			srvState.stoppedChan <- true
			return
		}
	}
}
