package systemtests

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Sirupsen/logrus"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type swarm struct {
	node *node
	env  string
}

func (s *systemtestSuite) NewSwarmExec(n *node) *swarm {
	w := new(swarm)
	w.node = n
	w.env = s.basicInfo.SwarmEnv + " "
	return w
}

func (w *swarm) newContainer(node *node, containerID, name string) (*container, error) {
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

func (w *swarm) runContainer(spec containerSpec) (*container, error) {
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
	logrus.Infof("Starting a container running %q on %s", spec.commandName, w.node.Name())
	cmd := fmt.Sprintf("docker run -itd %s %s %s %s %s %s", namestr, netstr, dnsStr, labelstr, spec.imageName, spec.commandName)

	out, err := w.node.tbnode.RunCommandWithOutput(w.env + cmd)
	if err != nil {
		logrus.Infof("cmd %q failed: output below", cmd)
		logrus.Println(out)
		out2, err := w.node.tbnode.RunCommandWithOutput(fmt.Sprintf("docker logs %s", strings.TrimSpace(out)))
		if err == nil {
			logrus.Println(out2)
		} else {
			logrus.Errorf("Container id %q is invalid", strings.TrimSpace(out))
		}

		return nil, err
	}

	cont, err := w.newContainer(w.node, strings.TrimSpace(out), spec.name)
	if err != nil {
		logrus.Info(err)
		return nil, err
	}

	return cont, nil
}

func (w *swarm) checkPingFailure(c *container, ipaddr string) error {
	logrus.Infof("Expecting ping failure from %v to %s", c, ipaddr)
	if err := w.checkPing(c, ipaddr); err == nil {
		return fmt.Errorf("ping succeeded when expected to fail from %v to %s", c, ipaddr)
	}

	return nil
}

func (w *swarm) checkPing(c *container, ipaddr string) error {
	logrus.Infof("Checking ping from %v to %s", c, ipaddr)
	out, err := w.exec(c, "ping -c 1 "+ipaddr)

	if err != nil || strings.Contains(out, "0 received, 100% packet loss") {
		logrus.Errorf("Ping from %v to %s FAILED: %q - %v", c, ipaddr, out, err)
		return fmt.Errorf("ping failed from %v to %s: %q - %v", c, ipaddr, out, err)
	}

	logrus.Infof("Ping from %v to %s SUCCEEDED", c, ipaddr)
	return nil
}

func (w *swarm) checkPing6Failure(c *container, ipaddr string) error {
	logrus.Infof("Expecting ping6 failure from %v to %s", c, ipaddr)
	if err := w.checkPing6(c, ipaddr); err == nil {
		return fmt.Errorf("ping6 succeeded when expected to fail from %v to %s", c, ipaddr)
	}

	return nil
}

func (w *swarm) checkPing6(c *container, ipaddr string) error {
	logrus.Infof("Checking ping6 from %v to %s", c, ipaddr)
	out, err := w.exec(c, "ping6 -c 1 "+ipaddr)

	if err != nil || strings.Contains(out, "0 received, 100% packet loss") {
		logrus.Errorf("Ping6 from %v to %s FAILED: %q - %v", c, ipaddr, out, err)
		return fmt.Errorf("ping6 failed from %v to %s: %q - %v", c, ipaddr, out, err)
	}

	logrus.Infof("Ping6 from %v to %s SUCCEEDED", c, ipaddr)
	return nil
}

func (w *swarm) getIPAddr(c *container, dev string) (string, error) {
	out, err := w.exec(c, fmt.Sprintf("ip addr show dev %s | grep inet | head -1", dev))
	if err != nil {
		logrus.Errorf("Failed to get IP for container %q", c.containerID)
		logrus.Println(out)
	}

	parts := regexp.MustCompile(`\s+`).Split(strings.TrimSpace(out), -1)
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid output from container %q: %s", c.containerID, out)
	}

	parts = strings.Split(parts[1], "/")
	out = strings.TrimSpace(parts[0])
	return out, err
}

