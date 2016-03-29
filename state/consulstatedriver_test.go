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

package state

import (
	"testing"

	"github.com/contiv/netplugin/core"
)

func setupConsulDriver(t *testing.T) *ConsulStateDriver {
	instInfo := core.InstanceInfo{DbURL: "consul://127.0.0.1:8500"}

	driver := &ConsulStateDriver{}

	err := driver.Init(&instInfo)
	if err != nil {
		t.Fatalf("driver init failed. Error: %s", err)
		return nil
	}

	return driver
}

func TestConsulStateDriverInit(t *testing.T) {
	setupConsulDriver(t)
}

func TestConsulStateDriverInitInvalidConfig(t *testing.T) {
	driver := &ConsulStateDriver{}
	commonTestStateDriverInitInvalidConfig(t, driver)
}

func TestConsulStateDriverWrite(t *testing.T) {
	driver := setupConsulDriver(t)
	commonTestStateDriverWrite(t, driver)
}

func TestConsulStateDriverRead(t *testing.T) {
	driver := setupConsulDriver(t)
	commonTestStateDriverRead(t, driver)
}

func TestConsulStateDriverWriteState(t *testing.T) {
	driver := setupConsulDriver(t)
	commonTestStateDriverWriteState(t, driver)
}

func TestConsulStateDriverWriteStateForUpdate(t *testing.T) {
	driver := setupConsulDriver(t)
	commonTestStateDriverWriteStateForUpdate(t, driver)
}

func TestConsulStateDriverClearState(t *testing.T) {
	driver := setupConsulDriver(t)
	commonTestStateDriverClearState(t, driver)
}

func TestConsulStateDriverReadState(t *testing.T) {
	driver := setupConsulDriver(t)
	commonTestStateDriverReadState(t, driver)
}

func TestConsulStateDriverReadStateAfterUpdate(t *testing.T) {
	driver := setupConsulDriver(t)
	commonTestStateDriverReadStateAfterUpdate(t, driver)
}

func TestConsulStateDriverReadStateAfterClear(t *testing.T) {
	driver := setupConsulDriver(t)
	commonTestStateDriverReadStateAfterClear(t, driver)
}

func TestConsulStateDriverWatchAllStateCreate(t *testing.T) {
	driver := setupConsulDriver(t)
	commonTestStateDriverWatchAllStateCreate(t, driver)
}

func TestConsulStateDriverWatchAllStateModify(t *testing.T) {
	driver := setupConsulDriver(t)
	commonTestStateDriverWatchAllStateModify(t, driver)
}

func TestConsulStateDriverWatchAllStateDelete(t *testing.T) {
	driver := setupConsulDriver(t)
	commonTestStateDriverWatchAllStateDelete(t, driver)
}
