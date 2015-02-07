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

func TestGlobalConfigAutoVlans(t *testing.T) {
	cfgData := []byte(`
        {
            "Version" : "0.01",
            "Tenant"  : "default",
            "Auto" : {
                "SubnetPool"        : "11.5.0.0",
                "SubnetLen"         : 16,
                "AllocSubnetLen"    : 24,
                "Vlans"             : "",
                "Vxlans"            : ""
            },
            "Deploy" : {
                "DefaultNetType"    : "vlan"
            }
        }`)
	var vlan uint
	var g *Oper
	defer func() { clearState("default") }()

	gc, err := Parse(cfgData)
	if err != nil {
		t.Fatalf("error '%s' parsing config '%s' \n", err, cfgData)
	}

	g, err = gc.Process()
	if err != nil {
		t.Fatalf("error '%s' processing config %v \n", err, gc)
	}

	vlan, err = g.AllocVlan()
	if err != nil {
		t.Fatalf("error - allocating vlan - %s \n", err)
	}
	if vlan == 0 {
		t.Fatalf("error - invalid vlan id allocated %d \n", vlan)
	}

	err = g.FreeVlan(vlan)
	if err != nil {
		t.Fatalf("error freeing allocated vlan %d - err '%s' \n", vlan, err)
	}
}

func TestGlobalConfigSpecificVlans(t *testing.T) {
	cfgData := []byte(`
        {
            "Version" : "0.01",
            "Tenant"  : "default",
            "Auto" : {
                "SubnetPool"        : "11.5.0.0",
                "SubnetLen"         : 16,
                "AllocSubnetLen"    : 24,
                "Vlans"             : "200-300,1000-1500",
                "Vxlans"            : ""
            },
            "Deploy" : {
                "DefaultNetType"    : "vlan"
            }
        }`)
	var vlan uint
	var g *Oper
	defer func() { clearState("default") }()

	gc, err := Parse(cfgData)
	if err != nil {
		t.Fatalf("error '%s' parsing config '%s' \n", err, cfgData)
	}

	g, err = gc.Process()
	if err != nil {
		t.Fatalf("error '%s' processing config %v \n", err, gc)
	}

	vlan, err = g.AllocVlan()
	if err != nil {
		t.Fatalf("error - allocating vlan - %s \n", err)
	}
	if vlan != 200 {
		t.Fatalf("error - expecting vlan %d but allocated %d \n", 200, vlan)
	}

	err = g.FreeVlan(vlan)
	if err != nil {
		t.Fatalf("error freeing allocated vlan %d - err '%s' \n", vlan, err)
	}
}

func TestGlobalConfigAutoVxlan(t *testing.T) {
	cfgData := []byte(`
        {
            "Version" : "0.01",
            "Tenant"  : "default",
            "Auto" : {
                "SubnetPool"        : "11.5.0.0",
                "SubnetLen"         : 16,
                "AllocSubnetLen"    : 24,
                "Vlans"             : "",
                "Vxlans"            : ""
            },
            "Deploy" : {
                "DefaultNetType"    : "vxlan"
            }
        }`)
	var vxlan, localVlan uint
	var g *Oper
	defer func() { clearState("default") }()

	gc, err := Parse(cfgData)
	if err != nil {
		t.Fatalf("error '%s' parsing config '%s' \n", err, cfgData)
	}

	g, err = gc.Process()
	if err != nil {
		t.Fatalf("error '%s' processing config %v \n", err, gc)
	}

	vxlan, localVlan, err = g.AllocVxlan()
	if err != nil {
		t.Fatalf("error - allocating vxlan - %s \n", err)
	}
	if vxlan == 0 {
		t.Fatalf("error - invalid vxlan allocated %d \n", vxlan)
	}
	if localVlan == 0 {
		t.Fatalf("error - invalid vlan allocated d \n", localVlan)
	}

	_, err = g.AllocVlan()
	if err == nil {
		t.Fatalf("error - was expecting vlan allocation to fail \n")
	}

	err = g.FreeVxlan(vxlan, localVlan)
	if err != nil {
		t.Fatalf("error freeing allocated vxlan %d localvlan %d - err '%s' \n",
			vxlan, localVlan, err)
	}
}

