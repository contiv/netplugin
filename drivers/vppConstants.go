package drivers

import "github.com/contiv/netplugin/netmaster/mastercfg"

const (
	// StateOperPath is the path to the operations stored in state.
	vppOperPathPrefix         = mastercfg.StateOperPath + "vpp-driver/"
	vppOperPath               = vppOperPathPrefix + "%s"
	vppNetworkOperPathPrefix  = mastercfg.StateOperPath + "nets/"
	vppEndpointOperPathPrefix = mastercfg.StateOperPath + "eps/"
	vppNetworkOperPath        = vppNetworkOperPathPrefix + "%s"
	vppEndpointOperPath       = vppEndpointOperPathPrefix + "%s"
)
