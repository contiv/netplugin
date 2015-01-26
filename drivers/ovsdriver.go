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
    "log"

	"github.com/contiv/netplugin/netutils"
	"github.com/contiv/netplugin/core"
	"github.com/socketplane/libovsdb"
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
	DEFAULT_BRIDGE_NAME = "vlanBr"
	PORT_NAME_FMT       = "port%d"

	CREATE_BRIDGE oper = iota
	DELETE_BRIDGE      = iota
	CREATE_PORT        = iota
	DELETE_PORT        = iota
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
		return &core.Error{Desc: fmt.Sprintf("Unexpected number of replies. Expected: %d, Recvd: %d",
			len(ops), len(reply))}
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
		return &core.Error{Desc: fmt.Sprintf("ovs operation failed. Error(s): %v",
			errors)}
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
	// the algorithm to take care of port being deleted and reuse unsed port
	// numbers
	d.currPortNum += 1
	return fmt.Sprintf(PORT_NAME_FMT, d.currPortNum)
}

func (d *OvsDriver) getPortNameFromId(id string) (string, error) {
	for _, row := range d.cache[PORT_TABLE] {
		if extIds, ok := row.Fields["external_ids"]; ok {
			extIdMap := extIds.(libovsdb.OvsMap).GoMap
			if portId, ok := extIdMap["endpoint-id"]; ok && portId == id {
				return row.Fields["name"].(string), nil
			}
		}
	}
	return "", &core.Error{Desc: fmt.Sprintf("Ovs port not found for id: %s", id)}
}

