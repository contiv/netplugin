package systemtests

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	. "github.com/contiv/check"
	"github.com/contiv/netplugin/contivmodel/client"
)

var providerIndex int

/*TestServiceAddDeleteService does the following:
1) Creates networks in each tenant
2) Creates Service Networks in each tenant
3) Runs containers (consumers) in each network per tenant
4) Creates Services
5) Creates Providers under each service
6) Checks reachability to service ip from consumer containers
*/
func (s *systemtestSuite) TestServiceAddDeleteServiceVxlan(c *C) {
	s.testServiceAddDeleteService(c, "vxlan")
}

func (s *systemtestSuite) TestServiceAddDeleteServiceVlan(c *C) {
	s.testServiceAddDeleteService(c, "vlan")
}

func (s *systemtestSuite) testServiceAddDeleteService(c *C, encap string) {

	mutex := sync.Mutex{}

	if s.fwdMode == "routing" && encap == "vlan" {
		s.SetupBgp(c, false)
		s.CheckBgpConnection(c)
	}

	for i := 0; i < s.basicInfo.Iterations; i++ {
		var (
			netNames          = []string{}
			containers        = map[string][]*container{}
			serviceContainers = map[string][]*container{}
			services          = map[string][]*client.ServiceLB{}
			serviceIPs        = map[string][]string{}
			serviceNetworks   = map[string][]string{}
			tenantNames       = map[string][]string{}
			servicesPerTenant = map[string]map[string][]*container{}
		)

		numContainer := s.basicInfo.Containers
		if numContainer < 4 {
			numContainer = 4
		}

		numSvcNet := 1 //numContainer / len(s.nodes)
		numLabels := 4
		numSvcs := 1
		numTenant := 1 //numContainer / len(s.nodes)

		for tenantNum := 0; tenantNum < numTenant; tenantNum++ {
			tenantName := fmt.Sprintf("tenant%d", tenantNum)
			if tenantNum == 0 {
				tenantName = "default"
			} else {
				c.Assert(s.cli.TenantPost(&client.Tenant{TenantName: tenantName}), IsNil)
			}
			tenantNames[tenantName] = []string{}

			for networkNum := 0; networkNum < numContainer/len(s.nodes); networkNum++ {

				network := &client.Network{
					TenantName:  tenantName,
					NetworkName: fmt.Sprintf("net%d-%d", networkNum, i),
					Subnet:      fmt.Sprintf("10.%d.%d.0/24", tenantNum, networkNum),
					Gateway:     fmt.Sprintf("10.%d.%d.254", tenantNum, networkNum),
					Encap:       encap,
				}
				logrus.Infof("Creating network %s on tenant %s", network.NetworkName, network.TenantName)

				c.Assert(s.cli.NetworkPost(network), IsNil)
				netNames = append(netNames, network.NetworkName)
				tenantNames[tenantName] = append(tenantNames[tenantName], network.NetworkName)
			}

			serviceNetworks[tenantName] = s.createServiceNetworks(c, i, numSvcNet, tenantName, encap)
		}
		time.Sleep(6 * time.Second)
		for tenant, networks := range tenantNames {
			endChan := make(chan error)
			for _, network := range networks {
				go func(network, tenant string, containers map[string][]*container) {
					net := network
					if tenant != "default" {
						net = network + "/" + tenant
					}
					var err error
					mutex.Lock()
					containers[network+tenant], err = s.runContainers(numContainer, false, net, "", nil, nil)
					mutex.Unlock()
					endChan <- err
				}(network, tenant, containers)
			}
		}
		if s.fwdMode == "routing" && encap == "vlan" {
			for _, cList := range containers {
				err := s.CheckBgpRouteDistribution(c, cList)
				c.Assert(err, IsNil)
			}
		}

		for tenant, networks := range serviceNetworks {
			ips := []string{}
			sv := []*client.ServiceLB{}
			for _, network := range networks {
				sv, ips = s.createServices(c, numSvcs, tenant, network, numLabels)
				services[tenant] = append(services[tenant], sv...)
				serviceIPs[tenant] = append(serviceIPs[tenant], ips...)
			}
		}

		for tenant, service := range services {
			for _, network := range tenantNames[tenant] {
				for _, svc := range service {
					serviceContainers[svc.ServiceName] = append(serviceContainers[svc.ServiceName],
						s.addProviders(c, svc.Selectors, numContainer, tenant, network, encap)...)
				}
				servicesPerTenant[tenant] = serviceContainers
			}
		}

		for tenant, serviceContainers := range servicesPerTenant {
			for serviceName, svcContainers := range serviceContainers {
				serviceInfo := &client.ServiceLB{}
				for _, service := range services[tenant] {
					if service.ServiceName == serviceName {
						serviceInfo = service
					}
				}
				s.startListenersOnProviders(svcContainers, serviceInfo.Ports)
			}
		}

		for _, ips := range serviceIPs {
			endChan := make(chan error)
			for _, conts := range containers {
				go func(c *C, conts []*container, ip []string, port int, protocol string) {
					endChan <- s.checkConnectionToService(conts, ips, port, "tcp")
				}(c, conts, ips, 80, "tcp")
			}

			for range serviceIPs {
				for range containers {
					c.Assert(<-endChan, IsNil)
				}
			}
		}

		for _, serviceContainers := range servicesPerTenant {
			for _, containers := range serviceContainers {
				s.deleteProviders(c, containers)
			}
		}

		for tenant, serviceList := range services {
			s.deleteServices(c, tenant, serviceList)
		}
		for tenant, networks := range serviceNetworks {
			s.deleteServiceNetworks(c, tenant, networks)
		}

		for tenant, networks := range tenantNames {
			for _, network := range networks {
				c.Assert(s.removeContainers(containers[network+tenant]), IsNil)
				c.Assert(s.cli.NetworkDelete(tenant, network), IsNil)
			}
		}
	}
	if encap == "vlan" && s.fwdMode == "routing" {
		s.TearDownBgp(c)
	}
}

