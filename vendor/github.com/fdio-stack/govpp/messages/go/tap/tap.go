// Package tap provides the Go interface to VPP binary API of the tap VPP module.
// Generated from 'tap.api.json' on Fri, 17 Mar 2017 17:11:50 UTC.
package tap

// VlApiVersion contains version of the API.
const VlAPIVersion = 0x1aedb9f2

// TapConnect is the Go representation of the VPP binary API message 'tap_connect'.
type TapConnect struct {
	UseRandomMac      uint8
	TapName           []byte `struc:"[64]byte"`
	MacAddress        []byte `struc:"[6]byte"`
	Renumber          uint8
	CustomDevInstance uint32
	IP4AddressSet     uint8
	IP4Address        []byte `struc:"[4]byte"`
	IP4MaskWidth      uint8
	IP6AddressSet     uint8
	IP6Address        []byte `struc:"[16]byte"`
	IP6MaskWidth      uint8
	Tag               []byte `struc:"[64]byte"`
}

func (*TapConnect) GetMessageName() string {
	return "tap_connect"
}
func (*TapConnect) GetCrcString() string {
	return "91720de3"
}

// TapConnectReply is the Go representation of the VPP binary API message 'tap_connect_reply'.
type TapConnectReply struct {
	Retval    int32
	SwIfIndex uint32
}

func (*TapConnectReply) GetMessageName() string {
	return "tap_connect_reply"
}
func (*TapConnectReply) GetCrcString() string {
	return "f47feac1"
}

// TapModify is the Go representation of the VPP binary API message 'tap_modify'.
type TapModify struct {
	SwIfIndex         uint32
	UseRandomMac      uint8
	TapName           []byte `struc:"[64]byte"`
	MacAddress        []byte `struc:"[6]byte"`
	Renumber          uint8
	CustomDevInstance uint32
}

func (*TapModify) GetMessageName() string {
	return "tap_modify"
}
func (*TapModify) GetCrcString() string {
	return "8abcd5f3"
}

// TapModifyReply is the Go representation of the VPP binary API message 'tap_modify_reply'.
type TapModifyReply struct {
	Retval    int32
	SwIfIndex uint32
}

func (*TapModifyReply) GetMessageName() string {
	return "tap_modify_reply"
}
func (*TapModifyReply) GetCrcString() string {
	return "00aaf940"
}

// TapDelete is the Go representation of the VPP binary API message 'tap_delete'.
type TapDelete struct {
	SwIfIndex uint32
}

func (*TapDelete) GetMessageName() string {
	return "tap_delete"
}
func (*TapDelete) GetCrcString() string {
	return "e99d41c1"
}

// TapDeleteReply is the Go representation of the VPP binary API message 'tap_delete_reply'.
type TapDeleteReply struct {
	Retval int32
}

func (*TapDeleteReply) GetMessageName() string {
	return "tap_delete_reply"
}
func (*TapDeleteReply) GetCrcString() string {
	return "0e47d140"
}

// SwInterfaceTapDump is the Go representation of the VPP binary API message 'sw_interface_tap_dump'.
type SwInterfaceTapDump struct {
}

func (*SwInterfaceTapDump) GetMessageName() string {
	return "sw_interface_tap_dump"
}
func (*SwInterfaceTapDump) GetCrcString() string {
	return "bc6ddbe2"
}

// SwInterfaceTapDetails is the Go representation of the VPP binary API message 'sw_interface_tap_details'.
type SwInterfaceTapDetails struct {
	SwIfIndex uint32
	DevName   []byte `struc:"[64]byte"`
}

func (*SwInterfaceTapDetails) GetMessageName() string {
	return "sw_interface_tap_details"
}
func (*SwInterfaceTapDetails) GetCrcString() string {
	return "0df07bc3"
}
