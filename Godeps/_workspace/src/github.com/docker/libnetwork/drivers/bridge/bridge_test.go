package bridge

import (
	"bytes"
	"fmt"
	"net"
	"regexp"
	"testing"

	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/iptables"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/netutils"
	"github.com/docker/libnetwork/types"
	"github.com/vishvananda/netlink"
)

func TestCreateFullOptions(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	d := newDriver()

	config := &configuration{
		EnableIPForwarding: true,
		EnableIPTables:     true,
	}

	// Test this scenario: Default gw address does not belong to
	// container network and it's greater than bridge address
	cip, cnw, _ := net.ParseCIDR("172.16.122.0/24")
	cnw.IP = cip
	ip, nw, _ := net.ParseCIDR("172.16.0.10/16")
	nw.IP = ip
	gw := net.ParseIP("172.16.0.1")

	netConfig := &networkConfiguration{
		BridgeName:         DefaultBridgeName,
		AddressIPv4:        nw,
		FixedCIDR:          cnw,
		DefaultGatewayIPv4: gw,
		EnableIPv6:         true,
	}
	_, netConfig.FixedCIDRv6, _ = net.ParseCIDR("2001:db8::/48")
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = config

	if err := d.Config(genericOption); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	netOption := make(map[string]interface{})
	netOption[netlabel.GenericData] = netConfig

	err := d.CreateNetwork("dummy", netOption)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	// Verify the IP address allocated for the endpoint belongs to the container network
	epOptions := make(map[string]interface{})
	te := &testEndpoint{ifaces: []*testInterface{}}
	err = d.CreateEndpoint("dummy", "ep1", te, epOptions)
	if err != nil {
		t.Fatalf("Failed to create an endpoint : %s", err.Error())
	}
	if !cnw.Contains(te.Interfaces()[0].Address().IP) {
		t.Fatalf("endpoint got assigned address outside of container network(%s): %s", cnw.String(), te.Interfaces()[0].Address())
	}
}

func TestCreate(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	d := newDriver()

	if err := d.Config(nil); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	netconfig := &networkConfiguration{BridgeName: DefaultBridgeName}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = netconfig

	if err := d.CreateNetwork("dummy", genericOption); err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	err := d.CreateNetwork("dummy", genericOption)
	if err == nil {
		t.Fatalf("Expected bridge driver to refuse creation of second network with default name")
	}
	if _, ok := err.(types.ForbiddenError); !ok {
		t.Fatalf("Creation of second network with default name failed with unexpected error type")
	}

	err = d.DeleteNetwork("dummy")
	if err == nil {
		t.Fatalf("deletion of network with default name should fail on this driver")
	}
	if _, ok := err.(types.ForbiddenError); !ok {
		t.Fatalf("deletion of network with default name failed with unexpected error type")
	}
}

func TestCreateFail(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	d := newDriver()

	if err := d.Config(nil); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	netconfig := &networkConfiguration{BridgeName: "dummy0"}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = netconfig

	if err := d.CreateNetwork("dummy", genericOption); err == nil {
		t.Fatal("Bridge creation was expected to fail")
	}
}

func TestCreateMultipleNetworks(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	d := newDriver()
	dd, _ := d.(*driver)

	config := &configuration{
		EnableIPTables: true,
	}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = config

	if err := d.Config(genericOption); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	config1 := &networkConfiguration{BridgeName: "net_test_1", AllowNonDefaultBridge: true}
	genericOption = make(map[string]interface{})
	genericOption[netlabel.GenericData] = config1
	if err := d.CreateNetwork("1", genericOption); err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	config2 := &networkConfiguration{BridgeName: "net_test_2", AllowNonDefaultBridge: true}
	genericOption[netlabel.GenericData] = config2
	if err := d.CreateNetwork("2", genericOption); err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	config3 := &networkConfiguration{BridgeName: "net_test_3", AllowNonDefaultBridge: true}
	genericOption[netlabel.GenericData] = config3
	if err := d.CreateNetwork("3", genericOption); err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	// Verify the network isolation rules are installed, each network subnet should appear 4 times
	verifyV4INCEntries(dd.networks, 4, t)

	config4 := &networkConfiguration{BridgeName: "net_test_4", AllowNonDefaultBridge: true}
	genericOption[netlabel.GenericData] = config4
	if err := d.CreateNetwork("4", genericOption); err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	// Now 6 times
	verifyV4INCEntries(dd.networks, 6, t)

	d.DeleteNetwork("1")
	verifyV4INCEntries(dd.networks, 4, t)

	d.DeleteNetwork("2")
	verifyV4INCEntries(dd.networks, 2, t)

	d.DeleteNetwork("3")
	verifyV4INCEntries(dd.networks, 0, t)

	d.DeleteNetwork("4")
	verifyV4INCEntries(dd.networks, 0, t)
}

