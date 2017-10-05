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
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/contiv/libovsdb"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/ofnet"

	log "github.com/Sirupsen/logrus"
)

// Max number of retries to get ofp port number
const maxOfportRetry = 20

// OvsdbDriver is responsible for programming OVS using ovsdb protocol. It also
// implements the libovsdb.Notifier interface to keep cache of ovs table state.
type OvsdbDriver struct {
	ovsSwitch    *OvsSwitch
	bridgeName   string // Name of the bridge we are operating on
	ovs          *libovsdb.OvsdbClient
	cache        map[string]map[libovsdb.UUID]libovsdb.Row
	cacheLock    sync.RWMutex // lock to protect cache accesses
	vxlanUDPPort string       // VxLAN UDP port number
}

// NewOvsdbDriver creates a new OVSDB driver instance.
// Create one ovsdb driver instance per OVS bridge that needs to be managed
func NewOvsdbDriver(bridgeName string, failMode string, vxlanUDPPort int) (*OvsdbDriver, error) {
	// Create a new driver instance
	d := new(OvsdbDriver)
	d.bridgeName = bridgeName
	d.vxlanUDPPort = fmt.Sprintf("%d", vxlanUDPPort)

	// Connect to OVS
	ovs, err := libovsdb.ConnectUnix("")
	if err != nil {
		log.Fatalf("Error connecting to OVS. Err: %v", err)
		return nil, err
	}

	d.ovs = ovs

	// Initialize the cache
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
		if row.Fields["name"] == bridgeName {
			brCreated = true
			break
		}
	}

	if !brCreated {
		err = d.createDeleteBridge(bridgeName, failMode, operCreateBridge)
		if err != nil {
			log.Fatalf("Error creating bridge %s. Err: %v", bridgeName, err)
			return nil, err
		}
	}

	return d, nil
}

// Delete : Cleanup the ovsdb driver. delete the bridge we created.
func (d *OvsdbDriver) Delete() error {
	if d.ovs != nil {
		log.Infof("Deleting OVS bridge: %s", d.bridgeName)
		for i := 0; i < 3; i++ {
			err := d.createDeleteBridge(d.bridgeName, "", operDeleteBridge)
			if err != nil {
				log.Errorf("Error deleting the bridge %s. Err: %v", d.bridgeName, err)
				time.Sleep(300 * time.Millisecond)
			} else {
				break
			}
		}
		(*d.ovs).Disconnect()
	}

	return nil
}

func (d *OvsdbDriver) getRootUUID() libovsdb.UUID {
	d.cacheLock.RLock()
	defer d.cacheLock.RUnlock()

	for uuid := range d.cache[rootTable] {
		return uuid
	}
	return libovsdb.UUID{}
}