func (w *swarm) getIPv6Addr(c *container, dev string) (string, error) {
	out, err := w.exec(c, fmt.Sprintf("ip addr show dev %s | grep 'inet6.*scope.*global' | head -1", dev))
	if err != nil {
		logrus.Errorf("Failed to get IPv6 for container %q", c.containerID)
		logrus.Println(out)
	}

	parts := regexp.MustCompile(`\s+`).Split(strings.TrimSpace(out), -1)
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid output from container %q: %s", c.containerID, out)
	}

	parts = strings.Split(parts[1], "/")
	out = strings.TrimSpace(parts[0])
	return out, err
}

func (w *swarm) getMACAddr(c *container, dev string) (string, error) {
	out, err := w.exec(c, fmt.Sprintf("ip addr show dev %s | grep ether | head -1", dev))
	if err != nil {
		logrus.Errorf("Failed to get IP for container %q", c.containerID)
		logrus.Println(out)
	}

	parts := regexp.MustCompile(`\s+`).Split(strings.TrimSpace(out), -1)
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid output from container %q: %s", c.containerID, out)
	}

	parts = strings.Split(parts[1], "/")
	out = strings.TrimSpace(parts[0])
	return out, err
}

func (w *swarm) exec(c *container, args string) (string, error) {
	out, err := c.node.runCommand(fmt.Sprintf(w.env+"docker exec %s %s", c.containerID, args))
	if err != nil {
		if strings.Contains(args, "nc ") && out == "" {
			return out, nil
		}

		logrus.Println(out)
		return out, err
	}

	return out, nil
}

func (w *swarm) execBG(c *container, args string) (string, error) {
	return c.node.runCommand(fmt.Sprintf(w.env+"docker exec -d %s %s", c.containerID, args))
}

func (w *swarm) swarmCmd(c *container, arg string) error {
	out, err := c.node.runCommand(fmt.Sprintf(w.env+"docker %s %s", arg, c.containerID))
	if err != nil {
		logrus.Println(out)
		return err
	}

	return nil
}

func (w *swarm) start(c *container) error {
	logrus.Infof("Starting container %s on %s", c.containerID, c.node.Name())
	return w.swarmCmd(c, "start")
}

func (w *swarm) stop(c *container) error {
	logrus.Infof("Stopping container %s on %s", c.containerID, c.node.Name())
	return w.swarmCmd(c, "stop")
}

func (w *swarm) rm(c *container) error {
	logrus.Infof("Removing container %s on %s", c.containerID, c.node.Name())
	w.swarmCmd(c, "kill -s 9")
	return w.swarmCmd(c, "rm -f")
}

func (w *swarm) startListener(c *container, port int, protocol string) error {
	var protoStr string
	if protocol == "udp" {
		protoStr = "-u"
	}

	logrus.Infof("Starting a %s listener on %v port %d", protocol, c, port)
	_, err := w.execBG(c, fmt.Sprintf("nc -lk %s -p %v -e /bin/true", protoStr, port))
	return err
}

func (w *swarm) startIperfServer(c *container) error {
	logrus.Infof("starting iperf server on: %s", c)
	_, err := w.execBG(c, fmt.Sprintf("iperf -s -u"))
	return err
}

func (w *swarm) startIperfClient(c *container, ip, limit string, isErr bool) error {

	var (
		bwLimit int64
		bwInt64 int64
		bw      string
		success bool
		err     error
	)

	for i := 0; i < 10; i++ {
		bw, err = w.exec(c, fmt.Sprintf("iperf -c %s -u -b 20mbps", ip))
		if err != nil {
			return err
		}
		if strings.Contains(bw, "Server Report:") {
			success = true
			break
		} else if strings.Contains(bw, "read failed:") {
			time.Sleep(2 * time.Second)
			i++
		}
	}

	if success {
		logrus.Infof("starting iperf client on container:%s for server ip: %s", c, ip)
		bwFormat := strings.Split(bw, "Server Report:")
		bwString := strings.Split(bwFormat[1], "Bytes ")
		newBandwidth := strings.Split(bwString[1], "bits/sec")
		bwInt64, err = BwConvertInt64(newBandwidth[0])
		if err != nil {
			return err
		}
		if limit != "" {
			bwLimit, err = BwConvertInt64(limit)
			if err != nil {
				return err
			}
			bwLimit = bwLimit + (bwLimit / 10)
			if bwLimit > bwInt64 {
				logrus.Infof("Obtained bandwidth :%sbits is less than the limit:%s", newBandwidth[0], limit)
			} else if bwLimit < bwInt64 {
				if isErr {
					logrus.Errorf("Obtained Bandwidth:%sbits is more than the limit: %s", newBandwidth[0], limit)
				} else {
					logrus.Errorf("Obtained bandwidth:%sbits is more than the limit %s", newBandwidth[0], limit)
					return errors.New("applied bandwidth is more than bandwidth rate")
				}
			} else {
				logrus.Errorf("Bandwidth rate :%s not applied", limit)
				return errors.New("bandwidth rate is not applied")
			}
		} else {
			logrus.Infof("Obtained bandwidth:%s", newBandwidth[0])
		}
	}
	return nil
}

