package skydns2extension

import (
	"net/url"
	"strconv"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/svcplugin/bridge"
	"github.com/coreos/go-etcd/etcd"
)

func init() {
	log.Infof("Calling skydns2 init")
	bridge.Register(new(Factory), "skydns2")
}

// Factory implementation to implement RegistryAdapter interface functions
type Factory struct{}

// New function to register Skydns2Adapter
func (f *Factory) New(uri *url.URL) bridge.RegistryAdapter {
	var urls []string

	// Default SkyDNS config
	uri, _ = url.ParseRequestURI("http://127.0.0.1:4001")

	if uri.Host != "" {
		urls = append(urls, "http://"+uri.Host)
	}

	return &Skydns2Adapter{client: etcd.NewClient(urls)}
}

// Skydns2Adapter implements skydns2 client interface
type Skydns2Adapter struct {
	client *etcd.Client
}

// Ping will try to connect to skydns2 by attempting to retrieve the current leader
func (r *Skydns2Adapter) Ping() error {
	rr := etcd.NewRawRequest("GET", "version", nil, nil)
	_, err := r.client.SendRequest(rr)
	if err != nil {
		return err
	}
	return nil
}

// Register will register Skydns2Adapter's interface with RegistryAdapter
func (r *Skydns2Adapter) Register(service *bridge.Service) error {
	port := strconv.Itoa(service.Port)
	record := `{"host":"` + service.IP + `","port":` + port + `}`
	_, err := r.client.Set(r.servicePath(service), record, uint64(service.TTL))
	if err != nil {
		log.Errorf("skydns2: failed to register service: %s", err)
	}
	return err
}

// Deregister will deregister Skydns2Adapter's interface from RegistryAdapter
func (r *Skydns2Adapter) Deregister(service *bridge.Service) error {
	_, err := r.client.Delete(r.servicePath(service), false)
	if err != nil {
		log.Warningf("skydns2: failed to deregister service: %s", err)
	}
	return err
}

// Refresh registers any pending services
func (r *Skydns2Adapter) Refresh(service *bridge.Service) error {
	return r.Register(service)
}

func (r *Skydns2Adapter) servicePath(service *bridge.Service) string {
	return "/skydns/" + service.Tenant + "/" + service.Network + "/" + service.Name + "/" + service.ID
}
