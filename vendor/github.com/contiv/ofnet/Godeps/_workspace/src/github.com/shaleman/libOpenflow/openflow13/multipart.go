package openflow13

import (
	"encoding/binary"

	log "github.com/Sirupsen/logrus"

	"github.com/shaleman/libOpenflow/common"
	"github.com/shaleman/libOpenflow/util"
)

// ofp_multipart_request 1.3
type MultipartRequest struct {
	common.Header
	Type  uint16
	Flags uint16
	pad   []byte // 4 bytes
	Body  util.Message
}

func (s *MultipartRequest) Len() (n uint16) {
	return s.Header.Len() + 8 + s.Body.Len()
}

func (s *MultipartRequest) MarshalBinary() (data []byte, err error) {
	s.Header.Length = s.Len()
	data, err = s.Header.MarshalBinary()

	b := make([]byte, 8)
	n := 0
	binary.BigEndian.PutUint16(b[n:], s.Type)
	n += 2
	binary.BigEndian.PutUint16(b[n:], s.Flags)
	n += 2
	n += 4 // for padding
	data = append(data, b...)

	b, err = s.Body.MarshalBinary()
	data = append(data, b...)

	log.Debugf("Sending MultipartRequest (%d): %v", len(data), data)

	return
}

func (s *MultipartRequest) UnmarshalBinary(data []byte) error {
	err := s.Header.UnmarshalBinary(data)
	n := s.Header.Len()

	s.Type = binary.BigEndian.Uint16(data[n:])
	n += 2
	s.Flags = binary.BigEndian.Uint16(data[n:])
	n += 2
	n += 4 // for padding

	var req util.Message
	switch s.Type {
	case MultipartType_Aggregate:
		req = s.Body.(*AggregateStatsRequest)
		err = req.UnmarshalBinary(data[n:])
	case MultipartType_Desc:
		break
	case MultipartType_Flow:
		req = s.Body.(*FlowStatsRequest)
		err = req.UnmarshalBinary(data[n:])
	case MultipartType_Port:
		req = s.Body.(*PortStatsRequest)
		err = req.UnmarshalBinary(data[n:])
	case MultipartType_Table:
		break
	case MultipartType_Queue:
		req = s.Body.(*QueueStatsRequest)
		err = req.UnmarshalBinary(data[n:])
	case MultipartType_Experimenter:
		break
	}
	return err
}

// ofp_multipart_reply 1.3
type MultipartReply struct {
	common.Header
	Type  uint16
	Flags uint16
	pad   []byte // 4 bytes
	Body  []util.Message
}

func (s *MultipartReply) Len() (n uint16) {
	n = s.Header.Len()
	n += 8
	for _, r := range s.Body {
		n += uint16(r.Len())
	}
	return
}

func (s *MultipartReply) MarshalBinary() (data []byte, err error) {
	s.Header.Length = s.Len()
	data, err = s.Header.MarshalBinary()

	b := make([]byte, 8)
	n := 0
	binary.BigEndian.PutUint16(b[n:], s.Type)
	n += 2
	binary.BigEndian.PutUint16(b[n:], s.Flags)
	n += 2
	n += 4 // for padding
	data = append(data, b...)

	for _, r := range s.Body {
		b, err = r.MarshalBinary()
		data = append(data, b...)
	}

	return
}

func (s *MultipartReply) UnmarshalBinary(data []byte) error {
	err := s.Header.UnmarshalBinary(data)
	n := s.Header.Len()

	s.Type = binary.BigEndian.Uint16(data[n:])
	n += 2
	s.Flags = binary.BigEndian.Uint16(data[n:])
	n += 2
	n += 4 // for padding
	var req []util.Message
	for n < s.Header.Length {
		var repl util.Message
		switch s.Type {
		case MultipartType_Aggregate:
			repl = new(AggregateStats)
		case MultipartType_Desc:
			repl = new(DescStats)
		case MultipartType_Flow:
			repl = new(FlowStats)
		case MultipartType_Port:
			repl = new(PortStats)
		case MultipartType_Table:
			repl = new(TableStats)
		case MultipartType_Queue:
			repl = new(QueueStats)
		// FIXME: Support all types
		case MultipartType_Experimenter:
			break
		}

		err = repl.UnmarshalBinary(data[n:])
		if err != nil {
			log.Printf("Error parsing stats reply")
		}
		n += repl.Len()
		req = append(req, repl)

	}

	s.Body = req

	return err
}

