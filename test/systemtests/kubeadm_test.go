package systemtests

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"os"
	"time"
)

type kubePod struct {
	node *node
}

var k8sMaster *node

const (
	k8sMasterNode        = "k8master"
	netmasterRestartFile = "/tmp/restart_netmaster"
	netpluginRestartFile = "/tmp/restart_netplugin"
	netmasterLogLocation = "/var/log/contiv/netmaster.log"
	netpluginLogLocation = "/var/log/contiv//netplugin.log"
)

func (s *systemtestSuite) NewK8sPodExec(n *node) *kubePod {
	k8 := new(kubePod)
	k8.node = n

	if n.Name() == k8sMasterNode {
		k8sMaster = n
	}
	return k8
}

func (k *kubePod) isMaster() bool {
	return (k.node.Name() == k8sMasterNode)
}

func (k *kubePod) newContainer(node *node, containerID, name string, spec containerSpec) (*container, error) {
	cont := &container{
		node:        node,
		containerID: containerID,
		name:        name,
	}

	out, err := k8sMaster.exec.getIPAddr(cont, "eth0")

	if err != nil {
		return nil, err
	}
	cont.eth0.ip = out

	out, err = k8sMaster.exec.getIPv6Addr(cont, "eth0")
	if err == nil {
		cont.eth0.ipv6 = out
	}

	return cont, nil
}

func (k *kubePod) runContainer(spec containerSpec) (*container, error) {
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

	image = "--image=contiv/alpine "

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
	out, err := k8sMaster.tbnode.RunCommandWithOutput(cmd)
	if err != nil {
		logrus.Errorf("cmd %q failed: output below", cmd)
		logrus.Println(out)
		return nil, err
	}

	//find out the node where pod is deployed

	for i := 0; i < 50; i++ {
		time.Sleep(5 * time.Second)
		cmd = fmt.Sprintf("kubectl get pods -o wide | grep %s", spec.name)
		out, err = k8sMaster.tbnode.RunCommandWithOutput(cmd)
		if strings.Contains(out, "Running") {
			break
		}
	}

	podInfoStr := strings.TrimSpace(out)

	if podInfoStr == "" {
		errStr := "Error Scheduling the pod"
		logrus.Errorf(errStr)
		return nil, errors.New(errStr)
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
		logrus.Error(err)
		return nil, err
	}

	return cont, nil
}

func (k *kubePod) checkPingFailure(c *container, ipaddr string) error {
	logrus.Infof("Expecting ping failure from %v to %s", c, ipaddr)
	if err := k.checkPing(c, ipaddr); err == nil {
		return fmt.Errorf("Ping succeeded when expected to fail from %v to %s", c, ipaddr)
	}

	return nil
}

func (k *kubePod) checkPing(c *container, ipaddr string) error {
	logrus.Infof("Checking ping from %v to %s", c, ipaddr)
	out, err := k.exec(c.containerID, "ping -c 1 "+ipaddr)

	if err != nil || strings.Contains(out, "0 received, 100% packet loss") {
		errStr := fmt.Sprintf("Ping from %s to %s FAILED: %q - %v", c, ipaddr, out, err)
		logrus.Errorf(errStr)
		return errors.New(errStr)
	}

	logrus.Infof("Ping from %s to %s SUCCEEDED", c, ipaddr)
	return nil
}

func (k *kubePod) checkPing6Failure(c *container, ipaddr string) error {
	logrus.Infof("Expecting ping failure from %v to %s", c, ipaddr)
	if err := k.checkPing6(c, ipaddr); err == nil {
		return fmt.Errorf("Ping succeeded when expected to fail from %v to %s", c, ipaddr)
	}

	return nil
}

