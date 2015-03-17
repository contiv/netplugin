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

package resources

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/contiv/netplugin/core"
)

const (
	testResourceDesc = "testResourceDesc"
	testResourceId   = "testResourceId"
)

var (
	gReadCtr int
)

type TestResource struct {
	stateDriver core.StateDriver
	id          string
	readCtr     int
}

func (r *TestResource) Write() error {
	return &core.Error{Desc: "Shouldn't be called"}
}

func (r *TestResource) Read(id string) error {
	return &core.Error{Desc: "Shouldn't be called"}
}

func (r *TestResource) Clear() error {
	return &core.Error{Desc: "Shouldn't be called"}
}

func (r *TestResource) ReadAll() ([]core.State, error) {
	if gReadCtr == 0 {
		gReadCtr = 1
		return []core.State{}, nil
	} else {
		return []core.State{core.State(r)}, nil
	}
}

func (r *TestResource) SetId(id string) {
	r.id = id
}

func (r *TestResource) Id() string {
	return r.id
}

func (r *TestResource) SetStateDriver(stateDriver core.StateDriver) {
	r.stateDriver = stateDriver
}

func (r *TestResource) StateDriver() core.StateDriver {
	return r.stateDriver
}

func (r *TestResource) Init(rsrcCfg interface{}) error {
	return nil
}

func (r *TestResource) Deinit() {
}

func (r *TestResource) Description() string {
	return testResourceDesc
}

func (r *TestResource) Allocate() (interface{}, error) {
	return 0, nil
}

func (r *TestResource) Deallocate(value interface{}) error {
	return nil
}

func TestEtcdResourceAllocatorDefineResource(t *testing.T) {
	ra := &EtcdResourceAllocator{Etcd: nil}
	ResourceRegistry[testResourceDesc] = reflect.TypeOf(TestResource{})
	defer func() { delete(ResourceRegistry, testResourceDesc) }()

	gReadCtr = 0
	err := ra.DefineResource(testResourceId, testResourceDesc, &TestResource{})
	if err != nil {
		t.Fatalf("Resource definition failed. Error: %s", err)
	}
}

func TestEtcdResourceAllocatorDefineInvalidResource(t *testing.T) {
	ra := &EtcdResourceAllocator{Etcd: nil}

	gReadCtr = 0
	err := ra.DefineResource(testResourceId, testResourceDesc, &TestResource{})
	if err == nil {
		t.Fatalf("Resource definition succeeded, expected to fail!")
	}
	if err.Error() != fmt.Sprintf("No resource found for description: %q",
		testResourceDesc) {
		t.Fatalf("Unexpected error. Error: %s", err)
	}
}

func TestEtcdResourceAllocatorUndefineResource(t *testing.T) {
	ra := &EtcdResourceAllocator{Etcd: nil}
	ResourceRegistry[testResourceDesc] = reflect.TypeOf(TestResource{})
	defer func() { delete(ResourceRegistry, testResourceDesc) }()

	gReadCtr = 0
	err := ra.DefineResource(testResourceId, testResourceDesc, &TestResource{})
	if err != nil {
		t.Fatalf("Resource definition failed. Error: %s", err)
	}

	err = ra.UndefineResource(testResourceId, testResourceDesc)
	if err != nil {
		t.Fatalf("Resource un-definition failed. Error: %s", err)
	}
}

func TestEtcdResourceAllocatorUndefineInvalidResource(t *testing.T) {
	ra := &EtcdResourceAllocator{Etcd: nil}

	gReadCtr = 0
	err := ra.UndefineResource(testResourceId, testResourceDesc)
	if err == nil {
		t.Fatalf("Resource un-definition succeeded, expected to fail!")
	}
	if err.Error() != fmt.Sprintf("No resource found for description: %q",
		testResourceDesc) {
		t.Fatalf("Unexpected error. Error: %s", err)
	}
}

func TestEtcdResourceAllocatorUndefineNonexistentResource(t *testing.T) {
	ra := &EtcdResourceAllocator{Etcd: nil}
	ResourceRegistry[testResourceDesc] = reflect.TypeOf(TestResource{})
	defer func() { delete(ResourceRegistry, testResourceDesc) }()

	gReadCtr = 0
	err := ra.UndefineResource(testResourceId, testResourceDesc)
	if err == nil {
		t.Fatalf("Resource un-definition succeeded, expected to fail!")
	}
	if err.Error() != fmt.Sprintf("No resource found for description: %q and id: %q",
		testResourceDesc, testResourceId) {
		t.Fatalf("Unexpected error. Error: %s", err)
	}
}

