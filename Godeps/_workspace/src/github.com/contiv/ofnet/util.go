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

package ofnet

import (
	"errors"
	"net"
	"strconv"
	"strings"

	"github.com/shaleman/libOpenflow/openflow13"
	"github.com/shaleman/libOpenflow/protocol"

	log "github.com/Sirupsen/logrus"
)

// ParseCIDR parses a CIDR string into a gateway IP and length.
func ParseCIDR(cidrStr string) (string, uint, error) {
	strs := strings.Split(cidrStr, "/")
	if len(strs) != 2 {
		return "", 0, errors.New("invalid cidr format")
	}

	subnetStr := strs[0]
	subnetLen, err := strconv.Atoi(strs[1])
	if subnetLen > 32 || err != nil {
		return "", 0, errors.New("invalid mask in gateway/mask specification ")
	}

	return subnetStr, uint(subnetLen), nil
}

// ParseIPAddrMaskString Parse IP addr string
func ParseIPAddrMaskString(ipAddr string) (*net.IP, *net.IP, error) {
	if strings.Contains(ipAddr, "/") {
		ipDav, ipNet, err := net.ParseCIDR(ipAddr)
		if err != nil {
			log.Errorf("Error parsing ip %s. Err: %v", ipAddr, err)
			return nil, nil, err
		}

		ipMask := net.ParseIP("255.255.255.255").Mask(ipNet.Mask)

		return &ipDav, &ipMask, nil
	}

	ipDav := net.ParseIP(ipAddr)
	if ipDav == nil {
		log.Errorf("Error parsing ip %s.", ipAddr)
		return nil, nil, errors.New("Error parsing ip address")
	}

	ipMask := net.ParseIP("255.255.255.255")

	return &ipDav, &ipMask, nil

}

// BuildGarpPkt builds a Gratuitous ARP packet
func BuildGarpPkt(ip net.IP, mac net.HardwareAddr, vlanID uint16) *openflow13.PacketOut {

    zMac, _ := net.ParseMAC("00:00:00:00:00:00")
    bMac, _ := net.ParseMAC("FF:FF:FF:FF:FF:FF")

	garpPkt, _ := protocol.NewARP(protocol.Type_Request)
	garpPkt.HWSrc = mac
	garpPkt.IPSrc = ip
	garpPkt.HWDst = zMac
	garpPkt.IPDst = ip

	// Build the ethernet packet
	ethPkt := protocol.NewEthernet()
	ethPkt.VLANID.VID = vlanID
	ethPkt.HWDst = bMac
	ethPkt.HWSrc = mac
	ethPkt.Ethertype = 0x0806
	ethPkt.Data = garpPkt

	// Construct Packet out
	pktOut := openflow13.NewPacketOut()
	pktOut.Data = ethPkt

    return pktOut
}
