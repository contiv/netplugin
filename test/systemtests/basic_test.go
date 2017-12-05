package systemtests

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	. "github.com/contiv/check"
	"github.com/contiv/netplugin/contivmodel/client"
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

	for i := 0; i < s.basicInfo.Iterations; i++ {
		containers, err := s.runContainers(s.basicInfo.Containers, false, "private", "", nil, nil)
		c.Assert(err, IsNil)

		if s.fwdMode == "routing" && encap == "vlan" {
			var err error
			err = s.CheckBgpRouteDistribution(c, containers)
			c.Assert(err, IsNil)
		}

		c.Assert(s.pingTest(containers), IsNil)
		c.Assert(s.removeContainers(containers), IsNil)
	}

	// epg pool
	c.Assert(s.cli.EndpointGroupPost(&client.EndpointGroup{
		GroupName:   "epg1",
		NetworkName: "private",
		IpPool:      "10.1.0.1-10.1.0.10",
		TenantName:  "default",
	}), IsNil)

	for i := 0; i < s.basicInfo.Iterations; i++ {
		containers, err := s.runContainersInService(s.basicInfo.Containers, "epg1", "private", "default",
			[]string{})
		c.Assert(err, IsNil)
		if s.fwdMode == "routing" && encap == "vlan" {
			var err error
			err = s.CheckBgpRouteDistribution(c, containers)
			c.Assert(err, IsNil)
		}
		c.Assert(s.pingTest(containers), IsNil)
		c.Assert(s.removeContainers(containers), IsNil)
	}
	c.Assert(s.cli.EndpointGroupDelete("default", "epg1"), IsNil)
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

	containers, err := s.runContainers(s.basicInfo.Containers, false, "private", "", nil, nil)
	c.Assert(err, IsNil)
	if s.fwdMode == "routing" && encap == "vlan" {
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

		if s.fwdMode == "routing" && encap == "vlan" {
			var err error
			err = s.CheckBgpRouteDistribution(c, containers)
			c.Assert(err, IsNil)
		}

	}

	c.Assert(s.removeContainers(containers), IsNil)
	c.Assert(s.cli.NetworkDelete("default", "private"), IsNil)
}

func (s *systemtestSuite) TestBasicSvcDiscoveryVXLAN(c *C) {
	if s.basicInfo.Scheduler == kubeScheduler {
		return
	}
	s.testBasicSvcDiscovery(c, "vxlan")
}

func (s *systemtestSuite) TestBasicSvcDiscoveryVLAN(c *C) {
	if s.basicInfo.Scheduler == kubeScheduler {
		return
	}
	s.testBasicSvcDiscovery(c, "vlan")
}

func (s *systemtestSuite) testBasicSvcDiscovery(c *C, encap string) {
	if s.basicInfo.ClusterStoreDriver == "etcd" {
		c.Skip("Skipping test")
	}

	if s.fwdMode == "routing" && encap == "vlan" {
		s.SetupBgp(c, false)
		s.CheckBgpConnection(c)
	}
	c.Assert(s.cli.NetworkPost(&client.Network{
		PktTag:      1001,
		NetworkName: "private",
		Subnet:      "10.100.1.0/24",
		Gateway:     "10.100.1.254",
		Encap:       encap,
		TenantName:  "default",
	}), IsNil)

	for i := 0; i < s.basicInfo.Iterations; i++ {
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

		// create DNS container
		dnsContainer, err := s.runContainersOnNode(1, "private", "default", "", s.nodes[0])
		c.Assert(err, IsNil)
		dnsIPAddr := dnsContainer[0].eth0.ip

		containers1, err := s.runContainersWithDNS(s.basicInfo.Containers, "default", "private",
			fmt.Sprintf("svc1%d", i), dnsIPAddr)
		c.Assert(err, IsNil)
		containers2, err := s.runContainersWithDNS(s.basicInfo.Containers, "default", "private",
			fmt.Sprintf("svc2%d", i), dnsIPAddr)
		c.Assert(err, IsNil)

		containers := append(containers1, containers2...)
		if s.fwdMode == "routing" && encap == "vlan" {
			var err error
			err = s.CheckBgpRouteDistribution(c, containers)
			c.Assert(err, IsNil)
		}
		if s.fwdMode == "routing" && encap == "vlan" {
			time.Sleep(5 * time.Second)
		}

		// Check name resolution
		c.Assert(s.pingTestByName(containers, fmt.Sprintf("svc1%d", i)), IsNil)
		c.Assert(s.pingTestByName(containers, fmt.Sprintf("svc2%d", i)), IsNil)

		// cleanup
		c.Assert(s.removeContainers(dnsContainer), IsNil)
		c.Assert(s.removeContainers(containers), IsNil)
		c.Assert(s.cli.EndpointGroupDelete(group1.TenantName, group1.GroupName), IsNil)
		c.Assert(s.cli.EndpointGroupDelete(group2.TenantName, group2.GroupName), IsNil)
	}

	c.Assert(s.cli.NetworkDelete("default", "private"), IsNil)
}

