package protocol

import (
	"encoding/binary"
	"errors"
	"net"
)

const (
	Type_Request = 1
	Type_Reply   = 2
)

type ARP struct {
	HWType      uint16
	ProtoType   uint16
	HWLength    uint8
	ProtoLength uint8
	Operation   uint16
	HWSrc       net.HardwareAddr
	IPSrc       net.IP
	HWDst       net.HardwareAddr
	IPDst       net.IP
}

func NewARP(opt int) (*ARP, error) {
	if opt != Type_Request && opt != Type_Reply {
		return nil, errors.New("Invalid ARP Operation.")
	}
	a := new(ARP)
	a.HWType = 1
	a.ProtoType = 0x800
	a.HWLength = 6
	a.ProtoLength = 4
	a.Operation = uint16(opt)
	a.HWSrc = net.HardwareAddr(make([]byte, 6))
	a.IPSrc = net.IP(make([]byte, 4))
	a.HWDst = net.HardwareAddr(make([]byte, 6))
	a.IPDst = net.IP(make([]byte, 4))
	return a, nil
}

func (a *ARP) Len() (n uint16) {
	n = 8
	n += uint16(a.HWLength*2 + a.ProtoLength*2)
	return
}

func (a *ARP) MarshalBinary() (data []byte, err error) {
	data = make([]byte, int(a.Len()))
	binary.BigEndian.PutUint16(data[:2], a.HWType)
	binary.BigEndian.PutUint16(data[2:4], a.ProtoType)
	data[4] = a.HWLength
	data[5] = a.ProtoLength
	binary.BigEndian.PutUint16(data[6:8], a.Operation)

	n := 8

	copy(data[n:n+int(a.HWLength)], a.HWSrc)
	n += int(a.HWLength)
	copy(data[n:n+int(a.ProtoLength)], a.IPSrc.To4())
	n += int(a.ProtoLength)
	copy(data[n:n+int(a.HWLength)], a.HWDst)
	n += int(a.HWLength)
	copy(data[n:n+int(a.ProtoLength)], a.IPDst.To4())
	return data, nil
}

func (a *ARP) UnmarshalBinary(data []byte) error {
	if len(data) < 8 {
		return errors.New("The []byte is too short to unmarshal a full ARP message.")
	}
	a.HWType = binary.BigEndian.Uint16(data[:2])
	a.ProtoType = binary.BigEndian.Uint16(data[2:4])
	a.HWLength = data[4]
	a.ProtoLength = data[5]
	a.Operation = binary.BigEndian.Uint16(data[6:8])

	n := 8
	if len(data[n:]) < (int(a.HWLength)*2 + int(a.ProtoLength)*2) {
		return errors.New("The []byte is too short to unmarshal a full ARP message.")
	}
	a.HWSrc = data[n : n+int(a.HWLength)]
	n += int(a.HWLength)
	a.IPSrc = data[n : n+int(a.ProtoLength)]
	n += int(a.ProtoLength)
	a.HWDst = data[n : n+int(a.HWLength)]
	n += int(a.HWLength)
	a.IPDst = data[n : n+int(a.ProtoLength)]
	return nil
}
