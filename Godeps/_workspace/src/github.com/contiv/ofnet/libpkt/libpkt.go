package libpkt

import (
	"bytes"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/contiv/ofnet/ovsdbDriver"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"net"
	"time"
)

var (
	snapshot_len int32 = 1024
	err          error
	timeout      time.Duration = 30 * time.Second
	handle       *pcap.Handle
	// Will reuse these for each packet
	ethLayer  layers.Ethernet
	ipLayer   layers.IPv4
	tcpLayer  layers.TCP
	vlanLayer layers.Dot1Q
	options   gopacket.SerializeOptions
)

type Packet struct {
	SrcMAC string // src mac
	DstMAC string // dst mac
	IPv4   *IPv4Layer
	IPv6   *IPv6Layer
	Arp    *ArpLayer
}

type IPv4Layer struct {
	SrcIP      string // Src ip address
	DstIP      string // dst ip address
	srcPort    int    // src tcp port
	dstPort    int    // dst tcp port
	TTL        uint8  // ip TTL
	IHL        uint8
	TOS        uint8
	Length     uint16
	Id         uint16
	Flags      uint8
	FragOffset uint16
	Protocol   layers.IPProtocol
	Checksum   uint16
	Options    []layers.IPv4Option
	Padding    []byte
}
type IPv6Layer struct {
	SrcIP        string // Src ip address
	DstIP        string // dst ip address
	srcPort      int    // src tcp port
	dstPort      int    // dst tcp port
	TrafficClass uint8
	FlowLabel    uint32
	Length       uint16
	Protocol     uint8
	HopLimit     uint8
	NextHeader   layers.IPProtocol
}

type ArpLayer struct {
	AddrType          layers.LinkType
	Protocol          layers.EthernetType
	HwAddressSize     uint8
	ProtAddressSize   uint8
	Operation         uint16
	SourceHwAddress   string
	SourceProtAddress string
	DstHwAddress      string
	DstProtAddress    string
}

func compareIP(A net.IP, B net.IP) bool {

	return bytes.Equal(A, B)
}
func buildIpv4(ipv4Layer *IPv4Layer) (ip *layers.IPv4) {

	ip = &layers.IPv4{
		Id:       1,
		TTL:      64,
		IHL:      20,
		SrcIP:    net.ParseIP(ipv4Layer.SrcIP),
		DstIP:    net.ParseIP(ipv4Layer.DstIP),
		Version:  4,
		Length:   0,
		Protocol: 255,
		Checksum: 0,
	}
	switch {
	case ipv4Layer.Id > 0:
		ip.Id = ipv4Layer.Id
	case ipv4Layer.TTL > 0:
		ip.TTL = ipv4Layer.TTL
	case ipv4Layer.IHL > 0:
		ip.IHL = ipv4Layer.IHL
	}
	return
}

func buildIpv6(ipv6Layer *IPv6Layer) (ip *layers.IPv6) {

	ip = &layers.IPv6{
		SrcIP:      net.ParseIP(ipv6Layer.SrcIP),
		DstIP:      net.ParseIP(ipv6Layer.DstIP),
		Version:    6,
		Length:     46,
		NextHeader: layers.IPProtocolNoNextHeader,
		HopLimit:   64,
	}
	return
}

func buildArp(arpLayer *ArpLayer) (arp *layers.ARP) {

	SourceHwAddress, _ := net.ParseMAC(arpLayer.SourceHwAddress)
	DstHwAddress, _ := net.ParseMAC(arpLayer.DstHwAddress)

	arp = &layers.ARP{
		AddrType:          layers.LinkTypeEthernet,
		Protocol:          layers.EthernetTypeIPv4,
		HwAddressSize:     6,
		ProtAddressSize:   4,
		Operation:         arpLayer.Operation,
		SourceHwAddress:   SourceHwAddress,
		SourceProtAddress: net.ParseIP(arpLayer.SourceProtAddress),
		DstHwAddress:      DstHwAddress,
		DstProtAddress:    net.ParseIP(arpLayer.DstProtAddress),
	}
	return
}

