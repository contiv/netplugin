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
	"reflect"
	"strconv"
	"strings"

	"github.com/contiv/libovsdb"
	"github.com/contiv/netplugin/core"

	log "github.com/Sirupsen/logrus"
)

// implements the NetworkDriver and EndpointDriver interface for an vlan based
// openvSwitch deployment

type oper int

const (
	DATABASE            = "Open_vSwitch"
	ROOT_TABLE          = "Open_vSwitch"
	BRIDGE_TABLE        = "Bridge"
	PORT_TABLE          = "Port"
	INTERFACE_TABLE     = "Interface"
	DEFAULT_BRIDGE_NAME = "contivBridge"
	PORT_NAME_FMT       = "port%d"
	VXLAN_IFNAME_FMT    = "vxif%s%s"

	CREATE_BRIDGE oper = iota
	DELETE_BRIDGE
	CREATE_PORT
	DELETE_PORT

	GET_PORT_NAME = true
	GET_INTF_NAME = false
)

type OvsDriverConfig struct {
	Ovs struct {
		DbIp   string
		DbPort int
	}
}

// OvsDriver implements the Layer 2 Network and Endpoint Driver interfaces
// specific to vlan based open-vswitch. It also implements the
// libovsdb.Notifier interface to keep cache of ovs table state.
type OvsDriver struct {
	ovs         *libovsdb.OvsdbClient
	cache       map[string]map[libovsdb.UUID]libovsdb.Row
	stateDriver core.StateDriver
	currPortNum int // used to allocate port names. XXX: should it be user controlled?
}

func (d *OvsDriver) getRootUuid() libovsdb.UUID {
	for uuid, _ := range d.cache[ROOT_TABLE] {
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
				d.cache[table][libovsdb.UUID{uuid}] = row.New
			} else {
				delete(d.cache[table], libovsdb.UUID{uuid})
			}
		}
	}
}

func (d *OvsDriver) Update(context interface{}, tableUpdates libovsdb.TableUpdates) {
	d.populateCache(tableUpdates)
}

func (d *OvsDriver) Locked([]interface{}) {
}

func (d *OvsDriver) Stolen([]interface{}) {
}

func (d *OvsDriver) Echo([]interface{}) {
}

func (d *OvsDriver) performOvsdbOps(ops []libovsdb.Operation) error {
	reply, _ := d.ovs.Transact(DATABASE, ops...)

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
	} else {
		return core.Errorf("ovs operation failed. Error(s): %v", errors)
	}
}

func (d *OvsDriver) createDeleteBridge(bridgeName string, op oper) error {
	namedUuidStr := "netplugin"
	brUuid := []libovsdb.UUID{libovsdb.UUID{namedUuidStr}}
	opStr := "insert"
	if op != CREATE_BRIDGE {
		opStr = "delete"
	}

	// simple insert/delete operation
	brOp := libovsdb.Operation{}
	if op == CREATE_BRIDGE {
		bridge := make(map[string]interface{})
		bridge["name"] = bridgeName
		brOp = libovsdb.Operation{
			Op:       opStr,
			Table:    BRIDGE_TABLE,
			Row:      bridge,
			UUIDName: namedUuidStr,
		}
	} else {
		condition := libovsdb.NewCondition("name", "==", bridgeName)
		brOp = libovsdb.Operation{
			Op:    opStr,
			Table: BRIDGE_TABLE,
			Where: []interface{}{condition},
		}
		// also fetch the br-uuid from cache
		for uuid, row := range d.cache[BRIDGE_TABLE] {
			name := row.Fields["name"].(string)
			if name == bridgeName {
				brUuid = []libovsdb.UUID{uuid}
				break
			}
		}
	}

	// Inserting/Deleting a Bridge row in Bridge table requires mutating
	// the open_vswitch table.
	mutateUuid := brUuid
	mutateSet, _ := libovsdb.NewOvsSet(mutateUuid)
	mutation := libovsdb.NewMutation("bridges", opStr, mutateSet)
	condition := libovsdb.NewCondition("_uuid", "==", d.getRootUuid())

	// simple mutate operation
	mutateOp := libovsdb.Operation{
		Op:        "mutate",
		Table:     ROOT_TABLE,
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}

	operations := []libovsdb.Operation{brOp, mutateOp}
	return d.performOvsdbOps(operations)
}

