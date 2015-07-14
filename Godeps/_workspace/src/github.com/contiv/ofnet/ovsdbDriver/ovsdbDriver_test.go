package ovsdbDriver

import (
	"fmt"
	"testing"
	"time"

	"github.com/contiv/symphony/pkg/netutils"
)

/*

func TestCreateBridge(t *testing.T) {
    // Connect to OVS
    ovsDriver := NewOvsDriver()

    // Test create
    err := ovsDriver.CreateBridge("ovsbr10")
    if (err != nil) {
        fmt.Printf("Error creating the bridge. Err: %v", err)
        t.Errorf("Failed to create a bridge")
    }
}

func TestDeleteBridge(t *testing.T) {
    // Connect to OVS
    ovsDriver := NewOvsDriver()

    // Test delete
    err := ovsDriver.DeleteBridge("ovsbr10")
    if (err != nil) {
        fmt.Printf("Error deleting the bridge. Err: %v", err)
        t.Errorf("Failed to delete a bridge")
    }

}

func TestCreateDeleteMultipleBridge(t *testing.T) {
    // Connect to OVS
    ovsDriver := NewOvsDriver()

    // Test create
    for i := 0; i < 10; i++ {
        brName := "ovsbr1" + fmt.Sprintf("%d", i)
        err := ovsDriver.CreateBridge(brName)
        if (err != nil) {
            fmt.Printf("Error creating the bridge. Err: %v", err)
            t.Errorf("Failed to create a bridge")
        }
        // time.Sleep(1 * time.Second)
    }

    // Test delete
    for i := 0; i < 10; i++ {
        brName := "ovsbr1" + fmt.Sprintf("%d", i)
        err := ovsDriver.DeleteBridge(brName)
        if (err != nil) {
            fmt.Printf("Error deleting the bridge. Err: %v", err)
            t.Errorf("Failed to delete a bridge")
        }
        // time.Sleep(1 * time.Second)
    }
}

*/

func TestCreatePort(t *testing.T) {
	// Connect to OVS
	ovsDriver := NewOvsDriver()

	// Create a port
	err := ovsDriver.CreatePort("port12", "internal", 11)
	if err != nil {
		fmt.Printf("Error creating the port. Err: %v", err)
		t.Errorf("Failed to create a port")
	}

	// HACK: wait a little so that interface is visible
	time.Sleep(time.Second * 1)

	contpid := 31936

	// Move the interface into a container namespace
	err = netutils.MoveIntfToNetns("port12", contpid)
	if err != nil {
		fmt.Printf("Error moving interface to container. Err %v\n", err)
	}

	// identity params
	identity := netutils.NetnsIntfIdentify{
		PortName:   "eth0",
		MacAddr:    "00:01:02:03:04:05",
		IPAddr:     "10.10.10.10",
		NetmaskLen: 24,
		DefaultGw:  "10.10.10.1",
	}

	// Set identity of the interface
	netutils.SetNetnsIntfIdentity(contpid, "port12", identity)
	if err != nil {
		fmt.Printf("Error setting interface identity. Err %v\n", err)
	}

	time.Sleep(time.Second * 1)

	ovsDriver.PrintCache()

	if ovsDriver.IsPortNamePresent("port12") {
		fmt.Printf("Interface exists\n")
	} else {
		fmt.Printf("Interface does not exist\n")
	}
}

func TestDeletePort(t *testing.T) {
	// Connect to OVS
	ovsDriver := NewOvsDriver()

	err := ovsDriver.DeletePort("port12")
	if err != nil {
		fmt.Printf("Error Deleting the port. Err: %v", err)
		t.Errorf("Failed to delete a port")
	}
}

func TestCreateVtep(t *testing.T) {
	// Connect to OVS
	ovsDriver := NewOvsDriver()

	// Create a port
	err := ovsDriver.CreateVtep("vtep1", "10.10.10.10")
	if err != nil {
		fmt.Printf("Error creating the VTEP. Err: %v", err)
		t.Errorf("Failed to create a port")
	}

	time.After(100 * time.Millisecond)

	isPresent, vtepName := ovsDriver.IsVtepPresent("10.10.10.10")
	if (!isPresent) || (vtepName != "vtep1") {
		t.Errorf("Unable to find the VTEP. present: %v, name: %s", isPresent, vtepName)
	}
}

func TestAddController(t *testing.T) {
	// Connect to OVS
	ovsDriver := NewOvsDriver()

	// Create a port
	err := ovsDriver.AddController("127.0.0.1", 6666)
	if err != nil {
		fmt.Printf("Error adding controller. Err: %v", err)
		t.Errorf("Failed to add controller")
	}
}
