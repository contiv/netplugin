/***
Copyright 2014 Cisco Systems Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ovsd

import (
	"errors"
	"fmt"
	"net"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/utils/netutils"
	"github.com/contiv/ofnet"
	cmap "github.com/streamrail/concurrent-map"

	log "github.com/Sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

const (
	useVethPair      = true
	vxlanEndpointMtu = 1450
	vxlanOfnetPort   = 9002
	vlanOfnetPort    = 9003
	unusedOfnetPort  = 9004
	vxlanCtrlerPort  = 6633
	vlanCtrlerPort   = 6634
	hostCtrlerPort   = 6635
	hostVLAN         = 2
)

// OvsSwitch represents on OVS bridge instance
type OvsSwitch struct {
	bridgeName    string
	netType       string
	uplinkDb      cmap.ConcurrentMap
	ovsdbDriver   *OvsdbDriver
	ofnetAgent    *ofnet.OfnetAgent
	hostPvtNW     int
	vxlanEncapMtu int
}

// getPvtIP returns a private IP for the port
func (sw *OvsSwitch) getPvtIP(portName string) net.IP {
	if strings.Contains(portName, "vport") {
		port := strings.Replace(portName, "vvport", "", 1)
		portNum, err := strconv.Atoi(port)
		if err == nil {
			ipStr, _ := netutils.PortToHostIPMAC(portNum, sw.hostPvtNW)
			log.Infof("PvtIP: %s", ipStr)
			ips := strings.Split(ipStr, "/")
			if len(ips) > 0 {
				return net.ParseIP(ips[0])
			}
		}

		log.Errorf("Error getting port number from %s - %v", port, err)
	}
	log.Infof("No pvt IP for port %s", portName)

	return nil
}

// GetUplinkInterfaces returns the list of interface associated with the uplink port
func (sw *OvsSwitch) GetUplinkInterfaces(uplinkID string) []string {
	uplink, _ := sw.uplinkDb.Get(uplinkID)
	if uplink == nil {
		return nil
	}
	return uplink.([]string)
}

// NewOvsSwitch Creates a new OVS switch instance
func NewOvsSwitch(bridgeName, netType, localIP, fwdMode string,
	vlanIntf []string, hostPvtNW int, vxlanUDPPort int) (*OvsSwitch, error) {
	var err error
	var datapath string
	var ofnetPort, ctrlrPort uint16
	log.Infof("Received request to create new ovs switch bridge:%s, localIP:%s, fwdMode:%s", bridgeName, localIP, fwdMode)
	sw := new(OvsSwitch)
	sw.bridgeName = bridgeName
	sw.netType = netType
	sw.uplinkDb = cmap.New()
	sw.hostPvtNW = hostPvtNW
	sw.vxlanEncapMtu, err = netutils.GetHostLowestLinkMtu()
	if err != nil {
		log.Fatalf("Failed to get Host Node MTU. Err: %v", err)
	}

	// Create OVS db driver
	sw.ovsdbDriver, err = NewOvsdbDriver(bridgeName, "secure", vxlanUDPPort)
	if err != nil {
		log.Fatalf("Error creating ovsdb driver. Err: %v", err)
	}

	sw.ovsdbDriver.ovsSwitch = sw

	if netType == "vxlan" {
		ofnetPort = vxlanOfnetPort
		ctrlrPort = vxlanCtrlerPort
		switch fwdMode {
		case "bridge":
			datapath = "vxlan"
		case "routing":
			datapath = "vrouter"
		default:
			log.Errorf("Invalid datapath mode")
			return nil, errors.New("invalid forwarding mode. Expects 'bridge' or 'routing'")
		}
		// Create an ofnet agent
		sw.ofnetAgent, err = ofnet.NewOfnetAgent(bridgeName, datapath, net.ParseIP(localIP),
			ofnetPort, ctrlrPort, vlanIntf)

		if err != nil {
			log.Fatalf("Error initializing ofnet")
			return nil, err
		}

	} else if netType == "vlan" {
		ofnetPort = vlanOfnetPort
		ctrlrPort = vlanCtrlerPort
		switch fwdMode {
		case "bridge":
			datapath = "vlan"
		case "routing":
			datapath = "vlrouter"
		default:
			log.Errorf("Invalid datapath mode")
			return nil, errors.New("invalid forwarding mode. Expects 'bridge' or 'routing'")
		}
		// Create an ofnet agent
		sw.ofnetAgent, err = ofnet.NewOfnetAgent(bridgeName, datapath, net.ParseIP(localIP),
			ofnetPort, ctrlrPort, vlanIntf)

		if err != nil {
			log.Fatalf("Error initializing ofnet")
			return nil, err
		}

	} else if netType == "host" {
		err = fmt.Errorf("Explicit host-net not supported")
		return nil, err
	}

	// Add controller to the OVS
	ctrlerIP := "127.0.0.1"
	target := fmt.Sprintf("tcp:%s:%d", ctrlerIP, ctrlrPort)
	if !sw.ovsdbDriver.IsControllerPresent(target) {
		err = sw.ovsdbDriver.AddController(ctrlerIP, ctrlrPort)
		if err != nil {
			log.Errorf("Error adding controller to switch: %s. Err: %v", bridgeName, err)
			return nil, err
		}
	}

	log.Infof("Waiting for OVS switch(%s) to connect..", netType)

	// Wait for a while for OVS switch to connect to agent
	if sw.ofnetAgent != nil {
		sw.ofnetAgent.WaitForSwitchConnection()
	}

	log.Infof("Switch (%s) connected.", netType)

	return sw, nil
}

// Delete performs cleanup prior to destruction of the OvsDriver
func (sw *OvsSwitch) Delete() {
	if sw.ofnetAgent != nil {
		sw.ofnetAgent.Delete()
	}
	if sw.ovsdbDriver != nil {
		sw.ovsdbDriver.Delete()

		// Wait a little for OVS switch to be deleted
		time.Sleep(300 * time.Millisecond)
	}
}

// CreateNetwork creates a new network/vlan
func (sw *OvsSwitch) CreateNetwork(pktTag uint16, extPktTag uint32, defaultGw string, Vrf string) error {
	// Add the vlan/vni to ofnet
	if sw.ofnetAgent != nil {
		err := sw.ofnetAgent.AddNetwork(pktTag, extPktTag, defaultGw, Vrf)
		if err != nil {
			log.Errorf("Error adding vlan/vni %d/%d. Err: %v", pktTag, extPktTag, err)
			return err
		}
	}
	return nil
}

// DeleteNetwork deletes a network/vlan
func (sw *OvsSwitch) DeleteNetwork(pktTag uint16, extPktTag uint32, gateway string, Vrf string) error {
	// Delete vlan/vni mapping
	if sw.ofnetAgent != nil {
		err := sw.ofnetAgent.RemoveNetwork(pktTag, extPktTag, gateway, Vrf)
		if err != nil {
			log.Errorf("Error removing vlan/vni %d/%d. Err: %v", pktTag, extPktTag, err)
			return err
		}
	}
	return nil
}

// createVethPair creates veth interface pairs with specified name
func createVethPair(name1, name2 string) error {
	log.Infof("Creating Veth pairs with name: %s, %s", name1, name2)

	// Veth pair params
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name:   name1,
			TxQLen: 0,
		},
		PeerName: name2,
	}

	// Create the veth pair
	if err := netlink.LinkAdd(veth); err != nil {
		log.Errorf("error creating veth pair: %v", err)
		return err
	}

	return nil
}

// deleteVethPair deletes veth interface pairs
func deleteVethPair(name1, name2 string) error {
	log.Infof("Deleting Veth pairs with name: %s, %s", name1, name2)

	// Veth pair params
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name:   name1,
			TxQLen: 0,
		},
		PeerName: name2,
	}

	// Create the veth pair
	if err := netlink.LinkDel(veth); err != nil {
		log.Errorf("error deleting veth pair: %v", err)
		return err
	}

	return nil
}

// setLinkUp sets the link up
func setLinkUp(name string) error {
	iface, err := netlink.LinkByName(name)
	if err != nil {
		return err
	}
	return netlink.LinkSetUp(iface)
}

// Set the link mtu
func setLinkMtu(name string, mtu int) error {
	iface, err := netlink.LinkByName(name)
	if err != nil {
		return err
	}
	return netlink.LinkSetMTU(iface, mtu)
}

// getOvsPortName returns OVS port name depending on if we use Veth pairs
// For infra nw, dont use Veth pair
func getOvsPortName(intfName string, skipVethPair bool) string {
	var ovsPortName string

	if useVethPair && !skipVethPair {
		ovsPortName = strings.Replace(intfName, "port", "vport", 1)
	} else {
		ovsPortName = intfName
	}

	return ovsPortName
}

// CreatePort creates a port in ovs switch
func (sw *OvsSwitch) CreatePort(intfName string, cfgEp *mastercfg.CfgEndpointState, pktTag, nwPktTag, burst, dscp int, skipVethPair bool, bandwidth int64) error {
	var ovsIntfType string
	var err error
	vethCreated := false
	dbUpdated := false

	// Get OVS port name
	ovsPortName := getOvsPortName(intfName, skipVethPair)
	defer func() {
		if err != nil {
			if vethCreated {
				deleteVethPair(intfName, ovsPortName)
			}
			if dbUpdated {
				sw.ovsdbDriver.DeletePort(intfName)
			}
		}
	}()

	// Create Veth pairs if required
	if useVethPair && !skipVethPair {
		ovsIntfType = ""

		// Create a Veth pair
		err = createVethPair(intfName, ovsPortName)
		if err != nil {
			log.Errorf("Error creating veth pairs. Err: %v", err)
			return err
		}
		vethCreated = true

		// Set the OVS side of the port as up
		err = setLinkUp(ovsPortName)
		if err != nil {
			log.Errorf("Error setting link %s up. Err: %v", ovsPortName, err)
			return err
		}
	} else {
		ovsPortName = intfName
		ovsIntfType = "internal"
	}

	// If the port already exists in OVS, remove it first
	if sw.ovsdbDriver.IsPortNamePresent(ovsPortName) {
		log.Debugf("Removing existing interface entry %s from OVS", ovsPortName)

		// Delete it from ovsdb
		err = sw.ovsdbDriver.DeletePort(ovsPortName)
		if err != nil {
			log.Errorf("Error deleting port %s from OVS. Err: %v", ovsPortName, err)
		}
	}
	// Ask OVSDB driver to add the port
	err = sw.ovsdbDriver.CreatePort(ovsPortName, ovsIntfType, cfgEp.ID, pktTag, burst, bandwidth)
	if err != nil {
		return err
	}
	dbUpdated = true

	// Wait a little for OVS to create the interface
	time.Sleep(300 * time.Millisecond)

	// Set the link mtu to 1450 to allow for 50 bytes vxlan encap
	// (inner eth header(14) + outer IP(20) outer UDP(8) + vxlan header(8))
	if sw.netType == "vxlan" {
		correctMtu := sw.vxlanEncapMtu - 50 //Include Vxlan header size
		err = setLinkMtu(intfName, correctMtu)
	} else {
		err = setLinkMtu(intfName, sw.vxlanEncapMtu)
	}
	if err != nil {
		log.Errorf("Error setting link %s mtu. Err: %v", intfName, err)
		return err
	}

	// Set the interface mac address
	err = netutils.SetInterfaceMac(intfName, cfgEp.MacAddress)
	if err != nil {
		log.Errorf("Error setting interface Mac %s on port %s", cfgEp.MacAddress, intfName)
		return err
	}

	// Add the endpoint to ofnet
	// Get the openflow port number for the interface
	ofpPort, err := sw.ovsdbDriver.GetOfpPortNo(ovsPortName)
	if err != nil {
		log.Errorf("Could not find the OVS port %s. Err: %v", ovsPortName, err)
		return err
	}

	macAddr, _ := net.ParseMAC(cfgEp.MacAddress)

	// Assign an IP based on the intfnumber
	pvtIP := sw.getPvtIP(ovsPortName)

	// Build the endpoint info
	endpoint := ofnet.EndpointInfo{
		PortNo:            ofpPort,
		MacAddr:           macAddr,
		Vlan:              uint16(nwPktTag),
		IpAddr:            net.ParseIP(cfgEp.IPAddress),
		Ipv6Addr:          net.ParseIP(cfgEp.IPv6Address),
		EndpointGroup:     cfgEp.EndpointGroupID,
		EndpointGroupVlan: uint16(pktTag),
		Dscp:              dscp,
		HostPvtIP:         pvtIP,
	}

	log.Infof("Adding local endpoint: {%+v}", endpoint)

	// Add the local port to ofnet
	err = sw.ofnetAgent.AddLocalEndpoint(endpoint)

	if err != nil {
		log.Errorf("Error adding local port %s to ofnet. Err: %v", ovsPortName, err)
		return err
	}
	return nil
}

// UpdateEndpoint updates endpoint state
func (sw *OvsSwitch) UpdateEndpoint(ovsPortName string, burst, dscp int, epgBandwidth int64) error {
	// update bandwidth
	err := sw.ovsdbDriver.UpdatePolicingRate(ovsPortName, burst, epgBandwidth)
	if err != nil {
		return err
	}

	// Get the openflow port number for the interface
	ofpPort, err := sw.ovsdbDriver.GetOfpPortNo(ovsPortName)
	if err != nil {
		log.Errorf("Could not find the OVS port %s. Err: %v", ovsPortName, err)
		return err
	}

	// Build the updated endpoint info
	endpoint := ofnet.EndpointInfo{
		PortNo: ofpPort,
		Dscp:   dscp,
	}

	// update endpoint state in ofnet
	err = sw.ofnetAgent.UpdateLocalEndpoint(endpoint)
	if err != nil {
		log.Errorf("Error updating local port %s to ofnet. Err: %v", ovsPortName, err)
		return err
	}

	return nil
}

// UpdatePort updates an OVS port without creating it
func (sw *OvsSwitch) UpdatePort(intfName string, cfgEp *mastercfg.CfgEndpointState, pktTag, nwPktTag, dscp int, skipVethPair bool) error {

	// Get OVS port name
	ovsPortName := getOvsPortName(intfName, skipVethPair)

	// Add the endpoint to ofnet
	// Get the openflow port number for the interface
	ofpPort, err := sw.ovsdbDriver.GetOfpPortNo(ovsPortName)
	if err != nil {
		log.Errorf("Could not find the OVS port %s. Err: %v", ovsPortName, err)
		return err
	}

	macAddr, _ := net.ParseMAC(cfgEp.MacAddress)

	// Build the endpoint info
	endpoint := ofnet.EndpointInfo{
		PortNo:            ofpPort,
		MacAddr:           macAddr,
		Vlan:              uint16(nwPktTag),
		IpAddr:            net.ParseIP(cfgEp.IPAddress),
		Ipv6Addr:          net.ParseIP(cfgEp.IPv6Address),
		EndpointGroup:     cfgEp.EndpointGroupID,
		EndpointGroupVlan: uint16(pktTag),
		Dscp:              dscp,
	}

	// Add the local port to ofnet
	if sw.ofnetAgent == nil {
		log.Infof("Skipping adding localport to ofnet")
		return nil
	}
	err = sw.ofnetAgent.AddLocalEndpoint(endpoint)
	if err != nil {
		log.Errorf("Error adding local port %s to ofnet. Err: %v", ovsPortName, err)
		return err
	}
	return nil
}

// DeletePort removes a port from OVS
func (sw *OvsSwitch) DeletePort(epOper *drivers.OperEndpointState, skipVethPair bool) error {

	if epOper.VtepIP != "" {
		return nil
	}

	// Get the OVS port name
	ovsPortName := getOvsPortName(epOper.PortName, skipVethPair)

	// Get the openflow port number for the interface and remove from ofnet
	ofpPort, err := sw.ovsdbDriver.GetOfpPortNo(ovsPortName)
	if err == nil {
		if sw.ofnetAgent != nil {
			err = sw.ofnetAgent.RemoveLocalEndpoint(ofpPort)
		}
	} else {
		if sw.ofnetAgent != nil {
			var tenantName string
			netParts := strings.Split(epOper.NetID, ".")
			if len(netParts) == 2 {
				tenantName = netParts[1]
			} else {
				tenantName = "default"
			}
			epID := sw.ofnetAgent.GetEndpointIdByIpVrf(net.ParseIP(epOper.IPAddress), tenantName)
			err = sw.ofnetAgent.RemoveLocalEndpointByID(epID)
		}
	}
	if err != nil {
		log.Errorf("Error removing endpoint %+v from ofnet. Err: %v", epOper, err)
		// continue with further cleanup
	}

	// Delete it from ovsdb
	err = sw.ovsdbDriver.DeletePort(ovsPortName)
	if err != nil {
		log.Errorf("Error deleting port %s from OVS. Err: %v", ovsPortName, err)
		// continue with further cleanup
	}

	// Delete the Veth pairs if required
	if useVethPair && !skipVethPair {
		// Delete a Veth pair
		verr := deleteVethPair(ovsPortName, epOper.PortName)
		if verr != nil {
			log.Errorf("Error deleting veth pairs. Err: %v", verr)
			return verr
		}
	}

	return err
}

// vxlanIfName returns formatted vxlan interface name
func vxlanIfName(vtepIP string) string {
	return fmt.Sprintf(vxlanIfNameFmt, strings.Replace(vtepIP, ".", "", -1))
}

// CreateVtep creates a VTEP interface
func (sw *OvsSwitch) CreateVtep(vtepIP string) error {
	// Create interface name for VTEP
	intfName := vxlanIfName(vtepIP)

	log.Infof("Creating VTEP intf %s for IP %s", intfName, vtepIP)

	// Check if it already exists
	isPresent, vsifName := sw.ovsdbDriver.IsVtepPresent(vtepIP)
	if !isPresent || (vsifName != intfName) {
		// Ask ovsdb to create it
		err := sw.ovsdbDriver.CreateVtep(intfName, vtepIP)
		if err != nil {
			log.Errorf("Error creating VTEP port %s. Err: %v", intfName, err)
		}
	}

	// Wait a little for OVS to create the interface
	time.Sleep(300 * time.Millisecond)

	// Get the openflow port number for the interface
	ofpPort, err := sw.ovsdbDriver.GetOfpPortNo(intfName)
	if err != nil {
		log.Errorf("Could not find the OVS port %s. Err: %v", intfName, err)
		return err
	}

	// Add info about VTEP port to ofnet
	if sw.ofnetAgent != nil {
		err = sw.ofnetAgent.AddVtepPort(ofpPort, net.ParseIP(vtepIP))
		if err != nil {
			log.Errorf("Error adding VTEP port %s to ofnet. Err: %v", intfName, err)
			return err
		}
	}

	return nil
}

// DeleteVtep deletes a VTEP
func (sw *OvsSwitch) DeleteVtep(vtepIP string) error {
	// Build vtep interface name
	intfName := vxlanIfName(vtepIP)

	log.Infof("Deleting VTEP intf %s for IP %s", intfName, vtepIP)

	// Get the openflow port number for the interface
	ofpPort, err := sw.ovsdbDriver.GetOfpPortNo(intfName)
	if err != nil {
		log.Errorf("Could not find the OVS port %s. Err: %v", intfName, err)
		return err
	}

	// Add info about VTEP port to ofnet
	if sw.ofnetAgent != nil {
		err = sw.ofnetAgent.RemoveVtepPort(ofpPort, net.ParseIP(vtepIP))
		if err != nil {
			log.Errorf("Error deleting VTEP port %s to ofnet. Err: %v", intfName, err)
			return err
		}
	}

	// ask ovsdb to delete the VTEP
	return sw.ovsdbDriver.DeleteVtep(intfName)
}

func (sw *OvsSwitch) cleanupOldUplinkState(portName string, intfList []string) (bool, error) {
	var err error
	var oldUplinkIntf []string
	portCreateReq := true

	// Check if uplink is already created
	// Case 1: Bonded ports - port name is the uplinkName
	// Case 2: Indiviual port - port name is the interface name
	portPresent := sw.ovsdbDriver.IsPortNamePresent(portName)
	if portPresent {
		/* If port already exists, make sure it has the same member links
		   If not, a cleanup is required */
		sort.Strings(intfList)
		oldUplinkIntf = sw.ovsdbDriver.GetInterfacesInPort(portName)
		if reflect.DeepEqual(intfList, oldUplinkIntf) {
			log.Warnf("Uplink already part of %s", sw.bridgeName)
			portCreateReq = false
		} else {
			log.Warnf("Deleting old uplink bond with intfs: %+v", oldUplinkIntf)
			err = sw.ovsdbDriver.DeletePortBond(portName, oldUplinkIntf)
			if err == nil {
				portCreateReq = true
			}
		}
		return portCreateReq, err
	}

	if len(intfList) == 1 && sw.ovsdbDriver.IsPortNamePresent(intfList[0]) {
		// Uplink port already part of switch. No change required.
		log.Debugf("Uplink intf %s already part of %s", intfList[0], sw.bridgeName)
		return false, nil
	}

	/* Cleanup any other individual ports that may exist */
	for _, intf := range intfList {
		if sw.ovsdbDriver.IsIntfNamePresent(intf) {
			log.Infof("Deleting old uplink port: %+v", intf)
			err = sw.ovsdbDriver.DeletePort(intf)
			if err != nil {
				break
			}
			portCreateReq = true
		}
	}

	// No cleanup done and new port creation required
	return portCreateReq, err
}