func verifyV4INCEntries(networks map[types.UUID]*bridgeNetwork, numEntries int, t *testing.T) {
	out, err := iptables.Raw("-L", "FORWARD")
	if err != nil {
		t.Fatal(err)
	}
	for _, nw := range networks {
		nt := types.GetIPNetCopy(nw.bridge.bridgeIPv4)
		nt.IP = nt.IP.Mask(nt.Mask)
		re := regexp.MustCompile(nt.String())
		matches := re.FindAllString(string(out[:]), -1)
		if len(matches) != numEntries {
			t.Fatalf("Cannot find expected inter-network isolation rules in IP Tables:\n%s", string(out[:]))
		}
	}
}

type testInterface struct {
	id      int
	mac     net.HardwareAddr
	addr    net.IPNet
	addrv6  net.IPNet
	srcName string
	dstName string
}

type testEndpoint struct {
	ifaces         []*testInterface
	gw             net.IP
	gw6            net.IP
	hostsPath      string
	resolvConfPath string
	routes         []types.StaticRoute
}

func (te *testEndpoint) Interfaces() []driverapi.InterfaceInfo {
	iList := make([]driverapi.InterfaceInfo, len(te.ifaces))

	for i, iface := range te.ifaces {
		iList[i] = iface
	}

	return iList
}

func (te *testEndpoint) AddInterface(id int, mac net.HardwareAddr, ipv4 net.IPNet, ipv6 net.IPNet) error {
	iface := &testInterface{id: id, addr: ipv4, addrv6: ipv6}
	te.ifaces = append(te.ifaces, iface)
	return nil
}

func (i *testInterface) ID() int {
	return i.id
}

func (i *testInterface) MacAddress() net.HardwareAddr {
	return i.mac
}

func (i *testInterface) Address() net.IPNet {
	return i.addr
}

func (i *testInterface) AddressIPv6() net.IPNet {
	return i.addrv6
}

func (i *testInterface) SetNames(srcName string, dstName string) error {
	i.srcName = srcName
	i.dstName = dstName
	return nil
}

func (te *testEndpoint) InterfaceNames() []driverapi.InterfaceNameInfo {
	iList := make([]driverapi.InterfaceNameInfo, len(te.ifaces))

	for i, iface := range te.ifaces {
		iList[i] = iface
	}

	return iList
}

func (te *testEndpoint) SetGateway(gw net.IP) error {
	te.gw = gw
	return nil
}

func (te *testEndpoint) SetGatewayIPv6(gw6 net.IP) error {
	te.gw6 = gw6
	return nil
}

func (te *testEndpoint) SetHostsPath(path string) error {
	te.hostsPath = path
	return nil
}

func (te *testEndpoint) SetResolvConfPath(path string) error {
	te.resolvConfPath = path
	return nil
}

func (te *testEndpoint) AddStaticRoute(destination *net.IPNet, routeType int, nextHop net.IP, interfaceID int) error {
	te.routes = append(te.routes, types.StaticRoute{Destination: destination, RouteType: routeType, NextHop: nextHop, InterfaceID: interfaceID})
	return nil
}

func TestQueryEndpointInfo(t *testing.T) {
	testQueryEndpointInfo(t, true)
}

func TestQueryEndpointInfoHairpin(t *testing.T) {
	testQueryEndpointInfo(t, false)
}

