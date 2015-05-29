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
	"strings"
	"testing"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/state"
)

const (
	testResourceDesc = "testResourceDesc"
	testResourceID   = "testResourceID"
)

var (
	gReadCtr int
)

type TestResource struct {
	core.CommonState
	readCtr int
}

func (r *TestResource) Write() error {
	return core.Errorf("Shouldn't be called")
}

func (r *TestResource) Read(id string) error {
	return core.Errorf("Shouldn't be called")
}

func (r *TestResource) Clear() error {
	return core.Errorf("Shouldn't be called")
}

func (r *TestResource) ReadAll() ([]core.State, error) {
	if gReadCtr == 0 {
		gReadCtr = 1
		return []core.State{}, nil
	}

	return []core.State{core.State(r)}, nil
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

var fakeDriver = &state.FakeStateDriver{}

func TestEtcdResourceManagerDefineResource(t *testing.T) {
	ra := &EtcdResourceManager{Etcd: fakeDriver}
	resourceRegistry[testResourceDesc] = reflect.TypeOf(TestResource{})
	defer func() { delete(resourceRegistry, testResourceDesc) }()

	gReadCtr = 0
	err := ra.DefineResource(testResourceID, testResourceDesc, &TestResource{})
	if err != nil {
		t.Fatalf("Resource definition failed. Error: %s", err)
	}
}

func TestEtcdResourceManagerDefineInvalidResource(t *testing.T) {
	ra := &EtcdResourceManager{Etcd: fakeDriver}

	gReadCtr = 0
	err := ra.DefineResource(testResourceID, testResourceDesc, &TestResource{})
	if err == nil {
		t.Fatalf("Resource definition succeeded, expected to fail!")
	}
	if !strings.Contains(err.Error(),
		fmt.Sprintf("No resource found for description: %q", testResourceDesc)) {
		t.Fatalf("Unexpected error. Error: %s", err)
	}
}

func TestEtcdResourceManagerUndefineResource(t *testing.T) {
	ra := &EtcdResourceManager{Etcd: fakeDriver}
	resourceRegistry[testResourceDesc] = reflect.TypeOf(TestResource{})
	defer func() { delete(resourceRegistry, testResourceDesc) }()

	gReadCtr = 0
	err := ra.DefineResource(testResourceID, testResourceDesc, &TestResource{})
	if err != nil {
		t.Fatalf("Resource definition failed. Error: %s", err)
	}

	err = ra.UndefineResource(testResourceID, testResourceDesc)
	if err != nil {
		t.Fatalf("Resource un-definition failed. Error: %s", err)
	}
}

func TestEtcdResourceManagerUndefineInvalidResource(t *testing.T) {
	ra := &EtcdResourceManager{Etcd: fakeDriver}

	gReadCtr = 0
	err := ra.UndefineResource(testResourceID, testResourceDesc)
	if err == nil {
		t.Fatalf("Resource un-definition succeeded, expected to fail!")
	}
	if !strings.Contains(err.Error(),
		fmt.Sprintf("No resource found for description: %q", testResourceDesc)) {
		t.Fatalf("Unexpected error. Error: %s", err)
	}
}

func TestEtcdResourceManagerUndefineNonexistentResource(t *testing.T) {
	ra := &EtcdResourceManager{Etcd: fakeDriver}
	resourceRegistry[testResourceDesc] = reflect.TypeOf(TestResource{})
	defer func() { delete(resourceRegistry, testResourceDesc) }()

	gReadCtr = 0
	err := ra.UndefineResource(testResourceID, testResourceDesc)
	if err == nil {
		t.Fatalf("Resource un-definition succeeded, expected to fail!")
	}
	if !strings.Contains(err.Error(),
		fmt.Sprintf("No resource found for description: %q and id: %q",
			testResourceDesc, testResourceID)) {
		t.Fatalf("Unexpected error. Error: %s", err)
	}
}

func TestEtcdResourceManagerAllocateResource(t *testing.T) {
	ra := &EtcdResourceManager{Etcd: fakeDriver}
	resourceRegistry[testResourceDesc] = reflect.TypeOf(TestResource{})
	defer func() { delete(resourceRegistry, testResourceDesc) }()

	gReadCtr = 0
	err := ra.DefineResource(testResourceID, testResourceDesc, &TestResource{})
	if err != nil {
		t.Fatalf("Resource definition failed. Error: %s", err)
	}

	_, err = ra.AllocateResourceVal(testResourceID, testResourceDesc)
	if err != nil {
		t.Fatalf("Resource allocation failed. Error: %s", err)
	}
}

func TestEtcdResourceManagerAllocateInvalidResource(t *testing.T) {
	ra := &EtcdResourceManager{Etcd: fakeDriver}

	gReadCtr = 0
	_, err := ra.AllocateResourceVal(testResourceID, testResourceDesc)
	if err == nil {
		t.Fatalf("Resource allocation succeeded, expected to fail!")
	}
	if !strings.Contains(err.Error(),
		fmt.Sprintf("No resource found for description: %q", testResourceDesc)) {
		t.Fatalf("Unexpected error. Error: %s", err)
	}
}

func TestEtcdResourceManagerAllocateiNonexistentResource(t *testing.T) {
	ra := &EtcdResourceManager{Etcd: fakeDriver}
	resourceRegistry[testResourceDesc] = reflect.TypeOf(TestResource{})
	defer func() { delete(resourceRegistry, testResourceDesc) }()

	gReadCtr = 0
	_, err := ra.AllocateResourceVal(testResourceID, testResourceDesc)
	if err == nil {
		t.Fatalf("Resource allocation succeeded, expected to fail!")
	}
	if !strings.Contains(err.Error(),
		fmt.Sprintf("No resource found for description: %q and id: %q",
			testResourceDesc, testResourceID)) {
		t.Fatalf("Unexpected error. Error: %s", err)
	}
}

func TestEtcdResourceManagerDeallocateResource(t *testing.T) {
	ra := &EtcdResourceManager{Etcd: fakeDriver}
	resourceRegistry[testResourceDesc] = reflect.TypeOf(TestResource{})
	defer func() { delete(resourceRegistry, testResourceDesc) }()

	gReadCtr = 0
	err := ra.DefineResource(testResourceID, testResourceDesc, &TestResource{})
	if err != nil {
		t.Fatalf("Resource definition failed. Error: %s", err)
	}

	_, err = ra.AllocateResourceVal(testResourceID, testResourceDesc)
	if err != nil {
		t.Fatalf("Resource allocation failed. Error: %s", err)
	}

	err = ra.DeallocateResourceVal(testResourceID, testResourceDesc, 0)
	if err != nil {
		t.Fatalf("Resource deallocation failed. Error: %s", err)
	}
}

func TestEtcdResourceManagerDeallocateInvalidResource(t *testing.T) {
	ra := &EtcdResourceManager{Etcd: fakeDriver}

	gReadCtr = 0
	err := ra.DeallocateResourceVal(testResourceID, testResourceDesc, 0)
	if err == nil {
		t.Fatalf("Resource deallocation succeeded, expected to fail!")
	}
	if !strings.Contains(err.Error(),
		fmt.Sprintf("No resource found for description: %q", testResourceDesc)) {
		t.Fatalf("Unexpected error. Error: %s", err)
	}
}

func TestEtcdResourceManagerDeallocateiNonexistentResource(t *testing.T) {
	ra := &EtcdResourceManager{Etcd: fakeDriver}
	resourceRegistry[testResourceDesc] = reflect.TypeOf(TestResource{})
	defer func() { delete(resourceRegistry, testResourceDesc) }()

	gReadCtr = 0
	err := ra.DeallocateResourceVal(testResourceID, testResourceDesc, 0)
	if err == nil {
		t.Fatalf("Resource allocation succeeded, expected to fail!")
	}
	if !strings.Contains(err.Error(),
		fmt.Sprintf("No resource found for description: %q and id: %q",
			testResourceDesc, testResourceID)) {
		t.Fatalf("Unexpected error. Error: %s", err)
	}
}