// AddUplink adds uplink port(s) to the OVS
func (sw *OvsSwitch) AddUplink(uplinkName string, intfList []string) error {
	var err error
	var links []*ofnet.LinkInfo
	var uplinkType string

	// some error checking
	if sw.netType != "vlan" {
		log.Fatalf("Can not add uplink to OVS type %s.", sw.netType)
	}

	createUplink, err := sw.cleanupOldUplinkState(uplinkName, intfList)
	if err != nil {
		log.Errorf("Could not cleanup previous uplink state")
		return err
	}

	if createUplink {
		if len(intfList) > 1 {
			log.Debugf("Creating uplink port bond: %s with intf: %+v", uplinkName, intfList)
			err = sw.ovsdbDriver.CreatePortBond(intfList, uplinkName)
			if err != nil {
				log.Errorf("Error adding uplink %s to OVS. Err: %v", intfList, err)
				return err
			}
			uplinkType = ofnet.BondType
		} else {
			log.Debugf("Creating uplink port: %s", intfList[0])
			// Ask OVSDB driver to add the port as a trunk port
			err = sw.ovsdbDriver.CreatePort(intfList[0], "", uplinkName, 0, 0, 0)
			if err != nil {
				log.Errorf("Error adding uplink %s to OVS. Err: %v", intfList[0], err)
				return err
			}
			uplinkType = ofnet.PortType
		}

		// HACK: When an uplink is added to OVS, it disconnects the controller connection.
		//       This is a hack to workaround this issue. We wait for the OVS to reconnect
		//       to the controller.
		// Wait for a while for OVS switch to disconnect/connect to ofnet agent
		time.Sleep(time.Second)
		sw.ofnetAgent.WaitForSwitchConnection()
	}

	uplinkInfo := ofnet.PortInfo{
		Name: uplinkName,
		Type: uplinkType,
	}

	for _, intf := range intfList {
		// Get the openflow port number for the interface
		ofpPort, err := sw.ovsdbDriver.GetOfpPortNo(intf)
		if err != nil {
			log.Errorf("Could not find the OVS port %s. Err: %v", intfList, err)
			return err
		}

		linkInfo := &ofnet.LinkInfo{
			Name:   intf,
			Port:   &uplinkInfo,
			OfPort: ofpPort,
		}

		links = append(links, linkInfo)
	}
	uplinkInfo.MbrLinks = links

	// Add the uplink to OVS switch
	err = sw.ofnetAgent.AddUplink(&uplinkInfo)
	if err != nil {
		log.Errorf("Error adding uplink %+v. Err: %v", uplinkInfo, err)
		return err
	}
	sw.uplinkDb.Set(uplinkName, intfList)
	log.Infof("Added uplink %s to OVS switch %s.", intfList, sw.bridgeName)

	defer func() {
		if err != nil {
			sw.uplinkDb.Remove(uplinkName)
			if uplinkType == ofnet.BondType {
				sw.ovsdbDriver.DeletePortBond(uplinkName, intfList)
			} else {
				sw.ovsdbDriver.DeletePort(intfList[0])
			}
		}
	}()

	return nil
}

