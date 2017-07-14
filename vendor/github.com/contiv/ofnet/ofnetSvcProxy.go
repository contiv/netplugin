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

package ofnet

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/contiv/libOpenflow/openflow13"
	"github.com/contiv/libOpenflow/protocol"
	"github.com/contiv/ofnet/pqueue"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/ofnet/ofctrl"
)

// service proxy implementation

const (
	watchedFlowMax = 2
	spDNAT         = "Dst"
	spSNAT         = "Src"
)

// PortSpec defines protocol/port info required to host the service
type PortSpec struct {
	Protocol string
	SvcPort  uint16 // advertised port
	ProvPort uint16 // actual port of provider
}

// ServiceSpec defines a service to be proxied
type ServiceSpec struct {
	IpAddress string
	Ports     []PortSpec
}

// Providers holds the current providers of a given service
type Providers struct {
	Providers map[string]bool // Provider IP as key
}

// svcCatalogue holds information about all services to be proxied
// Accessible by north-bound API
type svcCatalogue struct {
	SvcMap  map[string]ServiceSpec // service name as key
	ProvMap map[string]Providers   // service name as key
}

// provOper holds operational info for each provider
type provOper struct {
	ClientEPs map[string]bool // IP's of endpoints served by the provider
	pqHdl     *pqueue.Item    // handle into the providers pq
}

// proxyOper is operational state of the proxy
type proxyOper struct {
	Ports        []PortSpec
	ProvHdl      map[string]provOper     // provider IP as key
	provPQ       *pqueue.MinPQueue       // provider priority queue for load balancing
	watchedFlows []*ofctrl.Flow          // flows this service is watching
	natFlows     map[string]*ofctrl.Flow // epIP.[in|out] as key
}

// flow info for service
type flowHdl struct {
	SvcIP string
	flow  *ofctrl.Flow
}

// ServiceProxy is an instance of a service proxy
type ServiceProxy struct {
	ofSwitch         *ofctrl.OFSwitch               // openflow switch we are talking to
	agent            *OfnetAgent                    // Pointer back to ofnet agent that owns this
	dNATTable        *ofctrl.Table                  // proxy dNAT rules table
	sNATTable        *ofctrl.Table                  // proxy sNAT rules table
	dNATNext         *ofctrl.Table                  // Next table to goto for dNAT'ed packets
	sNATNext         *ofctrl.Table                  // Next table to goto for sNAT'ed packets
	catalogue        svcCatalogue                   // Services and providers added to the proxy
	oMutex           sync.Mutex                     // mutex between management and datapath
	operState        map[string]*proxyOper          // Operational state info, with service IP as key
	epStats          map[string]*OfnetEndpointStats // stats for the service
	flowMap          map[uint64]flowHdl             // flowId to Info map
	statsPollStarted bool                           // has the stats polling been started?
}

func getIPProto(prot string) uint8 {
	switch prot {
	case "TCP":
		return ofctrl.IP_PROTO_TCP
	case "UDP":
		return ofctrl.IP_PROTO_UDP
	default:
		return 0
	}
}

func matchSpec(s1, s2 *ServiceSpec) bool {
	if s1.IpAddress != s2.IpAddress {
		return false
	}

	if len(s1.Ports) != len(s2.Ports) {
		return false
	}

	// if the order or ports changes, this will treat as a mismatch
	// but, not a big deal...
	for ix := 0; ix < len(s1.Ports); ix++ {
		if s1.Ports[ix].Protocol != s2.Ports[ix].Protocol {
			return false
		}
		if s1.Ports[ix].SvcPort != s2.Ports[ix].SvcPort {
			return false
		}
		if s1.Ports[ix].ProvPort != s2.Ports[ix].ProvPort {
			return false
		}
	}

	return true
}

