package objdb

import (
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/go-etcd/etcd"
)

// Etcd error codes
// Not found
const EtcdErrorCodeNotFound = 100

// Key already exists
const EtcdErrorCodeKeyExists = 105

// Lock object
type Lock struct {
	name        string
	myID        string
	isAcquired  bool
	isReleased  bool
	holderID    string
	ttl         uint64
	timeout     uint64
	eventChan   chan LockEvent
	stopChan    chan bool
	watchCh     chan *etcd.Response
	watchStopCh chan bool
	client      *etcd.Client
	mutex       *sync.Mutex
}

// Create a new lock
func (ep *etcdPlugin) NewLock(name string, myID string, ttl uint64) (LockInterface, error) {
	// Create a lock
	return &Lock{
		name:        name,
		myID:        myID,
		ttl:         ttl,
		client:      ep.client,
		eventChan:   make(chan LockEvent, 1),
		stopChan:    make(chan bool, 1),
		watchCh:     make(chan *etcd.Response, 1),
		watchStopCh: make(chan bool, 1),
		mutex:       new(sync.Mutex),
	}, nil
}

// Acquire a lock
func (ep *Lock) Acquire(timeout uint64) error {
	ep.mutex.Lock()
	defer ep.mutex.Unlock()
	ep.timeout = timeout

	// Acquire in background
	go ep.acquireLock()

	return nil
}

// Release a lock
func (ep *Lock) Release() error {
	keyName := "/contiv.io/lock/" + ep.name

	ep.mutex.Lock()
	defer ep.mutex.Unlock()

	// Mark this as released
	ep.isReleased = true

	// Send stop signal on stop channel
	ep.stopChan <- true

	// If the lock was acquired, release it
	if ep.isAcquired {
		// Update TTL on the lock
		resp, err := ep.client.CompareAndDelete(keyName, ep.myID, 0)
		if err != nil {
			log.Errorf("Error Deleting key. Err: %v", err)
		} else {
			log.Infof("Deleted key lock %s, Resp: %+v", keyName, resp)
		}
	}

	return nil
}

// Note: This is for debug/test purposes only
// Stop a lock without releasing it.
// Let the etcd TTL expiry release it
func (ep *Lock) Kill() error {
	ep.mutex.Lock()
	defer ep.mutex.Unlock()
	// Mark this as released
	ep.isReleased = true

	// Send stop signal on stop channel
	ep.stopChan <- true

	return nil
}

// Return event channel
func (ep *Lock) EventChan() <-chan LockEvent {
	ep.mutex.Lock()
	defer ep.mutex.Unlock()
	return ep.eventChan
}

// Check if the lock is acquired
func (ep *Lock) IsAcquired() bool {
	ep.mutex.Lock()
	defer ep.mutex.Unlock()
	return ep.isAcquired
}

// Get current lock holder's ID
func (ep *Lock) GetHolder() string {
	ep.mutex.Lock()
	defer ep.mutex.Unlock()

	keyName := "/contiv.io/lock/" + ep.name

	// Get the current value
	resp, err := ep.client.Get(keyName, false, false)
	if err != nil {
		log.Warnf("Could not get current holder for lock %s", ep.name)
		return ""
	}

	return resp.Node.Value
}

// *********************** Internal functions *************
// Try acquiring a lock.
// This assumes its called in its own go routine
func (ep *Lock) acquireLock() {
	keyName := "/contiv.io/lock/" + ep.name

	// Start a watch on the lock first so that we dont loose any notifications
	go ep.watchLock()

	// Wait in this loop forever till lock times out or released
	for {
		log.Infof("Getting the lock %s to see if its acquired", keyName)
		// Get the key and see if we or someone else has already acquired the lock
		resp, err := ep.client.Get(keyName, false, false)
		if err != nil {
			if err.(*etcd.EtcdError).ErrorCode != EtcdErrorCodeNotFound {
				log.Errorf("Error getting the key %s. Err: %v", keyName, err)
			} else {
				log.Infof("Lock %s does not exist. trying to acquire it", keyName)
			}

			// Try to acquire the lock
			resp, err := ep.client.Create(keyName, ep.myID, ep.ttl)
			if err != nil {
				if err.(*etcd.EtcdError).ErrorCode != EtcdErrorCodeKeyExists {
					log.Errorf("Error creating key %s. Err: %v", keyName, err)
				} else {
					log.Infof("Lock %s acquired by someone else", keyName)
				}
			} else {
				log.Infof("Acquired lock %s. Resp: %#v, Node: %+v", keyName, resp, resp.Node)

				ep.mutex.Lock()
				// Successfully acquired the lock
				ep.isAcquired = true
				ep.holderID = ep.myID
				ep.mutex.Unlock()

				// Send acquired message to event channel
				ep.eventChan <- LockEvent{EventType: LockAcquired}

				// refresh it
				ep.refreshLock()

				ep.mutex.Lock()
				// If lock is released, we are done, else go back and try to acquire it
				if ep.isReleased {
					ep.mutex.Unlock()
					return
				}
				ep.mutex.Unlock()
			}
		} else if resp.Node.Value == ep.myID {
			log.Infof("Already Acquired key %s. Resp: %#v, Node: %+v", keyName, resp, resp.Node)

			ep.mutex.Lock()
			// We have already acquired the lock. just keep refreshing it
			ep.isAcquired = true
			ep.holderID = ep.myID
			ep.mutex.Unlock()

			// Send acquired message to event channel
			ep.eventChan <- LockEvent{EventType: LockAcquired}

			// Refresh lock
			ep.refreshLock()

			ep.mutex.Lock()
			// If lock is released, we are done, else go back and try to acquire it
			if ep.isReleased {
				ep.mutex.Unlock()
				return
			}
			ep.mutex.Unlock()
		} else if resp.Node.Value != ep.myID {
			log.Infof("Lock already acquired by someone else. Resp: %+v, Node: %+v", resp, resp.Node)

			ep.mutex.Lock()
			// Set the current holder's ID
			ep.holderID = resp.Node.Value
			ep.mutex.Unlock()

			// Wait for changes on the lock
			ep.waitForLock()

			ep.mutex.Lock()
			if ep.isReleased {
				ep.mutex.Unlock()
				return
			}
			ep.mutex.Unlock()
		}
	}
}

