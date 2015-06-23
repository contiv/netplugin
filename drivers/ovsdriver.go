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
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netutils"
	"github.com/contiv/ofnet"

	log "github.com/Sirupsen/logrus"
)

type oper int

// OvsDriverConfig defines the configuration required to initialize the
// OvsDriver.
type OvsDriverConfig struct {
	Ovs struct {
		DbIP   string
		DbPort int
	}
}

// OvsDriverOperState carries operational state of the OvsDriver.
type OvsDriverOperState struct {
	core.CommonState
	// used to allocate port names. XXX: should it be user controlled?
	CurrPortNum int `json:"currPortNum"`
}

// Write the state
func (s *OvsDriverOperState) Write() error {
	key := fmt.Sprintf(ovsOperPath, s.ID)
	return s.StateDriver.WriteState(key, s, json.Marshal)
}

// Read the state given an ID.
func (s *OvsDriverOperState) Read(id string) error {
	key := fmt.Sprintf(ovsOperPath, id)
	return s.StateDriver.ReadState(key, s, json.Unmarshal)
}

// ReadAll reads all the state
func (s *OvsDriverOperState) ReadAll() ([]core.State, error) {
	return s.StateDriver.ReadAllState(ovsOperPathPrefix, s, json.Unmarshal)
}

// Clear removes the state.
func (s *OvsDriverOperState) Clear() error {
	key := fmt.Sprintf(ovsOperPath, s.ID)
	return s.StateDriver.ClearState(key)
}

// OvsDriver implements the Layer 2 Network and Endpoint Driver interfaces
// specific to vlan based open-vswitch. It also implements the
// libovsdb.Notifier interface to keep cache of ovs table state.
type OvsDriver struct {
	ovsdbDriver *OvsdbDriver
	oper        OvsDriverOperState
	ofnetAgent  *ofnet.OfnetAgent
	ofnetMaster *ofnet.OfnetMaster
}

func (d *OvsDriver) getPortName() string {
	return fmt.Sprintf(portNameFmt, d.oper.CurrPortNum)
}

func vxlanIfName(netID, vtepIP string) string {
	return fmt.Sprintf(vxlanIfNameFmt,
		netID, strings.Replace(vtepIP, ".", "", -1))
}

func (d *OvsDriver) getCreateVtepProps(epCfg *OvsCfgEndpointState) (map[string]interface{},
	string, string, error) {
	cfgNw := OvsCfgNetworkState{}
	cfgNw.StateDriver = d.oper.StateDriver
	err := cfgNw.Read(epCfg.NetID)
	if err != nil {
		return nil, "", "", err
	}

	intfOptions := make(map[string]interface{})
	intfOptions["remote_ip"] = epCfg.VtepIP
	intfOptions["key"] = strconv.Itoa(cfgNw.ExtPktTag)

	intfName := vxlanIfName(epCfg.NetID, epCfg.VtepIP)
	return intfOptions, intfName, intfName, nil
}

func (d *OvsDriver) deleteVtep(epOper *OvsOperEndpointState) error {
	cfgNw := OvsCfgNetworkState{}
	cfgNw.StateDriver = d.oper.StateDriver
	err := cfgNw.Read(epOper.NetID)
	if err != nil {
		return err
	}

	intfName := vxlanIfName(epOper.NetID, epOper.VtepIP)
	err = d.ovsdbDriver.CreateDeletePort(intfName, intfName, "vxlan", cfgNw.ID,
		nil, cfgNw.PktTag, operDeletePort)
	if err != nil {
		log.Errorf("error '%s' deleting vxlan peer intfName %s, tag %d \n",
			err, intfName, cfgNw.PktTag)
		return err
	}

	return nil
}

