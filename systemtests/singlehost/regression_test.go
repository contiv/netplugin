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
	"io/ioutil"
	"testing"

	"github.com/contiv/netplugin/systemtests/utils"
	u "github.com/contiv/netplugin/utils"
)

func TestOneHostMultipleNets_regress(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, testbed.GetNodes())
		utils.StopOnError(t.Failed())
	}()

	cfgFile := utils.GetCfgFile("one_host_multiple_nets")
	jsonCfg, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("failed to read config file %s \n", err)
	}
	utils.ConfigSetupCommon(t, string(jsonCfg), testbed.GetNodes())

	node1 := testbed.GetNodes()[0]

	utils.StartServer(t, node1, "myContainer1")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer1")
	}()
	ipAddress := utils.GetIPAddress(t, node1, "orange-myContainer1", u.EtcdNameStr)
	utils.StartClient(t, node1, "myContainer2", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer2")
	}()

	utils.StartServer(t, node1, "myContainer4")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer4")
	}()
	ipAddress = utils.GetIPAddress(t, node1, "purple-myContainer4", u.EtcdNameStr)
	utils.StartClient(t, node1, "myContainer3", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer3")
	}()
}

func TestOneHostVlan_regress(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, testbed.GetNodes())
		utils.StopOnError(t.Failed())
	}()

	cfgFile := utils.GetCfgFile("one_host_vlan")
	jsonCfg, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("failed to read config file %s \n", err)
	}
	utils.ConfigSetupCommon(t, string(jsonCfg), testbed.GetNodes())

	node1 := testbed.GetNodes()[0]

	utils.StartServer(t, node1, "myContainer1")
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer1")
	}()

	ipAddress := utils.GetIPAddress(t, node1, "orange-myContainer1", u.EtcdNameStr)
	utils.StartClient(t, node1, "myContainer2", ipAddress)
	defer func() {
		utils.DockerCleanup(t, node1, "myContainer2")
	}()
}
