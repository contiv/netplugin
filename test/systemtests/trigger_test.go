package systemtests

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	. "github.com/contiv/check"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/contivmodel/client"
)

func (s *systemtestSuite) TestTriggerNetpluginUplinkUpgrade(c *C) {
	uplinkIntfs := strings.Split(s.hostInfo.HostDataInterfaces, ",")
	if len(uplinkIntfs) == 1 {
		c.Skip("Skipping upgrade test for single uplink interface")
	}

	// Take backup of interfaces
	originalUplinks := s.hostInfo.HostDataInterfaces
	singleUplink := uplinkIntfs[0]

	for _, node := range s.nodes {
		// Stop Netplugin
		c.Assert(node.exec.stopNetplugin(), IsNil)
		c.Assert(node.rotateLog("netplugin"), IsNil)
		node.cleanupSlave()

		// Run test case with just one single uplink
		logrus.Info("Verifying single uplink case")
		s.hostInfo.HostDataInterfaces = singleUplink
		c.Assert(node.startNetplugin(""), IsNil)
		c.Assert(node.exec.runCommandUntilNoNetpluginError(), IsNil)
		time.Sleep(20 * time.Second)
		c.Assert(node.waitForListeners(), IsNil)
		// Verify uplink state on each node
		c.Assert(node.verifyUplinkState([]string{singleUplink}), IsNil)

		// Uplink upgrade case. Run test case with multiple uplinks
		c.Assert(node.exec.stopNetplugin(), IsNil)
		c.Assert(node.rotateLog("netplugin"), IsNil)
		logrus.Info("Verifying uplink upgrade case with multiple uplinks")
		s.hostInfo.HostDataInterfaces = originalUplinks
		c.Assert(node.startNetplugin(""), IsNil)
		c.Assert(node.exec.runCommandUntilNoNetpluginError(), IsNil)
		time.Sleep(20 * time.Second)
		c.Assert(node.waitForListeners(), IsNil)
		// Verify uplink state on each node
		c.Assert(node.verifyUplinkState(uplinkIntfs), IsNil)

		// Uplink downgrade Rerun test case with just one single uplink
		logrus.Info("Verifying uplink downgrade case with single uplink")
		c.Assert(node.exec.stopNetplugin(), IsNil)
		c.Assert(node.rotateLog("netplugin"), IsNil)
		s.hostInfo.HostDataInterfaces = singleUplink
		c.Assert(node.startNetplugin(""), IsNil)
		c.Assert(node.exec.runCommandUntilNoNetpluginError(), IsNil)
		time.Sleep(20 * time.Second)
		c.Assert(node.waitForListeners(), IsNil)
		// Verify uplink state on each node
		c.Assert(node.verifyUplinkState([]string{singleUplink}), IsNil)

		s.hostInfo.HostDataInterfaces = originalUplinks
	}
}

func (s *systemtestSuite) TestTriggerNetmasterSwitchover(c *C) {

	if s.basicInfo.Scheduler == kubeScheduler {
		return
	}

	network := &client.Network{
		TenantName:  "default",
		NetworkName: "private",
		Subnet:      "10.1.0.0/16",
		Gateway:     "10.1.1.254",
		PktTag:      1001,
		Encap:       "vxlan",
	}
	if s.fwdMode != "routing" {
		network.Ipv6Subnet = "2016:0617::/100"
		network.Ipv6Gateway = "2016:0617::254"
	}
	c.Assert(s.cli.NetworkPost(network), IsNil)

	for i := 0; i < s.basicInfo.Iterations; i++ {
		containers, err := s.runContainers(s.basicInfo.Containers, false, "private", "", nil, nil)
		c.Assert(err, IsNil)

		var leader, oldLeader *node

		leaderURL, err := s.clusterStoreGet("/contiv.io/lock/netmaster/leader")
		c.Assert(err, IsNil)

		leaderIP := strings.Split(leaderURL, ":")[0]

		for _, node := range s.nodes {
			res, err := node.getIPAddr(s.hostInfo.HostMgmtInterface)
			c.Assert(err, IsNil)
			if leaderIP == res {
				leader = node
				logrus.Infof("Found leader %s/%s", node.Name(), leaderIP)
				break
			}
		}

		c.Assert(leader.exec.stopNetmaster(), IsNil)
		c.Assert(leader.rotateLog("netmaster"), IsNil)

		for x := 0; x < 15; x++ {
			logrus.Info("Waiting 5s for leader to change...")
			newLeaderURL, err := s.clusterStoreGet("/contiv.io/lock/netmaster/leader")
			c.Assert(err, IsNil)

			newLeaderIP := strings.Split(newLeaderURL, ":")[0]

			for _, node := range s.nodes {
				res, err := node.getIPAddr(s.hostInfo.HostMgmtInterface)
				c.Assert(err, IsNil)
				if res == newLeaderIP && res != leaderIP {
					oldLeader = leader
					leader = node
					logrus.Infof("Leader switched to %s/%s", node.Name(), newLeaderIP)
					goto finished
				}
			}

			time.Sleep(5 * time.Second)
		}

	finished:
		c.Assert(oldLeader.exec.startNetmaster(""), IsNil)
		time.Sleep(5 * time.Second)

		c.Assert(s.pingTest(containers), IsNil)

		c.Assert(s.removeContainers(containers), IsNil)
	}

	// delete the network
	c.Assert(s.cli.NetworkDelete("default", "private"), IsNil)
}

