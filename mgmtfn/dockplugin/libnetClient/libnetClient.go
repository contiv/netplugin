package libnetClient

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/config"
	"github.com/docker/libnetwork/sandbox"
	"github.com/docker/libnetwork/types"
)

// networkResource is the body of the "get network" http response message
type networkResource struct {
	Name      string              `json:"name"`
	ID        string              `json:"id"`
	Type      string              `json:"type"`
	Endpoints []*endpointResource `json:"endpoints"`
}

// endpointResource is the body of the "get endpoint" http response message
type endpointResource struct {
	Name    string `json:"name"`
	ID      string `json:"id"`
	Network string `json:"network"`
}

// containerResource is the body of "get service backend" response message
type containerResource struct {
	ID string `json:"id"`
	// will add more fields once labels change is in
}

// endpointInfoResource is the body of the "get endpoint info" http response message
type endpointInfoResource struct {
	Interfaces []interfaceResource `json:"interfaces"`
	Gateway4   string              `json:"gateway"`
	Gateway6   string              `json:"gateway6"`
	Sandbox    string              `json:"sandbox"`
}

type interfaceResource struct {
	MAC   string `json:"mac"`
	Addr  string `json:"addr"`
	Addr6 string `json:"addr6"`
}

/***********
  Body types
  ************/

// networkCreate is the expected body of the "create network" http request message
type networkCreate struct {
	Name        string                 `json:"name"`
	NetworkType string                 `json:"network_type"`
	Options     map[string]interface{} `json:"options"`
}

// endpointCreate represents the body of the "create endpoint" http request message
type endpointCreate struct {
	Name         string                `json:"name"`
	ExposedPorts []types.TransportPort `json:"exposed_ports"`
	PortMapping  []types.PortBinding   `json:"port_mapping"`
}

// endpointJoin represents the expected body of the "join endpoint" or "leave endpoint" http request messages
type endpointJoin struct {
	ContainerID       string                 `json:"container_id"`
	HostName          string                 `json:"host_name"`
	DomainName        string                 `json:"domain_name"`
	HostsPath         string                 `json:"hosts_path"`
	ResolvConfPath    string                 `json:"resolv_conf_path"`
	DNS               []string               `json:"dns"`
	ExtraHosts        []endpointExtraHost    `json:"extra_hosts"`
	ParentUpdates     []endpointParentUpdate `json:"parent_updates"`
	UseDefaultSandbox bool                   `json:"use_default_sandbox"`
}

// servicePublish represents the body of the "publish service" http request message
type servicePublish struct {
	Name         string                `json:"name"`
	Network      string                `json:"network_name"`
	ExposedPorts []types.TransportPort `json:"exposed_ports"`
	PortMapping  []types.PortBinding   `json:"port_mapping"`
}

// EndpointExtraHost represents the extra host object
type endpointExtraHost struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

// EndpointParentUpdate is the object carrying the information about the
// endpoint parent that needs to be updated
type endpointParentUpdate struct {
	EndpointID string `json:"endpoint_id"`
	Name       string `json:"name"`
	Address    string `json:"address"`
}

// ErrNotImplemented is returned for function that are not implemented
var ErrNotImplemented = errors.New("method not implemented")

// Implements NetworkController by means of HTTP api
type remoteController struct {
	baseURL *url.URL
	client  *http.Client
}

// NewRemoteAPI returns an object implementing NetworkController
// interface that forwards the requests to the daemon
func NewRemoteAPI(daemonUrl string) libnetwork.NetworkController {
	if daemonUrl == "" {
		daemonUrl = "unix:///var/run/docker.sock"
	}
	url, err := url.Parse(daemonUrl)
	if err != nil {
		return nil
	}

	if url.Scheme == "" || url.Scheme == "tcp" {
		url.Scheme = "http"
	}
	return &remoteController{
		baseURL: url,
		client:  newHTTPClient(url, nil),
	}
}

// ConfigureNetworkDriver applies the passed options to the driver instance for the specified network type
func (c *remoteController) ConfigureNetworkDriver(networkType string, options map[string]interface{}) error {
	return ErrNotImplemented
}

// Config method returns the bootup configuration for the controller
func (c *remoteController) Config() config.Config {
	panic("remoteController does not implement Config()")
}

