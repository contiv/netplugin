package drivers

import (
	"encoding/json"
	"fmt"

	"github.com/mapuri/netplugin/core"
)

// implements the State interface for an endpoint implemented using
// vlans with ovs. The state is stored as Json objects.

type OvsCfgEndpointState struct {
	stateDriver core.StateDriver `json:"-"`
	Id          string           `json:"id"`
	NetId       string           `json:"netId"`
	VlanTag     int              `json:"vlanTag"`
}

func (s *OvsCfgEndpointState) Write() error {
	return &core.Error{Desc: "Not implemented, shouldn't be called from plugin!"}
}

func (s *OvsCfgEndpointState) Read(id string) error {
	key := fmt.Sprintf(EP_CFG_PATH, id)
	state := core.State(s)
	return s.stateDriver.ReadState(key, state, json.Unmarshal)
}

func (s *OvsCfgEndpointState) Clear() error {
	return &core.Error{Desc: "Not implemented, shouldn't be called from plugin!"}
}

type OvsOperEndpointState struct {
	stateDriver core.StateDriver `json:"-"`
	Id          string           `json:"id"`
	NetId       string           `json:"netId"`
	PortName    string           `json:"portName"`
}

func (s *OvsOperEndpointState) Write() error {
	key := fmt.Sprintf(EP_OPER_PATH, s.Id)
	state := core.State(s)
	return s.stateDriver.WriteState(key, state, json.Marshal)
}

func (s *OvsOperEndpointState) Read(id string) error {
	key := fmt.Sprintf(EP_OPER_PATH, id)
	state := core.State(s)
	return s.stateDriver.ReadState(key, state, json.Unmarshal)
}

func (s *OvsOperEndpointState) Clear() error {
	key := fmt.Sprintf(EP_OPER_PATH, s.Id)
	return s.stateDriver.ClearState(key)
}
