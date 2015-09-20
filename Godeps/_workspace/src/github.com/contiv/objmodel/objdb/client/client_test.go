package client

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/contiv/objmodel/objdb"

	log "github.com/Sirupsen/logrus"
)

type JsonObj struct {
	Value string
}

// New objdb client
var client = NewClient()

func TestMain(m *testing.M) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	os.Exit(m.Run())
}

// Perform Set/Get operation on default conf store
func TestSetGet(t *testing.T) {
	// Set
	setVal := JsonObj{
		Value: "test1",
	}

	if err := client.SetObj("/contiv.io/test", setVal); err != nil {
		fmt.Printf("Fatal setting key. Err: %v\n", err)
		t.Fatalf("Fatal setting key")
	}

	var retVal JsonObj

	if err := client.GetObj("/contiv.io/test", &retVal); err != nil {
		fmt.Printf("Fatal getting key. Err: %v\n", err)
		t.Fatalf("Fatal getting key")
	}

	if retVal.Value != "test1" {
		fmt.Printf("Got invalid response: %+v\n", retVal)
		t.Fatalf("Got invalid response")
	}

	if err := client.DelObj("/contiv.io/test"); err != nil {
		t.Fatalf("Fatal deleting test object. Err: %v", err)
	}

	fmt.Printf("Set/Get/Del test successful\n")
}

func TestSetGetPerformance(t *testing.T) {
	// Set
	setVal := JsonObj{
		Value: "test1",
	}
	var retVal JsonObj

	const testCount = 100

	log.Infof("Performing %d write tests", testCount)

	startTime := time.Now()

	for i := 0; i < testCount; i++ {
		if err := client.SetObj("/contiv.io/test"+strconv.Itoa(i), setVal); err != nil {
			fmt.Printf("Fatal setting key. Err: %v\n", err)
			t.Fatalf("Fatal setting key")
		}
	}

	timeTook := time.Since(startTime).Nanoseconds() / 1000000
	log.Infof("Write Test took %d milli seconds per write. %d ms total", timeTook/testCount, timeTook)

	log.Infof("Performing %d read tests", testCount)

	// Get test
	startTime = time.Now()

	for i := 0; i < testCount; i++ {
		if err := client.GetObj("/contiv.io/test"+strconv.Itoa(i), &retVal); err != nil {
			fmt.Printf("Fatal getting key. Err: %v\n", err)
			t.Fatalf("Fatal getting key")
		}

		if retVal.Value != "test1" {
			fmt.Printf("Got invalid response: %+v\n", retVal)
			t.Fatalf("Got invalid response")
		}
	}

	timeTook = time.Since(startTime).Nanoseconds() / 1000000
	log.Infof("Read Test took %d milli seconds per read. %d ms total", timeTook/testCount, timeTook)

	startTime = time.Now()

	for i := 0; i < testCount; i++ {
		if err := client.DelObj("/contiv.io/test" + strconv.Itoa(i)); err != nil {
			t.Fatalf("Fatal deleting test object. Err: %v", err)
		}
	}

	timeTook = time.Since(startTime).Nanoseconds() / 1000000
	log.Infof("Delete Test took %d milli seconds per delete. %d ms total", timeTook/testCount, timeTook)

	fmt.Printf("Set/Get/Del test successful\n")
}

