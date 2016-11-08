package systemtests

import (
	"fmt"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/contivmodel/client"
	. "gopkg.in/check.v1"
)

func (s *systemtestSuite) TestNetprofileBasicUpdate(c *C) {
	doneChan := make(chan struct{}, 2)

	go s.testNetprofileBasicUpdate(c, EncapVXLAN, doneChan, 0)
	go s.testNetprofileBasicUpdate(c, EncapVLAN, doneChan, 1)

	for i := 0; i < 2; i++ {
		<-doneChan
	}
}

//func testNetprofileBasicUpdate will check :
//1)run containers with a group which does not have any netprofile attached to it,
//and check if the iperf client errors out when you give a dummy limit.
//check if the tc disc show - rate matches with the limit.
//attach the groups to a netprofile and check iperf.
func (s *systemtestSuite) testNetprofileBasicUpdate(c *C, encap string, doneChan chan struct{}, seq int) {
	defer func() { doneChan <- struct{}{} }()

	s.SetupBgp(c, encap, false)
	s.CheckBgpConnection(c, encap)

	tenantName := fmt.Sprintf("tenant-%d", seq)

	tenant := &client.Tenant{TenantName: tenantName}
	c.Assert(s.cli.TenantPost(tenant), IsNil)
	defer s.cli.TenantDelete(tenantName)

	network := &client.Network{
		TenantName:  tenantName,
		NetworkName: encap,
		Subnet:      fmt.Sprintf("10.1.%d.1/24", seq),
		Gateway:     fmt.Sprintf("10.1.%d.254", seq),
		PktTag:      1001 + seq,
		Encap:       encap,
	}

	c.Assert(s.cli.NetworkPost(network), IsNil)

	groups := []*client.EndpointGroup{}
	groupNames := []string{}
	for x := 0; x < s.basicInfo.Containers; x++ {
		nodeNum := s.basicInfo.Containers % len(s.nodes)
		epgName := fmt.Sprintf("%s-srv%d-%d", network.NetworkName, nodeNum, x)
		group := &client.EndpointGroup{
			GroupName:   epgName,
			NetworkName: encap,
			TenantName:  tenantName,
		}
		c.Assert(s.cli.EndpointGroupPost(group), IsNil)

		groups = append(groups, group)
		groupNames = append(groupNames, epgName)
	}

	containers, err := s.runContainers(s.basicInfo.Containers, true, encap, tenantName, groupNames, nil)
	c.Assert(err, IsNil)
	if s.fwdMode == FwdModeRouting && encap == EncapVLAN {
		err = s.CheckBgpRouteDistribution(c, containers)
		c.Assert(err, IsNil)
	}

	c.Assert(s.startIperfServers(containers), IsNil)
	c.Assert(s.startIperfClients(containers, "8Mbps", true), IsNil)

	netProfile := &client.Netprofile{
		ProfileName: "Netprofile",
		DSCP:        6,
		Bandwidth:   "6Mbps",
		Burst:       80,
		TenantName:  tenantName,
	}

	c.Assert(s.cli.NetprofilePost(netProfile), IsNil)

	groups = []*client.EndpointGroup{}
	groupNames = []string{}
	for x := 0; x < s.basicInfo.Containers; x++ {
		nodeNum := s.basicInfo.Containers % len(s.nodes)
		epgName := fmt.Sprintf("%s-srv%d-%d", network.NetworkName, nodeNum, x)
		group := &client.EndpointGroup{
			GroupName:   epgName,
			NetworkName: encap,
			NetProfile:  "Netprofile",
			TenantName:  tenantName,
		}
		c.Assert(s.cli.EndpointGroupPost(group), IsNil)

		groups = append(groups, group)
		groupNames = append(groupNames, epgName)
	}
	c.Assert(s.checkIngressRate(containers, netProfile.Bandwidth), IsNil)
	c.Assert(s.startIperfClients(containers, netProfile.Bandwidth, false), IsNil)

	netProfile = &client.Netprofile{
		ProfileName: "Netprofile",
		DSCP:        6,
		Bandwidth:   "16Mbps",
		Burst:       270,
		TenantName:  tenantName,
	}

	c.Assert(s.cli.NetprofilePost(netProfile), IsNil)

	groups = []*client.EndpointGroup{}
	groupNames = []string{}
	for x := 0; x < s.basicInfo.Containers; x++ {
		nodeNum := s.basicInfo.Containers % len(s.nodes)
		epgName := fmt.Sprintf("%s-srv%d-%d", network.NetworkName, nodeNum, x)
		group := &client.EndpointGroup{
			GroupName:   epgName,
			NetworkName: encap,
			NetProfile:  "Netprofile",
			TenantName:  tenantName,
		}
		c.Assert(s.cli.EndpointGroupPost(group), IsNil)

		groups = append(groups, group)
		groupNames = append(groupNames, epgName)
	}

	c.Assert(s.checkIngressRate(containers, netProfile.Bandwidth), IsNil)
	c.Assert(s.startIperfClients(containers, netProfile.Bandwidth, false), IsNil)

	c.Assert(s.removeContainers(containers), IsNil)

	for _, group := range groups {
		c.Assert(s.cli.EndpointGroupDelete(group.TenantName, group.GroupName), IsNil)
	}
	c.Assert(s.cli.NetprofileDelete(tenantName, "Netprofile"), IsNil)

	c.Assert(s.cli.NetworkDelete(tenantName, encap), IsNil)
}

