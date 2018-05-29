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
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unsafe"

	osexec "os/exec"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/core"
	"github.com/jainvipin/bitset"
	netlink "github.com/vishvananda/netlink"
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

func ipv6ToBigint(IPv6Addr string) *big.Int {
	IPv6Address := net.ParseIP(IPv6Addr)
	IPv6Int := big.NewInt(0)
	IPv6Int.SetBytes(IPv6Address.To16())
	return IPv6Int
}

func getIPv6Range(Subnetv6 string) (string, string) {
	_, ipv6IP, _ := net.ParseCIDR(Subnetv6)

	subnetMask := ipv6IP.Mask
	subnetStartRange := ipv6IP.IP

	for i := 0; i < 16; i++ {
		subnetMask[i] = ^subnetMask[i]
	}
	n := len(subnetStartRange)

	subnetEndRange := make(net.IP, n)
	for i := 0; i < n; i++ {
		subnetEndRange[i] = subnetStartRange[i] | subnetMask[i]
	}

	return subnetStartRange.String(), subnetEndRange.String()
}

// IsOverlappingSubnetv6 verifies the Overlapping of subnet for v6 networks
func IsOverlappingSubnetv6(inputIPv6Subnet string, existingIPv6Subnet string) bool {
	inputIPv6StartRange, inputIPv6EndRange := getIPv6Range(inputIPv6Subnet)
	existingIPv6StartRange, existingIPv6EndRange := getIPv6Range(existingIPv6Subnet)

	inputStartRange := ipv6ToBigint(inputIPv6StartRange)
	inputEndRange := ipv6ToBigint(inputIPv6EndRange)

	existingStartRange := ipv6ToBigint(existingIPv6StartRange)
	existingEndRange := ipv6ToBigint(existingIPv6EndRange)

	if (existingStartRange.Cmp(inputStartRange) <= 0 && inputStartRange.Cmp(existingEndRange) <= 0) ||
		(existingStartRange.Cmp(inputEndRange) <= 0 && inputEndRange.Cmp(existingEndRange) <= 0) {
		return true
	}

	if (inputStartRange.Cmp(existingStartRange) <= 0 && existingStartRange.Cmp(inputEndRange) <= 0) ||
		(inputStartRange.Cmp(existingEndRange) <= 0 && existingEndRange.Cmp(inputEndRange) <= 0) {
		return true
	}
	return false
}

// IsOverlappingSubnet verifies the Overlapping of subnet
func IsOverlappingSubnet(inputSubnet string, existingSubnet string) bool {
	inputSubnetIP, inputSubnetLen, _ := ParseCIDR(inputSubnet)
	existingSubnetIP, existingSubnetLen, _ := ParseCIDR(existingSubnet)

	inputStartRange, _ := ipv4ToUint32(getFirstAddrInRange(inputSubnetIP))
	inputEndRange, _ := ipv4ToUint32(getLastAddrInRange(inputSubnetIP, inputSubnetLen))
	existingStartRange, _ := ipv4ToUint32(getFirstAddrInRange(existingSubnetIP))
	existingEndRange, _ := ipv4ToUint32(getLastAddrInRange(existingSubnetIP, existingSubnetLen))

	if (existingStartRange <= inputStartRange && inputStartRange <= existingEndRange) ||
		(existingStartRange <= inputEndRange && inputEndRange <= existingEndRange) {
		return true
	}

	if (inputStartRange <= existingStartRange && existingStartRange <= inputEndRange) ||
		(inputStartRange <= existingEndRange && existingEndRange <= inputEndRange) {
		return true
	}
	return false

}

//ConvertBandwidth will change the format of the bandwidth to int64 format and return the updated bw.
//eg:- 2 kb bandwidth will be converted to 2*1024 and the result in Int64 format will be returned.
func ConvertBandwidth(bandwidth string) int64 {
	var rate int64

	const (
		kilobytes = 1024
		megabytes = 1024 * kilobytes
		gigabytes = 1024 * megabytes
	)

	regex := regexp.MustCompile("[0-9]+")
	bw := regex.FindAllString(bandwidth, -1)
	bwParseInt, err := strconv.ParseInt(bw[0], 10, 64)
	if err != nil {
		log.Errorf("error converting bandwidth string to uint64 %+v", err)
	}
	if strings.ContainsAny(bandwidth, "g|G") {
		rate = gigabytes
	} else if strings.ContainsAny(bandwidth, "m|M") {
		rate = megabytes
	} else if strings.ContainsAny(bandwidth, "k|K") {
		rate = kilobytes
	}
	bwInt := bwParseInt * rate
	bwInt = bwInt / 1000

	return bwInt
}

