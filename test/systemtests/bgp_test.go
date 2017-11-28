package systemtests

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	. "github.com/contiv/check"
	"github.com/contiv/netplugin/contivmodel/client"
	"github.com/contiv/remotessh"
)

/*TestBgpContainerToContainerPing tests the following:
1) Checks pings between containers on the same host
2) Checks pings between containers on different hosts connected to same Tor
3) Checks pings between containers on different hosts
4) Checks ping between containers on different networks
*/
func (s *systemtestSuite) TestBgpContainerToContainerPing(c *C) {
	if s.fwdMode != "routing" {
		c.Skip("Skipping test for bridge mode")
	}

	s.SetupBgp(c, false)
	s.CheckBgpConnection(c)

	for i := 0; i < s.basicInfo.Iterations; i++ {
		var (
			netNames      = []string{}
			containers    = map[string][]*container{}
			allcontainers = []*container{}
		)

		numContainer := s.basicInfo.Containers
		if numContainer < 3 {
			numContainer = 3
		}

		for networkNum := 0; networkNum < numContainer/len(s.nodes); networkNum++ {
			network := &client.Network{
				TenantName:  "default",
				NetworkName: fmt.Sprintf("net%d-%d", networkNum+1, 1),
				Subnet:      fmt.Sprintf("10.1.%d.0/24", networkNum+1),
				Gateway:     fmt.Sprintf("10.1.%d.254", networkNum+1),
				Encap:       "vlan",
			}

			c.Assert(s.cli.NetworkPost(network), IsNil)
			netNames = append(netNames, network.NetworkName)
		}

		for _, name := range netNames {
			var err error
			containers[name], err = s.runContainers(numContainer, false, name, "", nil, nil)
			c.Assert(err, IsNil)
			allcontainers = append(allcontainers, containers[name]...)
		}

		err := s.CheckBgpRouteDistribution(c, allcontainers)
		c.Assert(err, IsNil)
		endChan := make(chan error)

		logrus.Infof("Running ping test ")
		c.Assert(s.pingTest(allcontainers), IsNil)

		for name := range containers {
			go func(conts []*container) { endChan <- s.removeContainers(conts) }(containers[name])
		}

		for range containers {
			<-endChan
		}

		for _, netName := range netNames {
			c.Assert(s.cli.NetworkDelete("default", netName), IsNil)
		}
	}
	s.TearDownBgp(c)
}

/*TestBgpContainerBareMetalPing tests the following:
1) Checks pings between containers and non container workloads
*/
func (s *systemtestSuite) TestBgpContainerToNonContainerPing(c *C) {
	if s.fwdMode != "routing" {
		c.Skip("Skipping test for bridge mode")
	}

	var (
		netNames   = []string{}
		containers = map[string][]*container{}
		ips        = []string{}
	)

	numContainer := s.basicInfo.Containers
	if numContainer < 3 {
		numContainer = 3
	}

	s.SetupBgp(c, false)
	s.CheckBgpConnection(c)

	for networkNum := 0; networkNum < numContainer/len(s.nodes); networkNum++ {
		network := &client.Network{
			TenantName:  "default",
			NetworkName: fmt.Sprintf("net%d-%d", networkNum+1, 1),
			Subnet:      fmt.Sprintf("10.1.%d.0/24", networkNum+1),
			Gateway:     fmt.Sprintf("10.1.%d.254", networkNum+1),
			Encap:       "vlan",
		}

		c.Assert(s.cli.NetworkPost(network), IsNil)
		netNames = append(netNames, network.NetworkName)
	}

	for _, name := range netNames {
		var err error
		containers[name], err = s.runContainers(numContainer, false, name, "", nil, nil)
		c.Assert(err, IsNil)
	}

	time.Sleep(15 * time.Second)
	endChan := make(chan error)

	//FIXME make it variable number of quagga instances
	ips = append(ips, "50.1.1.200", "60.1.1.200", "80.1.1.200")

	for key, conts := range containers {
		logrus.Infof("Running ping test for network %q", key)
		go func(c *C, conts []*container) { endChan <- s.pingTestToNonContainer(conts, ips) }(c, conts)
	}

	for range containers {
		c.Assert(<-endChan, IsNil)
	}

	for name := range containers {
		go func(conts []*container) { endChan <- s.removeContainers(conts) }(containers[name])
	}

	s.TearDownBgp(c)
}