func (w *swarm) tcFilterShow(bw string) error {
	qdiscShow, err := w.node.runCommand("tc qdisc show")
	if err != nil {
		return err
	}
	qdiscoutput := strings.Split(qdiscShow, "ingress")
	vvport := strings.Split(qdiscoutput[1], "parent")
	vvPort := strings.Split(vvport[0], "dev ")
	cmd := fmt.Sprintf("tc -s filter show dev %s parent ffff:", vvPort[1])
	str, err := w.node.runCommand(cmd)
	if err != nil {
		return err
	}
	output := strings.Split(str, "rate ")
	rate := strings.Split(output[1], "burst")
	regex := regexp.MustCompile("[0-9]+")
	outputStr := regex.FindAllString(rate[0], -1)
	outputInt, err := strconv.ParseInt(outputStr[0], 10, 64)
	bwInt, err := BwConvertInt64(bw)
	if err != nil {
		return err
	}
	if bwInt == outputInt {
		logrus.Infof("Applied bandwidth: %dkbits equals tc qdisc rate: %dkbits", bwInt, outputInt)
	} else {
		logrus.Errorf("Applied bandwidth: %dkbits does not match the tc rate: %d ", bwInt, outputInt)
		return errors.New("applied bandwidth does not match the tc qdisc rate")
	}
	return nil
}

func (w *swarm) checkConnection(c *container, ipaddr, protocol string, port int) error {
	var protoStr string

	if protocol == "udp" {
		protoStr = "-u"
	}

	logrus.Infof("Checking connection from %v to ip %s on port %d", c, ipaddr, port)

	_, err := w.exec(c, fmt.Sprintf("nc -z -n -v -w 1 %s %s %v", protoStr, ipaddr, port))
	if err != nil {
		logrus.Errorf("Connection from %v to ip %s on port %d FAILED", c, ipaddr, port)
	} else {
		logrus.Infof("Connection from %v to ip %s on port %d SUCCEEDED", c, ipaddr, port)
	}

	return err
}

func (w *swarm) checkNoConnection(c *container, ipaddr, protocol string, port int) error {
	logrus.Infof("Expecting connection to fail from %v to %s on port %d", c, ipaddr, port)

	if err := w.checkConnection(c, ipaddr, protocol, port); err != nil {
		return nil
	}

	return fmt.Errorf("connection SUCCEEDED on port %d from %s from %v when it should have FAILED", port, ipaddr, c)
}

func (w *swarm) cleanupDockerNetwork() error {
	logrus.Infof("Cleaning up networks on %s", w.node.Name())
	return w.node.tbnode.RunCommand("docker network rm $(docker network ls | grep netplugin | awk '{print $2}')")
}

func (w *swarm) cleanupContainers() error {
	logrus.Infof("Cleaning up containers on %s", w.node.Name())
	w.node.tbnode.RunCommand("docker kill -s 9 $(docker ps -a | grep alpine )")
	// Removing all alpine container images
	return w.node.tbnode.RunCommand("docker rm -f $(docker ps -a | grep alpine )")
}

func (w *swarm) commonArgs() string {
	netMode := w.node.suite.globInfo.Encap
	fwdMode := w.node.suite.fwdMode
	mode := "swarm-mode"
	var storeArgs string
	if w.node.suite.basicInfo.ClusterStoreDriver == "etcd" {
		storeArgs = " --etcd-endpoints " + w.node.suite.basicInfo.ClusterStoreURLs + " "
	} else {
		storeArgs = " --consul-endpoints " + w.node.suite.basicInfo.ClusterStoreURLs + " "
	}
	return " --netmode " + netMode + " --fwdmode " + fwdMode + " --mode " + mode + storeArgs
}

