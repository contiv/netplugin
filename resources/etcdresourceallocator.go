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
	"log"
	"reflect"

	"github.com/contiv/netplugin/core"
)

// Etcd resource manager implements the core.ResourceManager interface.
// It manages the resource in a logically centralized manner using serialized
// writes to a etcd based datastore.

var ResourceRegistry = map[string]reflect.Type{
	AUTO_VLAN_RSRC:   reflect.TypeOf(AutoVlanCfgResource{}),
	AUTO_VXLAN_RSRC:  reflect.TypeOf(AutoVxlanCfgResource{}),
	AUTO_SUBNET_RSRC: reflect.TypeOf(AutoSubnetCfgResource{}),
}

type EtcdResourceManager struct {
	//XXX: should be '*drivers.EtcdStateDriver', but leaving is
	//core.StateDriver to get tests going and until the netmaster
	//is changed to pickup the resource-manager from config
	Etcd core.StateDriver
}

func (ra *EtcdResourceManager) Init() error {
	return nil
}

func (ra *EtcdResourceManager) Deinit() {
}

// XXX: It might be better to keep cache of resources and avoid frequent etcd reads
func (ra *EtcdResourceManager) findResource(id, desc string) (core.Resource, bool, error) {
	alreadyExists := false
	rsrcType, ok := ResourceRegistry[desc]
	if !ok {
		return nil, alreadyExists,
			&core.Error{Desc: fmt.Sprintf("No resource found for description: %q",
				desc)}
	}

	rsrc := reflect.New(rsrcType).Interface().(core.Resource)
	rsrc.SetId(id)
	rsrc.SetStateDriver(core.StateDriver(ra.Etcd))

	rsrcs, err := rsrc.ReadAll()
	if core.ErrIfKeyExists(err) != nil {
		log.Printf("ReadAll failed: %q", err)
		return nil, alreadyExists, err
	} else if err != nil {
		// set the slice as empty in case of 'key not found' error
		rsrcs = []core.State{}
	}

	for _, s := range rsrcs {
		r := s.(core.Resource)
		if r.Id() == id {
			alreadyExists = true
			return r, alreadyExists, nil
		}
	}
	return rsrc, alreadyExists, nil
}

func (ra *EtcdResourceManager) DefineResource(id, desc string,
	rsrcCfg interface{}) error {
	// XXX: need to take care of distibuted updates, locks etc here
	rsrc, alreadyExists, err := ra.findResource(id, desc)
	if err != nil {
		return err
	}

	if alreadyExists {
		return &core.Error{Desc: fmt.Sprintf("Resource with id: %q already exists",
			id)}
	}

	err = rsrc.Init(rsrcCfg)
	if err != nil {
		return err
	}

	return nil
}

func (ra *EtcdResourceManager) UndefineResource(id, desc string) error {
	// XXX: need to take care of distibuted updates, locks etc here
	rsrc, alreadyExists, err := ra.findResource(id, desc)
	if err != nil {
		return err
	}

	if !alreadyExists {
		return &core.Error{Desc: fmt.Sprintf("No resource found for description: %q and id: %q",
			desc, id)}
	}

	rsrc.Deinit()
	return nil

}

func (ra *EtcdResourceManager) AllocateResourceVal(id, desc string) (interface{},
	error) {
	// XXX: need to take care of distibuted updates, locks etc here
	rsrc, alreadyExists, err := ra.findResource(id, desc)
	if err != nil {
		return nil, err
	}

	if !alreadyExists {
		return nil, &core.Error{Desc: fmt.Sprintf("No resource found for description: %q and id: %q",
			desc, id)}
	}

	return rsrc.Allocate()
}

func (ra *EtcdResourceManager) DeallocateResourceVal(id, desc string,
	value interface{}) error {
	// XXX: need to take care of distibuted updates, locks etc here
	rsrc, alreadyExists, err := ra.findResource(id, desc)
	if err != nil {
		return err
	}

	if !alreadyExists {
		return &core.Error{Desc: fmt.Sprintf("No resource found for description: %q and id: %q",
			desc, id)}
	}

	return rsrc.Deallocate(value)
}
