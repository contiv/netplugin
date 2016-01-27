package drivers

import "github.com/contiv/netplugin/netmaster/mastercfg"

const (
	operCreateBridge oper = iota
	operDeleteBridge
	operCreatePort
	operDeletePort
)

const (
	ovsDataBase     = "Open_vSwitch"
	rootTable       = "Open_vSwitch"
	bridgeTable     = "Bridge"
	portTable       = "Port"
	interfaceTable  = "Interface"
	vlanBridgeName  = "contivVlanBridge"
	vxlanBridgeName = "contivVxlanBridge"
	portNameFmt     = "port%d"
	vxlanIfNameFmt  = "vxif%s"

	// StateOperPath is the path to the operations stored in state.
	ovsOperPathPrefix      = mastercfg.StateOperPath + "ovs-driver/"
	ovsOperPath            = ovsOperPathPrefix + "%s"
	networkOperPathPrefix  = mastercfg.StateOperPath + "nets/"
	endpointOperPathPrefix = mastercfg.StateOperPath + "eps/"
	networkOperPath        = networkOperPathPrefix + "%s"
	endpointOperPath       = endpointOperPathPrefix + "%s"
)
