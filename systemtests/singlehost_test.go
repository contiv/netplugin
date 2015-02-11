package systemtests

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

func TestSingleHostSingleVlanFromFile(t *testing.T) {
	node := vagrant.GetNodes()[0]
	defer func() {
		output, err := node.RunCommandWithOutput("sudo $GOSRC/github.com/contiv/netplugin/scripts/cleanup")
		if err != nil {
			t.Errorf("Failed to cleanup the left over test case state. Error: %s, Output: \n%s\n",
				err, output)
		}
	}()

	//start the netplugin
	output, err := node.RunCommandWithOutput("sudo PATH=$PATH nohup netplugin -host-label host1 0<&- &>/tmp/netplugin.log &")
	if err != nil {
		t.Fatalf("Failed to launch netplugin. Error: %s, Output: \n%s\n",
			err, output)
	}
	defer func() {
		//XXX: remove this once netplugin is capable of handling cleanup
		node.RunCommand("sudo pkill netplugin")
	}()

	//create a single vlan network, with two endpoints and static IPs
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
                }
                ]
            } ]
        } ]
        }`

	// replace newlines with space and " with \" for echo to consume and
	// produce desired json config
	jsonCfg = strings.Replace(
		strings.Replace(jsonCfg, "\n", " ", -1),
		"\"", "\\\"", -1)
	output, err = node.RunCommandWithOutput(fmt.Sprintf("echo %s > /tmp/netdcli.cfg",
		jsonCfg))
	if err != nil {
		t.Fatalf("Failed to create netdcli.cfg file. Error: %s, Output: \n%s\n",
			err, output)
	}

	output, err = node.RunCommandWithOutput("netdcli -cfg /tmp/netdcli.cfg")
	if err != nil {
		t.Fatalf("Failed to issue netdcli. Error: %s, Output: \n%s\n",
			err, output)
	}

	//start container 1, running a simple wait loop
	output, err = node.RunCommandWithOutput("sudo docker run -d --name=myContainer1 ubuntu /bin/bash -c 'mkfifo foo && < foo'")
	if err != nil {
		t.Fatalf("Failed to launch the container. Error: %s, Output: \n%s\n",
			err, output)
	}
	defer func() {
		node.RunCommand("sudo docker kill myContainer1")
		node.RunCommand("sudo docker rm myContainer1")
	}()

	//start container 2 with ping for container 1
	//XXX: for now hardcode the IP. Need a better way of figuring out the allocated IP.
	output, err = node.RunCommandWithOutput("sudo docker run --name=myContainer2 ubuntu /bin/bash -c 'ping -c5 11.1.0.1'")
	if err != nil {
		t.Fatalf("Failed to launch the container. Error: %s, Output: \n%s\n",
			err, output)
	}
	defer func() {
		node.RunCommand("sudo docker kill myContainer2")
		node.RunCommand("sudo docker rm myContainer2")
	}()

	//verify that the output indicates <100% loss (some loss is expected due to
	// timing of interface creation and starting ping)
	if strings.Contains(string(output), ", 100% packet loss,") {
		t.Fatalf("Ping test failed. Output: \n%s\n", output)
	}
}

func TestSingleHostMultiVlanFrom(t *testing.T) {
}
