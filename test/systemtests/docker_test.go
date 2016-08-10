package systemtests

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
)

type intf struct {
	ip   string
	ipv6 string
}

type container struct {
	node        *node
	containerID string
	name        string
	eth0        intf
}
type docker struct {
	node *node
}

func (s *systemtestSuite) NewDockerExec(n *node) *docker {
	d := new(docker)
	d.node = n
	return d
}

func (d *docker) newContainer(node *node, containerID, name string) (*container, error) {
	cont := &container{node: node, containerID: containerID, name: name}

	out, err := node.exec.getIPAddr(cont, "eth0")
	if err != nil {
		return nil, err
	}
	cont.eth0.ip = out

	out, err = cont.node.exec.getIPv6Addr(cont, "eth0")
	if err == nil {
		cont.eth0.ipv6 = out
	}

	return cont, nil
}

func (d *docker) runContainer(spec containerSpec) (*container, error) {
	var namestr, netstr, dnsStr, labelstr string

	if spec.networkName != "" {
		netstr = spec.networkName

		if spec.serviceName != "" {
			netstr = spec.serviceName
		}
		if spec.tenantName != "" && spec.tenantName != "default" {
			netstr = netstr + "/" + spec.tenantName
		}

		netstr = "--net=" + netstr
	}

	if spec.imageName == "" {
		spec.imageName = "alpine"
	}

	if spec.commandName == "" {
		spec.commandName = "sleep 600m"
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

	logrus.Infof("Starting a container running %q on %s", spec.commandName, d.node.Name())

	cmd := fmt.Sprintf("docker run -itd %s %s %s %s %s %s", namestr, netstr, dnsStr, labelstr, spec.imageName, spec.commandName)

	out, err := d.node.tbnode.RunCommandWithOutput(cmd)
	if err != nil {
		logrus.Infof("cmd %q failed: output below", cmd)
		logrus.Println(out)
		out2, err := d.node.tbnode.RunCommandWithOutput(fmt.Sprintf("docker logs %s", strings.TrimSpace(out)))
		if err == nil {
			logrus.Println(out2)
		} else {
			logrus.Errorf("Container id %q is invalid", strings.TrimSpace(out))
		}

		return nil, err
	}

	cont, err := d.newContainer(d.node, strings.TrimSpace(out), spec.name)
	if err != nil {
		logrus.Info(err)
		return nil, err
	}

	return cont, nil
}

func (d *docker) checkPingFailure(c *container, ipaddr string) error {
	logrus.Infof("Expecting ping failure from %v to %s", c, ipaddr)
	if err := d.checkPing(c, ipaddr); err == nil {
		return fmt.Errorf("Ping succeeded when expected to fail from %v to %s", c, ipaddr)
	}

	return nil
}

func (d *docker) checkPing(c *container, ipaddr string) error {
	logrus.Infof("Checking ping from %v to %s", c, ipaddr)
	out, err := d.exec(c, "ping -c 1 "+ipaddr)

	if err != nil || strings.Contains(out, "0 received, 100% packet loss") {
		logrus.Errorf("Ping from %v to %s FAILED: %q - %v", c, ipaddr, out, err)
		return fmt.Errorf("Ping failed from %v to %s: %q - %v", c, ipaddr, out, err)
	}

	logrus.Infof("Ping from %v to %s SUCCEEDED", c, ipaddr)
	return nil
}

func (d *docker) checkPing6Failure(c *container, ipaddr string) error {
	logrus.Infof("Expecting ping6 failure from %v to %s", c, ipaddr)
	if err := d.checkPing6(c, ipaddr); err == nil {
		return fmt.Errorf("Ping6 succeeded when expected to fail from %v to %s", c, ipaddr)
	}

	return nil
}

func (d *docker) checkPing6(c *container, ipaddr string) error {
	logrus.Infof("Checking ping6 from %v to %s", c, ipaddr)
	out, err := d.exec(c, "ping6 -c 1 "+ipaddr)

	if err != nil || strings.Contains(out, "0 received, 100% packet loss") {
		logrus.Errorf("Ping6 from %v to %s FAILED: %q - %v", c, ipaddr, out, err)
		return fmt.Errorf("Ping6 failed from %v to %s: %q - %v", c, ipaddr, out, err)
	}

	logrus.Infof("Ping6 from %v to %s SUCCEEDED", c, ipaddr)
	return nil
}

func (d *docker) getIPAddr(c *container, dev string) (string, error) {
	out, err := d.exec(c, fmt.Sprintf("ip addr show dev %s | grep inet | head -1", dev))
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

func (d *docker) getIPv6Addr(c *container, dev string) (string, error) {
	out, err := d.exec(c, fmt.Sprintf("ip addr show dev %s | grep 'inet6.*scope.*global' | head -1", dev))
	if err != nil {
		logrus.Errorf("Failed to get IPv6 for container %q", c.containerID)
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

func (d *docker) exec(c *container, args string) (string, error) {
	out, err := c.node.runCommand(fmt.Sprintf("docker exec %s %s", c.containerID, args))
	if err != nil {
		logrus.Println(out)
		return out, err
	}

	return out, nil
}

func (d *docker) execBG(c *container, args string) (string, error) {
	return c.node.runCommand(fmt.Sprintf("docker exec -d %s %s", c.containerID, args))
}

func (d *docker) dockerCmd(c *container, arg string) error {
	out, err := c.node.runCommand(fmt.Sprintf("docker %s %s", arg, c.containerID))
	if err != nil {
		logrus.Println(out)
		return err
	}

	return nil
}

func (d *docker) start(c *container) error {
	logrus.Infof("Starting container %s on %s", c.containerID, c.node.Name())
	return d.dockerCmd(c, "start")
}

func (d *docker) stop(c *container) error {
	logrus.Infof("Stopping container %s on %s", c.containerID, c.node.Name())
	return d.dockerCmd(c, "stop")
}

func (d *docker) rm(c *container) error {
	logrus.Infof("Removing container %s on %s", c.containerID, c.node.Name())
	d.dockerCmd(c, "kill -s 9")
	return d.dockerCmd(c, "rm -f")
}

func (d *docker) startListener(c *container, port int, protocol string) error {
	var protoStr string

	if protocol == "udp" {
		protoStr = "-u"
	}

	logrus.Infof("Starting a %s listener on %v port %d", protocol, c, port)
	_, err := d.execBG(c, fmt.Sprintf("nc -lk %s -p %v -e /bin/true", protoStr, port))
	return err
}

func (d *docker) checkConnection(c *container, ipaddr, protocol string, port int) error {
	var protoStr string

	if protocol == "udp" {
		protoStr = "-u"
	}

	logrus.Infof("Checking connection from %v to ip %s on port %d", c, ipaddr, port)

	_, err := d.exec(c, fmt.Sprintf("nc -z -n -v -w 1 %s %s %v", protoStr, ipaddr, port))
	if err != nil {
		logrus.Errorf("Connection from %v to ip %s on port %d FAILED", c, ipaddr, port)
	} else {
		logrus.Infof("Connection from %v to ip %s on port %d SUCCEEDED", c, ipaddr, port)
	}

	return err
}

func (d *docker) checkNoConnection(c *container, ipaddr, protocol string, port int) error {
	logrus.Infof("Expecting connection to fail from %v to %s on port %d", c, ipaddr, port)

	if err := d.checkConnection(c, ipaddr, protocol, port); err != nil {
		return nil
	}

	return fmt.Errorf("Connection SUCCEEDED on port %d from %s from %v when it should have FAILED.", port, ipaddr, c)
}

func (d *docker) cleanupDockerNetwork() error {
	logrus.Infof("Cleaning up networks on %s", d.node.Name())
	return d.node.tbnode.RunCommand("docker network ls | grep netplugin | awk '{print $2}'")
}

func (d *docker) cleanupContainers() error {
	logrus.Infof("Cleaning up containers on %s", d.node.Name())
	return d.node.tbnode.RunCommand("docker kill -s 9 `docker ps -aq`; docker rm -f `docker ps -aq`")
}

func (d *docker) startNetplugin(args string) error {
	logrus.Infof("Starting netplugin on %s", d.node.Name())
	return d.node.tbnode.RunCommandBackground("sudo " + d.node.suite.binpath + "/netplugin -plugin-mode docker -vlan-if " + d.node.suite.vlanIf + " --cluster-store " + d.node.suite.clusterStore + " " + args + "&> /tmp/netplugin.log")
}

func (d *docker) stopNetplugin() error {
	logrus.Infof("Stopping netplugin on %s", d.node.Name())
	return d.node.tbnode.RunCommand("sudo pkill netplugin")
}

func (d *docker) stopNetmaster() error {
	logrus.Infof("Stopping netmaster on %s", d.node.Name())
	return d.node.tbnode.RunCommand("sudo pkill netmaster")
}

func (d *docker) startNetmaster() error {
	logrus.Infof("Starting netmaster on %s", d.node.Name())
	dnsOpt := " --dns-enable=false "
	if d.node.suite.enableDNS {
		dnsOpt = " --dns-enable=true "
	}
	return d.node.tbnode.RunCommandBackground(d.node.suite.binpath + "/netmaster" + dnsOpt + " --cluster-store " + d.node.suite.clusterStore + " &> /tmp/netmaster.log")
}
func (d *docker) cleanupMaster() {
	logrus.Infof("Cleaning up master on %s", d.node.Name())
	vNode := d.node.tbnode
	vNode.RunCommand("etcdctl rm --recursive /contiv")
	vNode.RunCommand("etcdctl rm --recursive /contiv.io")
	vNode.RunCommand("etcdctl rm --recursive /docker")
	vNode.RunCommand("etcdctl rm --recursive /skydns")
	vNode.RunCommand("curl -X DELETE localhost:8500/v1/kv/contiv.io?recurse=true")
	vNode.RunCommand("curl -X DELETE localhost:8500/v1/kv/docker?recurse=true")
}

func (d *docker) cleanupSlave() {
	logrus.Infof("Cleaning up slave on %s", d.node.Name())
	vNode := d.node.tbnode
	vNode.RunCommand("sudo ovs-vsctl del-br contivVxlanBridge")
	vNode.RunCommand("sudo ovs-vsctl del-br contivVlanBridge")
	vNode.RunCommand("for p in `ifconfig  | grep vport | awk '{print $1}'`; do sudo ip link delete $p type veth; done")
	vNode.RunCommand("sudo rm /var/run/docker/plugins/netplugin.sock")
	vNode.RunCommand("sudo service docker restart")
}

func (d *docker) runCommandUntilNoNetpluginError() error {
	return d.node.runCommandUntilNoError("pgrep netplugin")
}

func (d *docker) runCommandUntilNoNetmasterError() error {
	return d.node.runCommandUntilNoError("pgrep netmaster")
}

func (d *docker) rotateNetmasterLog() error {
	return d.rotateLog("netmaster")
}

func (d *docker) rotateNetpluginLog() error {
	return d.rotateLog("netplugin")
}

func (d *docker) checkForNetpluginErrors() error {
	out, _ := d.node.tbnode.RunCommandWithOutput(`for i in /tmp/net*; do grep -A 5 "panic\|fatal" $i; done`)
	if out != "" {
		logrus.Errorf("Fatal error in logs on %s: \n", d.node.Name())
		fmt.Printf("%s\n==========================================\n", out)
		return fmt.Errorf("fatal error in netplugin logs")
	}

	out, _ = d.node.tbnode.RunCommandWithOutput(`for i in /tmp/net*; do grep "error" $i; done`)
	if out != "" {
		logrus.Errorf("error output in netplugin logs on %s: \n", d.node.Name())
		fmt.Printf("%s==========================================\n\n", out)
		// FIXME: We still have some tests that are failing error check
		// return fmt.Errorf("error output in netplugin logs")
	}

	return nil
}

func (d *docker) rotateLog(prefix string) error {
	oldPrefix := fmt.Sprintf("/tmp/%s", prefix)
	newPrefix := fmt.Sprintf("/tmp/_%s", prefix)
	_, err := d.node.runCommand(fmt.Sprintf("mv %s.log %s-`date +%%s`.log", oldPrefix, newPrefix))
	return err
}

func (d *docker) checkConnectionRetry(c *container, ipaddr, protocol string, port, delay, retries int) error {
	var protoStr string
	var err error

	err = nil

	if protocol == "udp" {
		protoStr = "-u"
	}

	logrus.Infof("Checking connection from %c to ip %s on port %d, delay: %d, retries: %d",
		c, ipaddr, port, delay, retries)

	for i := 0; i < retries; i++ {

		_, err = d.exec(c, fmt.Sprintf("nc -z -n -v -w 1 %s %s %v", protoStr, ipaddr, port))
		if err == nil {
			logrus.Infof("Connection to ip %s on port %d SUCCEEDED, tries: %d", ipaddr, port, i+1)
			return nil
		}
		time.Sleep(2 * time.Second)
	}

	logrus.Errorf("Connection  to ip %s on port %d FAILED %v", ipaddr, port, err)
	return err
}

func (d *docker) checkNoConnectionRetry(c *container, ipaddr, protocol string, port, delay, retries int) error {
	logrus.Infof("Expecting connection to fail from %v to %s on port %d", c, ipaddr, port)

	if err := d.checkConnectionRetry(c, ipaddr, protocol, port, delay, retries); err != nil {
		return nil
	}

	return fmt.Errorf("Connection SUCCEEDED on port %d from %s from %s when it should have FAILED.", port, ipaddr, c)
}

func (d *docker) checkPing6WithCount(c *container, ipaddr string, count int) error {
	logrus.Infof("Checking ping6 from %v to %s", c, ipaddr)
	cmd := fmt.Sprintf("ping6 -c %d %s", count, ipaddr)
	out, err := d.exec(c, cmd)

	if err != nil || strings.Contains(out, "0 received, 100% packet loss") {
		logrus.Errorf("Ping6 from %s to %s FAILED: %q - %v", c, ipaddr, out, err)
		return fmt.Errorf("Ping6 failed from %s to %s: %q - %v", c, ipaddr, out, err)
	}

	logrus.Infof("Ping6 from %s to %s SUCCEEDED", c, ipaddr)
	return nil
}

func (d *docker) checkPingWithCount(c *container, ipaddr string, count int) error {
	logrus.Infof("Checking ping from %s to %s", c, ipaddr)
	cmd := fmt.Sprintf("ping -c %d %s", count, ipaddr)
	out, err := d.exec(c, cmd)

	if err != nil || strings.Contains(out, "0 received, 100% packet loss") {
		logrus.Errorf("Ping from %s to %s FAILED: %q - %v", c, ipaddr, out, err)
		return fmt.Errorf("Ping failed from %s to %s: %q - %v", c, ipaddr, out, err)
	}

	logrus.Infof("Ping from %s to %s SUCCEEDED", c, ipaddr)
	return nil
}

func (d *docker) checkSchedulerNetworkCreated(nwName string, expectedOp bool) error {
	logrus.Infof("Checking whether docker network is created or not")
	cmd := fmt.Sprintf("docker network ls | grep netplugin | grep %s | awk \"{print \\$2}\"", nwName)
	logrus.Infof("Command to be executed is = %s", cmd)
	op, err := d.node.runCommand(cmd)

	if err == nil {
		// if networks are NOT meant to be created. In ACI mode netctl net create should
		// not create docker networks
		ret := strings.Contains(op, nwName)
		if expectedOp == false && ret != true {
			logrus.Infof("Network names Input=%s and Output=%s are NOT matching and thats expected", nwName, op)
		} else {
			// If netwokrs are meant to be created. In ACI Once you create EPG,
			// respective docker network should get created.
			if ret == true {
				logrus.Infof("Network names are matching.")
				return nil
			}
		}
		return nil
	}
	return err
}

func (d *docker) waitForListeners() error {
	return d.node.runCommandWithTimeOut("netstat -tlpn | grep 9090 | grep LISTEN", 500*time.Millisecond, 50*time.Second)
}

func (d *docker) verifyVTEPs(expVTEPS map[string]bool) (string, error) {
	var data interface{}
	actVTEPs := make(map[string]uint32)

	// read vtep information from inspect
	cmd := "curl -s localhost:9090/inspect/driver | python -mjson.tool"
	str, err := d.node.tbnode.RunCommandWithOutput(cmd)
	if err != nil {
		return "", err
	}

	err = json.Unmarshal([]byte(str), &data)
	if err != nil {
		logrus.Errorf("Unmarshal error: %v", err)
		return str, err
	}

	drvInfo := data.(map[string]interface{})
	vx, found := drvInfo["vxlan"]
	if !found {
		logrus.Errorf("vxlan not found in driver info")
		return str, errors.New("vxlan not found in driver info")
	}

	vt := vx.(map[string]interface{})
	v, found := vt["VtepTable"]
	if !found {
		logrus.Errorf("VtepTable not found in driver info")
		return str, errors.New("VtepTable not found in driver info")
	}

	vteps := v.(map[string]interface{})
	for key := range vteps {
		actVTEPs[key] = 1
	}

	// read local ip
	l, found := vt["LocalIp"]
	if found {
		switch l.(type) {
		case string:
			localVtep := l.(string)
			actVTEPs[localVtep] = 1
		}
	}

	for vtep := range expVTEPS {
		_, found := actVTEPs[vtep]
		if !found {
			return str, errors.New("VTEP " + vtep + " not found")
		}
	}

	return "", nil
}
func (d *docker) verifyEPs(epList []string) (string, error) {
	// read ep information from inspect
	cmd := "curl -s localhost:9090/inspect/driver | python -mjson.tool"
	str, err := d.node.tbnode.RunCommandWithOutput(cmd)
	if err != nil {
		return "", err
	}

	for _, ep := range epList {
		if !strings.Contains(str, ep) {
			return str, errors.New(ep + " not found on " + d.node.Name())
		}
	}

	return "", nil
}

func (d *docker) reloadNode(n *node) error {
	logrus.Infof("Reloading node %s", n.Name())

	out, err := exec.Command("vagrant", "reload", n.Name()).CombinedOutput()
	if err != nil {
		logrus.Errorf("Error reloading node %s. Err: %v\n Output: %s", n.Name(), err, string(out))
		return err
	}

	logrus.Infof("Reloaded node %s. Output:\n%s", n.Name(), string(out))
	return nil
}
func (d *docker) getMasterIP() (string, error) {
	return d.node.getIPAddr("eth1")
}
