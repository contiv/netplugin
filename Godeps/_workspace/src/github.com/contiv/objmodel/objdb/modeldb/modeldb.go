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

package modeldb

// Wrapper for persistently storing object model

import (
	"sync"

	"github.com/contiv/objmodel/objdb/client"

	log "github.com/Sirupsen/logrus"
)

type ModelObj interface {
	GetType() string
	GetKey() string
	Read() error
	Write() error
}

// Link is a one way relattion between two objects
type Link struct {
	ObjType string `json:"type,omitempty"`
	ObjKey  string `json:"key,omitempty"`

	mutex sync.Mutex
}

// AddLink adds a one way link to target object
func AddLink(link *Link, obj ModelObj) {
	link.mutex.Lock()
	defer link.mutex.Unlock()
	link.ObjType = obj.GetType()
	link.ObjKey = obj.GetKey()
}

// RemoveLink removes a one way link
func RemoveLink(link *Link, obj ModelObj) {
	link.ObjType = ""
	link.ObjKey = ""
}

// Add a link into linkset. initialize the linkset if required
func AddLinkSet(linkSet *(map[string]Link), obj ModelObj) error {
	// Allocate the linkset if its nil
	if *linkSet == nil {
		*linkSet = make(map[string]Link)
	}

	// add the link to map
	(*linkSet)[obj.GetKey()] = Link{
		ObjType: obj.GetType(),
		ObjKey:  obj.GetKey(),
	}

	return nil
}

func RemoveLinkSet(linkSet *(map[string]Link), obj ModelObj) error {
	// check is linkset is nil
	if *linkSet == nil {
		return nil
	}

	// remove the link from map
	delete(*linkSet, obj.GetKey())

	return nil
}

// persistent database
var cdb = client.NewClient()

func WriteObj(objType, objKey string, value interface{}) error {
	key := "/modeldb/" + objType + "/" + objKey
	err := cdb.SetObj(key, value)
	if err != nil {
		log.Errorf("Error storing object %s. Err: %v", key, err)
		return err
	}

	return nil
}

func ReadObj(objType, objKey string, retVal interface{}) error {
	key := "/modeldb/" + objType + "/" + objKey
	err := cdb.GetObj(key, retVal)
	if err != nil {
		log.Errorf("Error reading object: %s. Err: %v", key, err)
	}

	return nil
}

func DeleteObj(objType, objKey string) error {
	key := "/modeldb/" + objType + "/" + objKey
	err := cdb.DelObj(key)
	if err != nil {
		log.Errorf("Error deleting object: %s. Err: %v", key, err)
	}

	return nil
}

func ReadAllObj(objType string) ([]string, error) {
	key := "/modeldb/" + objType + "/"
	return cdb.ListDir(key)
}
