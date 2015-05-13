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
	"github.com/vishvananda/netlink"
	"net"
	"strconv"
	"strings"
	"unsafe"

	"github.com/contiv/netplugin/core"
)

var endianNess string

func init() {
	// Determine endianness
	var test uint32 = 0xff
	firstByte := (*byte)(unsafe.Pointer(&test))
	if *firstByte == 0 {
		endianNess = "big"
	} else {
		endianNess = "little"
	}
}

// initialize a bit set with 2^(32 - subnetLen) bits
func InitSubnetBitset(b *bitset.BitSet, subnetLen uint) {
	maxSize := 1 << (32 - subnetLen)
	b.Set(uint(maxSize))
	b.Set(uint(0))
}

// initialize a bit set with 2^numBitsWide bits
func CreateBitset(numBitsWide uint) *bitset.BitSet {
	maxSize := 1 << numBitsWide
	return bitset.New(uint(maxSize))
}

func ipv4ToUint32(ipaddr string) (uint32, error) {
	var ipUint32 uint32

	ip := net.ParseIP(ipaddr).To4()
	if ip == nil {
		return 0, core.Errorf("ipv4 to uint32 conversion: invalid ip format")
	}
	if endianNess == "little" {
		ipUint32 = (uint32(ip[3]) | (uint32(ip[2]) << 8) |
			(uint32(ip[1]) << 16) | (uint32(ip[0]) << 24))
	} else {
		ipUint32 = uint32(ip[0]) | (uint32(ip[1]) << 8) |
			(uint32(ip[2]) << 16) | (uint32(ip[3]) << 24)
	}

	return ipUint32, nil
}

func ipv4Uint32ToString(ipUint32 uint32) (string, error) {
	var b1, b2, b3, b4 byte

	if endianNess == "little" {
		b1, b2, b3, b4 = byte(ipUint32>>24), byte(ipUint32>>16),
			byte(ipUint32>>8), byte(ipUint32)
	} else {
		b1, b2, b3, b4 = byte(ipUint32), byte(ipUint32>>8),
			byte(ipUint32>>16), byte((ipUint32)>>24)
	}

	return fmt.Sprintf("%d.%d.%d.%d", b1, b2, b3, b4), nil
}

func GetSubnetIp(subnetIp string, subnetLen uint, allocSubnetLen, hostId uint) (string, error) {
	if subnetIp == "" {
		return "", core.Errorf("null subnet")
	}

	if subnetLen > 32 || subnetLen < 8 {
		return "", core.Errorf("subnet length %d not supported", subnetLen)
	}
	if subnetLen > allocSubnetLen {
		return "", core.Errorf("subnet length %d is bigger than subnet alloc len %d",
			subnetLen, allocSubnetLen)
	}

	maxHosts := uint(1 << (allocSubnetLen - subnetLen))
	if hostId >= maxHosts {
		return "", core.Errorf("host id %d is beyond subnet's capacity %d",
			hostId, maxHosts)
	}

	hostIpUint32, err := ipv4ToUint32(subnetIp)
	if err != nil {
		return "", core.Errorf("unable to convert subnet %s to uint32", subnetIp)
	}
	hostIpUint32 += uint32(hostId << (32 - allocSubnetLen))
	return ipv4Uint32ToString(hostIpUint32)
}

func GetIpNumber(subnetIp string, subnetLen uint, allocSubnetLen uint, hostIp string) (uint, error) {
	if subnetLen > 32 || subnetLen < 8 {
		return 0, core.Errorf("subnet length %d not supported", subnetLen)
	}
	if subnetLen > allocSubnetLen {
		return 0, core.Errorf("subnet length %d is bigger than subnet alloc len %d",
			subnetLen, allocSubnetLen)
	}

	hostIpUint32, err := ipv4ToUint32(hostIp)
	if err != nil {
		return 0, core.Errorf("unable to convert hostIp %s to uint32", hostIp)
	}

	subnetIpUint32, err := ipv4ToUint32(subnetIp)
	if err != nil {
		return 0, core.Errorf("unable to convert subnetIp %s to uint32", subnetIp)
	}
	hostId := uint((hostIpUint32 - subnetIpUint32) >> (32 - allocSubnetLen))

	maxHosts := uint(1 << (allocSubnetLen - subnetLen))
	if hostId >= maxHosts {
		return 0, core.Errorf("hostIp %s is exceeding beyond subnet %s/%d, hostId %d",
			hostIp, subnetIp, subnetLen, hostId)
	}

	return uint(hostId), nil
}

