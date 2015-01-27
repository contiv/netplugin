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
    "log"
    "errors"
    "encoding/json"
    "net"
    "fmt"

    "github.com/willf/bitset"

    "github.com/contiv/netplugin/netutils"
    "github.com/contiv/netplugin/core"
)

const (
    BASE_GLOBAL         = "/contiv/"
    CFG_GLOBAL          = BASE_GLOBAL + "config/global"
    OPER_GLOBAL         = BASE_GLOBAL + "oper/global"
)

const (
    VersionBeta1        = "0.01"
)

// specifies various parameters to choose the auto allocation values to pick from
// this allows mostly hands-free allocation of networks, endpoints, attach/detach
// operations without having to specify these each time an entity gets created
type AutoParams struct {
    SubnetPool          string
    SubnetLen           uint
    AllocSubnetLen      uint
    Vlans               string
    Vxlans              string
}

// specifies parameters that decides the deployment choices 
type DeployParams struct {
    DefaultNetType  string
}

// global state of the network plugin
type Cfg struct {
    Version     string
    Auto        AutoParams
    Deploy      DeployParams
}

type Oper struct {
    DefaultNetType      string
    SubnetPool          string
    SubnetLen           uint
    AllocSubnetLen      uint
    AllocedSubnets      bitset.BitSet
    FreeVlans           bitset.BitSet
}

var gCfg *Cfg
var gOper *Oper

func (gc *Cfg)UnMarshal (data string) (error) {
    err := json.Unmarshal([]byte(data), &gc)
    if err != nil {
        return err
    }

    return nil
}

func (gc *Cfg)Marshal() (string, error) {
    b, err := json.Marshal(gc)
    return string(b[:]), err
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

    if gc.Deploy.DefaultNetType == "vxlan" {
        return errors.New("vxlan support is coming soon... \n")
    }
    if gc.Deploy.DefaultNetType != "vlan" {
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

    err := json.Unmarshal(configBytes , &gc)
    if err != nil {
        return nil, err
    }

    err = gc.checkErrors()
    if err != nil {
        return nil, err
    }

    return &gc, err
}

func (gc *Cfg) Update(d core.StateDriver) error {
    value, err := json.Marshal(gc)
    if err != nil {
        return err
    }

    return d.Write(CFG_GLOBAL, value)
}

func (gc *Cfg) Read(d core.StateDriver) error {
    value, err := d.Read(CFG_GLOBAL)
    if err != nil {
        return err
    }
    err = json.Unmarshal(value, &gc)
    if err != nil {
        return err
    }

    return nil
}

func (g *Oper) Update(d core.StateDriver) error {
    value, err := json.Marshal(g)
    if err != nil {
        return err
    }

    return d.Write(OPER_GLOBAL, value)
}

func (g *Oper) Read(d core.StateDriver) error {
    value, err := d.Read(OPER_GLOBAL)
    if err != nil {
        return err
    }
    err = json.Unmarshal(value, &g)
    if err != nil {
        return err
    }

    return nil
}

func (g *Oper)initVlanBitset(vlans string) error {
    vlanRanges, err := netutils.ParseTagRanges(vlans, "vlan")
    if err != nil {
        return err
    }

    g.FreeVlans.Copy(netutils.CreateBitset(12))
    for _, vlanRange := range vlanRanges {
        for vlan := vlanRange.Min; vlan <= vlanRange.Max; vlan++ {
            g.FreeVlans.Set(uint(vlan))
        }
    }

    return nil
}

func (g *Oper)AllocVlan ()(uint, error) {
    vlan, ok := g.FreeVlans.NextSet(0)
    if !ok {
        return 0, errors.New("no vlans available ")
    }

    g.FreeVlans.Set(vlan)

    return vlan, nil
}

// be idempotent, don't complain if vlan is already freed
func (g *Oper)FreeVlan (vlan uint) error {
    if !g.FreeVlans.Test(vlan) {
        g.FreeVlans.Clear(vlan)
    }
    return nil
}

func (g *Oper)AllocSubnet ()(string, error) {

    subnetId, found := netutils.NextUnSet(&g.AllocedSubnets, 0)
    if !found {
        log.Printf("Bitmap: %s \n", g.AllocedSubnets.DumpAsBits())
        return "", errors.New("subnet exhaustion")
    }

    g.AllocedSubnets.Set(subnetId)
    subnetIp, err := netutils.GetSubnetIp(g.SubnetPool, g.SubnetLen,
        g.AllocSubnetLen, subnetId)
    if err != nil {
        return "", err
    }

    return subnetIp, nil
}

func (g *Oper)FreeSubnet(subnetIp string) error {

    return nil
}

// process config state and spew out new oper state
func (gc *Cfg) Process() (*Oper, error) {
    var err error

    if gc.Version != VersionBeta1 {
        return nil, errors.New(fmt.Sprintf("unsupported verison %s", 
            gc.Version))
    }

    err = gc.checkErrors()
    if err != nil {
        return nil, errors.New(fmt.Sprintf(
            "process failed on error checks %s \n", err))
    }
    if gCfg != nil || gOper != nil {
        return nil, errors.New("updates on global config not supported yet")
    }

    if gOper == nil {
        gOper = &Oper{ SubnetLen : gc.Auto.SubnetLen, 
            DefaultNetType : gc.Deploy.DefaultNetType,
            AllocSubnetLen : gc.Auto.AllocSubnetLen,
            SubnetPool : gc.Auto.SubnetPool}
        
        allocSubnetSize := gc.Auto.AllocSubnetLen - gc.Auto.SubnetLen 

        gOper.AllocedSubnets.Copy(netutils.CreateBitset(allocSubnetSize))
        gOper.AllocedSubnets.Set( 1 + (1 << allocSubnetSize))

        err = gOper.initVlanBitset(gc.Auto.Vlans)
        if err != nil {
            log.Printf("Error '%s' initializing vlans \n", err)
            return nil, err
        }

        log.Printf("created oper state %v \n", gOper)
    }

    log.Printf("updating the global config to new state %v \n", gc)
    gCfg = gc

    return gOper, nil
}
