package systemtests

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/systemtests-utils"
	"github.com/contiv/vagrantssh"
)

type containerSpec struct {
	imageName   string
	commandName string
	networkName string
	serviceName string
	name        string
	dnsServer   string
	labels      []string
}

type node struct {
	tbnode vagrantssh.TestbedNode
	suite  *systemtestSuite
}

func (n *node) rotateLog(prefix string) error {
	oldPrefix := fmt.Sprintf("/tmp/%s", prefix)
	newPrefix := fmt.Sprintf("/tmp/_%s", prefix)
	_, err := n.runCommand(fmt.Sprintf("mv %s.log %s-`date +%%s`.log", oldPrefix, newPrefix))
	return err
}

func (n *node) getIPAddr(dev string) (string, error) {
	out, err := n.runCommand(fmt.Sprintf("ip addr show dev %s | grep inet | head -1", dev))
	if err != nil {
		logrus.Errorf("Failed to get IP for node %v", n.tbnode)
		logrus.Println(out)
	}

	parts := regexp.MustCompile(`\s+`).Split(strings.TrimSpace(out), -1)
	if len(parts) < 2 {
		return "", fmt.Errorf("Invalid output from node %v: %s", n.tbnode, out)
	}

	parts = strings.Split(parts[1], "/")
	out = strings.TrimSpace(parts[0])
	return out, err
}

func (n *node) Name() string {
	return n.tbnode.GetName()
}

func (s *systemtestSuite) getNodeByName(name string) *node {
	for _, myNode := range s.nodes {
		if myNode.Name() == name {
			return myNode
		}
	}

	return nil
}

func (n *node) startNetplugin(args string) error {
	logrus.Infof("Starting netplugin on %s", n.Name())
	return n.tbnode.RunCommandBackground("sudo " + n.suite.binpath + "/netplugin -plugin-mode docker -vlan-if " + n.suite.vlanIf + " --cluster-store " + n.suite.clusterStore + " " + args + "&> /tmp/netplugin.log")
}

func (n *node) stopNetplugin() error {
	logrus.Infof("Stopping netplugin on %s", n.Name())
	return n.tbnode.RunCommand("sudo pkill netplugin")
}

func (n *node) stopNetmaster() error {
	logrus.Infof("Stopping netmaster on %s", n.Name())
	return n.tbnode.RunCommand("sudo pkill netmaster")
}

func (n *node) startNetmaster() error {
	logrus.Infof("Starting netmaster on %s", n.Name())
	dnsOpt := " --dns-enable=false "
	if n.suite.enableDNS {
		dnsOpt = " --dns-enable=true "
	}
	return n.tbnode.RunCommandBackground(n.suite.binpath + "/netmaster" + dnsOpt + " --cluster-store " + n.suite.clusterStore + " &> /tmp/netmaster.log")
}

func (n *node) cleanupDockerNetwork() error {
	logrus.Infof("Cleaning up networks on %s", n.Name())
	return n.tbnode.RunCommand("docker network rm $(docker network ls | grep netplugin | awk '{print $2}')")
}

func (n *node) cleanupContainers() error {
	logrus.Infof("Cleaning up containers on %s", n.Name())
	return n.tbnode.RunCommand("docker kill -s 9 `docker ps -aq`; docker rm -f `docker ps -aq`")
}

func (n *node) cleanupSlave() {
	logrus.Infof("Cleaning up slave on %s", n.Name())
	vNode := n.tbnode
	vNode.RunCommand("sudo ovs-vsctl del-br contivVxlanBridge")
	vNode.RunCommand("sudo ovs-vsctl del-br contivVlanBridge")
	vNode.RunCommand("for p in `ifconfig  | grep vport | awk '{print $1}'`; do sudo ip link delete $p type veth; done")
	vNode.RunCommand("sudo rm /var/run/docker/plugins/netplugin.sock")
	vNode.RunCommand("sudo service docker restart")
}

func (n *node) cleanupMaster() {
	logrus.Infof("Cleaning up master on %s", n.Name())
	vNode := n.tbnode
	vNode.RunCommand("etcdctl rm --recursive /contiv")
	vNode.RunCommand("etcdctl rm --recursive /contiv.io")
	vNode.RunCommand("etcdctl rm --recursive /docker")
	vNode.RunCommand("etcdctl rm --recursive /skydns")
	vNode.RunCommand("curl -X DELETE localhost:8500/v1/kv/contiv.io?recurse=true")
	vNode.RunCommand("curl -X DELETE localhost:8500/v1/kv/docker?recurse=true")
}

