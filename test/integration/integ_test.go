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

package integration

import (
	"flag"
	"os"
	"os/exec"
	. "testing"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/contivmodel/client"

	. "github.com/contiv/check"
)

type integTestSuite struct {
	// test params
	iterations         int    // number of iterations for multi-iteration tests
	parallels          int    // number of parallel tests to run
	fwdMode            string // forwarding mode bridging/routing
	arpMode            string // ARP mode proxy/flood
	encap              string // encap vlan/vxlan
	fabricMode         string // aci or default
	clusterStoreDriver string // cluster store URL
	clusterStoreURL    string // cluster store URL

	// internal state
	npcluster *NPCluster           // netplugin + netmaster cluster
	client    *client.ContivClient // contiv client
	uniqEPID  uint64               // rolling int to generate unique EP IDs
}

var integ = &integTestSuite{}
var integSuite = Suite(integ)

func TestMain(m *M) {
	flag.IntVar(&integ.iterations, "iterations", 3, "Number of iterations")
	flag.IntVar(&integ.parallels, "parallels", 10, "Number of parallel actions")
	flag.StringVar(&integ.fwdMode, "fwd-mode", "bridge", "forwarding mode [ bridge | routing ]")
	flag.StringVar(&integ.arpMode, "arp-mode", "proxy", "ARP mode [ proxy | flood ]")
	flag.StringVar(&integ.encap, "encap", "vlan", "Encap [ vlan | vxlan ]")
	flag.StringVar(&integ.fabricMode, "fabric-mode", "default", "fabric-mode [ aci | default ]")
	flag.StringVar(&integ.clusterStoreDriver, "cluster-store-driver", "etcd", "Cluster store driver")
	flag.StringVar(&integ.clusterStoreURL, "cluster-store-url", "http://127.0.0.1:2379", "Cluster store URL")

	flag.Parse()

	log.Infof("Running integration test with params: %+v", integ)

	os.Exit(m.Run())
}

func TestInteg(t *T) {
	TestingT(t)
}

func (its *integTestSuite) SetUpSuite(c *C) {
	log.Infof("Bootstrapping integration tests")

	// clear all etcd state before running the tests
	exec.Command("etcdctl", "rm", "--recursive", "/contiv.io").Output()

	npcluster, err := NewNPCluster(its)
	assertNoErr(err, c, "creating cluster")

	// create a new contiv client
	contivClient, err := client.NewContivClient("http://localhost:9999")
	assertNoErr(err, c, "creating contivmodel client")

	// setup test suite
	its.npcluster = npcluster
	its.client = contivClient
}

// SetUpTest gets called before each test is run
func (its *integTestSuite) SetUpTest(c *C) {
	log.Infof("============================= %s starting ==========================", c.TestName())
}

// TearDownTest gets called after each test is run
func (its *integTestSuite) TearDownTest(c *C) {
	log.Infof("============================= %s completed ==========================", c.TestName())
}

// TearDownSuite gets called after entire test suite is done
func (its *integTestSuite) TearDownSuite(c *C) {
	// FIXME: stop netplugin/netmaster
}
