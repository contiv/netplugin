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

// This file implements the vlan bridging datapath

import (
	"fmt"
	"net"
	"net/rpc"
	"sync"
	"syscall"
	"time"

	"github.com/contiv/libOpenflow/openflow13"
	"github.com/contiv/libOpenflow/protocol"
	"github.com/contiv/ofnet/ofctrl"
	cmap "github.com/streamrail/concurrent-map"
	"github.com/vishvananda/netlink"

	log "github.com/Sirupsen/logrus"
)

// Vlan bridging currently uses native OVS bridging.
// This is mostly stub code.

const (
	GARPRepeats = 15
	GARPDELAY   = 3
)

// VlanBridge has Vlan state.
type VlanBridge struct {
	agent       *OfnetAgent      // Pointer back to ofnet agent that owns this
	ofSwitch    *ofctrl.OFSwitch // openflow switch we are talking to
	policyAgent *PolicyAgent     // Policy agent
	svcProxy    *ServiceProxy    // Service proxy

	// Fgraph tables
	inputTable *ofctrl.Table // Packet lookup starts here
	vlanTable  *ofctrl.Table // Vlan Table. map port or VNI to vlan
	nmlTable   *ofctrl.Table // OVS normal lookup table

	// Flow Database
	portVlanFlowDb map[uint32]*ofctrl.Flow   // Database of flow entries
	portDnsFlowDb  cmap.ConcurrentMap        // Database of flow entries
	dscpFlowDb     map[uint32][]*ofctrl.Flow // Database of flow entries

	// Arp Flow
	arpRedirectFlow *ofctrl.Flow // ARP redirect flow entry

	uplinkPortDb cmap.ConcurrentMap // Database of uplink ports
	linkDb       cmap.ConcurrentMap // Database of all links
	garpMutex    *sync.Mutex
	epgToEPs     map[int]epgGARPInfo // Database of eps per epg
	garpBGActive bool
	updChan      chan netlink.LinkUpdate // channel to monitor for link events
	nlChan       chan struct{}           // channel to close the netlink listener
}

// epgGARPInfo holds info for epg
type epgGARPInfo struct {
	garpCount int                 // number of garps yet to be sent on this epg
	epMap     map[uint32]GARPInfo // map of eps on this epg
}

// GARPInfo for each EP
type GARPInfo struct {
	ip   net.IP
	mac  net.HardwareAddr
	vlan uint16
}

// GetUplink API gets the uplink port with uplinkID from uplink DB
func (vl *VlanBridge) GetUplink(uplinkID string) *PortInfo {
	uplink, ok := vl.uplinkPortDb.Get(uplinkID)
	if !ok {
		return nil
	}
	return uplink.(*PortInfo)
}

// GetLink API gets the interface with linkID from interface DB
func (vl *VlanBridge) GetLink(linkID string) *LinkInfo {
	link, ok := vl.linkDb.Get(linkID)
	if !ok {
		return nil
	}
	return link.(*LinkInfo)
}

// NewVlanBridge Create a new vlan instance
func NewVlanBridge(agent *OfnetAgent, rpcServ *rpc.Server) *VlanBridge {
	vlan := new(VlanBridge)

	// Keep a reference to the agent
	vlan.agent = agent

	// init maps
	vlan.portVlanFlowDb = make(map[uint32]*ofctrl.Flow)
	vlan.portDnsFlowDb = cmap.New()
	vlan.dscpFlowDb = make(map[uint32][]*ofctrl.Flow)
	vlan.uplinkPortDb = cmap.New()
	vlan.linkDb = cmap.New()
	vlan.epgToEPs = make(map[int]epgGARPInfo)
	vlan.garpMutex = &sync.Mutex{}
	vlan.garpBGActive = false

	vlan.svcProxy = NewServiceProxy(agent)
	// Create policy agent
	vlan.policyAgent = NewPolicyAgent(agent, rpcServ)

	return vlan
}

// MasterAdded Handle new master added event
func (vl *VlanBridge) MasterAdded(master *OfnetNode) error {

	return nil
}

// SwitchConnected Handle switch connected notification
func (vl *VlanBridge) SwitchConnected(sw *ofctrl.OFSwitch) {
	// Keep a reference to the switch
	vl.ofSwitch = sw

	vl.svcProxy.SwitchConnected(sw)
	// Tell the policy agent about the switch
	vl.policyAgent.SwitchConnected(sw)

	// Init the Fgraph
	vl.initFgraph()

	log.Infof("Switch connected(vlan)")
}

// SwitchDisconnected Handle switch disconnected notification
func (vl *VlanBridge) SwitchDisconnected(sw *ofctrl.OFSwitch) {

	vl.svcProxy.SwitchDisconnected(sw)
	// Tell the policy agent about the switch disconnected
	vl.policyAgent.SwitchDisconnected(sw)

	vl.ofSwitch = nil
}

