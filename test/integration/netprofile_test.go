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
	"time"

	"github.com/contiv/netplugin/contivmodel/client"

	. "github.com/contiv/check"
)

// TestNetprofileDscp tests dscp settings in net profile
func (its *integTestSuite) TestNetprofileDscp(c *C) {
	// Create a network
	err := its.client.NetworkPost(&client.Network{
		TenantName:  "default",
		NetworkName: "test",
		Subnet:      "10.1.1.0/24",
		Encap:       its.encap,
	})
	assertNoErr(err, c, "creating network")

	for i := 0; i < its.iterations; i++ {
		// create an epg
		c.Assert(its.client.EndpointGroupPost(&client.EndpointGroup{
			TenantName:  "default",
			NetworkName: "test",
			GroupName:   "epg1",
		}), IsNil)

		// create a net profile
		c.Assert(its.client.NetprofilePost(&client.Netprofile{
			TenantName:  "default",
			ProfileName: "profile1",
			DSCP:        10,
		}), IsNil)

		addr, err := its.allocAddress("", "test.default", "")
		assertNoErr(err, c, "allocating address")
		c.Assert(addr, Equals, "10.1.1.1")

		// create an endpoint in the epg
		epCfg1, err := its.createEndpoint("default", "test", "epg1", addr, "")
		assertNoErr(err, c, "creating endpoint")

		// verify port vlan flow is created
		its.verifyPortVlanFlow(epCfg1, 0, c)

		// add net profile to epg
		c.Assert(its.client.EndpointGroupPost(&client.EndpointGroup{
			TenantName:  "default",
			NetworkName: "test",
			GroupName:   "epg1",
			NetProfile:  "profile1",
		}), IsNil)

		// wait for a little for update to propagate
		time.Sleep(300 * time.Millisecond)

		// verify dscp flow is created
		its.verifyPortVlanFlow(epCfg1, 10, c)

		// change DSCP value
		c.Assert(its.client.NetprofilePost(&client.Netprofile{
			TenantName:  "default",
			ProfileName: "profile1",
			DSCP:        20,
		}), IsNil)

		// wait for a little for update to propagate
		time.Sleep(300 * time.Millisecond)

		// verify dscp value changes in OVS
		its.verifyPortVlanFlow(epCfg1, 20, c)

		// clear DSCP value
		c.Assert(its.client.NetprofilePost(&client.Netprofile{
			TenantName:  "default",
			ProfileName: "profile1",
			DSCP:        0,
		}), IsNil)

		// wait for a little for update to propagate
		time.Sleep(300 * time.Millisecond)

		// verify dscp flow is removed
		its.verifyPortVlanFlowRemoved(epCfg1, 20, true, c)

		// add DSCP value
		c.Assert(its.client.NetprofilePost(&client.Netprofile{
			TenantName:  "default",
			ProfileName: "profile1",
			DSCP:        30,
		}), IsNil)

		// wait for a little for update to propagate
		time.Sleep(300 * time.Millisecond)

		// verify dscp value changes in OVS
		its.verifyPortVlanFlow(epCfg1, 30, c)

		// create a new net profile
		c.Assert(its.client.NetprofilePost(&client.Netprofile{
			TenantName:  "default",
			ProfileName: "profile2",
			DSCP:        40,
		}), IsNil)

		// add new net profile to epg
		c.Assert(its.client.EndpointGroupPost(&client.EndpointGroup{
			TenantName:  "default",
			NetworkName: "test",
			GroupName:   "epg1",
			NetProfile:  "profile2",
		}), IsNil)

		// wait for a little for update to propagate
		time.Sleep(300 * time.Millisecond)

		// verify dscp value changes in OVS
		its.verifyPortVlanFlow(epCfg1, 40, c)

		// remove net profile from epg
		c.Assert(its.client.EndpointGroupPost(&client.EndpointGroup{
			TenantName:  "default",
			NetworkName: "test",
			GroupName:   "epg1",
			NetProfile:  "",
		}), IsNil)

		// wait for a little for update to propagate
		time.Sleep(300 * time.Millisecond)

		// verify dscp flows are gone
		its.verifyPortVlanFlowRemoved(epCfg1, 40, true, c)

		// delete the endpoint
		err = its.deleteEndpoint("default", "test", "", epCfg1)
		assertNoErr(err, c, "deleting endpoint")

		// delete epg
		err = its.client.EndpointGroupDelete("default", "epg1")
		assertNoErr(err, c, "deleting epg")

		// delete profiles
		assertNoErr(its.client.NetprofileDelete("default", "profile1"), c, "deleting netprofile")
		assertNoErr(its.client.NetprofileDelete("default", "profile2"), c, "deleting netprofile")

		// verify flows are also gone
		its.verifyEndpointFlowRemoved(epCfg1, c)
		its.verifyPortVlanFlowRemoved(epCfg1, 40, false, c)
	}

	// delete the network
	assertNoErr(its.client.NetworkDelete("default", "test"), c, "deleting network")
}

