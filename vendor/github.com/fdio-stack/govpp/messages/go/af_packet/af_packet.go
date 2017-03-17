// Package af_packet provides the Go interface to VPP binary API of the af_packet VPP module.
// Generated from 'af_packet.api.json' on Fri, 17 Mar 2017 17:11:50 UTC.
package af_packet

// VlApiVersion contains version of the API.
const VlAPIVersion = 0x4ca71f33

// AfPacketCreate is the Go representation of the VPP binary API message 'af_packet_create'.
type AfPacketCreate struct {
	HostIfName      []byte `struc:"[64]byte"`
	HwAddr          []byte `struc:"[6]byte"`
	UseRandomHwAddr uint8
}

func (*AfPacketCreate) GetMessageName() string {
	return "af_packet_create"
}
func (*AfPacketCreate) GetCrcString() string {
	return "92768640"
}

// AfPacketCreateReply is the Go representation of the VPP binary API message 'af_packet_create_reply'.
type AfPacketCreateReply struct {
	Retval    int32
	SwIfIndex uint32
}

func (*AfPacketCreateReply) GetMessageName() string {
	return "af_packet_create_reply"
}
func (*AfPacketCreateReply) GetCrcString() string {
	return "718bac92"
}

// AfPacketDelete is the Go representation of the VPP binary API message 'af_packet_delete'.
type AfPacketDelete struct {
	HostIfName []byte `struc:"[64]byte"`
}

func (*AfPacketDelete) GetMessageName() string {
	return "af_packet_delete"
}
func (*AfPacketDelete) GetCrcString() string {
	return "c063ce85"
}

// AfPacketDeleteReply is the Go representation of the VPP binary API message 'af_packet_delete_reply'.
type AfPacketDeleteReply struct {
	Retval int32
}

func (*AfPacketDeleteReply) GetMessageName() string {
	return "af_packet_delete_reply"
}
func (*AfPacketDeleteReply) GetCrcString() string {
	return "1a80431a"
}
