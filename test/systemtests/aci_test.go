package systemtests

import (
	//"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/contiv/contivmodel/client"
	//	"github.com/contiv/vagrantssh"
	. "gopkg.in/check.v1"
	//	"os"
	//	"strconv"
	//	"strings"
	"time"
)

func (s *systemtestSuite) TestACIMode(c *C) {
	if s.fwdMode == "routing" || s.scheduler == "k8" {
		return
	}
	c.Assert(s.cli.GlobalPost(&client.Global{
		Name:             "global",
		NetworkInfraType: "aci",
		Vlans:            "1-4094",
		Vxlans:           "1-10000",
		FwdMode:          "bridge",
	}), IsNil)
	c.Assert(s.cli.NetworkPost(&client.Network{
		TenantName:  "default",
		NetworkName: "aciNet",
		Subnet:      "22.2.2.0/24",
		Gateway:     "22.2.2.254",
		Encap:       "vlan",
	}), IsNil)

	err := s.nodes[0].checkSchedulerNetworkCreated("aciNet", false)
	c.Assert(err, IsNil)

	c.Assert(s.cli.EndpointGroupPost(&client.EndpointGroup{
		TenantName:  "default",
		NetworkName: "aciNet",
		GroupName:   "epga",
	}), IsNil)

	err = s.nodes[0].exec.checkSchedulerNetworkCreated("epga", true)
	c.Assert(err, IsNil)

	c.Assert(s.cli.EndpointGroupPost(&client.EndpointGroup{
		TenantName:  "default",
		NetworkName: "aciNet",
		GroupName:   "epgb",
	}), IsNil)

	err = s.nodes[0].checkSchedulerNetworkCreated("epgb", true)
	c.Assert(err, IsNil)

	containersA, err := s.runContainersOnNode(2, "aciNet", "", "epga", s.nodes[0])
	c.Assert(err, IsNil)
	containersB, err := s.runContainersOnNode(2, "aciNet", "", "epgb", s.nodes[0])
	c.Assert(err, IsNil)

	// Verify cA1 can ping cA2
	c.Assert(s.pingTest(containersA), IsNil)
	// Verify cB1 can ping cB2
	c.Assert(s.pingTest(containersB), IsNil)
	// Verify cA1 cannot ping cB1
	c.Assert(s.pingFailureTest(containersA, containersB), IsNil)

	log.Infof("Triggering netplugin restart")
	node1 := s.nodes[0]
	c.Assert(node1.stopNetplugin(), IsNil)
	c.Assert(node1.rotateLog("netplugin"), IsNil)
	c.Assert(node1.startNetplugin(""), IsNil)
	c.Assert(node1.runCommandUntilNoError("pgrep netplugin"), IsNil)
	time.Sleep(20 * time.Second)

	// Verify cA1 can ping cA2
	c.Assert(s.pingTest(containersA), IsNil)
	// Verify cB1 can ping cB2
	c.Assert(s.pingTest(containersB), IsNil)
	// Verify cA1 cannot ping cB1
	c.Assert(s.pingFailureTest(containersA, containersB), IsNil)

	c.Assert(s.removeContainers(containersA), IsNil)
	c.Assert(s.removeContainers(containersB), IsNil)
	c.Assert(s.cli.EndpointGroupDelete("default", "epga"), IsNil)
	c.Assert(s.cli.EndpointGroupDelete("default", "epgb"), IsNil)
	c.Assert(s.cli.NetworkDelete("default", "aciNet"), IsNil)
}

