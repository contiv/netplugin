package objdb

import (
	"errors"
	"sync"
	"time"

	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
	client "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
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
	stoppedChan chan bool
	watchCh     chan *client.WatchResponse
	watchCtx    context.Context
	watchCancel context.CancelFunc
	client      *client.Client
	mutex       *sync.Mutex
	leaseID     client.LeaseID
}

// NewLock Create a new lock
func (ep *EtcdClient) NewLock(name string, myID string, ttl uint64) (LockInterface, error) {
	watchCtx, watchCancel := context.WithCancel(context.Background())

	lease, err := ep.client.Grant(context.TODO(), int64(ttl))
	if err != nil {
		return nil, errors.New("Failed to create lease: " + err.Error())
	}

	// Create a lock
	return &etcdLock{
		name:        name,
		myID:        myID,
		ttl:         time.Duration(ttl) * time.Second,
		client:      ep.client,
		eventChan:   make(chan LockEvent, 1),
		stopChan:    make(chan bool, 1),
		stoppedChan: make(chan bool, 1),
		watchCh:     make(chan *client.WatchResponse, 1),
		watchCtx:    watchCtx,
		watchCancel: watchCancel,
		mutex:       new(sync.Mutex),
		leaseID:     lease.ID,
	}, nil
}

// Acquire a lock
func (lk *etcdLock) Acquire(timeout uint64) error {
	lk.mutex.Lock()
	defer lk.mutex.Unlock()

	// TODO: we probably shouldn't allow acquiring a new lock
	//       without explicitly freeing the old one (if one exists)...

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
	<-lk.stoppedChan

	// If the lock was acquired, release it
	if lk.isAcquired {
		// revoke the lease which deletes the attached lock key
		resp, err := lk.client.Revoke(context.TODO(), lk.leaseID)
		if err != nil {
			log.Errorf("Error revoking lease %d for key %s. Err: %v", lk.leaseID, keyName, err)
		} else {
			log.Infof("Revoked lease %d for key %s, Resp: %+v", lk.leaseID, keyName, resp)
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
	<-lk.stoppedChan

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
	resp, err := lk.client.KV.Get(context.Background(), keyName)
	if err != nil {
		log.Warnf("Could not get current holder for lock %s", lk.name)
		return ""
	}

	if resp.Count == 0 {
		return "" // key not found therefore no leader
	}

	return string(resp.Kvs[0].Value)
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
		resp, err := lk.client.KV.Get(context.Background(), keyName)

		if err != nil {
			if len(resp.Kvs) == 0 {
				log.Errorf("Error getting the key %s. Err: %v", keyName, err)
				// Retry after a second in case of error
				time.Sleep(time.Second)
				continue
			}
		}

		found := resp.Count > 0

		if !found {
			log.Infof("Lock %s does not exist. trying to acquire it", keyName)

			// Try to acquire the lock
			req := client.OpPut(keyName, lk.myID, client.WithLease(lk.leaseID))
			cond := client.Compare(client.Version(keyName), "=", 0)
			resp, err := lk.client.Txn(context.TODO()).If(cond).Then(req).Commit()
			if err != nil {
				log.Infof("Lock acquisition transaction failed: %v", err)
				time.Sleep(time.Second)
				continue
			}

			log.Debugf("Acquired lock %s. Resp: %#v, PrevNode: %+v", keyName, resp, resp.Responses)

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

		} else if string(resp.Kvs[0].Value) == lk.myID {
			log.Debugf("Already Acquired key %s. Resp: %#v, Node: %+v", keyName, resp, resp.Kvs[0])

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
		} else if string(resp.Kvs[0].Value) != lk.myID {
			log.Debugf("Lock already acquired by someone else. Resp: %+v, Node: %+v", resp, resp.Kvs[0])

			lk.mutex.Lock()
			// Set the current holder's ID
			lk.holderID = string(resp.Kvs[0].Value)
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

			// TODO: check to see if ev.Canceled and log ev.Err()

			if watchResp != nil {
				log.Debugf("Received watch notification(%s/%s): %+v", lk.name, lk.myID, watchResp)

				for _, ev := range watchResp.Events {
					if ev.Type == mvccpb.DELETE {
						log.Infof("Retrying to acquire lock")
						return
					}

				}
			}
		case <-lk.stopChan:
			log.Infof("Stopping lock")
			lk.watchCancel()
			lk.stoppedChan <- true
			return
		}
	}
}

// Refresh lock
func (lk *etcdLock) refreshLock() {
	// Refresh interval is 1/3rd of TTL
	refreshIntvl := lk.ttl / 3

	// Loop forever
	for {
		select {
		case <-time.After(refreshIntvl):

			//			log.Infof("Refreshing lock with id: %d", lk.leaseID)

			// Update TTL on the lock
			_, err := lk.client.KeepAliveOnce(context.TODO(), lk.leaseID)
			if err != nil {
				log.Errorf("Error updating TTL. Err: %v", err)

				lk.mutex.Lock()
				// We are not master anymore
				lk.isAcquired = false
				lk.mutex.Unlock()

				// Send lock lost event
				lk.eventChan <- LockEvent{EventType: LockLost}

				return
			}

			log.Debugf("Refreshed lock with id: %d", lk.leaseID)
		case watchResp := <-lk.watchCh:
			// Since we already acquired the lock, nothing to do here
			if watchResp != nil {
				log.Debugf("Received watch notification for(%s/%s): %+v",
					lk.name, lk.myID, watchResp)

				for _, ev := range watchResp.Events {
					// See if we lost the lock
					if string(ev.Kv.Value) != lk.myID {
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
			}
		case <-lk.stopChan:
			log.Infof("Stopping lock")
			lk.watchCancel()
			lk.stoppedChan <- true
			return
		}
	}
}

// Watch for changes on the lock
func (lk *etcdLock) watchLock() {
	keyName := "/contiv.io/lock/" + lk.name

	watcher := lk.client.Watch(context.Background(), keyName)

	for resp := range watcher {
		lk.watchCh <- &resp

		lk.mutex.Lock()
		// If the lock is released, we are done
		if lk.isReleased {
			lk.mutex.Unlock()
			return
		}
		lk.mutex.Unlock()

	}
}