func TestLockAcquireRelease(t *testing.T) {
	// Create a lock
	lock1, err := client.NewLock("master", "hostname1", 10)
	if err != nil {
		t.Fatal(err)
	}

	lock2, err := client.NewLock("master", "hostname2", 10)
	if err != nil {
		t.Fatal(err)
	}

	// Acquire the master lock
	if err := lock1.Acquire(0); err != nil {
		t.Fatalf("Fatal acquiring lock1")
	}

	time.Sleep(100 * time.Millisecond)

	// Try to acquire the same lock again. This should fail
	if err := lock2.Acquire(0); err != nil {
		t.Fatalf("Fatal acquiring lock2")
	}

	cnt := 1
	for {
		select {
		case event := <-lock1.EventChan():
			fmt.Printf("Event on Lock1: %+v\n\n", event)
			if event.EventType == objdb.LockAcquired {
				fmt.Printf("Master lock acquired by Lock1\n")
			}
		case event := <-lock2.EventChan():
			fmt.Printf("Event on Lock2: %+v\n\n", event)
			if event.EventType == objdb.LockAcquired {
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
	lock1, err := client.NewLock("master", "hostname1", 10)
	if err != nil {
		t.Fatal(err)
	}

	lock2, err := client.NewLock("master", "hostname2", 10)
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
			if event.EventType == objdb.LockAcquired {
				fmt.Printf("Master lock acquired by Lock1\n")
			}
		case event := <-lock2.EventChan():
			fmt.Printf("Event on Lock2: %+v\n\n", event)
			if event.EventType != objdb.LockAcquireTimeout {
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
	service1Info := objdb.ServiceInfo{
		ServiceName: "athena",
		HostAddr:    "10.10.10.10",
		Port:        4567,
	}
	service2Info := objdb.ServiceInfo{
		ServiceName: "athena",
		HostAddr:    "10.10.10.10",
		Port:        4568,
	}

	// register it
	if err := client.RegisterService(service1Info); err != nil {
		t.Fatalf("Fatal registering service. Err: %+v\n", err)
	}
	log.Infof("Registered service: %+v", service1Info)

	if err := client.RegisterService(service2Info); err != nil {
		t.Fatalf("Fatal registering service. Err: %+v\n", err)
	}
	log.Infof("Registered service: %+v", service2Info)

	resp, err := client.GetService("athena")
	if err != nil {
		t.Fatalf("Fatal getting service. Err: %+v\n", err)
	}

	log.Infof("Got service list: %+v\n", resp)

	if (len(resp) < 2) || (resp[0] != service1Info) || (resp[1] != service2Info) {
		t.Fatalf("Resp service list did not match input")
	}

	// Wait a while to make sure background refresh is working correctly
	time.Sleep(5 * time.Millisecond)

	resp, err = client.GetService("athena")
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
	service1Info := objdb.ServiceInfo{
		ServiceName: "athena",
		HostAddr:    "10.10.10.10",
		Port:        4567,
	}
	service2Info := objdb.ServiceInfo{
		ServiceName: "athena",
		HostAddr:    "10.10.10.10",
		Port:        4568,
	}

	// register it
	if err := client.DeregisterService(service1Info); err != nil {
		t.Fatalf("Fatal deregistering service. Err: %+v\n", err)
	}

	if err := client.DeregisterService(service2Info); err != nil {
		t.Fatalf("Fatal deregistering service. Err: %+v\n", err)
	}

	time.Sleep(time.Millisecond * 1)
}

func TestServiceWatch(t *testing.T) {
	service1Info := objdb.ServiceInfo{
		ServiceName: "athena",
		HostAddr:    "10.10.10.10",
		Port:        4567,
	}

	// register it

	if err := client.RegisterService(service1Info); err != nil {
		t.Fatalf("Fatal registering service. Err: %+v\n", err)
	}
	log.Infof("Registered service: %+v", service1Info)

	// Create event channel
	eventChan := make(chan objdb.WatchServiceEvent, 1)
	stopChan := make(chan bool, 1)

	// Start watching for service

	if err := client.WatchService("athena", eventChan, stopChan); err != nil {
		t.Fatalf("Fatal watching service. Err %v", err)
	}

	cnt := 1
	for {
		select {
		case srvEvent := <-eventChan:
			log.Infof("\n----\nReceived event: %+v\n----", srvEvent)
		case <-time.After(time.Millisecond * time.Duration(10)):
			service2Info := objdb.ServiceInfo{
				ServiceName: "athena",
				HostAddr:    "10.10.10.11",
				Port:        4567,
			}
			if cnt == 1 {
				// register it
				if err := client.RegisterService(service2Info); err != nil {
					t.Fatalf("Fatal registering service. Err: %+v\n", err)
				}
				log.Infof("Registered service: %+v", service2Info)
			} else if cnt == 5 {
				// deregister it
				if err := client.DeregisterService(service2Info); err != nil {
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
