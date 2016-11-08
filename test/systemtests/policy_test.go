package systemtests

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/contivmodel/client"
	. "gopkg.in/check.v1"
)

func (s *systemtestSuite) TestPolicyBasic(c *C) {
	doneChan := make(chan struct{}, 2)

	go s.testPolicyBasic(c, EncapVXLAN, doneChan, 0)
	go s.testPolicyBasic(c, EncapVLAN, doneChan, 1)

	for i := 0; i < 2; i++ {
		<-doneChan
	}
}

func (s *systemtestSuite) testPolicyBasic(c *C, encap string, doneChan chan struct{}, seq int) {
	defer func() { doneChan <- struct{}{} }()
	s.SetupBgp(c, encap, false)
	s.CheckBgpConnection(c, encap)

	tenantName := fmt.Sprintf("tenant-%d", seq)

	tenant := &client.Tenant{TenantName: tenantName}
	c.Assert(s.cli.TenantPost(tenant), IsNil)

	network := &client.Network{
		TenantName:  tenantName,
		NetworkName: encap,
		Subnet:      fmt.Sprintf("10.1.%d.0/24", seq),
		Gateway:     fmt.Sprintf("10.1.%d.254", seq),
		PktTag:      1000 + seq,
		Encap:       encap,
	}

	c.Assert(s.cli.NetworkPost(network), IsNil)

	for i := 0; i < s.basicInfo.Iterations; i++ {
		policyName := fmt.Sprintf("policy-%d", seq)
		c.Assert(s.cli.PolicyPost(&client.Policy{
			PolicyName: policyName,
			TenantName: tenantName,
		}), IsNil)

		rules := []*client.Rule{
			{
				RuleID:     "1",
				PolicyName: policyName,
				TenantName: tenantName,
				Direction:  "in",
				Protocol:   "tcp",
				Action:     "deny",
			},
			{
				RuleID:     "2",
				PolicyName: policyName,
				TenantName: tenantName,
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
				NetworkName: encap,
				Policies:    []string{policyName},
				TenantName:  tenantName,
			}
			c.Assert(s.cli.EndpointGroupPost(group), IsNil)

			groups = append(groups, group)
			groupNames = append(groupNames, epgName)
		}

		containers, err := s.runContainers(s.basicInfo.Containers, true, encap, tenantName, groupNames, nil)
		c.Assert(err, IsNil)
		if s.fwdMode == FwdModeRouting && encap == EncapVLAN {
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

		c.Assert(s.cli.PolicyDelete(tenantName, policyName), IsNil)
	}

	c.Assert(s.cli.NetworkDelete(tenantName, encap), IsNil)
	c.Assert(s.cli.TenantDelete(tenantName), IsNil)
}

func (s *systemtestSuite) TestPolicyAddDeleteRule(c *C) {
	doneChan := make(chan struct{}, 2)

	go s.testPolicyAddDeleteRule(c, EncapVXLAN, doneChan, 0)
	go s.testPolicyAddDeleteRule(c, EncapVLAN, doneChan, 1)

	for i := 0; i < 2; i++ {
		<-doneChan
	}
}

func (s *systemtestSuite) testPolicyAddDeleteRule(c *C, encap string, doneChan chan struct{}, seq int) {
	defer func() { doneChan <- struct{}{} }()

	s.SetupBgp(c, encap, false)
	s.CheckBgpConnection(c, encap)

	tenantName := fmt.Sprintf("tenant-%d", seq)

	tenant := &client.Tenant{TenantName: tenantName}
	c.Assert(s.cli.TenantPost(tenant), IsNil)

	network := &client.Network{
		TenantName:  tenantName,
		NetworkName: encap,
		Subnet:      fmt.Sprintf("10.1.%d.0/24", seq),
		Gateway:     fmt.Sprintf("10.1.%d.254", seq),
		PktTag:      1000 + seq,
		Encap:       encap,
	}

	policyName := fmt.Sprintf("policy-%d", seq)

	c.Assert(s.cli.NetworkPost(network), IsNil)
	c.Assert(s.cli.PolicyPost(&client.Policy{
		PolicyName: policyName,
		TenantName: tenantName,
	}), IsNil)

	rules := []*client.Rule{
		{
			RuleID:     "1",
			PolicyName: policyName,
			TenantName: tenantName,
			Direction:  "in",
			Protocol:   "tcp",
			Action:     "deny",
		},
		{
			RuleID:     "2",
			PolicyName: policyName,
			TenantName: tenantName,
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
			NetworkName: encap,
			Policies:    []string{policyName},
			TenantName:  tenantName,
		}
		c.Assert(s.cli.EndpointGroupPost(group), IsNil)

		groups = append(groups, group)
		groupNames = append(groupNames, epgName)
	}

	containers, err := s.runContainers(s.basicInfo.Containers, true, encap, tenantName, groupNames, nil)
	c.Assert(err, IsNil)
	if s.fwdMode == FwdModeRouting && encap == EncapVLAN {
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
			PolicyName: policyName,
			TenantName: tenantName,
			Direction:  "in",
			Protocol:   "tcp",
			Action:     "allow",
			Priority:   100,
			Port:       8001,
		}

		c.Assert(s.cli.RulePost(rule), IsNil)
		c.Assert(s.checkConnections(containers, 8001), IsNil)

		c.Assert(s.cli.RuleDelete(tenantName, policyName, "3"), IsNil)
		c.Assert(s.checkNoConnections(containers, 8001), IsNil)
	}

	c.Assert(s.removeContainers(containers), IsNil)

	for _, rule := range rules {
		c.Assert(s.cli.RuleDelete(rule.TenantName, rule.PolicyName, rule.RuleID), IsNil)
	}

	for _, group := range groups {
		c.Assert(s.cli.EndpointGroupDelete(group.TenantName, group.GroupName), IsNil)
	}

	c.Assert(s.cli.PolicyDelete(tenantName, policyName), IsNil)
	c.Assert(s.cli.NetworkDelete(tenantName, encap), IsNil)
	c.Assert(s.cli.TenantDelete(tenantName), IsNil)
}

