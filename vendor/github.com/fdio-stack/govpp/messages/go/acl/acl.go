// Package acl provides the Go interface to VPP binary API of the acl VPP module.
// Generated from 'acl.api.json' on Fri, 17 Mar 2017 17:11:50 UTC.
package acl

// VlApiVersion contains version of the API.
const VlAPIVersion = 0x3cd02d84

// ACLRule is the Go representation of the VPP binary API data type 'acl_rule'.
type ACLRule struct {
	IsPermit               uint8
	IsIpv6                 uint8
	SrcIPAddr              []byte `struc:"[16]byte"`
	SrcIPPrefixLen         uint8
	DstIPAddr              []byte `struc:"[16]byte"`
	DstIPPrefixLen         uint8
	Proto                  uint8
	SrcportOrIcmptypeFirst uint16
	SrcportOrIcmptypeLast  uint16
	DstportOrIcmpcodeFirst uint16
	DstportOrIcmpcodeLast  uint16
	TCPFlagsMask           uint8
	TCPFlagsValue          uint8
	RuleId                 string // Unique identifier for the rule
	Priority               int    // Priority for the rule (1..100. 100 is highest)
	SrcEndpointGroup       int    // Source endpoint group
	DstEndpointGroup       int    // Destination endpoint group
}

func (*ACLRule) GetTypeName() string {
	return "acl_rule"
}
func (*ACLRule) GetCrcString() string {
	return "2715e1c0"
}

// MacipACLRule is the Go representation of the VPP binary API data type 'macip_acl_rule'.
type MacipACLRule struct {
	IsPermit       uint8
	IsIpv6         uint8
	SrcMac         []byte `struc:"[6]byte"`
	SrcMacMask     []byte `struc:"[6]byte"`
	SrcIPAddr      []byte `struc:"[16]byte"`
	SrcIPPrefixLen uint8
}

func (*MacipACLRule) GetTypeName() string {
	return "macip_acl_rule"
}
func (*MacipACLRule) GetCrcString() string {
	return "6723f13e"
}

// ACLPluginGetVersion is the Go representation of the VPP binary API message 'acl_plugin_get_version'.
type ACLPluginGetVersion struct {
}

func (*ACLPluginGetVersion) GetMessageName() string {
	return "acl_plugin_get_version"
}
func (*ACLPluginGetVersion) GetCrcString() string {
	return "d7c07748"
}

// ACLPluginGetVersionReply is the Go representation of the VPP binary API message 'acl_plugin_get_version_reply'.
type ACLPluginGetVersionReply struct {
	Major uint32
	Minor uint32
}

func (*ACLPluginGetVersionReply) GetMessageName() string {
	return "acl_plugin_get_version_reply"
}
func (*ACLPluginGetVersionReply) GetCrcString() string {
	return "43eb59a5"
}

// ACLAddReplace is the Go representation of the VPP binary API message 'acl_add_replace'.
type ACLAddReplace struct {
	ACLIndex uint32
	Tag      []byte `struc:"[64]byte"`
	Count    uint32 `struc:"sizeof=R"`
	R        []ACLRule
}

func (*ACLAddReplace) GetMessageName() string {
	return "acl_add_replace"
}
func (*ACLAddReplace) GetCrcString() string {
	return "3c317936"
}

// ACLAddReplaceReply is the Go representation of the VPP binary API message 'acl_add_replace_reply'.
type ACLAddReplaceReply struct {
	ACLIndex uint32
	Retval   int32
}

func (*ACLAddReplaceReply) GetMessageName() string {
	return "acl_add_replace_reply"
}
func (*ACLAddReplaceReply) GetCrcString() string {
	return "a5e6d0cf"
}

// ACLDel is the Go representation of the VPP binary API message 'acl_del'.
type ACLDel struct {
	ACLIndex uint32
}

func (*ACLDel) GetMessageName() string {
	return "acl_del"
}
func (*ACLDel) GetCrcString() string {
	return "82cc30ed"
}

