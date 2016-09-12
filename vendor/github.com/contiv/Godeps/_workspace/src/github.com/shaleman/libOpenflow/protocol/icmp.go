package protocol

import (
	"encoding/binary"
	"errors"
)

type ICMP struct {
	Type     uint8
	Code     uint8
	Checksum uint16
	Data     []byte
}

func NewICMP() *ICMP {
	i := new(ICMP)
	i.Data = make([]byte, 0)
	return i
}

func (i *ICMP) Len() (n uint16) {
	return uint16(4 + len(i.Data))
}

func (i *ICMP) MarshalBinary() (data []byte, err error) {
	data = make([]byte, int(i.Len()))
	data[0] = i.Type
	data[1] = i.Code
	binary.BigEndian.PutUint16(data[2:4], i.Checksum)
	copy(data[4:], i.Data)
	return
}

func (i *ICMP) UnmarshalBinary(data []byte) error {
	if len(data) < 4 {
		return errors.New("The []byte is too short to unmarshal a full ARP message.")
	}
	i.Type = data[0]
	i.Code = data[1]
	i.Checksum = binary.BigEndian.Uint16(data[2:4])

	for n, _ := range data[4:] {
		i.Data = append(i.Data, data[n])
	}
	return nil
}
