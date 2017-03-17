// Package vpe provides the Go interface to VPP binary API of the vpe VPP module.
// Generated from 'vpe.api.json' on Fri, 17 Mar 2017 17:11:50 UTC.
package vpe

// VlApiVersion contains version of the API.
const VlAPIVersion = 0x80e93d89

// FibPath2 is the Go representation of the VPP binary API data type 'fib_path2'.
type FibPath2 struct {
	SwIfIndex  uint32
	Weight     uint32
	IsLocal    uint8
	IsDrop     uint8
	IsUnreach  uint8
	IsProhibit uint8
	Afi        uint8
	NextHop    []byte `struc:"[16]byte"`
}

func (*FibPath2) GetTypeName() string {
	return "fib_path2"
}
func (*FibPath2) GetCrcString() string {
	return "151e303b"
}

// IP4FibCounter is the Go representation of the VPP binary API data type 'ip4_fib_counter'.
type IP4FibCounter struct {
	Address       uint32
	AddressLength uint8
	Packets       uint64
	Bytes         uint64
}

func (*IP4FibCounter) GetTypeName() string {
	return "ip4_fib_counter"
}
func (*IP4FibCounter) GetCrcString() string {
	return "b2739495"
}

// IP6FibCounter is the Go representation of the VPP binary API data type 'ip6_fib_counter'.
type IP6FibCounter struct {
	Address       []uint64 `struc:"[2]uint64"`
	AddressLength uint8
	Packets       uint64
	Bytes         uint64
}

func (*IP6FibCounter) GetTypeName() string {
	return "ip6_fib_counter"
}
func (*IP6FibCounter) GetCrcString() string {
	return "cf35769b"
}

// LispAdjacency is the Go representation of the VPP binary API data type 'lisp_adjacency'.
type LispAdjacency struct {
	EidType       uint8
	Reid          []byte `struc:"[16]byte"`
	Leid          []byte `struc:"[16]byte"`
	ReidPrefixLen uint8
	LeidPrefixLen uint8
}

func (*LispAdjacency) GetTypeName() string {
	return "lisp_adjacency"
}
func (*LispAdjacency) GetCrcString() string {
	return "ade34024"
}

// CreateVlanSubif is the Go representation of the VPP binary API message 'create_vlan_subif'.
type CreateVlanSubif struct {
	SwIfIndex uint32
	VlanID    uint32
}

func (*CreateVlanSubif) GetMessageName() string {
	return "create_vlan_subif"
}
func (*CreateVlanSubif) GetCrcString() string {
	return "af9ae1e9"
}

// CreateVlanSubifReply is the Go representation of the VPP binary API message 'create_vlan_subif_reply'.
type CreateVlanSubifReply struct {
	Retval    int32
	SwIfIndex uint32
}

func (*CreateVlanSubifReply) GetMessageName() string {
	return "create_vlan_subif_reply"
}
func (*CreateVlanSubifReply) GetCrcString() string {
	return "8f36b888"
}

// SwInterfaceSetMplsEnable is the Go representation of the VPP binary API message 'sw_interface_set_mpls_enable'.
type SwInterfaceSetMplsEnable struct {
	SwIfIndex uint32
	Enable    uint8
}

func (*SwInterfaceSetMplsEnable) GetMessageName() string {
	return "sw_interface_set_mpls_enable"
}
func (*SwInterfaceSetMplsEnable) GetCrcString() string {
	return "37f6357e"
}

// SwInterfaceSetMplsEnableReply is the Go representation of the VPP binary API message 'sw_interface_set_mpls_enable_reply'.
type SwInterfaceSetMplsEnableReply struct {
	Retval int32
}

func (*SwInterfaceSetMplsEnableReply) GetMessageName() string {
	return "sw_interface_set_mpls_enable_reply"
}
func (*SwInterfaceSetMplsEnableReply) GetCrcString() string {
	return "5ffd3ca9"
}

// MplsRouteAddDel is the Go representation of the VPP binary API message 'mpls_route_add_del'.
type MplsRouteAddDel struct {
	MrLabel                uint32
	MrEos                  uint8
	MrTableID              uint32
	MrClassifyTableIndex   uint32
	MrCreateTableIfNeeded  uint8
	MrIsAdd                uint8
	MrIsClassify           uint8
	MrIsMultipath          uint8
	MrIsResolveHost        uint8
	MrIsResolveAttached    uint8
	MrNextHopProtoIsIP4    uint8
	MrNextHopWeight        uint8
	MrNextHop              []byte `struc:"[16]byte"`
	MrNextHopNOutLabels    uint8
	MrNextHopSwIfIndex     uint32
	MrNextHopTableID       uint32
	MrNextHopViaLabel      uint32 `struc:"sizeof=MrNextHopOutLabelStack"`
	MrNextHopOutLabelStack []uint32
}

func (*MplsRouteAddDel) GetMessageName() string {
	return "mpls_route_add_del"
}
func (*MplsRouteAddDel) GetCrcString() string {
	return "88073494"
}

// MplsRouteAddDelReply is the Go representation of the VPP binary API message 'mpls_route_add_del_reply'.
type MplsRouteAddDelReply struct {
	Retval int32
}

func (*MplsRouteAddDelReply) GetMessageName() string {
	return "mpls_route_add_del_reply"
}
func (*MplsRouteAddDelReply) GetCrcString() string {
	return "21a12fe9"
}

// MplsFibDump is the Go representation of the VPP binary API message 'mpls_fib_dump'.
type MplsFibDump struct {
}

func (*MplsFibDump) GetMessageName() string {
	return "mpls_fib_dump"
}
func (*MplsFibDump) GetCrcString() string {
	return "7e82659e"
}

// MplsFibDetails is the Go representation of the VPP binary API message 'mpls_fib_details'.
type MplsFibDetails struct {
	TableID uint32
	EosBit  uint8
	Label   uint32
	Count   uint32 `struc:"sizeof=Path"`
	Path    []FibPath2
}

func (*MplsFibDetails) GetMessageName() string {
	return "mpls_fib_details"
}
func (*MplsFibDetails) GetCrcString() string {
	return "2804be60"
}

// MplsIPBindUnbind is the Go representation of the VPP binary API message 'mpls_ip_bind_unbind'.
type MplsIPBindUnbind struct {
	MbMplsTableID         uint32
	MbLabel               uint32
	MbIPTableID           uint32
	MbCreateTableIfNeeded uint8
	MbIsBind              uint8
	MbIsIP4               uint8
	MbAddressLength       uint8
	MbAddress             []byte `struc:"[16]byte"`
}

func (*MplsIPBindUnbind) GetMessageName() string {
	return "mpls_ip_bind_unbind"
}
func (*MplsIPBindUnbind) GetCrcString() string {
	return "167f24bd"
}

// MplsIPBindUnbindReply is the Go representation of the VPP binary API message 'mpls_ip_bind_unbind_reply'.
type MplsIPBindUnbindReply struct {
	Retval int32
}

func (*MplsIPBindUnbindReply) GetMessageName() string {
	return "mpls_ip_bind_unbind_reply"
}
func (*MplsIPBindUnbindReply) GetCrcString() string {
	return "5753d1ed"
}

// MplsTunnelAddDel is the Go representation of the VPP binary API message 'mpls_tunnel_add_del'.
type MplsTunnelAddDel struct {
	MtSwIfIndex            uint32
	MtIsAdd                uint8
	MtL2Only               uint8
	MtNextHopProtoIsIP4    uint8
	MtNextHopWeight        uint8
	MtNextHop              []byte `struc:"[16]byte"`
	MtNextHopNOutLabels    uint8
	MtNextHopSwIfIndex     uint32
	MtNextHopTableID       uint32 `struc:"sizeof=MtNextHopOutLabelStack"`
	MtNextHopOutLabelStack []uint32
}

func (*MplsTunnelAddDel) GetMessageName() string {
	return "mpls_tunnel_add_del"
}
func (*MplsTunnelAddDel) GetCrcString() string {
	return "6aee5ee7"
}

// MplsTunnelAddDelReply is the Go representation of the VPP binary API message 'mpls_tunnel_add_del_reply'.
type MplsTunnelAddDelReply struct {
	Retval    int32
	SwIfIndex uint32
}

func (*MplsTunnelAddDelReply) GetMessageName() string {
	return "mpls_tunnel_add_del_reply"
}
func (*MplsTunnelAddDelReply) GetCrcString() string {
	return "bb483273"
}

// MplsTunnelDump is the Go representation of the VPP binary API message 'mpls_tunnel_dump'.
type MplsTunnelDump struct {
	TunnelIndex int32
}

func (*MplsTunnelDump) GetMessageName() string {
	return "mpls_tunnel_dump"
}
func (*MplsTunnelDump) GetCrcString() string {
	return "be9ada9c"
}

// MplsTunnelDetails is the Go representation of the VPP binary API message 'mpls_tunnel_details'.
type MplsTunnelDetails struct {
	TunnelIndex         uint32
	MtL2Only            uint8
	MtSwIfIndex         uint8
	MtNextHopProtoIsIP4 uint8
	MtNextHop           []byte `struc:"[16]byte"`
	MtNextHopSwIfIndex  uint32
	MtNextHopTableID    uint32
	MtNextHopNLabels    uint32 `struc:"sizeof=MtNextHopOutLabels"`
	MtNextHopOutLabels  []uint32
}

func (*MplsTunnelDetails) GetMessageName() string {
	return "mpls_tunnel_details"
}
func (*MplsTunnelDetails) GetCrcString() string {
	return "4fc1cd6f"
}

// ProxyArpAddDel is the Go representation of the VPP binary API message 'proxy_arp_add_del'.
type ProxyArpAddDel struct {
	VrfID      uint32
	IsAdd      uint8
	LowAddress []byte `struc:"[4]byte"`
	HiAddress  []byte `struc:"[4]byte"`
}

func (*ProxyArpAddDel) GetMessageName() string {
	return "proxy_arp_add_del"
}
func (*ProxyArpAddDel) GetCrcString() string {
	return "4bef9951"
}

// ProxyArpAddDelReply is the Go representation of the VPP binary API message 'proxy_arp_add_del_reply'.
type ProxyArpAddDelReply struct {
	Retval int32
}

func (*ProxyArpAddDelReply) GetMessageName() string {
	return "proxy_arp_add_del_reply"
}
func (*ProxyArpAddDelReply) GetCrcString() string {
	return "8e2d621d"
}

// ProxyArpIntfcEnableDisable is the Go representation of the VPP binary API message 'proxy_arp_intfc_enable_disable'.
type ProxyArpIntfcEnableDisable struct {
	SwIfIndex     uint32
	EnableDisable uint8
}

func (*ProxyArpIntfcEnableDisable) GetMessageName() string {
	return "proxy_arp_intfc_enable_disable"
}
func (*ProxyArpIntfcEnableDisable) GetCrcString() string {
	return "3ee1998e"
}

// ProxyArpIntfcEnableDisableReply is the Go representation of the VPP binary API message 'proxy_arp_intfc_enable_disable_reply'.
type ProxyArpIntfcEnableDisableReply struct {
	Retval int32
}

func (*ProxyArpIntfcEnableDisableReply) GetMessageName() string {
	return "proxy_arp_intfc_enable_disable_reply"
}
func (*ProxyArpIntfcEnableDisableReply) GetCrcString() string {
	return "23d273cd"
}

// ResetVrf is the Go representation of the VPP binary API message 'reset_vrf'.
type ResetVrf struct {
	IsIpv6 uint8
	VrfID  uint32
}

func (*ResetVrf) GetMessageName() string {
	return "reset_vrf"
}
func (*ResetVrf) GetCrcString() string {
	return "eb07deb0"
}

// ResetVrfReply is the Go representation of the VPP binary API message 'reset_vrf_reply'.
type ResetVrfReply struct {
	Retval int32
}

func (*ResetVrfReply) GetMessageName() string {
	return "reset_vrf_reply"
}
func (*ResetVrfReply) GetCrcString() string {
	return "5f283863"
}

// IsAddressReachable is the Go representation of the VPP binary API message 'is_address_reachable'.
type IsAddressReachable struct {
	NextHopSwIfIndex uint32
	IsKnown          uint8
	IsIpv6           uint8
	IsError          uint8
	Address          []byte `struc:"[16]byte"`
}

func (*IsAddressReachable) GetMessageName() string {
	return "is_address_reachable"
}
func (*IsAddressReachable) GetCrcString() string {
	return "a8b6e322"
}

// WantStats is the Go representation of the VPP binary API message 'want_stats'.
type WantStats struct {
	EnableDisable uint32
	Pid           uint32
}

func (*WantStats) GetMessageName() string {
	return "want_stats"
}
func (*WantStats) GetCrcString() string {
	return "4f2effb4"
}

// WantStatsReply is the Go representation of the VPP binary API message 'want_stats_reply'.
type WantStatsReply struct {
	Retval int32
}

func (*WantStatsReply) GetMessageName() string {
	return "want_stats_reply"
}
func (*WantStatsReply) GetCrcString() string {
	return "b36abf5f"
}

// VnetIP4FibCounters is the Go representation of the VPP binary API message 'vnet_ip4_fib_counters'.
type VnetIP4FibCounters struct {
	VrfID uint32
	Count uint32 `struc:"sizeof=C"`
	C     []IP4FibCounter
}

func (*VnetIP4FibCounters) GetMessageName() string {
	return "vnet_ip4_fib_counters"
}
func (*VnetIP4FibCounters) GetCrcString() string {
	return "1ab9d6c5"
}

// VnetIP6FibCounters is the Go representation of the VPP binary API message 'vnet_ip6_fib_counters'.
type VnetIP6FibCounters struct {
	VrfID uint32
	Count uint32 `struc:"sizeof=C"`
	C     []IP6FibCounter
}

func (*VnetIP6FibCounters) GetMessageName() string {
	return "vnet_ip6_fib_counters"
}
func (*VnetIP6FibCounters) GetCrcString() string {
	return "9ab453ae"
}

// VnetGetSummaryStats is the Go representation of the VPP binary API message 'vnet_get_summary_stats'.
type VnetGetSummaryStats struct {
}

func (*VnetGetSummaryStats) GetMessageName() string {
	return "vnet_get_summary_stats"
}
func (*VnetGetSummaryStats) GetCrcString() string {
	return "16435c20"
}

// VnetSummaryStatsReply is the Go representation of the VPP binary API message 'vnet_summary_stats_reply'.
type VnetSummaryStatsReply struct {
	Retval     int32
	TotalPkts  []uint64 `struc:"[2]uint64"`
	TotalBytes []uint64 `struc:"[2]uint64"`
	VectorRate float64
}

func (*VnetSummaryStatsReply) GetMessageName() string {
	return "vnet_summary_stats_reply"
}
func (*VnetSummaryStatsReply) GetCrcString() string {
	return "87a8fa9f"
}

// OamEvent is the Go representation of the VPP binary API message 'oam_event'.
type OamEvent struct {
	DstAddress []byte `struc:"[4]byte"`
	State      uint8
}

func (*OamEvent) GetMessageName() string {
	return "oam_event"
}
func (*OamEvent) GetCrcString() string {
	return "4f285ade"
}

// WantOamEvents is the Go representation of the VPP binary API message 'want_oam_events'.
type WantOamEvents struct {
	EnableDisable uint32
	Pid           uint32
}

func (*WantOamEvents) GetMessageName() string {
	return "want_oam_events"
}
func (*WantOamEvents) GetCrcString() string {
	return "948ef12a"
}

// WantOamEventsReply is the Go representation of the VPP binary API message 'want_oam_events_reply'.
type WantOamEventsReply struct {
	Retval int32
}

func (*WantOamEventsReply) GetMessageName() string {
	return "want_oam_events_reply"
}
func (*WantOamEventsReply) GetCrcString() string {
	return "266a677d"
}

// OamAddDel is the Go representation of the VPP binary API message 'oam_add_del'.
type OamAddDel struct {
	VrfID      uint32
	SrcAddress []byte `struc:"[4]byte"`
	DstAddress []byte `struc:"[4]byte"`
	IsAdd      uint8
}

func (*OamAddDel) GetMessageName() string {
	return "oam_add_del"
}
func (*OamAddDel) GetCrcString() string {
	return "b14bc7df"
}

// OamAddDelReply is the Go representation of the VPP binary API message 'oam_add_del_reply'.
type OamAddDelReply struct {
	Retval int32
}

func (*OamAddDelReply) GetMessageName() string {
	return "oam_add_del_reply"
}
func (*OamAddDelReply) GetCrcString() string {
	return "c5594eec"
}

// ResetFib is the Go representation of the VPP binary API message 'reset_fib'.
type ResetFib struct {
	VrfID  uint32
	IsIpv6 uint8
}

func (*ResetFib) GetMessageName() string {
	return "reset_fib"
}
func (*ResetFib) GetCrcString() string {
	return "6f17106b"
}

// ResetFibReply is the Go representation of the VPP binary API message 'reset_fib_reply'.
type ResetFibReply struct {
	Retval int32
}

func (*ResetFibReply) GetMessageName() string {
	return "reset_fib_reply"
}
func (*ResetFibReply) GetCrcString() string {
	return "990dcbf8"
}

// DhcpProxyConfig is the Go representation of the VPP binary API message 'dhcp_proxy_config'.
type DhcpProxyConfig struct {
	VrfID           uint32
	IsIpv6          uint8
	IsAdd           uint8
	InsertCircuitID uint8
	DhcpServer      []byte `struc:"[16]byte"`
	DhcpSrcAddress  []byte `struc:"[16]byte"`
}

func (*DhcpProxyConfig) GetMessageName() string {
	return "dhcp_proxy_config"
}
func (*DhcpProxyConfig) GetCrcString() string {
	return "864167ef"
}

// DhcpProxyConfigReply is the Go representation of the VPP binary API message 'dhcp_proxy_config_reply'.
type DhcpProxyConfigReply struct {
	Retval int32
}

func (*DhcpProxyConfigReply) GetMessageName() string {
	return "dhcp_proxy_config_reply"
}
func (*DhcpProxyConfigReply) GetCrcString() string {
	return "fe63196f"
}

// DhcpProxySetVss is the Go representation of the VPP binary API message 'dhcp_proxy_set_vss'.
type DhcpProxySetVss struct {
	TblID  uint32
	Oui    uint32
	FibID  uint32
	IsIpv6 uint8
	IsAdd  uint8
}

func (*DhcpProxySetVss) GetMessageName() string {
	return "dhcp_proxy_set_vss"
}
func (*DhcpProxySetVss) GetCrcString() string {
	return "be54d194"
}

