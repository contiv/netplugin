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

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/contiv/netplugin/mgmtfn/k8splugin/cniapi"
	"github.com/contiv/netplugin/mgmtfn/k8splugin/contivk8s/clients"
	"github.com/contiv/netplugin/version"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	logger "github.com/Sirupsen/logrus"
	ip "github.com/containernetworking/cni/pkg/types"
	cni "github.com/containernetworking/cni/pkg/types/current"
)

// CNIResponse response format expected from CNI plugins(version 0.6.0)
type CNIResponse struct {
	CNIVersion string `json:"cniVersion"`
	cni.Result
}

//CNIError : return format from CNI plugin
type CNIError struct {
	CNIVersion string `json:"cniVersion"`
	ip.Error
}

var log *logger.Entry

func getPodInfo(ppInfo *cniapi.CNIPodAttr) error {
	cniArgs := os.Getenv("CNI_ARGS")
	if cniArgs == "" {
		return fmt.Errorf("error reading CNI_ARGS")
	}

	// convert the cniArgs to json format
	cniArgs = "{\"" + cniArgs + "\"}"
	cniTmp1 := strings.Replace(cniArgs, "=", "\":\"", -1)
	cniJSON := strings.Replace(cniTmp1, ";", "\",\"", -1)
	err := json.Unmarshal([]byte(cniJSON), ppInfo)
	if err != nil {
		return fmt.Errorf("error parsing cni args: %s", err)
	}

	// nwNameSpace and ifname are passed as separate env vars
	ppInfo.NwNameSpace = os.Getenv("CNI_NETNS")
	ppInfo.IntfName = os.Getenv("CNI_IFNAME")
	return nil
}

// nsToPID is a utility that extracts the PID from the netns
func nsToPID(ns string) (int, error) {
	elements := strings.Split(ns, "/")
	return strconv.Atoi(elements[2])
}

func nsToFD(nspath string) (netns.NsHandle, error) {
	log.Infof(">> Get fd from ns: %v", nspath)

	ns, err := os.Readlink(nspath)
	if err != nil {
		log.Errorf("invalid netns path. Error: %s", err)
		return netns.None(), err
	}

	fd, err := netns.GetFromPath(ns)
	if err != nil {
		log.Errorf("fd not found. Error: %s", err)
		return netns.None(), err
	}

	return fd, nil
}

// getLink is a wrapper that fetches the netlink corresponding to the ifname
func getLink(ifname string) (netlink.Link, error) {
	// find the link
	link, err := netlink.LinkByName(ifname)
	if err != nil {
		if !strings.Contains(err.Error(), "Link not found") {
			log.Errorf("unable to find link %q. Error: %q", ifname, err)
			return link, err
		}
		// try once more as sometimes (somehow) link creation is taking
		// sometime, causing link not found error
		time.Sleep(1 * time.Second)
		link, err = netlink.LinkByName(ifname)
		if err != nil {
			log.Errorf("unable to find link %q. Error %q", ifname, err)
		}
		return link, err
	}
	return link, err
}

func moveToNS(ifname, nspath string) error {
	//find the link
	link, err := getLink(ifname)
	if err != nil {
		log.Errorf("unable to find link %q. Error %q", ifname, err)
		return err
	}
	log.Infof("> Netns Path %s", nspath)

	// convert netns to pid that netlink needs
	var pid int

	if ok := strings.HasPrefix(nspath, "/proc/"); ok {
		pid, err = nsToPID(nspath)
	} else {
		log.Info("Is not a process")
		pid = -1
	}

	log.Infof(">> Move to netns pid %d", pid)
	if pid != -1 {
		err = netlink.LinkSetNsPid(link, pid)
		if err != nil {
			log.Errorf("unable to move interface %s to pid %d. Error: %s",
				ifname, pid, err)
			return err
		}
		return nil
	}

	fd, err := nsToFD(nspath)
	if err != nil {
		return err
	}

	err = netlink.LinkSetNsFd(link, int(fd))
	if err != nil {
		log.Errorf("unable to move interface %s to fd %d. Error: %s",
			ifname, nspath, err)
		return err
	}

	return nil
}