func (k *kubePod) checkPing6(c *container, ipaddr string) error {
	logrus.Infof("Checking ping6 from %v to %s", c, ipaddr)
	out, err := k.exec(c.containerID, "ping6 -c 1 "+ipaddr)

	if err != nil || strings.Contains(out, "0 received, 100% packet loss") {
		return fmt.Errorf("Ping failed from %v to %s: %q - %v", c, ipaddr, out, err)
	}

	logrus.Infof("Ping from %v to %s SUCCEEDED", c, ipaddr)
	return nil
}

func (k *kubePod) getIPAddr(c *container, dev string) (string, error) {
	out, err := k8sMaster.tbnode.RunCommandWithOutput(fmt.Sprintf("kubectl exec %s -- ip addr show dev %s | grep inet | head -1", c.containerID, dev))
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

func (k *kubePod) getIPv6Addr(c *container, dev string) (string, error) {
	out, err := k8sMaster.tbnode.RunCommandWithOutput(
		fmt.Sprintf("kubectl exec %s ip addr show dev %s | grep 'inet6.*scope.*global' | head -1",
			c.containerID, dev))
	if err != nil {
		logrus.Errorf("Failed to get IPv6 for container %q", c.containerID)
		logrus.Println(out)
	}

	parts := regexp.MustCompile(`\s+`).Split(strings.TrimSpace(out), -1)
	if len(parts) < 2 {
		return "", fmt.Errorf("Invalid output from container %q: %s", c.containerID, out)
	}

	parts = strings.Split(parts[1], "/")
	return strings.TrimSpace(parts[0]), err
}

func (k *kubePod) getMACAddr(c *container, dev string) (string, error) {
	out, err := k8sMaster.tbnode.RunCommandWithOutput(fmt.Sprintf("kubectl exec %s -- ip addr show dev %s | grep ether | head -1", c.containerID, dev))
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

/*
* exec is used to run a specific command using kubectl on the host
 */
func (k *kubePod) exec(podName, args string, ns ...string) (string, error) {
	namespace := "default"
	if len(ns) != 0 {
		namespace = ns[0]
	}
	cmd := `kubectl -n ` + namespace + ` exec ` + podName + ` -- ` + args
	logrus.Debugf("Exec: Running command -- %s", cmd)
	out, err := k8sMaster.runCommand(cmd)
	if err != nil {
		return out, err
	}

	return out, nil
}

/*
* execBG executes a background process on the node
 */
func (k *kubePod) execBG(podName, args string, ns ...string) {
	namespace := "default"
	if len(ns) != 0 {
		namespace = ns[0]
	}
	cmd := `kubectl -n ` + namespace + ` exec ` + podName + ` -- ` + args
	logrus.Debugf("ExecBG: Running command -- %s", cmd)
	k8sMaster.tbnode.RunCommandBackground(cmd)
}

/*
* podExec function is used to run a command typically multiple commands
* with pipes and redirections within the pod rather than the node
 */
func (k *kubePod) podExec(podName, args string, ns ...string) (string, error) {
	// NOTE:
	// Remotessh library wraps this command as follows:
	// ssh <hostip> <port> <env-var> bash -lc '<cmd>'
	//
	// Backticks and quotes here ensure that the command
	// is properly wrapped to execute on pod rather than on the node
	podCmd := `sh -c '\''` + args + `'\''`
	return k.exec(podName, podCmd, ns...)
}

/*
* podExecBG function is used to run a background command
* within the pod rather than the node
 */
func (k *kubePod) podExecBG(podName, args string, ns ...string) error {
	namespace := "default"
	if len(ns) != 0 {
		namespace = ns[0]
	}

	// NOTE:
	// Remotessh library wraps this command as follows:
	// ssh <hostip> <port> <env-var> bash -lc '<cmd>'
	//
	//  - Backticks and quotes here ensure that the command
	//    is properly wrapped to execute on pod rather than on the node
	//  - nohup along with & ensures that the process continues to live
	//    after the shell is terminated
	// Since the command is to run in BG in the pod and not on the node,
	// RunCommandBackground is not required here

	podCmd := `kubectl -n ` + namespace + ` exec ` + podName + ` -- nohup sh -c '\''` + args + ` &'\''`
	logrus.Debugf("Pod Exec BG: Running command -- %s", podCmd)
	return k8sMaster.tbnode.RunCommand(podCmd)
}

func (k *kubePod) start(c *container) error {
	//Kubernetes does not support start/stop
	return nil
}

func (k *kubePod) stop(c *container) error {
	//Kubernetes does not support start/stop
	return nil
}

func (k *kubePod) rm(c *container) error {
	logrus.Infof("Removing Pod: %s on %s", c.containerID, c.node.Name())
	k8sMaster.tbnode.RunCommand(fmt.Sprintf("kubectl delete pod %s", c.name))
	for i := 0; i < 80; i++ {
		out, _ := k8sMaster.tbnode.RunCommandWithOutput(fmt.Sprintf("kubectl get pod %s", c.containerID))
		if strings.Contains(out, "not found") {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("Error Termininating pod %s on node %s", c.name, c.node.Name())
}

func (k *kubePod) startListener(c *container, port int, protocol string) error {
	var protoStr string

	if protocol == "udp" {
		protoStr = "-u"
	}

	k.execBG(c.containerID, fmt.Sprintf("nc -lk %s -p %v -e /bin/true", protoStr, port))
	return nil

}

func (k *kubePod) startIperfServer(c *container) error {
	k.execBG(c.containerID, "iperf -s")
	return nil
}

func (k *kubePod) startIperfClient(c *container, ip, limit string, isErr bool) error {
	var (
		bwLimit int64
		bwInt64 int64
	)
	bw, err := k.exec(c.containerID, fmt.Sprintf("iperf -c %s ", ip))
	logrus.Infof("starting iperf client on: %v for server ip: %s", c, ip)
	if err != nil {
		logrus.Errorf("Error starting the iperf client")
	}
	if strings.Contains(bw, "bits/sec") {
		bwString := strings.Split(bw, "Bytes ")
		bwInt64, err = BwConvertInt64(bwString[1])
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
				logrus.Infof("Obtained bandwidth: %dkbits is less than the limit: %dkbits", bwInt64, bwLimit)
			} else if bwLimit < bwInt64 {
				if isErr {
					logrus.Errorf("Obtained Bandwidth: %s is more than the limit: %s", strings.TrimSpace(bwString[1]), limit)
				} else {
					logrus.Errorf("Obtained bandwidth: %s is more than the limit %s", bwString[1], limit)
					return errors.New("the applied bandwidth is more than bandwidth rate")
				}
			} else {
				errStr := fmt.Sprintf("Bandwidth rate: %s not applied", limit)
				logrus.Errorf(errStr)
				return errors.New(errStr)
			}
		} else {
			logrus.Infof("Obtained bandwidth: %s", bwString[1])
		}
	} else {
		logrus.Errorf("Bandwidth string invalid: %s", bw)
	}
	return err
}

func (k *kubePod) tcFilterShow(bw string) error {
	if k.isMaster() {
		return nil
	}
	qdiscShow, err := k.node.tbnode.RunCommandWithOutput("tc qdisc show")
	if err != nil {
		return err
	}
	qdiscoutput := strings.Split(qdiscShow, "ingress")
	vvport := strings.Split(qdiscoutput[1], "parent")
	vvPort := strings.Split(vvport[0], "dev ")
	cmd := fmt.Sprintf("tc -s filter show dev %s parent ffff:", vvPort[1])
	str, err := k.node.runCommand(cmd)
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
		errStr := fmt.Sprintf("Applied bandwidth: %dkbits does not match the tc rate: %d", bwInt, outputInt)
		logrus.Errorf(errStr)
		return errors.New(errStr)
	}
	return nil
}

func (k *kubePod) checkConnection(c *container, ipaddr, protocol string, port int) error {
	var protoStr string

	if protocol == "udp" {
		protoStr = "-u"
	}

	logrus.Infof("Checking connection from %s to ip %s on port %d", c, ipaddr, port)

	out, err := k.exec(c.containerID, fmt.Sprintf("nc -z -n -v -w 1 %s %s %v", protoStr, ipaddr, port))
	if err != nil && !strings.Contains(out, "open") {
		logrus.Errorf("Connection from %v to ip %s on port %d FAILED", *c, ipaddr, port)
	} else {
		logrus.Infof("Connection from %v to ip %s on port %d SUCCEEDED", *c, ipaddr, port)
	}

	return err
}

func (k *kubePod) checkNoConnection(c *container, ipaddr, protocol string, port int) error {
	logrus.Infof("Expecting connection to fail from %v to %s on port %d", c, ipaddr, port)

	if err := k.checkConnection(c, ipaddr, protocol, port); err != nil {
		return nil
	}
	return fmt.Errorf("the connection SUCCEEDED on port %d from %s from %v when it should have FAILED", port, ipaddr, c)
}

func (k *kubePod) cleanupContainers() error {
	if k.isMaster() {
		logrus.Infof("Cleaning up containers on %s", k.node.Name())
		k8sMaster.tbnode.RunCommand(fmt.Sprintf("kubectl delete pod --all"))
	}
	return nil
}

func (k *kubePod) commonArgs() string {
	netMode := k.node.suite.globInfo.Encap
	fwdMode := k.node.suite.fwdMode
	mode := "kubernetes"
	var storeArgs string
	if k.node.suite.basicInfo.ClusterStoreDriver == "etcd" {
		storeArgs = " --etcd-endpoints " + k.node.suite.basicInfo.ClusterStoreURLs + " "
	} else {
		storeArgs = " --consul-endpoints " + k.node.suite.basicInfo.ClusterStoreURLs + " "
	}
	return " --netmode " + netMode + " --fwdmode " + fwdMode + " --mode " + mode + storeArgs
}

func (k *kubePod) startNetplugin(args string) error {
	podName, err := getPodName("netplugin", k.node.Name())
	if err != nil {
		logrus.Errorf("pod not found: %+v", err)
		return err
	}

	logrus.Infof("Starting netplugin on %s", k.node.Name())
	copyContivK8SCmd := `cp /contiv/bin/contivk8s /opt/cni/bin/contivk8s`
	k.exec(podName, copyContivK8SCmd, "kube-system")

	startNetpluginCmd := (k.node.suite.basicInfo.BinPath + `/netplugin --vlan-if=` +
		k.node.suite.hostInfo.HostDataInterfaces + k.commonArgs() + args + ` > ` + netpluginLogLocation + ` 2>&1`)

	return k.podExecBG(podName, startNetpluginCmd, "kube-system")
}

func (k *kubePod) stopNetplugin() error {
	podName, err := getPodName("netplugin", k.node.Name())
	if err != nil {
		logrus.Errorf("pod not found: %+v", err)
		return err
	}

	stopRestartCmd := `rm ` + netpluginRestartFile
	k.exec(podName, stopRestartCmd, "kube-system")

	logrus.Infof("Stopping netplugin on %s", k.node.Name())
	killNetpluginCmd := `pkill netplugin`
	_, err = k.exec(podName, killNetpluginCmd, "kube-system")
	return err
}

func (k *kubePod) stopNetmaster() error {
	if !k.isMaster() {
		return nil
	}
	podName, err := getPodName("netmaster", k.node.Name())
	if err != nil {
		logrus.Errorf("pod not found: %+v", err)
		return err
	}

	stopRestartCmd := `rm ` + netmasterRestartFile
	k.exec(podName, stopRestartCmd, "kube-system")

	logrus.Infof("Stopping netmaster on %s", k.node.Name())
	killNetmasterCmd := `pkill netmaster`
	_, err = k.exec(podName, killNetmasterCmd, "kube-system")
	return err
}

func (k *kubePod) startNetmaster(args string) error {
	if !k.isMaster() {
		return nil
	}
	logrus.Infof("Starting netmaster on %s", k.node.Name())
	podName, err := getPodName("netmaster", k.node.Name())
	if err != nil {
		logrus.Errorf("pod not found: %+v", err)
		return err
	}
	var infraType string
	if k.node.suite.basicInfo.AciMode == "on" {
		infraType = " --infra aci "
	}
	netmasterStartCmd := (k.node.suite.basicInfo.BinPath + `/netmaster` +
		infraType + k.commonArgs() + args + ` > ` + netmasterLogLocation + ` 2>&1`)

	return k.podExecBG(podName, netmasterStartCmd, "kube-system")
}

func (k *kubePod) cleanupMaster() {
	if !k.isMaster() {
		return
	}
	logrus.Infof("Cleaning up master on %s", k8sMaster.Name())
	podName, err := getPodName("contiv-etcd", k.node.Name())
	if err != nil {
		logrus.Errorf("pod not found: %+v", err)
		return
	}
	// TODO: support multi urls
	etcdClient := k.node.suite.basicInfo.ClusterStoreURLs
	logrus.Infof("Cleaning out etcd info on %s", etcdClient)

	k.podExec(podName, `etcdctl -C `+etcdClient+` rm --recursive /contiv`, "kube-system")
	k.podExec(podName, `etcdctl -C `+etcdClient+` rm --recursive /contiv.io`, "kube-system")
	k.podExec(podName, `etcdctl -C `+etcdClient+` rm --recursive /docker`, "kube-system")
}

func getPodName(podRegex, nodeName string) (string, error) {
	var err error
	var podName string
	for retry := 60; retry > 0; retry-- {
		// only get running pods name
		podNameCmd := `kubectl -n kube-system get pods -o wide | grep Running | grep ` + podRegex + ` | grep ` + nodeName + ` | cut -d " " -f 1`
		podName, err = k8sMaster.tbnode.RunCommandWithOutput(podNameCmd)
		podName = strings.TrimSpace(podName)
		if err != nil || podName == "" {
			logrus.Warnf("Couldn't fetch pod %s info on %s, retry in 5 sec", podRegex, nodeName)
			time.Sleep(5 * time.Second)
			continue
		}
		break
	}
	if podName == "" {
		err = fmt.Errorf("Failed to find running pod %s on node %s", podRegex, nodeName)
	}
	return podName, err
}

func (k *kubePod) cleanupSlave() {

	logrus.Infof("Cleaning up slave on %s", k.node.Name())
	podName, err := getPodName("netplugin", k.node.Name())
	if err != nil {
		logrus.Errorf("pod not found: %+v", err)
		return
	}

	flowCleanupCmd := `ovs-vsctl list-br | grep contiv | xargs -rt -I % ovs-ofctl --protocols=OpenFlow13 del-flows %`
	_, err = k.podExec(podName, flowCleanupCmd, "kube-system")
	if err != nil {
		logrus.Errorf("ovs flow cleanup failed with err: %+v", err)
	}

	ovsCleanupCmd := `ovs-vsctl list-br | grep contiv | xargs -rt -I % ovs-vsctl del-br %`
	_, err = k.podExec(podName, ovsCleanupCmd, "kube-system")
	if err != nil {
		logrus.Errorf("ovs cleanup failed with err: %+v", err)
	}

	linkCleanupCmd := `ifconfig | grep vport | cut -d " " -f 1 | xargs -rt -I % ip link delete %`
	_, err = k.podExec(podName, linkCleanupCmd, "kube-system")
	if err != nil {
		logrus.Errorf("link cleanup failed with err: %+v", err)
	}
}

func (k *kubePod) runCommandUntilNoNetmasterError() error {
	if !k.isMaster() {
		return nil
	}
	logrus.Infof("Checking for netmaster status on node: %s", k.node.Name())
	podName, err := getPodName("netmaster", k.node.Name())
	if err != nil {
		logrus.Errorf("pod not found: %+v", err)
		return err
	}

	processCheckCmd := `kubectl -n kube-system exec ` + podName + ` -- pgrep netmaster`
	if err := k8sMaster.runCommandUntilNoError(processCheckCmd); err != nil {
		return err
	}
	return waitUntilAllPodsReady()
}

func waitUntilAllPodsReady() error {
	// ensure all pods are running
	var cliErr error
	var badPodNum string
	// wait up to 10 min
	for retry := 120; retry > 0; retry-- {
		// can't use --no-headers because it will have non-zero return code
		badPodNum, cliErr = k8sMaster.tbnode.RunCommandWithOutput(`kubectl -n kube-system  get pods -owide |grep -c -v Running`)
		if cliErr != nil {
			logrus.Warnf("Got error %q while fetching pod status, retry in 5 sec", cliErr.Error())
		} else if strings.TrimSpace(badPodNum) != "1" {
			logrus.Warnf("Found %q pods are not running, retry in 5 sec", strings.TrimSpace(badPodNum))
		} else {
			return nil
		}
		time.Sleep(5 * time.Second)
	}

	if cliErr != nil {
		return cliErr
	}
	return errors.New("Failed to wait all pods to running status")
}

func (k *kubePod) runCommandUntilNoNetpluginError() error {
	logrus.Infof("Checking for netplugin status on: %s", k.node.Name())
	podName, err := getPodName("netplugin", k.node.Name())
	if err != nil {
		logrus.Errorf("pod not found: %+v", err)
		return err
	}

	processCheckCmd := `kubectl -n kube-system exec ` + podName + ` -- pgrep netplugin`
	return k8sMaster.runCommandUntilNoError(processCheckCmd)
}

func (k *kubePod) rotateNetmasterLog() error {
	if k.isMaster() {
		return k.rotateLog("netmaster")
	}
	return nil
}

func (k *kubePod) rotateNetpluginLog() error {
	return k.rotateLog("netplugin")
}

func (k *kubePod) checkForNetpluginErrors() error {
	podName, err := getPodName("netplugin", k.node.Name())
	if err != nil {
		logrus.Errorf("pod not found: %+v", err)
		return err
	}

	// NOTE: Checking for error here could result in Error code: 123
	// Err code 123 might be the case when grep results in no output
	fatalCheckCmd := `ls /var/log/contiv/net* | xargs -r -I % grep --text -A 5 "panic\|fatal" %`
	out, _ := k.podExec(podName, fatalCheckCmd, "kube-system")
	if out != "" {
		errStr := fmt.Sprintf("fatal error in netplugin logs on %s\n", k.node.Name())
		logrus.Errorf(errStr)
		fmt.Printf("%s\n==========================================\n", out)
		return errors.New(errStr)
	}

	errCheckCmd := `ls /var/log/contiv/net* | xargs -r -I {} grep --text "error" {}`
	out, _ = k.podExec(podName, errCheckCmd, "kube-system")
	if out != "" {
		logrus.Errorf("error output in netplugin logs on %s: \n", k.node.Name())
		fmt.Printf("%s==========================================\n\n", out)
		// FIXME: We still have some tests that are failing error check
		// return fmt.Errorf("error output in netplugin logs")
	}

	return nil
}

func (k *kubePod) rotateLog(processName string) error {
	podName, err := getPodName(processName, k.node.Name())
	if err != nil {
		logrus.Errorf("pod not found: %+v", err)
		return err
	}

	oldLogFile := fmt.Sprintf("/var/log/contiv/%s.log", processName)
	newLogFilePrefix := fmt.Sprintf("/var/log/contiv/_%s", processName)
	rotateLogCmd := `echo` + " `date +%s` " + `| xargs -I {} mv ` + oldLogFile + ` ` + newLogFilePrefix + `-{}.log`
	_, err = k.podExec(podName, rotateLogCmd, "kube-system")
	return err
}

func (k *kubePod) checkConnectionRetry(c *container, ipaddr, protocol string, port, delay, retries int) error {
	var protoStr string
	var err error

	err = nil

	if protocol == "udp" {
		protoStr = "-u"
	}

	logrus.Infof("Checking connection from %s to ip %s on port %d, delay: %d, retries: %d",
		c, ipaddr, port, delay, retries)

	for i := 0; i < retries; i++ {

		_, err = k.exec(c.containerID, fmt.Sprintf("nc -z -n -v -w 1 %s %s %v", protoStr, ipaddr, port))
		if err == nil {
			logrus.Infof("Connection to ip %s on port %d SUCCEEDED, tries: %d", ipaddr, port, i+1)
			return nil
		}
		time.Sleep(2 * time.Second)
	}

	logrus.Errorf("Connection  to ip %s on port %d FAILED %v", ipaddr, port, err)
	return err
}

func (k *kubePod) checkNoConnectionRetry(c *container, ipaddr, protocol string, port, delay, retries int) error {
	logrus.Infof("Expecting connection to fail from %v to %s on port %d", c, ipaddr, port)

	if err := k.checkConnectionRetry(c, ipaddr, protocol, port, delay, retries); err != nil {
		return nil
	}

	return fmt.Errorf("the connection SUCCEEDED on port %d from %s from %s when it should have FAILED", port, ipaddr, c)
}

func (k *kubePod) checkPing6WithCount(c *container, ipaddr string, count int) error {
	logrus.Infof("Checking ping6 from %v to %s", c, ipaddr)
	cmd := fmt.Sprintf("ping6 -c %d %s", count, ipaddr)
	out, err := k.exec(c.containerID, cmd)

	if err != nil || strings.Contains(out, "0 received, 100% packet loss") {
		errStr := fmt.Sprintf("Ping6 from %s to %s FAILED: %q - %v", c, ipaddr, out, err)
		logrus.Errorf(errStr)
		return errors.New(errStr)
	}

	logrus.Infof("Ping6 from %s to %s SUCCEEDED", c, ipaddr)
	return nil
}

func (k *kubePod) checkPingWithCount(c *container, ipaddr string, count int) error {
	logrus.Infof("Checking ping from %s to %s", c, ipaddr)
	cmd := fmt.Sprintf("ping -c %d %s", count, ipaddr)
	out, err := k.exec(c.containerID, cmd)

	if err != nil || strings.Contains(out, "0 received, 100% packet loss") {
		errStr := fmt.Sprintf("Ping from %s to %s FAILED: %q - %v", c, ipaddr, out, err)
		logrus.Errorf(errStr)
		return errors.New(errStr)
	}

	logrus.Infof("Ping from %s to %s SUCCEEDED", c, ipaddr)
	return nil
}
func (k *kubePod) checkSchedulerNetworkCreated(nwName string, expectedOp bool) error {
	return nil
}

func (k *kubePod) checkSchedulerNetworkOnNodeCreated(nwName []string, n *node) error {
	return nil
}

func (k *kubePod) waitForListeners() error {
	return k.node.runCommandWithTimeOut("netstat -tlpn | grep 9090 | grep LISTEN", 500*time.Millisecond, 50*time.Second)
}

func (k *kubePod) verifyAgents(agentIPs map[string]bool) (string, error) {
	if !k.isMaster() {
		return "", nil
	}

	var data interface{}
	actAgents := make(map[string]uint32)

	// read vtep information from inspect
	cmd := "curl -s localhost:9999/debug/ofnet | python -mjson.tool"
	str, err := k.node.tbnode.RunCommandWithOutput(cmd)
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
			return str, errors.New("Agent " + agent + " not found")
		}
	}

	// verify there are no extraneous Agents
	for agent := range actAgents {
		_, found := expAgents[agent]
		if !found {
			return str, errors.New("Unexpected Agent " + agent + " found on " + k.node.Name())
		}
	}

	return "", nil
}

