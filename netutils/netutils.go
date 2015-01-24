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
    "errors"
    "fmt"
    "net"
    "math"
    "unsafe"
    "strings"
    "strconv"
    "github.com/willf/bitset"
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

// TODO: replace this with function in the native lib once PR is accepted
func NextUnSet(b *bitset.BitSet, i uint) (uint, bool) {
    for ; i < b.Len() && b.Test(i); i++ { }
    if i == b.Len() {
        return 0, false
    } else {
        return i, true
    }
}

func InitBitset(b *bitset.BitSet, subnetLen uint) {
    maxSize := math.Pow(2, float64(32 - subnetLen))
    b.Set(uint(maxSize))
    b.Set(uint(0))
}

func ipv4ToUint32(ipaddr string) (uint32, error) {
    var ipUint32 uint32

    ip := net.ParseIP(ipaddr).To4()
    if (ip == nil) {
        return 0, errors.New("ipv4 to uint32 conversion: invalid ip format")
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

func GetSubnetIp(subnetIp string, subnetLen uint, hostId uint) (string, error) {
    if subnetIp == "" {
        return "", errors.New("null subnet")
    }

    if subnetLen > 32 || subnetLen < 8 {
        return "", errors.New(
            fmt.Sprintf("subnet length %d not supported \n", subnetLen))
    }

    maxHosts := (uint)(math.Pow(2, float64(32 - subnetLen)))
    if hostId >= maxHosts {
        return "", errors.New(
            fmt.Sprintf("host id %d is beyond subnet's capacity %d", hostId, maxHosts))
    }

    hostIpUint32, err := ipv4ToUint32(subnetIp)
    if err != nil {
        return "", errors.New(
            fmt.Sprintf("unable to convert subnet %s to uint32", subnetIp))
    }
    hostIpUint32 += uint32(hostId)
    return ipv4Uint32ToString(hostIpUint32)
}

func GetIpNumber(subnetIp string, subnetLen uint, hostIp string) (uint, error) {
    if subnetLen > 32 || subnetLen < 8 {
        return 0, errors.New(
            fmt.Sprintf("subnet length %d not supported \n", subnetLen))
    }

    hostIpUint32, err := ipv4ToUint32(hostIp)
    if err != nil {
        return 0, errors.New(
            fmt.Sprintf("unable to convert hostIp %s to uint32", hostIp))
    }

    subnetIpUint32, err := ipv4ToUint32(subnetIp)
    if err != nil {
        return 0, errors.New(
            fmt.Sprintf("unable to convert subnetIp %s to uint32", subnetIp))
    }
    hostId := hostIpUint32 - subnetIpUint32

    maxHosts := (uint)(math.Pow(2, float64(32 - subnetLen)))
    if uint(hostId) >= maxHosts {
        return 0, errors.New(
            fmt.Sprintf("hostIp %s is exceeding beyond subnet %s/%d, hostId %d ",
                hostIp, subnetIp, subnetLen, hostId))
    }

    return uint(hostId), nil
}

type TagRange struct {
    min     int
    max     int
}

func ParseTagRanges(ranges string, tagType string) ([]TagRange, error) {
    var err error

    if tagType != "vlan" && tagType != "vxlan" {
        return nil, errors.New(fmt.Sprintf("invalid tag type %s ", tagType))
    }
    rangesStr := strings.Split(ranges, ",")
    tagRanges := make([]TagRange, len(rangesStr), len(rangesStr))
    for idx, oneRangeStr := range rangesStr {
        oneRangeStr = strings.Trim(oneRangeStr, " ")
        tagNums := strings.Split(oneRangeStr, "-")
        if len(tagNums) > 2 {
            return nil, errors.New(fmt.Sprintf(
                "invalid tags %s, correct '10-50,70-100'", oneRangeStr))
        }
        tagRanges[idx].min, err = strconv.Atoi(tagNums[0])
        if err != nil {
            return nil, errors.New(fmt.Sprintf(
                "invalid integer %d conversion error '%s'", tagRanges[idx].min, err))
        }
        tagRanges[idx].max, err = strconv.Atoi(tagNums[1])
        if err != nil {
            return nil, errors.New(fmt.Sprintf(
                "invalid integer %d conversion error '%s'", tagRanges[idx].max, err))
        }

        if tagRanges[idx].min > tagRanges[idx].max {
            return nil, errors.New(fmt.Sprintf(
                "invalid range %s, min is greater than max", oneRangeStr))
        }
        if tagType == "vlan" && tagRanges[idx].max > 4095 {
            return nil, errors.New(fmt.Sprintf(
                "invalid range %s, vlan values exceed 4095 max allowed", oneRangeStr))
        }
        if tagType == "vxlan" && tagRanges[idx].max > 65535 {
            return nil, errors.New(fmt.Sprintf(
                "invalid range %s, vlan values exceed 65535 max allowed", oneRangeStr))
        }
    }

    return tagRanges, nil
}
