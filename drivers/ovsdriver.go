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

	"github.com/contiv/netplugin/core"

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
// specific to vlan based open-vswitch.
type OvsDriver struct {
	oper     OvsDriverOperState
	switchDb map[string]*OvsSwitch
}

func (d *OvsDriver) getIntfName() string {
	return fmt.Sprintf(portNameFmt, d.oper.CurrPortNum)
}

// Init initializes the OVS driver.
func (d *OvsDriver) Init(config *core.Config, info *core.InstanceInfo) error {

	if config == nil || info == nil || info.StateDriver == nil {
		return core.Errorf("Invalid arguments. cfg: %+v, instance-info: %+v",
			config, info)
	}

	_, ok := config.V.(*OvsDriverConfig)
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

	// Publish peer host info to registry
	err = publishHostInfo(info)
	if err != nil {
		log.Fatalf("Error publishing my host info. Err: %v", err)
	}

	log.Infof("Initializing ovsdriver")

	// Init switch DB
	d.switchDb = make(map[string]*OvsSwitch)

	// Create Vxlan switch
	d.switchDb["vxlan"], err = NewOvsSwitch(vxlanBridgeName, "vxlan", info.VtepIP)
	if err != nil {
		log.Fatalf("Error creating vlan switch. Err: %v", err)
	}

	// Create Vlan switch
	d.switchDb["vlan"], err = NewOvsSwitch(vlanBridgeName, "vlan", info.VtepIP)
	if err != nil {
		log.Fatalf("Error creating vlan switch. Err: %v", err)
	}

	// Add uplink to VLAN switch
	if info.VlanIntf != "" {
		err = d.switchDb["vlan"].AddUplinkPort(info.VlanIntf)
		if err != nil {
			log.Errorf("Could not add uplink %s to vlan OVS. Err: %v", info.VlanIntf, err)
		}
	}

	return nil
}

// Deinit performs cleanup prior to destruction of the OvsDriver
func (d *OvsDriver) Deinit() {
	log.Infof("Cleaning up ovsdriver")

	// cleanup both vlan and vxlan OVS instances
	if d.switchDb["vlan"] != nil {
		d.switchDb["vlan"].Delete()
	}
	if d.switchDb["vxlan"] != nil {
		d.switchDb["vxlan"].Delete()
	}
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

	// Find the switch based on network type
	var sw *OvsSwitch
	if cfgNw.PktTagType == "vxlan" {
		sw = d.switchDb["vxlan"]
	} else {
		sw = d.switchDb["vlan"]
	}

	return sw.CreateNetwork(uint16(cfgNw.PktTag), uint32(cfgNw.ExtPktTag))
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

	// Find the switch based on network type
	var sw *OvsSwitch
	if cfgNw.PktTagType == "vxlan" {
		sw = d.switchDb["vxlan"]
	} else {
		sw = d.switchDb["vlan"]
	}

	return sw.DeleteNetwork(uint16(cfgNw.PktTag), uint32(cfgNw.ExtPktTag))
}

// CreateEndpoint creates an endpoint by named identifier
func (d *OvsDriver) CreateEndpoint(id string) error {
	var (
		err      error
		intfName string
		intfType string
	)

	cfgEp := &OvsCfgEndpointState{}
	cfgEp.StateDriver = d.oper.StateDriver
	err = cfgEp.Read(id)
	if err != nil {
		return err
	}

	// Ignore VTEP endpoints and uplink endpoints
	// FIXME: we need to stop publiching these from netmaster
	if cfgEp.VtepIP != "" {
		return nil
	}
	if cfgEp.IntfName != "" {
		return nil
	}

	cfgNw := OvsCfgNetworkState{}
	cfgNw.StateDriver = d.oper.StateDriver
	err = cfgNw.Read(cfgEp.NetID)
	if err != nil {
		return err
	}

	// Find the switch based on network type
	var sw *OvsSwitch
	if cfgNw.PktTagType == "vxlan" {
		sw = d.switchDb["vxlan"]
	} else {
		sw = d.switchDb["vlan"]
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

			// Ask the switch to update the port
			err = sw.UpdatePort(operEp.PortName, cfgEp, cfgNw.PktTag)
			if err != nil {
				log.Errorf("Error creating port %s. Err: %v", intfName, err)
				return err
			}

			return nil
		}
		log.Printf("Found mismatching oper state for Ep, cleaning it. Config: %+v, Oper: %+v",
			cfgEp, operEp)
		d.DeleteEndpoint(operEp.ID)
	}

	// add an internal ovs port with vlan-tag information from the state

	// XXX: revisit, the port name might need to come from user. Also revisit
	// the algorithm to take care of port being deleted and reuse unused port
	// numbers
	d.oper.CurrPortNum++
	err = d.oper.Write()
	if err != nil {
		return err
	}
	intfName = d.getIntfName()
	intfType = "internal"

	// Ask the switch to create the port
	err = sw.CreatePort(intfName, intfType, cfgEp, cfgNw.PktTag)
	if err != nil {
		log.Errorf("Error creating port %s. Err: %v", intfName, err)
		return err
	}

	// Save the oper state
	operEp = &OvsOperEndpointState{
		NetID:      cfgEp.NetID,
		AttachUUID: cfgEp.AttachUUID,
		ContName:   cfgEp.ContName,
		IPAddress:  cfgEp.IPAddress,
		MacAddress: cfgEp.MacAddress,
		IntfName:   cfgEp.IntfName,
		PortName:   intfName,
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

	// Get the network state
	cfgNw := OvsCfgNetworkState{}
	cfgNw.StateDriver = d.oper.StateDriver
	err = cfgNw.Read(epOper.NetID)
	if err != nil {
		return err
	}

	// Find the switch based on network type
	var sw *OvsSwitch
	if cfgNw.PktTagType == "vxlan" {
		sw = d.switchDb["vxlan"]
	} else {
		sw = d.switchDb["vlan"]
	}

	err = sw.DeletePort(&epOper)
	if err != nil {
		log.Errorf("Error deleting endpoint: %+v. Err: %v", epOper, err)
	}

	return nil
}

// CreatePeerHost creates peer host object and adds VTEPs if necessary
func (d *OvsDriver) CreatePeerHost(id string) error {
	log.Infof("CreatePeerHost for %s", id)

	// Read the peer information
	peer := PeerHostState{}
	peer.StateDriver = d.oper.StateDriver
	err := peer.Read(id)
	if err != nil {
		return err
	}

	// Add the VTEP for the peer in vxlan switch.
	err = d.switchDb["vxlan"].CreateVtep(peer.VtepIPAddr)
	if err != nil {
		log.Errorf("Error adding the VTEP %s. Err: %s", peer.VtepIPAddr, err)
		return err
	}

	return nil
}

// DeletePeerHost deletes peer host info and associated VTEP
func (d *OvsDriver) DeletePeerHost(id string) error {
	log.Infof("DeletePeerHost for %s", id)

	// Read the peer information
	peer := PeerHostState{}
	peer.StateDriver = d.oper.StateDriver
	err := peer.Read(id)
	if err != nil {
		return err
	}

	// Add the VTEP for the peer in vxlan switch.
	err = d.switchDb["vxlan"].DeleteVtep(peer.VtepIPAddr)
	if err != nil {
		log.Errorf("Error deleting the VTEP %s. Err: %s", peer.VtepIPAddr, err)
		return err
	}

	return nil
}
