package objdb

import (
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/client"
)

// Lock object
type etcdLock struct {
	name        string
	myID        string
	isAcquired  bool
	isReleased  bool
	holderID    string
	ttl         time.Duration
	timeout     uint64
	eventChan   chan LockEvent
	stopChan    chan bool
	watchCh     chan *client.Response
	watchCtx    context.Context
	watchCancel context.CancelFunc
	kapi        client.KeysAPI
	mutex       *sync.Mutex
}

// NewLock Create a new lock
func (ep *EtcdClient) NewLock(name string, myID string, ttl uint64) (LockInterface, error) {
	watchCtx, watchCancel := context.WithCancel(context.Background())
	// Create a lock
	return &etcdLock{
		name:        name,
		myID:        myID,
		ttl:         time.Duration(ttl) * time.Second,
		kapi:        ep.kapi,
		eventChan:   make(chan LockEvent, 1),
		stopChan:    make(chan bool, 1),
		watchCh:     make(chan *client.Response, 1),
		watchCtx:    watchCtx,
		watchCancel: watchCancel,
		mutex:       new(sync.Mutex),
	}, nil
}

// Acquire a lock
func (lk *etcdLock) Acquire(timeout uint64) error {
	lk.mutex.Lock()
	defer lk.mutex.Unlock()
	lk.timeout = timeout

	// Acquire in background
	go lk.acquireLock()

	return nil
}

// Release a lock
func (lk *etcdLock) Release() error {
	keyName := "/contiv.io/lock/" + lk.name

	lk.mutex.Lock()
	defer lk.mutex.Unlock()

	// Mark this as released
	lk.isReleased = true

	// Send stop signal on stop channel
	lk.stopChan <- true

	// If the lock was acquired, release it
	if lk.isAcquired {
		// Delete the lock entry
		resp, err := lk.kapi.Delete(context.Background(), keyName, &client.DeleteOptions{PrevValue: lk.myID})
		if err != nil {
			log.Errorf("Error Deleting key. Err: %v", err)
		} else {
			log.Infof("Deleted key lock %s, Resp: %+v", keyName, resp)
		}

		lk.isAcquired = false
	}

	return nil
}

// Kill Stops a lock without releasing it.
// Let the etcd TTL expiry release it
// Note: This is for debug/test purposes only
func (lk *etcdLock) Kill() error {
	lk.mutex.Lock()
	defer lk.mutex.Unlock()
	// Mark this as released
	lk.isReleased = true

	// Send stop signal on stop channel
	lk.stopChan <- true

	return nil
}

// EventChan Returns event channel
func (lk *etcdLock) EventChan() <-chan LockEvent {
	lk.mutex.Lock()
	defer lk.mutex.Unlock()
	return lk.eventChan
}

// IsAcquired Checks if the lock is acquired
func (lk *etcdLock) IsAcquired() bool {
	lk.mutex.Lock()
	defer lk.mutex.Unlock()
	return lk.isAcquired
}

// GetHolder Gets current lock holder's ID
func (lk *etcdLock) GetHolder() string {
	lk.mutex.Lock()
	defer lk.mutex.Unlock()

	keyName := "/contiv.io/lock/" + lk.name

	// Get the current value
	resp, err := lk.kapi.Get(context.Background(), keyName, nil)
	if err != nil {
		log.Warnf("Could not get current holder for lock %s", lk.name)
		return ""
	}

	return resp.Node.Value
}

// *********************** Internal functions *************
// Try acquiring a lock.
// This assumes its called in its own go routine
func (lk *etcdLock) acquireLock() {
	keyName := "/contiv.io/lock/" + lk.name

	// Start a watch on the lock first so that we dont loose any notifications
	go lk.watchLock()

	// Wait in this loop forever till lock times out or released
	for {
		log.Infof("Getting the lock %s to see if its acquired", keyName)
		// Get the key and see if we or someone else has already acquired the lock
		resp, err := lk.kapi.Get(context.Background(), keyName, &client.GetOptions{Quorum: true})
		if err != nil {
			if !client.IsKeyNotFound(err) {
				log.Errorf("Error getting the key %s. Err: %v", keyName, err)
				// Retry after a second in case of error
				time.Sleep(time.Second)
				continue
			} else {
				log.Infof("Lock %s does not exist. trying to acquire it", keyName)
			}

			// Try to acquire the lock
			resp, err := lk.kapi.Set(context.Background(), keyName, lk.myID, &client.SetOptions{PrevExist: client.PrevNoExist, TTL: lk.ttl})
			if err != nil {
				if _, ok := err.(client.Error); ok && err.(client.Error).Code != client.ErrorCodeNodeExist {
					log.Errorf("Error creating key %s. Err: %v", keyName, err)
				} else {
					log.Infof("Lock %s acquired by someone else", keyName)
				}
			} else {
				log.Debugf("Acquired lock %s. Resp: %#v, Node: %+v", keyName, resp, resp.Node)

				lk.mutex.Lock()
				// Successfully acquired the lock
				lk.isAcquired = true
				lk.holderID = lk.myID
				lk.mutex.Unlock()

				// Send acquired message to event channel
				lk.eventChan <- LockEvent{EventType: LockAcquired}

				// refresh it
				lk.refreshLock()

				lk.mutex.Lock()
				// If lock is released, we are done, else go back and try to acquire it
				if lk.isReleased {
					lk.mutex.Unlock()
					return
				}
				lk.mutex.Unlock()
			}
		} else if resp.Node.Value == lk.myID {
			log.Debugf("Already Acquired key %s. Resp: %#v, Node: %+v", keyName, resp, resp.Node)

			lk.mutex.Lock()
			// We have already acquired the lock. just keep refreshing it
			lk.isAcquired = true
			lk.holderID = lk.myID
			lk.mutex.Unlock()

			// Send acquired message to event channel
			lk.eventChan <- LockEvent{EventType: LockAcquired}

			// Refresh lock
			lk.refreshLock()

			lk.mutex.Lock()
			// If lock is released, we are done, else go back and try to acquire it
			if lk.isReleased {
				lk.mutex.Unlock()
				return
			}
			lk.mutex.Unlock()
		} else if resp.Node.Value != lk.myID {
			log.Debugf("Lock already acquired by someone else. Resp: %+v, Node: %+v", resp, resp.Node)

			lk.mutex.Lock()
			// Set the current holder's ID
			lk.holderID = resp.Node.Value
			lk.mutex.Unlock()

			// Wait for changes on the lock
			lk.waitForLock()

			lk.mutex.Lock()
			if lk.isReleased {
				lk.mutex.Unlock()
				return
			}
			lk.mutex.Unlock()
		}
	}
}