func (s *systemtestSuite) TestBasicNetmasterPortListen(c *C) {
	var masterNodeIndex int32
	var masterNode *node
	masterDefaultPort := "9999"

	for _, node := range s.nodes {
		// Stop all netmaster instances
		c.Assert(node.stopNetmaster(), IsNil)
		if node.Name() == "k8master" {
			masterNode = node
		}
	}

	for i := 0; i < s.basicInfo.Iterations; i++ {
		masterNodeIndex = 0
		if s.basicInfo.Scheduler != kubeScheduler {
			masterNodeIndex = rand.Int31n(int32(len(s.nodes)))
			masterNode = s.nodes[masterNodeIndex]
		}
		masterIP, err := masterNode.getIPAddr(s.hostInfo.HostMgmtInterface)
		c.Assert(err, IsNil)

		masterListenPort := "999" + masterIP[len(masterIP)-1:]
		masterCtrlPort := "888" + masterIP[len(masterIP)-1:]

		// Case: --listen-url 0.0.0.0:XXXX (XXXX is not default port 9999)
		// Requests to port other than masterListenPort(XXXX) should not be honored
		logrus.Infof("Checking case: --listen-url 0.0.0.0:XXXX (XXXX is not default port 9999)")
		c.Assert(masterNode.startNetmaster(fmt.Sprintf("--listen-url=0.0.0.0:%s", masterListenPort)), IsNil)
		time.Sleep(40 * time.Second)
		c.Assert(checkNetmasterPortListener(masterDefaultPort), NotNil)
		c.Assert(checkNetmasterPortListener(masterListenPort), IsNil)
		c.Assert(checkNetmasterPortListener(masterCtrlPort), NotNil)
		c.Assert(masterNode.stopNetmaster(), IsNil)
		time.Sleep(5 * time.Second)

		// Case: --listen-url 0.0.0.0:YYYY --control-url A.B.C.D:YYYY
		// Requests to port other than YYYY(masterCtrlPort) should not be honored
		logrus.Infof("Checking case: --listen-url 0.0.0.0:YYYY --control-url A.B.C.D:YYYY")
		c.Assert(masterNode.startNetmaster(fmt.Sprintf("--listen-url=0.0.0.0:%s --control-url=%s:%s", masterCtrlPort, masterIP, masterCtrlPort)), IsNil)
		time.Sleep(40 * time.Second)
		c.Assert(checkNetmasterPortListener(masterDefaultPort), NotNil)
		c.Assert(checkNetmasterPortListener(masterListenPort), NotNil)
		c.Assert(checkNetmasterPortListener(masterCtrlPort), IsNil)
		c.Assert(masterNode.stopNetmaster(), IsNil)
		time.Sleep(5 * time.Second)

		// Case: --listen-url 0.0.0.0:XXXX --control-url=A.B.C.D:YYYY
		// Requests to port other than masterListenPort should not be honored
		// masterCtrlPort is accessible only within the cluster for control pkts
		logrus.Infof("Checking case: --listen-url 0.0.0.0:XXXX --control-url=A.B.C.D:YYYY")
		c.Assert(masterNode.startNetmaster(fmt.Sprintf("--listen-url=0.0.0.0:%s --control-url=%s:%s", masterListenPort, masterIP, masterCtrlPort)), IsNil)
		time.Sleep(40 * time.Second)
		c.Assert(checkNetmasterPortListener(masterDefaultPort), NotNil)
		c.Assert(checkNetmasterPortListener(masterListenPort), IsNil)
		c.Assert(checkNetmasterPortListener(masterCtrlPort), NotNil)
		c.Assert(masterNode.stopNetmaster(), IsNil)
		time.Sleep(5 * time.Second)
	}

}

func checkNetmasterPortListener(port string) error {
	clientURL := fmt.Sprintf("http://localhost:%s", port)
	cliClient, err := client.NewContivClient(clientURL)
	if err != nil {
		return fmt.Errorf("failed to initialize the contiv client. Err: %+v", err)
	}

	tenant, err := cliClient.TenantGet("default")
	if err != nil || !strings.Contains(tenant.TenantName, "default") {
		return fmt.Errorf("the client request to %s failed. tenant: %+v err: %+v", clientURL, tenant, err)
	}

	return nil
}