func (s *systemtestSuite) TestTriggerNetmasterControlPortSwitch(c *C) {
	var masterPort string
	network := &client.Network{
		TenantName:  "default",
		NetworkName: "private",
		Subnet:      "10.1.0.0/16",
		Gateway:     "10.1.1.254",
		PktTag:      1001,
		Encap:       "vxlan",
	}
	if s.fwdMode != "routing" {
		network.Ipv6Subnet = "2016:0617::/100"
		network.Ipv6Gateway = "2016:0617::254"
	}

	portBase := []string{"888", "999"}

	for i := 0; i < s.basicInfo.Iterations; i++ {
		for _, node := range s.nodes {
			c.Assert(node.stopNetmaster(), IsNil)
			c.Assert(node.rotateLog("netmaster"), IsNil)
			nodeIP, err := node.getIPAddr(s.hostInfo.HostMgmtInterface)
			c.Assert(err, IsNil)
			masterPort = portBase[i%2] + nodeIP[len(nodeIP)-1:]
			controlURLArg := "--listen-url " + "0.0.0.0:" + masterPort
			c.Assert(node.startNetmaster(controlURLArg), IsNil)
			logrus.Info("Sleeping for a while to wait for netmaster to restart")
			time.Sleep(15 * time.Second)
		}
		time.Sleep(20 * time.Second)
		leaderURL, err := s.clusterStoreGet("/contiv.io/lock/netmaster/leader")
		c.Assert(err, IsNil)
		leaderIP := strings.Split(leaderURL, ":")[0]
		leaderPort := strings.Split(leaderURL, ":")[1]
		masterPort = portBase[i%2] + leaderIP[len(leaderIP)-1:]

		if strings.Compare(leaderPort, masterPort) != 0 {
			err = fmt.Errorf("Netmaster port not using port %s. Using port: %s", masterPort, leaderPort)
		}
		c.Assert(err, IsNil)
		clientURL := fmt.Sprintf("http://localhost:%s", leaderPort)
		cliClient, err := client.NewContivClient(clientURL)
		if err != nil {
			logrus.Errorf("Error initializing the contiv client. Err: %+v", err)
		}

		c.Assert(cliClient.NetworkPost(network), IsNil)
		containers, err := s.runContainers(s.basicInfo.Containers, false, "private", "", nil, nil)
		c.Assert(err, IsNil)
		c.Assert(s.pingTest(containers), IsNil)
		c.Assert(s.removeContainers(containers), IsNil)

		// delete the network
		c.Assert(cliClient.NetworkDelete("default", "private"), IsNil)
	}
}

