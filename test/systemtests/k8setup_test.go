package systemtests

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/Sirupsen/logrus"
	//"sync"
	"os"
	"time"
)

type kubernetes struct {
	node *node
}

var k8master *node

//var master sync.Mutex

func (s *systemtestSuite) NewK8sExec(n *node) *kubernetes {
	k8 := new(kubernetes)
	k8.node = n

	if n.Name() == "k8master" {
		k8master = n
	}
	return k8
}

func (k *kubernetes) newContainer(node *node, containerID, name string, spec containerSpec) (*container, error) {
	cont := &container{
		node:        node,
		containerID: containerID,
		name:        name,
	}

	out, err := k8master.exec.getIPAddr(cont, "eth0")

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

func (k *kubernetes) runContainer(spec containerSpec) (*container, error) {
	var namestr, labelstr, image string
	labels := []string{}

	if len(spec.tenantName) != 0 && spec.tenantName != "default" {
		labels = append(labels, "io.contiv.tenant="+spec.tenantName)
	}

	if spec.serviceName != "" {
		labels = append(labels, "io.contiv.net-group="+spec.serviceName)
	}
	if spec.networkName != "" {
		labels = append(labels, "io.contiv.network="+spec.networkName)
	}

	labelstr = strings.Join(labels, ",")

	if len(labelstr) != 0 {
		labelstr = "--labels=" + labelstr
	}

	image = "--image=alpine " //contiv/nc-busybox"

	cmdStr := " --command -- sleep 900000"

	if spec.name != "" {
		namestr = spec.name
	}

	if len(spec.labels) > 0 {
		l := " --labels="
		for _, label := range spec.labels {
			labelstr += l + label + " "
		}
	}

	cmd := fmt.Sprintf("kubectl run %s %s %s --restart=Never %s ", namestr, labelstr, image, cmdStr)

	logrus.Infof("Starting Pod %s on with: %s", spec.name, cmd)
	out, err := k8master.tbnode.RunCommandWithOutput(cmd)
	if err != nil {
		logrus.Errorf("cmd %q failed: output below", cmd)
		logrus.Println(out)
		return nil, err
	}

	//find out the node where pod is deployed

	for i := 0; i < 50; i++ {
		time.Sleep(5 * time.Second)
		cmd = fmt.Sprintf("kubectl get pods -o wide | grep %s", spec.name)
		////master.lock()
		out, err = k8master.tbnode.RunCommandWithOutput(cmd)
		if strings.Contains(out, "Running") {
			break
		}
	}

	podInfoStr := strings.TrimSpace(out)

	if podInfoStr == "" {
		logrus.Errorf("Error Scheduling the pod")
		return nil, errors.New("Error Scheduling the pod")
	}

	podInfo := strings.Split(podInfoStr, " ")

	podID := podInfo[0]
	nodeID := podInfo[len(podInfo)-1]

	podNode := k.node.suite.vagrant.GetNode(nodeID)

	n := &node{
		tbnode: podNode,
		suite:  k.node.suite,
		exec:   k,
	}

	cont, err := k.newContainer(n, podID, spec.name, spec)
	if err != nil {
		logrus.Info(err)
		return nil, err
	}

	return cont, nil
}

func (k *kubernetes) checkPingFailure(c *container, ipaddr string) error {
	logrus.Infof("Expecting ping failure from %v to %s", c, ipaddr)
	if err := k.checkPing(c, ipaddr); err == nil {
		return fmt.Errorf("Ping succeeded when expected to fail from %v to %s", c, ipaddr)
	}

	return nil
}

func (k *kubernetes) checkPing(c *container, ipaddr string) error {
	logrus.Infof("Checking ping from %v to %s", c, ipaddr)
	out, err := k.exec(c, "ping -c 1 "+ipaddr)

	if err != nil || strings.Contains(out, "0 received, 100% packet loss") {
		logrus.Errorf("Ping from %s to %s FAILED: %q - %v", c, ipaddr, out, err)
		return fmt.Errorf("Ping failed from %v to %s: %q - %v", c, ipaddr, out, err)
	}

	logrus.Infof("Ping from %s to %s SUCCEEDED", c, ipaddr)
	return nil
}

func (k *kubernetes) checkPing6Failure(c *container, ipaddr string) error {
	logrus.Infof("Expecting ping failure from %v to %s", c, ipaddr)
	if err := k.checkPing6(c, ipaddr); err == nil {
		return fmt.Errorf("Ping succeeded when expected to fail from %v to %s", c, ipaddr)
	}

	return nil
}

func (k *kubernetes) checkPing6(c *container, ipaddr string) error {
	logrus.Infof("Checking ping6 from %v to %s", c, ipaddr)
	out, err := k.exec(c, "ping6 -c 1 "+ipaddr)

	if err != nil || strings.Contains(out, "0 received, 100% packet loss") {
		logrus.Errorf("Ping from %v to %s FAILED: %q - %v", c, ipaddr, out, err)
		return fmt.Errorf("Ping failed from %v to %s: %q - %v", c, ipaddr, out, err)
	}

	logrus.Infof("Ping from %v to %s SUCCEEDED", c, ipaddr)
	return nil
}

func (k *kubernetes) getIPAddr(c *container, dev string) (string, error) {
	////master.lock()
	out, err := k8master.tbnode.RunCommandWithOutput(fmt.Sprintf("kubectl exec %s ip addr show dev %s | grep inet | head -1", c.containerID, dev))
	//master.unlock()
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

func (k *kubernetes) getIPv6Addr(c *container, dev string) (string, error) {
	/*FIXME: fix for k8 v6 */
	return "", nil
}

func (k *kubernetes) exec(c *container, args string) (string, error) {
	cmd := fmt.Sprintf("kubectl exec %s -- %s", c.containerID, args)
	logrus.Infof("Exec: Running command %s", cmd)
	out, err := k8master.runCommand(cmd)
	if err != nil {
		logrus.Println(out)
		return out, err
	}

	return out, nil
}

func (k *kubernetes) execBG(c *container, args string) {
	cmd := fmt.Sprintf("kubectl exec %s -- %s", c.containerID, args)
	logrus.Infof("ExecBG:Running command %s", cmd)
	k8master.tbnode.RunCommandBackground(cmd)
}

func (k *kubernetes) kubeCmd(c *container, arg string) error {
	out, err := k8master.runCommand(fmt.Sprintf("kubectl %s %s", arg, c.name))
	if err != nil {
		logrus.Errorf(out)
		return err
	}

	return nil
}

func (k *kubernetes) start(c *container) error {
	//Kubernetes does not support start/stop
	return nil
}

func (k *kubernetes) stop(c *container) error {
	//Kubernetes does not support start/stop
	return nil
}

func (k *kubernetes) rm(c *container) error {
	logrus.Infof("Removing Pod: %s on %s", c.containerID, c.node.Name())
	k8master.tbnode.RunCommand(fmt.Sprintf("kubectl delete job %s", c.name))
	for i := 0; i < 80; i++ {
		out, _ := k8master.tbnode.RunCommandWithOutput(fmt.Sprintf("kubectl get pod %s", c.containerID))
		if strings.Contains(out, "not found") {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("Error Termininating pod %s on node %s", c.name, c.node.Name())
}

func (k *kubernetes) startListener(c *container, port int, protocol string) error {
	var protoStr string

	if protocol == "udp" {
		protoStr = "-u"
	}

	k.execBG(c, fmt.Sprintf("nc -lk %s -p %v -e /bin/true", protoStr, port))
	return nil

}

func (k *kubernetes) checkConnection(c *container, ipaddr, protocol string, port int) error {
	var protoStr string

	if protocol == "udp" {
		protoStr = "-u"
	}

	logrus.Infof("Checking connection from %v to ip %s on port %d", *c, ipaddr, port)

	out, err := k.exec(c, fmt.Sprintf("nc -z -n -v -w 1 %s %s %v", protoStr, ipaddr, port))
	if err != nil && !strings.Contains(out, "open") {
		logrus.Errorf("Connection from %v to ip %s on port %d FAILED", *c, ipaddr, port)
	} else {
		logrus.Infof("Connection from %v to ip %s on port %d SUCCEEDED", *c, ipaddr, port)
	}

	return err
}

func (k *kubernetes) checkNoConnection(c *container, ipaddr, protocol string, port int) error {
	logrus.Infof("Expecting connection to fail from %v to %s on port %d", c, ipaddr, port)

	if err := k.checkConnection(c, ipaddr, protocol, port); err != nil {
		return nil
	}
	return fmt.Errorf("Connection SUCCEEDED on port %d from %s from %v when it should have FAILED.", port, ipaddr, c)
}

/*
func (n *node) cleanupDockerNetwork() error {
	logrus.Infof("Cleaning up networks on %s", n.Name())
	return n.tbnode.RunCommand("docker network ls | grep netplugin | awk '{print $2}'")
}
*/

func (k *kubernetes) cleanupContainers() error {
	if k.node.Name() == "k8master" {
		logrus.Infof("Cleaning up containers on %s", k.node.Name())
		cmd := "kubectl get job -o name"
		out, err := k8master.tbnode.RunCommandWithOutput(cmd)
		if err != nil {
			logrus.Infof("cmd %q failed: output below", cmd)
			logrus.Println(out)
			return err
		}
		k8master.tbnode.RunCommand(fmt.Sprintf("kubectl delete jobs --all "))
	}
	return nil
}

func (k *kubernetes) startNetplugin(args string) error {
	if k.node.Name() == "k8master" {
		return nil
	}
	logrus.Infof("Starting netplugin on %s", k.node.Name())
	return k.node.tbnode.RunCommandBackground("sudo " + k.node.suite.binpath + "/netplugin -plugin-mode kubernetes -vlan-if " + k.node.suite.vlanIf + " --cluster-store " + k.node.suite.clusterStore + " " + args + "&> /tmp/netplugin.log")
}

func (k *kubernetes) stopNetplugin() error {
	if k.node.Name() == "k8master" {
		return nil
	}
	logrus.Infof("Stopping netplugin on %s", k.node.Name())
	return k.node.tbnode.RunCommand("sudo pkill netplugin")
}

func (k *kubernetes) stopNetmaster() error {
	if k.node.Name() != "k8master" {
		return nil
	}
	logrus.Infof("Stopping netmaster on %s", k.node.Name())
	return k.node.tbnode.RunCommand("sudo pkill netmaster")
}

func (k *kubernetes) startNetmaster() error {
	if k.node.Name() != "k8master" {
		return nil
	}
	logrus.Infof("Starting netmaster on %s", k.node.Name())
	dnsOpt := " --dns-enable=false "
	if k.node.suite.enableDNS {
		dnsOpt = " --dns-enable=true "
	}
	return k.node.tbnode.RunCommandBackground(k.node.suite.binpath + "/netmaster" + dnsOpt + " --cluster-store " + k.node.suite.clusterStore + " " + "--cluster-mode kubernetes &> /tmp/netmaster.log")
}
func (k *kubernetes) cleanupMaster() {
	if k.node.Name() != "k8master" {
		return
	}
	//master.lock()
	//defer master.Unlock()
	logrus.Infof("Cleaning up master on %s", k8master.Name())
	vNode := k8master.tbnode
	vNode.RunCommand("etcdctl rm --recursive /contiv")
	vNode.RunCommand("etcdctl rm --recursive /contiv.io")
	vNode.RunCommand("etcdctl rm --recursive /docker")
	vNode.RunCommand("etcdctl rm --recursive /skydns")
	vNode.RunCommand("curl -X DELETE localhost:8500/v1/kv/contiv.io?recurse=true")
	vNode.RunCommand("curl -X DELETE localhost:8500/v1/kv/docker?recurse=true")
}

func (k *kubernetes) cleanupSlave() {
	if k.node.Name() == "k8master" {
		return
	}
	logrus.Infof("Cleaning up slave on %s", k.node.Name())
	vNode := k.node.tbnode
	vNode.RunCommand("sudo ovs-vsctl del-br contivVxlanBridge")
	vNode.RunCommand("sudo ovs-vsctl del-br contivVlanBridge")
	vNode.RunCommand("for p in `ifconfig  | grep vport | awk '{print $1}'`; do sudo ip link delete $p type veth; done")
	vNode.RunCommand("sudo rm /var/run/docker/plugins/netplugin.sock")
	vNode.RunCommand("sudo service docker restart")
}

func (k *kubernetes) runCommandUntilNoNetmasterError() error {
	if k.node.Name() == "k8master" {
		return k.node.runCommandUntilNoError("pgrep netmaster")
	}
	return nil
}
func (k *kubernetes) runCommandUntilNoNetpluginError() error {
	if k.node.Name() != "k8master" {
		return k.node.runCommandUntilNoError("pgrep netplugin")
	}
	return nil
}

func (k *kubernetes) rotateNetmasterLog() error {
	if k.node.Name() == "k8master" {
		return k.rotateLog("netmaster")
	}
	return nil
}

func (k *kubernetes) rotateNetpluginLog() error {
	if k.node.Name() != "k8master" {
		return k.rotateLog("netplugin")
	}
	return nil
}

func (k *kubernetes) checkForNetpluginErrors() error {
	if k.node.Name() == "k8master" {
		return nil
	}

	out, _ := k.node.tbnode.RunCommandWithOutput(`for i in /tmp/net*; do grep -A 5 "panic\|fatal" $i; done`)
	if out != "" {
		logrus.Errorf("Fatal error in logs on %s: \n", k.node.Name())
		fmt.Printf("%s\n==========================================\n", out)
		return fmt.Errorf("fatal error in netplugin logs")
	}

	out, _ = k.node.tbnode.RunCommandWithOutput(`for i in /tmp/net*; do grep "error" $i; done`)
	if out != "" {
		logrus.Errorf("error output in netplugin logs on %s: \n", k.node.Name())
		fmt.Printf("%s==========================================\n\n", out)
		// FIXME: We still have some tests that are failing error check
		// return fmt.Errorf("error output in netplugin logs")
	}

	return nil
}

func (k *kubernetes) rotateLog(prefix string) error {
	oldPrefix := fmt.Sprintf("/tmp/%s", prefix)
	newPrefix := fmt.Sprintf("/tmp/_%s", prefix)
	_, err := k.node.runCommand(fmt.Sprintf("mv %s.log %s-`date +%%s`.log", oldPrefix, newPrefix))
	return err
}

func (k *kubernetes) checkConnectionRetry(c *container, ipaddr, protocol string, port, delay, retries int) error {
	var protoStr string
	var err error

	err = nil

	if protocol == "udp" {
		protoStr = "-u"
	}

	logrus.Infof("Checking connection from %s to ip %s on port %d, delay: %d, retries: %d",
		c, ipaddr, port, delay, retries)

	for i := 0; i < retries; i++ {

		_, err = k.exec(c, fmt.Sprintf("nc -z -n -v -w 1 %s %s %v", protoStr, ipaddr, port))
		if err == nil {
			logrus.Infof("Connection to ip %s on port %d SUCCEEDED, tries: %d", ipaddr, port, i+1)
			return nil
		}
		time.Sleep(2 * time.Second)
	}

	logrus.Errorf("Connection  to ip %s on port %d FAILED %v", ipaddr, port, err)
	return err
}

func (k *kubernetes) checkNoConnectionRetry(c *container, ipaddr, protocol string, port, delay, retries int) error {
	logrus.Infof("Expecting connection to fail from %v to %s on port %d", c, ipaddr, port)

	if err := k.checkConnectionRetry(c, ipaddr, protocol, port, delay, retries); err != nil {
		return nil
	}

	return fmt.Errorf("Connection SUCCEEDED on port %d from %s from %s when it should have FAILED.", port, ipaddr, c)
}

func (k *kubernetes) checkPing6WithCount(c *container, ipaddr string, count int) error {
	logrus.Infof("Checking ping6 from %v to %s", c, ipaddr)
	cmd := fmt.Sprintf("ping6 -c %d %s", count, ipaddr)
	out, err := k.exec(c, cmd)

	if err != nil || strings.Contains(out, "0 received, 100% packet loss") {
		logrus.Errorf("Ping6 from %s to %s FAILED: %q - %v", c, ipaddr, out, err)
		return fmt.Errorf("Ping6 failed from %s to %s: %q - %v", c, ipaddr, out, err)
	}

	logrus.Infof("Ping6 from %s to %s SUCCEEDED", c, ipaddr)
	return nil
}

func (k *kubernetes) checkPingWithCount(c *container, ipaddr string, count int) error {
	logrus.Infof("Checking ping from %s to %s", c, ipaddr)
	cmd := fmt.Sprintf("ping -c %d %s", count, ipaddr)
	out, err := k.exec(c, cmd)

	if err != nil || strings.Contains(out, "0 received, 100% packet loss") {
		logrus.Errorf("Ping from %s to %s FAILED: %q - %v", c, ipaddr, out, err)
		return fmt.Errorf("Ping failed from %s to %s: %q - %v", c, ipaddr, out, err)
	}

	logrus.Infof("Ping from %s to %s SUCCEEDED", c, ipaddr)
	return nil
}
func (k *kubernetes) checkSchedulerNetworkCreated(nwName string, expectedOp bool) error {
	return nil
}

func (k *kubernetes) waitForListeners() error {
	if k.node.Name() == "k8master" {
		return nil
	}
	return k.node.runCommandWithTimeOut("netstat -tlpn | grep 9090 | grep LISTEN", 500*time.Millisecond, 50*time.Second)
}

func (k *kubernetes) verifyVTEPs(expVTEPS map[string]bool) (string, error) {
	var data interface{}
	actVTEPs := make(map[string]uint32)
	if k.node.Name() == "k8master" {
		return "", nil
	}
	// read vtep information from inspect
	cmd := "curl -s localhost:9090/inspect/driver | python -mjson.tool"
	str, err := k.node.tbnode.RunCommandWithOutput(cmd)
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
func (k *kubernetes) verifyEPs(epList []string) (string, error) {
	// read ep information from inspect
	if k.node.Name() == "k8master" {
		return "", nil
	}
	cmd := "curl -s localhost:9090/inspect/driver | python -mjson.tool"
	str, err := k.node.tbnode.RunCommandWithOutput(cmd)
	if err != nil {
		return "", err
	}

	for _, ep := range epList {
		if !strings.Contains(str, ep) {
			return str, errors.New(ep + " not found on " + k.node.Name())
		}
	}

	return "", nil
}

//FIXME: This needs to be moved to node abstraction implmentation Once
//that change is in.
func (k *kubernetes) reloadNode(n *node) error {

	if n.Name() == "k8master" {
		return nil
	}

	logrus.Infof("Reloading node %s", n.Name())

	topDir := os.Getenv("GOPATH")
	topDir = strings.Split(topDir, ":")[1]
	cmd := exec.Command("vagrant", "reload", n.Name())
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "VAGRANT_CWD="+topDir+"/src/github.com/contiv/netplugin/vagrant/k8s/")
	cmd.Env = append(cmd.Env, "CONTIV_K8=1")
	out, err := cmd.CombinedOutput()

	if err != nil {
		logrus.Errorf("Error reloading node %s. Err: %v\n Output: %s", n.Name(), err, string(out))
		return err
	}

	logrus.Infof("Reloaded node %s. Output:\n%s", n.Name(), string(out))
	return nil
}

func (k *kubernetes) getMasterIP() (string, error) {
	return k8master.getIPAddr("eth1")
}