// DhcpProxySetVssReply is the Go representation of the VPP binary API message 'dhcp_proxy_set_vss_reply'.
type DhcpProxySetVssReply struct {
	Retval int32
}

func (*DhcpProxySetVssReply) GetMessageName() string {
	return "dhcp_proxy_set_vss_reply"
}
func (*DhcpProxySetVssReply) GetCrcString() string {
	return "5bb4e754"
}

// CreateLoopback is the Go representation of the VPP binary API message 'create_loopback'.
type CreateLoopback struct {
	MacAddress []byte `struc:"[6]byte"`
}

func (*CreateLoopback) GetMessageName() string {
	return "create_loopback"
}
func (*CreateLoopback) GetCrcString() string {
	return "b2602de5"
}

// CreateLoopbackReply is the Go representation of the VPP binary API message 'create_loopback_reply'.
type CreateLoopbackReply struct {
	Retval    int32
	SwIfIndex uint32
}

func (*CreateLoopbackReply) GetMessageName() string {
	return "create_loopback_reply"
}
func (*CreateLoopbackReply) GetCrcString() string {
	return "9520f804"
}

// DeleteLoopback is the Go representation of the VPP binary API message 'delete_loopback'.
type DeleteLoopback struct {
	SwIfIndex uint32
}

func (*DeleteLoopback) GetMessageName() string {
	return "delete_loopback"
}
func (*DeleteLoopback) GetCrcString() string {
	return "ded428b0"
}

// DeleteLoopbackReply is the Go representation of the VPP binary API message 'delete_loopback_reply'.
type DeleteLoopbackReply struct {
	Retval int32
}

func (*DeleteLoopbackReply) GetMessageName() string {
	return "delete_loopback_reply"
}
func (*DeleteLoopbackReply) GetCrcString() string {
	return "c91dafa5"
}

// ControlPing is the Go representation of the VPP binary API message 'control_ping'.
type ControlPing struct {
}

func (*ControlPing) GetMessageName() string {
	return "control_ping"
}
func (*ControlPing) GetCrcString() string {
	return "ea1bf4f7"
}

// ControlPingReply is the Go representation of the VPP binary API message 'control_ping_reply'.
type ControlPingReply struct {
	Retval int32
	VpePid uint32
}

func (*ControlPingReply) GetMessageName() string {
	return "control_ping_reply"
}
func (*ControlPingReply) GetCrcString() string {
	return "aa016e7b"
}

// CliRequest is the Go representation of the VPP binary API message 'cli_request'.
type CliRequest struct {
	CmdInShmem uint64
}

func (*CliRequest) GetMessageName() string {
	return "cli_request"
}
func (*CliRequest) GetCrcString() string {
	return "fef056d0"
}

// CliInband is the Go representation of the VPP binary API message 'cli_inband'.
type CliInband struct {
	Length uint32 `struc:"sizeof=Cmd"`
	Cmd    []byte
}

func (*CliInband) GetMessageName() string {
	return "cli_inband"
}
func (*CliInband) GetCrcString() string {
	return "22345937"
}

// CliReply is the Go representation of the VPP binary API message 'cli_reply'.
type CliReply struct {
	Retval       int32
	ReplyInShmem uint64
}

func (*CliReply) GetMessageName() string {
	return "cli_reply"
}
func (*CliReply) GetCrcString() string {
	return "594a0b2e"
}

// CliInbandReply is the Go representation of the VPP binary API message 'cli_inband_reply'.
type CliInbandReply struct {
	Retval int32
	Length uint32 `struc:"sizeof=Reply"`
	Reply  []byte
}

func (*CliInbandReply) GetMessageName() string {
	return "cli_inband_reply"
}
func (*CliInbandReply) GetCrcString() string {
	return "c1835761"
}

// SetArpNeighborLimit is the Go representation of the VPP binary API message 'set_arp_neighbor_limit'.
type SetArpNeighborLimit struct {
	IsIpv6           uint8
	ArpNeighborLimit uint32
}

func (*SetArpNeighborLimit) GetMessageName() string {
	return "set_arp_neighbor_limit"
}
func (*SetArpNeighborLimit) GetCrcString() string {
	return "c1690cb4"
}

// SetArpNeighborLimitReply is the Go representation of the VPP binary API message 'set_arp_neighbor_limit_reply'.
type SetArpNeighborLimitReply struct {
	Retval int32
}

func (*SetArpNeighborLimitReply) GetMessageName() string {
	return "set_arp_neighbor_limit_reply"
}
func (*SetArpNeighborLimitReply) GetCrcString() string {
	return "a6b30518"
}

// L2PatchAddDel is the Go representation of the VPP binary API message 'l2_patch_add_del'.
type L2PatchAddDel struct {
	RxSwIfIndex uint32
	TxSwIfIndex uint32
	IsAdd       uint8
}

func (*L2PatchAddDel) GetMessageName() string {
	return "l2_patch_add_del"
}
func (*L2PatchAddDel) GetCrcString() string {
	return "9b10029a"
}

// L2PatchAddDelReply is the Go representation of the VPP binary API message 'l2_patch_add_del_reply'.
type L2PatchAddDelReply struct {
	Retval int32
}

func (*L2PatchAddDelReply) GetMessageName() string {
	return "l2_patch_add_del_reply"
}
func (*L2PatchAddDelReply) GetCrcString() string {
	return "a85e37be"
}

// SrTunnelAddDel is the Go representation of the VPP binary API message 'sr_tunnel_add_del'.
type SrTunnelAddDel struct {
	IsAdd             uint8
	Name              []byte `struc:"[64]byte"`
	SrcAddress        []byte `struc:"[16]byte"`
	DstAddress        []byte `struc:"[16]byte"`
	DstMaskWidth      uint8
	InnerVrfID        uint32
	OuterVrfID        uint32
	FlagsNetByteOrder uint16
	NSegments         uint8
	NTags             uint8
	PolicyName        []byte `struc:"[64]byte"`
	SegsAndTags       []byte `struc:"sizeof=SegsAndTags"`
}

func (*SrTunnelAddDel) GetMessageName() string {
	return "sr_tunnel_add_del"
}
func (*SrTunnelAddDel) GetCrcString() string {
	return "4c1d2d59"
}

// SrTunnelAddDelReply is the Go representation of the VPP binary API message 'sr_tunnel_add_del_reply'.
type SrTunnelAddDelReply struct {
	Retval int32
}

func (*SrTunnelAddDelReply) GetMessageName() string {
	return "sr_tunnel_add_del_reply"
}
func (*SrTunnelAddDelReply) GetCrcString() string {
	return "76cbf0e5"
}

// SrPolicyAddDel is the Go representation of the VPP binary API message 'sr_policy_add_del'.
type SrPolicyAddDel struct {
	IsAdd       uint8
	Name        []byte `struc:"[64]byte"`
	TunnelNames []byte `struc:"sizeof=TunnelNames"`
}

func (*SrPolicyAddDel) GetMessageName() string {
	return "sr_policy_add_del"
}
func (*SrPolicyAddDel) GetCrcString() string {
	return "9c26297a"
}

// SrPolicyAddDelReply is the Go representation of the VPP binary API message 'sr_policy_add_del_reply'.
type SrPolicyAddDelReply struct {
	Retval int32
}

func (*SrPolicyAddDelReply) GetMessageName() string {
	return "sr_policy_add_del_reply"
}
func (*SrPolicyAddDelReply) GetCrcString() string {
	return "bc01a017"
}

// SrMulticastMapAddDel is the Go representation of the VPP binary API message 'sr_multicast_map_add_del'.
type SrMulticastMapAddDel struct {
	IsAdd            uint8
	MulticastAddress []byte `struc:"[16]byte"`
	PolicyName       []byte `struc:"[64]byte"`
}

func (*SrMulticastMapAddDel) GetMessageName() string {
	return "sr_multicast_map_add_del"
}
func (*SrMulticastMapAddDel) GetCrcString() string {
	return "2ffbba5a"
}

// SrMulticastMapAddDelReply is the Go representation of the VPP binary API message 'sr_multicast_map_add_del_reply'.
type SrMulticastMapAddDelReply struct {
	Retval int32
}

func (*SrMulticastMapAddDelReply) GetMessageName() string {
	return "sr_multicast_map_add_del_reply"
}
func (*SrMulticastMapAddDelReply) GetCrcString() string {
	return "58b0c821"
}

// SwInterfaceSetVpath is the Go representation of the VPP binary API message 'sw_interface_set_vpath'.
type SwInterfaceSetVpath struct {
	SwIfIndex uint32
	Enable    uint8
}

func (*SwInterfaceSetVpath) GetMessageName() string {
	return "sw_interface_set_vpath"
}
func (*SwInterfaceSetVpath) GetCrcString() string {
	return "1bc2fd5e"
}

// SwInterfaceSetVpathReply is the Go representation of the VPP binary API message 'sw_interface_set_vpath_reply'.
type SwInterfaceSetVpathReply struct {
	Retval int32
}

func (*SwInterfaceSetVpathReply) GetMessageName() string {
	return "sw_interface_set_vpath_reply"
}
func (*SwInterfaceSetVpathReply) GetCrcString() string {
	return "828dbe62"
}

// SwInterfaceSetVxlanBypass is the Go representation of the VPP binary API message 'sw_interface_set_vxlan_bypass'.
type SwInterfaceSetVxlanBypass struct {
	SwIfIndex uint32
	IsIpv6    uint8
	Enable    uint8
}

func (*SwInterfaceSetVxlanBypass) GetMessageName() string {
	return "sw_interface_set_vxlan_bypass"
}
func (*SwInterfaceSetVxlanBypass) GetCrcString() string {
	return "da63ecfd"
}

// SwInterfaceSetVxlanBypassReply is the Go representation of the VPP binary API message 'sw_interface_set_vxlan_bypass_reply'.
type SwInterfaceSetVxlanBypassReply struct {
	Retval int32
}

func (*SwInterfaceSetVxlanBypassReply) GetMessageName() string {
	return "sw_interface_set_vxlan_bypass_reply"
}
func (*SwInterfaceSetVxlanBypassReply) GetCrcString() string {
	return "c4609ab5"
}

// SwInterfaceSetL2Xconnect is the Go representation of the VPP binary API message 'sw_interface_set_l2_xconnect'.
type SwInterfaceSetL2Xconnect struct {
	RxSwIfIndex uint32
	TxSwIfIndex uint32
	Enable      uint8
}

func (*SwInterfaceSetL2Xconnect) GetMessageName() string {
	return "sw_interface_set_l2_xconnect"
}
func (*SwInterfaceSetL2Xconnect) GetCrcString() string {
	return "48a4c4c8"
}

// SwInterfaceSetL2XconnectReply is the Go representation of the VPP binary API message 'sw_interface_set_l2_xconnect_reply'.
type SwInterfaceSetL2XconnectReply struct {
	Retval int32
}

func (*SwInterfaceSetL2XconnectReply) GetMessageName() string {
	return "sw_interface_set_l2_xconnect_reply"
}
func (*SwInterfaceSetL2XconnectReply) GetCrcString() string {
	return "6e45eed4"
}

// SwInterfaceSetL2Bridge is the Go representation of the VPP binary API message 'sw_interface_set_l2_bridge'.
type SwInterfaceSetL2Bridge struct {
	RxSwIfIndex uint32
	BdID        uint32
	Shg         uint8
	Bvi         uint8
	Enable      uint8
}

func (*SwInterfaceSetL2Bridge) GetMessageName() string {
	return "sw_interface_set_l2_bridge"
}
func (*SwInterfaceSetL2Bridge) GetCrcString() string {
	return "36c739e8"
}

// SwInterfaceSetL2BridgeReply is the Go representation of the VPP binary API message 'sw_interface_set_l2_bridge_reply'.
type SwInterfaceSetL2BridgeReply struct {
	Retval int32
}

func (*SwInterfaceSetL2BridgeReply) GetMessageName() string {
	return "sw_interface_set_l2_bridge_reply"
}
func (*SwInterfaceSetL2BridgeReply) GetCrcString() string {
	return "347e08d9"
}

// L2fibAddDel is the Go representation of the VPP binary API message 'l2fib_add_del'.
type L2fibAddDel struct {
	Mac       uint64
	BdID      uint32
	SwIfIndex uint32
	IsAdd     uint8
	StaticMac uint8
	FilterMac uint8
	BviMac    uint8
}

func (*L2fibAddDel) GetMessageName() string {
	return "l2fib_add_del"
}
func (*L2fibAddDel) GetCrcString() string {
	return "604cc582"
}

// L2fibAddDelReply is the Go representation of the VPP binary API message 'l2fib_add_del_reply'.
type L2fibAddDelReply struct {
	Retval int32
}

func (*L2fibAddDelReply) GetMessageName() string {
	return "l2fib_add_del_reply"
}
func (*L2fibAddDelReply) GetCrcString() string {
	return "1be0875a"
}

// L2Flags is the Go representation of the VPP binary API message 'l2_flags'.
type L2Flags struct {
	SwIfIndex     uint32
	IsSet         uint8
	FeatureBitmap uint32
}

func (*L2Flags) GetMessageName() string {
	return "l2_flags"
}
func (*L2Flags) GetCrcString() string {
	return "987fb8e1"
}

// L2FlagsReply is the Go representation of the VPP binary API message 'l2_flags_reply'.
type L2FlagsReply struct {
	Retval                 int32
	ResultingFeatureBitmap uint32
}

func (*L2FlagsReply) GetMessageName() string {
	return "l2_flags_reply"
}
func (*L2FlagsReply) GetCrcString() string {
	return "bd749594"
}

// BridgeFlags is the Go representation of the VPP binary API message 'bridge_flags'.
type BridgeFlags struct {
	BdID          uint32
	IsSet         uint8
	FeatureBitmap uint32
}

func (*BridgeFlags) GetMessageName() string {
	return "bridge_flags"
}
func (*BridgeFlags) GetCrcString() string {
	return "c1d50251"
}

// BridgeFlagsReply is the Go representation of the VPP binary API message 'bridge_flags_reply'.
type BridgeFlagsReply struct {
	Retval                 int32
	ResultingFeatureBitmap uint32
}

func (*BridgeFlagsReply) GetMessageName() string {
	return "bridge_flags_reply"
}
func (*BridgeFlagsReply) GetCrcString() string {
	return "fa6b7397"
}

// BdIPMacAddDel is the Go representation of the VPP binary API message 'bd_ip_mac_add_del'.
type BdIPMacAddDel struct {
	BdID       uint32
	IsAdd      uint8
	IsIpv6     uint8
	IPAddress  []byte `struc:"[16]byte"`
	MacAddress []byte `struc:"[6]byte"`
}

func (*BdIPMacAddDel) GetMessageName() string {
	return "bd_ip_mac_add_del"
}
func (*BdIPMacAddDel) GetCrcString() string {
	return "ad819817"
}

// BdIPMacAddDelReply is the Go representation of the VPP binary API message 'bd_ip_mac_add_del_reply'.
type BdIPMacAddDelReply struct {
	Retval int32
}

func (*BdIPMacAddDelReply) GetMessageName() string {
	return "bd_ip_mac_add_del_reply"
}
func (*BdIPMacAddDelReply) GetCrcString() string {
	return "55bab3b4"
}

// ClassifyAddDelTable is the Go representation of the VPP binary API message 'classify_add_del_table'.
type ClassifyAddDelTable struct {
	IsAdd             uint8
	DelChain          uint8
	TableIndex        uint32
	Nbuckets          uint32
	MemorySize        uint32
	SkipNVectors      uint32
	MatchNVectors     uint32
	NextTableIndex    uint32
	MissNextIndex     uint32
	CurrentDataFlag   uint32
	CurrentDataOffset int32 `struc:"sizeof=Mask"`
	Mask              []byte
}

func (*ClassifyAddDelTable) GetMessageName() string {
	return "classify_add_del_table"
}
func (*ClassifyAddDelTable) GetCrcString() string {
	return "1120f35d"
}

// ClassifyAddDelTableReply is the Go representation of the VPP binary API message 'classify_add_del_table_reply'.
type ClassifyAddDelTableReply struct {
	Retval        int32
	NewTableIndex uint32
	SkipNVectors  uint32
	MatchNVectors uint32
}

func (*ClassifyAddDelTableReply) GetMessageName() string {
	return "classify_add_del_table_reply"
}
func (*ClassifyAddDelTableReply) GetCrcString() string {
	return "d4e63320"
}

// ClassifyAddDelSession is the Go representation of the VPP binary API message 'classify_add_del_session'.
type ClassifyAddDelSession struct {
	IsAdd        uint8
	TableIndex   uint32
	HitNextIndex uint32
	OpaqueIndex  uint32
	Advance      int32
	Action       uint8
	Metadata     uint32 `struc:"sizeof=Match"`
	Match        []byte
}

func (*ClassifyAddDelSession) GetMessageName() string {
	return "classify_add_del_session"
}
func (*ClassifyAddDelSession) GetCrcString() string {
	return "25a952f5"
}

// ClassifyAddDelSessionReply is the Go representation of the VPP binary API message 'classify_add_del_session_reply'.
type ClassifyAddDelSessionReply struct {
	Retval int32
}

func (*ClassifyAddDelSessionReply) GetMessageName() string {
	return "classify_add_del_session_reply"
}
func (*ClassifyAddDelSessionReply) GetCrcString() string {
	return "dd4aa9ac"
}

// ClassifySetInterfaceIPTable is the Go representation of the VPP binary API message 'classify_set_interface_ip_table'.
type ClassifySetInterfaceIPTable struct {
	IsIpv6     uint8
	SwIfIndex  uint32
	TableIndex uint32
}

func (*ClassifySetInterfaceIPTable) GetMessageName() string {
	return "classify_set_interface_ip_table"
}
func (*ClassifySetInterfaceIPTable) GetCrcString() string {
	return "0dc45308"
}

// ClassifySetInterfaceIPTableReply is the Go representation of the VPP binary API message 'classify_set_interface_ip_table_reply'.
type ClassifySetInterfaceIPTableReply struct {
	Retval int32
}

func (*ClassifySetInterfaceIPTableReply) GetMessageName() string {
	return "classify_set_interface_ip_table_reply"
}
func (*ClassifySetInterfaceIPTableReply) GetCrcString() string {
	return "dc391c34"
}

