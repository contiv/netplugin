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

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/contivmodel/client"

	. "gopkg.in/check.v1"
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