// ofp_multipart_request_flags & ofp_multipart_reply_flags 1.3
const (
	OFPMPF_REQ_MORE   = 1 << 0 /* More requests to follow. */
	OFPMPF_REPLY_MORE = 1 << 0 /* More replies to follow. */
)

// _stats_types
const (
	/* Description of this OpenFlow switch.
	 * The request body is empty.
	 * The reply body is struct ofp_desc_stats. */
	MultipartType_Desc = iota

	/* Individual flow statistics.
	 * The request body is struct ofp_flow_stats_request.
	 * The reply body is an array of struct ofp_flow_stats. */
	MultipartType_Flow

	/* Aggregate flow statistics.
	 * The request body is struct ofp_aggregate_stats_request.
	 * The reply body is struct ofp_aggregate_stats_reply. */
	MultipartType_Aggregate

	/* Flow table statistics.
	 * The request body is empty.
	 * The reply body is an array of struct ofp_table_stats. */
	MultipartType_Table

	/* Port statistics.
	 * The request body is struct ofp_port_stats_request.
	 * The reply body is an array of struct ofp_port_stats. */
	MultipartType_Port

	/* Queue statistics for a port
	 * The request body is struct _queue_stats_request.
	 * The reply body is an array of struct ofp_queue_stats */
	MultipartType_Queue

	/* Group counter statistics.
	 * The request body is struct ofp_group_stats_request.
	 * The reply is an array of struct ofp_group_stats. */
	MultipartType_Group

	/* Group description.
	 * The request body is empty.
	 * The reply body is an array of struct ofp_group_desc. */
	MultipartType_GroupDesc

	/* Group features.
	 * The request body is empty.
	 * The reply body is struct ofp_group_features. */
	MultipartType_GroupFeatures

	/* Meter statistics.
	 * The request body is struct ofp_meter_multipart_requests.
	 * The reply body is an array of struct ofp_meter_stats. */
	MultipartType_Meter

	/* Meter configuration.
	 * The request body is struct ofp_meter_multipart_requests.
	 * The reply body is an array of struct ofp_meter_config. */
	MultipartType_MeterConfig

	/* Meter features.
	 * The request body is empty.
	 * The reply body is struct ofp_meter_features. */
	MultipartType_MeterFeatures

	/* Table features.
	 * The request body is either empty or contains an array of
	 * struct ofp_table_features containing the controller’s
	 * desired view of the switch. If the switch is unable to
	 * set the specified view an error is returned.
	 * The reply body is an array of struct ofp_table_features. */
	MultipartType_TableFeatures

	/* Port description.
	 * The request body is empty.
	 * The reply body is an array of struct ofp_port. */
	MultipartType_PortDesc

	/* Experimenter extension.
	 * The request and reply bodies begin with
	 * struct ofp_experimenter_multipart_header.
	 * The request and reply bodies are otherwise experimenter-defined. */
	MultipartType_Experimenter = 0xffff
)

// ofp_desc_stats 1.3
type DescStats struct {
	MfrDesc   []byte // Size DESC_STR_LEN
	HWDesc    []byte // Size DESC_STR_LEN
	SWDesc    []byte // Size DESC_STR_LEN
	SerialNum []byte // Size SERIAL_NUM_LEN
	DPDesc    []byte // Size DESC_STR_LEN
}

func NewDescStats() *DescStats {
	s := new(DescStats)
	s.MfrDesc = make([]byte, DESC_STR_LEN)
	s.HWDesc = make([]byte, DESC_STR_LEN)
	s.SWDesc = make([]byte, DESC_STR_LEN)
	s.SerialNum = make([]byte, SERIAL_NUM_LEN)
	s.DPDesc = make([]byte, DESC_STR_LEN)
	return s
}

func (s *DescStats) Len() (n uint16) {
	return uint16(DESC_STR_LEN*4 + SERIAL_NUM_LEN)
}

