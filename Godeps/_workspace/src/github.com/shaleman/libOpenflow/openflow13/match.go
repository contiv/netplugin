package openflow13

import (
    "net"
    "log"
    "encoding/binary"

    "github.com/shaleman/libOpenflow/util"
)

// ofp_match 1.3
type Match struct {
    Type        uint16
    Length      uint16
    Fields      []MatchField
}

// One match field TLV
type MatchField struct {
    Class   uint16
    Field   uint8
    HasMask bool
    Length  uint8
    Value   util.Message
    Mask    util.Message
}

func NewMatch() *Match {
    m := new(Match)

    m.Type = MatchType_OXM
    m.Length = 4
    m.Fields = make([]MatchField, 0)

    return m
}

func (m *Match) Len() (n uint16) {
    n = 4
    for _, a := range m.Fields {
        n += a.Len()
    }

    // Round it to closest multiple of 8
    n = ((n + 7)/8)*8

    return
}

func (m *Match) MarshalBinary() (data []byte, err error) {
    data = make([]byte, int(m.Len()))

    n := 0
    binary.BigEndian.PutUint16(data[n:], m.Type)
    n += 2
    binary.BigEndian.PutUint16(data[n:], m.Length)
    n += 2

    for _, a := range m.Fields {
        b, err := a.MarshalBinary()
        if err != nil {
            return nil, err
        }
        copy(data[n:], b)
        n += len(b)
    }

    /* See if we need to pad it to make it align to 64bit boundary
    if ((n % 8) != 0) {
        toPad := 8 - (n % 8)
        b := make([]byte, toPad)
        data = append(data, b...)
    }
    */

    return
}

func (m *Match) UnmarshalBinary(data []byte) error {

    n := 0
    m.Type = binary.BigEndian.Uint16(data[n:])
    n += 2
    m.Length = binary.BigEndian.Uint16(data[n:])
    n += 2

    for ;n < int(m.Length); {
        field := new(MatchField)
        field.UnmarshalBinary(data[n:])
        m.Fields = append(m.Fields, *field)
        n += int(field.Len())
    }
    return nil
}

func (m *Match) AddField(f MatchField) {
    m.Fields = append(m.Fields, f)
    m.Length += f.Len()
}

func (m *MatchField) Len() (n uint16) {
    n = 4
    n += m.Value.Len()
    if (m.HasMask) {
        n += m.Mask.Len()
    }

    return
}

func (m *MatchField) MarshalBinary() (data []byte, err error) {
    data = make([]byte, int(m.Len()))

    n := 0
    binary.BigEndian.PutUint16(data[n:], m.Class)
    n += 2

    var fld uint8
    if (m.HasMask) {
        fld = (m.Field << 1) | 0x1
    } else {
        fld = (m.Field << 1) | 0x0
    }
    data[n] = fld
    n += 1

    data[n] = m.Length
    n += 1

    b, err := m.Value.MarshalBinary()
    copy(data[n:], b)
    n += len(b)

    if (m.HasMask) {
        b, err = m.Mask.MarshalBinary()
        copy(data[n:], b)
        n += len(b)
    }
    return
}

func (m *MatchField) UnmarshalBinary(data []byte) error {
    var n uint16 = 0
    m.Class = binary.BigEndian.Uint16(data[n:])
    n += 2

    fld := data[n]
    n += 1
    if ((fld & 0x1) == 1) {
        m.HasMask = true
    } else {
        m.HasMask = false
    }
    m.Field = fld >> 1

    m.Length = data[n]
    n += 1

    m.Value = DecodeMatchField(m.Class, m.Field, data[n:])
    n += m.Value.Len()

    if (m.HasMask) {
        m.Mask = DecodeMatchField(m.Class, m.Field, data[n:])
        n += m.Mask.Len()
    }
    return nil
}

