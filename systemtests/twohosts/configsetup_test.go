package twohosts

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
	err := vagrant.Setup(os.Getenv("CONTIV_ENV"), 2)
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
// - Create a single vlan network with two endpoints, one on each host
// - Verify that the endpoints are able to ping
func TestTwoHostsSingleVlanPingSuccess(t *testing.T) {
	defer func() {
		//utils.ConfigCleanupCommon(t, vagrant.GetNodes())
	}()

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
                "Name"                  : "infra",
                "PktTag"                : "0",
                "Endpoints" : [
                {
                    "Intf"              : "eth2",
                    "Host"              : "host1"
                },
                {
                    "Intf"              : "eth2",
                    "Host"              : "host2"
                } ]
            },
            {
                "Name"                  : "orange",
                "Endpoints" : [
                {
                    "Container"         : "myContainer1",
                    "Host"              : "host1"
                },
                {
                    "Container"         : "myContainer2",
                    "Host"              : "host2"
                } ]
            } ]
        } ]
        }`

	utils.ConfigSetupCommon(t, jsonCfg, vagrant.GetNodes())

	node1 := vagrant.GetNodes()[0]
	node2 := vagrant.GetNodes()[1]
	//start container 1, running a simple wait loop
	cmdStr := "sudo docker run -d --name=myContainer1 ubuntu /bin/bash -c 'mkfifo foo && < foo'"
	output, err := node1.RunCommandWithOutput(cmdStr)
	defer func() {
		cmdStr = "sudo docker kill myContainer1"
		node1.RunCommand(cmdStr)
		cmdStr = "sudo docker rm myContainer1"
		node1.RunCommand(cmdStr)
	}()
	if err != nil {
		t.Fatalf("Failed to launch the container. Error: %s, Output: \n%s\n",
			err, output)
	}

	//start container 2 with ping for container 1
	cmdStr = "netdcli -oper get -construct endpoint orange-myContainer1 2>&1 | grep IpAddress | awk -F : '{gsub(\"[,}{]\",\"\", $2); print $2}'"
	output, err = node2.RunCommandWithOutput(cmdStr)

	if err != nil || string(output) == "" {
		t.Fatalf("Failed to get ip address of the container. Error: %s, Output: \n%s\n",
			err, output)
	}

	ipAddress := string(output)
	cmdStr = fmt.Sprintf("sudo docker run --name=myContainer2 ubuntu /bin/bash -c 'ping -c5 %s'",
		ipAddress)
	output, err = node2.RunCommandWithOutput(cmdStr)
	defer func() {
		cmdStr = "sudo docker kill myContainer2"
		node2.RunCommand(cmdStr)
		cmdStr = "sudo docker rm myContainer2"
		node2.RunCommand(cmdStr)
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
// - Create a network with two vlans with two endpoints each (one endpoint per host)
// - Verify that the endpoints in same vlan are able to ping
func TestTwoHostsMultiVlanPingSuccess(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, vagrant.GetNodes())
	}()

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
                "Name"                  : "infra",
                "PktTag"                : "0",
                "Endpoints" : [
                {
                    "Intf"              : "eth2",
                    "Host"              : "host1"
                },
                {
                    "Intf"              : "eth2",
                    "Host"              : "host2"
                } ]
            },
            {
                "Name"                  : "orange",
                "Endpoints" : [
                {
                    "Container"         : "myContainer1",
                    "Host"              : "host1"
                },
                {
                    "Container"         : "myContainer2",
                    "Host"              : "host2"
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
                    "Host"              : "host2"
                } ]
            } ]
        } ]
        }`

	utils.ConfigSetupCommon(t, jsonCfg, vagrant.GetNodes())

	node1 := vagrant.GetNodes()[0]
	node2 := vagrant.GetNodes()[1]
	//start container 1, running a simple wait loop
	cmdStr := "sudo docker run -d --name=myContainer1 ubuntu /bin/bash -c 'mkfifo foo && < foo'"
	output, err := node1.RunCommandWithOutput(cmdStr)
	defer func() {
		cmdStr = "sudo docker kill myContainer1"
		node1.RunCommand(cmdStr)
		cmdStr = "sudo docker rm myContainer1"
		node1.RunCommand(cmdStr)
	}()
	if err != nil {
		t.Fatalf("Failed to launch the container. Error: %s, Output: \n%s\n",
			err, output)
	}

	//start container 2 with ping for container 1
	cmdStr = "netdcli -oper get -construct endpoint orange-myContainer1 2>&1 | grep IpAddress | awk -F : '{gsub(\"[,}{]\",\"\", $2); print $2}'"
	output, err = node2.RunCommandWithOutput(cmdStr)

	if err != nil || string(output) == "" {
		t.Fatalf("Failed to get ip address of the container. Error: %s, Output: \n%s\n",
			err, output)
	}

	ipAddress := string(output)
	cmdStr = fmt.Sprintf("sudo docker run --name=myContainer2 ubuntu /bin/bash -c 'ping -c5 %s'",
		ipAddress)
	output, err = node2.RunCommandWithOutput(cmdStr)
	defer func() {
		cmdStr = "sudo docker kill myContainer2"
		node2.RunCommand(cmdStr)
		cmdStr = "sudo docker rm myContainer2"
		node2.RunCommand(cmdStr)
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

	//start container 4, running a simple wait loop
	cmdStr = "sudo docker run -d --name=myContainer4 ubuntu /bin/bash -c 'mkfifo foo && < foo'"
	output, err = node2.RunCommandWithOutput(cmdStr)
	defer func() {
		cmdStr = "sudo docker kill myContainer4"
		node1.RunCommand(cmdStr)
		cmdStr = "sudo docker rm myContainer4"
		node1.RunCommand(cmdStr)
	}()
	if err != nil {
		t.Fatalf("Failed to launch the container. Error: %s, Output: \n%s\n",
			err, output)
	}

	//start container 3 with ping for container 1
	cmdStr = "netdcli -oper get -construct endpoint purple-myContainer4 2>&1 | grep IpAddress | awk -F : '{gsub(\"[,}{]\",\"\", $2); print $2}'"
	output, err = node1.RunCommandWithOutput(cmdStr)

	if err != nil || string(output) == "" {
		t.Fatalf("Failed to get ip address of the container. Error: %s, Output: \n%s\n",
			err, output)
	}

	ipAddress = string(output)
	cmdStr = fmt.Sprintf("sudo docker run --name=myContainer3 ubuntu /bin/bash -c 'ping -c5 %s'",
		ipAddress)
	output, err = node1.RunCommandWithOutput(cmdStr)
	defer func() {
		cmdStr = "sudo docker kill myContainer3"
		node2.RunCommand(cmdStr)
		cmdStr = "sudo docker rm myContainer3"
		node2.RunCommand(cmdStr)
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
func TestTwoHostsMultiVlanPingFailure(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, vagrant.GetNodes())
	}()

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
                "Name"                  : "infra",
                "PktTag"                : "0",
                "Endpoints" : [
                {
                    "Intf"              : "eth2",
                    "Host"              : "host1"
                },
                {
                    "Intf"              : "eth2",
                    "Host"              : "host2"
                } ]
            },
            {
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
                    "Host"              : "host2"
                } ]
            } ]
        } ]
        }`

	utils.ConfigSetupCommon(t, jsonCfg, vagrant.GetNodes())

	node1 := vagrant.GetNodes()[0]
	node2 := vagrant.GetNodes()[1]
	//start container 1, running a simple wait loop
	cmdStr := "sudo docker run -d --name=myContainer1 ubuntu /bin/bash -c 'mkfifo foo && < foo'"
	output, err := node1.RunCommandWithOutput(cmdStr)
	defer func() {
		cmdStr = "sudo docker kill myContainer1"
		node1.RunCommand(cmdStr)
		cmdStr = "sudo docker rm myContainer1"
		node1.RunCommand(cmdStr)
	}()
	if err != nil {
		t.Fatalf("Failed to launch the container. Error: %s, Output: \n%s\n",
			err, output)
	}

	//start container 2 with ping for container 1
	cmdStr = "netdcli -oper get -construct endpoint orange-myContainer1 2>&1 | grep IpAddress | awk -F : '{gsub(\"[,}{]\",\"\", $2); print $2}'"
	output, err = node2.RunCommandWithOutput(cmdStr)

	if err != nil || string(output) == "" {
		t.Fatalf("Failed to get ip address of the container. Error: %s, Output: \n%s\n",
			err, output)
	}

	//verify that the output indicates 100% loss
	ipAddress := string(output)
	cmdStr = fmt.Sprintf("sudo docker run --name=myContainer2 ubuntu /bin/bash -c 'ping -c5 %s'",
		ipAddress)
	output, err = node2.RunCommandWithOutput(cmdStr)
	defer func() {
		cmdStr = "sudo docker kill myContainer2"
		node2.RunCommand(cmdStr)
		cmdStr = "sudo docker rm myContainer2"
		node2.RunCommand(cmdStr)
	}()
	if err == nil || !strings.Contains(string(output), ", 100% packet loss,") {
		t.Fatalf("Ping succeeded, expected it to fail. Error: %s, Output: \n%s\n",
			err, output)
	}
}

// Testcase:
// - Create two vxlan networks with two endpoints each (one endpoint per host)
// - Verify that the endpoints in same vxlan are able to ping
func TestTwoHostsMultiVxlanPingSuccess(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, vagrant.GetNodes())
	}()

	//create a single vlan network, with two endpoints
	jsonCfg :=
		`{
        "Tenants" : [ {
            "Name"                      : "tenant-one",
            "DefaultNetType"            : "vxlan",
            "SubnetPool"                : "11.1.0.0/16",
            "AllocSubnetLen"            : 24,
            "VXlans"                    : "10001-20000",
            "Networks"  : [ 
            {
                "Name"                  : "orange",
                "Endpoints" : [
                {
                    "Container"         : "myContainer1",
                    "Host"              : "host1"
                },
                {
                    "Container"         : "myContainer2",
                    "Host"              : "host2"
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
                    "Host"              : "host2"
                } ]
            } ]
        } ]
        }`

	utils.ConfigSetupCommon(t, jsonCfg, vagrant.GetNodes())

	node1 := vagrant.GetNodes()[0]
	node2 := vagrant.GetNodes()[1]
	//start container 1, running a simple wait loop
	cmdStr := "sudo docker run -d --name=myContainer1 ubuntu /bin/bash -c 'mkfifo foo && < foo'"
	output, err := node1.RunCommandWithOutput(cmdStr)
	defer func() {
		cmdStr = "sudo docker kill myContainer1"
		node1.RunCommand(cmdStr)
		cmdStr = "sudo docker rm myContainer1"
		node1.RunCommand(cmdStr)
	}()
	if err != nil {
		t.Fatalf("Failed to launch the container. Error: %s, Output: \n%s\n",
			err, output)
	}

	//start container 2 with ping for container 1
	cmdStr = "netdcli -oper get -construct endpoint orange-myContainer1 2>&1 | grep IpAddress | awk -F : '{gsub(\"[,}{]\",\"\", $2); print $2}'"
	output, err = node2.RunCommandWithOutput(cmdStr)

	if err != nil || string(output) == "" {
		t.Fatalf("Failed to get ip address of the container. Error: %s, Output: \n%s\n",
			err, output)
	}

	cmdStr = "sudo ovs-vsctl show"
	output, _ = node1.RunCommandWithOutput(cmdStr)
	log.Printf("node1: ovs-vsctl %s \n", output)

	output, _ = node2.RunCommandWithOutput(cmdStr)
	log.Printf("node2: ovs-vsctl %s \n", output)

	ipAddress := string(output)
	cmdStr = fmt.Sprintf("sudo docker run --name=myContainer2 ubuntu /bin/bash -c 'ping -c5 %s'",
		ipAddress)
	output, err = node2.RunCommandWithOutput(cmdStr)
	defer func() {
		cmdStr = "sudo docker kill myContainer2"
		node2.RunCommand(cmdStr)
		cmdStr = "sudo docker rm myContainer2"
		node2.RunCommand(cmdStr)
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

	//start container 2, running a simple wait loop
	cmdStr = "sudo docker run -d --name=myContainer3 ubuntu /bin/bash -c 'mkfifo foo && < foo'"
	output, err = node1.RunCommandWithOutput(cmdStr)
	defer func() {
		cmdStr = "sudo docker kill myContainer3"
		node1.RunCommand(cmdStr)
		cmdStr = "sudo docker rm myContainer3"
		node1.RunCommand(cmdStr)
	}()
	if err != nil {
		t.Fatalf("Failed to launch the container. Error: %s, Output: \n%s\n",
			err, output)
	}

	//start container 2 with ping for container 1
	cmdStr = "netdcli -oper get -construct endpoint purple-myContainer3 2>&1 | grep IpAddress | awk -F : '{gsub(\"[,}{]\",\"\", $2); print $2}'"
	output, err = node2.RunCommandWithOutput(cmdStr)

	if err != nil || string(output) == "" {
		t.Fatalf("Failed to get ip address of the container. Error: %s, Output: \n%s\n",
			err, output)
	}

	ipAddress = string(output)
	cmdStr = fmt.Sprintf("sudo docker run --name=myContainer4 ubuntu /bin/bash -c 'ping -c5 %s'",
		ipAddress)
	output, err = node2.RunCommandWithOutput(cmdStr)
	defer func() {
		cmdStr = "sudo docker kill myContainer4"
		node2.RunCommand(cmdStr)
		cmdStr = "sudo docker rm myContainer4"
		node2.RunCommand(cmdStr)
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
// - Create two vxlan networks with two endpoints each (one endpoint per host)
// - Verify that the endpoints in same vxlan are able to ping
// - Verify that the endpoints in different vxlans are not able to ping
func TestTwoHostsMultiVxlanPingFailure(t *testing.T) {
	defer func() {
		utils.ConfigCleanupCommon(t, vagrant.GetNodes())
	}()

	//create a single vlan network, with two endpoints
	jsonCfg :=
		`{
        "Tenants" : [ {
            "Name"                      : "tenant-one",
            "DefaultNetType"            : "vxlan",
            "SubnetPool"                : "11.1.0.0/16",
            "AllocSubnetLen"            : 24,
            "VXlans"                    : "10001-20000",
            "Networks"  : [ 
            {
                "Name"                  : "orange",
                "Endpoints" : [
                {
                    "Container"         : "myContainer1",
                    "Host"              : "host1"
                },
                {
                    "Container"         : "myContainer2",
                    "Host"              : "host2"
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
                    "Host"              : "host2"
                } ]
            } ]
        } ]
        }`

	utils.ConfigSetupCommon(t, jsonCfg, vagrant.GetNodes())

	node1 := vagrant.GetNodes()[0]
	node2 := vagrant.GetNodes()[1]
	//start container 1, running a simple wait loop
	cmdStr := "sudo docker run -d --name=myContainer1 ubuntu /bin/bash -c 'mkfifo foo && < foo'"
	output, err := node1.RunCommandWithOutput(cmdStr)
	defer func() {
		cmdStr = "sudo docker kill myContainer1"
		node1.RunCommand(cmdStr)
		cmdStr = "sudo docker rm myContainer1"
		node1.RunCommand(cmdStr)
	}()
	if err != nil {
		t.Fatalf("Failed to launch the container. Error: %s, Output: \n%s\n",
			err, output)
	}

	//start container 2 with ping for container 1
	cmdStr = "netdcli -oper get -construct endpoint orange-myContainer1 2>&1 | grep IpAddress | awk -F : '{gsub(\"[,}{]\",\"\", $2); print $2}'"
	output, err = node2.RunCommandWithOutput(cmdStr)

	if err != nil || string(output) == "" {
		t.Fatalf("Failed to get ip address of the container. Error: %s, Output: \n%s\n",
			err, output)
	}

	//verify that the output indicates 100% loss
	ipAddress := string(output)
	cmdStr = fmt.Sprintf("sudo docker run --name=myContainer4 ubuntu /bin/bash -c 'ping -c5 %s'",
		ipAddress)
	output, err = node2.RunCommandWithOutput(cmdStr)
	defer func() {
		cmdStr = "sudo docker kill myContainer4"
		node2.RunCommand(cmdStr)
		cmdStr = "sudo docker rm myContainer4"
		node2.RunCommand(cmdStr)
	}()
	if err == nil || !strings.Contains(string(output), ", 100% packet loss,") {
		t.Fatalf("Ping succeeded, expected it to fail. Error: %s, Output: \n%s\n",
			err, output)
	}

	//verify that the output indicates 100% loss from container3 (same host)
	cmdStr = fmt.Sprintf("sudo docker run --name=myContainer3 ubuntu /bin/bash -c 'ping -c5 %s'",
		ipAddress)
	output, err = node1.RunCommandWithOutput(cmdStr)
	defer func() {
		cmdStr = "sudo docker kill myContainer3"
		node2.RunCommand(cmdStr)
		cmdStr = "sudo docker rm myContainer3"
		node2.RunCommand(cmdStr)
	}()
	if err == nil || !strings.Contains(string(output), ", 100% packet loss,") {
		t.Fatalf("Ping succeeded, expected it to fail. Error: %s, Output: \n%s\n",
			err, output)
	}
}