// We couldnt acquire lock, Wait for changes on the lock
func (ep *Lock) waitForLock() {
	// If timeout is not specified, set it to high value
	timeoutIntvl := time.Second * time.Duration(20000)
	if ep.timeout != 0 {
		timeoutIntvl = time.Second * time.Duration(ep.timeout)
	}

	log.Infof("Waiting to acquire lock (%s/%s)", ep.name, ep.myID)

	// Create a timer
	timer := time.NewTimer(timeoutIntvl)
	defer timer.Stop()

	// Wait for changes
	for {
		// wait on watch channel for holder to release the lock
		select {
		case <-timer.C:
			ep.mutex.Lock()
			if ep.timeout != 0 {
				ep.mutex.Unlock()
				log.Infof("Lock timeout on lock %s/%s", ep.name, ep.myID)

				ep.eventChan <- LockEvent{EventType: LockAcquireTimeout}

				log.Infof("Lock acquire timed out. Stopping lock")

				ep.watchStopCh <- true

				// Release the lock
				ep.Release()

				return
			}
			ep.mutex.Unlock()
		case watchResp := <-ep.watchCh:
			if watchResp != nil {
				log.Debugf("Received watch notification(%s/%s): %+v", ep.name, ep.myID, watchResp)

				if watchResp.Action == "expire" || watchResp.Action == "delete" ||
					watchResp.Action == "compareAndDelete" {
					log.Infof("Retrying to acquire lock")
					return
				}
			}
		case <-ep.stopChan:
			log.Infof("Stopping lock")
			ep.watchStopCh <- true

			return
		}
	}
}

// Refresh lock
func (ep *Lock) refreshLock() {
	// Refresh interval is 1/3rd of TTL
	refreshIntvl := time.Second * time.Duration(ep.ttl/3)
	keyName := "/contiv.io/lock/" + ep.name

	// Loop forever
	for {
		select {
		case <-time.After(refreshIntvl):
			// Update TTL on the lock
			resp, err := ep.client.CompareAndSwap(keyName, ep.myID, ep.ttl, ep.myID, 0)
			if err != nil {
				log.Errorf("Error updating TTl. Err: %v", err)

				ep.mutex.Lock()
				// We are not master anymore
				ep.isAcquired = false
				ep.mutex.Unlock()

				// Send lock lost event
				ep.eventChan <- LockEvent{EventType: LockLost}

				// FIXME: trigger a lock lost event
				return
			} else {
				log.Debugf("Refreshed TTL on lock %s, Resp: %+v", keyName, resp)
			}
		case watchResp := <-ep.watchCh:
			// Since we already acquired the lock, nothing to do here
			// FIXME: see if we lost the lock
			if watchResp != nil {
				log.Debugf("Received watch notification for(%s/%s): %+v",
					ep.name, ep.myID, watchResp)
			}
		case <-ep.stopChan:
			log.Infof("Stopping lock")
			ep.watchStopCh <- true
			return
		}
	}
}

// Watch for changes on the lock
func (ep *Lock) watchLock() {
	keyName := "/contiv.io/lock/" + ep.name

	for {
		resp, err := ep.client.Watch(keyName, 0, false, ep.watchCh, ep.watchStopCh)
		if err != nil {
			if err != etcd.ErrWatchStoppedByUser {
				log.Errorf("Error watching the key %s, Err %v", keyName, err)
			} else {
				log.Infof("Watch stopped for lock %s", keyName)
			}
		} else {
			log.Infof("Got Watch Resp: %+v", resp)
		}

		ep.mutex.Lock()
		// If the lock is released, we are done
		if ep.isReleased {
			ep.mutex.Unlock()
			return
		}
		ep.mutex.Unlock()

		// Wait for a second and go back to watching
		time.Sleep(1 * time.Second)
	}
}
