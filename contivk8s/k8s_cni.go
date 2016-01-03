/***
Copyright 2015 Cisco Systems Inc. All rights reserved.

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
	"fmt"
	"io/ioutil"
	"os"
	osexec "os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/contiv/netplugin/contivk8s/clients"
	"github.com/contiv/netplugin/netplugin/directapi"
	"github.com/vishvananda/netlink"

	log "github.com/Sirupsen/logrus"
)

const (
	contivCfgFile = "/opt/contiv/config/contiv.json"
)

// PodInfo holds information passed by CNI vis env vars
type PodInfo struct {
	Name             string `json:"K8S_POD_NAME,omitempty"`
	K8sNameSpace     string `json:"K8S_POD_NAMESPACE,omitempty"`
	InfraContainerID string `json:"K8S_POD_INFRA_CONTAINER_ID,omitempty"`
	NwNameSpace      string `json:"CNI_NETNS,omitempty"`
}

// ContivConfig holds information passed via config file during cluster set up
type ContivConfig struct {
	K8sAPIServer string `json:"K8S_API_SERVER,omitempty"`
	K8sCa        string `json:"K8S_CA,omitempty"`
	K8sKey       string `json:"K8S_KEY,omitempty"`
	K8sCert      string `json:"K8S_CERT,omitempty"`
}

func getConfig(cfgFile string, pCfg *ContivConfig) error {
	bytes, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		return err
	}

	err = json.Unmarshal(bytes, pCfg)
	if err != nil {
		return fmt.Errorf("Error parsing config file: %s", err)
	}

	return nil
}

func getPodInfo(ppInfo *PodInfo) error {
	cniArgs := os.Getenv("CNI_ARGS")
	if cniArgs == "" {
		return fmt.Errorf("Error reading CNI_ARGS")
	}

	// convert the cniArgs to json format
	cniArgs = "{\"" + cniArgs + "\"}"
	cniTmp1 := strings.Replace(cniArgs, "=", "\":\"", -1)
	cniJSON := strings.Replace(cniTmp1, ";", "\",\"", -1)
	err := json.Unmarshal([]byte(cniJSON), ppInfo)
	if err != nil {
		return fmt.Errorf("Error parsing cni args: %s", err)
	}

	// nwNameSpace is passed as a separate env var
	ppInfo.NwNameSpace = os.Getenv("CNI_NETNS")
	return nil
}

func nsToPID(ns string) (int, error) {
	// Make sure ns is well formed
	ok := strings.HasPrefix(ns, "/proc/")
	if !ok {
		return -1, fmt.Errorf("Invalid nw name space: %v", ns)
	}

	elements := strings.Split(ns, "/")
	return strconv.Atoi(elements[2])
}

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

func setIfAttrs(ifname, netns, cidr, newname string) error {

	// convert netns to pid that netlink needs
	pid, err := nsToPID(netns)
	if err != nil {
		return err
	}

	nsenterPath, err := osexec.LookPath("nsenter")
	if err != nil {
		return err
	}
	ipPath, err := osexec.LookPath("ip")
	if err != nil {
		return err
	}

	// find the link
	link, err := getLink(ifname)
	if err != nil {
		log.Errorf("unable to find link %q. Error %q", ifname, err)
		return err
	}

	// move to the desired netns
	err = netlink.LinkSetNsPid(link, pid)
	if err != nil {
		log.Errorf("unable to move interface %s to pid %d. Error: %s",
			ifname, pid, err)
		return err
	}

	// rename to the desired ifname
	nsPid := fmt.Sprintf("%d", pid)
	rename, err := osexec.Command(nsenterPath, "-t", nsPid, "-n", "-F", "--", ipPath, "link", "set", "dev", ifname, "name", newname).CombinedOutput()
	if err != nil {
		log.Errorf("unable to rename interface %s to %s. Error: %s",
			ifname, newname, err)
		return nil
	}
	log.Infof("Output from rename: %v", rename)

	// set the ip address
	assignIP, err := osexec.Command(nsenterPath, "-t", nsPid, "-n", "-F", "--", ipPath, "address", "add", cidr, "dev", newname).CombinedOutput()

	if err != nil {
		log.Errorf("unable to assign ip %s to %s. Error: %s",
			cidr, newname, err)
		return nil
	}
	log.Infof("Output from ip assign: %v", assignIP)

	// Finally, mark the link up
	bringUp, err := osexec.Command(nsenterPath, "-t", nsPid, "-n", "-F", "--", ipPath, "link", "set", "dev", newname, "up").CombinedOutput()

	if err != nil {
		log.Errorf("unable to assign ip %s to %s. Error: %s",
			cidr, newname, err)
		return nil
	}
	log.Infof("Output from ip assign: %v", bringUp)
	return nil

}

func main() {

	cCfg := ContivConfig{}
	pInfo := PodInfo{}

	// Open a logfile
	f, err := os.OpenFile("/var/log/contivk8s.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
		os.Exit(-1)
	}
	defer f.Close()

	log.SetOutput(f)
	log.Infof("==> Start New Log <==\n")
	log.Infof("command: %s, cni_args: %s", os.Getenv("CNI_COMMAND"), os.Getenv("CNI_ARGS"))
	log.Infof("netns: %s\n", os.Getenv("CNI_NETNS"))

	// Read config
	err = getConfig(contivCfgFile, &cCfg)
	if err != nil {
		log.Fatalf("Error parsing config. Err: %v", err)
		os.Exit(-1)
	}

	// Collect information passed by CNI
	err = getPodInfo(&pInfo)
	if err != nil {
		log.Fatalf("Error parsing config. Err: %v", err)
		os.Exit(-1)
	}

	// Get labels from the k8s api server
	ac := clients.NewAPIClient(cCfg.K8sAPIServer, cCfg.K8sCa,
		cCfg.K8sKey, cCfg.K8sCert)

	if ac == nil {
		os.Exit(-1)
	}

	epg, err1 := ac.GetPodLabel(pInfo.K8sNameSpace, pInfo.Name, "net-group")
	if err1 != nil {
		log.Fatalf("Error getting epg. Err: %v", err1)
		os.Exit(-1)
	}

	if epg == "" {
		log.Infof("net-group not found in podSpec for %v", pInfo.Name)
		epg = "default-epg"
	}

	log.Infof("EPG is %s for pod %s\n", epg, pInfo.Name)

	// Create the network endpoint
	nc := clients.NewNWClient()
	epSpec := directapi.ReqCreateEP{
		Tenant:     "default",
		Network:    "k8s-poc",
		Group:      epg,
		EndpointID: pInfo.InfraContainerID,
	}
	result, err := nc.AddEndpoint(epSpec)
	if err != nil {
		log.Fatalf("EP create failed -- %s", err)
		os.Exit(-1)
	} else {
		log.Infof("EP created Intf: %s IP: %s\n", result.IntfName, result.IPAddress)
	}

	// Move the network endpoint to the specified network name space
	log.Infof("Nw Ns: %s \n", pInfo.NwNameSpace)

	err = setIfAttrs(result.IntfName, pInfo.NwNameSpace, result.IPAddress, "eth0")
	if err != nil {
		log.Fatalf("failed -- %v", err)
		os.Exit(-1)
	} else {
		log.Infof("SUCCESS!\n")
	}

	// Finally, write the ip address of the created endpoint to stdout
	fmt.Printf("{\n\"cniVersion\": \"0.1.0\",\n")
	fmt.Printf("\"ip4\": {\n")
	fmt.Printf("\"ip\": \"%s\"\n}\n}\n", result.IPAddress)
}