// HandleLinkUpdates handle link updates and update the datapath
func (sw *OvsSwitch) HandleLinkUpdates(linkUpd ofnet.LinkUpdateInfo) {
	for intfListObj := range sw.uplinkDb.IterBuffered() {
		intfList := intfListObj.Val.([]string)
		for _, intf := range intfList {
			if intf == linkUpd.LinkName {
				portName := intfListObj.Key
				portUpds := ofnet.PortUpdates{
					PortName: portName,
					Updates: []ofnet.PortUpdate{
						{
							UpdateType: ofnet.LacpUpdate,
							UpdateInfo: linkUpd,
						},
					},
				}
				err := sw.ofnetAgent.UpdateUplink(portName, portUpds)
				if err != nil {
					log.Errorf("Update uplink failed. Err: %+v", err)
				}
				return
			}
		}
	}
}

// RemoveUplinks removes uplink ports from the OVS
func (sw *OvsSwitch) RemoveUplinks() error {

	var err error

	// some error checking
	if sw.netType != "vlan" {
		log.Fatalf("Can not remove uplink from OVS type %s.", sw.netType)
	}

	for intfListObj := range sw.uplinkDb.IterBuffered() {
		intfList := intfListObj.Val.([]string)
		portName := intfListObj.Key

		// Remove uplink from agent
		err = sw.ofnetAgent.RemoveUplink(portName)
		if err != nil {
			log.Errorf("Error removing uplink %s. Err: %v", portName, err)
			return err
		}
		log.Infof("Removed uplink %s from ofnet", portName)

		isPortPresent := sw.ovsdbDriver.IsPortNamePresent(portName)
		if len(intfList) == 1 {
			isPortPresent = sw.ovsdbDriver.IsPortNamePresent(intfList[0])
		}
		if isPortPresent {
			if len(intfList) == 1 {
				err = sw.ovsdbDriver.DeletePort(intfList[0])
			} else {
				err = sw.ovsdbDriver.DeletePortBond(portName, intfList)
			}
			if err != nil {
				log.Errorf("Error deleting uplink %s from OVS. Err: %v", portName, err)
				return err
			}
		}
		time.Sleep(time.Second)
		sw.uplinkDb.Remove(portName)

		log.Infof("Removed uplink %s(%+v) from OVS switch %s.", portName, intfList, sw.bridgeName)
	}

	return nil
}