// setIfAttrs sets the required attributes for the container interface
func setIfAttrs(nspath, ifname, cidr, cidr6, newname string) error {
	nsenterPath, err := exec.LookPath("nsenter")
	if err != nil {
		return err
	}
	ipPath, err := exec.LookPath("ip")
	if err != nil {
		return err
	}

	// rename to the desired ifname
	net := "--net=" + nspath
	rename, err := exec.Command(nsenterPath, net, "--", ipPath, "link",
		"set", "dev", ifname, "name", newname).CombinedOutput()
	if err != nil {
		log.Errorf("unable to rename interface %s to %s. Error: %s",
			ifname, newname, err)
		return nil
	}
	log.Infof("Output from rename: %v", rename)

	// set the ip address
	assignIP, err := exec.Command(nsenterPath, net, "--", ipPath,
		"address", "add", cidr, "dev", newname).CombinedOutput()

	if err != nil {
		log.Errorf("unable to assign ip %s to %s. Error: %s",
			cidr, newname, err)
		return nil
	}
	log.Infof("Output from ip assign: %v", assignIP)

	if cidr6 != "" {
		out, err := exec.Command(nsenterPath, net, "--", ipPath,
			"-6", "address", "add", cidr6, "dev", newname).CombinedOutput()
		if err != nil {
			log.Errorf("unable to assign IPv6 %s to %s. Error: %s",
				cidr6, newname, err)
			return nil
		}
		log.Infof("Output of IPv6 assign: %v", out)
	}

	// Finally, mark the link up
	bringUp, err := exec.Command(nsenterPath, net, "--", ipPath,
		"link", "set", "dev", newname, "up").CombinedOutput()

	if err != nil {
		log.Errorf("unable to assign ip %s to %s. Error: %s",
			cidr, newname, err)
		return nil
	}
	log.Debugf("Output from ip assign: %v", bringUp)
	return nil

}

// setDefGw sets the default gateway for the container namespace
func setDefGw(nspath, gw, gw6, intfName string) error {
	nsenterPath, err := exec.LookPath("nsenter")
	if err != nil {
		return err
	}
	routePath, err := exec.LookPath("route")
	if err != nil {
		return err
	}
	// set default gw
	net := "--net=" + nspath
	out, err := exec.Command(nsenterPath, net, "--", routePath, "add",
		"default", "gw", gw, intfName).CombinedOutput()
	if err != nil {
		log.Errorf("unable to set default gw %s. Error: %s - %s", gw, err, out)
		return nil
	}

	if gw6 != "" {
		out, err := exec.Command(nsenterPath, net, "--", routePath,
			"-6", "add", "default", "gw", gw6, intfName).CombinedOutput()
		if err != nil {
			log.Errorf("unable to set default IPv6 gateway %s. Error: %s - %s", gw6, err, out)
			return nil
		}
	}

	return nil
}
func addPodToContiv(nc *clients.NWClient, pInfo *cniapi.CNIPodAttr) error {

	// Add to contiv network
	result, err := nc.AddPod(pInfo)
	if err != nil || result.Result != 0 {
		log.Errorf("EP create failed for pod: %s/%s",
			pInfo.K8sNameSpace, pInfo.Name)
		cerr := CNIError{}
		cerr.CNIVersion = "0.3.1"

		if result != nil {
			cerr.Code = result.Result
			cerr.Msg = "Contiv:" + result.ErrMsg
			cerr.Details = result.ErrInfo
		} else {
			cerr.Code = 1
			cerr.Msg = "Contiv:" + err.Error()
		}

		eOut, err := json.Marshal(&cerr)
		if err == nil {
			log.Infof("cniErr: %s", eOut)
			fmt.Printf("%s", eOut)
		} else {
			log.Errorf("JSON error: %v", err)
		}
		os.Exit(1)
	}
	log.Infof("EP created IP: %s\n", result.Attr.IPAddress)

	// move to the desired netns
	err = moveToNS(result.Attr.PortName, pInfo.NwNameSpace)
	if err != nil {
		log.Errorf("Error moving to netns. Err: %v", err)
		return err
	}

	err = setIfAttrs(pInfo.NwNameSpace, result.Attr.PortName, result.Attr.IPAddress,
		result.Attr.IPv6Address, pInfo.IntfName)

	if err != nil {
		log.Errorf("Error setting interface attributes. Err: %v", err)
		return err
	}

	if result.Attr.Gateway != "" {
		// Set default gateway
		err = setDefGw(pInfo.NwNameSpace, result.Attr.Gateway, result.Attr.IPv6Gateway,
			pInfo.IntfName)
		if err != nil {
			log.Errorf("Error setting default gateway. Err: %v", err)
			return err
		}
	}

	// Write the ip address of the created endpoint to stdout

	// ParseCIDR returns a reference to IPNet
	ip4Net, err := ip.ParseCIDR(result.Attr.IPAddress)
	if err != nil {
		log.Errorf("Failed to parse IPv4 CIDR: %v", err)
		return err
	}

	out := CNIResponse{
		CNIVersion: "0.3.1",
	}

	out.IPs = append(out.IPs, &cni.IPConfig{
		Version: "4",
		Address: net.IPNet{IP: ip4Net.IP, Mask: ip4Net.Mask},
	})

	if result.Attr.IPv6Address != "" {
		ip6Net, err := ip.ParseCIDR(result.Attr.IPv6Address)
		if err != nil {
			log.Errorf("Failed to parse IPv6 CIDR: %v", err)
			return err
		}

		out.IPs = append(out.IPs, &cni.IPConfig{
			Version: "6",
			Address: net.IPNet{IP: ip6Net.IP, Mask: ip6Net.Mask},
		})
	}

	data, err := json.MarshalIndent(out, "", "    ")
	if err != nil {
		log.Errorf("Failed to marshal json: %v", err)
		return err
	}

	log.Infof("Response from CNI executable: \n%s", fmt.Sprintf("%s", data))
	fmt.Printf(fmt.Sprintf("%s", data))

	return nil
}

