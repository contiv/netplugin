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
	"fmt"
	"net"

	"github.com/jainvipin/bitset"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netutils"
	"github.com/contiv/netplugin/resources"

	log "github.com/Sirupsen/logrus"
)

const (
	baseGlobal       = "/contiv/"
	cfgGlobalPrefix  = baseGlobal + "config/global/"
	cfgGlobalPath    = cfgGlobalPrefix + "%s"
	operGlobalPrefix = baseGlobal + "oper/global/"
	operGlobalPath   = operGlobalPrefix + "%s"
)

// Version constants. Used in managing state variance.
const (
	VersionBeta1 = "0.01"
)

// AutoParams specifies various parameters for the auto allocation and resource
// management for networks and endpoints.  This allows for hands-free
// allocation of resources without having to specify these each time these
// constructs gets created.
type AutoParams struct {
	SubnetPool     string `json:"subnetPool"`
	SubnetLen      uint   `json:"subnetLen"`
	AllocSubnetLen uint   `json:"AllocSubnetLen"`
	VLANs          string `json:"VLANs"`
	VXLANs         string `json:"VXLANs"`
}

// DeployParams specifies parameters that decides the deployment choices
type DeployParams struct {
	DefaultNetType string `json:"defaultNetType"`
}

// Cfg is the configuration of a tenant.
type Cfg struct {
	core.CommonState
	Version string       `json:"version"`
	Tenant  string       `json:"tenant"`
	Auto    AutoParams   `json:"auto"`
	Deploy  DeployParams `json:"deploy"`
}

// Oper encapsulates operations on a tenant.
type Oper struct {
	core.CommonState
	Tenant          string `json:"tenant"`
	FreeVXLANsStart uint   `json:"freeVXLANsStart"`
}

// Dump is a debugging utility.
func (gc *Cfg) Dump() error {
	log.Debugf("Global State %v \n", gc)
	return nil
}

func (gc *Cfg) checkErrors() error {
	var err error

	if net.ParseIP(gc.Auto.SubnetPool) == nil {
		return core.Errorf("invalid ip address pool %s", gc.Auto.SubnetPool)
	}

	_, err = netutils.ParseTagRanges(gc.Auto.VLANs, "vlan")
	if err != nil {
		return err
	}

	_, err = netutils.ParseTagRanges(gc.Auto.VXLANs, "vxlan")
	if err != nil {
		return err
	}

	if gc.Deploy.DefaultNetType != "vlan" &&
		gc.Deploy.DefaultNetType != "vxlan" {
		return core.Errorf("unsupported net type %s", gc.Deploy.DefaultNetType)
	}

	if gc.Auto.SubnetLen > gc.Auto.AllocSubnetLen {
		return core.Errorf("subnet size %d is smaller than subnets (%d) to be allocated from it",
			gc.Auto.SubnetLen, gc.Auto.AllocSubnetLen)
	}
	return err
}

// Parse parses a JSON config into a *gstate.Cfg.
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

// Write the state
func (gc *Cfg) Write() error {
	key := fmt.Sprintf(cfgGlobalPath, gc.Tenant)
	return gc.StateDriver.WriteState(key, gc, json.Marshal)
}

// Read the state
func (gc *Cfg) Read(tenant string) error {
	key := fmt.Sprintf(cfgGlobalPath, tenant)
	return gc.StateDriver.ReadState(key, gc, json.Unmarshal)
}

// ReadAll global config state
func (gc *Cfg) ReadAll() ([]core.State, error) {
	return gc.StateDriver.ReadAllState(cfgGlobalPrefix, gc, json.Unmarshal)
}

// Clear the state
func (gc *Cfg) Clear() error {
	key := fmt.Sprintf(cfgGlobalPath, gc.Tenant)
	return gc.StateDriver.ClearState(key)
}

// Write the state
func (g *Oper) Write() error {
	key := fmt.Sprintf(operGlobalPath, g.Tenant)
	return g.StateDriver.WriteState(key, g, json.Marshal)
}

// Read the state
func (g *Oper) Read(tenant string) error {
	key := fmt.Sprintf(operGlobalPath, tenant)
	return g.StateDriver.ReadState(key, g, json.Unmarshal)
}

// ReadAll the global oper state
func (g *Oper) ReadAll() ([]core.State, error) {
	return g.StateDriver.ReadAllState(operGlobalPrefix, g, json.Unmarshal)
}

// Clear the state.
func (g *Oper) Clear() error {
	key := fmt.Sprintf(operGlobalPath, g.Tenant)
	return g.StateDriver.ClearState(key)
}