// PacketRcvd Handle incoming packet
func (vl *VlanBridge) PacketRcvd(sw *ofctrl.OFSwitch, pkt *ofctrl.PacketIn) {
	if pkt.TableId == SRV_PROXY_SNAT_TBL_ID || pkt.TableId == SRV_PROXY_DNAT_TBL_ID {
		// these are destined to service proxy
		vl.svcProxy.HandlePkt(pkt)
		return
	}

	switch pkt.Data.Ethertype {
	case 0x0806:
		if (pkt.Match.Type == openflow13.MatchType_OXM) &&
			(pkt.Match.Fields[0].Class == openflow13.OXM_CLASS_OPENFLOW_BASIC) &&
			(pkt.Match.Fields[0].Field == openflow13.OXM_FIELD_IN_PORT) {
			// Get the input port number
			switch t := pkt.Match.Fields[0].Value.(type) {
			case *openflow13.InPortField:
				var inPortFld openflow13.InPortField
				inPortFld = *t

				vl.processArp(pkt.Data, inPortFld.InPort)
			}
		}

	case protocol.IPv4_MSG:
		var inPort uint32
		if (pkt.TableId == 0) && (pkt.Match.Type == openflow13.MatchType_OXM) &&
			(pkt.Match.Fields[0].Class == openflow13.OXM_CLASS_OPENFLOW_BASIC) &&
			(pkt.Match.Fields[0].Field == openflow13.OXM_FIELD_IN_PORT) {
			// Get the input port number
			switch t := pkt.Match.Fields[0].Value.(type) {
			case *openflow13.InPortField:
				inPort = t.InPort
			default:
				log.Debugf("unknown match type %v for ipv4 pkt", t)
				return
			}
		}
		ipPkt := pkt.Data.Data.(*protocol.IPv4)
		switch ipPkt.Protocol {
		case protocol.Type_UDP:
			udpPkt := ipPkt.Data.(*protocol.UDP)
			switch udpPkt.PortDst {
			case 53:
				if pkt.Data.VLANID.VID != 0 {
					vl.agent.incrErrStats("dnsPktUplink")
					return
				}

				if dnsResp, err := processDNSPkt(vl.agent, inPort, udpPkt.Data); err == nil {
					if respPkt, err := buildUDPRespPkt(&pkt.Data, dnsResp); err == nil {
						vl.agent.incrStats("dnsPktReply")
						pktOut := openflow13.NewPacketOut()
						pktOut.Data = respPkt
						pktOut.AddAction(openflow13.NewActionOutput(inPort))
						vl.ofSwitch.Send(pktOut)
						return
					}
				}

				// re-inject DNS packet
				ethPkt := buildDnsForwardPkt(&pkt.Data)
				pktOut := openflow13.NewPacketOut()
				pktOut.Data = ethPkt
				pktOut.InPort = inPort

				pktOut.AddAction(openflow13.NewActionOutput(openflow13.P_TABLE))
				vl.agent.incrStats("dnsPktForward")
				vl.ofSwitch.Send(pktOut)
				return
			}
		}
	}
}

func (vl *VlanBridge) backGroundGARPs() {
	for {
		vl.garpMutex.Lock()

		workDone := false
		for epgID, epgInfo := range vl.epgToEPs {
			if epgInfo.garpCount <= 0 {
				continue
			}

			epgInfo.garpCount--
			for _, ep := range epgInfo.epMap {
				err := vl.sendGARP(ep.ip, ep.mac, ep.vlan)
				if err == nil {
					vl.agent.incrStats("GARPSent")
				} else {
					log.Warnf("Send GARP failed for ep IP: %v", ep.ip)
				}
				workDone = true
			}

			vl.epgToEPs[epgID] = epgInfo
		}

		if !workDone { // No epgs pending. Time to exit
			vl.garpBGActive = false
			vl.garpMutex.Unlock()
			return
		}

		vl.garpMutex.Unlock()
		time.Sleep(GARPDELAY * time.Second)
	}
}

// InjectGARPs for all endpoints on the epg
func (vl *VlanBridge) InjectGARPs(epgID int) {
	vl.garpMutex.Lock()
	defer vl.garpMutex.Unlock()

	epgInfo, found := vl.epgToEPs[epgID]
	if found { // only if this epg has endpoints here
		epgInfo.garpCount = GARPRepeats
		vl.epgToEPs[epgID] = epgInfo
		if !vl.garpBGActive {
			go vl.backGroundGARPs()
		}
	}
}

func (vl *VlanBridge) sendGARPAll() {
	vl.garpMutex.Lock()
	defer vl.garpMutex.Unlock()

	count := 0
	for epgID, epgInfo := range vl.epgToEPs {
		epgInfo.garpCount = GARPRepeats
		vl.epgToEPs[epgID] = epgInfo
		count++
	}

	if !vl.garpBGActive && count > 0 {
		log.Infof("GARPs will be sent for %d epgs", count)
		go vl.backGroundGARPs()
	}
}

