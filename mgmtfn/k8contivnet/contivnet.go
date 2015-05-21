/***
Copyright 2014 Cisco Systems Inc. All rights reserved.

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

// this package is a simple binary to invoke netdcli to ensure
// it can be self sufficient to run as network plugin for Kubernetes
// networking replacement

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster"

	log "github.com/Sirupsen/logrus"
)

const (
	LOG_FILE         = "/tmp/xx"
	TMP_NETDCLI_FILE = "/tmp/netdcli.cfg"
	NETDCLI_BIN      = "/root/go/bin/netdcli"
)

func getHostLabel() (string, error) {
	cmdStr := "ps ef -C netplugin "
	output, err := exec.Command("/bin/bash", "-c", cmdStr).Output()
	if err != nil {
		return "", err
	}

	fields := strings.Split(strings.TrimSpace(string(output)), " ")
	for idx, field := range fields {
		if field == "-host-label" && len(fields) > idx {
			return fields[idx+1], nil
		}
	}

	return "", core.Errorf("couldn't find host label")
}

func printUsage(arg0 string) {
	usageString := "" +
		arg0 + " init" + "\n" +
		arg0 + " setup <pod-name> <pod-namespace> <infra-container-uuid>" +
		"\n" +
		arg0 + " teardown <pod-name> <pod-namespace> <infra-container-uuid>" +
		"\n" +
		arg0 + " help" + "\n"

	log.Fatalf("Usage: \n%s\n", usageString)
	os.Exit(1)
}

func initPlugin() error {
	log.Printf("initializing the driver \n")
	return nil
}

func setUpPod(podNameSpace, podName, attachUUID string) error {
	hostLabel, err := getHostLabel()
	if err != nil {
		log.Fatalf("error %s getting host label \n", err)
		os.Exit(1)
	}

	epCfg := []netmaster.ConfigEp{
		{Host: hostLabel, Container: podName, AttachUUID: attachUUID}}

	bytes, err := json.Marshal(epCfg)
	if err != nil {
		log.Printf("error '%s' marshaling endpoint information \n", err)
		return err
	}

	jsonStr := string(bytes)
	jsonStr = strings.Replace(jsonStr, "\"", "\\\"", -1)
	cmdStr := fmt.Sprintf("echo \"%s\" > %s", jsonStr, TMP_NETDCLI_FILE)
	output, err := exec.Command("/bin/bash", "-c", cmdStr).Output()
	if err != nil {
		log.Printf("error '%s' marshaling endpoint information output \n%s\n",
			err, output)
		return err
	}

	cmdStr = NETDCLI_BIN + " -host-bindings-cfg " + TMP_NETDCLI_FILE + " 2>&1"
	output, err = exec.Command("/bin/bash", "-c", cmdStr).Output()
	if err != nil {
		log.Printf("error '%s' executing host bindings, output \n%s\n",
			err, output)
		return err
	}

	return nil
}

func tearDownPod(podNameSpace, podName, attachUUID string) error {
	hostLabel, err := getHostLabel()
	if err != nil {
		log.Fatalf("error %s getting host label \n", err)
		os.Exit(1)
	}

	epCfg := []netmaster.ConfigEp{
		{Host: hostLabel, Container: podName}}

	bytes, err := json.Marshal(epCfg)
	if err != nil {
		log.Printf("error '%s' marshaling endpoint information \n", err)
		return err
	}

	jsonStr := string(bytes)
	jsonStr = strings.Replace(jsonStr, "\"", "\\\"", -1)
	cmdStr := fmt.Sprintf("echo \"%s\" > %s", jsonStr, TMP_NETDCLI_FILE)
	output, err := exec.Command("/bin/bash", "-c", cmdStr).Output()
	if err != nil {
		log.Printf("error '%s' marshaling endpoint information output \n%s\n",
			err, output)
		return err
	}

	cmdStr = NETDCLI_BIN + " -host-bindings-cfg " + TMP_NETDCLI_FILE + " 2>&1"
	output, err = exec.Command("/bin/bash", "-c", cmdStr).Output()
	if err != nil {
		log.Printf("error '%s' executing host bindings, output \n%s\n",
			err, output)
		return err
	}

	return nil
}

func main() {
	var err error

	of, err := os.OpenFile(LOG_FILE, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err == nil {
		log.SetOutput(of)
	}
	defer of.Close()

	if len(os.Args) < 2 {
		printUsage(os.Args[0])
	}

	log.Printf("%s\n===============\n", os.Args)

	switch os.Args[1] {
	case "init":
		err = initPlugin()

	case "help":
		printUsage(os.Args[0])

	case "setup":
		if len(os.Args) < 5 {
			printUsage(os.Args[0])
		}
		err = setUpPod(os.Args[2], os.Args[3], os.Args[4])

	case "teardown":
		if len(os.Args) < 5 {
			printUsage(os.Args[0])
		}
		err = tearDownPod(os.Args[2], os.Args[3], os.Args[4])
	}

	if err != nil {
		log.Printf("error '%s' executing %s \n", err, os.Args)
		os.Exit(2)
	}

	os.Exit(0)
}
