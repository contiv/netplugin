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
	if s.scheduler == "k8" {
		return
	}
	s.testInfraNetworkAddDelete(c, "vxlan")
}

func (s *systemtestSuite) TestInfraNetworkAddDeleteVLAN(c *C) {
	if s.scheduler == "k8" {
		return
	}
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
			var v6subnet, v6gateway string
			if networkNum%2 == 0 {
				v6subnet = ""
				v6gateway = ""
			} else {
				v6subnet = fmt.Sprintf("1001:%d::/120", networkNum)
				v6gateway = fmt.Sprintf("1001:%d::254", networkNum)
			}
			network := &client.Network{
				TenantName:  "default",
				NetworkName: fmt.Sprintf("net%d-%d", networkNum, i),
				Subnet:      fmt.Sprintf("10.1.%d.0/24", networkNum),
				Gateway:     fmt.Sprintf("10.1.%d.254", networkNum),
				Ipv6Subnet:  v6subnet,
				Ipv6Gateway: v6gateway,
				PktTag:      1001 + networkNum,
				Encap:       encap,
			}

			c.Assert(s.cli.NetworkPost(network), IsNil)
			netNames = append(netNames, network.NetworkName)
		}

		for _, name := range netNames {
			var err error
			containers[name], err = s.runContainers(numContainer, false, name, "", nil, nil)
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
			var v6subnet string
			if networkNum%2 == 0 {
				v6subnet = ""
			} else {
				v6subnet = fmt.Sprintf("1001:%d::/120", networkNum)
			}
			network := &client.Network{
				TenantName:  "default",
				NetworkName: fmt.Sprintf("net%d-%d", networkNum, i),
				Subnet:      fmt.Sprintf("10.1.%d.0/24", networkNum),
				Ipv6Subnet:  v6subnet,
				Encap:       encap,
			}

			c.Assert(s.cli.NetworkPost(network), IsNil)
			netNames = append(netNames, network.NetworkName)
		}

		for _, name := range netNames {
			var err error
			// There seem to be a docker bug in creating external connectivity if we run
			// containers in parallel. So, running it serially for this test
			containers[name], err = s.runContainersSerial(numContainer, false, name, "", nil)
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
	s.testNetworkAddDeleteTenant(c, "vxlan", s.fwdMode)
}

func (s *systemtestSuite) TestNetworkAddDeleteTenantVLAN(c *C) {
	s.testNetworkAddDeleteTenant(c, "vlan", s.fwdMode)
}

func (s *systemtestSuite) testNetworkAddDeleteTenant(c *C, encap, fwdmode string) {
	mutex := sync.Mutex{}

	if encap == "vlan" && fwdmode == "routing" {

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
				var v6subnet, v6gateway string
				if networkNum%2 == 0 {
					v6subnet = ""
					v6gateway = ""
				} else {
					v6subnet = fmt.Sprintf("1001:%d:%d::/120", tenantNum, networkNum)
					v6gateway = fmt.Sprintf("1001:%d:%d::254", tenantNum, networkNum)
				}
				network := &client.Network{
					TenantName:  tenantName,
					NetworkName: fmt.Sprintf("net%d-%d", networkNum, i),
					Subnet:      fmt.Sprintf("10.%d.%d.0/24", tenantNum, networkNum),
					Gateway:     fmt.Sprintf("10.%d.%d.254", tenantNum, networkNum),
					Ipv6Subnet:  v6subnet,
					Ipv6Gateway: v6gateway,
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
					containers[network], err = s.runContainers(numContainer, false, network, tenant, nil, nil)
					mutex.Unlock()
					endChan <- err

					if fwdmode == "routing" && encap == "vlan" {
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
	if encap == "vlan" && fwdmode == "routing" {
		s.TearDownBgp(c)
	}

}

func (s *systemtestSuite) TestNetworkAddDeleteTenantFwdModeChangeVXLAN(c *C) {
	fwdMode := s.fwdMode
	for i := 0; i < s.iterations; i++ {
		s.testNetworkAddDeleteTenant(c, "vxlan", fwdMode)
		if fwdMode == "routing" {
			c.Assert(s.cli.GlobalPost(&client.Global{FwdMode: "bridge",
				Name:             "global",
				NetworkInfraType: "default",
				Vlans:            "1-4094",
				Vxlans:           "1-10000",
			}), IsNil)
			time.Sleep(60 * time.Second)
			s.testNetworkAddDeleteTenant(c, "vxlan", "bridge")
			fwdMode = "bridge"
		} else {
			c.Assert(s.cli.GlobalPost(&client.Global{FwdMode: "routing",
				Name:             "global",
				NetworkInfraType: "default",
				Vlans:            "100-2094",
				Vxlans:           "1-10000",
			}), IsNil)
			time.Sleep(60 * time.Second)
			s.testNetworkAddDeleteTenant(c, "vxlan", "routing")
			fwdMode = "routing"
		}
	}
}

func (s *systemtestSuite) TestNetworkAddDeleteTenantFwdModeChangeVLAN(c *C) {

	c.Skip("Skipping this tests temporarily")

	if s.fwdMode != "routing" {
		return
	}

	for i := 0; i < s.iterations; i++ {
		s.testNetworkAddDeleteTenant(c, "vlan", s.fwdMode)
		c.Assert(s.cli.GlobalPost(&client.Global{FwdMode: "bridge",
			Name:             "global",
			NetworkInfraType: "default",
			Vlans:            "1-4094",
			Vxlans:           "1-10000",
		}), IsNil)
		time.Sleep(60 * time.Second)
		c.Assert(s.cli.GlobalPost(&client.Global{FwdMode: "routing",
			Name:             "global",
			NetworkInfraType: "default",
			Vlans:            "1-4094",
			Vxlans:           "1-10000",
		}), IsNil)
		time.Sleep(60 * time.Second)
	}
}
