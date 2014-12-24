package drivers

import (
	"encoding/json"
	"fmt"

	"github.com/contiv/netplugin/core"
)

// implements the State interface for an endpoint implemented using
// vlans with ovs. The state is stored as Json objects.

type OvsCfgEndpointState struct {
	StateDriver core.StateDriver `json:"-"`
	Id          string           `json:"id"`
	NetId       string           `json:"netId"`
	VlanTag     int              `json:"vlanTag"`
}

func (s *OvsCfgEndpointState) Write() error {
	key := fmt.Sprintf(EP_CFG_PATH, s.Id)
	return s.StateDriver.WriteState(key, s, json.Marshal)
}

func (s *OvsCfgEndpointState) Read(id string) error {
	key := fmt.Sprintf(EP_CFG_PATH, id)
	return s.StateDriver.ReadState(key, s, json.Unmarshal)
}

func (s *OvsCfgEndpointState) Clear() error {
	key := fmt.Sprintf(EP_CFG_PATH, s.Id)
	return s.StateDriver.ClearState(key)
}

type OvsOperEndpointState struct {
	StateDriver core.StateDriver `json:"-"`
	Id          string           `json:"id"`
	NetId       string           `json:"netId"`
	PortName    string           `json:"portName"`
}

func (s *OvsOperEndpointState) Write() error {
	key := fmt.Sprintf(EP_OPER_PATH, s.Id)
	return s.StateDriver.WriteState(key, s, json.Marshal)
}

func (s *OvsOperEndpointState) Read(id string) error {
	key := fmt.Sprintf(EP_OPER_PATH, id)
	return s.StateDriver.ReadState(key, s, json.Unmarshal)
}

func (s *OvsOperEndpointState) Clear() error {
	key := fmt.Sprintf(EP_OPER_PATH, s.Id)
	return s.StateDriver.ClearState(key)
}