func (s *systemtestSuite) TestTriggerNetpluginDisconnect(c *C) {
	network := &client.Network{
		TenantName:  "default",
		NetworkName: "private",
		Subnet:      "10.1.0.0/16",
		Gateway:     "10.1.1.254",
		PktTag:      1001,
		Encap:       "vxlan",
	}
	if s.fwdMode != "routing" {
		network.Ipv6Subnet = "2016:0617::/100"
		network.Ipv6Gateway = "2016:0617::254"
	}
	c.Assert(s.cli.NetworkPost(network), IsNil)

	for i := 0; i < s.basicInfo.Iterations; i++ {
		containers, err := s.runContainers(s.basicInfo.Containers, false, "private", "", nil, nil)
		c.Assert(err, IsNil)

		for _, node := range s.nodes {
			c.Assert(node.stopNetplugin(), IsNil)
			logrus.Info("Sleeping for a while to wait for netplugin's TTLs to expire")
			time.Sleep(15 * time.Second)
			c.Assert(s.verifyNodeRemoved(node), IsNil)
			time.Sleep(5 * time.Second)
			c.Assert(node.rotateLog("netplugin"), IsNil)
			c.Assert(node.startNetplugin(""), IsNil)

			c.Assert(node.exec.runCommandUntilNoNetpluginError(), IsNil)
			time.Sleep(30 * time.Second)
			c.Assert(node.waitForListeners(), IsNil)
			c.Assert(s.verifyVTEPs(), IsNil)
			c.Assert(s.verifyEPs(containers), IsNil)
			c.Assert(s.pingTest(containers), IsNil)
		}

		c.Assert(s.removeContainers(containers), IsNil)

	}

	// delete the network
	c.Assert(s.cli.NetworkDelete("default", "private"), IsNil)
}

func (s *systemtestSuite) TestTriggerNodeReload(c *C) {
	// can not run this test on k8s
	if s.basicInfo.Scheduler == kubeScheduler {
		c.Skip("Skipping node reload test for k8s")
	}

	network := &client.Network{
		TenantName:  "default",
		NetworkName: "private",
		Subnet:      "10.1.0.0/16",
		Gateway:     "10.1.1.254",
		Encap:       "vxlan",
	}
	if s.fwdMode != "routing" {
		network.Ipv6Subnet = "2016:0617::/100"
		network.Ipv6Gateway = "2016:0617::254"
	}
	c.Assert(s.cli.NetworkPost(network), IsNil)
	for _, node := range s.nodes {
		c.Assert(node.checkSchedulerNetworkOnNodeCreated([]string{"private"}), IsNil)
	}

	numContainers := s.basicInfo.Containers
	if numContainers < (len(s.nodes) * 2) {
		numContainers = len(s.nodes) * 2
	}
	cntPerNode := numContainers / len(s.nodes)

	for i := 0; i < s.basicInfo.Iterations; i++ {
		containers := []*container{}

		// start containers on all nodes
		for _, node := range s.nodes {
			newContainers, err := s.runContainersOnNode(cntPerNode, "private", "", "", node)
			c.Assert(err, IsNil)
			containers = append(containers, newContainers...)
		}

		// test ping for all containers
		c.Assert(s.pingTest(containers), IsNil)

		// reload VMs one at a time
		for _, node := range s.nodes {
			if s.basicInfo.Scheduler == kubeScheduler && node.Name() == "k8master" {
				continue
			}
			c.Assert(node.reloadNode(), IsNil)

			time.Sleep(20 * time.Second)
			c.Assert(node.startNetplugin(""), IsNil)
			c.Assert(node.exec.runCommandUntilNoNetpluginError(), IsNil)
			time.Sleep(20 * time.Second)
			c.Assert(node.startNetmaster(""), IsNil)
			time.Sleep(1 * time.Second)
			c.Assert(node.exec.runCommandUntilNoNetmasterError(), IsNil)
			time.Sleep(20 * time.Second)

			// clear previous containers from reloaded node
			node.exec.cleanupContainers()

			exContainers := []*container{}
			for _, cont := range containers {
				if cont.node != node {
					exContainers = append(exContainers, cont)
				} else {
					logrus.Infof("Removing container %s", cont.containerID)
				}
			}

			// start new containers on reloaded node
			newContainers, err := s.runContainersOnNode(cntPerNode, "private", "", "", node)
			c.Assert(err, IsNil)
			containers = append(exContainers, newContainers...)

			// test ping for all containers
			c.Assert(s.pingTest(containers), IsNil)
		}

		c.Assert(s.removeContainers(containers), IsNil)
	}
}