func (s *DescStats) MarshalBinary() (data []byte, err error) {
	data = make([]byte, int(s.Len()))
	n := 0
	copy(data[n:], s.MfrDesc)
	n += len(s.MfrDesc)
	copy(data[n:], s.HWDesc)
	n += len(s.HWDesc)
	copy(data[n:], s.SWDesc)
	n += len(s.SWDesc)
	copy(data[n:], s.SerialNum)
	n += len(s.SerialNum)
	copy(data[n:], s.DPDesc)
	n += len(s.DPDesc)
	return
}

func (s *DescStats) UnmarshalBinary(data []byte) error {
	n := 0
	copy(s.MfrDesc, data[n:])
	n += len(s.MfrDesc)
	copy(s.HWDesc, data[n:])
	n += len(s.HWDesc)
	copy(s.SWDesc, data[n:])
	n += len(s.SWDesc)
	copy(s.SerialNum, data[n:])
	n += len(s.SerialNum)
	copy(s.DPDesc, data[n:])
	n += len(s.DPDesc)
	return nil
}

const (
	DESC_STR_LEN   = 256
	SERIAL_NUM_LEN = 32
)

const (
	OFPTT_MAX = 0xfe
	/* Fake tables. */
	OFPTT_ALL = 0xff /* Wildcard table used for table config, flow stats and flow deletes. */
)

// ofp_flow_stats_request 1.3
type FlowStatsRequest struct {
	TableId    uint8
	pad        []byte // 3 bytes
	OutPort    uint32
	OutGroup   uint32
	pad2       []byte // 4 bytes
	Cookie     uint64
	CookieMask uint64
	Match      Match
}

func NewFlowStatsRequest() *FlowStatsRequest {
	s := new(FlowStatsRequest)
	s.OutPort = P_ANY
	s.OutGroup = OFPG_ANY
	s.pad = make([]byte, 3)
	s.pad2 = make([]byte, 4)
	s.Match = *NewMatch()
	return s
}

func (s *FlowStatsRequest) Len() (n uint16) {
	return s.Match.Len() + 32
}

func (s *FlowStatsRequest) MarshalBinary() (data []byte, err error) {
	data = make([]byte, 32)
	n := 0
	data[n] = s.TableId
	n += 1
	copy(data[n:], s.pad)
	n += 3
	binary.BigEndian.PutUint32(data[n:], s.OutPort)
	n += 4
	binary.BigEndian.PutUint32(data[n:], s.OutGroup)
	n += 4
	copy(data[n:], s.pad2)
	n += 4
	binary.BigEndian.PutUint64(data[n:], s.Cookie)
	n += 8
	binary.BigEndian.PutUint64(data[n:], s.CookieMask)
	n += 8

	b, err := s.Match.MarshalBinary()
	data = append(data, b...)
	return
}

func (s *FlowStatsRequest) UnmarshalBinary(data []byte) error {
	n := 0
	s.TableId = data[n]
	n += 1
	copy(s.pad, data[n:n+3])
	n += 3
	s.OutPort = binary.BigEndian.Uint32(data[n:])
	n += 4
	s.OutGroup = binary.BigEndian.Uint32(data[n:])
	n += 4
	copy(s.pad2, data[n:n+4])
	n += 4
	s.Cookie = binary.BigEndian.Uint64(data[n:])
	n += 8
	s.CookieMask = binary.BigEndian.Uint64(data[n:])
	n += 8

	err := s.Match.UnmarshalBinary(data[n:])
	n += int(s.Match.Len())

	return err
}

// ofp_flow_stats 1.3
type FlowStats struct {
	Length       uint16
	TableId      uint8
	pad          uint8
	DurationSec  uint32
	DurationNSec uint32
	Priority     uint16
	IdleTimeout  uint16
	HardTimeout  uint16
	Flags        uint16
	pad2         []uint8 // Size 4
	Cookie       uint64
	PacketCount  uint64
	ByteCount    uint64
	Match        Match
	Instructions []Instruction
}

func NewFlowStats() *FlowStats {
	f := new(FlowStats)
	f.Match = *NewMatch()
	f.pad2 = make([]byte, 4)
	f.Instructions = make([]Instruction, 0)
	return f
}

func (s *FlowStats) Len() (n uint16) {
	n = 48 + s.Match.Len()
	for _, instr := range s.Instructions {
		n += instr.Len()
	}
	return
}

