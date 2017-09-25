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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/netplugin/nameserver"
	"github.com/contiv/netplugin/utils/netutils"
	"github.com/contiv/ofnet"
	"github.com/vishvananda/netlink"
)

type oper int

const (
	maxIntfRetry = 100
	hostPortName = "contivh0"
)

//EpInfo contains the ovsport and id of the group
type EpInfo struct {
	Ovsportname string `json:"Ovsportname"`
	EpgKey      string `json:"EpgKey"`
	BridgeType  string `json:"BridgeType"`
}

// OvsDriverOperState carries operational state of the OvsDriver.
type OvsDriverOperState struct {
	core.CommonState

	// used to allocate port names. XXX: should it be user controlled?
	CurrPortNum      int                `json:"currPortNum"`
	LocalEpInfo      map[string]*EpInfo `json:"LocalEpInfo"` // info about local endpoints
	localEpInfoMutex sync.Mutex
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
	oper       OvsDriverOperState    // Oper state of the driver
	localIP    string                // Local IP address
	switchDb   map[string]*OvsSwitch // OVS switch instances
	lock       sync.Mutex            // lock for modifying shared state
	HostProxy  *NodeSvcProxy
	nameServer *nameserver.NetpluginNameServer
}

func (d *OvsDriver) getIntfName() (string, error) {
	// take a lock for modifying shared state
	d.lock.Lock()
	defer d.lock.Unlock()

	// get the next available port number
	for i := 0; i < maxIntfRetry; i++ {
		// Pick next port number
		d.oper.CurrPortNum++
		if d.oper.CurrPortNum >= maxPortNum {
			d.oper.CurrPortNum = 0 // roll over
		}
		intfName := fmt.Sprintf("vport%d", d.oper.CurrPortNum)
		ovsIntfName := getOvsPortName(intfName, false)

		// check if the port name is already in use
		_, err := netlink.LinkByName(intfName)
		_, err2 := netlink.LinkByName(ovsIntfName)
		if err != nil && strings.Contains(err.Error(), "not found") &&
			err2 != nil && strings.Contains(err2.Error(), "not found") {
			// save the new state
			err = d.oper.Write()
			if err != nil {
				return "", err
			}
			return intfName, nil
		}
	}

	return "", core.Errorf("Could not get intf name. Max retry exceeded")
}

// Init initializes the OVS driver.
func (d *OvsDriver) Init(info *core.InstanceInfo) error {

	if info == nil || info.StateDriver == nil {
		return core.Errorf("Invalid arguments. instance-info: %+v", info)
	}

	d.oper.StateDriver = info.StateDriver
	d.localIP = info.VtepIP
	// restore the driver's runtime state if it exists
	err := d.oper.Read(info.HostLabel)
	if core.ErrIfKeyExists(err) != nil {
		log.Errorf("Failed to read driver oper state for key %q. Error: %s",
			info.HostLabel, err)
		return err
	} else if err != nil {
		// create the oper state as it is first time start up
		d.oper.ID = info.HostLabel
		d.oper.CurrPortNum = 0

		// create local endpoint info map
		d.oper.LocalEpInfo = make(map[string]*EpInfo)

		// write the oper
		err = d.oper.Write()
		if err != nil {
			return err
		}
	}

	// make sure LocalEpInfo exists
	if d.oper.LocalEpInfo == nil {
		d.oper.LocalEpInfo = make(map[string]*EpInfo)
		// write the oper
		err = d.oper.Write()
		if err != nil {
			return err
		}
	}

	log.Infof("Initializing ovsdriver")

	// Init switch DB
	d.switchDb = make(map[string]*OvsSwitch)

	// Create Vxlan switch
	d.switchDb["vxlan"], err = NewOvsSwitch(vxlanBridgeName, "vxlan", info.VtepIP,
		info.FwdMode, nil, info.HostPvtNW, info.VxlanUDPPort)
	if err != nil {
		log.Fatalf("Error creating vlan switch. Err: %v", err)
	}
	// Create Vlan switch
	d.switchDb["vlan"], err = NewOvsSwitch(vlanBridgeName, "vlan", info.VtepIP,
		info.FwdMode, info.UplinkIntf, info.HostPvtNW, info.VxlanUDPPort)
	if err != nil {
		log.Fatalf("Error creating vlan switch. Err: %v", err)
	}

	// Add name server
	d.nameServer = new(nameserver.NetpluginNameServer)
	d.nameServer.Init(info.StateDriver)
	d.switchDb["vxlan"].AddNameServer(d.nameServer)
	d.switchDb["vlan"].AddNameServer(d.nameServer)
	log.Infof("initialized nameserver")

	// Add uplink to VLAN switch
	if len(info.UplinkIntf) != 0 {
		err = d.switchDb["vlan"].AddUplink("uplinkPort", info.UplinkIntf)
		if err != nil {
			log.Errorf("Could not add uplink %v to vlan OVS. Err: %v", info.UplinkIntf, err)
		}
	}

	if maxPortNum > 0xfffe {
		log.Fatalf("Host bridge logic assumes maxPortNum <= 0xfffe")
	}

	// Add host port.
	_, err = d.switchDb["vxlan"].AddHostPort(hostPortName, maxPortNum, info.HostPvtNW, true)
	if err != nil {
		log.Errorf("Could not add host port %s to OVS. Err: %v", hostPortName, err)
	}

	// Add a masquerade rule to ip tables.
	netmask, _ := netutils.PortToHostIPMAC(0, info.HostPvtNW)
	netutils.SetIPMasquerade(hostPortName, netmask)

	// Initialize the node proxy
	d.HostProxy, err = NewNodeProxy()

	return err
}

