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
	"github.com/jainvipin/bitset"
	"testing"
)

type testSubnetInfo struct {
	subnetIP      string
	subnetLen     uint
	hostID        uint
	hostIP        string
	ipv6Subnet    string
	ipv6SubnetLen uint
	ipv6HostID    string
	ipv6HostIP    string
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
	{subnetIP: "11.2.1.10-11.2.1.100", subnetLen: 24},
	{subnetIP: "10.123.16.0", subnetLen: 26},
	{subnetIP: "10.123.16.66", subnetLen: 26},
	{subnetIP: "10.123.16.10-10.123.16.100", subnetLen: 22},
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
	{subnetIP: "10.123.16.10-10.123.16.100", subnetLen: 26},
	{subnetIP: "10.123.16.60-10.123.16.100", subnetLen: 26},
	{subnetIP: "10.1.1.0", subnetLen: 1},
	{subnetIP: "10.1.1.0", subnetLen: 33},
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

var testv6Subnets = []testSubnetInfo{
	{ipv6Subnet: "2016:430::", ipv6SubnetLen: 64, ipv6HostID: "::254", ipv6HostIP: "2016:430::254"},
	{ipv6Subnet: "babe:face::80", ipv6SubnetLen: 121, ipv6HostID: "::45", ipv6HostIP: "babe:face::c5"},
	{ipv6Subnet: "2001::", ipv6SubnetLen: 100, ipv6HostID: "::2", ipv6HostIP: "2001::2"},
}

func TestGetSubnetIPv6(t *testing.T) {
	for _, te := range testv6Subnets {
		ipv6HostIP, err := GetSubnetIPv6(te.ipv6Subnet, te.ipv6SubnetLen, te.ipv6HostID)
		if err != nil {
			t.Fatalf("error getting host ip from ipv6Subnet %s/%d for hostid %s - err '%s'",
				te.ipv6Subnet, te.ipv6SubnetLen, te.ipv6HostID, err)
		}
		if ipv6HostIP != te.ipv6HostIP {
			t.Fatalf("obtained ip %s doesn't match expected ip %s for ipv6Subnet %s/%d\n",
				ipv6HostIP, te.ipv6HostIP, te.ipv6Subnet, te.ipv6SubnetLen)
		}
	}
}

var testInvalidv6Subnets = []testSubnetInfo{
	{ipv6Subnet: "2016:430::", ipv6SubnetLen: 128, ipv6HostID: "::254"},
	{ipv6Subnet: "babe:face::80", ipv6SubnetLen: 121, ipv6HostID: "::c5"},
	{ipv6Subnet: "babe:face::80", ipv6SubnetLen: 121, ipv6HostID: "::105"},
	{ipv6Subnet: "2001::", ipv6SubnetLen: 130, ipv6HostID: "::2", ipv6HostIP: "2001::2"},
}

func TestInvalidGetSubnetIPv6(t *testing.T) {
	for _, te := range testInvalidSubnets {
		_, err := GetSubnetIPv6(te.ipv6Subnet, te.ipv6SubnetLen, te.ipv6HostID)
		if err == nil {
			t.Fatalf("Expecting error on invalid config subnet %s/%d for hostid %s",
				te.ipv6Subnet, te.ipv6SubnetLen, te.ipv6HostID)
		}
	}
}

type testHostID struct {
	hostID     string
	nextHostID string
	subnetAddr string
	subnetLen  uint
}

var testHostIDs = []testHostID{
	// verify initial case
	{hostID: "", nextHostID: "::1", subnetAddr: "1234::", subnetLen: 100},

	// lasthost set to 0 and ::1 allocated above
	{hostID: "::", nextHostID: "::2", subnetAddr: "1234::", subnetLen: 100},

	// find next of 4
	{hostID: "::4:FFFF:FFFF", nextHostID: "::5:0:0", subnetAddr: "1234::", subnetLen: 100},

	// this time next of 4 should skip 5, as it was alloc'd above
	{hostID: "::4:ffff:ffff", nextHostID: "::5:0:1", subnetAddr: "1234::", subnetLen: 100},

	// verify the corner case
	{hostID: "::FFFE", nextHostID: "::ffff", subnetAddr: "1234::", subnetLen: 100},

	// verify the wrap around case, with subnetLen 124
	{hostID: "::F", nextHostID: "::3", subnetAddr: "1234::", subnetLen: 124},
}

func TestGetNextIPv6HostID(t *testing.T) {
	var allocMap map[string]bool
	for _, te := range testHostIDs {
		nextHostID, err := GetNextIPv6HostID(te.hostID, te.subnetAddr, te.subnetLen, allocMap)
		if nextHostID != te.nextHostID || err != nil {
			t.Fatalf("obtained nextHostID %s doesn't match expected ID %s for %s\n",
				nextHostID, te.nextHostID, te.hostID)
		}
		ReserveIPv6HostID(nextHostID, &allocMap)
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

func TestGetIPRange(t *testing.T) {
	testData := []struct {
		subnetIP  string
		subnetLen uint
		startIdx  uint
		endIdx    uint
		result    string
	}{
		{subnetIP: "10.36.1.0", subnetLen: 24, startIdx: 1, endIdx: 2, result: "10.36.1.1-10.36.1.2"},
		{subnetIP: "11.2.1.0", subnetLen: 24, startIdx: 5, endIdx: 11, result: "11.2.1.5-11.2.1.11"},
		{subnetIP: "10.123.16.0", subnetLen: 22, startIdx: 1020, endIdx: 1022, result: "10.123.19.252-10.123.19.254"},
		{subnetIP: "172.12.0.0", subnetLen: 8, startIdx: 261, endIdx: 266, result: "172.12.1.5-172.12.1.10"},
		{subnetIP: "10.36.2.0", subnetLen: 24, startIdx: 5, endIdx: 6, result: "10.36.2.5-10.36.2.6"},
	}
	for _, i := range testData {
		r := getIPRange(i.subnetIP, i.subnetLen, i.startIdx, i.endIdx)
		if r != i.result {
			t.Fatalf("test failed: got %s instead of %s for %+v", r, i.result, i)
		}
	}
}

func assertOnTrue(t *testing.T, val bool, msg string) {
	if val == true {
		t.Fatalf("Error %s", msg)
	}
	// else continue
}

func TestSetIPAddrRange(t *testing.T) {
	var amap bitset.BitSet
	var err error

	testAddrRange := []struct {
		addrRange []string
		subnetIP  string
		subnetLen uint
		status    bool
		allocAddr string
		freeAddr  string
	}{
		{addrRange: []string{"10.36.1.1-10.36.1.10"}, subnetIP: "10.36.1.0", subnetLen: 24, status: true,
			allocAddr: "10.36.1.1-10.36.1.10", freeAddr: "10.36.1.11-10.36.1.254"},
		{addrRange: []string{"10.36.1.1-10.36.1.10", "10.36.1.11-10.36.1.254"}, subnetIP: "10.36.1.0",
			subnetLen: 24, status: true, allocAddr: "10.36.1.1-10.36.1.254", freeAddr: ""},
		{addrRange: []string{"10.36.1.1-10.36.1.10", "10.36.1.31-10.36.1.54"}, subnetIP: "10.36.1.0",
			subnetLen: 24, status: true, allocAddr: "10.36.1.1-10.36.1.10, 10.36.1.31-10.36.1.54",
			freeAddr: "10.36.1.11-10.36.1.30, 10.36.1.55-10.36.1.254"},
		{addrRange: []string{"10.36.2.1-10.36.2.10", "10.36.0.1-10.36.0.10"}, subnetIP: "10.36.0.0",
			subnetLen: 16, status: true, allocAddr: "10.36.0.1-10.36.0.10, 10.36.2.1-10.36.2.10",
			freeAddr: "10.36.0.11-10.36.2.0, 10.36.2.11-10.36.255.254"},
		{addrRange: []string{"10.36.1.1-10.36.1.10", "10.36.1.1-10.36.1.10"}, subnetIP: "10.36.2.0",
			subnetLen: 24, status: false},
	}

	for _, i := range testAddrRange {
		amap.ClearAll()
		InitSubnetBitset(&amap, i.subnetLen)
		for _, pool := range i.addrRange {
			err = SetIPAddrRange(&amap, pool, i.subnetIP, i.subnetLen)
			assertOnTrue(t, (err != nil) == (i.status == true), fmt.Sprintf("set failed %s %+v", err, i))
		}

		if i.status == true {
			assertOnTrue(t, err != nil, fmt.Sprintf("err: %s, failed for data %+v", err, i))
			f := ListAvailableIPs(amap, i.subnetIP, i.subnetLen)
			assertOnTrue(t, f != i.freeAddr, fmt.Sprintf("got avail addr: [%s], expected [%s] failed for data %+v",
				f, i.freeAddr, i))
			a := ListAllocatedIPs(amap, i.subnetIP, i.subnetIP, i.subnetLen)
			assertOnTrue(t, a != i.allocAddr, fmt.Sprintf("got allocated addr: [%s], expected [%s] failed for data %+v",
				a, i.allocAddr, i))
		} else {
			assertOnTrue(t, err == nil, fmt.Sprintf("err: %s, failed for data %+v", err, i))
		}
	}

}

func TestVerifyIPAddrRange(t *testing.T) {
	var amap bitset.BitSet
	var err error

	testAddrRange := []struct {
		addrRange []string
		testRange string
		subnetIP  string
		subnetLen uint
		status    bool
	}{
		{addrRange: []string{"10.36.1.1-10.36.1.10"}, testRange: "10.36.1.15-10.36.1.17", subnetIP: "10.36.1.0",
			subnetLen: 24, status: true},
		{addrRange: []string{"10.36.1.1-10.36.1.10", "10.36.1.11-10.36.1.204"}, testRange: "10.36.1.205-10.36.1.254",
			subnetIP: "10.36.1.0", subnetLen: 24, status: true},
		{addrRange: []string{"10.36.1.1-10.36.1.10", "10.36.1.11-10.36.1.54"}, testRange: "10.36.1.55-10.36.1.55",
			subnetIP: "10.36.1.0", subnetLen: 24, status: true},
		{addrRange: []string{"10.36.2.1-10.36.2.10", "10.36.0.1-10.36.0.10"}, subnetIP: "10.36.0.0",
			subnetLen: 16, status: false, testRange: "10.36.2.1-10.36.2.2"},
		{addrRange: []string{"10.36.1.1-10.36.1.10", "10.36.1.1-10.36.1.10"}, subnetIP: "10.36.1.0",
			testRange: "10.36.2.1-10.36.2.11", subnetLen: 24, status: false},
	}

	for _, i := range testAddrRange {
		amap.ClearAll()
		InitSubnetBitset(&amap, i.subnetLen)
		for _, pool := range i.addrRange {
			err = SetIPAddrRange(&amap, pool, i.subnetIP, i.subnetLen)
			assertOnTrue(t, err != nil, fmt.Sprintf("set failed %s %+v", err, i))
		}

		err = TestIPAddrRange(&amap, i.testRange, i.subnetIP, i.subnetLen)
		assertOnTrue(t, (err != nil) == (i.status == true), fmt.Sprintf("clear failed %s %+v", err, i))
	}
}

func TestClearIPAddrRange(t *testing.T) {
	var amap bitset.BitSet
	var err error

	testAddrRange := []struct {
		addrRange  []string
		clearRange string
		subnetIP   string
		subnetLen  uint
		status     bool
		allocAddr  string
		freeAddr   string
	}{
		{addrRange: []string{"10.36.1.1-10.36.1.10"}, clearRange: "10.36.1.5-10.36.1.7", subnetIP: "10.36.1.0",
			subnetLen: 24, status: true, allocAddr: "10.36.1.1-10.36.1.4, 10.36.1.8-10.36.1.10",
			freeAddr: "10.36.1.5-10.36.1.7, 10.36.1.11-10.36.1.254"},
		{addrRange: []string{"10.36.1.1-10.36.1.10", "10.36.1.11-10.36.1.254"}, clearRange: "10.36.1.101-10.36.1.254",
			subnetIP: "10.36.1.0", subnetLen: 24, status: true, allocAddr: "10.36.1.1-10.36.1.100",
			freeAddr: "10.36.1.101-10.36.1.254"},
		{addrRange: []string{"10.36.1.1-10.36.1.10", "10.36.1.11-10.36.1.54"}, clearRange: "10.36.1.1-10.36.1.54",
			subnetIP: "10.36.1.0", subnetLen: 24, status: true,
			allocAddr: "",
			freeAddr:  "10.36.1.1-10.36.1.254"},
		{addrRange: []string{"10.36.2.1-10.36.2.10", "10.36.0.1-10.36.0.10"}, subnetIP: "10.36.0.0",
			subnetLen: 16, status: true, allocAddr: "10.36.0.1-10.36.0.10, 10.36.2.3-10.36.2.10",
			clearRange: "10.36.2.1-10.36.2.2",
			freeAddr:   "10.36.0.11-10.36.2.2, 10.36.2.11-10.36.255.254"},

		{addrRange: []string{"10.36.1.1-10.36.1.10", "10.36.1.1-10.36.1.10"}, subnetIP: "10.36.1.0",
			clearRange: "10.36.2.1-10.36.2.11", subnetLen: 24, status: false},
	}

	for _, i := range testAddrRange {
		amap.ClearAll()
		InitSubnetBitset(&amap, i.subnetLen)
		for _, pool := range i.addrRange {
			err = SetIPAddrRange(&amap, pool, i.subnetIP, i.subnetLen)
			assertOnTrue(t, err != nil, fmt.Sprintf("set failed %s %+v", err, i))
		}

		err = ClearIPAddrRange(&amap, i.clearRange, i.subnetIP, i.subnetLen)
		assertOnTrue(t, (err != nil) == (i.status == true), fmt.Sprintf("clear failed %s %+v", err, i))

		if i.status == true {
			assertOnTrue(t, err != nil, fmt.Sprintf("err: %s, failed for data %+v", err, i))
			f := ListAvailableIPs(amap, i.subnetIP, i.subnetLen)
			assertOnTrue(t, f != i.freeAddr, fmt.Sprintf("got avail addr: [%s], expected [%s] failed for data %+v",
				f, i.freeAddr, i))
			a := ListAllocatedIPs(amap, i.subnetIP, i.subnetIP, i.subnetLen)
			assertOnTrue(t, a != i.allocAddr, fmt.Sprintf("got allocated addr: [%s], expected [%s] failed for data %+v",
				a, i.allocAddr, i))
		} else {
			assertOnTrue(t, err == nil, fmt.Sprintf("err: %s, failed for data %+v", err, i))
		}
	}

}

func TestIsOverlappingSubnet(t *testing.T) {
	testAddrRange := []struct {
		src     string
		match   string
		overlap bool
	}{
		{src: "10.36.0.1/32", match: "10.36.0.0/24", overlap: true},
		{src: "20.20.0.20/32", match: "20.20.0.0/16", overlap: true},
		{src: "20.20.5.20/32", match: "20.20.0.0/24", overlap: false},
	}

	for _, i := range testAddrRange {

		overlap := IsOverlappingSubnet(i.src, i.match)
		assertOnTrue(t, overlap != i.overlap, fmt.Sprintf("%+v got %v ", i, overlap))
	}

}
