package netlib

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"runtime"
	"strings"
	"syscall"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

var globalNs int

func init() {
	globalNs = os.Getpid()
}

// NICConfig stores the configuration for one NIC
type NICConfig struct {
	IPAddress string
	Name      string
	NewName   string
	GatewayIP string
	MoveToNS  bool
	link      netlink.Link
}

// MoveAndConfigureNICs moves a bunch of NICs to a network namespace,
// enters the namespace, configures the NICs and returns to the
// original network namespace
func MoveAndConfigureNICs(pid int, nics []NICConfig) error {
	runtime.LockOSThread()
	var err error
	for _, nic := range nics {
		if !nic.MoveToNS {
			continue
		}
		nic.link, err = netlink.LinkByName(nic.Name)
		if err != nil {
			return fmt.Errorf("failed to get %v by name: %v", nic.Name, err)
		}
		if err = moveInterfaceToPIDNetNS(nic.link, pid); err != nil {
			return fmt.Errorf("failed to move interface %v to network namespace: %v", nic.Name, err)
		}
	}

	err = enterPIDNetNS(pid)
	if err != nil {
		return fmt.Errorf("failed to enter network namespace of pid %v: %v", pid, err)
	}

	defer func() {
		enterPIDNetNS(globalNs)
		runtime.UnlockOSThread()
	}()

	for k := range nics {
		nics[k].link, err = netlink.LinkByName(nics[k].Name)
		if err != nil {
			return fmt.Errorf("failed to get %v by name: %v", nics[k].Name, err)
		}
	}
	for _, nic := range nics {
		if nic.NewName == "" {
			continue
		}
		if err = configureInterface(nic.Name, nic.IPAddress, nic.NewName); err != nil {
			return fmt.Errorf("failed to configure interface %v: %v", nic.Name, err)
		}
	}

	for _, nic := range nics {
		if err = netlink.LinkSetUp(nic.link); err != nil {
			return fmt.Errorf("failed to set interface %v up: %v", nic.Name, err)
		}
		if nic.GatewayIP == "" {
			continue
		}

		gwIP := net.ParseIP(nic.GatewayIP)
		if gwIP == nil {
			return fmt.Errorf("failed to parse default gateway IP %v", gwIP)
		}
		if err = setDefaultGateway(nic.link, gwIP); err != nil {
			return fmt.Errorf("failed to set default gateway %v for interface %v: %v", nic.GatewayIP, nic.Name, err)
		}

	}
	return nil
}

func setLinkAddressByName(netIf, address string) error {
	netif, err := netlink.LinkByName(netIf)
	if err != nil {
		return fmt.Errorf("encountered error while trying to find interface %v: %v", netif, err)
	}

	return setLinkAddress(netif, address)
}

func setLinkAddress(netif netlink.Link, address string) error {
	addr, err := netlink.ParseAddr(address)
	if err != nil {
		return fmt.Errorf("failed to parse address %s: %v", address, err)
	}

	return netlink.AddrAdd(netif, addr)
}

func setInterfaceUp(netIf string) error {
	netif, err := netlink.LinkByName(netIf)
	if err != nil {
		return fmt.Errorf("encountered error while trying to find interface %v: %v", netif, err)
	}

	return netlink.LinkSetUp(netif)
}

func setDefaultGatewayByLinkName(netIf, address string) error {
	netif, err := netlink.LinkByName(netIf)
	if err != nil {
		return fmt.Errorf("encountered error while trying to find interface %v: %v", netif, err)
	}

	addr := net.ParseIP(address)
	if addr == nil {
		return fmt.Errorf("failed to parse address %v", address)
	}

	return setDefaultGateway(netif, addr)
}

func setDefaultGateway(netif netlink.Link, address net.IP) error {
	r := netlink.Route{LinkIndex: netif.Attrs().Index,
		Gw: address,
	}
	return netlink.RouteAdd(&r)
}