// allocateProvider gets the provider with least load
// also updates the provider to client linkage
func (svcOp *proxyOper) allocateProvider(clientIP string) (net.IP, error) {
	if svcOp.provPQ.Len() <= 0 {
		return net.ParseIP("0.0.0.0"), errors.New("No provider")
	}
	prov := svcOp.provPQ.GetMin()
	svcOp.provPQ.IncreaseMin()
	svcOp.ProvHdl[prov].ClientEPs[clientIP] = true
	return net.ParseIP(prov), nil
}

func getNATKey(epIP, natT string, p *PortSpec) string {
	key := epIP + "." + natT + "." + p.Protocol + strconv.Itoa(int(p.SvcPort))
	return key
}

// addNATFlow sets up a NAT flow
// natT must be "Src" or "Dst"
func (svcOp *proxyOper) addNATFlow(this, next *ofctrl.Table, p *PortSpec,
	ipSa, ipDa, ipNew *net.IP, natT, macDA string) (*ofctrl.Flow, error) {

	// Check if we already installed this flow
	key := ""
	if natT == "Dst" {
		key = getNATKey(ipSa.String(), natT, p)
	} else {
		key = getNATKey(ipDa.String(), natT, p)
	}
	f, found := svcOp.natFlows[key]
	if found && f != nil {
		log.Infof("Flow already exists for %v", key)
		return f, nil
	}

	match := ofctrl.FlowMatch{
		Priority:  FLOW_MATCH_PRIORITY,
		Ethertype: 0x0800,
		IpSa:      ipSa,
		IpDa:      ipDa,
		IpProto:   getIPProto(p.Protocol),
	}

	if p.Protocol == "TCP" {
		if natT == spDNAT {
			match.TcpDstPort = p.SvcPort
		} else {
			match.TcpSrcPort = p.ProvPort
		}
	} else {
		if natT == spDNAT {
			match.UdpDstPort = p.SvcPort
		} else {
			match.UdpSrcPort = p.ProvPort
		}
	}

	natFlow, err := this.NewFlow(match)

	if err != nil {
		log.Errorf("Proxy addNATFlow failed")
		return nil, errors.New("Proxy addNATFlow failed")
	}

	l4field := p.Protocol + natT // evaluates to TCP[Src,Dst] or UDP[Src,Dst]

	// add actions to update ipda and dest port
	natFlow.SetIPField(*ipNew, natT)
	if natT == spDNAT {
		natFlow.SetL4Field(p.ProvPort, l4field)
	} else {
		natFlow.SetL4Field(p.SvcPort, l4field)
	}

	if macDA != "" {
		// Update dmac as well
		dmac, err := net.ParseMAC(macDA)
		if err != nil {
			log.Errorf("Error parsing mac: %v", err)
			return nil, errors.New("Proxy addNATFlow failed to parse MAC")
		}

		log.Infof("Rewrite mac to %s for provider", macDA)
		natFlow.SetMacDa(dmac)
	}

	natFlow.Next(next)
	svcOp.natFlows[key] = natFlow
	log.Infof("Added NAT %s to %s", key, ipNew.String())

	return natFlow, nil
}

func (svcOp *proxyOper) delNATFlow(proxy *ServiceProxy, epIP, natT string, p *PortSpec) {
	key := getNATKey(epIP, natT, p)

	flow, found := svcOp.natFlows[key]
	if found {
		log.Infof("Deleting NAT %s", key)
		delete(proxy.flowMap, flow.FlowID)
		flow.Delete()
		delete(svcOp.natFlows, key)
	} else {
		log.Infof("DEL NAT %s flow not found", key)
	}
}

func (svcOp *proxyOper) addProvHdl(provIP string) {
	clientMap := make(map[string]bool)
	item := pqueue.NewItem(provIP)
	pOper := provOper{
		ClientEPs: clientMap,
		pqHdl:     item,
	}
	svcOp.ProvHdl[provIP] = pOper
	svcOp.provPQ.PushItem(item)
}

