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
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/contiv/libovsdb"
	"github.com/contiv/netplugin/core"

	log "github.com/Sirupsen/logrus"
)

// OvsdbDriver is responsible for programming OVS using ovsdb protocol. It also
// implements the libovsdb.Notifier interface to keep cache of ovs table state.
type OvsdbDriver struct {
	bridgeName string // Name of the bridge we are operating on
	ovs        *libovsdb.OvsdbClient
	cache      map[string]map[libovsdb.UUID]libovsdb.Row
}

// NewOvsdbDriver creates a new OVSDB driver instance.
// Create one ovsdb driver instance per OVS bridge that needs to be managed
func NewOvsdbDriver(bridgeName string, failMode string) (*OvsdbDriver, error) {
	// Create a new driver instance
	d := new(OvsdbDriver)
	d.bridgeName = bridgeName

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
func (d *OvsdbDriver) Delete() {
	if d.ovs != nil {

		log.Infof("Deleting OVS bridge: %s", d.bridgeName)
		for {
			// corrects a race deleting and creating bridges at the same time
			// if the error is non-nil, assume the bridge is gone (or ovs at least does not care about it anymore).
			// until then, sleep and retry.
			if err := d.createDeleteBridge(d.bridgeName, "", operDeleteBridge); err != nil {
				break
			}

			time.Sleep(100 * time.Millisecond)
		}
		(*d.ovs).Disconnect()
	}
}

func (d *OvsdbDriver) getRootUUID() libovsdb.UUID {
	for uuid := range d.cache[rootTable] {
		return uuid
	}
	return libovsdb.UUID{}
}

func (d *OvsdbDriver) populateCache(updates libovsdb.TableUpdates) {
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

func (d *OvsdbDriver) performOvsdbOps(ops []libovsdb.Operation) (int, error) {
	reply, err := d.ovs.Transact(ovsDataBase, ops...)
	if err != nil {
		return 0, err
	}

	if len(reply) < len(ops) {
		return 0, core.Errorf("Unexpected number of replies. Expected: %d, Recvd: %d",
			len(ops), len(reply))
	}

	errors := []string{}
	totalCount := 0
	for _, o := range reply {
		if o.Error != "" {
			errors = append(errors, fmt.Sprintf("%s(%s)", o.Error, o.Details))
		}

		totalCount += o.Count
	}

	if len(errors) > 0 {
		log.Errorf("OVS operation failed for op: %+v", ops)
		err = core.Errorf("ovs operation failed. Error(s): %v", errors)
	}

	return totalCount, err
}

// Create or delete an OVS bridge instance
func (d *OvsdbDriver) createDeleteBridge(bridgeName, failMode string, op oper) error {
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
		var err error
		bridge := make(map[string]interface{})
		bridge["name"] = bridgeName

		// Enable Openflow1.3
		bridge["protocols"], err = libovsdb.NewOvsSet(protocols)
		if err != nil {
			return err
		}

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
	mutateSet, err := libovsdb.NewOvsSet(mutateUUID)
	if err != nil {
		return err
	}

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
	count, err := d.performOvsdbOps(operations)
	// corrects a race deleting and creating bridges at the same time
	// if the count is less than 2, error out because nothing was modified
	// therefore the operation (which is mutating) did not succeed. The OVSDB
	// response count will always be at least 1.
	if op == operDeleteBridge && count < 2 {
		return fmt.Errorf("Did not succeed in OVS bridge delete operation, no records modified.")
	}

	return err
}

// GetPortOrIntfNameFromID gets interface name from id
func (d *OvsdbDriver) GetPortOrIntfNameFromID(id string, isPort bool) (string, error) {
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

// CreatePort creates an OVS port
func (d *OvsdbDriver) CreatePort(intfName, intfType, id string, tag int) error {
	// intfName is assumed to be unique enough to become uuid
	portUUIDStr := intfName
	intfUUIDStr := fmt.Sprintf("Intf%s", intfName)
	portUUID := []libovsdb.UUID{libovsdb.UUID{GoUuid: portUUIDStr}}
	intfUUID := []libovsdb.UUID{libovsdb.UUID{GoUuid: intfUUIDStr}}
	opStr := "insert"

	var err error

	// insert/delete a row in Interface table
	idMap := make(map[string]string)
	intfOp := libovsdb.Operation{}
	intf := make(map[string]interface{})
	intf["name"] = intfName
	intf["type"] = intfType
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
	_, err = d.performOvsdbOps(operations)
	return err
}

// DeletePort deletes a port from OVS
func (d *OvsdbDriver) DeletePort(intfName string) error {
	portUUIDStr := intfName
	portUUID := []libovsdb.UUID{{portUUIDStr}}
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
	for uuid, row := range d.cache["Port"] {
		name := row.Fields["name"].(string)
		if name == intfName {
			portUUID = []libovsdb.UUID{uuid}
			break
		}
	}

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
	_, err := d.performOvsdbOps(operations)
	return err
}

// CreateVtep creates a VTEP port on the OVS
func (d *OvsdbDriver) CreateVtep(intfName string, vtepRemoteIP string) error {
	portUUIDStr := intfName
	intfUUIDStr := fmt.Sprintf("Intf%s", intfName)
	portUUID := []libovsdb.UUID{{portUUIDStr}}
	intfUUID := []libovsdb.UUID{{intfUUIDStr}}
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
	intfOptions["key"] = "flow" // Insert VNI per flow

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
	_, err = d.performOvsdbOps(operations)
	return err
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
	condition := libovsdb.NewCondition("name", "==", d.bridgeName)
	mutateOp := libovsdb.Operation{
		Op:        "mutate",
		Table:     "Bridge",
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}

	// Perform OVS transaction
	operations := []libovsdb.Operation{ctrlerOp, mutateOp}
	_, err := d.performOvsdbOps(operations)
	return err
}

// RemoveController : Remove controller configuration
func (d *OvsdbDriver) RemoveController(target string) error {
	// FIXME:
	return nil
}

// IsControllerPresent : Check if Controller already exists
func (d *OvsdbDriver) IsControllerPresent(target string) bool {
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
func (d *OvsdbDriver) IsPortNamePresent(intfName string) bool {
	for tName, table := range d.cache {
		if tName == "Port" {
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

// GetOfpPortNo : Return OFP port number for an interface
func (d *OvsdbDriver) GetOfpPortNo(intfName string) (uint32, error) {
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

// IsVtepPresent checks if VTEP already exists
func (d *OvsdbDriver) IsVtepPresent(remoteIP string) (bool, string) {
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
							return false, ""
						}
					}
				default:
					return false, ""
				}
			}
		}
	}

	// We could not find the interface name
	return false, ""
}