// ClassifySetInterfaceL2Tables is the Go representation of the VPP binary API message 'classify_set_interface_l2_tables'.
type ClassifySetInterfaceL2Tables struct {
	SwIfIndex       uint32
	IP4TableIndex   uint32
	IP6TableIndex   uint32
	OtherTableIndex uint32
	IsInput         uint8
}

func (*ClassifySetInterfaceL2Tables) GetMessageName() string {
	return "classify_set_interface_l2_tables"
}
func (*ClassifySetInterfaceL2Tables) GetCrcString() string {
	return "ed9ccf0d"
}

// ClassifySetInterfaceL2TablesReply is the Go representation of the VPP binary API message 'classify_set_interface_l2_tables_reply'.
type ClassifySetInterfaceL2TablesReply struct {
	Retval int32
}

func (*ClassifySetInterfaceL2TablesReply) GetMessageName() string {
	return "classify_set_interface_l2_tables_reply"
}
func (*ClassifySetInterfaceL2TablesReply) GetCrcString() string {
	return "8df20579"
}

// GetNodeIndex is the Go representation of the VPP binary API message 'get_node_index'.
type GetNodeIndex struct {
	NodeName []byte `struc:"[64]byte"`
}

func (*GetNodeIndex) GetMessageName() string {
	return "get_node_index"
}
func (*GetNodeIndex) GetCrcString() string {
	return "226d3f8c"
}

// GetNodeIndexReply is the Go representation of the VPP binary API message 'get_node_index_reply'.
type GetNodeIndexReply struct {
	Retval    int32
	NodeIndex uint32
}

func (*GetNodeIndexReply) GetMessageName() string {
	return "get_node_index_reply"
}
func (*GetNodeIndexReply) GetCrcString() string {
	return "29116865"
}

// AddNodeNext is the Go representation of the VPP binary API message 'add_node_next'.
type AddNodeNext struct {
	NodeName []byte `struc:"[64]byte"`
	NextName []byte `struc:"[64]byte"`
}

func (*AddNodeNext) GetMessageName() string {
	return "add_node_next"
}
func (*AddNodeNext) GetCrcString() string {
	return "e4202993"
}

// AddNodeNextReply is the Go representation of the VPP binary API message 'add_node_next_reply'.
type AddNodeNextReply struct {
	Retval    int32
	NextIndex uint32
}

func (*AddNodeNextReply) GetMessageName() string {
	return "add_node_next_reply"
}
func (*AddNodeNextReply) GetCrcString() string {
	return "e89d6eed"
}

// DhcpProxyConfig2 is the Go representation of the VPP binary API message 'dhcp_proxy_config_2'.
type DhcpProxyConfig2 struct {
	RxVrfID         uint32
	ServerVrfID     uint32
	IsIpv6          uint8
	IsAdd           uint8
	InsertCircuitID uint8
	DhcpServer      []byte `struc:"[16]byte"`
	DhcpSrcAddress  []byte `struc:"[16]byte"`
}

func (*DhcpProxyConfig2) GetMessageName() string {
	return "dhcp_proxy_config_2"
}
func (*DhcpProxyConfig2) GetCrcString() string {
	return "7cfeb2d1"
}

// DhcpProxyConfig2Reply is the Go representation of the VPP binary API message 'dhcp_proxy_config_2_reply'.
type DhcpProxyConfig2Reply struct {
	Retval int32
}

func (*DhcpProxyConfig2Reply) GetMessageName() string {
	return "dhcp_proxy_config_2_reply"
}
func (*DhcpProxyConfig2Reply) GetCrcString() string {
	return "cfb5aca9"
}

// L2tpv3CreateTunnel is the Go representation of the VPP binary API message 'l2tpv3_create_tunnel'.
type L2tpv3CreateTunnel struct {
	ClientAddress     []byte `struc:"[16]byte"`
	OurAddress        []byte `struc:"[16]byte"`
	IsIpv6            uint8
	LocalSessionID    uint32
	RemoteSessionID   uint32
	LocalCookie       uint64
	RemoteCookie      uint64
	L2SublayerPresent uint8
	EncapVrfID        uint32
}

func (*L2tpv3CreateTunnel) GetMessageName() string {
	return "l2tpv3_create_tunnel"
}
func (*L2tpv3CreateTunnel) GetCrcString() string {
	return "d9839424"
}

// L2tpv3CreateTunnelReply is the Go representation of the VPP binary API message 'l2tpv3_create_tunnel_reply'.
type L2tpv3CreateTunnelReply struct {
	Retval    int32
	SwIfIndex uint32
}

func (*L2tpv3CreateTunnelReply) GetMessageName() string {
	return "l2tpv3_create_tunnel_reply"
}
func (*L2tpv3CreateTunnelReply) GetCrcString() string {
	return "7007338f"
}

// L2tpv3SetTunnelCookies is the Go representation of the VPP binary API message 'l2tpv3_set_tunnel_cookies'.
type L2tpv3SetTunnelCookies struct {
	SwIfIndex       uint32
	NewLocalCookie  uint64
	NewRemoteCookie uint64
}

func (*L2tpv3SetTunnelCookies) GetMessageName() string {
	return "l2tpv3_set_tunnel_cookies"
}
func (*L2tpv3SetTunnelCookies) GetCrcString() string {
	return "3e783c95"
}

// L2tpv3SetTunnelCookiesReply is the Go representation of the VPP binary API message 'l2tpv3_set_tunnel_cookies_reply'.
type L2tpv3SetTunnelCookiesReply struct {
	Retval int32
}

func (*L2tpv3SetTunnelCookiesReply) GetMessageName() string {
	return "l2tpv3_set_tunnel_cookies_reply"
}
func (*L2tpv3SetTunnelCookiesReply) GetCrcString() string {
	return "0219718b"
}

// SwIfL2tpv3TunnelDetails is the Go representation of the VPP binary API message 'sw_if_l2tpv3_tunnel_details'.
type SwIfL2tpv3TunnelDetails struct {
	SwIfIndex         uint32
	InterfaceName     []byte `struc:"[64]byte"`
	ClientAddress     []byte `struc:"[16]byte"`
	OurAddress        []byte `struc:"[16]byte"`
	LocalSessionID    uint32
	RemoteSessionID   uint32
	LocalCookie       []uint64 `struc:"[2]uint64"`
	RemoteCookie      uint64
	L2SublayerPresent uint8
}

func (*SwIfL2tpv3TunnelDetails) GetMessageName() string {
	return "sw_if_l2tpv3_tunnel_details"
}
func (*SwIfL2tpv3TunnelDetails) GetCrcString() string {
	return "6e3e43d4"
}

// SwIfL2tpv3TunnelDump is the Go representation of the VPP binary API message 'sw_if_l2tpv3_tunnel_dump'.
type SwIfL2tpv3TunnelDump struct {
}

func (*SwIfL2tpv3TunnelDump) GetMessageName() string {
	return "sw_if_l2tpv3_tunnel_dump"
}
func (*SwIfL2tpv3TunnelDump) GetCrcString() string {
	return "597e7092"
}

// L2FibClearTable is the Go representation of the VPP binary API message 'l2_fib_clear_table'.
type L2FibClearTable struct {
}

func (*L2FibClearTable) GetMessageName() string {
	return "l2_fib_clear_table"
}
func (*L2FibClearTable) GetCrcString() string {
	return "40dc61e3"
}

// L2FibClearTableReply is the Go representation of the VPP binary API message 'l2_fib_clear_table_reply'.
type L2FibClearTableReply struct {
	Retval int32
}

func (*L2FibClearTableReply) GetMessageName() string {
	return "l2_fib_clear_table_reply"
}
func (*L2FibClearTableReply) GetCrcString() string {
	return "0425b038"
}

// L2InterfaceEfpFilter is the Go representation of the VPP binary API message 'l2_interface_efp_filter'.
type L2InterfaceEfpFilter struct {
	SwIfIndex     uint32
	EnableDisable uint32
}

func (*L2InterfaceEfpFilter) GetMessageName() string {
	return "l2_interface_efp_filter"
}
func (*L2InterfaceEfpFilter) GetCrcString() string {
	return "07c9d601"
}

// L2InterfaceEfpFilterReply is the Go representation of the VPP binary API message 'l2_interface_efp_filter_reply'.
type L2InterfaceEfpFilterReply struct {
	Retval int32
}

func (*L2InterfaceEfpFilterReply) GetMessageName() string {
	return "l2_interface_efp_filter_reply"
}
func (*L2InterfaceEfpFilterReply) GetCrcString() string {
	return "0f4bb0c0"
}

// L2tpv3InterfaceEnableDisable is the Go representation of the VPP binary API message 'l2tpv3_interface_enable_disable'.
type L2tpv3InterfaceEnableDisable struct {
	EnableDisable uint8
	SwIfIndex     uint32
}

func (*L2tpv3InterfaceEnableDisable) GetMessageName() string {
	return "l2tpv3_interface_enable_disable"
}
func (*L2tpv3InterfaceEnableDisable) GetCrcString() string {
	return "c74900bf"
}

// L2tpv3InterfaceEnableDisableReply is the Go representation of the VPP binary API message 'l2tpv3_interface_enable_disable_reply'.
type L2tpv3InterfaceEnableDisableReply struct {
	Retval int32
}

func (*L2tpv3InterfaceEnableDisableReply) GetMessageName() string {
	return "l2tpv3_interface_enable_disable_reply"
}
func (*L2tpv3InterfaceEnableDisableReply) GetCrcString() string {
	return "541a4157"
}

// L2tpv3SetLookupKey is the Go representation of the VPP binary API message 'l2tpv3_set_lookup_key'.
type L2tpv3SetLookupKey struct {
	Key uint8
}

func (*L2tpv3SetLookupKey) GetMessageName() string {
	return "l2tpv3_set_lookup_key"
}
func (*L2tpv3SetLookupKey) GetCrcString() string {
	return "b7152584"
}

// L2tpv3SetLookupKeyReply is the Go representation of the VPP binary API message 'l2tpv3_set_lookup_key_reply'.
type L2tpv3SetLookupKeyReply struct {
	Retval int32
}

func (*L2tpv3SetLookupKeyReply) GetMessageName() string {
	return "l2tpv3_set_lookup_key_reply"
}
func (*L2tpv3SetLookupKeyReply) GetCrcString() string {
	return "300e69f4"
}

// VxlanAddDelTunnel is the Go representation of the VPP binary API message 'vxlan_add_del_tunnel'.
type VxlanAddDelTunnel struct {
	IsAdd          uint8
	IsIpv6         uint8
	SrcAddress     []byte `struc:"[16]byte"`
	DstAddress     []byte `struc:"[16]byte"`
	McastSwIfIndex uint32
	EncapVrfID     uint32
	DecapNextIndex uint32
	Vni            uint32
}

func (*VxlanAddDelTunnel) GetMessageName() string {
	return "vxlan_add_del_tunnel"
}
func (*VxlanAddDelTunnel) GetCrcString() string {
	return "79be0753"
}

// VxlanAddDelTunnelReply is the Go representation of the VPP binary API message 'vxlan_add_del_tunnel_reply'.
type VxlanAddDelTunnelReply struct {
	Retval    int32
	SwIfIndex uint32
}

func (*VxlanAddDelTunnelReply) GetMessageName() string {
	return "vxlan_add_del_tunnel_reply"
}
func (*VxlanAddDelTunnelReply) GetCrcString() string {
	return "3965e5df"
}

// VxlanTunnelDump is the Go representation of the VPP binary API message 'vxlan_tunnel_dump'.
type VxlanTunnelDump struct {
	SwIfIndex uint32
}

func (*VxlanTunnelDump) GetMessageName() string {
	return "vxlan_tunnel_dump"
}
func (*VxlanTunnelDump) GetCrcString() string {
	return "7d29e867"
}

// VxlanTunnelDetails is the Go representation of the VPP binary API message 'vxlan_tunnel_details'.
type VxlanTunnelDetails struct {
	SwIfIndex      uint32
	SrcAddress     []byte `struc:"[16]byte"`
	DstAddress     []byte `struc:"[16]byte"`
	McastSwIfIndex uint32
	EncapVrfID     uint32
	DecapNextIndex uint32
	Vni            uint32
	IsIpv6         uint8
}

func (*VxlanTunnelDetails) GetMessageName() string {
	return "vxlan_tunnel_details"
}
func (*VxlanTunnelDetails) GetCrcString() string {
	return "fa28d42c"
}

// GreAddDelTunnel is the Go representation of the VPP binary API message 'gre_add_del_tunnel'.
type GreAddDelTunnel struct {
	IsAdd      uint8
	IsIpv6     uint8
	Teb        uint8
	SrcAddress []byte `struc:"[16]byte"`
	DstAddress []byte `struc:"[16]byte"`
	OuterFibID uint32
}

func (*GreAddDelTunnel) GetMessageName() string {
	return "gre_add_del_tunnel"
}
func (*GreAddDelTunnel) GetCrcString() string {
	return "8ab92528"
}

// GreAddDelTunnelReply is the Go representation of the VPP binary API message 'gre_add_del_tunnel_reply'.
type GreAddDelTunnelReply struct {
	Retval    int32
	SwIfIndex uint32
}

func (*GreAddDelTunnelReply) GetMessageName() string {
	return "gre_add_del_tunnel_reply"
}
func (*GreAddDelTunnelReply) GetCrcString() string {
	return "754a5956"
}

// GreTunnelDump is the Go representation of the VPP binary API message 'gre_tunnel_dump'.
type GreTunnelDump struct {
	SwIfIndex uint32
}

func (*GreTunnelDump) GetMessageName() string {
	return "gre_tunnel_dump"
}
func (*GreTunnelDump) GetCrcString() string {
	return "23d04dc0"
}

// GreTunnelDetails is the Go representation of the VPP binary API message 'gre_tunnel_details'.
type GreTunnelDetails struct {
	SwIfIndex  uint32
	IsIpv6     uint8
	Teb        uint8
	SrcAddress []byte `struc:"[16]byte"`
	DstAddress []byte `struc:"[16]byte"`
	OuterFibID uint32
}

func (*GreTunnelDetails) GetMessageName() string {
	return "gre_tunnel_details"
}
func (*GreTunnelDetails) GetCrcString() string {
	return "dd1d50aa"
}

// L2InterfaceVlanTagRewrite is the Go representation of the VPP binary API message 'l2_interface_vlan_tag_rewrite'.
type L2InterfaceVlanTagRewrite struct {
	SwIfIndex uint32
	VtrOp     uint32
	PushDot1q uint32
	Tag1      uint32
	Tag2      uint32
}

func (*L2InterfaceVlanTagRewrite) GetMessageName() string {
	return "l2_interface_vlan_tag_rewrite"
}
func (*L2InterfaceVlanTagRewrite) GetCrcString() string {
	return "b9dcbd39"
}

// L2InterfaceVlanTagRewriteReply is the Go representation of the VPP binary API message 'l2_interface_vlan_tag_rewrite_reply'.
type L2InterfaceVlanTagRewriteReply struct {
	Retval int32
}

func (*L2InterfaceVlanTagRewriteReply) GetMessageName() string {
	return "l2_interface_vlan_tag_rewrite_reply"
}
func (*L2InterfaceVlanTagRewriteReply) GetCrcString() string {
	return "901eddfb"
}

// CreateVhostUserIf is the Go representation of the VPP binary API message 'create_vhost_user_if'.
type CreateVhostUserIf struct {
	IsServer          uint8
	SockFilename      []byte `struc:"[256]byte"`
	Renumber          uint8
	CustomDevInstance uint32
	UseCustomMac      uint8
	MacAddress        []byte `struc:"[6]byte"`
	Tag               []byte `struc:"[64]byte"`
}

func (*CreateVhostUserIf) GetMessageName() string {
	return "create_vhost_user_if"
}
func (*CreateVhostUserIf) GetCrcString() string {
	return "65861d57"
}

// CreateVhostUserIfReply is the Go representation of the VPP binary API message 'create_vhost_user_if_reply'.
type CreateVhostUserIfReply struct {
	Retval    int32
	SwIfIndex uint32
}

func (*CreateVhostUserIfReply) GetMessageName() string {
	return "create_vhost_user_if_reply"
}
func (*CreateVhostUserIfReply) GetCrcString() string {
	return "20c3fea9"
}

// ModifyVhostUserIf is the Go representation of the VPP binary API message 'modify_vhost_user_if'.
type ModifyVhostUserIf struct {
	SwIfIndex         uint32
	IsServer          uint8
	SockFilename      []byte `struc:"[256]byte"`
	Renumber          uint8
	CustomDevInstance uint32
}

func (*ModifyVhostUserIf) GetMessageName() string {
	return "modify_vhost_user_if"
}
func (*ModifyVhostUserIf) GetCrcString() string {
	return "2ee245ad"
}

// ModifyVhostUserIfReply is the Go representation of the VPP binary API message 'modify_vhost_user_if_reply'.
type ModifyVhostUserIfReply struct {
	Retval int32
}

func (*ModifyVhostUserIfReply) GetMessageName() string {
	return "modify_vhost_user_if_reply"
}
func (*ModifyVhostUserIfReply) GetCrcString() string {
	return "dc039556"
}

// DeleteVhostUserIf is the Go representation of the VPP binary API message 'delete_vhost_user_if'.
type DeleteVhostUserIf struct {
	SwIfIndex uint32
}

func (*DeleteVhostUserIf) GetMessageName() string {
	return "delete_vhost_user_if"
}
func (*DeleteVhostUserIf) GetCrcString() string {
	return "ed230259"
}

// DeleteVhostUserIfReply is the Go representation of the VPP binary API message 'delete_vhost_user_if_reply'.
type DeleteVhostUserIfReply struct {
	Retval int32
}

func (*DeleteVhostUserIfReply) GetMessageName() string {
	return "delete_vhost_user_if_reply"
}
func (*DeleteVhostUserIfReply) GetCrcString() string {
	return "2d7b6fe6"
}

// CreateSubif is the Go representation of the VPP binary API message 'create_subif'.
type CreateSubif struct {
	SwIfIndex      uint32
	SubID          uint32
	NoTags         uint8
	OneTag         uint8
	TwoTags        uint8
	Dot1ad         uint8
	ExactMatch     uint8
	DefaultSub     uint8
	OuterVlanIDAny uint8
	InnerVlanIDAny uint8
	OuterVlanID    uint16
	InnerVlanID    uint16
}

func (*CreateSubif) GetMessageName() string {
	return "create_subif"
}
func (*CreateSubif) GetCrcString() string {
	return "150e6757"
}

// CreateSubifReply is the Go representation of the VPP binary API message 'create_subif_reply'.
type CreateSubifReply struct {
	Retval    int32
	SwIfIndex uint32
}

