package systemtests

import (
	"fmt"
	"math/rand"
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
		containers, err := s.runContainers(s.containers, false, "private", nil)
		c.Assert(err, IsNil)

		var leader, oldLeader *node
		var leaderIP string

		list, err := s.etcdList("/contiv.io/service/netmaster/", false)
		c.Assert(err, IsNil)

		for _, item := range list {
			if item["Role"] == "leader" {
				for _, node := range s.nodes {
					res, err := node.getIPAddr("eth1")
					c.Assert(err, IsNil)
					if res == item["HostAddr"] {
						leader = node
						leaderIP = res
					}
				}
			}
		}

		c.Assert(leader.stopNetmaster(), IsNil)

		for x := 0; x < 30; x++ {
			logrus.Info("Waiting 1s for leader to change...")
			list, err := s.etcdList("/contiv.io/service/netmaster/", false)
			c.Assert(err, IsNil)

			for _, item := range list {
				if item["Role"] == "leader" {
					for _, node := range s.nodes {
						res, err := node.getIPAddr("eth1")
						c.Assert(err, IsNil)
						if res == item["HostAddr"] && res != leaderIP {
							oldLeader = leader
							leader = node
							goto finished
						}
					}
				}
			}

			time.Sleep(1 * time.Second)
		}

	finished:
		c.Assert(s.pingTest(containers), IsNil)

		c.Assert(oldLeader.startNetmaster(), IsNil)
		time.Sleep(5 * time.Second)

		c.Assert(s.removeContainers(containers), IsNil)
	}
}

func (s *systemtestSuite) TestNetpluginDisconnect(c *C) {
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
		containers, err := s.runContainers(s.containers, false, "private", nil)
		c.Assert(err, IsNil)

		for _, node := range s.nodes {
			c.Assert(node.stopNetplugin(), IsNil)
			logrus.Info("Sleeping for a while to wait for netplugin's TTLs to expire")
			time.Sleep(2 * time.Minute)
			c.Assert(node.rotateLog("netplugin"), IsNil)
			c.Assert(node.startNetplugin(""), IsNil)
			c.Assert(node.runCommandUntilNoError("pgrep netplugin"), IsNil)
			time.Sleep(15 * time.Second)

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
				FromNetwork:       "other",
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

	for i := 0; i < 18; i++ {
		switch rand.Int() % 3 {
		case 0:
			logrus.Info("Triggering netplugin restart")
			for _, node := range s.nodes {
				c.Assert(node.stopNetplugin(), IsNil)
				c.Assert(node.rotateLog("netplugin"), IsNil)
				c.Assert(node.startNetplugin(""), IsNil)
				c.Assert(node.runCommandUntilNoError("pgrep netplugin"), IsNil)
			}
		case 1:
			logrus.Info("Triggering netmaster restart")
			for _, node := range s.nodes {
				c.Assert(node.stopNetmaster(), IsNil)
				time.Sleep(1 * time.Second)
				c.Assert(node.rotateLog("netmaster"), IsNil)
				c.Assert(node.startNetmaster(), IsNil)
				c.Assert(node.runCommandUntilNoError("pgrep netmaster"), IsNil)
			}
			time.Sleep(5 * time.Second)
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
		c.Assert(s.cli.EndpointGroupDelete("default", "other", group), IsNil)
		c.Assert(s.cli.PolicyDelete("default", group), IsNil)
	}

	c.Assert(s.cli.NetworkDelete("default", "other"), IsNil)
	c.Assert(s.cli.NetworkDelete("default", "private"), IsNil)
}

func (s *systemtestSuite) runTriggerContainers(groupNames []string) (map[*container]string, map[*container]string, []*container, error) {
	netContainers, err := s.runContainers(s.containers, false, "private", nil)
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
