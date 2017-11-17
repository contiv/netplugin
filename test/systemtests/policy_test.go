package systemtests

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
	. "github.com/contiv/check"
	"github.com/contiv/netplugin/contivmodel/client"
)

func (s *systemtestSuite) TestPolicyBasicVXLAN(c *C) {
	s.testPolicyBasic(c, "vxlan")
}

func (s *systemtestSuite) TestPolicyBasicVLAN(c *C) {
	s.testPolicyBasic(c, "vlan")
}

func (s *systemtestSuite) testPolicyBasic(c *C, encap string) {

	if encap == "vlan" && s.fwdMode == "routing" {

		s.SetupBgp(c, false)
		s.CheckBgpConnection(c)
	}

	network := &client.Network{
		TenantName:  "default",
		NetworkName: "private",
		Subnet:      "10.1.0.0/16",
		Gateway:     "10.1.1.254",
		PktTag:      1001,
		Encap:       encap,
	}

	c.Assert(s.cli.NetworkPost(network), IsNil)

	for i := 0; i < s.basicInfo.Iterations; i++ {
		c.Assert(s.cli.PolicyPost(&client.Policy{
			PolicyName: "policy",
			TenantName: "default",
		}), IsNil)

		rules := []*client.Rule{
			{
				RuleID:     "1",
				PolicyName: "policy",
				TenantName: "default",
				Direction:  "in",
				Protocol:   "tcp",
				Action:     "deny",
			},
			{
				RuleID:     "2",
				PolicyName: "policy",
				TenantName: "default",
				Priority:   100,
				Direction:  "in",
				Protocol:   "tcp",
				Port:       8000,
				Action:     "allow",
			},
		}

		for _, rule := range rules {
			c.Assert(s.cli.RulePost(rule), IsNil)
		}

		groups := []*client.EndpointGroup{}
		groupNames := []string{}
		for x := 0; x < s.basicInfo.Containers; x++ {
			nodeNum := s.basicInfo.Containers % len(s.nodes)
			epgName := fmt.Sprintf("%s-srv%d-%d", network.NetworkName, nodeNum, x)
			group := &client.EndpointGroup{
				GroupName:   epgName,
				NetworkName: "private",
				Policies:    []string{"policy"},
				TenantName:  "default",
			}
			c.Assert(s.cli.EndpointGroupPost(group), IsNil)

			groups = append(groups, group)
			groupNames = append(groupNames, epgName)
		}

		containers, err := s.runContainers(s.basicInfo.Containers, true, "private", "", groupNames, nil)
		c.Assert(err, IsNil)
		if s.fwdMode == "routing" && encap == "vlan" {
			err = s.CheckBgpRouteDistribution(c, containers)
			c.Assert(err, IsNil)
		}
		time.Sleep(15 * time.Second)
		c.Assert(s.startListeners(containers, []int{8000, 8001}), IsNil)
		time.Sleep(15 * time.Second)
		c.Assert(s.checkConnections(containers, 8000), IsNil)
		time.Sleep(15 * time.Second)
		c.Assert(s.checkNoConnections(containers, 8001), IsNil)

		c.Assert(s.removeContainers(containers), IsNil)

		for _, group := range groups {
			c.Assert(s.cli.EndpointGroupDelete(group.TenantName, group.GroupName), IsNil)
		}

		for _, rule := range rules {
			c.Assert(s.cli.RuleDelete(rule.TenantName, rule.PolicyName, rule.RuleID), IsNil)
		}

		c.Assert(s.cli.PolicyDelete("default", "policy"), IsNil)
	}

	c.Assert(s.cli.NetworkDelete("default", "private"), IsNil)
}

func (s *systemtestSuite) TestPolicyAddDeleteRuleVXLAN(c *C) {
	s.testPolicyAddDeleteRule(c, "vxlan")
}

func (s *systemtestSuite) TestPolicyAddDeleteRuleVLAN(c *C) {
	s.testPolicyAddDeleteRule(c, "vlan")
}