func (*CreateSubifReply) GetMessageName() string {
	return "create_subif_reply"
}
func (*CreateSubifReply) GetCrcString() string {
	return "92272bcb"
}

// ShowVersion is the Go representation of the VPP binary API message 'show_version'.
type ShowVersion struct {
}

func (*ShowVersion) GetMessageName() string {
	return "show_version"
}
func (*ShowVersion) GetCrcString() string {
	return "f18f9480"
}

// ShowVersionReply is the Go representation of the VPP binary API message 'show_version_reply'.
type ShowVersionReply struct {
	Retval         int32
	Program        []byte `struc:"[32]byte"`
	Version        []byte `struc:"[32]byte"`
	BuildDate      []byte `struc:"[32]byte"`
	BuildDirectory []byte `struc:"[256]byte"`
}

func (*ShowVersionReply) GetMessageName() string {
	return "show_version_reply"
}
func (*ShowVersionReply) GetCrcString() string {
	return "83186d9e"
}

// SwInterfaceVhostUserDetails is the Go representation of the VPP binary API message 'sw_interface_vhost_user_details'.
type SwInterfaceVhostUserDetails struct {
	SwIfIndex      uint32
	InterfaceName  []byte `struc:"[64]byte"`
	VirtioNetHdrSz uint32
	Features       uint64
	IsServer       uint8
	SockFilename   []byte `struc:"[256]byte"`
	NumRegions     uint32
	SockErrno      int32
}

func (*SwInterfaceVhostUserDetails) GetMessageName() string {
	return "sw_interface_vhost_user_details"
}
func (*SwInterfaceVhostUserDetails) GetCrcString() string {
	return "130369a8"
}

// SwInterfaceVhostUserDump is the Go representation of the VPP binary API message 'sw_interface_vhost_user_dump'.
type SwInterfaceVhostUserDump struct {
}

func (*SwInterfaceVhostUserDump) GetMessageName() string {
	return "sw_interface_vhost_user_dump"
}
func (*SwInterfaceVhostUserDump) GetCrcString() string {
	return "da1923e2"
}

// L2FibTableEntry is the Go representation of the VPP binary API message 'l2_fib_table_entry'.
type L2FibTableEntry struct {
	BdID      uint32
	Mac       uint64
	SwIfIndex uint32
	StaticMac uint8
	FilterMac uint8
	BviMac    uint8
}

func (*L2FibTableEntry) GetMessageName() string {
	return "l2_fib_table_entry"
}
func (*L2FibTableEntry) GetCrcString() string {
	return "9d6b8da5"
}

// L2FibTableDump is the Go representation of the VPP binary API message 'l2_fib_table_dump'.
type L2FibTableDump struct {
	BdID uint32
}

func (*L2FibTableDump) GetMessageName() string {
	return "l2_fib_table_dump"
}
func (*L2FibTableDump) GetCrcString() string {
	return "edcbdcf6"
}

// VxlanGpeAddDelTunnel is the Go representation of the VPP binary API message 'vxlan_gpe_add_del_tunnel'.
type VxlanGpeAddDelTunnel struct {
	IsIpv6     uint8
	Local      []byte `struc:"[16]byte"`
	Remote     []byte `struc:"[16]byte"`
	EncapVrfID uint32
	DecapVrfID uint32
	Protocol   uint8
	Vni        uint32
	IsAdd      uint8
}

func (*VxlanGpeAddDelTunnel) GetMessageName() string {
	return "vxlan_gpe_add_del_tunnel"
}
func (*VxlanGpeAddDelTunnel) GetCrcString() string {
	return "39488c0f"
}

// VxlanGpeAddDelTunnelReply is the Go representation of the VPP binary API message 'vxlan_gpe_add_del_tunnel_reply'.
type VxlanGpeAddDelTunnelReply struct {
	Retval    int32
	SwIfIndex uint32
}

func (*VxlanGpeAddDelTunnelReply) GetMessageName() string {
	return "vxlan_gpe_add_del_tunnel_reply"
}
func (*VxlanGpeAddDelTunnelReply) GetCrcString() string {
	return "563fedf9"
}

// VxlanGpeTunnelDump is the Go representation of the VPP binary API message 'vxlan_gpe_tunnel_dump'.
type VxlanGpeTunnelDump struct {
	SwIfIndex uint32
}

func (*VxlanGpeTunnelDump) GetMessageName() string {
	return "vxlan_gpe_tunnel_dump"
}
func (*VxlanGpeTunnelDump) GetCrcString() string {
	return "14423443"
}

// VxlanGpeTunnelDetails is the Go representation of the VPP binary API message 'vxlan_gpe_tunnel_details'.
type VxlanGpeTunnelDetails struct {
	SwIfIndex  uint32
	Local      []byte `struc:"[16]byte"`
	Remote     []byte `struc:"[16]byte"`
	Vni        uint32
	Protocol   uint8
	EncapVrfID uint32
	DecapVrfID uint32
	IsIpv6     uint8
}

func (*VxlanGpeTunnelDetails) GetMessageName() string {
	return "vxlan_gpe_tunnel_details"
}
func (*VxlanGpeTunnelDetails) GetCrcString() string {
	return "da8ca593"
}

// LispAddDelLocatorSet is the Go representation of the VPP binary API message 'lisp_add_del_locator_set'.
type LispAddDelLocatorSet struct {
	IsAdd          uint8
	LocatorSetName []byte `struc:"[64]byte"`
	LocatorNum     uint32 `struc:"sizeof=Locators"`
	Locators       []byte
}

func (*LispAddDelLocatorSet) GetMessageName() string {
	return "lisp_add_del_locator_set"
}
func (*LispAddDelLocatorSet) GetCrcString() string {
	return "2fd73ab7"
}

// LispAddDelLocatorSetReply is the Go representation of the VPP binary API message 'lisp_add_del_locator_set_reply'.
type LispAddDelLocatorSetReply struct {
	Retval  int32
	LsIndex uint32
}

func (*LispAddDelLocatorSetReply) GetMessageName() string {
	return "lisp_add_del_locator_set_reply"
}
func (*LispAddDelLocatorSetReply) GetCrcString() string {
	return "c167cab1"
}

// LispAddDelLocator is the Go representation of the VPP binary API message 'lisp_add_del_locator'.
type LispAddDelLocator struct {
	IsAdd          uint8
	LocatorSetName []byte `struc:"[64]byte"`
	SwIfIndex      uint32
	Priority       uint8
	Weight         uint8
}

func (*LispAddDelLocator) GetMessageName() string {
	return "lisp_add_del_locator"
}
func (*LispAddDelLocator) GetCrcString() string {
	return "442b7292"
}

// LispAddDelLocatorReply is the Go representation of the VPP binary API message 'lisp_add_del_locator_reply'.
type LispAddDelLocatorReply struct {
	Retval int32
}

func (*LispAddDelLocatorReply) GetMessageName() string {
	return "lisp_add_del_locator_reply"
}
func (*LispAddDelLocatorReply) GetCrcString() string {
	return "89ce7940"
}

// LispAddDelLocalEid is the Go representation of the VPP binary API message 'lisp_add_del_local_eid'.
type LispAddDelLocalEid struct {
	IsAdd          uint8
	EidType        uint8
	Eid            []byte `struc:"[16]byte"`
	PrefixLen      uint8
	LocatorSetName []byte `struc:"[64]byte"`
	Vni            uint32
	KeyID          uint16
	Key            []byte `struc:"[64]byte"`
}

func (*LispAddDelLocalEid) GetMessageName() string {
	return "lisp_add_del_local_eid"
}
func (*LispAddDelLocalEid) GetCrcString() string {
	return "c0af8d48"
}

// LispAddDelLocalEidReply is the Go representation of the VPP binary API message 'lisp_add_del_local_eid_reply'.
type LispAddDelLocalEidReply struct {
	Retval int32
}

func (*LispAddDelLocalEidReply) GetMessageName() string {
	return "lisp_add_del_local_eid_reply"
}
func (*LispAddDelLocalEidReply) GetCrcString() string {
	return "d4860470"
}

// LispGpeAddDelFwdEntry is the Go representation of the VPP binary API message 'lisp_gpe_add_del_fwd_entry'.
type LispGpeAddDelFwdEntry struct {
	IsAdd   uint8
	EidType uint8
	RmtEid  []byte `struc:"[16]byte"`
	LclEid  []byte `struc:"[16]byte"`
	RmtLen  uint8
	LclLen  uint8
	Vni     uint32
	DpTable uint32
	LocNum  uint32 `struc:"sizeof=LclLocs"`
	LclLocs []byte `struc:"sizeof=RmtLocs"`
	RmtLocs []byte
	Action  uint8
}

func (*LispGpeAddDelFwdEntry) GetMessageName() string {
	return "lisp_gpe_add_del_fwd_entry"
}
func (*LispGpeAddDelFwdEntry) GetCrcString() string {
	return "639b4c51"
}

// LispGpeAddDelFwdEntryReply is the Go representation of the VPP binary API message 'lisp_gpe_add_del_fwd_entry_reply'.
type LispGpeAddDelFwdEntryReply struct {
	Retval int32
}

func (*LispGpeAddDelFwdEntryReply) GetMessageName() string {
	return "lisp_gpe_add_del_fwd_entry_reply"
}
func (*LispGpeAddDelFwdEntryReply) GetCrcString() string {
	return "0fe17e5a"
}

// LispAddDelMapServer is the Go representation of the VPP binary API message 'lisp_add_del_map_server'.
type LispAddDelMapServer struct {
	IsAdd     uint8
	IsIpv6    uint8
	IPAddress []byte `struc:"[16]byte"`
}

func (*LispAddDelMapServer) GetMessageName() string {
	return "lisp_add_del_map_server"
}
func (*LispAddDelMapServer) GetCrcString() string {
	return "592b70b3"
}

// LispAddDelMapServerReply is the Go representation of the VPP binary API message 'lisp_add_del_map_server_reply'.
type LispAddDelMapServerReply struct {
	Retval int32
}

func (*LispAddDelMapServerReply) GetMessageName() string {
	return "lisp_add_del_map_server_reply"
}
func (*LispAddDelMapServerReply) GetCrcString() string {
	return "26f8f732"
}

// LispAddDelMapResolver is the Go representation of the VPP binary API message 'lisp_add_del_map_resolver'.
type LispAddDelMapResolver struct {
	IsAdd     uint8
	IsIpv6    uint8
	IPAddress []byte `struc:"[16]byte"`
}

func (*LispAddDelMapResolver) GetMessageName() string {
	return "lisp_add_del_map_resolver"
}
func (*LispAddDelMapResolver) GetCrcString() string {
	return "1d0303ff"
}

// LispAddDelMapResolverReply is the Go representation of the VPP binary API message 'lisp_add_del_map_resolver_reply'.
type LispAddDelMapResolverReply struct {
	Retval int32
}

func (*LispAddDelMapResolverReply) GetMessageName() string {
	return "lisp_add_del_map_resolver_reply"
}
func (*LispAddDelMapResolverReply) GetCrcString() string {
	return "da4d72e0"
}

// LispGpeEnableDisable is the Go representation of the VPP binary API message 'lisp_gpe_enable_disable'.
type LispGpeEnableDisable struct {
	IsEn uint8
}

func (*LispGpeEnableDisable) GetMessageName() string {
	return "lisp_gpe_enable_disable"
}
func (*LispGpeEnableDisable) GetCrcString() string {
	return "128269e2"
}

// LispGpeEnableDisableReply is the Go representation of the VPP binary API message 'lisp_gpe_enable_disable_reply'.
type LispGpeEnableDisableReply struct {
	Retval int32
}

func (*LispGpeEnableDisableReply) GetMessageName() string {
	return "lisp_gpe_enable_disable_reply"
}
func (*LispGpeEnableDisableReply) GetCrcString() string {
	return "e526fbae"
}

// LispEnableDisable is the Go representation of the VPP binary API message 'lisp_enable_disable'.
type LispEnableDisable struct {
	IsEn uint8
}

func (*LispEnableDisable) GetMessageName() string {
	return "lisp_enable_disable"
}
func (*LispEnableDisable) GetCrcString() string {
	return "6c27720f"
}

// LispEnableDisableReply is the Go representation of the VPP binary API message 'lisp_enable_disable_reply'.
type LispEnableDisableReply struct {
	Retval int32
}

func (*LispEnableDisableReply) GetMessageName() string {
	return "lisp_enable_disable_reply"
}
func (*LispEnableDisableReply) GetCrcString() string {
	return "36406c38"
}

// LispGpeAddDelIface is the Go representation of the VPP binary API message 'lisp_gpe_add_del_iface'.
type LispGpeAddDelIface struct {
	IsAdd   uint8
	IsL2    uint8
	DpTable uint32
	Vni     uint32
}

func (*LispGpeAddDelIface) GetMessageName() string {
	return "lisp_gpe_add_del_iface"
}
func (*LispGpeAddDelIface) GetCrcString() string {
	return "f594a1c9"
}

// LispGpeAddDelIfaceReply is the Go representation of the VPP binary API message 'lisp_gpe_add_del_iface_reply'.
type LispGpeAddDelIfaceReply struct {
	Retval int32
}

func (*LispGpeAddDelIfaceReply) GetMessageName() string {
	return "lisp_gpe_add_del_iface_reply"
}
func (*LispGpeAddDelIfaceReply) GetCrcString() string {
	return "648509de"
}

// LispPitrSetLocatorSet is the Go representation of the VPP binary API message 'lisp_pitr_set_locator_set'.
type LispPitrSetLocatorSet struct {
	IsAdd  uint8
	LsName []byte `struc:"[64]byte"`
}

func (*LispPitrSetLocatorSet) GetMessageName() string {
	return "lisp_pitr_set_locator_set"
}
func (*LispPitrSetLocatorSet) GetCrcString() string {
	return "59fbff25"
}

// LispPitrSetLocatorSetReply is the Go representation of the VPP binary API message 'lisp_pitr_set_locator_set_reply'.
type LispPitrSetLocatorSetReply struct {
	Retval int32
}

func (*LispPitrSetLocatorSetReply) GetMessageName() string {
	return "lisp_pitr_set_locator_set_reply"
}
func (*LispPitrSetLocatorSetReply) GetCrcString() string {
	return "46d996c2"
}

// ShowLispRlocProbeState is the Go representation of the VPP binary API message 'show_lisp_rloc_probe_state'.
type ShowLispRlocProbeState struct {
}

func (*ShowLispRlocProbeState) GetMessageName() string {
	return "show_lisp_rloc_probe_state"
}
func (*ShowLispRlocProbeState) GetCrcString() string {
	return "c0b0f08b"
}

// ShowLispRlocProbeStateReply is the Go representation of the VPP binary API message 'show_lisp_rloc_probe_state_reply'.
type ShowLispRlocProbeStateReply struct {
	Retval    int32
	IsEnabled uint8
}

func (*ShowLispRlocProbeStateReply) GetMessageName() string {
	return "show_lisp_rloc_probe_state_reply"
}
func (*ShowLispRlocProbeStateReply) GetCrcString() string {
	return "23a1e712"
}

// LispRlocProbeEnableDisable is the Go representation of the VPP binary API message 'lisp_rloc_probe_enable_disable'.
type LispRlocProbeEnableDisable struct {
	IsEnabled uint8
}

func (*LispRlocProbeEnableDisable) GetMessageName() string {
	return "lisp_rloc_probe_enable_disable"
}
func (*LispRlocProbeEnableDisable) GetCrcString() string {
	return "32c270ac"
}

// LispRlocProbeEnableDisableReply is the Go representation of the VPP binary API message 'lisp_rloc_probe_enable_disable_reply'.
type LispRlocProbeEnableDisableReply struct {
	Retval int32
}

func (*LispRlocProbeEnableDisableReply) GetMessageName() string {
	return "lisp_rloc_probe_enable_disable_reply"
}
func (*LispRlocProbeEnableDisableReply) GetCrcString() string {
	return "97d05bc4"
}

// LispMapRegisterEnableDisable is the Go representation of the VPP binary API message 'lisp_map_register_enable_disable'.
type LispMapRegisterEnableDisable struct {
	IsEnabled uint8
}

func (*LispMapRegisterEnableDisable) GetMessageName() string {
	return "lisp_map_register_enable_disable"
}
func (*LispMapRegisterEnableDisable) GetCrcString() string {
	return "8d0a81ca"
}

// LispMapRegisterEnableDisableReply is the Go representation of the VPP binary API message 'lisp_map_register_enable_disable_reply'.
type LispMapRegisterEnableDisableReply struct {
	Retval int32
}

func (*LispMapRegisterEnableDisableReply) GetMessageName() string {
	return "lisp_map_register_enable_disable_reply"
}
func (*LispMapRegisterEnableDisableReply) GetCrcString() string {
	return "99e7a700"
}

// ShowLispMapRegisterState is the Go representation of the VPP binary API message 'show_lisp_map_register_state'.
type ShowLispMapRegisterState struct {
}

func (*ShowLispMapRegisterState) GetMessageName() string {
	return "show_lisp_map_register_state"
}
func (*ShowLispMapRegisterState) GetCrcString() string {
	return "55fc9581"
}

// ShowLispMapRegisterStateReply is the Go representation of the VPP binary API message 'show_lisp_map_register_state_reply'.
type ShowLispMapRegisterStateReply struct {
	Retval    int32
	IsEnabled uint8
}

func (*ShowLispMapRegisterStateReply) GetMessageName() string {
	return "show_lisp_map_register_state_reply"
}
func (*ShowLispMapRegisterStateReply) GetCrcString() string {
	return "8d04052e"
}

// LispMapRequestMode is the Go representation of the VPP binary API message 'lisp_map_request_mode'.
type LispMapRequestMode struct {
	Mode uint8
}

func (*LispMapRequestMode) GetMessageName() string {
	return "lisp_map_request_mode"
}
func (*LispMapRequestMode) GetCrcString() string {
	return "d204de7c"
}

// LispMapRequestModeReply is the Go representation of the VPP binary API message 'lisp_map_request_mode_reply'.
type LispMapRequestModeReply struct {
	Retval int32
}

func (*LispMapRequestModeReply) GetMessageName() string {
	return "lisp_map_request_mode_reply"
}
func (*LispMapRequestModeReply) GetCrcString() string {
	return "930edf9f"
}

// ShowLispMapRequestMode is the Go representation of the VPP binary API message 'show_lisp_map_request_mode'.
type ShowLispMapRequestMode struct {
}