// Create a new network. The options parameter carries network specific options.
// Labels support will be added in the near future.
func (c *remoteController) NewNetwork(networkType, name string, options ...libnetwork.NetworkOption) (libnetwork.Network, error) {
	url := "/networks"

	// TODO: handle options somehow
	create := networkCreate{
		Name:        name,
		NetworkType: networkType,
	}

	nid := ""
	if err := c.httpPost(url, create, &nid); err != nil {
		return nil, fmt.Errorf("http error: %v", err)
	}

	return &remoteNetwork{
		rc: c,
		nr: networkResource{
			Name: name,
			ID:   nid,
			Type: networkType,
		},
	}, nil
}

// Networks returns the list of Network(s) managed by this controller.
func (c *remoteController) Networks() []libnetwork.Network {
	nrs := []networkResource{}
	if err := c.httpGet("/networks", &nrs); err != nil {
		return nil
	}

	ns := []libnetwork.Network{}
	for _, nr := range nrs {
		ns = append(ns, &remoteNetwork{c, nr})
	}
	return ns
}

// WalkNetworks uses the provided function to walk the Network(s) managed by this controller.
func (c *remoteController) WalkNetworks(walker libnetwork.NetworkWalker) {
	for _, n := range c.Networks() {
		if walker(n) {
			return
		}
	}
}

// NetworkByName returns the Network which has the passed name. If not found, the error ErrNoSuchNetwork is returned.
func (c *remoteController) NetworkByName(name string) (libnetwork.Network, error) {
	ns := []networkResource{}
	if err := c.httpGet("/networks?name="+name, &ns); err != nil {
		return nil, err
	}

	if len(ns) == 0 {
		return nil, libnetwork.ErrNoSuchNetwork(name)
	}
	return &remoteNetwork{c, ns[0]}, nil
}

// NetworkByID returns the Network which has the passed id. If not found, the error ErrNoSuchNetwork is returned.
func (c *remoteController) NetworkByID(id string) (libnetwork.Network, error) {
	n := networkResource{}
	if err := c.httpGet(path.Join("/networks", id), &n); err != nil {
		if se, ok := err.(httpStatusErr); ok && se.Code() == http.StatusNotFound {
			return nil, libnetwork.ErrNoSuchNetwork(id)
		}
		return nil, err
	}

	return &remoteNetwork{c, n}, nil
}

// GC triggers immediate garbage collection of resources which are garbage collected.
func (c *remoteController) GC() {
	panic("remoteController does not implement GC()")
}

// LeaveAll accepts a container id and attempts to leave all endpoints that the container has joined
func (c *remoteController) LeaveAll(id string) error {
	return ErrNotImplemented
}

type remoteNetwork struct {
	rc *remoteController
	nr networkResource
}

// A user chosen name for this network.
func (n *remoteNetwork) Name() string {
	return n.nr.Name
}

// A system generated id for this network.
func (n *remoteNetwork) ID() string {
	return n.nr.ID
}

// The type of network, which corresponds to its managing driver.
func (n *remoteNetwork) Type() string {
	return n.nr.Type
}

// Create a new endpoint to this network symbolically identified by the
// specified unique name. The options parameter carry driver specific options.
// Labels support will be added in the near future.
func (n *remoteNetwork) CreateEndpoint(name string, options ...libnetwork.EndpointOption) (libnetwork.Endpoint, error) {
	url := path.Join("/networks", n.nr.ID, "endpoints")
	// TODO: process options somehow
	create := endpointCreate{
		Name: name,
	}

	eid := ""
	if err := n.rc.httpPost(url, create, &eid); err != nil {
		return nil, err
	}

	return &remoteEndpoint{
		rc:        n.rc,
		networkID: n.nr.ID,
		er: endpointResource{
			Name:    name,
			ID:      eid,
			Network: n.nr.Name,
		},
	}, nil
}

// Delete the network.
func (n *remoteNetwork) Delete() error {
	url := path.Join("/networks", n.nr.ID)
	return n.rc.httpDelete(url)
}