// Update global config
func (vl *VlanBridge) GlobalConfigUpdate(cfg OfnetGlobalConfig) error {
	if vl.agent.arpMode == cfg.ArpMode {
		log.Warnf("no change in ARP mode %s", vl.agent.arpMode)
	} else {
		vl.agent.arpMode = cfg.ArpMode
		vl.updateArpRedirectFlow(cfg.ArpMode, cfg.ArpMode == ArpFlood)
	}
	return nil
}

// AddLocalEndpoint Add a local endpoint and install associated local route
func (vl *VlanBridge) AddLocalEndpoint(endpoint OfnetEndpoint) error {
	log.Infof("Adding local endpoint: %+v", endpoint)

	dNATTbl := vl.ofSwitch.GetTable(SRV_PROXY_DNAT_TBL_ID)

	// Install a flow entry for vlan mapping and point it to next table
	portVlanFlow, err := createPortVlanFlow(vl.agent, vl.vlanTable, dNATTbl, &endpoint)
	if err != nil {
		log.Errorf("Error creating portvlan entry. Err: %v", err)
		return err
	}

	// save the flow entry
	vl.portVlanFlowDb[endpoint.PortNo] = portVlanFlow

	// install DSCP flow entries if required
	if endpoint.Dscp != 0 {
		dscpV4Flow, dscpV6Flow, err := createDscpFlow(vl.agent, vl.vlanTable, dNATTbl, &endpoint)
		if err != nil {
			log.Errorf("Error installing DSCP flows. Err: %v", err)
			return err
		}

		// save it for tracking
		vl.dscpFlowDb[endpoint.PortNo] = []*ofctrl.Flow{dscpV4Flow, dscpV6Flow}
	}

	// Install dst group entry for the endpoint
	err = vl.policyAgent.AddEndpoint(&endpoint)
	if err != nil {
		log.Errorf("Error adding endpoint to policy agent{%+v}. Err: %v", endpoint, err)
		return err
	}

	// Send GARP
	mac, _ := net.ParseMAC(endpoint.MacAddrStr)
	err = vl.sendGARP(endpoint.IpAddr, mac, endpoint.Vlan)
	if err != nil {
		log.Warnf("Error in sending GARP packet for (%s,%s) in vlan %d. Err: %+v",
			endpoint.IpAddr.String(), endpoint.MacAddrStr, endpoint.Vlan, err)
	}

	// update epgDB
	if endpoint.EndpointGroupVlan != 0 {
		vl.garpMutex.Lock()

		gInfo := GARPInfo{mac: mac,
			ip:   endpoint.IpAddr,
			vlan: endpoint.EndpointGroupVlan}
		epgInfo, found := vl.epgToEPs[endpoint.EndpointGroup]
		if !found {
			epMap := make(map[uint32]GARPInfo)
			epgInfo = epgGARPInfo{garpCount: 0,
				epMap: epMap,
			}
		}

		epgInfo.epMap[endpoint.PortNo] = gInfo
		vl.epgToEPs[endpoint.EndpointGroup] = epgInfo

		vl.garpMutex.Unlock()
		vl.InjectGARPs(endpoint.EndpointGroup) // inject background arps as well
	}

	return nil
}

// RemoveLocalEndpoint Remove local endpoint
func (vl *VlanBridge) RemoveLocalEndpoint(endpoint OfnetEndpoint) error {
	// Remove the port vlan flow.
	portVlanFlow := vl.portVlanFlowDb[endpoint.PortNo]
	if portVlanFlow != nil {
		err := portVlanFlow.Delete()
		if err != nil {
			log.Errorf("Error deleting portvlan flow. Err: %v", err)
		}
	}

	// Remove dscp flows.
	dscpFlows, found := vl.dscpFlowDb[endpoint.PortNo]
	if found {
		for _, dflow := range dscpFlows {
			err := dflow.Delete()
			if err != nil {
				log.Errorf("Error deleting dscp flow {%+v}. Err: %v", dflow, err)
			}
		}
	}

	// Remove from epg DB
	vl.garpMutex.Lock()
	defer vl.garpMutex.Unlock()
	epgInfo, found := vl.epgToEPs[endpoint.EndpointGroup]
	if found {
		delete(epgInfo.epMap, endpoint.PortNo)
	}

	vl.svcProxy.DelEndpoint(&endpoint)
	// Remove the endpoint from policy tables
	err := vl.policyAgent.DelEndpoint(&endpoint)
	if err != nil {
		log.Errorf("Error deleting endpoint to policy agent{%+v}. Err: %v", endpoint, err)
		return err
	}

	return nil
}

