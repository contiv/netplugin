package systemtests

import (
	"errors"
	"time"

	log "github.com/Sirupsen/logrus"
	. "github.com/contiv/check"
	"github.com/contiv/netplugin/contivmodel/client"
)

/* TestACIMode does the following:
1). Checks ping success between containers of same EPGs.
2). Checks ping failure between containers from different EPGs.
3). Checks this behavior once netplugin is restarted.
*/
func (s *systemtestSuite) TestACIMode(c *C) {
	if s.fwdMode == "routing" {
		c.Skip("Skipping test for routing mode")
	}
	c.Assert(s.cli.GlobalPost(&client.Global{
		Name:             "global",
		NetworkInfraType: "aci",
		Vlans:            s.globInfo.Vlan,
		Vxlans:           s.globInfo.Vxlan,
		FwdMode:          "bridge",
		ArpMode:          "flood",
		PvtSubnet:        "172.19.0.0/16",
	}), IsNil)
	c.Assert(s.cli.NetworkPost(&client.Network{
		TenantName:  "default",
		NetworkName: s.globInfo.Network,
		Subnet:      s.globInfo.Subnet,
		Gateway:     s.globInfo.Gateway,
		Encap:       s.globInfo.Encap,
	}), IsNil)

	err := s.nodes[0].checkSchedulerNetworkCreated(s.globInfo.Network, false)
	c.Assert(err, IsNil)

	c.Assert(s.cli.EndpointGroupPost(&client.EndpointGroup{
		TenantName:  "default",
		NetworkName: s.globInfo.Network,
		GroupName:   "epga",
	}), IsNil)

	err = s.nodes[0].exec.checkSchedulerNetworkCreated("epga", true)
	c.Assert(err, IsNil)

	c.Assert(s.cli.EndpointGroupPost(&client.EndpointGroup{
		TenantName:  "default",
		NetworkName: s.globInfo.Network,
		GroupName:   "epgb",
	}), IsNil)

	err = s.nodes[0].checkSchedulerNetworkCreated("epgb", true)
	c.Assert(err, IsNil)

	containersA, err := s.runContainersOnNode(s.basicInfo.Containers, s.globInfo.Network, "", "epga", s.nodes[0])
	c.Assert(err, IsNil)
	containersB, err := s.runContainersOnNode(s.basicInfo.Containers, s.globInfo.Network, "", "epgb", s.nodes[0])
	c.Assert(err, IsNil)

	// Verify containers within epga can ping each other
	c.Assert(s.pingTest(containersA), IsNil)
	// Verify containers within epgb can ping each other
	c.Assert(s.pingTest(containersB), IsNil)
	// Verify containers from epga can't ping epgb containers
	c.Assert(s.pingFailureTest(containersA, containersB), IsNil)

	log.Infof("Triggering netplugin restart")
	node1 := s.nodes[0]
	c.Assert(node1.stopNetplugin(), IsNil)
	c.Assert(node1.rotateLog("netplugin"), IsNil)
	c.Assert(node1.startNetplugin(""), IsNil)
	c.Assert(node1.runCommandUntilNoError("pgrep netplugin"), IsNil)
	time.Sleep(20 * time.Second)

	c.Assert(s.pingTest(containersA), IsNil)
	c.Assert(s.pingTest(containersB), IsNil)
	c.Assert(s.pingFailureTest(containersA, containersB), IsNil)

	c.Assert(s.removeContainers(containersA), IsNil)
	c.Assert(s.removeContainers(containersB), IsNil)
	c.Assert(s.cli.EndpointGroupDelete("default", "epga"), IsNil)
	c.Assert(s.cli.EndpointGroupDelete("default", "epgb"), IsNil)
	c.Assert(s.cli.NetworkDelete("default", s.globInfo.Network), IsNil)
}

