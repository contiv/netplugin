/***
Copyright 2014 Cisco Systems Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package integration

import (
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/contivmodel/client"
	"github.com/contiv/netplugin/netmaster/mastercfg"

	. "github.com/contiv/check"
)

// TestEndpointCreateDelete test endpoint create and delete ops
func (its *integTestSuite) TestEndpointCreateDelete(c *C) {
	// Create a network
	err := its.client.NetworkPost(&client.Network{
		TenantName:  "default",
		NetworkName: "test",
		Subnet:      "10.1.1.0/24",
		Encap:       its.encap,
	})
	assertNoErr(err, c, "creating network")

	// verify network state is correct
	insp, err := its.client.NetworkInspect("default", "test")
	assertNoErr(err, c, "inspecting network")
	log.Infof("Inspecting network: %+v", insp)
	c.Assert(len(insp.Oper.Endpoints), Equals, 0)
	c.Assert(insp.Oper.AllocatedIPAddresses, Equals, "")
	c.Assert(insp.Oper.AvailableIPAddresses, Equals, "10.1.1.1-10.1.1.254")
	c.Assert(insp.Oper.PktTag, Equals, 1)
	c.Assert(insp.Oper.NumEndpoints, Equals, 0)

	for i := 0; i < its.iterations; i++ {
		addr, err := its.allocAddress("", "test.default", "")
		assertNoErr(err, c, "allocating address")
		c.Assert(addr, Equals, "10.1.1.1")

		// create an endpoint in the network
		epCfg1, err := its.createEndpoint("default", "test", "", addr, "")
		assertNoErr(err, c, "creating endpoint")

		// verify network & endpoint inspect output
		insp, err := its.client.NetworkInspect("default", "test")
		assertNoErr(err, c, "inspecting network")
		log.Infof("Inspecting network: %+v", insp)
		c.Assert(len(insp.Oper.Endpoints), Equals, 1)
		c.Assert(insp.Oper.AllocatedIPAddresses, Equals, "10.1.1.1")
		c.Assert(insp.Oper.AvailableIPAddresses, Equals, "10.1.1.2-10.1.1.254")
		c.Assert(insp.Oper.NumEndpoints, Equals, 1)

		// verify the endpoint inspect and flow
		its.verifyEndpointInspect("default", "test", epCfg1, c)
		its.verifyEndpointFlow(epCfg1, c)

		// allocate a specific address
		addr, err = its.allocAddress("", "test.default", "10.1.1.5")
		assertNoErr(err, c, "allocating address")
		c.Assert(addr, Equals, "10.1.1.5")

		// create an endpoint in the network
		epCfg2, err := its.createEndpoint("default", "test", "", addr, "")
		assertNoErr(err, c, "creating endpoint")

		// verify network & endpoint inspect output
		insp, err = its.client.NetworkInspect("default", "test")
		assertNoErr(err, c, "inspecting network")
		log.Infof("Inspecting network: %+v", insp)
		c.Assert(len(insp.Oper.Endpoints), Equals, 2)
		c.Assert(insp.Oper.AllocatedIPAddresses, Equals, "10.1.1.1, 10.1.1.5")
		c.Assert(insp.Oper.AvailableIPAddresses, Equals, "10.1.1.2-10.1.1.4, 10.1.1.6-10.1.1.254")
		c.Assert(insp.Oper.NumEndpoints, Equals, 2)

		// verify endpoint inspect and flows is added
		its.verifyEndpointInspect("default", "test", epCfg2, c)
		its.verifyEndpointFlow(epCfg2, c)

		// delete the endpoints
		err = its.deleteEndpoint("default", "test", "", epCfg1)
		assertNoErr(err, c, "deleting endpoint")
		err = its.deleteEndpoint("default", "test", "", epCfg2)
		assertNoErr(err, c, "deleting endpoint")

		// verify there are no more endpoints in the network
		insp, err = its.client.NetworkInspect("default", "test")
		assertNoErr(err, c, "inspecting network")
		c.Assert(len(insp.Oper.Endpoints), Equals, 0)
		log.Infof("Inspecting network: %+v", insp)
		c.Assert(len(insp.Oper.Endpoints), Equals, 0)
		c.Assert(insp.Oper.AllocatedIPAddresses, Equals, "")
		c.Assert(insp.Oper.AvailableIPAddresses, Equals, "10.1.1.1-10.1.1.254")
		c.Assert(insp.Oper.NumEndpoints, Equals, 0)

		// verify flows are also gone
		its.verifyEndpointFlowRemoved(epCfg1, c)
		its.verifyEndpointFlowRemoved(epCfg2, c)
	}

	assertNoErr(its.client.NetworkDelete("default", "test"), c, "deleting network")
}

// TestEndpointCreateDeleteParallel tests endpoint create and delete ops in parallel
func (its *integTestSuite) TestEndpointCreateDeleteParallel(c *C) {
	// Create a network
	err := its.client.NetworkPost(&client.Network{
		TenantName:  "default",
		NetworkName: "test",
		Subnet:      "10.1.1.0/24",
		Encap:       its.encap,
	})
	assertNoErr(err, c, "creating network")

	for i := 0; i < its.iterations; i++ {
		epCfgList, err := its.createEndpointsParallel("default", "test", "")
		assertNoErr(err, c, "creating endpoints in parallel")

		// verify network & endpoint inspect output
		insp, err := its.client.NetworkInspect("default", "test")
		assertNoErr(err, c, "inspecting network")
		log.Infof("Inspecting network: %+v", insp)
		c.Assert(len(insp.Oper.Endpoints), Equals, its.parallels)
		c.Assert(insp.Oper.NumEndpoints, Equals, its.parallels)
		c.Assert(insp.Oper.AllocatedIPAddresses, Equals, fmt.Sprintf("10.1.1.1-10.1.1.%d", its.parallels))
		c.Assert(insp.Oper.AvailableIPAddresses, Equals, fmt.Sprintf("10.1.1.%d-10.1.1.254", (its.parallels+1)))

		// verify all endpoints
		for j := 0; j < its.parallels; j++ {
			// verify endpoint
			its.verifyEndpointInspect("default", "test", epCfgList[j], c)
			// verify the flow
			its.verifyEndpointFlow(epCfgList[j], c)
		}

		// delete the endpoints
		err = its.deleteEndpointsParallel("default", "test", "", epCfgList)
		assertNoErr(err, c, "deleting endpoints in parallel")

		// verify there are no more endpoints in the network
		insp, err = its.client.NetworkInspect("default", "test")
		assertNoErr(err, c, "inspecting network")
		c.Assert(len(insp.Oper.Endpoints), Equals, 0)
		log.Infof("Inspecting network: %+v", insp)
		c.Assert(len(insp.Oper.Endpoints), Equals, 0)
		c.Assert(insp.Oper.AllocatedIPAddresses, Equals, "")
		c.Assert(insp.Oper.AvailableIPAddresses, Equals, "10.1.1.1-10.1.1.254")
		c.Assert(insp.Oper.NumEndpoints, Equals, 0)

		// verify flows are also gone
		for j := 0; j < its.parallels; j++ {
			its.verifyEndpointFlowRemoved(epCfgList[j], c)
		}
	}

	assertNoErr(its.client.NetworkDelete("default", "test"), c, "deleting network")
}

// TestEndpointGroupCreateDelete tests EPG create delete ops
func (its *integTestSuite) TestEndpointGroupCreateDelete(c *C) {
	// Create a network
	err := its.client.NetworkPost(&client.Network{
		TenantName:  "default",
		NetworkName: "test",
		Subnet:      "10.1.1.0/24",
		Encap:       its.encap,
	})
	assertNoErr(err, c, "creating network")

	for i := 0; i < its.iterations; i++ {
		err := its.client.EndpointGroupPost(&client.EndpointGroup{
			TenantName:       "default",
			NetworkName:      "test",
			GroupName:        "epg1",
			Policies:         []string{},
			ExtContractsGrps: []string{},
		})

		assertNoErr(err, c, "creating epg")

		addr, err := its.allocAddress("", "test.default", "")
		assertNoErr(err, c, "allocating address")
		c.Assert(addr, Equals, "10.1.1.1")

		// create an endpoint in the network
		epCfg1, err := its.createEndpoint("default", "test", "epg1", addr, "")
		assertNoErr(err, c, "creating endpoint")

		// delete epg with active endpoints - should FAIL
		err = its.client.EndpointGroupDelete("default", "epg1")
		assertErr(err, c, "deleting epg")

		// delete the endpoints
		err = its.deleteEndpoint("default", "test", "", epCfg1)
		assertNoErr(err, c, "deleting endpoint")

		// delete epg
		err = its.client.EndpointGroupDelete("default", "epg1")
		assertNoErr(err, c, "deleting epg")

		// verify flows are also gone
		its.verifyEndpointFlowRemoved(epCfg1, c)
	}

	assertNoErr(its.client.NetworkDelete("default", "test"), c, "deleting network")

	// test network/epg subnet
	nwData := []struct {
		subnet      string
		availableIP string
	}{

		{subnet: "10.100.100.194-10.100.100.222/27", availableIP: "10.100.100.194-10.100.100.222"},
		{subnet: "10.1.1.5-10.1.1.15/27", availableIP: "10.1.1.5-10.1.1.15"},
		{subnet: "10.1.1.0/27", availableIP: "10.1.1.1-10.1.1.30"},
		{subnet: "10.1.1.0/28", availableIP: "10.1.1.1-10.1.1.14"},
		{subnet: "10.1.1.0/29", availableIP: "10.1.1.1-10.1.1.6"},
		{subnet: "10.1.1.0/30", availableIP: "10.1.1.1-10.1.1.2"},
		{subnet: "10.1.1.0/31"},
	}
	for _, nwRange := range nwData {
		err := its.client.NetworkPost(&client.Network{
			TenantName:  "default",
			NetworkName: "subnet-test1",
			Subnet:      nwRange.subnet,
			Encap:       its.encap,
		})
		assertNoErr(err, c, "creating network")

		// inspect
		nInspect, err := its.client.NetworkInspect("default", "subnet-test1")
		assertNoErr(err, c, fmt.Sprintf("inspect failed for %+v", nwRange.subnet))
		assertOnTrue(c, nInspect.Oper.AllocatedIPAddresses != "",
			fmt.Sprintf("invalid allocated address %+v", nInspect))
		assertOnTrue(c, nInspect.Oper.AvailableIPAddresses != nwRange.availableIP,
			fmt.Sprintf("invalid available  address %+v", nInspect))

		// check epg
		err = its.client.EndpointGroupPost(&client.EndpointGroup{
			TenantName:       "default",
			NetworkName:      "subnet-test1",
			GroupName:        "epg1",
			IpPool:           nwRange.availableIP,
			Policies:         []string{},
			ExtContractsGrps: []string{},
		})
		assertNoErr(err, c, fmt.Sprintf("create epg %+v", "epg1"))
		// inspect
		dInspect, e := its.client.EndpointGroupInspect("default", "epg1")
		assertNoErr(e, c, fmt.Sprintf("inspect failed for %+v", nwRange))
		assertOnTrue(c, dInspect.Oper.AllocatedIPAddresses != "",
			fmt.Sprintf("invalid allocated address %+v", dInspect))
		assertOnTrue(c, dInspect.Oper.AvailableIPAddresses != nwRange.availableIP,
			fmt.Sprintf("invalid available  address %+v", dInspect))
		assertNoErr(its.client.EndpointGroupDelete("default", "epg1"), c, "delete epg")

		assertNoErr(its.client.NetworkDelete("default", "subnet-test1"), c, "deleting network")
	}

}

// TestEndpointGrouIPPoolCreateDelete tests EPG with IPAM create delete ops
func (its *integTestSuite) TestEndpointGroupIPPoolCreateDelete(c *C) {
	// Create a network
	nwName := "poolTest"
	err := its.client.NetworkPost(&client.Network{
		TenantName:  "default",
		NetworkName: nwName,
		Subnet:      "10.3.1.0/24",
		Encap:       its.encap,
	})
	assertNoErr(err, c, "creating network")
	epgSeg := []string{"10.3.1.10-10.3.1.20", "10.3.1.40-10.3.1.42", "10.3.1.110-10.3.1.120"}

	for i := 0; i < its.iterations; i++ {
		for _, epgPool := range epgSeg {
			err := its.client.EndpointGroupPost(&client.EndpointGroup{
				TenantName:       "default",
				NetworkName:      nwName,
				GroupName:        "epg1",
				IpPool:           epgPool,
				Policies:         []string{},
				ExtContractsGrps: []string{},
			})

			assertNoErr(err, c, "create epg")
			addr, err := its.allocAddress("", fmt.Sprintf("%s:epg1.default", nwName), "")
			assertNoErr(err, c, "allocating address")
			c.Assert(addr, Equals, strings.Split(epgPool, "-")[0])

			// create an endpoint in the network
			epCfg1, err := its.createEndpoint("default", nwName, "epg1", addr, "")
			assertNoErr(err, c, "creating endpoint")

			// delete epg with active endpoints - should FAIL
			err = its.client.EndpointGroupDelete("default", "epg1")
			assertErr(err, c, "deleting epg")

			// delete the endpoints
			err = its.deleteEndpoint("default", nwName, "epg1", epCfg1)
			assertNoErr(err, c, "deleting endpoint")

			// delete epg
			err = its.client.EndpointGroupDelete("default", "epg1")
			assertNoErr(err, c, "deleting epg")

			// verify flows are also gone
			its.verifyEndpointFlowRemoved(epCfg1, c)
		}
	}

	epgSeg = []string{"10.3.1.0-10.3.1.20", "10.3.1.254-10.3.1.255",
		"10.3.1.110-10.3.1.320", "10.3.2.0-10.3.2.20", "10.3.2.0/24"}
	for _, epgPool := range epgSeg {
		err := its.client.EndpointGroupPost(&client.EndpointGroup{
			TenantName:       "default",
			NetworkName:      nwName,
			GroupName:        "epg1",
			IpPool:           epgPool,
			Policies:         []string{},
			ExtContractsGrps: []string{},
		})
		assertErr(err, c, fmt.Sprintf("create epg %+v", epgPool))
	}

	epgSeg = []string{"10.3.1.30-10.3.1.50", "", "10.3.1.1-19.3.1.21"}
	err = its.client.EndpointGroupPost(&client.EndpointGroup{
		TenantName:       "default",
		NetworkName:      nwName,
		GroupName:        "epg1",
		IpPool:           "10.3.1.10-10.3.1.20",
		Policies:         []string{},
		ExtContractsGrps: []string{},
	})

	for _, epgPool := range epgSeg {
		err := its.client.EndpointGroupPost(&client.EndpointGroup{
			TenantName:       "default",
			NetworkName:      nwName,
			GroupName:        "epg1",
			IpPool:           epgPool,
			Policies:         []string{},
			ExtContractsGrps: []string{},
		})
		assertErr(err, c, fmt.Sprintf("create epg %+v", epgPool))
	}
	err = its.client.EndpointGroupDelete("default", "epg1")
	assertNoErr(err, c, "deleting epg")

	// exhaust pool
	err = its.client.EndpointGroupPost(&client.EndpointGroup{
		TenantName:       "default",
		NetworkName:      nwName,
		GroupName:        "epg1",
		IpPool:           "10.3.1.11-10.3.1.11",
		Policies:         []string{},
		ExtContractsGrps: []string{},
	})
	assertNoErr(err, c, "create epg")
	addr, err := its.allocAddress("", fmt.Sprintf("%s:epg1.default", nwName), "")
	assertNoErr(err, c, "allocating address")
	// create an endpoint in the network
	epCfg1, err := its.createEndpoint("default", nwName, "epg1", addr, "")
	assertNoErr(err, c, "creating endpoint")
	addr, err = its.allocAddress("", fmt.Sprintf("%s:epg1.default", nwName), "")
	assertErr(err, c, "allocating address")
	err = its.deleteEndpoint("default", nwName, "epg1", epCfg1)
	assertNoErr(err, c, "deleting endpoint")
	err = its.client.EndpointGroupDelete("default", "epg1")
	assertNoErr(err, c, "deleting epg")
	assertNoErr(its.client.NetworkDelete("default", nwName), c, "deleting network")

	// Test multiple epgs with/without ip pool
	nwName = "epgPoolTest"
	nwSubnet := "10.2.2.0/24"
	err = its.client.NetworkPost(&client.Network{
		TenantName:  "default",
		NetworkName: nwName,
		Subnet:      nwSubnet,
		Encap:       its.encap,
	})
	assertNoErr(err, c, "creating network")

	epgStruct := []struct {
		ipPool      string
		availableIP string
		allocatedIP string
	}{
		{ipPool: "", availableIP: "10.2.2.2-10.2.2.4, 10.2.2.181-10.2.2.254",
			allocatedIP: "10.2.2.1, 10.2.2.5-10.2.2.180"},
		{ipPool: "10.2.2.5-10.2.2.10", availableIP: "10.2.2.6-10.2.2.10",
			allocatedIP: "10.2.2.5"},
		{ipPool: "10.2.2.11-10.2.2.30", availableIP: "10.2.2.12-10.2.2.30",
			allocatedIP: "10.2.2.11"},
		{ipPool: "", availableIP: "10.2.2.3-10.2.2.4, 10.2.2.181-10.2.2.254",
			allocatedIP: "10.2.2.1-10.2.2.2, 10.2.2.5-10.2.2.180"},
		{ipPool: "10.2.2.31-10.2.2.132", availableIP: "10.2.2.32-10.2.2.132",
			allocatedIP: "10.2.2.31"},
		{ipPool: "10.2.2.133-10.2.2.180", availableIP: "10.2.2.134-10.2.2.180",
			allocatedIP: "10.2.2.133"},
	}

	for epgIndex, epgPool1 := range epgStruct {
		epgName := fmt.Sprintf("epgC-%d", epgIndex)
		err := its.client.EndpointGroupPost(&client.EndpointGroup{
			TenantName:       "default",
			NetworkName:      nwName,
			GroupName:        epgName,
			IpPool:           epgPool1.ipPool,
			Policies:         []string{},
			ExtContractsGrps: []string{},
		})
		assertNoErr(err, c, fmt.Sprintf("create epg %s  %+v", epgName, epgPool1.ipPool))
	}

	inspNw, err := its.client.NetworkInspect("default", nwName)
	assertNoErr(err, c, fmt.Sprintf("inspecting network %s", nwName))
	assertOnTrue(c, inspNw.Oper.AllocatedIPAddresses != "10.2.2.5-10.2.2.180",
		fmt.Sprintf("invalid alloc addr %s for nw %s", inspNw.Oper.AllocatedIPAddresses, nwName))
	assertOnTrue(c, inspNw.Oper.AvailableIPAddresses != "10.2.2.1-10.2.2.4, 10.2.2.181-10.2.2.254",
		fmt.Sprintf("invalid avail addr %s for nw %s", inspNw.Oper.AvailableIPAddresses, nwName))

	epgCfg := make([]*mastercfg.CfgEndpointState, len(epgStruct))
	for epgIndex, epgPool1 := range epgStruct {
		epgName := fmt.Sprintf("epgC-%d", epgIndex)
		addr, err := its.allocAddress("", fmt.Sprintf("%s:%s.default", nwName, epgName), "")
		assertNoErr(err, c, "allocating address")
		if len(epgPool1.ipPool) > 0 {
			c.Assert(addr, Equals, strings.Split(epgPool1.ipPool, "-")[0])
			inspEpg, err := its.client.EndpointGroupInspect("default", epgName)
			assertNoErr(err, c, fmt.Sprintf("inspecting epg %s", epgName))
			assertOnTrue(c, inspEpg.Oper.AllocatedIPAddresses != epgPool1.allocatedIP,
				fmt.Sprintf("invalid alloc addr %s for epg %s", inspEpg.Oper.AllocatedIPAddresses, epgName))
			assertOnTrue(c, inspEpg.Oper.AvailableIPAddresses != epgPool1.availableIP,
				fmt.Sprintf("invalid avail addr %s for epg %s", inspEpg.Oper.AvailableIPAddresses, epgName))
		} else {
			epIPList := strings.Split(strings.Split(epgPool1.allocatedIP, ",")[0], "-")
			epIP := ""
			if len(epIPList) > 1 {
				epIP = epIPList[1]
			} else {
				epIP = epIPList[0]
			}

			c.Assert(addr, Equals, epIP)
			inspNw, err = its.client.NetworkInspect("default", nwName)
			assertNoErr(err, c, fmt.Sprintf("inspecting network %s", nwName))
			assertOnTrue(c, inspNw.Oper.AllocatedIPAddresses != epgPool1.allocatedIP,
				fmt.Sprintf("invalid alloc addr %s for nw %s", inspNw.Oper.AllocatedIPAddresses, nwName))
			assertOnTrue(c, inspNw.Oper.AvailableIPAddresses != epgPool1.availableIP,
				fmt.Sprintf("invalid avail addr %s for nw %s", inspNw.Oper.AvailableIPAddresses, nwName))
		}

		// create an endpoint in the network
		epCfg1, err := its.createEndpoint("default", nwName, fmt.Sprintf("epgC-%d", epgIndex), addr, "")
		assertNoErr(err, c, "creating endpoint")
		epgCfg[epgIndex] = epCfg1
	}

	epgDelStr := []struct {
		ipPool      string
		availableIP string
		allocatedIP string
	}{
		{ipPool: "", availableIP: "10.2.2.1, 10.2.2.3-10.2.2.4, 10.2.2.181-10.2.2.254",
			allocatedIP: "10.2.2.2, 10.2.2.5-10.2.2.180"},
		{ipPool: "10.2.2.5-10.2.2.10", availableIP: "10.2.2.5-10.2.2.10",
			allocatedIP: ""},
		{ipPool: "10.2.2.11-10.2.2.30", availableIP: "10.2.2.11-10.2.2.30",
			allocatedIP: ""},
		{ipPool: "", availableIP: "10.2.2.1-10.2.2.4, 10.2.2.181-10.2.2.254",
			allocatedIP: "10.2.2.5-10.2.2.180"},
		{ipPool: "10.2.2.31-10.2.2.132", availableIP: "10.2.2.31-10.2.2.132",
			allocatedIP: ""},
		{ipPool: "10.2.2.133-10.2.2.180", availableIP: "10.2.2.133-10.2.2.180",
			allocatedIP: ""},
	}

	for epgIndex, epgDelPool := range epgDelStr {
		epgName := fmt.Sprintf("epgC-%d", epgIndex)
		// delete the endpoints
		err = its.deleteEndpoint("default", nwName, fmt.Sprintf("epgC-%d", epgIndex), epgCfg[epgIndex])
		assertNoErr(err, c, "deleting endpoint")

		if len(epgDelPool.ipPool) > 0 {
			inspEpg, err := its.client.EndpointGroupInspect("default", epgName)
			assertNoErr(err, c, fmt.Sprintf("inspecting epg %s", epgName))
			assertOnTrue(c, inspEpg.Oper.AllocatedIPAddresses != epgDelPool.allocatedIP,
				fmt.Sprintf("invalid alloc addr %s for epg %s", inspEpg.Oper.AllocatedIPAddresses, epgName))
			assertOnTrue(c, inspEpg.Oper.AvailableIPAddresses != epgDelPool.availableIP,
				fmt.Sprintf("invalid avail addr %s for epg %s", inspEpg.Oper.AvailableIPAddresses, epgName))
		} else {
			inspNw, err = its.client.NetworkInspect("default", nwName)
			assertNoErr(err, c, fmt.Sprintf("inspecting network %s", nwName))
			assertOnTrue(c, inspNw.Oper.AllocatedIPAddresses != epgDelPool.allocatedIP,
				fmt.Sprintf("invalid alloc addr %s for nw %s", inspNw.Oper.AllocatedIPAddresses, nwName))
			assertOnTrue(c, inspNw.Oper.AvailableIPAddresses != epgDelPool.availableIP,
				fmt.Sprintf("invalid avail addr %s for nw %s", inspNw.Oper.AvailableIPAddresses, nwName))
		}

	}

	epgDelStr = []struct {
		ipPool      string
		availableIP string
		allocatedIP string
	}{
		{ipPool: "", availableIP: "10.2.2.1-10.2.2.4, 10.2.2.181-10.2.2.254",
			allocatedIP: "10.2.2.5-10.2.2.180"},
		{ipPool: "10.2.2.5-10.2.2.10", availableIP: "10.2.2.1-10.2.2.10, 10.2.2.181-10.2.2.254",
			allocatedIP: "10.2.2.11-10.2.2.180"},
		{ipPool: "10.2.2.11-10.2.2.30", availableIP: "10.2.2.1-10.2.2.30, 10.2.2.181-10.2.2.254",
			allocatedIP: "10.2.2.31-10.2.2.180"},
		{ipPool: "", availableIP: "10.2.2.1-10.2.2.30, 10.2.2.181-10.2.2.254",
			allocatedIP: "10.2.2.31-10.2.2.180"},
		{ipPool: "10.2.2.31-10.2.2.132", availableIP: "10.2.2.1-10.2.2.132, 10.2.2.181-10.2.2.254",
			allocatedIP: "10.2.2.133-10.2.2.180"},
		{ipPool: "10.2.2.133-10.2.2.180", availableIP: "10.2.2.1-10.2.2.254",
			allocatedIP: ""},
	}

	for epgIndex, epgDelPool := range epgDelStr {
		// delete epg
		err = its.client.EndpointGroupDelete("default", fmt.Sprintf("epgC-%d", epgIndex))
		assertNoErr(err, c, "deleting epg")
		// verify flows are also gone
		its.verifyEndpointFlowRemoved(epgCfg[epgIndex], c)

		inspNw, err = its.client.NetworkInspect("default", nwName)
		assertNoErr(err, c, fmt.Sprintf("inspecting network %s", nwName))
		assertOnTrue(c, inspNw.Oper.AllocatedIPAddresses != epgDelPool.allocatedIP,
			fmt.Sprintf("invalid alloc addr %s for nw %s", inspNw.Oper.AllocatedIPAddresses, nwName))
		assertOnTrue(c, inspNw.Oper.AvailableIPAddresses != epgDelPool.availableIP,
			fmt.Sprintf("invalid avail addr %s for nw %s", inspNw.Oper.AvailableIPAddresses, nwName))
	}

	assertNoErr(its.client.NetworkDelete("default", nwName), c, "deleting network")
}

// TestEndpointGroupInspect test endpointGroup inspect command
func (its *integTestSuite) TestEndpointGroupInspect(c *C) {
	// Create a network
	err := its.client.NetworkPost(&client.Network{
		TenantName:  "default",
		NetworkName: "test",
		Subnet:      "10.1.1.0/24",
		Encap:       its.encap,
	})
	assertNoErr(err, c, "creating network")

	// Create a epg
	err = its.client.EndpointGroupPost(&client.EndpointGroup{
		TenantName:  "default",
		NetworkName: "test",
		GroupName:   "epgA",
	})
	assertNoErr(err, c, "creating endpointgroup")

	// verify endpointGroup state is correct
	insp, err := its.client.EndpointGroupInspect("default", "epgA")
	assertNoErr(err, c, "inspecting endpointGroup")
	log.Infof("Inspecting endpointGroup: %+v", insp)
	c.Assert(len(insp.Oper.Endpoints), Equals, 0)
	c.Assert(insp.Oper.PktTag, Equals, 1)
	c.Assert(insp.Oper.NumEndpoints, Equals, 0)

	for i := 0; i < its.iterations; i++ {
		addr, err := its.allocAddress("", "test.default", "")
		assertNoErr(err, c, "allocating address")
		c.Assert(addr, Equals, "10.1.1.1")

		// create an endpoint in epg
		epCfg1, err := its.createEndpoint("default", "test", "epgA", addr, "")
		assertNoErr(err, c, "creating endpoint")

		// verify endpointGroup & endpoint inspect output
		insp, err := its.client.EndpointGroupInspect("default", "epgA")
		assertNoErr(err, c, "inspecting endpointGroup")
		log.Infof("Inspecting endpointGroup: %+v", insp)
		c.Assert(len(insp.Oper.Endpoints), Equals, 1)
		c.Assert(insp.Oper.NumEndpoints, Equals, 1)

		// verify the endpoint inspect and flow
		its.verifyEndpointInspect("default", "test", epCfg1, c)
		its.verifyEndpointFlow(epCfg1, c)

		// allocate a specific address
		addr, err = its.allocAddress("", "test.default", "10.1.1.5")
		assertNoErr(err, c, "allocating address")
		c.Assert(addr, Equals, "10.1.1.5")

		// create an endpoint in epg
		epCfg2, err := its.createEndpoint("default", "test", "epgA", addr, "")
		assertNoErr(err, c, "creating endpoint")

		// verify network & endpoint inspect output
		insp, err = its.client.EndpointGroupInspect("default", "epgA")
		assertNoErr(err, c, "inspecting endpointGroup")
		log.Infof("Inspecting endpointGroup: %+v", insp)
		c.Assert(len(insp.Oper.Endpoints), Equals, 2)
		c.Assert(insp.Oper.NumEndpoints, Equals, 2)

		// verify endpoint inspect and flows is added
		its.verifyEndpointInspect("default", "test", epCfg2, c)
		its.verifyEndpointFlow(epCfg2, c)

		// delete the endpoints
		err = its.deleteEndpoint("default", "test", "", epCfg1)
		assertNoErr(err, c, "deleting endpoint")
		err = its.deleteEndpoint("default", "test", "", epCfg2)
		assertNoErr(err, c, "deleting endpoint")

		// verify there are no more endpoints in epg
		insp, err = its.client.EndpointGroupInspect("default", "epgA")
		assertNoErr(err, c, "inspecting endpointGroup")
		c.Assert(len(insp.Oper.Endpoints), Equals, 0)
		log.Infof("Inspecting endpointGroup: %+v", insp)
		c.Assert(len(insp.Oper.Endpoints), Equals, 0)
		c.Assert(insp.Oper.NumEndpoints, Equals, 0)

		// verify flows are also gone
		its.verifyEndpointFlowRemoved(epCfg1, c)
		its.verifyEndpointFlowRemoved(epCfg2, c)
	}

	assertNoErr(its.client.EndpointGroupDelete("default", "epgA"), c, "deleting endpointGroup")
	assertNoErr(its.client.NetworkDelete("default", "test"), c, "deleting network")
}

// TestTenantCreateDelete test tenant create and delete ops
func (its *integTestSuite) TestTenantCreateDelete(c *C) {
	// Create a tenant
	c.Assert(its.client.TenantPost(&client.Tenant{
		TenantName: "TestTenant",
	}), IsNil)

	err := its.client.NetworkPost(&client.Network{
		TenantName:  "TestTenant",
		NetworkName: "TestNet",
		Subnet:      "20.1.1.0/24",
		Encap:       its.encap,
	})
	assertNoErr(err, c, "creating network")

	// Create a epg
	err = its.client.EndpointGroupPost(&client.EndpointGroup{
		TenantName:  "TestTenant",
		NetworkName: "TestNet",
		GroupName:   "epgA",
	})
	assertNoErr(err, c, "creating endpointgroup")

	// Create a epg
	err = its.client.EndpointGroupPost(&client.EndpointGroup{
		TenantName:  "TestTenant",
		NetworkName: "TestNet",
		GroupName:   "epgB",
	})
	assertNoErr(err, c, "creating endpointgroup")

	// verify tenant state is correct
	insp, err := its.client.TenantInspect("TestTenant")
	assertNoErr(err, c, "inspecting tenant")
	log.Infof("Inspecting tenant: %+v", insp)
	c.Assert(insp.Oper.TotalEndpoints, Equals, 0)
	c.Assert(insp.Oper.TotalNetworks, Equals, 1)
	c.Assert(insp.Oper.TotalEPGs, Equals, 2)

	for i := 0; i < its.iterations; i++ {
		addr, err := its.allocAddress("", "TestNet.TestTenant", "")
		assertNoErr(err, c, "allocating address")
		c.Assert(addr, Equals, "20.1.1.1")
		epCfg1, err := its.createEndpoint("TestTenant", "TestNet", "epgA", addr, "")
		assertNoErr(err, c, "creating endpoint")
		insp, err := its.client.TenantInspect("TestTenant")
		assertNoErr(err, c, "inspecting tenant")
		log.Infof("Inspecting tenant: %+v", insp)
		c.Assert(insp.Oper.TotalEndpoints, Equals, 1)

		// allocate a specific address
		addr, err = its.allocAddress("", "TestNet.TestTenant", "20.1.1.2")
		assertNoErr(err, c, "allocating address")
		c.Assert(addr, Equals, "20.1.1.2")
		epCfg2, err := its.createEndpoint("TestTenant", "TestNet", "epgB", addr, "")
		assertNoErr(err, c, "creating endpoint")
		insp, err = its.client.TenantInspect("TestTenant")
		assertNoErr(err, c, "inspecting tenant")
		log.Infof("Inspecting tenant: %+v", insp)
		c.Assert(insp.Oper.TotalEndpoints, Equals, 2)

		// allocate a specific address
		addr, err = its.allocAddress("", "TestNet.TestTenant", "20.1.1.3")
		assertNoErr(err, c, "allocating address")
		c.Assert(addr, Equals, "20.1.1.3")
		epCfg3, err := its.createEndpoint("TestTenant", "TestNet", "", addr, "")
		assertNoErr(err, c, "creating endpoint")
		insp, err = its.client.TenantInspect("TestTenant")
		assertNoErr(err, c, "inspecting tenant")
		log.Infof("Inspecting tenant: %+v", insp)
		c.Assert(insp.Oper.TotalEndpoints, Equals, 3)

		err = its.deleteEndpoint("TestTenant", "TestNet", "epgA", epCfg1)
		assertNoErr(err, c, "deleting endpoint")

		err = its.deleteEndpoint("TestTenant", "TestNet", "epgB", epCfg2)
		assertNoErr(err, c, "deleting endpoint")

		err = its.deleteEndpoint("TestTenant", "TestNet", "", epCfg3)
		assertNoErr(err, c, "deleting endpoint")

		// verify there are no more endpoints in epg
		insp, err = its.client.TenantInspect("TestTenant")
		assertNoErr(err, c, "inspecting Tenant")
		c.Assert(len(insp.Oper.Endpoints), Equals, 0)
		log.Infof("Inspecting Tenant: %+v", insp)
		c.Assert(insp.Oper.TotalEndpoints, Equals, 0)

		// verify flows are also gone
		its.verifyEndpointFlowRemoved(epCfg1, c)
		its.verifyEndpointFlowRemoved(epCfg2, c)
		its.verifyEndpointFlowRemoved(epCfg3, c)
	}

	assertNoErr(its.client.EndpointGroupDelete("TestTenant", "epgA"), c, "deleting endpointGroup")
	assertNoErr(its.client.EndpointGroupDelete("TestTenant", "epgB"), c, "deleting endpointGroup")
	assertNoErr(its.client.NetworkDelete("TestTenant", "TestNet"), c, "deleting network")
	assertNoErr(its.client.TenantDelete("TestTenant"), c, "deleting Tenant")
}

// TestPolicyInspect test policy inspect command
func (its *integTestSuite) TestPolicyInspect(c *C) {
	// Create a network
	err := its.client.NetworkPost(&client.Network{
		TenantName:  "default",
		NetworkName: "test",
		Subnet:      "10.1.1.0/24",
		Encap:       its.encap,
	})
	assertNoErr(err, c, "creating network")

	// Create a policy
	c.Assert(its.client.PolicyPost(&client.Policy{
		PolicyName: "policy",
		TenantName: "default",
	}), IsNil)

	rules := []*client.Rule{
		{
			RuleID:     "1",
			PolicyName: "policy",
			TenantName: "default",
			Direction:  "in",
			Protocol:   "tcp",
			Action:     "deny",
		},
		{
			RuleID:     "2",
			PolicyName: "policy",
			TenantName: "default",
			Priority:   100,
			Direction:  "in",
			Protocol:   "tcp",
			Port:       8000,
			Action:     "allow",
		},
	}

	for _, rule := range rules {
		c.Assert(its.client.RulePost(rule), IsNil)
	}

	// Create a epgB and attach it to policy
	err = its.client.EndpointGroupPost(&client.EndpointGroup{
		TenantName:  "default",
		NetworkName: "test",
		Policies:    []string{"policy"},
		GroupName:   "epgA",
	})
	assertNoErr(err, c, "creating endpointgroup")

	// Create a epgB and attach it to policy
	err = its.client.EndpointGroupPost(&client.EndpointGroup{
		TenantName:  "default",
		NetworkName: "test",
		Policies:    []string{"policy"},
		GroupName:   "epgB",
	})
	assertNoErr(err, c, "creating endpointgroup")

	// verify Policy state is correct
	insp, err := its.client.PolicyInspect("default", "policy")
	assertNoErr(err, c, "inspecting policy")
	log.Infof("Inspecting policy: %+v", insp)
	c.Assert(len(insp.Oper.Endpoints), Equals, 0)
	c.Assert(insp.Oper.NumEndpoints, Equals, 0)

	for i := 0; i < its.iterations; i++ {
		addr, err := its.allocAddress("", "test.default", "")
		assertNoErr(err, c, "allocating address")
		c.Assert(addr, Equals, "10.1.1.1")

		// create an endpoint in epgA
		epCfg1, err := its.createEndpoint("default", "test", "epgA", addr, "")
		assertNoErr(err, c, "creating endpoint")

		// verify policy & endpoint inspect output
		insp, err := its.client.PolicyInspect("default", "policy")
		assertNoErr(err, c, "inspecting policy")
		log.Infof("Inspecting policy: %+v", insp)
		c.Assert(len(insp.Oper.Endpoints), Equals, 1)
		c.Assert(insp.Oper.NumEndpoints, Equals, 1)

		// verify the endpoint inspect and flow
		its.verifyEndpointInspect("default", "test", epCfg1, c)
		its.verifyEndpointFlow(epCfg1, c)

		// allocate a specific address
		addr, err = its.allocAddress("", "test.default", "10.1.1.5")
		assertNoErr(err, c, "allocating address")
		c.Assert(addr, Equals, "10.1.1.5")

		// create an endpoint in epgB
		epCfg2, err := its.createEndpoint("default", "test", "epgB", addr, "")
		assertNoErr(err, c, "creating endpoint")

		// verify policy & endpoint inspect output
		insp, err = its.client.PolicyInspect("default", "policy")
		assertNoErr(err, c, "inspecting policy")
		log.Infof("Inspecting policy: %+v", insp)
		c.Assert(len(insp.Oper.Endpoints), Equals, 2)
		c.Assert(insp.Oper.NumEndpoints, Equals, 2)

		// verify endpoint inspect and flows is added
		its.verifyEndpointInspect("default", "test", epCfg2, c)
		its.verifyEndpointFlow(epCfg2, c)

		// delete the endpoints
		err = its.deleteEndpoint("default", "test", "", epCfg1)
		assertNoErr(err, c, "deleting endpoint")
		err = its.deleteEndpoint("default", "test", "", epCfg2)
		assertNoErr(err, c, "deleting endpoint")

		// verify there are no more endpoints in epg
		insp, err = its.client.PolicyInspect("default", "policy")
		assertNoErr(err, c, "inspecting policy")
		c.Assert(len(insp.Oper.Endpoints), Equals, 0)
		log.Infof("Inspecting policy: %+v", insp)
		c.Assert(len(insp.Oper.Endpoints), Equals, 0)
		c.Assert(insp.Oper.NumEndpoints, Equals, 0)

		// verify flows are also gone
		its.verifyEndpointFlowRemoved(epCfg1, c)
		its.verifyEndpointFlowRemoved(epCfg2, c)
	}

	assertNoErr(its.client.EndpointGroupDelete("default", "epgA"), c, "deleting endpointGroup")
	assertNoErr(its.client.EndpointGroupDelete("default", "epgB"), c, "deleting endpointGroup")
	assertNoErr(its.client.PolicyDelete("default", "policy"), c, "deleting policy")
	assertNoErr(its.client.NetworkDelete("default", "test"), c, "deleting network")
}
