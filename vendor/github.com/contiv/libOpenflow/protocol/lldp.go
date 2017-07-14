package protocol

import (
	"bytes"
	"encoding/binary"
)

type LLDP struct {
	Chassis ChassisTLV
	Port    PortTLV
	TTL     TTLTLV
}

func (d *LLDP) Len() (n uint16) {
	return 15
}

func (d *LLDP) Read(b []byte) (n int, err error) {
	m, o, p := 0, 0, 0
	if m, err = d.Chassis.Read(b); m == 0 {
		return
	}
	n += m
	if o, err = d.Port.Read(b); o == 0 {
		return
	}
	n += o
	if p, err = d.Chassis.Read(b); p == 0 {
		return
	}
	n += p
	return
}

func (d *LLDP) Write(b []byte) (n int, err error) {
	m, o, p := 0, 0, 0
	if m, err = d.Chassis.Write(b); m == 0 {
		return
	}
	n += m
	if o, err = d.Port.Write(b[n:]); o == 0 {
		return
	}
	n += o
	if p, err = d.Chassis.Write(b[n:]); p == 0 {
		return
	}
	n += p
	return
}

// Chassis ID subtypes
const (
	_ = iota
	CH_CHASSIS_COMPONENT
	CH_IFACE_ALIAS
	CH_PORT_COMPONENT
	CH_MAC_ADDR
	CH_NET_ADDR
	CH_IFACE_NAME
	CH_LOCAL_ASSGN
)

type ChassisTLV struct {
	Type    uint8
	Length  uint16
	Subtype uint8
	Data    []uint8
}

func (t *ChassisTLV) Read(b []byte) (n int, err error) {
	buf := new(bytes.Buffer)
	var tni uint16 = 0
	typeAndLen := (tni | uint16(t.Type)<<9) + (tni | uint16(t.Length))
	binary.Write(buf, binary.BigEndian, typeAndLen)
	binary.Write(buf, binary.BigEndian, t.Subtype)
	binary.Write(buf, binary.BigEndian, t.Data)
	n, err = buf.Read(b)
	return
}

func (t *ChassisTLV) Write(b []byte) (n int, err error) {
	buf := bytes.NewBuffer(b)
	var typeAndLen uint16 = 0
	if err = binary.Read(buf, binary.BigEndian, &typeAndLen); err != nil {
		return
	}
	n += 2
	t.Type = uint8(typeAndLen >> 9)
	t.Length = uint16(uint16(0x01ff) & typeAndLen)
	if err = binary.Read(buf, binary.BigEndian, &t.Subtype); err != nil {
		return
	}
	n += 1
	t.Data = make([]uint8, t.Length)
	if err = binary.Read(buf, binary.BigEndian, &t.Data); err != nil {
		return
	}
	n += int(t.Length)
	return
}

// Port ID subtypes
const (
	_ = iota
	PT_IFACE_ALIAS
	PT_PORT_COMPONENT
	PT_MAC_ADDR
	PT_NET_ADDR
	PT_IFACE_NAME
	PT_CIRCUIT_ID
	PT_LOCAL_ASSGN
)

type PortTLV struct {
	Type    uint8  //7bits
	Length  uint16 //9bits
	Subtype uint8
	Data    []uint8
}

func (t *PortTLV) Read(b []byte) (n int, err error) {
	buf := new(bytes.Buffer)
	var tni uint16 = 0
	typeAndLen := (tni | uint16(t.Type)<<9) + (tni | uint16(t.Length))
	binary.Write(buf, binary.BigEndian, typeAndLen)
	binary.Write(buf, binary.BigEndian, t.Subtype)
	binary.Write(buf, binary.BigEndian, t.Data)
	n, err = buf.Read(b)
	return
}

func (t *PortTLV) Write(b []byte) (n int, err error) {
	buf := bytes.NewBuffer(b)
	var typeAndLen uint16 = 0
	if err = binary.Read(buf, binary.BigEndian, &typeAndLen); err != nil {
		return
	}
	n += 2
	t.Type = uint8(typeAndLen >> 9)
	t.Length = uint16(uint16(0x01ff) & typeAndLen)
	if err = binary.Read(buf, binary.BigEndian, &t.Subtype); err != nil {
		return
	}
	n += 1
	t.Data = make([]uint8, t.Length)
	if err = binary.Read(buf, binary.BigEndian, &t.Data); err != nil {
		return
	}
	n += int(t.Length)
	return
}

type TTLTLV struct {
	Type    uint8  //7 bits
	Length  uint16 //9 bits
	Seconds uint16
}

func (t *TTLTLV) Read(b []byte) (n int, err error) {
	buf := new(bytes.Buffer)
	var tni uint16 = 0
	typeAndLen := (tni | uint16(t.Type)<<9) + (tni | uint16(t.Length))
	binary.Write(buf, binary.BigEndian, typeAndLen)
	binary.Write(buf, binary.BigEndian, t.Seconds)
	n, err = buf.Read(b)
	return
}

func (t *TTLTLV) Write(b []byte) (n int, err error) {
	buf := bytes.NewBuffer(b)
	var typeAndLen uint16 = 0
	if err = binary.Read(buf, binary.BigEndian, &typeAndLen); err != nil {
		return
	}
	n += 2
	t.Type = uint8(typeAndLen >> 9)
	t.Length = uint16(uint16(0x01ff) & typeAndLen)
	if err = binary.Read(buf, binary.BigEndian, &t.Seconds); err != nil {
		return
	}
	n += 2
	return
}