func (d *OvsdbDriver) populateCache(updates libovsdb.TableUpdates) {
	d.cacheLock.Lock()
	defer func() { d.cacheLock.Unlock() }()

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
func (d *OvsdbDriver) Update(context interface{}, tableUpdates libovsdb.TableUpdates) {
	d.populateCache(tableUpdates)
	intfUpds, ok := tableUpdates.Updates["Interface"]
	if !ok {
		return
	}

	for _, intfUpd := range intfUpds.Rows {
		intf := intfUpd.New.Fields["name"]
		oldLacpStatus, ok := intfUpd.Old.Fields["lacp_current"]
		if !ok {
			return
		}
		newLacpStatus, ok := intfUpd.New.Fields["lacp_current"]
		if !ok {
			return
		}
		if oldLacpStatus == newLacpStatus || d.ovsSwitch == nil {
			return
		}

		linkUpd := ofnet.LinkUpdateInfo{
			LinkName:   intf.(string),
			LacpStatus: newLacpStatus.(bool),
		}
		log.Debugf("LACP_UPD: Interface: %+v. LACP Status - (Old: %+v, New: %+v)\n", intf, oldLacpStatus, newLacpStatus)
		d.ovsSwitch.HandleLinkUpdates(linkUpd)
	}
}

// Locked satisfies a libovsdb interface dependency.
func (d *OvsdbDriver) Locked([]interface{}) {
}

// Stolen satisfies a libovsdb interface dependency.
func (d *OvsdbDriver) Stolen([]interface{}) {
}

// Echo satisfies a libovsdb interface dependency.
func (d *OvsdbDriver) Echo([]interface{}) {
}

func (d *OvsdbDriver) performOvsdbOps(ops []libovsdb.Operation) error {
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

	log.Errorf("OVS operation failed for op: %+v: Errors: %v", ops, errors)

	return core.Errorf("ovs operation failed. Error(s): %v", errors)
}

// Create or delete an OVS bridge instance
func (d *OvsdbDriver) createDeleteBridge(bridgeName, failMode string, op oper) error {
	namedUUIDStr := "netplugin"
	brUUID := []libovsdb.UUID{{GoUuid: namedUUIDStr}}
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

		// Enable Openflow1.3
		bridge["protocols"], _ = libovsdb.NewOvsSet(protocols)

		// set fail-mode if required
		if failMode != "" {
			bridge["fail_mode"] = "secure"
		}

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

// GetPortOrIntfNameFromID gets interface name from id
func (d *OvsdbDriver) GetPortOrIntfNameFromID(id string, isPort bool) (string, error) {
	table := portTable
	if !isPort {
		table = interfaceTable
	}

	d.cacheLock.RLock()
	defer d.cacheLock.RUnlock()

	// walk thru all ports
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

// CreatePort creates an OVS port
func (d *OvsdbDriver) CreatePort(intfName, intfType, id string, tag, burst int, bandwidth int64) error {
	// intfName is assumed to be unique enough to become uuid
	portUUIDStr := intfName
	intfUUIDStr := fmt.Sprintf("Intf%s", intfName)
	portUUID := []libovsdb.UUID{{GoUuid: portUUIDStr}}
	intfUUID := []libovsdb.UUID{{GoUuid: intfUUIDStr}}
	opStr := "insert"

	var err error

	// insert/delete a row in Interface table
	idMap := make(map[string]string)
	intfOp := libovsdb.Operation{}
	intf := make(map[string]interface{})
	intf["name"] = intfName
	intf["type"] = intfType
	if bandwidth != 0 {
		intf["ingress_policing_rate"] = bandwidth
	}
	if burst != 0 {
		intf["ingress_policing_burst"] = burst
	}
	idMap["endpoint-id"] = id
	intf["external_ids"], err = libovsdb.NewOvsMap(idMap)
	if err != nil {
		return err
	}

	// interface table ops
	intfOp = libovsdb.Operation{
		Op:       opStr,
		Table:    interfaceTable,
		Row:      intf,
		UUIDName: intfUUIDStr,
	}

	// insert/delete a row in Port table
	portOp := libovsdb.Operation{}
	port := make(map[string]interface{})
	port["name"] = intfName
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

	// mutate the Ports column of the row in the Bridge table
	mutateSet, _ := libovsdb.NewOvsSet(portUUID)
	mutation := libovsdb.NewMutation("ports", opStr, mutateSet)
	condition := libovsdb.NewCondition("name", "==", d.bridgeName)
	mutateOp := libovsdb.Operation{
		Op:        "mutate",
		Table:     bridgeTable,
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}

	operations := []libovsdb.Operation{intfOp, portOp, mutateOp}
	return d.performOvsdbOps(operations)
}

// GetInterfacesInPort gets list of interfaces in a port in sorted order
func (d *OvsdbDriver) GetInterfacesInPort(portName string) []string {
	var intfList []string
	d.cacheLock.RLock()
	defer d.cacheLock.RUnlock()

	for _, row := range d.cache["Port"] {
		name := row.Fields["name"].(string)
		if name == portName {
			// Port found
			// Iterate over the list of interfaces
			switch (row.Fields["interfaces"]).(type) {
			case libovsdb.UUID: // Individual interface case
				intfUUID := row.Fields["interfaces"].(libovsdb.UUID)
				intfInfo := d.GetIntfInfo(intfUUID)
				if reflect.DeepEqual(intfInfo, libovsdb.Row{}) {
					log.Errorf("could not find interface with UUID: %+v", intfUUID)
					break
				}
				intfList = append(intfList, intfInfo.Fields["name"].(string))
			case libovsdb.OvsSet: // Port bond case
				intfUUIDList := row.Fields["interfaces"].(libovsdb.OvsSet)
				for _, intfUUID := range intfUUIDList.GoSet {
					intfInfo := d.GetIntfInfo(intfUUID.(libovsdb.UUID))
					if reflect.DeepEqual(intfInfo, libovsdb.Row{}) {
						continue
					}
					intfList = append(intfList, intfInfo.Fields["name"].(string))
				}
			}
			sort.Strings(intfList)
			break
		}
	}
	return intfList
}

// GetIntfInfo gets interface information from "Interface" table
func (d *OvsdbDriver) GetIntfInfo(uuid libovsdb.UUID) libovsdb.Row {
	d.cacheLock.RLock()
	defer d.cacheLock.RUnlock()

	for intfUUID, row := range d.cache["Interface"] {
		if intfUUID == uuid {
			return row
		}
	}

	return libovsdb.Row{}
}

//CreatePortBond creates port bond in OVS
func (d *OvsdbDriver) CreatePortBond(intfList []string, bondName string) error {

	var err error
	var ops []libovsdb.Operation
	var intfUUIDList []libovsdb.UUID
	opStr := "insert"

	// Add all the interfaces to the interface table
	for _, intf := range intfList {
		intfUUIDStr := fmt.Sprintf("Intf%s", intf)
		intfUUID := []libovsdb.UUID{{GoUuid: intfUUIDStr}}
		intfUUIDList = append(intfUUIDList, intfUUID...)

		// insert/delete a row in Interface table
		intfOp := libovsdb.Operation{}
		iface := make(map[string]interface{})
		iface["name"] = intf

		// interface table ops
		intfOp = libovsdb.Operation{
			Op:       opStr,
			Table:    interfaceTable,
			Row:      iface,
			UUIDName: intfUUIDStr,
		}
		ops = append(ops, intfOp)
	}

	// Insert bond information in Port table
	portOp := libovsdb.Operation{}
	port := make(map[string]interface{})
	port["name"] = bondName
	port["vlan_mode"] = "trunk"
	port["interfaces"], err = libovsdb.NewOvsSet(intfUUIDList)
	if err != nil {
		return err
	}

	// Set LACP and Hash properties
	// "balance-tcp" - balances flows among slaves based on L2, L3, and L4 protocol information such as
	// destination MAC address, IP address, and TCP port
	// lacp-fallback-ab:true - Fall back to activ-backup mode when LACP negotiation fails
	port["bond_mode"] = "balance-tcp"

	port["lacp"] = "active"
	lacpMap := make(map[string]string)
	lacpMap["lacp-fallback-ab"] = "true"
	port["other_config"], err = libovsdb.NewOvsMap(lacpMap)

	portUUIDStr := bondName
	portUUID := []libovsdb.UUID{{GoUuid: portUUIDStr}}
	portOp = libovsdb.Operation{
		Op:       opStr,
		Table:    portTable,
		Row:      port,
		UUIDName: portUUIDStr,
	}
	ops = append(ops, portOp)

	// Mutate the Ports column of the row in the Bridge table to include bond name
	mutateSet, _ := libovsdb.NewOvsSet(portUUID)
	mutation := libovsdb.NewMutation("ports", opStr, mutateSet)
	condition := libovsdb.NewCondition("name", "==", d.bridgeName)
	mutateOp := libovsdb.Operation{
		Op:        "mutate",
		Table:     bridgeTable,
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}
	ops = append(ops, mutateOp)

	return d.performOvsdbOps(ops)

}

// DeletePortBond deletes a port bond from OVS
func (d *OvsdbDriver) DeletePortBond(bondName string, intfList []string) error {

	var ops []libovsdb.Operation
	var condition []interface{}
	portUUIDStr := bondName
	portUUID := []libovsdb.UUID{{GoUuid: portUUIDStr}}
	opStr := "delete"

	for _, intfName := range intfList {
		// insert/delete a row in Interface table
		condition = libovsdb.NewCondition("name", "==", intfName)
		intfOp := libovsdb.Operation{
			Op:    opStr,
			Table: interfaceTable,
			Where: []interface{}{condition},
		}
		ops = append(ops, intfOp)
	}

	// insert/delete a row in Port table
	condition = libovsdb.NewCondition("name", "==", bondName)
	portOp := libovsdb.Operation{
		Op:    opStr,
		Table: portTable,
		Where: []interface{}{condition},
	}
	ops = append(ops, portOp)

	// also fetch the port-uuid from cache
	d.cacheLock.RLock()
	for uuid, row := range d.cache["Port"] {
		name := row.Fields["name"].(string)
		if name == bondName {
			portUUID = []libovsdb.UUID{uuid}
			break
		}
	}
	d.cacheLock.RUnlock()

	// mutate the Ports column of the row in the Bridge table
	mutateSet, _ := libovsdb.NewOvsSet(portUUID)
	mutation := libovsdb.NewMutation("ports", opStr, mutateSet)
	condition = libovsdb.NewCondition("name", "==", d.bridgeName)
	mutateOp := libovsdb.Operation{
		Op:        "mutate",
		Table:     bridgeTable,
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}
	ops = append(ops, mutateOp)

	// Perform OVS transaction
	return d.performOvsdbOps(ops)
}

//UpdatePolicingRate will update the ingress policing rate in interface table.
func (d *OvsdbDriver) UpdatePolicingRate(intfName string, burst int, bandwidth int64) error {
	bw := int(bandwidth)
	intf := make(map[string]interface{})
	intf["ingress_policing_rate"] = bw
	intf["ingress_policing_burst"] = burst

	condition := libovsdb.NewCondition("name", "==", intfName)
	if condition == nil {
		return errors.New("error getting the new condition")
	}
	mutateOp := libovsdb.Operation{
		Op:    "update",
		Table: interfaceTable,
		Row:   intf,
		Where: []interface{}{condition},
	}

	operations := []libovsdb.Operation{mutateOp}
	return d.performOvsdbOps(operations)

}

// DeletePort deletes a port from OVS
func (d *OvsdbDriver) DeletePort(intfName string) error {
	portUUIDStr := intfName
	portUUID := []libovsdb.UUID{{GoUuid: portUUIDStr}}
	opStr := "delete"

	// insert/delete a row in Interface table
	condition := libovsdb.NewCondition("name", "==", intfName)
	intfOp := libovsdb.Operation{
		Op:    opStr,
		Table: interfaceTable,
		Where: []interface{}{condition},
	}

	// insert/delete a row in Port table
	condition = libovsdb.NewCondition("name", "==", intfName)
	portOp := libovsdb.Operation{
		Op:    opStr,
		Table: portTable,
		Where: []interface{}{condition},
	}

	// also fetch the port-uuid from cache
	d.cacheLock.RLock()
	for uuid, row := range d.cache["Port"] {
		name := row.Fields["name"].(string)
		if name == intfName {
			portUUID = []libovsdb.UUID{uuid}
			break
		}
	}
	d.cacheLock.RUnlock()

	// mutate the Ports column of the row in the Bridge table
	mutateSet, _ := libovsdb.NewOvsSet(portUUID)
	mutation := libovsdb.NewMutation("ports", opStr, mutateSet)
	condition = libovsdb.NewCondition("name", "==", d.bridgeName)
	mutateOp := libovsdb.Operation{
		Op:        "mutate",
		Table:     bridgeTable,
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}

	// Perform OVS transaction
	operations := []libovsdb.Operation{intfOp, portOp, mutateOp}
	return d.performOvsdbOps(operations)
}

// CreateVtep creates a VTEP port on the OVS
func (d *OvsdbDriver) CreateVtep(intfName string, vtepRemoteIP string) error {
	portUUIDStr := intfName
	intfUUIDStr := fmt.Sprintf("Intf%s", intfName)
	portUUID := []libovsdb.UUID{{GoUuid: portUUIDStr}}
	intfUUID := []libovsdb.UUID{{GoUuid: intfUUIDStr}}
	opStr := "insert"
	intfType := "vxlan"
	var err error

	// insert/delete a row in Interface table
	intf := make(map[string]interface{})
	intf["name"] = intfName
	intf["type"] = intfType

	// Special handling for VTEP ports
	intfOptions := make(map[string]interface{})
	intfOptions["remote_ip"] = vtepRemoteIP
	intfOptions["key"] = "flow"              // Insert VNI per flow
	intfOptions["tos"] = "inherit"           // Copy DSCP from inner to outer IP header
	intfOptions["dst_port"] = d.vxlanUDPPort // Set the UDP port for VXLAN

	intf["options"], err = libovsdb.NewOvsMap(intfOptions)
	if err != nil {
		log.Errorf("error '%s' creating options from %v \n", err, intfOptions)
		return err
	}

	// Add an entry in Interface table
	intfOp := libovsdb.Operation{
		Op:       opStr,
		Table:    interfaceTable,
		Row:      intf,
		UUIDName: intfUUIDStr,
	}

	// insert/delete a row in Port table
	port := make(map[string]interface{})
	port["name"] = intfName
	port["vlan_mode"] = "trunk"

	port["interfaces"], err = libovsdb.NewOvsSet(intfUUID)
	if err != nil {
		return err
	}

	// Add an entry in Port table
	portOp := libovsdb.Operation{
		Op:       opStr,
		Table:    portTable,
		Row:      port,
		UUIDName: portUUIDStr,
	}

	// mutate the Ports column of the row in the Bridge table
	mutateSet, _ := libovsdb.NewOvsSet(portUUID)
	mutation := libovsdb.NewMutation("ports", opStr, mutateSet)
	condition := libovsdb.NewCondition("name", "==", d.bridgeName)
	mutateOp := libovsdb.Operation{
		Op:        "mutate",
		Table:     bridgeTable,
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}

	// Perform OVS transaction
	operations := []libovsdb.Operation{intfOp, portOp, mutateOp}
	return d.performOvsdbOps(operations)
}

// DeleteVtep deletes a VTEP port
func (d *OvsdbDriver) DeleteVtep(intfName string) error {
	return d.DeletePort(intfName)
}

// AddController : Add controller configuration to OVS
func (d *OvsdbDriver) AddController(ipAddr string, portNo uint16) error {
	// Format target string
	target := fmt.Sprintf("tcp:%s:%d", ipAddr, portNo)
	ctrlerUUIDStr := fmt.Sprintf("local")
	ctrlerUUID := []libovsdb.UUID{{GoUuid: ctrlerUUIDStr}}

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
	condition := libovsdb.NewCondition("name", "==", d.bridgeName)
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
func (d *OvsdbDriver) RemoveController(target string) error {
	// FIXME:
	return nil
}

// IsControllerPresent : Check if Controller already exists
func (d *OvsdbDriver) IsControllerPresent(target string) bool {
	d.cacheLock.RLock()
	defer d.cacheLock.RUnlock()

	// walk the local cache
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

// IsPortNamePresent checks if port already exists in OVS bridge
func (d *OvsdbDriver) IsPortNamePresent(portName string) bool {
	d.cacheLock.RLock()
	defer d.cacheLock.RUnlock()

	// walk the local cache
	for tName, table := range d.cache {
		if tName == "Port" {
			for _, row := range table {
				for fieldName, value := range row.Fields {
					if fieldName == "name" {
						if value == portName {
							// Port name exists.
							return true
						}
					}
				}
			}
		}
	}

	// We could not find the port name
	return false
}

// IsIntfNamePresent checks if intf already exists in OVS bridge
func (d *OvsdbDriver) IsIntfNamePresent(intfName string) bool {
	d.cacheLock.RLock()
	defer d.cacheLock.RUnlock()

	// walk the local cache
	for tName, table := range d.cache {
		if tName == "Interface" {
			for _, row := range table {
				for fieldName, value := range row.Fields {
					if fieldName == "name" {
						if value == intfName {
							// Interface name exists.
							return true
						}
					}
				}
			}
		}
	}

	// We could not find the interface name
	return false
}

// GetOfpPortNo returns OFP port number for an interface
func (d *OvsdbDriver) GetOfpPortNo(intfName string) (uint32, error) {
	retryNo := 0
	condition := libovsdb.NewCondition("name", "==", intfName)
	selectOp := libovsdb.Operation{
		Op:    "select",
		Table: "Interface",
		Where: []interface{}{condition},
	}

	for {
		row, err := d.ovs.Transact(ovsDataBase, selectOp)

		if err == nil && len(row) > 0 && len(row[0].Rows) > 0 {
			value := row[0].Rows[0]["ofport"]
			if reflect.TypeOf(value).Kind() == reflect.Float64 {
				//retry few more time. Due to asynchronous call between
				//port creation and populating ovsdb entry for the interface
				//may not be populated instantly.
				if ofpPort := reflect.ValueOf(value).Float(); ofpPort != -1 {
					return uint32(ofpPort), nil
				}
			}
		}
		time.Sleep(300 * time.Millisecond)

		if retryNo == maxOfportRetry {
			return 0, errors.New("ofPort not found")
		}
		retryNo++
	}
}

// IsVtepPresent checks if VTEP already exists
func (d *OvsdbDriver) IsVtepPresent(remoteIP string) (bool, string) {
	d.cacheLock.RLock()
	defer d.cacheLock.RUnlock()

	// walk the local cache
	for tName, table := range d.cache {
		if tName == "Interface" {
			for _, row := range table {
				options := row.Fields["options"]
				switch optMap := options.(type) {
				case libovsdb.OvsMap:
					if optMap.GoMap["remote_ip"] == remoteIP {
						value := row.Fields["name"]
						switch t := value.(type) {
						case string:
							return true, t
						default:
							// return false, ""
						}
					}
				default:
					// return false, ""
				}
			}
		}
	}

	// We could not find the interface name
	return false, ""
}
