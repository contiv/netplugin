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

package gstate

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"

	"github.com/jainvipin/bitset"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netutils"
	"github.com/contiv/netplugin/resources"
)

const (
	BASE_GLOBAL        = "/contiv/"
	CFG_GLOBAL_PREFIX  = BASE_GLOBAL + "config/global/"
	CFG_GLOBAL_PATH    = CFG_GLOBAL_PREFIX + "%s"
	OPER_GLOBAL_PREFIX = BASE_GLOBAL + "oper/global/"
	OPER_GLOBAL_PATH   = OPER_GLOBAL_PREFIX + "%s"
)

const (
	VersionBeta1 = "0.01"
)

// specifies various parameters to choose the auto allocation values to pick from
// this allows mostly hands-free allocation of networks, endpoints, attach/detach
// operations without having to specify these each time an entity gets created
type AutoParams struct {
	SubnetPool     string `json:"subnetPool"`
	SubnetLen      uint   `json:"subnetLen"`
	AllocSubnetLen uint   `json:"AllocSubnetLen"`
	Vlans          string `json:"Vlans"`
	Vxlans         string `json:"Vxlans"`
}

// specifies parameters that decides the deployment choices
type DeployParams struct {
	DefaultNetType string `json:"defaultNetType"`
}

// global state of the network plugin
type Cfg struct {
	StateDriver core.StateDriver `json:"-"`
	Version     string           `json:"version"`
	Tenant      string           `json:"tenant"`
	Auto        AutoParams       `json:auto"`
	Deploy      DeployParams     `json:"deploy"`
}

type Oper struct {
	StateDriver     core.StateDriver `json:"-"`
	Tenant          string           `json:"tenant"`
	FreeVxlansStart uint             `json:"freeVxlansStart"`
}

func (gc *Cfg) Dump() error {
	log.Printf("Global State %v \n", gc)
	return nil
}

func (gc *Cfg) checkErrors() error {
	var err error

	if net.ParseIP(gc.Auto.SubnetPool) == nil {
		return errors.New(fmt.Sprintf("invalid ip address pool %s",
			gc.Auto.SubnetPool))
	}

	_, err = netutils.ParseTagRanges(gc.Auto.Vlans, "vlan")
	if err != nil {
		return err
	}

	_, err = netutils.ParseTagRanges(gc.Auto.Vxlans, "vxlan")
	if err != nil {
		return err
	}

	if gc.Deploy.DefaultNetType != "vlan" &&
		gc.Deploy.DefaultNetType != "vxlan" {
		return errors.New(fmt.Sprintf("unsupported net type %s",
			gc.Deploy.DefaultNetType))
	}

	if gc.Auto.SubnetLen > gc.Auto.AllocSubnetLen {
		return errors.New(fmt.Sprintf(
			"subnet size %d is smaller than subnets to be allocated from it",
			gc.Auto.SubnetLen, gc.Auto.AllocSubnetLen))
	}
	return err
}

func Parse(configBytes []byte) (*Cfg, error) {
	var gc Cfg

	err := json.Unmarshal(configBytes, &gc)
	if err != nil {
		return nil, err
	}

	err = gc.checkErrors()
	if err != nil {
		return nil, err
	}

	return &gc, err
}

func (gc *Cfg) Write() error {
	key := fmt.Sprintf(CFG_GLOBAL_PATH, gc.Tenant)
	return gc.StateDriver.WriteState(key, gc, json.Marshal)
}

func (gc *Cfg) Read(tenant string) error {
	key := fmt.Sprintf(CFG_GLOBAL_PATH, tenant)
	return gc.StateDriver.ReadState(key, gc, json.Unmarshal)
}

