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

	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/consul/api"
)

// Service state
type consulServiceState struct {
	ServiceName string        // Name of the service
	TTL         string        // Service TTL
	HostAddr    string        // Host name or IP address where its running
	Port        int           // Port number where its listening
	SessionID   string        // session id assigned by consul
	stopChan    chan struct{} // Channel to stop ttl refresh
	Hostname    string        // Host name where its running
}

// RegisterService registers a service
func (cp *ConsulClient) RegisterService(serviceInfo ServiceInfo) error {
	keyName := "contiv.io/service/" + serviceInfo.ServiceName + "/" +
		serviceInfo.HostAddr + ":" + strconv.Itoa(serviceInfo.Port)

	log.Infof("Registering service key: %s, value: %+v", keyName, serviceInfo)

	// if there is a previously registered service, no need to register it again..
	if cp.serviceDb[keyName] != nil {
		srvState := cp.serviceDb[keyName]
		if (srvState.ServiceName == serviceInfo.ServiceName) && (srvState.HostAddr == serviceInfo.HostAddr) &&
			(srvState.Port == serviceInfo.Port) {
			return nil
		}

		// stop and release the old key
		close(srvState.stopChan)

		// Delete the service instance
		_, err := cp.client.KV().Delete(keyName, nil)
		if err != nil {
			log.Errorf("Error deleting key %s. Err: %v", keyName, err)
			return err
		}
	}

	// JSON format the object
	jsonVal, err := json.Marshal(serviceInfo)
	if err != nil {
		log.Errorf("Json conversion error. Err %v", err)
		return err
	}

	// session configuration
	sessCfg := api.SessionEntry{
		Name:      keyName,
		Behavior:  "delete",
		LockDelay: 10 * time.Millisecond,
		TTL:       fmt.Sprintf("%ds", serviceInfo.TTL),
	}

	// Create consul session
	sessionID, _, err := cp.client.Session().CreateNoChecks(&sessCfg, nil)
	if err != nil {
		log.Errorf("Error Creating session for lock %s. Err: %v", keyName, err)
		return err
	}

	// check if the key already exists
	resp, _, err := cp.client.KV().Get(keyName, nil)
	if err != nil {
		log.Errorf("Error getting key %s. Err: %v", keyName, err)
		return err
	}

	// Delete the old key if it exists..
	if resp != nil {
		log.Infof("Deleting old service entry for key %s", keyName)
		_, err = cp.client.KV().Delete(keyName, nil)
		if err != nil {
			log.Errorf("Error deleting key %s. Err: %v", keyName, err)
			return err
		}
	}

	// Set it via consul client
	succ, _, err := cp.client.KV().Acquire(&api.KVPair{Key: keyName, Value: jsonVal, Session: sessionID}, nil)
	if err != nil {
		log.Errorf("Error setting key %s, Err: %v", keyName, err)
		return err
	}

	if !succ {
		log.Errorf("Failed to acquire key %s. Already acquired", keyName)
		return errors.New("Key already acquired")
	}

	// Run refresh in background
	stopChan := make(chan struct{})
	go cp.renewService(keyName, sessCfg.TTL, sessionID, jsonVal, stopChan)

	// Store it in DB
	cp.serviceDb[keyName] = &consulServiceState{
		ServiceName: serviceInfo.ServiceName,
		TTL:         sessCfg.TTL,
		HostAddr:    serviceInfo.HostAddr,
		Port:        serviceInfo.Port,
		SessionID:   sessionID,
		stopChan:    stopChan,
		Hostname:    serviceInfo.Hostname,
	}

	return nil
}

// GetService gets all instances of a service
func (cp *ConsulClient) GetService(srvName string) ([]ServiceInfo, error) {
	keyName := "contiv.io/service/" + srvName + "/"
	srvList, _, err := cp.getServiceInstances(keyName, 0)

	return srvList, err
}

