package systemtests

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	. "testing"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/contivmodel/client"
	"github.com/contiv/vagrantssh"
	. "gopkg.in/check.v1"
)

type systemtestSuite struct {
	vagrant      vagrantssh.Vagrant
	cli          *client.ContivClient
	short        bool
	containers   int
	binpath      string
	iterations   int
	vlanIf       string
	nodes        []*node
	fwdMode      string
	clusterStore string
	enableDNS    bool
	scheduler    string
	// user       string
	// password   string
	// nodes      []string
}

var sts = &systemtestSuite{}

var _ = Suite(sts)

func TestMain(m *M) {
	// FIXME when we support non-vagrant environments, we will incorporate these changes
	// var nodes string
	//
	// flag.StringVar(&nodes, "nodes", "", "List of nodes to use (comma separated)")
	// flag.StringVar(&sts.user, "user", "vagrant", "User ID for SSH")
	// flag.StringVar(&sts.password, "password", "vagrant", "Password for SSH")
	flag.StringVar(&sts.vlanIf, "vlan-if", "eth2", "VLAN interface for OVS bridge")
	flag.IntVar(&sts.iterations, "iterations", 3, "Number of iterations")
	flag.StringVar(&sts.binpath, "binpath", "/opt/gopath/bin", "netplugin/netmaster binary path")
	flag.IntVar(&sts.containers, "containers", 3, "Number of containers to use")
	flag.BoolVar(&sts.short, "short", false, "Do a quick validation run instead of the full test suite")
	flag.BoolVar(&sts.enableDNS, "dns-enable", false, "Enable DNS service discovery")
	if os.Getenv("CONTIV_CLUSTER_STORE") == "" {
		flag.StringVar(&sts.clusterStore, "cluster-store", "etcd://localhost:2379", "cluster store URL")
	} else {
		flag.StringVar(&sts.clusterStore, "cluster-store", os.Getenv("CONTIV_CLUSTER_STORE"), "cluster store URL")
	}
	if os.Getenv("CONTIV_L3") == "" {
		flag.StringVar(&sts.fwdMode, "fwd-mode", "bridge", "forwarding mode to start the test ")
	} else {
		flag.StringVar(&sts.fwdMode, "fwd-mode", "routing", "forwarding mode to start the test ")
	}
	if os.Getenv("CONTIV_K8") != "" {
		flag.StringVar(&sts.scheduler, "scheduler", "k8", "scheduler used for testing")
	}
	flag.Parse()

	logrus.Infof("Running system test with params: %+v", sts)

	os.Exit(m.Run())
}

func TestSystem(t *T) {
	if os.Getenv("HOST_TEST") != "" {
		os.Exit(0)
	}

	TestingT(t)
}

func (s *systemtestSuite) SetUpSuite(c *C) {
	logrus.Infof("Bootstrapping system tests")

	s.vagrant = vagrantssh.Vagrant{}
	nodesStr := os.Getenv("CONTIV_NODES")
	var contivNodes int

	if nodesStr == "" {
		contivNodes = 2
	} else {
		var err error
		contivNodes, err = strconv.Atoi(nodesStr)
		if err != nil {
			c.Fatal(err)
		}
	}

	s.nodes = []*node{}

	if s.scheduler == "k8" {
		s.KubeNodeSetup(c)
	}

	if s.fwdMode == "routing" {
		contivL3Nodes := 2
		c.Assert(s.vagrant.Setup(false, "CONTIV_NODES=3 CONTIV_L3=2", contivNodes+contivL3Nodes), IsNil)
	} else {
		c.Assert(s.vagrant.Setup(false, "", contivNodes), IsNil)
	}
	for _, nodeObj := range s.vagrant.GetNodes() {
		nodeName := nodeObj.GetName()
		if strings.Contains(nodeName, "netplugin-node") {
			s.nodes = append(s.nodes, &node{tbnode: nodeObj, suite: s})
		}
	}

	logrus.Info("Pulling alpine on all nodes")
	s.vagrant.IterateNodes(func(node vagrantssh.TestbedNode) error {
		node.RunCommand("sudo rm /tmp/net*")
		return node.RunCommand("docker pull alpine")
	})

	s.cli, _ = client.NewContivClient("http://localhost:9999")
}

func (s *systemtestSuite) SetUpTest(c *C) {
	logrus.Infof("============================= %s starting ==========================", c.TestName())

	for _, node := range s.nodes {
		node.cleanupContainers()
		node.cleanupDockerNetwork()
		node.stopNetplugin()
		node.cleanupSlave()
	}

	for _, node := range s.nodes {
		node.stopNetmaster()

	}
	for _, node := range s.nodes {
		node.cleanupMaster()
	}

	for _, node := range s.nodes {
		if s.fwdMode == "bridge" {
			c.Assert(node.startNetplugin(""), IsNil)
			c.Assert(node.runCommandUntilNoError("pgrep netplugin"), IsNil)
		} else if s.fwdMode == "routing" {
			c.Assert(node.startNetplugin("-fwd-mode=routing -vlan-if=eth2"), IsNil)
			c.Assert(node.runCommandUntilNoError("pgrep netplugin"), IsNil)
		}
	}

	time.Sleep(15 * time.Second)

	// temporarily enable DNS for service discovery tests
	prevDNSEnabled := s.enableDNS
	if strings.Contains(c.TestName(), "SvcDiscovery") {
		s.enableDNS = true
	}
	defer func() { s.enableDNS = prevDNSEnabled }()

	for _, node := range s.nodes {
		c.Assert(node.startNetmaster(), IsNil)
		time.Sleep(1 * time.Second)
		c.Assert(node.runCommandUntilNoError("pgrep netmaster"), IsNil)
	}

	time.Sleep(5 * time.Second)
	for i := 0; i < 11; i++ {
		_, err := s.cli.TenantGet("default")
		if err == nil {
			break
		}
		// Fail if we reached last iteration
		c.Assert((i < 10), Equals, true)
		time.Sleep(500 * time.Millisecond)
	}

}

func (s *systemtestSuite) TearDownTest(c *C) {
	for _, node := range s.nodes {
		c.Check(node.checkForNetpluginErrors(), IsNil)
		c.Assert(node.rotateLog("netplugin"), IsNil)
		c.Assert(node.rotateLog("netmaster"), IsNil)
	}
	logrus.Infof("============================= %s completed ==========================", c.TestName())
}

func (s *systemtestSuite) TearDownSuite(c *C) {
	for _, node := range s.nodes {
		node.cleanupContainers()
	}

	// Print all errors and fatal messages
	for _, node := range s.nodes {
		logrus.Infof("Checking for errors on %v", node.Name())
		out, _ := node.runCommand(`for i in /tmp/_net*; do grep "error\|fatal\|panic" $i; done`)
		if out != "" {
			logrus.Errorf("Errors in logfiles on %s: \n", node.Name())
			fmt.Printf("%s\n==========================\n\n", out)
		}
	}

}

func (s *systemtestSuite) Test00SSH(c *C) {
	c.Assert(s.vagrant.IterateNodes(func(node vagrantssh.TestbedNode) error {
		return node.RunCommand("true")
	}), IsNil)
}

func (s *systemtestSuite) KubeNodeSetup(c *C) {
	cmd := exec.Command("/bin/sh", "./vagrant/k8s/setup_cluster.sh")
	c.Assert(cmd.Run(), IsNil)
}