/*TestBgpTriggerPeerAddDelete tests the following:
1) Checks withdrawal of bgp external routes learnt on Peer
2) Checks readdition of external routes on peer up
3) Checks ping success to remote endpoints
4) Checks bgp peering and route distribution for pre existing containers (before bgp peering)
*/
func (s *systemtestSuite) TestBgpTriggerPeerAddDelete(c *C) {
	if s.fwdMode != "routing" {
		c.Skip("Skipping test for bridge mode")
	}

	var (
		netNames      = []string{}
		containers    = map[string][]*container{}
		allcontainers = []*container{}
	)

	numContainer := s.basicInfo.Containers
	if numContainer < 3 {
		numContainer = 3
	}

	for networkNum := 0; networkNum < numContainer/len(s.nodes); networkNum++ {
		network := &client.Network{
			TenantName:  "default",
			NetworkName: fmt.Sprintf("net%d-%d", networkNum, 1),
			Subnet:      fmt.Sprintf("10.1.%d.0/24", networkNum),
			Gateway:     fmt.Sprintf("10.1.%d.254", networkNum),
			PktTag:      1001 + networkNum,
			Encap:       "vlan",
		}

		c.Assert(s.cli.NetworkPost(network), IsNil)
		netNames = append(netNames, network.NetworkName)
	}

	for _, name := range netNames {
		var err error
		containers[name], err = s.runContainers(numContainer, false, name, "", nil, nil)
		c.Assert(err, IsNil)
		allcontainers = append(allcontainers, containers[name]...)
	}

	time.Sleep(5 * time.Second)
	for i := 0; i < s.basicInfo.Iterations; i++ {
		s.SetupBgp(c, false)
		s.CheckBgpConnection(c)
		err := s.CheckBgpRouteDistribution(c, allcontainers)
		c.Assert(err, IsNil)
		logrus.Infof("Running ping test")
		c.Assert(s.pingTest(allcontainers), IsNil)

		s.TearDownBgp(c)
	}
}

/*TestBgpTriggerLinkUpDown tests the following:
1) Checks withdrawal of bgp external routes learnt on Peer
2) Checks readdition of external routes on peer add
3) Checks ping success to remote endpoints
*/
func (s *systemtestSuite) TestBgpTriggerLinkUpDown(c *C) {

	if s.fwdMode != "routing" {
		c.Skip("Skipping test for bridge mode")
	}
	for i := 0; i < s.basicInfo.Iterations; i++ {

		s.SetupBgp(c, false)
		s.CheckBgpConnection(c)

		var (
			netNames      = []string{}
			containers    = map[string][]*container{}
			allcontainers = []*container{}
		)

		numContainer := s.basicInfo.Containers
		if numContainer < 3 {
			numContainer = 3
		}

		for networkNum := 0; networkNum < numContainer/len(s.nodes); networkNum++ {
			network := &client.Network{
				TenantName:  "default",
				NetworkName: fmt.Sprintf("net%d-%d", networkNum, 1),
				Subnet:      fmt.Sprintf("10.1.%d.0/24", networkNum),
				Gateway:     fmt.Sprintf("10.1.%d.254", networkNum),
				PktTag:      1001 + networkNum,
				Encap:       "vlan",
			}

			c.Assert(s.cli.NetworkPost(network), IsNil)
			netNames = append(netNames, network.NetworkName)
		}

		for _, name := range netNames {
			var err error
			containers[name], err = s.runContainers(numContainer, false, name, "", nil, nil)
			c.Assert(err, IsNil)
			allcontainers = append(allcontainers, containers[name]...)
		}
		err := s.CheckBgpRouteDistribution(c, allcontainers)
		c.Assert(err, IsNil)
		endChan := make(chan error)

		logrus.Infof("Running ping test")
		c.Assert(s.pingTest(allcontainers), IsNil)

		s.vagrant.GetNode("netplugin-node1").RunCommand("sudo ip link set eth2 down")
		s.CheckBgpNoConnectionForaNode(c, s.vagrant.GetNode("netplugin-node1"))
		s.vagrant.GetNode("netplugin-node1").RunCommand("sudo ip link set eth2 up")
		s.CheckBgpConnectionForaNode(c, s.vagrant.GetNode("netplugin-node1"))
		err = s.CheckBgpRouteDistribution(c, allcontainers)
		c.Assert(err, IsNil)
		logrus.Infof("Running ping test")
		c.Assert(s.pingTest(allcontainers), IsNil)

		for name := range containers {
			go func(conts []*container) { endChan <- s.removeContainers(conts) }(containers[name])
		}

		for range containers {
			<-endChan
		}

		for _, netName := range netNames {
			c.Assert(s.cli.NetworkDelete("default", netName), IsNil)
		}
	}
	s.TearDownBgp(c)
}