// UpdateLocalEndpoint update local endpoint state
func (vl *VlanBridge) UpdateLocalEndpoint(endpoint *OfnetEndpoint, epInfo EndpointInfo) error {
	oldDscp := endpoint.Dscp
	// Remove existing DSCP flows if required
	if epInfo.Dscp == 0 || epInfo.Dscp != endpoint.Dscp {
		// remove old DSCP flows
		dscpFlows, found := vl.dscpFlowDb[endpoint.PortNo]
		if found {
			for _, dflow := range dscpFlows {
				err := dflow.Delete()
				if err != nil {
					log.Errorf("Error deleting dscp flow {%+v}. Err: %v", dflow, err)
					return err
				}
			}
		}
	}

	// change DSCP value
	endpoint.Dscp = epInfo.Dscp

	// Add new DSCP flows if required
	if epInfo.Dscp != 0 && epInfo.Dscp != oldDscp {
		dNATTbl := vl.ofSwitch.GetTable(SRV_PROXY_DNAT_TBL_ID)

		// add new dscp flows
		dscpV4Flow, dscpV6Flow, err := createDscpFlow(vl.agent, vl.vlanTable, dNATTbl, endpoint)
		if err != nil {
			log.Errorf("Error installing DSCP flows. Err: %v", err)
			return err
		}

		// save it for tracking
		vl.dscpFlowDb[endpoint.PortNo] = []*ofctrl.Flow{dscpV4Flow, dscpV6Flow}
	}

	return nil
}

// AddVtepPort Add virtual tunnel end point.
func (vl *VlanBridge) AddVtepPort(portNo uint32, remoteIP net.IP) error {
	return nil
}

// RemoveVtepPort Remove a VTEP port
func (vl *VlanBridge) RemoveVtepPort(portNo uint32, remoteIP net.IP) error {
	return nil
}

// AddVlan Add a vlan.
func (vl *VlanBridge) AddVlan(vlanID uint16, vni uint32, vrf string) error {
	vl.agent.vlanVrfMutex.Lock()
	vl.agent.vlanVrf[vlanID] = &vrf
	vl.agent.vlanVrfMutex.Unlock()
	vl.agent.createVrf(vrf)
	return nil
}

// RemoveVlan Remove a vlan
func (vl *VlanBridge) RemoveVlan(vlanID uint16, vni uint32, vrf string) error {
	vl.agent.vlanVrfMutex.Lock()
	delete(vl.agent.vlanVrf, vlanID)
	vl.agent.vlanVrfMutex.Unlock()
	vl.agent.deleteVrf(vrf)
	return nil
}

// AddEndpoint Add an endpoint to the datapath
func (vl *VlanBridge) AddEndpoint(endpoint *OfnetEndpoint) error {

	if endpoint.Vni != 0 {
		return nil
	}

	log.Infof("Received endpoint: %+v", endpoint)

	// Install dst group entry for the endpoint
	err := vl.policyAgent.AddEndpoint(endpoint)
	if err != nil {
		log.Errorf("Error adding endpoint to policy agent{%+v}. Err: %v", endpoint, err)
		return err
	}

	return nil
}

// RemoveEndpoint removes an endpoint from the datapath
func (vl *VlanBridge) RemoveEndpoint(endpoint *OfnetEndpoint) error {
	log.Infof("Received DELETE endpoint: %+v", endpoint)

	if endpoint.Vni != 0 {
		return nil
	}

	// Remove the endpoint from policy tables
	err := vl.policyAgent.DelEndpoint(endpoint)
	if err != nil {
		log.Errorf("Error deleting endpoint to policy agent{%+v}. Err: %v", endpoint, err)
		return err
	}

	return nil
}

// handleLinkUpDown triggers GARPs on all eps when a link flap is detected.
func (vl *VlanBridge) handleLinkUpDown() {

	for {
		select {
		case update := <-vl.updChan:
			if vl.linkDb.Count() == 0 {
				// We do not have any links that we are interested in
				// Skip further processing
				break
			}
			link := vl.GetLink(update.Link.Attrs().Name)
			if link == nil {
				// We are not interested in this interface
				// Skip further processing
				break
			}
			if update.IfInfomsg.Flags&syscall.IFF_UP != 0 {
				log.Infof("Link up received for %s", link.Name)
				vl.agent.incrStats("LinkupRcvd")
				port := link.Port

				prevPortStatus := port.LinkStatus
				link.setLinkStatus(linkUp)

				// If the uplink's link status has changed, send GARPs
				if prevPortStatus != port.LinkStatus {
					vl.sendGARPAll()
				}
			} else if update.IfInfomsg.Flags&^syscall.IFF_UP != 0 {
				log.Infof("Link down received for %s", link.Name)
				vl.agent.incrStats("LinkDownRcvd")
				link.setLinkStatus(linkDown)
			}

		case <-vl.nlChan:
			log.Debugf("Stop listening for netlink events")
			return

		}
	}
}

// startMonitoringLinks starts monitoring for Link Updates
func (vl *VlanBridge) startMonitoringLinks() {
	vl.updChan = make(chan netlink.LinkUpdate)
	vl.nlChan = make(chan struct{})
	if err := netlink.LinkSubscribe(vl.updChan, vl.nlChan); err != nil {
		log.Errorf("Error listening on netlink: %v", err)
		return
	}

	// Handle port up/down events
	go vl.handleLinkUpDown()
}