// ValidateNetworkRangeParams verifies the network range format
func ValidateNetworkRangeParams(ipRange string, subnetLen uint) error {
	rangeMin, _ := ipv4ToUint32(getFirstAddrInRange(ipRange))
	rangeMax, _ := ipv4ToUint32(getLastAddrInRange(ipRange, subnetLen))
	firstAddr, _ := ipv4ToUint32(GetSubnetAddr(ipRange, subnetLen))
	lastAddr, _ := ipv4ToUint32(getLastAddrInSubnet(ipRange, subnetLen))

	if rangeMin < firstAddr || rangeMax > lastAddr || rangeMin > rangeMax {
		return core.Errorf("Network subnet format not valid")
	}

	if subnetLen > 32 || subnetLen < 8 {
		return core.Errorf("subnet length %d not supported", subnetLen)
	}
	return nil
}

// IsIPv6 Checks if the address string is IPv6 address
func IsIPv6(ip string) bool { return strings.Contains(ip, ":") }

// InitSubnetBitset initializes a bit set with 2^(32 - subnetLen) bits
func InitSubnetBitset(b *bitset.BitSet, subnetLen uint) {
	maxSize := (1 << (32 - subnetLen)) - 1
	b.Set(uint(maxSize))
	b.Set(uint(0))
}

// ClearReservedEntries clears reserved bits
func ClearReservedEntries(b *bitset.BitSet, subnetLen uint) {
	maxSize := (1 << (32 - subnetLen)) - 1
	b.Clear(uint(maxSize))
	b.Clear(uint(0))
}

// SetBitsOutsideRange sets all IPs outside range as used
func SetBitsOutsideRange(ipAllocMap *bitset.BitSet, ipRange string, subnetLen uint) {
	var i uint32
	rangeMin, _ := ipv4ToUint32(getFirstAddrInRange(ipRange))
	rangeMax, _ := ipv4ToUint32(getLastAddrInRange(ipRange, subnetLen))
	firstAddr, _ := ipv4ToUint32(GetSubnetAddr(ipRange, subnetLen))
	lastAddr, _ := ipv4ToUint32(getLastAddrInSubnet(ipRange, subnetLen))

	// Set bits lower than rangeMin as used
	for i = 0; i < (rangeMin - firstAddr); i++ {
		ipAllocMap.Set(uint(i))
	}

	// Set bits greater than the rangeMax as used
	for i = ((rangeMin - firstAddr) + ((rangeMax - rangeMin) + 1)); i < (lastAddr - firstAddr); i++ {
		ipAllocMap.Set(uint(i))
	}
}

// ClearIPAddrRange marks range of IP address as used
func ClearIPAddrRange(ipAllocMap *bitset.BitSet, ipPool string, nwSubnetIP string, nwSubnetLen uint) error {

	addrRangeList := strings.Split(ipPool, "-")

	hostMin, err := GetIPNumber(nwSubnetIP, nwSubnetLen, 32, addrRangeList[0])
	if err != nil {
		log.Errorf("Error parsing first address %s. Err: %v", addrRangeList[0], err)
		return err
	}

	hostMax, err := GetIPNumber(nwSubnetIP, nwSubnetLen, 32, addrRangeList[1])
	if err != nil {
		log.Errorf("Error parsing last address %s. Err: %v", addrRangeList[1], err)
		return err
	}

	// Clear a range
	for i := hostMin; i <= hostMax; i++ {
		ipAllocMap.Clear(uint(i))
	}

	return nil
}

// SetIPAddrRange marks range of IP address as used
func SetIPAddrRange(ipAllocMap *bitset.BitSet, ipPool string, nwSubnetIP string,
	nwSubnetLen uint) error {

	addrRangeList := strings.Split(ipPool, "-")

	hostMin, err := GetIPNumber(nwSubnetIP, nwSubnetLen, 32, addrRangeList[0])
	if err != nil {
		log.Errorf("Error parsing first address %s. Err: %v", addrRangeList[0], err)
		return err
	}

	hostMax, err := GetIPNumber(nwSubnetIP, nwSubnetLen, 32, addrRangeList[1])
	if err != nil {
		log.Errorf("Error parsing last address %s. Err: %v", addrRangeList[1], err)
		return err
	}

	// Set a range
	for i := hostMin; i <= hostMax; i++ {
		ipAllocMap.Set(uint(i))
	}
	return nil
}