func DecodeMatchField(class uint16, field uint8, data []byte) util.Message {
    if (class == OXM_CLASS_OPENFLOW_BASIC) {
        var val util.Message
        switch (field) {
        case OXM_FIELD_IN_PORT:
            val = new(InPortField)
        case OXM_FIELD_IN_PHY_PORT:
        case OXM_FIELD_METADATA:
        case OXM_FIELD_ETH_DST:
            val = new(EthDstField)
        case OXM_FIELD_ETH_SRC:
            val = new(EthSrcField)
        case OXM_FIELD_ETH_TYPE:
            val = new(EthTypeField)
        case OXM_FIELD_VLAN_VID:
            val = new(VlanIdField)
        case OXM_FIELD_VLAN_PCP:
        case OXM_FIELD_IP_DSCP:
        case OXM_FIELD_IP_ECN:
        case OXM_FIELD_IP_PROTO:
        case OXM_FIELD_IPV4_SRC:
            val = new(Ipv4SrcField)
        case OXM_FIELD_IPV4_DST:
            val = new(Ipv4DstField)
        case OXM_FIELD_TCP_SRC:
        case OXM_FIELD_TCP_DST:
        case OXM_FIELD_UDP_SRC:
        case OXM_FIELD_UDP_DST:
        case OXM_FIELD_SCTP_SRC:
        case OXM_FIELD_SCTP_DST:
        case OXM_FIELD_ICMPV4_TYPE:
        case OXM_FIELD_ICMPV4_CODE:
        case OXM_FIELD_ARP_OP:
        case OXM_FIELD_ARP_SPA:
        case OXM_FIELD_ARP_TPA:
        case OXM_FIELD_ARP_SHA:
        case OXM_FIELD_ARP_THA:
        case OXM_FIELD_IPV6_SRC:
        case OXM_FIELD_IPV6_DST:
        case OXM_FIELD_IPV6_FLABEL:
        case OXM_FIELD_ICMPV6_TYPE:
        case OXM_FIELD_ICMPV6_CODE:
        case OXM_FIELD_IPV6_ND_TARGET:
        case OXM_FIELD_IPV6_ND_SLL:
        case OXM_FIELD_IPV6_ND_TLL:
        case OXM_FIELD_MPLS_LABEL:
        case OXM_FIELD_MPLS_TC:
        case OXM_FIELD_MPLS_BOS:
        case OXM_FIELD_PBB_ISID:
        case OXM_FIELD_TUNNEL_ID:
            val = new(TunnelIdField)
        case OXM_FIELD_IPV6_EXTHDR:
        }

        val.UnmarshalBinary(data)
        return val
    } else {
        log.Panic("Unsupported match field class")
    }

    return nil
}

//  ofp_match_type 1.3
const (
    MatchType_Standard = iota /* Deprecated. */
    MatchType_OXM
)

// ofp_oxm_class 1.3
const (
    OXM_CLASS_NXM_0          = 0x0000     /* Backward compatibility with NXM */
    OXM_CLASS_NXM_1          = 0x0001     /* Backward compatibility with NXM */
    OXM_CLASS_OPENFLOW_BASIC = 0x8000     /* Basic class for OpenFlow */
    OXM_CLASS_EXPERIMENTER   = 0xFFFF     /* Experimenter class */
)