/*TestBgpTriggerLoopbackDownUp tests the following:
1) Checks withdrawal of routes learnt from the host
2) Checks readdition of external routes on peer up
3) Checks ping success from remote endpoints
*/
func (s *systemtestSuite) TestBgpTriggerLoopbackDownUp(c *C) {

	if s.fwdMode != "routing" {
		c.Skip("Skipping test for bridge mode")
	}

	var (
		netNames      = []string{}
		containers    = map[string][]*container{}
		allcontainers = []*container{}
	)

	numContainer := s.basicInfo.Containers
	if numContainer < 3 {
		numContainer = 3
	}

	for networkNum := 0; networkNum < numContainer/len(s.nodes); networkNum++ {
		network := &client.Network{
			TenantName:  "default",
			NetworkName: fmt.Sprintf("net%d-%d", networkNum, 1),
			Subnet:      fmt.Sprintf("10.1.%d.0/24", networkNum),
			Gateway:     fmt.Sprintf("10.1.%d.254", networkNum),
			PktTag:      1001 + networkNum,
			Encap:       "vlan",
		}

		c.Assert(s.cli.NetworkPost(network), IsNil)
		netNames = append(netNames, network.NetworkName)
	}

	endChan := make(chan error)
	for _, name := range netNames {
		var err error
		containers[name], err = s.runContainers(numContainer, false, name, "", nil, nil)
		c.Assert(err, IsNil)
		allcontainers = append(allcontainers, containers[name]...)
	}

	for i := 0; i < s.basicInfo.Iterations; i++ {
		s.SetupBgp(c, false)
		s.CheckBgpConnection(c)

		err := s.CheckBgpRouteDistribution(c, allcontainers)
		c.Assert(err, IsNil)
		logrus.Infof("Running ping test")
		c.Assert(s.pingTest(allcontainers), IsNil)

		s.vagrant.GetNode("netplugin-node1").RunCommand("sudo ip link set inb01 down")
		s.CheckBgpNoConnectionForaNode(c, s.vagrant.GetNode("netplugin-node1"))

		s.vagrant.GetNode("netplugin-node1").RunCommand("sudo ip link set inb01 up")
		s.vagrant.GetNode("netplugin-node1").RunCommand("sudo ip addr add 50.1.1.2/24 dev inb01")
		s.CheckBgpConnectionForaNode(c, s.vagrant.GetNode("netplugin-node1"))
		err = s.CheckBgpRouteDistribution(c, allcontainers)
		c.Assert(err, IsNil)
		logrus.Infof("Running ping test")
		c.Assert(s.pingTest(allcontainers), IsNil)

		s.TearDownBgp(c)
	}

	for name := range containers {
		go func(conts []*container) { endChan <- s.removeContainers(conts) }(containers[name])
	}

	for range containers {
		<-endChan
	}

	for _, netName := range netNames {
		c.Assert(s.cli.NetworkDelete("default", netName), IsNil)
	}

}

/*TestBgpTriggerContainerDelete tests the following:
1) Checks non reachbility to the deleted container from other containers
2) Checks non reachabiluty to the deleted container from non container workloads
*/

