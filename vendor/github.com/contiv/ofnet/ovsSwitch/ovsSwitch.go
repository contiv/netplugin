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

package ovsSwitch

import (
	"net"
	"strings"
	"time"

	"github.com/contiv/ofnet"
	"github.com/contiv/ofnet/ovsdbDriver"

	log "github.com/Sirupsen/logrus"
)

const OVS_CTRLER_PORT = 6633
const OVS_CTRLER_IP = "127.0.0.1"

const USE_VETH_PAIR = true

// OvsSwitch represents on OVS bridge instance
type OvsSwitch struct {
	bridgeName  string
	netType     string
	ovsdbDriver *ovsdbDriver.OvsDriver
	ofnetAgent  *ofnet.OfnetAgent
}

// NewOvsSwitch Creates a new OVS switch instance
func NewOvsSwitch(bridgeName, netType, localIP string) (*OvsSwitch, error) {
	var err error

	sw := new(OvsSwitch)
	sw.bridgeName = bridgeName
	sw.netType = netType

	// Create OVS db driver
	sw.ovsdbDriver = ovsdbDriver.NewOvsDriver(bridgeName)

	// Create an ofnet agent
	sw.ofnetAgent, err = ofnet.NewOfnetAgent(netType, net.ParseIP(localIP), ofnet.OFNET_AGENT_PORT, OVS_CTRLER_PORT)
	if err != nil {
		log.Fatalf("Error initializing ofnet")
		return nil, err
	}

	// Add controller to the OVS
	if !sw.ovsdbDriver.IsControllerPresent(OVS_CTRLER_IP, OVS_CTRLER_PORT) {
		err = sw.ovsdbDriver.AddController(OVS_CTRLER_IP, OVS_CTRLER_PORT)
		if err != nil {
			log.Fatalf("Error adding controller to OVS. Err: %v", err)
			return nil, err
		}
	}

	// Wait a little for master to be ready before we connect
	time.Sleep(300 * time.Millisecond)

	log.Infof("Waiting for OVS switch to connect..")

	// Wait for a while for OVS switch to connect to ofnet agent
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
		if sw.ofnetAgent.IsSwitchConnected() {
			break
		}
	}

	log.Infof("Switch connected.")

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
func (sw *OvsSwitch) CreateNetwork(pktTag uint16, extPktTag uint32) error {
	// Add the vlan/vni to ofnet
	err := sw.ofnetAgent.AddVlan(pktTag, extPktTag)
	if err != nil {
		log.Errorf("Error adding vlan/vni %d/%d. Err: %v", pktTag, extPktTag, err)
		return err
	}

	return nil
}

// DeleteNetwork deletes a network/vlan
func (sw *OvsSwitch) DeleteNetwork(pktTag uint16, extPktTag uint32) error {
	// Delete vlan/vni mapping
	err := sw.ofnetAgent.RemoveVlan(pktTag, extPktTag)
	if err != nil {
		log.Errorf("Error removing vlan/vni %d/%d. Err: %v", pktTag, extPktTag, err)
		return err
	}

	return nil
}

// CreateEndpoint creates a port in ovs switch
func (sw *OvsSwitch) CreateEndpoint(intfName, macAddr, ipAddr string, pktTag int) error {
	var ovsPortName string
	var ovsIntfType string
	if USE_VETH_PAIR {
		// Generate interface
		ovsPortName = strings.Replace(intfName, "port", "vport", 1)
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

	// Ask OVSDB driver to add the port
	err := sw.ovsdbDriver.CreatePort(ovsPortName, ovsIntfType, uint(pktTag))
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			sw.ovsdbDriver.DeletePort(ovsPortName)
		}
	}()

	// Wait a little for OVS to create the interface
	time.Sleep(300 * time.Millisecond)

	// Set the interface mac address
	err = setInterfaceMac(intfName, macAddr)
	if err != nil {
		log.Errorf("Error setting interface Mac %s on port %s", macAddr, intfName)
		return err
	}

	// Add the endpoint to ofnet
	// Get the openflow port number for the interface
	ofpPort, err := sw.ovsdbDriver.GetOfpPortNo(ovsPortName)
	if err != nil {
		log.Errorf("Could not find the OVS port %s. Err: %v", ovsPortName, err)
		return err
	}

	mAddr, _ := net.ParseMAC(macAddr)

	// Build the endpoint info
	endpoint := ofnet.EndpointInfo{
		PortNo:  ofpPort,
		MacAddr: mAddr,
		Vlan:    uint16(pktTag),
		IpAddr:  net.ParseIP(ipAddr),
	}

	// Add the local port to ofnet
	err = sw.ofnetAgent.AddLocalEndpoint(endpoint)
	if err != nil {
		log.Errorf("Error adding local port %s to ofnet. Err: %v", ovsPortName, err)
		return err
	}

	return nil
}

// UpdateEndpoint updates an OVS port without creating it
func (sw *OvsSwitch) UpdateEndpoint(intfName, macAddr, ipAddr string, pktTag int) error {
	var ovsPortName string
	if USE_VETH_PAIR {
		// Generate interface
		ovsPortName = strings.Replace(intfName, "port", "vport", 1)
	} else {
		ovsPortName = intfName
	}

	// Add the endpoint to ofnet
	// Get the openflow port number for the interface
	ofpPort, err := sw.ovsdbDriver.GetOfpPortNo(ovsPortName)
	if err != nil {
		log.Errorf("Could not find the OVS port %s. Err: %v", ovsPortName, err)
		return err
	}

	mAddr, _ := net.ParseMAC(macAddr)

	// Build the endpoint info
	endpoint := ofnet.EndpointInfo{
		PortNo:  ofpPort,
		MacAddr: mAddr,
		Vlan:    uint16(pktTag),
		IpAddr:  net.ParseIP(ipAddr),
	}

	// Add the local port to ofnet
	err = sw.ofnetAgent.AddLocalEndpoint(endpoint)
	if err != nil {
		log.Errorf("Error adding local port %s to ofnet. Err: %v", intfName, err)
		return err
	}

	return nil
}

// DeleteEndpoint removes a port from OVS
func (sw *OvsSwitch) DeleteEndpoint(intfName string) error {
	var ovsPortName string
	if USE_VETH_PAIR {
		// Generate interface
		ovsPortName = strings.Replace(intfName, "port", "vport", 1)
	} else {
		ovsPortName = intfName
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

// AddPeer creates a VTEP interface
func (sw *OvsSwitch) AddPeer(vtepIP string) error {
	// Setup VTEPs only in vxlan and vrouter mode
	if sw.netType != "vxlan" || sw.netType != "vrouter" {
		return nil
	}

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

// RemovePeer deletes a VTEP
func (sw *OvsSwitch) RemovePeer(vtepIP string) error {
	// delete VTEPs only in vxlan and vrouter mode
	if sw.netType != "vxlan" || sw.netType != "vrouter" {
		return nil
	}

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

	// Check if port is already part of the OVS and add it
	if !sw.ovsdbDriver.IsPortNamePresent(intfName) {
		// Ask OVSDB driver to add the port as a trunk port
		err = sw.ovsdbDriver.CreatePort(intfName, "", 0)
		if err != nil {
			log.Errorf("Error adding uplink %s to OVS. Err: %v", intfName, err)
			return err
		}
	}

	log.Infof("Added uplink %s to OVS switch %s.", intfName, sw.bridgeName)

	defer func() {
		if err != nil {
			sw.ovsdbDriver.DeletePort(intfName)
		}
	}()

	return nil
}
