package drivers

import (
	"encoding/json"
	"fmt"

	"github.com/mapuri/netplugin/core"
)

// implements the State interface for a network implemented using
// vlans with ovs. The state is stored as Json objects.
const (
	BASE_PATH          = "contiv.io/"
	CFG_PATH           = BASE_PATH + "config/"
	NW_CFG_PATH_PREFIX = CFG_PATH + "nets/"
	NW_CFG_PATH        = NW_CFG_PATH_PREFIX + "%s/"
	EP_CFG_PATH_PREFIX = CFG_PATH + "eps/"
	EP_CFG_PATH        = EP_CFG_PATH_PREFIX + "%s/"
	OPER_PATH          = BASE_PATH + "oper/"
	NW_OPER_PATH       = OPER_PATH + "nets/%s/"
	EP_OPER_PATH       = OPER_PATH + "eps/%s/"
)

type OvsCfgNetworkState struct {
	stateDriver core.StateDriver `json:"-"`
	Id          string           `json:"id"`
}

func (s *OvsCfgNetworkState) Write() error {
	return &core.Error{Desc: "Not implemented, shouldn't be called from plugin!"}
}

func (s *OvsCfgNetworkState) Read(id string) error {
	key := fmt.Sprintf(NW_CFG_PATH, id)
	state := core.State(s)
	return s.stateDriver.ReadState(key, state, json.Unmarshal)
}

func (s *OvsCfgNetworkState) Clear() error {
	return &core.Error{"Not implemented, shouldn't be called from plugin!"}
}

type OvsOperNetworkState struct {
	stateDriver core.StateDriver `json:"-"`
	Id          string           `json:"id"`
	EpCount     int              `json:"epCount"`
}

func (s *OvsOperNetworkState) Write() error {
	key := fmt.Sprintf(NW_OPER_PATH, s.Id)
	state := core.State(s)
	return s.stateDriver.WriteState(key, state, json.Marshal)
}

func (s *OvsOperNetworkState) Read(id string) error {
	key := fmt.Sprintf(NW_OPER_PATH, id)
	state := core.State(s)
	return s.stateDriver.ReadState(key, state, json.Unmarshal)
}

func (s *OvsOperNetworkState) Clear() error {
	key := fmt.Sprintf(NW_OPER_PATH, s.Id)
	return s.stateDriver.ClearState(key)
}