func (*ShowLispMapRequestMode) GetMessageName() string {
	return "show_lisp_map_request_mode"
}
func (*ShowLispMapRequestMode) GetCrcString() string {
	return "da78d18a"
}

// ShowLispMapRequestModeReply is the Go representation of the VPP binary API message 'show_lisp_map_request_mode_reply'.
type ShowLispMapRequestModeReply struct {
	Retval int32
	Mode   uint8
}

func (*ShowLispMapRequestModeReply) GetMessageName() string {
	return "show_lisp_map_request_mode_reply"
}
func (*ShowLispMapRequestModeReply) GetCrcString() string {
	return "ed3aadef"
}

// LispAddDelRemoteMapping is the Go representation of the VPP binary API message 'lisp_add_del_remote_mapping'.
type LispAddDelRemoteMapping struct {
	IsAdd    uint8
	IsSrcDst uint8
	DelAll   uint8
	Vni      uint32
	Action   uint8
	EidType  uint8
	Eid      []byte `struc:"[16]byte"`
	EidLen   uint8
	Seid     []byte `struc:"[16]byte"`
	SeidLen  uint8
	RlocNum  uint32 `struc:"sizeof=Rlocs"`
	Rlocs    []byte
}

func (*LispAddDelRemoteMapping) GetMessageName() string {
	return "lisp_add_del_remote_mapping"
}
func (*LispAddDelRemoteMapping) GetCrcString() string {
	return "7225327f"
}

// LispAddDelRemoteMappingReply is the Go representation of the VPP binary API message 'lisp_add_del_remote_mapping_reply'.
type LispAddDelRemoteMappingReply struct {
	Retval int32
}

func (*LispAddDelRemoteMappingReply) GetMessageName() string {
	return "lisp_add_del_remote_mapping_reply"
}
func (*LispAddDelRemoteMappingReply) GetCrcString() string {
	return "4cae72c9"
}

// LispAddDelAdjacency is the Go representation of the VPP binary API message 'lisp_add_del_adjacency'.
type LispAddDelAdjacency struct {
	IsAdd   uint8
	Vni     uint32
	EidType uint8
	Reid    []byte `struc:"[16]byte"`
	Leid    []byte `struc:"[16]byte"`
	ReidLen uint8
	LeidLen uint8
}

func (*LispAddDelAdjacency) GetMessageName() string {
	return "lisp_add_del_adjacency"
}
func (*LispAddDelAdjacency) GetCrcString() string {
	return "2bbefa02"
}

// LispAddDelAdjacencyReply is the Go representation of the VPP binary API message 'lisp_add_del_adjacency_reply'.
type LispAddDelAdjacencyReply struct {
	Retval int32
}

func (*LispAddDelAdjacencyReply) GetMessageName() string {
	return "lisp_add_del_adjacency_reply"
}
func (*LispAddDelAdjacencyReply) GetCrcString() string {
	return "4628e1a8"
}

// LispAddDelMapRequestItrRlocs is the Go representation of the VPP binary API message 'lisp_add_del_map_request_itr_rlocs'.
type LispAddDelMapRequestItrRlocs struct {
	IsAdd          uint8
	LocatorSetName []byte `struc:"[64]byte"`
}

func (*LispAddDelMapRequestItrRlocs) GetMessageName() string {
	return "lisp_add_del_map_request_itr_rlocs"
}
func (*LispAddDelMapRequestItrRlocs) GetCrcString() string {
	return "3376a927"
}

// LispAddDelMapRequestItrRlocsReply is the Go representation of the VPP binary API message 'lisp_add_del_map_request_itr_rlocs_reply'.
type LispAddDelMapRequestItrRlocsReply struct {
	Retval int32
}

func (*LispAddDelMapRequestItrRlocsReply) GetMessageName() string {
	return "lisp_add_del_map_request_itr_rlocs_reply"
}
func (*LispAddDelMapRequestItrRlocsReply) GetCrcString() string {
	return "712498b4"
}

// LispEidTableAddDelMap is the Go representation of the VPP binary API message 'lisp_eid_table_add_del_map'.
type LispEidTableAddDelMap struct {
	IsAdd   uint8
	Vni     uint32
	DpTable uint32
	IsL2    uint8
}

func (*LispEidTableAddDelMap) GetMessageName() string {
	return "lisp_eid_table_add_del_map"
}
func (*LispEidTableAddDelMap) GetCrcString() string {
	return "598a91ce"
}

// LispEidTableAddDelMapReply is the Go representation of the VPP binary API message 'lisp_eid_table_add_del_map_reply'.
type LispEidTableAddDelMapReply struct {
	Retval int32
}

func (*LispEidTableAddDelMapReply) GetMessageName() string {
	return "lisp_eid_table_add_del_map_reply"
}
func (*LispEidTableAddDelMapReply) GetCrcString() string {
	return "9c948155"
}

// LispLocatorDump is the Go representation of the VPP binary API message 'lisp_locator_dump'.
type LispLocatorDump struct {
	LsIndex    uint32
	LsName     []byte `struc:"[64]byte"`
	IsIndexSet uint8
}

func (*LispLocatorDump) GetMessageName() string {
	return "lisp_locator_dump"
}
func (*LispLocatorDump) GetCrcString() string {
	return "35176bc8"
}

// LispLocatorDetails is the Go representation of the VPP binary API message 'lisp_locator_details'.
type LispLocatorDetails struct {
	Local     uint8
	SwIfIndex uint32
	IsIpv6    uint8
	IPAddress []byte `struc:"[16]byte"`
	Priority  uint8
	Weight    uint8
}

func (*LispLocatorDetails) GetMessageName() string {
	return "lisp_locator_details"
}
func (*LispLocatorDetails) GetCrcString() string {
	return "cb282c00"
}

// LispLocatorSetDetails is the Go representation of the VPP binary API message 'lisp_locator_set_details'.
type LispLocatorSetDetails struct {
	LsIndex uint32
	LsName  []byte `struc:"[64]byte"`
}

func (*LispLocatorSetDetails) GetMessageName() string {
	return "lisp_locator_set_details"
}
func (*LispLocatorSetDetails) GetCrcString() string {
	return "4ab2d4cf"
}

// LispLocatorSetDump is the Go representation of the VPP binary API message 'lisp_locator_set_dump'.
type LispLocatorSetDump struct {
	Filter uint8
}

func (*LispLocatorSetDump) GetMessageName() string {
	return "lisp_locator_set_dump"
}
func (*LispLocatorSetDump) GetCrcString() string {
	return "0f3d315b"
}

// LispEidTableDetails is the Go representation of the VPP binary API message 'lisp_eid_table_details'.
type LispEidTableDetails struct {
	LocatorSetIndex uint32
	Action          uint8
	IsLocal         uint8
	EidType         uint8
	IsSrcDst        uint8
	Vni             uint32
	Eid             []byte `struc:"[16]byte"`
	EidPrefixLen    uint8
	Seid            []byte `struc:"[16]byte"`
	SeidPrefixLen   uint8
	TTL             uint32
	Authoritative   uint8
	KeyID           uint16
	Key             []byte `struc:"[64]byte"`
}

func (*LispEidTableDetails) GetMessageName() string {
	return "lisp_eid_table_details"
}
func (*LispEidTableDetails) GetCrcString() string {
	return "b93cde6b"
}

// LispEidTableDump is the Go representation of the VPP binary API message 'lisp_eid_table_dump'.
type LispEidTableDump struct {
	EidSet       uint8
	PrefixLength uint8
	Vni          uint32
	EidType      uint8
	Eid          []byte `struc:"[16]byte"`
	Filter       uint8
}

func (*LispEidTableDump) GetMessageName() string {
	return "lisp_eid_table_dump"
}
func (*LispEidTableDump) GetCrcString() string {
	return "354e0c1a"
}

// LispAdjacenciesGetReply is the Go representation of the VPP binary API message 'lisp_adjacencies_get_reply'.
type LispAdjacenciesGetReply struct {
	Retval      int32
	Count       uint32 `struc:"sizeof=Adjacencies"`
	Adjacencies []LispAdjacency
}

func (*LispAdjacenciesGetReply) GetMessageName() string {
	return "lisp_adjacencies_get_reply"
}
func (*LispAdjacenciesGetReply) GetCrcString() string {
	return "00dcfe1d"
}

// LispAdjacenciesGet is the Go representation of the VPP binary API message 'lisp_adjacencies_get'.
type LispAdjacenciesGet struct {
	Vni uint32
}

func (*LispAdjacenciesGet) GetMessageName() string {
	return "lisp_adjacencies_get"
}
func (*LispAdjacenciesGet) GetCrcString() string {
	return "f0252c92"
}

// LispEidTableMapDetails is the Go representation of the VPP binary API message 'lisp_eid_table_map_details'.
type LispEidTableMapDetails struct {
	Vni     uint32
	DpTable uint32
}

func (*LispEidTableMapDetails) GetMessageName() string {
	return "lisp_eid_table_map_details"
}
func (*LispEidTableMapDetails) GetCrcString() string {
	return "c5f081e9"
}

// LispEidTableMapDump is the Go representation of the VPP binary API message 'lisp_eid_table_map_dump'.
type LispEidTableMapDump struct {
	IsL2 uint8
}

func (*LispEidTableMapDump) GetMessageName() string {
	return "lisp_eid_table_map_dump"
}
func (*LispEidTableMapDump) GetCrcString() string {
	return "b0704823"
}

// LispEidTableVniDump is the Go representation of the VPP binary API message 'lisp_eid_table_vni_dump'.
type LispEidTableVniDump struct {
}

func (*LispEidTableVniDump) GetMessageName() string {
	return "lisp_eid_table_vni_dump"
}
func (*LispEidTableVniDump) GetCrcString() string {
	return "3456e06a"
}

// LispEidTableVniDetails is the Go representation of the VPP binary API message 'lisp_eid_table_vni_details'.
type LispEidTableVniDetails struct {
	Vni uint32
}

func (*LispEidTableVniDetails) GetMessageName() string {
	return "lisp_eid_table_vni_details"
}
func (*LispEidTableVniDetails) GetCrcString() string {
	return "e2f8a8b9"
}

// LispGpeTunnelDetails is the Go representation of the VPP binary API message 'lisp_gpe_tunnel_details'.
type LispGpeTunnelDetails struct {
	Tunnels       uint32
	IsIpv6        uint8
	SourceIP      []byte `struc:"[16]byte"`
	DestinationIP []byte `struc:"[16]byte"`
	EncapFibID    uint32
	DecapFibID    uint32
	DcapNext      uint32
	LispVer       uint8
	NextProtocol  uint8
	Flags         uint8
	VerRes        uint8
	Res           uint8
	Iid           uint32
}

func (*LispGpeTunnelDetails) GetMessageName() string {
	return "lisp_gpe_tunnel_details"
}
func (*LispGpeTunnelDetails) GetCrcString() string {
	return "3681753b"
}

// LispGpeTunnelDump is the Go representation of the VPP binary API message 'lisp_gpe_tunnel_dump'.
type LispGpeTunnelDump struct {
}

func (*LispGpeTunnelDump) GetMessageName() string {
	return "lisp_gpe_tunnel_dump"
}
func (*LispGpeTunnelDump) GetCrcString() string {
	return "04c7c390"
}

// LispMapResolverDetails is the Go representation of the VPP binary API message 'lisp_map_resolver_details'.
type LispMapResolverDetails struct {
	IsIpv6    uint8
	IPAddress []byte `struc:"[16]byte"`
}

func (*LispMapResolverDetails) GetMessageName() string {
	return "lisp_map_resolver_details"
}
func (*LispMapResolverDetails) GetCrcString() string {
	return "e8c68ebd"
}

// LispMapResolverDump is the Go representation of the VPP binary API message 'lisp_map_resolver_dump'.
type LispMapResolverDump struct {
}

func (*LispMapResolverDump) GetMessageName() string {
	return "lisp_map_resolver_dump"
}
func (*LispMapResolverDump) GetCrcString() string {
	return "4e5e2003"
}

// LispMapServerDetails is the Go representation of the VPP binary API message 'lisp_map_server_details'.
type LispMapServerDetails struct {
	IsIpv6    uint8
	IPAddress []byte `struc:"[16]byte"`
}

func (*LispMapServerDetails) GetMessageName() string {
	return "lisp_map_server_details"
}
func (*LispMapServerDetails) GetCrcString() string {
	return "4ef38e5a"
}

// LispMapServerDump is the Go representation of the VPP binary API message 'lisp_map_server_dump'.
type LispMapServerDump struct {
}

func (*LispMapServerDump) GetMessageName() string {
	return "lisp_map_server_dump"
}
func (*LispMapServerDump) GetCrcString() string {
	return "2b2998e2"
}

// ShowLispStatus is the Go representation of the VPP binary API message 'show_lisp_status'.
type ShowLispStatus struct {
}

func (*ShowLispStatus) GetMessageName() string {
	return "show_lisp_status"
}
func (*ShowLispStatus) GetCrcString() string {
	return "8092ab77"
}

// ShowLispStatusReply is the Go representation of the VPP binary API message 'show_lisp_status_reply'.
type ShowLispStatusReply struct {
	Retval        int32
	FeatureStatus uint8
	GpeStatus     uint8
}

func (*ShowLispStatusReply) GetMessageName() string {
	return "show_lisp_status_reply"
}
func (*ShowLispStatusReply) GetCrcString() string {
	return "6aa3f21d"
}

// LispGetMapRequestItrRlocs is the Go representation of the VPP binary API message 'lisp_get_map_request_itr_rlocs'.
type LispGetMapRequestItrRlocs struct {
}

func (*LispGetMapRequestItrRlocs) GetMessageName() string {
	return "lisp_get_map_request_itr_rlocs"
}
func (*LispGetMapRequestItrRlocs) GetCrcString() string {
	return "1e2d23a4"
}

// LispGetMapRequestItrRlocsReply is the Go representation of the VPP binary API message 'lisp_get_map_request_itr_rlocs_reply'.
type LispGetMapRequestItrRlocsReply struct {
	Retval         int32
	LocatorSetName []byte `struc:"[64]byte"`
}

func (*LispGetMapRequestItrRlocsReply) GetMessageName() string {
	return "lisp_get_map_request_itr_rlocs_reply"
}
func (*LispGetMapRequestItrRlocsReply) GetCrcString() string {
	return "39bfca79"
}

// ShowLispPitr is the Go representation of the VPP binary API message 'show_lisp_pitr'.
type ShowLispPitr struct {
}

func (*ShowLispPitr) GetMessageName() string {
	return "show_lisp_pitr"
}
func (*ShowLispPitr) GetCrcString() string {
	return "d4a061e6"
}

// ShowLispPitrReply is the Go representation of the VPP binary API message 'show_lisp_pitr_reply'.
type ShowLispPitrReply struct {
	Retval         int32
	Status         uint8
	LocatorSetName []byte `struc:"[64]byte"`
}

func (*ShowLispPitrReply) GetMessageName() string {
	return "show_lisp_pitr_reply"
}
func (*ShowLispPitrReply) GetCrcString() string {
	return "e730f16e"
}

// InterfaceNameRenumber is the Go representation of the VPP binary API message 'interface_name_renumber'.
type InterfaceNameRenumber struct {
	SwIfIndex          uint32
	NewShowDevInstance uint32
}

func (*InterfaceNameRenumber) GetMessageName() string {
	return "interface_name_renumber"
}
func (*InterfaceNameRenumber) GetCrcString() string {
	return "11b7bcec"
}

// InterfaceNameRenumberReply is the Go representation of the VPP binary API message 'interface_name_renumber_reply'.
type InterfaceNameRenumberReply struct {
	Retval int32
}

func (*InterfaceNameRenumberReply) GetMessageName() string {
	return "interface_name_renumber_reply"
}
func (*InterfaceNameRenumberReply) GetCrcString() string {
	return "31594963"
}

// WantIP4ArpEvents is the Go representation of the VPP binary API message 'want_ip4_arp_events'.
type WantIP4ArpEvents struct {
	EnableDisable uint8
	Pid           uint32
	Address       uint32
}

func (*WantIP4ArpEvents) GetMessageName() string {
	return "want_ip4_arp_events"
}
func (*WantIP4ArpEvents) GetCrcString() string {
	return "5ae044c2"
}

// WantIP4ArpEventsReply is the Go representation of the VPP binary API message 'want_ip4_arp_events_reply'.
type WantIP4ArpEventsReply struct {
	Retval int32
}

func (*WantIP4ArpEventsReply) GetMessageName() string {
	return "want_ip4_arp_events_reply"
}
func (*WantIP4ArpEventsReply) GetCrcString() string {
	return "e1c0b59e"
}

// IP4ArpEvent is the Go representation of the VPP binary API message 'ip4_arp_event'.
type IP4ArpEvent struct {
	Address   uint32
	Pid       uint32
	SwIfIndex uint32
	NewMac    []byte `struc:"[6]byte"`
	MacIP     uint8
}

func (*IP4ArpEvent) GetMessageName() string {
	return "ip4_arp_event"
}
func (*IP4ArpEvent) GetCrcString() string {
	return "7de1837b"
}

// WantIP6NdEvents is the Go representation of the VPP binary API message 'want_ip6_nd_events'.
type WantIP6NdEvents struct {
	EnableDisable uint8
	Pid           uint32
	Address       []byte `struc:"[16]byte"`
}

func (*WantIP6NdEvents) GetMessageName() string {
	return "want_ip6_nd_events"
}
func (*WantIP6NdEvents) GetCrcString() string {
	return "9586ba55"
}

// WantIP6NdEventsReply is the Go representation of the VPP binary API message 'want_ip6_nd_events_reply'.
type WantIP6NdEventsReply struct {
	Retval int32
}

func (*WantIP6NdEventsReply) GetMessageName() string {
	return "want_ip6_nd_events_reply"
}
func (*WantIP6NdEventsReply) GetCrcString() string {
	return "95458aad"
}

// IP6NdEvent is the Go representation of the VPP binary API message 'ip6_nd_event'.
type IP6NdEvent struct {
	Pid       uint32
	SwIfIndex uint32
	Address   []byte `struc:"[16]byte"`
	NewMac    []byte `struc:"[6]byte"`
	MacIP     uint8
}

func (*IP6NdEvent) GetMessageName() string {
	return "ip6_nd_event"
}
func (*IP6NdEvent) GetCrcString() string {
	return "777bb71c"
}

