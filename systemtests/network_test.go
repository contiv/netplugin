package systemtests

import (
	"fmt"

	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/contivmodel/client"
	. "gopkg.in/check.v1"
)

func (s *systemtestSuite) TestInfraNetworkAddDeleteVXLAN(c *C) {
	s.testInfraNetworkAddDelete(c, "vxlan")
}

func (s *systemtestSuite) TestInfraNetworkAddDeleteVLAN(c *C) {
	s.testInfraNetworkAddDelete(c, "vlan")
}

func (s *systemtestSuite) testInfraNetworkAddDelete(c *C, encap string) {

	if s.fwdMode == "routing" && encap == "vlan" {
		s.SetupBgp(c, false)
		s.CheckBgpConnection(c)
	}

	for i := 0; i < s.iterations; i++ {
		var (
			netNames = []string{}
		)

		numInfraNw := 3
		for networkNum := 0; networkNum < numInfraNw; networkNum++ {
			network := &client.Network{
				TenantName:  "default",
				NwType:      "infra",
				NetworkName: fmt.Sprintf("net%d", networkNum),
				Subnet:      fmt.Sprintf("10.1.%d.0/24", networkNum),
				Gateway:     fmt.Sprintf("10.1.%d.254", networkNum),
				PktTag:      1001 + networkNum,
				Encap:       encap,
			}

			c.Assert(s.cli.NetworkPost(network), IsNil)

			// TBD: Need to fix timing issue
			// where endpoint create is received on non-master node
			// before network create is received
			time.Sleep(5 * time.Second)

			netNames = append(netNames, network.NetworkName)
		}

		node1 := s.nodes[0]

		for networkNum := 0; networkNum < numInfraNw; networkNum++ {
			// From first node, ping every node on this network
			for nodeNum := 1; nodeNum <= len(s.nodes); nodeNum++ {
				logrus.Infof("Running ping test for network %q node %d", netNames[networkNum], nodeNum)
				ipaddr := fmt.Sprintf("10.1.%d.%d", networkNum, nodeNum)
				if s.fwdMode == "routing" && encap == "vlan" {
					_, err := s.CheckBgpRouteDistributionIPList(c, []string{ipaddr})
					c.Assert(err, IsNil)
				}
				c.Assert(node1.checkPing(ipaddr), IsNil)

			}
		}

		for _, netName := range netNames {
			c.Assert(s.cli.NetworkDelete("default", netName), IsNil)
		}

		time.Sleep(5 * time.Second)
	}
}

func (s *systemtestSuite) TestNetworkAddDeleteVXLAN(c *C) {
	s.testNetworkAddDelete(c, "vxlan")
}

func (s *systemtestSuite) TestNetworkAddDeleteVLAN(c *C) {
	s.testNetworkAddDelete(c, "vlan")
}

