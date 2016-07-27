package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math/rand"
	"net"
)

const (
	DHCP_MSG_BOOT_REQ byte = iota
	DHCP_MSG_BOOT_RES
)

type DHCPOperation byte

const (
	DHCP_MSG_UNSPEC DHCPOperation = iota
	DHCP_MSG_DISCOVER
	DHCP_MSG_OFFER
	DHCP_MSG_REQUEST
	DHCP_MSG_DECLINE
	DHCP_MSG_ACK
	DHCP_MSG_NAK
	DHCP_MSG_RELEASE
	DHCP_MSG_INFORM
)

var dhcpMagic uint32 = 0x63825363

type DHCP struct {
	Operation    DHCPOperation
	HardwareType byte
	HardwareLen  uint8
	HardwareOpts uint8
	Xid          uint32
	Secs         uint16
	Flags        uint16
	ClientIP     net.IP
	YourIP       net.IP
	ServerIP     net.IP
	GatewayIP    net.IP
	ClientHWAddr net.HardwareAddr
	ServerName   [64]byte
	File         [128]byte
	Options      []DHCPOption
}

const (
	DHCP_OPT_REQUEST_IP     byte = iota + 50 // 0x32, 4, net.IP
	DHCP_OPT_LEASE_TIME                      // 0x33, 4, uint32
	DHCP_OPT_EXT_OPTS                        // 0x34, 1, 1/2/3
	DHCP_OPT_MESSAGE_TYPE                    // 0x35, 1, 1-7
	DHCP_OPT_SERVER_ID                       // 0x36, 4, net.IP
	DHCP_OPT_PARAMS_REQUEST                  // 0x37, n, []byte
	DHCP_OPT_MESSAGE                         // 0x38, n, string
	DHCP_OPT_MAX_DHCP_SIZE                   // 0x39, 2, uint16
	DHCP_OPT_T1                              // 0x3a, 4, uint32
	DHCP_OPT_T2                              // 0x3b, 4, uint32
	DHCP_OPT_CLASS_ID                        // 0x3c, n, []byte
	DHCP_OPT_CLIENT_ID                       // 0x3d, n >=  2, []byte

)

const (
	DHCP_HW_ETHERNET byte = 0x01
)

const (
	DHCP_FLAG_BROADCAST uint16 = 0x80

//	FLAG_BROADCAST_MASK uint16 = (1 << FLAG_BROADCAST)
)

func NewDHCP(xid uint32, op DHCPOperation, hwtype byte) (*DHCP, error) {
	if xid == 0 {
		xid = rand.Uint32()
	}
	switch hwtype {
	case DHCP_HW_ETHERNET:
		break
	default:
		return nil, errors.New("Bad HardwareType")
	}
	d := &DHCP{
		Operation:    op,
		HardwareType: hwtype,
		Xid:          xid,
		ClientIP:     make([]byte, 4),
		YourIP:       make([]byte, 4),
		ServerIP:     make([]byte, 4),
		GatewayIP:    make([]byte, 4),
		ClientHWAddr: make([]byte, 16),
	}
	return d, nil
}

func (d *DHCP) Len() (n uint16) {
	n += uint16(240)
	optend := false
	for _, opt := range d.Options {
		n += opt.Len()
		if opt.OptionType() == DHCP_OPT_END {
			optend = true
		}
	}
	if !optend {
		n += 1
	}
	return
}