func (proxy *ServiceProxy) addService(svcName string) error {
	// make sure we have a spec and at least one provider
	services := proxy.catalogue.SvcMap
	spec, found := services[svcName]
	if !found {
		log.Debugf("No spec for %s", svcName)
		return nil
	}

	providers := proxy.catalogue.ProvMap
	prov, found := providers[svcName]
	if !found {
		log.Debugf("No providers for %s", svcName)
		return nil
	}

	// Build operational state
	oper := proxy.operState
	proxy.oMutex.Lock()
	defer proxy.oMutex.Unlock()
	_, found = oper[spec.IpAddress]
	if found {
		log.Errorf("Unexpected... operstate found for %s", spec.IpAddress)
		return errors.New("Service IP already exists")
	}

	wFlows := make([]*ofctrl.Flow, watchedFlowMax)
	pq := pqueue.NewMinPQueue()
	pHdl := make(map[string]provOper)
	nFlows := make(map[string]*ofctrl.Flow)
	oState := &proxyOper{Ports: spec.Ports,
		provPQ:       pq,
		watchedFlows: wFlows,
		ProvHdl:      pHdl,
		natFlows:     nFlows,
	}

	// add all providers
	for p := range prov.Providers {
		oState.addProvHdl(p)
	}

	// add the service state to oper map
	oper[spec.IpAddress] = oState

	// add ovs rule to catch service traffic
	// TBD -- handle arps to cover service in same subnet case
	protMap := make(map[string]uint8)
	protMap["TCP"] = 6
	protMap["UDP"] = 17

	count := 0
	svcDA := net.ParseIP(spec.IpAddress)
	for _, port := range spec.Ports {
		prot, found := protMap[port.Protocol]
		if found {
			if count >= watchedFlowMax {
				log.Errorf("Flow count exceeded")
				break
			}
			watchedFlow, err := proxy.dNATTable.NewFlow(ofctrl.FlowMatch{
				Priority:  FLOW_FLOOD_PRIORITY,
				Ethertype: 0x0800,
				IpDa:      &svcDA,
				IpProto:   prot,
			})
			if err != nil {
				log.Errorf("Watch %s proto: %d err: %v", spec.IpAddress,
					prot, err)
				continue
			}
			watchedFlow.Next(proxy.ofSwitch.SendToController())
			oState.watchedFlows[count] = watchedFlow
			delete(protMap, port.Protocol) // add only once
			count++
		}
	}

	return nil
}

// delService deletes a service
func (proxy *ServiceProxy) delService(svcName string) {
	// make sure we have a spec
	services := proxy.catalogue.SvcMap
	spec, found := services[svcName]
	if !found {
		log.Debugf("delService %s not found", svcName)
		return // service does not exist
	}

	// Clean up operational state
	proxy.oMutex.Lock()
	defer proxy.oMutex.Unlock()
	oper := proxy.operState
	operEntry, found := oper[spec.IpAddress]
	if !found {
		log.Debugf("delService operEntry not found for %s", svcName)
		return
	}

	// delete the watched flows from OVS
	for _, flow := range operEntry.watchedFlows {
		if flow != nil {
			flow.Delete()
		}
	}

	// delete the nat'ed flows
	for key, flow := range operEntry.natFlows {
		if flow != nil {
			flow.Delete()
			log.Infof("NAT flow %s deleted", key)
		} else {
			log.Infof("NAT flow %s not found", key)
		}
	}

	// remove the operEntry
	delete(oper, spec.IpAddress)
}

// AddSvcSpec adds or updates a service spec.
func (proxy *ServiceProxy) AddSvcSpec(svcName string, spec *ServiceSpec) error {
	log.Infof("AddSvcSpec %s %v", svcName, spec)
	// Make sure spec is valid
	for _, p := range spec.Ports {
		if p.Protocol != "TCP" && p.Protocol != "UDP" {
			return errors.New("Invalid protocol in spec")
		}
	}

	services := proxy.catalogue.SvcMap
	oldSpec, found := services[svcName]
	if found {
		if matchSpec(&oldSpec, spec) {
			log.Debugf("No change in spec for %s", svcName)
			return nil
		}

		// delete existing spec and add the new one
		proxy.DelSvcSpec(svcName, &oldSpec)
	}

	services[svcName] = *spec
	return proxy.addService(svcName)
}