func (s *systemtestSuite) testPolicyAddDeleteRule(c *C, encap string) {

	if encap == "vlan" && s.fwdMode == "routing" {

		s.SetupBgp(c, false)
		s.CheckBgpConnection(c)
	}

	network := &client.Network{
		TenantName:  "default",
		NetworkName: "private",
		Subnet:      "10.1.0.0/16",
		Gateway:     "10.1.1.254",
		PktTag:      1001,
		Encap:       encap,
	}
	c.Assert(s.cli.NetworkPost(network), IsNil)
	c.Assert(s.cli.PolicyPost(&client.Policy{
		PolicyName: "policy",
		TenantName: "default",
	}), IsNil)

	rules := []*client.Rule{
		{
			RuleID:     "1",
			PolicyName: "policy",
			TenantName: "default",
			Direction:  "in",
			Protocol:   "tcp",
			Action:     "deny",
		},
		{
			RuleID:     "2",
			PolicyName: "policy",
			TenantName: "default",
			Priority:   100,
			Direction:  "in",
			Protocol:   "tcp",
			Port:       8000,
			Action:     "allow",
		},
	}

	for _, rule := range rules {
		c.Assert(s.cli.RulePost(rule), IsNil)
	}

	groups := []*client.EndpointGroup{}
	groupNames := []string{}
	for x := 0; x < s.basicInfo.Containers; x++ {
		nodeNum := s.basicInfo.Containers % len(s.nodes)
		epgName := fmt.Sprintf("%s-srv%d-%d", network.NetworkName, nodeNum, x)
		group := &client.EndpointGroup{
			GroupName:   epgName,
			NetworkName: "private",
			Policies:    []string{"policy"},
			TenantName:  "default",
		}
		c.Assert(s.cli.EndpointGroupPost(group), IsNil)

		groups = append(groups, group)
		groupNames = append(groupNames, epgName)
	}

	containers, err := s.runContainers(s.basicInfo.Containers, true, "private", "", groupNames, nil)
	c.Assert(err, IsNil)
	if s.fwdMode == "routing" && encap == "vlan" {
		err = s.CheckBgpRouteDistribution(c, containers)
		c.Assert(err, IsNil)
	}
	time.Sleep(15 * time.Second)
	c.Assert(s.startListeners(containers, []int{8000, 8001}), IsNil)
	time.Sleep(15 * time.Second)
	c.Assert(s.checkConnections(containers, 8000), IsNil)
	time.Sleep(15 * time.Second)
	c.Assert(s.checkNoConnections(containers, 8001), IsNil)

	for i := 0; i < s.basicInfo.Iterations; i++ {
		rule := &client.Rule{
			RuleID:     "3",
			PolicyName: "policy",
			TenantName: "default",
			Direction:  "in",
			Protocol:   "tcp",
			Action:     "allow",
			Priority:   100,
			Port:       8001,
		}

		c.Assert(s.cli.RulePost(rule), IsNil)
		c.Assert(s.checkConnections(containers, 8001), IsNil)

		c.Assert(s.cli.RuleDelete("default", "policy", "3"), IsNil)
		c.Assert(s.checkNoConnections(containers, 8001), IsNil)
	}

	c.Assert(s.removeContainers(containers), IsNil)

	for _, rule := range rules {
		c.Assert(s.cli.RuleDelete(rule.TenantName, rule.PolicyName, rule.RuleID), IsNil)
	}

	for _, group := range groups {
		c.Assert(s.cli.EndpointGroupDelete(group.TenantName, group.GroupName), IsNil)
	}

	c.Assert(s.cli.PolicyDelete("default", "policy"), IsNil)
	c.Assert(s.cli.NetworkDelete("default", "private"), IsNil)
}

func (s *systemtestSuite) TestPolicyFromEPGVXLAN(c *C) {
	s.testPolicyFromEPG(c, "vxlan")
}

func (s *systemtestSuite) TestPolicyFromEPGVLAN(c *C) {
	s.testPolicyFromEPG(c, "vlan")
}

