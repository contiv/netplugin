package drivers

const (
	operCreateBridge oper = iota
	operDeleteBridge
	operCreatePort
	operDeletePort
)

const (
	// StateBasePath is the base path for all state operations.
	StateBasePath = "/contiv/"
	// StateConfigPath is the path to the root of the configuration state
	StateConfigPath = StateBasePath + "config/"
	// StateOperPath is the path to the operations stored in state.
	StateOperPath = StateBasePath + "oper/"

	networkConfigPathPrefix  = StateConfigPath + "nets/"
	networkConfigPath        = networkConfigPathPrefix + "%s"
	endpointConfigPathPrefix = StateConfigPath + "eps/"
	endpointConfigPath       = endpointConfigPathPrefix + "%s"
	networkOperPathPrefix    = StateOperPath + "nets/"
	networkOperPath          = networkOperPathPrefix + "%s"
	endpointOperPathPrefix   = StateOperPath + "eps/"
	endpointOperPath         = endpointOperPathPrefix + "%s"
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

	getPortName = true
	getIntfName = false

	ovsOperPathPrefix = StateOperPath + "ovs-driver/"
	ovsOperPath       = ovsOperPathPrefix + "%s"
)