// ACLDelReply is the Go representation of the VPP binary API message 'acl_del_reply'.
type ACLDelReply struct {
	Retval int32
}

func (*ACLDelReply) GetMessageName() string {
	return "acl_del_reply"
}
func (*ACLDelReply) GetCrcString() string {
	return "bbb83d84"
}

// ACLInterfaceAddDel is the Go representation of the VPP binary API message 'acl_interface_add_del'.
type ACLInterfaceAddDel struct {
	IsAdd     uint8
	IsInput   uint8
	SwIfIndex uint32
	ACLIndex  uint32
}

func (*ACLInterfaceAddDel) GetMessageName() string {
	return "acl_interface_add_del"
}
func (*ACLInterfaceAddDel) GetCrcString() string {
	return "98b53725"
}

// ACLInterfaceAddDelReply is the Go representation of the VPP binary API message 'acl_interface_add_del_reply'.
type ACLInterfaceAddDelReply struct {
	Retval int32
}

func (*ACLInterfaceAddDelReply) GetMessageName() string {
	return "acl_interface_add_del_reply"
}
func (*ACLInterfaceAddDelReply) GetCrcString() string {
	return "c1b3c077"
}

// ACLInterfaceSetACLList is the Go representation of the VPP binary API message 'acl_interface_set_acl_list'.
type ACLInterfaceSetACLList struct {
	SwIfIndex uint32
	Count     uint8
	NInput    uint8 `struc:"sizeof=Acls"`
	Acls      []uint32
}

func (*ACLInterfaceSetACLList) GetMessageName() string {
	return "acl_interface_set_acl_list"
}
func (*ACLInterfaceSetACLList) GetCrcString() string {
	return "7562419c"
}

// ACLInterfaceSetACLListReply is the Go representation of the VPP binary API message 'acl_interface_set_acl_list_reply'.
type ACLInterfaceSetACLListReply struct {
	Retval int32
}

func (*ACLInterfaceSetACLListReply) GetMessageName() string {
	return "acl_interface_set_acl_list_reply"
}
func (*ACLInterfaceSetACLListReply) GetCrcString() string {
	return "435ddc2b"
}

// ACLDump is the Go representation of the VPP binary API message 'acl_dump'.
type ACLDump struct {
	ACLIndex uint32
}

func (*ACLDump) GetMessageName() string {
	return "acl_dump"
}
func (*ACLDump) GetCrcString() string {
	return "c188156d"
}

// ACLDetails is the Go representation of the VPP binary API message 'acl_details'.
type ACLDetails struct {
	ACLIndex uint32
	Tag      []byte `struc:"[64]byte"`
	Count    uint32 `struc:"sizeof=R"`
	R        []ACLRule
}

func (*ACLDetails) GetMessageName() string {
	return "acl_details"
}
func (*ACLDetails) GetCrcString() string {
	return "1c8916b7"
}

// ACLInterfaceListDump is the Go representation of the VPP binary API message 'acl_interface_list_dump'.
type ACLInterfaceListDump struct {
	SwIfIndex uint32
}

func (*ACLInterfaceListDump) GetMessageName() string {
	return "acl_interface_list_dump"
}
func (*ACLInterfaceListDump) GetCrcString() string {
	return "adfe84b8"
}

// ACLInterfaceListDetails is the Go representation of the VPP binary API message 'acl_interface_list_details'.
type ACLInterfaceListDetails struct {
	SwIfIndex uint32
	Count     uint8
	NInput    uint8 `struc:"sizeof=Acls"`
	Acls      []uint32
}

func (*ACLInterfaceListDetails) GetMessageName() string {
	return "acl_interface_list_details"
}
func (*ACLInterfaceListDetails) GetCrcString() string {
	return "c8150656"
}

// MacipACLAdd is the Go representation of the VPP binary API message 'macip_acl_add'.
type MacipACLAdd struct {
	Tag   []byte `struc:"[64]byte"`
	Count uint32 `struc:"sizeof=R"`
	R     []MacipACLRule
}