// AddUplink adds an uplink to the switch
func (vl *VlanBridge) AddUplink(uplinkPort *PortInfo) error {
	log.Infof("Adding uplink port: %+v", uplinkPort)

	for _, link := range uplinkPort.MbrLinks {
		dnsUplinkFlow, err := vl.inputTable.NewFlow(ofctrl.FlowMatch{
			Priority:   DNS_FLOW_MATCH_PRIORITY + 2,
			InputPort:  link.OfPort,
			Ethertype:  protocol.IPv4_MSG,
			IpProto:    protocol.Type_UDP,
			UdpDstPort: 53,
		})
		if err != nil {
			log.Errorf("Error creating nameserver flow entry. Err: %v", err)
			return err
		}
		dnsUplinkFlow.Next(vl.vlanTable)

		// Install a flow entry for vlan mapping and point it to Mac table
		portVlanFlow, err := vl.vlanTable.NewFlow(ofctrl.FlowMatch{
			Priority:  FLOW_MATCH_PRIORITY,
			InputPort: link.OfPort,
		})
		if err != nil {
			log.Errorf("Error creating portvlan entry. Err: %v", err)
			return err
		}

		// Packets coming from uplink go thru normal lookup(bypass policy)
		sNATTbl := vl.ofSwitch.GetTable(SRV_PROXY_SNAT_TBL_ID)
		err = portVlanFlow.Next(sNATTbl)
		if err != nil {
			log.Errorf("Error installing portvlan entry. Err: %v", err)
			return err
		}

		// save the flow entry
		vl.portVlanFlowDb[link.OfPort] = portVlanFlow
		vl.portDnsFlowDb.Set(fmt.Sprintf("%d", link.OfPort), dnsUplinkFlow)
	}

	err := uplinkPort.checkLinkStatus()
	if err != nil {
		log.Errorf("Error checking link status. Err: %+v", err)
		return err
	}

	if vl.uplinkPortDb.Count() == 0 {
		vl.startMonitoringLinks()
	}

	vl.uplinkPortDb.Set(uplinkPort.Name, uplinkPort)
	for _, link := range uplinkPort.MbrLinks {
		vl.linkDb.Set(link.Name, link)
	}

	return nil
}

// UpdateUplink updates uplink info
func (vl *VlanBridge) UpdateUplink(uplinkName string, updates PortUpdates) error {
	for uplinkObj := range vl.uplinkPortDb.IterBuffered() {
		uplink := uplinkObj.Val.(*PortInfo)
		if uplink.Name != uplinkName {
			continue
		}
		for _, update := range updates.Updates {
			switch update.UpdateType {
			case LacpUpdate:
				lacpUpd := update.UpdateInfo.(LinkUpdateInfo)
				linkName := lacpUpd.LinkName
				for _, link := range uplink.MbrLinks {
					if link.Name == linkName {
						link.handleLacpUpdate(lacpUpd.LacpStatus)
					}
				}
			default:
				log.Errorf("Unknown update: (%s, %+v)", update.UpdateType, update.UpdateInfo)
			}
		}
	}

	return nil
}

// RemoveUplink remove an uplink to the switch
func (vl *VlanBridge) RemoveUplink(uplinkName string) error {
	uplinkPort := vl.GetUplink(uplinkName)

	if uplinkPort == nil {
		err := fmt.Errorf("Could not get uplink with name: %s", uplinkName)
		return err
	}

	// Stop monitoring links in the port
	for _, link := range uplinkPort.MbrLinks {
		// Uninstall the flow entry
		portVlanFlow := vl.portVlanFlowDb[link.OfPort]
		if portVlanFlow != nil {
			portVlanFlow.Delete()
			delete(vl.portVlanFlowDb, link.OfPort)
		}

		// Remove from linkDb
		vl.linkDb.Remove(link.Name)
		if f, ok := vl.portDnsFlowDb.Get(fmt.Sprintf("%d", link.OfPort)); ok {
			if dnsUplinkFlow, ok := f.(*ofctrl.Flow); ok {
				if err := dnsUplinkFlow.Delete(); err != nil {
					log.Errorf("Error deleting nameserver flow. Err: %v", err)
				}
			}
		}
		vl.portDnsFlowDb.Remove(fmt.Sprintf("%d", link.OfPort))
	}
	vl.uplinkPortDb.Remove(uplinkName)

	// Stop receving link updates when there are no more ports to monitor
	if vl.uplinkPortDb.Count() == 0 {
		close(vl.nlChan)
	}
	return nil
}

// AddHostPort is not implemented
func (vl *VlanBridge) AddHostPort(hp HostPortInfo) error {
	return nil
}

// RemoveHostPort is not implemented
func (vl *VlanBridge) RemoveHostPort(hp uint32) error {
	return nil
}

// AddSvcSpec adds a service spec to proxy
func (vl *VlanBridge) AddSvcSpec(svcName string, spec *ServiceSpec) error {
	return vl.svcProxy.AddSvcSpec(svcName, spec)
}

// DelSvcSpec removes a service spec from proxy
func (vl *VlanBridge) DelSvcSpec(svcName string, spec *ServiceSpec) error {
	return vl.svcProxy.DelSvcSpec(svcName, spec)
}

