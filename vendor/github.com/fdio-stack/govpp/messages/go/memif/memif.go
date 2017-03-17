// Package memif provides the Go interface to VPP binary API of the memif VPP module.
// Generated from 'memif.api.json' on Fri, 17 Mar 2017 17:11:50 UTC.
package memif

// VlApiVersion contains version of the API.
const VlAPIVersion = 0x93b504b3

// MemifCreate is the Go representation of the VPP binary API message 'memif_create'.
type MemifCreate struct {
	Role           uint8
	Key            uint64
	SocketFilename []byte `struc:"[128]byte"`
	RingSize       uint32
	HwAddr         []byte `struc:"[6]byte"`
}

func (*MemifCreate) GetMessageName() string {
	return "memif_create"
}
func (*MemifCreate) GetCrcString() string {
	return "d407f0c7"
}

// MemifCreateReply is the Go representation of the VPP binary API message 'memif_create_reply'.
type MemifCreateReply struct {
	Retval    int32
	SwIfIndex uint32
}

func (*MemifCreateReply) GetMessageName() string {
	return "memif_create_reply"
}
func (*MemifCreateReply) GetCrcString() string {
	return "93d7498b"
}

// MemifDelete is the Go representation of the VPP binary API message 'memif_delete'.
type MemifDelete struct {
	SwIfIndex uint32
}

func (*MemifDelete) GetMessageName() string {
	return "memif_delete"
}
func (*MemifDelete) GetCrcString() string {
	return "12814e3d"
}

// MemifDeleteReply is the Go representation of the VPP binary API message 'memif_delete_reply'.
type MemifDeleteReply struct {
	Retval int32
}

func (*MemifDeleteReply) GetMessageName() string {
	return "memif_delete_reply"
}
func (*MemifDeleteReply) GetCrcString() string {
	return "72c9fa3c"
}

// MemifDetails is the Go representation of the VPP binary API message 'memif_details'.
type MemifDetails struct {
	SwIfIndex      uint32
	IfName         []byte `struc:"[64]byte"`
	HwAddr         []byte `struc:"[6]byte"`
	Key            uint64
	Role           uint8
	SocketFilename []byte `struc:"[128]byte"`
	RingSize       uint32
	AdminUpDown    uint8
	LinkUpDown     uint8
}

func (*MemifDetails) GetMessageName() string {
	return "memif_details"
}
func (*MemifDetails) GetCrcString() string {
	return "5b4aada8"
}

// MemifDump is the Go representation of the VPP binary API message 'memif_dump'.
type MemifDump struct {
}

func (*MemifDump) GetMessageName() string {
	return "memif_dump"
}
func (*MemifDump) GetCrcString() string {
	return "68d39e95"
}
