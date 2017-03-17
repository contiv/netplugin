// Package interfaces provides the Go interface to VPP binary API of the interfaces VPP module.
// Generated from 'interface.api.json' on Fri, 17 Mar 2017 17:11:50 UTC.
package interfaces

// VlApiVersion contains version of the API.
const VlAPIVersion = 0x92be38ed

// SwInterfaceSetFlags is the Go representation of the VPP binary API message 'sw_interface_set_flags'.
type SwInterfaceSetFlags struct {
	SwIfIndex   uint32
	AdminUpDown uint8
	LinkUpDown  uint8
	Deleted     uint8
}

func (*SwInterfaceSetFlags) GetMessageName() string {
	return "sw_interface_set_flags"
}
func (*SwInterfaceSetFlags) GetCrcString() string {
	return "c230f9b1"
}

// SwInterfaceSetFlagsReply is the Go representation of the VPP binary API message 'sw_interface_set_flags_reply'.
type SwInterfaceSetFlagsReply struct {
	Retval int32
}

func (*SwInterfaceSetFlagsReply) GetMessageName() string {
	return "sw_interface_set_flags_reply"
}
func (*SwInterfaceSetFlagsReply) GetCrcString() string {
	return "dfbf3afa"
}

// SwInterfaceSetMtu is the Go representation of the VPP binary API message 'sw_interface_set_mtu'.
type SwInterfaceSetMtu struct {
	SwIfIndex uint32
	Mtu       uint16
}

func (*SwInterfaceSetMtu) GetMessageName() string {
	return "sw_interface_set_mtu"
}
func (*SwInterfaceSetMtu) GetCrcString() string {
	return "535dab1d"
}

// SwInterfaceSetMtuReply is the Go representation of the VPP binary API message 'sw_interface_set_mtu_reply'.
type SwInterfaceSetMtuReply struct {
	Retval int32
}

func (*SwInterfaceSetMtuReply) GetMessageName() string {
	return "sw_interface_set_mtu_reply"
}
func (*SwInterfaceSetMtuReply) GetCrcString() string {
	return "0cc22552"
}

// WantInterfaceEvents is the Go representation of the VPP binary API message 'want_interface_events'.
type WantInterfaceEvents struct {
	EnableDisable uint32
	Pid           uint32
}

func (*WantInterfaceEvents) GetMessageName() string {
	return "want_interface_events"
}
func (*WantInterfaceEvents) GetCrcString() string {
	return "a0cbf57e"
}

// WantInterfaceEventsReply is the Go representation of the VPP binary API message 'want_interface_events_reply'.
type WantInterfaceEventsReply struct {
	Retval int32
}

func (*WantInterfaceEventsReply) GetMessageName() string {
	return "want_interface_events_reply"
}
func (*WantInterfaceEventsReply) GetCrcString() string {
	return "33788c73"
}

// SwInterfaceDetails is the Go representation of the VPP binary API message 'sw_interface_details'.
type SwInterfaceDetails struct {
	SwIfIndex         uint32
	SupSwIfIndex      uint32
	L2AddressLength   uint32
	L2Address         []byte `struc:"[8]byte"`
	InterfaceName     []byte `struc:"[64]byte"`
	AdminUpDown       uint8
	LinkUpDown        uint8
	LinkDuplex        uint8
	LinkSpeed         uint8
	LinkMtu           uint16
	SubID             uint32
	SubDot1ad         uint8
	SubNumberOfTags   uint8
	SubOuterVlanID    uint16
	SubInnerVlanID    uint16
	SubExactMatch     uint8
	SubDefault        uint8
	SubOuterVlanIDAny uint8
	SubInnerVlanIDAny uint8
	VtrOp             uint32
	VtrPushDot1q      uint32
	VtrTag1           uint32
	VtrTag2           uint32
	Tag               []byte `struc:"[64]byte"`
}

func (*SwInterfaceDetails) GetMessageName() string {
	return "sw_interface_details"
}
func (*SwInterfaceDetails) GetCrcString() string {
	return "be58e53e"
}

// SwInterfaceDump is the Go representation of the VPP binary API message 'sw_interface_dump'.
type SwInterfaceDump struct {
	NameFilterValid uint8
	NameFilter      []byte `struc:"[49]byte"`
}

func (*SwInterfaceDump) GetMessageName() string {
	return "sw_interface_dump"
}
func (*SwInterfaceDump) GetCrcString() string {
	return "9a2f9d4d"
}

// SwInterfaceAddDelAddress is the Go representation of the VPP binary API message 'sw_interface_add_del_address'.
type SwInterfaceAddDelAddress struct {
	SwIfIndex     uint32
	IsAdd         uint8
	IsIpv6        uint8
	DelAll        uint8
	AddressLength uint8
	Address       []byte `struc:"[16]byte"`
}

func (*SwInterfaceAddDelAddress) GetMessageName() string {
	return "sw_interface_add_del_address"
}
func (*SwInterfaceAddDelAddress) GetCrcString() string {
	return "4e24d2df"
}

// SwInterfaceAddDelAddressReply is the Go representation of the VPP binary API message 'sw_interface_add_del_address_reply'.
type SwInterfaceAddDelAddressReply struct {
	Retval int32
}

