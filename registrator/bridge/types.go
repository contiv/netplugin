package bridge

//go:generate go-extpoints . AdapterFactory

import (
	"net/url"

	dockerapi "github.com/fsouza/go-dockerclient"
)

// AdapterFactory implements the ServiceRegistrator
type AdapterFactory interface {
	New(uri *url.URL) RegistryAdapter
}

// RegistryAdapter defines the interface that ServiceRestrators must implement
type RegistryAdapter interface {
	Ping() error
	Register(service *Service) error
	Deregister(service *Service) error
	Refresh(service *Service) error
}

// Config maintains the supplementing state of the service
type Config struct {
	HostIP          string
	Internal        bool
	ForceTags       string
	RefreshTTL      int
	RefreshInterval int
	DeregisterCheck string
}

// Service defines the state required for the configuration on the service
type Service struct {
	ID     string
	Name   string
	Tenant string
	Port   int
	IP     string
	Tags   []string
	Attrs  map[string]string
	TTL    int

	Origin ServicePort
}

// DeadContainer maintains the containers that are killed
type DeadContainer struct {
	TTL      int
	Services []*Service
}

// ServicePort maintains the port level service details exposed by a container
type ServicePort struct {
	HostPort          string
	HostIP            string
	ExposedPort       string
	ExposedIP         string
	PortType          string
	ContainerHostname string
	ContainerID       string
	container         *dockerapi.Container
}
