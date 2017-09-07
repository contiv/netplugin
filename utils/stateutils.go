package utils

import (
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/netmaster/mastercfg"
)

// GetEndpoint is a utility that reads the EP oper state
func GetEndpoint(epID string) (*drivers.OperEndpointState, error) {
	// Get hold of the state driver
	stateDriver, err := GetStateDriver()
	if err != nil {
		return nil, err
	}

	operEp := &drivers.OperEndpointState{}
	operEp.StateDriver = stateDriver
	err = operEp.Read(epID)
	if err != nil {
		return nil, err
	}

	return operEp, nil
}

// GetNetwork is a utility that reads the n/w oper state
func GetNetwork(networkID string) (*mastercfg.CfgNetworkState, error) {
	// Get hold of the state driver
	stateDriver, err := GetStateDriver()
	if err != nil {
		return nil, err
	}

	// find the network from network id
	nwCfg := &mastercfg.CfgNetworkState{}
	nwCfg.StateDriver = stateDriver
	err = nwCfg.Read(networkID)
	if err != nil {
		return nil, err
	}

	return nwCfg, nil
}
