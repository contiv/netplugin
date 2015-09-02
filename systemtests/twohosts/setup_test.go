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

package twohosts

import (
	"os"
	"testing"

	utils "github.com/contiv/systemtests-utils"

	log "github.com/Sirupsen/logrus"
)

var testbed utils.Testbed

func TestMain(m *testing.M) {
	if os.Getenv("CONTIV_TESTBED") == "DIND" {
		testbed = &utils.Dind{}
	} else {
		testbed = &utils.Vagrant{}
	}
	log.Printf("Starting testbed setup...")
	err := testbed.Setup(os.Getenv("CONTIV_ENV"), 2)
	log.Printf("Done with testbed setup...")
	if err != nil {
		testbed.Teardown()
		log.Fatalf("Testbed setup failed. Error: %s", err)
	}

	exitCode := m.Run()

	if utils.OkToCleanup(exitCode != 0) {
		testbed.Teardown()
	}

	os.Exit(exitCode)
}
