package systemtests

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/remotessh"
	"github.com/contiv/systemtests-utils"
)

type node struct {
	tbnode remotessh.TestbedNode
	suite  *systemtestSuite
	exec   systemTestScheduler
}

type containerSpec struct {
	imageName   string
	commandName string
	networkName string
	serviceName string
	tenantName  string
	name        string
	dnsServer   string
	labels      []string
	epGroup     string
}

func (n *node) rotateLog(prefix string) error {
	if prefix == "netmaster" {
		return n.exec.rotateNetmasterLog()
	} else if prefix == "netplugin" {
		return n.exec.rotateNetpluginLog()
	}
	return nil
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
	return n.exec.startNetplugin(args)
}

func (n *node) stopNetplugin() error {
	return n.exec.stopNetplugin()
}

func (s *systemtestSuite) copyBinary(fileName string) error {
	logrus.Infof("Copying %s binary to %s", fileName, s.binpath)
	hostIPs := strings.Split(os.Getenv("HOST_IPS"), ",")
	srcFile := s.binpath + "/" + fileName
	destFile := s.binpath + "/" + fileName
	for i := 1; i < len(s.nodes); i++ {
		logrus.Infof("Copying %s binary to IP= %s and Directory = %s", srcFile, hostIPs[i], destFile)
		s.nodes[0].tbnode.RunCommand("scp -i " + s.keyFile + " " + srcFile + " " + hostIPs[i] + ":" + destFile)
	}
	return nil
}

func (n *node) deleteFile(file string) error {
	logrus.Infof("Deleting %s file ", file)
	return n.tbnode.RunCommand("sudo rm " + file)
}

func (n *node) stopNetmaster() error {
	return n.exec.stopNetmaster()
}

func (n *node) startNetmaster() error {
	return n.exec.startNetmaster()
}

func (n *node) cleanupSlave() {
	n.exec.cleanupSlave()
}

func (n *node) cleanupMaster() {
	n.exec.cleanupMaster()
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

func (n *node) checkForNetpluginErrors() error {
	return n.exec.checkForNetpluginErrors()
}

func (n *node) runCommandWithTimeOut(cmd string, tick, timeout time.Duration) error {
	runCmd := func() (string, bool) {
		if err := n.tbnode.RunCommand(cmd); err != nil {
			return "", false
		}
		return "", true
	}
	timeoutMessage := fmt.Sprintf("timeout reached trying to run %v on %q", cmd, n.Name())
	_, err := utils.WaitForDone(runCmd, tick, timeout, timeoutMessage)
	return err
}

func (n *node) runCommandUntilNoError(cmd string) error {
	return n.runCommandWithTimeOut(cmd, 10*time.Millisecond, 10*time.Second)
}

func (n *node) checkPingWithCount(ipaddr string, count int) error {
	logrus.Infof("Checking ping from %s to %s", n.Name(), ipaddr)
	cmd := fmt.Sprintf("ping -c %d %s", count, ipaddr)
	out, err := n.tbnode.RunCommandWithOutput(cmd)

	if err != nil || strings.Contains(out, "0 received, 100% packet loss") {
		logrus.Errorf("Ping from %s to %s FAILED: %q - %v", n.Name(), ipaddr, out, err)
		return fmt.Errorf("Ping failed from %s to %s: %q - %v", n.Name(), ipaddr, out, err)
	}

	logrus.Infof("Ping from %s to %s SUCCEEDED", n.Name(), ipaddr)
	return nil
}

func (n *node) checkPing(ipaddr string) error {
	return n.checkPingWithCount(ipaddr, 1)
}

func (n *node) reloadNode() error {
	return n.exec.reloadNode(n)
}

func (n *node) restartClusterStore() error {
	if strings.Contains(n.suite.clusterStore, "etcd://") {
		logrus.Infof("Restarting etcd on %s", n.Name())

		n.runCommand("sudo systemctl stop etcd")
		time.Sleep(5 * time.Second)
		n.runCommand("sudo systemctl start etcd")

		logrus.Infof("Restarted etcd on %s", n.Name())
	} else if strings.Contains(n.suite.clusterStore, "consul://") {
		logrus.Infof("Restarting consul on %s", n.Name())

		n.runCommand("sudo systemctl stop consul")
		time.Sleep(5 * time.Second)
		n.runCommand("sudo systemctl start consul")

		logrus.Infof("Restarted consul on %s", n.Name())
	}

	return nil
}

func (n *node) waitForListeners() error {
	return n.exec.waitForListeners()
}

func (n *node) verifyVTEPs(expVTEPS map[string]bool) (string, error) {
	return n.exec.verifyVTEPs(expVTEPS)
}
func (n *node) verifyEPs(epList []string) (string, error) {
	// read ep information from inspect
	return n.exec.verifyEPs(epList)
}

func (c *container) String() string {
	return fmt.Sprintf("(container: %s (name: %q ip: %s ipv6: %s host: %s))", c.containerID, c.name, c.eth0.ip, c.eth0.ipv6, c.node.Name())
}

func (n *node) checkSchedulerNetworkCreated(nwName string, expectedOp bool) error {
	return n.exec.checkSchedulerNetworkCreated(nwName, expectedOp)
}
