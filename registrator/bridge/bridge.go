package bridge

import (
	"errors"
	"net/url"
	"sync"

	log "github.com/Sirupsen/logrus"
)

// Bridge maintains consolidated information to process Service registering
type Bridge struct {
	sync.Mutex
	registry RegistryAdapter
	services map[string][]*Service
	config   Config
}

// DefaultBridgeConfig sets the default configuration for Bridge config
func DefaultBridgeConfig() Config {
	return Config{
		HostIP:          "",
		Internal:        false,
		ForceTags:       "",
		RefreshTTL:      0,
		RefreshInterval: 0,
		DeregisterCheck: "always",
	}
}

// New creates a new bridge between the Registry providers and containers
func New(adapterURI string, config Config) (*Bridge, error) {
	uri, err := url.Parse(adapterURI)
	if err != nil {
		return nil, errors.New("bad adapter uri: " + adapterURI)
	}
	factory, found := AdapterFactories.Lookup(uri.Scheme)
	if !found {
		return nil, errors.New("unrecognized adapter: " + adapterURI)
	}

	log.Println("Using", uri.Scheme, "adapter:", uri)
	return &Bridge{
		config:   config,
		registry: factory.New(uri),
		services: make(map[string][]*Service),
	}, nil
}

// Ping will try to connect to the registry
func (b *Bridge) Ping() error {
	return b.registry.Ping()
}

// Refresh will keep the registry uptodate by monitoring for any changes
func (b *Bridge) Refresh() {
	b.Lock()
	defer b.Unlock()

	for serviceID, services := range b.services {
		for _, service := range services {
			err := b.registry.Refresh(service)
			if err != nil {
				log.Println("refresh failed:", service.ID, err)
				continue
			}
			log.Println("refreshed:", serviceID)
		}
	}
}

// AddService adds a new service to registry when triggered from dockplugin
func (b *Bridge) AddService(epName string, serviceName string, epIP string) {
	b.Lock()
	defer b.Unlock()

	log.Println("Called AddService for ", epName)
	service := b.createService(epName, serviceName, epIP)
	err := b.registry.Register(service)
	if err != nil {
		log.Println("service registration failed:", service, err)
	}
	b.services[epName+serviceName] = append(b.services[epName+serviceName], service)
}

// RemoveService removes service from registry when triggered from dockplugin
func (b *Bridge) RemoveService(epName string, serviceName string, epIP string) {
	b.Lock()
	defer b.Unlock()

	log.Println("Called RemoveService for ", epName)
	service := b.createService(epName, serviceName, epIP)
	err := b.registry.Deregister(service)
	if err != nil {
		log.Println("service removal failed:", service, err)
	}
	delete(b.services, epName+serviceName)
}

func (b *Bridge) createService(epName string, serviceName string, epIP string) *Service {
	service := new(Service)
	service.ID = epName
	service.Name = serviceName
	service.IP = epIP
	service.TTL = b.config.RefreshTTL

	return service
}
