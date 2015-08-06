package etcdClient

import (
	"sync"
	"time"

	api "github.com/contiv/objmodel/objdb"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/go-etcd/etcd"
)

// Etcd error codes
const EtcdErrorCodeNotFound = 100
const EtcdErrorCodeKeyExists = 105

// Lock object
type Lock struct {
	name          string
	myId          string
	isAcquired    bool
	isReleased    bool
	holderId      string
	ttl           uint64
	timeout       uint64
	modifiedIndex uint64
	eventChan     chan api.LockEvent
	stopChan      chan bool
	watchCh       chan *etcd.Response
	watchStopCh   chan bool
	client        *etcd.Client
	mutex         *sync.Mutex
}

// Create a new lock
func (self *EtcdPlugin) NewLock(name string, myId string, ttl uint64) (api.LockInterface, error) {
	// Create a lock
	return &Lock{
		name:        name,
		myId:        myId,
		ttl:         ttl,
		client:      self.client,
		eventChan:   make(chan api.LockEvent, 1),
		stopChan:    make(chan bool, 1),
		watchCh:     make(chan *etcd.Response, 1),
		watchStopCh: make(chan bool, 1),
		mutex:       new(sync.Mutex),
	}, nil
}

// Acquire a lock
func (self *Lock) Acquire(timeout uint64) error {
	self.mutex.Lock()
	defer self.mutex.Unlock()
	self.timeout = timeout

	// Acquire in background
	go self.acquireLock()

	return nil
}

// Release a lock
func (self *Lock) Release() error {
	keyName := "/contiv.io/lock/" + self.name

	self.mutex.Lock()
	defer self.mutex.Unlock()

	// Mark this as released
	self.isReleased = true

	// Send stop signal on stop channel
	self.stopChan <- true

	// If the lock was acquired, release it
	if self.isAcquired {
		// Update TTL on the lock
		resp, err := self.client.CompareAndDelete(keyName, self.myId, self.modifiedIndex)
		if err != nil {
			log.Errorf("Error Deleting key. Err: %v", err)
		} else {
			log.Infof("Deleted key lock %s, Resp: %+v", keyName, resp)

			// Update modifiedIndex
			self.modifiedIndex = resp.Node.ModifiedIndex
		}
	}

	return nil
}

// Note: This is for debug/test purposes only
// Stop a lock without releasing it.
// Let the etcd TTL expiry release it
func (self *Lock) Kill() error {
	self.mutex.Lock()
	defer self.mutex.Unlock()
	// Mark this as released
	self.isReleased = true

	// Send stop signal on stop channel
	self.stopChan <- true

	return nil
}

// Return event channel
func (self *Lock) EventChan() <-chan api.LockEvent {
	self.mutex.Lock()
	defer self.mutex.Unlock()
	return self.eventChan
}

// Check if the lock is acquired
func (self *Lock) IsAcquired() bool {
	self.mutex.Lock()
	defer self.mutex.Unlock()
	return self.isAcquired
}

// Get current lock holder's Id
func (self *Lock) GetHolder() string {
	self.mutex.Lock()
	defer self.mutex.Unlock()
	return self.holderId
}

// *********************** Internal functions *************
// Try acquiring a lock.
// This assumes its called in its own go routine
func (self *Lock) acquireLock() {
	keyName := "/contiv.io/lock/" + self.name

	// Start a watch on the lock first so that we dont loose any notifications
	go self.watchLock()

	// Wait in this loop forever till lock times out or released
	for {
		log.Infof("Getting the lock %s to see if its acquired", keyName)
		// Get the key and see if we or someone else has already acquired the lock
		resp, err := self.client.Get(keyName, false, false)
		if err != nil {
			if err.(*etcd.EtcdError).ErrorCode != EtcdErrorCodeNotFound {
				log.Errorf("Error getting the key %s. Err: %v", keyName, err)
			} else {
				log.Infof("Lock %s does not exist. trying to acquire it", keyName)
			}

			// Try to acquire the lock
			resp, err := self.client.Create(keyName, self.myId, self.ttl)
			if err != nil {
				if err.(*etcd.EtcdError).ErrorCode != EtcdErrorCodeKeyExists {
					log.Errorf("Error creating key %s. Err: %v", keyName, err)
				} else {
					log.Infof("Lock %s acquired by someone else", keyName)
				}
			} else {
				log.Infof("Acquired lock %s. Resp: %#v, Node: %+v", keyName, resp, resp.Node)

				self.mutex.Lock()
				// Successfully acquired the lock
				self.isAcquired = true
				self.holderId = self.myId
				self.modifiedIndex = resp.Node.ModifiedIndex
				self.mutex.Unlock()

				// Send acquired message to event channel
				self.eventChan <- api.LockEvent{EventType: api.LockAcquired}

				// refresh it
				self.refreshLock()

				self.mutex.Lock()
				// If lock is released, we are done, else go back and try to acquire it
				if self.isReleased {
					self.mutex.Unlock()
					return
				}
				self.mutex.Unlock()
			}
		} else if resp.Node.Value == self.myId {
			log.Infof("Already Acquired key %s. Resp: %#v, Node: %+v", keyName, resp, resp.Node)

			self.mutex.Lock()
			// We have already acquired the lock. just keep refreshing it
			self.isAcquired = true
			self.holderId = self.myId
			self.modifiedIndex = resp.Node.ModifiedIndex
			self.mutex.Unlock()

			// Send acquired message to event channel
			self.eventChan <- api.LockEvent{EventType: api.LockAcquired}

			// Refresh lock
			self.refreshLock()

			self.mutex.Lock()
			// If lock is released, we are done, else go back and try to acquire it
			if self.isReleased {
				self.mutex.Unlock()
				return
			}
			self.mutex.Unlock()
		} else if resp.Node.Value != self.myId {
			log.Infof("Lock already acquired by someone else. Resp: %+v, Node: %+v", resp, resp.Node)

			self.mutex.Lock()
			// Set the current holder's Id
			self.holderId = resp.Node.Value
			self.mutex.Unlock()

			// Wait for changes on the lock
			self.waitForLock()

			self.mutex.Lock()
			if self.isReleased {
				self.mutex.Unlock()
				return
			}
			self.mutex.Unlock()
		}
	}
}

