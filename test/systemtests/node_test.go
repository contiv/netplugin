package systemtests

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/contiv/remotessh"
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
		return "", fmt.Errorf("invalid output from node %v: %s", n.tbnode, out)
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
	logrus.Infof("Copying %s binary to %s", fileName, s.basicInfo.BinPath)
	hostIPs := strings.Split(s.hostInfo.HostIPs, ",")
	srcFile := s.basicInfo.BinPath + "/" + fileName
	destFile := s.basicInfo.BinPath + "/" + fileName
	for i := 1; i < len(s.nodes); i++ {
		logrus.Infof("Copying %s binary to IP= %s and Directory = %s", srcFile, hostIPs[i], destFile)
		s.nodes[0].tbnode.RunCommand("scp -oStrictHostKeyChecking=no -i " + s.basicInfo.KeyFile + " " + srcFile + " " + hostIPs[i] + ":" + destFile)
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

func (n *node) startNetmaster(args string) error {
	return n.exec.startNetmaster(args)
}

func (n *node) cleanupSlave() {
	n.exec.cleanupSlave()
}

func (n *node) cleanupMaster() {
	n.exec.cleanupMaster()
}

func (n *node) verifyUplinkState(uplinks []string) error {
	return n.exec.verifyUplinkState(n, uplinks)
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
		logrus.Debugf("Running cmd: %s", cmd)
		if err := n.tbnode.RunCommand(cmd); err != nil {
			return "", false
		}
		return "", true
	}
	timeoutMessage := fmt.Sprintf("timeout reached trying to run %v on %q", cmd, n.Name())
	_, err := waitForDone(runCmd, tick, timeout, timeoutMessage)
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
		return fmt.Errorf("ping failed from %s to %s: %q - %v", n.Name(), ipaddr, out, err)
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
	if n.suite.basicInfo.ClusterStoreDriver == "etcd" {
		logrus.Infof("Restarting etcd on %s", n.Name())

		n.runCommand("sudo systemctl stop etcd")
		time.Sleep(5 * time.Second)
		n.runCommand("sudo systemctl start etcd")

		logrus.Infof("Restarted etcd on %s", n.Name())
	} else if n.suite.basicInfo.ClusterStoreDriver == "consul" {
		logrus.Infof("Restarting consul on %s", n.Name())

		n.runCommand("sudo systemctl stop consul")
		time.Sleep(5 * time.Second)
		n.runCommand("sudo systemctl start consul")

		logrus.Infof("Restarted consul on %s", n.Name())
	}

	return nil
}

// bring down an interface on a node
func (n *node) bringDownIf(ifname string) error {
	logrus.Infof("Bringing down interface %s on node %s", ifname, n.Name())
	out, err := n.runCommand(fmt.Sprintf("sudo ifconfig %s down", ifname))
	if err != nil {
		logrus.Errorf("Failed to bring down interface %s on node %v", ifname, n.Name())
		logrus.Println(out)
		return err
	}

	return nil
}

// bring up an interface on a node
func (n *node) bringUpIf(ifname, ipAddr string) error {
	logrus.Infof("Bringing up interface %s with addr %s/24 on node %s", ifname, ipAddr, n.Name())

	for i := 0; i < 3; i++ {
		// bring up the interface
		out, err := n.runCommand(fmt.Sprintf("sudo ifconfig %s %s/24 up", ifname, ipAddr))
		if err != nil {
			logrus.Errorf("Failed to bring up interface %s on node %v", ifname, n.Name())
			logrus.Println(out)
			return err
		}

		// verify link is up
		out, err = n.runCommand(fmt.Sprintf("sudo ifconfig %s", ifname))
		if err != nil {
			logrus.Errorf("Failed to check interface %s status on node %v", ifname, n.Name())
			logrus.Println(out)
			return err
		}

		// if the interface is up, we are done
		if strings.Contains(out, "RUNNING") {
			return nil
		}
	}

	return fmt.Errorf("failed to bring up interface")
}

func (n *node) waitForListeners() error {
	return n.exec.waitForListeners()
}

func (n *node) verifyAgentDB(expAgents map[string]bool) (string, error) {
	return n.exec.verifyAgents(expAgents)
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

func (n *node) checkSchedulerNetworkOnNodeCreated(nwName []string) error {
	return n.exec.checkSchedulerNetworkOnNodeCreated(nwName, n)
}

// waitForDone polls for checkDoneFn function to return true up until specified timeout
func waitForDone(doneFn func() (string, bool), tickDur time.Duration, timeoutDur time.Duration, timeoutMsg string) (string, error) {
	tick := time.Tick(tickDur)
	timeout := time.After(timeoutDur)
	doneCount := 0
	for {
		select {
		case <-tick:
			if ctxt, done := doneFn(); done {
				doneCount++
				// add some resilliency to poll inorder to avoid false positives,
				// while polling more frequently
				if doneCount == 2 {
					// end poll
					return ctxt, nil

				}

			}
			//continue polling
		case <-timeout:
			ctxt, done := doneFn()
			if !done {
				return ctxt, fmt.Errorf("wait timeout. Msg: %s", timeoutMsg)

			}
			return ctxt, nil

		}

	}

}