func (s *systemtestSuite) testPolicyFromEPG(c *C, encap string) {

	if encap == "vlan" && s.fwdMode == "routing" {

		s.SetupBgp(c, false)
		s.CheckBgpConnection(c)
	}

	network := &client.Network{
		TenantName:  "default",
		NetworkName: "private",
		Subnet:      "10.1.0.0/16",
		Gateway:     "10.1.1.254",
		PktTag:      1001,
		Encap:       encap,
	}
	c.Assert(s.cli.NetworkPost(network), IsNil)

	group := &client.EndpointGroup{
		GroupName:   "common",
		NetworkName: "private",
		TenantName:  "default",
	}
	c.Assert(s.cli.EndpointGroupPost(group), IsNil)

	for i := 0; i < s.basicInfo.Iterations; i++ {
		policies := []*client.Policy{}
		policyNames := []string{}

		for nodeNum := 0; nodeNum < s.basicInfo.Containers; nodeNum++ {
			policyName := fmt.Sprintf("srv%d-%d", i, nodeNum)
			policy := &client.Policy{
				TenantName: "default",
				PolicyName: policyName,
			}

			c.Assert(s.cli.PolicyPost(policy), IsNil)
			policies = append(policies, policy)

			rules := []*client.Rule{
				{
					RuleID:     "1",
					PolicyName: policyName,
					TenantName: "default",
					Direction:  "in",
					Protocol:   "tcp",
					Action:     "deny",
				},
				{
					RuleID:     "2",
					PolicyName: policyName,
					TenantName: "default",
					Priority:   100,
					Direction:  "in",
					Protocol:   "tcp",
					Port:       8000,
					Action:     "allow",
				},
				{
					RuleID:            "3",
					PolicyName:        policyName,
					TenantName:        "default",
					Priority:          100,
					Direction:         "in",
					Protocol:          "tcp",
					Port:              8001,
					Action:            "allow",
					FromEndpointGroup: "common",
				},
			}

			for _, rule := range rules {
				c.Assert(s.cli.RulePost(rule), IsNil)
			}

			logrus.Infof("Posting EPG for Policy %q", policyName)

			c.Assert(s.cli.EndpointGroupPost(&client.EndpointGroup{
				GroupName:   policyName,
				Policies:    []string{policyName},
				NetworkName: "private",
				TenantName:  "default",
			}), IsNil)

			policyNames = append(policyNames, policyName)
		}

		containers, err := s.runContainers(s.basicInfo.Containers, true, "private", "", policyNames, nil)
		c.Assert(err, IsNil)
		if s.fwdMode == "routing" && encap == "vlan" {
			err = s.CheckBgpRouteDistribution(c, containers)
			c.Assert(err, IsNil)
		}

		commonNames := []string{}
		for _, name := range policyNames {
			commonNames = append(commonNames, fmt.Sprintf("common-%s", name))
		}

		cmnContainers, err := s.runContainersInService(s.basicInfo.Containers, "common", "private", "", commonNames)
		c.Assert(err, IsNil)

		if s.fwdMode == "routing" && encap == "vlan" {
			err = s.CheckBgpRouteDistribution(c, cmnContainers)
			c.Assert(err, IsNil)
		}
		time.Sleep(15 * time.Second)
		c.Assert(s.startListeners(containers, []int{8000, 8001}), IsNil)
		time.Sleep(15 * time.Second)
		c.Assert(s.checkConnections(containers, 8000), IsNil)
		time.Sleep(15 * time.Second)
		c.Assert(s.checkNoConnections(containers, 8001), IsNil)
		time.Sleep(15 * time.Second)
		c.Assert(s.checkConnectionPair(cmnContainers, containers, 8001), IsNil)

		c.Assert(s.removeContainers(containers), IsNil)
		c.Assert(s.removeContainers(cmnContainers), IsNil)

		for _, policy := range policies {
			c.Assert(s.cli.EndpointGroupDelete("default", policy.PolicyName), IsNil)
			c.Assert(s.cli.PolicyDelete("default", policy.PolicyName), IsNil)
		}
	}

	c.Assert(s.cli.EndpointGroupDelete(group.TenantName, group.GroupName), IsNil)
	c.Assert(s.cli.NetworkDelete("default", "private"), IsNil)
}