/*TestServiceAddDeleteProviders does the following:
1) Creates networks in each tenant
2) Creates Service Networks in each tenant
3) Runs containers (consumers) in each network per tenant
4) Creates Services
5) Adds and delete service provider containers and checks service ip reachability
*/

func (s *systemtestSuite) TestServiceAddDeleteProvidersVxlan(c *C) {
	s.testServiceAddDeleteProviders(c, "vxlan")
}

func (s *systemtestSuite) TestServiceAddDeleteProvidersVlan(c *C) {
	s.testServiceAddDeleteProviders(c, "vlan")
}

func (s *systemtestSuite) testServiceAddDeleteProviders(c *C, encap string) {

	mutex := sync.Mutex{}
	if s.fwdMode == "routing" && encap == "vlan" {
		s.SetupBgp(c, false)
		s.CheckBgpConnection(c)
	}

	for i := 0; i < s.basicInfo.Iterations; i++ {
		var (
			netNames          = []string{}
			containers        = map[string][]*container{}
			serviceContainers = map[string][]*container{}
			services          = map[string][]*client.ServiceLB{}
			serviceIPs        = map[string][]string{}
			serviceNetworks   = map[string][]string{}
			tenantNames       = map[string][]string{}
			servicesPerTenant = map[string]map[string][]*container{}
		)

		numContainer := s.basicInfo.Containers
		if numContainer < 4 {
			numContainer = 4
		}

		numSvcNet := 1 //numContainer / len(s.nodes)
		numLabels := 4
		numSvcs := 1
		numTenant := 1 //numContainer / len(s.nodes)

		for tenantNum := 0; tenantNum < numTenant; tenantNum++ {
			tenantName := fmt.Sprintf("tenant%d", tenantNum)
			if tenantNum == 0 {
				tenantName = "default"
			} else {
				c.Assert(s.cli.TenantPost(&client.Tenant{TenantName: tenantName}), IsNil)
			}
			tenantNames[tenantName] = []string{}

			for networkNum := 0; networkNum < numContainer/len(s.nodes); networkNum++ {

				network := &client.Network{
					TenantName:  tenantName,
					NetworkName: fmt.Sprintf("net%d-%d", networkNum, i),
					Subnet:      fmt.Sprintf("10.%d.%d.0/24", tenantNum, networkNum),
					Gateway:     fmt.Sprintf("10.%d.%d.254", tenantNum, networkNum),
					Encap:       encap,
				}
				logrus.Infof("Creating network %s on tenant %s", network.NetworkName, network.TenantName)

				c.Assert(s.cli.NetworkPost(network), IsNil)
				netNames = append(netNames, network.NetworkName)
				tenantNames[tenantName] = append(tenantNames[tenantName], network.NetworkName)
			}
			serviceNetworks[tenantName] = s.createServiceNetworks(c, i, numSvcNet, tenantName, encap)
		}
		for tenant, networks := range tenantNames {
			endChan := make(chan error)
			for _, network := range networks {
				go func(network, tenant string, containers map[string][]*container) {
					net := network
					if tenant != "default" {
						net = network + "/" + tenant
					}
					var err error
					mutex.Lock()
					containers[network+":"+tenant], err = s.runContainers(numContainer, false, net, "", nil, nil)
					mutex.Unlock()
					endChan <- err
				}(network, tenant, containers)
			}
		}

		if s.fwdMode == "routing" && encap == "vlan" {
			for _, cList := range containers {
				err := s.CheckBgpRouteDistribution(c, cList)
				c.Assert(err, IsNil)
			}
		}
		for tenant, networks := range serviceNetworks {
			ips := []string{}
			for _, network := range networks {
				services[tenant], ips = s.createServices(c, numSvcs, tenant, network, numLabels)
				serviceIPs[tenant] = append(serviceIPs[tenant], ips...)
			}
		}

		for tenant, service := range services {
			for _, svc := range service {
				serviceContainers[svc.ServiceName] = s.addProviders(c, svc.Selectors, numContainer, tenant, svc.NetworkName, encap)
			}
			servicesPerTenant[tenant] = serviceContainers
		}

		for tenant, serviceContainers := range servicesPerTenant {
			for serviceName, svcContainers := range serviceContainers {
				serviceInfo := &client.ServiceLB{}
				for _, service := range services[tenant] {
					if service.ServiceName == serviceName {
						serviceInfo = service
					}
				}
				s.startListenersOnProviders(svcContainers, serviceInfo.Ports)
			}
		}

		for tenant, service := range services {
			for _, svc := range service {
				endChan := make(chan error)
				for iter := 0; iter < s.basicInfo.Iterations; iter++ {
					for tenantNetwork, conts := range containers {
						contTenant := "default"
						if strings.Contains(tenantNetwork, ":") {
							contTenant = strings.Split(tenantNetwork, ":")[1]
							if contTenant != tenant {
								continue
							}
						}
						go func(c *C, conts []*container, ips []string, port int, protocol string) {
							endChan <- s.checkConnectionToService(conts, ips, port, "tcp")
						}(c, conts, serviceIPs[tenant], 80, "tcp")
					}

					for range containers {
						c.Assert(<-endChan, IsNil)
					}
					numSvcContainers := len(serviceContainers[svc.ServiceName])
					s.deleteProviders(c, serviceContainers[svc.ServiceName][:numSvcContainers/2])
					serviceContainers[svc.ServiceName] = serviceContainers[svc.ServiceName][numSvcContainers/2:]
					servicesPerTenant[tenant] = serviceContainers

					for tenantNetwork, conts := range containers {
						contTenant := "default"
						if strings.Contains(tenantNetwork, ":") {
							contTenant = strings.Split(tenantNetwork, ":")[1]
							if contTenant != tenant {
								continue
							}
						}
						go func(c *C, conts []*container, ips []string, port int, protocol string) {
							endChan <- s.checkConnectionToService(conts, ips, port, "tcp")
						}(c, conts, serviceIPs[tenant], 80, "tcp")
					}

					for range containers {
						c.Assert(<-endChan, IsNil)
					}
					serviceContainers[svc.ServiceName] = append(serviceContainers[svc.ServiceName],
						s.addProviders(c, svc.Selectors, numContainer, tenant, svc.NetworkName, encap)...)
					servicesPerTenant[tenant] = serviceContainers

					s.startListenersOnProviders(serviceContainers[svc.ServiceName], svc.Ports)

				}
			}
		}
		for _, serviceContainers := range servicesPerTenant {
			for _, containers := range serviceContainers {
				s.deleteProviders(c, containers)
			}
		}

		for tenant, serviceList := range services {
			s.deleteServices(c, tenant, serviceList)
		}
		for tenant, networks := range serviceNetworks {
			s.deleteServiceNetworks(c, tenant, networks)
		}

		for _, conts := range containers {
			c.Assert(s.removeContainers(conts), IsNil)
		}
		for tenant, networks := range tenantNames {
			for _, network := range networks {
				c.Assert(s.cli.NetworkDelete(tenant, network), IsNil)
			}
		}
	}
	if encap == "vlan" && s.fwdMode == "routing" {
		s.TearDownBgp(c)
	}
}