// The following function derives the number of available vlan tags.
// XXX: Since it is run at netmaster, it is guaranteed to have a consistent view of
// resource usage. Revisit if this assumption changes, as then it might need to
// be moved to resource-manager
func deriveAvailableVLANs(stateDriver core.StateDriver) (*bitset.BitSet, error) {
	// available vlans = vlan-space - For each tenant (vlans + local-vxlan-vlans)
	availableVLANs := netutils.CreateBitset(12)

	// get all vlans
	readVLANRsrc := &resources.AutoVLANCfgResource{}
	readVLANRsrc.StateDriver = stateDriver
	vlanRsrcs, err := readVLANRsrc.ReadAll()
	if core.ErrIfKeyExists(err) != nil {
		return nil, err
	} else if err != nil {
		vlanRsrcs = []core.State{}
	}
	for _, rsrc := range vlanRsrcs {
		cfg := rsrc.(*resources.AutoVLANCfgResource)
		availableVLANs = availableVLANs.Union(cfg.VLANs)
	}

	//get all vxlan-vlans
	readVXLANRsrc := &resources.AutoVXLANCfgResource{}
	readVXLANRsrc.StateDriver = stateDriver
	vxlanRsrcs, err := readVXLANRsrc.ReadAll()
	if core.ErrIfKeyExists(err) != nil {
		return nil, err
	} else if err != nil {
		vxlanRsrcs = []core.State{}
	}
	for _, rsrc := range vxlanRsrcs {
		cfg := rsrc.(*resources.AutoVXLANCfgResource)
		availableVLANs = availableVLANs.Union(cfg.LocalVLANs)
	}

	// subtract to get availableVLANs
	availableVLANs = availableVLANs.Complement()
	clearReservedVLANs(availableVLANs)
	return availableVLANs, nil
}

func (gc *Cfg) initVXLANBitset(vxlans string) (*resources.AutoVXLANCfgResource,
	uint, error) {

	vxlanRsrcCfg := &resources.AutoVXLANCfgResource{}
	vxlanRsrcCfg.VXLANs = netutils.CreateBitset(14)

	vxlanRange := netutils.TagRange{}
	vxlanRanges, err := netutils.ParseTagRanges(vxlans, "vxlan")
	if err != nil {
		return nil, 0, err
	}
	// XXX: REVISIT, we seem to accept one contiguous vxlan range
	vxlanRange = vxlanRanges[0]

	freeVXLANsStart := uint(vxlanRange.Min)
	for vxlan := vxlanRange.Min; vxlan <= vxlanRange.Max; vxlan++ {
		vxlanRsrcCfg.VXLANs.Set(uint(vxlan - vxlanRange.Min))
	}

	availableVLANs, err := deriveAvailableVLANs(gc.StateDriver)
	if err != nil {
		return nil, 0, err
	}

	localVLANsReqd := vxlanRange.Max - vxlanRange.Min + 1
	if count := availableVLANs.Count(); int(count) < localVLANsReqd {
		return nil, 0, core.Errorf("Available free local vlans (%d) is less than possible vxlans (%d)",
			count, vxlanRange.Max-vxlanRange.Min)
	} else if int(count) > localVLANsReqd {
		//only reserve the #vxlan amount of bits
		var clearBitMarker uint
		for i := 0; i < localVLANsReqd; i++ {
			clearBitMarker, _ = availableVLANs.NextSet(clearBitMarker)
			clearBitMarker++
		}
		clearBitMarker++
		for {
			if bit, ok := availableVLANs.NextSet(clearBitMarker); ok {
				availableVLANs.Clear(bit)
			} else {
				break
			}
		}
	}

	vxlanRsrcCfg.LocalVLANs = availableVLANs

	return vxlanRsrcCfg, freeVXLANsStart, nil
}

// AllocVXLAN allocates a new vxlan; ids for both the vxlan and vlan are returned.
func (gc *Cfg) AllocVXLAN(ra core.ResourceManager) (vxlan uint,
	localVLAN uint, err error) {

	pair, err1 := ra.AllocateResourceVal(gc.Tenant, resources.AutoVXLANResource)
	if err1 != nil {
		return 0, 0, err1
	}

	g := &Oper{}
	g.StateDriver = gc.StateDriver
	err = g.Read(gc.Tenant)
	if err != nil {
		return 0, 0, err
	}

	vxlan = pair.(resources.VXLANVLANPair).VXLAN + g.FreeVXLANsStart
	localVLAN = pair.(resources.VXLANVLANPair).VLAN

	return
}

