package protocol

import (
	"encoding/binary"
	"errors"
	"net"

	"github.com/shaleman/libOpenflow/util"
)

// see http://en.wikipedia.org/wiki/EtherType
const (
	IPv4_MSG = 0x0800
	ARP_MSG  = 0x0806
	LLDP_MSG = 0x88cc
	WOL_MSG  = 0x0842
	RARP_MSG = 0x8035
	VLAN_MSG = 0x8100

	IPv6_MSG     = 0x86DD
	STP_MSG      = 0x4242
	STP_BPDU_MSG = 0xAAAA
)

type Ethernet struct {
	Delimiter uint8
	HWDst     net.HardwareAddr
	HWSrc     net.HardwareAddr
	VLANID    VLAN
	Ethertype uint16
	Data      util.Message
}

func NewEthernet() *Ethernet {
	eth := new(Ethernet)
	eth.HWDst = net.HardwareAddr(make([]byte, 6))
	eth.HWSrc = net.HardwareAddr(make([]byte, 6))
	eth.VLANID = *NewVLAN()
	eth.Ethertype = 0x800
	eth.Data = nil
	return eth
}

func (e *Ethernet) Len() (n uint16) {
	n = 0
	n += 12
	if e.VLANID.VID != 0 {
		n += 4
	}
	n += 2
	if e.Data != nil {
		n += e.Data.Len()
	}
	return
}

func (e *Ethernet) MarshalBinary() (data []byte, err error) {
	data = make([]byte, int(e.Len()))
	bytes := make([]byte, 0)
	n := 0
	copy(data[n:], e.HWDst)
	n += len(e.HWDst)
	copy(data[n:], e.HWSrc)
	n += len(e.HWSrc)

	if e.VLANID.VID != 0 {
		bytes, err = e.VLANID.MarshalBinary()
		if err != nil {
			return
		}
		copy(data[n:], bytes)
		n += len(bytes)
	}

	binary.BigEndian.PutUint16(data[n:n+2], e.Ethertype)
	n += 2

	if e.Data != nil {
		bytes, err = e.Data.MarshalBinary()
		if err != nil {
			return
		}
		copy(data[n:n+len(bytes)], bytes)
	}
	return
}

func (e *Ethernet) UnmarshalBinary(data []byte) error {
	if len(data) < 14 {
		return errors.New("The []byte is too short to unmarshal a full Ethernet message.")
	}
	n := 0
	e.HWDst = net.HardwareAddr(make([]byte, 6))
	copy(e.HWDst, data[n:n+6])
	n += 6
	e.HWSrc = net.HardwareAddr(make([]byte, 6))
	copy(e.HWSrc, data[n:n+6])
	n += 6

	e.Ethertype = binary.BigEndian.Uint16(data[n:])
	if e.Ethertype == VLAN_MSG {
		e.VLANID = *new(VLAN)
		err := e.VLANID.UnmarshalBinary(data[n:])
		if err != nil {
			return err
		}
		n += int(e.VLANID.Len())

		e.Ethertype = binary.BigEndian.Uint16(data[n:])
	} else {
		e.VLANID = *new(VLAN)
		e.VLANID.VID = 0
	}
	n += 2

	switch e.Ethertype {
	case IPv4_MSG:
		e.Data = new(IPv4)
	case ARP_MSG:
		e.Data = new(ARP)
	default:
		e.Data = new(util.Buffer)
	}
	return e.Data.UnmarshalBinary(data[n:])
}

const (
	PCP_MASK = 0xe000
	DEI_MASK = 0x1000
	VID_MASK = 0x0fff
)

type VLAN struct {
	TPID uint16
	PCP  uint8
	DEI  uint8
	VID  uint16
}

func NewVLAN() *VLAN {
	v := new(VLAN)
	v.TPID = 0x8100
	v.VID = 0
	return v
}

func (v *VLAN) Len() (n uint16) {
	return 4
}

func (v *VLAN) MarshalBinary() (data []byte, err error) {
	data = make([]byte, v.Len())
	binary.BigEndian.PutUint16(data[:2], v.TPID)
	var tci uint16
	tci = (tci | uint16(v.PCP)<<13) + (tci | uint16(v.DEI)<<12) + (tci | v.VID)
	binary.BigEndian.PutUint16(data[2:], tci)
	return
}

func (v *VLAN) UnmarshalBinary(data []byte) error {
	if len(data) < 4 {
		return errors.New("The []byte is too short to unmarshal a full VLAN header.")
	}
	v.TPID = binary.BigEndian.Uint16(data[:2])
	var tci uint16
	tci = binary.BigEndian.Uint16(data[2:])
	v.PCP = uint8(PCP_MASK & tci >> 13)
	v.DEI = uint8(DEI_MASK & tci >> 12)
	v.VID = VID_MASK & tci
	return nil
}
