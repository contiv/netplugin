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
	"reflect"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/core"
)

var resourceRegistry = map[string]reflect.Type{
	AutoVLANResource:  reflect.TypeOf(AutoVLANCfgResource{}),
	AutoVXLANResource: reflect.TypeOf(AutoVXLANCfgResource{}),
}

// StateResourceManager implements the core.ResourceManager interface.
// It manages the resources in a logically centralized manner using serialized
// writes to underlying state store.
type StateResourceManager struct {
	stateDriver core.StateDriver
}

var gStateResourceManager *StateResourceManager

// NewStateResourceManager instantiates a state based resource manager
func NewStateResourceManager(sd core.StateDriver) (*StateResourceManager, error) {
	if gStateResourceManager != nil {
		return nil, core.Errorf("state-based resource manager instance already exists.")
	}

	gStateResourceManager = &StateResourceManager{stateDriver: sd}
	err := gStateResourceManager.Init()
	if err != nil {
		return nil, err
	}

	return gStateResourceManager, nil
}

// GetStateResourceManager returns the singleton instance of the state based
// resource manager
func GetStateResourceManager() (*StateResourceManager, error) {
	if gStateResourceManager == nil {
		return nil, core.Errorf("state-based resource manager has not been not created.")
	}

	return gStateResourceManager, nil
}

// ReleaseStateResourceManager releases the singleton instance of the state
// based resource manager
func ReleaseStateResourceManager() {
	if gStateResourceManager != nil {
		gStateResourceManager.Deinit()
	}
	gStateResourceManager = nil
}

// Init initializes the resource manager
func (rm *StateResourceManager) Init() error { return nil }

// Deinit cleans up the resource manager
func (rm *StateResourceManager) Deinit() {}

// XXX: It might be better to keep cache of resources and avoid frequent etcd reads
func (rm *StateResourceManager) findResource(id, desc string) (core.Resource, bool, error) {
	alreadyExists := false
	rsrcType, ok := resourceRegistry[desc]
	if !ok {
		return nil, alreadyExists,
			core.Errorf("No resource found for description: %q", desc)
	}

	val := reflect.New(rsrcType)
	// sanity checks
	if !val.Elem().FieldByName("CommonState").IsValid() {
		return nil, false, core.Errorf("The state structure %v is missing core.CommonState", rsrcType)
	}
	//the following works as every core.State is expected to embed core.CommonState struct
	val.Elem().FieldByName("CommonState").FieldByName("StateDriver").Set(reflect.ValueOf(rm.stateDriver))
	val.Elem().FieldByName("CommonState").FieldByName("ID").Set(reflect.ValueOf(id))

	rsrc := val.Interface().(core.Resource)
	rsrcs, err := rsrc.ReadAll()
	if core.ErrIfKeyExists(err) != nil {
		return nil, alreadyExists, err
	} else if err != nil {
		// set the slice as empty in case of 'key not found' error
		rsrcs = []core.State{}
	}

	for _, r := range rsrcs {
		//the following works as every core.State is expected to embed core.CommonState struct
		cs := reflect.ValueOf(r).Elem().FieldByName("CommonState").Interface().(core.CommonState)
		if cs.ID == id {
			alreadyExists = true
			return r.(core.Resource), alreadyExists, nil
		}
	}
	return rsrc, alreadyExists, nil
}

// DefineResource initializes a new resource.
func (rm *StateResourceManager) DefineResource(id, desc string,
	rsrcCfg interface{}) error {
	// XXX: need to take care of distibuted updates, locks etc here
	rsrc, alreadyExists, err := rm.findResource(id, desc)
	if err != nil {
		return err
	}

	if alreadyExists {
		return core.Errorf("Resource with id: %q already exists", id)
	}

	return rsrc.Init(rsrcCfg)
}

// UndefineResource deinitializes a resource.
func (rm *StateResourceManager) UndefineResource(id, desc string) error {
	// XXX: need to take care of distibuted updates, locks etc here
	rsrc, alreadyExists, err := rm.findResource(id, desc)
	if err != nil {
		return err
	}

	if !alreadyExists {
		return core.Errorf("No resource found for description: %q and id: %q",
			desc, id)
	}

	rsrc.Deinit()
	return nil

}

// RedefineResource deinitializes a resource.
func (rm *StateResourceManager) RedefineResource(id, desc string, rsrcCfg interface{}) error {

	rsrc, alreadyExists, err := rm.findResource(id, desc)
	if err != nil {
		return err
	}

	if !alreadyExists {
		return core.Errorf("No resource found for description: %q and id: %q",
			desc, id)
	}

	rsrc.Reinit(rsrcCfg)
	return nil

}

// GetResourceList get the list of allocated as string for inspection
func (rm *StateResourceManager) GetResourceList(id, desc string) (uint, string) {
	// XXX: need to take care of distibuted updates, locks etc here
	rsrc, _, err := rm.findResource(id, desc)
	if err != nil {
		log.Errorf("unable to find resource %s desc %s", id, desc)
		return 0, ""
	}

	return rsrc.GetList()
}

// AllocateResourceVal yields the core.Resource for the id and description.
func (rm *StateResourceManager) AllocateResourceVal(id, desc string, reqValue interface{}) (interface{},
	error) {
	// XXX: need to take care of distibuted updates, locks etc here
	rsrc, alreadyExists, err := rm.findResource(id, desc)
	if err != nil {
		return nil, err
	}

	if !alreadyExists {
		return nil, core.Errorf("No resource found for description: %q and id: %q",
			desc, id)
	}

	return rsrc.Allocate(reqValue)
}

// DeallocateResourceVal removes a value from the resource.
func (rm *StateResourceManager) DeallocateResourceVal(id, desc string,
	value interface{}) error {
	// XXX: need to take care of distibuted updates, locks etc here
	rsrc, alreadyExists, err := rm.findResource(id, desc)
	if err != nil {
		return err
	}

	if !alreadyExists {
		return core.Errorf("No resource found for description: %q and id: %q",
			desc, id)
	}

	return rsrc.Deallocate(value)
}