func (s *FlowStats) MarshalBinary() (data []byte, err error) {
	data = make([]byte, 48)
	n := 0

	binary.BigEndian.PutUint16(data[n:], s.Length)
	n += 2
	data[n] = s.TableId
	n += 1
	data[n] = s.pad
	n += 1

	binary.BigEndian.PutUint32(data[n:], s.DurationSec)
	n += 4
	binary.BigEndian.PutUint32(data[n:], s.DurationNSec)
	n += 4
	binary.BigEndian.PutUint16(data[n:], s.Priority)
	n += 2
	binary.BigEndian.PutUint16(data[n:], s.IdleTimeout)
	n += 2
	binary.BigEndian.PutUint16(data[n:], s.HardTimeout)
	n += 2
	binary.BigEndian.PutUint16(data[n:], s.Flags)
	n += 2
	copy(data[n:], s.pad2)
	n += len(s.pad2)
	binary.BigEndian.PutUint64(data[n:], s.Cookie)
	n += 8
	binary.BigEndian.PutUint64(data[n:], s.PacketCount)
	n += 8
	binary.BigEndian.PutUint64(data[n:], s.ByteCount)
	n += 8

	b, err := s.Match.MarshalBinary()
	data = append(data, b...)
	n += len(b)

	for _, instr := range s.Instructions {
		b, err = instr.MarshalBinary()
		data = append(data, b...)
		n += len(b)
	}
	return
}

func (s *FlowStats) UnmarshalBinary(data []byte) error {
	n := 0
	s.Length = binary.BigEndian.Uint16(data[n:])
	n += 2
	s.TableId = data[n]
	n += 1
	s.pad = data[n]
	n += 1
	s.DurationSec = binary.BigEndian.Uint32(data[n:])
	n += 4
	s.DurationNSec = binary.BigEndian.Uint32(data[n:])
	n += 4
	s.Priority = binary.BigEndian.Uint16(data[n:])
	n += 2
	s.IdleTimeout = binary.BigEndian.Uint16(data[n:])
	n += 2
	s.HardTimeout = binary.BigEndian.Uint16(data[n:])
	n += 2
	s.Flags = binary.BigEndian.Uint16(data[n:])
	n += 2
	copy(s.pad2, data[n:n+4])
	n += 4
	s.Cookie = binary.BigEndian.Uint64(data[n:])
	n += 8
	s.PacketCount = binary.BigEndian.Uint64(data[n:])
	n += 8
	s.ByteCount = binary.BigEndian.Uint64(data[n:])
	n += 8
	err := s.Match.UnmarshalBinary(data[n:])
	n += int(s.Match.Len())

	for n < int(s.Length) {
		instr := DecodeInstr(data[n:])
		s.Instructions = append(s.Instructions, instr)
		n += int(instr.Len())
	}
	return err
}

// ofp_aggregate_stats_request 1.3
type AggregateStatsRequest struct {
	TableId    uint8
	pad        []byte // 3 bytes
	OutPort    uint32
	OutGroup   uint32
	pad2       []byte // 4 bytes
	Cookie     uint64
	CookieMask uint64
	Match
}

func NewAggregateStatsRequest() *AggregateStatsRequest {
	a := new(AggregateStatsRequest)
	a.pad = make([]byte, 3)
	a.pad2 = make([]byte, 4)
	a.Match = *NewMatch()

	return a
}

func (s *AggregateStatsRequest) Len() (n uint16) {
	return s.Match.Len() + 32
}

func (s *AggregateStatsRequest) MarshalBinary() (data []byte, err error) {
	data = make([]byte, 32)
	n := 0
	data[n] = s.TableId
	n += 1
	copy(data[n:], s.pad)
	n += 3
	binary.BigEndian.PutUint32(data[n:], s.OutPort)
	n += 4
	binary.BigEndian.PutUint32(data[n:], s.OutGroup)
	n += 4
	copy(data[n:], s.pad2)
	n += 4
	binary.BigEndian.PutUint64(data[n:], s.Cookie)
	n += 8
	binary.BigEndian.PutUint64(data[n:], s.CookieMask)
	n += 8

	b, err := s.Match.MarshalBinary()
	data = append(data, b...)
	return
}

