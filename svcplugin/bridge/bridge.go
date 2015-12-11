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
		RetryAttempts:   0,
		RetryInterval:   2000,
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

	log.Infof("Using", uri.Scheme, "service adapter:", uri)
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
				log.Errorf("Service refresh failed for %s. Error: %s",
					serviceID, err)
				continue
			}
		}
	}
}

// AddService adds a new service to registry when triggered from dockplugin
func (b *Bridge) AddService(srvID string, srvName string, nwName string, tenantName string, srvIP string) {
	b.Lock()
	defer b.Unlock()

	log.Infof("Called AddService for ", srvName)
	service := b.createService(srvID, srvName, nwName, tenantName, srvIP)
	err := b.registry.Register(service)
	if err != nil {
		log.Errorf("Service registration failed for %v. Error: %s",
			service, err)
	}
	b.services[srvName+nwName] = append(b.services[srvName+nwName], service)
}

// RemoveService removes service from registry when triggered from dockplugin
func (b *Bridge) RemoveService(srvID string, srvName string, nwName string, tenantName string, srvIP string) {
	b.Lock()
	defer b.Unlock()

	log.Infof("Called RemoveService for ", srvName)
	service := b.createService(srvID, srvName, nwName, tenantName, srvIP)
	err := b.registry.Deregister(service)
	if err != nil {
		log.Warningf("Service removal failed for service %v. Error: %s:",
			service, err)
	}
	delete(b.services, srvName+nwName)
}

func (b *Bridge) createService(srvID string, srvName string, nwName string, tenantName string, srvIP string) *Service {
	service := new(Service)
	service.ID = srvID
	service.Name = srvName
	service.Network = nwName
	service.Tenant = tenantName
	service.IP = srvIP
	service.TTL = b.config.RefreshTTL

	return service
}