/*
func (s *systemtestSuite) TestACIPingGateway(c *C) {
	if s.fwdMode == "routing" {
		return
	}
	c.Assert(s.cli.GlobalPost(&client.Global{
		Name:             "global",
		NetworkInfraType: "aci",
		Vlans:            "1100-1200",
		Vxlans:           "1-10000",
		FwdMode:          "bridge",
	}), IsNil)
	c.Assert(s.cli.TenantPost(&client.Tenant{
		TenantName: "aciTenant",
	}), IsNil)
	c.Assert(s.cli.NetworkPost(&client.Network{
		TenantName:  "aciTenant",
		NetworkName: "aciNet",
		Subnet:      "20.1.1.0/24",
		Gateway:     "20.1.1.254",
		Encap:       "vlan",
	}), IsNil)

	c.Assert(s.cli.EndpointGroupPost(&client.EndpointGroup{
		TenantName:  "aciTenant",
		NetworkName: "aciNet",
		GroupName:   "epga",
	}), IsNil)

	c.Assert(s.cli.AppProfilePost(&client.AppProfile{
		TenantName:     "aciTenant",
		EndpointGroups: []string{"epga"},
		AppProfileName: "profile1",
	}), IsNil)

	containersA, err := s.runContainersOnNode(1, "aciNet", "aciTenant", "epga", s.nodes[0])
	c.Assert(err, IsNil)

	// Verify cA1 can ping default gateway
	c.Assert(s.pingTestToNonContainer(containersA, []string{"20.1.1.254"}), IsNil)

	c.Assert(s.removeContainers(containersA), IsNil)
	c.Assert(s.cli.AppProfileDelete("aciTenant", "profile1"), IsNil)
	c.Assert(s.cli.EndpointGroupDelete("aciTenant", "epga"), IsNil)
	c.Assert(s.cli.NetworkDelete("aciTenant", "aciNet"), IsNil)
}


func (s *systemtestSuite) TestACIProfile(c *C) {
	if s.fwdMode == "routing" {
		return
	}
	c.Assert(s.cli.GlobalPost(&client.Global{
		Name:             "global",
		NetworkInfraType: "aci",
		Vlans:            "1120-1200",
		Vxlans:           "1-10000",
		FwdMode:          "bridge",
	}), IsNil)
	c.Assert(s.cli.TenantPost(&client.Tenant{
		TenantName: "aciTenant",
	}), IsNil)

	for i := 0; i < 2; i++ {
		log.Infof(">>ITERATION #%d<<", i)
		c.Assert(s.cli.NetworkPost(&client.Network{
			TenantName:  "aciTenant",
			NetworkName: "aciNet",
			Subnet:      "20.1.1.0/24",
			Gateway:     "20.1.1.254",
			Encap:       "vlan",
		}), IsNil)

		c.Assert(s.cli.EndpointGroupPost(&client.EndpointGroup{
			TenantName:  "aciTenant",
			NetworkName: "aciNet",
			GroupName:   "epgA",
		}), IsNil)

		c.Assert(s.cli.EndpointGroupPost(&client.EndpointGroup{
			TenantName:  "aciTenant",
			NetworkName: "aciNet",
			GroupName:   "epgB",
		}), IsNil)

		c.Assert(s.cli.AppProfilePost(&client.AppProfile{
			TenantName:     "aciTenant",
			EndpointGroups: []string{"epgA", "epgB"},
			AppProfileName: "profile1",
		}), IsNil)

		time.Sleep(5 * time.Second)
		cA1, err := s.nodes[0].runContainer(containerSpec{networkName: "epgA/aciTenant"})
		c.Assert(err, IsNil)

		// Verify cA1 can ping default gateway
		c.Assert(cA1.checkPingWithCount("20.1.1.254", 5), IsNil)

		cB1, err := s.nodes[0].runContainer(containerSpec{networkName: "epgB/aciTenant"})
		c.Assert(err, IsNil)

		// Verify cA1 cannot ping cB1
		c.Assert(cA1.checkPingFailureWithCount(cB1.eth0.ip, 5), IsNil)
		// Verify cB1 can ping default gateway
		c.Assert(cB1.checkPingWithCount("20.1.1.254", 5), IsNil)

		// Create a policy that allows ICMP and apply between A and B
		c.Assert(s.cli.PolicyPost(&client.Policy{
			PolicyName: "policyAB",
			TenantName: "aciTenant",
		}), IsNil)

		c.Assert(s.cli.RulePost(&client.Rule{
			RuleID:            "1",
			PolicyName:        "policyAB",
			TenantName:        "aciTenant",
			FromEndpointGroup: "epgA",
			Direction:         "in",
			Protocol:          "icmp",
			Action:            "allow",
		}), IsNil)

		c.Assert(s.cli.EndpointGroupPost(&client.EndpointGroup{
			TenantName:  "aciTenant",
			NetworkName: "aciNet",
			Policies:    []string{"policyAB"},
			GroupName:   "epgB",
		}), IsNil)
		time.Sleep(time.Second * 5)

		c.Assert(checkACILearning("aciTenant",
			"profile1",
			"epgA",
			cA1), IsNil)

		c.Assert(checkACILearning("aciTenant",
			"profile1",
			"epgB",
			cB1), IsNil)

		// Verify cA1 can ping cB1
		cA1.checkPingWithCount(cB1.eth0.ip, 5)
		cA1.checkPingWithCount(cB1.eth0.ip, 5)
		cA1.checkPingWithCount(cB1.eth0.ip, 5)
		c.Assert(cA1.checkPingWithCount(cB1.eth0.ip, 5), IsNil)

		// Verify TCP is not allowed.
		containers := []*container{cA1, cB1}
		from := []*container{cA1}
		to := []*container{cB1}

		c.Assert(s.startListeners(containers, []int{8000, 8001}), IsNil)
		c.Assert(s.checkNoConnectionPairRetry(from, to, 8000, 1, 3), IsNil)
		c.Assert(s.checkNoConnectionPairRetry(from, to, 8001, 1, 3), IsNil)

		// Add a rule to allow 8000
		c.Assert(s.cli.RulePost(&client.Rule{
			RuleID:            "2",
			PolicyName:        "policyAB",
			TenantName:        "aciTenant",
			FromEndpointGroup: "epgA",
			Direction:         "in",
			Protocol:          "tcp",
			Port:              8000,
			Action:            "allow",
		}), IsNil)
		time.Sleep(time.Second * 5)
		c.Assert(checkACILearning("aciTenant",
			"profile1",
			"epgA",
			cA1), IsNil)

		c.Assert(checkACILearning("aciTenant",
			"profile1",
			"epgB",
			cB1), IsNil)

		c.Assert(s.checkConnectionPairRetry(from, to, 8000, 1, 3), IsNil)
		c.Assert(s.checkNoConnectionPairRetry(from, to, 8001, 1, 3), IsNil)
		c.Assert(cA1.checkPingWithCount(cB1.eth0.ip, 5), IsNil)

		// Add a rule to allow 8001
		c.Assert(s.cli.RulePost(&client.Rule{
			RuleID:            "3",
			PolicyName:        "policyAB",
			TenantName:        "aciTenant",
			FromEndpointGroup: "epgA",
			Direction:         "in",
			Protocol:          "tcp",
			Port:              8001,
			Action:            "allow",
		}), IsNil)
		//cA1.checkPing("20.1.1.254")
		//cB1.checkPing("20.1.1.254")
		time.Sleep(time.Second * 5)

		c.Assert(checkACILearning("aciTenant",
			"profile1",
			"epgA",
			cA1), IsNil)

		c.Assert(checkACILearning("aciTenant",
			"profile1",
			"epgB",
			cB1), IsNil)

		c.Assert(s.checkConnectionPairRetry(from, to, 8000, 1, 3), IsNil)
		c.Assert(s.checkConnectionPairRetry(from, to, 8001, 1, 3), IsNil)
		c.Assert(cA1.checkPingWithCount(cB1.eth0.ip, 5), IsNil)

		// Delete ICMP rule
		c.Assert(s.cli.RuleDelete("aciTenant", "policyAB", "1"), IsNil)
		time.Sleep(time.Second * 5)

		c.Assert(checkACILearning("aciTenant",
			"profile1",
			"epgA",
			cA1), IsNil)

		c.Assert(checkACILearning("aciTenant",
			"profile1",
			"epgB",
			cB1), IsNil)

		c.Assert(cA1.checkPingFailureWithCount(cB1.eth0.ip, 5), IsNil)
		c.Assert(s.checkConnectionPairRetry(from, to, 8000, 1, 3), IsNil)
		c.Assert(s.checkConnectionPairRetry(from, to, 8001, 1, 3), IsNil)

		// Delete TCP 8000 rule
		c.Assert(s.cli.RuleDelete("aciTenant", "policyAB", "2"), IsNil)
		time.Sleep(time.Second * 5)
		c.Assert(checkACILearning("aciTenant",
			"profile1",
			"epgA",
			cA1), IsNil)

		c.Assert(checkACILearning("aciTenant",
			"profile1",
			"epgB",
			cB1), IsNil)

		c.Assert(s.checkNoConnectionPairRetry(from, to, 8000, 1, 3), IsNil)
		c.Assert(s.checkConnectionPairRetry(from, to, 8001, 1, 3), IsNil)
		c.Assert(cA1.checkPingFailureWithCount(cB1.eth0.ip, 5), IsNil)

		// Delete the app profile
		c.Assert(s.cli.AppProfileDelete("aciTenant", "profile1"), IsNil)
		time.Sleep(time.Second * 5)
		//cA1.checkPingWithCount("20.1.1.254", 5)
		//cB1.checkPingWithCount("20.1.1.254", 5)
		c.Assert(s.checkNoConnectionPairRetry(from, to, 8000, 1, 3), IsNil)
		c.Assert(s.checkNoConnectionPairRetry(from, to, 8001, 1, 3), IsNil)
		c.Assert(cA1.checkPingFailureWithCount(cB1.eth0.ip, 5), IsNil)

		// Create the app profile with a different name
		c.Assert(s.cli.AppProfilePost(&client.AppProfile{
			TenantName:     "aciTenant",
			EndpointGroups: []string{"epgA", "epgB"},
			AppProfileName: "profile2",
		}), IsNil)
		time.Sleep(time.Second * 5)
		c.Assert(checkACILearning("aciTenant",
			"profile2",
			"epgA",
			cA1), IsNil)

		c.Assert(checkACILearning("aciTenant",
			"profile2",
			"epgB",
			cB1), IsNil)

		//cA1.checkPingWithCount("20.1.1.254", 5)
		//cB1.checkPingWithCount("20.1.1.254", 5)
		cA2, err := s.nodes[0].runContainer(containerSpec{networkName: "epgA/aciTenant"})
		c.Assert(err, IsNil)
		cB2, err := s.nodes[0].runContainer(containerSpec{networkName: "epgB/aciTenant"})
		c.Assert(err, IsNil)
		time.Sleep(time.Second * 10)
		from = []*container{cA2}
		to = []*container{cB2}
		c.Assert(s.startListeners([]*container{cA2, cB2}, []int{8000, 8001}), IsNil)

		c.Assert(s.checkNoConnectionPairRetry(from, to, 8000, 1, 3), IsNil)
		c.Assert(s.checkConnectionPairRetry(from, to, 8001, 1, 3), IsNil)
		c.Assert(cA2.checkPingFailureWithCount(cB2.eth0.ip, 5), IsNil)

		// Add TCP 8000 rule.
		c.Assert(s.cli.RulePost(&client.Rule{
			RuleID:            "2",
			PolicyName:        "policyAB",
			TenantName:        "aciTenant",
			FromEndpointGroup: "epgA",
			Direction:         "in",
			Protocol:          "tcp",
			Port:              8000,
			Action:            "allow",
		}), IsNil)
		err = errors.New("Forced")
		//c.Assert(err, IsNil)
		time.Sleep(time.Second * 5)
		c.Assert(checkACILearning("aciTenant",
			"profile2",
			"epgA",
			cA2), IsNil)

		c.Assert(checkACILearning("aciTenant",
			"profile2",
			"epgB",
			cB2), IsNil)

		c.Assert(s.checkConnectionPairRetry(from, to, 8000, 1, 3), IsNil)
		c.Assert(s.checkConnectionPairRetry(from, to, 8001, 1, 3), IsNil)
		c.Assert(cA2.node.exec.checkPingFailureWithCount(cA2, cB2.eth0.ip, 5), IsNil)

		// Delete the app profile
		c.Assert(s.cli.AppProfileDelete("aciTenant", "profile2"), IsNil)
		time.Sleep(time.Second * 5)

		c.Assert(s.checkNoConnectionPairRetry(from, to, 8000, 1, 3), IsNil)
		c.Assert(s.checkNoConnectionPairRetry(from, to, 8001, 1, 3), IsNil)
		c.Assert(cA2.node.exec.checkPingFailureWithCount(cA2, cB2.eth0.ip, 5), IsNil)

		c.Assert(s.removeContainers([]*container{cA1, cB1, cA2, cB2}), IsNil)
		c.Assert(s.cli.EndpointGroupDelete("aciTenant", "epgA"), IsNil)
		c.Assert(s.cli.EndpointGroupDelete("aciTenant", "epgB"), IsNil)
		c.Assert(s.cli.RuleDelete("aciTenant", "policyAB", "2"), IsNil)
		c.Assert(s.cli.RuleDelete("aciTenant", "policyAB", "3"), IsNil)
		c.Assert(s.cli.PolicyDelete("aciTenant", "policyAB"), IsNil)
		c.Assert(s.cli.NetworkDelete("aciTenant", "aciNet"), IsNil)
	}
}

func (s *systemtestSuite) AciTestSetup(c *C) {

	log.Infof("ACI_SYS_TEST_MODE is ON")
	log.Infof("Private keyFile = %s", s.keyFile)
	log.Infof("Binary binpath = %s", s.binpath)
	log.Infof("Interface vlanIf = %s", s.vlanIf)

	s.baremetal = vagrantssh.Baremetal{}
	bm := &s.baremetal

	// To fill the hostInfo data structure for Baremetal VMs
	name := "aci-swarm-node"
	hostIPs := strings.Split(os.Getenv("HOST_IPS"), ",")
	hostNames := strings.Split(os.Getenv("HOST_USER_NAMES"), ",")
	hosts := make([]vagrantssh.HostInfo, 2)

	for i := range hostIPs {
		hosts[i].Name = name + strconv.Itoa(i+1)
		log.Infof("Name=%s", hosts[i].Name)

		hosts[i].SSHAddr = hostIPs[i]
		log.Infof("SHAddr=%s", hosts[i].SSHAddr)

		hosts[i].SSHPort = "22"

		hosts[i].User = hostNames[i]
		log.Infof("User=%s", hosts[i].User)

		hosts[i].PrivKeyFile = s.keyFile
		log.Infof("PrivKeyFile=%s", hosts[i].PrivKeyFile)
	}

	c.Assert(bm.Setup(hosts), IsNil)

	s.nodes = []*node{}

	for _, nodeObj := range s.baremetal.GetNodes() {
		s.nodes = append(s.nodes, &node{tbnode: nodeObj, suite: s})
	}

	log.Info("Pulling alpine on all nodes")

	s.baremetal.IterateNodes(func(node vagrantssh.TestbedNode) error {
		node.RunCommand("sudo rm /tmp/*net*")
		return node.RunCommand("docker pull alpine")
	})

	//Copying binaries
	s.copyBinary("netmaster")
	s.copyBinary("netplugin")
	s.copyBinary("netctl")
	s.copyBinary("contivk8s")

}*/
