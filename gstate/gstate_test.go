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

	"github.com/contiv/netplugin/resources"
	"github.com/contiv/netplugin/state"
)

var (
	gstateSD = &state.FakeStateDriver{}
)

func TestGlobalConfigAutoVLANs(t *testing.T) {
	cfgData := []byte(`
        {
            "Version" : "0.01",
            "Tenant"  : "default",
            "Auto" : {
                "SubnetPool"        : "11.5.0.0",
                "SubnetLen"         : 16,
                "AllocSubnetLen"    : 24,
                "VLANs"             : "1-10",
                "VXLANs"            : "15000-17000"
            },
            "Deploy" : {
                "DefaultNetType"    : "vlan"
            }
        }`)
	var vlan uint

	gc, err := Parse(cfgData)
	if err != nil {
		t.Fatalf("error '%s' parsing config '%s' \n", err, cfgData)
	}

	gstateSD.Init(nil)
	defer func() { gstateSD.Deinit() }()
	gc.StateDriver = gstateSD
	rm, err := resources.NewStateResourceManager(gstateSD)
	if err != nil {
		t.Fatalf("Failed to instantiate resource manager. Error: %s", err)
	}
	defer func() { resources.ReleaseStateResourceManager() }()

	err = gc.Process(rm)
	if err != nil {
		t.Fatalf("error '%s' processing config %v \n", err, gc)
	}

	vlan, err = gc.AllocVLAN(rm)
	if err != nil {
		t.Fatalf("error - allocating vlan - %s \n", err)
	}
	if vlan == 0 {
		t.Fatalf("error - invalid vlan id allocated %d \n", vlan)
	}
	if vlan != 1 {
		t.Fatalf("error - expecting vlan %d but allocated %d \n", 1, vlan)
	}

	err = gc.FreeVLAN(rm, vlan)
	if err != nil {
		t.Fatalf("error freeing allocated vlan %d - err '%s' \n", vlan, err)
	}
}

func TestGlobalConfigAutoVXLAN(t *testing.T) {
	cfgData := []byte(`
        {
            "Version" : "0.01",
            "Tenant"  : "default",
            "Auto" : {
                "SubnetPool"        : "11.5.0.0",
                "SubnetLen"         : 16,
                "AllocSubnetLen"    : 24,
                "VLANs"             : "1-10",
                "VXLANs"            : "15000-17000"
            },
            "Deploy" : {
                "DefaultNetType"    : "vxlan"
            }
        }`)
	var vxlan, localVLAN uint

	gc, err := Parse(cfgData)
	if err != nil {
		t.Fatalf("error '%s' parsing config '%s' \n", err, cfgData)
	}

	gstateSD.Init(nil)
	defer func() { gstateSD.Deinit() }()
	gc.StateDriver = gstateSD
	rm, err := resources.NewStateResourceManager(gstateSD)
	if err != nil {
		t.Fatalf("Failed to instantiate resource manager. Error: %s", err)
	}
	defer func() { resources.ReleaseStateResourceManager() }()

	err = gc.Process(rm)
	if err != nil {
		t.Fatalf("error '%s' processing config %v \n", err, gc)
	}

	vxlan, localVLAN, err = gc.AllocVXLAN(rm)
	if err != nil {
		t.Fatalf("error - allocating vxlan - %s \n", err)
	}
	if vxlan == 0 {
		t.Fatalf("error - invalid vxlan allocated %d \n", vxlan)
	}
	if localVLAN == 0 {
		t.Fatalf("error - invalid vlan allocated %d \n", localVLAN)
	}

	err = gc.FreeVXLAN(rm, vxlan, localVLAN)
	if err != nil {
		t.Fatalf("error freeing allocated vxlan %d localvlan %d - err '%s' \n",
			vxlan, localVLAN, err)
	}
}