// DelSvcSpec deletes a service spec.
func (proxy *ServiceProxy) DelSvcSpec(svcName string, spec *ServiceSpec) error {
	log.Infof("DelSvcSpec %s %v", svcName, spec)
	services := proxy.catalogue.SvcMap
	_, found := services[svcName]
	if !found {
		log.Warnf("DelSvcSpec service %s not found", svcName)
		return errors.New("Service not found")
	} else {
		proxy.delService(svcName)
		delete(services, svcName)
	}

	return nil
}

// GetEndpointStats fetches ep stats
func (proxy *ServiceProxy) GetEndpointStats() (map[string]*OfnetEndpointStats, error) {
	return proxy.epStats, nil
}

// addProvider adds the given provider to operational State
func (proxy *ServiceProxy) addProvider(svcIP, provIP string) error {
	oper := proxy.operState
	proxy.oMutex.Lock()
	defer proxy.oMutex.Unlock()
	operEntry, found := oper[svcIP]
	if !found {
		log.Errorf("addProvider operEntry not found for %s", svcIP)
		return errors.New("operEntry not found")
	}
	operEntry.addProvHdl(provIP)
	log.Infof("Added provider %s for serviceIP %s", provIP, svcIP)
	return nil
}

// delProvider deletes the given provider from operational State
func (proxy *ServiceProxy) delProvider(svcIP, provIP string) error {
	oper := proxy.operState
	proxy.oMutex.Lock()
	defer proxy.oMutex.Unlock()
	operEntry, found := oper[svcIP]
	if !found {
		log.Errorf("delProvider operEntry not found for %s", svcIP)
		return errors.New("operEntry not found")
	}

	// Remove flows NAT'ed to this provider
	for epIP := range operEntry.ProvHdl[provIP].ClientEPs {
		for _, p := range operEntry.Ports {
			operEntry.delNATFlow(proxy, epIP, "Dst", &p)
			operEntry.delNATFlow(proxy, epIP, "Src", &p)
		}
	}

	// Remove provider from the loadbalancer pq
	pqItem := operEntry.ProvHdl[provIP].pqHdl
	operEntry.provPQ.RemoveItem(pqItem)
	// remove the provider handle for this provider
	delete(operEntry.ProvHdl, provIP)
	log.Infof("Removed provider %s for serviceIP %s", provIP, svcIP)
	return nil
}

// ProviderUpdate updates the list of providers of the service
func (proxy *ServiceProxy) ProviderUpdate(svcName string, providers []string) {
	log.Infof("ProviderUpdate %s %v", svcName, providers)
	newProvs := make(map[string]bool)

	for _, p := range providers {
		newProvs[p] = true
	}

	pMap := Providers{
		Providers: newProvs,
	}
	// if we don't have the service spec yet, just save the provider
	// map and return
	sSpec, found := proxy.catalogue.SvcMap[svcName]
	if !found {
		proxy.catalogue.ProvMap[svcName] = pMap
		log.Debugf("Service %s -- no spec yet", svcName)
		return
	}

	// if the service is not created, just use the new map and
	// add the service
	svcIP := sSpec.IpAddress
	_, found = proxy.operState[svcIP]
	if !found && len(providers) == 0 {
		log.Debugf("Service %s -- no providers", svcName)
		return
	}

	if !found {
		proxy.catalogue.ProvMap[svcName] = pMap
		err := proxy.addService(svcName)
		if err != nil {
			log.Errorf("ProviderUpdate failed for %s", svcName)
		}
		return
	}

	currProvs := proxy.catalogue.ProvMap[svcName]
	proxy.catalogue.ProvMap[svcName] = pMap

	// if the new provider list is empty, delete the service
	if len(providers) == 0 {
		log.Infof("No providers for service %s, deleting", svcName)
		proxy.delService(svcName)
		return
	}

	// Add any new providers first
	for _, p := range providers {
		_, found = currProvs.Providers[p]
		if !found {
			proxy.addProvider(sSpec.IpAddress, p)
		}
	}

	// Delete any providers that disappeared
	for p := range currProvs.Providers {
		_, found = newProvs[p]
		if !found {
			proxy.delProvider(sSpec.IpAddress, p)
		}
	}
}

