package bridge

import (
	"errors"
	"log"
	"net"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"

	dockerapi "github.com/fsouza/go-dockerclient"
)

// Bridge maintains consolidated information to process Service registering
type Bridge struct {
	sync.Mutex
	registry       RegistryAdapter
	docker         *dockerapi.Client
	services       map[string][]*Service
	deadContainers map[string]*DeadContainer
	config         Config
}

// New creates a new bridge between the Registry providers and containers
func New(docker *dockerapi.Client, adapterURI string, config Config) (*Bridge, error) {
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
		docker:         docker,
		config:         config,
		registry:       factory.New(uri),
		services:       make(map[string][]*Service),
		deadContainers: make(map[string]*DeadContainer),
	}, nil
}

// Ping will try to connect to the registry
func (b *Bridge) Ping() error {
	return b.registry.Ping()
}

// Add will add the containers information to service registry
func (b *Bridge) Add(containerID string) {
	b.Lock()
	defer b.Unlock()
	b.add(containerID, false)
}

// Remove will remove the service entry from the registry
func (b *Bridge) Remove(containerID string) {
	b.remove(containerID, true)
}

// RemoveOnExit acts when a container dies and removes the service
func (b *Bridge) RemoveOnExit(containerID string) {
	b.remove(containerID, b.config.DeregisterCheck == "always" || b.didExitCleanly(containerID))
}

// Refresh will keep the registry uptodate by monitoring for any changes
func (b *Bridge) Refresh() {
	b.Lock()
	defer b.Unlock()

	for containerID, deadContainer := range b.deadContainers {
		deadContainer.TTL -= b.config.RefreshInterval
		if deadContainer.TTL <= 0 {
			delete(b.deadContainers, containerID)
		}
	}

	for containerID, services := range b.services {
		for _, service := range services {
			err := b.registry.Refresh(service)
			if err != nil {
				log.Println("refresh failed:", service.ID, err)
				continue
			}
			log.Println("refreshed:", containerID[:12], service.ID)
		}
	}
}

// Sync will resynchronize services
func (b *Bridge) Sync(quiet bool) {
	b.Lock()
	defer b.Unlock()

	containers, err := b.docker.ListContainers(dockerapi.ListContainersOptions{})
	if err != nil && quiet {
		log.Println("error listing containers, skipping sync")
		return
	} else if err != nil && !quiet {
		log.Fatal(err)
	}

	log.Printf("Syncing services on %d containers", len(containers))

	// NOTE: This assumes reregistering will do the right thing, i.e. nothing.
	// NOTE: This will NOT remove services.
	for _, listing := range containers {
		services := b.services[listing.ID]
		if services == nil {
			b.add(listing.ID, quiet)
		} else {
			for _, service := range services {
				err := b.registry.Register(service)
				if err != nil {
					log.Println("sync register failed:", service, err)
				}
			}
		}
	}
}

func (b *Bridge) add(containerID string, quiet bool) {
	if d := b.deadContainers[containerID]; d != nil {
		b.services[containerID] = d.Services
		delete(b.deadContainers, containerID)
	}

	if b.services[containerID] != nil {
		log.Println("container, ", containerID[:12], ", already exists, ignoring")
		// Alternatively, remove and readd or resubmit.
		return
	}

	container, err := b.docker.InspectContainer(containerID)
	if err != nil {
		log.Println("unable to inspect container:", containerID[:12], err)
		return
	}

	ports := make(map[string]ServicePort)

	// Extract configured host port mappings, relevant when using --net=host
	for port, published := range container.HostConfig.PortBindings {
		ports[string(port)] = servicePort(container, port, published)
	}

	// Extract runtime port mappings, relevant when using --net=bridge
	for port, published := range container.NetworkSettings.Ports {
		ports[string(port)] = servicePort(container, port, published)
	}

	if len(ports) == 0 && !quiet {
		log.Println("ignored:", container.ID[:12], "no published ports")
		return
	}

	for _, port := range ports {
		if b.config.Internal != true && port.HostPort == "" {
			if !quiet {
				log.Println("ignored:", container.ID[:12], "port", port.ExposedPort, "not published on host")
			}
			continue
		}
		service := b.newService(port, len(ports) > 1)
		if service == nil {
			if !quiet {
				log.Println("ignored:", container.ID[:12], "service on port", port.ExposedPort)
			}
			continue
		}
		err := b.registry.Register(service)
		if err != nil {
			log.Println("register failed:", service, err)
			continue
		}
		b.services[container.ID] = append(b.services[container.ID], service)
		log.Println("added:", container.ID[:12], service.ID)
	}
}

