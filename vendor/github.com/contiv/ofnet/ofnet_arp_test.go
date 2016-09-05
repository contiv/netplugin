package ofnet

import (
	"fmt"
	"net"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/ofnet/ofctrl"
	"github.com/shaleman/libOpenflow/openflow13"
	"github.com/shaleman/libOpenflow/protocol"
	"github.com/vishvananda/netlink"
)

// Verify if the flow entries are installed on vlan bridge
func TestVlanArpRedirectFlowEntry(t *testing.T) {
	for i := 0; i < NUM_AGENT; i++ {
		brName := "vlanBridge" + fmt.Sprintf("%d", i)

		flowList, err := ofctlFlowDump(brName)
		if err != nil {
			t.Errorf("Error getting flow entries. Err: %v", err)
		}

		// Check if ARP Request redirect entry is installed
		arpFlowMatch := fmt.Sprintf("priority=100,arp,arp_op=1 actions=CONTROLLER")
		if !ofctlFlowMatch(flowList, 0, arpFlowMatch) {
			t.Errorf("Could not find the route %s on ovs %s", arpFlowMatch, brName)
			return
		}
		log.Infof("Found arp redirect flow %s on ovs %s", arpFlowMatch, brName)
	}
}

// Verify if the flow entries are installed on vlan bridge
func TestVxlanArpRedirectFlowEntry(t *testing.T) {
	for i := 0; i < NUM_AGENT; i++ {
		brName := "vxlanBridge" + fmt.Sprintf("%d", i)

		flowList, err := ofctlFlowDump(brName)
		if err != nil {
			t.Errorf("Error getting flow entries. Err: %v", err)
		}

		// Check if ARP Request redirect entry is installed
		arpFlowMatch := fmt.Sprintf("priority=100,arp,arp_op=1 actions=CONTROLLER")
		if !ofctlFlowMatch(flowList, 0, arpFlowMatch) {
			t.Errorf("Could not find the route %s on ovs %s", arpFlowMatch, brName)
			return
		}
		log.Infof("Found arp redirect flow %s on ovs %s", arpFlowMatch, brName)
	}
}

func vlanAddEP(epID, epgID int, add bool) error {
	macAddr, _ := net.ParseMAC(fmt.Sprintf("02:02:02:%02x:%02x:%02x", epID, epID, epID))
	ipAddr := net.ParseIP(fmt.Sprintf("10.11.%d.%d", epID, epID))
	endpoint := EndpointInfo{
		PortNo:            uint32(NUM_AGENT + epID),
		MacAddr:           macAddr,
		Vlan:              uint16(epgID),
		EndpointGroup:     epgID,
		EndpointGroupVlan: uint16(epgID),
		IpAddr:            ipAddr,
	}

	if add {
		return vlanAgents[0].AddLocalEndpoint(endpoint)
	}
	return vlanAgents[0].RemoveLocalEndpoint(uint32(NUM_AGENT + epID))
}

// TestOfnetVlanGARPInject verifies GARP injection
func TestOfnetVlanGArpInject(t *testing.T) {
	var resp bool

	err1 := vlanAgents[0].AddNetwork(uint16(5), uint32(5), "", "test1")
	err2 := vlanAgents[0].AddNetwork(uint16(6), uint32(6), "", "test1")

	if err1 != nil || err2 != nil {
		t.Errorf("Error adding vlan %v, %v", err1, err2)
		return
	}

	// Add one endpoint
	err := vlanAddEP(5, 5, true)
	if err != nil {
		t.Errorf("Error adding EP")
		return
	}

	time.Sleep(5 * time.Second)

	// Look for stats update
	count := vlanAgents[0].getStats("GARPSent")
	if count == 0 {
		t.Errorf("GARP stats wasn't updated ok: count: %v", count)
		return
	}

	// Add two endpoints to another epg
	vlanAddEP(6, 6, true)
	log.Infof("Testing GARP injection.. this might take a while")
	time.Sleep(GARP_EXPIRY_DELAY * time.Second)
	vlanAddEP(7, 6, true)
	time.Sleep(GARP_EXPIRY_DELAY * time.Second)
	count = vlanAgents[0].getStats("GARPSent")
	if count != 4*GARPRepeats {
		t.Errorf("GARP stats incorrect count: %v exp: %v",
			count, 4*GARPRepeats)
		return
	}

	// delete one of the eps
	vlanAddEP(6, 6, false)
	vlanAgents[0].InjectGARPs(6, &resp)
	time.Sleep(GARP_EXPIRY_DELAY * time.Second)
	count = vlanAgents[0].getStats("GARPSent")
	if count != 5*GARPRepeats {
		t.Errorf("GARP stats incorrect count: %v exp: %v",
			count, 5*GARPRepeats)
		return
	}

	// Test link status triggered GARP
	link, err := addUplink(vlanAgents[0], "vvport200", 88)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(GARP_EXPIRY_DELAY * time.Second)
	count = vlanAgents[0].getStats("GARPSent")
	if count != 7*GARPRepeats {
		netlink.LinkDel(link)
		t.Errorf("GARP stats incorrect count: %v exp: %v",
			count, 7*GARPRepeats)
	}

	err = delUplink(vlanAgents[0], 88, link)
	if err != nil {
		t.Fatalf("Error deleting uplink. Err: %v", err)
	}

	vlanAddEP(7, 6, false)
	vlanAddEP(5, 5, false)

}