// NewServiceProxy Creates a new service proxy
func NewServiceProxy(agent *OfnetAgent) *ServiceProxy {
	svcProxy := new(ServiceProxy)

	// initialize
	svcProxy.agent = agent
	svcProxy.catalogue.SvcMap = make(map[string]ServiceSpec)
	svcProxy.catalogue.ProvMap = make(map[string]Providers)
	svcProxy.operState = make(map[string]*proxyOper)
	svcProxy.flowMap = make(map[uint64]flowHdl)
	svcProxy.epStats = make(map[string]*OfnetEndpointStats)
	svcProxy.statsPollStarted = false

	return svcProxy
}

// Handle switch connected notification
func (proxy *ServiceProxy) SwitchConnected(sw *ofctrl.OFSwitch) {
	// Keep a reference to the switch
	proxy.ofSwitch = sw

	log.Infof("Switch connected(svcProxy).")
}

// Handle switch disconnected notification
func (proxy *ServiceProxy) SwitchDisconnected(sw *ofctrl.OFSwitch) {
	// FIXME: ??
}

// DelEndpoint handles an endpoint delete
func (proxy *ServiceProxy) DelEndpoint(endpoint *OfnetEndpoint) {
	epIP := endpoint.IpAddr.String()

	// delete all nat'ed flows and update loadbalancer
	proxy.oMutex.Lock()
	defer proxy.oMutex.Unlock()
	for _, operEntry := range proxy.operState {
		for _, p := range operEntry.Ports {
			// this client exists iff DNAT flow exists
			key := getNATKey(epIP, "Dst", &p)
			flow, found := operEntry.natFlows[key]
			if found {
				provIP := flow.Match.IpDa.String()
				// delete both flows and remove the client
				operEntry.delNATFlow(proxy, epIP, "Dst", &p)
				operEntry.delNATFlow(proxy, epIP, "Src", &p)
				hdl, ok := operEntry.ProvHdl[provIP]
				if ok {
					delete(hdl.ClientEPs, epIP)
					pqItem := hdl.pqHdl
					operEntry.provPQ.DecreaseItem(pqItem)
				}
			}

		}
	}
}

func getInPort(pkt *ofctrl.PacketIn) uint32 {
	if (pkt.Match.Type == openflow13.MatchType_OXM) &&
		(pkt.Match.Fields[0].Class == openflow13.OXM_CLASS_OPENFLOW_BASIC) &&
		(pkt.Match.Fields[0].Field == openflow13.OXM_FIELD_IN_PORT) {
		// Get the input port number
		switch t := pkt.Match.Fields[0].Value.(type) {
		case *openflow13.InPortField:
			var inPortFld openflow13.InPortField
			inPortFld = *t
			return inPortFld.InPort
		}
	}

	return openflow13.P_ANY
}

// getRewriteMAC returns the providers mac if the client and provider are
// in the same subnet
func (proxy *ServiceProxy) getRewriteMAC(inPort uint32, provIP net.IP) string {
	clientEP := proxy.agent.getLocalEndpoint(inPort)
	if clientEP != nil {
		provEP := proxy.agent.getEndpointByIpVlan(provIP, clientEP.Vlan)
		if provEP != nil {
			if provEP.Vlan == clientEP.Vlan {
				return provEP.MacAddrStr
			}
		}

		log.Debugf("provEP not found for vlan %d", clientEP.Vlan)
	} else {
		log.Debugf("ClientEP not found for port %d", inPort)
	}

	log.Debugf("ProvMac not found for %s", provIP.String())
	return ""
}

