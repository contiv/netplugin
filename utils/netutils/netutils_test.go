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

package netutils

import (
	"fmt"
	"testing"
)

type testSubnetInfo struct {
	subnetIP  string
	subnetLen uint
	hostID    uint
	hostIP    string
}

var testSubnets = []testSubnetInfo{
	{subnetIP: "11.2.1.0", subnetLen: 24, hostID: 5, hostIP: "11.2.1.5"},
	{subnetIP: "10.123.16.0", subnetLen: 22, hostID: 513, hostIP: "10.123.18.1"},
	{subnetIP: "172.12.0.0", subnetLen: 16, hostID: 261, hostIP: "172.12.1.5"},
}

func TestGetSubnetIP(t *testing.T) {
	for _, te := range testSubnets {
		hostIP, err := GetSubnetIP(te.subnetIP, te.subnetLen, 32, te.hostID)
		if err != nil {
			t.Fatalf("error getting host ip from subnet %s/%d for hostid %d - err '%s'",
				te.subnetIP, te.subnetLen, te.hostID, err)
		}
		if hostIP != te.hostIP {
			t.Fatalf("obtained ip %s doesn't match expected ip %s for subnet %s/%d\n",
				hostIP, te.hostIP, te.subnetIP, te.subnetLen)
		}
	}
}

var testInvalidSubnets = []testSubnetInfo{
	{subnetIP: "11.2.1.0", subnetLen: 32, hostID: 5, hostIP: "11.2.1.5"},
	{subnetIP: "10.123.16.0", subnetLen: 22, hostID: 1025, hostIP: "10.123.18.1"},
	{subnetIP: "172.12.0.0", subnetLen: 4, hostID: 261, hostIP: "172.12.1.5"},
}

func TestInvalidGetSubnetIP(t *testing.T) {
	for _, te := range testInvalidSubnets {
		_, err := GetSubnetIP(te.subnetIP, te.subnetLen, 32, te.hostID)
		if err == nil {
			t.Fatalf("Expecting error on invalid config subnet %s/%d for hostid %d",
				te.subnetIP, te.subnetLen, te.hostID)
		}
	}
}

func TestGetIPNumber(t *testing.T) {
	for _, te := range testSubnets {
		hostID, err := GetIPNumber(te.subnetIP, te.subnetLen, 32, te.hostIP)
		if err != nil {
			t.Fatalf("error getting host ip from subnet %s/%d for hostid %d ",
				te.subnetIP, te.subnetLen, te.hostID)
		}
		if hostID != te.hostID {
			t.Fatalf("obtained ip %d doesn't match with expected ip %d \n",
				hostID, te.hostID)
		}
	}
}

var testValidNetParams = []testSubnetInfo{
	{subnetIP: "11.2.1.10-100", subnetLen: 24},
	{subnetIP: "10.123.16.0", subnetLen: 26},
	{subnetIP: "10.123.16.66", subnetLen: 26},
	{subnetIP: "10.123.16.10-100", subnetLen: 22},
}

func TestValidNetworkRangeParams(t *testing.T) {
	for _, tvnp := range testValidNetParams {
		err := ValidateNetworkRangeParams(tvnp.subnetIP, tvnp.subnetLen)
		if err != nil {
			t.Fatalf("Error validating network params for subnet: %s/%d",
				tvnp.subnetIP, tvnp.subnetLen)
		}
	}
}

var testInvalidNetParams = []testSubnetInfo{
	{subnetIP: "10.123.16.10-100", subnetLen: 26},
	{subnetIP: "10.123.16.60-100", subnetLen: 26},
}

func TestInvalidNetworkRangeParams(t *testing.T) {
	for _, tinp := range testInvalidNetParams {
		err := ValidateNetworkRangeParams(tinp.subnetIP, tinp.subnetLen)
		if err == nil {
			t.Fatalf("Expected error for invalid network params for subnet: %s/%d",
				tinp.subnetIP, tinp.subnetLen)
		}
	}
}

type testSubnetAddrInfo struct {
	ipRange        string
	subnetLen      uint
	expectedSubnet string
}

var testSubnetAddr = []testSubnetAddrInfo{
	{ipRange: "10.1.1.10", subnetLen: 16, expectedSubnet: "10.1.0.0"},
	{ipRange: "10.1.1.10-20", subnetLen: 24, expectedSubnet: "10.1.1.0"},
	{ipRange: "10.1.1.100", subnetLen: 26, expectedSubnet: "10.1.1.64"},
	{ipRange: "10.1.1.70-100", subnetLen: 26, expectedSubnet: "10.1.1.64"},
}

func TestGetSubnetAddr(t *testing.T) {
	for _, addr := range testSubnetAddr {
		subnetAddr := GetSubnetAddr(addr.ipRange, addr.subnetLen)
		if subnetAddr != addr.expectedSubnet {
			t.Fatalf("Invalid subnet address %s obtained for IPRange: %s/%d. Expected: %s",
				subnetAddr, addr.ipRange, addr.subnetLen, addr.expectedSubnet)
		}
	}
}