// Init initializes the OVS driver.
func (d *OvsDriver) Init(config *core.Config, info *core.InstanceInfo) error {

	if config == nil || info == nil || info.StateDriver == nil {
		return core.Errorf("Invalid arguments. cfg: %+v, instance-info: %+v",
			config, info)
	}

	cfg, ok := config.V.(*OvsDriverConfig)
	if !ok {
		return core.Errorf("Invalid type passed")
	}

	d.oper.StateDriver = info.StateDriver
	// restore the driver's runtime state if it exists
	err := d.oper.Read(info.HostLabel)
	if core.ErrIfKeyExists(err) != nil {
		log.Printf("Failed to read driver oper state for key %q. Error: %s",
			info.HostLabel, err)
		return err
	} else if err != nil {
		// create the oper state as it is first time start up
		d.oper.ID = info.HostLabel
		d.oper.CurrPortNum = 0
		err = d.oper.Write()
		if err != nil {
			return err
		}
	}

	log.Infof("Initializing ovsdriver")

	// Create OVS db driver
	d.ovsdbDriver, err = NewOvsdbDriver(defaultBridgeName, cfg.Ovs.DbIP, cfg.Ovs.DbPort)
	if err != nil {
		log.Fatalf("Error creating ovsdb driver. Err: %v", err)
	}

	// Create ofnet master
	// FIXME: Move ofnet master to netmaster.
	d.ofnetMaster = ofnet.NewOfnetMaster(ofnet.OFNET_MASTER_PORT)
	if d.ofnetMaster == nil {
		log.Fatalf("Error creating ofnet master")
	}

	// Create an ofnet agent
	// FIXME: hard code local interface for now
	localIP, err := netutils.GetInterfaceIP("eth1")
	if err != nil {
		log.Fatalf("Error getting local IP addr")
	}
	d.ofnetAgent, err = ofnet.NewOfnetAgent(defaultBridgeName, "vxlan",
		net.ParseIP(localIP), ofnet.OFNET_AGENT_PORT, 6633)
	if err != nil {
		log.Fatalf("Error initializing ofnet")
		return err
	}

	// Add controller to the OVS
	ctrlerIP := "127.0.0.1"
	ctrlerPort := uint16(6633)
	target := fmt.Sprintf("tcp:%s:%d", ctrlerIP, ctrlerPort)
	if !d.ovsdbDriver.IsControllerPresent(target) {
		err = d.ovsdbDriver.AddController(ctrlerIP, ctrlerPort)
		if err != nil {
			log.Fatalf("Error adding controller to OVS. Err: %v", err)
			return err
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
	err = d.ofnetAgent.AddMaster(&masterInfo, &resp)
	if err != nil {
		log.Errorf("Error adding %s as ofnet master. Err: %v", localIP, err)
	}

	return nil
}

// Deinit performs cleanup prior to destruction of the OvsDriver
func (d *OvsDriver) Deinit() {
	d.ovsdbDriver.Delete()
}

// CreateNetwork creates a network by named identifier
func (d *OvsDriver) CreateNetwork(id string) error {
	cfgNw := OvsCfgNetworkState{}
	cfgNw.StateDriver = d.oper.StateDriver
	err := cfgNw.Read(id)
	if err != nil {
		log.Errorf("Failed to read net %s \n", cfgNw.ID)
		return err
	}
	log.Infof("create net %s \n", cfgNw.ID)

	// Add the vlan/vni to ofnet
	err = d.ofnetAgent.AddVlan(uint16(cfgNw.PktTag), uint32(cfgNw.ExtPktTag))
	if err != nil {
		log.Errorf("Error adding vlan/vni %d/%d. Err: %v", cfgNw.PktTag, cfgNw.ExtPktTag, err)
		return err
	}

	return nil
}

// DeleteNetwork deletes a network by named identifier
func (d *OvsDriver) DeleteNetwork(id string) error {
	log.Infof("delete net %s \n", id)

	cfgNw := OvsCfgNetworkState{}
	cfgNw.StateDriver = d.oper.StateDriver
	err := cfgNw.Read(id)
	if err != nil {
		log.Errorf("Failed to read net %s \n", cfgNw.ID)
		return err
	}

	// Delete vlan/vni mapping
	err = d.ofnetAgent.RemoveVlan(uint16(cfgNw.PktTag), uint32(cfgNw.ExtPktTag))
	if err != nil {
		log.Errorf("Error removing vlan/vni %d/%d. Err: %v", cfgNw.PktTag, cfgNw.ExtPktTag, err)
		return err
	}

	return nil
}

// CreateEndpoint creates an endpoint by named identifier
func (d *OvsDriver) CreateEndpoint(id string) error {
	var (
		err      error
		intfOpts map[string]interface{}
		portName string
		intfName string
		intfType string
	)

	cfgEp := &OvsCfgEndpointState{}
	cfgEp.StateDriver = d.oper.StateDriver
	err = cfgEp.Read(id)
	if err != nil {
		return err
	}

	operEp := &OvsOperEndpointState{}
	operEp.StateDriver = d.oper.StateDriver
	err = operEp.Read(id)
	if core.ErrIfKeyExists(err) != nil {
		return err
	} else if err == nil {
		// check if oper state matches cfg state. In case of mismatch cleanup
		// up the EP and continue add new one. In case of match just return.
		if operEp.Matches(cfgEp) {
			log.Printf("Found matching oper state for ep %s, noop", id)
			return nil
		}
		log.Printf("Found mismatching oper state for Ep, cleaning it. Config: %+v, Oper: %+v",
			cfgEp, operEp)
		d.DeleteEndpoint(operEp.ID)
	}

	if cfgEp.VtepIP != "" {
		intfOpts, portName, intfName, err = d.getCreateVtepProps(cfgEp)
		if err != nil {
			log.Errorf("error '%s' creating vtep interface(s) for "+
				"remote endpoint %s\n", err, cfgEp.VtepIP)
			return err
		}
		intfType = "vxlan"
	} else {
		// add an internal ovs port with vlan-tag information from the state

		// XXX: revisit, the port name might need to come from user. Also revisit
		// the algorithm to take care of port being deleted and reuse unused port
		// numbers
		d.oper.CurrPortNum++
		err = d.oper.Write()
		if err != nil {
			return err
		}
		portName = d.getPortName()
		intfName = portName
		intfType = "internal"
		intfOpts = nil
	}

	// use the user provided interface name. The primary usecase for such
	// endpoints is for adding the host-interfaces to the ovs bridge.
	// But other usecases might involve user created linux interface
	// devices for containers like SRIOV, that need to be bridged using ovs
	// Also, if the interface name is provided by user then we don't create
	// ovs-internal interface
	if cfgEp.IntfName != "" {
		intfName = cfgEp.IntfName
		intfType = ""
	}

	cfgNw := OvsCfgNetworkState{}
	cfgNw.StateDriver = d.oper.StateDriver
	err = cfgNw.Read(cfgEp.NetID)
	if err != nil {
		return err
	}

	err = d.ovsdbDriver.CreateDeletePort(portName, intfName, intfType, cfgEp.ID,
		intfOpts, cfgNw.PktTag, operCreatePort)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			d.ovsdbDriver.CreateDeletePort(portName, intfName, intfType, "", nil, 0,
				operDeletePort)
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
	ofpPort, err := d.ovsdbDriver.GetOfpPortNo(portName)
	if err != nil {
		log.Errorf("Could not find the OVS port %s. Err: %v", portName, err)
		return err
	}

	// Add the VTEP/endpoint to ofnet
	if intfType == "vxlan" {
		err = d.ofnetAgent.AddVtepPort(ofpPort, net.ParseIP(cfgEp.VtepIP))
		if err != nil {
			log.Errorf("Error adding VTEP port %s to ofnet. Err: %v", portName, err)
			return err
		}

		// Add each of the peers as ofnet masters
		// FIXME: Move ofnet master to netmaster.
		var resp bool
		masterInfo := ofnet.OfnetNode{
			HostAddr: cfgEp.VtepIP,
			HostPort: ofnet.OFNET_MASTER_PORT,
		}
		err = d.ofnetAgent.AddMaster(&masterInfo, &resp)
		if err != nil {
			log.Errorf("Error adding %s as ofnet master. Err: %v", cfgEp.VtepIP, err)
		}
	} else if intfType == "internal" {
		macAddr, _ := net.ParseMAC(cfgEp.MacAddress)
		// Build the endpoint info
		endpoint := ofnet.EndpointInfo{
			PortNo:  ofpPort,
			MacAddr: macAddr,
			Vlan:    uint16(cfgNw.PktTag),
			IpAddr:  net.ParseIP(cfgEp.IPAddress),
		}

		// Add the local port to ofnet
		err = d.ofnetAgent.AddLocalEndpoint(endpoint)
		if err != nil {
			log.Errorf("Error adding local port %s to ofnet. Err: %v", portName, err)
			return err
		}
	}

	// Save the oper state
	operEp = &OvsOperEndpointState{
		PortName:   portName,
		NetID:      cfgEp.NetID,
		AttachUUID: cfgEp.AttachUUID,
		ContName:   cfgEp.ContName,
		IPAddress:  cfgEp.IPAddress,
		IntfName:   cfgEp.IntfName,
		HomingHost: cfgEp.HomingHost,
		VtepIP:     cfgEp.VtepIP}
	operEp.StateDriver = d.oper.StateDriver
	operEp.ID = id
	err = operEp.Write()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			operEp.Clear()
		}
	}()

	return nil
}

// DeleteEndpoint deletes an endpoint by named identifier.
func (d *OvsDriver) DeleteEndpoint(id string) (err error) {

	epOper := OvsOperEndpointState{}
	epOper.StateDriver = d.oper.StateDriver
	err = epOper.Read(id)
	if err != nil {
		return err
	}
	defer func() {
		epOper.Clear()
	}()

	if epOper.VtepIP != "" {
		err = d.deleteVtep(&epOper)
		if err != nil {
			log.Errorf("error '%s' deleting vtep interface(s) for "+
				"remote endpoint %s\n", err, epOper.VtepIP)
		}
		return
	}

	portName, err := d.ovsdbDriver.GetPortOrIntfNameFromID(epOper.ID, getPortName)
	if err != nil {
		return err
	}

	intfName := ""
	intfName, err = d.ovsdbDriver.GetPortOrIntfNameFromID(epOper.ID, getIntfName)
	if err != nil {
		return err
	}

	err = d.ovsdbDriver.CreateDeletePort(portName, intfName, "", "", nil, 0, operDeletePort)
	if err != nil {
		return err
	}

	return nil
}