func (*MacipACLAdd) GetMessageName() string {
	return "macip_acl_add"
}
func (*MacipACLAdd) GetCrcString() string {
	return "33356284"
}

// MacipACLAddReply is the Go representation of the VPP binary API message 'macip_acl_add_reply'.
type MacipACLAddReply struct {
	ACLIndex uint32
	Retval   int32
}

func (*MacipACLAddReply) GetMessageName() string {
	return "macip_acl_add_reply"
}
func (*MacipACLAddReply) GetCrcString() string {
	return "472edb4c"
}

// MacipACLDel is the Go representation of the VPP binary API message 'macip_acl_del'.
type MacipACLDel struct {
	ACLIndex uint32
}

func (*MacipACLDel) GetMessageName() string {
	return "macip_acl_del"
}
func (*MacipACLDel) GetCrcString() string {
	return "dde1141f"
}

// MacipACLDelReply is the Go representation of the VPP binary API message 'macip_acl_del_reply'.
type MacipACLDelReply struct {
	Retval int32
}

func (*MacipACLDelReply) GetMessageName() string {
	return "macip_acl_del_reply"
}
func (*MacipACLDelReply) GetCrcString() string {
	return "eeb60e0f"
}

// MacipACLInterfaceAddDel is the Go representation of the VPP binary API message 'macip_acl_interface_add_del'.
type MacipACLInterfaceAddDel struct {
	IsAdd     uint8
	SwIfIndex uint32
	ACLIndex  uint32
}

func (*MacipACLInterfaceAddDel) GetMessageName() string {
	return "macip_acl_interface_add_del"
}
func (*MacipACLInterfaceAddDel) GetCrcString() string {
	return "03a4fab2"
}

// MacipACLInterfaceAddDelReply is the Go representation of the VPP binary API message 'macip_acl_interface_add_del_reply'.
type MacipACLInterfaceAddDelReply struct {
	Retval int32
}

func (*MacipACLInterfaceAddDelReply) GetMessageName() string {
	return "macip_acl_interface_add_del_reply"
}
func (*MacipACLInterfaceAddDelReply) GetCrcString() string {
	return "9e9ee485"
}

// MacipACLDump is the Go representation of the VPP binary API message 'macip_acl_dump'.
type MacipACLDump struct {
	ACLIndex uint32
}

func (*MacipACLDump) GetMessageName() string {
	return "macip_acl_dump"
}
func (*MacipACLDump) GetCrcString() string {
	return "d38227cb"
}

// MacipACLDetails is the Go representation of the VPP binary API message 'macip_acl_details'.
type MacipACLDetails struct {
	ACLIndex uint32
	Tag      []byte `struc:"[64]byte"`
	Count    uint32 `struc:"sizeof=R"`
	R        []MacipACLRule
}

func (*MacipACLDetails) GetMessageName() string {
	return "macip_acl_details"
}
func (*MacipACLDetails) GetCrcString() string {
	return "ee1c50db"
}

// MacipACLInterfaceGet is the Go representation of the VPP binary API message 'macip_acl_interface_get'.
type MacipACLInterfaceGet struct {
}

func (*MacipACLInterfaceGet) GetMessageName() string {
	return "macip_acl_interface_get"
}
func (*MacipACLInterfaceGet) GetCrcString() string {
	return "317ce31c"
}

// MacipACLInterfaceGetReply is the Go representation of the VPP binary API message 'macip_acl_interface_get_reply'.
type MacipACLInterfaceGetReply struct {
	Count uint32 `struc:"sizeof=Acls"`
	Acls  []uint32
}

func (*MacipACLInterfaceGetReply) GetMessageName() string {
	return "macip_acl_interface_get_reply"
}
func (*MacipACLInterfaceGetReply) GetCrcString() string {
	return "6c86a56c"
}
