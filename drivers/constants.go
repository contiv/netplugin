package drivers

const (
	// StateBasePath is the base path for all state operations.
	StateBasePath = "/contiv/"

	// StateConfigPath is the path to the root of the configuration state
	StateConfigPath = StateBasePath + "config/"

	// StateOperPath is the path to the operations stored in state.
	StateOperPath = StateBasePath + "oper/"

	// NetworkConfigPathPrefix is a the prefix for all network configuration
	NetworkConfigPathPrefix = StateConfigPath + "nets/"

	// NetworkConfigPath is a format string used to template new additions to the
	// configuration.
	NetworkConfigPath = NetworkConfigPathPrefix + "%s"

	// EndpointConfigPathPrefix is a the prefix for all endpoint configuration
	EndpointConfigPathPrefix = StateConfigPath + "eps/"

	// EndpointConfigPath is a format string used to template new additions to the
	// configuration.
	EndpointConfigPath = EndpointConfigPathPrefix + "%s"

	// NetworkOperPathPrefix is the prefix for all network operation entries.
	NetworkOperPathPrefix = StateOperPath + "nets/"

	// NetworkOperPath is a format string used to template operations
	NetworkOperPath = NetworkOperPathPrefix + "%s"

	// EndpointOperPathPrefix is the prefix for all endpoint operation entries.
	EndpointOperPathPrefix = StateOperPath + "eps/"

	// EndpointOperPath is a format string used for location specific operations.
	EndpointOperPath = EndpointOperPathPrefix + "%s"
)