func (d *DHCP) Read(b []byte) (n int, err error) {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, d.Operation)
	n += 1
	binary.Write(buf, binary.BigEndian, d.HardwareType)
	n += 1
	binary.Write(buf, binary.BigEndian, d.HardwareLen)
	n += 1
	binary.Write(buf, binary.BigEndian, d.HardwareOpts)
	n += 1
	binary.Write(buf, binary.BigEndian, d.Xid)
	n += 4
	binary.Write(buf, binary.BigEndian, d.Secs)
	n += 2
	binary.Write(buf, binary.BigEndian, d.Flags)
	n += 2
	binary.Write(buf, binary.BigEndian, d.ClientIP)
	n += 4
	binary.Write(buf, binary.BigEndian, d.YourIP)
	n += 4
	binary.Write(buf, binary.BigEndian, d.ServerIP)
	n += 4
	binary.Write(buf, binary.BigEndian, d.GatewayIP)
	n += 4
	clientHWAddr := make([]byte, 16)
	copy(clientHWAddr[0:], d.ClientHWAddr)
	binary.Write(buf, binary.BigEndian, clientHWAddr)
	n += 16
	binary.Write(buf, binary.BigEndian, d.ServerName)
	n += 64
	binary.Write(buf, binary.BigEndian, d.File)
	n += 128
	binary.Write(buf, binary.BigEndian, dhcpMagic)
	n += 4

	optend := false
	for _, opt := range d.Options {
		m, err := DHCPWriteOption(buf, opt)
		n += m
		if err != nil {
			return n, err
		}
		if opt.OptionType() == DHCP_OPT_END {
			optend = true
		}
	}
	if !optend {
		m, err := DHCPWriteOption(buf, DHCPNewOption(DHCP_OPT_END, nil))
		n += m
		if err != nil {
			return n, err
		}
	}
	if n, err = buf.Read(b); n == 0 {
		return
	}
	return n, nil
}

func (d *DHCP) Write(b []byte) (n int, err error) {
	if len(b) < 240 {
		return 0, errors.New("ErrTruncated")
	}
	buf := bytes.NewBuffer(b)

	if err = binary.Read(buf, binary.BigEndian, &d.Operation); err != nil {
		return
	}
	n += 1
	if err = binary.Read(buf, binary.BigEndian, &d.HardwareType); err != nil {
		return
	}
	n += 1
	if err = binary.Read(buf, binary.BigEndian, &d.HardwareLen); err != nil {
		return
	}
	n += 1
	if err = binary.Read(buf, binary.BigEndian, &d.HardwareOpts); err != nil {
		return
	}
	n += 1
	if err = binary.Read(buf, binary.BigEndian, &d.Xid); err != nil {
		return
	}
	n += 4
	if err = binary.Read(buf, binary.BigEndian, &d.Secs); err != nil {
		return
	}
	n += 2
	if err = binary.Read(buf, binary.BigEndian, &d.Flags); err != nil {
		return
	}
	n += 2
	d.ClientIP = make([]byte, 4)
	if err = binary.Read(buf, binary.BigEndian, &d.ClientIP); err != nil {
		return
	}
	n += 4
	d.YourIP = make([]byte, 4)
	if err = binary.Read(buf, binary.BigEndian, &d.YourIP); err != nil {
		return
	}
	n += 4
	d.ServerIP = make([]byte, 4)
	if err = binary.Read(buf, binary.BigEndian, &d.ServerIP); err != nil {
		return
	}
	n += 4
	d.GatewayIP = make([]byte, 4)
	if err = binary.Read(buf, binary.BigEndian, &d.GatewayIP); err != nil {
		return
	}
	n += 4
	clientHWAddr := make([]byte, 16)
	if err = binary.Read(buf, binary.BigEndian, &clientHWAddr); err != nil {
		return
	}
	d.ClientHWAddr = net.HardwareAddr(clientHWAddr[:d.HardwareLen])
	n += 16

	if err = binary.Read(buf, binary.BigEndian, &d.ServerName); err != nil {
		return
	}
	n += 64
	if err = binary.Read(buf, binary.BigEndian, &d.File); err != nil {
		return
	}
	n += 128

	var magic uint32
	if err = binary.Read(buf, binary.BigEndian, &magic); err != nil {
		return
	}
	n += 4

	if magic != dhcpMagic {
		return n, errors.New("Bad DHCP header")
	}

	optlen := buf.Len()
	opts := make([]byte, optlen)
	if err = binary.Read(buf, binary.BigEndian, &opts); err != nil {
		return
	}
	n += optlen

	if d.Options, err = DHCPParseOptions(opts); err != nil {
		return
	}

	return
}

