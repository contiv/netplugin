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
func NewOvsdbDriver(bridgeName string) (*OvsdbDriver, error) {
	// Create a new driver instance
	d := new(OvsdbDriver)
	d.bridgeName = bridgeName

	// Connect to OVS
	ovs, err := libovsdb.ConnectUnix()
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
		err = d.createDeleteBridge(bridgeName, operCreateBridge)
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
		d.createDeleteBridge(d.bridgeName, operDeleteBridge)
		(*d.ovs).Disconnect()
	}

	return nil
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

	return core.Errorf("ovs operation failed. Error(s): %v", errors)
}

func (d *OvsdbDriver) createDeleteBridge(bridgeName string, op oper) error {
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

// CreateDeletePort creates or deletes a OVS port
func (d *OvsdbDriver) CreateDeletePort(portName, intfName, intfType, id string,
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
	return d.performOvsdbOps(operations)
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
