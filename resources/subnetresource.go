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
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/utils/netutils"
	"github.com/jainvipin/bitset"
)

// implements the Resource interface for an 'auto-subnet' resource.
// 'auto-subnet' resource allocates a subnet of a fixed len from a larger subnet
// specified at time of resource instantiation

const (
	// AutoSubnetResource is the name of the resource, for storing state.
	AutoSubnetResource = "auto-subnet"
)

const (
	subnetResourceConfigPathPrefix = mastercfg.StateConfigPath + AutoSubnetResource + "/"
	subnetResourceConfigPath       = subnetResourceConfigPathPrefix + "%s"
	subnetResourceOperPathPrefix   = drivers.StateOperPath + AutoSubnetResource + "/"
	subnetResourceOperPath         = subnetResourceOperPathPrefix + "%s"
)

// AutoSubnetCfgResource is an implementation of core.State and core.Resource
// for configuration of the subnet.
type AutoSubnetCfgResource struct {
	core.CommonState
	SubnetPool     net.IP `json:"subnetPool"`
	SubnetPoolLen  uint   `json:"subnetPoolLen"`
	AllocSubnetLen uint   `json:"allocSubnetLen"`
}

// SubnetIPLenPair structurally represents a CIDR notated address.
type SubnetIPLenPair struct {
	IP  net.IP
	Len uint
}

// Write the state
func (r *AutoSubnetCfgResource) Write() error {
	key := fmt.Sprintf(subnetResourceConfigPath, r.ID)
	return r.StateDriver.WriteState(key, r, json.Marshal)
}

// Read the state
func (r *AutoSubnetCfgResource) Read(id string) error {
	key := fmt.Sprintf(subnetResourceConfigPath, id)
	return r.StateDriver.ReadState(key, r, json.Unmarshal)
}

// Clear the state
func (r *AutoSubnetCfgResource) Clear() error {
	key := fmt.Sprintf(subnetResourceConfigPath, r.ID)
	return r.StateDriver.ClearState(key)
}

// ReadAll state from the resource prefix.
func (r *AutoSubnetCfgResource) ReadAll() ([]core.State, error) {
	return r.StateDriver.ReadAllState(subnetResourceConfigPathPrefix, r,
		json.Unmarshal)
}

// Init the state from configuration.
func (r *AutoSubnetCfgResource) Init(rsrcCfg interface{}) error {
	cfg, ok := rsrcCfg.(*AutoSubnetCfgResource)
	if !ok {
		return core.Errorf("Invalid type for subnet resource config")
	}
	r.SubnetPool = cfg.SubnetPool
	r.SubnetPoolLen = cfg.SubnetPoolLen
	r.AllocSubnetLen = cfg.AllocSubnetLen

	if cfg.AllocSubnetLen < cfg.SubnetPoolLen {
		return core.Errorf("AllocSubnetLen should be greater than or equal to SubnetPoolLen")
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
	oper := &AutoSubnetOperResource{FreeSubnets: netutils.CreateBitset(allocSubnetSize).Complement()}
	oper.StateDriver = r.StateDriver
	oper.ID = r.ID
	err = oper.Write()
	if err != nil {
		return err
	}

	return nil
}

// Deinit the state
func (r *AutoSubnetCfgResource) Deinit() {
	oper := &AutoSubnetOperResource{}
	oper.StateDriver = r.StateDriver
	err := oper.Read(r.ID)
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

// Description returns a string that describe the type of state.
func (r *AutoSubnetCfgResource) Description() string {
	return AutoSubnetResource
}

// Allocate a new subnet. Returns an interface{} which for this method is always
// SubnetIPLenPair.
func (r *AutoSubnetCfgResource) Allocate() (interface{}, error) {
	oper := &AutoSubnetOperResource{}
	oper.StateDriver = r.StateDriver
	err := oper.Read(r.ID)
	if err != nil {
		return nil, err
	}

	subnet, ok := oper.FreeSubnets.NextSet(0)
	if !ok {
		return nil, core.Errorf("no subnets available.")
	}

	oper.FreeSubnets.Clear(subnet)

	var subnetIP string
	subnetIP, err = netutils.GetSubnetIP(r.SubnetPool.String(), r.SubnetPoolLen,
		r.AllocSubnetLen, subnet)
	if err != nil {
		return nil, err
	}

	pair := SubnetIPLenPair{IP: net.ParseIP(subnetIP), Len: r.AllocSubnetLen}
	err = oper.Write()
	if err != nil {
		return nil, err
	}
	return pair, nil
}

// Deallocate the resource. Must be passed a SubnetIPLenPair.
func (r *AutoSubnetCfgResource) Deallocate(value interface{}) error {
	oper := &AutoSubnetOperResource{}
	oper.StateDriver = r.StateDriver
	err := oper.Read(r.ID)
	if err != nil {
		return err
	}

	pair, ok := value.(SubnetIPLenPair)
	if !ok {
		return core.Errorf("Invalid type for subnet value")
	}

	if pair.Len != r.AllocSubnetLen {
		return core.Errorf("Invalid subnet length. Exp: %d Rcvd: %d",
			r.AllocSubnetLen, pair.Len)
	}

	var subnet uint
	subnet, err = netutils.GetIPNumber(r.SubnetPool.String(), r.SubnetPoolLen,
		pair.Len, pair.IP.String())
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

// AutoSubnetOperResource is an implementation of core.State relating to subnets.
type AutoSubnetOperResource struct {
	core.CommonState
	FreeSubnets *bitset.BitSet `json:"freeSubnets"`
}

// Write the state.
func (r *AutoSubnetOperResource) Write() error {
	key := fmt.Sprintf(subnetResourceOperPath, r.ID)
	return r.StateDriver.WriteState(key, r, json.Marshal)
}

// Read the state.
func (r *AutoSubnetOperResource) Read(id string) error {
	key := fmt.Sprintf(subnetResourceOperPath, id)
	return r.StateDriver.ReadState(key, r, json.Unmarshal)
}

// ReadAll state under the prefix.
func (r *AutoSubnetOperResource) ReadAll() ([]core.State, error) {
	return r.StateDriver.ReadAllState(subnetResourceOperPathPrefix, r,
		json.Unmarshal)
}

// Clear the state.
func (r *AutoSubnetOperResource) Clear() error {
	key := fmt.Sprintf(subnetResourceOperPath, r.ID)
	return r.StateDriver.ClearState(key)
}
