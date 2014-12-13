package core

import (
	"fmt"
	"github.com/socketplane/libovsdb"
	"log"
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

	CREATE_BRIDGE oper = iota
	DELETE_BRIDGE      = iota
	CREATE_PORT        = iota
	DELETE_PORT        = iota
)

type OvsDriverConfig struct {
	Ovs struct {
		OvsDbIp   string
		OvsDbPort int
	}
}

// OvsDriver implements the Layer 2 Network and Endpoint Driver interfaces
// specific to vlan based open-vswitch. It also implements the
// libovsdb.Notifier interface to keep cache of ovs table state.
type OvsDriver struct {
	ovs         *libovsdb.OvsdbClient
	cache       map[string]map[string]libovsdb.Row
	stateDriver *StateDriver
	currPortNum int // used to allocate port names. XXX: should it be user controlled?
}

func (d *OvsDriver) getRootUuid() string {
	for uuid, _ := range d.cache[ROOT_TABLE] {
		return uuid
	}
	return ""
}

func (d *OvsDriver) populateCache(updates libovsdb.TableUpdates) {
	for table, tableUpdate := range updates.Updates {
		if _, ok := d.cache[table]; !ok {
			d.cache[table] = make(map[string]libovsdb.Row)
		}
		for uuid, row := range tableUpdate.Rows {
			empty := libovsdb.Row{}
			if !reflect.DeepEqual(row.New, empty) {
				d.cache[table][uuid] = row.New
			} else {
				delete(d.cache[table], uuid)
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

func (d *OvsDriver) createDeleteBridge(bridgeName string, op oper) error {
	namedUuid := "netplugin"
	opStr = "insert"
	if op != CREATE_BRIDGE {
		opStr = "delete"
	}

	// bridge row to insert
	bridge := make(map[string]interface{})
	bridge["name"] = bridgeName

	// simple insert/delete operation
	insertOp := libovsdb.Operation{
		Op:       opStr,
		Table:    BRIDGE_TABLE,
		Row:      bridge,
		UUIDName: namedUuid,
	}

	// Inserting/Deleting a Bridge row in Bridge table requires mutating
	// the open_vswitch table.
	mutateUuid := []libovsdb.UUID{libovsdb.UUID{namedUuid}}
	mutateSet, _ := libovsdb.NewOvsSet(mutateUuid)
	mutation := libovsdb.NewMutation("bridges", opStr, mutateSet)
	condition := libovsdb.NewCondition("_uuid", "==", libovsdb.UUID{getRootUuid()})

	// simple mutate operation
	mutateOp := libovsdb.Operation{
		Op:        "mutate",
		Table:     ROOT_TABLE,
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}

	operations := []libovsdb.Operation{insertOp, mutateOp}
	reply, _ := d.ovs.Transact(DATABASE, operations...)

	if len(reply) < len(operations) {
		log.Println("Number of Replies should be atleast equal to ",
			"number of Operations")
		return error{"Unexpected number of replies"}
	}
	ok := true
	for i, o := range reply {
		if o.Error != "" && i < len(operations) {
			log.Println("Transaction Failed due to an error :", o.Error,
				" details:", o.Details, " in ", operations[i])
			ok = false
		} else if o.Error != "" {
			log.Println("Transaction Failed due to an error :", o.Error)
			ok = false
		}
	}
	if ok {
		log.Println("Bridge operation ", opStr, " Successful: ", reply[0].UUID.GoUuid)
		return nil
	} else {
		return error{"Bridge operation ", opStr, " Addition Failed"}
	}
}

func (d *OvsDriver) getPortName() string {
	// XXX: revisit, the port name might need to come from user. Also revisit
	// the algorithm to take care of port being deleted and reuse unsed port
	// numbers
	d.currPortNum += 1
	return "port" + d.currPortNum
}

func (d *OvsDriver) getPortNameFromId(id string) string {
	for _, row := range d.cache[PORT_TABLE] {
		if portId, ok = row.Fields["external_ids"]["endpoint-id"]; ok && portId == id {
			return row.Fields["name"]
		}
	}
	return ""
}

func (d *OvsDriver) createDeletePort(portName string, tag string, id string,
	op oper) error {
	// portName is assumed to be unique enough to become uuid
	namedUuid := portName
	opStr = "insert"
	if op != CREATE_PORT {
		opStr = "delete"
	}

	// insert/delete a row in Interface table
	intf := make(map[string]interface{})
	intf["name"] = portName
	if op == CREATE_PORT {
		intf["type"] = "internal"
		idMap = make(map[string]string)
		idMap["endpoint-id"] = id
		intf["external_ids"] = libovsdb.NewOvsMap(idMap)
	}
	intfOp := libovsdb.Operation{
		Op:       opStr,
		Table:    INTERFACE_TABLE,
		Row:      intf,
		UUIDName: namedUuid,
	}

	// insert/delete a row in Port table
	port := make(map[string]interface{})
	port["name"] = portName
	if op == CREATE_PORT {
		port["vlan_mode"] = "access"
		port["tag"] = tag
		intfUuid = []libovsdb.UUID{libovsdb.UUID{namedUuid}}
		port["interfaces"] = libovsdb.NewOvsSet(intfUuid)
		port["external_ids"] = libovsdb.NewOvsMap(idMap)
	}
	portOp := libovsdb.Operation{
		Op:       opStr,
		Table:    PORT_TABLE,
		Row:      port,
		UUIDName: namedUuid,
	}

	// mutate the Ports column of the row in the Bridge table
	mutateSet, _ := libovsdb.NewOvsSet(intfUuid)
	mutation := libovsdb.NewMutation("ports", opStr, mutateSet)
	condition := libovsdb.NewCondition("name", "==", DEFAULT_BRIDGE_NAME)
	mutateOp := libovsdb.Operation{
		Op:        "mutate",
		Table:     BRIDGE_TABLE,
		Mutations: []interface{}{mutation},
		Where:     []interface{}{condition},
	}

	operations := []libovsdb.Operation{intfOp, portOp, mutateOp}
	reply, _ := d.ovs.Transact(DATABASE, operations...)

	if len(reply) < len(operations) {
		log.Println("Number of Replies should be atleast equal to ",
			"number of Operations")
		return error{"Unexpected number of replies"}
	}
	ok := true
	for i, o := range reply {
		if o.Error != "" && i < len(operations) {
			log.Println("Transaction Failed due to an error :", o.Error,
				" details:", o.Details, " in ", operations[i])
			ok = false
		} else if o.Error != "" {
			log.Println("Transaction Failed due to an error :", o.Error)
			ok = false
		}
	}
	if ok {
		log.Println("Port operation ", opStr, " Successful: ",
			reply[0].UUID.GoUuid)
		return nil
	} else {
		return error{"Port operation ", opStr, " Addition Failed"}
	}
}

func (d *OvsDriver) Init(config *Config, stateDriver *StateDriver) error {
	cfg := OvsDriverConfig(config.v)

	ovs, err := libovsdb.Connect(cfg.ovs.ovsdbIp, cfg.ovs.OvsDbPort)
	if err != nil {
		log.Println("Unable to Connect ", err)
		return err
	}

	d.ovs = ovs
	d.stateDriver = stateDriver

	// Create a bridge
	// XXX: revisit if the bridge-name needs to be configurable
	err = createDeleteBridge(d.ovs, DEFAULT_BRIDGE_NAME, CREATE_BRIDGE)
	if err != nil {
		return err
	}

	d.ovs.Register(*d)
	initial, _ := d.ovs.MonitorAll(DATABASE, "")
	d.populateCache(*initial)

	return nil
}

func (d *OvsDriver) Deinit() {
	if d.ovs != nil {
		createDeleteBridge(d.ovs, DEFAULT_BRIDGE_NAME, DELETE_BRIDGE)
		(*d.ovs).Disconnect()
	}
}

func (d *OvsDriver) CreateNetwork(id string) error {
	// no-op for a vlan based network
	return nil
}

func (d *OvsDriver) DeleteNetwork(id string) error {
	// no-op for vlan based network
	return nil
}

func (d *OvsDriver) CreateEndpoint(id string) error {
	// add an internal ovs port with vlan-tag information from the state
	portName := d.getPortName()
	cfgEpState := OvsCfgEndpointState{stateDriver: d.stateDriver}

	err := cfgEpState.Read(id)
	if err != nil {
		return err
	}

	err = createDeletePort(d.ovs, portName, cfgEpState.Id, cfgEpState.VlanTag,
		CREATE_PORT)
	if err != nil {
		return err
	}

	//all went well, update the runtime state of network and endpoint
	operEpState := OvsOperEndpointState{stateDriver: d.stateDriver, Id: id,
		PortName: portName, NetId: cfgEpState.netId}
	err = operEpState.Write()
	if err != nil {
		return err
	}

	operNwState := OvsOperNetworkState{stateDriver: d.stateDriver}
	err = operNwState.Read(cfgEpState.NetId)
	if err != nil {
		return err
	}

	operNwState.EpCount += 1
	err = operNwState.Write()
	if err != nil {
		return err
	}

	return nil
}

func (d *OvsDriver) DeleteEndpoint(id string) error {
	// delete the internal ovs port corresponding to the endpoint
	portName := d.getPortNameFromId(id)
	if portName == "" {
		return error{fmt.Sprintf("Ovs port not found for id: %s", id)}
	}

	err := createDeletePort(d.ovs, portName, 0, 0, DELETE_PORT)
	if err != nil {
		return err
	}

	operEpState := OvsOperEndpointState{stateDriver: d.stateDriver}
	err = operEpState.Read(id)
	if err != nil {
		return err
	}

	operNwState := OvsOperNetworkState{stateDriver: d.stateDriver}
	err = operNwState.Read(operEpState.NetId)
	if err != nil {
		return err
	}

	operNwState.EpCount -= 1
	err = operNwState.Write()
	if err != nil {
		return err
	}

	err = operEpState.Clear(id)
	if err != nil {
		return err
	}

	return nil
}

func (d *OvsDriver) GetEndpointAddress() (Address, error) {
	return nil, error{"Not supported"}
}