// AddHostPort adds a host port to the OVS
func (sw *OvsSwitch) AddHostPort(intfName string, intfNum, network int, isHostNS bool) (string, error) {
	var err error

	// some error checking
	if sw.netType != "vxlan" {
		log.Fatalf("Can not add host port to OVS type %s.", sw.netType)
	}

	ovsPortType := ""
	ovsPortName := getOvsPortName(intfName, isHostNS)
	if isHostNS {
		ovsPortType = "internal"
	} else {
		log.Infof("Host port in container name space -- ignore")
		return "", nil
	}

	portID := "host" + intfName

	// If the port already exists in OVS, remove it first
	if sw.ovsdbDriver.IsPortNamePresent(ovsPortName) {
		log.Infof("Removing existing interface entry %s from OVS", ovsPortName)

		// Delete it from ovsdb
		err := sw.ovsdbDriver.DeletePort(ovsPortName)
		if err != nil {
			log.Errorf("Error deleting port %s from OVS. Err: %v", ovsPortName, err)
		}
	}

	// Ask OVSDB driver to add the port as an access port
	err = sw.ovsdbDriver.CreatePort(ovsPortName, ovsPortType, portID, hostVLAN, 0, 0)
	if err != nil {
		log.Errorf("Error adding hostport %s to OVS. Err: %v", intfName, err)
		return "", err
	}

	// Get the openflow port number for the interface
	ofpPort, err := sw.ovsdbDriver.GetOfpPortNo(ovsPortName)
	if err != nil {
		log.Errorf("Could not find the OVS port %s. Err: %v", intfName, err)
		return "", err
	}

	// Assign an IP based on the intfnumber
	ipStr, macStr := netutils.PortToHostIPMAC(intfNum, network)
	mac, _ := net.ParseMAC(macStr)

	portInfo := ofnet.HostPortInfo{
		PortNo:  ofpPort,
		MacAddr: mac,
		IpAddr:  ipStr,
		Kind:    "NAT",
	}
	// Add to ofnet if this is the hostNS port.
	netutils.SetInterfaceMac(intfName, macStr)
	netutils.SetInterfaceIP(intfName, ipStr)
	err = setLinkUp(intfName)

	if sw.ofnetAgent != nil {
		err = sw.ofnetAgent.AddHostPort(portInfo)
		if err != nil {
			log.Errorf("Error adding host port %s. Err: %v", intfName, err)
			return "", err
		}
	}

	log.Infof("Added host port %s to OVS switch %s.", intfName, sw.bridgeName)

	defer func() {
		if err != nil {
			sw.ovsdbDriver.DeletePort(intfName)
		}
	}()

	return ipStr, nil
}