func testQueryEndpointInfo(t *testing.T, ulPxyEnabled bool) {
	defer netutils.SetupTestNetNS(t)()
	d := newDriver()
	dd, _ := d.(*driver)

	config := &configuration{
		EnableIPTables:      true,
		EnableUserlandProxy: ulPxyEnabled,
	}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = config

	if err := d.Config(genericOption); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	netconfig := &networkConfiguration{
		BridgeName: DefaultBridgeName,
		EnableICC:  false,
	}
	genericOption = make(map[string]interface{})
	genericOption[netlabel.GenericData] = netconfig

	err := d.CreateNetwork("net1", genericOption)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	portMappings := getPortMapping()
	epOptions := make(map[string]interface{})
	epOptions[netlabel.PortMap] = portMappings

	te := &testEndpoint{ifaces: []*testInterface{}}
	err = d.CreateEndpoint("net1", "ep1", te, epOptions)
	if err != nil {
		t.Fatalf("Failed to create an endpoint : %s", err.Error())
	}

	network, ok := dd.networks["net1"]
	if !ok {
		t.Fatalf("Cannot find network %s inside driver", "net1")
	}
	ep, _ := network.endpoints["ep1"]
	data, err := d.EndpointOperInfo(network.id, ep.id)
	if err != nil {
		t.Fatalf("Failed to ask for endpoint operational data:  %v", err)
	}
	pmd, ok := data[netlabel.PortMap]
	if !ok {
		t.Fatalf("Endpoint operational data does not contain port mapping data")
	}
	pm, ok := pmd.([]types.PortBinding)
	if !ok {
		t.Fatalf("Unexpected format for port mapping in endpoint operational data")
	}
	if len(ep.portMapping) != len(pm) {
		t.Fatalf("Incomplete data for port mapping in endpoint operational data")
	}
	for i, pb := range ep.portMapping {
		if !pb.Equal(&pm[i]) {
			t.Fatalf("Unexpected data for port mapping in endpoint operational data")
		}
	}

	// Cleanup as host ports are there
	err = network.releasePorts(ep)
	if err != nil {
		t.Fatalf("Failed to release mapped ports: %v", err)
	}
}

func TestCreateLinkWithOptions(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	d := newDriver()

	if err := d.Config(nil); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	netconfig := &networkConfiguration{BridgeName: DefaultBridgeName}
	netOptions := make(map[string]interface{})
	netOptions[netlabel.GenericData] = netconfig

	err := d.CreateNetwork("net1", netOptions)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	mac := net.HardwareAddr([]byte{0x1e, 0x67, 0x66, 0x44, 0x55, 0x66})
	epOptions := make(map[string]interface{})
	epOptions[netlabel.MacAddress] = mac

	te := &testEndpoint{ifaces: []*testInterface{}}
	err = d.CreateEndpoint("net1", "ep", te, epOptions)
	if err != nil {
		t.Fatalf("Failed to create an endpoint: %s", err.Error())
	}

	err = d.Join("net1", "ep", "sbox", te, nil)
	if err != nil {
		t.Fatalf("Failed to join the endpoint: %v", err)
	}

	ifaceName := te.ifaces[0].srcName
	veth, err := netlink.LinkByName(ifaceName)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(mac, veth.Attrs().HardwareAddr) {
		t.Fatalf("Failed to parse and program endpoint configuration")
	}
}

func getExposedPorts() []types.TransportPort {
	return []types.TransportPort{
		types.TransportPort{Proto: types.TCP, Port: uint16(5000)},
		types.TransportPort{Proto: types.UDP, Port: uint16(400)},
		types.TransportPort{Proto: types.TCP, Port: uint16(600)},
	}
}

func getPortMapping() []types.PortBinding {
	return []types.PortBinding{
		types.PortBinding{Proto: types.TCP, Port: uint16(230), HostPort: uint16(23000)},
		types.PortBinding{Proto: types.UDP, Port: uint16(200), HostPort: uint16(22000)},
		types.PortBinding{Proto: types.TCP, Port: uint16(120), HostPort: uint16(12000)},
	}
}