/* TestACIDefaultGroup does the following:
Logic is similar to TestACIMode but with containers using "default-group"
Containers get assigned to "default-group" either implicitly (when they have no group label)
or explicitly (when they have a group label set to "default-group")
The test covers both types of containers
*/

func (s *systemtestSuite) TestACIDefaultGroup(c *C) {
	if s.fwdMode == "routing" {
		c.Skip("Skipping test for routing mode")
	}
	c.Assert(s.cli.GlobalPost(&client.Global{
		Name:             "global",
		NetworkInfraType: "aci",
		Vlans:            s.globInfo.Vlan,
		Vxlans:           s.globInfo.Vxlan,
		FwdMode:          "bridge",
		ArpMode:          "flood",
		PvtSubnet:        "172.19.0.0/16",
	}), IsNil)
	c.Assert(s.cli.NetworkPost(&client.Network{
		TenantName:  "default",
		NetworkName: "default-net-aci",
		Subnet:      s.globInfo.Subnet,
		Gateway:     s.globInfo.Gateway,
		Encap:       s.globInfo.Encap,
	}), IsNil)

	err := s.nodes[0].checkSchedulerNetworkCreated("default-net-aci", true)
	c.Assert(err, IsNil)

	c.Assert(s.cli.EndpointGroupPost(&client.EndpointGroup{
		TenantName:  "default",
		NetworkName: "default-net-aci",
		GroupName:   "default-group",
	}), IsNil)

	err = s.nodes[0].exec.checkSchedulerNetworkCreated("default-group", true)
	c.Assert(err, IsNil)

	c.Assert(s.cli.EndpointGroupPost(&client.EndpointGroup{
		TenantName:  "default",
		NetworkName: "default-net-aci",
		GroupName:   "epgb",
	}), IsNil)

	err = s.nodes[0].checkSchedulerNetworkCreated("epgb", true)
	c.Assert(err, IsNil)

	// Containers created without any explicit group label
	containersA, err := s.runContainersOnNode(s.basicInfo.Containers, "default-net-aci", "", "", s.nodes[0])
	c.Assert(err, IsNil)
	// Containers created with explicit group label "epgb"
	containersB, err := s.runContainersOnNode(s.basicInfo.Containers, "default-net-aci", "", "epgb", s.nodes[0])
	c.Assert(err, IsNil)
	// Containers created with explicit group label "default-group"
	containersC, err := s.runContainersOnNode(s.basicInfo.Containers, "default-net-aci", "", "default-group", s.nodes[0])
	c.Assert(err, IsNil)

	// Combine containersA and containersC since they are both effectively in the same default-group
	containersDefault := append(containersA, containersC...)

	// Verify containers within the combined default-group can ping each other
	c.Assert(s.pingTest(containersDefault), IsNil)
	// Verify containers within epgb can ping each other
	c.Assert(s.pingTest(containersB), IsNil)
	// Verify containers within the combined default-group can't ping epgb containers
	c.Assert(s.pingFailureTest(containersDefault, containersB), IsNil)

	c.Assert(s.removeContainers(containersDefault), IsNil)
	c.Assert(s.removeContainers(containersB), IsNil)
	c.Assert(s.cli.EndpointGroupDelete("default", "default-group"), IsNil)
	c.Assert(s.cli.EndpointGroupDelete("default", "epgb"), IsNil)
	c.Assert(s.cli.NetworkDelete("default", "default-net-aci"), IsNil)
}

