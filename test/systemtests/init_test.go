package systemtests

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	. "testing"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/contivmodel/client"
	"github.com/contiv/remotessh"
	. "gopkg.in/check.v1"
)

type systemtestSuite struct {
	vagrant      remotessh.Vagrant
	baremetal    remotessh.Baremetal
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
	keyFile      string
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
	flag.IntVar(&sts.iterations, "iterations", 3, "Number of iterations")

	if os.Getenv("ACI_SYS_TEST_MODE") == "ON" {
		flag.StringVar(&sts.vlanIf, "vlan-if", os.Getenv("HOST_DATA_INTERFACE"), "Data interface in Baremetal setup node")
		flag.StringVar(&sts.binpath, "binpath", "/home/admin/bin", "netplugin/netmaster binary path")
		if os.Getenv("KEY_FILE") == "" {
			flag.StringVar(&sts.keyFile, "keyFile", "/home/admin/.ssh/id_rsa", "Insecure private key in ACI-systemtests")
		} else {
			keyFileValue := os.Getenv("KEY_FILE")
			flag.StringVar(&sts.keyFile, "keyFile", keyFileValue, "Insecure private key in ACI-systemtests")
		}
	} else {
		flag.StringVar(&sts.binpath, "binpath", "/opt/gopath/bin", "netplugin/netmaster binary path")
		flag.StringVar(&sts.vlanIf, "vlan-if", "eth2", "VLAN interface for OVS bridge")
	}

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

	if os.Getenv("ACI_SYS_TEST_MODE") == "ON" {

		logrus.Infof("ACI_SYS_TEST_MODE is ON")
		logrus.Infof("Private keyFile = %s", s.keyFile)
		logrus.Infof("Binary binpath = %s", s.binpath)
		logrus.Infof("Interface vlanIf = %s", s.vlanIf)

		s.baremetal = remotessh.Baremetal{}
		bm := &s.baremetal

		// To fill the hostInfo data structure for Baremetal VMs
		name := "aci-swarm-node"
		hostIPs := strings.Split(os.Getenv("HOST_IPS"), ",")
		hostNames := strings.Split(os.Getenv("HOST_USER_NAMES"), ",")
		hosts := make([]remotessh.HostInfo, 2)

		for i := range hostIPs {
			hosts[i].Name = name + strconv.Itoa(i+1)
			logrus.Infof("Name=%s", hosts[i].Name)

			hosts[i].SSHAddr = hostIPs[i]
			logrus.Infof("SHAddr=%s", hosts[i].SSHAddr)

			hosts[i].SSHPort = "22"

			hosts[i].User = hostNames[i]
			logrus.Infof("User=%s", hosts[i].User)

			hosts[i].PrivKeyFile = s.keyFile
			logrus.Infof("PrivKeyFile=%s", hosts[i].PrivKeyFile)
		}

		c.Assert(bm.Setup(hosts), IsNil)

		s.nodes = []*node{}

		for _, nodeObj := range s.baremetal.GetNodes() {
			s.nodes = append(s.nodes, &node{tbnode: nodeObj, suite: s})
		}

		logrus.Info("Pulling alpine on all nodes")

		s.baremetal.IterateNodes(func(node remotessh.TestbedNode) error {
			node.RunCommand("sudo rm /tmp/*net*")
			return node.RunCommand("docker pull alpine")
		})

		//Copying binaries
		s.copyBinary("netmaster")
		s.copyBinary("netplugin")
		s.copyBinary("netctl")
		s.copyBinary("contivk8s")

	} else {
		s.vagrant = remotessh.Vagrant{}
		nodesStr := os.Getenv("CONTIV_NODES")
		var contivNodes int

		if nodesStr == "" {
			contivNodes = 3
		} else {
			var err error
			contivNodes, err = strconv.Atoi(nodesStr)
			if err != nil {
				c.Fatal(err)
			}
		}

		s.nodes = []*node{}

		if s.fwdMode == "routing" {
			contivL3Nodes := 2
			if s.scheduler == "k8" {
				topDir := os.Getenv("GOPATH")
				//topDir contains the godeps path. hence purging the gopath
				topDir = strings.Split(topDir, ":")[1]

				contivNodes = 4 // 3 contiv nodes + 1 k8master
				c.Assert(s.vagrant.Setup(false, []string{"CONTIV_L3=1 VAGRANT_CWD=" + topDir + "/src/github.com/contiv/netplugin/vagrant/k8s/"}, contivNodes), IsNil)
			} else {
				c.Assert(s.vagrant.Setup(false, []string{"CONTIV_NODES=3 CONTIV_L3=1"}, contivNodes+contivL3Nodes), IsNil)
			}
		} else {
			if s.scheduler == "k8" {
				contivNodes = contivNodes + 1 //k8master

				topDir := os.Getenv("GOPATH")
				//topDir may contain the godeps path. hence purging the gopath
				dirs := strings.Split(topDir, ":")
				if len(dirs) > 1 {
					topDir = dirs[1]
				} else {
					topDir = dirs[0]
				}

				c.Assert(s.vagrant.Setup(false, []string{"VAGRANT_CWD=" + topDir + "/src/github.com/contiv/netplugin/vagrant/k8s/"}, contivNodes), IsNil)
			} else {
				c.Assert(s.vagrant.Setup(false, []string{}, contivNodes), IsNil)
			}
		}

		for _, nodeObj := range s.vagrant.GetNodes() {
			nodeName := nodeObj.GetName()
			if strings.Contains(nodeName, "netplugin-node") ||
				strings.Contains(nodeName, "k8") {
				node := &node{}
				node.tbnode = nodeObj
				node.suite = s

				switch s.scheduler {
				case "k8":
					node.exec = s.NewK8sExec(node)
				default:
					node.exec = s.NewDockerExec(node)
				}
				s.nodes = append(s.nodes, node)
			}
		}

		logrus.Info("Pulling alpine on all nodes")
		s.vagrant.IterateNodes(func(node remotessh.TestbedNode) error {
			node.RunCommand("sudo rm /tmp/net*")
			return node.RunCommand("docker pull alpine")
		})
	}
	s.cli, _ = client.NewContivClient("http://localhost:9999")
}

