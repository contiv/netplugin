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

package utils

import (
	"testing"
)

func TestSystemName(t *testing.T) {
	attrLines := []string{"        contiv-aci-leaf2"}
	sysName, err := lldpReadSystemName(attrLines)

	if err != nil || sysName != "contiv-aci-leaf2" {
		t.Fatalf("failed to get parse lldp system name\n")
	}
}

func TestSystemDescription(t *testing.T) {
	attrLines := []string{"        topology/pod-1/node-102"}
	sysDesc, err := lldpReadSystemDescription(attrLines)

	if err != nil || sysDesc != "topology/pod-1/node-102" {
		t.Fatalf("failed to get parse lldp system description\n")
	}
}

func TestPortDesc(t *testing.T) {
	attrLines := []string{"        topology/pod-1/paths-102/pathep-[eth1/11]"}
	portDesc, err := lldpReadPortDescription(attrLines)

	if err != nil || portDesc != "topology/pod-1/paths-102/pathep-[eth1/11]" {
		t.Fatalf("failed to get parse lldp port description\n")
	}
}

func TestPortID(t *testing.T) {
	attrLines := []string{"        Local: Eth1/11"}
	portID, err := lldpReadPortID(attrLines)

	if err != nil || portID != "Eth1/11" {
		t.Fatalf("failed to get parse lldp port id\n")
	}
}

func TestACIInfo(t *testing.T) {
	podID, err := lldpReadACIPodID("topology/pod-2/node-103")
	if err != nil || podID != "2" {
		t.Fatalf("failed to parse lldp system description for pod id\n")
	}

	nodeID, err := lldpReadACINodeID("topology/pod-2/node-103")
	if err != nil || nodeID != "103" {
		t.Fatalf("failed to parse system info for node id\n")
	}
}

func TestLLDPAttrs(t *testing.T) {
	lldpToolOutput := `
Chassis ID TLV
        MAC: a4:6c:2a:1e:0c:b9
Port ID TLV
        Local: Eth1/11
Time to Live TLV
        120
Port Description TLV
        topology/pod-1/paths-102/pathep-[eth1/11]
System Name TLV
        contiv-aci-leaf2
System Description TLV
        topology/pod-1/node-102
System Capabilities TLV
        System capabilities:  Bridge, Router
        Enabled capabilities: Bridge, Router
Management Address TLV
        MAC: a4:6c:2a:1e:0c:b9
        Ifindex: 83886080
Unidentified Org Specific TLV
        OUI: 0x000142, Subtype: 201, Info: 02
Unidentified Org Specific TLV
        OUI: 0x000142, Subtype: 212, Info: 53414c3139313845394e46
Unidentified Org Specific TLV
        OUI: 0x000142, Subtype: 210, Info: 6e393030302d31312e3128312e32353229
Unidentified Org Specific TLV
        OUI: 0x000142, Subtype: 202, Info: 01
Unidentified Org Specific TLV
        OUI: 0x000142, Subtype: 205, Info: 0001
Unidentified Org Specific TLV
        OUI: 0x000142, Subtype: 208, Info: 0a00505d
Unidentified Org Specific TLV
        OUI: 0x000142, Subtype: 203, Info: 00000066
Unidentified Org Specific TLV
        OUI: 0x000142, Subtype: 206, Info: 636f6e7469762d616369
Unidentified Org Specific TLV
        OUI: 0x000142, Subtype: 207, Info: 010a00000139653462313438362d313939392
End of LLDPDU TLV`

	var parsedInfo LLDPNeighbor

	err := parseLLDPArgs(lldpToolOutput, &parsedInfo)
	if err != nil {
		t.Fatalf("failed to parse lldp attributes\n")
	}
	if parsedInfo.SystemDescription != "topology/pod-1/node-102" ||
		parsedInfo.SystemName != "contiv-aci-leaf2" ||
		parsedInfo.CiscoACIPodID != "1" ||
		parsedInfo.CiscoACINodeID != "102" ||
		parsedInfo.PortDesc != "topology/pod-1/paths-102/pathep-[eth1/11]" ||
		parsedInfo.PortID != "Eth1/11" {
		t.Fatalf("parsed params invalid %v \n", parsedInfo)
	}
}