func (s *AggregateStatsRequest) UnmarshalBinary(data []byte) error {
	n := 0
	s.TableId = data[n]
	n += 1
	copy(s.pad, data[n:n+3])
	n += 3
	s.OutPort = binary.BigEndian.Uint32(data[n:])
	n += 4
	s.OutGroup = binary.BigEndian.Uint32(data[n:])
	n += 4
	copy(s.pad2, data[n:n+4])
	n += 4
	s.Cookie = binary.BigEndian.Uint64(data[n:])
	n += 8
	s.CookieMask = binary.BigEndian.Uint64(data[n:])
	n += 8

	s.Match.UnmarshalBinary(data[n:])
	n += int(s.Match.Len())
	return nil
}

// ofp_aggregate_stats_reply 1.3
type AggregateStats struct {
	PacketCount uint64
	ByteCount   uint64
	FlowCount   uint32
	pad         []uint8 // Size 4
}

func NewAggregateStats() *AggregateStats {
	s := new(AggregateStats)
	s.pad = make([]byte, 4)
	return s
}

func (s *AggregateStats) Len() (n uint16) {
	return 24
}

func (s *AggregateStats) MarshalBinary() (data []byte, err error) {
	data = make([]byte, int(s.Len()))
	n := 0
	binary.BigEndian.PutUint64(data[n:], s.PacketCount)
	n += 8
	binary.BigEndian.PutUint64(data[n:], s.ByteCount)
	n += 8
	binary.BigEndian.PutUint32(data[n:], s.FlowCount)
	n += 4
	copy(data[n:], s.pad)
	n += 4
	return
}

func (s *AggregateStats) UnmarshalBinary(data []byte) error {
	n := 0
	s.PacketCount = binary.BigEndian.Uint64(data[n:])
	n += 8
	s.ByteCount = binary.BigEndian.Uint64(data[n:])
	n += 8
	s.FlowCount = binary.BigEndian.Uint32(data[n:])
	n += 4
	copy(s.pad, data[n:])
	return nil
}

// FIXME: Everything below this needs to be changed for ofp1.3
// ofp_table_stats 1.0
type TableStats struct {
	TableId      uint8
	pad          []uint8 // Size 3
	Name         []byte  // Size MAX_TABLE_NAME_LEN
	Wildcards    uint32
	MaxEntries   uint32
	ActiveCount  uint32
	LookupCount  uint64
	MatchedCount uint64
}

func NewTableStats() *TableStats {
	s := new(TableStats)
	s.pad = make([]byte, 3)
	s.Name = make([]byte, MAX_TABLE_NAME_LEN)
	return s
}

func (s *TableStats) Len() (n uint16) {
	return 4 + MAX_TABLE_NAME_LEN + 28
}

func (s *TableStats) MarshalBinary() (data []byte, err error) {
	data = make([]byte, int(s.Len()))
	n := 0
	data[n] = s.TableId
	n += 1
	copy(data[n:], s.pad)
	n += len(s.pad)
	copy(data[n:], s.Name)
	n += len(s.Name)
	binary.BigEndian.PutUint32(data[n:], s.Wildcards)
	n += 4
	binary.BigEndian.PutUint32(data[n:], s.MaxEntries)
	n += 4
	binary.BigEndian.PutUint32(data[n:], s.ActiveCount)
	n += 4
	binary.BigEndian.PutUint64(data[n:], s.LookupCount)
	n += 8
	binary.BigEndian.PutUint64(data[n:], s.MatchedCount)
	n += 8
	return
}

func (s *TableStats) UnmarshalBinary(data []byte) error {
	n := 0
	s.TableId = data[0]
	n += 1
	copy(s.pad, data[n:])
	n += len(s.pad)
	copy(s.Name, data[n:])
	n += len(s.Name)
	s.Wildcards = binary.BigEndian.Uint32(data[n:])
	n += 4
	s.MaxEntries = binary.BigEndian.Uint32(data[n:])
	n += 4
	s.ActiveCount = binary.BigEndian.Uint32(data[n:])
	n += 4
	s.LookupCount = binary.BigEndian.Uint64(data[n:])
	n += 8
	s.MatchedCount = binary.BigEndian.Uint64(data[n:])
	n += 8
	return nil
}