func (s *systemtestSuite) TestPolicyFromEPG(c *C) {
	doneChan := make(chan struct{}, 2)

	go s.testPolicyFromEPG(c, EncapVXLAN, doneChan, 0)
	go s.testPolicyFromEPG(c, EncapVLAN, doneChan, 1)

	for i := 0; i < 2; i++ {
		<-doneChan
	}
}

func (s *systemtestSuite) testPolicyFromEPG(c *C, encap string, doneChan chan struct{}, seq int) {
	defer func() { doneChan <- struct{}{} }()

	s.SetupBgp(c, encap, false)
	s.CheckBgpConnection(c, encap)

	tenantName := fmt.Sprintf("tenant-%d", seq)

	tenant := &client.Tenant{TenantName: tenantName}
	c.Assert(s.cli.TenantPost(tenant), IsNil)

	network := &client.Network{
		TenantName:  tenantName,
		NetworkName: encap,
		Subnet:      fmt.Sprintf("10.1.%d.0/24", seq),
		Gateway:     fmt.Sprintf("10.1.%d.254", seq),
		PktTag:      1000 + seq,
		Encap:       encap,
	}
	c.Assert(s.cli.NetworkPost(network), IsNil)

	epgName := fmt.Sprintf("common-%d", seq)

	group := &client.EndpointGroup{
		GroupName:   epgName,
		NetworkName: encap,
		TenantName:  tenantName,
	}
	c.Assert(s.cli.EndpointGroupPost(group), IsNil)

	syncChan := make(chan struct{}, s.basicInfo.Iterations)

	for i := 0; i < s.basicInfo.Iterations; i++ {
		go func(i int) {
			defer func() { syncChan <- struct{}{} }()
			policies := []*client.Policy{}
			policyNames := []string{}

			for nodeNum := 0; nodeNum < s.basicInfo.Containers; nodeNum++ {
				policyName := fmt.Sprintf("srv%d-%d-%d", i, nodeNum, seq)
				policy := &client.Policy{
					TenantName: tenantName,
					PolicyName: policyName,
				}

				c.Assert(s.cli.PolicyPost(policy), IsNil)
				policies = append(policies, policy)

				rules := []*client.Rule{
					{
						RuleID:     "1",
						PolicyName: policyName,
						TenantName: tenantName,
						Direction:  "in",
						Protocol:   "tcp",
						Action:     "deny",
					},
					{
						RuleID:     "2",
						PolicyName: policyName,
						TenantName: tenantName,
						Priority:   100,
						Direction:  "in",
						Protocol:   "tcp",
						Port:       8000,
						Action:     "allow",
					},
					{
						RuleID:            "3",
						PolicyName:        policyName,
						TenantName:        tenantName,
						Priority:          100,
						Direction:         "in",
						Protocol:          "tcp",
						Port:              8001,
						Action:            "allow",
						FromEndpointGroup: epgName,
					},
				}

				for _, rule := range rules {
					c.Assert(s.cli.RulePost(rule), IsNil)
				}

				logrus.Infof("Posting EPG for Policy %q", policyName)

				c.Assert(s.cli.EndpointGroupPost(&client.EndpointGroup{
					GroupName:   policyName,
					Policies:    []string{policyName},
					NetworkName: encap,
					TenantName:  tenantName,
				}), IsNil)

				policyNames = append(policyNames, policyName)
			}

			containers, err := s.runContainers(s.basicInfo.Containers, true, encap, tenantName, policyNames, nil)
			c.Assert(err, IsNil)
			if s.fwdMode == FwdModeRouting && encap == EncapVLAN {
				err = s.CheckBgpRouteDistribution(c, containers)
				c.Assert(err, IsNil)
			}

			commonNames := []string{}
			for _, name := range policyNames {
				commonNames = append(commonNames, fmt.Sprintf("common-%s-%d", name, i))
			}

			cmnContainers, err := s.runContainersInService(s.basicInfo.Containers, epgName, encap, tenantName, commonNames)
			c.Assert(err, IsNil)

			if s.fwdMode == FwdModeRouting && encap == EncapVLAN {
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
				c.Assert(s.cli.EndpointGroupDelete(tenantName, policy.PolicyName), IsNil)
				c.Assert(s.cli.PolicyDelete(tenantName, policy.PolicyName), IsNil)
			}
		}(i)
	}

	for i := 0; i < s.basicInfo.Iterations; i++ {
		<-syncChan
	}

	c.Assert(s.cli.EndpointGroupDelete(tenantName, epgName), IsNil)
	c.Assert(s.cli.NetworkDelete(tenantName, encap), IsNil)
	c.Assert(s.cli.TenantDelete(tenantName), IsNil)
}