// HandlePkt processes a received pkt from a matching table entry
func (proxy *ServiceProxy) HandlePkt(pkt *ofctrl.PacketIn) {

	if pkt.TableId != SRV_PROXY_DNAT_TBL_ID {
		return // ignore other packets
	}

	if pkt.Data.Ethertype != protocol.IPv4_MSG {
		return // ignore non-IP pkts
	}

	if pkt.Data.HWSrc.String() == "00:00:11:11:11:11" {
		log.Warnf("Pkt received with our src mac. Loop???")
		return // pkt we sent
	}

	ip := pkt.Data.Data.(*protocol.IPv4)

	svcIP := ip.NWDst.String()

	log.Infof("HandlePkt svcIP: %s", svcIP)
	proxy.oMutex.Lock()
	defer proxy.oMutex.Unlock()

	operEntry, found := proxy.operState[svcIP]
	if !found {
		return // this means service was just deleted
	}
	clientIP := ip.NWSrc.String()
	provIP, err := operEntry.allocateProvider(clientIP)
	if err != nil {
		log.Warnf("allocateProvider failed for %s - %v", svcIP, err)
		return
	}

	inPort := getInPort(pkt)
	provMac := proxy.getRewriteMAC(inPort, provIP)
	// use copies of fields from the pkt
	ipSrc := net.ParseIP(ip.NWSrc.String())
	ipDst := net.ParseIP(ip.NWDst.String())
	fInfo := flowHdl{SvcIP: svcIP}

	// setup nat rules in both directions for all ports of the service
	for _, p := range operEntry.Ports {
		// set up outgoing NAT
		f, err := operEntry.addNATFlow(proxy.dNATTable, proxy.dNATNext, &p, &ipSrc, &ipDst, &provIP, spDNAT, provMac)
		if err == nil {
			fInfo.flow = f
			proxy.flowMap[f.FlowID] = fInfo
		} else {
			continue
		}

		// set up incoming NAT
		f, err = operEntry.addNATFlow(proxy.sNATTable, proxy.sNATNext, &p, &provIP, &ipSrc, &ipDst, spSNAT, "")
		if err == nil {
			fInfo.flow = f
			proxy.flowMap[f.FlowID] = fInfo
		}
	}

	if pkt.Data.HWSrc.String() == "00:00:00:00:00:00" {
		return // don't inject crafted pkt
	}

	// re-inject this pkt, change src mac to allow loop detection
	pkt.Data.HWSrc, _ = net.ParseMAC("00:00:11:11:11:11")

	// Packet out
	pktOut := openflow13.NewPacketOut()
	pktOut.InPort = inPort
	pktOut.Data = &pkt.Data
	pktOut.AddAction(openflow13.NewActionOutput(openflow13.P_TABLE))

	// Send it out
	proxy.ofSwitch.Send(pktOut)

	// Start stats polling if it hasnt started already
	if !proxy.statsPollStarted {
		proxy.statsPollStarted = true
		go proxy.pollStats()
	}
}

