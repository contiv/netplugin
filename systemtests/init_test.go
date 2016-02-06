package systemtests

import (
	"flag"
	"os"
	"strconv"
	. "testing"
	"time"

	. "gopkg.in/check.v1"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/contivmodel/client"
	"github.com/contiv/vagrantssh"
)

type systemtestSuite struct {
	vagrant    vagrantssh.Vagrant
	cli        *client.ContivClient
	short      bool
	containers int
	binpath    string
	iterations int
	vlanIf     string
	nodes      []*node
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
	flag.Parse()

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
	var nodes int
	if nodesStr == "" {
		nodes = 2
	} else {
		var err error
		nodes, err = strconv.Atoi(nodesStr)
		if err != nil {
			c.Fatal(err)
		}
	}

	s.nodes = []*node{}

	c.Assert(s.vagrant.Setup(false, "", nodes), IsNil)

	for _, nodeObj := range s.vagrant.GetNodes() {
		s.nodes = append(s.nodes, &node{tbnode: nodeObj, suite: s})
	}

	logrus.Info("Pulling alpine on all nodes")
	s.vagrant.IterateNodes(func(node vagrantssh.TestbedNode) error {
		return node.RunCommand("docker pull alpine")
	})

	s.cli, _ = client.NewContivClient("http://localhost:9999")
}

func (s *systemtestSuite) SetUpTest(c *C) {
	for _, node := range s.nodes {
		node.cleanupContainers()
		node.cleanupDockerNetwork()
		node.stopNetplugin()
		node.cleanupSlave()
	}

	for _, node := range s.nodes {
		node.stopNetmaster()
	}

	s.getNodeByName("netplugin-node1").cleanupMaster()

	for _, node := range s.nodes {
		c.Assert(node.startNetplugin(""), IsNil)
		c.Assert(node.runCommandUntilNoError("pgrep netplugin"), IsNil)
	}

	for _, node := range s.nodes {
		c.Assert(node.startNetmaster(), IsNil)
		c.Assert(node.runCommandUntilNoError("pgrep netmaster"), IsNil)
	}

	for {
		_, err := s.cli.TenantGet("default")
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	time.Sleep(5 * time.Second)
}

func (s *systemtestSuite) TearDownTest(c *C) {
	for _, node := range s.nodes {
		c.Assert(node.checkForNetpluginErrors(), IsNil)
	}
}

func (s *systemtestSuite) TearDownSuite(c *C) {
	for _, node := range s.nodes {
		node.cleanupContainers()
	}
}

func (s *systemtestSuite) Test00SSH(c *C) {
	c.Assert(s.vagrant.IterateNodes(func(node vagrantssh.TestbedNode) error {
		return node.RunCommand("true")
	}), IsNil)
}
