package systemtests

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/contivmodel/client"
	. "gopkg.in/check.v1"
)

func (s *systemtestSuite) TestBasicStart(c *C) {
	s.testBasicStartRemoveContainerWrap(c)
	s.testBasicStartStopContainerWrap(c)
}

func (s *systemtestSuite) testBasicStartRemoveContainerWrap(c *C) {
	doneChan := make(chan struct{}, 2)

	go s.testBasicStartRemoveContainer(c, EncapVXLAN, doneChan, 0)
	go s.testBasicStartRemoveContainer(c, EncapVLAN, doneChan, 1)

	for i := 0; i < 2; i++ {
		<-doneChan
	}
}

func (s *systemtestSuite) testBasicStartRemoveContainer(c *C, encap string, doneChan chan struct{}, seq int) {
	defer func() { doneChan <- struct{}{} }()

	s.SetupBgp(c, encap, false)
	s.CheckBgpConnection(c, encap)

	c.Assert(s.cli.NetworkPost(&client.Network{
		PktTag:      1001 + seq,
		NetworkName: encap,
		Subnet:      fmt.Sprintf("10.1.%d.0/24", seq),
		Gateway:     fmt.Sprintf("10.1.%d.254", seq),
		Encap:       encap,
		TenantName:  "default",
	}), IsNil)

	for i := 0; i < s.basicInfo.Iterations; i++ {
		containers, err := s.runContainers(s.basicInfo.Containers, false, encap, "", nil, nil)
		c.Assert(err, IsNil)

		if s.isBGP(encap) {
			var err error
			err = s.CheckBgpRouteDistribution(c, containers)
			c.Assert(err, IsNil)
		}

		c.Assert(s.pingTest(containers), IsNil)
		c.Assert(s.removeContainers(containers), IsNil)
	}

	c.Assert(s.cli.NetworkDelete("default", encap), IsNil)
}

func (s *systemtestSuite) testBasicStartStopContainerWrap(c *C) {
	doneChan := make(chan struct{}, 2)

	go s.testBasicStartStopContainer(c, EncapVXLAN, doneChan, 0)
	go s.testBasicStartStopContainer(c, EncapVLAN, doneChan, 1)

	for i := 0; i < 2; i++ {
		<-doneChan
	}
}

func (s *systemtestSuite) testBasicStartStopContainer(c *C, encap string, doneChan chan struct{}, seq int) {
	defer func() { doneChan <- struct{}{} }()

	s.SetupBgp(c, encap, false)
	s.CheckBgpConnection(c, encap)

	c.Assert(s.cli.NetworkPost(&client.Network{
		PktTag:      1001 + seq,
		NetworkName: encap,
		Subnet:      fmt.Sprintf("10.1.%d.0/24", seq),
		Gateway:     fmt.Sprintf("10.1.%d.254", seq),
		Encap:       encap,
		TenantName:  "default",
	}), IsNil)

	containers, err := s.runContainers(s.basicInfo.Containers, false, encap, "", nil, nil)
	c.Assert(err, IsNil)
	if s.isBGP(encap) {
		var err error
		err = s.CheckBgpRouteDistribution(c, containers)
		c.Assert(err, IsNil)
	}

	for i := 0; i < s.basicInfo.Iterations; i++ {
		c.Assert(s.pingTest(containers), IsNil)

		errChan := make(chan error)
		for _, cont := range containers {
			go func(cont *container) { errChan <- cont.node.exec.stop(cont) }(cont)
		}

		for range containers {
			c.Assert(<-errChan, IsNil)
		}

		for _, cont := range containers {
			go func(cont *container) { errChan <- cont.node.exec.start(cont) }(cont)
		}

		for range containers {
			c.Assert(<-errChan, IsNil)
		}

		if s.isBGP(encap) {
			var err error
			err = s.CheckBgpRouteDistribution(c, containers)
			c.Assert(err, IsNil)
		}

	}

	c.Assert(s.removeContainers(containers), IsNil)
	c.Assert(s.cli.NetworkDelete("default", encap), IsNil)
}

func (s *systemtestSuite) TestBasicSvcDiscovery(c *C) {
	if s.basicInfo.Scheduler == "k8" {
		return
	}

	doneChan := make(chan struct{}, 2)

	go s.testBasicSvcDiscovery(c, EncapVXLAN, doneChan, 0)
	go s.testBasicSvcDiscovery(c, EncapVLAN, doneChan, 1)

	for i := 0; i < 2; i++ {
		<-doneChan
	}
}

func (s *systemtestSuite) testBasicSvcDiscovery(c *C, encap string, doneChan chan struct{}, seq int) {
	defer func() { doneChan <- struct{}{} }()

	if !strings.Contains(s.basicInfo.ClusterStore, "etcd") {
		c.Skip("Skipping test")
	}
	// HACK: "--dns" option is broken in docker 1.10.3. skip this test
	if os.Getenv("CONTIV_DOCKER_VERSION") == "1.10.3" {
		c.Skip("Skipping dns test docker 1.10.3")
	}

	s.SetupBgp(c, encap, false)
	s.CheckBgpConnection(c, encap)

	c.Assert(s.cli.NetworkPost(&client.Network{
		PktTag:      1000 + seq,
		NetworkName: encap,
		Subnet:      fmt.Sprintf("10.1.%d.0/24", seq),
		Gateway:     fmt.Sprintf("10.1.%d.254", seq),
		Encap:       encap,
		TenantName:  "default",
	}), IsNil)

	for i := 0; i < s.basicInfo.Iterations; i++ {
		group1 := &client.EndpointGroup{
			GroupName:   fmt.Sprintf("svc1%d-%d", i, seq),
			NetworkName: encap,
			Policies:    []string{},
			TenantName:  "default",
		}
		group2 := &client.EndpointGroup{
			GroupName:   fmt.Sprintf("svc2%d-%d", i, seq),
			NetworkName: encap,
			Policies:    []string{},
			TenantName:  "default",
		}
		logrus.Infof("Creating epg: %s", group1.GroupName)
		c.Assert(s.cli.EndpointGroupPost(group1), IsNil)
		logrus.Infof("Creating epg: %s", group2.GroupName)
		c.Assert(s.cli.EndpointGroupPost(group2), IsNil)

		containers1, err := s.runContainersWithDNS(s.basicInfo.Containers, "default", encap, fmt.Sprintf("svc1%d-%d", i, seq))
		c.Assert(err, IsNil)
		containers2, err := s.runContainersWithDNS(s.basicInfo.Containers, "default", encap, fmt.Sprintf("svc2%d-%d", i, seq))
		c.Assert(err, IsNil)

		containers := append(containers1, containers2...)

		if s.isBGP(encap) {
			var err error
			err = s.CheckBgpRouteDistribution(c, containers)
			c.Assert(err, IsNil)
			time.Sleep(5 * time.Second)
		}

		// Check name resolution
		c.Assert(s.pingTestByName(containers, fmt.Sprintf("svc1%d-%d.default", i, seq)), IsNil)
		c.Assert(s.pingTestByName(containers, fmt.Sprintf("svc2%d-%d.default", i, seq)), IsNil)

		// cleanup
		c.Assert(s.removeContainers(containers), IsNil)
		c.Assert(s.cli.EndpointGroupDelete(group1.TenantName, group1.GroupName), IsNil)
		c.Assert(s.cli.EndpointGroupDelete(group2.TenantName, group2.GroupName), IsNil)
	}

	c.Assert(s.cli.NetworkDelete("default", encap), IsNil)
}
