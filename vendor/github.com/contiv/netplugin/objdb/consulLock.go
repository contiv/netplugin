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
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/consul/api"
)

// Lock object
type consulLock struct {
	name       string
	keyName    string
	myID       string
	isAcquired bool
	isReleased bool
	ttl        string
	sessionID  string
	eventChan  chan LockEvent
	stopChan   chan struct{}
	mutex      *sync.Mutex
	client     *api.Client
}

// NewLock returns a new lock instance
func (cp *ConsulClient) NewLock(name string, myID string, ttl uint64) (LockInterface, error) {
	// Create a lock
	return &consulLock{
		name:      name,
		keyName:   "contiv.io/lock/" + name,
		myID:      myID,
		ttl:       fmt.Sprintf("%ds", ttl),
		eventChan: make(chan LockEvent, 1),
		stopChan:  make(chan struct{}, 1),
		mutex:     new(sync.Mutex),
		client:    cp.client,
	}, nil
}

// Acquire a lock
func (lk *consulLock) Acquire(timeout uint64) error {
	// Create consul session
	err := lk.createSession()
	if err != nil {
		log.Errorf("Error Creating session for lock %s. Err: %v", lk.keyName, err)
		return err
	}

	// Refresh the session in background
	go lk.renewSession()

	// Watch for changes on the lock
	go lk.acquireLock()

	// Wait till timeout and see if we were able to acquire the lock
	if timeout != 0 {
		go func() {
			time.Sleep(time.Duration(timeout) * time.Second)

			if !lk.IsAcquired() {
				lk.eventChan <- LockEvent{EventType: LockAcquireTimeout}

				// release the lock
				lk.Release()
			}
		}()
	}

	return nil
}

// Release a lock
func (lk *consulLock) Release() error {

	// Mark this as released
	lk.mutex.Lock()
	lk.isReleased = true
	lk.mutex.Unlock()

	// Send stop signal on stop channel
	close(lk.stopChan)

	// If the lock was acquired, release it
	if lk.IsAcquired() {
		lk.setAcquired(false)

		// Release it via consul client
		succ, _, err := lk.client.KV().Release(&api.KVPair{Key: lk.keyName, Value: []byte(lk.myID), Session: lk.sessionID}, nil)
		if err != nil {
			log.Errorf("Error releasing key %s/%s, Err: %v", lk.keyName, lk.myID, err)
			return err
		}

		if !succ {
			log.Warnf("Failed to release the lock %s/%s. !success", lk.name, lk.myID)
		}

	}

	return nil
}

// Kill Stops a lock without releasing it.
// Let the consul TTL expiry release it
// Note: This is for debug/test purposes only
func (lk *consulLock) Kill() error {
	lk.mutex.Lock()
	defer lk.mutex.Unlock()

	// Send stop signal on stop channel
	close(lk.stopChan)

	// Mark this as released
	lk.isReleased = true

	return nil
}

// EventChan Returns event channel
func (lk *consulLock) EventChan() <-chan LockEvent {
	lk.mutex.Lock()
	defer lk.mutex.Unlock()
	return lk.eventChan
}

// IsAcquired Checks if the lock is acquired
func (lk *consulLock) IsAcquired() bool {
	lk.mutex.Lock()
	defer lk.mutex.Unlock()
	return lk.isAcquired
}

// IsReleased Checks if the lock is released
func (lk *consulLock) IsReleased() bool {
	lk.mutex.Lock()
	defer lk.mutex.Unlock()
	return lk.isReleased
}

// GetHolder Gets current lock holder's ID
func (lk *consulLock) GetHolder() string {
	lk.mutex.Lock()
	defer lk.mutex.Unlock()

	// Get the current value from consul client
	resp, _, err := lk.client.KV().Get(lk.keyName, &api.QueryOptions{RequireConsistent: true})
	if err != nil {
		log.Errorf("Error getting key %s. Err: %v", lk.keyName, err)
		return ""
	}

	// return the current holder if exists
	if resp != nil {
		return string(resp.Value)
	}

	return ""
}