func (its *integTestSuite) TestNetprofileBandwidth(c *C) {
	// Create a network
	err := its.client.NetworkPost(&client.Network{
		TenantName:  "default",
		NetworkName: "test",
		Subnet:      "10.1.1.0/24",
		Encap:       its.encap,
	})
	assertNoErr(err, c, "creating network")

	for i := 0; i < its.iterations; i++ {
		// create an epg
		c.Assert(its.client.EndpointGroupPost(&client.EndpointGroup{
			TenantName:  "default",
			NetworkName: "test",
			GroupName:   "epg1",
		}), IsNil)

		// create a net profile
		c.Assert(its.client.NetprofilePost(&client.Netprofile{
			TenantName:  "default",
			ProfileName: "profile1",
			Bandwidth:   "10Mbps",
			Burst:       100,
		}), IsNil)

		addr, err := its.allocAddress("", "test.default", "")
		assertNoErr(err, c, "allocating address")
		c.Assert(addr, Equals, "10.1.1.1")

		// create an endpoint in the epg
		epCfg1, err := its.createEndpoint("default", "test", "epg1", addr, "")
		assertNoErr(err, c, "creating endpoint")

		// verify tc qdisc is empty
		c.Assert(tcFilterVerifyEmpty(20), IsNil)

		// add net profile to epg
		c.Assert(its.client.EndpointGroupPost(&client.EndpointGroup{
			TenantName:  "default",
			NetworkName: "test",
			GroupName:   "epg1",
			NetProfile:  "profile1",
		}), IsNil)

		// wait for a little for update to propagate
		time.Sleep(1 * time.Second)

		// verify TC bandwidth
		c.Assert(tcFilterCheckBwRetry(10, 100), IsNil)

		// change bandwidthvalue
		c.Assert(its.client.NetprofilePost(&client.Netprofile{
			TenantName:  "default",
			ProfileName: "profile1",
			Bandwidth:   "20Mbps",
			Burst:       200,
		}), IsNil)

		// wait for a little for update to propagate
		time.Sleep(1 * time.Second)

		// verify TC bandwidth
		c.Assert(tcFilterCheckBwRetry(20, 200), IsNil)

		// clear bandwidth value
		c.Assert(its.client.NetprofilePost(&client.Netprofile{
			TenantName:  "default",
			ProfileName: "profile1",
			Bandwidth:   "",
			Burst:       0,
		}), IsNil)

		// wait for a little for update to propagate
		time.Sleep(300 * time.Millisecond)

		// verify tc qdisc is empty
		c.Assert(tcFilterVerifyEmpty(20), IsNil)

		// add bandwidth again
		c.Assert(its.client.NetprofilePost(&client.Netprofile{
			TenantName:  "default",
			ProfileName: "profile1",
			Bandwidth:   "30Mbps",
			Burst:       300,
		}), IsNil)

		// wait for a little for update to propagate
		time.Sleep(1 * time.Second)

		// verify TC bandwidth
		c.Assert(tcFilterCheckBwRetry(30, 300), IsNil)

		// create a new net profile
		c.Assert(its.client.NetprofilePost(&client.Netprofile{
			TenantName:  "default",
			ProfileName: "profile2",
			Bandwidth:   "40Mbps",
			Burst:       400,
		}), IsNil)

		// add new net profile to epg
		c.Assert(its.client.EndpointGroupPost(&client.EndpointGroup{
			TenantName:  "default",
			NetworkName: "test",
			GroupName:   "epg1",
			NetProfile:  "profile2",
		}), IsNil)

		// wait for a little for update to propagate
		time.Sleep(1 * time.Second)

		// verify TC bandwidth
		c.Assert(tcFilterCheckBwRetry(40, 400), IsNil)

		// remove net profile from epg
		c.Assert(its.client.EndpointGroupPost(&client.EndpointGroup{
			TenantName:  "default",
			NetworkName: "test",
			GroupName:   "epg1",
			NetProfile:  "",
		}), IsNil)

		// wait for a little for update to propagate
		time.Sleep(1 * time.Second)

		// verify tc qdisc is empty
		c.Assert(tcFilterVerifyEmpty(20), IsNil)

		// delete the endpoint
		err = its.deleteEndpoint("default", "test", "", epCfg1)
		assertNoErr(err, c, "deleting endpoint")

		// delete epg
		err = its.client.EndpointGroupDelete("default", "epg1")
		assertNoErr(err, c, "deleting epg")

		// delete profiles
		assertNoErr(its.client.NetprofileDelete("default", "profile1"), c, "deleting netprofile")
		assertNoErr(its.client.NetprofileDelete("default", "profile2"), c, "deleting netprofile")
	}

	// delete the network
	assertNoErr(its.client.NetworkDelete("default", "test"), c, "deleting network")
}