func SendPacket(ovsDriver *ovsdbDriver.OvsDriver, srcPort string, pkt *Packet, numPkts int) error {

	options = gopacket.SerializeOptions{}
	var buffer gopacket.SerializeBuffer
	handle, err = pcap.OpenLive("port12", snapshot_len, true, pcap.BlockForever)
	if err != nil {
		log.Errorf("openlive: %v", err)
		return err
	}
	defer handle.Close()

	buffer = gopacket.NewSerializeBuffer()
	options.FixLengths = true
	options.ComputeChecksums = true

	sMac, _ := net.ParseMAC(pkt.SrcMAC)
	dMac, _ := net.ParseMAC(pkt.DstMAC)
	ethLayer := &layers.Ethernet{
		SrcMAC: sMac,
		DstMAC: dMac,
	}
	switch {
	case pkt.Arp != nil:
		arpLayer := buildArp(pkt.Arp)
		log.Infof("Sending an ARP packet on %s", srcPort)
		ethLayer.EthernetType = layers.EthernetTypeARP
		gopacket.SerializeLayers(buffer,
			options,
			ethLayer,
			arpLayer)
	case pkt.IPv4 != nil:
		ipLayer := buildIpv4(pkt.IPv4)
		log.Infof("Sending an IPv4 packer on %s", srcPort)
		ethLayer.EthernetType = layers.EthernetTypeIPv4
		gopacket.SerializeLayers(buffer,
			options,
			ethLayer,
			ipLayer)
	case pkt.IPv6 != nil:
		fmt.Println("ipv6")
		log.Infof("Sending an IPv6 packet on %s", srcPort)
		ipLayer := buildIpv6(pkt.IPv6)
		ethLayer.EthernetType = layers.EthernetTypeIPv6
		gopacket.SerializeLayers(buffer,
			options,
			ethLayer,
			ipLayer)
	default:
		ethLayer.EthernetType = 0xFFFF
		log.Infof("Sending an Ethernet packet on %s", srcPort)
		gopacket.SerializeLayers(buffer,
			options,
			ethLayer)
	}

	outgoingPacket := buffer.Bytes()
	for i := 0; i < numPkts; i++ {
		fmt.Println(outgoingPacket)
		handle.WritePacketData(outgoingPacket)
	}

	return err
}