type TagRange struct {
	Min int
	Max int
}

func ParseTagRanges(ranges string, tagType string) ([]TagRange, error) {
	var err error

	if ranges == "" {
		return []TagRange{{0, 0}}, nil
	}

	if tagType != "vlan" && tagType != "vxlan" {
		return nil, core.Errorf("invalid tag type %s", tagType)
	}
	rangesStr := strings.Split(ranges, ",")

	if len(rangesStr) > 1 && tagType == "vxlan" {
		return nil, core.Errorf("do not support more than 2 vxlan tag ranges")
	}

	tagRanges := make([]TagRange, len(rangesStr), len(rangesStr))
	for idx, oneRangeStr := range rangesStr {
		oneRangeStr = strings.Trim(oneRangeStr, " ")
		tagNums := strings.Split(oneRangeStr, "-")
		if len(tagNums) > 2 {
			return nil, core.Errorf("invalid tags %s, correct '10-50,70-100'",
				oneRangeStr)
		}
		tagRanges[idx].Min, err = strconv.Atoi(tagNums[0])
		if err != nil {
			return nil, core.Errorf("invalid integer %d conversion error '%s'",
				tagRanges[idx].Min, err)
		}
		tagRanges[idx].Max, err = strconv.Atoi(tagNums[1])
		if err != nil {
			return nil, core.Errorf("invalid integer %d conversion error '%s'",
				tagRanges[idx].Max, err)
		}

		if tagRanges[idx].Min > tagRanges[idx].Max {
			return nil, core.Errorf("invalid range %s, min is greater than max",
				oneRangeStr)
		}
		if tagType == "vlan" && tagRanges[idx].Max > 4095 {
			return nil, core.Errorf("invalid range %s, vlan values exceed 4095 max allowed",
				oneRangeStr)
		}
		if tagType == "vxlan" && tagRanges[idx].Max > 65535 {
			return nil, core.Errorf("invalid range %s, vlan values exceed 65535 max allowed",
				oneRangeStr)
		}
		if tagType == "vxlan" &&
			(tagRanges[idx].Max-tagRanges[idx].Min > 16000) {
			return nil, core.Errorf("does not allow vxlan range to exceed 16000 range %s",
				oneRangeStr)
		}
	}

	return tagRanges, nil
}

func GetLocalIp() (string, error) {
	var addrs []netlink.Addr
	localIpAddr := ""

	for idx := 0; idx < 3; idx++ {
		linkName := "eth" + strconv.Itoa(idx)
		link, err := netlink.LinkByName(linkName)
		if err != nil {
			if strings.Contains(err.Error(), "Link not found") {
				continue
			}
			return "", err
		}
		addrs, err = netlink.AddrList(link, netlink.FAMILY_V4)
		if err != nil {
			if strings.Contains(err.Error(), "Link not found") {
				continue
			}
			return "", err
		}
		if len(addrs) > 0 {
			localIpAddr = addrs[0].IP.String()
		}
	}

	err := core.Errorf("local ip not found")
	if localIpAddr != "" {
		err = nil
	}

	return localIpAddr, err
}

func ParseCIDR(cidrStr string) (string, uint, error) {
	strs := strings.Split(cidrStr, "/")
	if len(strs) != 2 {
		return "", 0, core.Errorf("invalid cidr format")
	}

	subnetStr := strs[0]
	subnetLen, _ := strconv.Atoi(strs[1])
	if subnetLen > 32 {
		return "", 0, core.Errorf("invalid mask in gateway/mask specification ")
	}

	return subnetStr, uint(subnetLen), nil
}