func (s *systemtestSuite) TestPolicyFeaturesVXLAN(c *C) {
	s.testPolicyFeatures(c, "vxlan")
}

func (s *systemtestSuite) TestPolicyFeaturesVLAN(c *C) {
	s.testPolicyFeatures(c, "vlan")
}

func (s *systemtestSuite) testPolicyFeatures(c *C, encap string) {

	if encap == "vlan" && s.fwdMode == "routing" {

		s.SetupBgp(c, false)
		s.CheckBgpConnection(c)
	}

	network := &client.Network{
		TenantName:  "default",
		NetworkName: "private",
		Subnet:      "10.1.0.0/16",
		Gateway:     "10.1.1.254",
		PktTag:      10,
		Encap:       encap,
	}
	c.Assert(s.cli.NetworkPost(network), IsNil)
	dummyNet := &client.Network{
		TenantName:  "default",
		NetworkName: "dummy",
		Subnet:      "20.1.0.0/16",
		Gateway:     "20.1.1.254",
		PktTag:      20,
		Encap:       encap,
	}
	c.Assert(s.cli.NetworkPost(dummyNet), IsNil)

	pol1 := &client.Policy{
		TenantName: "default",
		PolicyName: "first",
	}

	c.Assert(s.cli.PolicyPost(pol1), IsNil)

	pol2 := &client.Policy{
		TenantName: "default",
		PolicyName: "second",
	}

	c.Assert(s.cli.PolicyPost(pol2), IsNil)

	group1 := &client.EndpointGroup{
		GroupName:   "srv1",
		Policies:    []string{"first"},
		TenantName:  "default",
		NetworkName: "private",
	}

	c.Assert(s.cli.EndpointGroupPost(group1), IsNil)

	group2 := &client.EndpointGroup{
		GroupName:   "srv2",
		Policies:    []string{"second"},
		TenantName:  "default",
		NetworkName: "private",
	}

	c.Assert(s.cli.EndpointGroupPost(group2), IsNil)

	container1, err := s.nodes[0].exec.runContainer(containerSpec{name: "srv1-private", serviceName: "srv1", networkName: "private"})
	c.Assert(err, IsNil)
	container2, err := s.nodes[0].exec.runContainer(containerSpec{name: "srv2-private", serviceName: "srv2", networkName: "private"})
	c.Assert(err, IsNil)
	if s.fwdMode == "routing" && encap == "vlan" {
		err = s.CheckBgpRouteDistribution(c, []*container{container1})
		c.Assert(err, IsNil)
		err = s.CheckBgpRouteDistribution(c, []*container{container2})
		c.Assert(err, IsNil)

	}
	time.Sleep(15 * time.Second)
	c.Assert(container1.node.exec.startListener(container1, 8000, "tcp"), IsNil)
	c.Assert(container1.node.exec.startListener(container1, 8001, "tcp"), IsNil)
	c.Assert(container2.node.exec.startListener(container2, 8000, "tcp"), IsNil)
	c.Assert(container2.node.exec.startListener(container2, 8001, "tcp"), IsNil)

	c.Assert(container2.node.exec.checkConnection(container2, container1.eth0.ip, "tcp", 8000), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName: "first",
		TenantName: "default",
		RuleID:     "1",
		Direction:  "in",
		Protocol:   "tcp",
		Action:     "deny",
	}), IsNil)

	c.Assert(container2.node.exec.checkNoConnection(container2, container1.eth0.ip, "tcp", 8000), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName: "first",
		TenantName: "default",
		RuleID:     "2",
		Priority:   100,
		Direction:  "in",
		Protocol:   "tcp",
		Port:       8000,
		Action:     "allow",
	}), IsNil)

	c.Assert(container2.node.exec.checkConnection(container2, container1.eth0.ip, "tcp", 8000), IsNil)
	c.Assert(container2.node.exec.checkNoConnection(container2, container1.eth0.ip, "tcp", 8001), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName:        "first",
		FromEndpointGroup: "srv2",
		TenantName:        "default",
		RuleID:            "3",
		Priority:          100,
		Direction:         "in",
		Protocol:          "tcp",
		Port:              8001,
		Action:            "allow",
	}), IsNil)
	c.Assert(container2.node.exec.checkConnection(container2, container1.eth0.ip, "tcp", 8001), IsNil)

	c.Assert(s.cli.RuleDelete("default", "first", "3"), IsNil)
	c.Assert(container2.node.exec.checkNoConnection(container2, container1.eth0.ip, "tcp", 8001), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName:  "first",
		FromNetwork: "private",
		TenantName:  "default",
		RuleID:      "3",
		Priority:    10,
		Direction:   "in",
		Protocol:    "tcp",
		Action:      "allow",
	}), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName:  "first",
		FromNetwork: "dummy",
		TenantName:  "default",
		RuleID:      "4",
		Priority:    100,
		Direction:   "in",
		Protocol:    "tcp",
		Action:      "deny",
	}), IsNil)

	c.Assert(container2.node.exec.checkConnection(container2, container1.eth0.ip, "tcp", 8001), IsNil)

	c.Assert(s.cli.RuleDelete("default", "first", "3"), IsNil)
	c.Assert(s.cli.RuleDelete("default", "first", "4"), IsNil)
	c.Assert(container2.node.exec.checkNoConnection(container2, container1.eth0.ip, "tcp", 8001), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName:    "first",
		FromIpAddress: container2.eth0.ip,
		TenantName:    "default",
		RuleID:        "3",
		Priority:      10,
		Direction:     "in",
		Protocol:      "tcp",
		Action:        "allow",
	}), IsNil)

	time.Sleep(2 * time.Second)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName:    "first",
		FromIpAddress: dummyNet.Subnet,
		TenantName:    "default",
		RuleID:        "4",
		Priority:      100,
		Direction:     "in",
		Protocol:      "tcp",
		Action:        "deny",
	}), IsNil)
	time.Sleep(2 * time.Second)

	c.Assert(container2.node.exec.checkConnection(container2, container1.eth0.ip, "tcp", 8001), IsNil)

	for i := 1; i <= 4; i++ {
		c.Assert(s.cli.RuleDelete("default", "first", strconv.Itoa(i)), IsNil)
	}
	time.Sleep(2 * time.Second)

	c.Assert(container2.node.exec.checkConnection(container2, container1.eth0.ip, "tcp", 8000), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName: "first",
		TenantName: "default",
		RuleID:     "1",
		Direction:  "out",
		Protocol:   "tcp",
		Action:     "deny",
	}), IsNil)

	time.Sleep(2 * time.Second)

	c.Assert(container1.node.exec.checkNoConnection(container1, container2.eth0.ip, "tcp", 8000), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName: "first",
		TenantName: "default",
		RuleID:     "2",
		Priority:   100,
		Direction:  "out",
		Protocol:   "tcp",
		Port:       8000,
		Action:     "allow",
	}), IsNil)

	time.Sleep(2 * time.Second)

	c.Assert(container1.node.exec.checkConnection(container1, container2.eth0.ip, "tcp", 8000), IsNil)
	c.Assert(container1.node.exec.checkNoConnection(container1, container2.eth0.ip, "tcp", 8001), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName:      "first",
		TenantName:      "default",
		RuleID:          "3",
		Priority:        100,
		ToEndpointGroup: "srv2",
		Direction:       "out",
		Protocol:        "tcp",
		Port:            8001,
		Action:          "allow",
	}), IsNil)

	time.Sleep(2 * time.Second)

	c.Assert(container1.node.exec.checkConnection(container1, container2.eth0.ip, "tcp", 8001), IsNil)

	c.Assert(s.cli.RuleDelete("default", "first", "3"), IsNil)
	c.Assert(container1.node.exec.checkNoConnection(container1, container2.eth0.ip, "tcp", 8001), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName: "first",
		TenantName: "default",
		RuleID:     "3",
		Priority:   10,
		ToNetwork:  "private",
		Direction:  "out",
		Protocol:   "tcp",
		Action:     "allow",
	}), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName: "first",
		TenantName: "default",
		RuleID:     "4",
		Priority:   100,
		ToNetwork:  "dummy",
		Direction:  "out",
		Protocol:   "tcp",
		Action:     "deny",
	}), IsNil)

	time.Sleep(2 * time.Second)

	c.Assert(container1.node.exec.checkConnection(container1, container2.eth0.ip, "tcp", 8001), IsNil)

	c.Assert(s.cli.RuleDelete("default", "first", "3"), IsNil)
	c.Assert(s.cli.RuleDelete("default", "first", "4"), IsNil)

	c.Assert(container1.node.exec.checkNoConnection(container1, container2.eth0.ip, "tcp", 8001), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName:  "first",
		TenantName:  "default",
		RuleID:      "3",
		Priority:    10,
		ToIpAddress: container2.eth0.ip,
		Direction:   "out",
		Protocol:    "tcp",
		Action:      "allow",
	}), IsNil)

	time.Sleep(2 * time.Second)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName:  "first",
		TenantName:  "default",
		RuleID:      "4",
		Priority:    100,
		ToIpAddress: dummyNet.Subnet,
		Direction:   "out",
		Protocol:    "tcp",
		Action:      "deny",
	}), IsNil)

	time.Sleep(2 * time.Second)
	c.Assert(container1.node.exec.checkConnection(container1, container2.eth0.ip, "tcp", 8001), IsNil)

	for i := 1; i <= 4; i++ {
		c.Assert(s.cli.RuleDelete("default", "first", strconv.Itoa(i)), IsNil)
	}

	time.Sleep(2 * time.Second)

	c.Assert(container1.node.exec.checkPing(container1, container2.eth0.ip), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName: "first",
		TenantName: "default",
		RuleID:     "1",
		Direction:  "in",
		Protocol:   "icmp",
		Action:     "deny",
	}), IsNil)

	time.Sleep(2 * time.Second)

	c.Assert(container1.node.exec.checkPingFailure(container1, container2.eth0.ip), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName:    "first",
		TenantName:    "default",
		RuleID:        "2",
		Priority:      100,
		FromIpAddress: container2.eth0.ip,
		Direction:     "in",
		Protocol:      "icmp",
		Action:        "allow",
	}), IsNil)

	time.Sleep(2 * time.Second)

	c.Assert(container1.node.exec.checkPing(container1, container2.eth0.ip), IsNil)

	c.Assert(s.cli.RuleDelete("default", "first", "2"), IsNil)

	time.Sleep(2 * time.Second)

	c.Assert(container1.node.exec.checkPingFailure(container1, container2.eth0.ip), IsNil)

	c.Assert(s.cli.RuleDelete("default", "first", "1"), IsNil)
	time.Sleep(2 * time.Second)

	c.Assert(container1.node.exec.checkPing(container1, container2.eth0.ip), IsNil)

	c.Assert(s.removeContainers([]*container{container1, container2}), IsNil)
	c.Assert(s.cli.EndpointGroupDelete("default", "srv1"), IsNil)
	c.Assert(s.cli.EndpointGroupDelete("default", "srv2"), IsNil)

	c.Assert(s.cli.PolicyDelete("default", "first"), IsNil)
	c.Assert(s.cli.PolicyDelete("default", "second"), IsNil)

	c.Assert(s.cli.NetworkDelete("default", "dummy"), IsNil)
	c.Assert(s.cli.NetworkDelete("default", "private"), IsNil)
}
