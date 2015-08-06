package objdb

import (
	"sync"

	log "github.com/Sirupsen/logrus"
)

// Lock event types
const (
	LockAcquired       = iota // Successfully acquired
	LockReleased              // explicitly released
	LockAcquireTimeout        // Timeout trying to acquire lock
	LockAcquireError          // Error while acquiring
	LockRefreshError          // Error during ttl refresh
	LockLost                  // We lost the lock
)

// Lock Event notifications
type LockEvent struct {
	EventType uint // Type of event
}

// Lock interface
type LockInterface interface {
	// Acquire a lock.
	// Give up acquiring lock after timeout seconds. if timeout is 0, wait forever
	Acquire(timeout uint64) error

	// Release the lock. This explicitly releases the lock and cleans up all state
	// If we were still waiting to acquire lock, it stops it and cleans up
	Release() error

	// For debug purposes only.
	// Just stops refreshing the lock
	Kill() error

	// Get the event channel
	EventChan() <-chan LockEvent

	// Is the lock acquired
	IsAcquired() bool

	// Get current holder of the lock
	GetHolder() string
}

// Information about a service
// Notes:
//      There could be multiple instances of a service. hostname:port uniquely
//      identify an instance of a service
type ServiceInfo struct {
	ServiceName string // Name of the service
	HostAddr    string // Host name or IP address where its running
	Port        int    // Port number where its listening
}

const (
	WatchServiceEventAdd   = iota // New Service endpoint added
	WatchServiceEventDel          // A service endpoint was deleted
	WatchServiceEventError        // Error occurred while watching for service
)

type WatchServiceEvent struct {
	EventType   uint        // event type
	ServiceInfo ServiceInfo // Information about the service
}

// Plugin API
type ObjdbApi interface {
	// Initialize the plugin, only called once
	Init(seedHosts []string) error

	// Return local address used by conf store
	GetLocalAddr() (string, error)

	// Get a Key from conf store
	GetObj(key string, retValue interface{}) error

	// Set a key in conf store
	SetObj(key string, value interface{}) error

	// Remove an object
	DelObj(key string) error

	// List all objects in a directory
	ListDir(key string) ([]string, error)

	// Create a new lock
	NewLock(name string, holderId string, ttl uint64) (LockInterface, error)

	// Register a service
	// Service is registered with a ttl for 60sec and a goroutine is created
	// to refresh the ttl.
	RegisterService(serviceInfo ServiceInfo) error

	// List all end points for a service
	GetService(name string) ([]ServiceInfo, error)

	// Watch for addition/deletion of service end points
	WatchService(name string, eventCh chan WatchServiceEvent, stopCh chan bool) error

	// Deregister a service
	// This removes the service from the registry and stops the refresh groutine
	DeregisterService(serviceInfo ServiceInfo) error
}

var (
	// List of plugins available
	pluginList  = make(map[string]ObjdbApi)
	pluginMutex = new(sync.Mutex)
)

// Register a plugin
func RegisterPlugin(name string, plugin ObjdbApi) error {
	pluginMutex.Lock()
	defer pluginMutex.Unlock()
	pluginList[name] = plugin

	return nil
}

// Return the plugin by name
func GetPlugin(name string) ObjdbApi {
	// Find the conf store
	pluginMutex.Lock()
	defer pluginMutex.Unlock()
	if pluginList[name] == nil {
		log.Errorf("Confstore Plugin %s not registered", name)
		log.Fatal("Confstore plugin not registered")
	}

	return pluginList[name]
}