// addEndpoint adds an endpoint
func addEndpoint(ofa *OfnetAgent, portNo uint32, vlan uint16, macAddrStr, ipAddrStr string) error {
	macAddr, _ := net.ParseMAC(macAddrStr)
	endpoint := EndpointInfo{
		PortNo:            portNo,
		MacAddr:           macAddr,
		Vlan:              vlan,
		EndpointGroup:     0,
		EndpointGroupVlan: vlan,
		IpAddr:            net.ParseIP(ipAddrStr),
	}

	return ofa.AddLocalEndpoint(endpoint)
}

// addUplink adds a dummy uplink to ofnet agent
func addUplink(ofa *OfnetAgent, linkName string, ofpPortNo uint32) (*netlink.Veth, error) {
	link := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name:   linkName,
			TxQLen: 100,
			MTU:    1400,
		},
		PeerName: linkName + "peer",
	}
	// delete old link if it exists.. and ignore error
	netlink.LinkDel(link)
	time.Sleep(100 * time.Millisecond)

	if err := netlink.LinkAdd(link); err != nil {
		return nil, err
	}

	// add it to ofnet
	err := ofa.AddUplink(ofpPortNo, linkName)
	if err != nil {
		return nil, err
	}
	time.Sleep(time.Second)

	// mark the link as up
	if err := netlink.LinkSetUp(link); err != nil {
		return nil, err
	}

	return link, nil
}

// delUplink deletes an uplink from ofnet agent
func delUplink(ofa *OfnetAgent, ofpPortNo uint32, link *netlink.Veth) error {
	err := ofa.RemoveUplink(ofpPortNo)
	if err != nil {
		return fmt.Errorf("Error deleting uplink. Err: %v", err)
	}

	// cleanup the uplink
	if err := netlink.LinkDel(link); err != nil {
		return fmt.Errorf("Error deleting link: %v", err)
	}

	return nil
}

// injectArpReq injects an ARP request into ofnet
func injectArpReq(ofa *OfnetAgent, inPort, vlan int, macSrc, macDst, ipSrc, ipDst string) error {
	if macDst == "" {
		macDst = "ff:ff:ff:ff:ff:ff"
	}

	// inject an ARP request from ep1 for ep2
	arpReq := openflow13.NewPacketIn()
	arpReq.Match.Type = openflow13.MatchType_OXM
	arpReq.Match.AddField(*openflow13.NewInPortField(uint32(inPort)))
	arpReq.Data = *protocol.NewEthernet()
	arpReq.Data.Ethertype = protocol.ARP_MSG
	arpReq.Data.HWDst, _ = net.ParseMAC(macDst)
	arpReq.Data.HWSrc, _ = net.ParseMAC(macSrc)
	if vlan != 0 {
		arpReq.Data.VLANID.VID = uint16(vlan)
	}
	arpPkt, _ := protocol.NewARP(protocol.Type_Request)
	arpPkt.HWSrc, _ = net.ParseMAC(macSrc)
	arpPkt.IPSrc = net.ParseIP(ipSrc)
	arpPkt.HWDst, _ = net.ParseMAC("00:00:00:00:00:00")
	arpPkt.IPDst = net.ParseIP(ipDst)

	arpReq.Data.Data = arpPkt
	pkt := ofctrl.PacketIn(*arpReq)
	ofa.PacketRcvd(ofa.ofSwitch, &pkt)

	log.Debugf("Injected ARP request: %+v\n Packet: %+v", arpPkt, arpReq)
	return nil
}

