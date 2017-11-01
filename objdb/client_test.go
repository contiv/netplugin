package objdb

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
)

type JSONObj struct {
	Value string
}

// New objdb client
var etcdClient API
var consulClient API

func TestMain(m *testing.M) {
	var err error
	runtime.GOMAXPROCS(4)

	log.SetFormatter(&log.TextFormatter{FullTimestamp: true, TimestampFormat: time.StampNano})

	// Init clients
	etcdClient, err = NewClient("")
	if err != nil {
		log.Fatalf("Error creating etcd client. Err: %v", err)
	}

	consulClient, err = NewClient("consul://localhost:8500")
	if err != nil {
		log.Fatalf("Error creating consul client. Err: %v", err)
	}

	os.Exit(m.Run())
}

// Verify only valid DB urls are accepted
func TestDbUrl(t *testing.T) {
	_, err := NewClient("invalid")
	if err == nil {
		t.Fatalf("Invalid URL accepted")
	}

	_, err = NewClient("invalid://localhost:2379")
	if err == nil {
		t.Fatalf("Invalid URL accepted")
	}

	_, err = NewClient("etcd:/localhost:2379")
	if err == nil {
		t.Fatalf("Invalid URL accepted")
	}

	_, err = NewClient("etcd://localhost")
	if err == nil {
		t.Fatalf("Invalid URL accepted")
	}

	_, err = NewClient("etcd://localhost:5000")
	if err == nil {
		t.Fatalf("Invalid URL accepted")
	}

	_, err = NewClient("consul://localhost")
	if err == nil {
		t.Fatalf("Invalid URL accepted")
	}

	_, err = NewClient("consul://localhost:5000")
	if err == nil {
		t.Fatalf("Invalid URL accepted")
	}
}

// Perform Set/Get operation on default conf store
func TestSetGet(t *testing.T) {
	// Set
	setVal := JSONObj{
		Value: "test1",
	}

	if err := etcdClient.SetObj("/contiv.io/test", setVal); err != nil {
		fmt.Printf("Fatal setting key. Err: %v\n", err)
		t.Fatalf("Fatal setting key")
	}

	var retVal JSONObj

	if err := etcdClient.GetObj("/contiv.io/test", &retVal); err != nil {
		fmt.Printf("Fatal getting key. Err: %v\n", err)
		t.Fatalf("Fatal getting key")
	}

	if retVal.Value != "test1" {
		fmt.Printf("Got invalid response: %+v\n", retVal)
		t.Fatalf("Got invalid response")
	}

	if err := etcdClient.DelObj("/contiv.io/test"); err != nil {
		t.Fatalf("Fatal deleting test object. Err: %v", err)
	}

	fmt.Printf("Set/Get/Del test successful\n")
}

func BenchmarkEtcdSet(b *testing.B) {
	setVal := JSONObj{
		Value: "test1",
	}
	for n := 0; n < b.N; n++ {
		if err := etcdClient.SetObj("/contiv.io/test"+strconv.Itoa(n), setVal); err != nil {
			b.Fatalf("Fatal setting key. Err: %v", err)
		}
	}
}

func BenchmarkEtcdGet(b *testing.B) {
	var retVal JSONObj
	setVal := JSONObj{
		Value: "test1",
	}

	for n := 0; n < b.N; n++ {
		if err := etcdClient.SetObj("/contiv.io/test"+strconv.Itoa(n), setVal); err != nil {
			b.Fatalf("Fatal setting key. Err: %v", err)
		}
	}

	// Reset timer so that only gets are tested
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		if err := etcdClient.GetObj("/contiv.io/test"+strconv.Itoa(n), &retVal); err != nil {
			b.Fatalf("Fatal getting key. Err: %v\n", err)
		}

		if retVal.Value != "test1" {
			b.Fatalf("Got invalid response: %+v\n", retVal)
		}
	}
}

func BenchmarkEtcdDel(b *testing.B) {
	setVal := JSONObj{
		Value: "test1",
	}

	for n := 0; n < b.N; n++ {
		if err := etcdClient.SetObj("/contiv.io/test"+strconv.Itoa(n), setVal); err != nil {
			b.Fatalf("Fatal setting key. Err: %v", err)
		}
	}

	// Reset timer so that only gets are tested
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		if err := etcdClient.DelObj("/contiv.io/test" + strconv.Itoa(n)); err != nil {
			b.Fatalf("Fatal deleting test object. Err: %v", err)
		}
	}
}