func TestLinkContainers(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()

	d := newDriver()

	config := &configuration{
		EnableIPTables: true,
	}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = config

	if err := d.Config(genericOption); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	netconfig := &networkConfiguration{
		BridgeName: DefaultBridgeName,
		EnableICC:  false,
	}
	genericOption = make(map[string]interface{})
	genericOption[netlabel.GenericData] = netconfig

	err := d.CreateNetwork("net1", genericOption)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	exposedPorts := getExposedPorts()
	epOptions := make(map[string]interface{})
	epOptions[netlabel.ExposedPorts] = exposedPorts

	te1 := &testEndpoint{ifaces: []*testInterface{}}
	err = d.CreateEndpoint("net1", "ep1", te1, epOptions)
	if err != nil {
		t.Fatalf("Failed to create an endpoint : %s", err.Error())
	}

	addr1 := te1.ifaces[0].addr
	if addr1.IP.To4() == nil {
		t.Fatalf("No Ipv4 address assigned to the endpoint:  ep1")
	}

	te2 := &testEndpoint{ifaces: []*testInterface{}}
	err = d.CreateEndpoint("net1", "ep2", te2, nil)
	if err != nil {
		t.Fatalf("Failed to create an endpoint : %s", err.Error())
	}

	addr2 := te2.ifaces[0].addr
	if addr2.IP.To4() == nil {
		t.Fatalf("No Ipv4 address assigned to the endpoint:  ep2")
	}

	ce := []string{"ep1"}
	cConfig := &containerConfiguration{ChildEndpoints: ce}
	genericOption = make(map[string]interface{})
	genericOption[netlabel.GenericData] = cConfig

	err = d.Join("net1", "ep2", "", te2, genericOption)
	if err != nil {
		t.Fatalf("Failed to link ep1 and ep2")
	}

	out, err := iptables.Raw("-L", DockerChain)
	for _, pm := range exposedPorts {
		regex := fmt.Sprintf("%s dpt:%d", pm.Proto.String(), pm.Port)
		re := regexp.MustCompile(regex)
		matches := re.FindAllString(string(out[:]), -1)
		if len(matches) != 1 {
			t.Fatalf("IP Tables programming failed %s", string(out[:]))
		}

		regex = fmt.Sprintf("%s spt:%d", pm.Proto.String(), pm.Port)
		matched, _ := regexp.MatchString(regex, string(out[:]))
		if !matched {
			t.Fatalf("IP Tables programming failed %s", string(out[:]))
		}
	}

	err = d.Leave("net1", "ep2")
	if err != nil {
		t.Fatalf("Failed to unlink ep1 and ep2")
	}

	out, err = iptables.Raw("-L", DockerChain)
	for _, pm := range exposedPorts {
		regex := fmt.Sprintf("%s dpt:%d", pm.Proto.String(), pm.Port)
		re := regexp.MustCompile(regex)
		matches := re.FindAllString(string(out[:]), -1)
		if len(matches) != 0 {
			t.Fatalf("Leave should have deleted relevant IPTables rules  %s", string(out[:]))
		}

		regex = fmt.Sprintf("%s spt:%d", pm.Proto.String(), pm.Port)
		matched, _ := regexp.MatchString(regex, string(out[:]))
		if matched {
			t.Fatalf("Leave should have deleted relevant IPTables rules  %s", string(out[:]))
		}
	}

	// Error condition test with an invalid endpoint-id "ep4"
	ce = []string{"ep1", "ep4"}
	cConfig = &containerConfiguration{ChildEndpoints: ce}
	genericOption = make(map[string]interface{})
	genericOption[netlabel.GenericData] = cConfig

	err = d.Join("net1", "ep2", "", te2, genericOption)
	if err != nil {
		out, err = iptables.Raw("-L", DockerChain)
		for _, pm := range exposedPorts {
			regex := fmt.Sprintf("%s dpt:%d", pm.Proto.String(), pm.Port)
			re := regexp.MustCompile(regex)
			matches := re.FindAllString(string(out[:]), -1)
			if len(matches) != 0 {
				t.Fatalf("Error handling should rollback relevant IPTables rules  %s", string(out[:]))
			}

			regex = fmt.Sprintf("%s spt:%d", pm.Proto.String(), pm.Port)
			matched, _ := regexp.MatchString(regex, string(out[:]))
			if matched {
				t.Fatalf("Error handling should rollback relevant IPTables rules  %s", string(out[:]))
			}
		}
	} else {
		t.Fatalf("Expected Join to fail given link conditions are not satisfied")
	}
}