// Standard options (RFC1533)
const (
	DHCP_OPT_PAD                      byte = iota
	DHCP_OPT_SUBNET_MASK                   // 0x01, 4, net.IP
	DHCP_OPT_TIME_OFFSET                   // 0x02, 4, int32 (signed seconds from UTC)
	DHCP_OPT_DEFAULT_GATEWAY               // 0x03, n*4, [n]net.IP
	DHCP_OPT_TIME_SERVER                   // 0x04, n*4, [n]net.IP
	DHCP_OPT_NAME_SERVER                   // 0x05, n*4, [n]net.IP
	DHCP_OPT_DOMAIN_NAME_SERVERS           // 0x06, n*4, [n]net.IP
	DHCP_OPT_LOG_SERVER                    // 0x07, n*4, [n]net.IP
	DHCP_OPT_COOKIE_SERVER                 // 0x08, n*4, [n]net.IP
	DHCP_OPT_LPR_SERVER                    // 0x09, n*4, [n]net.IP
	DHCP_OPT_IMPRESS_SERVER                // 0x0a, n*4, [n]net.IP
	DHCP_OPT_RLSERVER                      // 0x0b, n*4, [n]net.IP
	DHCP_OPT_HOST_NAME                     // 0x0c, n, string
	DHCP_OPT_BOOTFILE_SIZE                 // 0x0d, 2, uint16
	DHCP_OPT_MERIT_DUMP_FILE               // 0x0e, >1, string
	DHCP_OPT_DOMAIN_NAME                   // 0x0f, n, string
	DHCP_OPT_SWAP_SERVER                   // 0x10, n*4, [n]net.IP
	DHCP_OPT_ROOT_PATH                     // 0x11, n, string
	DHCP_OPT_EXTENSIONS_PATH               // 0x12, n, string
	DHCP_OPT_IP_FORWARDING                 // 0x13, 1, bool
	DHCP_OPT_SOURCE_ROUTING                // 0x14, 1, bool
	DHCP_OPT_POLICY_FILTER                 // 0x15, 8*n, [n]{net.IP/net.IP}
	DHCP_OPT_DGRAM_MTU                     // 0x16, 2, uint16
	DHCP_OPT_DEFAULT_TTL                   // 0x17, 1, byte
	DHCP_OPT_PATH_MTU_AGING_TIMEOUT        // 0x18, 4, uint32
	DHCP_OPT_PATH_PLATEU_TABLE_OPTION      // 0x19, 2*n, []uint16
	DHCP_OPT_INTERFACE_MTU                 //0x1a, 2, uint16
	DHCP_OPT_ALL_SUBS_LOCAL                // 0x1b, 1, bool
	DHCP_OPT_BROADCAST_ADDR                // 0x1c, 4, net.IP
	DHCP_OPT_MASK_DISCOVERY                // 0x1d, 1, bool
	DHCP_OPT_MASK_SUPPLIER                 // 0x1e, 1, bool
	DHCP_OPT_ROUTER_DISCOVERY              // 0x1f, 1, bool
	DHCP_OPT_ROUTER_SOLICIT_ADDR           // 0x20, 4, net.IP
	DHCP_OPT_STATIC_ROUTE                  // 0x21, n*8, [n]{net.IP/net.IP} -- note the 2nd is router not mask
	DHCP_OPT_ARP_TRAILERS                  // 0x22, 1, bool
	DHCP_OPT_ARP_TIMEOUT                   // 0x23, 4, uint32
	DHCP_OPT_ETHERNET_ENCAP                // 0x24, 1, bool
	DHCP_OPT_TCP_TTL                       // 0x25,1, byte
	DHCP_OPT_TCP_KEEPALIVE_INT             // 0x26,4, uint32
	DHCP_OPT_TCP_KEEPALIVE_GARBAGE         // 0x27,1, bool
	DHCP_OPT_NIS_DOMAIN                    // 0x28,n, string
	DHCP_OPT_NIS_SERVERS                   // 0x29,4*n,  [n]net.IP
	DHCP_OPT_NTP_SERVERS                   // 0x2a, 4*n, [n]net.IP
	DHCP_OPT_VENDOR_OPT                    // 0x2b, n, [n]byte // may be encapsulated.
	DHCP_OPT_NETBIOS_IPNS                  // 0x2c, 4*n, [n]net.IP
	DHCP_OPT_NETBIOS_DDS                   // 0x2d, 4*n, [n]net.IP
	DHCP_OPT_NETBIOS_NODE_TYPE             // 0x2e, 1, magic byte
	DHCP_OPT_NETBIOS_SCOPE                 // 0x2f, n, string
	DHCP_OPT_X_FONT_SERVER                 // 0x30, n, string
	DHCP_OPT_X_DISPLAY_MANAGER             // 0x31, n, string

	DHCP_OPT_SIP_SERVERS byte = 0x78 // 0x78!, n, url
	DHCP_OPT_END         byte = 0xff
)

