package ovs

import "github.com/contiv/netplugin/drivers"

const (
	operCreateBridge oper = iota
	operDeleteBridge
	operCreatePort
	operDeletePort
)

const (
	ovsDataBase       = "Open_vSwitch"
	rootTable         = "Open_vSwitch"
	bridgeTable       = "Bridge"
	portTable         = "Port"
	interfaceTable    = "Interface"
	defaultBridgeName = "contivBridge"
	portNameFmt       = "port%d"
	vxlanIfNameFmt    = "vxif%s%s"

	getPortName = true
	getIntfName = false

	ovsOperPathPrefix = drivers.StateOperPath + "ovs-driver/"
	ovsOperPath       = ovsOperPathPrefix + "%s"
)