func (proxy *ServiceProxy) updateSNATStats(fs *openflow13.FlowStats) {
	// find svcIP
	flowInfo, found := proxy.flowMap[fs.Cookie]
	if !found {
		return // Flow is probably deleted
	}

	svcIP := flowInfo.SvcIP
	fm := flowInfo.flow.Match
	provIP := fm.IpSa.String()
	epIP := fm.IpDa.String()
	entry, found := proxy.epStats[epIP]
	if !found {
		entry = &OfnetEndpointStats{}
		entry.EndpointIP = epIP
		entry.SvcStats = make(map[string]OfnetSvcStats)
		proxy.epStats[epIP] = entry
	}

	_, found = entry.SvcStats[svcIP]
	if !found {
		entry.SvcStats[svcIP] = OfnetSvcStats{
			ServiceIP: svcIP,
			ProvStats: make(map[string]OfnetSvcProviderStats),
		}
	}

	stats := entry.SvcStats[svcIP]
	if fm.IpProto == ofctrl.IP_PROTO_TCP {
		stats.Protocol = "TCP"
		stats.ProvPort = strconv.Itoa(int(fm.TcpSrcPort))
	} else {
		stats.Protocol = "UDP"
		stats.ProvPort = strconv.Itoa(int(fm.UdpSrcPort))
	}

	_, found = stats.ProvStats[provIP]
	if !found {
		stats.ProvStats[provIP] = OfnetSvcProviderStats{ProviderIP: provIP}
	}
	provStats := stats.ProvStats[provIP]
	provStats.PacketsIn = fs.PacketCount
	provStats.BytesIn = fs.ByteCount

	stats.ProvStats[provIP] = provStats
	entry.SvcStats[svcIP] = stats

	log.Debugf("SNAT Stats: epIP: %s, svcIp: %s, entry: %+v", epIP, svcIP, entry)

}

func (proxy *ServiceProxy) updateDNATStats(fs *openflow13.FlowStats) {
	flowInfo, found := proxy.flowMap[fs.Cookie]
	if !found {
		return // Flow is probably deleted
	}

	svcIP := flowInfo.SvcIP
	fm := flowInfo.flow.Match
	if fm.TcpSrcPort == 0 && fm.TcpDstPort == 0 &&
		fm.UdpSrcPort == 0 && fm.UdpDstPort == 0 {
		return // watch flow
	}
	provIP := fm.IpSa.String()
	epIP := fm.IpSa.String()
	entry, found := proxy.epStats[epIP]
	if !found {
		entry = &OfnetEndpointStats{}
		entry.EndpointIP = epIP
		entry.SvcStats = make(map[string]OfnetSvcStats)
		proxy.epStats[epIP] = entry
	}

	_, found = entry.SvcStats[svcIP]
	if !found {
		entry.SvcStats[svcIP] = OfnetSvcStats{
			ServiceIP: svcIP,
			ProvStats: make(map[string]OfnetSvcProviderStats),
		}
	}
	stats := entry.SvcStats[svcIP]

	if fm.IpProto == ofctrl.IP_PROTO_TCP {
		stats.Protocol = "TCP"
		stats.SvcPort = strconv.Itoa(int(fm.TcpDstPort))
	} else {
		stats.Protocol = "UDP"
		stats.SvcPort = strconv.Itoa(int(fm.UdpDstPort))
	}

	_, found = stats.ProvStats[provIP]
	if !found {
		stats.ProvStats[provIP] = OfnetSvcProviderStats{ProviderIP: provIP}
	}
	provStats := stats.ProvStats[provIP]
	provStats.PacketsOut = fs.PacketCount
	provStats.BytesOut = fs.ByteCount

	stats.ProvStats[provIP] = provStats
	entry.SvcStats[svcIP] = stats

	log.Debugf("DNAT Stats: epIP: %s, svcIp: %s, entry: %+v", epIP, svcIP, entry)
}

// FlowStats handles a stats response from the switch
func (proxy *ServiceProxy) FlowStats(reply *openflow13.MultipartReply) {
	if reply.Type != openflow13.MultipartType_Flow {
		log.Warnf("Unexpected MP Reply type: %+v", reply)
		return
	}

	flowArr := reply.Body
	for _, entry := range flowArr {
		flowStats := entry.(*openflow13.FlowStats)

		log.Debugf("Got flow stats: %+v", flowStats)

		if flowStats.TableId == SRV_PROXY_DNAT_TBL_ID {
			proxy.updateDNATStats(flowStats)
		}

		if flowStats.TableId == SRV_PROXY_SNAT_TBL_ID {
			proxy.updateSNATStats(flowStats)
		}
	}
}