// AddBridge creates a bridge interface, brings it up
// and sets up the address
func AddBridge(address, name string) error {
	_, err := net.InterfaceByName(name)
	if err == nil {
		return nil
	}

	if !strings.Contains(err.Error(), "no such network interface") {
		return err
	}

	la := netlink.NewLinkAttrs()
	la.Name = name
	br := &netlink.Bridge{la}
	if err := netlink.LinkAdd(br); err != nil {
		return fmt.Errorf("failed to add bridge: %v", err)
	}

	if err := netlink.LinkSetUp(br); err != nil {
		return err
	}

	return setLinkAddressByName(name, address)
}

// GenerateVethNames generates veth names
func GenerateVethNames() (string, string) {
	if1 := fmt.Sprintf("cntbrmesos%d", rand.Intn(100000))
	if2 := fmt.Sprintf("cntbrmesos%d", rand.Intn(100000))

	return if1, if2
}

func pidToProcPath(pid int) (string, error) {
	procPath := fmt.Sprintf("/proc/%v/ns/net", pid)
	_, err := os.Stat(procPath)
	if err != nil {
		return "", fmt.Errorf("couldn't find process in proc: %v", err)
	}

	return procPath, nil
}

// CreateVethPair creates a veth pair on a given bridge
func CreateVethPair(pid int, bridge, parent, peer string) error {
	// get bridge to set as master for one side of veth-pair
	br, err := netlink.LinkByName(bridge)
	if err != nil {
		return err
	}

	la := netlink.LinkAttrs{
		MasterIndex: br.Attrs().Index,
		Name:        parent,
		TxQLen:      0,
	}
	veth := &netlink.Veth{LinkAttrs: la, PeerName: peer}
	if err := netlink.LinkAdd(veth); err != nil {
		return fmt.Errorf("veth creation (%s, %s) failed: %v", parent, peer, err)
	}

	peerIf, err := netlink.LinkByName(peer)
	if err != nil {
		return fmt.Errorf("failed LinkByName for peer: %v", err)
	}

	if err := netlink.LinkSetNsPid(peerIf, pid); err != nil {
		return fmt.Errorf("failed to move peer to ns of PID %d: %v", pid, err)
	}

	return netlink.LinkSetUp(veth)
}

func moveInterfaceByNameToPIDNetNS(netIf string, pid int) error {
	intIf, err := netlink.LinkByName(netIf)
	if err != nil {
		return fmt.Errorf("failed to get the interface: %v", err)
	}

	return moveInterfaceToPIDNetNS(intIf, pid)
}

func moveInterfaceToPIDNetNS(netIf netlink.Link, pid int) error {
	if err := netlink.LinkSetNsPid(netIf, pid); err != nil {
		return fmt.Errorf("failed to move interface %v to NetNS of PID %d: %v", netIf, pid, err)
	}

	return nil
}

func renameInterface(netIf, name string) error {
	intIf, err := netlink.LinkByName(netIf)
	if err != nil {
		return fmt.Errorf("failed to get the interface: %v", err)
	}

	if err := netlink.LinkSetName(intIf, name); err != nil {
		return fmt.Errorf("failed to rename interface %v to %v: %v", netIf, name, err)
	}

	return nil
}

func enterPIDNetNS(pid int) error {
	procPath, err := pidToProcPath(pid)
	if err != nil {
		return err
	}

	f, err := os.Open(procPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, _, err := unix.RawSyscall(unix.SYS_SETNS, f.Fd(), syscall.CLONE_NEWNET, 0); err != 0 {
		return err
	}

	return nil
}

func configureInterface(name, IP, newname string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("encountered error while trying to find interface %v: %v", name, err)
	}

	if err := netlink.LinkSetName(link, newname); err != nil {
		return fmt.Errorf("failed to rename interface %v to %v: %v", name, newname, err)
	}

	// set up IP address
	if err := setLinkAddress(link, IP); err != nil {
		return fmt.Errorf("failed to set the IP address %v on interface %v", IP, newname)
	}

	// bring up the interface
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("failed to bring interface up: %v", err)
	}

	return nil
}