func deletePodFromContiv(nc *clients.NWClient, pInfo *cniapi.CNIPodAttr) {

	err := nc.DelPod(pInfo)
	if err != nil {
		log.Errorf("DelEndpoint returned %v", err)
	} else {
		log.Infof("EP deleted pod: %s\n", pInfo.Name)
	}
}

func getPrefixedLogger() *logger.Entry {
	var nsID string

	netNS := os.Getenv("CNI_NETNS")
	ok := strings.HasPrefix(netNS, "/proc/")
	if ok {
		elements := strings.Split(netNS, "/")
		nsID = elements[2]
	} else {
		nsID = "EMPTY"
	}

	l := logger.WithFields(logger.Fields{
		"NETNS": nsID,
	})

	return l
}

func main() {
	var showVersion bool

	// parse rest of the args that require creating state
	flagSet := flag.NewFlagSet("contivk8s", flag.ExitOnError)

	flagSet.BoolVar(&showVersion,
		"version",
		false,
		"Show version")

	if err := flagSet.Parse(os.Args[1:]); err != nil {
		logger.Fatalf("Failed to parse command. Error: %s", err)
	}
	if showVersion {
		fmt.Printf(version.String())
		os.Exit(0)
	}

	mainfunc()
}

func mainfunc() {
	pInfo := cniapi.CNIPodAttr{}
	cniCmd := os.Getenv("CNI_COMMAND")

	// Open a logfile
	f, err := os.OpenFile("/var/log/contivk8s.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		logger.Fatalf("error opening file: %v", err)
	}
	defer f.Close()

	logger.SetOutput(f)
	log = getPrefixedLogger()

	log.Infof("==> Start New Log <==\n")
	log.Infof("command: %s, cni_args: %s", cniCmd, os.Getenv("CNI_ARGS"))

	// Collect information passed by CNI
	err = getPodInfo(&pInfo)
	if err != nil {
		log.Fatalf("Error parsing environment. Err: %v", err)
	}

	nc := clients.NewNWClient()
	if cniCmd == "ADD" {
		err = addPodToContiv(nc, &pInfo)
		if err != nil {
			deletePodFromContiv(nc, &pInfo)
			log.Infof("Error on add pod. Err %v", err)
		}
	} else if cniCmd == "DEL" {
		deletePodFromContiv(nc, &pInfo)
	}

}