//DeleteHostAccPort deletes the access port
func (d *OvsDriver) DeleteHostAccPort(id string) error {
	sw, found := d.switchDb["host"]
	if found {
		operEp := &drivers.OperEndpointState{}
		operEp.StateDriver = d.oper.StateDriver
		err := operEp.Read(id)
		if err != nil {
			return err
		}
		d.HostProxy.DeleteLocalIP(operEp.IPAddress)
		portName := operEp.PortName
		intfName := netutils.GetHostIntfName(portName)
		return sw.DelHostPort(intfName, false)
	}

	return errors.New("host bridge not found")
}

// CreateHostAccPort creates an access port
func (d *OvsDriver) CreateHostAccPort(portName, globalIP string, net int) (string, error) {
	sw, found := d.switchDb["host"]
	if found {
		num := strings.Replace(portName, "hport", "", 1)
		intfNum, err := strconv.Atoi(num)
		if err != nil {
			return "", err
		}

		hostIP, err := sw.AddHostPort(portName, intfNum, net, false)
		if err == nil {
			d.HostProxy.AddLocalIP(globalIP, hostIP)
			return hostIP, nil
		}
	}

	return "", errors.New("host bridge not found")
}

// Deinit performs cleanup prior to destruction of the OvsDriver
func (d *OvsDriver) Deinit() {
	log.Infof("Cleaning up ovsdriver")

	// cleanup both vlan and vxlan OVS instances
	if d.switchDb["vlan"] != nil {
		d.switchDb["vlan"].RemoveUplinks()
		d.switchDb["vlan"].Delete()
	}
	if d.switchDb["vxlan"] != nil {
		d.switchDb["vxlan"].DelHostPort(hostPortName, true)
		d.switchDb["vxlan"].Delete()
	}
}

// CreateNetwork creates a network by named identifier
func (d *OvsDriver) CreateNetwork(id string) error {
	cfgNw := mastercfg.CfgNetworkState{}
	cfgNw.StateDriver = d.oper.StateDriver
	err := cfgNw.Read(id)
	if err != nil {
		log.Errorf("Failed to read net %s \n", cfgNw.ID)
		return err
	}
	log.Infof("create net %+v \n", cfgNw)

	// Find the switch based on network type
	var sw *OvsSwitch
	if cfgNw.PktTagType == "vxlan" {
		sw = d.switchDb["vxlan"]
	} else {
		sw = d.switchDb["vlan"]
	}

	return sw.CreateNetwork(uint16(cfgNw.PktTag), uint32(cfgNw.ExtPktTag), cfgNw.Gateway, cfgNw.Tenant)
}