func (s *systemtestSuite) TestTriggerClusterStoreRestart(c *C) {
	c.Skip("Skipping this tests temporarily")

	// create network
	network := &client.Network{
		TenantName:  "default",
		NetworkName: "private",
		Subnet:      "10.1.1.0/24",
		Gateway:     "10.1.1.254",
		Encap:       "vxlan",
	}
	c.Assert(s.cli.NetworkPost(network), IsNil)

	for i := 0; i < s.basicInfo.Iterations; i++ {
		containers, err := s.runContainers(s.basicInfo.Containers, false, "private", "default", nil, nil)
		c.Assert(err, IsNil)

		// test ping for all containers
		c.Assert(s.pingTest(containers), IsNil)

		// reload VMs one at a time
		for _, node := range s.nodes {
			c.Assert(node.restartClusterStore(), IsNil)

			time.Sleep(20 * time.Second)
			c.Assert(s.verifyVTEPs(), IsNil)

			c.Assert(s.verifyEPs(containers), IsNil)
			time.Sleep(2 * time.Second)

			// test ping for all containers
			c.Assert(s.pingTest(containers), IsNil)
		}

		c.Assert(s.removeContainers(containers), IsNil)
	}

	// delete the network
	c.Assert(s.cli.NetworkDelete("default", "private"), IsNil)
}

// TestTriggerNetPartition tests network partition by flapping uplink
func (s *systemtestSuite) TestTriggerNetPartition(c *C) {
	// create network
	network := &client.Network{
		TenantName:  "default",
		NetworkName: "private",
		Subnet:      "10.1.1.0/24",
		Gateway:     "10.1.1.254",
		Encap:       "vxlan",
	}
	c.Assert(s.cli.NetworkPost(network), IsNil)

	for i := 0; i < s.basicInfo.Iterations; i++ {

		containers, err := s.runContainers(s.basicInfo.Containers, false, "private", "default", nil, nil)
		c.Assert(err, IsNil)

		// test ping for all containers
		c.Assert(s.pingTest(containers), IsNil)

		// reload VMs one at a time
		for _, node := range s.nodes {
			if s.basicInfo.Scheduler == kubeScheduler && node.Name() == "k8master" {
				continue
			}
			nodeIP, err := node.getIPAddr("eth1")
			c.Assert(err, IsNil)

			// flap the control interface
			c.Assert(node.bringDownIf("eth1"), IsNil)
			time.Sleep(25 * time.Second) // wait till sessions/locks timeout
			c.Assert(node.bringUpIf("eth1", nodeIP), IsNil)
			time.Sleep(5 * time.Second) // wait till sessions/locks timeout

			c.Assert(s.verifyVTEPs(), IsNil)

			c.Assert(s.verifyEPs(containers), IsNil)
			time.Sleep(2 * time.Second)
			// test ping for all containers
			c.Assert(s.pingTest(containers), IsNil)
		}

		for _, node := range s.nodes {
			c.Assert(node.checkSchedulerNetworkOnNodeCreated([]string{"private"}), IsNil)
		}
		c.Assert(s.removeContainers(containers), IsNil)
	}

	// delete the network
	c.Assert(s.cli.NetworkDelete("default", "private"), IsNil)
}