/* TesACIPingGateway checks ping success from containers running in a EPG to the default gateway */
func (s *systemtestSuite) TestACIPingGateway(c *C) {
	if s.fwdMode == "routing" {
		c.Skip("Skipping test for routing mode")
	}
	c.Assert(s.cli.GlobalPost(&client.Global{
		Name:             "global",
		NetworkInfraType: "aci",
		FwdMode:          "bridge",
		ArpMode:          "flood",
		Vlans:            s.globInfo.Vlan,
		Vxlans:           s.globInfo.Vxlan,
		PvtSubnet:        "172.19.0.0/16",
	}), IsNil)
	c.Assert(s.cli.TenantPost(&client.Tenant{
		TenantName: s.globInfo.Tenant,
	}), IsNil)

	containersA := []*container{}

	c.Assert(s.cli.NetworkPost(&client.Network{
		TenantName:  s.globInfo.Tenant,
		NetworkName: s.globInfo.Network,
		Subnet:      s.globInfo.Subnet,
		Gateway:     s.globInfo.Gateway,
		Encap:       s.globInfo.Encap,
	}), IsNil)

	err := s.nodes[0].checkSchedulerNetworkCreated(s.globInfo.Network, false)
	c.Assert(err, IsNil)

	c.Assert(s.cli.EndpointGroupPost(&client.EndpointGroup{
		TenantName:  s.globInfo.Tenant,
		NetworkName: s.globInfo.Network,
		GroupName:   "epga",
	}), IsNil)

	err = s.nodes[0].exec.checkSchedulerNetworkCreated("epga", true)
	c.Assert(err, IsNil)

	c.Assert(s.cli.AppProfilePost(&client.AppProfile{
		TenantName:     s.globInfo.Tenant,
		EndpointGroups: []string{"epga"},
		AppProfileName: "profile1",
	}), IsNil)
	time.Sleep(5 * time.Second)
	containers, err := s.runContainersInGroups(s.basicInfo.Containers, s.globInfo.Network, s.globInfo.Tenant, []string{"epga"})
	c.Assert(err, IsNil)

	for key := range containers {
		containersA = append(containersA, key)
	}

	// Verify containers in epga can ping default gateway
	c.Assert(s.pingTestByName(containersA, s.globInfo.Gateway), IsNil)

	c.Assert(s.removeContainers(containersA), IsNil)
	containersA = nil
	c.Assert(s.cli.AppProfileDelete(s.globInfo.Tenant, "profile1"), IsNil)
	time.Sleep(time.Second * 5)
	c.Assert(s.cli.EndpointGroupDelete(s.globInfo.Tenant, "epga"), IsNil)
	c.Assert(s.cli.NetworkDelete(s.globInfo.Tenant, s.globInfo.Network), IsNil)
}