// *********************** Internal functions *************

// acquireLock watches for changes on a lock and tries to acquire it
func (lk *consulLock) acquireLock() {

	// save the modified idx
	var waitIdx uint64

	// loop forever
	for {
		// Get the object from consul client
		resp, meta, err := lk.client.KV().Get(lk.keyName, &api.QueryOptions{RequireConsistent: true, WaitIndex: waitIdx})
		if err != nil {
			log.Errorf("Error getting key %s. Err: %v", lk.keyName, err)

			// sleep for a bit a continue
			time.Sleep(time.Second)
			continue
		}

		log.Debugf("Got lock(%s) watch Resp: %+v", lk.myID, resp)

		// exit the loop if lock is released
		if lk.IsReleased() {
			log.Infof("Lock is released. exiting watch")
			return
		}

		// check if we are holding the lock
		if lk.IsAcquired() {
			// check if we lost the lock
			if resp == nil || resp.Session != lk.sessionID || string(resp.Value) != lk.myID {
				// lock is released
				lk.setAcquired(false)

				log.Infof("Lost lock %s", lk.name, lk.myID)

				// Send lock lost event
				lk.eventChan <- LockEvent{EventType: LockLost}
			} else {
				log.Debugf("Lock %s is held by me(%s)", lk.name, lk.myID)
			}
		} else {
			if resp == nil || resp.Session == "" {
				// try to acquire the lock
				succ, _, err := lk.client.KV().Acquire(&api.KVPair{Key: lk.keyName, Value: []byte(lk.myID), Session: lk.sessionID}, nil)
				if err != nil || !succ {
					log.Warnf("Error acquiring key %s/%s, Err: %v, succ: %v", lk.keyName, lk.myID, err, succ)

					// sleep for a bit a continue
					time.Sleep(time.Millisecond * 100)
					continue
				}

				log.Infof("Acquired lock %s/%s", lk.name, lk.myID)

				// Mark the lock as acquired
				lk.setAcquired(true)

				// Send acquired message to event channel
				lk.eventChan <- LockEvent{EventType: LockAcquired}
			} else {
				log.Debugf("Lock %s is held by some one else: %s", resp.Key, string(resp.Value))
			}
		}

		// use the last modified index
		waitIdx = meta.LastIndex
	}
}

// setAcquired marks the lock as acquired/not
func (lk *consulLock) setAcquired(isAcquired bool) {
	lk.mutex.Lock()
	lk.isAcquired = isAcquired
	lk.mutex.Unlock()
}

// createSession creates a consul-session for the lock
func (lk *consulLock) createSession() error {
	// session configuration
	sessCfg := api.SessionEntry{
		Name:      lk.keyName,
		Behavior:  "delete",
		LockDelay: 10 * time.Millisecond,
		TTL:       lk.ttl,
	}

	// Create consul session
	sessionID, _, err := lk.client.Session().CreateNoChecks(&sessCfg, nil)
	if err != nil {
		log.Errorf("Error Creating session for lock %s. Err: %v", lk.keyName, err)
		return err
	}

	log.Infof("Created session: %s for lock %s/%s", sessionID, lk.name, lk.myID)

	// save the session ID for later
	lk.mutex.Lock()
	lk.sessionID = sessionID
	lk.mutex.Unlock()

	return nil
}

// renewSession keeps the session alive.. If a session expires, it creates new one..
func (lk *consulLock) renewSession() {
	for {
		err := lk.client.Session().RenewPeriodic(lk.ttl, lk.sessionID, nil, lk.stopChan)
		if err == nil || lk.IsReleased() {
			// If lock was released, exit this go routine
			return
		}

		// Create new consul session
		err = lk.createSession()
		if err != nil {
			log.Errorf("Error Creating session for lock %s. Err: %v", lk.keyName, err)
		}
	}

}
