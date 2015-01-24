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
    "testing"
)

// specifies parameters that decides the deployment choices

func TestGlobalConfig(t *testing.T) {
    cfgData := []byte(`
        {
            "Version" : "0.1",
            "Auto" : {
                "IpSubnetPool" : "11.5.0.0",
                "IpSubnetLen"  : 24,
                "Vlans"        : "100-400,500-900",
                "Vxlans"       : "10000-20000"
            },
            "Deploy" : {
                "DefaultNetType" : "vlan"
            }
        }`)

    _, err := Parse(cfgData)
    if err != nil {
        t.Fatalf("error '%s' parsing config '%s' \n", err, cfgData)
    }
}


func TestInvalidGlobalConfig(t *testing.T) {
    cfgData := []byte(`
        {
            "Version" : "0.1",
            "Auto" : {
                "IpSubnetPool" : "11..5.0.0",
                "IpSubnetLen"  : 24,
                "Vlans"        : "100-400,500-900",
                "Vxlans"       : "10000-20000"
            },
            "Deploy" : {
                "DefaultNetType" : "vlan"
            }
        }`)

    _, err := Parse(cfgData)
    if err == nil {
        t.Fatalf("parsed invalid data '%s' \n", cfgData)
    }

    cfgData = []byte(`
        {
            "Version" : "0.1",
            "Auto" : {
                "IpSubnetPool" : "11..5.0.0",
                "IpSubnetLen"  : 36,
                "Vlans"        : "100-400,500-900",
                "Vxlans"       : "10000-20000"
            },
            "Deploy" : {
                "DefaultNetType" : "vlan"
            }
        }`)

    _, err = Parse(cfgData)
    if err == nil {
        t.Fatalf("parsed invalid data '%s' \n", cfgData)
    }
}