//This function checks for various updates like:
//1) attach a netprofile to the group- eg:-g1, then update the netprofile and check iperf -c
//2) detach the netprofile from group-g1 and check if the limit is not set anymore(which means any amount of bandwidth is allowed.)
//3)Delete the netprofile to see when no groups are attached to a netprofile, it can be deleted.
//4) create another netprofile, attach the same group-g1 to it to check if no stale state is present.
//5)make the netprofile.Bandwidth ="" and check if group also has no limit.
func (s *systemtestSuite) TestNetprofileUpdateBandwidth(c *C) {
	doneChan := make(chan struct{}, 2)

	go s.testNetprofileUpdateBandwidth(c, EncapVXLAN, doneChan, 0)
	go s.testNetprofileUpdateBandwidth(c, EncapVLAN, doneChan, 1)

	for i := 0; i < 2; i++ {
		<-doneChan
	}
}

func (s *systemtestSuite) testNetprofileUpdateBandwidth(c *C, encap string, doneChan chan struct{}, seq int) {
	defer func() { doneChan <- struct{}{} }()

	s.SetupBgp(c, encap, false)
	s.CheckBgpConnection(c, encap)

	tenantName := fmt.Sprintf("tenant-%d", seq)

	tenant := &client.Tenant{TenantName: tenantName}
	c.Assert(s.cli.TenantPost(tenant), IsNil)
	defer s.cli.TenantDelete(tenantName)

	syncChan := make(chan struct{}, s.basicInfo.Iterations)

	for i := 0; i < s.basicInfo.Iterations; i++ {
		go func(i int) {
			defer func() { syncChan <- struct{}{} }()

			network := &client.Network{
				TenantName:  tenantName,
				NetworkName: fmt.Sprintf("%s-%d-%d", encap, i, seq),
				Subnet:      fmt.Sprintf("10.%d.%d.1/24", seq, i),
				Gateway:     fmt.Sprintf("10.%d.%d.254", seq, i),
				PktTag:      1001 + 10*(1+seq)*(1+i),
				Encap:       encap,
			}
			c.Assert(s.cli.NetworkPost(network), IsNil)

			np1Name := fmt.Sprintf("Netprofile1-%d-%d-%s", i, seq, encap)
			np2Name := fmt.Sprintf("Netprofile2-%d-%d-%s", i, seq, encap)

			netProfile1 := &client.Netprofile{
				ProfileName: np1Name,
				DSCP:        10,
				Bandwidth:   "10Mbps",
				Burst:       180,
				TenantName:  tenantName,
			}

			c.Assert(s.cli.NetprofilePost(netProfile1), IsNil)

			groups := []*client.EndpointGroup{}
			groupNames := []string{}
			for x := 0; x < s.basicInfo.Containers; x++ {
				nodeNum := s.basicInfo.Containers % len(s.nodes)
				epgName := fmt.Sprintf("%s-srv%d-%d-%d", network.NetworkName, nodeNum, x, i)
				group := &client.EndpointGroup{
					GroupName:   epgName,
					NetworkName: network.NetworkName,
					NetProfile:  np1Name,
					TenantName:  tenantName,
				}
				c.Assert(s.cli.EndpointGroupPost(group), IsNil)

				groups = append(groups, group)
				groupNames = append(groupNames, epgName)
			}

			containers, err := s.runContainers(s.basicInfo.Containers, true, network.NetworkName, tenantName, groupNames, nil)
			c.Assert(err, IsNil)
			if s.fwdMode == FwdModeRouting && encap == EncapVLAN {
				err = s.CheckBgpRouteDistribution(c, containers)
				c.Assert(err, IsNil)
			}

			c.Assert(s.startIperfServers(containers), IsNil)
			c.Assert(s.checkIngressRate(containers, netProfile1.Bandwidth), IsNil)
			c.Assert(s.startIperfClients(containers, netProfile1.Bandwidth, false), IsNil)

			netProfile1 = &client.Netprofile{
				ProfileName: np1Name,
				DSCP:        6,
				Bandwidth:   "18Mb",
				Burst:       320,
				TenantName:  tenantName,
			}

			c.Assert(s.cli.NetprofilePost(netProfile1), IsNil)
			c.Assert(s.checkIngressRate(containers, netProfile1.Bandwidth), IsNil)
			c.Assert(s.startIperfClients(containers, netProfile1.Bandwidth, false), IsNil)

			groups = []*client.EndpointGroup{}
			groupNames = []string{}
			for x := 0; x < s.basicInfo.Containers; x++ {
				nodeNum := s.basicInfo.Containers % len(s.nodes)
				epgName := fmt.Sprintf("%s-srv%d-%d-%d", network.NetworkName, nodeNum, x, i)
				group := &client.EndpointGroup{
					GroupName:   epgName,
					NetworkName: network.NetworkName,
					NetProfile:  "",
					TenantName:  tenantName,
				}
				c.Assert(s.cli.EndpointGroupPost(group), IsNil)

				groups = append(groups, group)
				groupNames = append(groupNames, epgName)
			}
			c.Assert(s.startIperfClients(containers, "", false), IsNil)

			c.Assert(s.cli.NetprofileDelete(tenantName, np1Name), IsNil)

			netProfile2 := &client.Netprofile{
				ProfileName: np2Name,
				DSCP:        10,
				Bandwidth:   "6 Mbps",
				Burst:       100,
				TenantName:  tenantName,
			}

			c.Assert(s.cli.NetprofilePost(netProfile2), IsNil)

			groups = []*client.EndpointGroup{}
			groupNames = []string{}
			for x := 0; x < s.basicInfo.Containers; x++ {
				nodeNum := s.basicInfo.Containers % len(s.nodes)
				epgName := fmt.Sprintf("%s-srv%d-%d-%d", network.NetworkName, nodeNum, x, i)
				group := &client.EndpointGroup{
					GroupName:   epgName,
					NetworkName: network.NetworkName,
					NetProfile:  np2Name,
					TenantName:  tenantName,
				}
				c.Assert(s.cli.EndpointGroupPost(group), IsNil)

				groups = append(groups, group)
				groupNames = append(groupNames, epgName)
			}
			c.Assert(s.checkIngressRate(containers, netProfile2.Bandwidth), IsNil)
			c.Assert(s.startIperfClients(containers, netProfile2.Bandwidth, false), IsNil)

			netProfile2 = &client.Netprofile{
				ProfileName: np2Name,
				DSCP:        10,
				Bandwidth:   "",
				TenantName:  tenantName,
			}

			c.Assert(s.cli.NetprofilePost(netProfile2), IsNil)
			c.Assert(s.startIperfClients(containers, netProfile2.Bandwidth, false), IsNil)

			c.Assert(s.removeContainers(containers), IsNil)

			for _, group := range groups {
				c.Assert(s.cli.EndpointGroupDelete(group.TenantName, group.GroupName), IsNil)
			}
			c.Assert(s.cli.NetprofileDelete(tenantName, np2Name), IsNil)

			c.Assert(s.cli.NetworkDelete(tenantName, network.NetworkName), IsNil)
		}(i)
	}

	for i := 0; i < s.basicInfo.Iterations; i++ {
		<-syncChan
	}
}