func (s *systemtestSuite) TestBgpTriggerContainerAddDelete(c *C) {

	if s.fwdMode != "routing" {
		c.Skip("Skipping test for bridge mode")
	}

	s.SetupBgp(c, false)
	s.CheckBgpConnection(c)

	var (
		netNames = []string{}
	)

	numContainer := s.basicInfo.Containers
	if numContainer < 3 {
		numContainer = 3
	}

	for networkNum := 0; networkNum < numContainer/len(s.nodes); networkNum++ {
		network := &client.Network{
			TenantName:  "default",
			NetworkName: fmt.Sprintf("net%d-%d", networkNum, 1),
			Subnet:      fmt.Sprintf("10.1.%d.0/24", networkNum),
			Gateway:     fmt.Sprintf("10.1.%d.254", networkNum),
			PktTag:      1001 + networkNum,
			Encap:       "vlan",
		}

		c.Assert(s.cli.NetworkPost(network), IsNil)
		netNames = append(netNames, network.NetworkName)
	}

	for i := 0; i < s.basicInfo.Iterations; i++ {
		var (
			containers    = map[string][]*container{}
			allcontainers = []*container{}
		)

		for _, name := range netNames {
			var err error
			containers[name], err = s.runContainers(numContainer, false, name, "", nil, nil)
			c.Assert(err, IsNil)
			allcontainers = append(allcontainers, containers[name]...)
		}

		err := s.CheckBgpRouteDistribution(c, allcontainers)
		c.Assert(err, IsNil)
		endChan := make(chan error)

		logrus.Infof("Running ping test")
		c.Assert(s.pingTest(allcontainers), IsNil)

		for _, name := range netNames {
			for _, cont := range containers[name] {
				go func(cont *container) { endChan <- cont.node.exec.stop(cont) }(cont)
			}
		}

		for _, name := range netNames {
			for range containers[name] {
				c.Assert(<-endChan, IsNil)
			}
		}
		for _, name := range netNames {
			for _, cont := range containers[name] {
				go func(cont *container) { endChan <- cont.node.exec.start(cont) }(cont)
			}
		}

		for _, name := range netNames {
			for range containers[name] {
				c.Assert(<-endChan, IsNil)
			}
		}

		err = s.CheckBgpRouteDistribution(c, allcontainers)
		c.Assert(err, IsNil)
		logrus.Infof("Running ping test")
		c.Assert(s.pingTest(allcontainers), IsNil)
		for name := range containers {
			go func(conts []*container) { endChan <- s.removeContainers(conts) }(containers[name])
		}

		for range containers {
			<-endChan
		}
		allcontainers = nil
	}
	s.TearDownBgp(c)

}

/*TestBgpTriggerNetpluginRestart tests the following:
1) Checks bgp peering restablished on netplugin restart
2) Checks ping success between containers on netplugin restart
3) Checks ping success between containers and non container workloads
*/
func (s *systemtestSuite) TestBgpTriggerNetpluginRestart(c *C) {

	if s.fwdMode != "routing" {
		c.Skip("Skipping test for bridge mode")
	}

	s.SetupBgp(c, false)
	s.CheckBgpConnection(c)

	var (
		netNames      = []string{}
		containers    = map[string][]*container{}
		allcontainers = []*container{}
	)

	numContainer := s.basicInfo.Containers
	if numContainer < 3 {
		numContainer = 3
	}

	for networkNum := 0; networkNum < numContainer/len(s.nodes); networkNum++ {
		network := &client.Network{
			TenantName:  "default",
			NetworkName: fmt.Sprintf("net%d-%d", networkNum, 1),
			Subnet:      fmt.Sprintf("10.1.%d.0/24", networkNum),
			Gateway:     fmt.Sprintf("10.1.%d.254", networkNum),
			PktTag:      1001 + networkNum,
			Encap:       "vlan",
		}

		c.Assert(s.cli.NetworkPost(network), IsNil)
		netNames = append(netNames, network.NetworkName)
	}

	for _, name := range netNames {
		var err error
		containers[name], err = s.runContainers(numContainer, false, name, "", nil, nil)
		c.Assert(err, IsNil)
		allcontainers = append(allcontainers, containers[name]...)
	}

	err := s.CheckBgpRouteDistribution(c, allcontainers)
	c.Assert(err, IsNil)
	endChan := make(chan error)

	logrus.Infof("Running ping test")
	c.Assert(s.pingTest(allcontainers), IsNil)

	for _, node := range s.nodes {
		var err error
		c.Assert(node.stopNetplugin(), IsNil)
		logrus.Info("Sleeping for a while to wait for netplugin's TTLs to expire")
		time.Sleep(1 * time.Minute)
		time.Sleep(30 * time.Second)
		c.Assert(node.rotateLog("netplugin"), IsNil)
		c.Assert(node.startNetplugin(""), IsNil)
		c.Assert(node.exec.runCommandUntilNoNetpluginError(), IsNil)
		time.Sleep(15 * time.Second)
		s.CheckBgpConnection(c)
		err = s.CheckBgpRouteDistribution(c, allcontainers)
		c.Assert(err, IsNil)
		logrus.Infof("Running ping test")
		c.Assert(s.pingTest(allcontainers), IsNil)
		time.Sleep(5 * time.Minute)
	}

	for name := range containers {
		go func(conts []*container) { endChan <- s.removeContainers(conts) }(containers[name])
	}

	for range containers {
		<-endChan
	}

	for _, netName := range netNames {
		c.Assert(s.cli.NetworkDelete("default", netName), IsNil)
	}

	s.TearDownBgp(c)

}