// checkArpReqHandling injects ARP requests and checks expected count is incremented
func checkArpReqHandling(ofa *OfnetAgent, inPort, vlan int, macSrc, macDst, ipSrc, ipDst, expStat string, t *testing.T) {
	// get previous count
	prevCount := ofa.getStats(expStat)
	log.Debugf("BeforeStats: %+v", ofa.stats)

	// inject the packet
	err := injectArpReq(ofa, inPort, vlan, macSrc, macDst, ipSrc, ipDst)
	if err != nil {
		t.Fatalf("Error injecting ARP req. Err: %v", err)
	}

	log.Debugf("AfterStats: %+v", ofa.stats)
	newCount := ofa.getStats(expStat)
	if newCount != (prevCount + 1) {
		log.Infof("checkArpReqHandling: AfterStats: %+v", ofa.stats)
		t.Fatalf("%s value %d did not match expected value %d", expStat, newCount, (prevCount + 1))
	}
}

// TestVlanProxyArp tests proxy ARP in vlan mode
func TestVlanProxyArp(t *testing.T) {
	err := vlanAgents[0].AddNetwork(uint16(1), uint32(1), "", "test1")
	if err != nil {
		t.Errorf("Error adding vlan %v", err)
		return
	}

	// Add two endpoints
	err = addEndpoint(vlanAgents[0], 1, 1, "02:02:0A:01:01:01", "10.1.1.1")
	if err != nil {
		t.Errorf("Error adding endpoint")
		return
	}
	err = addEndpoint(vlanAgents[0], 2, 1, "02:02:0A:01:01:02", "10.1.1.2")
	if err != nil {
		t.Errorf("Error adding endpoint")
		return
	}
	err = addEndpoint(vlanAgents[1], 3, 1, "02:02:0A:01:01:03", "10.1.1.3")
	if err != nil {
		t.Errorf("Error adding endpoint")
		return
	}

	// add an uplink
	link, err := addUplink(vlanAgents[0], "vvport200", 99)
	if err != nil {
		t.Fatalf("Error adding uplink. Err: %v", err)
	}

	// inject an ARP request from ep1 for ep2
	checkArpReqHandling(vlanAgents[0], 1, 0, "02:02:0A:01:01:01", "", "10.1.1.1", "10.1.1.2", "ArpReqRespSent", t)

	// inject a unicast ARP request from ep1 for ep2
	checkArpReqHandling(vlanAgents[0], 1, 0, "02:02:0A:01:01:01", "02:02:0A:01:01:02", "10.1.1.1", "10.1.1.2", "ArpReqRespSent", t)

	// inject ARP req from ep1 to unknown
	checkArpReqHandling(vlanAgents[0], 1, 0, "02:02:0A:01:01:01", "", "10.1.1.1", "10.1.1.254", "ArpReqReinject", t)

	// inject ARP req from uplink to local addr
	checkArpReqHandling(vlanAgents[0], 99, 1, "02:02:0A:01:01:FE", "", "10.1.1.254", "10.1.1.1", "ArpReqRespSent", t)

	// inject ARP req from uplink to unknown
	checkArpReqHandling(vlanAgents[0], 99, 1, "02:02:0A:01:01:FE", "", "10.1.1.254", "10.1.1.200", "ArpRequestUnknownSrcDst", t)

	// inject ARP req from uplink to unknown dest with known src
	checkArpReqHandling(vlanAgents[0], 99, 1, "02:02:0A:01:01:01", "", "10.1.1.1", "10.1.1.200", "ArpReqUnknownDestFromUplink", t)

	// inject ARP req from uplink to non-local dest
	checkArpReqHandling(vlanAgents[0], 99, 1, "02:02:0A:01:01:01", "", "10.1.1.1", "10.1.1.3", "ArpReqNonLocalDestFromUplink", t)

	// cleanup uplink
	err = delUplink(vlanAgents[0], 99, link)
	if err != nil {
		t.Fatalf("Error deleting uplink. Err: %v", err)
	}

	// cleanup endpoints
	err = vlanAgents[0].RemoveLocalEndpoint(1)
	if err != nil {
		t.Fatalf("Error deleting endpoint. Err: %v", err)
	}
	err = vlanAgents[0].RemoveLocalEndpoint(2)
	if err != nil {
		t.Fatalf("Error deleting endpoint. Err: %v", err)
	}
}

