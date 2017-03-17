package systemtests

import (
	"flag"
	"fmt"
	"os"
	"strings"
	. "testing"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/contivmodel/client"
	"github.com/contiv/remotessh"
	. "gopkg.in/check.v1"
)

type systemtestSuite struct {
	vagrant   remotessh.Vagrant
	baremetal remotessh.Baremetal
	cli       *client.ContivClient
	nodes     []*node
	fwdMode   string
	basicInfo BasicInfo
	hostInfo  HostInfo
	globInfo  GlobInfo
}
type BasicInfo struct {
	Scheduler    string `json:"scheduler"`      //swarm, k8s or plain docker
	SwarmEnv     string `json:"swarm_variable"` //env variables to be set with swarm
	Platform     string `json:"platform"`       //vagrant or baremetal
	Product      string `json:"product"`        //for netplugin / volplugin
	AciMode      string `json:"aci_mode"`       //on/off
	Short        bool   `json:"short"`
	Containers   int    `json:"containers"`
	Iterations   int    `json:"iterations"`
	EnableDNS    bool   `json:"enableDNS"`
	ClusterStore string `json:"contiv_cluster_store"`
	ContivL3     string `json:"contiv_l3"`
	KeyFile      string `json:"keyFile"`
	BinPath      string `json:"binpath"` // /home/admin/bin or /opt/gopath/bin
}

type HostInfo struct {
	HostIPs            string `json:"hostips"`
	HostUsernames      string `json:"hostusernames"`
	HostDataInterfaces string `json:"dataInterfaces"`
	HostMgmtInterface  string `json:"mgmtInterface"`
}

type GlobInfo struct {
	Vlan    string `json:"vlan"`
	Vxlan   string `json:"vxlan"`
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway"`
	Network string `json:"network"`
	Tenant  string `json:"tenant"`
	Encap   string `json:"encap"`
}

var sts = &systemtestSuite{}

var _ = Suite(sts)

func TestMain(m *M) {
	mastbasic, _, _, err := getInfo("cfg.json")
	if err != nil {
		logrus.Errorf("failed to load cfg.json config: %v", err)
		os.Exit(1)
	}

	if mastbasic.ContivL3 == "" {
		flag.StringVar(&sts.fwdMode, "fwd-mode", "bridge", "forwarding mode to start the test ")
	} else {
		flag.StringVar(&sts.fwdMode, "fwd-mode", "routing", "forwarding mode to start the test ")
	}

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
	var err error
	logrus.Infof("Bootstrapping system tests")
	s.basicInfo, s.hostInfo, s.globInfo, err = getInfo("cfg.json")
	if err != nil {
		logrus.Infof("failed to load cfg.json: %v", err)
		os.Exit(1)
	}

	switch s.basicInfo.Platform {
	case "baremetal":
		s.SetUpSuiteBaremetal(c)

	case "vagrant":
		s.SetUpSuiteVagrant(c)
	default:
		logrus.Errorf("unsupported platform: %v", s.basicInfo.Platform)
		os.Exit(1)
	}

	s.cli, _ = client.NewContivClient("http://localhost:9999")
}

func (s *systemtestSuite) SetUpTest(c *C) {
	logrus.Infof("============================= %s starting ==========================", c.TestName())
	switch s.basicInfo.Platform {
	case "baremetal":
		s.SetUpTestBaremetal(c)

	case "vagrant":
		s.SetUpTestVagrant(c)
	}

}

func (s *systemtestSuite) TearDownTest(c *C) {

	errors := s.parallelExec(func(node *node) error {
		return node.checkForNetpluginErrors()
	})
	for _, err := range errors {
		c.Check(err, IsNil)
	}

	errors = s.parallelExec(func(node *node) error {
		return node.exec.rotateNetpluginLog()
	})
	for _, err := range errors {
		c.Assert(err, IsNil)
	}

	errors = s.parallelExec(func(node *node) error {
		return node.exec.rotateNetmasterLog()
	})
	for _, err := range errors {
		c.Assert(err, IsNil)
	}

	logrus.Infof("============================= %s completed ==========================", c.TestName())
}

func (s *systemtestSuite) TearDownSuite(c *C) {
	for _, node := range s.nodes {
		node.exec.cleanupContainers()
	}

	// Print all errors and fatal messages
	for _, node := range s.nodes {
		logrus.Infof("Checking for errors on %v", node.Name())
		out, _ := node.runCommand(`for i in /tmp/net*; do grep "error\|fatal\|panic" $i; done`)
		if strings.Contains(out, "No such file or directory") {
			continue
		}
		if out != "" {
			logrus.Errorf("Errors in logfiles on %s: \n", node.Name())
			fmt.Printf("%s\n==========================\n\n", out)
		}
	}
}

func (s *systemtestSuite) Test00SSH(c *C) {
	switch s.basicInfo.Platform {
	case "baremetal":
		c.Assert(s.baremetal.IterateNodes(func(node remotessh.TestbedNode) error {
			return node.RunCommand("true")
		}), IsNil)
	case "vagrant":
		c.Assert(s.vagrant.IterateNodes(func(node remotessh.TestbedNode) error {
			return node.RunCommand("true")
		}), IsNil)
	}
}