func (d *OvsDriver) createDeletePort(portName string, id string, tag int,
	op oper) error {
	// portName is assumed to be unique enough to become uuid
	portUuidStr := portName
	intfUuidStr := fmt.Sprintf("Inft%s", portName)
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
		intf["name"] = portName
		intf["type"] = "internal"
		idMap["endpoint-id"] = id
		intf["external_ids"], err = libovsdb.NewOvsMap(idMap)
		if err != nil {
			return err
		}
		intfOp = libovsdb.Operation{
			Op:       opStr,
			Table:    INTERFACE_TABLE,
			Row:      intf,
			UUIDName: intfUuidStr,
		}
	} else {
		condition := libovsdb.NewCondition("name", "==", portName)
		intfOp = libovsdb.Operation{
			Op:    opStr,
			Table: INTERFACE_TABLE,
			Where: []interface{}{condition},
		}
		// also fetch the intf-uuid from cache
		for uuid, row := range d.cache[INTERFACE_TABLE] {
			name := row.Fields["name"].(string)
			if name == portName {
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
		port["vlan_mode"] = "access"
		port["tag"] = tag
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

func (d *OvsDriver) Init(config *core.Config, stateDriver core.StateDriver) error {

	if config == nil || stateDriver == nil {
		return &core.Error{Desc: fmt.Sprintf("Invalid arguments. cfg: %v, stateDriver: %v", config, stateDriver)}
	}

	cfg, ok := config.V.(*OvsDriverConfig)
	if !ok {
		return &core.Error{Desc: "Invalid type passed"}
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
    var err error

	// no-op for a vlan based network, just create oper state
	cfgNetState := OvsCfgNetworkState{StateDriver: d.stateDriver}
	err = cfgNetState.Read(id)
	if err != nil {
		return err
	}

	operNwState := OvsOperNetworkState{StateDriver: d.stateDriver, 
        Id: cfgNetState.Id, PktTagType : cfgNetState.PktTagType,
        PktTag: cfgNetState.PktTag, DefaultGw: cfgNetState.DefaultGw,
        SubnetIp: cfgNetState.SubnetIp, SubnetLen: cfgNetState.SubnetLen}
    netutils.InitSubnetBitset(&operNwState.IpAllocMap, cfgNetState.SubnetLen)
	err = operNwState.Write()
	if err != nil {
		return err
	}

	return nil
}

func (d *OvsDriver) DeleteNetwork(id string) error {
	// no-op for a vlan based network, just delete oper state
	operNwState := OvsOperNetworkState{StateDriver: d.stateDriver, Id: id}
	err := operNwState.Clear()
	if err != nil {
		return err
	}

	return nil
}

// fetches various parameters in ep context to allow container to be
// associated to an ep
func (d *OvsDriver) GetEndpointContainerContext(epId string) (*core.ContainerEpContext, error) {
    var epCtx core.ContainerEpContext
    var err error

	cfgEpState := OvsCfgEndpointState{StateDriver: d.stateDriver}
	err = cfgEpState.Read(epId)
	if err != nil {
		return &epCtx, nil
	}
    epCtx.NewContId = cfgEpState.ContId

	cfgNetState := OvsCfgNetworkState{StateDriver: d.stateDriver}
	err = cfgNetState.Read(cfgEpState.NetId)
	if err != nil {
		return &epCtx, err
    }
    epCtx.DefaultGw = cfgNetState.DefaultGw
    epCtx.SubnetLen = cfgNetState.SubnetLen

    operEpState := OvsOperEndpointState{StateDriver: d.stateDriver}
    err = operEpState.Read(epId)
    if err != nil {
        return &epCtx, nil
    }
    epCtx.CurrContId = operEpState.ContId
    epCtx.InterfaceId = operEpState.PortName
    epCtx.IpAddress = operEpState.IpAddress

	return &epCtx, err
}

func (d *OvsDriver) CreateEndpoint(id string) error {
    var err error

	// add an internal ovs port with vlan-tag information from the state
	portName := d.getPortName()

	cfgEpState := OvsCfgEndpointState{StateDriver: d.stateDriver}
	err = cfgEpState.Read(id)
	if err != nil {
		return err
	}

	cfgNetState := OvsCfgNetworkState{StateDriver: d.stateDriver}
	err = cfgNetState.Read(cfgEpState.NetId)
	if err != nil {
		return err
	}

    // TODO: some updates may mean implicit delete of the previous state
	err = d.createDeletePort(portName, cfgEpState.Id, cfgNetState.PktTag,
		CREATE_PORT)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			d.createDeletePort(portName, "", 0, DELETE_PORT)
		}
	}()

    // TODO: uae etcd distributed lock to ensure atomicity of this operation
    // this is valid for EpCount, defer the unlocking for intermediate returns
	operNwState := OvsOperNetworkState{StateDriver: d.stateDriver}
	err = operNwState.Read(cfgEpState.NetId)
	if err != nil {
		return err
	}

    var ipAddrBit uint = 0
    var found bool

    ipAddress := cfgEpState.IpAddress
    if ipAddress == "auto" {
        if ipAddrBit, found = netutils.NextUnSet(&operNwState.IpAllocMap, 0); !found {
            log.Printf("auto allocation failed - address exhaustion in subnet %s/%d \n", 
                       operNwState.SubnetIp, operNwState.SubnetLen)
            return err
        }
        ipAddress, err = netutils.GetSubnetIp(
            operNwState.SubnetIp, operNwState.SubnetLen, 32, ipAddrBit)
        if err != nil {
            log.Printf("error acquiring subnet ip '%s' \n", err)
            return err
        }
        operNwState.IpAllocMap.Set(ipAddrBit)
        log.Printf("Ep %s was allocated ip address %s \n", id, ipAddress) 
    } else if ipAddress != "" && operNwState.SubnetIp != "" {
        ipAddrBit, err = netutils.GetIpNumber(
            operNwState.SubnetIp, operNwState.SubnetLen, 32, ipAddress)
        if err != nil {
            log.Printf("error getting host id from hostIp %s Subnet %s/%d err '%s'\n", 
                ipAddress, operNwState.SubnetIp, operNwState.SubnetLen, err)
            return err
        }
        operNwState.IpAllocMap.Set(ipAddrBit)
    }

    // deprecate - bitset.WordCount gives the following value
	operNwState.EpCount += 1        
	err = operNwState.Write()
	if err != nil {
		return err
	}

	//all went well, update the runtime state of network and endpoint
	operEpState := OvsOperEndpointState{
                        StateDriver: d.stateDriver, Id: id, PortName: portName, 
                        NetId: cfgEpState.NetId, ContId: cfgEpState.ContId,
                        IpAddress: ipAddress}
	err = operEpState.Write()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			operEpState.Clear()
		}
	}()

	return nil
}

func (d *OvsDriver) DeleteEndpoint(id string) error {
	// delete the internal ovs port corresponding to the endpoint
	portName, err := d.getPortNameFromId(id)
	if err != nil {
		return err
	}

	err = d.createDeletePort(portName, "", 0, DELETE_PORT)
	if err != nil {
		return err
	}

	operEpState := OvsOperEndpointState{StateDriver: d.stateDriver}
	err = operEpState.Read(id)
	if err != nil {
		return err
	}
	defer func() {
		operEpState.Clear()
	}()

	operNwState := OvsOperNetworkState{StateDriver: d.stateDriver}
	err = operNwState.Read(operEpState.NetId)
	if err != nil {
		return err
	}

	operNwState.EpCount -= 1
	err = operNwState.Write()
	if err != nil {
		return err
	}

	return nil
}

func (d *OvsDriver) MakeEndpointAddress() (*core.Address, error) {
	return nil, &core.Error{Desc: "Not supported"}
}
