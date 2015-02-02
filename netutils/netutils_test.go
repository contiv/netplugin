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
	"testing"
)

type testSubnetInfo struct {
	subnetIp  string
	subnetLen uint
	hostId    uint
	hostIp    string
}

var testSubnets = []testSubnetInfo{
	{subnetIp: "11.2.1.0", subnetLen: 24, hostId: 5, hostIp: "11.2.1.5"},
	{subnetIp: "10.123.16.0", subnetLen: 22, hostId: 513, hostIp: "10.123.18.1"},
	{subnetIp: "172.12.0.0", subnetLen: 16, hostId: 261, hostIp: "172.12.1.5"},
}

func TestGetSubnetIp(t *testing.T) {
	for _, te := range testSubnets {
		hostIp, err := GetSubnetIp(te.subnetIp, te.subnetLen, 32, te.hostId)
		if err != nil {
			t.Fatalf("error getting host ip from subnet %s/%d for hostid %d - err '%s'",
				te.subnetIp, te.subnetLen, te.hostId, err)
		}
		if hostIp != te.hostIp {
			t.Fatalf("obtained ip %s doesn't match expected ip %s for subnet %s/%d\n",
				hostIp, te.hostIp, te.subnetIp, te.subnetLen)
		}
	}
}

var testInvalidSubnets = []testSubnetInfo{
	{subnetIp: "11.2.1.0", subnetLen: 32, hostId: 5, hostIp: "11.2.1.5"},
	{subnetIp: "10.123.16.0", subnetLen: 22, hostId: 1025, hostIp: "10.123.18.1"},
	{subnetIp: "172.12.0.0", subnetLen: 4, hostId: 261, hostIp: "172.12.1.5"},
}

func TestInvalidGetSubnetIp(t *testing.T) {
	for _, te := range testInvalidSubnets {
		_, err := GetSubnetIp(te.subnetIp, te.subnetLen, 32, te.hostId)
		if err == nil {
			t.Fatalf("Expecting error on invalid config subnet %s/%d for hostid %d",
				te.subnetIp, te.subnetLen, te.hostId)
		}
	}
}

func TestGetIpNumber(t *testing.T) {
	for _, te := range testSubnets {
		hostId, err := GetIpNumber(te.subnetIp, te.subnetLen, 32, te.hostIp)
		if err != nil {
			t.Fatalf("error getting host ip from subnet %s/%d for hostid %d ",
				te.subnetIp, te.subnetLen, te.hostId)
		}
		if hostId != te.hostId {
			t.Fatalf("obtained ip %d doesn't match with expected ip %d \n",
				hostId, te.hostId)
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
	subnetIp       string
	subnetLen      uint
	subnetAllocLen uint
	hostId         uint
	hostIp         string
}

var testSubnetAllocs = []testSubnetAllocInfo{
	{subnetIp: "11.1.0.0", subnetLen: 16, subnetAllocLen: 24,
		hostId: 5, hostIp: "11.1.5.0"},
	{subnetIp: "10.0.0.0", subnetLen: 8, subnetAllocLen: 24,
		hostId: 5, hostIp: "10.0.5.0"},
}

func TestGetSubnetAlloc(t *testing.T) {
	for _, te := range testSubnetAllocs {
		hostIp, err := GetSubnetIp(te.subnetIp, te.subnetLen,
			te.subnetAllocLen, te.hostId)
		if err != nil {
			t.Fatalf("error getting subnet ip from subnet-range %s/%d alloc-size %d "+
				"for id %d - err '%s'",
				te.subnetIp, te.subnetLen, te.subnetAllocLen, te.hostId, err)
		}
		if hostIp != te.hostIp {
			t.Fatalf("obtained ip %s doesn't match expected ip %s for subnet %s/%d "+
				"for AllocLen %d \n",
				hostIp, te.hostIp, te.subnetIp, te.subnetLen, te.subnetAllocLen)
		}
	}
}

func TestGetSubnetNumber(t *testing.T) {
	for _, te := range testSubnetAllocs {
		hostId, err := GetIpNumber(te.subnetIp, te.subnetLen,
			te.subnetAllocLen, te.hostIp)
		if err != nil {
			t.Fatalf("error getting subnet ip from subnet %s/%d for hostid %d "+
				"for subnet alloc size %d \n",
				te.subnetIp, te.subnetLen, te.hostId, te.subnetAllocLen)
		}
		if hostId != te.hostId {
			t.Fatalf("obtained subnet ip %d doesn't match with expected ip %d "+
				"for subnet %s/%d alloc size %d \n",
				hostId, te.hostId, te.subnetIp, te.subnetLen, te.subnetAllocLen)
		}
	}
}

func TestGetLocalIp(t *testing.T) {
	ipAddr, err := GetLocalIp()
	if ipAddr == "" {
		t.Fatalf("error obtaining local IP of the host '%s' \n", err)
	}
}
