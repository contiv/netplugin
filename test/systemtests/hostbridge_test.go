package systemtests

import (
	"github.com/contiv/contivmodel/client"
	. "gopkg.in/check.v1"
)

func (s *systemtestSuite) TestHostBridge(c *C) {
	if s.scheduler != "k8" {
		return
	}
	c.Assert(s.cli.GlobalPost(&client.Global{
		Name:             "global",
		NetworkInfraType: "default",
		Vlans:            "1-4094",
		Vxlans:           "1-10000",
		FwdMode:          "bridge",
	}), IsNil)
	c.Assert(s.cli.NetworkPost(&client.Network{
		TenantName:  "default",
		NetworkName: "rogue-net",
		Subnet:      "23.4.5.0/24",
		Encap:       "vxlan",
	}), IsNil)

	c.Assert(s.cli.EndpointGroupPost(&client.EndpointGroup{
		TenantName:  "default",
		NetworkName: "rogue-net",
		GroupName:   "epg-a",
	}), IsNil)

	// Create num_nodes + 1 containers
	numContainters := len(s.nodes) + 1
	epgNames := make([]string, numContainters)
	for ix := 0; ix < numContainters; ix++ {
		epgNames[ix] = "epg-a"
	}

	cList, err := s.runContainers(numContainters, true, "rogue-net", "default",
		epgNames, []string{})
	c.Assert(err, IsNil)
	err = s.verifyEPs(cList)
	c.Assert(err, IsNil)
	masterIP, err := s.nodes[0].exec.getMasterIP()
	c.Assert(err, IsNil)
	//make sure they can ping the master node.
	dest := []string{masterIP}
	c.Assert(s.pingTestToNonContainer(cList, dest), IsNil)
	// verify the containers cannot ping each other on the host bridge interface
	c.Assert(s.hostIsolationTest(cList), IsNil)

	c.Assert(s.removeContainers(cList), IsNil)
	c.Assert(s.cli.EndpointGroupDelete("default", "epg-a"), IsNil)
	c.Assert(s.cli.NetworkDelete("default", "rogue-net"), IsNil)
}