/*
func readPacket(handle *pcap.Handle, pkt *Packet, dstPort string) int {
	log.Infof("Received: Decoding the packet ")
	pktTimer := time.NewTimer(time.Second * 120)
	verifiedCount := 0
	count := 0
	src := gopacket.NewPacketSource(handle, layers.LayerTypeEthernet)
	for {
		select {
		case <-pktTimer.C:
			fmt.Println("Verified count --->", verifiedCount)
			return verifiedCount
		case packet := <-src.Packets():
			count++
			fmt.Println("This is the loop no:", count)
			switch {
			case pkt.Arp != nil:
				arpLayer := packet.Layer(layers.LayerTypeARP)
				if arpLayer == nil {
					continue
				}
				arp := arpLayer.(*layers.ARP)
				if compareIP(arp.SourceProtAddress, net.ParseIP(pkt.Arp.SourceProtAddress)) {
					if compareIP(arp.DstProtAddress, net.ParseIP(pkt.Arp.DstProtAddress)) {
						log.Infof("Verified: Received ARP packetfrom %v", arp.SourceProtAddress)
						verifiedCount++
					}
				}
			case pkt.IPv4 != nil:
				ipLayer := packet.Layer(layers.LayerTypeIPv4)
				if ipLayer == nil {
					continue
				}
				ip := ipLayer.(*layers.IPv4)
				fmt.Println("Received IPv4 packet from %v on %s", ip.SrcIP, dstPort)
				if compareIP(ip.SrcIP.To16(), net.ParseIP(pkt.IPv4.SrcIP)) {
					if compareIP(ip.DstIP.To16(), net.ParseIP(pkt.IPv4.DstIP)) {
						fmt.Println("Incrementing the count")
						fmt.Println("Verified: Received IPv4 packet from %v on %s", ip.SrcIP, dstPort)
						verifiedCount++
					}
				}
			case pkt.IPv6 != nil:
				ipLayer := packet.Layer(layers.LayerTypeIPv6)
				if ipLayer == nil {
					continue
				}
				ip := ipLayer.(*layers.IPv6)
				if compareIP(ip.SrcIP, net.ParseIP(pkt.IPv6.SrcIP)) {
					if compareIP(ip.DstIP, net.ParseIP(pkt.IPv6.DstIP)) {
						log.Infof("Verified: Received IPv6 Packet from %v on %s", ip.SrcIP, dstPort)
						verifiedCount++
					}
				}
			default:
				ethLayer := packet.Layer(layers.LayerTypeEthernet)
				eth := ethLayer.(*layers.Ethernet)
				log.Infof("Verified: Received ethernet packet from %v", eth.SrcMAC)
			}
		}
	}
}
func VerifyPacket(ovsDriver *ovsdbDriver.OvsDriver, dstPort string, pkt *Packet, ch chan bool, timeout int, numPkts int) {
	pktTimer := time.NewTimer(time.Second * 120)
	verifiedCountTotal := 0
		handle, err := pcap.OpenLive(dstPort, 1024, true, pcap.BlockForever)
		if err != nil {
			fmt.Println("Error capturing packet")
			continue
		}
		if handle == nil {
			continue
		}
		defer handle.Close()
		// Start up a goroutine to read in packet data.
		//	stop := make(chan struct{}, 1)
		//defer close(stop)
		//done := make(chan int, 1000)
		for {
		fmt.Println("Calling Go routine Read packet", numPkts)
		verifiedCountTotal += readPacket(handle, pkt, dstPort)
		select {
		case <-pktTimer.C:
			fmt.Println(verifiedCountTotal)
			ch <- (verifiedCountTotal == numPkts)
			//close(stop)
			return
		default:
			continue
		}
	}
}*/
func readPacket(handle *pcap.Handle, pkt *Packet, dstPort string, done chan int) {
	log.Infof("Received: Decoding the packet ")

	pktTimer := time.NewTimer(time.Second * 20)
	verifiedCount := 0
	count := 0
	src := gopacket.NewPacketSource(handle, layers.LayerTypeEthernet)
	for {
		select {
		case <-pktTimer.C:
			fmt.Println("Verified count --->", verifiedCount)
			done <- verifiedCount
		case packet := <-src.Packets():
			count++
			fmt.Println("This is the loop no:", count)
			switch {
			case pkt.Arp != nil:
				arpLayer := packet.Layer(layers.LayerTypeARP)
				if arpLayer == nil {
					continue
				}
				arp := arpLayer.(*layers.ARP)
				if compareIP(arp.SourceProtAddress, net.ParseIP(pkt.Arp.SourceProtAddress)) {
					if compareIP(arp.DstProtAddress, net.ParseIP(pkt.Arp.DstProtAddress)) {
						log.Infof("Verified: Received ARP packetfrom %v", arp.SourceProtAddress)
						verifiedCount++
					}
				}
			case pkt.IPv4 != nil:
				ipLayer := packet.Layer(layers.LayerTypeIPv4)
				if ipLayer == nil {
					continue
				}
				ip := ipLayer.(*layers.IPv4)

				fmt.Println("Received IPv4 packet from %v on %s", ip.SrcIP, dstPort)
				if compareIP(ip.SrcIP.To16(), net.ParseIP(pkt.IPv4.SrcIP)) {
					if compareIP(ip.DstIP.To16(), net.ParseIP(pkt.IPv4.DstIP)) {
						fmt.Println("Incrementing the count")
						fmt.Println("Verified: Received IPv4 packet from %v on %s", ip.SrcIP, dstPort)
						verifiedCount++
					}
				}
			case pkt.IPv6 != nil:
				ipLayer := packet.Layer(layers.LayerTypeIPv6)
				if ipLayer == nil {
					continue
				}
				ip := ipLayer.(*layers.IPv6)
				if compareIP(ip.SrcIP, net.ParseIP(pkt.IPv6.SrcIP)) {
					if compareIP(ip.DstIP, net.ParseIP(pkt.IPv6.DstIP)) {
						log.Infof("Verified: Received IPv6 Packet from %v on %s", ip.SrcIP, dstPort)
						verifiedCount++
					}
				}
			default:
				ethLayer := packet.Layer(layers.LayerTypeEthernet)
				eth := ethLayer.(*layers.Ethernet)
				if eth.SrcMAC.String() == pkt.SrcMAC && eth.DstMAC.String() == pkt.DstMAC {
					log.Infof("Verified: Received Eth Packet from %v on %s", eth.SrcMAC, dstPort)
					verifiedCount++
				}
			}
		}
	}
}

func VerifyPacket(ovsDriver *ovsdbDriver.OvsDriver, dstPort string, pkt *Packet, ch chan bool, timeout int, numPkts int) {
	verifiedCountTotal := 0
	handle, err := pcap.OpenLive(dstPort, 1024, true, pcap.BlockForever)

	if err != nil {
		fmt.Println("Error capturing packet")
		ch <- false
	}

	if handle == nil {
		ch <- false
	}
	defer handle.Close()
	// Start up a goroutine to read in packet data.
	//	stop := make(chan struct{}, 1)
	//defer close(stop)
	done := make(chan int)

	fmt.Println("Calling Go routine Read packet", numPkts)
	go readPacket(handle, pkt, dstPort, done)
	verifiedCountTotal = <-done
	fmt.Println(verifiedCountTotal)
	ch <- (verifiedCountTotal == numPkts)
	close(done)
	return
}
