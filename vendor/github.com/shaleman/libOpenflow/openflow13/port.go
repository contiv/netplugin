package openflow13

import (
	"encoding/binary"
	"net"

	"github.com/shaleman/libOpenflow/common"
)

// ofp_port 1.3
type PhyPort struct {
	PortNo uint32
	pad    []byte // 4 bytes
	HWAddr net.HardwareAddr
	pad2   []byte // 2 bytes for 64bit alignment
	Name   []byte // Size 16

	Config uint32
	State  uint32

	Curr       uint32
	Advertised uint32
	Supported  uint32
	Peer       uint32

	CurrSpeed uint32
	MaxSpeed  uint32
}

func NewPhyPort() *PhyPort {
	p := new(PhyPort)
	p.HWAddr = make([]byte, ETH_ALEN)
	p.Name = make([]byte, 16)
	return p
}

func (p *PhyPort) Len() (n uint16) {
	n += 4
	n += 6 // padding
	n += uint16(len(p.HWAddr) + len(p.Name))
	n += 32
	return
}

func (p *PhyPort) MarshalBinary() (data []byte, err error) {
	data = make([]byte, int(p.Len()))
	binary.BigEndian.PutUint32(data, p.PortNo)
	n := 4
	copy(data[n:], p.pad)
	n += 4
	copy(data[n:], p.HWAddr)
	n += len(p.HWAddr)
	copy(data[n:], p.pad2)
	n += 2
	copy(data[n:], p.Name)
	n += len(p.Name)

	binary.BigEndian.PutUint32(data[n:], p.Config)
	n += 4
	binary.BigEndian.PutUint32(data[n:], p.State)
	n += 4
	binary.BigEndian.PutUint32(data[n:], p.Curr)
	n += 4
	binary.BigEndian.PutUint32(data[n:], p.Advertised)
	n += 4
	binary.BigEndian.PutUint32(data[n:], p.Supported)
	n += 4
	binary.BigEndian.PutUint32(data[n:], p.Peer)
	n += 4
	binary.BigEndian.PutUint32(data[n:], p.CurrSpeed)
	n += 4
	binary.BigEndian.PutUint32(data[n:], p.MaxSpeed)
	n += 4
	return
}

func (p *PhyPort) UnmarshalBinary(data []byte) error {
	p.PortNo = binary.BigEndian.Uint32(data)
	n := 4
	copy(p.pad, data[n:n+4])
	n += 4
	copy(p.HWAddr, data[n:n+6])
	n += 6
	copy(p.pad2, data[n:n+2])
	n += 2
	copy(p.Name, data[n:n+16])
	n += 16

	p.Config = binary.BigEndian.Uint32(data[n:])
	n += 4
	p.State = binary.BigEndian.Uint32(data[n:])
	n += 4
	p.Curr = binary.BigEndian.Uint32(data[n:])
	n += 4
	p.Advertised = binary.BigEndian.Uint32(data[n:])
	n += 4
	p.Supported = binary.BigEndian.Uint32(data[n:])
	n += 4
	p.Peer = binary.BigEndian.Uint32(data[n:])
	n += 4
	p.CurrSpeed = binary.BigEndian.Uint32(data[n:])
	n += 4
	p.MaxSpeed = binary.BigEndian.Uint32(data[n:])
	n += 4
	return nil
}

// ofp_port_mod 1.3
type PortMod struct {
	common.Header
	PortNo uint32
	pad    []byte // 4 bytes
	HWAddr []uint8
	pad2   []byte // 2 bytes for 64byte alignment

	Config    uint32
	Mask      uint32
	Advertise uint32
	pad3      []uint8 // Size 4
}

func NewPortMod(port int) *PortMod {
	p := new(PortMod)
	p.Header.Type = Type_PortMod
	p.PortNo = uint32(port)
	p.HWAddr = make([]byte, ETH_ALEN)
	p.pad = make([]byte, 4)
	p.pad2 = make([]byte, 2)
	p.pad3 = make([]byte, 4)
	return p
}

func (p *PortMod) Len() (n uint16) {
	return p.Header.Len() + 4 + 4 + ETH_ALEN + 2 + 12 + 4
}

func (p *PortMod) MarshalBinary() (data []byte, err error) {
	p.Header.Length = p.Len()
	data, err = p.Header.MarshalBinary()

	b := make([]byte, 32)
	n := 0
	binary.BigEndian.PutUint32(b[n:], p.PortNo)
	n += 4
	copy(b[n:], p.pad)
	n += 4
	copy(b[n:], p.HWAddr)
	n += ETH_ALEN
	copy(b[n:], p.pad2)
	n += 2
	binary.BigEndian.PutUint32(b[n:], p.Config)
	n += 4
	binary.BigEndian.PutUint32(b[n:], p.Mask)
	n += 4
	binary.BigEndian.PutUint32(b[n:], p.Advertise)
	n += 4
	copy(b[n:], p.pad3)
	n += 4
	data = append(data, b...)
	return
}

func (p *PortMod) UnmarshalBinary(data []byte) error {
	err := p.Header.UnmarshalBinary(data)
	n := int(p.Header.Len())

	p.PortNo = binary.BigEndian.Uint32(data[n:])
	n += 4
	copy(p.pad, data[n:n+4])
	n += 4
	copy(p.HWAddr, data[n:])
	n += len(p.HWAddr)
	copy(p.pad2, data[n:n+2])
	n += 2
	p.Config = binary.BigEndian.Uint32(data[n:])
	n += 4
	p.Mask = binary.BigEndian.Uint32(data[n:])
	n += 4
	p.Advertise = binary.BigEndian.Uint32(data[n:])
	n += 4
	copy(p.pad3, data[n:])
	n += 4
	return err
}

const (
	ETH_ALEN          = 6
	MAX_PORT_NAME_LEN = 16
)

// ofp_port_config 1.3
const (
	PC_PORT_DOWN = 1 << 0

	PC_NO_RECV      = 1 << 2
	PC_NO_FWD       = 1 << 5
	PC_NO_PACKET_IN = 1 << 6
)

// ofp_port_state 1.3
const (
	PS_LINK_DOWN = 1 << 0
	PS_BLOCKED   = 1 << 1
	PS_LIVE      = 1 << 2
)

// ofp_port_no 1.3
const (
	P_MAX = 0xffffff00

	P_IN_PORT = 0xfffffff8
	P_TABLE   = 0xfffffff9

	P_NORMAL = 0xfffffffa
	P_FLOOD  = 0xfffffffb

	P_ALL        = 0xfffffffc
	P_CONTROLLER = 0xfffffffd
	P_LOCAL      = 0xfffffffe
	P_ANY        = 0xffffffff
)

// ofp_port_features 1.3
const (
	PF_10MB_HD  = 1 << 0
	PF_10MB_FD  = 1 << 1
	PF_100MB_HD = 1 << 2
	PF_100MB_FD = 1 << 3
	PF_1GB_HD   = 1 << 4
	PF_1GB_FD   = 1 << 5
	PF_10GB_FD  = 1 << 6
	PF_40GB_FD  = 1 << 7
	PF_100GB_FD = 1 << 8
	PF_1TB_FD   = 1 << 9
	PF_OTHER    = 1 << 10

	PF_COPPER     = 1 << 11
	PF_FIBER      = 1 << 12
	PF_AUTONEG    = 1 << 13
	PF_PAUSE      = 1 << 14
	PF_PAUSE_ASYM = 1 << 15
)

// END: 13 - 7.2.1