func (s *systemtestSuite) TestPolicyFeatures(c *C) {
	doneChan := make(chan struct{}, 2)
	go s.testPolicyFeatures(c, EncapVXLAN, doneChan, 0)
	go s.testPolicyFeatures(c, EncapVLAN, doneChan, 1)

	for i := 0; i < 2; i++ {
		<-doneChan
	}
}

func (s *systemtestSuite) testPolicyFeatures(c *C, encap string, doneChan chan struct{}, seq int) {
	defer func() { doneChan <- struct{}{} }()

	tenantName := fmt.Sprintf("tenant-%d", seq)
	tenant := &client.Tenant{TenantName: tenantName}
	c.Assert(s.cli.TenantPost(tenant), IsNil)

	s.SetupBgp(c, encap, false)
	s.CheckBgpConnection(c, encap)

	dummyName := fmt.Sprintf("dummy-%d", seq)

	network := &client.Network{
		TenantName:  tenantName,
		NetworkName: encap,
		Subnet:      fmt.Sprintf("10.1.%d.0/24", seq),
		Gateway:     fmt.Sprintf("10.1.%d.254", seq),
		PktTag:      10 + seq,
		Encap:       encap,
	}
	c.Assert(s.cli.NetworkPost(network), IsNil)
	dummyNet := &client.Network{
		TenantName:  tenantName,
		NetworkName: dummyName,
		Subnet:      fmt.Sprintf("20.1.%d.0/24", seq),
		Gateway:     fmt.Sprintf("20.1.%d.254", seq),
		PktTag:      20 + seq,
		Encap:       encap,
	}
	c.Assert(s.cli.NetworkPost(dummyNet), IsNil)

	firstPolicyName := fmt.Sprintf("first-%d", seq)
	secondPolicyName := fmt.Sprintf("second-%d", seq)

	pol1 := &client.Policy{
		TenantName: tenantName,
		PolicyName: firstPolicyName,
	}

	c.Assert(s.cli.PolicyPost(pol1), IsNil)

	pol2 := &client.Policy{
		TenantName: tenantName,
		PolicyName: secondPolicyName,
	}

	c.Assert(s.cli.PolicyPost(pol2), IsNil)

	epg1GroupName := fmt.Sprintf("srv1-%d", seq)
	epg2GroupName := fmt.Sprintf("srv2-%d", seq)

	group1 := &client.EndpointGroup{
		GroupName:   epg1GroupName,
		Policies:    []string{firstPolicyName},
		TenantName:  tenantName,
		NetworkName: encap,
	}

	c.Assert(s.cli.EndpointGroupPost(group1), IsNil)

	group2 := &client.EndpointGroup{
		GroupName:   epg2GroupName,
		Policies:    []string{secondPolicyName},
		TenantName:  tenantName,
		NetworkName: encap,
	}

	c.Assert(s.cli.EndpointGroupPost(group2), IsNil)

	container1, err := s.nodes[0].exec.runContainer(containerSpec{name: fmt.Sprintf("srv1-%d-private", seq), serviceName: fmt.Sprintf("%s/%s", epg1GroupName, tenantName), networkName: encap})
	c.Assert(err, IsNil)
	container2, err := s.nodes[0].exec.runContainer(containerSpec{name: fmt.Sprintf("srv2-%d-private", seq), serviceName: fmt.Sprintf("%s/%s", epg2GroupName, tenantName), networkName: encap})
	c.Assert(err, IsNil)
	if s.fwdMode == FwdModeRouting && encap == EncapVLAN {
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
		PolicyName: firstPolicyName,
		TenantName: tenantName,
		RuleID:     "1",
		Direction:  "in",
		Protocol:   "tcp",
		Action:     "deny",
	}), IsNil)

	c.Assert(container2.node.exec.checkNoConnection(container2, container1.eth0.ip, "tcp", 8000), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName: firstPolicyName,
		TenantName: tenantName,
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
		PolicyName:        firstPolicyName,
		FromEndpointGroup: epg2GroupName,
		TenantName:        tenantName,
		RuleID:            "3",
		Priority:          100,
		Direction:         "in",
		Protocol:          "tcp",
		Port:              8001,
		Action:            "allow",
	}), IsNil)
	c.Assert(container2.node.exec.checkConnection(container2, container1.eth0.ip, "tcp", 8001), IsNil)

	c.Assert(s.cli.RuleDelete(tenantName, firstPolicyName, "3"), IsNil)
	c.Assert(container2.node.exec.checkNoConnection(container2, container1.eth0.ip, "tcp", 8001), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName:  firstPolicyName,
		FromNetwork: encap,
		TenantName:  tenantName,
		RuleID:      "3",
		Priority:    10,
		Direction:   "in",
		Protocol:    "tcp",
		Action:      "allow",
	}), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName:  firstPolicyName,
		FromNetwork: dummyName,
		TenantName:  tenantName,
		RuleID:      "4",
		Priority:    100,
		Direction:   "in",
		Protocol:    "tcp",
		Action:      "deny",
	}), IsNil)

	c.Assert(container2.node.exec.checkConnection(container2, container1.eth0.ip, "tcp", 8001), IsNil)

	c.Assert(s.cli.RuleDelete(tenantName, firstPolicyName, "3"), IsNil)
	c.Assert(s.cli.RuleDelete(tenantName, firstPolicyName, "4"), IsNil)
	c.Assert(container2.node.exec.checkNoConnection(container2, container1.eth0.ip, "tcp", 8001), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName:    firstPolicyName,
		FromIpAddress: container2.eth0.ip,
		TenantName:    tenantName,
		RuleID:        "3",
		Priority:      10,
		Direction:     "in",
		Protocol:      "tcp",
		Action:        "allow",
	}), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName:    firstPolicyName,
		FromIpAddress: dummyNet.Subnet,
		TenantName:    tenantName,
		RuleID:        "4",
		Priority:      100,
		Direction:     "in",
		Protocol:      "tcp",
		Action:        "deny",
	}), IsNil)

	c.Assert(container2.node.exec.checkConnection(container2, container1.eth0.ip, "tcp", 8001), IsNil)

	for i := 1; i <= 4; i++ {
		c.Assert(s.cli.RuleDelete(tenantName, firstPolicyName, strconv.Itoa(i)), IsNil)
	}

	c.Assert(container2.node.exec.checkConnection(container2, container1.eth0.ip, "tcp", 8000), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName: firstPolicyName,
		TenantName: tenantName,
		RuleID:     "1",
		Direction:  "out",
		Protocol:   "tcp",
		Action:     "deny",
	}), IsNil)

	c.Assert(container1.node.exec.checkNoConnection(container1, container2.eth0.ip, "tcp", 8000), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName: firstPolicyName,
		TenantName: tenantName,
		RuleID:     "2",
		Priority:   100,
		Direction:  "out",
		Protocol:   "tcp",
		Port:       8000,
		Action:     "allow",
	}), IsNil)

	c.Assert(container1.node.exec.checkConnection(container1, container2.eth0.ip, "tcp", 8000), IsNil)
	c.Assert(container1.node.exec.checkNoConnection(container1, container2.eth0.ip, "tcp", 8001), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName:      firstPolicyName,
		TenantName:      tenantName,
		RuleID:          "3",
		Priority:        100,
		ToEndpointGroup: epg2GroupName,
		Direction:       "out",
		Protocol:        "tcp",
		Port:            8001,
		Action:          "allow",
	}), IsNil)

	c.Assert(container1.node.exec.checkConnection(container1, container2.eth0.ip, "tcp", 8001), IsNil)

	c.Assert(s.cli.RuleDelete(tenantName, firstPolicyName, "3"), IsNil)
	c.Assert(container1.node.exec.checkNoConnection(container1, container2.eth0.ip, "tcp", 8001), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName: firstPolicyName,
		TenantName: tenantName,
		RuleID:     "3",
		Priority:   10,
		ToNetwork:  encap,
		Direction:  "out",
		Protocol:   "tcp",
		Action:     "allow",
	}), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName: firstPolicyName,
		TenantName: tenantName,
		RuleID:     "4",
		Priority:   100,
		ToNetwork:  dummyName,
		Direction:  "out",
		Protocol:   "tcp",
		Action:     "deny",
	}), IsNil)

	c.Assert(container1.node.exec.checkConnection(container1, container2.eth0.ip, "tcp", 8001), IsNil)

	c.Assert(s.cli.RuleDelete(tenantName, firstPolicyName, "3"), IsNil)
	c.Assert(s.cli.RuleDelete(tenantName, firstPolicyName, "4"), IsNil)

	c.Assert(container1.node.exec.checkNoConnection(container1, container2.eth0.ip, "tcp", 8001), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName:  firstPolicyName,
		TenantName:  tenantName,
		RuleID:      "3",
		Priority:    10,
		ToIpAddress: container2.eth0.ip,
		Direction:   "out",
		Protocol:    "tcp",
		Action:      "allow",
	}), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName:  firstPolicyName,
		TenantName:  tenantName,
		RuleID:      "4",
		Priority:    100,
		ToIpAddress: dummyNet.Subnet,
		Direction:   "out",
		Protocol:    "tcp",
		Action:      "deny",
	}), IsNil)

	c.Assert(container1.node.exec.checkConnection(container1, container2.eth0.ip, "tcp", 8001), IsNil)

	for i := 1; i <= 4; i++ {
		c.Assert(s.cli.RuleDelete(tenantName, firstPolicyName, strconv.Itoa(i)), IsNil)
	}

	c.Assert(container1.node.exec.checkPing(container1, container2.eth0.ip), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName: firstPolicyName,
		TenantName: tenantName,
		RuleID:     "1",
		Direction:  "in",
		Protocol:   "icmp",
		Action:     "deny",
	}), IsNil)

	c.Assert(container1.node.exec.checkPingFailure(container1, container2.eth0.ip), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		PolicyName:    firstPolicyName,
		TenantName:    tenantName,
		RuleID:        "2",
		Priority:      100,
		FromIpAddress: container2.eth0.ip,
		Direction:     "in",
		Protocol:      "icmp",
		Action:        "allow",
	}), IsNil)

	c.Assert(container1.node.exec.checkPing(container1, container2.eth0.ip), IsNil)

	c.Assert(s.cli.RuleDelete(tenantName, firstPolicyName, "2"), IsNil)
	c.Assert(container1.node.exec.checkPingFailure(container1, container2.eth0.ip), IsNil)

	c.Assert(s.cli.RuleDelete(tenantName, firstPolicyName, "1"), IsNil)
	c.Assert(container1.node.exec.checkPing(container1, container2.eth0.ip), IsNil)

	c.Assert(s.removeContainers([]*container{container1, container2}), IsNil)
	c.Assert(s.cli.EndpointGroupDelete(tenantName, epg1GroupName), IsNil)
	c.Assert(s.cli.EndpointGroupDelete(tenantName, epg2GroupName), IsNil)

	c.Assert(s.cli.PolicyDelete(tenantName, firstPolicyName), IsNil)
	c.Assert(s.cli.PolicyDelete(tenantName, secondPolicyName), IsNil)

	c.Assert(s.cli.NetworkDelete(tenantName, dummyName), IsNil)
	c.Assert(s.cli.NetworkDelete(tenantName, encap), IsNil)
}