/* TestACIProfile does the following:
1). TestACIMode and TestACIPingGateway for containers in two EPGs.
2). Checks policies and rules learned on the APIC controller.
3). Checks policies behavior before and after deleting app-profiles.
*/
func (s *systemtestSuite) TestACIProfile(c *C) {
	if s.fwdMode == "routing" {
		c.Skip("Skipping test for routing mode")
	}
	c.Assert(s.cli.GlobalPost(&client.Global{
		Name:             "global",
		NetworkInfraType: "aci",
		FwdMode:          "bridge",
		ArpMode:          "flood",
		Vlans:            s.globInfo.Vlan,
		Vxlans:           s.globInfo.Vxlan,
		PvtSubnet:        "172.19.0.0/16",
	}), IsNil)
	c.Assert(s.cli.TenantPost(&client.Tenant{
		TenantName: s.globInfo.Tenant,
	}), IsNil)

	containersA := []*container{}
	containersB := []*container{}

	containersA2 := []*container{}
	containersB2 := []*container{}

	c.Assert(s.cli.NetworkPost(&client.Network{
		TenantName:  s.globInfo.Tenant,
		NetworkName: s.globInfo.Network,
		Subnet:      s.globInfo.Subnet,
		Gateway:     s.globInfo.Gateway,
		Encap:       s.globInfo.Encap,
	}), IsNil)

	err := s.nodes[0].checkSchedulerNetworkCreated(s.globInfo.Network, false)
	c.Assert(err, IsNil)

	c.Assert(s.cli.EndpointGroupPost(&client.EndpointGroup{
		TenantName:  s.globInfo.Tenant,
		NetworkName: s.globInfo.Network,
		GroupName:   "epga",
	}), IsNil)

	c.Assert(s.cli.EndpointGroupPost(&client.EndpointGroup{
		TenantName:  s.globInfo.Tenant,
		NetworkName: s.globInfo.Network,
		GroupName:   "epgb",
	}), IsNil)

	c.Assert(s.cli.AppProfilePost(&client.AppProfile{
		TenantName:     s.globInfo.Tenant,
		EndpointGroups: []string{"epga", "epgb"},
		AppProfileName: "profile1",
	}), IsNil)

	time.Sleep(5 * time.Second)

	groups := []string{"epga", "epgb"}
	containers, err := s.runContainersInGroups(s.basicInfo.Containers, s.globInfo.Network, s.globInfo.Tenant, groups)
	c.Assert(err, IsNil)
	time.Sleep(time.Second * 20)
	for key, value := range containers {
		if value == "epga" {
			containersA = append(containersA, key)
		} else {
			containersB = append(containersB, key)
		}
	}

	// Verify containers in epga can ping default gateway
	c.Assert(s.pingTestByName(containersA, s.globInfo.Gateway), IsNil)
	// Verify containers in epga cannot ping containers in epgb
	c.Assert(s.pingFailureTest(containersA, containersB), IsNil)
	// Verify containers in epgb can ping default gateway
	c.Assert(s.pingTestByName(containersB, s.globInfo.Gateway), IsNil)

	// Create a policy that allows ICMP and apply between A and B
	c.Assert(s.cli.PolicyPost(&client.Policy{
		PolicyName: "policyAB",
		TenantName: s.globInfo.Tenant,
	}), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		RuleID:            "1",
		PolicyName:        "policyAB",
		TenantName:        s.globInfo.Tenant,
		FromEndpointGroup: "epga",
		Direction:         "in",
		Protocol:          "icmp",
		Action:            "allow",
	}), IsNil)

	c.Assert(s.cli.EndpointGroupPost(&client.EndpointGroup{
		TenantName:  s.globInfo.Tenant,
		NetworkName: s.globInfo.Network,
		Policies:    []string{"policyAB"},
		GroupName:   "epgb",
	}), IsNil)
	time.Sleep(time.Second * 10)

	c.Assert(s.checkACILearning(s.globInfo.Tenant,
		"profile1",
		"epga",
		containersA), IsNil)

	c.Assert(s.checkACILearning(s.globInfo.Tenant,
		"profile1",
		"epgb",
		containersB), IsNil)

	// Verify containers in epga can ping containers in epgb
	for _, cB := range containersB {
		c.Assert(s.pingTestByName(containersA, cB.eth0.ip), IsNil)
	}

	// Verify TCP is not allowed.

	c.Assert(s.startListeners(containersA, []int{8000, 8001}), IsNil)
	c.Assert(s.startListeners(containersB, []int{8000, 8001}), IsNil)
	c.Assert(s.checkNoConnectionPairRetry(containersA, containersB, 8000, 1, 3), IsNil)
	c.Assert(s.checkNoConnectionPairRetry(containersA, containersB, 8000, 1, 3), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		RuleID:            "2",
		PolicyName:        "policyAB",
		TenantName:        s.globInfo.Tenant,
		FromEndpointGroup: "epga",
		Direction:         "in",
		Protocol:          "tcp",
		Port:              8000,
		Action:            "allow",
	}), IsNil)
	time.Sleep(time.Second * 10)

	c.Assert(s.checkACILearning(s.globInfo.Tenant,
		"profile1",
		"epga",
		containersA), IsNil)

	c.Assert(s.checkACILearning(s.globInfo.Tenant,
		"profile1",
		"epgb",
		containersB), IsNil)

	c.Assert(s.checkConnectionPairRetry(containersA, containersB, 8000, 1, 3), IsNil)
	c.Assert(s.checkNoConnectionPairRetry(containersA, containersB, 8001, 1, 3), IsNil)
	for _, cB := range containersB {
		c.Assert(s.pingTestByName(containersA, cB.eth0.ip), IsNil)
	}

	// Add a rule to allow 8001
	c.Assert(s.cli.RulePost(&client.Rule{
		RuleID:            "3",
		PolicyName:        "policyAB",
		TenantName:        s.globInfo.Tenant,
		FromEndpointGroup: "epga",
		Direction:         "in",
		Protocol:          "tcp",
		Port:              8001,
		Action:            "allow",
	}), IsNil)
	time.Sleep(time.Second * 10)

	c.Assert(s.checkACILearning(s.globInfo.Tenant,
		"profile1",
		"epga",
		containersA), IsNil)

	c.Assert(s.checkACILearning(s.globInfo.Tenant,
		"profile1",
		"epgb",
		containersB), IsNil)

	c.Assert(s.checkConnectionPairRetry(containersA, containersB, 8000, 1, 3), IsNil)
	c.Assert(s.checkConnectionPairRetry(containersA, containersB, 8001, 1, 3), IsNil)
	for _, cB := range containersB {
		c.Assert(s.pingTestByName(containersA, cB.eth0.ip), IsNil)
	}

	// Delete ICMP rule
	c.Assert(s.cli.RuleDelete(s.globInfo.Tenant, "policyAB", "1"), IsNil)
	time.Sleep(time.Second * 10)

	c.Assert(s.checkACILearning(s.globInfo.Tenant,
		"profile1",
		"epga",
		containersA), IsNil)

	c.Assert(s.checkACILearning(s.globInfo.Tenant,
		"profile1",
		"epgb",
		containersB), IsNil)

	c.Assert(s.pingFailureTest(containersA, containersB), IsNil)
	c.Assert(s.checkConnectionPairRetry(containersA, containersB, 8000, 1, 3), IsNil)
	c.Assert(s.checkConnectionPairRetry(containersA, containersB, 8001, 1, 3), IsNil)
	// Delete TCP 8000 rule
	c.Assert(s.cli.RuleDelete(s.globInfo.Tenant, "policyAB", "2"), IsNil)
	time.Sleep(time.Second * 10)
	c.Assert(s.checkACILearning(s.globInfo.Tenant,
		"profile1",
		"epga",
		containersA), IsNil)

	c.Assert(s.checkACILearning(s.globInfo.Tenant,
		"profile1",
		"epgb",
		containersB), IsNil)
	c.Assert(s.checkNoConnectionPairRetry(containersA, containersB, 8000, 1, 3), IsNil)
	c.Assert(s.checkConnectionPairRetry(containersA, containersB, 8001, 1, 3), IsNil)
	c.Assert(s.pingFailureTest(containersA, containersB), IsNil)

	// Delete the app profile
	c.Assert(s.cli.AppProfileDelete(s.globInfo.Tenant, "profile1"), IsNil)
	time.Sleep(time.Second * 5)

	c.Assert(s.checkNoConnectionPairRetry(containersA, containersB, 8000, 1, 3), IsNil)
	c.Assert(s.checkNoConnectionPairRetry(containersA, containersB, 8001, 1, 3), IsNil)
	c.Assert(s.pingFailureTest(containersA, containersB), IsNil)

	// Create the app profile with a different name
	c.Assert(s.cli.AppProfilePost(&client.AppProfile{
		TenantName:     s.globInfo.Tenant,
		EndpointGroups: []string{"epga", "epgb"},
		AppProfileName: "profile2",
	}), IsNil)
	time.Sleep(time.Second * 10)
	c.Assert(s.checkACILearning(s.globInfo.Tenant,
		"profile2",
		"epga",
		containersA), IsNil)

	c.Assert(s.checkACILearning(s.globInfo.Tenant,
		"profile2",
		"epgb",
		containersB), IsNil)
	c.Assert(s.removeContainers(append(containersA, containersB...)), IsNil)
	containersA = nil
	containersB = nil
	containers, err = s.runContainersInGroups(s.basicInfo.Containers, s.globInfo.Network, s.globInfo.Tenant, groups)
	c.Assert(err, IsNil)
	time.Sleep(time.Second * 20)
	for key, value := range containers {
		if value == "epga" {
			containersA2 = append(containersA2, key)
		} else {
			containersB2 = append(containersB2, key)
		}
	}
	c.Assert(s.startListeners(containersA2, []int{8000, 8001}), IsNil)
	c.Assert(s.startListeners(containersB2, []int{8000, 8001}), IsNil)
	c.Assert(s.checkNoConnectionPairRetry(containersA2, containersB2, 8000, 1, 3), IsNil)
	c.Assert(s.checkConnectionPairRetry(containersA2, containersB2, 8001, 1, 3), IsNil)
	c.Assert(s.pingFailureTest(containersA2, containersB2), IsNil)

	// Add TCP 8000 rule.
	c.Assert(s.cli.RulePost(&client.Rule{
		RuleID:            "2",
		PolicyName:        "policyAB",
		TenantName:        s.globInfo.Tenant,
		FromEndpointGroup: "epga",
		Direction:         "in",
		Protocol:          "tcp",
		Port:              8000,
		Action:            "allow",
	}), IsNil)
	err = errors.New("forced")
	//c.Assert(err, IsNil)
	time.Sleep(time.Second * 10)
	c.Assert(s.checkACILearning(s.globInfo.Tenant,
		"profile2",
		"epga",
		containersA2), IsNil)

	c.Assert(s.checkACILearning(s.globInfo.Tenant,
		"profile2",
		"epgb",
		containersB2), IsNil)

	c.Assert(s.checkConnectionPairRetry(containersA2, containersB2, 8000, 1, 3), IsNil)
	c.Assert(s.checkConnectionPairRetry(containersA2, containersB2, 8001, 1, 3), IsNil)
	c.Assert(s.pingFailureTest(containersA2, containersB2), IsNil)

	// Delete the app profile
	c.Assert(s.cli.AppProfileDelete(s.globInfo.Tenant, "profile2"), IsNil)
	time.Sleep(time.Second * 5)
	c.Assert(s.checkNoConnectionPairRetry(containersA2, containersB2, 8000, 1, 3), IsNil)
	c.Assert(s.checkNoConnectionPairRetry(containersA2, containersB2, 8001, 1, 3), IsNil)
	c.Assert(s.pingFailureTest(containersA2, containersB2), IsNil)

	c.Assert(s.removeContainers(append(containersA2, containersB2...)), IsNil)
	containersA2 = nil
	containersB2 = nil
	c.Assert(s.cli.EndpointGroupDelete(s.globInfo.Tenant, "epga"), IsNil)
	c.Assert(s.cli.EndpointGroupDelete(s.globInfo.Tenant, "epgb"), IsNil)
	c.Assert(s.cli.RuleDelete(s.globInfo.Tenant, "policyAB", "2"), IsNil)
	c.Assert(s.cli.RuleDelete(s.globInfo.Tenant, "policyAB", "3"), IsNil)

	c.Assert(s.cli.PolicyDelete(s.globInfo.Tenant, "policyAB"), IsNil)
	c.Assert(s.cli.NetworkDelete(s.globInfo.Tenant, s.globInfo.Network), IsNil)
}

