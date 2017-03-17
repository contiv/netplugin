// Package ip provides the Go interface to VPP binary API of the ip VPP module.
// Generated from 'ip.api.json' on Fri, 17 Mar 2017 17:11:50 UTC.
package ip

// VlApiVersion contains version of the API.
const VlAPIVersion = 0x6a819870

// FibPath is the Go representation of the VPP binary API data type 'fib_path'.
type FibPath struct {
	SwIfIndex  uint32
	Weight     uint32
	IsLocal    uint8
	IsDrop     uint8
	IsUnreach  uint8
	IsProhibit uint8
	Afi        uint8
	NextHop    []byte `struc:"[16]byte"`
}

func (*FibPath) GetTypeName() string {
	return "fib_path"
}
func (*FibPath) GetCrcString() string {
	return "315b1889"
}

// IPFibDump is the Go representation of the VPP binary API message 'ip_fib_dump'.
type IPFibDump struct {
}

func (*IPFibDump) GetMessageName() string {
	return "ip_fib_dump"
}
func (*IPFibDump) GetCrcString() string {
	return "5fe56ca3"
}

// IPFibDetails is the Go representation of the VPP binary API message 'ip_fib_details'.
type IPFibDetails struct {
	TableID       uint32
	AddressLength uint8
	Address       []byte `struc:"[4]byte"`
	Count         uint32 `struc:"sizeof=Path"`
	Path          []FibPath
}

func (*IPFibDetails) GetMessageName() string {
	return "ip_fib_details"
}
func (*IPFibDetails) GetCrcString() string {
	return "fd8c6584"
}

// IP6FibDump is the Go representation of the VPP binary API message 'ip6_fib_dump'.
type IP6FibDump struct {
}

func (*IP6FibDump) GetMessageName() string {
	return "ip6_fib_dump"
}
func (*IP6FibDump) GetCrcString() string {
	return "25c89676"
}

// IP6FibDetails is the Go representation of the VPP binary API message 'ip6_fib_details'.
type IP6FibDetails struct {
	TableID       uint32
	AddressLength uint8
	Address       []byte `struc:"[16]byte"`
	Count         uint32 `struc:"sizeof=Path"`
	Path          []FibPath
}

func (*IP6FibDetails) GetMessageName() string {
	return "ip6_fib_details"
}
func (*IP6FibDetails) GetCrcString() string {
	return "e0825cb5"
}

// IPNeighborDump is the Go representation of the VPP binary API message 'ip_neighbor_dump'.
type IPNeighborDump struct {
	SwIfIndex uint32
	IsIpv6    uint8
}

func (*IPNeighborDump) GetMessageName() string {
	return "ip_neighbor_dump"
}
func (*IPNeighborDump) GetCrcString() string {
	return "3289e160"
}

// IPNeighborDetails is the Go representation of the VPP binary API message 'ip_neighbor_details'.
type IPNeighborDetails struct {
	IsStatic   uint8
	IsIpv6     uint8
	MacAddress []byte `struc:"[6]byte"`
	IPAddress  []byte `struc:"[16]byte"`
}

func (*IPNeighborDetails) GetMessageName() string {
	return "ip_neighbor_details"
}
func (*IPNeighborDetails) GetCrcString() string {
	return "3a00e32a"
}

// IPNeighborAddDel is the Go representation of the VPP binary API message 'ip_neighbor_add_del'.
type IPNeighborAddDel struct {
	VrfID      uint32
	SwIfIndex  uint32
	IsAdd      uint8
	IsIpv6     uint8
	IsStatic   uint8
	MacAddress []byte `struc:"[6]byte"`
	DstAddress []byte `struc:"[16]byte"`
}

func (*IPNeighborAddDel) GetMessageName() string {
	return "ip_neighbor_add_del"
}
func (*IPNeighborAddDel) GetCrcString() string {
	return "66f2112c"
}

// IPNeighborAddDelReply is the Go representation of the VPP binary API message 'ip_neighbor_add_del_reply'.
type IPNeighborAddDelReply struct {
	Retval int32
}

func (*IPNeighborAddDelReply) GetMessageName() string {
	return "ip_neighbor_add_del_reply"
}
func (*IPNeighborAddDelReply) GetCrcString() string {
	return "e5b0f318"
}

// SetIPFlowHash is the Go representation of the VPP binary API message 'set_ip_flow_hash'.
type SetIPFlowHash struct {
	VrfID   uint32
	IsIpv6  uint8
	Src     uint8
	Dst     uint8
	Sport   uint8
	Dport   uint8
	Proto   uint8
	Reverse uint8
}

func (*SetIPFlowHash) GetMessageName() string {
	return "set_ip_flow_hash"
}
func (*SetIPFlowHash) GetCrcString() string {
	return "92ad3798"
}

// SetIPFlowHashReply is the Go representation of the VPP binary API message 'set_ip_flow_hash_reply'.
type SetIPFlowHashReply struct {
	Retval int32
}

