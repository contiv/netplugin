package protocol

import (
	"encoding/binary"
	"errors"
)

type UDP struct {
	PortSrc  uint16
	PortDst  uint16
	Length   uint16
	Checksum uint16
	Data     []byte
}

func NewUDP() *UDP {
	u := new(UDP)
	u.Data = make([]byte, 0)
	return u
}

func (u *UDP) Len() (n uint16) {
	if u.Data != nil {
		return uint16(8 + len(u.Data))
	}
	return uint16(8)
}

func (u *UDP) MarshalBinary() (data []byte, err error) {
	data = make([]byte, int(u.Len()))
	binary.BigEndian.PutUint16(data[:2], u.PortSrc)
	binary.BigEndian.PutUint16(data[2:4], u.PortDst)
	binary.BigEndian.PutUint16(data[4:6], u.Length)
	binary.BigEndian.PutUint16(data[6:8], u.Checksum)
	copy(data[8:], u.Data)
	return
}

func (u *UDP) UnmarshalBinary(data []byte) error {
	if len(data) < 8 {
		return errors.New("The []byte is too short to unmarshal a full ARP message.")
	}
	u.PortSrc = binary.BigEndian.Uint16(data[:2])
	u.PortDst = binary.BigEndian.Uint16(data[2:4])
	u.Length = binary.BigEndian.Uint16(data[4:6])
	u.Checksum = binary.BigEndian.Uint16(data[6:8])

	for n, _ := range data[8:] {
		u.Data = append(u.Data, data[n])
	}
	return nil
}