func TestGlobalConfigSpecificVxlans(t *testing.T) {
	cfgData := []byte(`
        {
            "Version" : "0.01",
            "Tenant"  : "default",
            "Auto" : {
                "SubnetPool"        : "11.5.0.0",
                "SubnetLen"         : 16,
                "AllocSubnetLen"    : 24,
                "Vlans"             : "",
                "Vxlans"            : "11111-15000"
            },
            "Deploy" : {
                "DefaultNetType"    : "vxlan"
            }
        }`)
	var vxlan, localVlan uint
	var g *Oper
	defer func() { clearState("default") }()

	gc, err := Parse(cfgData)
	if err != nil {
		t.Fatalf("error '%s' parsing config '%s' \n", err, cfgData)
	}

	g, err = gc.Process()
	if err != nil {
		t.Fatalf("error '%s' processing config %v \n", err, gc)
	}

	vxlan, localVlan, err = g.AllocVxlan()
	if err != nil {
		t.Fatalf("error - allocating vxlan - %s \n", err)
	}
	if vxlan != 11111 {
		t.Fatalf("error - invalid vxlan allocated %d expecting %d\n", vxlan, 11111)
	}
	if localVlan == 0 {
		t.Fatalf("error - invalid vlan allocated d \n", localVlan)
	}

	_, err = g.AllocVlan()
	if err == nil {
		t.Fatalf("error - was expecting vlan allocation to fail \n")
	}

	err = g.FreeVxlan(vxlan, localVlan)
	if err != nil {
		t.Fatalf("error freeing allocated vxlan %d localvlan %d - err '%s' \n",
			vxlan, localVlan, err)
	}
}

func TestGlobalConfigDefaultVxlanWithVlans(t *testing.T) {
	cfgData := []byte(`
        {
            "Version" : "0.01",
            "Tenant"  : "default",
            "Auto" : {
                "SubnetPool"        : "11.5.0.0",
                "SubnetLen"         : 16,
                "AllocSubnetLen"    : 24,
                "Vlans"             : "100-400,500-900",
                "Vxlans"            : "10000-20000"
            },
            "Deploy" : {
                "DefaultNetType"    : "vxlan"
            }
        }`)
	var vlan, localVlan, vxlan uint
	var g *Oper
	defer func() { clearState("default") }()

	gc, err := Parse(cfgData)
	if err != nil {
		t.Fatalf("error '%s' parsing config '%s' \n", err, cfgData)
	}

	g, err = gc.Process()
	if err != nil {
		t.Fatalf("error '%s' processing config %v \n", err, gc)
	}

	vlan, err = g.AllocVlan()
	if err != nil {
		t.Fatalf("error - allocating vlan - %s \n", err)
	}
	if vlan != 100 {
		t.Fatalf("error - expecting vlan %d but allocated %d \n", 100, vlan)
	}

	vxlan, localVlan, err = g.AllocVxlan()
	if err != nil {
		t.Fatalf("error - allocating vxlan - %s \n", err)
	}
	if vxlan != 10000 {
		t.Fatalf("error - expecting vlan %d but allocated %d \n", 10000, vxlan)
	}
	if localVlan == 0 {
		t.Fatalf("error - invalid vlan allocated %d \n", localVlan)
	}

	err = g.FreeVlan(vlan)
	if err != nil {
		t.Fatalf("error freeing allocated vlan %d - err '%s' \n", vlan, err)
	}

	err = g.FreeVxlan(vxlan, localVlan)
	if err != nil {
		t.Fatalf("error freeing allocated vxlan %d localvlan %d - err '%s' \n",
			vxlan, localVlan, err)
	}
}

func TestInvalidGlobalConfig(t *testing.T) {
	cfgData := []byte(`
        {
            "Version" : "0.01",
            "Tenant"  : "default",
            "Auto" : {
                "SubnetPool"        : "11..5.0.0",
                "SubnetLen"         : 16,
                "AllocSubnetLen"    : 24,
                "Vlans"             : "100-400,500-900",
                "Vxlans"            : "10000-20000"
            },
            "Deploy" : {
                "DefaultNetType"    : "vlan"
            }
        }`)

	_, err := Parse(cfgData)
	if err == nil {
		t.Fatalf("Error: was able to parse invalid subnet pool '%s' \n", err, cfgData)
	}

	cfgData = []byte(`
        {
            "Version" : "0.01",
            "Tenant"  : "default",
            "Auto" : {
                "SubnetPool"        : "11.5.0.0",
                "SubnetLen"         : 16,
                "AllocSubnetLen"    : 24,
                "Vlans"             : "100-400,900-500",
                "Vxlans"            : "10000-20000"
            },
            "Deploy" : {
                "DefaultNetType"    : "vlan"
            }
        }`)

	_, err = Parse(cfgData)
	if err == nil {
		t.Fatalf("Error: was able to parse invalid vlan range '%s' \n", err, cfgData)
	}

	cfgData = []byte(`
        {
            "Version" : "0.01",
            "Tenant"  : "default",
            "Auto" : {
                "SubnetPool"        : "11.5.0.0",
                "SubnetLen"         : 22,
                "AllocSubnetLen"    : 20,
                "Vlans"             : "100-400,500-900",
                "Vxlans"            : "10000-20000"
            },
            "Deploy" : {
                "DefaultNetType"    : "vlan"
            }
        }`)

	_, err = Parse(cfgData)
	if err == nil {
		t.Fatalf("Error: was able to parse invalid subnetlen/allcocsubnetlen %s'\n",
			err, cfgData)
	}

}