func (*SetIPFlowHashReply) GetMessageName() string {
	return "set_ip_flow_hash_reply"
}
func (*SetIPFlowHashReply) GetCrcString() string {
	return "35a9e5eb"
}

// SwInterfaceIP6ndRaConfig is the Go representation of the VPP binary API message 'sw_interface_ip6nd_ra_config'.
type SwInterfaceIP6ndRaConfig struct {
	SwIfIndex       uint32
	Suppress        uint8
	Managed         uint8
	Other           uint8
	LlOption        uint8
	SendUnicast     uint8
	Cease           uint8
	IsNo            uint8
	DefaultRouter   uint8
	MaxInterval     uint32
	MinInterval     uint32
	Lifetime        uint32
	InitialCount    uint32
	InitialInterval uint32
}

func (*SwInterfaceIP6ndRaConfig) GetMessageName() string {
	return "sw_interface_ip6nd_ra_config"
}
func (*SwInterfaceIP6ndRaConfig) GetCrcString() string {
	return "ec4a29f6"
}

// SwInterfaceIP6ndRaConfigReply is the Go representation of the VPP binary API message 'sw_interface_ip6nd_ra_config_reply'.
type SwInterfaceIP6ndRaConfigReply struct {
	Retval int32
}

func (*SwInterfaceIP6ndRaConfigReply) GetMessageName() string {
	return "sw_interface_ip6nd_ra_config_reply"
}
func (*SwInterfaceIP6ndRaConfigReply) GetCrcString() string {
	return "16e25c5b"
}

// SwInterfaceIP6ndRaPrefix is the Go representation of the VPP binary API message 'sw_interface_ip6nd_ra_prefix'.
type SwInterfaceIP6ndRaPrefix struct {
	SwIfIndex     uint32
	Address       []byte `struc:"[16]byte"`
	AddressLength uint8
	UseDefault    uint8
	NoAdvertise   uint8
	OffLink       uint8
	NoAutoconfig  uint8
	NoOnlink      uint8
	IsNo          uint8
	ValLifetime   uint32
	PrefLifetime  uint32
}

func (*SwInterfaceIP6ndRaPrefix) GetMessageName() string {
	return "sw_interface_ip6nd_ra_prefix"
}
func (*SwInterfaceIP6ndRaPrefix) GetCrcString() string {
	return "5db6555c"
}

// SwInterfaceIP6ndRaPrefixReply is the Go representation of the VPP binary API message 'sw_interface_ip6nd_ra_prefix_reply'.
type SwInterfaceIP6ndRaPrefixReply struct {
	Retval int32
}

func (*SwInterfaceIP6ndRaPrefixReply) GetMessageName() string {
	return "sw_interface_ip6nd_ra_prefix_reply"
}
func (*SwInterfaceIP6ndRaPrefixReply) GetCrcString() string {
	return "8050adb3"
}

// SwInterfaceIP6EnableDisable is the Go representation of the VPP binary API message 'sw_interface_ip6_enable_disable'.
type SwInterfaceIP6EnableDisable struct {
	SwIfIndex uint32
	Enable    uint8
}

func (*SwInterfaceIP6EnableDisable) GetMessageName() string {
	return "sw_interface_ip6_enable_disable"
}
func (*SwInterfaceIP6EnableDisable) GetCrcString() string {
	return "4a4e5405"
}

// SwInterfaceIP6EnableDisableReply is the Go representation of the VPP binary API message 'sw_interface_ip6_enable_disable_reply'.
type SwInterfaceIP6EnableDisableReply struct {
	Retval int32
}

func (*SwInterfaceIP6EnableDisableReply) GetMessageName() string {
	return "sw_interface_ip6_enable_disable_reply"
}
func (*SwInterfaceIP6EnableDisableReply) GetCrcString() string {
	return "eb8b4a40"
}

// SwInterfaceIP6SetLinkLocalAddress is the Go representation of the VPP binary API message 'sw_interface_ip6_set_link_local_address'.
type SwInterfaceIP6SetLinkLocalAddress struct {
	SwIfIndex uint32
	Address   []byte `struc:"[16]byte"`
}

func (*SwInterfaceIP6SetLinkLocalAddress) GetMessageName() string {
	return "sw_interface_ip6_set_link_local_address"
}
func (*SwInterfaceIP6SetLinkLocalAddress) GetCrcString() string {
	return "3db6d52b"
}

// SwInterfaceIP6SetLinkLocalAddressReply is the Go representation of the VPP binary API message 'sw_interface_ip6_set_link_local_address_reply'.
type SwInterfaceIP6SetLinkLocalAddressReply struct {
	Retval int32
}

func (*SwInterfaceIP6SetLinkLocalAddressReply) GetMessageName() string {
	return "sw_interface_ip6_set_link_local_address_reply"
}
func (*SwInterfaceIP6SetLinkLocalAddressReply) GetCrcString() string {
	return "0a781e17"
}

