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
	"strings"
	"testing"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/resources"
)

var gstateTestRA = &resources.EtcdResourceManager{Etcd: gstateSD}

type ValueData struct {
	value []byte
}

type FakeStateDriver struct {
	TestState map[string]ValueData
}

func (d *FakeStateDriver) Init(config *core.Config) error {
	d.TestState = make(map[string]ValueData)

	return nil
}

func (d *FakeStateDriver) Deinit() {
	d.TestState = nil
}

func (d *FakeStateDriver) Write(key string, value []byte) error {
	val := ValueData{value: value}
	d.TestState[key] = val

	return nil
}

func (d *FakeStateDriver) Read(key string) ([]byte, error) {
	if val, ok := d.TestState[key]; ok {
		return val.value, nil
	}

	return []byte{}, &core.Error{Desc: "Key not found!"}
}

func (d *FakeStateDriver) ReadAll(baseKey string) ([][]byte, error) {
	values := [][]byte{}

	for key, val := range d.TestState {
		if strings.Contains(key, baseKey) {
			values = append(values, val.value)
		}
	}
	return values, nil
}

func (d *FakeStateDriver) ClearState(key string) error {
	if _, ok := d.TestState[key]; ok {
		delete(d.TestState, key)
	}
	return nil
}

func (d *FakeStateDriver) ReadState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) error {
	encodedState, err := d.Read(key)
	if err != nil {
		return err
	}

	err = unmarshal(encodedState, value)
	if err != nil {
		return err
	}

	return nil
}

func (d *FakeStateDriver) WriteState(key string, value core.State,
	marshal func(interface{}) ([]byte, error)) error {
	encodedState, err := marshal(value)
	if err != nil {
		return err
	}

	err = d.Write(key, encodedState)
	if err != nil {
		return err
	}

	return nil
}

func (d *FakeStateDriver) DumpState() {
	for key, v := range d.TestState {
		log.Printf("key: %q value: %q\n", key, string(v.value))
	}
}

var gstateSD = &FakeStateDriver{}

func TestGlobalConfigAutoVlans(t *testing.T) {
	cfgData := []byte(`
        {
            "Version" : "0.01",
            "Tenant"  : "default",
            "Auto" : {
                "SubnetPool"        : "11.5.0.0",
                "SubnetLen"         : 16,
                "AllocSubnetLen"    : 24,
                "Vlans"             : "1-10",
                "Vxlans"            : "15000-17000"
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
	gstateTestRA.Init()
	defer func() { gstateTestRA.Deinit() }()

	err = gc.Process(gstateTestRA)
	if err != nil {
		t.Fatalf("error '%s' processing config %v \n", err, gc)
	}

	vlan, err = gc.AllocVlan(gstateTestRA)
	if err != nil {
		t.Fatalf("error - allocating vlan - %s \n", err)
	}
	if vlan == 0 {
		t.Fatalf("error - invalid vlan id allocated %d \n", vlan)
	}
	if vlan != 1 {
		t.Fatalf("error - expecting vlan %d but allocated %d \n", 1, vlan)
	}

	err = gc.FreeVlan(gstateTestRA, vlan)
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
                "Vlans"             : "1-10",
                "Vxlans"            : "15000-17000"
            },
            "Deploy" : {
                "DefaultNetType"    : "vxlan"
            }
        }`)
	var vxlan, localVlan uint

	gc, err := Parse(cfgData)
	if err != nil {
		t.Fatalf("error '%s' parsing config '%s' \n", err, cfgData)
	}

	gstateSD.Init(nil)
	defer func() { gstateSD.Deinit() }()
	gc.StateDriver = gstateSD
	gstateTestRA.Init()
	defer func() { gstateTestRA.Deinit() }()

	err = gc.Process(gstateTestRA)
	if err != nil {
		t.Fatalf("error '%s' processing config %v \n", err, gc)
	}

	vxlan, localVlan, err = gc.AllocVxlan(gstateTestRA)
	if err != nil {
		t.Fatalf("error - allocating vxlan - %s \n", err)
	}
	if vxlan == 0 {
		t.Fatalf("error - invalid vxlan allocated %d \n", vxlan)
	}
	if localVlan == 0 {
		t.Fatalf("error - invalid vlan allocated %d \n", localVlan)
	}

	err = gc.FreeVxlan(gstateTestRA, vxlan, localVlan)
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
                "Vxlans"            : "10000-12000"
            },
            "Deploy" : {
                "DefaultNetType"    : "vxlan"
            }
        }`)
	var vlan, localVlan, vxlan uint

	gc, err := Parse(cfgData)
	if err != nil {
		t.Fatalf("error '%s' parsing config '%s' \n", err, cfgData)
	}

	gstateSD.Init(nil)
	defer func() { gstateSD.Deinit() }()
	gc.StateDriver = gstateSD
	gstateTestRA.Init()
	defer func() { gstateTestRA.Deinit() }()

	err = gc.Process(gstateTestRA)
	if err != nil {
		t.Fatalf("error '%s' processing config %v \n", err, gc)
	}

	vlan, err = gc.AllocVlan(gstateTestRA)
	if err != nil {
		t.Fatalf("error - allocating vlan - %s \n", err)
	}
	if vlan != 100 {
		t.Fatalf("error - expecting vlan %d but allocated %d \n", 100, vlan)
	}

	vxlan, localVlan, err = gc.AllocVxlan(gstateTestRA)
	if err != nil {
		t.Fatalf("error - allocating vxlan - %s \n", err)
	}
	if vxlan != 10000 {
		t.Fatalf("error - expecting vlan %d but allocated %d \n", 10000, vxlan)
	}
	if localVlan == 0 {
		t.Fatalf("error - invalid vlan allocated %d \n", localVlan)
	}

	err = gc.FreeVlan(gstateTestRA, vlan)
	if err != nil {
		t.Fatalf("error freeing allocated vlan %d - err '%s' \n", vlan, err)
	}

	err = gc.FreeVxlan(gstateTestRA, vxlan, localVlan)
	if err != nil {
		t.Fatalf("error freeing allocated vxlan %d localvlan %d - err '%s' \n",
			vxlan, localVlan, err)
	}
}

func TestInvalidGlobalConfigNoLocalVlans(t *testing.T) {
	cfgData := []byte(`
        {
            "Version" : "0.01",
            "Tenant"  : "default",
            "Auto" : {
                "SubnetPool"        : "11.5.0.0",
                "SubnetLen"         : 16,
                "AllocSubnetLen"    : 24,
                "Vlans"             : "1-4095",
                "Vxlans"            : "10000-10001"
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
	gstateTestRA.Init()
	defer func() { gstateTestRA.Deinit() }()

	err = gc.Process(gstateTestRA)
	if err == nil {
		t.Fatalf("Was able to process the config, expected to fail!")
	}
}

func TestInvalidGlobalConfigMoreThan4KVlans(t *testing.T) {
	cfgData := []byte(`
        {
            "Version" : "0.01",
            "Tenant"  : "default",
            "Auto" : {
                "SubnetPool"        : "11.5.0.0",
                "SubnetLen"         : 16,
                "AllocSubnetLen"    : 24,
                "Vlans"             : "1-5000",
                "Vxlans"            : "10000-10001"
            },
            "Deploy" : {
                "DefaultNetType"    : "vlan"
            }
        }`)

	_, err := Parse(cfgData)
	if err == nil {
		t.Fatalf("Error: was able to parse invalid vlan pool '%s' \n", err, cfgData)
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