// FreeVXLAN returns a VXLAN id to the pool.
func (gc *Cfg) FreeVXLAN(ra core.ResourceManager, vxlan uint, localVLAN uint) error {
	g := &Oper{}
	g.StateDriver = gc.StateDriver
	err := g.Read(gc.Tenant)
	if err != nil {
		return nil
	}

	return ra.DeallocateResourceVal(gc.Tenant, resources.AutoVXLANResource,
		resources.VXLANVLANPair{
			VXLAN: vxlan - g.FreeVXLANsStart,
			VLAN:  localVLAN})
}

func clearReservedVLANs(vlanBitset *bitset.BitSet) {
	vlanBitset.Clear(0)
	vlanBitset.Clear(4095)
}

func (gc *Cfg) initVLANBitset(vlans string) (*bitset.BitSet, error) {

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
	clearReservedVLANs(vlanBitset)

	return vlanBitset, nil
}

// AllocVLAN allocates a new VLAN resource. Returns an ID.
func (gc *Cfg) AllocVLAN(ra core.ResourceManager) (uint, error) {
	vlan, err := ra.AllocateResourceVal(gc.Tenant, resources.AutoVLANResource)
	if err != nil {
		log.Errorf("alloc vlan failed: %q", err)
		return 0, err
	}

	return vlan.(uint), err
}

// FreeVLAN releases a VLAN for a given ID.
func (gc *Cfg) FreeVLAN(ra core.ResourceManager, vlan uint) error {
	return ra.DeallocateResourceVal(gc.Tenant, resources.AutoVLANResource, vlan)
}

// AllocSubnet allocates a new subnet. Returns a CIDR.
func (gc *Cfg) AllocSubnet(ra core.ResourceManager) (string, error) {
	pair, err := ra.AllocateResourceVal(gc.Tenant, resources.AutoSubnetResource)
	if err != nil {
		return "", err
	}

	return pair.(resources.SubnetIPLenPair).IP.String(), err
}

// FreeSubnet releases a subnet derived from it's CIDR.
func (gc *Cfg) FreeSubnet(ra core.ResourceManager, subnetIP string) error {
	return ra.DeallocateResourceVal(gc.Tenant, resources.AutoSubnetResource,
		resources.SubnetIPLenPair{
			IP:  net.ParseIP(subnetIP),
			Len: gc.Auto.AllocSubnetLen})
}

// Process validates, implements, and writes the state.
func (gc *Cfg) Process(ra core.ResourceManager) error {
	var err error

	if gc.Version != VersionBeta1 {
		return core.Errorf("unsupported version %s", gc.Version)
	}

	err = gc.checkErrors()
	if err != nil {
		return core.Errorf("process failed on error checks %s", err)
	}

	tenant := gc.Tenant
	if tenant == "" {
		return core.Errorf("null tenant")
	}

	subnetRsrcCfg := &resources.AutoSubnetCfgResource{
		SubnetPool:     net.ParseIP(gc.Auto.SubnetPool),
		SubnetPoolLen:  gc.Auto.SubnetLen,
		AllocSubnetLen: gc.Auto.AllocSubnetLen}
	err = ra.DefineResource(tenant, resources.AutoSubnetResource, subnetRsrcCfg)
	if err != nil {
		return err
	}

	// Only define a vlan resource if a valid range was specified
	if gc.Auto.VLANs != "" {
		var vlanRsrcCfg *bitset.BitSet
		vlanRsrcCfg, err = gc.initVLANBitset(gc.Auto.VLANs)
		if err != nil {
			return err
		}
		err = ra.DefineResource(tenant, resources.AutoVLANResource, vlanRsrcCfg)
		if err != nil {
			return err
		}
	}

	// Only define a vxlan resource if a valid range was specified
	var freeVXLANsStart uint
	if gc.Auto.VXLANs != "" {
		var vxlanRsrcCfg *resources.AutoVXLANCfgResource
		vxlanRsrcCfg, freeVXLANsStart, err = gc.initVXLANBitset(gc.Auto.VXLANs)
		if err != nil {
			return err
		}
		err = ra.DefineResource(tenant, resources.AutoVXLANResource, vxlanRsrcCfg)
		if err != nil {
			return err
		}
	}

	g := &Oper{Tenant: gc.Tenant,
		FreeVXLANsStart: freeVXLANsStart}
	g.StateDriver = gc.StateDriver
	err = g.Write()
	if err != nil {
		log.Errorf("error '%s' updating goper state %v \n", err, g)
		return err
	}

	log.Debugf("updating the global config to new state %v \n", gc)
	return nil
}