// DelHostPort removes a host port from the OVS
func (sw *OvsSwitch) DelHostPort(intfName string, isHostNS bool) error {
	var err error

	ovsPortName := getOvsPortName(intfName, isHostNS)
	// Get the openflow port number for the interface
	ofpPort, err := sw.ovsdbDriver.GetOfpPortNo(ovsPortName)
	if err != nil {
		log.Errorf("Could not find the OVS port %s. Err: %v", intfName, err)
	}
	// If the port already exists in OVS, remove it first
	if sw.ovsdbDriver.IsPortNamePresent(ovsPortName) {
		log.Debugf("Removing interface entry %s from OVS", ovsPortName)

		// Delete it from ovsdb
		err := sw.ovsdbDriver.DeletePort(ovsPortName)
		if err != nil {
			log.Errorf("Error deleting port %s from OVS. Err: %v", ovsPortName, err)
		}
	}

	if isHostNS && sw.ofnetAgent != nil {
		err = sw.ofnetAgent.RemoveHostPort(ofpPort)
		if err != nil {
			log.Errorf("Error deleting host port %s. Err: %v", intfName, err)
			return err
		}
	}

	return nil
}

// AddMaster adds master node
func (sw *OvsSwitch) AddMaster(node core.ServiceInfo) error {
	var resp bool

	// Build master info
	masterInfo := ofnet.OfnetNode{
		HostAddr: node.HostAddr,
		HostPort: uint16(node.Port),
	}

	// Add the master
	if sw.ofnetAgent != nil {
		err := sw.ofnetAgent.AddMaster(&masterInfo, &resp)
		if err != nil {
			log.Errorf("Error adding ofnet master %+v. Err: %v", masterInfo, err)
			return err
		}
	}

	return nil
}

