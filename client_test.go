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
	runtime.GOMAXPROCS(runtime.NumCPU())

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

func TestLockAcquireRelease(t *testing.T) {
	// Create a lock
	lock1, err := etcdClient.NewLock("master", "hostname1", 10)
	if err != nil {
		t.Fatal(err)
	}

	lock2, err := etcdClient.NewLock("master", "hostname2", 10)
	if err != nil {
		t.Fatal(err)
	}

	// Acquire the master lock
	if err := lock1.Acquire(0); err != nil {
		t.Fatalf("Fatal acquiring lock1")
	}

	time.Sleep(100 * time.Millisecond)

	// Make sure lock1 is acquired
	if !lock1.IsAcquired() {
		t.Fatalf("Lock1 is not in acquired state")
	}

	// Try to acquire the same lock again. This should fail
	if err := lock2.Acquire(0); err != nil {
		t.Fatalf("Fatal acquiring lock2")
	}

	cnt := 1
	for {
		select {
		case event := <-lock1.EventChan():
			fmt.Printf("Event on Lock1: %+v\n\n", event)
			if event.EventType == LockAcquired {
				fmt.Printf("Master lock acquired by Lock1\n")
			}
		case event := <-lock2.EventChan():
			fmt.Printf("Event on Lock2: %+v\n\n", event)
			if event.EventType == LockAcquired {
				fmt.Printf("Master lock acquired by Lock2\n")
			}
		case <-time.After(100 * time.Millisecond):
			if cnt == 1 {
				fmt.Printf("100 ms timer. releasing Lock1\n\n")
				// At this point, lock1 should be holding the lock
				if !lock1.IsAcquired() {
					t.Fatalf("Lock1 failed to acquire lock\n\n")
				}

				// Release lock1 so that lock2 can acquire it
				lock1.Release()
				cnt++
			} else {
				fmt.Printf("200 ms timer. checking if lock2 is acquired\n\n")

				// At this point, lock2 should be holding the lock
				if !lock2.IsAcquired() {
					t.Fatalf("Lock2 failed to acquire lock\n\n")
				}

				fmt.Printf("Success. Lock2 Successfully acquired. releasing it\n")
				// we are done with the test
				lock2.Release()

				return
			}
		}
	}
}

func TestLockAcquireTimeout(t *testing.T) {
	fmt.Printf("\n\n\n =========================================================== \n\n\n")
	// Create a lock
	lock1, err := etcdClient.NewLock("master", "hostname1", 10)
	if err != nil {
		t.Fatal(err)
	}

	lock2, err := etcdClient.NewLock("master", "hostname2", 10)
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

	for {
		select {
		case event := <-lock1.EventChan():
			fmt.Printf("Event on Lock1: %+v\n\n", event)
			if event.EventType == LockAcquired {
				fmt.Printf("Master lock acquired by Lock1\n")
			}
		case event := <-lock2.EventChan():
			fmt.Printf("Event on Lock2: %+v\n\n", event)
			if event.EventType != LockAcquireTimeout {
				fmt.Printf("Invalid event on Lock2\n")
			} else {
				fmt.Printf("Lock2 timeout as expected")
			}
		case <-time.After(1 * time.Millisecond):
			fmt.Printf("1sec timer. releasing Lock1\n\n")
			// At this point, lock1 should be holding the lock
			if !lock1.IsAcquired() {
				t.Fatalf("Lock1 failed to acquire lock\n\n")
			}
			lock1.Release()

			return
		}
	}
}

func TestServiceRegister(t *testing.T) {
	// Service info
	service1Info := ServiceInfo{
		ServiceName: "athena",
		HostAddr:    "10.10.10.10",
		Port:        4567,
	}
	service2Info := ServiceInfo{
		ServiceName: "athena",
		HostAddr:    "10.10.10.10",
		Port:        4568,
	}

	// register it
	if err := etcdClient.RegisterService(service1Info); err != nil {
		t.Fatalf("Fatal registering service. Err: %+v\n", err)
	}
	log.Infof("Registered service: %+v", service1Info)

	if err := etcdClient.RegisterService(service2Info); err != nil {
		t.Fatalf("Fatal registering service. Err: %+v\n", err)
	}
	log.Infof("Registered service: %+v", service2Info)

	resp, err := etcdClient.GetService("athena")
	if err != nil {
		t.Fatalf("Fatal getting service. Err: %+v\n", err)
	}

	log.Infof("Got service list: %+v\n", resp)

	if (len(resp) < 2) || (resp[0] != service1Info) || (resp[1] != service2Info) {
		t.Fatalf("Resp service list did not match input")
	}

	// Wait a while to make sure background refresh is working correctly
	time.Sleep(5 * time.Millisecond)

	resp, err = etcdClient.GetService("athena")
	if err != nil {
		t.Fatalf("Fatal getting service. Err: %+v\n", err)
	}

	log.Infof("Got service list: %+v\n", resp)

	if (len(resp) < 2) || (resp[0] != service1Info) || (resp[1] != service2Info) {
		t.Fatalf("Resp service list did not match input")
	}
}

func TestServiceDeregister(t *testing.T) {
	// Service info
	service1Info := ServiceInfo{
		ServiceName: "athena",
		HostAddr:    "10.10.10.10",
		Port:        4567,
	}
	service2Info := ServiceInfo{
		ServiceName: "athena",
		HostAddr:    "10.10.10.10",
		Port:        4568,
	}

	// register it
	if err := etcdClient.DeregisterService(service1Info); err != nil {
		t.Fatalf("Fatal deregistering service. Err: %+v\n", err)
	}

	if err := etcdClient.DeregisterService(service2Info); err != nil {
		t.Fatalf("Fatal deregistering service. Err: %+v\n", err)
	}

	time.Sleep(time.Millisecond * 1)
}

func TestServiceWatch(t *testing.T) {
	service1Info := ServiceInfo{
		ServiceName: "athena",
		HostAddr:    "10.10.10.10",
		Port:        4567,
	}

	// register it

	if err := etcdClient.RegisterService(service1Info); err != nil {
		t.Fatalf("Fatal registering service. Err: %+v\n", err)
	}
	log.Infof("Registered service: %+v", service1Info)

	// Create event channel
	eventChan := make(chan WatchServiceEvent, 1)
	stopChan := make(chan bool, 1)

	// Start watching for service
	if err := etcdClient.WatchService("athena", eventChan, stopChan); err != nil {
		t.Fatalf("Fatal watching service. Err %v", err)
	}

	cnt := 1
	for {
		select {
		case srvEvent := <-eventChan:
			log.Infof("\n----\nReceived event: %+v\n----", srvEvent)
		case <-time.After(time.Millisecond * time.Duration(10)):
			service2Info := ServiceInfo{
				ServiceName: "athena",
				HostAddr:    "10.10.10.11",
				Port:        4567,
			}
			if cnt == 1 {
				// register it
				if err := etcdClient.RegisterService(service2Info); err != nil {
					t.Fatalf("Fatal registering service. Err: %+v\n", err)
				}
				log.Infof("Registered service: %+v", service2Info)
			} else if cnt == 5 {
				// deregister it
				if err := etcdClient.DeregisterService(service2Info); err != nil {
					t.Fatalf("Fatal deregistering service. Err: %+v\n", err)
				}
				log.Infof("Deregistered service: %+v", service2Info)
			} else if cnt == 7 {
				// Stop the watch
				stopChan <- true

				// wait a little and exit
				time.Sleep(time.Millisecond)

				return
			}
			cnt++
		}
	}
}