// IPAddDelRoute is the Go representation of the VPP binary API message 'ip_add_del_route'.
type IPAddDelRoute struct {
	NextHopSwIfIndex     uint32
	TableID              uint32
	ClassifyTableIndex   uint32
	NextHopTableID       uint32
	CreateVrfIfNeeded    uint8
	IsAdd                uint8
	IsDrop               uint8
	IsUnreach            uint8
	IsProhibit           uint8
	IsIpv6               uint8
	IsLocal              uint8
	IsClassify           uint8
	IsMultipath          uint8
	IsResolveHost        uint8
	IsResolveAttached    uint8
	NotLast              uint8
	NextHopWeight        uint8
	DstAddressLength     uint8
	DstAddress           []byte `struc:"[16]byte"`
	NextHopAddress       []byte `struc:"[16]byte"`
	NextHopNOutLabels    uint8
	NextHopViaLabel      uint32 `struc:"sizeof=NextHopOutLabelStack"`
	NextHopOutLabelStack []uint32
}

func (*IPAddDelRoute) GetMessageName() string {
	return "ip_add_del_route"
}
func (*IPAddDelRoute) GetCrcString() string {
	return "a0ab24bf"
}

// IPAddDelRouteReply is the Go representation of the VPP binary API message 'ip_add_del_route_reply'.
type IPAddDelRouteReply struct {
	Retval int32
}

func (*IPAddDelRouteReply) GetMessageName() string {
	return "ip_add_del_route_reply"
}
func (*IPAddDelRouteReply) GetCrcString() string {
	return "ea57492b"
}

// IPMrouteAddDel is the Go representation of the VPP binary API message 'ip_mroute_add_del'.
type IPMrouteAddDel struct {
	NextHopSwIfIndex  uint32
	TableID           uint32
	EntryFlags        uint32
	ItfFlags          uint32
	GrpAddressLength  uint16
	CreateVrfIfNeeded uint8
	IsAdd             uint8
	IsIpv6            uint8
	IsLocal           uint8
	GrpAddress        []byte `struc:"[16]byte"`
	SrcAddress        []byte `struc:"[16]byte"`
}

func (*IPMrouteAddDel) GetMessageName() string {
	return "ip_mroute_add_del"
}
func (*IPMrouteAddDel) GetCrcString() string {
	return "8312830f"
}

// IPMrouteAddDelReply is the Go representation of the VPP binary API message 'ip_mroute_add_del_reply'.
type IPMrouteAddDelReply struct {
	Retval int32
}

func (*IPMrouteAddDelReply) GetMessageName() string {
	return "ip_mroute_add_del_reply"
}
func (*IPMrouteAddDelReply) GetCrcString() string {
	return "8cabe02c"
}

// IPAddressDetails is the Go representation of the VPP binary API message 'ip_address_details'.
type IPAddressDetails struct {
	IP           []byte `struc:"[16]byte"`
	PrefixLength uint8
	SwIfIndex    uint32
	IsIpv6       uint8
}

func (*IPAddressDetails) GetMessageName() string {
	return "ip_address_details"
}
func (*IPAddressDetails) GetCrcString() string {
	return "190d4266"
}

// IPAddressDump is the Go representation of the VPP binary API message 'ip_address_dump'.
type IPAddressDump struct {
	SwIfIndex uint32
	IsIpv6    uint8
}

func (*IPAddressDump) GetMessageName() string {
	return "ip_address_dump"
}
func (*IPAddressDump) GetCrcString() string {
	return "632e859a"
}

// IPDetails is the Go representation of the VPP binary API message 'ip_details'.
type IPDetails struct {
	SwIfIndex uint32
	IsIpv6    uint8
}

func (*IPDetails) GetMessageName() string {
	return "ip_details"
}
func (*IPDetails) GetCrcString() string {
	return "695c8227"
}

// IPDump is the Go representation of the VPP binary API message 'ip_dump'.
type IPDump struct {
	IsIpv6 uint8
}

func (*IPDump) GetMessageName() string {
	return "ip_dump"
}
func (*IPDump) GetCrcString() string {
	return "3c1e33e0"
}

// MfibSignalDump is the Go representation of the VPP binary API message 'mfib_signal_dump'.
type MfibSignalDump struct {
}

func (*MfibSignalDump) GetMessageName() string {
	return "mfib_signal_dump"
}
func (*MfibSignalDump) GetCrcString() string {
	return "bbbbd40d"
}

// MfibSignalDetails is the Go representation of the VPP binary API message 'mfib_signal_details'.
type MfibSignalDetails struct {
	SwIfIndex     uint32
	TableID       uint32
	GrpAddressLen uint16
	GrpAddress    []byte `struc:"[16]byte"`
	SrcAddress    []byte `struc:"[16]byte"`
	IPPacketLen   uint16
	IPPacketData  []byte `struc:"[256]byte"`
}

func (*MfibSignalDetails) GetMessageName() string {
	return "mfib_signal_details"
}
func (*MfibSignalDetails) GetCrcString() string {
	return "6ba92c72"
}