// I'm amazed that this is syntactically valid.
// cool though.
var DHCPOptionTypeStrings = [256]string{
	DHCP_OPT_PAD:                      "(padding)",
	DHCP_OPT_SUBNET_MASK:              "SubnetMask",
	DHCP_OPT_TIME_OFFSET:              "TimeOffset",
	DHCP_OPT_DEFAULT_GATEWAY:          "DefaultGateway",
	DHCP_OPT_TIME_SERVER:              "rfc868", // old time server protocol, stringified to dissuade confusion w. NTP
	DHCP_OPT_NAME_SERVER:              "ien116", // obscure nameserver protocol, stringified to dissuade confusion w. DNS
	DHCP_OPT_DOMAIN_NAME_SERVERS:      "DNS",
	DHCP_OPT_LOG_SERVER:               "mitLCS", // MIT LCS server protocol, yada yada w. Syslog
	DHCP_OPT_COOKIE_SERVER:            "OPT_COOKIE_SERVER",
	DHCP_OPT_LPR_SERVER:               "OPT_LPR_SERVER",
	DHCP_OPT_IMPRESS_SERVER:           "OPT_IMPRESS_SERVER",
	DHCP_OPT_RLSERVER:                 "OPT_RLSERVER",
	DHCP_OPT_HOST_NAME:                "Hostname",
	DHCP_OPT_BOOTFILE_SIZE:            "BootfileSize",
	DHCP_OPT_MERIT_DUMP_FILE:          "OPT_MERIT_DUMP_FILE",
	DHCP_OPT_DOMAIN_NAME:              "DomainName",
	DHCP_OPT_SWAP_SERVER:              "OPT_SWAP_SERVER",
	DHCP_OPT_ROOT_PATH:                "RootPath",
	DHCP_OPT_EXTENSIONS_PATH:          "OPT_EXTENSIONS_PATH",
	DHCP_OPT_IP_FORWARDING:            "OPT_IP_FORWARDING",
	DHCP_OPT_SOURCE_ROUTING:           "OPT_SOURCE_ROUTING",
	DHCP_OPT_POLICY_FILTER:            "OPT_POLICY_FILTER",
	DHCP_OPT_DGRAM_MTU:                "OPT_DGRAM_MTU",
	DHCP_OPT_DEFAULT_TTL:              "OPT_DEFAULT_TTL",
	DHCP_OPT_PATH_MTU_AGING_TIMEOUT:   "OPT_PATH_MTU_AGING_TIMEOUT",
	DHCP_OPT_PATH_PLATEU_TABLE_OPTION: "OPT_PATH_PLATEU_TABLE_OPTION",
	DHCP_OPT_INTERFACE_MTU:            "OPT_INTERFACE_MTU",
	DHCP_OPT_ALL_SUBS_LOCAL:           "OPT_ALL_SUBS_LOCAL",
	DHCP_OPT_BROADCAST_ADDR:           "OPT_BROADCAST_ADDR",
	DHCP_OPT_MASK_DISCOVERY:           "OPT_MASK_DISCOVERY",
	DHCP_OPT_MASK_SUPPLIER:            "OPT_MASK_SUPPLIER",
	DHCP_OPT_ROUTER_DISCOVERY:         "OPT_ROUTER_DISCOVERY",
	DHCP_OPT_ROUTER_SOLICIT_ADDR:      "OPT_ROUTER_SOLICIT_ADDR",
	DHCP_OPT_STATIC_ROUTE:             "OPT_STATIC_ROUTE",
	DHCP_OPT_ARP_TRAILERS:             "OPT_ARP_TRAILERS",
	DHCP_OPT_ARP_TIMEOUT:              "OPT_ARP_TIMEOUT",
	DHCP_OPT_ETHERNET_ENCAP:           "OPT_ETHERNET_ENCAP",
	DHCP_OPT_TCP_TTL:                  "OPT_TCP_TTL",
	DHCP_OPT_TCP_KEEPALIVE_INT:        "OPT_TCP_KEEPALIVE_INT",
	DHCP_OPT_TCP_KEEPALIVE_GARBAGE:    "OPT_TCP_KEEPALIVE_GARBAGE",
	DHCP_OPT_NIS_DOMAIN:               "OPT_NIS_DOMAIN",
	DHCP_OPT_NIS_SERVERS:              "OPT_NIS_SERVERS",
	DHCP_OPT_NTP_SERVERS:              "OPT_NTP_SERVERS",
	DHCP_OPT_VENDOR_OPT:               "OPT_VENDOR_OPT",
	DHCP_OPT_NETBIOS_IPNS:             "OPT_NETBIOS_IPNS",
	DHCP_OPT_NETBIOS_DDS:              "OPT_NETBIOS_DDS",
	DHCP_OPT_NETBIOS_NODE_TYPE:        "OPT_NETBIOS_NODE_TYPE",
	DHCP_OPT_NETBIOS_SCOPE:            "OPT_NETBIOS_SCOPE",
	DHCP_OPT_X_FONT_SERVER:            "OPT_X_FONT_SERVER",
	DHCP_OPT_X_DISPLAY_MANAGER:        "OPT_X_DISPLAY_MANAGER",
	DHCP_OPT_END:                      "(end)",
	DHCP_OPT_SIP_SERVERS:              "SipServers",
	DHCP_OPT_REQUEST_IP:               "RequestIP",
	DHCP_OPT_LEASE_TIME:               "LeaseTime",
	DHCP_OPT_EXT_OPTS:                 "ExtOpts",
	DHCP_OPT_MESSAGE_TYPE:             "MessageType",
	DHCP_OPT_SERVER_ID:                "ServerID",
	DHCP_OPT_PARAMS_REQUEST:           "ParamsRequest",
	DHCP_OPT_MESSAGE:                  "Message",
	DHCP_OPT_MAX_DHCP_SIZE:            "MaxDHCPSize",
	DHCP_OPT_T1:                       "Timer1",
	DHCP_OPT_T2:                       "Timer2",
	DHCP_OPT_CLASS_ID:                 "ClassID",
	DHCP_OPT_CLIENT_ID:                "ClientID",
}

