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
	"os"
	"strings"

	"github.com/contiv/netplugin/mgmtfn/k8splugin/cniapi"
	"github.com/contiv/netplugin/mgmtfn/k8splugin/contivk8s/clients"
	"github.com/contiv/netplugin/version"

	logger "github.com/Sirupsen/logrus"
)

var log *logger.Entry

func getPodInfo(ppInfo *cniapi.CNIPodAttr) error {
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

	// nwNameSpace and ifname are passed as separate env vars
	ppInfo.NwNameSpace = os.Getenv("CNI_NETNS")
	ppInfo.IntfName = os.Getenv("CNI_IFNAME")
	return nil
}

func addPodToContiv(nc *clients.NWClient, pInfo *cniapi.CNIPodAttr) {

	// Add to contiv network
	result, err := nc.AddPod(pInfo)
	if err != nil {
		log.Fatalf("EP create failed -- %s", err)
	} else {
		log.Infof("EP created IP: %s\n", result.IPAddress)
	}

	// Write the ip address of the created endpoint to stdout
	fmt.Printf("{\n\"cniVersion\": \"0.1.0\",\n")
	fmt.Printf("\"ip4\": {\n")
	fmt.Printf("\"ip\": \"%s\"\n}\n}\n", result.IPAddress)
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
	flagSet := flag.NewFlagSet("netd", flag.ExitOnError)

	flagSet.BoolVar(&showVersion,
		"version",
		false,
		"Show version")

	if err := flagSet.Parse(os.Args[1:]); err != nil {
		logger.Fatalf("Failed to parse command. Error: %s", err)
	}
	if showVersion {
		fmt.Printf(version.Print(version.Get()))
		os.Exit(0)
	}

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
		addPodToContiv(nc, &pInfo)
	} else if cniCmd == "DEL" {
		deletePodFromContiv(nc, &pInfo)
	}

}