// BridgeDomainAddDel is the Go representation of the VPP binary API message 'bridge_domain_add_del'.
type BridgeDomainAddDel struct {
	BdID    uint32
	Flood   uint8
	UuFlood uint8
	Forward uint8
	Learn   uint8
	ArpTerm uint8
	MacAge  uint8
	IsAdd   uint8
}

func (*BridgeDomainAddDel) GetMessageName() string {
	return "bridge_domain_add_del"
}
func (*BridgeDomainAddDel) GetCrcString() string {
	return "bddc9ff1"
}

// BridgeDomainAddDelReply is the Go representation of the VPP binary API message 'bridge_domain_add_del_reply'.
type BridgeDomainAddDelReply struct {
	Retval int32
}

func (*BridgeDomainAddDelReply) GetMessageName() string {
	return "bridge_domain_add_del_reply"
}
func (*BridgeDomainAddDelReply) GetCrcString() string {
	return "d5e138e4"
}

// BridgeDomainDump is the Go representation of the VPP binary API message 'bridge_domain_dump'.
type BridgeDomainDump struct {
	BdID uint32
}

func (*BridgeDomainDump) GetMessageName() string {
	return "bridge_domain_dump"
}
func (*BridgeDomainDump) GetCrcString() string {
	return "68d5401d"
}

// BridgeDomainDetails is the Go representation of the VPP binary API message 'bridge_domain_details'.
type BridgeDomainDetails struct {
	BdID         uint32
	Flood        uint8
	UuFlood      uint8
	Forward      uint8
	Learn        uint8
	ArpTerm      uint8
	MacAge       uint8
	BviSwIfIndex uint32
	NSwIfs       uint32
}

func (*BridgeDomainDetails) GetMessageName() string {
	return "bridge_domain_details"
}
func (*BridgeDomainDetails) GetCrcString() string {
	return "c68cb6d1"
}

// BridgeDomainSwIfDetails is the Go representation of the VPP binary API message 'bridge_domain_sw_if_details'.
type BridgeDomainSwIfDetails struct {
	BdID      uint32
	SwIfIndex uint32
	Shg       uint8
}

func (*BridgeDomainSwIfDetails) GetMessageName() string {
	return "bridge_domain_sw_if_details"
}
func (*BridgeDomainSwIfDetails) GetCrcString() string {
	return "c95a6381"
}

// DhcpClientConfig is the Go representation of the VPP binary API message 'dhcp_client_config'.
type DhcpClientConfig struct {
	SwIfIndex     uint32
	Hostname      []byte `struc:"[64]byte"`
	IsAdd         uint8
	WantDhcpEvent uint8
	Pid           uint32
}

func (*DhcpClientConfig) GetMessageName() string {
	return "dhcp_client_config"
}
func (*DhcpClientConfig) GetCrcString() string {
	return "87920bb6"
}

// DhcpClientConfigReply is the Go representation of the VPP binary API message 'dhcp_client_config_reply'.
type DhcpClientConfigReply struct {
	Retval int32
}

func (*DhcpClientConfigReply) GetMessageName() string {
	return "dhcp_client_config_reply"
}
func (*DhcpClientConfigReply) GetCrcString() string {
	return "d947f4c8"
}

// InputACLSetInterface is the Go representation of the VPP binary API message 'input_acl_set_interface'.
type InputACLSetInterface struct {
	SwIfIndex     uint32
	IP4TableIndex uint32
	IP6TableIndex uint32
	L2TableIndex  uint32
	IsAdd         uint8
}

func (*InputACLSetInterface) GetMessageName() string {
	return "input_acl_set_interface"
}
func (*InputACLSetInterface) GetCrcString() string {
	return "34d2fc33"
}

// InputACLSetInterfaceReply is the Go representation of the VPP binary API message 'input_acl_set_interface_reply'.
type InputACLSetInterfaceReply struct {
	Retval int32
}

func (*InputACLSetInterfaceReply) GetMessageName() string {
	return "input_acl_set_interface_reply"
}
func (*InputACLSetInterfaceReply) GetCrcString() string {
	return "ba0110e3"
}

// IpsecSpdAddDel is the Go representation of the VPP binary API message 'ipsec_spd_add_del'.
type IpsecSpdAddDel struct {
	IsAdd uint8
	SpdID uint32
}

func (*IpsecSpdAddDel) GetMessageName() string {
	return "ipsec_spd_add_del"
}
func (*IpsecSpdAddDel) GetCrcString() string {
	return "9b42314b"
}

// IpsecSpdAddDelReply is the Go representation of the VPP binary API message 'ipsec_spd_add_del_reply'.
type IpsecSpdAddDelReply struct {
	Retval int32
}

func (*IpsecSpdAddDelReply) GetMessageName() string {
	return "ipsec_spd_add_del_reply"
}
func (*IpsecSpdAddDelReply) GetCrcString() string {
	return "c7439119"
}

// IpsecInterfaceAddDelSpd is the Go representation of the VPP binary API message 'ipsec_interface_add_del_spd'.
type IpsecInterfaceAddDelSpd struct {
	IsAdd     uint8
	SwIfIndex uint32
	SpdID     uint32
}

func (*IpsecInterfaceAddDelSpd) GetMessageName() string {
	return "ipsec_interface_add_del_spd"
}
func (*IpsecInterfaceAddDelSpd) GetCrcString() string {
	return "52de89dc"
}

// IpsecInterfaceAddDelSpdReply is the Go representation of the VPP binary API message 'ipsec_interface_add_del_spd_reply'.
type IpsecInterfaceAddDelSpdReply struct {
	Retval int32
}

func (*IpsecInterfaceAddDelSpdReply) GetMessageName() string {
	return "ipsec_interface_add_del_spd_reply"
}
func (*IpsecInterfaceAddDelSpdReply) GetCrcString() string {
	return "977b7be9"
}

// IpsecSpdAddDelEntry is the Go representation of the VPP binary API message 'ipsec_spd_add_del_entry'.
type IpsecSpdAddDelEntry struct {
	IsAdd              uint8
	SpdID              uint32
	Priority           int32
	IsOutbound         uint8
	IsIpv6             uint8
	IsIPAny            uint8
	RemoteAddressStart []byte `struc:"[16]byte"`
	RemoteAddressStop  []byte `struc:"[16]byte"`
	LocalAddressStart  []byte `struc:"[16]byte"`
	LocalAddressStop   []byte `struc:"[16]byte"`
	Protocol           uint8
	RemotePortStart    uint16
	RemotePortStop     uint16
	LocalPortStart     uint16
	LocalPortStop      uint16
	Policy             uint8
	SaID               uint32
}

func (*IpsecSpdAddDelEntry) GetMessageName() string {
	return "ipsec_spd_add_del_entry"
}
func (*IpsecSpdAddDelEntry) GetCrcString() string {
	return "d85e0ed5"
}

// IpsecSpdAddDelEntryReply is the Go representation of the VPP binary API message 'ipsec_spd_add_del_entry_reply'.
type IpsecSpdAddDelEntryReply struct {
	Retval int32
}

func (*IpsecSpdAddDelEntryReply) GetMessageName() string {
	return "ipsec_spd_add_del_entry_reply"
}
func (*IpsecSpdAddDelEntryReply) GetCrcString() string {
	return "f96c8b2d"
}

// IpsecSadAddDelEntry is the Go representation of the VPP binary API message 'ipsec_sad_add_del_entry'.
type IpsecSadAddDelEntry struct {
	IsAdd                     uint8
	SadID                     uint32
	Spi                       uint32
	Protocol                  uint8
	CryptoAlgorithm           uint8
	CryptoKeyLength           uint8
	CryptoKey                 []byte `struc:"[128]byte"`
	IntegrityAlgorithm        uint8
	IntegrityKeyLength        uint8
	IntegrityKey              []byte `struc:"[128]byte"`
	UseExtendedSequenceNumber uint8
	IsTunnel                  uint8
	IsTunnelIpv6              uint8
	TunnelSrcAddress          []byte `struc:"[16]byte"`
	TunnelDstAddress          []byte `struc:"[16]byte"`
}

func (*IpsecSadAddDelEntry) GetMessageName() string {
	return "ipsec_sad_add_del_entry"
}
func (*IpsecSadAddDelEntry) GetCrcString() string {
	return "7d6709e1"
}

// IpsecSadAddDelEntryReply is the Go representation of the VPP binary API message 'ipsec_sad_add_del_entry_reply'.
type IpsecSadAddDelEntryReply struct {
	Retval int32
}

func (*IpsecSadAddDelEntryReply) GetMessageName() string {
	return "ipsec_sad_add_del_entry_reply"
}
func (*IpsecSadAddDelEntryReply) GetCrcString() string {
	return "5cf382d8"
}

// IpsecSaSetKey is the Go representation of the VPP binary API message 'ipsec_sa_set_key'.
type IpsecSaSetKey struct {
	SaID               uint32
	CryptoKeyLength    uint8
	CryptoKey          []byte `struc:"[128]byte"`
	IntegrityKeyLength uint8
	IntegrityKey       []byte `struc:"[128]byte"`
}

func (*IpsecSaSetKey) GetMessageName() string {
	return "ipsec_sa_set_key"
}
func (*IpsecSaSetKey) GetCrcString() string {
	return "99a67c60"
}

// IpsecSaSetKeyReply is the Go representation of the VPP binary API message 'ipsec_sa_set_key_reply'.
type IpsecSaSetKeyReply struct {
	Retval int32
}

func (*IpsecSaSetKeyReply) GetMessageName() string {
	return "ipsec_sa_set_key_reply"
}
func (*IpsecSaSetKeyReply) GetCrcString() string {
	return "5c5b7b46"
}

// Ikev2ProfileAddDel is the Go representation of the VPP binary API message 'ikev2_profile_add_del'.
type Ikev2ProfileAddDel struct {
	Name  []byte `struc:"[64]byte"`
	IsAdd uint8
}

func (*Ikev2ProfileAddDel) GetMessageName() string {
	return "ikev2_profile_add_del"
}
func (*Ikev2ProfileAddDel) GetCrcString() string {
	return "37b6925c"
}

// Ikev2ProfileAddDelReply is the Go representation of the VPP binary API message 'ikev2_profile_add_del_reply'.
type Ikev2ProfileAddDelReply struct {
	Retval int32
}

func (*Ikev2ProfileAddDelReply) GetMessageName() string {
	return "ikev2_profile_add_del_reply"
}
func (*Ikev2ProfileAddDelReply) GetCrcString() string {
	return "7621f627"
}

// Ikev2ProfileSetAuth is the Go representation of the VPP binary API message 'ikev2_profile_set_auth'.
type Ikev2ProfileSetAuth struct {
	Name       []byte `struc:"[64]byte"`
	AuthMethod uint8
	IsHex      uint8
	DataLen    uint32 `struc:"sizeof=Data"`
	Data       []byte
}

func (*Ikev2ProfileSetAuth) GetMessageName() string {
	return "ikev2_profile_set_auth"
}
func (*Ikev2ProfileSetAuth) GetCrcString() string {
	return "a0747739"
}

// Ikev2ProfileSetAuthReply is the Go representation of the VPP binary API message 'ikev2_profile_set_auth_reply'.
type Ikev2ProfileSetAuthReply struct {
	Retval int32
}

func (*Ikev2ProfileSetAuthReply) GetMessageName() string {
	return "ikev2_profile_set_auth_reply"
}
func (*Ikev2ProfileSetAuthReply) GetCrcString() string {
	return "46083d00"
}

// Ikev2ProfileSetID is the Go representation of the VPP binary API message 'ikev2_profile_set_id'.
type Ikev2ProfileSetID struct {
	Name    []byte `struc:"[64]byte"`
	IsLocal uint8
	IDType  uint8
	DataLen uint32 `struc:"sizeof=Data"`
	Data    []byte
}

func (*Ikev2ProfileSetID) GetMessageName() string {
	return "ikev2_profile_set_id"
}
func (*Ikev2ProfileSetID) GetCrcString() string {
	return "0c2331dc"
}

// Ikev2ProfileSetIDReply is the Go representation of the VPP binary API message 'ikev2_profile_set_id_reply'.
type Ikev2ProfileSetIDReply struct {
	Retval int32
}

func (*Ikev2ProfileSetIDReply) GetMessageName() string {
	return "ikev2_profile_set_id_reply"
}
func (*Ikev2ProfileSetIDReply) GetCrcString() string {
	return "66803be5"
}

// Ikev2ProfileSetTs is the Go representation of the VPP binary API message 'ikev2_profile_set_ts'.
type Ikev2ProfileSetTs struct {
	Name      []byte `struc:"[64]byte"`
	IsLocal   uint8
	Proto     uint8
	StartPort uint16
	EndPort   uint16
	StartAddr uint32
	EndAddr   uint32
}

func (*Ikev2ProfileSetTs) GetMessageName() string {
	return "ikev2_profile_set_ts"
}
func (*Ikev2ProfileSetTs) GetCrcString() string {
	return "69587e0e"
}

// Ikev2ProfileSetTsReply is the Go representation of the VPP binary API message 'ikev2_profile_set_ts_reply'.
type Ikev2ProfileSetTsReply struct {
	Retval int32
}

func (*Ikev2ProfileSetTsReply) GetMessageName() string {
	return "ikev2_profile_set_ts_reply"
}
func (*Ikev2ProfileSetTsReply) GetCrcString() string {
	return "e1c33583"
}

// Ikev2SetLocalKey is the Go representation of the VPP binary API message 'ikev2_set_local_key'.
type Ikev2SetLocalKey struct {
	KeyFile []byte `struc:"[256]byte"`
}

func (*Ikev2SetLocalKey) GetMessageName() string {
	return "ikev2_set_local_key"
}
func (*Ikev2SetLocalKey) GetCrcString() string {
	return "a99b238a"
}

// Ikev2SetLocalKeyReply is the Go representation of the VPP binary API message 'ikev2_set_local_key_reply'.
type Ikev2SetLocalKeyReply struct {
	Retval int32
}

func (*Ikev2SetLocalKeyReply) GetMessageName() string {
	return "ikev2_set_local_key_reply"
}
func (*Ikev2SetLocalKeyReply) GetCrcString() string {
	return "8f7a80e0"
}

// DhcpComplEvent is the Go representation of the VPP binary API message 'dhcp_compl_event'.
type DhcpComplEvent struct {
	Pid           uint32
	Hostname      []byte `struc:"[64]byte"`
	IsIpv6        uint8
	HostAddress   []byte `struc:"[16]byte"`
	RouterAddress []byte `struc:"[16]byte"`
	HostMac       []byte `struc:"[6]byte"`
}

func (*DhcpComplEvent) GetMessageName() string {
	return "dhcp_compl_event"
}
func (*DhcpComplEvent) GetCrcString() string {
	return "5218db55"
}

// CopInterfaceEnableDisable is the Go representation of the VPP binary API message 'cop_interface_enable_disable'.
type CopInterfaceEnableDisable struct {
	SwIfIndex     uint32
	EnableDisable uint8
}

func (*CopInterfaceEnableDisable) GetMessageName() string {
	return "cop_interface_enable_disable"
}
func (*CopInterfaceEnableDisable) GetCrcString() string {
	return "1c65bd42"
}

// CopInterfaceEnableDisableReply is the Go representation of the VPP binary API message 'cop_interface_enable_disable_reply'.
type CopInterfaceEnableDisableReply struct {
	Retval int32
}

func (*CopInterfaceEnableDisableReply) GetMessageName() string {
	return "cop_interface_enable_disable_reply"
}
func (*CopInterfaceEnableDisableReply) GetCrcString() string {
	return "123dd020"
}

// CopWhitelistEnableDisable is the Go representation of the VPP binary API message 'cop_whitelist_enable_disable'.
type CopWhitelistEnableDisable struct {
	SwIfIndex  uint32
	FibID      uint32
	IP4        uint8
	IP6        uint8
	DefaultCop uint8
}

func (*CopWhitelistEnableDisable) GetMessageName() string {
	return "cop_whitelist_enable_disable"
}
func (*CopWhitelistEnableDisable) GetCrcString() string {
	return "9a0ec2ec"
}

// CopWhitelistEnableDisableReply is the Go representation of the VPP binary API message 'cop_whitelist_enable_disable_reply'.
type CopWhitelistEnableDisableReply struct {
	Retval int32
}

func (*CopWhitelistEnableDisableReply) GetMessageName() string {
	return "cop_whitelist_enable_disable_reply"
}
func (*CopWhitelistEnableDisableReply) GetCrcString() string {
	return "3f660dee"
}

// GetNodeGraph is the Go representation of the VPP binary API message 'get_node_graph'.
type GetNodeGraph struct {
}

func (*GetNodeGraph) GetMessageName() string {
	return "get_node_graph"
}
func (*GetNodeGraph) GetCrcString() string {
	return "f8636a76"
}

// GetNodeGraphReply is the Go representation of the VPP binary API message 'get_node_graph_reply'.
type GetNodeGraphReply struct {
	Retval       int32
	ReplyInShmem uint64
}

func (*GetNodeGraphReply) GetMessageName() string {
	return "get_node_graph_reply"
}
func (*GetNodeGraphReply) GetCrcString() string {
	return "816d91b6"
}

// IoamEnable is the Go representation of the VPP binary API message 'ioam_enable'.
type IoamEnable struct {
	ID          uint16
	Seqno       uint8
	Analyse     uint8
	PotEnable   uint8
	TraceEnable uint8
	NodeID      uint32
}

func (*IoamEnable) GetMessageName() string {
	return "ioam_enable"
}
func (*IoamEnable) GetCrcString() string {
	return "7bd4abf9"
}

// IoamEnableReply is the Go representation of the VPP binary API message 'ioam_enable_reply'.
type IoamEnableReply struct {
	Retval int32
}

func (*IoamEnableReply) GetMessageName() string {
	return "ioam_enable_reply"
}
func (*IoamEnableReply) GetCrcString() string {
	return "58a8fedc"
}

// IoamDisable is the Go representation of the VPP binary API message 'ioam_disable'.
type IoamDisable struct {
	ID uint16
}

func (*IoamDisable) GetMessageName() string {
	return "ioam_disable"
}
func (*IoamDisable) GetCrcString() string {
	return "aff26d33"
}

// IoamDisableReply is the Go representation of the VPP binary API message 'ioam_disable_reply'.
type IoamDisableReply struct {
	Retval int32
}

func (*IoamDisableReply) GetMessageName() string {
	return "ioam_disable_reply"
}
func (*IoamDisableReply) GetCrcString() string {
	return "ef118a9d"
}

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