// TestVxlanProxyArp tests proxy ARP in vxlan mode
func TestVxlanProxyArp(t *testing.T) {
	err := vxlanAgents[0].AddNetwork(uint16(1), uint32(1), "", "test1")
	if err != nil {
		t.Errorf("Error adding vxlan %v", err)
		return
	}

	// Add two endpoints
	err = addEndpoint(vxlanAgents[0], 1, 1, "02:02:0A:01:01:01", "10.1.1.1")
	if err != nil {
		t.Errorf("Error adding endpoint")
		return
	}
	err = addEndpoint(vxlanAgents[0], 2, 1, "02:02:0A:01:01:02", "10.1.1.2")
	if err != nil {
		t.Errorf("Error adding endpoint")
		return
	}
	err = addEndpoint(vxlanAgents[1], 3, 1, "02:02:0A:01:01:03", "10.1.1.3")
	if err != nil {
		t.Errorf("Error adding endpoint")
		return
	}

	// add a vtep
	err = vxlanAgents[0].AddVtepPort(88, net.ParseIP("192.168.2.11"))
	if err != nil {
		t.Fatalf("Error adding VTEP. Err: %v", err)
	}

	// inject an ARP request from ep1 for ep2
	checkArpReqHandling(vxlanAgents[0], 1, 0, "02:02:0A:01:01:01", "", "10.1.1.1", "10.1.1.2", "ArpReqRespSent", t)

	// inject a unicast ARP request from ep1 for ep2
	checkArpReqHandling(vxlanAgents[0], 1, 0, "02:02:0A:01:01:01", "02:02:0A:01:01:02", "10.1.1.1", "10.1.1.2", "ArpReqRespSent", t)

	// inject ARP req from ep1 to unknown
	checkArpReqHandling(vxlanAgents[0], 1, 0, "02:02:0A:01:01:01", "", "10.1.1.1", "10.1.1.254", "ArpReqReinject", t)

	// inject ARP req from uplink to local addr
	checkArpReqHandling(vxlanAgents[0], 88, 1, "02:02:0A:01:01:FE", "", "10.1.1.254", "10.1.1.1", "ArpReqRespSent", t)

	// inject ARP req from uplink to unknown
	checkArpReqHandling(vxlanAgents[0], 88, 1, "02:02:0A:01:01:FE", "", "10.1.1.254", "10.1.1.200", "ArpRequestUnknownSrcDst", t)

	// inject ARP req from uplink to unknown dest with known src
	checkArpReqHandling(vxlanAgents[0], 88, 1, "02:02:0A:01:01:01", "", "10.1.1.1", "10.1.1.200", "ArpReqUnknownDestFromVtep", t)

	// inject ARP req from uplink to non-local dest
	checkArpReqHandling(vxlanAgents[0], 88, 1, "02:02:0A:01:01:01", "", "10.1.1.1", "10.1.1.3", "ArpReqNonLocalDestFromVtep", t)

	// cleanup vtep
	err = vxlanAgents[0].RemoveVtepPort(88, net.ParseIP("192.168.2.11"))
	if err != nil {
		t.Fatalf("Error deleting vtep. Err: %v", err)
	}

	// cleanup endpoints
	err = vxlanAgents[0].RemoveLocalEndpoint(1)
	if err != nil {
		t.Fatalf("Error deleting endpoint. Err: %v", err)
	}
	err = vxlanAgents[0].RemoveLocalEndpoint(2)
	if err != nil {
		t.Fatalf("Error deleting endpoint. Err: %v", err)
	}
}