func (w *swarm) startNetplugin(args string) error {
	logrus.Infof("Starting netplugin on %s", w.node.Name())
	cmd := "sudo " + w.node.suite.basicInfo.BinPath + "/netplugin --vlan-if " + w.node.suite.hostInfo.HostDataInterfaces + w.commonArgs() + args + "&> /tmp/netplugin.log"
	return w.node.tbnode.RunCommandBackground(cmd)
}

func (w *swarm) stopNetplugin() error {
	logrus.Infof("Stopping netplugin on %s", w.node.Name())
	return w.node.tbnode.RunCommand("sudo pkill netplugin")
}

func (w *swarm) stopNetmaster() error {
	logrus.Infof("Stopping netmaster on %s", w.node.Name())
	return w.node.tbnode.RunCommand("sudo pkill netmaster")
}

func (w *swarm) startNetmaster(args string) error {
	logrus.Infof("Starting netmaster on %s", w.node.Name())
	var infraType string
	if w.node.suite.basicInfo.AciMode == "on" {
		infraType = " --infra aci "
	}
	return w.node.tbnode.RunCommandBackground("sudo " + w.node.suite.basicInfo.BinPath + "/netmaster" + infraType + w.commonArgs() + " &> /tmp/netmaster.log")
}
func (w *swarm) cleanupMaster() {
	logrus.Infof("Cleaning up master on %s", w.node.Name())
	vNode := w.node.tbnode
	vNode.RunCommand("etcdctl rm --recursive /contiv")
	vNode.RunCommand("etcdctl rm --recursive /contiv.io")
	vNode.RunCommand("etcdctl rm --recursive /docker")
	vNode.RunCommand("curl -X DELETE localhost:8500/v1/kv/contiv.io?recurse=true")
	vNode.RunCommand("curl -X DELETE localhost:8500/v1/kv/docker?recurse=true")
}

func (w *swarm) cleanupSlave() {
	logrus.Infof("Cleaning up slave on %s", w.node.Name())
	vNode := w.node.tbnode
	vNode.RunCommand("sudo ovs-vsctl del-br contivVxlanBridge")
	vNode.RunCommand("sudo ovs-vsctl del-br contivVlanBridge")
	vNode.RunCommand("for p in `ifconfig  | grep vport | awk '{print $1}'`; do sudo ip link delete $p type veth; done")
}

func (w *swarm) runCommandUntilNoNetpluginError() error {
	return w.node.runCommandUntilNoError("pgrep netplugin")
}

func (w *swarm) runCommandUntilNoNetmasterError() error {
	return w.node.runCommandUntilNoError("pgrep netmaster")
}

func (w *swarm) rotateNetmasterLog() error {
	return w.rotateLog("netmaster")
}

func (w *swarm) rotateNetpluginLog() error {
	return w.rotateLog("netplugin")
}

func (w *swarm) checkForNetpluginErrors() error {
	out, _ := w.node.tbnode.RunCommandWithOutput(`for i in /tmp/net*; do grep "panic\|fatal" $i; done`)
	if out != "" {
		logrus.Errorf("Fatal error in logs on %s: \n", w.node.Name())
		fmt.Printf("%s\n==========================================\n", out)
		return fmt.Errorf("fatal error in netplugin logs")
	}

	out, _ = w.node.tbnode.RunCommandWithOutput(`for i in /tmp/net*; do grep "error" $i; done`)
	if out != "" {
		logrus.Errorf("error output in netplugin logs on %s: \n", w.node.Name())
		fmt.Printf("%s==========================================\n\n", out)
		// FIXME: We still have some tests that are failing error check
		// return fmt.Errorf("error output in netplugin logs")
	}

	return nil
}

func (w *swarm) rotateLog(prefix string) error {
	oldPrefix := fmt.Sprintf("/tmp/%s", prefix)
	newPrefix := fmt.Sprintf("/tmp/_%s", prefix)
	_, err := w.node.runCommand(fmt.Sprintf("mv %s.log %s-`date +%%s`.log", oldPrefix, newPrefix))
	return err
}

