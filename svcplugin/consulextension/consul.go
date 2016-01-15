package consulextension

import (
	"fmt"
	"net/url"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/svcplugin/bridge"
	consulapi "github.com/hashicorp/consul/api"
)

// DefaultInterval time to perform health checks for services
const DefaultInterval = "10s"

// InitConsulAdapter registers as a new bridge
func init() {
	log.Debugf("Calling consul init")
	bridge.Register(new(Factory), "consul")
}

// Factory implementation to implement RegistryAdapter interface functions
type Factory struct{}

// New function to register ConsulAdapter
func (f *Factory) New(uri *url.URL) bridge.RegistryAdapter {
	config := consulapi.DefaultConfig()
	if uri.Host != "" {
		config.Address = uri.Host
	}
	client, err := consulapi.NewClient(config)
	if err != nil {
		log.Fatal("consul: ", uri.Scheme)
	}
	return &ConsulAdapter{client: client}
}

// ConsulAdapter implements consulapi client interface
type ConsulAdapter struct {
	client *consulapi.Client
}

// Ping will try to connect to consul by attempting to retrieve the current leader.
func (r *ConsulAdapter) Ping() error {
	status := r.client.Status()
	leader, err := status.Leader()
	if err != nil {
		return err
	}
	log.Infof("consul: current leader ", leader)

	return nil
}

// Register will register ConsulAdapter's interface with RegistryAdapter
func (r *ConsulAdapter) Register(service *bridge.Service) error {
	registration := new(consulapi.AgentServiceRegistration)
	registration.ID = service.ID
	registration.Name = service.Name
	registration.Port = service.Port
	registration.Tags = service.Tags
	registration.Address = service.IP
	registration.Check = r.buildCheck(service)
	return r.client.Agent().ServiceRegister(registration)
}

func (r *ConsulAdapter) buildCheck(service *bridge.Service) *consulapi.AgentServiceCheck {
	check := new(consulapi.AgentServiceCheck)
	if path := service.Attrs["check_http"]; path != "" {
		check.HTTP = fmt.Sprintf("http://%s:%d%s", service.IP, service.Port, path)
		if timeout := service.Attrs["check_timeout"]; timeout != "" {
			check.Timeout = timeout
		}
	} else if ttl := service.Attrs["check_ttl"]; ttl != "" {
		check.TTL = ttl
	} else {
		return nil
	}

	if check.Script != "" || check.HTTP != "" {
		if interval := service.Attrs["check_interval"]; interval != "" {
			check.Interval = interval
		} else {
			check.Interval = DefaultInterval
		}
	}
	return check
}

// Deregister will deregister ConsulAdapter's interface from RegistryAdapter
func (r *ConsulAdapter) Deregister(service *bridge.Service) error {
	return r.client.Agent().ServiceDeregister(service.ID)
}

// Refresh - do nothing for now
func (r *ConsulAdapter) Refresh(service *bridge.Service) error {
	return nil
}
