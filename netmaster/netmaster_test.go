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

package netmaster

import (
	"encoding/json"
	"errors"
	"log"
	"strings"
	"testing"

	"github.com/contiv/netplugin/core"
)

type ValueData struct {
	value []byte
}

var testState map[string]ValueData

// fake implementation of state driver for the tests
var d = &fakeStateDriver{}

type fakeStateDriver struct {
}

func (d *fakeStateDriver) Init(config *core.Config) error {
	testState = make(map[string]ValueData)

	return nil
}

func (d *fakeStateDriver) Deinit() {
	testState = make(map[string]ValueData)
}

func (d *fakeStateDriver) Write(key string, value []byte) error {
	val := ValueData{value: value}
	testState[key] = val

	return nil
}

func (d *fakeStateDriver) Read(key string) ([]byte, error) {
	if val, ok := testState[key]; ok {
		return val.value, nil
	}

	return []byte{}, errors.New("key not found!")
}

func (d *fakeStateDriver) ClearState(key string) error {
	if _, ok := testState[key]; ok {
		delete(testState, key)
	}
	return nil
}

func (d *fakeStateDriver) ReadState(key string, value core.State,
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

func (d *fakeStateDriver) WriteState(key string, value core.State,
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

func (d *fakeStateDriver) DumpState() {
	for key, _ := range testState {
		log.Printf("key %s \n", key)
	}
}

func applyConfig(t *testing.T, cfgBytes []byte) {
	tenant := &ConfigTenant{}
	err := json.Unmarshal(cfgBytes, tenant)
	if err != nil {
		t.Fatalf("error '%s' parsing config '%s'\n", err, cfgBytes)
	}

	d.Init(nil)
	err = CreateTenant(d, tenant)
	if err != nil {
		t.Fatalf("error '%s' creating tenant\n", err)
	}

	err = CreateNetworks(d, tenant)
	if err != nil {
		t.Fatalf("error '%s' creating networks\n", err)
	}

	// d.DumpState()

	err = CreateEndpoints(d, tenant)
	if err != nil {
		t.Fatalf("error '%s' creating endpoints\n", err)
	}
}

func verifyKeys(t *testing.T, keys []string) {

	for _, key := range keys {
		found := false
		for stateKey, _ := range testState {
			if found = strings.Contains(stateKey, key); found {
				break
			}
		}
		if !found {
			t.Fatalf("key '%s' was not populated in db", key)
		}
	}
}

func TestVlanConfig(t *testing.T) {
	cfgBytes := []byte(`
	{
		"Name"                      : "tenant-one",
		"DefaultNetType"            : "vlan",
		"SubnetPool"                : "11.1.0.0/16",
		"AllocSubnetLen"            : 24,
		"Vlans"                     : "11-28",
		"Networks"  : [
		{
			"Name"                  : "orange",
			"Endpoints" : [
			{
				"Container"         : "myContainer1"
			},
			{
				"Container"         : "myContainer2"
			}
			]
		},
		{
			"Name"                  : "purple",
			"Endpoints" : [
			{
				"Container"         : "myContainer3"
			},
			{
				"Container"         : "myContainer4"
			}
			]
		}
		]
    }`)

	applyConfig(t, cfgBytes)

	keys := []string{"tenant-one", "orange", "myContainer1", "myContainer2",
		"purple", "myContainer3", "myContainer4"}

	verifyKeys(t, keys)
}

func TestVlanWithUnderlayConfig(t *testing.T) {
	cfgBytes := []byte(`
	{
		"Name"					  : "tenant-one",
		"DefaultNetType"			: "vlan",
		"SubnetPool"				: "11.1.0.0/16",
		"AllocSubnetLen"			: 24,
		"Vlans"					 : "11-48",
		"Networks"  : [
		{
			"Name"				  : "infra",
			"PktTag"				: "0",
			"Endpoints" : [
			{
				"Host"			  : "host1",
				"Intf"			  : "eth2"
			},
			{
				"Host"			  : "host2",
				"Intf"			  : "eth2"
			}
			]
		},
		{
			"Name"				  : "orange",
			"Endpoints" : [
			{
				"Container"		 : "myContainer1",
				"Host"			  : "host1"
			},
			{
				"Container"		 : "myContainer3",
				"Host"			  : "host2"
			}
			]
		},
		{
			"Name"				  : "purple",
			"Endpoints" : [
			{
				"Container"		 : "myContainer2",
				"Host"			  : "host1"
			},
			{
				"Container"		 : "myContainer4",
				"Host"			  : "host2"
			}
			]
		}
		]
	}`)

	applyConfig(t, cfgBytes)

	keys := []string{"tenant-one", "orange", "myContainer1", "myContainer2",
		"purple", "myContainer3", "myContainer4"}

	verifyKeys(t, keys)
}

func TestVxlanConfig(t *testing.T) {
	cfgBytes := []byte(`
	{
		"Name"					  : "tenant-one",
		"DefaultNetType"			: "vxlan",
		"SubnetPool"				: "11.1.0.0/16",
		"AllocSubnetLen"			: 24,
		"Vxlans"					: "10001-20000",
		"Networks"  : [
		{
			"Name"				  : "orange",
			"Endpoints" : [
			{
				"Container"		 : "myContainer1",
				"Host"			  : "host1"
			},
			{
				"Container"		 : "myContainer3",
				"Host"			  : "host2"
			}
			]
		},
		{
			"Name"				  : "purple",
			"Endpoints" : [
			{
				"Container"		 : "myContainer2",
				"Host"			  : "host1"
			},
			{
				"Container"		 : "myContainer4",
				"Host"			  : "host2"
			}
			]
		}
		]
	}`)

	applyConfig(t, cfgBytes)

	keys := []string{"tenant-one", "orange", "myContainer1", "myContainer2",
		"purple", "myContainer3", "myContainer4"}

	verifyKeys(t, keys)
}