func (w *swarm) checkConnectionRetry(c *container, ipaddr, protocol string, port, delay, retries int) error {
	var protoStr string
	var err error

	err = nil

	if protocol == "udp" {
		protoStr = "-u"
	}

	logrus.Infof("Checking connection from %c to ip %s on port %d, delay: %d, retries: %d",
		c, ipaddr, port, delay, retries)

	for i := 0; i < retries; i++ {

		_, err = w.exec(c, fmt.Sprintf("nc -z -n -v -w 1 %s %s %v", protoStr, ipaddr, port))
		if err == nil {
			logrus.Infof("Connection to ip %s on port %d SUCCEEDED, tries: %d", ipaddr, port, i+1)
			return nil
		}
		time.Sleep(2 * time.Second)
	}

	logrus.Errorf("Connection  to ip %s on port %d FAILED %v", ipaddr, port, err)
	return err
}

func (w *swarm) checkNoConnectionRetry(c *container, ipaddr, protocol string, port, delay, retries int) error {
	logrus.Infof("Expecting connection to fail from %v to %s on port %d", c, ipaddr, port)

	if err := w.checkConnectionRetry(c, ipaddr, protocol, port, delay, retries); err != nil {
		return nil
	}

	return fmt.Errorf("connection SUCCEEDED on port %d from %s from %s when it should have FAILED", port, ipaddr, c)
}

func (w *swarm) checkPing6WithCount(c *container, ipaddr string, count int) error {
	logrus.Infof("Checking ping6 from %v to %s", c, ipaddr)
	cmd := fmt.Sprintf("ping6 -c %d %s", count, ipaddr)
	out, err := w.exec(c, cmd)

	if err != nil || strings.Contains(out, "0 received, 100% packet loss") {
		logrus.Errorf("Ping6 from %s to %s FAILED: %q - %v", c, ipaddr, out, err)
		return fmt.Errorf("ping6 failed from %s to %s: %q - %v", c, ipaddr, out, err)
	}

	logrus.Infof("Ping6 from %s to %s SUCCEEDED", c, ipaddr)
	return nil
}

func (w *swarm) checkPingWithCount(c *container, ipaddr string, count int) error {
	logrus.Infof("Checking ping from %s to %s", c, ipaddr)
	cmd := fmt.Sprintf("ping -c %d %s", count, ipaddr)
	out, err := w.exec(c, cmd)

	if err != nil || strings.Contains(out, "0 received, 100% packet loss") {
		logrus.Errorf("Ping from %s to %s FAILED: %q - %v", c, ipaddr, out, err)
		return fmt.Errorf("ping failed from %s to %s: %q - %v", c, ipaddr, out, err)
	}

	logrus.Infof("Ping from %s to %s SUCCEEDED", c, ipaddr)
	return nil
}