// TestIPAddrRange checks if any IP address from the subnet is already used
func TestIPAddrRange(ipAllocMap *bitset.BitSet, ipPool string, nwSubnetIP string,
	nwSubnetLen uint) error {
	addrRangeList := strings.Split(ipPool, "-")
	hostMin, err := GetIPNumber(nwSubnetIP, nwSubnetLen, 32, addrRangeList[0])
	if err != nil {
		log.Errorf("failed to get host id for %s, Err: %v", addrRangeList[0], err)
		return fmt.Errorf("failed to get host id for %s", addrRangeList[0])
	}

	hostMax, err := GetIPNumber(nwSubnetIP, nwSubnetLen, 32, addrRangeList[1])
	if err != nil {
		log.Errorf("failed to get host id for %s, Err: %v", addrRangeList[1], err)
		return fmt.Errorf("failed to get host id for %s", addrRangeList[1])
	}

	// Test range
	for i := hostMin; i <= hostMax; i++ {

		if ipAllocMap.Test(uint(i)) != false {
			if s, err := GetSubnetIP(nwSubnetIP, nwSubnetLen, 32, i); err == nil {
				log.Infof("ip address %s, hostid %d is not available", s, i)
				return fmt.Errorf("ip address %s is not available", s)
			}
			log.Errorf("failed to convert host id %d to ip address", i)
			return err
		}
	}

	return nil
}

// GetIPAddrRange returns IP CIDR as a ip address range
func GetIPAddrRange(ipCIDR string, subnetLen uint) string {
	rangeMin, _ := ipv4ToUint32(getFirstAddrInRange(ipCIDR))
	rangeMax, _ := ipv4ToUint32(getLastAddrInRange(ipCIDR, subnetLen))
	firstAddr, _ := ipv4ToUint32(GetSubnetAddr(ipCIDR, subnetLen))
	lastAddr, _ := ipv4ToUint32(getLastAddrInSubnet(ipCIDR, subnetLen))

	if rangeMin < firstAddr {
		rangeMin = firstAddr
	}
	if rangeMax > lastAddr {
		rangeMax = lastAddr
	}

	minAddr, _ := ipv4Uint32ToString(rangeMin)
	maxAddr, _ := ipv4Uint32ToString(rangeMax)

	return minAddr + "-" + maxAddr
}

// ClearBitsOutsideRange sets all IPs outside range as used
func ClearBitsOutsideRange(ipAllocMap *bitset.BitSet, ipRange string, subnetLen uint) {
	var i uint32
	rangeMin, _ := ipv4ToUint32(getFirstAddrInRange(ipRange))
	rangeMax, _ := ipv4ToUint32(getLastAddrInRange(ipRange, subnetLen))
	firstAddr, _ := ipv4ToUint32(GetSubnetAddr(ipRange, subnetLen))
	lastAddr, _ := ipv4ToUint32(getLastAddrInSubnet(ipRange, subnetLen))

	// Set bits lower than rangeMin as used
	for i = 0; i < (rangeMin - firstAddr); i++ {
		ipAllocMap.Clear(uint(i))
	}

	// Set bits greater than the rangeMax as used
	for i = ((rangeMin - firstAddr) + ((rangeMax - rangeMin) + 1)); i < (lastAddr - firstAddr); i++ {
		ipAllocMap.Clear(uint(i))
	}
}

// CreateBitset initializes a bit set with 2^numBitsWide bits
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

