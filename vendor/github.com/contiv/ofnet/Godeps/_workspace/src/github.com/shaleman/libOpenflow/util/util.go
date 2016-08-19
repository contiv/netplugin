package util

import (
	"bytes"
)

type Message interface {
	//encoding.BinaryMarshaler
	//encoding.BinaryUnmarshaler
	MarshalBinary() (data []byte, err error)
	UnmarshalBinary(data []byte) error

	Len() uint16
}

type Buffer struct{ bytes.Buffer }

func NewBuffer(buf []byte) *Buffer {
	b := new(Buffer)
	b.Buffer = *bytes.NewBuffer(buf)
	return b
}

func (b *Buffer) Len() uint16 {
	return uint16(b.Buffer.Len())
}

func (b *Buffer) MarshalBinary() (data []byte, err error) {
	return b.Buffer.Bytes(), nil
}

func (b *Buffer) UnmarshalBinary(data []byte) error {
	b.Buffer.Reset()
	_, err := b.Buffer.Write(data)
	return err
}