//TestNetprofileMultipleTenantVXLAN creates multiple tenants which has multiple networks, netprofile and groups under it.
func (s *systemtestSuite) TestNetprofileMultipleTenantVXLAN(c *C) {
	s.testNetprofileMultipleTenant(c, EncapVXLAN)
}

func (s *systemtestSuite) TestNetprofileMultipleTenantVLAN(c *C) {
	s.testNetprofileMultipleTenant(c, EncapVLAN)
}

func (s *systemtestSuite) testNetprofileMultipleTenant(c *C, encap string) {
	mutex := sync.Mutex{}
	s.SetupBgp(c, encap, false)
	s.CheckBgpConnection(c, encap)

	for i := 0; i < s.basicInfo.Iterations-2; i++ {

		var (
			groupNames = make(map[string][]string)
			groupsInNp = make(map[string][]string)
			bandwidth  = make(map[string]string)
			npTenant   = make(map[string][]string)
			networks   = make(map[string][]string)
			netName    = make(map[string]string)
			containers = map[string][]*container{}
			pktTag     = 0
			epgName    string
		)
		numContainer := s.basicInfo.Containers

		for tenantNum := 0; tenantNum < (s.basicInfo.Containers); tenantNum++ {

			tenantName := fmt.Sprintf("tenant%d", tenantNum)
			logrus.Infof("Creating %s", tenantName)
			c.Assert(s.cli.TenantPost(&client.Tenant{TenantName: tenantName}), IsNil)

			for networkNum := 0; networkNum < (numContainer - 1); networkNum++ {
				networkName := fmt.Sprintf("net%d-%s", networkNum, tenantName)
				network := &client.Network{
					TenantName:  tenantName,
					NetworkName: networkName,
					Subnet:      fmt.Sprintf("10.%d.%d.1/24", tenantNum, networkNum),
					Gateway:     fmt.Sprintf("10.%d.%d.254", tenantNum, networkNum),
					PktTag:      pktTag + 1000,
					Encap:       encap,
				}

				logrus.Infof("Creating %s with %s", network.NetworkName, network.TenantName)

				c.Assert(s.cli.NetworkPost(network), IsNil)
				networks[tenantName] = append(networks[network.TenantName], network.NetworkName)
				pktTag++

				profileName := fmt.Sprintf("netprofile%d%d-%s", networkNum, tenantNum, tenantName)
				bwInt := 10 + tenantNum + networkNum
				burst := (bwInt * 13)
				netprofile := &client.Netprofile{
					ProfileName: profileName,
					DSCP:        networkNum + i,
					Bandwidth:   fmt.Sprintf("%dMbps", bwInt),
					Burst:       burst,
					TenantName:  tenantName,
				}

				c.Assert(s.cli.NetprofilePost(netprofile), IsNil)
				logrus.Infof("Creating:%s with %s", netprofile.ProfileName, netprofile.TenantName)
				epgName = fmt.Sprintf("epg%d-%s", networkNum, networkName)
				group := &client.EndpointGroup{
					GroupName:   epgName,
					NetworkName: network.NetworkName,
					NetProfile:  netprofile.ProfileName,
					TenantName:  tenantName,
				}
				c.Assert(s.cli.EndpointGroupPost(group), IsNil)
				logrus.Infof("Creating %s with %s and %s", group.GroupName, group.NetProfile, group.TenantName)
				groupNames[tenantName] = append(groupNames[group.TenantName], group.GroupName)
				netName[epgName] = group.NetworkName
				groupsInNp[profileName] = append(groupsInNp[group.NetProfile], group.GroupName)
				bandwidth[group.NetProfile] = netprofile.Bandwidth
				npTenant[tenantName] = append(npTenant[netprofile.TenantName], netprofile.ProfileName)
			}
		}

		for tenant, groups := range groupNames {
			endChan := make(chan error)
			for _, groupName := range groups {
				go func(groupName, tenant string, netName map[string]string, containers map[string][]*container) {
					var err error
					mutex.Lock()
					logrus.Infof("Creating containers in group:%s", groupName)
					containers[groupName], err = s.runContainersInService(numContainer, groupName, netName[groupName], tenant, nil)
					mutex.Unlock()
					endChan <- err

					if s.fwdMode == FwdModeRouting && encap == EncapVLAN {
						err := s.CheckBgpRouteDistribution(c, containers[groupName])
						c.Assert(err, IsNil)
					}

				}(groupName, tenant, netName, containers)
			}
			for i := 0; i < len(groups); i++ {
				c.Assert(<-endChan, IsNil)
			}
		}

		for netprofiles, groups := range groupsInNp {
			for _, group := range groups {
				logrus.Infof("Running iperf server on %s", group)
				c.Assert(s.startIperfServers(containers[group]), IsNil)
				logrus.Infof("running iperf client on %s", group)
				c.Assert(s.startIperfClients(containers[group], bandwidth[netprofiles], false), IsNil)
			}
		}

		for tenant, groups := range groupNames {
			for _, group := range groups {
				c.Assert(s.removeContainers(containers[group]), IsNil)
				logrus.Infof("Deleting: %s", group)
				c.Assert(s.cli.EndpointGroupDelete(tenant, group), IsNil)
			}
		}

		for tenant, netprofiles := range npTenant {
			for _, netprofile := range netprofiles {
				logrus.Infof("Deleting %s ", netprofile)
				c.Assert(s.cli.NetprofileDelete(tenant, netprofile), IsNil)
			}
		}
		for tenant, networkNames := range networks {
			for _, network := range networkNames {
				logrus.Infof("Deleting:%s attached to:%s", network, tenant)
				c.Assert(s.cli.NetworkDelete(tenant, network), IsNil)
			}
			logrus.Infof("Deleting Tenant:%s", tenant)
			c.Assert(s.cli.TenantDelete(tenant), IsNil)
		}
	}

	s.TearDownBgp(c, encap)
}

