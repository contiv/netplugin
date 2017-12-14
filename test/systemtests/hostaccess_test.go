package systemtests

import (
	. "github.com/contiv/check"
	"github.com/contiv/netplugin/contivmodel/client"
	"time"
)

func (s *systemtestSuite) TestBasicHostAccess(c *C) {
	if s.fwdMode == "routing" {
		c.Skip("Skipping basic host access test for routing mode")
	}

	global, err := s.cli.GlobalGet("global")
	c.Assert(err, IsNil)
	// save the FwdMode
	fm := global.FwdMode
	global.FwdMode = "routing"

	c.Assert(s.TearDownDefaultNetwork(), IsNil)
	c.Assert(s.cli.GlobalPost(global), IsNil)
	time.Sleep(30 * time.Second)
	c.Assert(s.SetupDefaultNetwork(), IsNil)

	s.hostAccTest(c)
	global.FwdMode = fm

	c.Assert(s.TearDownDefaultNetwork(), IsNil)
	c.Assert(s.cli.GlobalPost(global), IsNil)
	time.Sleep(30 * time.Second)
	c.Assert(s.SetupDefaultNetwork(), IsNil)
}

func (s *systemtestSuite) hostAccTest(c *C) {
	c.Assert(s.cli.NetworkPost(&client.Network{
		TenantName:  "default",
		NetworkName: "pipe-net",
		Subnet:      "17.5.4.0/22",
		Gateway:     "17.5.7.1",
		Encap:       "vxlan",
	}), IsNil)

	c.Assert(s.cli.NetworkPost(&client.Network{
		TenantName:  "default",
		NetworkName: "contivh1",
		Subnet:      "123.4.5.0/24",
		Gateway:     "123.4.5.1",
		Encap:       "vxlan",
		NwType:      "infra",
	}), IsNil)

	c.Assert(s.cli.NetworkPost(&client.Network{
		TenantName:  "default",
		NetworkName: "bunker-net",
		Subnet:      "13.5.7.0/24",
		Gateway:     "13.5.7.1",
		Encap:       "vxlan",
	}), IsNil)

	c.Assert(s.cli.EndpointGroupPost(&client.EndpointGroup{
		TenantName:  "default",
		NetworkName: "bunker-net",
		GroupName:   "epg-a",
	}), IsNil)

	time.Sleep(15 * time.Second)
	c.Assert(s.verifyHostRoutes([]string{"17.5.4.0/22", "13.5.7.0/24"}, true), IsNil)
	// Create num_nodes + 1 containers
	numContainters := len(s.nodes) + 1
	epgNames := make([]string, numContainters)
	for ix := 0; ix < numContainters; ix++ {
		epgNames[ix] = "epg-a"
	}

	cList, err := s.runContainers(numContainters, true, "bunker-net", "default",
		epgNames, []string{})
	c.Assert(err, IsNil)
	err = s.verifyEPs(cList)
	c.Assert(err, IsNil)
	masterIP, err := s.nodes[0].exec.getMasterIP()
	c.Assert(err, IsNil)
	//make sure they can ping the master node.
	dest := []string{masterIP}
	c.Assert(s.pingTestToNonContainer(cList, dest), IsNil)
	// verify the containers cannot ping each other on the NAT interface
	c.Assert(s.IsolationTest(cList), IsNil)
	c.Assert(s.verifyHostPing(cList), IsNil)

	c.Assert(s.removeContainers(cList), IsNil)
	c.Assert(s.cli.EndpointGroupDelete("default", "epg-a"), IsNil)
	c.Assert(s.cli.NetworkDelete("default", "bunker-net"), IsNil)
	c.Assert(s.verifyHostRoutes([]string{"13.5.7.0/24"}, false), IsNil)
	c.Assert(s.verifyHostRoutes([]string{"17.5.4.0/22"}, true), IsNil)
	c.Assert(s.cli.NetworkDelete("default", "pipe-net"), IsNil)
	c.Assert(s.verifyHostRoutes([]string{"17.5.4.0/22"}, false), IsNil)
	c.Assert(s.cli.NetworkDelete("default", "contivh1"), IsNil)
}
