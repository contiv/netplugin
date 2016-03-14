package systemtests

import (
	"fmt"
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
}

type node struct {
	tbnode vagrantssh.TestbedNode
	suite  *systemtestSuite
}

func (n *node) rotateLog(prefix string) error {
	prefix = fmt.Sprintf("/tmp/%s", prefix)
	_, err := n.runCommand(fmt.Sprintf("mv %s.log %s-`date +%%s`.log", prefix, prefix))
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
	return n.tbnode.RunCommandBackground("sudo " + n.suite.binpath + "/netplugin -plugin-mode docker -vlan-if " + n.suite.vlanIf + " " + args + "&> /tmp/netplugin.log")
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
	return n.tbnode.RunCommandBackground(n.suite.binpath + "/netmaster &> /tmp/netmaster.log")
}

func (n *node) cleanupDockerNetwork() error {
	logrus.Infof("Cleaning up networks on %s", n.Name())
	return n.tbnode.RunCommand("docker network ls | grep netplugin | awk '{print $2}'")
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
	vNode.RunCommand("sudo rm /tmp/net*")
	vNode.RunCommand("sudo service docker restart")
}

func (n *node) cleanupMaster() {
	logrus.Infof("Cleaning up master on %s", n.Name())
	vNode := n.tbnode
	vNode.RunCommand("etcdctl rm --recursive /contiv")
	vNode.RunCommand("etcdctl rm --recursive /contiv.io")
	vNode.RunCommand("etcdctl rm --recursive /docker")
	vNode.RunCommand("etcdctl rm --recursive /skydns")
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
	var namestr, netstr string

	if spec.networkName != "" {
		netstr = spec.networkName

		if spec.serviceName != "" {
			netstr = spec.serviceName + "." + netstr
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

	logrus.Infof("Starting a container running %q on %s", spec.commandName, n.Name())

	cmd := fmt.Sprintf("docker run -itd %s %s %s %s", namestr, netstr, spec.imageName, spec.commandName)

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
	out, _ := n.tbnode.RunCommandWithOutput(`for i in /tmp/net*; do grep "error|fatal" $i; done`)
	if out != "" {
		return fmt.Errorf("error output in netplugin logs: %q", out)
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