func (d *OvsDriver) getPortName() string {
	// XXX: revisit, the port name might need to come from user. Also revisit
	// the algorithm to take care of port being deleted and reuse unused port
	// numbers
	d.currPortNum += 1
	return fmt.Sprintf(PORT_NAME_FMT, d.currPortNum)
}

func (d *OvsDriver) getPortOrIntfNameFromId(id string, isPort bool) (string, error) {
	table := PORT_TABLE
	if !isPort {
		table = INTERFACE_TABLE
	}

	for _, row := range d.cache[table] {
		if extIds, ok := row.Fields["external_ids"]; ok {
			extIdMap := extIds.(libovsdb.OvsMap).GoMap
			if portId, ok := extIdMap["endpoint-id"]; ok && portId == id {
				return row.Fields["name"].(string), nil
			}
		}
	}
	return "", core.Errorf("Ovs port/intf not found for id: %s", id)
}

func (d *OvsDriver) createDeletePort(portName, intfName, intfType, id string,
	intfOptions map[string]interface{}, tag int, op oper) error {
	// portName is assumed to be unique enough to become uuid
	portUuidStr := portName
	intfUuidStr := fmt.Sprintf("Intf%s", portName)
	portUuid := []libovsdb.UUID{libovsdb.UUID{portUuidStr}}
	intfUuid := []libovsdb.UUID{libovsdb.UUID{intfUuidStr}}
	opStr := "insert"
	if op != CREATE_PORT {
		opStr = "delete"
	}
	var err error = nil

	// insert/delete a row in Interface table
	idMap := make(map[string]string)
	intfOp := libovsdb.Operation{}
	if op == CREATE_PORT {
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
				log.Printf("error '%s' creating options from %v \n", err, intfOptions)
				return err
			}
		}
		intfOp = libovsdb.Operation{
			Op:       opStr,
			Table:    INTERFACE_TABLE,
			Row:      intf,
			UUIDName: intfUuidStr,
		}
	} else {
		condition := libovsdb.NewCondition("name", "==", intfName)
		intfOp = libovsdb.Operation{
			Op:    opStr,
			Table: INTERFACE_TABLE,
			Where: []interface{}{condition},
		}
		// also fetch the intf-uuid from cache
		for uuid, row := range d.cache[INTERFACE_TABLE] {
			name := row.Fields["name"].(string)
			if name == intfName {
				intfUuid = []libovsdb.UUID{uuid}
				break
			}
		}
	}

	// insert/delete a row in Port table
	portOp := libovsdb.Operation{}
	if op == CREATE_PORT {
		port := make(map[string]interface{})
		port["name"] = portName
		if tag != 0 {
			port["vlan_mode"] = "access"
			port["tag"] = tag
		} else {
			port["vlan_mode"] = "trunk"
		}
		port["interfaces"], err = libovsdb.NewOvsSet(intfUuid)
		if err != nil {
			return err
		}
		port["external_ids"], err = libovsdb.NewOvsMap(idMap)
		if err != nil {
			return err
		}
		portOp = libovsdb.Operation{
			Op:       opStr,
			Table:    PORT_TABLE,
			Row:      port,
			UUIDName: portUuidStr,
		}
	} else {
		condition := libovsdb.NewCondition("name", "==", portName)
		portOp = libovsdb.Operation{
			Op:    opStr,
			Table: PORT_TABLE,
			Where: []interface{}{condition},
		}
		// also fetch the port-uuid from cache
		for uuid, row := range d.cache[PORT_TABLE] {
			name := row.Fields["name"].(string)
			if name == portName {
				portUuid = []libovsdb.UUID{uuid}
				break
			}
		}
	}

	// mutate the Ports column of the row in the Bridge table
	mutateSet, _ := libovsdb.NewOvsSet(portUuid)
	mutation := libovsdb.NewMutation("ports", opStr, mutateSet)
	condition := libovsdb.NewCondition("name", "==", DEFAULT_BRIDGE_NAME)
	mutateOp := libovsdb.Operation{
		Op:        "mutate",
		Table:     BRIDGE_TABLE,
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}

	operations := []libovsdb.Operation{intfOp, portOp, mutateOp}
	return d.performOvsdbOps(operations)
}