const (
    OXM_FIELD_IN_PORT      = 0    /* Switch input port. */
    OXM_FIELD_IN_PHY_PORT  = 1    /* Switch physical input port. */
    OXM_FIELD_METADATA     = 2    /* Metadata passed between tables. */
    OXM_FIELD_ETH_DST      = 3    /* Ethernet destination address. */
    OXM_FIELD_ETH_SRC      = 4    /* Ethernet source address. */
    OXM_FIELD_ETH_TYPE     = 5    /* Ethernet frame type. */
    OXM_FIELD_VLAN_VID     = 6    /* VLAN id. */
    OXM_FIELD_VLAN_PCP     = 7    /* VLAN priority. */
    OXM_FIELD_IP_DSCP      = 8    /* IP DSCP (6 bits in ToS field). */
    OXM_FIELD_IP_ECN       = 9    /* IP ECN (2 bits in ToS field). */
    OXM_FIELD_IP_PROTO     = 10   /* IP protocol. */
    OXM_FIELD_IPV4_SRC     = 11   /* IPv4 source address. */
    OXM_FIELD_IPV4_DST     = 12   /* IPv4 destination address. */
    OXM_FIELD_TCP_SRC      = 13   /* TCP source port. */
    OXM_FIELD_TCP_DST      = 14   /* TCP destination port. */
    OXM_FIELD_UDP_SRC      = 15   /* UDP source port. */
    OXM_FIELD_UDP_DST      = 16   /* UDP destination port. */
    OXM_FIELD_SCTP_SRC     = 17   /* SCTP source port. */
    OXM_FIELD_SCTP_DST     = 18   /* SCTP destination port. */
    OXM_FIELD_ICMPV4_TYPE  = 19   /* ICMP type. */
    OXM_FIELD_ICMPV4_CODE  = 20   /* ICMP code. */
    OXM_FIELD_ARP_OP       = 21   /* ARP opcode. */
    OXM_FIELD_ARP_SPA      = 22   /* ARP source IPv4 address. */
    OXM_FIELD_ARP_TPA      = 23   /* ARP target IPv4 address. */
    OXM_FIELD_ARP_SHA      = 24   /* ARP source hardware address. */
    OXM_FIELD_ARP_THA      = 25   /* ARP target hardware address. */
    OXM_FIELD_IPV6_SRC     = 26   /* IPv6 source address. */
    OXM_FIELD_IPV6_DST     = 27   /* IPv6 destination address. */
    OXM_FIELD_IPV6_FLABEL  = 28   /* IPv6 Flow Label */
    OXM_FIELD_ICMPV6_TYPE  = 29   /* ICMPv6 type. */
    OXM_FIELD_ICMPV6_CODE  = 30   /* ICMPv6 code. */
    OXM_FIELD_IPV6_ND_TARGET = 31 /* Target address for ND. */
    OXM_FIELD_IPV6_ND_SLL  = 32   /* Source link-layer for ND. */
    OXM_FIELD_IPV6_ND_TLL  = 33   /* Target link-layer for ND. */
    OXM_FIELD_MPLS_LABEL   = 34   /* MPLS label. */
    OXM_FIELD_MPLS_TC      = 35   /* MPLS TC. */
    OXM_FIELD_MPLS_BOS     = 36   /* MPLS BoS bit. */
    OXM_FIELD_PBB_ISID     = 37   /* PBB I-SID. */
    OXM_FIELD_TUNNEL_ID    = 38   /* Logical Port Metadata. */
    OXM_FIELD_IPV6_EXTHDR  = 39   /* IPv6 Extension Header pseudo-field */
)

// IN_PORT field
type InPortField struct {
    InPort      uint32
}

func (m *InPortField) Len() uint16 {
    return 4
}
func (m *InPortField) MarshalBinary() (data []byte, err error) {
    data = make([]byte, 4)

    binary.BigEndian.PutUint32(data, m.InPort)
    return
}
func (m *InPortField) UnmarshalBinary(data []byte) error {
    m.InPort = binary.BigEndian.Uint32(data)
    return nil
}

// Return a MatchField for Input port matching
func NewInPortField(inPort uint32) *MatchField {
    f := new(MatchField)
    f.Class = OXM_CLASS_OPENFLOW_BASIC
    f.Field = OXM_FIELD_IN_PORT
    f.HasMask = false

    inPortField := new(InPortField)
    inPortField.InPort = inPort
    f.Value = inPortField
    f.Length = uint8(inPortField.Len())

    return f
}


// ETH_DST field
type EthDstField struct {
    EthDst      net.HardwareAddr
}

func (m *EthDstField) Len() uint16 {
    return 6
}
func (m *EthDstField) MarshalBinary() (data []byte, err error) {
    data = make([]byte, 6)
    copy(data, m.EthDst)
    return
}

func (m *EthDstField) UnmarshalBinary(data []byte) error {
    copy(m.EthDst, data)
    return nil
}

// Return a MatchField for ethernet dest addr
func NewEthDstField(ethDst net.HardwareAddr, ethDstMask *net.HardwareAddr) *MatchField {
    f := new(MatchField)
    f.Class = OXM_CLASS_OPENFLOW_BASIC
    f.Field = OXM_FIELD_ETH_DST
    f.HasMask = false

    ethDstField := new(EthDstField)
    ethDstField.EthDst = ethDst
    f.Value = ethDstField
    f.Length = uint8(ethDstField.Len())

    // Add the mask
    if (ethDstMask != nil) {
        mask := new(EthDstField)
        mask.EthDst = *ethDstMask
        f.Mask = mask
        f.HasMask = true
        f.Length += uint8(mask.Len())
    }

    return f
}