func TestConsulClientSetGet(t *testing.T) {
	setVal := JSONObj{
		Value: "test1",
	}

	if err := consulClient.SetObj("/contiv.io/test", setVal); err != nil {
		fmt.Printf("Fatal setting key. Err: %v\n", err)
		t.Fatalf("Fatal setting key")
	}

	var retVal JSONObj

	if err := consulClient.GetObj("/contiv.io/test", &retVal); err != nil {
		fmt.Printf("Fatal getting key. Err: %v\n", err)
		t.Fatalf("Fatal getting key")
	}

	if retVal.Value != "test1" {
		fmt.Printf("Got invalid response: %+v\n", retVal)
		t.Fatalf("Got invalid response")
	}

	if err := consulClient.DelObj("/contiv.io/test"); err != nil {
		t.Fatalf("Fatal deleting test object. Err: %v", err)
	}

	fmt.Printf("Consul Set/Get/Del test successful\n")
}

func BenchmarkConsulSet(b *testing.B) {
	setVal := JSONObj{
		Value: "test1",
	}
	for n := 0; n < b.N; n++ {
		if err := consulClient.SetObj("/contiv.io/test"+strconv.Itoa(n), setVal); err != nil {
			b.Fatalf("Fatal setting key. Err: %v", err)
		}
	}
}

func BenchmarkConsulGet(b *testing.B) {
	var retVal JSONObj
	setVal := JSONObj{
		Value: "test1",
	}

	for n := 0; n < b.N; n++ {
		if err := consulClient.SetObj("/contiv.io/test"+strconv.Itoa(n), setVal); err != nil {
			b.Fatalf("Fatal setting key. Err: %v", err)
		}
	}

	// Reset timer so that only gets are tested
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		if err := consulClient.GetObj("/contiv.io/test"+strconv.Itoa(n), &retVal); err != nil {
			b.Fatalf("Fatal getting key. Err: %v\n", err)
		}

		if retVal.Value != "test1" {
			b.Fatalf("Got invalid response: %+v\n", retVal)
		}
	}
}

func BenchmarkConsulDel(b *testing.B) {
	setVal := JSONObj{
		Value: "test1",
	}

	for n := 0; n < b.N; n++ {
		if err := consulClient.SetObj("/contiv.io/test"+strconv.Itoa(n), setVal); err != nil {
			b.Fatalf("Fatal setting key. Err: %v", err)
		}
	}

	// Reset timer so that only gets are tested
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		if err := consulClient.DelObj("/contiv.io/test" + strconv.Itoa(n)); err != nil {
			b.Fatalf("Fatal deleting test object. Err: %v", err)
		}
	}
}

func TestEtcdLockAcquireRelease(t *testing.T) {
	for i := 0; i < 3; i++ {
		testLockAcquireRelease(t, etcdClient)
	}
}

func TestConsulLockAcquireRelease(t *testing.T) {
	for i := 0; i < 3; i++ {
		testLockAcquireRelease(t, consulClient)
	}
}

func testLockAcquireRelease(t *testing.T, dbclient API) {
	lockTTL := uint64(10)

	// Create a lock
	lock1, err := dbclient.NewLock("master", "hostname1", lockTTL)
	if err != nil {
		t.Fatal(err)
	}

	lock2, err := dbclient.NewLock("master", "hostname2", lockTTL)
	if err != nil {
		t.Fatal(err)
	}

	// Acquire the master lock
	if err := lock1.Acquire(0); err != nil {
		t.Fatalf("Fatal acquiring lock1")
	}

	time.Sleep(300 * time.Millisecond)

	// Make sure lock1 is acquired
	if !lock1.IsAcquired() {
		t.Fatalf("Lock1 is not in acquired state")
	}

	// Try to acquire the same lock again. This should fail
	if err := lock2.Acquire(0); err != nil {
		t.Fatalf("Fatal acquiring lock2")
	}

	go func() {
		for {
			select {
			case event := <-lock1.EventChan():
				log.Infof("Event on Lock1: %+v", event)
				if event.EventType == LockAcquired {
					log.Infof("Master lock acquired by Lock1")
				}
			case event := <-lock2.EventChan():
				log.Infof("Event on Lock2: %+v", event)
				if event.EventType == LockAcquired {
					log.Infof("Master lock acquired by Lock2")
				}
			}
		}
	}()

	time.Sleep(time.Second)

	log.Infof("2 timer. releasing Lock1")
	// At this point, lock1 should be holding the lock
	if !lock1.IsAcquired() {
		t.Fatalf("Lock1 failed to acquire lock")
	}

	// Release lock1 so that lock2 can acquire it
	err = lock1.Release()
	if err != nil {
		t.Fatalf("Error releasing lock")
	}

	time.Sleep(time.Second)

	log.Infof("4s timer. checking if lock2 is acquired")

	// At this point, lock2 should be holding the lock
	if !lock2.IsAcquired() {
		t.Fatalf("Lock2 failed to acquire lock")
	}

	log.Infof("Success. Lock2 Successfully acquired. releasing it")

	// we are done with the test
	err = lock2.Release()
	if err != nil {
		t.Fatalf("Error releasing lock")
	}
}