// PolicerAddDel is the Go representation of the VPP binary API message 'policer_add_del'.
type PolicerAddDel struct {
	IsAdd             uint8
	Name              []byte `struc:"[64]byte"`
	Cir               uint32
	Eir               uint32
	Cb                uint64
	Eb                uint64
	RateType          uint8
	RoundType         uint8
	Type              uint8
	ColorAware        uint8
	ConformActionType uint8
	ConformDscp       uint8
	ExceedActionType  uint8
	ExceedDscp        uint8
	ViolateActionType uint8
	ViolateDscp       uint8
}

func (*PolicerAddDel) GetMessageName() string {
	return "policer_add_del"
}
func (*PolicerAddDel) GetCrcString() string {
	return "e1bba755"
}

// PolicerAddDelReply is the Go representation of the VPP binary API message 'policer_add_del_reply'.
type PolicerAddDelReply struct {
	Retval       int32
	PolicerIndex uint32
}

func (*PolicerAddDelReply) GetMessageName() string {
	return "policer_add_del_reply"
}
func (*PolicerAddDelReply) GetCrcString() string {
	return "ddb244b0"
}

// PolicerDump is the Go representation of the VPP binary API message 'policer_dump'.
type PolicerDump struct {
	MatchNameValid uint8
	MatchName      []byte `struc:"[64]byte"`
}

func (*PolicerDump) GetMessageName() string {
	return "policer_dump"
}
func (*PolicerDump) GetCrcString() string {
	return "526b205b"
}

// PolicerDetails is the Go representation of the VPP binary API message 'policer_details'.
type PolicerDetails struct {
	Name               []byte `struc:"[64]byte"`
	Cir                uint32
	Eir                uint32
	Cb                 uint64
	Eb                 uint64
	RateType           uint8
	RoundType          uint8
	Type               uint8
	ConformActionType  uint8
	ConformDscp        uint8
	ExceedActionType   uint8
	ExceedDscp         uint8
	ViolateActionType  uint8
	ViolateDscp        uint8
	SingleRate         uint8
	ColorAware         uint8
	Scale              uint32
	CirTokensPerPeriod uint32
	PirTokensPerPeriod uint32
	CurrentLimit       uint32
	CurrentBucket      uint32
	ExtendedLimit      uint32
	ExtendedBucket     uint32
	LastUpdateTime     uint64
}

func (*PolicerDetails) GetMessageName() string {
	return "policer_details"
}
func (*PolicerDetails) GetCrcString() string {
	return "a9729913"
}

// PolicerClassifySetInterface is the Go representation of the VPP binary API message 'policer_classify_set_interface'.
type PolicerClassifySetInterface struct {
	SwIfIndex     uint32
	IP4TableIndex uint32
	IP6TableIndex uint32
	L2TableIndex  uint32
	IsAdd         uint8
}

func (*PolicerClassifySetInterface) GetMessageName() string {
	return "policer_classify_set_interface"
}
func (*PolicerClassifySetInterface) GetCrcString() string {
	return "4ad6b5a8"
}

// PolicerClassifySetInterfaceReply is the Go representation of the VPP binary API message 'policer_classify_set_interface_reply'.
type PolicerClassifySetInterfaceReply struct {
	Retval int32
}

func (*PolicerClassifySetInterfaceReply) GetMessageName() string {
	return "policer_classify_set_interface_reply"
}
func (*PolicerClassifySetInterfaceReply) GetCrcString() string {
	return "003dd29f"
}

// PolicerClassifyDump is the Go representation of the VPP binary API message 'policer_classify_dump'.
type PolicerClassifyDump struct {
	Type uint8
}

func (*PolicerClassifyDump) GetMessageName() string {
	return "policer_classify_dump"
}
func (*PolicerClassifyDump) GetCrcString() string {
	return "593ab73c"
}

// PolicerClassifyDetails is the Go representation of the VPP binary API message 'policer_classify_details'.
type PolicerClassifyDetails struct {
	SwIfIndex  uint32
	TableIndex uint32
}

func (*PolicerClassifyDetails) GetMessageName() string {
	return "policer_classify_details"
}
func (*PolicerClassifyDetails) GetCrcString() string {
	return "e3439be8"
}

// NetmapCreate is the Go representation of the VPP binary API message 'netmap_create'.
type NetmapCreate struct {
	NetmapIfName    []byte `struc:"[64]byte"`
	HwAddr          []byte `struc:"[6]byte"`
	UseRandomHwAddr uint8
	IsPipe          uint8
	IsMaster        uint8
}

func (*NetmapCreate) GetMessageName() string {
	return "netmap_create"
}
func (*NetmapCreate) GetCrcString() string {
	return "0f13a603"
}

// NetmapCreateReply is the Go representation of the VPP binary API message 'netmap_create_reply'.
type NetmapCreateReply struct {
	Retval int32
}

func (*NetmapCreateReply) GetMessageName() string {
	return "netmap_create_reply"
}
func (*NetmapCreateReply) GetCrcString() string {
	return "70d29cfe"
}

// NetmapDelete is the Go representation of the VPP binary API message 'netmap_delete'.
type NetmapDelete struct {
	NetmapIfName []byte `struc:"[64]byte"`
}

func (*NetmapDelete) GetMessageName() string {
	return "netmap_delete"
}
func (*NetmapDelete) GetCrcString() string {
	return "43e5c963"
}

// NetmapDeleteReply is the Go representation of the VPP binary API message 'netmap_delete_reply'.
type NetmapDeleteReply struct {
	Retval int32
}

func (*NetmapDeleteReply) GetMessageName() string {
	return "netmap_delete_reply"
}
func (*NetmapDeleteReply) GetCrcString() string {
	return "81ecfa0d"
}

// ClassifyTableIds is the Go representation of the VPP binary API message 'classify_table_ids'.
type ClassifyTableIds struct {
}

func (*ClassifyTableIds) GetMessageName() string {
	return "classify_table_ids"
}
func (*ClassifyTableIds) GetCrcString() string {
	return "fea5bc0b"
}

// ClassifyTableIdsReply is the Go representation of the VPP binary API message 'classify_table_ids_reply'.
type ClassifyTableIdsReply struct {
	Retval int32
	Count  uint32 `struc:"sizeof=Ids"`
	Ids    []uint32
}

func (*ClassifyTableIdsReply) GetMessageName() string {
	return "classify_table_ids_reply"
}
func (*ClassifyTableIdsReply) GetCrcString() string {
	return "6eacee4e"
}

// ClassifyTableByInterface is the Go representation of the VPP binary API message 'classify_table_by_interface'.
type ClassifyTableByInterface struct {
	SwIfIndex uint32
}

func (*ClassifyTableByInterface) GetMessageName() string {
	return "classify_table_by_interface"
}
func (*ClassifyTableByInterface) GetCrcString() string {
	return "2dc2ff38"
}

// ClassifyTableByInterfaceReply is the Go representation of the VPP binary API message 'classify_table_by_interface_reply'.
type ClassifyTableByInterfaceReply struct {
	Retval     int32
	SwIfIndex  uint32
	L2TableID  uint32
	IP4TableID uint32
	IP6TableID uint32
}

func (*ClassifyTableByInterfaceReply) GetMessageName() string {
	return "classify_table_by_interface_reply"
}
func (*ClassifyTableByInterfaceReply) GetCrcString() string {
	return "7a9ac873"
}

// ClassifyTableInfo is the Go representation of the VPP binary API message 'classify_table_info'.
type ClassifyTableInfo struct {
	TableID uint32
}

func (*ClassifyTableInfo) GetMessageName() string {
	return "classify_table_info"
}
func (*ClassifyTableInfo) GetCrcString() string {
	return "33caf8c6"
}

// ClassifyTableInfoReply is the Go representation of the VPP binary API message 'classify_table_info_reply'.
type ClassifyTableInfoReply struct {
	Retval         int32
	TableID        uint32
	Nbuckets       uint32
	MatchNVectors  uint32
	SkipNVectors   uint32
	ActiveSessions uint32
	NextTableIndex uint32
	MissNextIndex  uint32
	MaskLength     uint32 `struc:"sizeof=Mask"`
	Mask           []byte
}

func (*ClassifyTableInfoReply) GetMessageName() string {
	return "classify_table_info_reply"
}
func (*ClassifyTableInfoReply) GetCrcString() string {
	return "60312b83"
}

// ClassifySessionDump is the Go representation of the VPP binary API message 'classify_session_dump'.
type ClassifySessionDump struct {
	TableID uint32
}

func (*ClassifySessionDump) GetMessageName() string {
	return "classify_session_dump"
}
func (*ClassifySessionDump) GetCrcString() string {
	return "87d2ca2b"
}

// ClassifySessionDetails is the Go representation of the VPP binary API message 'classify_session_details'.
type ClassifySessionDetails struct {
	Retval       int32
	TableID      uint32
	HitNextIndex uint32
	Advance      int32
	OpaqueIndex  uint32
	MatchLength  uint32 `struc:"sizeof=Match"`
	Match        []byte
}

func (*ClassifySessionDetails) GetMessageName() string {
	return "classify_session_details"
}
func (*ClassifySessionDetails) GetCrcString() string {
	return "95efd073"
}

// SetIpfixExporter is the Go representation of the VPP binary API message 'set_ipfix_exporter'.
type SetIpfixExporter struct {
	CollectorAddress []byte `struc:"[16]byte"`
	CollectorPort    uint16
	SrcAddress       []byte `struc:"[16]byte"`
	VrfID            uint32
	PathMtu          uint32
	TemplateInterval uint32
	UDPChecksum      uint8
}

func (*SetIpfixExporter) GetMessageName() string {
	return "set_ipfix_exporter"
}
func (*SetIpfixExporter) GetCrcString() string {
	return "d4c80f5c"
}

// SetIpfixExporterReply is the Go representation of the VPP binary API message 'set_ipfix_exporter_reply'.
type SetIpfixExporterReply struct {
	Retval int32
}

func (*SetIpfixExporterReply) GetMessageName() string {
	return "set_ipfix_exporter_reply"
}
func (*SetIpfixExporterReply) GetCrcString() string {
	return "40b502e9"
}

// IpfixExporterDump is the Go representation of the VPP binary API message 'ipfix_exporter_dump'.
type IpfixExporterDump struct {
}

func (*IpfixExporterDump) GetMessageName() string {
	return "ipfix_exporter_dump"
}
func (*IpfixExporterDump) GetCrcString() string {
	return "81a51716"
}

// IpfixExporterDetails is the Go representation of the VPP binary API message 'ipfix_exporter_details'.
type IpfixExporterDetails struct {
	CollectorAddress []byte `struc:"[16]byte"`
	CollectorPort    uint16
	SrcAddress       []byte `struc:"[16]byte"`
	VrfID            uint32
	PathMtu          uint32
	TemplateInterval uint32
	UDPChecksum      uint8
}

func (*IpfixExporterDetails) GetMessageName() string {
	return "ipfix_exporter_details"
}
func (*IpfixExporterDetails) GetCrcString() string {
	return "e7cc717b"
}

// SetIpfixClassifyStream is the Go representation of the VPP binary API message 'set_ipfix_classify_stream'.
type SetIpfixClassifyStream struct {
	DomainID uint32
	SrcPort  uint16
}

func (*SetIpfixClassifyStream) GetMessageName() string {
	return "set_ipfix_classify_stream"
}
func (*SetIpfixClassifyStream) GetCrcString() string {
	return "7ee60f3a"
}

// SetIpfixClassifyStreamReply is the Go representation of the VPP binary API message 'set_ipfix_classify_stream_reply'.
type SetIpfixClassifyStreamReply struct {
	Retval int32
}

func (*SetIpfixClassifyStreamReply) GetMessageName() string {
	return "set_ipfix_classify_stream_reply"
}
func (*SetIpfixClassifyStreamReply) GetCrcString() string {
	return "a4d2d102"
}

// IpfixClassifyStreamDump is the Go representation of the VPP binary API message 'ipfix_classify_stream_dump'.
type IpfixClassifyStreamDump struct {
}

func (*IpfixClassifyStreamDump) GetMessageName() string {
	return "ipfix_classify_stream_dump"
}
func (*IpfixClassifyStreamDump) GetCrcString() string {
	return "81842294"
}

// IpfixClassifyStreamDetails is the Go representation of the VPP binary API message 'ipfix_classify_stream_details'.
type IpfixClassifyStreamDetails struct {
	DomainID uint32
	SrcPort  uint16
}

func (*IpfixClassifyStreamDetails) GetMessageName() string {
	return "ipfix_classify_stream_details"
}
func (*IpfixClassifyStreamDetails) GetCrcString() string {
	return "6b9383aa"
}

// IpfixClassifyTableAddDel is the Go representation of the VPP binary API message 'ipfix_classify_table_add_del'.
type IpfixClassifyTableAddDel struct {
	TableID           uint32
	IPVersion         uint8
	TransportProtocol uint8
	IsAdd             uint8
}

func (*IpfixClassifyTableAddDel) GetMessageName() string {
	return "ipfix_classify_table_add_del"
}
func (*IpfixClassifyTableAddDel) GetCrcString() string {
	return "52cc2ed9"
}

// IpfixClassifyTableAddDelReply is the Go representation of the VPP binary API message 'ipfix_classify_table_add_del_reply'.
type IpfixClassifyTableAddDelReply struct {
	Retval int32
}

func (*IpfixClassifyTableAddDelReply) GetMessageName() string {
	return "ipfix_classify_table_add_del_reply"
}
func (*IpfixClassifyTableAddDelReply) GetCrcString() string {
	return "3116af60"
}

// IpfixClassifyTableDump is the Go representation of the VPP binary API message 'ipfix_classify_table_dump'.
type IpfixClassifyTableDump struct {
}

func (*IpfixClassifyTableDump) GetMessageName() string {
	return "ipfix_classify_table_dump"
}
func (*IpfixClassifyTableDump) GetCrcString() string {
	return "b2ce9db1"
}

// IpfixClassifyTableDetails is the Go representation of the VPP binary API message 'ipfix_classify_table_details'.
type IpfixClassifyTableDetails struct {
	TableID           uint32
	IPVersion         uint8
	TransportProtocol uint8
}

func (*IpfixClassifyTableDetails) GetMessageName() string {
	return "ipfix_classify_table_details"
}
func (*IpfixClassifyTableDetails) GetCrcString() string {
	return "d0ec861f"
}

// FlowClassifySetInterface is the Go representation of the VPP binary API message 'flow_classify_set_interface'.
type FlowClassifySetInterface struct {
	SwIfIndex     uint32
	IP4TableIndex uint32
	IP6TableIndex uint32
	IsAdd         uint8
}

func (*FlowClassifySetInterface) GetMessageName() string {
	return "flow_classify_set_interface"
}
func (*FlowClassifySetInterface) GetCrcString() string {
	return "6e0a565e"
}

// FlowClassifySetInterfaceReply is the Go representation of the VPP binary API message 'flow_classify_set_interface_reply'.
type FlowClassifySetInterfaceReply struct {
	Retval int32
}

func (*FlowClassifySetInterfaceReply) GetMessageName() string {
	return "flow_classify_set_interface_reply"
}
func (*FlowClassifySetInterfaceReply) GetCrcString() string {
	return "3407e7c3"
}

// FlowClassifyDump is the Go representation of the VPP binary API message 'flow_classify_dump'.
type FlowClassifyDump struct {
	Type uint8
}

func (*FlowClassifyDump) GetMessageName() string {
	return "flow_classify_dump"
}
func (*FlowClassifyDump) GetCrcString() string {
	return "97f781c8"
}

// FlowClassifyDetails is the Go representation of the VPP binary API message 'flow_classify_details'.
type FlowClassifyDetails struct {
	SwIfIndex  uint32
	TableIndex uint32
}

func (*FlowClassifyDetails) GetMessageName() string {
	return "flow_classify_details"
}
func (*FlowClassifyDetails) GetCrcString() string {
	return "08475e65"
}

// GetNextIndex is the Go representation of the VPP binary API message 'get_next_index'.
type GetNextIndex struct {
	NodeName []byte `struc:"[64]byte"`
	NextName []byte `struc:"[64]byte"`
}

func (*GetNextIndex) GetMessageName() string {
	return "get_next_index"
}
func (*GetNextIndex) GetCrcString() string {
	return "52f0e416"
}

// GetNextIndexReply is the Go representation of the VPP binary API message 'get_next_index_reply'.
type GetNextIndexReply struct {
	Retval    int32
	NextIndex uint32
}

func (*GetNextIndexReply) GetMessageName() string {
	return "get_next_index_reply"
}
func (*GetNextIndexReply) GetCrcString() string {
	return "671fbdb1"
}

// PgCreateInterface is the Go representation of the VPP binary API message 'pg_create_interface'.
type PgCreateInterface struct {
	InterfaceID uint32
}

func (*PgCreateInterface) GetMessageName() string {
	return "pg_create_interface"
}
func (*PgCreateInterface) GetCrcString() string {
	return "253c5959"
}

// PgCreateInterfaceReply is the Go representation of the VPP binary API message 'pg_create_interface_reply'.
type PgCreateInterfaceReply struct {
	Retval    int32
	SwIfIndex uint32
}

func (*PgCreateInterfaceReply) GetMessageName() string {
	return "pg_create_interface_reply"
}
func (*PgCreateInterfaceReply) GetCrcString() string {
	return "21b4f949"
}

// PgCapture is the Go representation of the VPP binary API message 'pg_capture'.
type PgCapture struct {
	InterfaceID    uint32
	IsEnabled      uint8
	Count          uint32
	PcapNameLength uint32 `struc:"sizeof=PcapFileName"`
	PcapFileName   []byte
}

func (*PgCapture) GetMessageName() string {
	return "pg_capture"
}
func (*PgCapture) GetCrcString() string {
	return "6ac7fe78"
}

// PgCaptureReply is the Go representation of the VPP binary API message 'pg_capture_reply'.
type PgCaptureReply struct {
	Retval int32
}

func (*PgCaptureReply) GetMessageName() string {
	return "pg_capture_reply"
}
func (*PgCaptureReply) GetCrcString() string {
	return "f403693b"
}

// PgEnableDisable is the Go representation of the VPP binary API message 'pg_enable_disable'.
type PgEnableDisable struct {
	IsEnabled        uint8
	StreamNameLength uint32 `struc:"sizeof=StreamName"`
	StreamName       []byte
}

func (*PgEnableDisable) GetMessageName() string {
	return "pg_enable_disable"
}
func (*PgEnableDisable) GetCrcString() string {
	return "7d0b90ff"
}