func (k *kubePod) verifyVTEPs(expVTEPS map[string]bool) (string, error) {
	if k.isMaster() {
		return "", nil
	}
	var data interface{}
	actVTEPs := make(map[string]uint32)

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
		errStr := fmt.Sprintf("vxlan not found in driver info")
		logrus.Errorf(errStr)
		return str, errors.New(errStr)
	}

	vt := vx.(map[string]interface{})
	v, found := vt["VtepTable"]
	if !found {
		errStr := fmt.Sprintf("VtepTable not found in driver info")
		logrus.Errorf(errStr)
		return str, errors.New(errStr)
	}

	vteps := v.(map[string]interface{})
	for key := range vteps {
		actVTEPs[key] = 1
	}

	// read local ip
	localVtep := ""
	l, found := vt["LocalIp"]
	if found {
		switch l.(type) {
		case string:
			localVtep = l.(string)
			actVTEPs[localVtep] = 1
		}
	}

	for vtep := range expVTEPS {
		_, found := actVTEPs[vtep]
		if !found {
			return str, errors.New("VTEP " + vtep + " not found")
		}
	}

	// verify there are no extraneous VTEPs
	for vtep := range actVTEPs {
		_, found := expVTEPS[vtep]
		if !found {
			return str, errors.New("Unexpected VTEP " + vtep + " found on " + localVtep)
		}
	}

	return "", nil
}