func (s *systemtestSuite) TestACIGWRestart(c *C) {
	if s.fwdMode == "routing" {
		c.Skip("Skipping test for routing mode")
	}

	c.Assert(s.cli.GlobalPost(&client.Global{
		Name:             "global",
		NetworkInfraType: "aci",
		Vlans:            s.globInfo.Vlan,
		Vxlans:           s.globInfo.Vxlan,
		PvtSubnet:        "172.19.0.0/16",
		FwdMode:          "bridge",
		ArpMode:          "flood",
	}), IsNil)

	c.Assert(s.cli.TenantPost(&client.Tenant{
		TenantName: s.globInfo.Tenant,
	}), IsNil)

	containersA := []*container{}
	containersB := []*container{}

	c.Assert(s.cli.NetworkPost(&client.Network{
		TenantName:  s.globInfo.Tenant,
		NetworkName: s.globInfo.Network,
		Subnet:      s.globInfo.Subnet,
		Gateway:     s.globInfo.Gateway,
		Encap:       s.globInfo.Encap,
	}), IsNil)

	err := s.nodes[0].checkSchedulerNetworkCreated(s.globInfo.Network, false)
	c.Assert(err, IsNil)

	c.Assert(s.cli.EndpointGroupPost(&client.EndpointGroup{
		TenantName:  s.globInfo.Tenant,
		NetworkName: s.globInfo.Network,
		GroupName:   "epga",
	}), IsNil)

	c.Assert(s.cli.EndpointGroupPost(&client.EndpointGroup{
		TenantName:  s.globInfo.Tenant,
		NetworkName: s.globInfo.Network,
		GroupName:   "epgb",
	}), IsNil)

	c.Assert(s.cli.AppProfilePost(&client.AppProfile{
		TenantName:     s.globInfo.Tenant,
		EndpointGroups: []string{"epga", "epgb"},
		AppProfileName: "profile1",
	}), IsNil)

	time.Sleep(5 * time.Second)

	groups := []string{"epga", "epgb"}
	containers, err := s.runContainersInGroups(s.basicInfo.Containers, s.globInfo.Network, s.globInfo.Tenant, groups)
	c.Assert(err, IsNil)
	time.Sleep(time.Second * 20)
	for key, value := range containers {
		if value == "epga" {
			containersA = append(containersA, key)
		} else {
			containersB = append(containersB, key)
		}
	}

	// Verify containers in epga can ping default gateway
	c.Assert(s.pingTestByName(containersA, s.globInfo.Gateway), IsNil)
	// Verify containers in epga cannot ping containers in epgb
	c.Assert(s.pingFailureTest(containersA, containersB), IsNil)
	// Verify containers in epgb can ping default gateway
	c.Assert(s.pingTestByName(containersB, s.globInfo.Gateway), IsNil)
	// Verify containers within epga can ping each other
	c.Assert(s.pingTest(containersA), IsNil)
	// Verify containers within epgb can ping each other
	c.Assert(s.pingTest(containersB), IsNil)

	c.Assert(s.cli.PolicyPost(&client.Policy{
		PolicyName: "policyAB",
		TenantName: s.globInfo.Tenant,
	}), IsNil)

	c.Assert(s.cli.RulePost(&client.Rule{
		RuleID:            "1",
		PolicyName:        "policyAB",
		TenantName:        s.globInfo.Tenant,
		FromEndpointGroup: "epga",
		Direction:         "in",
		Protocol:          "icmp",
		Action:            "allow",
	}), IsNil)
	c.Assert(s.cli.EndpointGroupPost(&client.EndpointGroup{
		TenantName:  s.globInfo.Tenant,
		NetworkName: s.globInfo.Network,
		Policies:    []string{"policyAB"},
		GroupName:   "epgb",
	}), IsNil)
	time.Sleep(time.Second * 10)

	c.Assert(s.checkACILearning(s.globInfo.Tenant,
		"profile1",
		"epga",
		containersA), IsNil)

	c.Assert(s.checkACILearning(s.globInfo.Tenant,
		"profile1",
		"epgb",
		containersB), IsNil)

	// Verify containers in epga can ping containers in epgb
	for _, cB := range containersB {
		c.Assert(s.pingTestByName(containersA, cB.eth0.ip), IsNil)
	}
	c.Assert(s.cli.RulePost(&client.Rule{
		RuleID:            "2",
		PolicyName:        "policyAB",
		TenantName:        s.globInfo.Tenant,
		FromEndpointGroup: "epga",
		Direction:         "in",
		Protocol:          "tcp",
		Port:              8000,
		Action:            "allow",
	}), IsNil)
	time.Sleep(time.Second * 10)

	c.Assert(s.checkACILearning(s.globInfo.Tenant,
		"profile1",
		"epga",
		containersA), IsNil)

	c.Assert(s.checkACILearning(s.globInfo.Tenant,
		"profile1",
		"epgb",
		containersB), IsNil)

	c.Assert(s.startListeners(containersA, []int{8000, 8001}), IsNil)
	c.Assert(s.startListeners(containersB, []int{8000, 8001}), IsNil)
	c.Assert(s.checkConnectionPairRetry(containersA, containersB, 8000, 1, 3), IsNil)
	c.Assert(s.checkNoConnectionPairRetry(containersA, containersB, 8001, 1, 3), IsNil)
	for _, cB := range containersB {
		c.Assert(s.pingTestByName(containersA, cB.eth0.ip), IsNil)
	}

	log.Infof("Restarting ACI gateway on all nodes")
	for iNode := range s.nodes {
		s.nodes[iNode].tbnode.RunCommand("sudo service aci-gw restart")
	}
	time.Sleep(10 * time.Millisecond)

	//Verifying the behavior didn't change after restarting the gateway
	for _, cB := range containersB {
		c.Assert(s.pingTestByName(containersA, cB.eth0.ip), IsNil)
	}
	c.Assert(s.pingTestByName(containersA, s.globInfo.Gateway), IsNil)
	c.Assert(s.pingTestByName(containersB, s.globInfo.Gateway), IsNil)
	c.Assert(s.pingTest(containersA), IsNil)
	c.Assert(s.pingTest(containersB), IsNil)

	// Add a rule to allow 8001
	c.Assert(s.cli.RulePost(&client.Rule{
		RuleID:            "3",
		PolicyName:        "policyAB",
		TenantName:        s.globInfo.Tenant,
		FromEndpointGroup: "epga",
		Direction:         "in",
		Protocol:          "tcp",
		Port:              8001,
		Action:            "allow",
	}), IsNil)
	time.Sleep(time.Second * 10)

	c.Assert(s.checkACILearning(s.globInfo.Tenant,
		"profile1",
		"epga",
		containersA), IsNil)

	c.Assert(s.checkACILearning(s.globInfo.Tenant,
		"profile1",
		"epgb",
		containersB), IsNil)
	c.Assert(s.checkConnectionPairRetry(containersA, containersB, 8000, 1, 3), IsNil)
	c.Assert(s.checkConnectionPairRetry(containersA, containersB, 8001, 1, 3), IsNil)

	// Delete the app profile
	c.Assert(s.cli.AppProfileDelete(s.globInfo.Tenant, "profile1"), IsNil)
	time.Sleep(time.Second * 5)

	c.Assert(s.removeContainers(append(containersA, containersB...)), IsNil)
	containersA = nil
	containersB = nil
	c.Assert(s.cli.EndpointGroupDelete(s.globInfo.Tenant, "epga"), IsNil)
	c.Assert(s.cli.EndpointGroupDelete(s.globInfo.Tenant, "epgb"), IsNil)
	c.Assert(s.cli.RuleDelete(s.globInfo.Tenant, "policyAB", "2"), IsNil)
	c.Assert(s.cli.RuleDelete(s.globInfo.Tenant, "policyAB", "3"), IsNil)

	c.Assert(s.cli.PolicyDelete(s.globInfo.Tenant, "policyAB"), IsNil)
	c.Assert(s.cli.NetworkDelete(s.globInfo.Tenant, s.globInfo.Network), IsNil)
}