// GetSubnetIP given a subnet IP and host identifier, calculates an IP within
// the subnet for use.
func GetSubnetIP(subnetIP string, subnetLen uint, allocSubnetLen, hostID uint) (string, error) {
	if subnetIP == "" {
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
	if hostID >= maxHosts {
		return "", core.Errorf("host id %d is beyond subnet's capacity %d",
			hostID, maxHosts)
	}

	hostIPUint32, err := ipv4ToUint32(subnetIP)
	if err != nil {
		return "", core.Errorf("unable to convert subnet %s to uint32", subnetIP)
	}
	hostIPUint32 += uint32(hostID << (32 - allocSubnetLen))
	return ipv4Uint32ToString(hostIPUint32)
}

// GetIPNumber obtains the host id from the host IP. SEe `GetSubnetIP` for more information.
func GetIPNumber(subnetIP string, subnetLen uint, allocSubnetLen uint, hostIP string) (uint, error) {
	if subnetLen > 32 || subnetLen < 8 {
		return 0, core.Errorf("subnet length %d not supported", subnetLen)
	}
	if subnetLen > allocSubnetLen {
		return 0, core.Errorf("subnet length %d is bigger than subnet alloc len %d",
			subnetLen, allocSubnetLen)
	}

	hostIPUint32, err := ipv4ToUint32(hostIP)
	if err != nil {
		return 0, core.Errorf("unable to convert hostIP %s to uint32", hostIP)
	}

	subnetIPUint32, err := ipv4ToUint32(subnetIP)
	if err != nil {
		return 0, core.Errorf("unable to convert subnetIP %s to uint32", subnetIP)
	}
	hostID := uint((hostIPUint32 - subnetIPUint32) >> (32 - allocSubnetLen))

	maxHosts := uint(1 << (allocSubnetLen - subnetLen))
	if hostID >= maxHosts {
		return 0, core.Errorf("hostIP %s is exceeding beyond subnet %s/%d, hostID %d",
			hostIP, subnetIP, subnetLen, hostID)
	}

	return uint(hostID), nil
}

// ReserveIPv6HostID sets the hostId in the AllocMap
func ReserveIPv6HostID(hostID string, IPv6AllocMap *map[string]bool) {
	if hostID == "" {
		return
	}
	if *IPv6AllocMap == nil {
		*IPv6AllocMap = make(map[string]bool)
	}
	(*IPv6AllocMap)[hostID] = true
}

// GetNextIPv6HostID returns the next available hostId in the AllocMap
func GetNextIPv6HostID(hostID, subnetAddr string, subnetLen uint, IPv6AllocMap map[string]bool) (string, error) {
	if hostID == "" {
		hostID = "::"
	}
	if subnetLen == 0 {
		return "", core.Errorf("subnet length %d is invalid", subnetLen)
	}

	hostidIP := net.ParseIP(hostID)

	// start with the carryOver 1 to get the next hostID
	var carryOver = 1
	var allocd = true

	for allocd == true {
		// Add 1 to hostID
		for i := len(hostidIP) - 1; i >= 0; i-- {
			var temp int
			temp = int(hostidIP[i]) + carryOver
			if temp > int(0xFF) {
				hostidIP[i] = 0
				carryOver = 1
			} else {
				hostidIP[i] = uint8(temp)
				carryOver = 0
				break
			}
		}
		// Check if this hostID is already allocated
		if _, allocd = IPv6AllocMap[hostidIP.String()]; allocd == true {
			// Already allocated find the next hostID
			carryOver = 1
		} else {
			// allocd == false. check if we reached MaxHosts
			offset := (subnetLen - 1) / 8
			masklen := subnetLen % 8
			mask := ((1 << masklen) - 1) << (8 - masklen)
			if (hostidIP[offset] & byte(mask)) != 0 {
				// if hostID is outside subnet range,
				//	check if we have reached MaxHosts
				maxHosts := math.Pow(2, float64(128-subnetLen)) - 1
				if float64(len(IPv6AllocMap)) < maxHosts {
					hostID = "::"
					hostidIP = net.ParseIP(hostID)
					carryOver = 1
					allocd = true // continue the loop
				} else {
					return "", core.Errorf("Reached MaxHosts (%v). Cannot allocate more hosts", maxHosts)
				}
			}
		}
	}
	return hostidIP.String(), nil
}

// GetSubnetIPv6 given a subnet IP and host identifier, calculates an IPv6 address
// within the subnet for use.
func GetSubnetIPv6(subnetAddr string, subnetLen uint, hostID string) (string, error) {
	if subnetAddr == "" {
		return "", core.Errorf("null subnet")
	}

	if subnetLen > 128 || subnetLen < 16 {
		return "", core.Errorf("subnet length %d not supported", subnetLen)
	}

	subnetIP := net.ParseIP(subnetAddr)
	hostidIP := net.ParseIP(hostID)
	hostIP := net.IPv6zero

	var offset int
	for offset = 0; offset < int(subnetLen/8); offset++ {
		hostIP[offset] = subnetIP[offset]
	}
	// copy the overlapping byte in subnetIP and hostID
	if subnetLen%8 != 0 && subnetIP[offset] != 0 {
		if hostidIP[offset]&subnetIP[offset] != 0 {
			return "", core.Errorf("host id %s exceeds subnet %s capacity ",
				hostID, subnetAddr)
		}
		hostIP[offset] = hostidIP[offset] | subnetIP[offset]
		offset++
	}

	for ; offset < len(hostidIP); offset++ {
		hostIP[offset] = hostidIP[offset]
	}
	return hostIP.String(), nil
}

// GetIPv6HostID obtains the host id from the host IP. SEe `GetSubnetIP` for more information.
func GetIPv6HostID(subnetAddr string, subnetLen uint, hostAddr string) (string, error) {
	if subnetLen > 128 || subnetLen < 16 {
		return "", core.Errorf("subnet length %d not supported", subnetLen)
	}
	// Initialize hostID
	hostID := net.IPv6zero

	var offset uint

	// get the overlapping byte
	offset = subnetLen / 8
	subnetIP := net.ParseIP(subnetAddr)
	if subnetIP == nil {
		return "", core.Errorf("Invalid subnetAddr %s ", subnetAddr)
	}
	s := uint8(subnetIP[offset])
	hostIP := net.ParseIP(hostAddr)
	if hostIP == nil {
		return "", core.Errorf("Invalid hostAddr %s ", hostAddr)
	}
	h := uint8(hostIP[offset])
	hostID[offset] = byte(h - s)

	// Copy the rest of the bytes
	for i := (offset + 1); i < 16; i++ {
		hostID[i] = hostIP[i]
		offset++
	}
	return hostID.String(), nil
}

// TagRange represents a range of integers, intended for VLAN and VXLAN
// tagging.
type TagRange struct {
	Min int
	Max int
}

// ParseTagRanges takes a string such as 12-24,48-64 and turns it into a series
// of TagRange.
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
		if tagRanges[idx].Min < 1 {
			return nil, core.Errorf("invalid range %s, values less than 1",
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

// ParseCIDR parses a CIDR string into a gateway IP and length.
func ParseCIDR(cidrStr string) (string, uint, error) {
	strs := strings.Split(cidrStr, "/")
	if len(strs) != 2 {
		return "", 0, core.Errorf("invalid cidr format")
	}

	subnetStr := strs[0]
	subnetLen, err := strconv.Atoi(strs[1])
	if (IsIPv6(subnetStr) && subnetLen > 128) || err != nil || (!IsIPv6(subnetStr) && subnetLen > 32) {
		return "", 0, core.Errorf("invalid mask in gateway/mask specification ")
	}

	return subnetStr, uint(subnetLen), nil
}

// GetInterfaceIP obtains the ip addr of a local interface on the host.
func GetInterfaceIP(linkName string) (string, error) {
	var addrs []netlink.Addr
	localIPAddr := ""

	link, err := netlink.LinkByName(linkName)
	if err != nil {
		return "", err
	}
	addrs, err = netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return "", err
	}
	if len(addrs) > 0 {
		localIPAddr = addrs[0].IP.String()
	}

	err = core.Errorf("local ip not found")
	if localIPAddr != "" {
		err = nil
	}

	return localIPAddr, err
}

// SetInterfaceIP : Set IP address of an interface
func SetInterfaceIP(name string, ipstr string) error {
	iface, err := netlink.LinkByName(name)
	if err != nil {
		return err
	}
	ipaddr, err := netlink.ParseAddr(ipstr)
	if err != nil {
		return err
	}
	netlink.LinkSetUp(iface)
	return netlink.AddrAdd(iface, ipaddr)
}

// SetInterfaceMac : Set mac address of an interface
func SetInterfaceMac(name string, macaddr string) error {
	iface, err := netlink.LinkByName(name)
	if err != nil {
		return err
	}
	hwaddr, err := net.ParseMAC(macaddr)
	if err != nil {
		return err
	}
	return netlink.LinkSetHardwareAddr(iface, hwaddr)
}

// GetNetlinkAddrList returns a list of local IP addresses
func GetNetlinkAddrList() ([]string, error) {
	var addrList []string
	// get the link list
	linkList, err := netlink.LinkList()
	if err != nil {
		return addrList, err
	}

	log.Debugf("Got link list(%d): %+v", len(linkList), linkList)

	// Loop thru each interface and add its ip addr to list
	for _, link := range linkList {
		if strings.HasPrefix(link.Attrs().Name, "docker") || strings.HasPrefix(link.Attrs().Name, "veth") ||
			strings.HasPrefix(link.Attrs().Name, "vport") || strings.HasPrefix(link.Attrs().Name, "lo") {
			continue
		}
		addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
		if err != nil {
			return addrList, err
		}

		for _, addr := range addrs {
			addrList = append(addrList, addr.IP.String())
		}
	}

	return addrList, err
}

// GetLocalAddrList returns a list of local IP addresses
func GetLocalAddrList() ([]string, error) {
	var addrList []string
	// get the link list
	intfList, err := net.Interfaces()
	if err != nil {
		return addrList, err
	}

	log.Debugf("Got address list(%d): %+v", len(intfList), intfList)

	// Loop thru each interface and add its ip addr to list
	for _, intf := range intfList {
		if strings.HasPrefix(intf.Name, "docker") || strings.HasPrefix(intf.Name, "veth") ||
			strings.HasPrefix(intf.Name, "vport") || strings.HasPrefix(intf.Name, "lo") {
			continue
		}

		addrs, err := intf.Addrs()
		if err != nil {
			return addrList, err
		}

		for _, addr := range addrs {
			addrList = append(addrList, addr.String())
		}
	}

	return addrList, err
}

//GetHostLowestLinkMtu return lowest mtu for host interface(excluding ovs
//interface
func GetHostLowestLinkMtu() (int, error) {

	lowestMTU := 9000 //Jumbo frame MTU
	intfList, err := net.Interfaces()
	if err != nil {
		return 0, err
	}
	// Loop thru each interface and add its ip addr to list
	for _, intf := range intfList {
		if strings.HasPrefix(intf.Name, "docker") || strings.HasPrefix(intf.Name, "veth") ||
			strings.HasPrefix(intf.Name, "vport") || strings.HasPrefix(intf.Name, "lo") {
			continue
		}

		lowestMTU = int(math.Min(float64(lowestMTU), float64(intf.MTU)))
	}
	if lowestMTU == 0 {
		return 0, errors.New("Failed to find minimum MTU")
	}
	return lowestMTU, nil
}

// IsAddrLocal check if an address is local
func IsAddrLocal(findAddr string) bool {
	// get the local addr list
	addrList, err := GetNetlinkAddrList()
	if err != nil {
		return false
	}

	// find the address
	for _, addr := range addrList {
		if addr == findAddr {
			return true
		}
	}

	return false
}

// GetFirstLocalAddr returns the first ip address
func GetFirstLocalAddr() (string, error) {
	// get the local addr list
	addrList, err := GetNetlinkAddrList()
	if err != nil {
		return "", err
	}

	if len(addrList) > 0 {
		return addrList[0], nil
	}

	return "", errors.New("no address was found")
}

// GetDefaultAddr gets default address of local hostname
func GetDefaultAddr() (string, error) {
	// get the ip address by local hostname
	localIP, err := GetMyAddr()
	if err == nil && IsAddrLocal(localIP) {
		return localIP, nil
	}

	// Return first available address if we could not find by hostname
	return GetFirstLocalAddr()
}

// GetSubnetAddr returns a subnet given a subnet range
func GetSubnetAddr(ipStr string, length uint) string {
	subnetStr := ipStr
	if isSubnetIPRange(ipStr) {
		subnetStr = strings.Split(ipStr, "-")[0]
	}

	subnet, _ := ipv4ToUint32(subnetStr)
	subnetMask := -1 << (32 - length)

	ipSubnet, _ := ipv4Uint32ToString(uint32(subnetMask) & subnet)
	return ipSubnet
}

// getLastAddrInSubnet returns the last address in a subnet
func getLastAddrInSubnet(ipStr string, length uint) string {
	subnetStr := ipStr
	if isSubnetIPRange(ipStr) {
		subnetStr = strings.Split(ipStr, "-")[0]
	}

	subnet, _ := ipv4ToUint32(subnetStr)
	subnetMask := -1 << (32 - length)

	lastIP, _ := ipv4Uint32ToString(uint32(^subnetMask) | subnet)
	return lastIP
}

// getFirstAddrInRange returns the first IP in the subnet range
func getFirstAddrInRange(ipRange string) string {
	firstIP := ipRange
	if isSubnetIPRange(ipRange) {
		firstIP = strings.Split(ipRange, "-")[0]
	}

	return firstIP
}

// getLastAddrInRange returns the first IP in the subnet range
func getLastAddrInRange(ipRange string, subnetLen uint) string {
	var lastIP string

	if isSubnetIPRange(ipRange) {
		lastIP = strings.Split(ipRange, "-")[1]
	} else {
		lastIP = getLastAddrInSubnet(ipRange, subnetLen)
	}

	return lastIP
}

// isSubnetIPRange returns a boolean indication if it's an IP range
func isSubnetIPRange(ipRange string) bool {
	return strings.Contains(ipRange, "-")
}

// GetMyAddr returns ip address of current host
func GetMyAddr() (string, error) {
	host, err := os.Hostname()
	if err != nil {
		return "", err
	}

	if host == "localhost" {
		return "", errors.New("could not get hostname")
	}

	addrs, err := net.LookupIP(host)
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipv4 := addr.To4(); ipv4 != nil && !ipv4.IsLoopback() {
			return ipv4.String(), nil
		}
	}

	return "", errors.New("could not find ip addr")
}

// PortToHostIPMAC gets IP and MAC based on port number
func PortToHostIPMAC(port, subnet int) (string, string) {
	b0 := subnet >> 24
	b1 := (subnet >> 16) & 0xff
	b2 := (port >> 8) & 0xff
	b3 := port & 0xff
	ipStr := fmt.Sprintf("%d.%d.%d.%d/16", b0, b1, b2, b3)
	macStr := fmt.Sprintf("02:02:%02x:%02x:%02x:%02x", b0, b1, b2, b3)

	return ipStr, macStr
}

// GetHostIntfName gets the host access interface name
func GetHostIntfName(intf string) string {
	return strings.Replace(intf, "vport", "hport", 1)
}

// SetIPMasquerade sets a ip masquerade rule.
func SetIPMasquerade(intf, netmask string) error {
	ipTablesPath, err := osexec.LookPath("iptables")
	if err != nil {
		return err
	}
	_, err = osexec.Command(ipTablesPath, "-t", "nat", "-C", "POSTROUTING", "-s", netmask,
		"!", "-o", intf, "-j", "MASQUERADE").CombinedOutput()

	// If the rule already exists, just return
	if err == nil {
		return nil
	}

	out, err := osexec.Command(ipTablesPath, "-t", "nat", "-A", "POSTROUTING", "-s", netmask,
		"!", "-o", intf, "-j", "MASQUERADE").CombinedOutput()
	if err != nil {
		log.Errorf("Setting ip tables failed: %v %s", err, out)
	} else {
		log.Infof("####Set ip tables success: %s", out)
	}

	return err
}

// HostIPToGateway gets the gateway based on the IP
func HostIPToGateway(hostIP string) (string, error) {
	ip := strings.Split(hostIP, ".")
	if len(ip) != 4 {
		return "", errors.New("bad host IP")
	}

	return ip[0] + "." + ip[1] + ".255.254", nil
}

// CIDRToMask converts a CIDR to corresponding network number
func CIDRToMask(cidr string) (int, error) {
	_, net, err := net.ParseCIDR(cidr)
	if err != nil {
		return -1, err
	}
	ip := net.IP
	if len(ip) == 16 {
		return int(binary.BigEndian.Uint32(ip[12:16])), nil
	}
	return int(binary.BigEndian.Uint32(ip)), nil
}

func getIPRange(subnetIP string, subnetLen uint, startIdx, endIdx uint) string {
	startAddress, err := GetSubnetIP(subnetIP, subnetLen, 32, startIdx)
	if err != nil {
		log.Errorf("GetAllocatedIPs: getting ipAddress for idx %d: %s", startIdx, err)
		startAddress = ""
	}
	if startIdx == endIdx {
		return startAddress
	}
	endAddress, err := GetSubnetIP(subnetIP, subnetLen, 32, endIdx)
	if err != nil {
		log.Errorf("GetAllocatedIPs: getting ipAddress for idx %d: %s", endIdx, err)
		endAddress = ""
	}
	return startAddress + "-" + endAddress
}

// ListAllocatedIPs returns a string of allocated IPs in a network
func ListAllocatedIPs(allocMap bitset.BitSet, ipPool string, subnetIP string, subnetLen uint) string {
	idx := uint(0)
	startIdx := idx
	list := []string{}
	inRange := false

	ClearReservedEntries(&allocMap, subnetLen)
	ClearBitsOutsideRange(&allocMap, ipPool, subnetLen)
	for {
		foundValue, found := allocMap.NextSet(idx)
		if !found {
			break
		}

		if !inRange { // begin of range
			startIdx = foundValue
			inRange = true
		} else if foundValue > idx { // end of range
			thisRange := getIPRange(subnetIP, subnetLen, startIdx, idx-1)
			list = append(list, thisRange)
			startIdx = foundValue
		}
		idx = foundValue + 1
	}

	// list end with allocated value
	if inRange {
		thisRange := getIPRange(subnetIP, subnetLen, startIdx, idx-1)
		list = append(list, thisRange)
	}

	return strings.Join(list, ", ")
}

// NextClear wrapper around Bitset to check max id
func NextClear(allocMap bitset.BitSet, idx uint, subnetLen uint) (uint, bool) {
	maxHosts := uint(1 << (32 - subnetLen))

	value, found := allocMap.NextClear(idx)
	if found && value >= maxHosts {
		return 0, false
	}
	return value, found
}

// ListAvailableIPs returns a string of available IPs in a network
func ListAvailableIPs(allocMap bitset.BitSet, subnetIP string, subnetLen uint) string {
	idx := uint(0)
	startIdx := idx
	list := []string{}
	inRange := false

	for {
		foundValue, found := NextClear(allocMap, idx, subnetLen)
		if !found {
			break
		}

		if !inRange { // begin of range
			startIdx = foundValue
			inRange = true
		} else if foundValue > idx { // end of range
			thisRange := getIPRange(subnetIP, subnetLen, startIdx, idx-1)
			list = append(list, thisRange)
			startIdx = foundValue
		}
		idx = foundValue + 1
	}

	// list end with allocated value
	if inRange {
		thisRange := getIPRange(subnetIP, subnetLen, startIdx, idx-1)
		list = append(list, thisRange)
	}

	return strings.Join(list, ", ")
}

// AddIPRoute adds the specified ip route
func AddIPRoute(cidr, gw string) error {

	_, dst, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}

	gwIP := net.ParseIP(gw)
	if gwIP == nil {
		return fmt.Errorf("Unable to parse gw %s", gw)
	}

	match, err := netlink.RouteListFiltered(netlink.FAMILY_V4,
		&netlink.Route{Dst: dst}, netlink.RT_FILTER_DST)

	if err == nil && len(match) != 0 {
		if len(match) == 1 && match[0].Gw.Equal(gwIP) {
			// the exact same route exists -- be idempotent
			log.Infof("Route %s --> %s present", cidr, gw)
			return nil
		}
		log.Errorf("AddIPRoute(%s, %s): exists %+v", cidr, gw, match)
		return fmt.Errorf("Route exists")
	}

	newRoute := netlink.Route{
		Dst: dst,
		Gw:  gwIP,
	}

	return netlink.RouteAdd(&newRoute)
}

// DelIPRoute deletes the specified ip route
func DelIPRoute(cidr, gw string) error {

	_, dst, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}

	gwIP := net.ParseIP(gw)
	if gwIP == nil {
		return fmt.Errorf("Unable to parse gw %s", gw)
	}

	return netlink.RouteDel(&netlink.Route{Dst: dst, Gw: gwIP})
}

// ValidateBindAddress format in "address:port"
func ValidateBindAddress(address string) error {
	addr := strings.Split(address, ":")
	if len(addr) != 2 {
		return fmt.Errorf("bind address is not in 'ip:port' format, got %s", address)
	}
	port, err := strconv.Atoi(addr[1])
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("bind port is a integer between 1-65535, got %v", port)
	}
	return nil
}