func vxlanIfName(netId, vtepIp string) string {
	return fmt.Sprintf(VXLAN_IFNAME_FMT,
		netId, strings.Replace(vtepIp, ".", "", -1))
}

func (d *OvsDriver) createVtep(epCfg *OvsCfgEndpointState) error {
	cfgNw := OvsCfgNetworkState{}
	cfgNw.StateDriver = d.stateDriver
	err := cfgNw.Read(epCfg.NetId)
	if err != nil {
		return err
	}

	intfOptions := make(map[string]interface{})
	intfOptions["remote_ip"] = epCfg.VtepIp
	intfOptions["key"] = strconv.Itoa(cfgNw.ExtPktTag)

	intfName := vxlanIfName(epCfg.NetId, epCfg.VtepIp)
	err = d.createDeletePort(intfName, intfName, "vxlan", cfgNw.Id,
		intfOptions, cfgNw.PktTag, CREATE_PORT)
	if err != nil {
		log.Printf("error '%s' creating vxlan peer intfName %s, options %s, tag %d \n",
			err, intfName, intfOptions, cfgNw.PktTag)
		return err
	}

	return nil
}

func (d *OvsDriver) deleteVtep(epOper *OvsOperEndpointState) error {

	cfgNw := OvsCfgNetworkState{}
	cfgNw.StateDriver = d.stateDriver
	err := cfgNw.Read(epOper.NetId)
	if err != nil {
		return err
	}

	intfName := vxlanIfName(epOper.NetId, epOper.VtepIp)
	err = d.createDeletePort(intfName, intfName, "vxlan", cfgNw.Id,
		nil, cfgNw.PktTag, DELETE_PORT)
	if err != nil {
		log.Printf("error '%s' deleting vxlan peer intfName %s, tag %d \n",
			err, intfName, cfgNw.PktTag)
		return err
	}

	return nil
}

