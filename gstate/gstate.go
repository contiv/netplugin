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
    BASE_GLOBAL         = "/contiv/global/"
    CFG_GLOBAL          = BASE_GLOBAL + "config/"
    OPER_GLOBAL         = BASE_GLOBAL + "oper/"
)

// specifies various parameters to choose the auto allocation values to pick from
// this allows mostly hands-free allocation of networks, endpoints, attach/detach
// operations without having to specify these each time an entity gets created
type AutoParams struct {
    SubnetPool      string
    SubnetLen       uint
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

func (gstate *CfgGstate)UnMarshal (data string) (error) {
    err := json.Unmarshal([]byte(data), &gstate)
    if err != nil {
        return err
    }

    return nil
}

func (gstate *CfgGstate)Marshal() (string, error) {
    b, err := json.Marshal(gstate)
    return string(b[:]), err
}

func (gstate *CfgGstate) Dump() error {
    log.Printf("Global State %v \n", gstate)
    return nil
}

func (gstate *CfgGstate) checkErrors() error {
    var err error

    if net.ParseIP(gstate.Auto.SubnetPool) == nil {
        return errors.New(fmt.Sprintf("invalid ip address pool %s", 
            gstate.Auto.SubnetPool))
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

func (gstate *CfgGstate) Update(d core.StateDriver) error {
    value, err := json.Marshal(gstate)
    if err != nil {
        return err
    }

    return d.Write(CFG_GLOBAL, value)
}

func (gstate *CfgGstate) Read(d core.StateDriver) error {
    value, err := d.Read(CFG_GLOBAL)
    if err != nil {
        return err
    }
    err = json.Unmarshal(value, &gstate)
    if err != nil {
        return err
    }

    return nil
}