/*TestBgpTriggerNetmasterRestart tests the following:
1) Checks bgp peering on netmaster restart
2) Checks ping success between containers on netmaster restart
3) Checks ping success between containers and non container workloads
*/
/*
func (s *systemtestSuite) TestBgpTriggerNetmasterRestart(c *C) {
	if s.fwdMode != "routing" {
		c.Skip("Skipping test for bridge mode")
	}
	s.SetupBgp(c, false)
	s.CheckBgpConnection(c)
	var (
		netNames      = []string{}
		containers    = map[string][]*container{}
		allcontainers = []*container{}
	)
	numContainer := s.basicInfo.Containers
	if numContainer < 3 {
		numContainer = 3
	}
	for networkNum := 0; networkNum < numContainer/len(s.nodes); networkNum++ {
		network := &client.Network{
			TenantName:  "default",
			NetworkName: fmt.Sprintf("net%d-%d", networkNum, 1),
			Subnet:      fmt.Sprintf("10.1.%d.0/24", networkNum),
			Gateway:     fmt.Sprintf("10.1.%d.254", networkNum),
			PktTag:      1001 + networkNum,
			Encap:       "vlan",
		}
		c.Assert(s.cli.NetworkPost(network), IsNil)
		netNames = append(netNames, network.NetworkName)
	}
	for _, name := range netNames {
		var err error
		containers[name], err = s.runContainers(numContainer, false, name, nil)
		c.Assert(err, IsNil)
		allcontainers = append(allcontainers, containers[name]...)
	}
	time.Sleep(5 * time.Second)
	endChan := make(chan error)
	logrus.Infof("Running ping test")
	c.Assert(s.pingTest(allcontainers), IsNil)
	for i := 0; i < 2; i++ {
		for _, node := range s.nodes {
			c.Assert(node.stopNetmaster(), IsNil)
			time.Sleep(1 * time.Minute)
			c.Assert(node.rotateLog("netmaster"), IsNil)
			c.Assert(node.startNetmaster(), IsNil)
			time.Sleep(5 * time.Second)
			s.CheckBgpConnection(c)
			logrus.Infof("Running ping test")
			c.Assert(s.pingTest(allcontainers), IsNil)
		}
	}
	for name := range containers {
		go func(conts []*container) { endChan <- s.removeContainers(conts) }(containers[name])
	}
	for range containers {
		<-endChan
	}
	for _, netName := range netNames {
		c.Assert(s.cli.NetworkDelete("default", netName), IsNil)
	}
	s.TearDownBgp(c)
}
*/