//testNetprofileTriggerNetpluginRestart function checks if netprofile can be updated when netplugin is down.
//and the netplugin comes back up with the updated bandwidth.
func (s *systemtestSuite) TestNetprofileTriggerNetpluginRestartVLAN(c *C) {
	s.testNetprofileTriggerNetpluginRestart(c, EncapVLAN)
}
func (s *systemtestSuite) TestNetprofileTriggerNetpluginRestartVXLAN(c *C) {
	s.testNetprofileTriggerNetpluginRestart(c, EncapVXLAN)
}

func (s *systemtestSuite) testNetprofileTriggerNetpluginRestart(c *C, encap string) {
	s.SetupBgp(c, encap, false)
	s.CheckBgpConnection(c, encap)

	network := &client.Network{
		TenantName:  "default",
		NetworkName: "private",
		Subnet:      "10.1.1.1/24",
		Gateway:     "10.1.1.254",
		PktTag:      1001,
		Encap:       encap,
	}

	c.Assert(s.cli.NetworkPost(network), IsNil)

	netProfile := &client.Netprofile{
		ProfileName: "Netprofile",
		DSCP:        2,
		Bandwidth:   "17Mbps",
		Burst:       280,
		TenantName:  "default",
	}
	c.Assert(s.cli.NetprofilePost(netProfile), IsNil)

	groups := []*client.EndpointGroup{}
	groupNames := []string{}
	for x := 0; x < s.basicInfo.Containers; x++ {
		nodeNum := s.basicInfo.Containers % len(s.nodes)
		epgName := fmt.Sprintf("%s-srv%d-%d", network.NetworkName, nodeNum, x)
		group := &client.EndpointGroup{
			GroupName:   epgName,
			NetworkName: "private",
			TenantName:  "default",
		}
		c.Assert(s.cli.EndpointGroupPost(group), IsNil)

		groups = append(groups, group)
		groupNames = append(groupNames, epgName)
	}

	containers, err := s.runContainers(s.basicInfo.Containers, true, "private", "", groupNames, nil)
	c.Assert(err, IsNil)
	if s.fwdMode == FwdModeRouting && encap == EncapVLAN {
		err = s.CheckBgpRouteDistribution(c, containers)
		c.Assert(err, IsNil)
	}

	c.Assert(s.startIperfServers(containers), IsNil)
	c.Assert(s.startIperfClients(containers, "7Mbps", true), IsNil)

	groups = []*client.EndpointGroup{}
	groupNames = []string{}
	for x := 0; x < s.basicInfo.Containers; x++ {
		nodeNum := s.basicInfo.Containers % len(s.nodes)
		epgName := fmt.Sprintf("%s-srv%d-%d", network.NetworkName, nodeNum, x)
		group := &client.EndpointGroup{
			GroupName:   epgName,
			NetworkName: "private",
			NetProfile:  "Netprofile",
			TenantName:  "default",
		}
		c.Assert(s.cli.EndpointGroupPost(group), IsNil)

		groups = append(groups, group)
		groupNames = append(groupNames, epgName)
	}
	c.Assert(s.checkIngressRate(containers, netProfile.Bandwidth), IsNil)
	c.Assert(s.startIperfClients(containers, netProfile.Bandwidth, false), IsNil)

	logrus.Infof("Triggering netplugin restart!!!")
	//stop the netplugin.
	for _, node := range s.nodes {
		c.Assert(node.exec.stopNetplugin(), IsNil)
		c.Assert(node.rotateLog("netplugin"), IsNil)

		netProfile = &client.Netprofile{
			ProfileName: "Netprofile",
			DSCP:        9,
			Bandwidth:   "9Mbps",
			Burst:       100,
			TenantName:  "default",
		}
		c.Assert(s.cli.NetprofilePost(netProfile), IsNil)

		c.Assert(node.startNetplugin(""), IsNil)

		c.Assert(node.exec.runCommandUntilNoNetpluginError(), IsNil)
		time.Sleep(20 * time.Second)

		c.Assert(s.checkIngressRate(containers, netProfile.Bandwidth), IsNil)
		c.Assert(s.startIperfClients(containers, netProfile.Bandwidth, false), IsNil)
	}

	c.Assert(s.removeContainers(containers), IsNil)

	for _, group := range groups {
		c.Assert(s.cli.EndpointGroupDelete(group.TenantName, group.GroupName), IsNil)
	}
	c.Assert(s.cli.NetprofileDelete("default", "Netprofile"), IsNil)

	c.Assert(s.cli.NetworkDelete("default", "private"), IsNil)

}