// SvcProviderUpdate Service Proxy Back End update
func (vl *VlanBridge) SvcProviderUpdate(svcName string, providers []string) {
	vl.svcProxy.ProviderUpdate(svcName, providers)
}

// GetEndpointStats fetches ep stats
func (vl *VlanBridge) GetEndpointStats() (map[string]*OfnetEndpointStats, error) {
	return vl.svcProxy.GetEndpointStats()
}

// MultipartReply handles stats reply
func (vl *VlanBridge) MultipartReply(sw *ofctrl.OFSwitch, reply *openflow13.MultipartReply) {
	if reply.Type == openflow13.MultipartType_Flow {
		vl.svcProxy.FlowStats(reply)
	}
}

// InspectState returns current state
func (vl *VlanBridge) InspectState() (interface{}, error) {
	vlExport := struct {
		PolicyAgent *PolicyAgent // Policy agent
		SvcProxy    interface{}  // Service proxy
		// VlanDb      map[uint16]*Vlan // Database of known vlans
	}{
		vl.policyAgent,
		vl.svcProxy.InspectState(),
		// vr.vlanDb,
	}
	return vlExport, nil
}

// initialize Fgraph on the switch
func (vl *VlanBridge) initFgraph() error {
	sw := vl.ofSwitch

	log.Infof("Installing initial flow entries")

	// Create all tables
	vl.inputTable = sw.DefaultTable()
	vl.vlanTable, _ = sw.NewTable(VLAN_TBL_ID)
	vl.nmlTable, _ = sw.NewTable(MAC_DEST_TBL_ID)

	// setup SNAT table
	// Matches in SNAT table (i.e. incoming) go to mac dest
	vl.svcProxy.InitSNATTable(MAC_DEST_TBL_ID)

	// Init policy tables
	err := vl.policyAgent.InitTables(SRV_PROXY_SNAT_TBL_ID)
	if err != nil {
		log.Fatalf("Error installing policy table. Err: %v", err)
		return err
	}

	// Matches in DNAT go to Policy
	vl.svcProxy.InitDNATTable(DST_GRP_TBL_ID)

	// Send all packets to vlan lookup
	validPktFlow, _ := vl.inputTable.NewFlow(ofctrl.FlowMatch{
		Priority: FLOW_MISS_PRIORITY,
	})
	validPktFlow.Next(vl.vlanTable)

	// If we miss Vlan lookup, continue to next lookup
	vlanMissFlow, _ := vl.vlanTable.NewFlow(ofctrl.FlowMatch{
		Priority: FLOW_MISS_PRIORITY,
	})
	dstGrpTbl := vl.ofSwitch.GetTable(DST_GRP_TBL_ID)
	vlanMissFlow.Next(dstGrpTbl)

	// if arp-mode is ArpProxy, redirect ARP packets to controller
	// In ArpFlood mode, ARP packets are flooded in datapath and
	// there is no proxy-arp functionality
	if vl.agent.arpMode == ArpProxy {
		vl.updateArpRedirectFlow(vl.agent.arpMode, false)
	}

	// redirect dns requests from containers (oui 02:02:xx) to controller
	macSaMask := net.HardwareAddr{0xFF, 0xFF, 0x00, 0x00, 0x00, 0x00}
	macSa := net.HardwareAddr{0x02, 0x02, 0x00, 0x00, 0x00, 0x00}
	dnsRedirectFlow, _ := vl.inputTable.NewFlow(ofctrl.FlowMatch{
		Priority:   DNS_FLOW_MATCH_PRIORITY,
		MacSa:      &macSa,
		MacSaMask:  &macSaMask,
		Ethertype:  protocol.IPv4_MSG,
		IpProto:    protocol.Type_UDP,
		UdpDstPort: 53,
	})
	dnsRedirectFlow.Next(sw.SendToController())

	// re-inject dns requests
	dnsReinjectFlow, _ := vl.inputTable.NewFlow(ofctrl.FlowMatch{
		Priority:   DNS_FLOW_MATCH_PRIORITY + 1,
		MacSa:      &macSa,
		MacSaMask:  &macSaMask,
		VlanId:     nameServerInternalVlanId,
		Ethertype:  protocol.IPv4_MSG,
		IpProto:    protocol.Type_UDP,
		UdpDstPort: 53,
	})
	dnsReinjectFlow.PopVlan()
	dnsReinjectFlow.Next(vl.vlanTable)

	// All packets that have gone thru policy lookup go thru normal OVS switching
	normalLookupFlow, _ := vl.nmlTable.NewFlow(ofctrl.FlowMatch{
		Priority: FLOW_MISS_PRIORITY,
	})
	normalLookupFlow.Next(sw.NormalLookup())

	// Drop all
	return nil
}

