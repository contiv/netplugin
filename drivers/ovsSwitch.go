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

package drivers

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/utils/netutils"
	"github.com/contiv/ofnet"

	log "github.com/Sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

const (
	useVethPair      = true
	vxlanEndpointMtu = 1450
	vxlanOfnetPort   = 9002
	vlanOfnetPort    = 9003
	vxlanCtrlerPort  = 6633
	vlanCtrlerPort   = 6634
)

// OvsSwitch represents on OVS bridge instance
type OvsSwitch struct {
	bridgeName  string
	netType     string
	ovsdbDriver *OvsdbDriver
	ofnetAgent  *ofnet.OfnetAgent
}

// NewOvsSwitch Creates a new OVS switch instance
func NewOvsSwitch(bridgeName, netType, localIP string, routerInfo ...string) (*OvsSwitch, error) {
	var err error
	var ofnetPort, ctrlerPort uint16

	sw := new(OvsSwitch)
	sw.bridgeName = bridgeName
	sw.netType = netType

	// Create OVS db driver
	sw.ovsdbDriver, err = NewOvsdbDriver(bridgeName, "secure")
	if err != nil {
		log.Fatalf("Error creating ovsdb driver. Err: %v", err)
	}

	// determine ofnet and ctrler ports to use
	if netType == "vxlan" {
		ofnetPort = vxlanOfnetPort
		ctrlerPort = vxlanCtrlerPort
	} else {
		ofnetPort = vlanOfnetPort
		ctrlerPort = vlanCtrlerPort
	}
	// For Vxlan, initialize ofnet. For VLAN mode, we use OVS normal forwarding
	if netType == "vxlan" {
		// Create an ofnet agent
		sw.ofnetAgent, err = ofnet.NewOfnetAgent("vxlan", net.ParseIP(localIP),
			ofnet.OFNET_AGENT_VXLAN_PORT, 6633)
		if err != nil {
			log.Fatalf("Error initializing ofnet")
			return nil, err
		}

	// Create an ofnet agent
	sw.ofnetAgent, err = ofnet.NewOfnetAgent(netType, net.ParseIP(localIP), ofnetPort, ctrlerPort)
	if err != nil {
		log.Fatalf("Error initializing ofnet")
		return nil, err
	}

	// Add controller to the OVS
	ctrlerIP := "127.0.0.1"
	target := fmt.Sprintf("tcp:%s:%d", ctrlerIP, ctrlerPort)
	if !sw.ovsdbDriver.IsControllerPresent(target) {
		err = sw.ovsdbDriver.AddController(ctrlerIP, ctrlerPort)
		// For Vxlan, initialize ofnet. For VLAN mode, we use OVS normal forwarding
		if netType == "vxlan" {
			// Create an ofnet agent
			sw.ofnetAgent, err = ofnet.NewOfnetAgent("vxlan", net.ParseIP(localIP),
				ofnet.OFNET_AGENT_VXLAN_PORT, 6633)
			if err != nil {
				log.Fatalf("Error adding controller to OVS. Err: %v", err)
				return nil, err
			}
		}

		log.Infof("Waiting for OVS switch(%s) to connect..", netType)

		// Wait for a while for OVS switch to connect to ofnet agent
		sw.ofnetAgent.WaitForSwitchConnection()

		log.Infof("Switch (%s) connected.", netType)
		// Wait for a while for OVS switch to connect to ofnet agent
		sw.ofnetAgent.WaitForSwitchConnection()

		log.Infof("Switch (vxlan) connected.")
	}

	if netType == "vlan" {
		// Create an ofnet agent
		sw.ofnetAgent, err = ofnet.NewOfnetAgent("vlrouter", net.ParseIP(localIP),
			ofnet.OFNET_AGENT_VLAN_PORT, 6634, routerInfo...)
		if err != nil {
			log.Fatalf("Error initializing ofnet")
			return nil, err
		}

		// Add controller to the OVS
		ctrlerIP := "127.0.0.1"
		ctrlerPort := uint16(6634)
		target := fmt.Sprintf("tcp:%s:%d", ctrlerIP, ctrlerPort)
		if !sw.ovsdbDriver.IsControllerPresent(target) {
			err = sw.ovsdbDriver.AddController(ctrlerIP, ctrlerPort)
			if err != nil {
				log.Fatalf("Error adding controller to OVS. Err: %v", err)
				return nil, err
			}
		}

		log.Infof("Waiting for OVS switch to connect..")

		// Wait for a while for OVS switch to connect to ofnet agent
		sw.ofnetAgent.WaitForSwitchConnection()

		log.Infof("Switch (vlan) connected.")
	}

	if netType == "vlan" {
		// Create an ofnet agent
		sw.ofnetAgent, err = ofnet.NewOfnetAgent("vlrouter", net.ParseIP(localIP),
			ofnet.OFNET_AGENT_VLAN_PORT, 6634, routerInfo...)
		if err != nil {
			log.Fatalf("Error initializing ofnet")
			return nil, err
		}

		// Add controller to the OVS
		ctrlerIP := "127.0.0.1"
		ctrlerPort := uint16(6634)
		target := fmt.Sprintf("tcp:%s:%d", ctrlerIP, ctrlerPort)
		if !sw.ovsdbDriver.IsControllerPresent(target) {
			err = sw.ovsdbDriver.AddController(ctrlerIP, ctrlerPort)
			if err != nil {
				log.Fatalf("Error adding controller to OVS. Err: %v", err)
				return nil, err
			}
		}

		log.Infof("Waiting for OVS switch to connect..")

		// Wait for a while for OVS switch to connect to ofnet agent
		sw.ofnetAgent.WaitForSwitchConnection()

		log.Infof("Switch (vlan) connected.")
	}

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

func (sw *OvsSwitch) CreateNetwork(pktTag uint16, extPktTag uint32, defaultGw string) error {
	// Add the vlan/vni to ofnet
	err := sw.ofnetAgent.AddNetwork(pktTag, extPktTag, defaultGw)
	if err != nil {
		log.Errorf("Error adding vlan/vni %d/%d. Err: %v", pktTag, extPktTag, err)
		return err
	}
	return nil
}

// DeleteNetwork deletes a network/vlan
func (sw *OvsSwitch) DeleteNetwork(pktTag uint16, extPktTag uint32, Gw string) error {
	// Delete vlan/vni mapping
	err := sw.ofnetAgent.RemoveNetwork(pktTag, extPktTag, Gw)
	if err != nil {
		log.Errorf("Error removing vlan/vni %d/%d. Err: %v", pktTag, extPktTag, err)
		return err
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

// getOvsPostName returns OVS port name depending on if we use Veth pairs
func getOvsPostName(intfName string) string {
	var ovsPortName string

	if useVethPair {
		ovsPortName = strings.Replace(intfName, "port", "vport", 1)
	} else {
		ovsPortName = intfName
	}

	return ovsPortName
}

// CreatePort creates a port in ovs switch
func (sw *OvsSwitch) CreatePort(intfName string, cfgEp *mastercfg.CfgEndpointState, pktTag int) error {
	var ovsIntfType string

	// Get OVS port name
	ovsPortName := getOvsPostName(intfName)

	// Create Veth pairs if required
	if useVethPair {
		ovsIntfType = ""

		// Create a Veth pair
		err := createVethPair(intfName, ovsPortName)
		if err != nil {
			log.Errorf("Error creating veth pairs. Err: %v", err)
			return err
		}

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

	// Set the link mtu to 1450 to allow for 50 bytes vxlan encap
	// (inner eth header(14) + outer IP(20) outer UDP(8) + vxlan header(8))
	err := setLinkMtu(intfName, vxlanEndpointMtu)
	if err != nil {
		log.Errorf("Error setting link %s mtu. Err: %v", intfName, err)
		return err
	}

	// Ask OVSDB driver to add the port
	err = sw.ovsdbDriver.CreatePort(ovsPortName, ovsIntfType, cfgEp.ID, pktTag)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			sw.ovsdbDriver.DeletePort(intfName)
		}
	}()

	// Wait a little for OVS to create the interface
	time.Sleep(300 * time.Millisecond)

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

	// Build the endpoint info
	endpoint := ofnet.EndpointInfo{
		PortNo:        ofpPort,
		MacAddr:       macAddr,
		Vlan:          uint16(pktTag),
		IpAddr:        net.ParseIP(cfgEp.IPAddress),
		EndpointGroup: cfgEp.EndpointGroupID,
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

// UpdatePort updates an OVS port without creating it
func (sw *OvsSwitch) UpdatePort(intfName string, cfgEp *mastercfg.CfgEndpointState, pktTag int) error {
	// Get OVS port name
	ovsPortName := getOvsPostName(intfName)

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
		PortNo:        ofpPort,
		MacAddr:       macAddr,
		Vlan:          uint16(pktTag),
		IpAddr:        net.ParseIP(cfgEp.IPAddress),
		EndpointGroup: cfgEp.EndpointGroupID,
	}

	// Add the local port to ofnet
	err = sw.ofnetAgent.AddLocalEndpoint(endpoint)
	if err != nil {
		log.Errorf("Error adding local port %s to ofnet. Err: %v", ovsPortName, err)
		return err
	}

	return nil
}

// DeletePort removes a port from OVS
func (sw *OvsSwitch) DeletePort(epOper *OvsOperEndpointState) error {

	if epOper.VtepIP != "" {
		return nil
	}

	// Get the OVS port name
	ovsPortName := getOvsPostName(epOper.PortName)

	// Delete the Veth pairs if required
	if useVethPair {
		// Delete a Veth pair
		err := deleteVethPair(ovsPortName, epOper.PortName)
		if err != nil {
			log.Errorf("Error creating veth pairs. Err: %v", err)
			// Continue to cleanup remaining state
		}

	} else {
		ovsPortName = epOper.PortName
	}

	// Remove info from ofnet
	// Get the openflow port number for the interface
	ofpPort, err := sw.ovsdbDriver.GetOfpPortNo(ovsPortName)
	if err != nil {
		log.Errorf("Could not find the OVS port %s. Err: %v", ovsPortName, err)
		return err
	}

	err = sw.ofnetAgent.RemoveLocalEndpoint(ofpPort)
	if err != nil {
		log.Errorf("Error removing port %s from ofnet. Err: %v", ovsPortName, err)
	}

	// Delete it from ovsdb
	err = sw.ovsdbDriver.DeletePort(ovsPortName)
	if err != nil {
		return err
	}

	return nil
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
	err = sw.ofnetAgent.AddVtepPort(ofpPort, net.ParseIP(vtepIP))
	if err != nil {
		log.Errorf("Error adding VTEP port %s to ofnet. Err: %v", intfName, err)
		return err
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
	err = sw.ofnetAgent.RemoveVtepPort(ofpPort, net.ParseIP(vtepIP))
	if err != nil {
		log.Errorf("Error deleting VTEP port %s to ofnet. Err: %v", intfName, err)
		return err
	}

	// ask ovsdb to delete the VTEP
	return sw.ovsdbDriver.DeleteVtep(intfName)
}

// AddUplinkPort adds uplink port to the OVS
func (sw *OvsSwitch) AddUplinkPort(intfName string) error {
	var err error

	// some error checking
	if sw.netType != "vlan" {
		log.Fatalf("Can not add uplink to OVS type %s.", sw.netType)
	}

	uplinkID := "uplink" + intfName

	// Check if port is already part of the OVS and add it
	if !sw.ovsdbDriver.IsPortNamePresent(intfName) {
		// Ask OVSDB driver to add the port as a trunk port
		err = sw.ovsdbDriver.CreatePort(intfName, "", uplinkID, 0)
		if err != nil {
			log.Errorf("Error adding uplink %s to OVS. Err: %v", intfName, err)
			return err
		}
	}

	// HACK: When an uplink is added to OVS, it disconnects the controller connection.
	//       This is a hack to workaround this issue. We wait for the OVS to reconnect
	//       to the controller.
	// Wait for a while for OVS switch to disconnect/connect to ofnet agent
	time.Sleep(time.Second)
	sw.ofnetAgent.WaitForSwitchConnection()

	// Get the openflow port number for the interface
	ofpPort, err := sw.ovsdbDriver.GetOfpPortNo(intfName)
	if err != nil {
		log.Errorf("Could not find the OVS port %s. Err: %v", intfName, err)
		return err
	}

	// Add the master
	err = sw.ofnetAgent.AddUplink(ofpPort)
	if err != nil {
		log.Errorf("Error adding uplink %s. Err: %v", uplinkID, err)
		return err
	}

	log.Infof("Added uplink %s to OVS switch %s.", intfName, sw.bridgeName)

	defer func() {
		if err != nil {
			sw.ovsdbDriver.DeletePort(intfName)
		}
	}()

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
	err := sw.ofnetAgent.AddMaster(&masterInfo, &resp)
	if err != nil {
		log.Errorf("Error adding ofnet master %+v. Err: %v", masterInfo, err)
		return err
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
	err := sw.ofnetAgent.RemoveMaster(&masterInfo)
	if err != nil {
		log.Errorf("Error deleting ofnet master %+v. Err: %v", masterInfo, err)
		return err
	}

	return nil
}

// AddBgpNeighbors adds a bgp neighbor to host
func (sw *OvsSwitch) AddBgpNeighbors(hostname string, As string, neighbor string) error {
	if sw.netType == "vlan" {
		err := sw.ofnetAgent.AddBgpNeighbors(As, neighbor)
		if err != nil {
			log.Errorf("Error adding BGP server")
			return err
		}
	}

	return nil
}

// DeleteBgpNeighbors deletes bgp config for host
func (sw *OvsSwitch) DeleteBgpNeighbors() error {
	if sw.netType == "vlan" {
		// Delete vlan/vni mapping
		err := sw.ofnetAgent.DeleteBgpNeighbors()

		if err != nil {
			log.Errorf("Error removing bgp server Err: %v", err)
			return err
		}
	}
	return nil
}
