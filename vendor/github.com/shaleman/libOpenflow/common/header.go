package common

// Package common defines OpenFlow message types that are version independent.

import (
	"encoding/binary"
	"errors"

	"github.com/shaleman/libOpenflow/util"
)

var messageXid uint32 = 1

func NewHeaderGenerator(ver int) func() Header {
	return func() Header {
		messageXid += 1
		p := Header{uint8(ver), 0, 8, messageXid}
		return p
	}
}

// The version specifies the OpenFlow protocol version being
// used. During the current draft phase of the OpenFlow
// Protocol, the most significant bit will be set to indicate an
// experimental version and the lower bits will indicate a
// revision number. The current version is 0x01. The final
// version for a Type 0 switch will be 0x00. The length field
// indicates the total length of the message, so no additional
// framing is used to distinguish one frame from the next.
type Header struct {
	Version uint8
	Type    uint8
	Length  uint16
	Xid     uint32
}

func (h *Header) Header() *Header {
	return h
}

func (h *Header) Len() (n uint16) {
	return 8
}

func (h *Header) MarshalBinary() (data []byte, err error) {
	data = make([]byte, 8)
	data[0] = h.Version
	data[1] = h.Type
	binary.BigEndian.PutUint16(data[2:4], h.Length)
	binary.BigEndian.PutUint32(data[4:8], h.Xid)
	return
}

func (h *Header) UnmarshalBinary(data []byte) error {
	if len(data) < 4 {
		return errors.New("The []byte is too short to unmarshel a full HelloElemHeader.")
	}
	h.Version = data[0]
	h.Type = data[1]
	h.Length = binary.BigEndian.Uint16(data[2:4])
	h.Xid = binary.BigEndian.Uint32(data[4:8])
	return nil
}

const (
	reserved = iota
	HelloElemType_VersionBitmap
)

type HelloElem interface {
	Header() *HelloElemHeader
	util.Message
}

type HelloElemHeader struct {
	Type   uint16
	Length uint16
}

func NewHelloElemHeader() *HelloElemHeader {
	h := new(HelloElemHeader)
	h.Type = HelloElemType_VersionBitmap
	h.Length = 4
	return h
}

func (h *HelloElemHeader) Header() *HelloElemHeader {
	return h
}

func (h *HelloElemHeader) Len() (n uint16) {
	return 4
}

func (h *HelloElemHeader) MarshalBinary() (data []byte, err error) {
	data = make([]byte, 4)
	binary.BigEndian.PutUint16(data[:2], h.Type)
	binary.BigEndian.PutUint16(data[2:4], h.Length)
	return
}

func (h *HelloElemHeader) UnmarshalBinary(data []byte) error {
	if len(data) < 4 {
		return errors.New("The []byte is too short to unmarshal a full HelloElemHeader.")
	}
	h.Type = binary.BigEndian.Uint16(data[:2])
	h.Length = binary.BigEndian.Uint16(data[2:4])
	return nil
}

type HelloElemVersionBitmap struct {
	HelloElemHeader
	Bitmaps []uint32
}

func NewHelloElemVersionBitmap() *HelloElemVersionBitmap {
	h := new(HelloElemVersionBitmap)
	h.HelloElemHeader = *NewHelloElemHeader()
	h.Bitmaps = make([]uint32, 0)
	// 10010 meaning openflow 1.0 & 1.3 support
	h.Bitmaps = append(h.Bitmaps, uint32(1<<4)|uint32(1<<1))
	h.Length = h.Length + uint16(len(h.Bitmaps)*4)
	return h
}

func (h *HelloElemVersionBitmap) Header() *HelloElemHeader {
	return &h.HelloElemHeader
}

func (h *HelloElemVersionBitmap) Len() (n uint16) {
	n = h.HelloElemHeader.Len()
	n += uint16(len(h.Bitmaps) * 4)
	return
}

func (h *HelloElemVersionBitmap) MarshalBinary() (data []byte, err error) {
	data = make([]byte, int(h.Len()))
	bytes := make([]byte, 0)
	next := 0

	bytes, err = h.HelloElemHeader.MarshalBinary()
	copy(data[next:], bytes)
	next += len(bytes)

	for _, m := range h.Bitmaps {
		binary.BigEndian.PutUint32(data[next:], m)
		next += 4
	}
	return
}

func (h *HelloElemVersionBitmap) UnmarshalBinary(data []byte) error {
	length := len(data)
	read := 0
	if err := h.HelloElemHeader.UnmarshalBinary(data[:4]); err != nil {
		return err
	}
	read += int(h.HelloElemHeader.Len())

	h.Bitmaps = make([]uint32, 0)
	for read < length {
		h.Bitmaps = append(h.Bitmaps, binary.BigEndian.Uint32(data[read:read+4]))
		read += 4
	}
	return nil
}

// The OFPT_HELLO message consists of an OpenFlow header plus a set of variable
// size hello elements. The version field part of the header field (see 7.1)
// must be set to the highest OpenFlow switch protocol version supported by the
// sender (see 6.3.1).  The elements field is a set of hello elements,
// containing optional data to inform the initial handshake of the connection.
// Implementations must ignore (skip) all elements of a Hello message that they
// do not support.
// The version field part of the header field (see 7.1) must be set to the
// highest OpenFlow switch protocol version supported by the sender (see 6.3.1).
// The elements field is a set of hello elements, containing optional data to
// inform the initial handshake of the connection. Implementations must ignore
// (skip) all elements of a Hello message that they do not support.
type Hello struct {
	Header
	Elements []HelloElem
}

func NewHello(ver int) (h *Hello, err error) {
	h = new(Hello)
	h.Header = NewHeaderGenerator(ver)()
	h.Elements = make([]HelloElem, 0)
	h.Elements = append(h.Elements, NewHelloElemVersionBitmap())

	return
}

func (h *Hello) Len() (n uint16) {
	n = h.Header.Len()
	for _, e := range h.Elements {
		n += e.Len()
	}
	return
}

func (h *Hello) MarshalBinary() (data []byte, err error) {
	data = make([]byte, int(h.Len()))
	bytes := make([]byte, 0)
	next := 0

	h.Header.Length = h.Len()
	bytes, err = h.Header.MarshalBinary()
	copy(data[next:], bytes)
	next += len(bytes)

	for _, e := range h.Elements {
		bytes, err = e.MarshalBinary()
		copy(data[next:], bytes)
		next += len(bytes)
	}
	return
}

func (h *Hello) UnmarshalBinary(data []byte) error {
	next := 0
	err := h.Header.UnmarshalBinary(data[next:])
	next += int(h.Header.Len())

	h.Elements = make([]HelloElem, 0)
	for next < len(data) {
		e := NewHelloElemHeader()
		e.UnmarshalBinary(data[next:])

		switch e.Type {
		case HelloElemType_VersionBitmap:
			v := NewHelloElemVersionBitmap()
			err = v.UnmarshalBinary(data[next:])
			next += int(v.Len())
			h.Elements = append(h.Elements, v)
		}
	}
	return err
}
