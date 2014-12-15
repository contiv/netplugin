package core

import (
	"encoding/json"
	"fmt"
)

// implements the State interface for an endpoint implemented using
// vlans with ovs. The state is stored as Json objects.

type OvsCfgEndpointState struct {
	stateDriver *StateDriver `json:"-"`
	Id          string       `json:"id"`
	NetId       string       `json:"netId"`
	VlanTag     int          `json:"vlanTag"`
}

func (s *OvsCfgEndpointState) Write() error {
	return error{"Not implemented, shouldn't be called from plugin!"}
}

func (s *OvsCfgEndpointState) Read(id string) error {
	key = fmt.Sprintf(EP_CFG_PATH, id)
	return s.stateDriver.ReadState(key, s, json.Marshal)
}

func (s *OvsCfgEndpointState) Clear(id string) error {
	return error{"Not implemented, shouldn't be called from plugin!"}
}

type OvsOperEndpointState struct {
	stateDriver *StateDriver `json:"-"`
	Id          string       `json:"id"`
	NetId       string       `json:"netId"`
	PortName    string       `json:"portName"`
}

func (s *OvsOperEndpointState) Write() error {
	key = fmt.Sprintf(EP_OPER_PATH, id)
	return s.stateDriver.WriteState(key, s, json.Marshal)
}

func (s *OvsOperEndpointState) Read(id string) error {
	key = fmt.Sprintf(EP_OPER_PATH, id)
	return s.stateDriver.ReadState(key, s, json.Marshal)
}

func (s *OvsOperEndpointState) Clear() error {
	key = fmt.Sprintf(EP_OPER_PATH, id)
	return s.stateDriver.ClearState(key)
}
