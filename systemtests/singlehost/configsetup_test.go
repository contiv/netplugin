package singlehost

import (
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/contiv/netplugin/systemtests/utils"
)

var vagrant *utils.Vagrant

func TestMain(m *testing.M) {
	// setup a single node vagrant testbed
	vagrant = &utils.Vagrant{}
	log.Printf("Starting vagrant up...")
	err := vagrant.Setup(os.Getenv("CONTIV_ENV"), 1)
	log.Printf("Done with vagrant up...")
	if err != nil {
		log.Printf("Vagrant setup failed. Error: %s", err)
		vagrant.Teardown()
		os.Exit(1)
	}

	exitCode := m.Run()

	vagrant.Teardown()

	os.Exit(exitCode)
}

// Testcase:
// - Create a single vlan network with two endpoints
// - Verify that the endpoints are able to ping
func TestSingleHostSingleVlanPingSuccess(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, vagrant.GetNodes())
	}()

	node := vagrant.GetNodes()[0]
	//create a single vlan network, with two endpoints
	jsonCfg :=
		`{
        "Tenants" : [ {
            "Name"                      : "tenant-one",
            "DefaultNetType"            : "vlan",
            "SubnetPool"                : "11.1.0.0/16",
            "AllocSubnetLen"            : 24,
            "Vlans"                     : "11-48",
            "Networks"  : [ {
                "Name"                  : "orange",
                "Endpoints" : [
                {
                    "Container"         : "myContainer1",
                    "Host"              : "host1"
                },
                {
                    "Container"         : "myContainer2",
                    "Host"              : "host1"
                } ]
            } ]
        } ]
        }`

	utils.ConfigSetupCommon(t, jsonCfg, vagrant.GetNodes())

	//start container 1, running a simple wait loop
	cmdStr := "sudo docker run -d --name=myContainer1 ubuntu /bin/bash -c 'mkfifo foo && < foo'"
	output, err := node.RunCommandWithOutput(cmdStr)
	defer func() {
		cmdStr = "sudo docker kill myContainer1"
		node.RunCommand(cmdStr)
		cmdStr = "sudo docker rm myContainer1"
		node.RunCommand(cmdStr)
	}()
	if err != nil {
		t.Fatalf("Failed to launch the container. Error: %s, Output: \n%s\n",
			err, output)
	}

	//start container 2 with ping for container 1
	cmdStr = "netdcli -oper get -construct endpoint orange-myContainer1 2>&1 | grep IpAddress | awk -F : '{gsub(\"[,}{]\",\"\", $2); print $2}'"
	output, err = node.RunCommandWithOutput(cmdStr)

	if err != nil || string(output) == "" {
		t.Fatalf("Failed to get ip address of the container. Error: %s, Output: \n%s\n",
			err, output)
	}

	ipAddress := string(output)
	cmdStr = fmt.Sprintf("sudo docker run --name=myContainer2 ubuntu /bin/bash -c 'ping -c5 %s'",
		ipAddress)
	output, err = node.RunCommandWithOutput(cmdStr)
	defer func() {
		cmdStr = "sudo docker kill myContainer2"
		node.RunCommand(cmdStr)
		cmdStr = "sudo docker rm myContainer2"
		node.RunCommand(cmdStr)
	}()
	if err != nil {
		t.Fatalf("Failed to launch the container. Error: %s, Output: \n%s\n",
			err, output)
	}

	//verify that the output indicates <100% loss (some loss is expected due to
	// timing of interface creation and starting ping)
	if strings.Contains(string(output), ", 100% packet loss,") {
		t.Fatalf("Ping test failed. Output: \n%s\n", output)
	}
}