// Endpoints returns the list of Endpoint(s) in this network.
func (n *remoteNetwork) Endpoints() []libnetwork.Endpoint {
	endpoints := make([]libnetwork.Endpoint, 0, len(n.nr.Endpoints))
	for _, er := range n.nr.Endpoints {
		endpoints = append(endpoints, &remoteEndpoint{
			rc:        n.rc,
			er:        *er,
			networkID: n.nr.ID,
		})
	}
	return endpoints
}

// WalkEndpoints uses the provided function to walk the Endpoints
func (n *remoteNetwork) WalkEndpoints(walker libnetwork.EndpointWalker) {
	for _, e := range n.Endpoints() {
		if walker(e) {
			return
		}
	}
}

// EndpointByName returns the Endpoint which has the passed name. If not found, the error ErrNoSuchEndpoint is returned.
func (n *remoteNetwork) EndpointByName(name string) (libnetwork.Endpoint, error) {
	// TODO: should this make an RPC
	for _, er := range n.nr.Endpoints {
		if er.Name == name {
			return &remoteEndpoint{
				rc:        n.rc,
				er:        *er,
				networkID: n.nr.ID,
			}, nil
		}
	}
	return nil, libnetwork.ErrNoSuchEndpoint(name)
}

// EndpointByID returns the Endpoint which has the passed id. If not found, the error ErrNoSuchEndpoint is returned.
func (n *remoteNetwork) EndpointByID(id string) (libnetwork.Endpoint, error) {
	// TODO: should this make an RPC
	for _, er := range n.nr.Endpoints {
		if er.ID == id {
			return &remoteEndpoint{
				rc:        n.rc,
				er:        *er,
				networkID: n.nr.ID,
			}, nil
		}
	}
	return nil, libnetwork.ErrNoSuchEndpoint(id)
}

type remoteEndpoint struct {
	rc        *remoteController
	networkID string
	er        endpointResource
}

// A system generated id for this endpoint.
func (e *remoteEndpoint) ID() string {
	return e.er.ID
}

// Name returns the name of this endpoint.
func (e *remoteEndpoint) Name() string {
	return e.er.Name
}

// Network returns the name of the network to which this endpoint is attached.
func (e *remoteEndpoint) Network() string {
	return e.er.Network
}

// Join creates a new sandbox for the given container ID and populates the
// network resources allocated for the endpoint and joins the sandbox to
// the endpoint. It returns the sandbox key to the caller
func (e *remoteEndpoint) Join(containerID string, options ...libnetwork.EndpointOption) error {
	url := path.Join("/networks", e.networkID, "endpoints", e.er.ID, "containers")

	//TODO: process options somehow
	join := endpointJoin{
		ContainerID: containerID,
	}

	sk := ""
	return e.rc.httpPost(url, join, &sk)
}

// Leave removes the sandbox associated with  container ID and detaches
// the network resources populated in the sandbox
func (e *remoteEndpoint) Leave(containerID string, options ...libnetwork.EndpointOption) error {
	url := path.Join("/networks", e.networkID, "endpoints", e.er.ID, "containers", containerID)
	return e.rc.httpDelete(url)
}

// Return certain operational data belonging to this endpoint
func (e *remoteEndpoint) Info() libnetwork.EndpointInfo {
	url := path.Join("/networks", e.networkID, "endpoints", e.er.ID, "info")
	eir := &endpointInfoResource{}
	if err := e.rc.httpGet(url, eir); err != nil {
		return nil
	}

	return (*remoteEndpointInfo)(eir)
}

// DriverInfo returns a collection of driver operational data related to this endpoint retrieved from the driver
func (e *remoteEndpoint) DriverInfo() (map[string]interface{}, error) {
	return nil, ErrNotImplemented
}

// ContainerInfo returns the info available at the endpoint about the attached container
func (e *remoteEndpoint) ContainerInfo() libnetwork.ContainerInfo {
	return nil
}

// Delete and detaches this endpoint from the network.
func (e *remoteEndpoint) Delete() error {
	url := path.Join("/networks", e.networkID, "endpoints", e.er.ID)
	return e.rc.httpDelete(url)
}

// Retrieve the interfaces' statistics from the sandbox
func (e *remoteEndpoint) Statistics() (map[string]*sandbox.InterfaceStatistics, error) {
	return nil, ErrNotImplemented
}

type remoteEndpointInfo endpointInfoResource

