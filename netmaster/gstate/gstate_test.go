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
	"math/rand"
	"testing"

	"github.com/contiv/netplugin/netmaster/resources"
	"github.com/contiv/netplugin/state"
)

var (
	gstateSD = &state.FakeStateDriver{}
)

func TestGlobalConfigAutoVLANs(t *testing.T) {
	cfgData := []byte(`
        {
            "Auto" : {
                "VLANs"             : "1-10",
                "VXLANs"            : "15000-17000"
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
	_, err = resources.NewStateResourceManager(gstateSD)
	if err != nil {
		t.Fatalf("Failed to instantiate resource manager. Error: %s", err)
	}
	defer func() { resources.ReleaseStateResourceManager() }()

	err = gc.Process("vxlan")
	if err != nil {
		t.Fatalf("error '%s' processing config %v \n", err, gc)
	}
	err = gc.Process("vlan")
	if err != nil {
		t.Fatalf("error '%s' processing config %v \n", err, gc)
	}

	vlan, err = gc.AllocVLAN(uint(0))
	if err != nil {
		t.Fatalf("error - allocating vlan - %s \n", err)
	}
	if vlan == 0 {
		t.Fatalf("error - invalid vlan id allocated %d \n", vlan)
	}
	if vlan != 1 {
		t.Fatalf("error - expecting vlan %d but allocated %d \n", 1, vlan)
	}

	err = gc.FreeVLAN(vlan)
	if err != nil {
		t.Fatalf("error freeing allocated vlan %d - err '%s' \n", vlan, err)
	}
}

func TestGlobalConfigAutoVXLAN(t *testing.T) {
	cfgData := []byte(`
        {
            "Auto" : {
                "VLANs"             : "1-10",
                "VXLANs"            : "15000-17000"
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
	_, err = resources.NewStateResourceManager(gstateSD)
	if err != nil {
		t.Fatalf("Failed to instantiate resource manager. Error: %s", err)
	}
	defer func() { resources.ReleaseStateResourceManager() }()

	err = gc.Process("vlan")
	if err != nil {
		t.Fatalf("error '%s' processing config %v \n", err, gc)
	}
	err = gc.Process("vxlan")
	if err != nil {
		t.Fatalf("error '%s' processing config %v \n", err, gc)
	}
	vxlan, localVLAN, err = gc.AllocVXLAN(uint(0))
	if err != nil {
		t.Fatalf("error - allocating vxlan - %s \n", err)
	}
	if vxlan == 0 {
		t.Fatalf("error - invalid vxlan allocated %d \n", vxlan)
	}
	if localVLAN == 0 {
		t.Fatalf("error - invalid vlan allocated %d \n", localVLAN)
	}

	err = gc.FreeVXLAN(vxlan, localVLAN)
	if err != nil {
		t.Fatalf("error freeing allocated vxlan %d localvlan %d - err '%s' \n",
			vxlan, localVLAN, err)
	}
}

func TestGlobalConfigDefaultVXLANWithVLANs(t *testing.T) {
	cfgData := []byte(`
        {
            "Auto" : {
                "VLANs"             : "100-400,500-900",
                "VXLANs"            : "10000-12000"
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
	_, err = resources.NewStateResourceManager(gstateSD)
	if err != nil {
		t.Fatalf("Failed to instantiate resource manager. Error: %s", err)
	}
	defer func() { resources.ReleaseStateResourceManager() }()

	err = gc.Process("vlan")
	if err != nil {
		t.Fatalf("error '%s' processing config %v \n", err, gc)
	}
	err = gc.Process("vxlan")
	if err != nil {
		t.Fatalf("error '%s' processing config %v \n", err, gc)
	}
	vlan, err = gc.AllocVLAN(uint(0))
	if err != nil {
		t.Fatalf("error - allocating vlan - %s \n", err)
	}
	if vlan != 100 {
		t.Fatalf("error - expecting vlan %d but allocated %d \n", 100, vlan)
	}

	vxlan, localVLAN, err = gc.AllocVXLAN(uint(0))
	if err != nil {
		t.Fatalf("error - allocating vxlan - %s \n", err)
	}
	if vxlan != 10000 {
		t.Fatalf("error - expecting vlan %d but allocated %d \n", 10000, vxlan)
	}
	if localVLAN == 0 {
		t.Fatalf("error - invalid vlan allocated %d \n", localVLAN)
	}

	err = gc.FreeVLAN(vlan)
	if err != nil {
		t.Fatalf("error freeing allocated vlan %d - err '%s' \n", vlan, err)
	}

	err = gc.FreeVXLAN(vxlan, localVLAN)
	if err != nil {
		t.Fatalf("error freeing allocated vxlan %d localvlan %d - err '%s' \n",
			vxlan, localVLAN, err)
	}
}

func TestInvalidGlobalConfigMoreThan4KVLANs(t *testing.T) {
	cfgData := []byte(`
        {
            "Auto" : {
                "VLANs"             : "1-5000",
                "VXLANs"            : "10000-10001"
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
            "Auto" : {
                "VLANs"             : "100-400,900-500",
                "VXLANs"            : "10000-20000"
            }
        }`)

	_, err := Parse(cfgData)
	if err == nil {
		t.Fatalf("Error: was able to parse invalid vlan range '%s'", cfgData)
	}
}

func TestDefaultNetwork(t *testing.T) {
	cfgData := []byte(`
        {
            "Auto" : {
                "VLANs"             : "100-400"
            }
        }`)

	gc, err := Parse(cfgData)
	if err != nil {
		t.Fatalf("Error: was able to parse config '%s'", cfgData)
	}

	gstateSD.Init(nil)
	defer func() { gstateSD.Deinit() }()
	gc.StateDriver = gstateSD

	_, err = gc.AssignDefaultNetwork("orange")
	if err != nil {
		t.Fatalf("Error: '%s' unable to assign default network '%s'", err, cfgData)
	}

	if err := gc.UnassignNetwork("orange"); err != nil {
		t.Fatalf("Error: '%s' could unassign a network that was not default network", err)
	}

	if err := gc.UnassignNetwork("purple"); err != nil {
		t.Fatalf("Error: '%s' could not unassign default network", err)
	}
}

func TestAutoDefaultNetwork(t *testing.T) {
	cfgData := []byte(`
        {
            "Auto" : {
                "VLANs"             : "100-400"
            }
        }`)

	gc, err := Parse(cfgData)
	if err != nil {
		t.Fatalf("Error: was able to parse config '%s'", cfgData)
	}

	gstateSD.Init(nil)
	defer func() { gstateSD.Deinit() }()
	gc.StateDriver = gstateSD

	defName, err := gc.AssignDefaultNetwork("orange")
	if err != nil {
		t.Fatalf("Error: '%s' unable to assign default network '%s'", err, cfgData)
	}

	if defName != "orange" {
		t.Fatalf("Error: assigned invalid default network '%s' cfg '%s'", defName, cfgData)
	}

	if err := gc.UnassignNetwork("orange"); err != nil {
		t.Fatalf("Error: '%s' could not unassign default network", err)
	}
}

func TestGlobalEPGResource(t *testing.T) {
	var gc Cfg

	gstateSD.Init(nil)
	defer func() { gstateSD.Deinit() }()
	gc.StateDriver = gstateSD

	_, err := resources.NewStateResourceManager(gstateSD)
	if err != nil {
		t.Fatalf("Failed to instantiate resource manager. Error: %s", err)
	}
	defer func() { resources.ReleaseStateResourceManager() }()

	err = gc.Process("epg")
	if err != nil {
		t.Fatalf("error '%s' processing config %v for EPG resource\n", err, gc)
	}

	// Check if allocation of EPGs start from 1
	epgID, err := gc.AllocNextEPG()
	if epgID != 1 {
		t.Fatalf("EPG allocation doesn't start from 1")
	}

	// Allocate a few more EPGs
	for i := 1; i < 10; i++ {
		epgID, err = gc.AllocNextEPG()
		if err != nil {
			t.Fatalf("Error allocation next EPG\n", err)
		}
	}

	// Check allocation of the same EPG fails
	reqEpgID := uint(10)
	_, err = gc.AllocEPG(reqEpgID)
	if err == nil {
		t.Fatalf("Same EPG allocated again")
	}

	// Check if the freed EPG IDs are getting re-used
	freeEpgID := uint(rand.Intn(int(epgID)))
	err = gc.FreeEPG(freeEpgID)
	if err != nil {
		t.Fatalf("Error freeing EPG: %d", epgID)
	}

	epgID, err = gc.AllocNextEPG()
	if epgID != freeEpgID {
		t.Fatalf("Freed EPG not getting re-used. Freed EPG: %d, Allocated EPG: %d", freeEpgID, epgID)
	}

}