func TestEtcdResourceAllocatorAllocateResource(t *testing.T) {
	ra := &EtcdResourceAllocator{Etcd: nil}
	ResourceRegistry[testResourceDesc] = reflect.TypeOf(TestResource{})
	defer func() { delete(ResourceRegistry, testResourceDesc) }()

	gReadCtr = 0
	err := ra.DefineResource(testResourceId, testResourceDesc, &TestResource{})
	if err != nil {
		t.Fatalf("Resource definition failed. Error: %s", err)
	}

	_, err = ra.AllocateResourceVal(testResourceId, testResourceDesc)
	if err != nil {
		t.Fatalf("Resource allocation failed. Error: %s", err)
	}
}

func TestEtcdResourceAllocatorAllocateInvalidResource(t *testing.T) {
	ra := &EtcdResourceAllocator{Etcd: nil}

	gReadCtr = 0
	_, err := ra.AllocateResourceVal(testResourceId, testResourceDesc)
	if err == nil {
		t.Fatalf("Resource allocation succeeded, expected to fail!")
	}
	if err.Error() != fmt.Sprintf("No resource found for description: %q",
		testResourceDesc) {
		t.Fatalf("Unexpected error. Error: %s", err)
	}
}

func TestEtcdResourceAllocatorAllocateiNonexistentResource(t *testing.T) {
	ra := &EtcdResourceAllocator{Etcd: nil}
	ResourceRegistry[testResourceDesc] = reflect.TypeOf(TestResource{})
	defer func() { delete(ResourceRegistry, testResourceDesc) }()

	gReadCtr = 0
	_, err := ra.AllocateResourceVal(testResourceId, testResourceDesc)
	if err == nil {
		t.Fatalf("Resource allocation succeeded, expected to fail!")
	}
	if err.Error() != fmt.Sprintf("No resource found for description: %q and id: %q",
		testResourceDesc, testResourceId) {
		t.Fatalf("Unexpected error. Error: %s", err)
	}
}

func TestEtcdResourceAllocatorDeallocateResource(t *testing.T) {
	ra := &EtcdResourceAllocator{Etcd: nil}
	ResourceRegistry[testResourceDesc] = reflect.TypeOf(TestResource{})
	defer func() { delete(ResourceRegistry, testResourceDesc) }()

	gReadCtr = 0
	err := ra.DefineResource(testResourceId, testResourceDesc, &TestResource{})
	if err != nil {
		t.Fatalf("Resource definition failed. Error: %s", err)
	}

	_, err = ra.AllocateResourceVal(testResourceId, testResourceDesc)
	if err != nil {
		t.Fatalf("Resource allocation failed. Error: %s", err)
	}

	err = ra.DeallocateResourceVal(testResourceId, testResourceDesc, 0)
	if err != nil {
		t.Fatalf("Resource deallocation failed. Error: %s", err)
	}
}

func TestEtcdResourceAllocatorDeallocateInvalidResource(t *testing.T) {
	ra := &EtcdResourceAllocator{Etcd: nil}

	gReadCtr = 0
	err := ra.DeallocateResourceVal(testResourceId, testResourceDesc, 0)
	if err == nil {
		t.Fatalf("Resource deallocation succeeded, expected to fail!")
	}
	if err.Error() != fmt.Sprintf("No resource found for description: %q",
		testResourceDesc) {
		t.Fatalf("Unexpected error. Error: %s", err)
	}
}

func TestEtcdResourceAllocatorDeallocateiNonexistentResource(t *testing.T) {
	ra := &EtcdResourceAllocator{Etcd: nil}
	ResourceRegistry[testResourceDesc] = reflect.TypeOf(TestResource{})
	defer func() { delete(ResourceRegistry, testResourceDesc) }()

	gReadCtr = 0
	err := ra.DeallocateResourceVal(testResourceId, testResourceDesc, 0)
	if err == nil {
		t.Fatalf("Resource allocation succeeded, expected to fail!")
	}
	if err.Error() != fmt.Sprintf("No resource found for description: %q and id: %q",
		testResourceDesc, testResourceId) {
		t.Fatalf("Unexpected error. Error: %s", err)
	}
}