func (*SwInterfaceAddDelAddressReply) GetMessageName() string {
	return "sw_interface_add_del_address_reply"
}
func (*SwInterfaceAddDelAddressReply) GetCrcString() string {
	return "abe29452"
}

// SwInterfaceSetTable is the Go representation of the VPP binary API message 'sw_interface_set_table'.
type SwInterfaceSetTable struct {
	SwIfIndex uint32
	IsIpv6    uint8
	VrfID     uint32
}

func (*SwInterfaceSetTable) GetMessageName() string {
	return "sw_interface_set_table"
}
func (*SwInterfaceSetTable) GetCrcString() string {
	return "a94df510"
}

// SwInterfaceSetTableReply is the Go representation of the VPP binary API message 'sw_interface_set_table_reply'.
type SwInterfaceSetTableReply struct {
	Retval int32
}

func (*SwInterfaceSetTableReply) GetMessageName() string {
	return "sw_interface_set_table_reply"
}
func (*SwInterfaceSetTableReply) GetCrcString() string {
	return "99df273c"
}

// SwInterfaceGetTable is the Go representation of the VPP binary API message 'sw_interface_get_table'.
type SwInterfaceGetTable struct {
	SwIfIndex uint32
	IsIpv6    uint8
}

func (*SwInterfaceGetTable) GetMessageName() string {
	return "sw_interface_get_table"
}
func (*SwInterfaceGetTable) GetCrcString() string {
	return "f5a1d557"
}

// SwInterfaceGetTableReply is the Go representation of the VPP binary API message 'sw_interface_get_table_reply'.
type SwInterfaceGetTableReply struct {
	Retval int32
	VrfID  uint32
}

func (*SwInterfaceGetTableReply) GetMessageName() string {
	return "sw_interface_get_table_reply"
}
func (*SwInterfaceGetTableReply) GetCrcString() string {
	return "ab44111d"
}

// VnetInterfaceCounters is the Go representation of the VPP binary API message 'vnet_interface_counters'.
type VnetInterfaceCounters struct {
	VnetCounterType uint8
	IsCombined      uint8
	FirstSwIfIndex  uint32
	Count           uint32 `struc:"sizeof=Data"`
	Data            []byte
}

func (*VnetInterfaceCounters) GetMessageName() string {
	return "vnet_interface_counters"
}
func (*VnetInterfaceCounters) GetCrcString() string {
	return "312082b4"
}

// SwInterfaceSetUnnumbered is the Go representation of the VPP binary API message 'sw_interface_set_unnumbered'.
type SwInterfaceSetUnnumbered struct {
	SwIfIndex           uint32
	UnnumberedSwIfIndex uint32
	IsAdd               uint8
}

func (*SwInterfaceSetUnnumbered) GetMessageName() string {
	return "sw_interface_set_unnumbered"
}
func (*SwInterfaceSetUnnumbered) GetCrcString() string {
	return "ee0047b0"
}

// SwInterfaceSetUnnumberedReply is the Go representation of the VPP binary API message 'sw_interface_set_unnumbered_reply'.
type SwInterfaceSetUnnumberedReply struct {
	Retval int32
}

func (*SwInterfaceSetUnnumberedReply) GetMessageName() string {
	return "sw_interface_set_unnumbered_reply"
}
func (*SwInterfaceSetUnnumberedReply) GetCrcString() string {
	return "5b2275e1"
}

// SwInterfaceClearStats is the Go representation of the VPP binary API message 'sw_interface_clear_stats'.
type SwInterfaceClearStats struct {
	SwIfIndex uint32
}

func (*SwInterfaceClearStats) GetMessageName() string {
	return "sw_interface_clear_stats"
}
func (*SwInterfaceClearStats) GetCrcString() string {
	return "9600fd50"
}

// SwInterfaceClearStatsReply is the Go representation of the VPP binary API message 'sw_interface_clear_stats_reply'.
type SwInterfaceClearStatsReply struct {
	Retval int32
}

func (*SwInterfaceClearStatsReply) GetMessageName() string {
	return "sw_interface_clear_stats_reply"
}
func (*SwInterfaceClearStatsReply) GetCrcString() string {
	return "21f50dd9"
}

// SwInterfaceTagAddDel is the Go representation of the VPP binary API message 'sw_interface_tag_add_del'.
type SwInterfaceTagAddDel struct {
	IsAdd     uint8
	SwIfIndex uint32
	Tag       []byte `struc:"[64]byte"`
}

func (*SwInterfaceTagAddDel) GetMessageName() string {
	return "sw_interface_tag_add_del"
}
func (*SwInterfaceTagAddDel) GetCrcString() string {
	return "50ae8d92"
}

// SwInterfaceTagAddDelReply is the Go representation of the VPP binary API message 'sw_interface_tag_add_del_reply'.
type SwInterfaceTagAddDelReply struct {
	Retval int32
}

func (*SwInterfaceTagAddDelReply) GetMessageName() string {
	return "sw_interface_tag_add_del_reply"
}
func (*SwInterfaceTagAddDelReply) GetCrcString() string {
	return "761cbcb0"
}