/*TestServiceAddDeleteProviders does the following:
1) Creates networks in each tenant
2) Creates Service Networks in each tenant
3) Runs containers (consumers) in each network per tenant
4) Creates Service
5) Adds Providers with labels
6) Restarts netmaster one by one on every node and creates/deletes service,providers
and verfies the reachability.
*/

func (s systemtestSuite) TestServiceTriggerNetmasterSwitchoverVxlan(c *C) {
	s.testServiceTriggerNetmasterSwitchover(c, "vxlan")
}
func (s systemtestSuite) TestServiceTriggerNetmasterSwitchoverVlan(c *C) {
	s.testServiceTriggerNetmasterSwitchover(c, "vlan")
}

func (s systemtestSuite) testServiceTriggerNetmasterSwitchover(c *C, encap string) {

	if s.fwdMode == "routing" && encap == "vlan" {
		s.SetupBgp(c, false)
		s.CheckBgpConnection(c)
	}

	mutex := sync.Mutex{}

	for i := 0; i < s.basicInfo.Iterations; i++ {
		var (
			netNames          = []string{}
			containers        = map[string][]*container{}
			serviceContainers = map[string][]*container{}
			services          = map[string][]*client.ServiceLB{}
			serviceIPs        = map[string][]string{}
			serviceNetworks   = map[string][]string{}
			tenantNames       = map[string][]string{}
			servicesPerTenant = map[string]map[string][]*container{}
		)

		var leader, oldLeader *node

		numContainer := s.basicInfo.Containers
		if numContainer < 4 {
			numContainer = 4
		}

		numSvcNet := 1 //numContainer / len(s.nodes)
		numLabels := 4
		numSvcs := 1
		numTenant := 1 //numContainer / len(s.nodes)

		for tenantNum := 0; tenantNum < numTenant; tenantNum++ {
			tenantName := fmt.Sprintf("tenant%d", tenantNum)
			if tenantNum == 0 {
				tenantName = "default"
			} else {
				c.Assert(s.cli.TenantPost(&client.Tenant{TenantName: tenantName}), IsNil)
			}
			tenantNames[tenantName] = []string{}

			for networkNum := 0; networkNum < numContainer/len(s.nodes); networkNum++ {
				network := &client.Network{
					TenantName:  tenantName,
					NetworkName: fmt.Sprintf("net%d-%d", networkNum, i),
					Subnet:      fmt.Sprintf("10.%d.%d.0/24", tenantNum, networkNum),
					Gateway:     fmt.Sprintf("10.%d.%d.254", tenantNum, networkNum),
					Encap:       encap,
				}

				logrus.Infof("Creating network %s on tenant %s", network.NetworkName, network.TenantName)

				c.Assert(s.cli.NetworkPost(network), IsNil)
				netNames = append(netNames, network.NetworkName)
				tenantNames[tenantName] = append(tenantNames[tenantName], network.NetworkName)
			}
			serviceNetworks[tenantName] = s.createServiceNetworks(c, i, numSvcNet, tenantName, encap)
		}

		for tenant, networks := range tenantNames {
			endChan := make(chan error)
			for _, network := range networks {
				go func(network, tenant string, containers map[string][]*container) {
					net := network
					if tenant != "default" {
						net = network + "/" + tenant
					}
					var err error
					mutex.Lock()
					c, err := s.runContainers(numContainer, false, net, "", nil, nil)
					containers[network+tenant] = append(containers[network+tenant], c...)
					mutex.Unlock()
					endChan <- err
				}(network, tenant, containers)
			}
		}
		if s.fwdMode == "routing" && encap == "vlan" {
			for _, cList := range containers {
				err := s.CheckBgpRouteDistribution(c, cList)
				c.Assert(err, IsNil)
			}
		}
		for x := 0; x < s.basicInfo.Iterations; x++ {

			for tenant, networks := range serviceNetworks {
				ips := []string{}
				sv := []*client.ServiceLB{}
				for _, network := range networks {
					sv, ips = s.createServices(c, numSvcs, tenant, network, numLabels)
					services[tenant] = append(services[tenant], sv...)
					serviceIPs[tenant] = append(serviceIPs[tenant], ips...)
				}
			}
			for tenant, service := range services {
				for _, network := range tenantNames[tenant] {
					for _, svc := range service {
						serviceContainers[svc.ServiceName] = append(serviceContainers[svc.ServiceName],
							s.addProviders(c, svc.Selectors, numContainer, tenant, network, encap)...)
					}
					servicesPerTenant[tenant] = serviceContainers
				}
			}

			for tenant, serviceContainers := range servicesPerTenant {
				for serviceName, svcContainers := range serviceContainers {
					serviceInfo := &client.ServiceLB{}
					for _, service := range services[tenant] {
						if service.ServiceName == serviceName {
							serviceInfo = service
						}
					}
					s.startListenersOnProviders(svcContainers, serviceInfo.Ports)
				}
			}
			for _, ips := range serviceIPs {
				endChan := make(chan error)
				for _, conts := range containers {
					go func(c *C, conts []*container, ip []string, port int, protocol string) {
						endChan <- s.checkConnectionToService(conts, ips, port, "tcp")
					}(c, conts, ips, 80, "tcp")
				}

				for range serviceIPs {
					for range containers {
						c.Assert(<-endChan, IsNil)
					}
				}
			}

			leaderURL, err := s.clusterStoreGet("/contiv.io/lock/netmaster/leader")
			c.Assert(err, IsNil)

			leaderIP := strings.Split(leaderURL, ":")[0]

			for _, node := range s.nodes {
				res, err := node.getIPAddr("eth1")
				c.Assert(err, IsNil)
				if res == leaderIP {
					leader = node
					leaderIP = res
					logrus.Infof("Found leader %s/%s", node.Name(), leaderIP)
				}
			}

			c.Assert(leader.stopNetmaster(), IsNil)
			c.Assert(leader.rotateLog("netmaster"), IsNil)

			for x := 0; x < 15; x++ {
				logrus.Info("Waiting 5s for leader to change...")
				newLeaderURL, err := s.clusterStoreGet("/contiv.io/lock/netmaster/leader")
				c.Assert(err, IsNil)

				newLeaderIP := strings.Split(newLeaderURL, ":")[0]

				for _, node := range s.nodes {
					res, err := node.getIPAddr("eth1")
					c.Assert(err, IsNil)
					if res == newLeaderIP && res != leaderIP {
						oldLeader = leader
						leader = node
						logrus.Infof("Leader switched to %s/%s", node.Name(), newLeaderIP)
						goto finished
					}
				}

				time.Sleep(5 * time.Second)
			}
		finished:
			c.Assert(oldLeader.startNetmaster(""), IsNil)
			time.Sleep(10 * time.Second)

			for tenant, serviceContainers := range servicesPerTenant {
				for name, containers := range serviceContainers {
					s.deleteProviders(c, containers)
					serviceContainers[name] = nil
				}
				servicesPerTenant[tenant] = nil
			}

			for tenant, serviceList := range services {
				s.deleteServices(c, tenant, serviceList)
				services[tenant] = nil
			}
		}

		for tenant, networks := range serviceNetworks {
			s.deleteServiceNetworks(c, tenant, networks)
		}

		for tenant, networks := range tenantNames {
			for _, network := range networks {
				c.Assert(s.removeContainers(containers[network+tenant]), IsNil)
				c.Assert(s.cli.NetworkDelete(tenant, network), IsNil)
			}
		}
	}
	if encap == "vlan" && s.fwdMode == "routing" {
		s.TearDownBgp(c)
	}
}