func (s *systemtestSuite) TestBgpMultiTrigger(c *C) {

	if s.fwdMode != "routing" {
		c.Skip("Skipping test for bridge mode")
	}
	var (
		iter = 0
	)

	for _, nodeToStop := range s.nodes {
		var (
			netNames      = []string{}
			containers    = map[string][]*container{}
			allcontainers = []*container{}
		)
		iter++
		c.Assert(nodeToStop.stopNetplugin(), IsNil)
		logrus.Info("Sleeping for a while to wait for netplugin's TTLs to expire")
		time.Sleep(2 * time.Minute)
		s.SetupBgp(c, false)
		for _, node := range s.nodes {

			if node != nodeToStop {
				node.tbnode.RunCommandWithOutput("sudo ip link set inb01 up")
				s.CheckBgpConnectionForaNode(c, node.tbnode)
			}
		}
		c.Assert(nodeToStop.startNetplugin(""), IsNil)
		time.Sleep(120 * time.Second)
		nodeToStop.tbnode.RunCommandWithOutput("sudo ip link set inb01 up")
		s.CheckBgpConnectionForaNode(c, nodeToStop.tbnode)

		numContainer := s.basicInfo.Containers
		if numContainer < 3 {
			numContainer = 3
		}

		for networkNum := 0; networkNum < numContainer/len(s.nodes); networkNum++ {
			network := &client.Network{
				TenantName:  "default",
				NetworkName: fmt.Sprintf("net%d-%d", networkNum, iter),
				Subnet:      fmt.Sprintf("10.1.%d.0/24", networkNum),
				Gateway:     fmt.Sprintf("10.1.%d.254", networkNum),
				PktTag:      1001 + networkNum,
				Encap:       "vlan",
			}

			c.Assert(s.cli.NetworkPost(network), IsNil)
			netNames = append(netNames, network.NetworkName)
		}

		for _, name := range netNames {
			var err error
			containers[name], err = s.runContainers(numContainer, false, name, "", nil, nil)
			c.Assert(err, IsNil)
			allcontainers = append(allcontainers, containers[name]...)
		}

		err := s.CheckBgpRouteDistribution(c, allcontainers)
		c.Assert(err, IsNil)
		endChan := make(chan error)

		logrus.Infof("Running ping test")
		c.Assert(s.pingTest(allcontainers), IsNil)

		for name := range containers {
			go func(conts []*container) { endChan <- s.removeContainers(conts) }(containers[name])
		}

		for range containers {
			<-endChan
		}

		for _, netName := range netNames {
			c.Assert(s.cli.NetworkDelete("default", netName), IsNil)
		}

		s.TearDownBgp(c)
	}
}

/*TestBgpSequencePeerAddLinkDown tests the following:
1) Verifies sequence in which peer is configured followed by
link up established bgp.
*/
func (s *systemtestSuite) TestBgpSequencePeerAddLinkDown(c *C) {

	if s.fwdMode != "routing" {
		c.Skip("Skipping test for bridge mode")
	}
	for _, node := range s.nodes {
		logrus.Infof("Bringing down uplinks")
		dataIntfs := strings.Split(s.hostInfo.HostDataInterfaces, ",")
		for _, dataIntf := range dataIntfs {
			node.tbnode.RunCommandWithOutput(fmt.Sprintf("sudo ip link set %s down", dataIntf))
		}
	}
	s.SetupBgp(c, false)

	for _, node := range s.nodes {
		logrus.Infof("Bringing up uplinks")
		dataIntfs := strings.Split(s.hostInfo.HostDataInterfaces, ",")
		for _, dataIntf := range dataIntfs {
			node.tbnode.RunCommandWithOutput(fmt.Sprintf("sudo ip link set %s up", dataIntf))
		}
	}
	s.CheckBgpConnection(c)

	var (
		netNames      = []string{}
		containers    = map[string][]*container{}
		allcontainers = []*container{}
	)

	numContainer := s.basicInfo.Containers
	if numContainer < 3 {
		numContainer = 3
	}

	for networkNum := 0; networkNum < numContainer/len(s.nodes); networkNum++ {
		network := &client.Network{
			TenantName:  "default",
			NetworkName: fmt.Sprintf("net%d-%d", networkNum, 1),
			Subnet:      fmt.Sprintf("10.1.%d.0/24", networkNum),
			Gateway:     fmt.Sprintf("10.1.%d.254", networkNum),
			PktTag:      1001 + networkNum,
			Encap:       "vlan",
		}

		c.Assert(s.cli.NetworkPost(network), IsNil)
		netNames = append(netNames, network.NetworkName)
	}

	for _, name := range netNames {
		var err error
		containers[name], err = s.runContainers(numContainer, false, name, "", nil, nil)
		c.Assert(err, IsNil)
		allcontainers = append(allcontainers, containers[name]...)
	}
	err := s.CheckBgpRouteDistribution(c, allcontainers)
	c.Assert(err, IsNil)
	logrus.Infof("Running ping test")
	c.Assert(s.pingTest(allcontainers), IsNil)
	s.TearDownBgp(c)

}

