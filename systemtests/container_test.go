package systemtests

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Sirupsen/logrus"
)

type container struct {
	node        *node
	containerID string
	name        string
	eth0        string
}

func newContainer(node *node, containerID, name string) (*container, error) {
	cont := &container{node: node, containerID: containerID, name: name}

	out, err := cont.getIPAddr("eth0")
	if err != nil {
		return nil, err
	}
	cont.eth0 = out

	return cont, nil
}

func (c *container) String() string {
	return fmt.Sprintf("(container: %s (name: %q ip: %s host: %s))", c.containerID, c.name, c.eth0, c.node.Name())
}

func (c *container) checkPingFailure(ipaddr string) error {
	logrus.Infof("Expecting ping failure from %v to %s", c, ipaddr)
	if err := c.checkPing(ipaddr); err == nil {
		return fmt.Errorf("Ping succeeded when expected to fail from %v to %s", c, ipaddr)
	}

	return nil
}

func (c *container) checkPing(ipaddr string) error {
	logrus.Infof("Checking ping from %v to %s", c, ipaddr)
	out, err := c.exec("ping -c 1 " + ipaddr)

	if err != nil || strings.Contains(out, "0 received, 100% packet loss") {
		logrus.Errorf("Ping from %v to %s FAILED: %q - %v", c, ipaddr, out, err)
		return fmt.Errorf("Ping failed from %v to %s: %q - %v", c, ipaddr, out, err)
	}

	logrus.Infof("Ping from %v to %s SUCCEEDED", c, ipaddr)
	return nil
}

func (c *container) getIPAddr(dev string) (string, error) {
	out, err := c.exec(fmt.Sprintf("ip addr show dev %s | grep inet | head -1", dev))
	if err != nil {
		logrus.Errorf("Failed to get IP for container %q", c.containerID)
		logrus.Println(out)
	}

	parts := regexp.MustCompile(`\s+`).Split(strings.TrimSpace(out), -1)
	if len(parts) < 2 {
		return "", fmt.Errorf("Invalid output from container %q: %s", c.containerID, out)
	}

	parts = strings.Split(parts[1], "/")
	out = strings.TrimSpace(parts[0])
	return out, err
}

func (c *container) exec(args string) (string, error) {
	return c.node.runCommand(fmt.Sprintf("docker exec %s %s", c.containerID, args))
}

func (c *container) execBG(args string) (string, error) {
	return c.node.runCommand(fmt.Sprintf("docker exec -d %s %s", c.containerID, args))
}

func (c *container) dockerCmd(arg string) error {
	out, err := c.node.runCommand(fmt.Sprintf("docker %s %s", arg, c.containerID))
	if err != nil {
		logrus.Println(out)
		return err
	}

	return nil
}

func (c *container) start() error {
	logrus.Infof("Starting container %s on %s", c.containerID, c.node.Name())
	return c.dockerCmd("start")
}

func (c *container) stop() error {
	logrus.Infof("Stopping container %s on %s", c.containerID, c.node.Name())
	return c.dockerCmd("stop")
}

func (c *container) rm() error {
	logrus.Infof("Removing container %s on %s", c.containerID, c.node.Name())
	c.dockerCmd("kill -s 9")
	return c.dockerCmd("rm -f")
}

func (c *container) startListener(port int, protocol string) error {
	var protoStr string

	if protocol == "udp" {
		protoStr = "-u"
	}

	logrus.Infof("Starting a %s listener on %v port %d", protocol, c, port)
	_, err := c.execBG(fmt.Sprintf("nc -lk %s -p %v -e /bin/true", protoStr, port))
	return err
}

func (c *container) checkConnection(ipaddr, protocol string, port int) error {
	var protoStr string

	if protocol == "udp" {
		protoStr = "-u"
	}

	logrus.Infof("Checking connection from %v to ip %s on port %d", c, ipaddr, port)

	_, err := c.exec(fmt.Sprintf("nc -z -n -v -w 1 %s %s %v", protoStr, ipaddr, port))
	if err != nil {
		logrus.Errorf("Connection from %v to ip %s on port %d FAILED", c, ipaddr, port)
	} else {
		logrus.Infof("Connection from %v to ip %s on port %d SUCCEEDED", c, ipaddr, port)
	}

	return err
}

func (c *container) checkNoConnection(ipaddr, protocol string, port int) error {
	logrus.Infof("Expecting connection to fail from %v to %s on port %d", c, ipaddr, port)

	if err := c.checkConnection(ipaddr, protocol, port); err != nil {
		return nil
	}

	return fmt.Errorf("Connection SUCCEEDED on port %d from %s from %v when it should have FAILED.", port, ipaddr, c)
}