func getMPReq() *openflow13.MultipartRequest {
	mp := &openflow13.MultipartRequest{}
	mp.Type = openflow13.MultipartType_Flow
	mp.Header = openflow13.NewOfp13Header()
	mp.Header.Type = openflow13.Type_MultiPartRequest
	return mp
}

func (proxy *ServiceProxy) pollStats() {
	time.Sleep(20 * time.Second)

	for {
		time.Sleep(1 * time.Second)
		//inject a stats request
		dnatReq := openflow13.NewFlowStatsRequest()
		dnatReq.TableId = SRV_PROXY_DNAT_TBL_ID
		mp1 := getMPReq()
		mp1.Body = dnatReq
		proxy.ofSwitch.Send(mp1)
		log.Debugf("Sent DNAT stats req")
		time.Sleep(1 * time.Second)
		//inject a stats request
		snatReq := openflow13.NewFlowStatsRequest()
		snatReq.TableId = SRV_PROXY_SNAT_TBL_ID
		mp2 := getMPReq()
		mp2.Body = snatReq
		proxy.ofSwitch.Send(mp2)
		log.Debugf("Sent SNAT stats req")
	}
}

// InitSNATTable initializes the sNAT table
func (proxy *ServiceProxy) InitSNATTable(nextIDsNAT uint8) error {
	sw := proxy.ofSwitch

	nextTbl := sw.GetTable(nextIDsNAT)
	if nextTbl == nil {
		log.Fatalf("Error getting table id: %d", nextIDsNAT)
	}

	proxy.sNATNext = nextTbl
	// Create table
	proxy.sNATTable, _ = sw.NewTable(SRV_PROXY_SNAT_TBL_ID)
	// Packets that didnt match any rule go to next table
	proxyMissFlow, _ := proxy.sNATTable.NewFlow(ofctrl.FlowMatch{
		Priority: FLOW_MISS_PRIORITY,
	})
	proxyMissFlow.Next(proxy.sNATNext)

	return nil
}

// InitDNATTable initializes the dNAT table
func (proxy *ServiceProxy) InitDNATTable(nextIDdNAT uint8) error {
	sw := proxy.ofSwitch

	nextTbl := sw.GetTable(nextIDdNAT)
	if nextTbl == nil {
		log.Fatalf("Error getting table id: %d", nextIDdNAT)
	}

	proxy.dNATNext = nextTbl

	// Create dNAT table
	proxy.dNATTable, _ = sw.NewTable(SRV_PROXY_DNAT_TBL_ID)

	proxyMissFlow, _ := proxy.dNATTable.NewFlow(ofctrl.FlowMatch{
		Priority: FLOW_MISS_PRIORITY,
	})
	proxyMissFlow.Next(proxy.dNATNext)

	return nil
}

// InspectState returns current state of svc proxy
func (proxy *ServiceProxy) InspectState() interface{} {
	proxyExport := struct {
		Catalogue svcCatalogue                   // Services and providers added to the proxy
		OperState map[string]*proxyOper          // Operational state info, with service IP as key
		EpStats   map[string]*OfnetEndpointStats // stats for the service
		// FlowMap   map[uint64]flowHdl             // flowId to Info map
	}{
		proxy.catalogue,
		proxy.operState,
		proxy.epStats,
		// proxy.flowMap,
	}

	return &proxyExport
}

// GetSvcProxyMAC looks up the service IP and returns a mac if the IP exists.
// if the svc IP is not found, returns empty string
func (proxy *ServiceProxy) GetSvcProxyMAC(svcIP net.IP) string {
	ip := svcIP.String()

	proxy.oMutex.Lock()
	defer proxy.oMutex.Unlock()

	_, found := proxy.operState[ip]
	if found {
		ipv4 := svcIP.To4()
		return fmt.Sprintf("02:02:%02x:%02x:%02x:%02x", ipv4[0], ipv4[1], ipv4[2], ipv4[3])
	}

	return ""

}
