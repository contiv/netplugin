package systemtests

import (
	"github.com/contiv/contivmodel/client"
	. "gopkg.in/check.v1"
)

func (s *systemtestSuite) TestACIMode(c *C) {
	c.Assert(s.cli.GlobalPost(&client.Global{
		Name:             "global",
		NetworkInfraType: "aci",
		Vlans:            "1-4094",
		Vxlans:           "1-10000",
	}), IsNil)
	c.Assert(s.cli.NetworkPost(&client.Network{
		TenantName:  "default",
		NetworkName: "aciNet",
		Subnet:      "22.2.2.0/24",
		Gateway:     "22.2.2.254",
		Encap:       "vlan",
	}), IsNil)

	c.Assert(s.cli.EndpointGroupPost(&client.EndpointGroup{
		TenantName:  "default",
		NetworkName: "aciNet",
		GroupName:   "epgA",
	}), IsNil)

	c.Assert(s.cli.EndpointGroupPost(&client.EndpointGroup{
		TenantName:  "default",
		NetworkName: "aciNet",
		GroupName:   "epgB",
	}), IsNil)

	cA1, err := s.nodes[0].runContainer(containerSpec{networkName: "epgA.aciNet"})
	c.Assert(err, IsNil)

	cA2, err := s.nodes[0].runContainer(containerSpec{networkName: "epgA.aciNet"})
	c.Assert(err, IsNil)

	cB1, err := s.nodes[0].runContainer(containerSpec{networkName: "epgB.aciNet"})
	c.Assert(err, IsNil)

	cB2, err := s.nodes[0].runContainer(containerSpec{networkName: "epgB.aciNet"})
	c.Assert(err, IsNil)

	// Verify cA1 can ping cA2
	c.Assert(cA1.checkPing(cA2.eth0), IsNil)
	// Verify cB1 can ping cB2
	c.Assert(cB1.checkPing(cB2.eth0), IsNil)
	// Verify cA1 cannot ping cB1
	c.Assert(cA1.checkPingFailure(cB1.eth0), IsNil)

	c.Assert(s.removeContainers([]*container{cA1, cA2, cB1, cB2}), IsNil)
	c.Assert(s.cli.EndpointGroupDelete("default", "aciNet", "epgA"), IsNil)
	c.Assert(s.cli.EndpointGroupDelete("default", "aciNet", "epgB"), IsNil)
	c.Assert(s.cli.NetworkDelete("default", "aciNet"), IsNil)
}