/*TestBgpMisconfigRecovery checks the following:
1) if after a misconfig bgp can be reconfigured
2) Bgp is established and ping works*/
func (s *systemtestSuite) TestBgpMisconfigRecovery(c *C) {
	if s.fwdMode != "routing" {
		c.Skip("Skipping test for bridge mode")
	}

	s.SetupBgp(c, true)

	time.Sleep(2 * time.Second)

	s.SetupBgp(c, false)

	s.CheckBgpConnection(c)

	var (
		netNames      = []string{}
		containers    = map[string][]*container{}
		allcontainers = []*container{}
	)

	numContainer := s.basicInfo.Containers
	if numContainer < 3 {
		numContainer = 3
	}

	for networkNum := 0; networkNum < numContainer/len(s.nodes); networkNum++ {
		network := &client.Network{
			TenantName:  "default",
			NetworkName: fmt.Sprintf("net%d-%d", networkNum, 1),
			Subnet:      fmt.Sprintf("10.1.%d.0/24", networkNum),
			Gateway:     fmt.Sprintf("10.1.%d.254", networkNum),
			PktTag:      1001 + networkNum,
			Encap:       "vlan",
		}

		c.Assert(s.cli.NetworkPost(network), IsNil)
		netNames = append(netNames, network.NetworkName)
	}

	for _, name := range netNames {
		var err error
		containers[name], err = s.runContainers(numContainer, false, name, "", nil, nil)
		c.Assert(err, IsNil)
		allcontainers = append(allcontainers, containers[name]...)
	}
	err := s.CheckBgpRouteDistribution(c, allcontainers)
	c.Assert(err, IsNil)
	endChan := make(chan error)

	logrus.Infof("Running ping test")
	c.Assert(s.pingTest(allcontainers), IsNil)

	for name := range containers {
		go func(conts []*container) { endChan <- s.removeContainers(conts) }(containers[name])
	}

	for range containers {
		<-endChan
	}

	for _, netName := range netNames {
		c.Assert(s.cli.NetworkDelete("default", netName), IsNil)
	}
	s.TearDownBgp(c)

}

func (s *systemtestSuite) SetupBgp(c *C, misConfig bool) {
	var neighborIP, routerIP, hostname string
	for num := 0; num < len(s.nodes); num++ {
		hostname = fmt.Sprintf("netplugin-node%d", num+1)
		logrus.Infof("Adding bgp host to %s", hostname)
		netNum := (num + 1) % 3
		if netNum == 0 {
			routerIP = "80.1.1.2/24"
			neighborIP = "80.1.1.200"
		} else if netNum == 1 {
			routerIP = "50.1.1.2/24"
			neighborIP = "50.1.1.200"
		} else {
			routerIP = "60.1.1.1/24"
			neighborIP = "60.1.1.200"
		}
		nAs := "500"
		as := "65002"

		if misConfig {
			nAs = "600"
			as = "65002"
			neighborIP = "90.1.1.1"
		}
		bgpCfg := &client.Bgp{
			As:         as,
			Hostname:   hostname,
			Neighbor:   neighborIP,
			NeighborAs: nAs,
			Routerip:   routerIP,
		}
		c.Assert(s.cli.BgpPost(bgpCfg), IsNil)
	}
}

func (s *systemtestSuite) TearDownBgp(c *C) {
	var hostname string
	for num := 0; num < len(s.nodes); num++ {
		hostname = fmt.Sprintf("netplugin-node%d", num+1)
		c.Assert(s.cli.BgpDelete(hostname), IsNil)
	}
}

func (s *systemtestSuite) CheckBgpConnection(c *C) {

	endChan := make(chan error)
	for _, n := range s.nodes {
		go func(n node) {
			for i := 0; i < 100; i++ {
				time.Sleep(3 * time.Second)
				bgp, err := s.cli.BgpInspect(n.Name())
				if err != nil {
					continue
				}
				if strings.Contains(bgp.Oper.NeighborStatus, "established") {
					logrus.Infof("the BGP Connection on Node:%s established", n.Name())
					endChan <- nil
					return
				}
			}
			endChan <- errors.New("the BGP connection failed")
		}(*n)
	}

	for range s.nodes {
		c.Assert(<-endChan, IsNil)
	}
}

