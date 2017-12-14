package systemtests

import (
	"flag"
	"os"
	. "testing"

	"github.com/Sirupsen/logrus"
	. "github.com/contiv/check"
	"github.com/contiv/netplugin/contivmodel/client"
	"github.com/contiv/remotessh"
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
	Scheduler          string `json:"scheduler"`      //swarm, k8s or plain docker
	InstallMode        string `json:"install_mode"`   //legacy or kubeadm
	SwarmEnv           string `json:"swarm_variable"` //env variables to be set with swarm
	Platform           string `json:"platform"`       //vagrant or baremetal
	Product            string `json:"product"`        //for netplugin / volplugin
	AciMode            string `json:"aci_mode"`       //on/off
	Short              bool   `json:"short"`
	Containers         int    `json:"containers"`
	Iterations         int    `json:"iterations"`
	EnableDNS          bool   `json:"enableDNS"`
	ClusterStoreDriver string `json:"contiv_cluster_store_driver"`
	ClusterStoreURLs   string `json:"contiv_cluster_store_urls"`
	ContivL3           string `json:"contiv_l3"`
	KeyFile            string `json:"keyFile"`
	BinPath            string `json:"binpath"` // /home/admin/bin or /opt/gopath/bin
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

const (
	kubeScheduler  string = "k8s"
	swarmScheduler string = "swarm"
	legacyInstall  string = "legacy"
	kubeadmInstall string = "kubeadm"
)

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