// DeleteMaster deletes master node
func (sw *OvsSwitch) DeleteMaster(node core.ServiceInfo) error {
	// Build master info
	masterInfo := ofnet.OfnetNode{
		HostAddr: node.HostAddr,
		HostPort: uint16(node.Port),
	}

	// remove the master
	if sw.ofnetAgent != nil {
		err := sw.ofnetAgent.RemoveMaster(&masterInfo)
		if err != nil {
			log.Errorf("Error deleting ofnet master %+v. Err: %v", masterInfo, err)
			return err
		}
	}

	return nil
}

// AddBgp adds a bgp config to host
func (sw *OvsSwitch) AddBgp(hostname string, routerIP string,
	As string, neighborAs, neighbor string) error {
	if sw.netType == "vlan" && sw.ofnetAgent != nil {
		err := sw.ofnetAgent.AddBgp(routerIP, As, neighborAs, neighbor)
		if err != nil {
			log.Errorf("Error adding BGP server")
			return err
		}
	}

	return nil
}

// DeleteBgp deletes bgp config from host
func (sw *OvsSwitch) DeleteBgp() error {
	if sw.netType == "vlan" && sw.ofnetAgent != nil {
		// Delete vlan/vni mapping
		err := sw.ofnetAgent.DeleteBgp()

		if err != nil {
			log.Errorf("Error removing bgp server Err: %v", err)
			return err
		}
	}
	return nil
}