func TestEtcdLockAcquireTimeout(t *testing.T) {
	testLockAcquireTimeout(t, etcdClient)
}

func TestConsulLockAcquireTimeout(t *testing.T) {
	testLockAcquireTimeout(t, consulClient)
}

func testLockAcquireTimeout(t *testing.T, dbClient API) {
	lockTTL := uint64(10)

	// Create a lock
	lock1, err := dbClient.NewLock("master", "hostnamet1", lockTTL)
	if err != nil {
		t.Fatal(err)
	}

	lock2, err := dbClient.NewLock("master", "hostnamet2", lockTTL)
	if err != nil {
		t.Fatal(err)
	}

	// Acquire the lock

	if err := lock1.Acquire(0); err != nil {
		t.Fatalf("Fatal acquiring lock1")
	}

	time.Sleep(100 * time.Millisecond)

	if err := lock2.Acquire(2); err != nil {
		t.Fatalf("Fatal acquiring lock2")
	}

	lock1AcquiredEvent := false
	lock2TimeoutEvent := false

	go func() {
		for {
			select {
			case event := <-lock1.EventChan():
				log.Infof("Event on Lock1: %+v\n\n", event)
				if event.EventType == LockAcquired {
					log.Infof("Master lock acquired by Lock1\n")
					lock1AcquiredEvent = true
				}
			case event := <-lock2.EventChan():
				log.Infof("Event on Lock2: %+v\n\n", event)
				if event.EventType != LockAcquireTimeout {
					t.Fatalf("Invalid event on Lock2\n")
				} else {
					log.Infof("Lock2 timeout as expected")
					lock2TimeoutEvent = true
				}
			}
		}
	}()

	time.Sleep(3 * time.Second)

	log.Infof("5sec timer. releasing Lock1\n\n")
	// At this point, lock1 should be holding the lock
	if !lock1.IsAcquired() {
		t.Fatalf("Lock1 failed to acquire lock\n\n")
	}

	if !lock1AcquiredEvent {
		t.Fatalf("Never received lock1 acquired event")
	}

	if !lock2TimeoutEvent {
		t.Fatalf("Never received lock2 timeout event")
	}

	err = lock1.Release()
	if err != nil {
		t.Fatalf("Error releasing lock1")
	}
}

func TestEtcdLockAcquireKill(t *testing.T) {
	testLockAcquireKill(t, etcdClient)
}

func TestConsulLockAcquireKill(t *testing.T) {
	testLockAcquireKill(t, consulClient)
}