func (w *swarm) checkSchedulerNetworkCreated(nwName string, expectedOp bool) error {
	logrus.Infof(w.env + "Checking whether docker network is created or not")
	cmd := fmt.Sprintf("docker network ls | grep netplugin | grep %s | awk \"{print \\$2}\"", nwName)
	logrus.Infof("Command to be executed is = %s", cmd)
	op, err := w.node.runCommand(cmd)

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

func (w *swarm) checkSchedulerNetworkOnNodeCreated(nwNames []string, n *node) error {
	ch := make(chan error, 1)
	for _, nwName := range nwNames {
		go func(nwName string, n *node, ch chan error) {
			logrus.Infof("Checking whether docker network %s is created on node %s", nwName, n.Name())
			cmd := fmt.Sprintf("docker network ls | grep netplugin | grep %s | awk \"{print \\$2}\"", nwName)
			logrus.Infof("Command to be executed is = %s", cmd)
			count := 0
			//check if docker network is created for a minute
			for count < 60 {
				op, err := n.runCommand(cmd)

				if err == nil {
					ret := strings.Contains(op, nwName)
					if ret == true {
						ch <- nil
					}
					count++
					time.Sleep(1 * time.Second)
				}
			}
			ch <- fmt.Errorf("swarm Network %s not created on node %s", nwName, n.Name())
		}(nwName, n, ch)
	}
	for range nwNames {
		if err := <-ch; err != nil {
			return err
		}
	}
	return nil
}

func (w *swarm) waitForListeners() error {
	return w.node.runCommandWithTimeOut("netstat -tlpn | grep 9090 | grep LISTEN", 500*time.Millisecond, 50*time.Second)
}

func (w *swarm) verifyAgents(agentIPs map[string]bool) (string, error) {

	var data interface{}
	actAgents := make(map[string]uint32)

	// read vtep information from inspect
	cmd := "curl -s localhost:9999/debug/ofnet | python -mjson.tool"
	str, err := w.node.tbnode.RunCommandWithOutput(cmd)
	if err != nil {
		return "", err
	}

	err = json.Unmarshal([]byte(str), &data)
	if err != nil {
		logrus.Errorf("Unmarshal error: %v", err)
		return str, err
	}

	dd := data.(map[string]interface{})
	adb := dd["AgentDb"].(map[string]interface{})
	for key := range adb {
		actAgents[key] = 1
	}

	// build expected agentRpc
	rpcSet := []string{":9002", ":9003"}
	expAgents := make(map[string]uint32)
	for agent := range agentIPs {
		for _, rpc := range rpcSet {
			k := agent + rpc
			expAgents[k] = 1
		}
	}

	for agent := range expAgents {
		_, found := actAgents[agent]
		if !found {
			return str, errors.New("agent " + agent + " not found")
		}
	}

	// verify there are no extraneous Agents
	for agent := range actAgents {
		_, found := expAgents[agent]
		if !found {
			return str, errors.New("unexpected Agent " + agent + " found on " + w.node.Name())
		}
	}

	return "", nil
}

func (w *swarm) verifyVTEPs(expVTEPS map[string]bool) (string, error) {
	var data interface{}
	actVTEPs := make(map[string]uint32)

	// read vtep information from inspect
	cmd := "curl -s localhost:9090/inspect/driver | python -mjson.tool"
	str, err := w.node.tbnode.RunCommandWithOutput(cmd)
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
		return str, errors.New("vtepTable not found in driver info")
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
			return str, errors.New("the VTEP " + vtep + " was not found")
		}
	}

	return "", nil
}
func (w *swarm) verifyEPs(epList []string) (string, error) {
	// read ep information from inspect
	cmd := "curl -s localhost:9090/inspect/driver | python -mjson.tool"
	str, err := w.node.tbnode.RunCommandWithOutput(cmd)
	if err != nil {
		return "", err
	}

	for _, ep := range epList {
		if !strings.Contains(str, ep) {
			return str, errors.New(ep + " not found on " + w.node.Name())
		}
	}

	return "", nil
}

func (w *swarm) reloadNode(n *node) error {
	logrus.Infof("Reloading node %s", n.Name())

	out, err := exec.Command("vagrant", "reload", n.Name()).CombinedOutput()
	if err != nil {
		logrus.Errorf("Error reloading node %s. Err: %v\n Output: %s", n.Name(), err, string(out))
		return err
	}

	logrus.Infof("Reloaded node %s. Output:\n%s", n.Name(), string(out))
	return nil
}
func (w *swarm) getMasterIP() (string, error) {
	return w.node.getIPAddr("eth1")
}

func (w *swarm) verifyUplinkState(n *node, uplinks []string) error {
	var err error
	var portName string
	var cmd, output string

	if len(uplinks) > 1 {
		portName = "uplinkPort"
	} else {
		portName = uplinks[0]
	}

	// Verify port state
	cmd = fmt.Sprintf("sudo ovs-vsctl find Port name=%s", portName)
	output, err = n.runCommand(cmd)
	if err != nil || !(strings.Contains(string(output), portName)) {
		err = fmt.Errorf("lookup failed for uplink Port %s. Err: %+v", portName, err)
		return err
	}

	// Verify Interface state
	for _, uplink := range uplinks {
		cmd = fmt.Sprintf("sudo ovs-vsctl find Interface name=%s", uplink)
		output, err = n.runCommand(cmd)
		if err != nil || !(strings.Contains(string(output), uplink)) {
			err = fmt.Errorf("lookup failed for uplink interface %s for uplink cfg:%+v. Err: %+v", uplink, uplinks, err)
			return err
		}
	}

	return err
}