/*TestServiceAddDeleteProviders does the following:
1) Creates networks in each tenant
2) Creates Service Networks in each tenant
3) Runs containers (consumers) in each network per tenant
4) Creates Service
5) Adds Providers with labels
6) Restarts netplugin one by one on every node and creates/deletes service,providers
and verfies the reachability.
*/

func (s systemtestSuite) TestServiceTriggerNetpluginRestartVlan(c *C) {
	s.testServiceTriggerNetpluginRestart(c, "vlan")
}
func (s systemtestSuite) TestServiceTriggerNetpluginRestartVxlan(c *C) {
	s.testServiceTriggerNetpluginRestart(c, "vxlan")
}

func (s systemtestSuite) testServiceTriggerNetpluginRestart(c *C, encap string) {

	mutex := sync.Mutex{}
	if s.fwdMode == "routing" && encap == "vlan" {
		s.SetupBgp(c, false)
		s.CheckBgpConnection(c)
	}

	for i := 0; i < s.basicInfo.Iterations; i++ {
		var (
			netNames          = []string{}
			containers        = map[string][]*container{}
			serviceContainers = map[string][]*container{}
			services          = map[string][]*client.ServiceLB{}
			serviceIPs        = map[string][]string{}
			serviceNetworks   = map[string][]string{}
			tenantNames       = map[string][]string{}
			servicesPerTenant = map[string]map[string][]*container{}
			pktTag            = 0
		)

		numContainer := s.basicInfo.Containers
		if numContainer < 4 {
			numContainer = 4
		}

		numSvcNet := 1 //numContainer / len(s.nodes)
		numLabels := 4
		numSvcs := 1
		numTenant := 1 //numContainer / len(s.nodes)

		for tenantNum := 0; tenantNum < numTenant; tenantNum++ {
			tenantName := fmt.Sprintf("tenant%d", tenantNum)
			if tenantNum == 0 {
				tenantName = "default"
			} else {
				c.Assert(s.cli.TenantPost(&client.Tenant{TenantName: tenantName}), IsNil)
			}
			tenantNames[tenantName] = []string{}

			for networkNum := 0; networkNum < numContainer/len(s.nodes); networkNum++ {
				network := &client.Network{
					TenantName:  tenantName,
					NetworkName: fmt.Sprintf("net%d-%d", networkNum, i),
					Subnet:      fmt.Sprintf("10.%d.%d.0/24", tenantNum, networkNum),
					Gateway:     fmt.Sprintf("10.%d.%d.254", tenantNum, networkNum),
					Encap:       encap,
					PktTag:      pktTag + 1000,
				}

				logrus.Infof("Creating network %s on tenant %s", network.NetworkName, network.TenantName)

				c.Assert(s.cli.NetworkPost(network), IsNil)
				netNames = append(netNames, network.NetworkName)
				tenantNames[tenantName] = append(tenantNames[tenantName], network.NetworkName)
				pktTag++
			}
			serviceNetworks[tenantName] = s.createServiceNetworks(c, i, numSvcNet, tenantName, encap)
		}

		for tenant, networks := range tenantNames {
			endChan := make(chan error)
			for _, network := range networks {
				go func(network, tenant string, containers map[string][]*container) {
					net := network
					if tenant != "default" {
						net = network + "/" + tenant
					}
					var err error
					mutex.Lock()
					c, err := s.runContainers(numContainer, false, net, "", nil, nil)
					containers[network+tenant] = append(containers[network+tenant], c...)
					mutex.Unlock()
					endChan <- err
				}(network, tenant, containers)
			}
		}
		if s.fwdMode == "routing" && encap == "vlan" {
			for _, cList := range containers {
				err := s.CheckBgpRouteDistribution(c, cList)
				c.Assert(err, IsNil)
			}
		}

		for _, node := range s.nodes {
			for tenant, networks := range serviceNetworks {
				ips := []string{}
				sv := []*client.ServiceLB{}
				for _, network := range networks {
					sv, ips = s.createServices(c, numSvcs, tenant, network, numLabels)
					services[tenant] = append(services[tenant], sv...)
					serviceIPs[tenant] = append(serviceIPs[tenant], ips...)
				}
			}

			for tenant, service := range services {
				for _, network := range tenantNames[tenant] {
					for _, svc := range service {
						serviceContainers[svc.ServiceName] = append(serviceContainers[svc.ServiceName],
							s.addProviders(c, svc.Selectors, numContainer, tenant, network, encap)...)
					}
					servicesPerTenant[tenant] = serviceContainers
				}
			}

			for tenant, serviceContainers := range servicesPerTenant {
				for serviceName, svcContainers := range serviceContainers {
					serviceInfo := &client.ServiceLB{}
					for _, service := range services[tenant] {
						if service.ServiceName == serviceName {
							serviceInfo = service
						}
					}
					s.startListenersOnProviders(svcContainers, serviceInfo.Ports)
				}
			}
			c.Assert(node.stopNetplugin(), IsNil)
			logrus.Info("Sleeping for a while to wait for netplugin's TTLs to expire")
			time.Sleep(2 * time.Minute)
			c.Assert(node.rotateLog("netplugin"), IsNil)
			c.Assert(node.startNetplugin(""), IsNil)
			c.Assert(node.exec.runCommandUntilNoNetpluginError(), IsNil)
			time.Sleep(20 * time.Second)
			if s.fwdMode == "routing" && encap == "vlan" {
				s.CheckBgpConnection(c)
				for _, cList := range containers {
					err := s.CheckBgpRouteDistribution(c, cList)
					c.Assert(err, IsNil)
				}
			} else {
				c.Assert(s.verifyVTEPs(), IsNil)
				time.Sleep(2 * time.Second)
			}
			for _, ips := range serviceIPs {
				endChan := make(chan error)
				for _, conts := range containers {
					go func(c *C, conts []*container, ip []string, port int, protocol string) {
						endChan <- s.checkConnectionToService(conts, ips, port, "tcp")
					}(c, conts, ips, 80, "tcp")
				}
				for range containers {
					c.Assert(<-endChan, IsNil)
				}
			}
			for tenant, serviceContainers := range servicesPerTenant {
				for name, containers := range serviceContainers {
					s.deleteProviders(c, containers)
					serviceContainers[name] = nil
				}
				servicesPerTenant[tenant] = nil
			}

			for tenant, serviceList := range services {
				s.deleteServices(c, tenant, serviceList)
				services[tenant] = nil
			}
		}

		for tenant, networks := range serviceNetworks {
			s.deleteServiceNetworks(c, tenant, networks)
		}

		for tenant, networks := range tenantNames {
			for _, network := range networks {
				c.Assert(s.removeContainers(containers[network+tenant]), IsNil)
				c.Assert(s.cli.NetworkDelete(tenant, network), IsNil)
			}
			if tenant != "default" {
				c.Assert(s.cli.TenantDelete(tenant), IsNil)
			}
		}
	}
	if encap == "vlan" && s.fwdMode == "routing" {
		s.TearDownBgp(c)
	}
}