func (s *systemtestSuite) testNetworkAddDelete(c *C, encap string) {

	if s.fwdMode == "routing" && encap == "vlan" {

		s.SetupBgp(c, false)
		s.CheckBgpConnection(c)
	}

	for i := 0; i < s.iterations; i++ {
		var (
			netNames   = []string{}
			containers = map[string][]*container{}
		)

		numContainer := s.containers
		if numContainer < 4 {
			numContainer = 4
		}

		for networkNum := 0; networkNum < numContainer/len(s.nodes); networkNum++ {
			network := &client.Network{
				TenantName:  "default",
				NetworkName: fmt.Sprintf("net%d-%d", networkNum, i),
				Subnet:      fmt.Sprintf("10.1.%d.0/24", networkNum),
				Gateway:     fmt.Sprintf("10.1.%d.254", networkNum),
				PktTag:      1001 + networkNum,
				Encap:       encap,
			}

			c.Assert(s.cli.NetworkPost(network), IsNil)
			netNames = append(netNames, network.NetworkName)
		}

		for _, name := range netNames {
			var err error
			containers[name], err = s.runContainers(numContainer, false, name, nil, nil)
			c.Assert(err, IsNil)
		}

		if s.fwdMode == "routing" && encap == "vlan" {
			var err error
			for _, name := range netNames {
				_, err = s.CheckBgpRouteDistribution(c, containers[name])
				c.Assert(err, IsNil)
			}
		}

		endChan := make(chan error)

		for key, conts := range containers {
			logrus.Infof("Running ping test for network %q", key)
			go func(c *C, conts []*container) { endChan <- s.pingTest(conts) }(c, conts)
		}

		for range containers {
			c.Assert(<-endChan, IsNil)
		}

		count := 0

		if s.fwdMode != "routing" {
			for key := range containers {
				for key2 := range containers {
					if key == key2 {
						continue
					}

					count++
					go func(conts1, conts2 []*container) { endChan <- s.pingFailureTest(conts1, conts2) }(containers[key], containers[key2])
				}
			}

			for i := 0; i < count; i++ {
				c.Assert(<-endChan, IsNil)
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
	}
}

func (s *systemtestSuite) TestNetworkAddDeleteNoGatewayVXLAN(c *C) {
	s.testNetworkAddDeleteNoGateway(c, "vxlan")
}

func (s *systemtestSuite) TestNetworkAddDeleteNoGatewayVLAN(c *C) {
	s.testNetworkAddDeleteNoGateway(c, "vlan")
}
func (s *systemtestSuite) testNetworkAddDeleteNoGateway(c *C, encap string) {

	if s.fwdMode == "routing" && encap == "vlan" {

		s.SetupBgp(c, false)
		s.CheckBgpConnection(c)
	}

	for i := 0; i < s.iterations; i++ {
		var (
			netNames   = []string{}
			containers = map[string][]*container{}
		)

		numContainer := s.containers
		if numContainer < 4 {
			numContainer = 4
		}

		for networkNum := 0; networkNum < numContainer/len(s.nodes); networkNum++ {
			network := &client.Network{
				TenantName:  "default",
				NetworkName: fmt.Sprintf("net%d-%d", networkNum, i),
				Subnet:      fmt.Sprintf("10.1.%d.0/24", networkNum),
				Encap:       encap,
			}

			c.Assert(s.cli.NetworkPost(network), IsNil)
			netNames = append(netNames, network.NetworkName)
		}

		for _, name := range netNames {
			var err error
			// There seem to be a docker bug in creating external connectivity if we run
			// containers in parallel. So, running it serially for this test
			containers[name], err = s.runContainersSerial(numContainer, false, name, nil)
			c.Assert(err, IsNil)
		}

		if s.fwdMode == "routing" && encap == "vlan" {
			for _, name := range netNames {
				s.CheckBgpRouteDistribution(c, containers[name])
			}
		}

		endChan := make(chan error)

		for key, conts := range containers {
			logrus.Infof("Running ping test for network %q", key)
			go func(c *C, conts []*container) { endChan <- s.pingTest(conts) }(c, conts)
		}

		for range containers {
			c.Assert(<-endChan, IsNil)
		}

		for name := range containers {
			s.removeContainers(containers[name])
		}

		for _, netName := range netNames {
			c.Assert(s.cli.NetworkDelete("default", netName), IsNil)
		}
	}
}

func (s *systemtestSuite) TestNetworkAddDeleteTenantVXLAN(c *C) {
	s.testNetworkAddDeleteTenant(c, "vxlan")
}

func (s *systemtestSuite) TestNetworkAddDeleteTenantVLAN(c *C) {
	s.testNetworkAddDeleteTenant(c, "vlan")
}

func (s *systemtestSuite) testNetworkAddDeleteTenant(c *C, encap string) {
	mutex := sync.Mutex{}

	if encap == "vlan" && s.fwdMode == "routing" {

		s.SetupBgp(c, false)
		s.CheckBgpConnection(c)
	}

	for i := 0; i < s.iterations; i++ {
		var (
			tenantNames = map[string][]string{}
			netNames    = []string{}
			containers  = map[string][]*container{}
			pktTag      = 0
		)

		numContainer := s.containers
		if numContainer < 4 {
			numContainer = 4
		}

		for tenantNum := 0; tenantNum < (s.containers / 2); tenantNum++ {
			tenantName := fmt.Sprintf("tenant%d", tenantNum)
			c.Assert(s.cli.TenantPost(&client.Tenant{TenantName: tenantName}), IsNil)
			tenantNames[tenantName] = []string{}

			for networkNum := 0; networkNum < numContainer/len(s.nodes); networkNum++ {
				network := &client.Network{
					TenantName:  tenantName,
					NetworkName: fmt.Sprintf("net%d-%d", networkNum, i),
					Subnet:      fmt.Sprintf("10.%d.%d.0/24", tenantNum, networkNum),
					Gateway:     fmt.Sprintf("10.%d.%d.254", tenantNum, networkNum),
					PktTag:      pktTag + 1000,
					Encap:       encap,
				}

				logrus.Infof("Creating network %s on tenant %s", network.NetworkName, network.TenantName)

				c.Assert(s.cli.NetworkPost(network), IsNil)
				netNames = append(netNames, network.NetworkName)
				tenantNames[tenantName] = append(tenantNames[tenantName], network.NetworkName)
				pktTag++
			}
		}

		for tenant, networks := range tenantNames {
			endChan := make(chan error)
			for _, network := range networks {
				go func(network, tenant string, containers map[string][]*container) {
					var err error
					mutex.Lock()
					containers[network], err = s.runContainers(numContainer, false, fmt.Sprintf("%s/%s", network, tenant), nil, nil)
					mutex.Unlock()
					endChan <- err

					if s.fwdMode == "routing" && encap == "vlan" {
						_, err = s.CheckBgpRouteDistribution(c, containers[network])
						c.Assert(err, IsNil)
					}
					endChan <- s.pingTest(containers[network])
				}(network, tenant, containers)
			}

			for i := 0; i < len(networks)*2; i++ {
				c.Assert(<-endChan, IsNil)
			}

			for _, network := range networks {
				c.Assert(s.removeContainers(containers[network]), IsNil)
				c.Assert(s.cli.NetworkDelete(tenant, network), IsNil)
			}

			c.Assert(s.cli.TenantDelete(tenant), IsNil)
		}
	}
}