func (b *Bridge) newService(port ServicePort, isgroup bool) *Service {
	container := port.container
	defaultName := strings.Split(path.Base(container.Config.Image), ":")[0]

	// not sure about this logic. kind of want to remove it.
	hostname, err := os.Hostname()
	if err != nil {
		hostname = port.HostIP
	} else {
		if port.HostIP == "0.0.0.0" {
			ip, err := net.ResolveIPAddr("ip", hostname)
			if err == nil {
				port.HostIP = ip.String()
			}
		}
	}

	if b.config.HostIP != "" {
		port.HostIP = b.config.HostIP
	}

	metadata := serviceMetaData(container.Config, port.ExposedPort)

	ignore := mapDefault(metadata, "ignore", "")
	if ignore != "" {
		return nil
	}

	service := new(Service)
	service.Origin = port
	service.ID = hostname + ":" + container.Name[1:] + ":" + port.ExposedPort
	service.Name = mapDefault(metadata, "name", defaultName)
	if isgroup {
		service.Name += "-" + port.ExposedPort
	}
	var p int
	if b.config.Internal == true {
		service.IP = port.ExposedIP
		p, _ = strconv.Atoi(port.ExposedPort)
	} else {
		service.IP = port.HostIP
		p, _ = strconv.Atoi(port.HostPort)
	}
	service.Port = p

	if port.PortType == "udp" {
		service.Tags = combineTags(
			mapDefault(metadata, "tags", ""), b.config.ForceTags, "udp")
		service.ID = service.ID + ":udp"
	} else {
		service.Tags = combineTags(
			mapDefault(metadata, "tags", ""), b.config.ForceTags)
	}

	id := mapDefault(metadata, "id", "")
	if id != "" {
		service.ID = id
	}

	delete(metadata, "id")
	delete(metadata, "tags")
	delete(metadata, "name")
	service.Attrs = metadata
	service.TTL = b.config.RefreshTTL

	return service
}

func (b *Bridge) remove(containerID string, deregister bool) {
	b.Lock()
	defer b.Unlock()

	if deregister {
		deregisterAll := func(services []*Service) {
			for _, service := range services {
				err := b.registry.Deregister(service)
				if err != nil {
					log.Println("deregister failed:", service.ID, err)
					continue
				}
				log.Println("removed:", containerID[:12], service.ID)
			}
		}
		deregisterAll(b.services[containerID])
		if d := b.deadContainers[containerID]; d != nil {
			deregisterAll(d.Services)
			delete(b.deadContainers, containerID)
		}
	} else if b.config.RefreshTTL != 0 && b.services[containerID] != nil {
		// need to stop the refreshing, but can't delete it yet
		b.deadContainers[containerID] = &DeadContainer{b.config.RefreshTTL, b.services[containerID]}
	}
	delete(b.services, containerID)
}

func (b *Bridge) didExitCleanly(containerID string) bool {
	container, err := b.docker.InspectContainer(containerID)
	if _, ok := err.(*dockerapi.NoSuchContainer); ok {
		// the container has already been removed from Docker
		// e.g. probabably run with "--rm" to remove immediately
		// so its exit code is not accessible
		log.Printf("registrator: container %v was removed, could not fetch exit code", containerID[:12])
		return true
	} else if err != nil {
		log.Printf("registrator: error fetching status for container %v on \"die\" event: %v\n", containerID[:12], err)
		return false
	}
	return !container.State.Running && container.State.ExitCode == 0
}
