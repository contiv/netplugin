package systemtests

import (
	"github.com/contiv/contivmodel/client"
	. "gopkg.in/check.v1"
	"time"
)

var privateNetwork = &client.Network{
	PktTag:      1001,
	NetworkName: "private",
	Subnet:      "10.1.0.0/16",
	Gateway:     "10.1.1.254",
	Encap:       "vxlan",
	TenantName:  "default",
}

func (s *systemtestSuite) TestBasicStartRemoveContainerVXLAN(c *C) {
	s.testBasicStartRemoveContainer(c, "vxlan")
}

func (s *systemtestSuite) TestBasicStartRemoveContainerVLAN(c *C) {
	s.testBasicStartRemoveContainer(c, "vlan")
}

func (s *systemtestSuite) testBasicStartRemoveContainer(c *C, encap string) {

	if s.fwdMode == "routing" && encap == "vlan" {
		s.SetupBgp(c, false)
		s.CheckBgpConnection(c)
	}
	c.Assert(s.cli.NetworkPost(&client.Network{
		PktTag:      1001,
		NetworkName: "private",
		Subnet:      "10.1.0.0/16",
		Gateway:     "10.1.1.254",
		Encap:       encap,
		TenantName:  "default",
	}), IsNil)

	for i := 0; i < s.iterations; i++ {
		containers, err := s.runContainers(s.containers, false, "private", nil)
		if s.fwdMode == "routing" && encap == "vlan" {
			time.Sleep(5 * time.Second)
		}
		c.Assert(err, IsNil)
		c.Assert(s.pingTest(containers), IsNil)
		c.Assert(s.removeContainers(containers), IsNil)
	}

	c.Assert(s.cli.NetworkDelete("default", "private"), IsNil)
}

func (s *systemtestSuite) TestBasicStartStopContainerVXLAN(c *C) {
	s.testBasicStartStopContainer(c, "vxlan")
}

func (s *systemtestSuite) TestBasicStartStopContainerVLAN(c *C) {
	s.testBasicStartStopContainer(c, "vlan")
}

func (s *systemtestSuite) testBasicStartStopContainer(c *C, encap string) {
	if s.fwdMode == "routing" && encap == "vlan" {
		s.SetupBgp(c, false)
		s.CheckBgpConnection(c)
	}
	c.Assert(s.cli.NetworkPost(&client.Network{
		PktTag:      1001,
		NetworkName: "private",
		Subnet:      "10.1.0.0/16",
		Gateway:     "10.1.1.254",
		Encap:       encap,
		TenantName:  "default",
	}), IsNil)

	for i := 0; i < s.iterations; i++ {
		containers, err := s.runContainers(s.containers, false, "private", nil)
		c.Assert(err, IsNil)

		errChan := make(chan error)
		for _, cont := range containers {
			go func(cont *container) { errChan <- cont.stop() }(cont)
		}

		for range containers {
			c.Assert(<-errChan, IsNil)
		}

		for _, cont := range containers {
			go func(cont *container) { errChan <- cont.start() }(cont)
		}

		for range containers {
			c.Assert(<-errChan, IsNil)
		}

		if s.fwdMode == "routing" && encap == "vlan" {
			time.Sleep(5 * time.Second)
		}

		c.Assert(s.pingTest(containers), IsNil)
		c.Assert(s.removeContainers(containers), IsNil)
	}

	c.Assert(s.cli.NetworkDelete("default", "private"), IsNil)
}