// DeleteNetwork deletes a network by named identifier
func (d *OvsDriver) DeleteNetwork(id, subnet, nwType, encap string, pktTag, extPktTag int, gateway string, tenant string) error {
	log.Infof("delete net %s, nwType %s, encap %s, tags: %d/%d", id, nwType, encap, pktTag, extPktTag)

	// Find the switch based on network type
	var sw *OvsSwitch
	if encap == "vxlan" {
		sw = d.switchDb["vxlan"]
	} else {
		sw = d.switchDb["vlan"]
	}

	// Delete infra nw endpoint if present
	if nwType == "infra" {
		hostName, _ := os.Hostname()
		epID := id + "-" + hostName

		epOper := drivers.OperEndpointState{}
		epOper.StateDriver = d.oper.StateDriver
		err := epOper.Read(epID)
		if err == nil {
			err = sw.DeletePort(&epOper, true)
			if err != nil {
				log.Errorf("Error deleting endpoint: %+v. Err: %v", epOper, err)
			}
			epOper.Clear()
		}
	}

	return sw.DeleteNetwork(uint16(pktTag), uint32(extPktTag), gateway, tenant)
}

// CreateEndpoint creates an endpoint by named identifier
func (d *OvsDriver) CreateEndpoint(id string) error {
	var (
		err          error
		intfName     string
		epgKey       string
		epgBandwidth int64
		dscp         int
	)

	cfgEp := &mastercfg.CfgEndpointState{}
	cfgEp.StateDriver = d.oper.StateDriver
	err = cfgEp.Read(id)
	if err != nil {
		return err
	}

	// Get the nw config.
	cfgNw := mastercfg.CfgNetworkState{}
	cfgNw.StateDriver = d.oper.StateDriver
	err = cfgNw.Read(cfgEp.NetID)
	if err != nil {
		log.Errorf("Unable to get network %s. Err: %v", cfgEp.NetID, err)
		return err
	}

	pktTagType := cfgNw.PktTagType
	pktTag := cfgNw.PktTag
	cfgEpGroup := &mastercfg.EndpointGroupState{}
	// Read pkt tags from endpoint group if available
	if cfgEp.EndpointGroupKey != "" {
		cfgEpGroup.StateDriver = d.oper.StateDriver

		err = cfgEpGroup.Read(cfgEp.EndpointGroupKey)
		if err == nil {
			log.Debugf("pktTag: %v ", cfgEpGroup.PktTag)
			pktTagType = cfgEpGroup.PktTagType
			pktTag = cfgEpGroup.PktTag
			epgKey = cfgEp.EndpointGroupKey
			dscp = cfgEpGroup.DSCP
			if cfgEpGroup.Bandwidth != "" {
				epgBandwidth = netutils.ConvertBandwidth(cfgEpGroup.Bandwidth)
			}

		} else if core.ErrIfKeyExists(err) == nil {
			log.Infof("EPG %s not found: %v. will use network based tag ", cfgEp.EndpointGroupKey, err)
		} else {
			return err
		}
	}

	// Find the switch based on network type
	var sw *OvsSwitch
	if pktTagType == "vxlan" {
		sw = d.switchDb["vxlan"]
	} else {
		sw = d.switchDb["vlan"]
	}

	// Skip Veth pair creation for infra nw endpoints
	skipVethPair := (cfgNw.NwType == "infra")

	operEp := &drivers.OperEndpointState{}
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
			err = sw.UpdatePort(operEp.PortName, cfgEp, pktTag, cfgNw.PktTag, dscp, skipVethPair)
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

	if cfgNw.NwType == "infra" {
		// For infra nw, port name is network name
		intfName = cfgNw.NetworkName
	} else {
		// Get the interface name to use
		intfName, err = d.getIntfName()
		if err != nil {
			return err
		}
	}

	// Get OVS port name
	ovsPortName := getOvsPortName(intfName, skipVethPair)

	// Ask the switch to create the port
	err = sw.CreatePort(intfName, cfgEp, pktTag, cfgNw.PktTag, cfgEpGroup.Burst, dscp, skipVethPair, epgBandwidth)
	if err != nil {
		log.Errorf("Error creating port %s. Err: %v", intfName, err)
		return err
	}

	// save local endpoint info
	d.oper.localEpInfoMutex.Lock()
	d.oper.LocalEpInfo[id] = &EpInfo{
		Ovsportname: ovsPortName,
		EpgKey:      epgKey,
		BridgeType:  pktTagType,
	}
	d.oper.localEpInfoMutex.Unlock()
	err = d.oper.Write()
	if err != nil {
		return err
	}
	// Save the oper state
	operEp = &drivers.OperEndpointState{
		NetID:       cfgEp.NetID,
		EndpointID:  cfgEp.EndpointID,
		ServiceName: cfgEp.ServiceName,
		IPAddress:   cfgEp.IPAddress,
		IPv6Address: cfgEp.IPv6Address,
		MacAddress:  cfgEp.MacAddress,
		IntfName:    cfgEp.IntfName,
		PortName:    intfName,
		HomingHost:  cfgEp.HomingHost,
		VtepIP:      cfgEp.VtepIP}
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

//UpdateEndpointGroup updates the epg
func (d *OvsDriver) UpdateEndpointGroup(id string) error {
	log.Infof("Received endpoint group update for %s", id)
	var (
		err          error
		epgBandwidth int64
		sw           *OvsSwitch
	)
	//gets the EndpointGroupState object
	cfgEpGroup := &mastercfg.EndpointGroupState{}
	cfgEpGroup.StateDriver = d.oper.StateDriver
	err = cfgEpGroup.Read(id)
	if err != nil {
		return err
	}

	if cfgEpGroup.ID != "" {
		if cfgEpGroup.Bandwidth != "" {
			epgBandwidth = netutils.ConvertBandwidth(cfgEpGroup.Bandwidth)
		}

		d.oper.localEpInfoMutex.Lock()
		defer d.oper.localEpInfoMutex.Unlock()
		for _, epInfo := range d.oper.LocalEpInfo {
			if epInfo.EpgKey == id {
				log.Debugf("Applying bandwidth: %s on: %s ", cfgEpGroup.Bandwidth, epInfo.Ovsportname)
				// Find the switch based on network type
				if epInfo.BridgeType == "vxlan" {
					sw = d.switchDb["vxlan"]
				} else {
					sw = d.switchDb["vlan"]
				}

				// update the endpoint in ovs switch
				err = sw.UpdateEndpoint(epInfo.Ovsportname, cfgEpGroup.Burst, cfgEpGroup.DSCP, epgBandwidth)
				if err != nil {
					log.Errorf("Error adding bandwidth %v , err: %+v", epgBandwidth, err)
					return err
				}
			}
		}
	}
	return err
}

// DeleteEndpoint deletes an endpoint by named identifier.
func (d *OvsDriver) DeleteEndpoint(id string) error {
	epOper := drivers.OperEndpointState{}
	epOper.StateDriver = d.oper.StateDriver
	err := epOper.Read(id)
	if err != nil {
		return err
	}
	defer func() {
		epOper.Clear()
	}()

	// Get the network state
	cfgNw := mastercfg.CfgNetworkState{}
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

	skipVethPair := (cfgNw.NwType == "infra")
	err = sw.DeletePort(&epOper, skipVethPair)
	if err != nil {
		log.Errorf("Error deleting endpoint: %+v. Err: %v", epOper, err)
	}

	d.oper.localEpInfoMutex.Lock()
	delete(d.oper.LocalEpInfo, id)
	d.oper.localEpInfoMutex.Unlock()
	err = d.oper.Write()
	if err != nil {
		return err
	}

	return nil
}

// CreateRemoteEndpoint creates a remote endpoint by named identifier
func (d *OvsDriver) CreateRemoteEndpoint(id string) error {

	log.Debug("OVS driver ignoring remote EP create as it uses its own EP sync")
	return nil
}

// DeleteRemoteEndpoint deletes a remote endpoint by named identifier
func (d *OvsDriver) DeleteRemoteEndpoint(id string) error {
	log.Debug("OVS driver ignoring remote EP delete as it uses its own EP sync")
	return nil
}

// AddPeerHost adds VTEPs if necessary
func (d *OvsDriver) AddPeerHost(node core.ServiceInfo) error {
	// Nothing to do if this is our own IP
	if node.HostAddr == d.localIP {
		return nil
	}

	log.Infof("CreatePeerHost for %+v", node)

	// Add the VTEP for the peer in vxlan switch.
	err := d.switchDb["vxlan"].CreateVtep(node.HostAddr)
	if err != nil {
		log.Errorf("Error adding the VTEP %s. Err: %s", node.HostAddr, err)
		return err
	}

	return nil
}

// DeletePeerHost deletes associated VTEP
func (d *OvsDriver) DeletePeerHost(node core.ServiceInfo) error {
	// Nothing to do if this is our own IP
	if node.HostAddr == d.localIP {
		return nil
	}

	log.Infof("DeletePeerHost for %+v", node)

	// Remove the VTEP for the peer in vxlan switch.
	err := d.switchDb["vxlan"].DeleteVtep(node.HostAddr)
	if err != nil {
		log.Errorf("Error deleting the VTEP %s. Err: %s", node.HostAddr, err)
		return err
	}

	return nil
}

// AddMaster adds master node
func (d *OvsDriver) AddMaster(node core.ServiceInfo) error {
	log.Infof("AddMaster for %+v", node)

	// Add master to vlan and vxlan datapaths
	err := d.switchDb["vlan"].AddMaster(node)
	if err != nil {
		return err
	}
	return d.switchDb["vxlan"].AddMaster(node)
}

// DeleteMaster deletes master node
func (d *OvsDriver) DeleteMaster(node core.ServiceInfo) error {
	log.Infof("DeleteMaster for %+v", node)

	// Delete master from vlan and vxlan datapaths
	err := d.switchDb["vlan"].DeleteMaster(node)
	if err != nil {
		return err
	}
	return d.switchDb["vxlan"].DeleteMaster(node)
}

// AddBgp adds bgp config by named identifier
func (d *OvsDriver) AddBgp(id string) error {
	var sw *OvsSwitch

	cfg := mastercfg.CfgBgpState{}
	cfg.StateDriver = d.oper.StateDriver
	err := cfg.Read(id)
	if err != nil {
		log.Errorf("Failed to read router state %s \n", cfg.Hostname)
		return err
	}
	log.Infof("Create Bgp :%+v", cfg)

	// Find the switch based on network type
	sw = d.switchDb["vlan"]

	return sw.AddBgp(cfg.Hostname, cfg.RouterIP, cfg.As, cfg.NeighborAs, cfg.Neighbor)
}

// DeleteBgp deletes bgp config by named identifier
func (d *OvsDriver) DeleteBgp(id string) error {
	log.Infof("Delete Bgp Neighbor %s \n", id)
	//FixME: We are not maintaining oper state for Bgp
	//Need to Revisit again
	// Find the switch based on network type
	var sw *OvsSwitch
	sw = d.switchDb["vlan"]
	return sw.DeleteBgp()

}

// convSvcSpec converts core.ServiceSpec to ofnet.ServiceSpec
func convSvcSpec(spec *core.ServiceSpec) *ofnet.ServiceSpec {
	pSpec := make([]ofnet.PortSpec, len(spec.Ports))
	for ix, p := range spec.Ports {
		pSpec[ix].Protocol = p.Protocol
		pSpec[ix].SvcPort = p.SvcPort
		pSpec[ix].ProvPort = p.ProvPort
	}

	ofnetSS := ofnet.ServiceSpec{
		IpAddress: spec.IPAddress,
		Ports:     pSpec,
	}
	return &ofnetSS
}

// AddSvcSpec invokes switch api
func (d *OvsDriver) AddSvcSpec(svcName string, spec *core.ServiceSpec) error {
	log.Infof("AddSvcSpec: %s", svcName)
	ss := convSvcSpec(spec)
	errs := ""
	for _, sw := range d.switchDb {
		log.Infof("sw AddSvcSpec: %s", svcName)
		err := sw.AddSvcSpec(svcName, ss)
		if err != nil {
			errs += err.Error()
		}
	}

	err := d.HostProxy.AddSvcSpec(svcName, spec)
	if err != nil {
		errs += err.Error()
	}

	if errs != "" {
		return errors.New(errs)
	}

	d.nameServer.AddLbService(nameserver.K8sDefaultTenant, svcName, spec.IPAddress)

	return nil
}

// DelSvcSpec invokes switch api
func (d *OvsDriver) DelSvcSpec(svcName string, spec *core.ServiceSpec) error {
	ss := convSvcSpec(spec)
	errs := ""
	for _, sw := range d.switchDb {
		err := sw.DelSvcSpec(svcName, ss)
		if err != nil {
			errs += err.Error()
		}
	}

	err := d.HostProxy.DelSvcSpec(svcName, spec)
	if err != nil {
		errs += err.Error()
	}

	if errs != "" {
		return errors.New(errs)
	}

	d.nameServer.DelLbService(nameserver.K8sDefaultTenant, svcName)

	return nil
}

// SvcProviderUpdate invokes switch api
func (d *OvsDriver) SvcProviderUpdate(svcName string, providers []string) {
	for _, sw := range d.switchDb {
		sw.SvcProviderUpdate(svcName, providers)
	}

	d.HostProxy.SvcProviderUpdate(svcName, providers)
}

// GetEndpointStats gets all endpoints from all ovs instances
func (d *OvsDriver) GetEndpointStats() ([]byte, error) {
	vxlanStats, err := d.switchDb["vxlan"].GetEndpointStats()
	if err != nil {
		log.Errorf("Error getting vxlan stats. Err: %v", err)
		return []byte{}, err
	}

	vlanStats, err := d.switchDb["vlan"].GetEndpointStats()
	if err != nil {
		log.Errorf("Error getting vlan stats. Err: %v", err)
		return []byte{}, err
	}

	// combine the maps
	for key, val := range vxlanStats {
		vlanStats[key] = val
	}

	jsonStats, err := json.Marshal(vlanStats)
	if err != nil {
		log.Errorf("Error encoding epstats. Err: %v", err)
		return jsonStats, err
	}

	return jsonStats, nil
}

// InspectState returns driver state as json string
func (d *OvsDriver) InspectState() ([]byte, error) {
	driverState := make(map[string]interface{})

	// get vlan switch state
	vlanState, err := d.switchDb["vlan"].InspectState()
	if err != nil {
		return []byte{}, err
	}

	// get vxlan switch state
	vxlanState, err := d.switchDb["vxlan"].InspectState()
	if err != nil {
		return []byte{}, err
	}

	// build the map
	driverState["vlan"] = vlanState
	driverState["vxlan"] = vxlanState

	// json marshall the map
	jsonState, err := json.Marshal(driverState)
	if err != nil {
		log.Errorf("Error encoding epstats. Err: %v", err)
		return []byte{}, err
	}

	return jsonState, nil
}

// InspectBgp returns bgp state as json string
func (d *OvsDriver) InspectBgp() ([]byte, error) {

	// get vlan switch state
	bgpState, err := d.switchDb["vlan"].InspectBgp()
	if err != nil {
		return []byte{}, err
	}

	// json marshall the map
	jsonState, err := json.Marshal(bgpState)
	if err != nil {
		log.Errorf("Error encoding epstats. Err: %v", err)
		return []byte{}, err
	}

	return jsonState, nil
}

// GlobalConfigUpdate sets the global level configs like arp-mode
func (d *OvsDriver) GlobalConfigUpdate(inst core.InstanceInfo) error {
	// convert the netplugin config to ofnet config
	// currently, its only ArpMode
	var cfg ofnet.OfnetGlobalConfig
	switch inst.ArpMode {
	case "flood":
		cfg.ArpMode = ofnet.ArpFlood
	default:
		// set the default to proxy for graceful upgrade
		cfg.ArpMode = ofnet.ArpProxy
	}

	errs := ""
	for _, sw := range d.switchDb {
		err := sw.GlobalConfigUpdate(cfg)
		if err != nil {
			errs += err.Error()
		}
	}
	if errs != "" {
		return errors.New(errs)
	}

	return nil
}

// InspectNameserver returns nameserver state as json string
func (d *OvsDriver) InspectNameserver() ([]byte, error) {
	if d.nameServer == nil {
		return []byte{}, nil
	}

	ns, err := d.nameServer.InspectState()
	jsonState, err := json.Marshal(ns)
	if err != nil {
		log.Errorf("Error encoding nameserver state. Err: %v", err)
		return []byte{}, err
	}

	return jsonState, nil
}

// AddPolicyRule creates a policy rule
func (d *OvsDriver) AddPolicyRule(id string) error {
	log.Debug("OVS driver ignoring PolicyRule create as it uses ofnet sync")
	return nil
}

// DelPolicyRule deletes a policy rule
func (d *OvsDriver) DelPolicyRule(id string) error {
	log.Debug("OVS driver ignoring PolicyRule delete as it uses ofnet sync")
	return nil
}
