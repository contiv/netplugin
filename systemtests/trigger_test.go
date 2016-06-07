package systemtests

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	. "gopkg.in/check.v1"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/contivmodel/client"
)

func (s *systemtestSuite) TestTriggerNetmasterSwitchover(c *C) {
	network := &client.Network{
		TenantName:  "default",
		NetworkName: "private",
		Subnet:      "10.1.0.0/16",
		Gateway:     "10.1.1.254",
		PktTag:      1001,
		Encap:       "vxlan",
	}
	c.Assert(s.cli.NetworkPost(network), IsNil)

	for i := 0; i < s.iterations; i++ {
		containers, err := s.runContainers(s.containers, false, "private", nil, nil)
		c.Assert(err, IsNil)

		var leader, oldLeader *node

		leaderIP, err := s.clusterStoreGet("/contiv.io/lock/netmaster/leader")
		c.Assert(err, IsNil)

		for _, node := range s.nodes {
			res, err := node.getIPAddr("eth1")
			c.Assert(err, IsNil)
			if leaderIP == res {
				leader = node
				logrus.Infof("Found leader %s/%s", node.Name(), leaderIP)
			}
		}

		c.Assert(leader.stopNetmaster(), IsNil)
		c.Assert(leader.rotateLog("netmaster"), IsNil)

		for x := 0; x < 15; x++ {
			logrus.Info("Waiting 5s for leader to change...")
			newLeaderIP, err := s.clusterStoreGet("/contiv.io/lock/netmaster/leader")
			c.Assert(err, IsNil)

			for _, node := range s.nodes {
				res, err := node.getIPAddr("eth1")
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
		c.Assert(oldLeader.startNetmaster(), IsNil)
		time.Sleep(5 * time.Second)

		c.Assert(s.pingTest(containers), IsNil)

		c.Assert(s.removeContainers(containers), IsNil)
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
	c.Assert(s.cli.NetworkPost(network), IsNil)

	for i := 0; i < s.iterations; i++ {
		containers, err := s.runContainers(s.containers, false, "private", nil, nil)
		c.Assert(err, IsNil)

		for _, node := range s.nodes {
			c.Assert(node.stopNetplugin(), IsNil)
			logrus.Info("Sleeping for a while to wait for netplugin's TTLs to expire")
			time.Sleep(50 * time.Second)
			c.Assert(node.rotateLog("netplugin"), IsNil)
			if s.fwdMode == "routing" {
				c.Assert(node.startNetplugin("-fwd-mode=routing"), IsNil)
			} else {
				c.Assert(node.startNetplugin(""), IsNil)
			}
			c.Assert(node.runCommandUntilNoError("pgrep netplugin"), IsNil)
			time.Sleep(20 * time.Second)

			c.Assert(s.pingTest(containers), IsNil)
		}

		c.Assert(s.removeContainers(containers), IsNil)
	}
}

func (s *systemtestSuite) TestTriggerNodeReload(c *C) {
	if os.Getenv("CONTIV_DOCKER_VERSION") != "1.11.1" {
		c.Skip("Skipping node reload test on older docker version")
	}
	network := &client.Network{
		TenantName:  "default",
		NetworkName: "private",
		Subnet:      "10.1.0.0/16",
		Gateway:     "10.1.1.254",
		Encap:       "vxlan",
	}
	c.Assert(s.cli.NetworkPost(network), IsNil)

	numContainers := s.containers
	if numContainers < (len(s.nodes) * 2) {
		numContainers = len(s.nodes) * 2
	}
	cntPerNode := numContainers / len(s.nodes)

	for i := 0; i < s.iterations; i++ {
		containers := []*container{}

		// start containers on all nodes
		for _, node := range s.nodes {
			newContainers, err := s.runContainersOnNode(cntPerNode, "private", node)
			c.Assert(err, IsNil)
			containers = append(containers, newContainers...)
		}

		// test ping for all containers
		c.Assert(s.pingTest(containers), IsNil)

		// reload VMs one at a time
		for _, node := range s.nodes {
			c.Assert(node.reloadNode(), IsNil)
			c.Assert(node.rotateLog("netplugin"), IsNil)
			c.Assert(node.rotateLog("netmaster"), IsNil)

			if s.fwdMode == "routing" {
				c.Assert(node.startNetplugin("-fwd-mode=routing"), IsNil)
			} else {
				c.Assert(node.startNetplugin(""), IsNil)
			}
			c.Assert(node.runCommandUntilNoError("pgrep netplugin"), IsNil)
			time.Sleep(20 * time.Second)
			c.Assert(node.startNetmaster(), IsNil)
			time.Sleep(1 * time.Second)
			c.Assert(node.runCommandUntilNoError("pgrep netmaster"), IsNil)
			time.Sleep(20 * time.Second)

			// clear previous containers from reloaded node
			node.cleanupContainers()

			exContainers := []*container{}
			for _, cont := range containers {
				if cont.node != node {
					exContainers = append(exContainers, cont)
				} else {
					logrus.Infof("Removing container %s", cont.containerID)
				}
			}

			// start new containers on reloaded node
			newContainers, err := s.runContainersOnNode(cntPerNode, "private", node)
			c.Assert(err, IsNil)
			containers = append(exContainers, newContainers...)

			// test ping for all containers
			c.Assert(s.pingTest(containers), IsNil)
		}

		c.Assert(s.removeContainers(containers), IsNil)
	}
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
	c.Assert(s.cli.NetworkPost(network), IsNil)

	network = &client.Network{
		TenantName:  "default",
		NetworkName: "other",
		Subnet:      "10.2.0.0/16",
		Gateway:     "10.2.1.254",
		PktTag:      1002,
		Encap:       "vxlan",
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

	for i := 0; i < (s.iterations * 6); i++ {
		switch rand.Int() % 3 {
		case 0:
			logrus.Info("Triggering netplugin restart")
			for _, node := range s.nodes {
				c.Assert(node.stopNetplugin(), IsNil)
				c.Assert(node.rotateLog("netplugin"), IsNil)
				if s.fwdMode == "routing" {
					c.Assert(node.startNetplugin("-fwd-mode=routing"), IsNil)
				} else {
					c.Assert(node.startNetplugin(""), IsNil)
				}
				c.Assert(node.runCommandUntilNoError("pgrep netplugin"), IsNil)
				time.Sleep(20 * time.Second)
			}
		case 1:
			logrus.Info("Triggering netmaster restart")
			for _, node := range s.nodes {
				c.Assert(node.stopNetmaster(), IsNil)
				c.Assert(node.rotateLog("netmaster"), IsNil)

				time.Sleep(1 * time.Second)

				c.Assert(node.startNetmaster(), IsNil)
				time.Sleep(1 * time.Second)
				c.Assert(node.runCommandUntilNoError("pgrep netmaster"), IsNil)
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
	netContainers, err := s.runContainers(s.containers, false, "private", nil, nil)
	if err != nil {
		return nil, nil, nil, err
	}

	groupMapContainers, err := s.runContainersInGroups(s.containers, "other", groupNames)
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