type DHCPOption interface {
	OptionType() byte
	Bytes() []byte
	Len() uint16
}

// Write an option to an io.Writer, including tag  & length
// (if length is appropriate to the tag type).
// Utilizes the MarshalOption as the underlying serializer.
func DHCPWriteOption(w io.Writer, a DHCPOption) (n int, err error) {
	out, err := DHCPMarshalOption(a)
	if err == nil {
		n, err = w.Write(out)
	}
	return
}

type dhcpoption struct {
	tag  byte
	data []byte
}

// A more json.Marshal like version of WriteOption.
func DHCPMarshalOption(o DHCPOption) (out []byte, err error) {
	switch o.OptionType() {
	case DHCP_OPT_PAD, DHCP_OPT_END:
		out = []byte{o.OptionType()}
	default:
		dlen := len(o.Bytes())
		if dlen > 253 {
			err = errors.New("Data too long to marshal")
		} else {
			out = make([]byte, dlen+2)
			out[0], out[1] = o.OptionType(), byte(dlen)
			copy(out[2:], o.Bytes())
		}
	}
	return
}

func (self dhcpoption) Len() uint16      { return uint16(len(self.data) + 2) }
func (self dhcpoption) Bytes() []byte    { return self.data }
func (self dhcpoption) OptionType() byte { return self.tag }

func DHCPNewOption(tag byte, data []byte) DHCPOption {
	return &dhcpoption{tag: tag, data: data}
}