func (d *OvsDriver) Init(config *core.Config, stateDriver core.StateDriver) error {

	if config == nil || stateDriver == nil {
		return core.Errorf("Invalid arguments. cfg: %v, stateDriver: %v",
			config, stateDriver)
	}

	cfg, ok := config.V.(*OvsDriverConfig)
	if !ok {
		return core.Errorf("Invalid type passed")
	}

	ovs, err := libovsdb.Connect(cfg.Ovs.DbIp, cfg.Ovs.DbPort)
	if err != nil {
		return err
	}

	d.ovs = ovs
	d.stateDriver = stateDriver
	d.cache = make(map[string]map[libovsdb.UUID]libovsdb.Row)
	d.ovs.Register(d)
	initial, _ := d.ovs.MonitorAll(DATABASE, "")
	d.populateCache(*initial)

	// Create a bridge after registering for events as we depend on ovsdb cache.
	// Since the same dirver is used as endpoint driver, only create the bridge
	// if it's not already created
	// XXX: revisit if the bridge-name needs to be configurable
	brCreated := false
	for _, row := range d.cache[BRIDGE_TABLE] {
		if row.Fields["name"] == DEFAULT_BRIDGE_NAME {
			brCreated = true
			break
		}
	}

	if !brCreated {
		err = d.createDeleteBridge(DEFAULT_BRIDGE_NAME, CREATE_BRIDGE)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *OvsDriver) Deinit() {
	if d.ovs != nil {
		d.createDeleteBridge(DEFAULT_BRIDGE_NAME, DELETE_BRIDGE)
		(*d.ovs).Disconnect()
	}
}

func (d *OvsDriver) CreateNetwork(id string) error {
	cfgNw := OvsCfgNetworkState{}
	cfgNw.StateDriver = d.stateDriver
	err := cfgNw.Read(id)
	if err != nil {
		log.Printf("Failed to read net %s \n", cfgNw.Id)
		return err
	}
	log.Printf("create net %s \n", cfgNw.Id)

	return nil
}

func (d *OvsDriver) DeleteNetwork(id string) error {

	// no driver operation for network delete
	log.Printf("delete net %s \n", id)

	return nil
}

func (d *OvsDriver) CreateEndpoint(id string) error {
	var err error

	// add an internal ovs port with vlan-tag information from the state
	portName := d.getPortName()
	intfName := portName
	intfType := "internal"

	epCfg := OvsCfgEndpointState{}
	epCfg.StateDriver = d.stateDriver
	err = epCfg.Read(id)
	if err != nil {
		return err
	}

	if epCfg.VtepIp != "" {
		err = d.createVtep(&epCfg)
		if err != nil {
			log.Printf("error '%s' creating vtep interface(s) for "+
				"remote endpoint %s\n", err, epCfg.VtepIp)
		}
		return err
	}

	// use the user provided interface name. The primary usecase for such
	// endpoints is for adding the host-interfaces to the ovs bridge.
	// But other usecases might involve user created linux interface
	// devices for containers like SRIOV, that need to be bridged using ovs
	// Also, if the interface name is provided by user then we don't create
	// ovs-internal interface
	if epCfg.IntfName != "" {
		intfName = epCfg.IntfName
		intfType = ""
	}

	cfgNw := OvsCfgNetworkState{}
	cfgNw.StateDriver = d.stateDriver
	err = cfgNw.Read(epCfg.NetId)
	if err != nil {
		return err
	}

	// TODO: some updates may mean implicit delete of the previous state
	err = d.createDeletePort(portName, intfName, intfType, epCfg.Id,
		nil, cfgNw.PktTag, CREATE_PORT)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			d.createDeletePort(portName, intfName, intfType, "", nil, 0, DELETE_PORT)
		}
	}()

	operEp := OvsOperEndpointState{
		PortName:   portName,
		NetId:      epCfg.NetId,
		AttachUUID: epCfg.AttachUUID,
		ContName:   epCfg.ContName,
		IpAddress:  epCfg.IpAddress,
		IntfName:   intfName,
		HomingHost: epCfg.HomingHost,
		VtepIp:     epCfg.VtepIp}
	operEp.StateDriver = d.stateDriver
	operEp.Id = id
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

func (d *OvsDriver) DeleteEndpoint(id string) (err error) {

	epOper := OvsOperEndpointState{}
	epOper.StateDriver = d.stateDriver
	err = epOper.Read(id)
	if err != nil {
		return err
	}
	defer func() {
		epOper.Clear()
	}()

	if epOper.VtepIp != "" {
		err = d.deleteVtep(&epOper)
		if err != nil {
			log.Printf("error '%s' deleting vtep interface(s) for "+
				"remote endpoint %s\n", err, epOper.VtepIp)
		}
		return
	}

	portName, err := d.getPortOrIntfNameFromId(epOper.Id, GET_PORT_NAME)
	if err != nil {
		return err
	}

	intfName := ""
	intfName, err = d.getPortOrIntfNameFromId(epOper.Id, GET_INTF_NAME)
	if err != nil {
		return err
	}

	err = d.createDeletePort(portName, intfName, "", "", nil, 0, DELETE_PORT)
	if err != nil {
		return err
	}

	return nil
}

func (d *OvsDriver) MakeEndpointAddress() (*core.Address, error) {
	return nil, core.Errorf("Not supported")
}
