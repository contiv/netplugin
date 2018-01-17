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

package ovsd

import (
	"fmt"
	osexec "os/exec"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/core"
)

const (
	contivNPChain    = "CONTIV-NODEPORT"
	iptablesWaitLock = "5"
)

// Presence indicates presence of an item
type Presence struct {
	Items map[string]bool
}

// NodeSvcProxy holds service proxy info
type NodeSvcProxy struct {
	Mutex        sync.Mutex
	SvcMap       map[string]core.ServiceSpec // service name as key
	ProvMap      map[string]Presence         // service name as key
	LocalIP      map[string]string           // globalIP as key
	ipTablesPath string
	natRules     map[string][]string // natRule for the service
}

// NewNodeProxy creates an instance of the node proxy
func NewNodeProxy() (*NodeSvcProxy, error) {
	ipTablesPath, err := osexec.LookPath("iptables")
	if err != nil {
		return nil, err
	}

	// Install contiv chain and jump
	out, err := osexec.Command(ipTablesPath, "-w", iptablesWaitLock,
		"-t", "nat", "-N", contivNPChain).CombinedOutput()
	if err != nil {
		if !strings.Contains(string(out), "Chain already exists") {
			log.Errorf("Failed to setup contiv nodeport chain %v out: %s",
				err, out)
			return nil, err
		}
	}

	_, err = osexec.Command(ipTablesPath, "-w", iptablesWaitLock, "-t", "nat",
		"-C", "PREROUTING", "-m", "addrtype", "--dst-type", "LOCAL", "-j",
		contivNPChain).CombinedOutput()
	if err != nil {
		out, err = osexec.Command(ipTablesPath, "-w", iptablesWaitLock,
			"-t", "nat", "-I", "PREROUTING", "-m", "addrtype", "--dst-type",
			"LOCAL", "-j", contivNPChain).CombinedOutput()
		if err != nil {
			log.Errorf("Failed to setup contiv nodeport chain jump %v out: %s",
				err, out)
			return nil, err
		}
	}

	// Flush any old rules we might have added. They will get re-added
	// if the service is still active
	osexec.Command(ipTablesPath, "-w", iptablesWaitLock, "-t", "nat", "-F",
		contivNPChain).CombinedOutput()

	proxy := NodeSvcProxy{}
	proxy.SvcMap = make(map[string]core.ServiceSpec)
	proxy.ProvMap = make(map[string]Presence)
	proxy.LocalIP = make(map[string]string)
	proxy.ipTablesPath = ipTablesPath
	proxy.natRules = make(map[string][]string)
	return &proxy, nil
}

// DeleteLocalIP removes an entry from the localIP map
func (p *NodeSvcProxy) DeleteLocalIP(globalIP string) {
	// strip cidr
	globalIP = strings.Split(globalIP, "/")[0]
	p.Mutex.Lock()
	defer p.Mutex.Unlock()
	delete(p.LocalIP, globalIP)
}

// AddLocalIP adds an entry to the localIP map
func (p *NodeSvcProxy) AddLocalIP(globalIP, localIP string) {
	// strip cidr
	globalIP = strings.Split(globalIP, "/")[0]
	localIP = strings.Split(localIP, "/")[0]
	p.Mutex.Lock()
	defer p.Mutex.Unlock()
	p.LocalIP[globalIP] = localIP
}

func (p *NodeSvcProxy) detectClash(svcName string, nodePort uint16) bool {
	// verify if there is a clashing nodeport
	for svc, s := range p.SvcMap {
		if svc == svcName {
			continue
		}

		for _, port := range s.Ports {
			if port.NodePort == nodePort && port.Protocol == "TCP" {
				log.Errorf("CONTIV-NODEPORT: %s/%d clashes with %s/%d",
					svcName, nodePort, svc, nodePort)
				return true
			}
		}
	}

	return false
}

// AddSvcSpec adds a service to the proxy
func (p *NodeSvcProxy) AddSvcSpec(svcName string, spec *core.ServiceSpec) error {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()
	log.Infof("Node proxy AddSvcSpec: %s", svcName)
	// Determine if this is a node service
	isNodeSvc := false
	for _, port := range spec.Ports {
		if port.NodePort != 0 && port.Protocol == "TCP" {
			isNodeSvc = true
			if p.detectClash(svcName, port.NodePort) {
				return nil
			}
		}
	}

	if !isNodeSvc {
		p.deleteSvc(svcName) // delete it if it exists
		return nil
	}

	p.SvcMap[svcName] = *spec
	p.syncSvc(svcName)
	return nil
}

