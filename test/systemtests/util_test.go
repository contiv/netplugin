package systemtests

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	. "github.com/contiv/check"
	"github.com/contiv/netplugin/contivmodel/client"
	"github.com/contiv/remotessh"
)

func (s *systemtestSuite) parallelExec(fn func(*node) error) []error {
	errChan := make(chan map[int]error, len(s.nodes))
	nodeError := "encountered error on node %v: %v"
	runner := func(n *node, i int, wg *sync.WaitGroup) {
		defer wg.Done()
		err := fn(n)
		if err != nil {
			err = fmt.Errorf(nodeError, n.Name(), err)
		}
		errChan <- map[int]error{i: err}
	}
	var wg sync.WaitGroup
	for i, n := range s.nodes {
		wg.Add(1)
		go runner(n, i, &wg)
	}
	wg.Wait()

	errors := map[int]error{}
	for i := 0; i < len(s.nodes); i++ {
		res := <-errChan
		for k, v := range res {
			if v != nil {
				errors[k] = v
			}
		}
	}

	finalErrors := make([]error, len(errors))
	for i := range s.nodes {
		err, ok := errors[i]
		if !ok {
			finalErrors = append(finalErrors, err)
		}
	}

	return finalErrors
}

func (s *systemtestSuite) checkConnectionPair(containers1, containers2 []*container, port int) error {
	for _, cont := range containers1 {
		for _, cont2 := range containers2 {
			if err := cont.node.exec.checkConnection(cont, cont2.eth0.ip, "tcp", port); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *systemtestSuite) checkConnectionPairRetry(containers1, containers2 []*container, port, delay, retries int) error {
	for _, cont := range containers1 {
		for _, cont2 := range containers2 {
			if err := cont.node.exec.checkConnectionRetry(cont, cont2.eth0.ip, "tcp", port, delay, retries); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *systemtestSuite) checkNoConnectionPairRetry(containers1, containers2 []*container, port, delay, retries int) error {
	for _, cont := range containers1 {
		for _, cont2 := range containers2 {
			if err := cont.node.exec.checkNoConnectionRetry(cont, cont2.eth0.ip, "tcp", port, delay, retries); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *systemtestSuite) runContainersInGroups(num int, netName string, tenantName string, groupNames []string) (map[*container]string, error) {
	containers := map[*container]string{}
	for _, groupName := range groupNames {
		names := []string{}

		for i := 0; i < num; i++ {
			names = append(names, fmt.Sprintf("grp-%s-%d", groupName, i))
		}

		// XXX we don't use anything but private for this function right now
		conts, err := s.runContainersInService(num, groupName, netName, tenantName, names)
		if err != nil {
			return nil, err
		}

		for _, cont := range conts {
			containers[cont] = groupName
		}
	}

	return containers, nil
}

func (s *systemtestSuite) runContainersInService(num int, serviceName, networkName string, tenantName string, names []string) ([]*container, error) {
	containers := []*container{}
	mutex := sync.Mutex{}

	if networkName == "" {
		networkName = "private"
	}

	errChan := make(chan error)

	for i := 0; i < num; i++ {
		go func(i int) {
			nodeNum := i % len(s.nodes)
			var name string

			mutex.Lock()
			if len(names) > 0 {
				name = names[0]
				names = names[1:]
			}
			mutex.Unlock()

			if name == "" {
				name = fmt.Sprintf("%s-srv%d-%d", strings.Replace(networkName, "/", "-", -1), i, nodeNum)
			}

			spec := containerSpec{
				imageName:   "contiv/alpine",
				networkName: networkName,
				name:        name,
				serviceName: serviceName,
				tenantName:  tenantName,
			}

			cont, err := s.nodes[nodeNum].exec.runContainer(spec)
			if err != nil {
				errChan <- err
			}

			mutex.Lock()
			containers = append(containers, cont)
			mutex.Unlock()

			errChan <- nil
		}(i)
	}

	for i := 0; i < num; i++ {
		if err := <-errChan; err != nil {
			return nil, err
		}
	}

	return containers, nil
}

func (s *systemtestSuite) runContainers(num int, withService bool, networkName string, tenantName string,
	names []string, labels []string) ([]*container, error) {
	containers := []*container{}
	mutex := sync.Mutex{}

	if networkName == "" {
		networkName = "private"
	}

	errChan := make(chan error)

	for i := 0; i < num; i++ {
		go func(i int) {
			nodeNum := i % len(s.nodes)
			var name string

			mutex.Lock()
			if len(names) > 0 {
				name = names[0]
				names = names[1:]
			}
			mutex.Unlock()

			if name == "" {
				name = fmt.Sprintf("%s-srv%d-%d", strings.Replace(networkName, "/", "-", -1), i, nodeNum)
			}

			var serviceName string

			if withService {
				serviceName = name
			}

			cname := fmt.Sprintf("%s-%d", name, i)
			spec := containerSpec{
				imageName:   "contiv/alpine",
				networkName: networkName,
				name:        cname,
				serviceName: serviceName,
				tenantName:  tenantName,
			}
			if len(labels) > 0 {
				spec.labels = append(spec.labels, labels...)
			}

			cont, err := s.nodes[nodeNum].exec.runContainer(spec)
			if err != nil {
				errChan <- err
			}

			mutex.Lock()
			containers = append(containers, cont)
			mutex.Unlock()

			errChan <- nil
		}(i)
	}

	for i := 0; i < num; i++ {
		if err := <-errChan; err != nil {
			return nil, err
		}
	}

	return containers, nil
}

func (s *systemtestSuite) runContainersSerial(num int, withService bool, networkName string, tenantName string, names []string) ([]*container, error) {
	containers := []*container{}
	mutex := sync.Mutex{}

	if networkName == "" {
		networkName = "private"
	}

	for i := 0; i < num; i++ {
		nodeNum := i % len(s.nodes)
		var name string

		mutex.Lock()
		if len(names) > 0 {
			name = names[0]
			names = names[1:]
		}
		mutex.Unlock()

		if name == "" {
			name = fmt.Sprintf("%s-srv%d-%d", strings.Replace(networkName, "/", "-", -1), i, nodeNum)
		}

		var serviceName string

		if withService {
			serviceName = name
		}

		spec := containerSpec{
			imageName:   "contiv/alpine",
			networkName: networkName,
			name:        name,
			serviceName: serviceName,
			tenantName:  tenantName,
		}

		cont, err := s.nodes[nodeNum].exec.runContainer(spec)
		if err != nil {
			return nil, err
		}

		mutex.Lock()
		containers = append(containers, cont)
		mutex.Unlock()

	}

	return containers, nil
}

func randSeq(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func (s *systemtestSuite) runContainersOnNode(num int, networkName, tenantName, groupname string, n *node) ([]*container, error) {
	containers := []*container{}
	mutex := sync.Mutex{}

	errChan := make(chan error)

	for i := 0; i < num; i++ {
		go func(i int) {
			spec := containerSpec{
				imageName:   "contiv/alpine",
				networkName: networkName,
				tenantName:  tenantName,
				serviceName: groupname,
			}
			if groupname == "" {
				spec.name = strings.ToLower(fmt.Sprintf("%s-%d-%s", n.Name(), i, randSeq(16)))
			} else {
				spec.name = fmt.Sprintf("%s-%d-%s", n.Name(), i, groupname)
			}

			cont, err := n.exec.runContainer(spec)
			if err != nil {
				errChan <- err
			}

			mutex.Lock()
			containers = append(containers, cont)
			mutex.Unlock()

			errChan <- nil
		}(i)
	}

	for i := 0; i < num; i++ {
		if err := <-errChan; err != nil {
			return nil, err
		}
	}

	return containers, nil
}

func (s *systemtestSuite) runContainersWithDNS(num int, tenantName, networkName,
	serviceName, dnsServer string) ([]*container, error) {

	containers := []*container{}
	mutex := sync.Mutex{}

	errChan := make(chan error)

	if len(dnsServer) <= 0 {
		logrus.Errorf("no dns specified")
		return nil, fmt.Errorf("no dns")
	}

	docknetName := fmt.Sprintf("%s/%s", networkName, tenantName)
	if tenantName == "default" {
		docknetName = networkName
	}
	docSrvName := docknetName
	if serviceName != "" {
		docSrvName = fmt.Sprintf("%s.%s", serviceName, docknetName)
	}

	for i := 0; i < num; i++ {
		go func(i int) {
			nodeNum := i % len(s.nodes)
			name := fmt.Sprintf("%s-srv%d-%d", strings.Replace(docSrvName, "/", "-", -1), i, nodeNum)
			spec := containerSpec{
				imageName:   "contiv/alpine",
				networkName: networkName,
				name:        name,
				serviceName: serviceName,
				dnsServer:   dnsServer,
				tenantName:  tenantName,
			}

			cont, err := s.nodes[nodeNum].exec.runContainer(spec)
			if err != nil {
				errChan <- err
			}

			mutex.Lock()
			containers = append(containers, cont)
			mutex.Unlock()

			errChan <- nil
		}(i)
	}

	for i := 0; i < num; i++ {
		if err := <-errChan; err != nil {
			return nil, err
		}
	}

	return containers, nil
}

func (s *systemtestSuite) pingTest(containers []*container) error {
	ips := []string{}
	v6ips := []string{}
	for _, cont := range containers {
		ips = append(ips, cont.eth0.ip)
		if cont.eth0.ipv6 != "" {
			v6ips = append(v6ips, cont.eth0.ipv6)
		}
	}

	errChan := make(chan error, len(containers)*len(ips))

	for _, cont := range containers {
		for _, ip := range ips {
			go func(cont *container, ip string) { errChan <- cont.node.exec.checkPingWithCount(cont, ip, 3) }(cont, ip)
		}
	}

	for i := 0; i < len(containers)*len(ips); i++ {
		if err := <-errChan; err != nil {
			return err
		}
	}

	if len(v6ips) > 0 {
		v6errChan := make(chan error, len(containers)*len(v6ips))

		for _, cont := range containers {
			for _, ipv6 := range v6ips {
				go func(cont *container, ipv6 string) { v6errChan <- cont.node.exec.checkPing6WithCount(cont, ipv6, 2) }(cont, ipv6)
			}
		}

		for i := 0; i < len(containers)*len(v6ips); i++ {
			if err := <-v6errChan; err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *systemtestSuite) pingTestByName(containers []*container, hostName string) error {

	errChan := make(chan error, len(containers))

	for _, cont := range containers {
		go func(cont *container, hostName string) { errChan <- cont.node.exec.checkPing(cont, hostName) }(cont, hostName)
	}

	for i := 0; i < len(containers); i++ {
		if err := <-errChan; err != nil {
			return err
		}
	}

	return nil
}

func (s *systemtestSuite) pingFailureTest(containers1 []*container, containers2 []*container) error {
	errChan := make(chan error, len(containers1)*len(containers2))

	for _, cont1 := range containers1 {
		for _, cont2 := range containers2 {
			go func(cont1 *container, cont2 *container) {
				errChan <- cont1.node.exec.checkPingFailure(cont1, cont2.eth0.ip)
			}(cont1, cont2)
		}
	}

	for i := 0; i < len(containers1)*len(containers2); i++ {
		if err := <-errChan; err != nil {
			return err
		}
	}

	return nil
}

func (s *systemtestSuite) hostIsolationTest(containers []*container) error {
	numTests := 0
	errChan := make(chan error, len(containers)-1)
	hBridgeIPs := make(map[string]string)
	for _, cont := range containers {
		ip, err := cont.node.exec.getIPAddr(cont, "host1")
		if err != nil {
			logrus.Errorf("Error getting host1 ip for container: %+v err: %v",
				cont, err)
			return err
		}
		hBridgeIPs[cont.containerID] = ip
	}

	for _, cont := range containers {
		for _, hIP := range hBridgeIPs {
			if hIP != hBridgeIPs[cont.containerID] {
				go func(c *container, dest string) {
					errChan <- c.node.exec.checkPingFailure(c, dest)
				}(cont, hIP)
				numTests++
			}
		}

		break // ping from one container to all others is sufficient
	}

	for i := 0; i < numTests; i++ {
		if err := <-errChan; err != nil {
			return err
		}
	}

	return nil
}

func (s *systemtestSuite) verifyPvtIP(containers []*container, pvtNet string) error {
	for _, cont := range containers {
		ip, err := cont.node.exec.getIPAddr(cont, "host1")
		if err != nil {
			logrus.Errorf("Error getting host1 ip for container: %+v err: %v",
				cont, err)
			return err
		}

		ipBytes := strings.Split(ip, ".")
		if len(ipBytes) != 4 {
			logrus.Errorf("Error bad host1 ip for container: %+v ip: %s",
				cont, ip)
			return errors.New("bad host1 IP")
		}

		ipStr := ipBytes[0] + ipBytes[1] + "0.0"
		if ipStr != pvtNet {
			logrus.Errorf("Incorrect pvt subnet %s for container %v, exp: %s", ipStr, cont, pvtNet)
			return errors.New("pvt subnet does not match")
		}
	}

	return nil
}

func (s *systemtestSuite) removeContainers(containers []*container) error {
	errChan := make(chan error, len(containers))
	for _, cont := range containers {
		go func(cont *container) { errChan <- cont.node.exec.rm(cont) }(cont)
	}

	for range containers {
		if err := <-errChan; err != nil {
			return err
		}
	}

	return nil
}

func (s *systemtestSuite) startListeners(containers []*container, ports []int) error {
	errChan := make(chan error, len(containers)*len(ports))

	for _, cont := range containers {
		for _, port := range ports {
			go func(cont *container, port int) { errChan <- cont.node.exec.startListener(cont, port, "tcp") }(cont, port)
		}
	}

	for i := 0; i < len(containers)*len(ports); i++ {
		if err := <-errChan; err != nil {
			return err
		}
	}

	return nil
}

func (s *systemtestSuite) startIperfServers(containers []*container) error {
	for _, cont := range containers {
		err := cont.node.exec.startIperfServer(cont)
		if err != nil {
			logrus.Errorf("Error starting the iperf server")
			return err
		}
	}

	return nil
}

func (s *systemtestSuite) checkConnections(containers []*container, port int) error {
	ips := []string{}
	for _, cont := range containers {
		ips = append(ips, cont.eth0.ip)
	}

	endChan := make(chan error, len(containers))

	for _, cont := range containers {
		for _, ip := range ips {
			if cont.eth0.ip == ip {
				continue
			}

			go func(cont *container, ip string, port int) {
				endChan <- cont.node.exec.checkConnection(cont, ip, "tcp", port)
			}(cont, ip, port)
		}
	}

	for i := 0; i < len(containers)*(len(ips)-1); i++ {
		if err := <-endChan; err != nil {
			return err
		}
	}

	return nil
}

func (s *systemtestSuite) startIperfClients(containers []*container, limit string, isErr bool) error {

	// get the ips of containers in a slice.
	ips := []string{}
	for _, cont := range containers {
		ips = append(ips, cont.eth0.ip)
	}

	for _, cont := range containers {
		for _, ip := range ips {
			if cont.eth0.ip == ip {
				continue
			}
			err := cont.node.exec.startIperfClient(cont, ip, limit, isErr)
			if err != nil {
				logrus.Errorf("Error starting the iperf client")
				return err
			}
		}
	}
	return nil
}

func (s *systemtestSuite) checkIperfAcrossGroup(containers []*container, containers1 []*container, limit string, isErr bool) error {
	// get the ips of containers in a slice.
	ips := []string{}
	for _, cont := range containers1 {
		ips = append(ips, cont.eth0.ip)
	}

	for _, cont := range containers {
		for _, ip := range ips {
			if cont.eth0.ip == ip {
				continue
			}
			err := cont.node.exec.startIperfClient(cont, ip, limit, isErr)
			if err != nil {
				logrus.Errorf("Error starting the iperf client")
				return err
			}
		}
	}
	return nil
}

func (s *systemtestSuite) checkIngressRate(containers []*container, bw string) error {
	for _, cont := range containers {
		fmt.Printf("Checking IngressRate for container %s for bw :%s ", cont, bw)
		err := cont.node.exec.tcFilterShow(bw)
		return err
	}
	return nil
}

func (s *systemtestSuite) checkNoConnections(containers []*container, port int) error {
	ips := []string{}
	for _, cont := range containers {
		ips = append(ips, cont.eth0.ip)
	}

	endChan := make(chan error, len(containers))

	for _, cont := range containers {
		for _, ip := range ips {
			if cont.eth0.ip == ip {
				continue
			}

			go func(cont *container, ip string, port int) {
				endChan <- cont.node.exec.checkNoConnection(cont, ip, "tcp", port)
			}(cont, ip, port)
		}
	}

	for i := 0; i < len(containers)*(len(ips)-1); i++ {
		if err := <-endChan; err != nil {
			return err
		}
	}

	return nil
}

func (s *systemtestSuite) checkConnectionsAcrossGroup(containers map[*container]string, port int, expFail bool) error {
	groups := map[string][]*container{}

	for cont1, group := range containers {
		if _, ok := groups[group]; !ok {
			groups[group] = []*container{}
		}

		groups[group] = append(groups[group], cont1)
	}

	for cont1, group := range containers {
		for group2, conts := range groups {
			if group != group2 {
				for _, cont := range conts {
					err := cont1.node.exec.checkConnection(cont1, cont.eth0.ip, "tcp", port)
					if !expFail && err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func (s *systemtestSuite) checkConnectionsWithinGroup(containers map[*container]string, port int) error {
	groups := map[string][]*container{}

	for cont1, group := range containers {
		if _, ok := groups[group]; !ok {
			groups[group] = []*container{}
		}

		groups[group] = append(groups[group], cont1)
	}

	for cont1, group := range containers {
		for group2, conts := range groups {
			if group == group2 {
				for _, cont := range conts {
					if err := cont1.node.exec.checkConnection(cont1, cont.eth0.ip, "tcp", port); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func (s *systemtestSuite) checkPingContainersInNetworks(containers map[*container]string) error {
	networks := map[string][]*container{}

	for cont1, network := range containers {
		if _, ok := networks[network]; !ok {
			networks[network] = []*container{}
		}

		networks[network] = append(networks[network], cont1)
	}

	for cont1, network := range containers {
		for network2, conts := range networks {
			if network2 == network {
				for _, cont := range conts {
					if err := cont1.node.exec.checkPing(cont1, cont.eth0.ip); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func (s *systemtestSuite) checkAllConnection(netContainers map[*container]string, groupContainers map[*container]string) error {
	if err := s.checkPingContainersInNetworks(netContainers); err != nil {
		return err
	}

	if err := s.checkConnectionsWithinGroup(groupContainers, 8000); err != nil {
		return err
	}

	if err := s.checkConnectionsWithinGroup(groupContainers, 8001); err != nil {
		return err
	}

	if err := s.checkConnectionsAcrossGroup(groupContainers, 8000, false); err != nil {
		return err
	}

	if err := s.checkConnectionsAcrossGroup(groupContainers, 8001, true); err != nil {
		return fmt.Errorf("connections across group achieved for port 8001")
	}

	return nil
}

func (s *systemtestSuite) pingFailureTestDifferentNode(containers1 []*container, containers2 []*container) error {

	count := 0

	for _, cont1 := range containers1 {
		for _, cont2 := range containers2 {
			if cont1.node != cont2.node {
				count++
			}
		}
	}
	errChan := make(chan error, count)

	for _, cont1 := range containers1 {
		for _, cont2 := range containers2 {
			if cont1.node != cont2.node {
				go func(cont1 *container, cont2 *container) {
					errChan <- cont1.node.exec.checkPingFailure(cont1, cont2.eth0.ip)
				}(cont1, cont2)
			}
		}
	}

	for i := 0; i < count; i++ {
		if err := <-errChan; err != nil {
			return err
		}
	}

	return nil
}

func (s *systemtestSuite) pingTestToNonContainer(containers []*container, nonContIps []string) error {

	errChan := make(chan error, len(containers)*len(nonContIps))

	for _, cont := range containers {
		for _, ip := range nonContIps {
			go func(cont *container, ip string) { errChan <- cont.node.exec.checkPingWithCount(cont, ip, 3) }(cont, ip)
		}
	}

	for i := 0; i < len(containers)*len(nonContIps); i++ {
		if err := <-errChan; err != nil {
			return err
		}
	}
	return nil
}

func (s *systemtestSuite) getJSON(url string, target interface{}) error {
	content, err := s.nodes[0].runCommand(fmt.Sprintf("curl -s %s", url))
	if err != nil {
		logrus.Errorf("Error getting curl output: Err %v", err)
		return err
	}
	return json.Unmarshal([]byte(content), target)
}

func (s *systemtestSuite) clusterStoreGet(path string) (string, error) {
	if s.basicInfo.ClusterStoreDriver == "etcd" {
		var etcdKv map[string]interface{}

		// Get from etcd
		// TODO: support multi urls
		etcdURL := s.basicInfo.ClusterStoreURLs + "/v2/keys" + path

		// get json from etcd
		err := s.getJSON(etcdURL, &etcdKv)
		if err != nil {
			logrus.Errorf("Error getting json from host. Err: %v", err)
			return "", err
		}

		node, ok := etcdKv["node"]
		if !ok {
			logrus.Errorf("Invalid json from etcd. %+v", etcdKv)
			return "", errors.New("node not found")
		}
		value, ok := node.(map[string]interface{})["value"]
		if !ok {
			logrus.Errorf("Invalid json from etcd. %+v", etcdKv)
			return "", errors.New("value not found")
		}

		return value.(string), nil
	} else if s.basicInfo.ClusterStoreDriver == "consul" {
		var consulKv []map[string]interface{}

		// Get from consul
		// TODO: support multi urls
		consulURL := s.basicInfo.ClusterStoreURLs + "/v1/kv" + path

		// get kv json from consul
		err := s.getJSON(consulURL, &consulKv)
		if err != nil {
			return "", err
		}

		value, ok := consulKv[0]["Value"]
		if !ok {
			logrus.Errorf("Invalid json from consul. %+v", consulKv)
			return "", errors.New("value not found")
		}

		retVal, err := base64.StdEncoding.DecodeString(value.(string))
		return string(retVal), err
	} else {
		// Unknown cluster store
		return "", errors.New("unknown cluster store")
	}
}

func (s *systemtestSuite) checkConnectionToService(containers []*container, ips []string, port int, protocol string) error {

	for _, cont := range containers {
		for _, ip := range ips {
			if err := cont.node.exec.checkConnection(cont, ip, "tcp", port); err != nil {
				return err
			}
		}
	}
	return nil
}

//ports is of the form 80:8080:TCP
func (s *systemtestSuite) startListenersOnProviders(containers []*container, ports []string) error {

	portList := []int{}

	for _, port := range ports {
		p := strings.Split(port, ":")
		providerPort, _ := strconv.Atoi(p[1])
		portList = append(portList, providerPort)
	}

	errChan := make(chan error, len(containers)*len(portList))

	for _, cont := range containers {
		for _, port := range portList {
			go func(cont *container, port int) { errChan <- cont.node.exec.startListener(cont, port, "tcp") }(cont, port)
		}
	}

	for i := 0; i < len(containers)*len(portList); i++ {
		if err := <-errChan; err != nil {
			return err
		}
	}

	return nil
}

func (s *systemtestSuite) getVTEPList() (map[string]bool, error) {
	vtepMap := make(map[string]bool)
	for _, n := range s.nodes {
		if s.basicInfo.Scheduler == kubeScheduler && n.Name() == "k8master" {
			continue
		}
		vtep, err := n.getIPAddr(s.hostInfo.HostMgmtInterface)
		if err != nil {
			logrus.Errorf("Error getting eth1 IP address for node %s", n.Name())
			return nil, err
		}

		vtepMap[vtep] = true
	}

	return vtepMap, nil
}

// Verify that the node is removed from VTEP table and the agentDB
func (s *systemtestSuite) verifyNodeRemoved(removed *node) error {
	vteps, err := s.getVTEPList()
	if err != nil {
		logrus.Errorf("Failed to get VTEPs")
		return err
	}

	nutIP, err := removed.getIPAddr(s.hostInfo.HostMgmtInterface)
	if err != nil {
		logrus.Errorf("Failed to get node VTEP")
		return err
	}

	// Exclude the node-under-test
	delete(vteps, nutIP)
	failNode := ""
	err = nil
	dbgOut := ""
	for try := 0; try < 20; try++ {
		for _, n := range s.nodes {
			if n.Name() == removed.Name() {
				continue
			}

			dbgOut, err = n.verifyAgentDB(vteps)
			if err != nil {
				failNode = n.Name()
				break
			}

			if n.Name() == "k8master" {
				continue
			}

			dbgOut, err = n.verifyVTEPs(vteps)
			if err != nil {
				failNode = n.Name()
				break
			}
		}

		if err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	logrus.Errorf("Node %s failed to verify node removal ERR: %v", failNode, err)
	logrus.Infof("Debug output:\n %s", dbgOut)
	return errors.New("failed to verify VTEPs after 20 sec")
}

func (s *systemtestSuite) verifyVTEPs() error {
	// get all expected VTEPs
	var err error

	expVTEPs, err := s.getVTEPList()
	if err != nil {
		logrus.Errorf("Failed to get VTEPs")
		return err
	}

	failNode := ""
	err = nil
	dbgOut := ""
	for try := 0; try < 60; try++ {
		for _, n := range s.nodes {
			if n.Name() == "k8master" {
				continue
			}
			dbgOut, err = n.verifyVTEPs(expVTEPs)
			if err != nil {
				failNode = n.Name()
				break
			}
		}

		if err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	logrus.Errorf("Node %s failed to verify all VTEPs. ERR: %v", failNode, err)
	logrus.Infof("Debug output:\n %s", dbgOut)
	return errors.New("failed to verify VTEPs after 60 sec")
}

func (s *systemtestSuite) verifyEPs(containers []*container) error {
	var err error

	// get the list of eps to verify
	epList := make([]string, len(containers))
	for ix, cont := range containers {
		epList[ix] = cont.eth0.ip
	}

	err = nil
	dbgOut := ""
	for try := 0; try < 20; try++ {
		for _, n := range s.nodes {
			if n.Name() == "k8master" {
				continue
			}
			dbgOut, err = n.verifyEPs(epList)
			if err != nil {
				break
			}
		}

		if err == nil {
			logrus.Infof("EPs %v verified on all nodes", epList)
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	logrus.Errorf("Failed to verify EPs after 20 sec %v", err)
	logrus.Infof("Debug output:\n %s", dbgOut)
	return err
}

func (s *systemtestSuite) verifyIPs(ipaddrs []string) error {
	var err error

	err = nil
	dbgOut := ""
	for try := 0; try < 20; try++ {
		for _, n := range s.nodes {

			if n.Name() == "k8master" {
				continue
			}

			dbgOut, err = n.verifyEPs(ipaddrs)
			if err != nil {
				break
			}

			if err == nil {
				logrus.Info("IPs %v verified on node %s", ipaddrs, n.Name())
				return nil
			}
		}
		time.Sleep(1 * time.Second)
	}

	logrus.Errorf("Failed to verify EP after 20 sec %v ", err)
	logrus.Info("Debug output:\n %s", dbgOut)
	return err
}

//Function to extract cfg Info from JSON file
func getInfo(file string) (BasicInfo, HostInfo, GlobInfo, error) {
	var (
		b BasicInfo
		c HostInfo
		d GlobInfo
	)
	raw, err := ioutil.ReadFile(file)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	err = json.Unmarshal(raw, &b)
	if err != nil {
		return b, c, d, err
	}
	err = json.Unmarshal(raw, &c)
	if err != nil {
		return b, c, d, err
	}
	err = json.Unmarshal(raw, &d)
	return b, c, d, err
}

// Setup suite and test methods for all platforms
func (s *systemtestSuite) SetUpSuiteBaremetal(c *C) {

	logrus.Infof("Private keyFile = %s", s.basicInfo.KeyFile)
	logrus.Infof("Binary binpath = %s", s.basicInfo.BinPath)
	logrus.Infof("Uplink data Interface(s) = %s", s.hostInfo.HostDataInterfaces)

	s.baremetal = remotessh.Baremetal{}
	bm := &s.baremetal

	// To fill the hostInfo data structure for Baremetal VMs
	var name string
	if s.basicInfo.AciMode == "on" {
		name = "aci-" + s.basicInfo.Scheduler + "-baremetal-node"
	} else {
		name = s.basicInfo.Scheduler + "-baremetal-node"
	}
	hostIPs := strings.Split(s.hostInfo.HostIPs, ",")
	hostNames := strings.Split(s.hostInfo.HostUsernames, ",")
	hosts := make([]remotessh.HostInfo, len(hostNames))

	for i := range hostIPs {
		hosts[i].Name = name + strconv.Itoa(i+1)
		logrus.Infof("Name=%s", hosts[i].Name)

		hosts[i].SSHAddr = hostIPs[i]
		logrus.Infof("SHAddr=%s", hosts[i].SSHAddr)

		hosts[i].SSHPort = "22"

		hosts[i].User = hostNames[i]
		logrus.Infof("User=%s", hosts[i].User)

		hosts[i].PrivKeyFile = s.basicInfo.KeyFile
		logrus.Infof("PrivKeyFile=%s", hosts[i].PrivKeyFile)

		hosts[i].Env = append([]string{}, s.basicInfo.SwarmEnv)
		logrus.Infof("Env variables are =%s", hosts[i].Env)

	}
	c.Assert(bm.Setup(hosts), IsNil)
	s.nodes = []*node{}

	for _, nodeObj := range s.baremetal.GetNodes() {
		nodeName := nodeObj.GetName()
		if strings.Contains(nodeName, s.basicInfo.Scheduler) {
			node := &node{}
			node.tbnode = nodeObj
			node.suite = s

			switch s.basicInfo.Scheduler {
			case kubeScheduler:
				node.exec = s.NewK8sExec(node)
			case swarmScheduler:
				node.exec = s.NewSwarmExec(node)
			default:
				node.exec = s.NewDockerExec(node)
			}
			s.nodes = append(s.nodes, node)
		}
		//s.nodes = append(s.nodes, &node{tbnode: nodeObj, suite: s})
	}
	logrus.Info("Pulling alpine on all nodes")
	s.baremetal.IterateNodes(func(node remotessh.TestbedNode) error {
		node.RunCommand("sudo rm /tmp/*net*")
		return node.RunCommand("docker pull contiv/alpine")
	})
	//Copying binaries
	s.copyBinary("netmaster")
	s.copyBinary("netplugin")
	s.copyBinary("netctl")
	s.copyBinary("contivk8s")
}

func (s *systemtestSuite) SetUpSuiteVagrant(c *C) {
	s.vagrant = remotessh.Vagrant{}
	nodesStr := os.Getenv("CONTIV_NODES")
	var contivNodes int

	logrus.Infof("Running tests with Forwarding mode: %s", s.fwdMode)
	if nodesStr == "" {
		contivNodes = 3
	} else {
		var err error
		contivNodes, err = strconv.Atoi(nodesStr)
		if err != nil {
			c.Fatal(err)
		}
	}

	s.nodes = []*node{}
	if s.fwdMode == "routing" {
		contivL3Nodes := 2
		switch s.basicInfo.Scheduler {
		case kubeScheduler:
			topDir := os.Getenv("GOPATH")
			//topDir contains the godeps path. hence purging the gopath
			topDir = strings.Split(topDir, ":")[1]

			contivNodes = 4 // 3 contiv nodes + 1 k8master
			c.Assert(s.vagrant.Setup(false, []string{"CONTIV_L3=1 VAGRANT_CWD=" + topDir + "/src/github.com/contiv/netplugin/vagrant/k8s/"}, contivNodes), IsNil)

		case swarmScheduler:
			c.Assert(s.vagrant.Setup(false, append([]string{"CONTIV_NODES=3 CONTIV_L3=1"}, s.basicInfo.SwarmEnv), contivNodes+contivL3Nodes), IsNil)
		default:
			c.Assert(s.vagrant.Setup(false, []string{"CONTIV_NODES=3 CONTIV_L3=1"}, contivNodes+contivL3Nodes), IsNil)

		}

	} else {
		switch s.basicInfo.Scheduler {
		case kubeScheduler:
			contivNodes = contivNodes + 1 //k8master

			topDir := os.Getenv("GOPATH")
			//topDir may contain the godeps path. hence purging the gopath
			dirs := strings.Split(topDir, ":")
			if len(dirs) > 1 {
				topDir = dirs[1]
			} else {
				topDir = dirs[0]
			}

			c.Assert(s.vagrant.Setup(false, []string{"VAGRANT_CWD=" + topDir + "/src/github.com/contiv/netplugin/vagrant/k8s/"}, contivNodes), IsNil)

		case swarmScheduler:
			c.Assert(s.vagrant.Setup(false, append([]string{}, s.basicInfo.SwarmEnv), contivNodes), IsNil)
		default:
			c.Assert(s.vagrant.Setup(false, []string{}, contivNodes), IsNil)

		}

	}

	for _, nodeObj := range s.vagrant.GetNodes() {
		nodeName := nodeObj.GetName()
		if strings.Contains(nodeName, "netplugin-node") ||
			strings.Contains(nodeName, "k8") {
			node := &node{}
			node.tbnode = nodeObj
			node.suite = s
			switch s.basicInfo.Scheduler {
			case kubeScheduler:
				if s.basicInfo.InstallMode == kubeadmInstall {
					node.exec = s.NewK8sPodExec(node)
				} else {
					node.exec = s.NewK8sExec(node)
				}
			case swarmScheduler:
				node.exec = s.NewSwarmExec(node)
			default:
				node.exec = s.NewDockerExec(node)
			}
			s.nodes = append(s.nodes, node)
		}
	}

	logrus.Info("Pulling alpine on all nodes")
	s.vagrant.IterateNodes(func(node remotessh.TestbedNode) error {
		node.RunCommand("sudo rm /tmp/net*")
		return node.RunCommand("docker pull contiv/alpine")
	})
}

func (s *systemtestSuite) SetUpTestBaremetal(c *C) {

	for _, node := range s.nodes {
		node.exec.cleanupContainers()
		//node.exec.cleanupDockerNetwork()

		node.stopNetplugin()
		node.cleanupSlave()
		node.deleteFile("/etc/systemd/system/netplugin.service")
		node.stopNetmaster()
		node.deleteFile("/etc/systemd/system/netmaster.service")
		node.deleteFile("/usr/bin/netplugin")
		node.deleteFile("/usr/bin/netmaster")
		node.deleteFile("/usr/bin/netctl")
	}

	for _, node := range s.nodes {
		node.cleanupMaster()
	}

	for _, node := range s.nodes {

		c.Assert(node.startNetplugin(""), IsNil)
		c.Assert(node.exec.runCommandUntilNoNetpluginError(), IsNil)

	}

	for _, node := range s.nodes {
		c.Assert(node.startNetmaster(""), IsNil)
		time.Sleep(1 * time.Second)
		c.Assert(node.exec.runCommandUntilNoNetmasterError(), IsNil)
	}

	if s.basicInfo.Scheduler != kubeScheduler {
		for i := 0; i < 11; i++ {
			_, err := s.cli.TenantGet("default")
			if err == nil {
				break
			}
			// Fail if we reached last iteration
			c.Assert((i < 10), Equals, true)
			time.Sleep(500 * time.Millisecond)
		}
	}

	time.Sleep(10 * time.Second)

}

func (s *systemtestSuite) SetUpTestVagrant(c *C) {
	s.parallelExec(func(node *node) error {
		node.exec.cleanupContainers()
		node.stopNetplugin()
		node.cleanupSlave()
		return nil
	})

	s.parallelExec(func(node *node) error {
		node.stopNetmaster()
		node.cleanupMaster()
		return nil
	})

	errors := s.parallelExec(func(node *node) error {
		return node.startNetplugin("")

	})
	for _, err := range errors {
		c.Assert(err, IsNil)
	}

	errors = s.parallelExec(func(node *node) error {
		return node.exec.runCommandUntilNoNetpluginError()
	})
	for _, err := range errors {
		c.Assert(err, IsNil)
	}

	errors = s.parallelExec(func(node *node) error {
		return node.startNetmaster("")
	})
	for _, err := range errors {
		c.Assert(err, IsNil)
	}

	errors = s.parallelExec(func(node *node) error {
		return node.exec.runCommandUntilNoNetmasterError()
	})
	for _, err := range errors {
		c.Assert(err, IsNil)
	}

	if s.basicInfo.Scheduler != kubeScheduler {
		for i := 0; i < 21; i++ {

			_, err := s.cli.TenantGet("default")
			if err == nil {
				break
			}
			// Fail if we reached last iteration
			c.Assert((i < 30), Equals, true)
			time.Sleep(1 * time.Second)
		}
	}
	time.Sleep(10 * time.Second)

	if s.basicInfo.Scheduler == kubeScheduler {
		c.Assert(s.SetupDefaultNetwork(), IsNil)
	}
}

func (s *systemtestSuite) verifyHostRoutes(routes []string, expect bool) error {

	for _, n := range s.nodes {
		if s.basicInfo.Scheduler == kubeScheduler && n.Name() == "k8master" {
			continue
		}
		out, err := n.runCommand("ip route")
		if err != nil {
			logrus.Errorf("Error getting routes: %v", err)
			return err
		}

		for _, r := range routes {
			present := strings.Contains(out, r)
			if expect != present {
				logrus.Errorf("Node %s route: %s expected: %v actual: %v", n.Name(), r, expect, present)
				logrus.Errorf("Out: %v", out)
				return fmt.Errorf("Node %s route: %s expected: %v actual: %v", n.Name(), r, expect, present)
			}
		}
	}

	return nil
}

func (s *systemtestSuite) verifyHostPing(containers []*container) error {

	for _, c := range containers {
		err := c.node.checkPingWithCount(c.eth0.ip, 3)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *systemtestSuite) IsolationTest(containers []*container) error {
	for _, c := range containers {
		err := c.node.exec.checkPingFailure(c, "172.19.255.254")
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *systemtestSuite) TearDownDefaultNetwork() error {
	if s.basicInfo.Scheduler != kubeScheduler {
		return nil
	}

	err := s.cli.NetworkDelete("default", "default-net")
	if err != nil {
		logrus.Errorf("default-net not deleted. Err: %+v", err)
		return err
	}
	time.Sleep(time.Second)
	return nil
}

func (s *systemtestSuite) SetupDefaultNetwork() error {
	if s.basicInfo.Scheduler != kubeScheduler {
		return nil
	}

	err := s.cli.NetworkPost(&client.Network{
		TenantName:  "default",
		NetworkName: "default-net",
		Subnet:      "100.10.1.0/24",
		Gateway:     "100.10.1.254",
		Encap:       "vxlan",
	})
	if err != nil {
		logrus.Errorf("default-net not created. Err: %+v", err)
		return err
	}
	time.Sleep(time.Second)
	return nil
}