// Testcase:
// - Create a network with two vlans with two endpoints each
// - Verify that the endpoints in same vlan are able to ping
func TestSingleHostMultiVlanPingSuccess(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, vagrant.GetNodes())
	}()

	node := vagrant.GetNodes()[0]
	//create a single vlan network, with two endpoints
	jsonCfg :=
		`{
        "Tenants" : [ {
            "Name"                      : "tenant-one",
            "DefaultNetType"            : "vlan",
            "SubnetPool"                : "11.1.0.0/16",
            "AllocSubnetLen"            : 24,
            "Vlans"                     : "11-48",
            "Networks"  : [ {
                "Name"                  : "orange",
                "Endpoints" : [
                {
                    "Container"         : "myContainer1",
                    "Host"              : "host1"
                },
                {
                    "Container"         : "myContainer2",
                    "Host"              : "host1"
                } ]
            },
            {
                "Name"                  : "purple",
                "Endpoints" : [
                {
                    "Container"         : "myContainer3",
                    "Host"              : "host1"
                },
                {
                    "Container"         : "myContainer4",
                    "Host"              : "host1"
                } ]
            } ]
        } ]
        }`

	utils.ConfigSetupCommon(t, jsonCfg, vagrant.GetNodes())

	//start container 1, running a simple wait loop
	cmdStr := "sudo docker run -d --name=myContainer1 ubuntu /bin/bash -c 'mkfifo foo && < foo'"
	output, err := node.RunCommandWithOutput(cmdStr)
	defer func() {
		cmdStr = "sudo docker kill myContainer1"
		node.RunCommand(cmdStr)
		cmdStr = "sudo docker rm myContainer1"
		node.RunCommand(cmdStr)
	}()
	if err != nil {
		t.Fatalf("Failed to launch the container. Error: %s, Output: \n%s\n",
			err, output)
	}

	//start container 2 with ping for container 1
	cmdStr = "netdcli -oper get -construct endpoint orange-myContainer1 2>&1 | grep IpAddress | awk -F : '{gsub(\"[,}{]\",\"\", $2); print $2}'"
	output, err = node.RunCommandWithOutput(cmdStr)

	if err != nil || string(output) == "" {
		t.Fatalf("Failed to get ip address of the container. Error: %s, Output: \n%s\n",
			err, output)
	}

	ipAddress := string(output)
	cmdStr = fmt.Sprintf("sudo docker run --name=myContainer2 ubuntu /bin/bash -c 'ping -c5 %s'",
		ipAddress)
	output, err = node.RunCommandWithOutput(cmdStr)
	defer func() {
		cmdStr = "sudo docker kill myContainer2"
		node.RunCommand(cmdStr)
		cmdStr = "sudo docker rm myContainer2"
		node.RunCommand(cmdStr)
	}()
	if err != nil {
		t.Fatalf("Failed to launch the container. Error: %s, Output: \n%s\n",
			err, output)
	}

	//verify that the output indicates <100% loss (some loss is expected due to
	// timing of interface creation and starting ping)
	if strings.Contains(string(output), ", 100% packet loss,") {
		t.Fatalf("Ping test failed. Output: \n%s\n", output)
	}
}

// Testcase:
// - Create a network with two vlans with one endpoints each
// - Verify that the endpoints in different vlans are not able to ping
func TestSingleHostMultiVlanPingFailure(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, vagrant.GetNodes())
	}()

	node := vagrant.GetNodes()[0]
	//create a single vlan network, with two endpoints
	jsonCfg :=
		`{
        "Tenants" : [ {
            "Name"                      : "tenant-one",
            "DefaultNetType"            : "vlan",
            "SubnetPool"                : "11.1.0.0/16",
            "AllocSubnetLen"            : 24,
            "Vlans"                     : "11-48",
            "Networks"  : [ {
                "Name"                  : "orange",
                "Endpoints" : [
                {
                    "Container"         : "myContainer1",
                    "Host"              : "host1"
                } ]
            },
            {
                "Name"                  : "purple",
                "Endpoints" : [
                {
                    "Container"         : "myContainer2",
                    "Host"              : "host1"
                } ]
            } ]
        } ]
        }`

	utils.ConfigSetupCommon(t, jsonCfg, vagrant.GetNodes())

	//start container 1, running a simple wait loop
	cmdStr := "sudo docker run -d --name=myContainer1 ubuntu /bin/bash -c 'mkfifo foo && < foo'"
	output, err := node.RunCommandWithOutput(cmdStr)
	defer func() {
		cmdStr = "sudo docker kill myContainer1"
		node.RunCommand(cmdStr)
		cmdStr = "sudo docker rm myContainer1"
		node.RunCommand(cmdStr)
	}()
	if err != nil {
		t.Fatalf("Failed to launch the container. Error: %s, Output: \n%s\n",
			err, output)
	}

	//start container 2 with ping for container 1
	cmdStr = "netdcli -oper get -construct endpoint orange-myContainer1 2>&1 | grep IpAddress | awk -F : '{gsub(\"[,}{]\",\"\", $2); print $2}'"
	output, err = node.RunCommandWithOutput(cmdStr)

	if err != nil || string(output) == "" {
		t.Fatalf("Failed to get ip address of the container. Error: %s, Output: \n%s\n",
			err, output)
	}

	//verify that the output indicates 100% loss
	ipAddress := string(output)
	cmdStr = fmt.Sprintf("sudo docker run --name=myContainer2 ubuntu /bin/bash -c 'ping -c5 %s'",
		ipAddress)
	output, err = node.RunCommandWithOutput(cmdStr)
	defer func() {
		cmdStr = "sudo docker kill myContainer2"
		node.RunCommand(cmdStr)
		cmdStr = "sudo docker rm myContainer2"
		node.RunCommand(cmdStr)
	}()
	if err == nil || !strings.Contains(string(output), ", 100% packet loss,") {
		t.Fatalf("Ping succeeded, expected it to fail. Error: %s, Output: \n%s\n",
			err, output)
	}
}