func (s *systemtestSuite) TestTriggers(c *C) {

	groupNames := []string{}
	network := &client.Network{
		TenantName:  "default",
		NetworkName: "private",
		Subnet:      "10.1.0.0/16",
		Gateway:     "10.1.1.254",
		PktTag:      1001,
		Encap:       "vxlan",
	}
	if s.fwdMode != "routing" {
		network.Ipv6Subnet = "2016:0617::/100"
		network.Ipv6Gateway = "2016:0617::254"
	}
	c.Assert(s.cli.NetworkPost(network), IsNil)

	network = &client.Network{
		TenantName:  "default",
		NetworkName: "other",
		Subnet:      "10.2.0.0/16",
		Gateway:     "10.2.1.254",
		PktTag:      1002,
		Encap:       "vxlan",
	}
	if s.fwdMode != "routing" {
		network.Ipv6Subnet = "2016:0718::/100"
		network.Ipv6Gateway = "2016:0718::254"
	}
	c.Assert(s.cli.NetworkPost(network), IsNil)

	for policyNum := 0; policyNum < 2; policyNum++ {
		srvName := fmt.Sprintf("srv%d", policyNum)

		policy := &client.Policy{
			PolicyName: srvName,
			TenantName: "default",
		}

		c.Assert(s.cli.PolicyPost(policy), IsNil)

		group := &client.EndpointGroup{
			GroupName:   srvName,
			Policies:    []string{srvName},
			TenantName:  "default",
			NetworkName: "other",
		}

		c.Assert(s.cli.EndpointGroupPost(group), IsNil)

		groupNames = append(groupNames, group.GroupName)

		rules := []*client.Rule{
			{
				RuleID:     "1",
				PolicyName: srvName,
				TenantName: "default",
				Direction:  "in",
				Protocol:   "tcp",
				Action:     "deny",
			},
			{
				RuleID:     "2",
				PolicyName: srvName,
				TenantName: "default",
				Priority:   100,
				Direction:  "in",
				Protocol:   "tcp",
				Port:       8000,
				Action:     "allow",
			},
			{
				RuleID:            "3",
				Direction:         "in",
				PolicyName:        srvName,
				TenantName:        "default",
				Priority:          100,
				FromEndpointGroup: srvName,
				Protocol:          "tcp",
				Port:              8001,
				Action:            "allow",
			},
		}

		for _, rule := range rules {
			c.Assert(s.cli.RulePost(rule), IsNil)
		}
	}

	// here

	netMapContainers, groupMapContainers, containers, err := s.runTriggerContainers(groupNames)
	c.Assert(err, IsNil)
	c.Assert(s.checkAllConnection(netMapContainers, groupMapContainers), IsNil)

	for i := 0; i < (s.basicInfo.Iterations * 6); i++ {
		switch rand.Int() % 3 {
		case 0:
			logrus.Info("Triggering netplugin restart")
			for _, node := range s.nodes {
				c.Assert(node.exec.stopNetplugin(), IsNil)
				c.Assert(node.rotateLog("netplugin"), IsNil)
				c.Assert(node.startNetplugin(""), IsNil)
				c.Assert(node.exec.runCommandUntilNoNetpluginError(), IsNil)
				time.Sleep(30 * time.Second)
			}
		case 1:
			logrus.Info("Triggering netmaster restart")
			for _, node := range s.nodes {
				c.Assert(node.exec.stopNetmaster(), IsNil)
				c.Assert(node.rotateLog("netmaster"), IsNil)

				time.Sleep(1 * time.Second)

				c.Assert(node.exec.startNetmaster(""), IsNil)
				time.Sleep(1 * time.Second)
				c.Assert(node.exec.runCommandUntilNoNetmasterError(), IsNil)
			}
			time.Sleep(30 * time.Second)
		case 2:
			logrus.Info("Reloading containers")
			c.Assert(s.removeContainers(containers), IsNil)
			netMapContainers, groupMapContainers, containers, err = s.runTriggerContainers(groupNames)
			c.Assert(err, IsNil)
		}

		c.Assert(s.checkAllConnection(netMapContainers, groupMapContainers), IsNil)
	}

	c.Assert(s.removeContainers(containers), IsNil)

	for _, group := range groupNames {
		c.Assert(s.cli.EndpointGroupDelete("default", group), IsNil)
		c.Assert(s.cli.PolicyDelete("default", group), IsNil)
	}

	c.Assert(s.cli.NetworkDelete("default", "other"), IsNil)
	c.Assert(s.cli.NetworkDelete("default", "private"), IsNil)
}

func (s *systemtestSuite) runTriggerContainers(groupNames []string) (map[*container]string, map[*container]string, []*container, error) {
	netContainers, err := s.runContainers(s.basicInfo.Containers, false, "private", "", nil, nil)
	if err != nil {
		return nil, nil, nil, err
	}

	groupMapContainers, err := s.runContainersInGroups(s.basicInfo.Containers, "other", "", groupNames)
	if err != nil {
		return nil, nil, nil, err
	}

	netMapContainers := map[*container]string{}

	for _, cont := range netContainers {
		netMapContainers[cont] = "private"
	}

	containers := []*container{}
	for _, cont := range netContainers {
		containers = append(containers, cont)
	}

	for cont := range groupMapContainers {
		containers = append(containers, cont)
	}

	if err := s.startListeners(containers, []int{8000, 8001}); err != nil {
		return nil, nil, nil, err
	}

	return netMapContainers, groupMapContainers, containers, nil
}