// We couldnt acquire lock, Wait for changes on the lock
func (lk *etcdLock) waitForLock() {
	// If timeout is not specified, set it to high value
	timeoutIntvl := time.Second * time.Duration(20000)
	if lk.timeout != 0 {
		timeoutIntvl = time.Second * time.Duration(lk.timeout)
	}

	log.Infof("Waiting to acquire lock (%s/%s)", lk.name, lk.myID)

	// Create a timer
	timer := time.NewTimer(timeoutIntvl)
	defer timer.Stop()

	// Wait for changes
	for {
		// wait on watch channel for holder to release the lock
		select {
		case <-timer.C:
			lk.mutex.Lock()
			if lk.timeout != 0 {
				lk.mutex.Unlock()
				log.Infof("Lock timeout on lock %s/%s", lk.name, lk.myID)

				lk.eventChan <- LockEvent{EventType: LockAcquireTimeout}

				log.Infof("Lock acquire timed out. Stopping lock")

				lk.watchCancel()

				// Release the lock
				lk.Release()

				return
			}
			lk.mutex.Unlock()
		case watchResp := <-lk.watchCh:
			if watchResp != nil {
				log.Debugf("Received watch notification(%s/%s): %+v", lk.name, lk.myID, watchResp)

				if watchResp.Action == "expire" || watchResp.Action == "delete" ||
					watchResp.Action == "compareAndDelete" {
					log.Infof("Retrying to acquire lock")
					return
				}
			}
		case <-lk.stopChan:
			log.Infof("Stopping lock")
			lk.watchCancel()
			return
		}
	}
}

// Refresh lock
func (lk *etcdLock) refreshLock() {
	// Refresh interval is 1/3rd of TTL
	refreshIntvl := lk.ttl / 3
	keyName := "/contiv.io/lock/" + lk.name

	// Loop forever
	for {
		select {
		case <-time.After(refreshIntvl):
			// Update TTL on the lock
			resp, err := lk.kapi.Set(context.Background(), keyName, lk.myID, &client.SetOptions{PrevExist: client.PrevExist, PrevValue: lk.myID, TTL: lk.ttl})
			if err != nil {
				log.Errorf("Error updating TTl. Err: %v", err)

				lk.mutex.Lock()
				// We are not master anymore
				lk.isAcquired = false
				lk.mutex.Unlock()

				// Send lock lost event
				lk.eventChan <- LockEvent{EventType: LockLost}

				return
			}

			log.Debugf("Refreshed TTL on lock %s, Resp: %+v", keyName, resp)
		case watchResp := <-lk.watchCh:
			// Since we already acquired the lock, nothing to do here
			if watchResp != nil {
				log.Debugf("Received watch notification for(%s/%s): %+v",
					lk.name, lk.myID, watchResp)

				// See if we lost the lock
				if string(watchResp.Node.Value) != lk.myID {
					log.Infof("Holder %s lost the lock %s", lk.myID, lk.name)

					lk.mutex.Lock()
					// We are not master anymore
					lk.isAcquired = false
					lk.mutex.Unlock()

					// Send lock lost event
					lk.eventChan <- LockEvent{EventType: LockLost}

					return
				}
			}
		case <-lk.stopChan:
			log.Infof("Stopping lock")
			lk.watchCancel()
			return
		}
	}
}

// Watch for changes on the lock
func (lk *etcdLock) watchLock() {
	keyName := "/contiv.io/lock/" + lk.name

	watcher := lk.kapi.Watcher(keyName, nil)
	if watcher == nil {
		log.Errorf("Error creating the watcher")
		return
	}
	for {
		resp, err := watcher.Next(lk.watchCtx)
		if err != nil && (err.Error() == client.ErrClusterUnavailable.Error() ||
			strings.Contains(err.Error(), "context canceled")) {
			log.Infof("Stopping watch on key %s", keyName)
			return
		} else if err != nil {
			log.Errorf("Error watching the key %s, Err %v.", keyName, err)
		} else {
			log.Debugf("Got Watch Resp: %+v", resp)

			// send the event to watch channel
			lk.watchCh <- resp
		}

		lk.mutex.Lock()
		// If the lock is released, we are done
		if lk.isReleased {
			lk.mutex.Unlock()
			return
		}
		lk.mutex.Unlock()
	}
}