func (s *systemtestSuite) createServiceNetworks(c *C, i int, numNets int, tenant, encap string) []string {

	networks := []string{}
	for count := 0; count < numNets; count++ {
		network := &client.Network{
			TenantName:  tenant,
			NetworkName: fmt.Sprintf("svc-net-%d-%d", i, count),
			Subnet:      fmt.Sprintf("30.%d.%d.0/24", i, count),
			Gateway:     fmt.Sprintf("30.%d.%d.254", i, count),
			Encap:       encap,
		}
		logrus.Infof("Creating service Network %s on tenant %s", network.NetworkName, tenant)
		c.Assert(s.cli.NetworkPost(network), IsNil)
		networks = append(networks, network.NetworkName)
	}

	return networks
}

func (s *systemtestSuite) createServices(c *C, numServices int, tenant string, svcNetwork string, numLabels int) ([]*client.ServiceLB, []string) {

	services := []*client.ServiceLB{}
	serviceIPs := []string{}

	for serviceNum := 0; serviceNum < numServices; serviceNum++ {
		service := &client.ServiceLB{
			ServiceName: fmt.Sprintf("svc-%d-%s", serviceNum, tenant),
			TenantName:  tenant,
			NetworkName: svcNetwork,
			Ports:       []string{"80:8080:TCP", "643:7070:UDP"},
		}
		for index := serviceNum; index < serviceNum+numLabels; index++ {
			service.Selectors = append(service.Selectors, fmt.Sprintf("key%d=value%d", index, index+1))
		}

		c.Assert(s.cli.ServiceLBPost(service), IsNil)
		logrus.Infof("Creating service %s tenant %s on network %s with label %v", service.ServiceName, tenant, svcNetwork, service.Selectors)
		services = append(services, service)

		// Get service IP
		svcInspect, err := s.cli.ServiceLBInspect(tenant, service.ServiceName)
		c.Assert(err, IsNil)
		serviceIPs = append(serviceIPs, svcInspect.Oper.ServiceVip)

	}

	return services, serviceIPs
}

