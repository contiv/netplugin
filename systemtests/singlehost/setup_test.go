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

package singlehost

import (
	"log"
	"os"
	"testing"

	"github.com/contiv/netplugin/systemtests/utils"
)

var vagrant *utils.Vagrant

func TestMain(m *testing.M) {
	// setup a single node vagrant testbed
	vagrant = &utils.Vagrant{}
	log.Printf("Starting vagrant up...")
	err := vagrant.Setup(os.Getenv("CONTIV_ENV"), 1)
	log.Printf("Done with vagrant up...")
	if err != nil {
		log.Printf("Vagrant setup failed. Error: %s", err)
		vagrant.Teardown()
		os.Exit(1)
	}

	exitCode := m.Run()

	if utils.OkToCleanup(exitCode != 0) {
		vagrant.Teardown()
	}

	os.Exit(exitCode)
}
