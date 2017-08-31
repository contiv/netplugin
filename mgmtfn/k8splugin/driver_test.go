/***
Copyright 2016 Cisco Systems Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package k8splugin

import (
	"fmt"
	"net"
	"os/exec"
	"runtime"

	. "github.com/contiv/check"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

type NetSetup struct {
	newNS    netns.NsHandle
	globalNS netns.NsHandle
	pid      int
	cmd      *exec.Cmd
	ifName   string
	link     netlink.Link
}

var _ = Suite(&NetSetup{})

func (s *NetSetup) SetUpTest(c *C) {
	la := netlink.NewLinkAttrs()
	s.ifName = "testlinkfoo"
	la.Name = s.ifName
	runtime.LockOSThread()
	locked := true
	defer func() {
		if locked {
			runtime.UnlockOSThread()
		}
	}()

	globalNS, err := netns.Get()
	if err != nil {
		c.Fatalf("failed to get the global network namespace: %v", err)
	}
	s.globalNS = globalNS
	defer func() {
		netns.Set(globalNS)
	}()

	newNS, err := netns.New()
	if err != nil {
		c.Fatal("failed to create new network namespace")
	}
	s.newNS = newNS

	cmd := exec.Command("sleep", "infinity")
	if err = cmd.Start(); err != nil {
		c.Fatalf("failed to start the 'sleep 9999' process: %v", err)
	}
	s.cmd = cmd

	s.pid = cmd.Process.Pid

	if err = netns.Set(globalNS); err != nil {
		c.Fatalf("failed to return to the global netns: %v", err)
	}

	// the rest of the code should run without a locked thread
	if locked {
		runtime.UnlockOSThread()
		locked = false
	}

	dummy := &netlink.Dummy{LinkAttrs: la}
	if err := netlink.LinkAdd(dummy); err != nil {
		c.Fatalf("failed to add dummy interface: %v", err)
	}

	link, err := netlink.LinkByName(la.Name)
	if err != nil {
		c.Fatalf("failed to get interface by name: %v", err)
	}
	s.link = link

	netIf, err := net.InterfaceByName(la.Name)
	if err != nil {
		c.Fatalf("InterfaceByName failed: %v", err)
	}

	if netIf.Flags&net.FlagUp != 0 {
		c.Fatalf("expected interface to be down, but it's up")
	}
}

func (s *NetSetup) TearDownTest(c *C) {
	s.newNS.Close()
	s.cmd.Process.Kill()
	netns.Set(s.globalNS)
	s.globalNS.Close()
	netlink.LinkDel(s.link)
}

func (s *NetSetup) TestNetSetup(c *C) {
	newName := "testlinknewname"
	address := "192.168.68.68/24"
	defGW := "192.168.68.1"
	staticRoute := "192.168.32.0/24"
	ipv6Address := "2001::100/100"
	ipv6Gateway := "2001::1/100"

	if err := setIfAttrs(s.pid, s.ifName, address, ipv6Address, newName); err != nil {
		c.Fatalf("setIfAttrs failed: %v", err)
	}

	if err := setDefGw(s.pid, defGW, ipv6Gateway, newName); err != nil {
		c.Fatalf("setDefGw failed: %v", err)
	}

	if err := addStaticRoute(s.pid, staticRoute, newName); err != nil {
		c.Fatalf("addStaticRoute failed: %v", err)
	}

	// check if the interface still has its old name & is in globalNS
	if _, err := netlink.LinkByName(s.ifName); err == nil {
		c.Fatal("interface wasn't moved to the namespace")
	}

	// check if the interface has been renamed & is in globalNS
	if _, err := netlink.LinkByName(newName); err == nil {
		c.Fatal("interface wasn't moved to the namespace")
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := netns.Set(s.newNS); err != nil {
		c.Fatalf("failed to enter the new network namespace: %v", err)
	}

	// ensure the interface's name has been changed
	newLink, err := netlink.LinkByName(newName)
	if err != nil {
		c.Fatalf("failed to get interface by name: %v", err)
	}
	defer netlink.LinkDel(newLink)

	// ensure that the interface's IP address has been set properly
	addresses, err := netlink.AddrList(newLink, netlink.FAMILY_V4)
	ifAddr := addresses[0].IPNet.String()
	if address != ifAddr {
		c.Errorf("expected IP address %v, found: %v", address, ifAddr)
	}

	netIf, err := net.InterfaceByName(newName)
	if err != nil {
		c.Errorf("InterfaceByName failed: %v", err)
	}

	// ensure the default gateway route has been set properly
	routes, err := netlink.RouteList(newLink, netlink.FAMILY_V4)
	if err != nil {
		c.Errorf("failed to fetch routes")
	}

	foundDefaultGW := false
	foundStaticRoute := false
	for _, route := range routes {
		gw := route.Gw.String()
		if gw != defGW {
			continue
		}
		if route.Dst != nil {
			c.Errorf("expected nil GW Dst, found: %v", route.Dst)
		}
		if route.Src != nil {
			c.Errorf("expected nil GW Src, found: %v", route.Dst)
		}
		foundDefaultGW = true
	}
	if !foundDefaultGW {
		c.Error("couldn't find default gateway")
	}

	for _, route := range routes {
		dst := fmt.Sprintf("%v", route.Dst)
		if dst != staticRoute {
			continue
		}
		if route.Gw != nil {
			c.Errorf("expected nil gateway, found: %v", route.Gw)
		}
		if route.Src != nil {
			c.Errorf("expected nil source, found: %v", route.Dst)
		}
		foundStaticRoute = true
	}

	if !foundStaticRoute {
		c.Error("couldn't find static route")
	}

	// ensure that the interface is up
	if netIf.Flags&net.FlagUp == 0 {
		c.Errorf("expected interface to be up, but it's down")
	}
}
