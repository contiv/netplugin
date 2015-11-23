/***
Copyright 2015 Cisco Systems Inc. All rights reserved.

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
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/core"
)

const (
	invalidAttr         = ""
	lldpadContainerName = "lldpad"
)

// Implement utilities to fetch lldp information aobut connecting neighbors.
// The lldp information can be used in non cloud environment to automate
// networking configuration on the physical devices

// LLDPNeighbor lists the neighbor attributes as learned by the host
type LLDPNeighbor struct {
	SystemName        string
	SystemDescription string
	PortID            string
	PortDesc          string
	CiscoACIPodID     string
	CiscoACINodeID    string
}

/* FIXME: Migrate this to samalba/dockerclient
// FetchLLDPInfo reads the lldp information using lldptool
// A native go lib could be done but it requires that lldpad daemon is
// accessible natively on the lldp ipc path
func FetchLLDPInfo(crt *crt.CRT, ifname string) (*LLDPNeighbor, error) {
	lldp := &LLDPNeighbor{}

	if crt.GetContainerID(lldpadContainerName) == "" {
		log.Errorf("Unable to fetch %q container information - please check if it is running", lldpadContainerName)
		return nil, core.Errorf("unable to fetch container %s info", lldpadContainerName)
	}

	cmdWithArgs := []string{"lldptool", "-n", "-t", "-i", ifname}
	output, err := crt.ExecContainer(lldpadContainerName, cmdWithArgs...)
	if err != nil {
		log.Errorf("Error reading lldp information. Error: %s Output: \n%s", err, output)
		return nil, err
	}

	err = parseLLDPArgs(string(output), lldp)

	return lldp, err
}
*/

func parseLLDPArgs(lldpToolOutput string, lldp *LLDPNeighbor) error {
	var err error
	lines := strings.Split(lldpToolOutput, "\n")
	maxIdx := len(lines) - 1
	for idx := 0; idx < maxIdx; {
		line := strings.Trim(lines[idx], "\n")
		idx++

		if !strings.Contains(line, "TLV") ||
			strings.Contains(line, "End of LLDPDU TLV") {
			continue
		}

		attrIdx := idx
		for !strings.Contains(lines[attrIdx], "TLV") && attrIdx <= maxIdx {
			lines[attrIdx] = strings.Trim(lines[attrIdx], "\n")
			attrIdx++
		}
		tlvLines := lines[idx:attrIdx]
		idx = attrIdx
		err = nil
		switch {
		case strings.Contains(line, "System Name TLV"):
			lldp.SystemName, err = lldpReadSystemName(tlvLines)
		case strings.Contains(line, "System Description TLV"):
			lldp.SystemDescription, err = lldpReadSystemDescription(tlvLines)
		case strings.Contains(line, "Port ID TLV"):
			lldp.PortID, err = lldpReadPortID(tlvLines)
		case strings.Contains(line, "Port Description TLV"):
			lldp.PortDesc, err = lldpReadPortDescription(tlvLines)
		}
		if err != nil {
			log.Errorf("Error parsing lldp information. Error '%v'", err)
		}
	}
	err = deriveLLDPArgs(lldp)
	if err != nil {
		log.Errorf("Error parsing lldp information. Error '%v'", err)
	}

	return err
}

func deriveLLDPArgs(lldp *LLDPNeighbor) error {
	var err error

	lldp.CiscoACIPodID, err = lldpReadACIPodID(lldp.SystemDescription)
	if err != nil {
		log.Errorf("Error parsing lldp information. Error '%v'", err)
	}

	lldp.CiscoACINodeID, err = lldpReadACINodeID(lldp.SystemDescription)
	if err != nil {
		log.Errorf("Error parsing lldp information. Error '%v'", err)
	}

	return err
}

func lldpReadSystemName(attrLines []string) (string, error) {
	if len(attrLines) < 1 {
		return invalidAttr, core.Errorf("empty attributes")
	}

	return strings.Trim(attrLines[0], " "), nil
}

func lldpReadSystemDescription(attrLines []string) (string, error) {
	if len(attrLines) < 1 {
		return invalidAttr, core.Errorf("empty attributes")
	}

	return strings.Trim(attrLines[0], " "), nil
}

func lldpReadPortID(attrLines []string) (string, error) {
	if len(attrLines) < 1 {
		return invalidAttr, core.Errorf("empty attributes")
	}

	words := strings.Split(attrLines[0], ":")
	portID := ""
	if len(words) >= 2 {
		portID = strings.Trim(words[1], " ")
	}
	return portID, nil
}

func lldpReadPortDescription(attrLines []string) (string, error) {
	if len(attrLines) < 1 {
		return invalidAttr, core.Errorf("empty attributes")
	}

	return strings.Trim(attrLines[0], " "), nil
}

func lldpReadACIPodID(systemDesc string) (string, error) {
	if !strings.Contains(systemDesc, "pod-") {
		return invalidAttr, core.Errorf("unable to parse pod id from sysname")
	}
	words := strings.Split(systemDesc, "/")
	podID := ""
	if len(words) >= 2 {
		podID = strings.Split(words[1], "-")[1]
	}
	return podID, nil
}

func lldpReadACINodeID(systemDesc string) (string, error) {
	if !strings.Contains(systemDesc, "node-") {
		return invalidAttr, core.Errorf("unable to parse node id from sysname")
	}
	words := strings.Split(systemDesc, "/")
	nodeID := ""
	if len(words) >= 2 {
		nodeID = strings.Split(words[2], "-")[1]
	}
	return nodeID, nil
}
