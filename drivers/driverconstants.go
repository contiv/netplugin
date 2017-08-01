package drivers

import "github.com/contiv/netplugin/netmaster/mastercfg"

const (
	// StateOperPath is the path to the operations stored in state.
	networkOperPathPrefix  = mastercfg.StateOperPath + "nets/"
	networkOperPath        = networkOperPathPrefix + "%s"
	endpointOperPathPrefix = mastercfg.StateOperPath + "eps/"
	endpointOperPath       = endpointOperPathPrefix + "%s"
)