func getProxyARPResp(arpIn *protocol.ARP, tgtMac string, vid uint16, inPort uint32) *openflow13.PacketOut {
	arpPkt, _ := protocol.NewARP(protocol.Type_Reply)
	arpPkt.HWSrc, _ = net.ParseMAC(tgtMac)
	arpPkt.IPSrc = arpIn.IPDst
	arpPkt.HWDst = arpIn.HWSrc
	arpPkt.IPDst = arpIn.IPSrc
	log.Debugf("Sending Proxy ARP response: %+v", arpPkt)

	// Build the ethernet packet
	ethPkt := protocol.NewEthernet()
	ethPkt.VLANID.VID = vid
	ethPkt.HWDst = arpPkt.HWDst
	ethPkt.HWSrc = arpPkt.HWSrc
	ethPkt.Ethertype = 0x0806
	ethPkt.Data = arpPkt
	log.Debugf("Sending Proxy ARP response Ethernet: %+v", ethPkt)

	// Construct Packet out
	pktOut := openflow13.NewPacketOut()
	pktOut.Data = ethPkt
	pktOut.AddAction(openflow13.NewActionOutput(inPort))

	return pktOut
}

// add a flow to redirect ARP packet to controller for arp-proxy
func (vl *VlanBridge) updateArpRedirectFlow(newArpMode ArpModeT, sendGARPs bool) {
	sw := vl.ofSwitch

	add := (newArpMode == ArpProxy)
	if add {
		// Redirect ARP Request packets to controller
		arpFlow, _ := vl.inputTable.NewFlow(ofctrl.FlowMatch{
			Priority:  FLOW_MATCH_PRIORITY,
			Ethertype: 0x0806,
			ArpOper:   protocol.Type_Request,
		})
		arpFlow.Next(sw.SendToController())
		vl.arpRedirectFlow = arpFlow
	} else {
		if vl.arpRedirectFlow != nil {
			vl.arpRedirectFlow.Delete()
		}
	}

	if sendGARPs {
		// When arp mode changes to ArpFlood, send GARP for all endpoints
		// so that external network can learn endpoints from ARP packets
		vl.sendGARPAll()
	}
}

/*
 * Process incoming ARP packets
 * ARP request handling in various scenarios:
 * Src and Dest EP known:
 *      - Proxy ARP if Dest EP is present locally on the host
 * Src EP known, Dest EP not known:
 *      - ARP Request to a router/VM scenario. Reinject ARP request to uplinks
 * Src EP not known, Dest EP known:
 *      - Proxy ARP if Dest EP is present locally on the host
 * Src and Dest EP not known:
 *      - Ignore processing the request
 */