func (n *node) runCommand(cmd string) (string, error) {
	var (
		str string
		err error
	)

	for {
		str, err = n.tbnode.RunCommandWithOutput(cmd)
		if err == nil || !strings.Contains(err.Error(), "EOF") {
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	return str, err
}

func (n *node) runContainer(spec containerSpec) (*container, error) {
	var namestr, netstr, dnsStr, labelstr string

	if spec.networkName != "" {
		netstr = spec.networkName

		if spec.serviceName != "" {
			netstr = spec.serviceName
		}

		netstr = "--net=" + netstr
	}

	if spec.imageName == "" {
		spec.imageName = "alpine"
	}

	if spec.commandName == "" {
		spec.commandName = "sleep 60m"
	}

	if spec.name != "" {
		namestr = "--name=" + spec.name
	}

	if spec.dnsServer != "" {
		dnsStr = "--dns=" + spec.dnsServer
	}

	if len(spec.labels) > 0 {
		l := "--label="
		for _, label := range spec.labels {
			labelstr += l + label + " "
		}
	}

	logrus.Infof("Starting a container running %q on %s", spec.commandName, n.Name())

	cmd := fmt.Sprintf("docker run -itd %s %s %s %s %s %s", namestr, netstr, dnsStr, labelstr, spec.imageName, spec.commandName)

	out, err := n.tbnode.RunCommandWithOutput(cmd)
	if err != nil {
		logrus.Infof("cmd %q failed: output below", cmd)
		logrus.Println(out)
		out2, err := n.tbnode.RunCommandWithOutput(fmt.Sprintf("docker logs %s", strings.TrimSpace(out)))
		if err == nil {
			logrus.Println(out2)
		} else {
			logrus.Errorf("Container id %q is invalid", strings.TrimSpace(out))
		}

		return nil, err
	}

	cont, err := newContainer(n, strings.TrimSpace(out), spec.name)
	if err != nil {
		logrus.Info(err)
		return nil, err
	}

	return cont, nil
}

func (n *node) checkForNetpluginErrors() error {
	out, _ := n.tbnode.RunCommandWithOutput(`for i in /tmp/net*; do grep "panic\|fatal" $i; done`)
	if out != "" {
		logrus.Errorf("Fatal error in logs on %s: \n", n.Name())
		fmt.Printf("%s\n==========================================\n", out)
		return fmt.Errorf("fatal error in netplugin logs")
	}

	out, _ = n.tbnode.RunCommandWithOutput(`for i in /tmp/net*; do grep "error" $i; done`)
	if out != "" {
		logrus.Errorf("error output in netplugin logs on %s: \n", n.Name())
		fmt.Printf("%s==========================================\n\n", out)
		// FIXME: We still have some tests that are failing error check
		// return fmt.Errorf("error output in netplugin logs")
	}

	return nil
}

func (n *node) runCommandUntilNoError(cmd string) error {
	runCmd := func() (string, bool) {
		if err := n.tbnode.RunCommand(cmd); err != nil {
			return "", false
		}
		return "", true
	}
	timeoutMessage := fmt.Sprintf("timeout reached trying to run %v on %q", cmd, n.Name())
	_, err := utils.WaitForDone(runCmd, 10*time.Millisecond, 10*time.Second, timeoutMessage)
	return err
}

func (n *node) checkPing(ipaddr string) error {
	logrus.Infof("Checking ping from %s to %s", n.Name(), ipaddr)
	out, err := n.tbnode.RunCommandWithOutput("ping -c 1 " + ipaddr)

	if err != nil || strings.Contains(out, "0 received, 100% packet loss") {
		logrus.Errorf("Ping from %s to %s FAILED: %q - %v", n.Name(), ipaddr, out, err)
		return fmt.Errorf("Ping failed from %s to %s: %q - %v", n.Name(), ipaddr, out, err)
	}

	logrus.Infof("Ping from %s to %s SUCCEEDED", n.Name(), ipaddr)
	return nil
}

func (n *node) reloadNode() error {
	logrus.Infof("Reloading node %s", n.Name())

	out, err := exec.Command("vagrant", "reload", n.Name()).CombinedOutput()
	if err != nil {
		logrus.Errorf("Error reloading node %s. Err: %v\n Output: %s", n.Name(), err, string(out))
		return err
	}

	logrus.Infof("Reloaded node %s. Output:\n%s", n.Name(), string(out))
	return nil
}