func testLockAcquireKill(t *testing.T, dbclient API) {
	lockTTL := uint64(10)

	// Create a lock
	lock1, err := dbclient.NewLock("master", "hostnamek1", lockTTL)
	if err != nil {
		t.Fatal(err)
	}

	lock2, err := dbclient.NewLock("master", "hostnamek2", lockTTL)
	if err != nil {
		t.Fatal(err)
	}

	// Acquire the master lock
	if err := lock1.Acquire(0); err != nil {
		t.Fatalf("Fatal acquiring lock1")
	}

	time.Sleep(time.Second)

	// Make sure lock1 is acquired
	if !lock1.IsAcquired() {
		t.Fatalf("Lock1 is not in acquired state")
	}

	// Try to acquire the same lock again. This should fail
	if err := lock2.Acquire(0); err != nil {
		t.Fatalf("Fatal acquiring lock2")
	}

	go func() {
		for {
			select {
			case event := <-lock1.EventChan():
				log.Infof("Event on Lock1: %+v", event)
				if event.EventType == LockAcquired {
					log.Infof("Master lock acquired by Lock1")
				} else {
					t.Fatalf("Unexpected event %d on lock1", event.EventType)
				}
			case event := <-lock2.EventChan():
				log.Infof("Event on Lock2: %+v", event)
				if event.EventType == LockAcquired {
					log.Infof("Master lock acquired by Lock2")
				} else {
					t.Fatalf("Unexpected event %d on lock2", event.EventType)
				}
			}
		}
	}()

	time.Sleep(time.Second * time.Duration(lockTTL*2))

	log.Infof("%ds timer. killing Lock1", (2 * lockTTL))
	// At this point, lock1 should be holding the lock
	if !lock1.IsAcquired() {
		t.Fatalf("Lock1 failed to acquire lock")
	}

	// Release lock1 so that lock2 can acquire it
	err = lock1.Kill()
	if err != nil {
		t.Fatalf("Error releasing lock")
	}

	time.Sleep(time.Second * time.Duration(lockTTL*2))

	log.Infof("%ds timer. checking if lock2 is acquired", (lockTTL * 2))

	// At this point, lock2 should be holding the lock
	if !lock2.IsAcquired() {
		t.Fatalf("Lock2 failed to acquire lock")
	}

	log.Infof("Success. Lock2 Successfully acquired. releasing it")

	// we are done with the test
	err = lock2.Release()
	if err != nil {
		t.Fatalf("Error releasing lock")
	}

}

func TestEtcdServiceRegisterDeregister(t *testing.T) {
	testServiceRegisterDeregister(t, etcdClient)
}

func TestConsulServiceRegisterDeregister(t *testing.T) {
	testServiceRegisterDeregister(t, consulClient)
}

func testServiceRegisterDeregister(t *testing.T, dbClient API) {
	srvTTL := 10

	// Service info
	service1Info := ServiceInfo{
		ServiceName: "athena",
		TTL:         srvTTL,
		HostAddr:    "10.10.10.10",
		Port:        4567,
	}
	service2Info := ServiceInfo{
		ServiceName: "athena",
		TTL:         srvTTL,
		HostAddr:    "10.10.10.10",
		Port:        4568,
	}

	// register it
	if err := dbClient.RegisterService(service1Info); err != nil {
		t.Fatalf("Fatal registering service. Err: %+v\n", err)
	}
	log.Infof("Registered service: %+v", service1Info)

	if err := dbClient.RegisterService(service2Info); err != nil {
		t.Fatalf("Fatal registering service. Err: %+v\n", err)
	}
	log.Infof("Registered service: %+v", service2Info)

	// Wait for a second for registration to happen in background
	time.Sleep(time.Second)

	resp, err := dbClient.GetService("athena")
	if err != nil {
		t.Fatalf("Fatal getting service. Err: %+v\n", err)
	}

	log.Infof("Got service list: %+v\n", resp)

	if (len(resp) < 2) || (resp[0] != service1Info) || (resp[1] != service2Info) {
		t.Fatalf("Resp service list did not match input")
	}

	// Wait a while to make sure background refresh is working correctly
	time.Sleep(time.Duration(srvTTL*2) * time.Second)

	resp, err = dbClient.GetService("athena")
	if err != nil {
		t.Fatalf("Fatal getting service. Err: %+v\n", err)
	}

	log.Infof("Got service list: %+v\n", resp)

	if (len(resp) < 2) || (resp[0] != service1Info) || (resp[1] != service2Info) {
		t.Fatalf("Resp service list did not match input")
	}

	// deregister it
	if err := dbClient.DeregisterService(service1Info); err != nil {
		t.Fatalf("Fatal deregistering service. Err: %+v\n", err)
	}

	if err := dbClient.DeregisterService(service2Info); err != nil {
		t.Fatalf("Fatal deregistering service. Err: %+v\n", err)
	}

	resp, err = dbClient.GetService("athena")
	if err != nil {
		t.Fatalf("Fatal getting service. Err: %+v\n", err)
	}

	log.Infof("Got service list: %+v\n", resp)

	if len(resp) != 0 {
		t.Fatalf("Service still in list after deregister")
	}
}