// WatchService watches for service instance changes
func (cp *ConsulClient) WatchService(srvName string, eventCh chan WatchServiceEvent, stopCh chan bool) error {
	keyName := "contiv.io/service/" + srvName + "/"

	// Run in background
	go func() {
		var currSrvMap = make(map[string]ServiceInfo)

		// Get current list of services
		srvList, lastIdx, err := cp.getServiceInstances(keyName, 0)
		if err != nil {
			log.Errorf("Error getting service instances for (%s): Err: %v", srvName, err)
		} else {
			// for each instance trigger an add event
			for _, srvInfo := range srvList {
				eventCh <- WatchServiceEvent{
					EventType:   WatchServiceEventAdd,
					ServiceInfo: srvInfo,
				}

				// Add the service to local cache
				srvKey := srvInfo.HostAddr + ":" + strconv.Itoa(srvInfo.Port)
				currSrvMap[srvKey] = srvInfo
			}
		}

		// Loop till asked to stop
		for {
			// Check if we should quit
			select {
			case <-stopCh:
				return
			default:
				// Read the service instances
				srvList, lastIdx, err = cp.getServiceInstances(keyName, lastIdx)
				if err != nil {
					if api.IsServerError(err) || strings.Contains(err.Error(), "EOF") || strings.Contains(err.Error(), "connection refused") {
						log.Warnf("Consul service watch: server error: %v Retrying..", err)
					} else {
						log.Errorf("Error getting service instances for (%s): Err: %v. Exiting watch", srvName, err)
					}

					// Wait a little and continue
					time.Sleep(5 * time.Second)
					continue
				} else {
					log.Debugf("Got consul srv list: {%+v}. Curr: {%+v}", srvList, currSrvMap)
					var newSrvMap = make(map[string]ServiceInfo)

					// Check if there are any new services
					for _, srvInfo := range srvList {
						srvKey := srvInfo.HostAddr + ":" + strconv.Itoa(srvInfo.Port)

						// If the entry didnt exists previously, trigger add event
						if _, ok := currSrvMap[srvKey]; !ok {
							log.Debugf("Sending add event for srv: %v", srvInfo)
							eventCh <- WatchServiceEvent{
								EventType:   WatchServiceEventAdd,
								ServiceInfo: srvInfo,
							}
						}

						// create new service map
						newSrvMap[srvKey] = srvInfo
					}

					// for all entries in old service list, see if we need to delete any
					for _, srvInfo := range currSrvMap {
						srvKey := srvInfo.HostAddr + ":" + strconv.Itoa(srvInfo.Port)

						// if the entry does not exists in new list, delete it
						if _, ok := newSrvMap[srvKey]; !ok {
							log.Debugf("Sending delete event for srv: %v", srvInfo)
							eventCh <- WatchServiceEvent{
								EventType:   WatchServiceEventDel,
								ServiceInfo: srvInfo,
							}
						}
					}

					// set new srv map as the current
					currSrvMap = newSrvMap
				}
			}
		}
	}()
	return nil
}

// DeregisterService deregisters a service instance
func (cp *ConsulClient) DeregisterService(serviceInfo ServiceInfo) error {
	keyName := "contiv.io/service/" + serviceInfo.ServiceName + "/" +
		serviceInfo.HostAddr + ":" + strconv.Itoa(serviceInfo.Port)

	// Find it in the database
	srvState := cp.serviceDb[keyName]
	if srvState == nil {
		log.Errorf("Could not find the service in db %s", keyName)
		return errors.New("Service not found")
	}

	log.Infof("Deregistering service key: %s, value: %+v", keyName, serviceInfo)

	// stop the refresh thread and delete service
	close(srvState.stopChan)
	delete(cp.serviceDb, keyName)

	// Delete the service instance
	_, err := cp.client.KV().Delete(keyName, nil)
	if err != nil {
		log.Errorf("Error deleting key %s. Err: %v", keyName, err)
		return err
	}

	return nil
}

//--------------------- Internal functions -------------------
func (cp *ConsulClient) renewService(keyName, ttl, sessionID string, jsonVal []byte, stopChan chan struct{}) {
	for {
		err := cp.client.Session().RenewPeriodic(ttl, sessionID, nil, stopChan)
		if err == nil {
			log.Infof("Stoping renew on %s", keyName)
			return
		}
		log.Infof("RenewPeriodic for session %s exited with error: %v. Retrying..", keyName, err)

		// session configuration
		sessCfg := api.SessionEntry{
			Name:      keyName,
			Behavior:  "delete",
			LockDelay: 10 * time.Millisecond,
			TTL:       ttl,
		}

		// Create consul session
		sessionID, _, err = cp.client.Session().CreateNoChecks(&sessCfg, nil)
		if err != nil {
			log.Errorf("Error Creating session for lock %s. Err: %v", keyName, err)
		}

		// Delete the old key if it exists..
		log.Infof("Deleting old service entry for key %s", keyName)
		_, err = cp.client.KV().Delete(keyName, nil)
		if err != nil {
			log.Errorf("Error deleting key %s. Err: %v", keyName, err)
		}

		// Set it via consul client
		succ, _, err := cp.client.KV().Acquire(&api.KVPair{Key: keyName, Value: jsonVal, Session: sessionID}, nil)
		if err != nil {
			log.Errorf("Error setting key %s, Err: %v", keyName, err)
		} else if !succ {
			log.Errorf("Failed to acquire key %s. Already acquired", keyName)
		}
	}
}

// getServiceInstances gets the current list of service instances
func (cp *ConsulClient) getServiceInstances(key string, waitIdx uint64) ([]ServiceInfo, uint64, error) {
	var srvcList []ServiceInfo

	// Get the object from consul client
	kvs, meta, err := cp.client.KV().List(key, &api.QueryOptions{WaitIndex: waitIdx})
	if err != nil {
		log.Errorf("Error getting key %s. Err: %v", key, err)
		return nil, 0, err
	}

	// Consul returns success and a nil kv when a key is not found,
	// translate it to 'Key not found' error
	if kvs == nil {
		return []ServiceInfo{}, meta.LastIndex, nil
	}

	// Parse each node in the directory
	for _, kv := range kvs {
		var respSrvc ServiceInfo
		// Parse JSON response
		err = json.Unmarshal([]byte(kv.Value), &respSrvc)
		if err != nil {
			log.Errorf("Error parsing object %+v, Err %v", kv, err)
			return nil, 0, err
		}

		srvcList = append(srvcList, respSrvc)
	}

	return srvcList, meta.LastIndex, nil
}