const (
	MAX_TABLE_NAME_LEN = 32
)

// ofp_port_stats_request 1.0
type PortStatsRequest struct {
	PortNo uint16
	pad    []uint8 // Size 6
}

func NewPortStatsRequest() *PortStatsRequest {
	p := new(PortStatsRequest)
	p.pad = make([]byte, 6)
	return p
}

func (s *PortStatsRequest) Len() (n uint16) {
	return 8
}

func (s *PortStatsRequest) MarshalBinary() (data []byte, err error) {
	data = make([]byte, int(s.Len()))
	n := 0
	binary.BigEndian.PutUint16(data[n:], s.PortNo)
	n += 2
	copy(data[n:], s.pad)
	n += len(s.pad)
	return
}

func (s *PortStatsRequest) UnmarshalBinary(data []byte) error {
	n := 0
	s.PortNo = binary.BigEndian.Uint16(data[n:])
	n += 2
	copy(s.pad, data[n:])
	n += len(s.pad)
	return nil
}

// ofp_port_stats 1.0
type PortStats struct {
	PortNo     uint16
	pad        []uint8 // Size 6
	RxPackets  uint64
	TxPackets  uint64
	RxBytes    uint64
	TxBytes    uint64
	RxDropped  uint64
	TxDropped  uint64
	RxErrors   uint64
	TxErrors   uint64
	RxFrameErr uint64
	RxOverErr  uint64
	RxCRCErr   uint64
	Collisions uint64
}

func NewPortStats() *PortStats {
	p := new(PortStats)
	p.pad = make([]byte, 6)
	return p
}

func (s *PortStats) Len() (n uint16) {
	return 104
}

func (s *PortStats) MarshalBinary() (data []byte, err error) {
	data = make([]byte, int(s.Len()))
	n := 0
	binary.BigEndian.PutUint16(data[n:], s.PortNo)
	n += 2
	copy(data[n:], s.pad)
	n += len(s.pad)
	binary.BigEndian.PutUint64(data[n:], s.RxPackets)
	n += 8
	binary.BigEndian.PutUint64(data[n:], s.TxPackets)
	n += 8
	binary.BigEndian.PutUint64(data[n:], s.RxBytes)
	n += 8
	binary.BigEndian.PutUint64(data[n:], s.TxBytes)
	n += 8
	binary.BigEndian.PutUint64(data[n:], s.RxDropped)
	n += 8
	binary.BigEndian.PutUint64(data[n:], s.TxDropped)
	n += 8
	binary.BigEndian.PutUint64(data[n:], s.RxErrors)
	n += 8
	binary.BigEndian.PutUint64(data[n:], s.TxErrors)
	n += 8
	binary.BigEndian.PutUint64(data[n:], s.RxFrameErr)
	n += 8
	binary.BigEndian.PutUint64(data[n:], s.RxOverErr)
	n += 8
	binary.BigEndian.PutUint64(data[n:], s.RxCRCErr)
	n += 8
	binary.BigEndian.PutUint64(data[n:], s.Collisions)
	n += 8
	return
}

func (s *PortStats) UnmarshalBinary(data []byte) error {
	n := 0
	s.PortNo = binary.BigEndian.Uint16(data[n:])
	n += 2
	copy(s.pad, data[n:])
	n += len(s.pad)
	s.RxPackets = binary.BigEndian.Uint64(data[n:])
	n += 8
	s.TxPackets = binary.BigEndian.Uint64(data[n:])
	n += 8
	s.RxBytes = binary.BigEndian.Uint64(data[n:])
	n += 8
	s.TxBytes = binary.BigEndian.Uint64(data[n:])
	n += 8
	s.RxDropped = binary.BigEndian.Uint64(data[n:])
	n += 8
	s.TxDropped = binary.BigEndian.Uint64(data[n:])
	n += 8
	s.RxErrors = binary.BigEndian.Uint64(data[n:])
	n += 8
	s.TxErrors = binary.BigEndian.Uint64(data[n:])
	n += 8
	s.RxFrameErr = binary.BigEndian.Uint64(data[n:])
	n += 8
	s.RxOverErr = binary.BigEndian.Uint64(data[n:])
	n += 8
	s.RxCRCErr = binary.BigEndian.Uint64(data[n:])
	n += 8
	s.Collisions = binary.BigEndian.Uint64(data[n:])
	n += 8
	return nil
}