// DelSvcSpec deletes a service from the proxy
func (p *NodeSvcProxy) DelSvcSpec(svcName string, spec *core.ServiceSpec) error {
	p.Mutex.Lock()
	defer p.Mutex.Unlock()
	p.deleteSvc(svcName) // delete it if it exists
	return nil
}

// SvcProviderUpdate invokes switch api
func (p *NodeSvcProxy) SvcProviderUpdate(svcName string, providers []string) {
	log.Infof("Node proxy AddSvcSpec: %s %v", svcName, providers)
	p.Mutex.Lock()
	defer p.Mutex.Unlock()
	// check if there is at least one local provider
	localProv := false
	for _, prov := range providers {
		_, localProv = p.LocalIP[prov]
		if localProv {
			break
		}
	}

	if !localProv { // get rid of the natRules if it exists
		p.deleteSvcRules(svcName)
		delete(p.ProvMap, svcName)
		return
	}

	// Make a new provider map
	newProv := make(map[string]bool)
	provMap := Presence{
		Items: newProv,
	}
	p.ProvMap[svcName] = provMap

	for _, prov := range providers {
		_, found := p.LocalIP[prov]
		if found {
			provMap.Items[prov] = true
		}
	}

	p.syncSvc(svcName)
}

func (p *NodeSvcProxy) execNATRule(act, dport, dest string) (string, error) {
	out, err := osexec.Command(p.ipTablesPath, "-w", iptablesWaitLock,
		"-t", "nat", act, contivNPChain, "-p", "tcp", "-m", "tcp", "--dport",
		dport, "-j", "DNAT", "--to-destination",
		dest).CombinedOutput()
	return string(out), err
}
func (p *NodeSvcProxy) syncSvc(svcName string) {
	// check if the service is active
	_, found := p.SvcMap[svcName]

	if !found {
		p.deleteSvcRules(svcName)
		return
	}

	pMap, found := p.ProvMap[svcName]
	if !found {
		p.deleteSvcRules(svcName)
		return
	}

	active := false
	prov := ""
	for prov = range pMap.Items {
		active = true
		break
	}

	if active {
		p.installSvcRules(svcName, p.LocalIP[prov])
		return
	}
	p.deleteSvcRules(svcName)
}
func findString(lines []string, matchStr string) bool {
	for _, line := range lines {
		if strings.Contains(line, matchStr) {
			return true
		}
	}

	return false
}

func (p *NodeSvcProxy) installSvcRules(svcName, prov string) {
	allPresent := true
	spec := p.SvcMap[svcName]
	providers := p.ProvMap[svcName]
	natRules, found := p.natRules[svcName]
	provToUse := prov
	if found {
		if len(natRules) == 0 {
			found = false
		}
	}

	if !found {
		allPresent = false
	} else {
		// find the in-use provider
		for prov := range providers.Items {
			localProv := p.LocalIP[prov]
			if strings.Contains(natRules[0], localProv) {
				provToUse = localProv
				break
			}
		}

		// Check if all required NAT rules are present
		for _, port := range spec.Ports {
			matchStr := fmt.Sprintf(":%d", port.ProvPort)
			if !findString(natRules, matchStr) {
				allPresent = false
				break
			}
		}
	}

	if allPresent {
		log.Infof("Svc %s -- all rules present", svcName)
		return
	}

	// Remove all previous rules and install new ones
	p.deleteSvcRules(svcName)

	natRules = make([]string, 0, len(spec.Ports))
	for _, port := range spec.Ports {
		if port.NodePort == 0 || port.Protocol != "TCP" {
			continue
		}

		dport := fmt.Sprintf("%d", port.NodePort)
		dest := fmt.Sprintf("%s:%d", provToUse, port.ProvPort)
		out, err := p.execNATRule("-A", dport, dest)
		addRule := dport + "/" + dest
		if err != nil {
			log.Errorf("Failed to add rule: %s, err: %v - %s",
				addRule, err, out)
		} else {
			natRules = append(natRules, addRule)
			log.Infof("Added %s", addRule)
		}
	}

	p.natRules[svcName] = natRules
}

func (p *NodeSvcProxy) deleteSvcRules(svcName string) {
	natRules, found := p.natRules[svcName]
	if !found {
		return
	}
	// Remove all rules
	for _, rule := range natRules {
		delRule := strings.Split(rule, "/")
		out, err := p.execNATRule("-D", delRule[0], delRule[1])
		if err != nil {
			log.Errorf("Failed to delete rule: %s, err: %v - %s",
				rule, err, out)
		} else {
			log.Infof("Deleted %s", rule)
		}
	}

	delete(p.natRules, svcName)
}

func (p *NodeSvcProxy) deleteSvc(svcName string) {
	p.deleteSvcRules(svcName)
	delete(p.SvcMap, svcName)
}