// ETH_SRC field
type EthSrcField struct {
    EthSrc      net.HardwareAddr
}

func (m *EthSrcField) Len() uint16 {
    return 6
}
func (m *EthSrcField) MarshalBinary() (data []byte, err error) {
    data = make([]byte, 6)
    copy(data, m.EthSrc)
    return
}

func (m *EthSrcField) UnmarshalBinary(data []byte) error {
    copy(m.EthSrc, data)
    return nil
}

// Return a MatchField for ethernet src addr
func NewEthSrcField(ethSrc net.HardwareAddr, ethSrcMask *net.HardwareAddr) *MatchField {
    f := new(MatchField)
    f.Class = OXM_CLASS_OPENFLOW_BASIC
    f.Field = OXM_FIELD_ETH_SRC
    f.HasMask = false

    ethSrcField := new(EthSrcField)
    ethSrcField.EthSrc = ethSrc
    f.Value = ethSrcField
    f.Length = uint8(ethSrcField.Len())

    // Add the mask
    if (ethSrcMask != nil) {
        mask := new(EthSrcField)
        mask.EthSrc = *ethSrcMask
        f.Mask = mask
        f.HasMask = true
        f.Length += uint8(mask.Len())
    }

    return f
}

// ETH_TYPE field
type EthTypeField struct {
    EthType      uint16
}

func (m *EthTypeField) Len() uint16 {
    return 2
}
func (m *EthTypeField) MarshalBinary() (data []byte, err error) {
    data = make([]byte, 2)

    binary.BigEndian.PutUint16(data, m.EthType)
    return
}
func (m *EthTypeField) UnmarshalBinary(data []byte) error {
    m.EthType = binary.BigEndian.Uint16(data)
    return nil
}

// Return a MatchField for ethertype matching
func NewEthTypeField(ethType uint16) *MatchField {
    f := new(MatchField)
    f.Class = OXM_CLASS_OPENFLOW_BASIC
    f.Field = OXM_FIELD_ETH_TYPE
    f.HasMask = false

    ethTypeField := new(EthTypeField)
    ethTypeField.EthType = ethType
    f.Value = ethTypeField
    f.Length = uint8(ethTypeField.Len())

    return f
}

const OFPVID_PRESENT = 0x1000   /* Bit that indicate that a VLAN id is set */
const OFPVID_NONE = 0x0000      /* No VLAN id was set. */

// VLAN_ID field
type VlanIdField struct {
    VlanId      uint16
}

func (m *VlanIdField) Len() uint16 {
    return 2
}
func (m *VlanIdField) MarshalBinary() (data []byte, err error) {
    data = make([]byte, 2)

    binary.BigEndian.PutUint16(data, m.VlanId)
    return
}
func (m *VlanIdField) UnmarshalBinary(data []byte) error {
    m.VlanId = binary.BigEndian.Uint16(data)
    return nil
}

// Return a MatchField for vlan id matching
func NewVlanIdField(vlanId uint16) *MatchField {
    f := new(MatchField)
    f.Class = OXM_CLASS_OPENFLOW_BASIC
    f.Field = OXM_FIELD_VLAN_VID
    f.HasMask = false

    vlanIdField := new(VlanIdField)
    vlanIdField.VlanId = vlanId | OFPVID_PRESENT
    f.Value = vlanIdField
    f.Length = uint8(vlanIdField.Len())

    return f
}

// IPV4_SRC field
type Ipv4SrcField struct {
    Ipv4Src      net.IP
}

func (m *Ipv4SrcField) Len() uint16 {
    return 4
}
func (m *Ipv4SrcField) MarshalBinary() (data []byte, err error) {
    data = make([]byte, 4)
    copy(data, m.Ipv4Src.To4())
    return
}

func (m *Ipv4SrcField) UnmarshalBinary(data []byte) error {
    m.Ipv4Src = net.IPv4(data[0], data[1], data[2], data[3])
    return nil
}

