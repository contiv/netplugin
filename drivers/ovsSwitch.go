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
	"time"
	"strings"

	"github.com/contiv/netplugin/netutils"
	"github.com/contiv/ofnet"

	log "github.com/Sirupsen/logrus"
)

// OvsSwitch represents on OVS bridge instance
type OvsSwitch struct {
	bridgeName  string
	netType     string
	ovsdbDriver *OvsdbDriver
	ofnetAgent  *ofnet.OfnetAgent
	ofnetMaster *ofnet.OfnetMaster
}

// Create a new OVS switch instance
func NewOvsSwitch(bridgeName, netType, localIP string) (*OvsSwitch, error) {
	var err error

	sw := new(OvsSwitch)
	sw.bridgeName = bridgeName
	sw.netType = netType

	// Create OVS db driver
	sw.ovsdbDriver, err = NewOvsdbDriver(bridgeName)
	if err != nil {
		log.Fatalf("Error creating ovsdb driver. Err: %v", err)
	}

	// For Vxlan, initialize ofnet. For VLAN mode, we use OVS normal forwarding
	if netType == "vxlan" {
		// Create ofnet master
		// FIXME: Move ofnet master to netmaster.
		sw.ofnetMaster = ofnet.NewOfnetMaster(ofnet.OFNET_MASTER_PORT)
		if sw.ofnetMaster == nil {
			log.Fatalf("Error creating ofnet master")
		}

		// Create an ofnet agent
		sw.ofnetAgent, err = ofnet.NewOfnetAgent(defaultBridgeName, "vxlan",
							net.ParseIP(localIP), ofnet.OFNET_AGENT_PORT, 6633)
		if err != nil {
			log.Fatalf("Error initializing ofnet")
			return nil, err
		}

		// Add controller to the OVS
		ctrlerIP := "127.0.0.1"
		ctrlerPort := uint16(6633)
		target := fmt.Sprintf("tcp:%s:%d", ctrlerIP, ctrlerPort)
		if !sw.ovsdbDriver.IsControllerPresent(target) {
			err = sw.ovsdbDriver.AddController(ctrlerIP, ctrlerPort)
			if err != nil {
				log.Fatalf("Error adding controller to OVS. Err: %v", err)
				return nil, err
			}
		}

		// Wait a little for master to be ready before we connect
		time.Sleep(300 * time.Millisecond)

		// Let local ofnet agent connect to local master too.
		var resp bool
		masterInfo := ofnet.OfnetNode{
			HostAddr: localIP,
			HostPort: ofnet.OFNET_MASTER_PORT,
		}
		err = sw.ofnetAgent.AddMaster(&masterInfo, &resp)
		if err != nil {
			log.Errorf("Error adding %s as ofnet master. Err: %v", localIP, err)
		}

		log.Infof("Waiting for OVS switch to connect..")

		// Wait for a while for OVS switch to connect to ofnet agent
		for i := 0; i < 10; i++ {
			time.Sleep(300 * time.Millisecond)
			if sw.ofnetAgent.IsSwitchConnected() {
				break
			}
		}

		log.Infof("Switch connected.")
	}

	return sw, nil
}

// Delete performs cleanup prior to destruction of the OvsDriver
func (sw *OvsSwitch) Delete() {
	sw.ovsdbDriver.Delete()
}

func (sw *OvsSwitch) CreateNetwork(pktTag uint16, extPktTag uint32) error {
	if sw.netType == "vxlan" {
		// Add the vlan/vni to ofnet
		err := sw.ofnetAgent.AddVlan(pktTag, extPktTag)
		if err != nil {
			log.Errorf("Error adding vlan/vni %d/%d. Err: %v", pktTag, extPktTag, err)
			return err
		}
	}

	return nil
}