func ReadAllGlobalCfg(d core.StateDriver) ([]*Cfg, error) {
	values := []*Cfg{}
	byteValues, err := d.ReadAll(CFG_GLOBAL_PREFIX)
	if err != nil {
		return nil, err
	}
	for _, byteValue := range byteValues {
		value := &Cfg{StateDriver: d}
		err = json.Unmarshal(byteValue, value)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func (gc *Cfg) Clear() error {
	key := fmt.Sprintf(CFG_GLOBAL_PATH, gc.Tenant)
	return gc.StateDriver.ClearState(key)
}

func (g *Oper) Write() error {
	key := fmt.Sprintf(OPER_GLOBAL_PATH, g.Tenant)
	return g.StateDriver.WriteState(key, g, json.Marshal)
}

func (g *Oper) Read(tenant string) error {
	key := fmt.Sprintf(OPER_GLOBAL_PATH, tenant)
	return g.StateDriver.ReadState(key, g, json.Unmarshal)
}

func (g *Oper) Clear() error {
	key := fmt.Sprintf(OPER_GLOBAL_PATH, g.Tenant)
	return g.StateDriver.ClearState(key)
}

func (gc *Cfg) initVxlanBitset(vxlans string) (*resources.AutoVxlanCfgResource,
	uint, error) {

	vxlanRsrcCfg := &resources.AutoVxlanCfgResource{}
	vxlanRsrcCfg.Vxlans = netutils.CreateBitset(14)

	vxlanRange := netutils.TagRange{}
	vxlanRanges, err := netutils.ParseTagRanges(vxlans, "vxlan")
	if err != nil {
		return nil, 0, err
	}
	// XXX: REVISIT, we seem to accept one contiguous vxlan range
	vxlanRange = vxlanRanges[0]

	freeVxlansStart := uint(vxlanRange.Min)
	for vxlan := vxlanRange.Min; vxlan <= vxlanRange.Max; vxlan++ {
		vxlanRsrcCfg.Vxlans.Set(uint(vxlan - vxlanRange.Min))
	}

	// XXX: we should derive free-vlans by looking at all tenants,
	// instead of just one.
	vlanRsrcCfg := &resources.AutoVlanCfgResource{}
	vlanRsrcCfg.SetStateDriver(gc.StateDriver)
	err = vlanRsrcCfg.Read(gc.Tenant)
	if core.ErrIfKeyExists(err) != nil {
		return nil, 0, err
	} else if err != nil {
		// a vlan resource has not been defined, assume entire space available
		// for vxlan.
		vlanRsrcCfg.Vlans = netutils.CreateBitset(12)
		vlanRsrcCfg.Vlans.ClearAll()
	}
	// available vlans are the ones that are configured to be not consumed by
	// vlan networks
	availableVlans := vlanRsrcCfg.Vlans.Complement()
	clearReservedVlans(availableVlans)
	if count := availableVlans.Count(); int(count) < vxlanRange.Max-vxlanRange.Min {
		return nil, 0, &core.Error{Desc: fmt.Sprintf("Available free local vlans (%d) is less than possible vxlans (%d)",
			count, vxlanRange.Max-vxlanRange.Min)}
	}

	vxlanRsrcCfg.LocalVlans = availableVlans

	return vxlanRsrcCfg, freeVxlansStart, nil
}

func (gc *Cfg) AllocVxlan(ra core.ResourceManager) (vxlan uint,
	localVlan uint, err error) {

	pair, err1 := ra.AllocateResourceVal(gc.Tenant, resources.AUTO_VXLAN_RSRC)
	if err1 != nil {
		return 0, 0, err1
	}

	g := Oper{StateDriver: gc.StateDriver}
	err = g.Read(gc.Tenant)
	if err != nil {
		return 0, 0, err
	}

	vxlan = pair.(resources.VxlanVlanPair).Vxlan + g.FreeVxlansStart
	localVlan = pair.(resources.VxlanVlanPair).Vlan

	return
}

func (gc *Cfg) FreeVxlan(ra core.ResourceManager, vxlan uint, localVlan uint) error {
	g := Oper{StateDriver: gc.StateDriver}
	err := g.Read(gc.Tenant)
	if err != nil {
		return nil
	}

	return ra.DeallocateResourceVal(gc.Tenant, resources.AUTO_VXLAN_RSRC,
		resources.VxlanVlanPair{
			Vxlan: vxlan - g.FreeVxlansStart,
			Vlan:  localVlan})
}

func clearReservedVlans(vlanBitset *bitset.BitSet) {
	vlanBitset.Clear(0)
	vlanBitset.Clear(4095)
}

func (gc *Cfg) initVlanBitset(vlans string) (*bitset.BitSet, error) {

	vlanBitset := netutils.CreateBitset(12)

	vlanRanges, err := netutils.ParseTagRanges(vlans, "vlan")
	if err != nil {
		return nil, err
	}

	for _, vlanRange := range vlanRanges {
		for vlan := vlanRange.Min; vlan <= vlanRange.Max; vlan++ {
			vlanBitset.Set(uint(vlan))
		}
	}
	clearReservedVlans(vlanBitset)

	return vlanBitset, nil
}

func (gc *Cfg) AllocVlan(ra core.ResourceManager) (uint, error) {
	vlan, err := ra.AllocateResourceVal(gc.Tenant, resources.AUTO_VLAN_RSRC)
	if err != nil {
		log.Printf("alloc vlan failed: %q", err)
		return 0, err
	}

	return vlan.(uint), err
}

func (gc *Cfg) FreeVlan(ra core.ResourceManager, vlan uint) error {
	return ra.DeallocateResourceVal(gc.Tenant, resources.AUTO_VLAN_RSRC, vlan)
}

func (gc *Cfg) AllocSubnet(ra core.ResourceManager) (string, error) {
	pair, err := ra.AllocateResourceVal(gc.Tenant, resources.AUTO_SUBNET_RSRC)
	if err != nil {
		return "", err
	}

	return pair.(resources.SubnetIpLenPair).Ip.String(), err
}

func (gc *Cfg) FreeSubnet(ra core.ResourceManager, subnetIp string) error {
	return ra.DeallocateResourceVal(gc.Tenant, resources.AUTO_SUBNET_RSRC,
		resources.SubnetIpLenPair{
			Ip:  net.ParseIP(subnetIp),
			Len: gc.Auto.AllocSubnetLen})
}

func (gc *Cfg) Process(ra core.ResourceManager) error {
	var err error

	if gc.Version != VersionBeta1 {
		return &core.Error{Desc: fmt.Sprintf("unsupported verison %s",
			gc.Version)}
	}

	err = gc.checkErrors()
	if err != nil {
		return &core.Error{Desc: fmt.Sprintf(
			"process failed on error checks %s \n", err)}
	}

	tenant := gc.Tenant
	if tenant == "" {
		return &core.Error{Desc: "null tenant"}
	}

	subnetRsrcCfg := &resources.AutoSubnetCfgResource{
		SubnetPool:     net.ParseIP(gc.Auto.SubnetPool),
		SubnetPoolLen:  gc.Auto.SubnetLen,
		AllocSubnetLen: gc.Auto.AllocSubnetLen}
	err = ra.DefineResource(tenant, resources.AUTO_SUBNET_RSRC, subnetRsrcCfg)
	if err != nil {
		return err
	}

	// Only define a vlan resource if a valid range was specified
	if gc.Auto.Vlans != "" {
		var vlanRsrcCfg *bitset.BitSet
		vlanRsrcCfg, err = gc.initVlanBitset(gc.Auto.Vlans)
		if err != nil {
			return err
		}
		err = ra.DefineResource(tenant, resources.AUTO_VLAN_RSRC, vlanRsrcCfg)
		if err != nil {
			return err
		}
	}

	// Only define a vxlan resource if a valid range was specified
	var freeVxlansStart uint = 0
	if gc.Auto.Vxlans != "" {
		var vxlanRsrcCfg *resources.AutoVxlanCfgResource
		vxlanRsrcCfg, freeVxlansStart, err = gc.initVxlanBitset(gc.Auto.Vxlans)
		if err != nil {
			return err
		}
		err = ra.DefineResource(tenant, resources.AUTO_VXLAN_RSRC, vxlanRsrcCfg)
		if err != nil {
			return err
		}
	}

	g := &Oper{
		StateDriver:     gc.StateDriver,
		Tenant:          gc.Tenant,
		FreeVxlansStart: freeVxlansStart}
	err = g.Write()
	if err != nil {
		log.Printf("error '%s' updating goper state %v \n", err, g)
		return err
	}

	log.Printf("updating the global config to new state %v \n", gc)
	return nil
}