func TestValidRange(t *testing.T) {
	rangeStr := "5-100, 101-200"
	_, err := ParseTagRanges(rangeStr, "vlan")
	if err != nil {
		t.Fatalf("error '%s' parsing valid vlan range '%s'\n", err, rangeStr)
	}
}

func TestInvalidVlanRange(t *testing.T) {
	rangeStr := "5--100, 101-200"
	_, err := ParseTagRanges(rangeStr, "vlan")
	if err == nil {
		t.Fatalf("successfully parsed invalid vlan range '%s'\n", rangeStr)
	}
}

func TestInvalidVlanValue(t *testing.T) {
	rangeStr := "5-100, 101-5000"
	_, err := ParseTagRanges(rangeStr, "vlan")
	if err == nil {
		t.Fatalf("successfully parsed invalid vlan id '%s'\n", rangeStr)
	}
}

func TestInvalidMinMaxVlan(t *testing.T) {
	rangeStr := "5-100, 200-101"
	_, err := ParseTagRanges(rangeStr, "vlan")
	if err == nil {
		t.Fatalf("successfully parsed invalid min-max vlan values '%s'\n", rangeStr)
	}
}

func TestInvalidRangeExtraSeperators(t *testing.T) {
	rangeStr := "5-100,,101-200"
	_, err := ParseTagRanges(rangeStr, "vlan")
	if err == nil {
		t.Fatalf("successfully parsed vlan range with extra seperators '%s'\n", rangeStr)
	}
}

func TestValidVxlanRange(t *testing.T) {
	rangeStr := "10000-16000"
	_, err := ParseTagRanges(rangeStr, "vxlan")
	if err != nil {
		t.Fatalf("error '%s' parsing valid vxlan range '%s'\n", err, rangeStr)
	}
}

func TestInvalidVxlanMultipleRanges(t *testing.T) {
	rangeStr := "101-400, 10000-15000"
	_, err := ParseTagRanges(rangeStr, "vxlan")
	if err == nil {
		t.Fatalf("successfully parsed invalid vxlan value '%s'\n", rangeStr)
	}
}

func TestInvalidVxlanValue(t *testing.T) {
	rangeStr := "101-75535"
	_, err := ParseTagRanges(rangeStr, "vxlan")
	if err == nil {
		t.Fatalf("successfully parsed invalid vxlan value '%s'\n", rangeStr)
	}
}

func TestInvalidMinMaxVxlan(t *testing.T) {
	rangeStr := "8000-7999"
	_, err := ParseTagRanges(rangeStr, "vxlan")
	if err == nil {
		t.Fatalf("successfully parsed invalid min-max vxlan values '%s'\n", rangeStr)
	}
}

type testSubnetAllocInfo struct {
	subnetIP       string
	subnetLen      uint
	subnetAllocLen uint
	hostID         uint
	hostIP         string
}

var testSubnetAllocs = []testSubnetAllocInfo{
	{subnetIP: "11.1.0.0", subnetLen: 16, subnetAllocLen: 24,
		hostID: 5, hostIP: "11.1.5.0"},
	{subnetIP: "10.0.0.0", subnetLen: 8, subnetAllocLen: 24,
		hostID: 5, hostIP: "10.0.5.0"},
}

func TestGetSubnetAlloc(t *testing.T) {
	for _, te := range testSubnetAllocs {
		hostIP, err := GetSubnetIP(te.subnetIP, te.subnetLen,
			te.subnetAllocLen, te.hostID)
		if err != nil {
			t.Fatalf("error getting subnet ip from subnet-range %s/%d alloc-size %d "+
				"for id %d - err '%s'",
				te.subnetIP, te.subnetLen, te.subnetAllocLen, te.hostID, err)
		}
		if hostIP != te.hostIP {
			t.Fatalf("obtained ip %s doesn't match expected ip %s for subnet %s/%d "+
				"for AllocLen %d \n",
				hostIP, te.hostIP, te.subnetIP, te.subnetLen, te.subnetAllocLen)
		}
	}
}

func TestGetSubnetNumber(t *testing.T) {
	for _, te := range testSubnetAllocs {
		hostID, err := GetIPNumber(te.subnetIP, te.subnetLen,
			te.subnetAllocLen, te.hostIP)
		if err != nil {
			t.Fatalf("error getting subnet ip from subnet %s/%d for hostid %d "+
				"for subnet alloc size %d \n",
				te.subnetIP, te.subnetLen, te.hostID, te.subnetAllocLen)
		}
		if hostID != te.hostID {
			t.Fatalf("obtained subnet ip %d doesn't match with expected ip %d "+
				"for subnet %s/%d alloc size %d \n",
				hostID, te.hostID, te.subnetIP, te.subnetLen, te.subnetAllocLen)
		}
	}
}

func TestGetAddrList(t *testing.T) {
	addrList, err := GetNetlinkAddrList()
	if err != nil {
		t.Fatalf("Error getting address list. Err: %v", err)
	}

	fmt.Printf("Got netlink address list: %v\n", addrList)

	addrList, err = GetLocalAddrList()
	if err != nil {
		t.Fatalf("Error getting address list. Err: %v", err)
	}

	fmt.Printf("Got local address list: %v\n", addrList)
}