func (s *systemtestSuite) addProviders(c *C, labels []string, numProviders int, tenant string, netName string, encap string) []*container {

	containers := []*container{}
	logrus.Infof("Adding Providers with labels %v , on tenant %s , network %s", labels, tenant, netName)

	names := generateProviderNames(numProviders, netName, tenant)

	var err error

	containers, err = s.runContainers(numProviders, false, netName, tenant, names, labels)
	c.Assert(err, IsNil)
	if s.fwdMode == "routing" && encap == "vlan" {
		err = s.CheckBgpRouteDistribution(c, containers)
		c.Assert(err, IsNil)
	}
	return containers
}

func (s *systemtestSuite) deleteProviders(c *C, svcContainers []*container) {
	//c.Assert(s.removeContainers(svcContainers), IsNil)
	endChan := make(chan error)
	go func(conts []*container) { endChan <- s.removeContainers(conts) }(svcContainers)
	<-endChan
}

func (s *systemtestSuite) deleteServiceNetworks(c *C, tenant string, networks []string) {
	for _, network := range networks {
		logrus.Infof("Deleting Service networks %s on tenant %s", network, tenant)
		c.Assert(s.cli.NetworkDelete(tenant, network), IsNil)
	}
}

func (s *systemtestSuite) deleteServices(c *C, tenant string, services []*client.ServiceLB) {
	for _, service := range services {
		logrus.Infof("Deleting service %s on tenant %s , len(%d)", service.ServiceName, tenant, len(services))
		c.Assert(s.cli.ServiceLBDelete(tenant, service.ServiceName), IsNil)
	}
}

func generateProviderNames(numProviders int, network string, tenant string) []string {
	names := []string{}
	for i := 0; i < numProviders; i++ {
		names = append(names, fmt.Sprintf("srv-%s-%s-%d", tenant, network, providerIndex))
		providerIndex++
	}
	return names
}

func getLabels(numLabels, serviceNum int) []string {
	labels := []string{}

	for index := serviceNum; index < serviceNum+numLabels; index++ {
		labels = append(labels, fmt.Sprintf("key%d=value%d", index, index+1))
	}
	return labels
}