func TestEtcdServiceRegisterMultiple(t *testing.T) {
	testServiceMultipleRegister(t, etcdClient)
}

func TestConsulServiceRegisterMultiple(t *testing.T) {
	testServiceMultipleRegister(t, consulClient)
}

func testServiceMultipleRegister(t *testing.T, dbClient API) {
	srvTTL := 10
	// Service info
	service1Info := ServiceInfo{
		ServiceName: "athena",
		TTL:         srvTTL,
		HostAddr:    "10.10.10.10",
		Port:        4567,
	}

	// register it multiple times
	for i := 0; i < 3; i++ {
		if err := dbClient.RegisterService(service1Info); err != nil {
			t.Fatalf("Fatal registering service. Err: %+v\n", err)
		}
		log.Infof("Registered service: %+v", service1Info)

		// sleep for a second
		time.Sleep(time.Second)
	}

	resp, err := dbClient.GetService("athena")
	if err != nil {
		t.Fatalf("Fatal getting service. Err: %+v\n", err)
	}

	log.Infof("Got service list: %+v\n", resp)

	if (len(resp) != 1) || (resp[0] != service1Info) {
		t.Fatalf("Resp service list did not match input")
	}

	// deregister it
	if err := dbClient.DeregisterService(service1Info); err != nil {
		t.Fatalf("Fatal deregistering service. Err: %+v\n", err)
	}

	resp, err = dbClient.GetService("athena")
	if err != nil {
		t.Fatalf("Fatal getting service. Err: %+v\n", err)
	}

	log.Infof("Got service list: %+v\n", resp)

	if len(resp) != 0 {
		t.Fatalf("Service still in list after deregister")
	}
}

func TestEtcdServiceWatch(t *testing.T) {
	testServiceWatch(t, etcdClient)
}

func TestConsulServiceWatch(t *testing.T) {
	testServiceWatch(t, consulClient)
}

func testServiceWatch(t *testing.T, dbClient API) {
	srvTTL := 10
	service1Info := ServiceInfo{
		ServiceName: "athena",
		TTL:         srvTTL,
		HostAddr:    "10.10.10.10",
		Port:        4567,
	}

	service2Info := ServiceInfo{
		ServiceName: "athena",
		TTL:         srvTTL,
		HostAddr:    "10.10.10.11",
		Port:        4567,
	}

	// Create event channel
	eventChan := make(chan WatchServiceEvent, 10)
	stopChan := make(chan bool, 1)

	// Start watching for service
	if err := dbClient.WatchService("athena", eventChan, stopChan); err != nil {
		t.Fatalf("Fatal watching service. Err %v", err)
	}

	// register it
	if err := dbClient.RegisterService(service1Info); err != nil {
		t.Fatalf("Fatal registering service. Err: %+v\n", err)
	}
	log.Infof("Registered service: %+v", service1Info)

	regCount := 0
	deregCount := 0
	go func() {
		for {
			select {
			case srvEvent := <-eventChan:
				log.Infof("\n----\nReceived event: %+v\n----", srvEvent)
				if srvEvent.EventType == WatchServiceEventAdd {
					regCount++
				}
				if srvEvent.EventType == WatchServiceEventDel {
					deregCount++
				}
			}
		}
	}()

	time.Sleep(time.Millisecond * time.Duration(100))

	// register it
	if err := dbClient.RegisterService(service2Info); err != nil {
		t.Fatalf("Fatal registering service. Err: %+v\n", err)
	}
	log.Infof("Registered service: %+v", service2Info)

	time.Sleep(time.Millisecond * time.Duration(300))

	// deregister it
	if err := dbClient.DeregisterService(service2Info); err != nil {
		t.Fatalf("Fatal deregistering service. Err: %+v\n", err)
	}
	log.Infof("Deregistered service: %+v", service2Info)

	time.Sleep(time.Millisecond * time.Duration(300))

	// Stop the watch
	stopChan <- true

	if regCount != 2 {
		t.Fatalf("Did not receive expected number of reg watch event for service")
	}

	if deregCount != 1 {
		t.Fatalf("Did not receive expected number of dereg watch event for service")
	}
}