// InterfaceList returns an interface list which were assigned to the endpoint
// by the driver. This can be used after the endpoint has been created.
func (ei *remoteEndpointInfo) InterfaceList() []libnetwork.InterfaceInfo {
	iis := []libnetwork.InterfaceInfo{}
	for _, ii := range ei.Interfaces {
		iis = append(iis, (*remoteInterfaceInfo)(&ii))
	}
	return iis
}

// Gateway returns the IPv4 gateway assigned by the driver.
// This will only return a valid value if a container has joined the endpoint.
func (ei *remoteEndpointInfo) Gateway() net.IP {
	return net.ParseIP(ei.Gateway4)
}

// GatewayIPv6 returns the IPv6 gateway assigned by the driver.
// This will only return a valid value if a container has joined the endpoint.
func (ei *remoteEndpointInfo) GatewayIPv6() net.IP {
	return net.ParseIP(ei.Gateway6)
}

// SandboxKey returns the sanbox key for the container which has joined
// the endpoint. If there is no container joined then this will return an
// empty string.
func (ei *remoteEndpointInfo) SandboxKey() string {
	return ei.Sandbox
}

type remoteInterfaceInfo interfaceResource

// MacAddress returns the MAC address assigned to the endpoint.
func (ii *remoteInterfaceInfo) MacAddress() net.HardwareAddr {
	if mac, err := net.ParseMAC(ii.MAC); err == nil {
		return mac
	}
	return nil
}

// Address returns the IPv4 address assigned to the endpoint.
func (ii *remoteInterfaceInfo) Address() net.IPNet {
	ip, ipn, err := net.ParseCIDR(ii.Addr)
	if err != nil || ip.To4() == nil {
		return net.IPNet{}
	}
	ipn.IP = ip
	return *ipn
}

// AddressIPv6 returns the IPv6 address assigned to the endpoint.
func (ii *remoteInterfaceInfo) AddressIPv6() net.IPNet {
	ip, ipn, err := net.ParseCIDR(ii.Addr)
	if err != nil || ip.To16() == nil {
		return net.IPNet{}
	}
	ipn.IP = ip
	return *ipn
}

const defaultTimeout = 30 * time.Second

func newHTTPClient(u *url.URL, tlsConfig *tls.Config) *http.Client {
	timeout := time.Duration(defaultTimeout)
	httpTransport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	switch u.Scheme {
	default:
		httpTransport.Dial = func(proto, addr string) (net.Conn, error) {
			return net.DialTimeout(proto, addr, timeout)
		}
	case "unix":
		socketPath := u.Path
		unixDial := func(proto, addr string) (net.Conn, error) {
			return net.DialTimeout("unix", socketPath, timeout)
		}
		httpTransport.Dial = unixDial
		// Override the main URL object so the HTTP lib won't complain
		u.Scheme = "http"
		u.Host = "unix.sock"
		u.Path = ""
	}
	return &http.Client{Transport: httpTransport}
}

type httpStatusErr int

func (se httpStatusErr) Error() string {
	return fmt.Sprintf("HTTP status error: %v", int(se))
}

func (se httpStatusErr) Code() int {
	return int(se)
}

func httpErr(resp *http.Response) error {
	msg, _ := ioutil.ReadAll(resp.Body)
	return fmt.Errorf("http status error: %v: %v", resp.StatusCode, string(msg))
}

func (rc *remoteController) httpGet(path string, res interface{}) error {
	req, err := http.NewRequest("GET", rc.baseURL.String()+path, nil)
	if err != nil {
		return err
	}
	resp, err := rc.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return httpErr(resp)
	}

	return json.NewDecoder(resp.Body).Decode(res)
}

func (rc *remoteController) httpPost(path string, body interface{}, res interface{}) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", rc.baseURL.String()+path, bytes.NewBuffer(encoded))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := rc.client.Do(req)
	// resp, err := rc.client.Post(url, "application/json", bytes.NewBuffer(encoded))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return httpErr(resp)
	}

	return json.NewDecoder(resp.Body).Decode(res)
}

func (rc *remoteController) httpDelete(path string) error {
	req, err := http.NewRequest("DELETE", rc.baseURL.String()+path, nil)
	if err != nil {
		return err
	}
	resp, err := rc.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return httpErr(resp)
	}

	return nil
}