func TestValidateConfig(t *testing.T) {

	// Test mtu
	c := networkConfiguration{Mtu: -2}
	err := c.Validate()
	if err == nil {
		t.Fatalf("Failed to detect invalid MTU number")
	}

	c.Mtu = 9000
	err = c.Validate()
	if err != nil {
		t.Fatalf("unexpected validation error on MTU number")
	}

	// Bridge network
	_, network, _ := net.ParseCIDR("172.28.0.0/16")

	// Test FixedCIDR
	_, containerSubnet, _ := net.ParseCIDR("172.27.0.0/16")
	c = networkConfiguration{
		AddressIPv4: network,
		FixedCIDR:   containerSubnet,
	}

	err = c.Validate()
	if err == nil {
		t.Fatalf("Failed to detect invalid FixedCIDR network")
	}

	_, containerSubnet, _ = net.ParseCIDR("172.28.0.0/16")
	c.FixedCIDR = containerSubnet
	err = c.Validate()
	if err != nil {
		t.Fatalf("Unexpected validation error on FixedCIDR network")
	}

	_, containerSubnet, _ = net.ParseCIDR("172.28.0.0/15")
	c.FixedCIDR = containerSubnet
	err = c.Validate()
	if err == nil {
		t.Fatalf("Failed to detect invalid FixedCIDR network")
	}

	_, containerSubnet, _ = net.ParseCIDR("172.28.0.0/17")
	c.FixedCIDR = containerSubnet
	err = c.Validate()
	if err != nil {
		t.Fatalf("Unexpected validation error on FixedCIDR network")
	}

	// Test v4 gw
	c.DefaultGatewayIPv4 = net.ParseIP("172.27.30.234")
	err = c.Validate()
	if err == nil {
		t.Fatalf("Failed to detect invalid default gateway")
	}

	c.DefaultGatewayIPv4 = net.ParseIP("172.28.30.234")
	err = c.Validate()
	if err != nil {
		t.Fatalf("Unexpected validation error on default gateway")
	}

	// Test v6 gw
	_, containerSubnet, _ = net.ParseCIDR("2001:1234:ae:b004::/64")
	c = networkConfiguration{
		EnableIPv6:         true,
		FixedCIDRv6:        containerSubnet,
		DefaultGatewayIPv6: net.ParseIP("2001:1234:ac:b004::bad:a55"),
	}
	err = c.Validate()
	if err == nil {
		t.Fatalf("Failed to detect invalid v6 default gateway")
	}

	c.DefaultGatewayIPv6 = net.ParseIP("2001:1234:ae:b004::bad:a55")
	err = c.Validate()
	if err != nil {
		t.Fatalf("Unexpected validation error on v6 default gateway")
	}

	c.FixedCIDRv6 = nil
	err = c.Validate()
	if err == nil {
		t.Fatalf("Failed to detect invalid v6 default gateway")
	}
}

func TestSetDefaultGw(t *testing.T) {
	defer netutils.SetupTestNetNS(t)()
	d := newDriver()

	if err := d.Config(nil); err != nil {
		t.Fatalf("Failed to setup driver config: %v", err)
	}

	_, subnetv6, _ := net.ParseCIDR("2001:db8:ea9:9abc:b0c4::/80")
	gw4 := bridgeNetworks[0].IP.To4()
	gw4[3] = 254
	gw6 := net.ParseIP("2001:db8:ea9:9abc:b0c4::254")

	config := &networkConfiguration{
		BridgeName:         DefaultBridgeName,
		EnableIPv6:         true,
		FixedCIDRv6:        subnetv6,
		DefaultGatewayIPv4: gw4,
		DefaultGatewayIPv6: gw6,
	}

	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = config

	err := d.CreateNetwork("dummy", genericOption)
	if err != nil {
		t.Fatalf("Failed to create bridge: %v", err)
	}

	te := &testEndpoint{ifaces: []*testInterface{}}
	err = d.CreateEndpoint("dummy", "ep", te, nil)
	if err != nil {
		t.Fatalf("Failed to create endpoint: %v", err)
	}

	err = d.Join("dummy", "ep", "sbox", te, nil)
	if err != nil {
		t.Fatalf("Failed to join endpoint: %v", err)
	}

	if !gw4.Equal(te.gw) {
		t.Fatalf("Failed to configure default gateway. Expected %v. Found %v", gw4, te.gw)
	}

	if !gw6.Equal(te.gw6) {
		t.Fatalf("Failed to configure default gateway. Expected %v. Found %v", gw6, te.gw6)
	}
}
