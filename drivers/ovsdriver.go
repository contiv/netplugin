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
	"errors"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/contiv/libovsdb"
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
	OvsBridgeName string // Name of the bridge we are operating on
	ovs           *libovsdb.OvsdbClient
	cache         map[string]map[libovsdb.UUID]libovsdb.Row
	oper          OvsDriverOperState
	agent         *ofnet.OfnetAgent
	ofnetMaster   *ofnet.OfnetMaster
}

func (d *OvsDriver) getRootUUID() libovsdb.UUID {
	for uuid := range d.cache[rootTable] {
		return uuid
	}
	return libovsdb.UUID{}
}

func (d *OvsDriver) populateCache(updates libovsdb.TableUpdates) {
	for table, tableUpdate := range updates.Updates {
		if _, ok := d.cache[table]; !ok {
			d.cache[table] = make(map[libovsdb.UUID]libovsdb.Row)
		}
		for uuid, row := range tableUpdate.Rows {
			empty := libovsdb.Row{}
			if !reflect.DeepEqual(row.New, empty) {
				d.cache[table][libovsdb.UUID{GoUuid: uuid}] = row.New
			} else {
				delete(d.cache[table], libovsdb.UUID{GoUuid: uuid})
			}
		}
	}
}

// Update updates the ovsdb with the libovsdb.TableUpdates.
func (d *OvsDriver) Update(context interface{}, tableUpdates libovsdb.TableUpdates) {
	d.populateCache(tableUpdates)
}

// Locked satisfies a libovsdb interface dependency.
func (d *OvsDriver) Locked([]interface{}) {
}

// Stolen satisfies a libovsdb interface dependency.
func (d *OvsDriver) Stolen([]interface{}) {
}

// Echo satisfies a libovsdb interface dependency.
func (d *OvsDriver) Echo([]interface{}) {
}

func (d *OvsDriver) performOvsdbOps(ops []libovsdb.Operation) error {
	reply, _ := d.ovs.Transact(ovsDataBase, ops...)

	if len(reply) < len(ops) {
		return core.Errorf("Unexpected number of replies. Expected: %d, Recvd: %d",
			len(ops), len(reply))
	}
	ok := true
	errors := []string{}
	for i, o := range reply {
		if o.Error != "" && i < len(ops) {
			errors = append(errors, fmt.Sprintf("%s(%s)", o.Error, o.Details))
			ok = false
		} else if o.Error != "" {
			errors = append(errors, fmt.Sprintf("%s(%s)", o.Error, o.Details))
			ok = false
		}
	}

	if ok {
		return nil
	}

	return core.Errorf("ovs operation failed. Error(s): %v", errors)
}

func (d *OvsDriver) createDeleteBridge(bridgeName string, op oper) error {
	namedUUIDStr := "netplugin"
	brUUID := []libovsdb.UUID{libovsdb.UUID{GoUuid: namedUUIDStr}}
	protocols := []string{"OpenFlow10", "OpenFlow11", "OpenFlow12", "OpenFlow13"}
	opStr := "insert"
	if op != operCreateBridge {
		opStr = "delete"
	}

	// simple insert/delete operation
	brOp := libovsdb.Operation{}
	if op == operCreateBridge {
		bridge := make(map[string]interface{})
		bridge["name"] = bridgeName
		bridge["protocols"], _ = libovsdb.NewOvsSet(protocols)
		brOp = libovsdb.Operation{
			Op:       opStr,
			Table:    bridgeTable,
			Row:      bridge,
			UUIDName: namedUUIDStr,
		}
	} else {
		condition := libovsdb.NewCondition("name", "==", bridgeName)
		brOp = libovsdb.Operation{
			Op:    opStr,
			Table: bridgeTable,
			Where: []interface{}{condition},
		}
		// also fetch the br-uuid from cache
		for uuid, row := range d.cache[bridgeTable] {
			name := row.Fields["name"].(string)
			if name == bridgeName {
				brUUID = []libovsdb.UUID{uuid}
				break
			}
		}
	}

	// Inserting/Deleting a Bridge row in Bridge table requires mutating
	// the open_vswitch table.
	mutateUUID := brUUID
	mutateSet, _ := libovsdb.NewOvsSet(mutateUUID)
	mutation := libovsdb.NewMutation("bridges", opStr, mutateSet)
	condition := libovsdb.NewCondition("_uuid", "==", d.getRootUUID())

	// simple mutate operation
	mutateOp := libovsdb.Operation{
		Op:        "mutate",
		Table:     rootTable,
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}

	operations := []libovsdb.Operation{brOp, mutateOp}
	return d.performOvsdbOps(operations)
}