func (s *systemtestSuite) CheckBgpNoConnection(c *C) {

	endChan := make(chan error)
	for _, n := range s.nodes {
		go func(n node) {
			for i := 0; i < 100; i++ {
				time.Sleep(3 * time.Second)
				bgp, err := s.cli.BgpInspect(n.Name())
				if err != nil {
					continue
				}
				if !strings.Contains(bgp.Oper.NeighborStatus, "established") {
					logrus.Infof("Bgp Connection on Node:%s established", n.Name())
					endChan <- nil
					return
				}
			}
			endChan <- errors.New("the BGP connection failed")
		}(*n)
	}

	for range s.nodes {
		c.Assert(<-endChan, IsNil)
	}
}

func (s *systemtestSuite) CheckBgpConnectionForaNode(c *C, node remotessh.TestbedNode) error {
	for i := 0; i < 100; i++ {
		time.Sleep(3 * time.Second)
		bgp, err := s.cli.BgpInspect(node.GetName())
		if err != nil {
			continue
		}
		if strings.Contains(bgp.Oper.NeighborStatus, "established") {
			logrus.Infof("Bgp Connection on Node:%s established", node.GetName())
			return nil
		}
	}
	return errors.New("the BGP connection not established")
}

func (s *systemtestSuite) CheckBgpNoConnectionForaNode(c *C, node remotessh.TestbedNode) error {
	for i := 0; i < 100; i++ {
		time.Sleep(3 * time.Second)
		bgp, err := s.cli.BgpInspect(node.GetName())
		if err != nil {
			continue
		}
		if !strings.Contains(bgp.Oper.NeighborStatus, "established") {
			logrus.Infof("Bgp Connection on Node:%s Disconnected", node.GetName())
			return nil
		}
	}
	return errors.New("bgp connection persists")
}

func (s *systemtestSuite) CheckBgpRouteDistribution(c *C, containers []*container) error {
	endChan := make(chan error)
	for _, n := range s.nodes {
		go func(n *node, containers []*container) {
			logrus.Infof("Checking Bgp container route distribution on node %s", n.Name())
			remoteRoutes := 0
			for _, cont := range containers {
				if cont.node != n {
					remoteRoutes++
				}
			}
			for i := 0; i < 120; i++ {
				time.Sleep(1 * time.Second)
				contCount := 0
				bgp, err := s.cli.BgpInspect(n.Name())
				if err == nil {
					routes := strings.Join(bgp.Oper.Routes, ",")
					for _, cont := range containers {
						if strings.Contains(routes, cont.eth0.ip) {
							contCount++
						}
					}
				}
				if contCount == remoteRoutes {
					endChan <- nil
					logrus.Infof("Done checking container route distribution on node %s", n.Name())
					return
				}
				time.Sleep(1 * time.Second)
			}
			endChan <- errors.New("the BGP route distribution is not complete")
		}(n, containers)
	}
	for range s.nodes {
		c.Assert(<-endChan, IsNil)
	}
	time.Sleep(4 * time.Second)
	return nil
}

func (s *systemtestSuite) CheckBgpRouteDistributionIPList(c *C, ips []string, inCluster bool) error {
	ipList := []string{}
	expCount := len(s.nodes)
	if inCluster {
		expCount--
	}
	for i := 0; i < 120; i++ {
		logrus.Infof("Checking Bgp container route distribution")
		time.Sleep(1 * time.Second)
		ipList = nil
		nodeCount := 0
		ch := make(chan bool)
		for _, ip := range ips {
			for _, n := range s.nodes {
				go func(n *node, ip string, ch chan bool) {
					bgp, err := s.cli.BgpInspect(n.Name())
					if err == nil {
						routes := strings.Join(bgp.Oper.Routes, ",")
						if strings.Contains(routes, ip) {
							ch <- true
							return
						}
					}
					ch <- false
				}(n, ip, ch)
			}
			for i := 0; i < len(s.nodes); i++ {
				if <-ch {
					nodeCount++
				}
			}
			if nodeCount == expCount {
				ipList = append(ipList, ip)
			}
			if len(ipList) == len(ips) {
				return nil
			}
		}
		time.Sleep(1 * time.Second)
	}
	return errors.New("bgp Route distribution not complete")
}
