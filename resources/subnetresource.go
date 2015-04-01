/***
Copyright 2014 Cisco Systems Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resources

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/netutils"
	"github.com/jainvipin/bitset"
)

// implements the Resource interface for an 'auto-subnet' resource.
// 'auto-subnet' resource allocates a subnet of a fixed len from a larger subnet
// specified at time of resource instantiation

const (
	AUTO_SUBNET_RSRC = "auto-subnet"
)

const (
	SUBNET_RSRC_CFG_PATH_PREFIX  = drivers.CFG_PATH + AUTO_SUBNET_RSRC + "/"
	SUBNET_RSRC_CFG_PATH         = SUBNET_RSRC_CFG_PATH_PREFIX + "%s"
	SUBNET_RSRC_OPER_PATH_PREFIX = drivers.OPER_PATH + AUTO_SUBNET_RSRC + "/"
	SUBNET_RSRC_OPER_PATH        = SUBNET_RSRC_OPER_PATH_PREFIX + "%s"
)

type AutoSubnetCfgResource struct {
	stateDriver    core.StateDriver `json:"-"`
	ResId          string           `json:"id"`
	SubnetPool     net.IP           `json:"subnetPool"`
	SubnetPoolLen  uint             `json:"subnetPoolLen"`
	AllocSubnetLen uint             `json:"allocSubnetLen"`
}

type SubnetIpLenPair struct {
	Ip  net.IP
	Len uint
}

func (r *AutoSubnetCfgResource) Write() error {
	key := fmt.Sprintf(SUBNET_RSRC_CFG_PATH, r.Id())
	return r.stateDriver.WriteState(key, r, json.Marshal)
}

func (r *AutoSubnetCfgResource) Read(id string) error {
	key := fmt.Sprintf(SUBNET_RSRC_CFG_PATH, id)
	return r.stateDriver.ReadState(key, r, json.Unmarshal)
}

func (r *AutoSubnetCfgResource) Clear() error {
	key := fmt.Sprintf(SUBNET_RSRC_CFG_PATH, r.Id())
	return r.stateDriver.ClearState(key)
}

func (r *AutoSubnetCfgResource) ReadAll() ([]core.State, error) {
	values := []*AutoSubnetCfgResource{}
	byteValues, err := r.stateDriver.ReadAll(SUBNET_RSRC_CFG_PATH_PREFIX)
	if err != nil {
		return nil, err
	}
	for _, byteValue := range byteValues {
		value := &AutoSubnetCfgResource{}
		value.SetStateDriver(r.StateDriver())
		err = json.Unmarshal(byteValue, value)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}

	stateValues := []core.State{}
	for _, val := range values {
		stateValues = append(stateValues, core.State(val))
	}
	return stateValues, nil
}

func (r *AutoSubnetCfgResource) Id() string {
	return r.ResId
}

func (r *AutoSubnetCfgResource) SetId(id string) {
	r.ResId = id
}

func (r *AutoSubnetCfgResource) StateDriver() core.StateDriver {
	return r.stateDriver
}

func (r *AutoSubnetCfgResource) SetStateDriver(stateDriver core.StateDriver) {
	r.stateDriver = stateDriver
}

func (r *AutoSubnetCfgResource) Init(rsrcCfg interface{}) error {
	cfg, ok := rsrcCfg.(*AutoSubnetCfgResource)
	if !ok {
		return &core.Error{Desc: "Invalid type for subnet resource config"}
	}
	r.SubnetPool = cfg.SubnetPool
	r.SubnetPoolLen = cfg.SubnetPoolLen
	r.AllocSubnetLen = cfg.AllocSubnetLen

	if cfg.AllocSubnetLen < cfg.SubnetPoolLen {
		return &core.Error{Desc: "AllocSubnetLen should be greater than or equal to SubnetPoolLen"}
	}

	err := r.Write()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			r.Clear()
		}
	}()

	allocSubnetSize := r.AllocSubnetLen - r.SubnetPoolLen
	oper := AutoSubnetOperResource{StateDriver: r.StateDriver(), Id: r.Id(),
		FreeSubnets: netutils.CreateBitset(allocSubnetSize).Complement()}
	err = oper.Write()
	if err != nil {
		return err
	}

	return nil
}

func (r *AutoSubnetCfgResource) Deinit() {
	oper := AutoSubnetOperResource{StateDriver: r.StateDriver()}
	err := oper.Read(r.Id())
	if err != nil {
		// continue cleanup
	} else {
		err = oper.Clear()
		if err != nil {
			// continue cleanup
		}
	}

	r.Clear()
}

func (r *AutoSubnetCfgResource) Description() string {
	return AUTO_SUBNET_RSRC
}

func (r *AutoSubnetCfgResource) Allocate() (interface{}, error) {
	oper := &AutoSubnetOperResource{StateDriver: r.StateDriver()}
	err := oper.Read(r.Id())
	if err != nil {
		return nil, err
	}

	subnet, ok := oper.FreeSubnets.NextSet(0)
	if !ok {
		return nil, &core.Error{Desc: "no subnets available."}
	}

	oper.FreeSubnets.Clear(subnet)

	var subnetIp string
	subnetIp, err = netutils.GetSubnetIp(r.SubnetPool.String(), r.SubnetPoolLen,
		r.AllocSubnetLen, subnet)
	if err != nil {
		return nil, err
	}

	pair := SubnetIpLenPair{Ip: net.ParseIP(subnetIp), Len: r.AllocSubnetLen}
	err = oper.Write()
	if err != nil {
		return nil, err
	}
	return pair, nil
}

func (r *AutoSubnetCfgResource) Deallocate(value interface{}) error {
	oper := &AutoSubnetOperResource{StateDriver: r.StateDriver()}
	err := oper.Read(r.Id())
	if err != nil {
		return err
	}

	pair, ok := value.(SubnetIpLenPair)
	if !ok {
		return &core.Error{Desc: "Invalid type for subnet value"}
	}

	if pair.Len != r.AllocSubnetLen {
		return &core.Error{Desc: fmt.Sprintf("Invalid subnet length. Exp: %d Rcvd: %d",
			r.AllocSubnetLen, pair.Len)}
	}

	var subnet uint
	subnet, err = netutils.GetIpNumber(r.SubnetPool.String(), r.SubnetPoolLen,
		pair.Len, pair.Ip.String())
	if err != nil {
		return err
	}

	if oper.FreeSubnets.Test(subnet) {
		return nil
	}
	oper.FreeSubnets.Set(subnet)

	err = oper.Write()
	if err != nil {
		return err
	}
	return nil
}

type AutoSubnetOperResource struct {
	StateDriver core.StateDriver `json:"-"`
	Id          string           `json:"id"`
	FreeSubnets *bitset.BitSet   `json:"freeSubnets"`
}

func (r *AutoSubnetOperResource) Write() error {
	key := fmt.Sprintf(SUBNET_RSRC_OPER_PATH, r.Id)
	return r.StateDriver.WriteState(key, r, json.Marshal)
}

func (r *AutoSubnetOperResource) Read(id string) error {
	key := fmt.Sprintf(SUBNET_RSRC_OPER_PATH, id)
	return r.StateDriver.ReadState(key, r, json.Unmarshal)
}

func (r *AutoSubnetOperResource) Clear() error {
	key := fmt.Sprintf(SUBNET_RSRC_OPER_PATH, r.Id)
	return r.StateDriver.ClearState(key)
}