//TestNetprofileUpdateNetmasterSwitchover checks after netmaster goes down, and comes back up,
//it comes up with the bandwidth value that was previously present and it can be updated once
//the netmaster is back up and running.
func (s *systemtestSuite) TestNetprofileUpdateNetmasterSwitchover(c *C) {

	if s.basicInfo.Scheduler == "k8" {
		return
	}

	network := &client.Network{
		TenantName:  "default",
		NetworkName: "private",
		Subnet:      "10.1.1.1/24",
		Gateway:     "10.1.1.254",
		PktTag:      1001,
		Encap:       EncapVXLAN,
	}

	c.Assert(s.cli.NetworkPost(network), IsNil)
	for i := 0; i < s.basicInfo.Iterations; i++ {

		netProfile := &client.Netprofile{
			ProfileName: "Netprofile",
			DSCP:        2,
			Bandwidth:   "16Mbps",
			Burst:       250,
			TenantName:  "default",
		}
		c.Assert(s.cli.NetprofilePost(netProfile), IsNil)

		groups := []*client.EndpointGroup{}
		groupNames := []string{}
		for x := 0; x < s.basicInfo.Containers; x++ {
			nodeNum := s.basicInfo.Containers % len(s.nodes)
			epgName := fmt.Sprintf("%s-srv%d-%d", network.NetworkName, nodeNum, x)
			group := &client.EndpointGroup{
				GroupName:   epgName,
				NetworkName: "private",
				NetProfile:  "Netprofile",
				TenantName:  "default",
			}
			c.Assert(s.cli.EndpointGroupPost(group), IsNil)

			groups = append(groups, group)
			groupNames = append(groupNames, epgName)
		}

		containers, err := s.runContainers(s.basicInfo.Containers, true, "private", "", groupNames, nil)
		c.Assert(err, IsNil)

		c.Assert(s.startIperfServers(containers), IsNil)
		c.Assert(s.checkIngressRate(containers, netProfile.Bandwidth), IsNil)
		c.Assert(s.startIperfClients(containers, netProfile.Bandwidth, false), IsNil)

		var leader, oldLeader *node

		leaderIP, err := s.clusterStoreGet("/contiv.io/lock/netmaster/leader")
		c.Assert(err, IsNil)

		for _, node := range s.nodes {
			res, err := node.getIPAddr("eth1")
			c.Assert(err, IsNil)
			if leaderIP == res {
				leader = node
				logrus.Infof("Found leader %s/%s", node.Name(), leaderIP)
			}
		}

		c.Assert(leader.exec.stopNetmaster(), IsNil)
		c.Assert(leader.rotateLog("netmaster"), IsNil)

		for x := 0; x < 15; x++ {
			logrus.Info("Waiting 5s for leader to change...")
			newLeaderIP, err := s.clusterStoreGet("/contiv.io/lock/netmaster/leader")
			c.Assert(err, IsNil)

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

		c.Assert(oldLeader.exec.startNetmaster(), IsNil)
		time.Sleep(5 * time.Second)
		c.Assert(s.checkIngressRate(containers, netProfile.Bandwidth), IsNil)
		c.Assert(s.startIperfClients(containers, netProfile.Bandwidth, false), IsNil)

		netProfile = &client.Netprofile{
			ProfileName: "Netprofile",
			DSCP:        1,
			Bandwidth:   "15Mbps",
			Burst:       200,
			TenantName:  "default",
		}
		c.Assert(s.cli.NetprofilePost(netProfile), IsNil)
		c.Assert(s.checkIngressRate(containers, netProfile.Bandwidth), IsNil)
		c.Assert(s.startIperfClients(containers, netProfile.Bandwidth, false), IsNil)

		c.Assert(s.removeContainers(containers), IsNil)

		for _, group := range groups {
			c.Assert(s.cli.EndpointGroupDelete(group.TenantName, group.GroupName), IsNil)
		}
		c.Assert(s.cli.NetprofileDelete("default", "Netprofile"), IsNil)
	}
	// delete the network
	c.Assert(s.cli.NetworkDelete("default", "private"), IsNil)
}

//testNetprofileAcrossGroup checks the bandwidth across a group of containers
//It verifies that no matter source bandwidth limit be more or less than the limit of client,
//It will never affect the client's traffic.
func (s *systemtestSuite) TestNetprofileAcrossGroupVXLAN(c *C) {
	s.testNetprofileAcrossGroup(c, EncapVXLAN)
}

func (s *systemtestSuite) TestNetprofileAcrossGroupVLAN(c *C) {
	s.testNetprofileAcrossGroup(c, EncapVLAN)
}

func (s *systemtestSuite) testNetprofileAcrossGroup(c *C, encap string) {
	s.SetupBgp(c, encap, false)
	s.CheckBgpConnection(c, encap)

	network := &client.Network{
		TenantName:  "default",
		NetworkName: "private",
		Subnet:      "10.1.1.1/24",
		Gateway:     "10.1.1.254",
		PktTag:      1001,
		Encap:       encap,
	}

	c.Assert(s.cli.NetworkPost(network), IsNil)

	netProfile := &client.Netprofile{
		ProfileName: "Netprofile",
		DSCP:        6,
		Bandwidth:   "6Mbps",
		Burst:       80,
		TenantName:  "default",
	}

	c.Assert(s.cli.NetprofilePost(netProfile), IsNil)

	netProfile1 := &client.Netprofile{
		ProfileName: "Netprofile1",
		DSCP:        6,
		Bandwidth:   "19Mbps",
		Burst:       220,
		TenantName:  "default",
	}

	c.Assert(s.cli.NetprofilePost(netProfile1), IsNil)

	groups := []*client.EndpointGroup{}
	groupNames := []string{}
	for x := 0; x < s.basicInfo.Containers; x++ {
		nodeNum := s.basicInfo.Containers % len(s.nodes)
		epgName := fmt.Sprintf("%s-srv%d-%d", network.NetworkName, nodeNum, x)
		group := &client.EndpointGroup{
			GroupName:   epgName,
			NetworkName: "private",
			NetProfile:  "Netprofile",
			TenantName:  "default",
		}
		c.Assert(s.cli.EndpointGroupPost(group), IsNil)

		groups = append(groups, group)
		groupNames = append(groupNames, epgName)
	}

	groupsNew := []*client.EndpointGroup{}
	NewGroupNames := []string{}
	for x := 0; x < s.basicInfo.Containers; x++ {
		nodeNum := s.basicInfo.Containers % len(s.nodes)
		epgName := fmt.Sprintf("%d-srv%d-%s", nodeNum, x, network.NetworkName)
		group := &client.EndpointGroup{
			GroupName:   epgName,
			NetworkName: "private",
			NetProfile:  "Netprofile1",
			TenantName:  "default",
		}
		c.Assert(s.cli.EndpointGroupPost(group), IsNil)

		groupsNew = append(groupsNew, group)
		NewGroupNames = append(NewGroupNames, epgName)
	}

	containersNew, err := s.runContainers(s.basicInfo.Containers, true, "private", "", NewGroupNames, nil)
	c.Assert(err, IsNil)
	if s.fwdMode == FwdModeRouting && encap == EncapVLAN {
		err = s.CheckBgpRouteDistribution(c, containersNew)
		c.Assert(err, IsNil)
	}

	containers, err := s.runContainers(s.basicInfo.Containers, true, "private", "", groupNames, nil)
	c.Assert(err, IsNil)
	if s.fwdMode == FwdModeRouting && encap == EncapVLAN {
		err = s.CheckBgpRouteDistribution(c, containers)
		c.Assert(err, IsNil)
	}

	c.Assert(s.startIperfServers(containers), IsNil)
	c.Assert(s.checkIngressRate(containers, netProfile1.Bandwidth), IsNil)
	c.Assert(s.checkIperfAcrossGroup(containersNew, containers, netProfile1.Bandwidth, false), IsNil)

	netProfile = &client.Netprofile{
		ProfileName: "Netprofile",
		DSCP:        6,
		Bandwidth:   "",
		Burst:       0,
		TenantName:  "default",
	}

	c.Assert(s.cli.NetprofilePost(netProfile), IsNil)

	c.Assert(s.checkIngressRate(containersNew, netProfile1.Bandwidth), IsNil)
	c.Assert(s.checkIperfAcrossGroup(containersNew, containers, netProfile1.Bandwidth, false), IsNil)

	c.Assert(s.removeContainers(containers), IsNil)
	c.Assert(s.removeContainers(containersNew), IsNil)

	for _, group := range groups {
		c.Assert(s.cli.EndpointGroupDelete(group.TenantName, group.GroupName), IsNil)
	}
	for _, group := range groupsNew {
		c.Assert(s.cli.EndpointGroupDelete(group.TenantName, group.GroupName), IsNil)
	}
	c.Assert(s.cli.NetprofileDelete("default", "Netprofile"), IsNil)
	c.Assert(s.cli.NetprofileDelete("default", "Netprofile1"), IsNil)
	c.Assert(s.cli.NetworkDelete("default", "private"), IsNil)

}