// AddSvcSpec invokes ofnetAgent api
func (sw *OvsSwitch) AddSvcSpec(svcName string, spec *ofnet.ServiceSpec) error {
	log.Infof("OvsSwitch AddSvcSpec %s", svcName)
	if sw.ofnetAgent != nil {
		return sw.ofnetAgent.AddSvcSpec(svcName, spec)
	}

	return nil
}

// DelSvcSpec invokes ofnetAgent api
func (sw *OvsSwitch) DelSvcSpec(svcName string, spec *ofnet.ServiceSpec) error {
	if sw.ofnetAgent != nil {
		return sw.ofnetAgent.DelSvcSpec(svcName, spec)
	}

	return nil
}

// SvcProviderUpdate invokes ofnetAgent api
func (sw *OvsSwitch) SvcProviderUpdate(svcName string, providers []string) {
	if sw.ofnetAgent != nil {
		sw.ofnetAgent.SvcProviderUpdate(svcName, providers)
	}
}

// GetEndpointStats invokes ofnetAgent api
func (sw *OvsSwitch) GetEndpointStats() (map[string]*ofnet.OfnetEndpointStats, error) {
	if sw.ofnetAgent == nil {
		return nil, errors.New("no ofnet agent")
	}

	stats, err := sw.ofnetAgent.GetEndpointStats()
	if err != nil {
		log.Errorf("Error: %v", err)
		return nil, err
	}

	log.Debugf("stats: %+v", stats)

	return stats, nil
}

// InspectState ireturns ofnet state in json form
func (sw *OvsSwitch) InspectState() (interface{}, error) {
	if sw.ofnetAgent == nil {
		return nil, errors.New("no ofnet agent")
	}
	return sw.ofnetAgent.InspectState()
}

// InspectBgp returns ofnet state in json form
func (sw *OvsSwitch) InspectBgp() (interface{}, error) {
	if sw.ofnetAgent == nil {
		return nil, errors.New("no ofnet agent")
	}
	return sw.ofnetAgent.InspectBgp()
}

// GlobalConfigUpdate updates the global configs like arp-mode
func (sw *OvsSwitch) GlobalConfigUpdate(cfg ofnet.OfnetGlobalConfig) error {
	if sw.ofnetAgent == nil {
		return errors.New("no ofnet agent")
	}
	return sw.ofnetAgent.GlobalConfigUpdate(cfg)
}

// AddNameServer returns ofnet state in json form
func (sw *OvsSwitch) AddNameServer(ns ofnet.NameServer) {
	if sw.ofnetAgent != nil {
		sw.ofnetAgent.AddNameServer(ns)
	}
}