func TestGlobalConfigDefaultVXLANWithVLANs(t *testing.T) {
	cfgData := []byte(`
        {
            "Version" : "0.01",
            "Tenant"  : "default",
            "Auto" : {
                "SubnetPool"        : "11.5.0.0",
                "SubnetLen"         : 16,
                "AllocSubnetLen"    : 24,
                "VLANs"             : "100-400,500-900",
                "VXLANs"            : "10000-12000"
            },
            "Deploy" : {
                "DefaultNetType"    : "vxlan"
            }
        }`)
	var vlan, localVLAN, vxlan uint

	gc, err := Parse(cfgData)
	if err != nil {
		t.Fatalf("error '%s' parsing config '%s' \n", err, cfgData)
	}

	gstateSD.Init(nil)
	defer func() { gstateSD.Deinit() }()
	gc.StateDriver = gstateSD
	rm, err := resources.NewStateResourceManager(gstateSD)
	if err != nil {
		t.Fatalf("Failed to instantiate resource manager. Error: %s", err)
	}
	defer func() { resources.ReleaseStateResourceManager() }()

	err = gc.Process(rm)
	if err != nil {
		t.Fatalf("error '%s' processing config %v \n", err, gc)
	}

	vlan, err = gc.AllocVLAN(rm)
	if err != nil {
		t.Fatalf("error - allocating vlan - %s \n", err)
	}
	if vlan != 100 {
		t.Fatalf("error - expecting vlan %d but allocated %d \n", 100, vlan)
	}

	vxlan, localVLAN, err = gc.AllocVXLAN(rm)
	if err != nil {
		t.Fatalf("error - allocating vxlan - %s \n", err)
	}
	if vxlan != 10000 {
		t.Fatalf("error - expecting vlan %d but allocated %d \n", 10000, vxlan)
	}
	if localVLAN == 0 {
		t.Fatalf("error - invalid vlan allocated %d \n", localVLAN)
	}

	err = gc.FreeVLAN(rm, vlan)
	if err != nil {
		t.Fatalf("error freeing allocated vlan %d - err '%s' \n", vlan, err)
	}

	err = gc.FreeVXLAN(rm, vxlan, localVLAN)
	if err != nil {
		t.Fatalf("error freeing allocated vxlan %d localvlan %d - err '%s' \n",
			vxlan, localVLAN, err)
	}
}

func TestInvalidGlobalConfigNoLocalVLANs(t *testing.T) {
	cfgData := []byte(`
        {
            "Version" : "0.01",
            "Tenant"  : "default",
            "Auto" : {
                "SubnetPool"        : "11.5.0.0",
                "SubnetLen"         : 16,
                "AllocSubnetLen"    : 24,
                "VLANs"             : "1-4095",
                "VXLANs"            : "10000-10001"
            },
            "Deploy" : {
                "DefaultNetType"    : "vlan"
            }
        }`)

	gc, err := Parse(cfgData)
	if err != nil {
		t.Fatalf("error '%s' parsing config '%s' \n", err, cfgData)
	}

	gstateSD.Init(nil)
	defer func() { gstateSD.Deinit() }()
	gc.StateDriver = gstateSD
	rm, err := resources.NewStateResourceManager(gstateSD)
	if err != nil {
		t.Fatalf("Failed to instantiate resource manager. Error: %s", err)
	}
	defer func() { resources.ReleaseStateResourceManager() }()

	err = gc.Process(rm)
	if err == nil {
		t.Fatalf("Was able to process the config, expected to fail!")
	}
}

func TestInvalidGlobalConfigMoreThan4KVLANs(t *testing.T) {
	cfgData := []byte(`
        {
            "Version" : "0.01",
            "Tenant"  : "default",
            "Auto" : {
                "SubnetPool"        : "11.5.0.0",
                "SubnetLen"         : 16,
                "AllocSubnetLen"    : 24,
                "VLANs"             : "1-5000",
                "VXLANs"            : "10000-10001"
            },
            "Deploy" : {
                "DefaultNetType"    : "vlan"
            }
        }`)

	_, err := Parse(cfgData)
	if err == nil {
		t.Fatalf("Error: was able to parse invalid vlan pool '%s'", cfgData)
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
                "VLANs"             : "100-400,500-900",
                "VXLANs"            : "10000-20000"
            },
            "Deploy" : {
                "DefaultNetType"    : "vlan"
            }
        }`)

	_, err := Parse(cfgData)
	if err == nil {
		t.Fatalf("Error: was able to parse invalid subnet pool '%s'", cfgData)
	}

	cfgData = []byte(`
        {
            "Version" : "0.01",
            "Tenant"  : "default",
            "Auto" : {
                "SubnetPool"        : "11.5.0.0",
                "SubnetLen"         : 16,
                "AllocSubnetLen"    : 24,
                "VLANs"             : "100-400,900-500",
                "VXLANs"            : "10000-20000"
            },
            "Deploy" : {
                "DefaultNetType"    : "vlan"
            }
        }`)

	_, err = Parse(cfgData)
	if err == nil {
		t.Fatalf("Error: was able to parse invalid vlan range '%s'", cfgData)
	}

	cfgData = []byte(`
        {
            "Version" : "0.01",
            "Tenant"  : "default",
            "Auto" : {
                "SubnetPool"        : "11.5.0.0",
                "SubnetLen"         : 22,
                "AllocSubnetLen"    : 20,
                "VLANs"             : "100-400,500-900",
                "VXLANs"            : "10000-20000"
            },
            "Deploy" : {
                "DefaultNetType"    : "vlan"
            }
        }`)

	_, err = Parse(cfgData)
	if err == nil {
		t.Fatalf("Error: was able to parse invalid subnetlen/allcocsubnetlen %s'",
			cfgData)
	}
}