// Return a MatchField for ipv4 src addr
func NewIpv4SrcField(ipSrc net.IP, ipSrcMask *net.IP) *MatchField {
    f := new(MatchField)
    f.Class = OXM_CLASS_OPENFLOW_BASIC
    f.Field = OXM_FIELD_IPV4_SRC
    f.HasMask = false

    ipSrcField := new(Ipv4SrcField)
    ipSrcField.Ipv4Src = ipSrc
    f.Value = ipSrcField
    f.Length = uint8(ipSrcField.Len())

    // Add the mask
    if (ipSrcMask != nil) {
        mask := new(Ipv4SrcField)
        mask.Ipv4Src = *ipSrcMask
        f.Mask = mask
        f.HasMask = true
        f.Length += uint8(mask.Len())
    }

    return f
}

// IPV4_DST field
type Ipv4DstField struct {
    Ipv4Dst      net.IP
}

func (m *Ipv4DstField) Len() uint16 {
    return 4
}
func (m *Ipv4DstField) MarshalBinary() (data []byte, err error) {
    data = make([]byte, 4)
    copy(data, m.Ipv4Dst.To4())
    return
}

func (m *Ipv4DstField) UnmarshalBinary(data []byte) error {
    m.Ipv4Dst = net.IPv4(data[0], data[1], data[2], data[3])
    return nil
}

// Return a MatchField for ipv4 dest addr
func NewIpv4DstField(ipDst net.IP, ipDstMask *net.IP) *MatchField {
    f := new(MatchField)
    f.Class = OXM_CLASS_OPENFLOW_BASIC
    f.Field = OXM_FIELD_IPV4_DST
    f.HasMask = false

    ipDstField := new(Ipv4DstField)
    ipDstField.Ipv4Dst = ipDst
    f.Value = ipDstField
    f.Length = uint8(ipDstField.Len())

    // Add the mask
    if (ipDstMask != nil) {
        mask := new(Ipv4DstField)
        mask.Ipv4Dst = *ipDstMask
        f.Mask = mask
        f.HasMask = true
        f.Length += uint8(mask.Len())
    }

    return f
}

// TUNNEL_ID field
type TunnelIdField struct {
    TunnelId      uint64
}

func (m *TunnelIdField) Len() uint16 {
    return 8
}
func (m *TunnelIdField) MarshalBinary() (data []byte, err error) {
    data = make([]byte, m.Len())

    binary.BigEndian.PutUint64(data, m.TunnelId)
    return
}
func (m *TunnelIdField) UnmarshalBinary(data []byte) error {
    m.TunnelId = binary.BigEndian.Uint64(data)
    return nil
}

// Return a MatchField for tunel id matching
func NewTunnelIdField(tunnelId uint64) *MatchField {
    f := new(MatchField)
    f.Class = OXM_CLASS_OPENFLOW_BASIC
    f.Field = OXM_FIELD_TUNNEL_ID
    f.HasMask = false

    tunnelIdField := new(TunnelIdField)
    tunnelIdField.TunnelId = tunnelId
    f.Value = tunnelIdField
    f.Length = uint8(tunnelIdField.Len())

    return f
}

// METADATA field
type MetadataField struct {
    Metadata      uint64
}

func (m *MetadataField) Len() uint16 {
    return 8
}
func (m *MetadataField) MarshalBinary() (data []byte, err error) {
    data = make([]byte, m.Len())

    binary.BigEndian.PutUint64(data, m.Metadata)
    return
}
func (m *MetadataField) UnmarshalBinary(data []byte) error {
    m.Metadata = binary.BigEndian.Uint64(data)
    return nil
}

// Return a MatchField for tunel id matching
func NewMetadataField(metadata uint64, metadataMask *uint64) *MatchField {
    f := new(MatchField)
    f.Class = OXM_CLASS_OPENFLOW_BASIC
    f.Field = OXM_FIELD_METADATA
    f.HasMask = false

    metadataField := new(MetadataField)
    metadataField.Metadata = metadata
    f.Value = metadataField
    f.Length = uint8(metadataField.Len())

    // Add the mask
    if (metadataMask != nil) {
        mask := new(MetadataField)
        mask.Metadata = *metadataMask
        f.Mask = mask
        f.HasMask = true
        f.Length += uint8(mask.Len())
    }

    return f
}


// FIXME: Need to add following fields
// IP_PROTO
// IPV4_SRC
// IPV4_DST
// ARP_OP
// TCP_SRC
// TCP_DST
// UDP_SRC
// UDP_DST