// ofp_queue_stats_request 1.0
type QueueStatsRequest struct {
	PortNo  uint16
	pad     []uint8 // Size 2
	QueueId uint32
}

func NewQueueStatsRequest() *QueueStatsRequest {
	q := new(QueueStatsRequest)
	q.pad = make([]byte, 2)
	return q
}

func (s *QueueStatsRequest) Len() (n uint16) {
	return 8
}

func (s *QueueStatsRequest) MarshalBinary() (data []byte, err error) {
	data = make([]byte, int(s.Len()))
	n := 0
	binary.BigEndian.PutUint16(data[n:], s.PortNo)
	n += 2
	copy(data[n:], s.pad)
	n += 2
	binary.BigEndian.PutUint32(data[n:], s.QueueId)
	n += 4
	return
}

func (s *QueueStatsRequest) UnmarshalBinary(data []byte) error {
	n := 0
	s.PortNo = binary.BigEndian.Uint16(data[n:])
	n += 2
	copy(s.pad, data[n:])
	n += 2
	s.QueueId = binary.BigEndian.Uint32(data[n:])
	return nil
}

// ofp_queue_stats 1.0
type QueueStats struct {
	PortNo    uint16
	pad       []uint8 // Size 2
	QueueId   uint32
	TxBytes   uint64
	TxPackets uint64
	TxErrors  uint64
}

func (s *QueueStats) Len() (n uint16) {
	return 32
}

func (s *QueueStats) MarshalBinary() (data []byte, err error) {
	data = make([]byte, 32)
	n := 0

	binary.BigEndian.PutUint16(data[n:], s.PortNo)
	n += 2
	copy(data[n:], s.pad)
	n += 2
	binary.BigEndian.PutUint32(data[n:], s.QueueId)
	n += 4
	binary.BigEndian.PutUint64(data[n:], s.TxBytes)
	n += 8
	binary.BigEndian.PutUint64(data[n:], s.TxPackets)
	n += 8
	binary.BigEndian.PutUint64(data[n:], s.TxErrors)
	n += 8
	return
}

func (s *QueueStats) UnmarshalBinary(data []byte) error {
	n := 0
	s.PortNo = binary.BigEndian.Uint16(data[n:])
	n += 2
	copy(s.pad, data[n:])
	n += len(s.pad)
	s.QueueId = binary.BigEndian.Uint32(data[n:])
	n += 4
	s.TxBytes = binary.BigEndian.Uint64(data[n:])
	n += 8
	s.TxPackets = binary.BigEndian.Uint64(data[n:])
	n += 8
	s.TxErrors = binary.BigEndian.Uint64(data[n:])
	n += 8
	return nil
}

// ofp_port_status
type PortStatus struct {
	common.Header
	Reason uint8
	pad    []uint8 // Size 7
	Desc   PhyPort
}

func NewPortStatus() *PortStatus {
	p := new(PortStatus)
	p.Header = NewOfp13Header()
	p.pad = make([]byte, 7)
	return p
}

func (p *PortStatus) Len() (n uint16) {
	n = p.Header.Len()
	n += 8
	n += p.Desc.Len()
	return
}

func (s *PortStatus) MarshalBinary() (data []byte, err error) {
	s.Header.Length = s.Len()
	data, err = s.Header.MarshalBinary()

	b := make([]byte, 8)
	n := 0
	b[0] = s.Reason
	n += 1
	copy(b[n:], s.pad)
	data = append(data, b...)

	b, err = s.Desc.MarshalBinary()
	data = append(data, b...)
	return
}

func (s *PortStatus) UnmarshalBinary(data []byte) error {
	err := s.Header.UnmarshalBinary(data)
	n := int(s.Header.Len())

	s.Reason = data[n]
	n += 1
	copy(s.pad, data[n:])
	n += len(s.pad)

	err = s.Desc.UnmarshalBinary(data[n:])
	return err
}

// ofp_port_reason 1.0
const (
	PR_ADD = iota
	PR_DELETE
	PR_MODIFY
)
