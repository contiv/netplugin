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
    // "github.com/contiv/netplugin/core"
    // "github.com/contiv/netplugin/drivers"
)

const (
    BASE_GLOBAL         = "/contiv/global/"
    CFG_GLOBAL          = BASE_GLOBAL + "config/"
    OPER_GLOBAL         = BASE_GLOBAL + "oper/"
)

// specifies various parameters to choose the auto allocation values to pick from
// this allows mostly hands-free allocation of networks, endpoints, attach/detach
// operations without having to specify these each time an entity gets created
type AutoParams struct {
    IpSubnetPool    string
    IpSubnetLen     uint
    Vlans           string
    Vxlans          string
}

// specifies parameters that decides the deployment choices 
type DeployParams struct {
    DefaultNetType  string
}

// global state of the network plugin
type CfgGstate struct {
    Version     string
    Auto        AutoParams
    Deploy      DeployParams
}

type OperGstate struct {
    AllocedVlans   bitset.BitSet
    AllocedVxlans   map[int]string
}

func UnMarshal(data string) (*CfgGstate, error) {
    var gstate CfgGstate

    err := json.Unmarshal([]byte(data), &gstate)
    if err != nil {
        return nil, err
    }

    return &gstate, nil
}

func Marshal(gstate *CfgGstate) (string, error) {
    b, err := json.Marshal(gstate)
    return string(b[:]), err
}

func (gstate *CfgGstate) Dump() error {
    log.Printf("Global State %v \n", gstate)
    return nil
}

func (gstate *CfgGstate) checkErrors() error {
    var err error

    if net.ParseIP(gstate.Auto.IpSubnetPool) == nil {
        return errors.New(fmt.Sprintf("invalid ip address pool %s", 
            gstate.Auto.IpSubnetPool))
    }
    
    _, err = netutils.ParseTagRanges(gstate.Auto.Vlans, "vlan")
    if err != nil {
        return err
    }

    _, err = netutils.ParseTagRanges(gstate.Auto.Vxlans, "vxlan")
    if err != nil {
        return err
    }

    if gstate.Deploy.DefaultNetType == "vxlan" {
        return errors.New("vxlan support is coming soon... \n")
    }
    if gstate.Deploy.DefaultNetType != "vlan" {
        return errors.New(fmt.Sprintf("unsupported net type %s", 
            gstate.Deploy.DefaultNetType))
    }

    return err
}

func Parse(configBytes []byte) (*CfgGstate, error) {
    var gstate CfgGstate

    err := json.Unmarshal(configBytes , &gstate)
    if err != nil {
        return nil, err
    }

    err = gstate.checkErrors()
    if err != nil {
        return nil, err
    }

    return &gstate, err
}

/*
func (gstate *CfgGstate) Update(d *drivers.StateDriver) error {
    return d.WriteState(CFG_GLOBAL, gstate, json.Marshal)
}

func (gstate *CfgGstate) Delete(d *StateDriver) error {
    return s.StateDriver.ClearState(CFG_GLOBAL)
}

func (gstate *CfgGstate) Read(d *StateDriver) error {
    return s.StateDriver.ReadState(CFG_GLOBAL, gstate, json.Unmarshal)
}
*/