func (vl *VlanBridge) processArp(pkt protocol.Ethernet, inPort uint32) {
	switch t := pkt.Data.(type) {
	case *protocol.ARP:
		log.Debugf("Processing ARP packet on port %d: %+v", inPort, *t)
		var arpIn protocol.ARP = *t

		vl.agent.incrStats("ArpPktRcvd")

		switch arpIn.Operation {
		case protocol.Type_Request:
			// If it's a GARP packet, ignore processing
			if arpIn.IPSrc.String() == arpIn.IPDst.String() {
				log.Debugf("Ignoring GARP packet")
				return
			}

			vl.agent.incrStats("ArpReqRcvd")

			// Lookup the Source and Dest IP in the endpoint table
			//Vrf derivation logic :
			var vlan uint16

			fromUplink := false
			for uplinkObj := range vl.uplinkPortDb.IterBuffered() {
				uplink := uplinkObj.Val.(*PortInfo)
				for _, link := range uplink.MbrLinks {
					if link.OfPort == inPort {
						fromUplink = true
						break
					}
				}
				if fromUplink {
					break
				}
			}

			if fromUplink {
				//arp packet came in from uplink hence tagged
				vlan = pkt.VLANID.VID
			} else {
				//arp packet came from local endpoints - derive vrf from inport
				if pVl := vl.agent.getPortVlanMap(inPort); pVl != nil {
					vlan = *(pVl)
				} else {
					log.Debugf("Invalid port vlan mapping. Ignoring arp packet")
					vl.agent.incrStats("ArpReqInvalidPortVlan")
					return
				}
			}
			srcEp := vl.agent.getEndpointByIpVlan(arpIn.IPSrc, vlan)
			dstEp := vl.agent.getEndpointByIpVlan(arpIn.IPDst, vlan)

			// No information about the src or dest EP. Drop the pkt.
			if srcEp == nil && dstEp == nil {
				log.Debugf("No information on source/destination. Ignoring ARP request.")
				vl.agent.incrStats("ArpRequestUnknownSrcDst")
				return
			}

			// if it came from uplink and the destination is not local, drop it
			if fromUplink {
				if dstEp == nil {
					vl.agent.incrStats("ArpReqUnknownDestFromUplink")
					return
				}

				if dstEp.OriginatorIp.String() != vl.agent.localIp.String() {
					vl.agent.incrStats("ArpReqNonLocalDestFromUplink")
					return
				}
			}

			// If we know the dstEp to be present locally, send the Proxy ARP response
			if dstEp != nil {
				// Container to Container communication. Send proxy ARP response.
				// Unknown node to Container communication
				//   -> Send proxy ARP response only if Endpoint is local.
				//   -> This is to avoid sending ARP responses from ofnet agent on multiple hosts
				if srcEp != nil ||
					(srcEp == nil && dstEp.OriginatorIp.String() == vl.agent.localIp.String()) {
					// Send the packet out
					pktOut := getProxyARPResp(&arpIn, dstEp.MacAddrStr,
						pkt.VLANID.VID, inPort)
					vl.ofSwitch.Send(pktOut)

					vl.agent.incrStats("ArpReqRespSent")

					return
				}
			}

			proxyMac := vl.svcProxy.GetSvcProxyMAC(arpIn.IPDst)
			if proxyMac != "" {
				pktOut := getProxyARPResp(&arpIn, proxyMac,
					pkt.VLANID.VID, inPort)
				vl.ofSwitch.Send(pktOut)
				vl.agent.incrStats("ArpReqRespSent")
				return
			}

			if srcEp != nil && dstEp == nil {
				// ARP request from local container to unknown IP
				// Reinject ARP to uplinks
				ethPkt := protocol.NewEthernet()
				ethPkt.VLANID.VID = srcEp.EndpointGroupVlan
				ethPkt.HWDst = pkt.HWDst
				ethPkt.HWSrc = pkt.HWSrc
				ethPkt.Ethertype = 0x0806
				ethPkt.Data = &arpIn

				log.Infof("Received ARP request for unknown IP: %v. "+
					"Reinjecting ARP request Ethernet to uplinks: %+v", arpIn.IPDst, ethPkt)

				// Packet out
				pktOut := openflow13.NewPacketOut()
				pktOut.InPort = inPort
				pktOut.Data = ethPkt
				for uplinkObj := range vl.uplinkPortDb.IterBuffered() {
					uplink := uplinkObj.Val.(*PortInfo)
					uplinkMemberLink := uplink.getActiveLink(pkt.HWSrc.String())
					if uplinkMemberLink == nil {
						log.Infof("No active interface on uplink. Not reinjecting ARP request pkt(%+v)", ethPkt)
						return
					}

					log.Infof("Sending to uplink: %+v on interface: %d", uplink, uplinkMemberLink.OfPort)
					pktOut.AddAction(openflow13.NewActionOutput(uplinkMemberLink.OfPort))
				}

				// Send the packet out
				vl.ofSwitch.Send(pktOut)

				vl.agent.incrStats("ArpReqReinject")
			}

		case protocol.Type_Reply:
			log.Debugf("Received ARP response packet: %+v from port %d", arpIn, inPort)
			vl.agent.incrStats("ArpRespRcvd")

			ethPkt := protocol.NewEthernet()
			ethPkt.VLANID = pkt.VLANID
			ethPkt.HWDst = pkt.HWDst
			ethPkt.HWSrc = pkt.HWSrc
			ethPkt.Ethertype = 0x0806
			ethPkt.Data = &arpIn
			log.Debugf("Sending ARP response Ethernet: %+v", ethPkt)

			// Packet out
			pktOut := openflow13.NewPacketOut()
			pktOut.InPort = inPort
			pktOut.Data = ethPkt
			pktOut.AddAction(openflow13.NewActionOutput(openflow13.P_NORMAL))

			log.Debugf("Reinjecting ARP reply packet: %+v", pktOut)
			// Send it out
			vl.ofSwitch.Send(pktOut)
		}
	}
}

// sendGARP sends GARP for the specified IP, MAC
func (vl *VlanBridge) sendGARP(ip net.IP, mac net.HardwareAddr, vlanID uint16) error {
	pktOut := BuildGarpPkt(ip, mac, vlanID)

	for uplinkObj := range vl.uplinkPortDb.IterBuffered() {
		uplink := uplinkObj.Val.(*PortInfo)
		uplinkMemberLink := uplink.getActiveLink(mac.String())
		if uplinkMemberLink == nil {
			log.Debugf("No active interface on uplink. Not sending GARP for ip:%v mac:%v vlan:%d", ip.String(), mac.String(), vlanID)
			break
		}

		log.Debugf("Sending GARP on uplink: %+v on interface %d, ip:%v vlan: %d", uplink, uplinkMemberLink.OfPort, ip.String(), vlanID)
		pktOut.AddAction(openflow13.NewActionOutput(uplinkMemberLink.OfPort))

		// NOTE: Sending it on only one uplink to avoid loops
		// Once MAC pinning mode is supported, this logic has to change
		break
	}

	// Send it out
	if vl.ofSwitch != nil {
		vl.ofSwitch.Send(pktOut)
		vl.agent.incrStats("GarpPktSent")
	}

	return nil
}

func (vl *VlanBridge) listLinks() []string {
	var links []string
	for linkObj := range vl.linkDb.Iter() {
		link := linkObj.Val.(*LinkInfo)
		links = append(links, link.Name)
	}

	return links
}

//FlushEndpoints flushes endpoints from ovs
func (vl *VlanBridge) FlushEndpoints(endpointType int) {
}