func (d *OvsDriver) getPortName() string {
	return fmt.Sprintf(portNameFmt, d.oper.CurrPortNum)
}

func (d *OvsDriver) getPortOrIntfNameFromID(id string, isPort bool) (string, error) {
	table := portTable
	if !isPort {
		table = interfaceTable
	}

	for _, row := range d.cache[table] {
		if extIDs, ok := row.Fields["external_ids"]; ok {
			extIDMap := extIDs.(libovsdb.OvsMap).GoMap
			if portID, ok := extIDMap["endpoint-id"]; ok && portID == id {
				return row.Fields["name"].(string), nil
			}
		}
	}
	return "", core.Errorf("Ovs port/intf not found for id: %s", id)
}

func (d *OvsDriver) createDeletePort(portName, intfName, intfType, id string,
	intfOptions map[string]interface{}, tag int, op oper) error {
	// portName is assumed to be unique enough to become uuid
	portUUIDStr := portName
	intfUUIDStr := fmt.Sprintf("Intf%s", portName)
	portUUID := []libovsdb.UUID{libovsdb.UUID{GoUuid: portUUIDStr}}
	intfUUID := []libovsdb.UUID{libovsdb.UUID{GoUuid: intfUUIDStr}}
	opStr := "insert"
	if op != operCreatePort {
		opStr = "delete"
	}

	var err error

	// insert/delete a row in Interface table
	idMap := make(map[string]string)
	intfOp := libovsdb.Operation{}
	if op == operCreatePort {
		intf := make(map[string]interface{})
		intf["name"] = intfName
		intf["type"] = intfType
		idMap["endpoint-id"] = id
		intf["external_ids"], err = libovsdb.NewOvsMap(idMap)
		if err != nil {
			return err
		}

		if intfOptions != nil {
			intf["options"], err = libovsdb.NewOvsMap(intfOptions)
			if err != nil {
				log.Errorf("error '%s' creating options from %v \n", err, intfOptions)
				return err
			}
		}
		intfOp = libovsdb.Operation{
			Op:       opStr,
			Table:    interfaceTable,
			Row:      intf,
			UUIDName: intfUUIDStr,
		}
	} else {
		condition := libovsdb.NewCondition("name", "==", intfName)
		intfOp = libovsdb.Operation{
			Op:    opStr,
			Table: interfaceTable,
			Where: []interface{}{condition},
		}
		// also fetch the intf-uuid from cache
		for uuid, row := range d.cache[interfaceTable] {
			name := row.Fields["name"].(string)
			if name == intfName {
				intfUUID = []libovsdb.UUID{uuid}
				break
			}
		}
	}

	// insert/delete a row in Port table
	portOp := libovsdb.Operation{}
	if op == operCreatePort {
		port := make(map[string]interface{})
		port["name"] = portName
		if tag != 0 {
			port["vlan_mode"] = "access"
			port["tag"] = tag
		} else {
			port["vlan_mode"] = "trunk"
		}
		port["interfaces"], err = libovsdb.NewOvsSet(intfUUID)
		if err != nil {
			return err
		}
		port["external_ids"], err = libovsdb.NewOvsMap(idMap)
		if err != nil {
			return err
		}
		portOp = libovsdb.Operation{
			Op:       opStr,
			Table:    portTable,
			Row:      port,
			UUIDName: portUUIDStr,
		}
	} else {
		condition := libovsdb.NewCondition("name", "==", portName)
		portOp = libovsdb.Operation{
			Op:    opStr,
			Table: portTable,
			Where: []interface{}{condition},
		}
		// also fetch the port-uuid from cache
		for uuid, row := range d.cache[portTable] {
			name := row.Fields["name"].(string)
			if name == portName {
				portUUID = []libovsdb.UUID{uuid}
				break
			}
		}
	}

	// mutate the Ports column of the row in the Bridge table
	mutateSet, _ := libovsdb.NewOvsSet(portUUID)
	mutation := libovsdb.NewMutation("ports", opStr, mutateSet)
	condition := libovsdb.NewCondition("name", "==", defaultBridgeName)
	mutateOp := libovsdb.Operation{
		Op:        "mutate",
		Table:     bridgeTable,
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}

	operations := []libovsdb.Operation{intfOp, portOp, mutateOp}
	return d.performOvsdbOps(operations)
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
	err = d.createDeletePort(intfName, intfName, "vxlan", cfgNw.ID,
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

	ovs, err := libovsdb.Connect(cfg.Ovs.DbIP, cfg.Ovs.DbPort)
	if err != nil {
		return err
	}

	d.ovs = ovs
	d.OvsBridgeName = defaultBridgeName
	d.oper.StateDriver = info.StateDriver
	// restore the driver's runtime state if it exists
	err = d.oper.Read(info.HostLabel)
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

	d.cache = make(map[string]map[libovsdb.UUID]libovsdb.Row)
	d.ovs.Register(d)
	initial, _ := d.ovs.MonitorAll(ovsDataBase, "")
	d.populateCache(*initial)

	// Create a bridge after registering for events as we depend on ovsdb cache.
	// Since the same dirver is used as endpoint driver, only create the bridge
	// if it's not already created
	// XXX: revisit if the bridge-name needs to be configurable
	brCreated := false
	for _, row := range d.cache[bridgeTable] {
		if row.Fields["name"] == defaultBridgeName {
			brCreated = true
			break
		}
	}

	if !brCreated {
		err = d.createDeleteBridge(defaultBridgeName, operCreateBridge)
		if err != nil {
			return err
		}
	}

	log.Errorf("Initializing ovsdriver")

	// Create an ofnet agent
	// FIXME: hard code local interface for now
	localIP, err := netutils.GetInterfaceIP("eth1")
	if err != nil {
		log.Fatalf("Error getting local IP addr")
	}
	d.agent, err = ofnet.NewOfnetAgent(defaultBridgeName, "vxlan", net.ParseIP(localIP))
	if err != nil {
		log.Fatalf("Error initializing ofnet")
		return err
	}

	// Add controller to the OVS
	ctrlerIP := "127.0.0.1"
	ctrlerPort := uint16(6633)
	target := fmt.Sprintf("tcp:%s:%d", ctrlerIP, ctrlerPort)
	if !d.IsControllerPresent(target) {
		err = d.AddController(ctrlerIP, ctrlerPort)
		if err != nil {
			log.Fatalf("Error adding controller to OVS. Err: %v", err)
			return err
		}
	}

	// FIXME: We need to elect just few nodes as masters
	var resp bool
	masterIP := "192.168.2.10"
	err = d.agent.AddMaster(&masterIP, &resp)
	if err != nil {
		log.Errorf("Error adding %s as ofnet master. Err: %v", masterIP, err)
	}

	return nil
}

// Deinit performs cleanup prior to destruction of the OvsDriver
func (d *OvsDriver) Deinit() {
	if d.ovs != nil {
		d.createDeleteBridge(defaultBridgeName, operDeleteBridge)
		(*d.ovs).Disconnect()
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

	// Add the vlan/vni to ofnet
	err = d.agent.AddVlan(uint16(cfgNw.PktTag), uint32(cfgNw.ExtPktTag))
	if err != nil {
		log.Errorf("Error adding vlan/vni %d/%d. Err: %v", cfgNw.PktTag, cfgNw.ExtPktTag, err)
		return err
	}

	return nil
}

// DeleteNetwork deletes a network by named identifier
func (d *OvsDriver) DeleteNetwork(id string) error {

	// no driver operation for network delete
	log.Infof("delete net %s \n", id)

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

	err = d.createDeletePort(portName, intfName, intfType, cfgEp.ID,
		intfOpts, cfgNw.PktTag, operCreatePort)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			d.createDeletePort(portName, intfName, intfType, "", nil, 0,
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
	ofpPort, err := d.GetOfpPortNo(portName)
	if err != nil {
		log.Errorf("Could not find the OVS port %s. Err: %v", portName, err)
		return err
	}

	// Add the VTEP/endpoint to ofnet
	if intfType == "vxlan" {
		err = d.agent.AddVtepPort(ofpPort, net.ParseIP(cfgEp.VtepIP))
		if err != nil {
			log.Errorf("Error adding VTEP port %s to ofnet. Err: %v", portName, err)
			return err
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
		err = d.agent.AddLocalEndpoint(endpoint)
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

	portName, err := d.getPortOrIntfNameFromID(epOper.ID, getPortName)
	if err != nil {
		return err
	}

	intfName := ""
	intfName, err = d.getPortOrIntfNameFromID(epOper.ID, getIntfName)
	if err != nil {
		return err
	}

	err = d.createDeletePort(portName, intfName, "", "", nil, 0, operDeletePort)
	if err != nil {
		return err
	}

	return nil
}

// MakeEndpointAddress is currently unsupported.
func (d *OvsDriver) MakeEndpointAddress() (*core.Address, error) {
	return nil, core.Errorf("Not supported")
}

// AddController : Add controller configuration to OVS
func (d *OvsDriver) AddController(ipAddr string, portNo uint16) error {
	// Format target string
	target := fmt.Sprintf("tcp:%s:%d", ipAddr, portNo)
	ctrlerUUIDStr := fmt.Sprintf("local")
	ctrlerUUID := []libovsdb.UUID{libovsdb.UUID{GoUuid: ctrlerUUIDStr}}

	// If controller already exists, nothing to do
	if d.IsControllerPresent(target) {
		return nil
	}

	// insert a row in Controller table
	controller := make(map[string]interface{})
	controller["target"] = target

	// Add an entry in Controller table
	ctrlerOp := libovsdb.Operation{
		Op:       "insert",
		Table:    "Controller",
		Row:      controller,
		UUIDName: ctrlerUUIDStr,
	}

	// mutate the Controller column of the row in the Bridge table
	mutateSet, _ := libovsdb.NewOvsSet(ctrlerUUID)
	mutation := libovsdb.NewMutation("controller", "insert", mutateSet)
	condition := libovsdb.NewCondition("name", "==", d.OvsBridgeName)
	mutateOp := libovsdb.Operation{
		Op:        "mutate",
		Table:     "Bridge",
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}

	// Perform OVS transaction
	operations := []libovsdb.Operation{ctrlerOp, mutateOp}
	return d.performOvsdbOps(operations)
}

// RemoveController : Remove controller configuration
func (d *OvsDriver) RemoveController(target string) error {
	// FIXME:
	return nil
}

// IsControllerPresent : Check if Controller already exists
func (d *OvsDriver) IsControllerPresent(target string) bool {
	for tName, table := range d.cache {
		if tName == "Controller" {
			for _, row := range table {
				for fieldName, value := range row.Fields {
					if fieldName == "target" {
						if value == target {
							// Controller exists.
							return true
						}
					}
				}
			}
		}
	}

	// We could not find the controller
	return false
}

// GetOfpPortNo : Return OFP port number for an interface
func (d *OvsDriver) GetOfpPortNo(intfName string) (uint32, error) {
	for tName, table := range d.cache {
		if tName == "Interface" {
			for _, row := range table {
				if row.Fields["name"] == intfName {
					value := row.Fields["ofport"]
					switch t := value.(type) {
					case uint32:
						return t, nil
					case float64:
						ofpPort := uint32(t)
						return ofpPort, nil
					default:
						return 0, errors.New("Unknown field type")
					}
				}
			}
		}
	}

	// We could not find the interface name
	return 0, errors.New("Interface not found")
}
