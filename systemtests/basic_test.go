package systemtests

import (
	"fmt"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/contivmodel/client"
	. "gopkg.in/check.v1"
)

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
		c.Assert(err, IsNil)

		if s.fwdMode == "routing" && encap == "vlan" {
			s.CheckBgpRouteDistribution(c, s.vagrant.GetNode("quagga1"), containers)
		}

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

	containers, err := s.runContainers(s.containers, false, "private", nil)
	c.Assert(err, IsNil)
	if s.fwdMode == "routing" && encap == "vlan" {
		s.CheckBgpRouteDistribution(c, s.vagrant.GetNode("quagga1"), containers)
	}

	for i := 0; i < s.iterations; i++ {
		c.Assert(s.pingTest(containers), IsNil)

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
			s.CheckBgpRouteDistribution(c, s.vagrant.GetNode("quagga1"), containers)
		}

	}

	c.Assert(s.removeContainers(containers), IsNil)
	c.Assert(s.cli.NetworkDelete("default", "private"), IsNil)
}

func (s *systemtestSuite) TestBasicSvcDiscoveryVXLAN(c *C) {
	s.testBasicSvcDiscovery(c, "vxlan")
}

func (s *systemtestSuite) TestBasicSvcDiscoveryVLAN(c *C) {
	s.testBasicSvcDiscovery(c, "vlan")
}

func (s *systemtestSuite) testBasicSvcDiscovery(c *C, encap string) {
	if !strings.Contains(s.clusterStore, "etcd") || !s.enableDNS {
		c.Skip("Skipping test")
	}

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
		group1 := &client.EndpointGroup{
			GroupName:   fmt.Sprintf("svc1%d", i),
			NetworkName: "private",
			Policies:    []string{},
			TenantName:  "default",
		}
		group2 := &client.EndpointGroup{
			GroupName:   fmt.Sprintf("svc2%d", i),
			NetworkName: "private",
			Policies:    []string{},
			TenantName:  "default",
		}
		logrus.Infof("Creating epg: %s", group1.GroupName)
		c.Assert(s.cli.EndpointGroupPost(group1), IsNil)
		logrus.Infof("Creating epg: %s", group2.GroupName)
		c.Assert(s.cli.EndpointGroupPost(group2), IsNil)

		containers1, err := s.runContainersWithDNS(s.containers, "default", "private", fmt.Sprintf("svc1%d", i))
		c.Assert(err, IsNil)
		containers2, err := s.runContainersWithDNS(s.containers, "default", "private", fmt.Sprintf("svc2%d", i))
		c.Assert(err, IsNil)

		containers := append(containers1, containers2...)
		if s.fwdMode == "routing" && encap == "vlan" {
			time.Sleep(5 * time.Second)
		}

		// Check name resolution
		c.Assert(s.pingTestByName(containers, fmt.Sprintf("svc1%d.private.default", i)), IsNil)
		c.Assert(s.pingTestByName(containers, fmt.Sprintf("svc2%d.private.default", i)), IsNil)

		// cleanup
		c.Assert(s.removeContainers(containers), IsNil)
		c.Assert(s.cli.EndpointGroupDelete(group1.TenantName, group1.NetworkName, group1.GroupName), IsNil)
		c.Assert(s.cli.EndpointGroupDelete(group2.TenantName, group2.NetworkName, group2.GroupName), IsNil)
	}

	c.Assert(s.cli.NetworkDelete("default", "private"), IsNil)
}
