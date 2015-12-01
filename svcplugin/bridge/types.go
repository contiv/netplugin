package bridge

//go:generate go-extpoints . AdapterFactory

import (
	"net/url"
)

// AdapterFactory implements the ServiceRegistrator
type AdapterFactory interface {
	New(uri *url.URL) RegistryAdapter
}

// RegistryAdapter defines the interface that ServiceRegistrators must implement
type RegistryAdapter interface {
	Ping() error
	Register(service *Service) error
	Deregister(service *Service) error
	Refresh(service *Service) error
}

// Config maintains the supplementing state of the service
/* Field descriptions:
 * HostIP:          IP for ports mapped to the host
 * Internal:        Use internal ports instead of published ones
 * ForceTags:       Append tags for all registered services
 * RefreshTTL:      TTL for services (default is no expiry)
 * RefreshInterval: Frequency with which service TTLs are refreshed
 * DeregisterCheck: Deregister exited services "always" or "on-success"
 * RetryAttempts:   Max retry attempts to establish a connection with the
 *                  backend. Use -1 for infinite retries
 * RetryInterval:   Interval (in millisecond) between retry-attempts
 */
type Config struct {
	HostIP          string
	Internal        bool
	ForceTags       string
	RefreshTTL      int
	RefreshInterval int
	DeregisterCheck string
	RetryInterval   int
	RetryAttempts   int
}

// Service defines the state required for the configuration on the service
type Service struct {
	ID      string
	Name    string
	Network string
	Tenant  string
	Port    int
	IP      string
	Tags    []string
	Attrs   map[string]string
	TTL     int
}
