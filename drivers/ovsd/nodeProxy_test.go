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

package ovsd

import (
	"fmt"
	osexec "os/exec"
	"testing"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/state"
)

var ipTablesPath string

func verifyNATRule(nodePort uint16, destIP string, destPort uint16) error {
	dport := fmt.Sprintf("%d", nodePort)
	dest := fmt.Sprintf("%s:%d", destIP, destPort)
	_, err := osexec.Command(ipTablesPath, "-w", "5", "-t", "nat", "-C",
		contivNPChain, "-p", "tcp", "-m", "tcp", "--dport", dport, "-j",
		"DNAT", "--to-destination", dest).CombinedOutput()
	return err
}

func TestNodeProxy(t *testing.T) {
	driver := initOvsDriver(t, bridgeMode, defPvtNW)
	defer func() { driver.Deinit() }()
	var err error
	ipTablesPath, err = osexec.LookPath("iptables")
	if err != nil {
		t.Errorf("iptables not found %v", err)
	}

	// Verify PREROUTING jump rule exists
	out, err := osexec.Command(ipTablesPath, "-w", "5", "-t", "nat", "-C",
		"PREROUTING", "-m", "addrtype", "--dst-type", "LOCAL", "-j",
		contivNPChain).CombinedOutput()
	if err != nil {
		t.Logf("Output: %s", out)
		t.Errorf("PREROUTING jump rule not found %v", err)
	}

	// Add a nodePort service
	// Create a service spec
	svcPorts := make([]core.PortSpec, 2)
	svcPorts[0] = core.PortSpec{
		Protocol: "TCP",
		SvcPort:  5600,
		ProvPort: 9600,
	}
	svcPorts[1] = core.PortSpec{
		Protocol: "TCP",
		SvcPort:  5601,
		ProvPort: 9601,
		NodePort: 19201,
	}

	svc := core.ServiceSpec{
		IPAddress: "10.254.0.10",
		Ports:     svcPorts,
	}

	driver.HostProxy.AddSvcSpec("LipService", &svc)

	// add a local IP
	driver.HostProxy.AddLocalIP("23.4.5.6", "172.20.0.2")
	driver.HostProxy.SvcProviderUpdate("LipService", []string{"23.4.5.6"})

	// Verify the iptables rule is installed
	err = verifyNATRule(19201, "172.20.0.2", 9601)
	if err != nil {
		t.Errorf("NAT rule not found for 19201=>172.20.0.2:9601 -- err: %v",
			err)
	}

	// Change the provider to non-local
	driver.HostProxy.SvcProviderUpdate("LipService", []string{"23.4.5.7"})
	// Verify the iptables rule is removed
	err = verifyNATRule(19201, "172.20.0.2", 9601)
	if err == nil {
		t.Errorf("NAT rule still exists for 19201=>172.20.0.2:9601")
	}

	// Add another local provider to service
	driver.HostProxy.AddLocalIP("23.4.5.8", "172.20.0.3")
	driver.HostProxy.SvcProviderUpdate("LipService", []string{"23.4.5.7", "23.4.5.8"})
	// Verify the iptables rule is installed
	err = verifyNATRule(19201, "172.20.0.3", 9601)
	if err != nil {
		t.Errorf("NAT rule not found for 19201=>172.20.0.3:9601 -- err: %v",
			err)
	}

	// Change the local provider
	driver.HostProxy.SvcProviderUpdate("LipService", []string{"23.4.5.6", "23.4.5.7"})
	// Verify the iptables rules are updated
	err = verifyNATRule(19201, "172.20.0.3", 9601)
	if err == nil {
		t.Errorf("NAT rule still exists for 19201=>172.20.0.3:9601")
	}
	err = verifyNATRule(19201, "172.20.0.2", 9601)
	if err != nil {
		t.Errorf("NAT rule not found for 19201=>172.20.0.2:9601 -- err: %v",
			err)
	}

	// Issue another provider update
	driver.HostProxy.SvcProviderUpdate("LipService", []string{"23.4.5.6", "23.4.5.7", "23.4.5.8"})
	// verify no change to rule
	err = verifyNATRule(19201, "172.20.0.2", 9601)
	if err != nil {
		t.Errorf("NAT rule not found for 19201=>172.20.0.2:9601 -- err: %v",
			err)
	}

	// Add a second service with same nodeport
	svcPortsNew := make([]core.PortSpec, 1)
	svcPortsNew[0] = core.PortSpec{
		Protocol: "TCP",
		SvcPort:  5602,
		ProvPort: 9602,
		NodePort: 19201,
	}
	svcNew := core.ServiceSpec{
		IPAddress: "10.254.0.11",
		Ports:     svcPortsNew,
	}

	driver.HostProxy.AddSvcSpec("FakeService", &svcNew)
	driver.HostProxy.SvcProviderUpdate("FakeService", []string{"23.4.5.8"})
	err = verifyNATRule(19201, "172.20.0.3", 9602)
	if err == nil {
		t.Errorf("Clashing nodeport not detected")
	}

	// Swicth to a different port
	svcPortsNew[0] = core.PortSpec{
		Protocol: "TCP",
		SvcPort:  5602,
		ProvPort: 9602,
		NodePort: 19202,
	}
	driver.HostProxy.AddSvcSpec("FakeService", &svcNew)
	err = verifyNATRule(19202, "172.20.0.3", 9602)
	if err != nil {
		t.Errorf("NAT rule for 19202 => 172.20.0.3:9602 not found")
	}

	// Remove provider
	driver.HostProxy.SvcProviderUpdate("FakeService", []string{"23.4.5.7"})
	err = verifyNATRule(19202, "172.20.0.3", 9602)
	if err == nil {
		t.Errorf("NAT rule for 19202 => 172.20.0.3:9602 still exists")
	}

	// Delete the service
	driver.HostProxy.DelSvcSpec("LipService", &svc)
	err = verifyNATRule(19201, "172.20.0.2", 9601)
	if err == nil {
		t.Errorf("NAT rule still exists for 19201=>172.20.0.2:9601")
	}
}
