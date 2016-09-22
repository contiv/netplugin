package openflow13

// This file contains OFP 1.3 instruction defenitions

import (
	"encoding/binary"
	"errors"

	"github.com/shaleman/libOpenflow/util"
)

// ofp_instruction_type 1.3
const (
	InstrType_GOTO_TABLE     = 1      /* Setup the next table in the lookup pipeline */
	InstrType_WRITE_METADATA = 2      /* Setup the metadata field for use later in pipeline */
	InstrType_WRITE_ACTIONS  = 3      /* Write the action(s) onto the datapath action set */
	InstrType_APPLY_ACTIONS  = 4      /* Applies the action(s) immediately */
	InstrType_CLEAR_ACTIONS  = 5      /* Clears all actions from the datapath action set */
	InstrType_METER          = 6      /* Apply meter (rate limiter) */
	InstrType_EXPERIMENTER   = 0xFFFF /* Experimenter instruction */
)

// Generic instruction header
type InstrHeader struct {
	Type   uint16
	Length uint16
}

type Instruction interface {
	util.Message
	AddAction(act Action, prepend bool) error
}

func (a *InstrHeader) Len() (n uint16) {
	return 4
}

func (a *InstrHeader) MarshalBinary() (data []byte, err error) {
	data = make([]byte, a.Len())
	binary.BigEndian.PutUint16(data[:2], a.Type)
	binary.BigEndian.PutUint16(data[2:4], a.Length)
	return
}

func (a *InstrHeader) UnmarshalBinary(data []byte) error {
	if len(data) != 4 {
		return errors.New("Wrong size to unmarshal an InstrHeader message.")
	}
	a.Type = binary.BigEndian.Uint16(data[:2])
	a.Length = binary.BigEndian.Uint16(data[2:4])
	return nil
}

func DecodeInstr(data []byte) Instruction {
	t := binary.BigEndian.Uint16(data[:2])
	var a Instruction
	switch t {
	case InstrType_GOTO_TABLE:
		a = new(InstrGotoTable)
	case InstrType_WRITE_METADATA:
		a = new(InstrWriteMetadata)
	case InstrType_WRITE_ACTIONS:
		a = new(InstrActions)
	case InstrType_APPLY_ACTIONS:
		a = new(InstrActions)
	case InstrType_CLEAR_ACTIONS:
		a = new(InstrActions)
	case InstrType_METER:
		a = new(InstrMeter)
	case InstrType_EXPERIMENTER:
	}

	a.UnmarshalBinary(data)
	return a
}

type InstrGotoTable struct {
	InstrHeader
	TableId uint8
	pad     []byte // 3 bytes
}

func (instr *InstrGotoTable) Len() (n uint16) {
	return 8
}

func (instr *InstrGotoTable) MarshalBinary() (data []byte, err error) {
	data, err = instr.InstrHeader.MarshalBinary()

	b := make([]byte, 4)
	b[0] = instr.TableId
	copy(b[3:], instr.pad)

	data = append(data, b...)
	return
}

func (instr *InstrGotoTable) UnmarshalBinary(data []byte) error {
	instr.InstrHeader.UnmarshalBinary(data[:4])

	instr.TableId = data[4]
	copy(instr.pad, data[5:8])

	return nil
}

func NewInstrGotoTable(tableId uint8) *InstrGotoTable {
	instr := new(InstrGotoTable)
	instr.Type = InstrType_GOTO_TABLE
	instr.TableId = tableId
	instr.pad = make([]byte, 3)
	instr.Length = instr.Len()

	return instr
}

func (instr *InstrGotoTable) AddAction(act Action, prepend bool) error {
	return errors.New("Not supported on this instrction")
}

type InstrWriteMetadata struct {
	InstrHeader
	pad          []byte // 4 bytes
	Metadata     uint64 /* Metadata value to write */
	MetadataMask uint64 /* Metadata write bitmask */
}

// FIXME: we need marshall/unmarshall/len/new functions for write metadata instr
func (instr *InstrWriteMetadata) Len() (n uint16) {
	return 24
}

func (instr *InstrWriteMetadata) MarshalBinary() (data []byte, err error) {
	data, err = instr.InstrHeader.MarshalBinary()

	b := make([]byte, 20)
	copy(b, instr.pad)
	binary.BigEndian.PutUint64(b[4:], instr.Metadata)
	binary.BigEndian.PutUint64(b[12:], instr.MetadataMask)

	data = append(data, b...)
	return
}

func (instr *InstrWriteMetadata) UnmarshalBinary(data []byte) error {
	instr.InstrHeader.UnmarshalBinary(data[:4])

	copy(instr.pad, data[4:8])
	instr.Metadata = binary.BigEndian.Uint64(data[8:16])
	instr.MetadataMask = binary.BigEndian.Uint64(data[16:24])

	return nil
}

func NewInstrWriteMetadata(metadata, metadataMask uint64) *InstrWriteMetadata {
	instr := new(InstrWriteMetadata)
	instr.Type = InstrType_WRITE_METADATA
	instr.pad = make([]byte, 4)
	instr.Metadata = metadata
	instr.MetadataMask = metadataMask
	instr.Length = instr.Len()

	return instr
}

func (instr *InstrWriteMetadata) AddAction(act Action, prepend bool) error {
	return errors.New("Not supported on this instrction")
}

// *_ACTION instructions
type InstrActions struct {
	InstrHeader
	pad     []byte   // 4 bytes
	Actions []Action /* 0 or more actions associated with OFPIT_WRITE_ACTIONS and OFPIT_APPLY_ACTIONS */
}

func (instr *InstrActions) Len() (n uint16) {
	n = 8

	for _, act := range instr.Actions {
		n += act.Len()
	}

	return
}

func (instr *InstrActions) MarshalBinary() (data []byte, err error) {
	data, err = instr.InstrHeader.MarshalBinary()

	b := make([]byte, 4)
	copy(b, instr.pad)
	data = append(data, b...)

	for _, act := range instr.Actions {
		b, err = act.MarshalBinary()
		data = append(data, b...)
	}

	return
}

func (instr *InstrActions) UnmarshalBinary(data []byte) error {
	instr.InstrHeader.UnmarshalBinary(data[:4])

	n := 8
	for n < int(instr.Length) {
		act := DecodeAction(data[n:])
		instr.Actions = append(instr.Actions, act)
		n += int(act.Len())
	}

	return nil
}

func (instr *InstrActions) AddAction(act Action, prepend bool) error {
	// Append or prepend to the list
	if prepend {
		instr.Actions = append([]Action{act}, instr.Actions...)
	} else {
		instr.Actions = append(instr.Actions, act)
	}

	instr.Length = instr.Len()
	return nil
}

func NewInstrWriteActions() *InstrActions {
	instr := new(InstrActions)
	instr.Type = InstrType_WRITE_ACTIONS
	instr.pad = make([]byte, 4)
	instr.Actions = make([]Action, 0)
	instr.Length = instr.Len()

	return instr
}

func NewInstrApplyActions() *InstrActions {
	instr := new(InstrActions)
	instr.Type = InstrType_APPLY_ACTIONS
	instr.pad = make([]byte, 4)
	instr.Actions = make([]Action, 0)
	instr.Length = instr.Len()

	return instr
}

type InstrMeter struct {
	InstrHeader
	MeterId uint32
}

func (instr *InstrMeter) AddAction(act Action, prepend bool) error {
	return errors.New("Not supported on this instrction")
}