// NB: We don't validate that you have /any/ IP's in the option here,
// simply that if you do that they're valid. Most DHCP options are only
// valid with 1(+|) values
func DHCPIP4sOption(tag byte, ips []net.IP) (opt DHCPOption, err error) {
	var out []byte = make([]byte, 4*len(ips))
	for i := range ips {
		ip := ips[i].To4()
		if ip == nil {
			err = errors.New("ip is not a valid IPv4 address")
		} else {
			copy(out[i*4:], []byte(ip))
		}
		if err != nil {
			break
		}
	}
	opt = DHCPNewOption(tag, out)
	return
}

// NB: We don't validate that you have /any/ IP's in the option here,
// simply that if you do that they're valid. Most DHCP options are only
// valid with 1(+|) values
func DHCPIP4Option(tag byte, ips net.IP) (opt DHCPOption, err error) {
	ips = ips.To4()
	if ips == nil {
		err = errors.New("ip is not a valid IPv4 address")
		return
	}
	opt = DHCPNewOption(tag, []byte(ips))
	return
}

// NB: I'm not checking tag : min length here!
func DHCPStringOption(tag byte, s string) (opt DHCPOption, err error) {
	opt = &dhcpoption{tag: tag, data: bytes.NewBufferString(s).Bytes()}
	return
}

func DHCPParseOptions(in []byte) (opts []DHCPOption, err error) {
	pos := 0
	for pos < len(in) && err == nil {
		var tag = in[pos]
		pos++
		switch tag {
		case DHCP_OPT_PAD:
			opts = append(opts, DHCPNewOption(tag, []byte{}))
		case DHCP_OPT_END:
			return
		default:
			if len(in)-pos >= 1 {
				_len := in[pos]
				pos++
				opts = append(opts, DHCPNewOption(tag, in[pos:pos+int(_len)]))
				pos += int(_len)
			}
		}
	}
	return
}

func NewDHCPDiscover(xid uint32, hwAddr net.HardwareAddr) (d *DHCP, err error) {
	if d, err = NewDHCP(xid, DHCP_MSG_DISCOVER, DHCP_HW_ETHERNET); err != nil {
		return
	}
	d.HardwareLen = uint8(len(hwAddr))
	d.ClientHWAddr = hwAddr
	d.Options = append(d.Options, DHCPNewOption(53, []byte{byte(DHCP_MSG_DISCOVER)}))
	d.Options = append(d.Options, DHCPNewOption(DHCP_OPT_CLIENT_ID, hwAddr))
	return
}

func NewDHCPOffer(xid uint32, hwAddr net.HardwareAddr) (d *DHCP, err error) {
	if d, err = NewDHCP(xid, DHCP_MSG_OFFER, DHCP_HW_ETHERNET); err != nil {
		return
	}
	d.HardwareLen = uint8(len(hwAddr))
	d.ClientHWAddr = hwAddr
	d.Options = append(d.Options, DHCPNewOption(53, []byte{byte(DHCP_MSG_OFFER)}))
	return
}

func NewDHCPRequest(xid uint32, hwAddr net.HardwareAddr) (d *DHCP, err error) {
	if d, err = NewDHCP(xid, DHCP_MSG_REQUEST, DHCP_HW_ETHERNET); err != nil {
		return
	}
	d.HardwareLen = uint8(len(hwAddr))
	d.ClientHWAddr = hwAddr
	d.Options = append(d.Options, DHCPNewOption(53, []byte{byte(DHCP_MSG_REQUEST)}))
	return
}

func NewDHCPAck(xid uint32, hwAddr net.HardwareAddr) (d *DHCP, err error) {
	if d, err = NewDHCP(xid, DHCP_MSG_ACK, DHCP_HW_ETHERNET); err != nil {
		return
	}
	d.HardwareLen = uint8(len(hwAddr))
	d.ClientHWAddr = hwAddr
	d.Options = append(d.Options, DHCPNewOption(53, []byte{byte(DHCP_MSG_ACK)}))
	return
}

func NewDHCPNak(xid uint32, hwAddr net.HardwareAddr) (d *DHCP, err error) {
	if d, err = NewDHCP(xid, DHCP_MSG_NAK, DHCP_HW_ETHERNET); err != nil {
		return
	}
	d.HardwareLen = uint8(len(hwAddr))
	d.ClientHWAddr = hwAddr
	d.Options = append(d.Options, DHCPNewOption(53, []byte{byte(DHCP_MSG_NAK)}))
	return
}