// We couldnt acquire lock, Wait for changes on the lock
func (self *Lock) waitForLock() {
	// If timeout is not specified, set it to high value
	timeoutIntvl := time.Second * time.Duration(20000)
	if self.timeout != 0 {
		timeoutIntvl = time.Second * time.Duration(self.timeout)
	}

	log.Infof("Waiting to acquire lock (%s/%s)", self.name, self.myId)

	// Create a timer
	timer := time.NewTimer(timeoutIntvl)
	defer timer.Stop()

	// Wait for changes
	for {
		// wait on watch channel for holder to release the lock
		select {
		case <-timer.C:
			self.mutex.Lock()
			if self.timeout != 0 {
				self.mutex.Unlock()
				log.Infof("Lock timeout on lock %s/%s", self.name, self.myId)

				self.eventChan <- api.LockEvent{EventType: api.LockAcquireTimeout}

				log.Infof("Lock acquire timed out. Stopping lock")

				self.watchStopCh <- true

				// Release the lock
				self.Release()

				return
			}
			self.mutex.Unlock()
		case watchResp := <-self.watchCh:
			if watchResp != nil {
				log.Debugf("Received watch notification(%s/%s): %+v", self.name, self.myId, watchResp)

				if watchResp.Action == "expire" || watchResp.Action == "delete" ||
					watchResp.Action == "compareAndDelete" {
					log.Infof("Retrying to acquire lock")
					return
				}
			}
		case <-self.stopChan:
			log.Infof("Stopping lock")
			self.watchStopCh <- true

			return
		}
	}
}

// Refresh lock
func (self *Lock) refreshLock() {
	// Refresh interval is 40% of TTL
	refreshIntvl := time.Second * time.Duration(self.ttl*3/10)
	keyName := "/contiv.io/lock/" + self.name

	// Loop forever
	for {
		select {
		case <-time.After(refreshIntvl):
			// Update TTL on the lock
			resp, err := self.client.CompareAndSwap(keyName, self.myId, self.ttl,
				self.myId, self.modifiedIndex)
			if err != nil {
				log.Errorf("Error updating TTl. Err: %v", err)

				self.mutex.Lock()
				// We are not master anymore
				self.isAcquired = false
				self.mutex.Unlock()

				// Send lock lost event
				self.eventChan <- api.LockEvent{EventType: api.LockLost}

				// FIXME: trigger a lock lost event
				return
			} else {
				log.Debugf("Refreshed TTL on lock %s, Resp: %+v", keyName, resp)

				self.mutex.Lock()
				// Update modifiedIndex
				self.modifiedIndex = resp.Node.ModifiedIndex
				self.mutex.Unlock()
			}
		case watchResp := <-self.watchCh:
			// Since we already acquired the lock, nothing to do here
			// FIXME: see if we lost the lock
			if watchResp != nil {
				log.Debugf("Received watch notification for(%s/%s): %+v",
					self.name, self.myId, watchResp)
			}
		case <-self.stopChan:
			log.Infof("Stopping lock")
			self.watchStopCh <- true
			return
		}
	}
}

// Watch for changes on the lock
func (self *Lock) watchLock() {
	keyName := "/contiv.io/lock/" + self.name

	for {
		resp, err := self.client.Watch(keyName, 0, false, self.watchCh, self.watchStopCh)
		if err != nil {
			if err != etcd.ErrWatchStoppedByUser {
				log.Errorf("Error watching the key %s, Err %v", keyName, err)
			} else {
				log.Infof("Watch stopped for lock %s", keyName)
			}
		} else {
			log.Infof("Got Watch Resp: %+v", resp)
		}

		self.mutex.Lock()
		// If the lock is released, we are done
		if self.isReleased {
			self.mutex.Unlock()
			return
		}
		self.mutex.Unlock()

		// Wait for a second and go back to watching
		time.Sleep(1 * time.Second)
	}
}
