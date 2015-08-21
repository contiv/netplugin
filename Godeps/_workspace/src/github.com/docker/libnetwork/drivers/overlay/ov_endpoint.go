package overlay

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/netutils"
	"github.com/docker/libnetwork/types"
)

type endpointTable map[types.UUID]*endpoint

type endpoint struct {
	id   types.UUID
	mac  net.HardwareAddr
	addr *net.IPNet
}

func (n *network) endpoint(eid types.UUID) *endpoint {
	n.Lock()
	defer n.Unlock()

	return n.endpoints[eid]
}

func (n *network) addEndpoint(ep *endpoint) {
	n.Lock()
	n.endpoints[ep.id] = ep
	n.Unlock()
}

func (n *network) deleteEndpoint(eid types.UUID) {
	n.Lock()
	delete(n.endpoints, eid)
	n.Unlock()
}

func (d *driver) CreateEndpoint(nid, eid types.UUID, epInfo driverapi.EndpointInfo,
	epOptions map[string]interface{}) error {
	if err := validateID(nid, eid); err != nil {
		return err
	}

	n := d.network(nid)
	if n == nil {
		return fmt.Errorf("network id %q not found", nid)
	}

	ep := &endpoint{
		id: eid,
	}

	if epInfo != nil && (len(epInfo.Interfaces()) > 0) {
		addr := epInfo.Interfaces()[0].Address()
		ep.addr = &addr
		ep.mac = epInfo.Interfaces()[0].MacAddress()
		n.addEndpoint(ep)
		return nil
	}

	ipID, err := d.ipAllocator.GetID()
	if err != nil {
		return fmt.Errorf("could not allocate ip from subnet %s: %v",
			bridgeSubnet.String(), err)
	}

	ep.addr = &net.IPNet{
		Mask: bridgeSubnet.Mask,
	}
	ep.addr.IP = make([]byte, 4)

	binary.BigEndian.PutUint32(ep.addr.IP, bridgeSubnetInt+ipID)

	ep.mac = netutils.GenerateRandomMAC()

	err = epInfo.AddInterface(1, ep.mac, *ep.addr, net.IPNet{})
	if err != nil {
		return fmt.Errorf("could not add interface to endpoint info: %v", err)
	}

	n.addEndpoint(ep)

	return nil
}

func (d *driver) DeleteEndpoint(nid, eid types.UUID) error {
	if err := validateID(nid, eid); err != nil {
		return err
	}

	n := d.network(nid)
	if n == nil {
		return fmt.Errorf("network id %q not found", nid)
	}

	ep := n.endpoint(eid)
	if ep == nil {
		return fmt.Errorf("endpoint id %q not found", eid)
	}

	d.ipAllocator.Release(binary.BigEndian.Uint32(ep.addr.IP) - bridgeSubnetInt)
	n.deleteEndpoint(eid)
	return nil
}

func (d *driver) EndpointOperInfo(nid, eid types.UUID) (map[string]interface{}, error) {
	return make(map[string]interface{}, 0), nil
}