func (s *systemtestSuite) SetUpTest(c *C) {
	logrus.Infof("============================= %s starting ==========================", c.TestName())

	if os.Getenv("ACI_SYS_TEST_MODE") == "ON" {
		//s.AciTestSetup(c)
	} else {
		for _, node := range s.nodes {
			node.exec.cleanupContainers()
			//node.cleanupDockerNetwork()
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
			c.Assert(node.startNetplugin(""), IsNil)
			c.Assert(node.exec.runCommandUntilNoNetpluginError(), IsNil)
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
			c.Assert(node.exec.runCommandUntilNoNetmasterError(), IsNil)
		}

		time.Sleep(5 * time.Second)
		if s.scheduler != "k8" {
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

		if s.fwdMode == "routing" {
			c.Assert(s.cli.GlobalPost(&client.Global{FwdMode: "routing",
				Name:             "global",
				NetworkInfraType: "default",
				Vlans:            "1-4094",
				Vxlans:           "1-10000",
			}), IsNil)
			time.Sleep(40 * time.Second)
		}
	}
}

func (s *systemtestSuite) TearDownTest(c *C) {
	for _, node := range s.nodes {
		c.Check(node.checkForNetpluginErrors(), IsNil)
		c.Assert(node.exec.rotateNetpluginLog(), IsNil)
		c.Assert(node.exec.rotateNetmasterLog(), IsNil)
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
		out, _ := node.runCommand(`for i in /tmp/_net*; do grep "error\|fatal\|panic" $i; done`)
		if out != "" {
			logrus.Errorf("Errors in logfiles on %s: \n", node.Name())
			fmt.Printf("%s\n==========================\n\n", out)
		}
	}
}

func (s *systemtestSuite) Test00SSH(c *C) {
	c.Assert(s.vagrant.IterateNodes(func(node remotessh.TestbedNode) error {
		return node.RunCommand("true")
	}), IsNil)
}
