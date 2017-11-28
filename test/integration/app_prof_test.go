/***
Copyright 2016 Cisco Systems Inc. All rights reserved.

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
	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/contivmodel/client"

	. "github.com/contiv/check"
)

// TestSingleAppProfile verifies a simple app-profile creation
func (its *integTestSuite) TestSingleAppProfile(c *C) {

	if its.fabricMode != "aci" {
		return // run only in aci mode
	}

	// Create a tenant
	c.Assert(its.client.TenantPost(&client.Tenant{
		TenantName: "BBTenant",
	}), IsNil)

	// Create an external contract
	err := its.client.ExtContractsGroupPost(&client.ExtContractsGroup{
		TenantName:         "BBTenant",
		ContractsGroupName: "extETCD",
		ContractsType:      "consumed",
		Contracts:          []string{"uni/tn-BBTenant/brc-etcdAllow"},
	})
	assertNoErr(err, c, "creating external contract")

	// Create another external contract
	err = its.client.ExtContractsGroupPost(&client.ExtContractsGroup{
		TenantName:         "BBTenant",
		ContractsGroupName: "extWeb",
		ContractsType:      "provided",
		Contracts:          []string{"uni/tn-BBTenant/brc-webAllow"},
	})
	assertNoErr(err, c, "creating external contract")

	// Create a network
	err = its.client.NetworkPost(&client.Network{
		TenantName:  "BBTenant",
		NetworkName: "NetNet",
		Subnet:      "29.1.1.0/24",
		Encap:       its.encap,
	})
	assertNoErr(err, c, "creating network")

	// Create a policy
	c.Assert(its.client.PolicyPost(&client.Policy{
		PolicyName: "Policy1",
		TenantName: "BBTenant",
	}), IsNil)

	// Create a epg
	err = its.client.EndpointGroupPost(&client.EndpointGroup{
		TenantName:       "BBTenant",
		NetworkName:      "NetNet",
		GroupName:        "epgA",
		Policies:         []string{"Policy1"},
		ExtContractsGrps: []string{"extWeb", "extETCD"},
	})
	assertNoErr(err, c, "creating endpointgroup")

	// Create another epg
	err = its.client.EndpointGroupPost(&client.EndpointGroup{
		TenantName:       "BBTenant",
		NetworkName:      "NetNet",
		GroupName:        "epgB",
		ExtContractsGrps: []string{"extETCD"},
	})
	assertNoErr(err, c, "creating endpointgroup")

	// add some rules to the policy
	rules := []*client.Rule{
		{
			RuleID:            "1",
			PolicyName:        "Policy1",
			TenantName:        "BBTenant",
			Direction:         "in",
			Protocol:          "tcp",
			Action:            "allow",
			FromEndpointGroup: "epgB",
		},
		{
			RuleID:            "2",
			PolicyName:        "Policy1",
			TenantName:        "BBTenant",
			Priority:          100,
			Direction:         "in",
			Protocol:          "udp",
			Port:              8000,
			Action:            "allow",
			FromEndpointGroup: "epgB",
		},
	}
	for _, rule := range rules {
		c.Assert(its.client.RulePost(rule), IsNil)
	}

	// if aciGw present, delete it
	its.client.AciGwDelete("aciGw")

	// Create an app-profile
	err = its.client.AppProfilePost(&client.AppProfile{
		TenantName:     "BBTenant",
		AppProfileName: "TestProfile",
		EndpointGroups: []string{"epgA", "epgB"},
	})
	assertErr(err, c, "creating application-profile without aci-gw config")

	// set aci-gw config
	err = its.client.AciGwPost(&client.AciGw{
		Name:                "aciGw",
		PhysicalDomain:      "testDomain",
		EnforcePolicies:     "yes",
		IncludeCommonTenant: "no",
	})
	assertNoErr(err, c, "creating aciGw config")

	// Create an app-profile
	err = its.client.AppProfilePost(&client.AppProfile{
		TenantName:     "BBTenant",
		AppProfileName: "TestProfile",
		EndpointGroups: []string{"epgA", "epgB"},
	})
	assertErr(err, c, "creating application-profile without bindings")

	// set aci-gw config
	err = its.client.AciGwPost(&client.AciGw{
		Name:                "aciGw",
		PathBindings:        "topology/pod-1/paths-101/pathep-[eth1/14]",
		PhysicalDomain:      "testDomain",
		EnforcePolicies:     "yes",
		IncludeCommonTenant: "no",
	})
	assertNoErr(err, c, "creating aciGw config")

	// Create an app-profile
	err = its.client.AppProfilePost(&client.AppProfile{
		TenantName:     "BBTenant",
		AppProfileName: "TestProfile",
		EndpointGroups: []string{"epgA", "epgB"},
	})
	assertNoErr(err, c, "creating application-profile")

	gwInsp, err := its.client.AciGwInspect("aciGw")
	assertNoErr(err, c, "inspecting aciGw")
	c.Assert(gwInsp.Oper.NumAppProfiles, Equals, 1)

	// verify tenant state is correct
	insp, err := its.client.TenantInspect("BBTenant")
	assertNoErr(err, c, "inspecting tenant")
	log.Infof("Inspecting tenant: %+v", insp)
	c.Assert(insp.Oper.TotalEndpoints, Equals, 0)
	c.Assert(insp.Oper.TotalNetworks, Equals, 1)
	c.Assert(insp.Oper.TotalEPGs, Equals, 2)
	c.Assert(insp.Oper.TotalPolicies, Equals, 1)
	c.Assert(insp.Oper.TotalAppProfiles, Equals, 1)

	assertNoErr(its.client.AppProfileDelete("BBTenant", "TestProfile"), c, "deleting app profile")
	gwInsp, err = its.client.AciGwInspect("aciGw")
	assertNoErr(err, c, "inspecting aciGw")
	c.Assert(gwInsp.Oper.NumAppProfiles, Equals, 0)

	assertNoErr(its.client.EndpointGroupDelete("BBTenant", "epgA"), c, "deleting endpointGroup")
	assertNoErr(its.client.EndpointGroupDelete("BBTenant", "epgB"), c, "deleting endpointGroup")
	assertNoErr(its.client.PolicyDelete("BBTenant", "Policy1"), c, "deleting policy")
	assertNoErr(its.client.NetworkDelete("BBTenant", "NetNet"), c, "deleting network")
	assertNoErr(its.client.ExtContractsGroupDelete("BBTenant", "extWeb"), c, "deleting ext contract")
	assertNoErr(its.client.ExtContractsGroupDelete("BBTenant", "extETCD"), c, "deleting ext contract")
	assertNoErr(its.client.TenantDelete("BBTenant"), c, "deleting Tenant")
	assertNoErr(its.client.AciGwDelete("aciGw"), c, "deleting aci gw config")
}

func (its *integTestSuite) TestMultiAppProfile(c *C) {
	if its.fabricMode != "aci" {
		return // run only in aci mode
	}

	// Create a tenant
	c.Assert(its.client.TenantPost(&client.Tenant{
		TenantName: "AATenant",
	}), IsNil)

	// Create an external contract
	err := its.client.ExtContractsGroupPost(&client.ExtContractsGroup{
		TenantName:         "AATenant",
		ContractsGroupName: "extETCD",
		ContractsType:      "consumed",
		Contracts:          []string{"uni/tn-AATenant/brc-etcdAllow"},
	})
	assertNoErr(err, c, "creating external contract")

	// Create a network
	err = its.client.NetworkPost(&client.Network{
		TenantName:  "AATenant",
		NetworkName: "NetNet",
		Subnet:      "29.1.1.0/24",
		Encap:       its.encap,
	})
	assertNoErr(err, c, "creating network")

	// Create a policy
	c.Assert(its.client.PolicyPost(&client.Policy{
		PolicyName: "Policy1",
		TenantName: "AATenant",
	}), IsNil)

	// Create another policy
	c.Assert(its.client.PolicyPost(&client.Policy{
		PolicyName: "Policy2",
		TenantName: "AATenant",
	}), IsNil)

	// Create a epg
	err = its.client.EndpointGroupPost(&client.EndpointGroup{
		TenantName:       "AATenant",
		NetworkName:      "NetNet",
		GroupName:        "epgA",
		Policies:         []string{"Policy1"},
		ExtContractsGrps: []string{"extETCD"},
	})
	assertNoErr(err, c, "creating endpointgroup")

	// Create another epg
	err = its.client.EndpointGroupPost(&client.EndpointGroup{
		TenantName:  "AATenant",
		NetworkName: "NetNet",
		GroupName:   "epgB",
		Policies:    []string{"Policy2"},
	})
	assertNoErr(err, c, "creating endpointgroup")

	// Create another epg
	err = its.client.EndpointGroupPost(&client.EndpointGroup{
		TenantName:  "AATenant",
		NetworkName: "NetNet",
		GroupName:   "epgC",
	})
	assertNoErr(err, c, "creating endpointgroup")

	// add some rules to the policy
	rules := []*client.Rule{
		{
			RuleID:            "1",
			PolicyName:        "Policy1",
			TenantName:        "AATenant",
			Direction:         "in",
			Protocol:          "tcp",
			Action:            "allow",
			FromEndpointGroup: "epgB",
		},
		{
			RuleID:            "2",
			PolicyName:        "Policy1",
			TenantName:        "AATenant",
			Priority:          100,
			Direction:         "in",
			Protocol:          "udp",
			Port:              8000,
			Action:            "allow",
			FromEndpointGroup: "epgB",
		},
		{
			RuleID:          "3",
			PolicyName:      "Policy2",
			TenantName:      "AATenant",
			Priority:        100,
			Direction:       "out",
			Protocol:        "tcp",
			Port:            8001,
			Action:          "allow",
			ToEndpointGroup: "epgC",
		},
	}
	for _, rule := range rules {
		c.Assert(its.client.RulePost(rule), IsNil)
	}

	// set aci-gw config
	err = its.client.AciGwPost(&client.AciGw{
		Name:                "aciGw",
		PathBindings:        "topology/pod-1/paths-101/pathep-[eth1/14]",
		PhysicalDomain:      "testDomain",
		EnforcePolicies:     "yes",
		IncludeCommonTenant: "no",
	})
	assertNoErr(err, c, "creating aciGw config")

	// Create an app-profile
	err = its.client.AppProfilePost(&client.AppProfile{
		TenantName:     "AATenant",
		AppProfileName: "TestProfile1",
		EndpointGroups: []string{"epgA", "epgB"},
	})
	assertNoErr(err, c, "creating application-profile")

	// Create another app-profile
	err = its.client.AppProfilePost(&client.AppProfile{
		TenantName:     "AATenant",
		AppProfileName: "TestProfile2",
		EndpointGroups: []string{"epgC"},
	})
	assertNoErr(err, c, "creating application-profile")

	// verify tenant state is correct
	insp, err := its.client.TenantInspect("AATenant")
	assertNoErr(err, c, "inspecting tenant")
	log.Infof("Inspecting tenant: %+v", insp)
	c.Assert(insp.Oper.TotalEndpoints, Equals, 0)
	c.Assert(insp.Oper.TotalNetworks, Equals, 1)
	c.Assert(insp.Oper.TotalEPGs, Equals, 3)
	c.Assert(insp.Oper.TotalPolicies, Equals, 2)
	c.Assert(insp.Oper.TotalAppProfiles, Equals, 2)

	assertNoErr(its.client.AppProfileDelete("AATenant", "TestProfile1"), c, "deleting app profile")
	assertNoErr(its.client.AppProfileDelete("AATenant", "TestProfile2"), c, "deleting app profile")
	assertNoErr(its.client.EndpointGroupDelete("AATenant", "epgA"), c, "deleting endpointGroup")
	assertNoErr(its.client.EndpointGroupDelete("AATenant", "epgB"), c, "deleting endpointGroup")
	assertNoErr(its.client.EndpointGroupDelete("AATenant", "epgC"), c, "deleting endpointGroup")
	assertNoErr(its.client.PolicyDelete("AATenant", "Policy1"), c, "deleting policy")
	assertNoErr(its.client.PolicyDelete("AATenant", "Policy2"), c, "deleting policy")
	assertNoErr(its.client.NetworkDelete("AATenant", "NetNet"), c, "deleting network")
	assertNoErr(its.client.TenantDelete("AATenant"), c, "deleting Tenant")
}
