package skydns2extension

import (
	"net/url"
	"strconv"
	"time"

	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/svcplugin/bridge"
	"github.com/coreos/etcd/client"
)

func init() {
	log.Debugf("Calling skydns2 init")
	bridge.Register(new(Factory), "etcd")
}

// Factory implementation to implement RegistryAdapter interface functions
type Factory struct{}

// New function to register Skydns2Adapter
func (f *Factory) New(uri *url.URL) bridge.RegistryAdapter {
	if uri.Host == "" {
		uri.Host = "localhost:2379"
	}

	etcdConfig := client.Config{
		Endpoints: []string{"http://" + uri.Host},
	}

	etcdClient, err := client.New(etcdConfig)
	if err != nil {
		log.Fatalf("Error creating etcd client. Err: %v", err)
	}

	return &Skydns2Adapter{client: client.NewKeysAPI(etcdClient)}
}

// Skydns2Adapter implements skydns2 client interface
type Skydns2Adapter struct {
	client client.KeysAPI
}

// Ping will try to connect to skydns2 by attempting to retrieve a key
func (r *Skydns2Adapter) Ping() error {
	_, err := r.client.Get(context.Background(), "/", &client.GetOptions{Recursive: true, Sort: true})
	if err != nil {
		return err
	}
	return nil
}

// Register will register Skydns2Adapter's interface with RegistryAdapter
func (r *Skydns2Adapter) Register(service *bridge.Service) error {
	port := strconv.Itoa(service.Port)
	record := `{"host":"` + service.IP + `","port":` + port + `}`
	ttlOpt := client.SetOptions{TTL: time.Duration(service.TTL) * time.Second}
	_, err := r.client.Set(context.Background(), r.servicePath(service), record, &ttlOpt)
	if err != nil {
		log.Errorf("skydns2: failed to register service: %s", err)
	}
	return err
}

// Deregister will deregister Skydns2Adapter's interface from RegistryAdapter
func (r *Skydns2Adapter) Deregister(service *bridge.Service) error {
	_, err := r.client.Delete(context.Background(), r.servicePath(service), nil)
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