func (k *kubePod) verifyEPs(epList []string) (string, error) {
	if k.isMaster() {
		return "", nil
	}
	// read ep information from inspect
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
func (k *kubePod) reloadNode(n *node) error {

	if n.Name() == k8sMasterNode {
		return nil
	}

	logrus.Infof("Reloading node %s", n.Name())

	topDir := os.Getenv("GOPATH")
	topDir = strings.Split(topDir, ":")[1]
	cmd := exec.Command("vagrant", "reload", n.Name())
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "VAGRANT_CWD="+topDir+"/src/github.com/contiv/netplugin/vagrant/k8s/")
	cmd.Env = append(cmd.Env, "CONTIV_K8S_USE_KUBEADM=1")
	out, err := cmd.CombinedOutput()

	if err != nil {
		logrus.Errorf("Error reloading node %s. Err: %v\n Output: %s", n.Name(), err, string(out))
		return err
	}

	logrus.Infof("Reloaded node %s. Output:\n%s", n.Name(), string(out))
	return nil
}

func (k *kubePod) getMasterIP() (string, error) {
	return k8sMaster.getIPAddr("eth1")
}

func (k *kubePod) verifyUplinkState(n *node, uplinks []string) error {
	var err error
	var portName string
	var cmd, output string

	if n.Name() == k8sMasterNode {
		return nil
	}

	if len(uplinks) > 1 {
		portName = "uplinkPort"
	} else {
		portName = uplinks[0]
	}

	// Verify port state
	cmd = fmt.Sprintf("sudo ovs-vsctl find Port name=%s", portName)
	output, err = n.runCommand(cmd)
	if err != nil || !(strings.Contains(string(output), portName)) {
		err = fmt.Errorf("Lookup failed for uplink Port %s. Err: %+v", portName, err)
		return err
	}

	// Verify Interface state
	for _, uplink := range uplinks {
		cmd = fmt.Sprintf("sudo ovs-vsctl find Interface name=%s", uplink)
		output, err = n.runCommand(cmd)
		if err != nil || !(strings.Contains(string(output), uplink)) {
			err = fmt.Errorf("Lookup failed for uplink interface %s for uplink cfg:%+v. Err: %+v", uplink, uplinks, err)
			return err
		}
	}

	return err
}