// PgEnableDisableReply is the Go representation of the VPP binary API message 'pg_enable_disable_reply'.
type PgEnableDisableReply struct {
	Retval int32
}

func (*PgEnableDisableReply) GetMessageName() string {
	return "pg_enable_disable_reply"
}
func (*PgEnableDisableReply) GetCrcString() string {
	return "02253bd6"
}

// IPSourceAndPortRangeCheckAddDel is the Go representation of the VPP binary API message 'ip_source_and_port_range_check_add_del'.
type IPSourceAndPortRangeCheckAddDel struct {
	IsIpv6         uint8
	IsAdd          uint8
	MaskLength     uint8
	Address        []byte `struc:"[16]byte"`
	NumberOfRanges uint8
	LowPorts       []uint16 `struc:"[32]uint16"`
	HighPorts      []uint16 `struc:"[32]uint16"`
	VrfID          uint32
}

func (*IPSourceAndPortRangeCheckAddDel) GetMessageName() string {
	return "ip_source_and_port_range_check_add_del"
}
func (*IPSourceAndPortRangeCheckAddDel) GetCrcString() string {
	return "0f8c6ba0"
}

// IPSourceAndPortRangeCheckAddDelReply is the Go representation of the VPP binary API message 'ip_source_and_port_range_check_add_del_reply'.
type IPSourceAndPortRangeCheckAddDelReply struct {
	Retval int32
}

func (*IPSourceAndPortRangeCheckAddDelReply) GetMessageName() string {
	return "ip_source_and_port_range_check_add_del_reply"
}
func (*IPSourceAndPortRangeCheckAddDelReply) GetCrcString() string {
	return "35df8160"
}

// IPSourceAndPortRangeCheckInterfaceAddDel is the Go representation of the VPP binary API message 'ip_source_and_port_range_check_interface_add_del'.
type IPSourceAndPortRangeCheckInterfaceAddDel struct {
	IsAdd       uint8
	SwIfIndex   uint32
	TCPInVrfID  uint32
	TCPOutVrfID uint32
	UDPInVrfID  uint32
	UDPOutVrfID uint32
}

func (*IPSourceAndPortRangeCheckInterfaceAddDel) GetMessageName() string {
	return "ip_source_and_port_range_check_interface_add_del"
}
func (*IPSourceAndPortRangeCheckInterfaceAddDel) GetCrcString() string {
	return "4a6438f1"
}

// IPSourceAndPortRangeCheckInterfaceAddDelReply is the Go representation of the VPP binary API message 'ip_source_and_port_range_check_interface_add_del_reply'.
type IPSourceAndPortRangeCheckInterfaceAddDelReply struct {
	Retval int32
}

func (*IPSourceAndPortRangeCheckInterfaceAddDelReply) GetMessageName() string {
	return "ip_source_and_port_range_check_interface_add_del_reply"
}
func (*IPSourceAndPortRangeCheckInterfaceAddDelReply) GetCrcString() string {
	return "6b940f04"
}

// IpsecGreAddDelTunnel is the Go representation of the VPP binary API message 'ipsec_gre_add_del_tunnel'.
type IpsecGreAddDelTunnel struct {
	LocalSaID  uint32
	RemoteSaID uint32
	IsAdd      uint8
	SrcAddress []byte `struc:"[4]byte"`
	DstAddress []byte `struc:"[4]byte"`
}

func (*IpsecGreAddDelTunnel) GetMessageName() string {
	return "ipsec_gre_add_del_tunnel"
}
func (*IpsecGreAddDelTunnel) GetCrcString() string {
	return "8e39c05c"
}

// IpsecGreAddDelTunnelReply is the Go representation of the VPP binary API message 'ipsec_gre_add_del_tunnel_reply'.
type IpsecGreAddDelTunnelReply struct {
	Retval    int32
	SwIfIndex uint32
}

func (*IpsecGreAddDelTunnelReply) GetMessageName() string {
	return "ipsec_gre_add_del_tunnel_reply"
}
func (*IpsecGreAddDelTunnelReply) GetCrcString() string {
	return "91f136aa"
}

// IpsecGreTunnelDump is the Go representation of the VPP binary API message 'ipsec_gre_tunnel_dump'.
type IpsecGreTunnelDump struct {
	SwIfIndex uint32
}

func (*IpsecGreTunnelDump) GetMessageName() string {
	return "ipsec_gre_tunnel_dump"
}
func (*IpsecGreTunnelDump) GetCrcString() string {
	return "63d70659"
}

// IpsecGreTunnelDetails is the Go representation of the VPP binary API message 'ipsec_gre_tunnel_details'.
type IpsecGreTunnelDetails struct {
	SwIfIndex  uint32
	LocalSaID  uint32
	RemoteSaID uint32
	SrcAddress []byte `struc:"[4]byte"`
	DstAddress []byte `struc:"[4]byte"`
}

func (*IpsecGreTunnelDetails) GetMessageName() string {
	return "ipsec_gre_tunnel_details"
}
func (*IpsecGreTunnelDetails) GetCrcString() string {
	return "1fe2b580"
}

// DeleteSubif is the Go representation of the VPP binary API message 'delete_subif'.
type DeleteSubif struct {
	SwIfIndex uint32
}

func (*DeleteSubif) GetMessageName() string {
	return "delete_subif"
}
func (*DeleteSubif) GetCrcString() string {
	return "6038f848"
}

// DeleteSubifReply is the Go representation of the VPP binary API message 'delete_subif_reply'.
type DeleteSubifReply struct {
	Retval int32
}

func (*DeleteSubifReply) GetMessageName() string {
	return "delete_subif_reply"
}
func (*DeleteSubifReply) GetCrcString() string {
	return "9d6015dc"
}

// SwInterfaceSetDpdkHqosPipe is the Go representation of the VPP binary API message 'sw_interface_set_dpdk_hqos_pipe'.
type SwInterfaceSetDpdkHqosPipe struct {
	SwIfIndex uint32
	Subport   uint32
	Pipe      uint32
	Profile   uint32
}

func (*SwInterfaceSetDpdkHqosPipe) GetMessageName() string {
	return "sw_interface_set_dpdk_hqos_pipe"
}
func (*SwInterfaceSetDpdkHqosPipe) GetCrcString() string {
	return "be9b2181"
}

// SwInterfaceSetDpdkHqosPipeReply is the Go representation of the VPP binary API message 'sw_interface_set_dpdk_hqos_pipe_reply'.
type SwInterfaceSetDpdkHqosPipeReply struct {
	Retval int32
}

func (*SwInterfaceSetDpdkHqosPipeReply) GetMessageName() string {
	return "sw_interface_set_dpdk_hqos_pipe_reply"
}
func (*SwInterfaceSetDpdkHqosPipeReply) GetCrcString() string {
	return "1222fa49"
}

// SwInterfaceSetDpdkHqosSubport is the Go representation of the VPP binary API message 'sw_interface_set_dpdk_hqos_subport'.
type SwInterfaceSetDpdkHqosSubport struct {
	SwIfIndex uint32
	Subport   uint32
	TbRate    uint32
	TbSize    uint32
	TcRate    []uint32 `struc:"[4]uint32"`
	TcPeriod  uint32
}

func (*SwInterfaceSetDpdkHqosSubport) GetMessageName() string {
	return "sw_interface_set_dpdk_hqos_subport"
}
func (*SwInterfaceSetDpdkHqosSubport) GetCrcString() string {
	return "05a9fa2c"
}

// SwInterfaceSetDpdkHqosSubportReply is the Go representation of the VPP binary API message 'sw_interface_set_dpdk_hqos_subport_reply'.
type SwInterfaceSetDpdkHqosSubportReply struct {
	Retval int32
}

func (*SwInterfaceSetDpdkHqosSubportReply) GetMessageName() string {
	return "sw_interface_set_dpdk_hqos_subport_reply"
}
func (*SwInterfaceSetDpdkHqosSubportReply) GetCrcString() string {
	return "147f258a"
}

// SwInterfaceSetDpdkHqosTctbl is the Go representation of the VPP binary API message 'sw_interface_set_dpdk_hqos_tctbl'.
type SwInterfaceSetDpdkHqosTctbl struct {
	SwIfIndex uint32
	Entry     uint32
	Tc        uint32
	Queue     uint32
}

func (*SwInterfaceSetDpdkHqosTctbl) GetMessageName() string {
	return "sw_interface_set_dpdk_hqos_tctbl"
}
func (*SwInterfaceSetDpdkHqosTctbl) GetCrcString() string {
	return "f9d98f13"
}

// SwInterfaceSetDpdkHqosTctblReply is the Go representation of the VPP binary API message 'sw_interface_set_dpdk_hqos_tctbl_reply'.
type SwInterfaceSetDpdkHqosTctblReply struct {
	Retval int32
}

func (*SwInterfaceSetDpdkHqosTctblReply) GetMessageName() string {
	return "sw_interface_set_dpdk_hqos_tctbl_reply"
}
func (*SwInterfaceSetDpdkHqosTctblReply) GetCrcString() string {
	return "4e2f524d"
}

// L2InterfacePbbTagRewrite is the Go representation of the VPP binary API message 'l2_interface_pbb_tag_rewrite'.
type L2InterfacePbbTagRewrite struct {
	SwIfIndex uint32
	VtrOp     uint32
	OuterTag  uint16
	BDmac     []byte `struc:"[6]byte"`
	BSmac     []byte `struc:"[6]byte"`
	BVlanid   uint16
	ISid      uint32
}

func (*L2InterfacePbbTagRewrite) GetMessageName() string {
	return "l2_interface_pbb_tag_rewrite"
}
func (*L2InterfacePbbTagRewrite) GetCrcString() string {
	return "b7706c15"
}

// L2InterfacePbbTagRewriteReply is the Go representation of the VPP binary API message 'l2_interface_pbb_tag_rewrite_reply'.
type L2InterfacePbbTagRewriteReply struct {
	Retval int32
}

func (*L2InterfacePbbTagRewriteReply) GetMessageName() string {
	return "l2_interface_pbb_tag_rewrite_reply"
}
func (*L2InterfacePbbTagRewriteReply) GetCrcString() string {
	return "2d083312"
}

// Punt is the Go representation of the VPP binary API message 'punt'.
type Punt struct {
	IsAdd      uint8
	Ipv        uint8
	L4Protocol uint8
	L4Port     uint16
}

func (*Punt) GetMessageName() string {
	return "punt"
}
func (*Punt) GetCrcString() string {
	return "4559c976"
}

// PuntReply is the Go representation of the VPP binary API message 'punt_reply'.
type PuntReply struct {
	Retval int32
}

func (*PuntReply) GetMessageName() string {
	return "punt_reply"
}
func (*PuntReply) GetCrcString() string {
	return "cca27fbe"
}

// IpsecSpdDump is the Go representation of the VPP binary API message 'ipsec_spd_dump'.
type IpsecSpdDump struct {
	SpdID uint32
	SaID  uint32
}

func (*IpsecSpdDump) GetMessageName() string {
	return "ipsec_spd_dump"
}
func (*IpsecSpdDump) GetCrcString() string {
	return "5e9ae88e"
}

// IpsecSpdDetails is the Go representation of the VPP binary API message 'ipsec_spd_details'.
type IpsecSpdDetails struct {
	SpdID           uint32
	Priority        int32
	IsOutbound      uint8
	IsIpv6          uint8
	LocalStartAddr  []byte `struc:"[16]byte"`
	LocalStopAddr   []byte `struc:"[16]byte"`
	LocalStartPort  uint16
	LocalStopPort   uint16
	RemoteStartAddr []byte `struc:"[16]byte"`
	RemoteStopAddr  []byte `struc:"[16]byte"`
	RemoteStartPort uint16
	RemoteStopPort  uint16
	Protocol        uint8
	Policy          uint8
	SaID            uint32
	Bytes           uint64
	Packets         uint64
}

func (*IpsecSpdDetails) GetMessageName() string {
	return "ipsec_spd_details"
}
func (*IpsecSpdDetails) GetCrcString() string {
	return "6f7821b0"
}

// FeatureEnableDisable is the Go representation of the VPP binary API message 'feature_enable_disable'.
type FeatureEnableDisable struct {
	SwIfIndex   uint32
	Enable      uint8
	ArcName     []byte `struc:"[64]byte"`
	FeatureName []byte `struc:"[64]byte"`
}

func (*FeatureEnableDisable) GetMessageName() string {
	return "feature_enable_disable"
}
func (*FeatureEnableDisable) GetCrcString() string {
	return "bc86393b"
}

// FeatureEnableDisableReply is the Go representation of the VPP binary API message 'feature_enable_disable_reply'.
type FeatureEnableDisableReply struct {
	Retval int32
}

func (*FeatureEnableDisableReply) GetMessageName() string {
	return "feature_enable_disable_reply"
}
func (*FeatureEnableDisableReply) GetCrcString() string {
	return "f6e14373"
}

// BfdSetConfig is the Go representation of the VPP binary API message 'bfd_set_config'.
type BfdSetConfig struct {
	SlowTimer  uint32
	MinTx      uint32
	MinRx      uint32
	DetectMult uint8
}

func (*BfdSetConfig) GetMessageName() string {
	return "bfd_set_config"
}
func (*BfdSetConfig) GetCrcString() string {
	return "ab63cc63"
}

// BfdSetConfigReply is the Go representation of the VPP binary API message 'bfd_set_config_reply'.
type BfdSetConfigReply struct {
	Retval int32
}

func (*BfdSetConfigReply) GetMessageName() string {
	return "bfd_set_config_reply"
}
func (*BfdSetConfigReply) GetCrcString() string {
	return "ddd8026e"
}

// BfdGetConfig is the Go representation of the VPP binary API message 'bfd_get_config'.
type BfdGetConfig struct {
}

func (*BfdGetConfig) GetMessageName() string {
	return "bfd_get_config"
}
func (*BfdGetConfig) GetCrcString() string {
	return "d3e64ab6"
}

// BfdGetConfigReply is the Go representation of the VPP binary API message 'bfd_get_config_reply'.
type BfdGetConfigReply struct {
	SlowTimer  uint32
	MinTx      uint32
	MinRx      uint32
	DetectMult uint8
}

func (*BfdGetConfigReply) GetMessageName() string {
	return "bfd_get_config_reply"
}
func (*BfdGetConfigReply) GetCrcString() string {
	return "79ff1083"
}

// BfdUDPAdd is the Go representation of the VPP binary API message 'bfd_udp_add'.
type BfdUDPAdd struct {
	SwIfIndex     uint32
	DesiredMinTx  uint32
	RequiredMinRx uint32
	LocalAddr     []byte `struc:"[16]byte"`
	PeerAddr      []byte `struc:"[16]byte"`
	IsIpv6        uint8
	DetectMult    uint8
}

func (*BfdUDPAdd) GetMessageName() string {
	return "bfd_udp_add"
}
func (*BfdUDPAdd) GetCrcString() string {
	return "e6631839"
}

// BfdUDPAddReply is the Go representation of the VPP binary API message 'bfd_udp_add_reply'.
type BfdUDPAddReply struct {
	Retval  int32
	BsIndex uint32
}

func (*BfdUDPAddReply) GetMessageName() string {
	return "bfd_udp_add_reply"
}
func (*BfdUDPAddReply) GetCrcString() string {
	return "af5e0fd1"
}

// BfdUDPDel is the Go representation of the VPP binary API message 'bfd_udp_del'.
type BfdUDPDel struct {
	SwIfIndex uint32
	LocalAddr []byte `struc:"[16]byte"`
	PeerAddr  []byte `struc:"[16]byte"`
	IsIpv6    uint8
}

func (*BfdUDPDel) GetMessageName() string {
	return "bfd_udp_del"
}
func (*BfdUDPDel) GetCrcString() string {
	return "e95cc3ee"
}

// BfdUDPDelReply is the Go representation of the VPP binary API message 'bfd_udp_del_reply'.
type BfdUDPDelReply struct {
	Retval int32
}

func (*BfdUDPDelReply) GetMessageName() string {
	return "bfd_udp_del_reply"
}
func (*BfdUDPDelReply) GetCrcString() string {
	return "b9b0b355"
}

// BfdUDPSessionDump is the Go representation of the VPP binary API message 'bfd_udp_session_dump'.
type BfdUDPSessionDump struct {
}

func (*BfdUDPSessionDump) GetMessageName() string {
	return "bfd_udp_session_dump"
}
func (*BfdUDPSessionDump) GetCrcString() string {
	return "b5bd25a6"
}

// BfdUDPSessionDetails is the Go representation of the VPP binary API message 'bfd_udp_session_details'.
type BfdUDPSessionDetails struct {
	BsIndex   uint32
	SwIfIndex uint32
	LocalAddr []byte `struc:"[16]byte"`
	PeerAddr  []byte `struc:"[16]byte"`
	IsIpv6    uint8
	State     uint8
}

func (*BfdUDPSessionDetails) GetMessageName() string {
	return "bfd_udp_session_details"
}
func (*BfdUDPSessionDetails) GetCrcString() string {
	return "e6c6bcb4"
}

// BfdSessionSetFlags is the Go representation of the VPP binary API message 'bfd_session_set_flags'.
type BfdSessionSetFlags struct {
	BsIndex     uint32
	AdminUpDown uint8
}

func (*BfdSessionSetFlags) GetMessageName() string {
	return "bfd_session_set_flags"
}
func (*BfdSessionSetFlags) GetCrcString() string {
	return "9c5c29eb"
}

// BfdSessionSetFlagsReply is the Go representation of the VPP binary API message 'bfd_session_set_flags_reply'.
type BfdSessionSetFlagsReply struct {
	Retval int32
}

func (*BfdSessionSetFlagsReply) GetMessageName() string {
	return "bfd_session_set_flags_reply"
}
func (*BfdSessionSetFlagsReply) GetCrcString() string {
	return "186f1fac"
}

// WantBfdEvents is the Go representation of the VPP binary API message 'want_bfd_events'.
type WantBfdEvents struct {
	EnableDisable uint32
	Pid           uint32
}

func (*WantBfdEvents) GetMessageName() string {
	return "want_bfd_events"
}
func (*WantBfdEvents) GetCrcString() string {
	return "bc6547f0"
}

// WantBfdEventsReply is the Go representation of the VPP binary API message 'want_bfd_events_reply'.
type WantBfdEventsReply struct {
	Retval int32
}

func (*WantBfdEventsReply) GetMessageName() string {
	return "want_bfd_events_reply"
}
func (*WantBfdEventsReply) GetCrcString() string {
	return "be8b3ff3"
}