func (sw *OvsSwitch) DeleteNetwork(pktTag uint16, extPktTag uint32) error {
	if sw.netType == "vxlan" {
		// Delete vlan/vni mapping
		err := sw.ofnetAgent.RemoveVlan(pktTag, extPktTag)
		if err != nil {
			log.Errorf("Error removing vlan/vni %d/%d. Err: %v", pktTag, extPktTag, err)
			return err
		}
	}

	return nil
}

func (sw *OvsSwitch) CreatePort(portName, intfName, intfType string,
	cfgEp *OvsCfgEndpointState, cfgNw *OvsCfgNetworkState,
	intfOptions map[string]interface{}) error {
	// Ask OVSDB driver to add/delete the port
	err := sw.ovsdbDriver.CreatePort(portName, intfName, intfType, cfgEp.ID,
									intfOptions, cfgNw.PktTag)
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
	if intfType == "internal" {
		err = netutils.SetInterfaceMac(portName, cfgEp.MacAddress)
		if err != nil {
			log.Errorf("Error setting interface Mac %s on port %s", cfgEp.MacAddress, portName)
			return err
		}
	}

	// Get the openflow port number for the interface
	ofpPort, err := sw.ovsdbDriver.GetOfpPortNo(portName)
	if err != nil {
		log.Errorf("Could not find the OVS port %s. Err: %v", portName, err)
		return err
	}

	// Add the endpoint to ofnet
	if intfType == "vxlan" {
		log.Fatalf("Not expecting vxlan interfaces here..")
	} else if intfType == "internal" && sw.netType == "vxlan" {
		macAddr, _ := net.ParseMAC(cfgEp.MacAddress)
		// Build the endpoint info
		endpoint := ofnet.EndpointInfo{
			PortNo:  ofpPort,
			MacAddr: macAddr,
			Vlan:    uint16(cfgNw.PktTag),
			IpAddr:  net.ParseIP(cfgEp.IPAddress),
		}

		// Add the local port to ofnet
		err = sw.ofnetAgent.AddLocalEndpoint(endpoint)
		if err != nil {
			log.Errorf("Error adding local port %s to ofnet. Err: %v", portName, err)
			return err
		}
	}

	return nil
}

func (sw *OvsSwitch) DeletePort(epOper *OvsOperEndpointState) error {
	if epOper.VtepIP != "" {
		return nil
	}

	intfName, err := sw.ovsdbDriver.GetPortOrIntfNameFromID(epOper.ID, getIntfName)
	if err != nil {
		return err
	}

	// Remove info from ofnet
	if sw.netType == "vxlan" {
		// Get the openflow port number for the interface
		ofpPort, err := sw.ovsdbDriver.GetOfpPortNo(intfName)
		if err != nil {
			log.Errorf("Could not find the OVS port %s. Err: %v", intfName, err)
			return err
		}

		err = sw.ofnetAgent.RemoveLocalEndpoint(ofpPort)
		if err != nil {
			log.Errorf("Error removing port %s from ofnet. Err: %v", intfName, err)
		}
	}

	// Delete it from ovsdb
	err = sw.ovsdbDriver.DeletePort(intfName)
	if err != nil {
		return err
	}

	return nil
}

func (sw *OvsSwitch) CreateVtep(vtepIP string) error {
	// Create interface name for VTEP
	intfName := fmt.Sprintf(vxlanIfNameFmt, strings.Replace(vtepIP, ".", "", -1))

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

	// Add each of the peers as ofnet masters
	// FIXME: Move ofnet master to netmaster.
	var resp bool
	masterInfo := ofnet.OfnetNode{
		HostAddr: vtepIP,
		HostPort: ofnet.OFNET_MASTER_PORT,
	}
	err = sw.ofnetAgent.AddMaster(&masterInfo, &resp)
	if err != nil {
		log.Errorf("Error adding %s as ofnet master. Err: %v", vtepIP, err)
		return err
	}

	return nil
}

func (sw *OvsSwitch) DeleteVtep(vtepIP string) error {
	// Build vtep interface name
	intfName := fmt.Sprintf(vxlanIfNameFmt, strings.Replace(vtepIP, ".", "", -1))

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
