package systemtests

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
)

func (s *systemtestSuite) checkConnectionPair(containers1, containers2 []*container, port int) error {
	for _, cont := range containers1 {
		for _, cont2 := range containers2 {
			if err := cont.checkConnection(cont2.eth0, "tcp", port); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *systemtestSuite) runContainersInGroups(num int, netName string, groupNames []string) (map[*container]string, error) {
	containers := map[*container]string{}
	for _, groupName := range groupNames {
		names := []string{}

		for i := 0; i < num; i++ {
			names = append(names, fmt.Sprintf("%s-%d", groupName, i))
		}

		// XXX we don't use anything but private for this function right now
		conts, err := s.runContainersInService(num, groupName, netName, names)
		if err != nil {
			return nil, err
		}

		for _, cont := range conts {
			containers[cont] = groupName
		}
	}

	return containers, nil
}

func (s *systemtestSuite) runContainersInService(num int, serviceName, networkName string, names []string) ([]*container, error) {
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
				imageName:   "alpine",
				networkName: networkName,
				name:        name,
				serviceName: serviceName,
			}

			cont, err := s.nodes[nodeNum].runContainer(spec)
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

func (s *systemtestSuite) runContainers(num int, withService bool, networkName string,
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

			spec := containerSpec{
				imageName:   "alpine",
				networkName: networkName,
				name:        name,
				serviceName: serviceName,
			}
			if len(labels) > 0 {
				spec.labels = append(spec.labels, labels...)
			}

			cont, err := s.nodes[nodeNum].runContainer(spec)
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

func (s *systemtestSuite) runContainersSerial(num int, withService bool, networkName string, names []string) ([]*container, error) {
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
			imageName:   "alpine",
			networkName: networkName,
			name:        name,
			serviceName: serviceName,
		}

		cont, err := s.nodes[nodeNum].runContainer(spec)
		if err != nil {
			return nil, err
		}

		mutex.Lock()
		containers = append(containers, cont)
		mutex.Unlock()

	}

	return containers, nil
}

func (s *systemtestSuite) runContainersOnNode(num int, networkName string, n *node) ([]*container, error) {
	containers := []*container{}
	mutex := sync.Mutex{}

	errChan := make(chan error)

	for i := 0; i < num; i++ {
		go func(i int) {
			spec := containerSpec{
				imageName:   "alpine",
				networkName: networkName,
			}

			cont, err := n.runContainer(spec)
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

func (s *systemtestSuite) runContainersWithDNS(num int, tenantName, networkName, serviceName string) ([]*container, error) {
	containers := []*container{}
	mutex := sync.Mutex{}

	errChan := make(chan error)

	// Get the dns server for the network
	dnsServer, err := s.getNetworkDNSServer(tenantName, networkName)
	if err != nil {
		logrus.Errorf("Error getting DNS server for network %s/%s", networkName, tenantName)
		return nil, err
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
				imageName:   "alpine",
				networkName: docknetName,
				name:        name,
				serviceName: serviceName,
				dnsServer:   dnsServer,
			}

			cont, err := s.nodes[nodeNum].runContainer(spec)
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
	for _, cont := range containers {
		ips = append(ips, cont.eth0)
	}

	errChan := make(chan error, len(containers)*len(ips))

	for _, cont := range containers {
		for _, ip := range ips {
			go func(cont *container, ip string) { errChan <- cont.checkPing(ip) }(cont, ip)
		}
	}

	for i := 0; i < len(containers)*len(ips); i++ {
		if err := <-errChan; err != nil {
			return err
		}
	}

	return nil
}

func (s *systemtestSuite) pingTestByName(containers []*container, hostName string) error {

	errChan := make(chan error, len(containers))

	for _, cont := range containers {
		go func(cont *container, hostName string) { errChan <- cont.checkPing(hostName) }(cont, hostName)
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
			go func(cont1 *container, cont2 *container) { errChan <- cont1.checkPingFailure(cont2.eth0) }(cont1, cont2)
		}
	}

	for i := 0; i < len(containers1)*len(containers2); i++ {
		if err := <-errChan; err != nil {
			return err
		}
	}

	return nil
}

func (s *systemtestSuite) removeContainers(containers []*container) error {
	errChan := make(chan error, len(containers))
	for _, cont := range containers {
		go func(cont *container) { errChan <- cont.rm() }(cont)
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
			go func(cont *container, port int) { errChan <- cont.startListener(port, "tcp") }(cont, port)
		}
	}

	for i := 0; i < len(containers)*len(ports); i++ {
		if err := <-errChan; err != nil {
			return err
		}
	}

	return nil
}

func (s *systemtestSuite) checkConnections(containers []*container, port int) error {
	ips := []string{}
	for _, cont := range containers {
		ips = append(ips, cont.eth0)
	}

	endChan := make(chan error, len(containers))

	for _, cont := range containers {
		for _, ip := range ips {
			if cont.eth0 == ip {
				continue
			}

			go func(cont *container, ip string, port int) { endChan <- cont.checkConnection(ip, "tcp", port) }(cont, ip, port)
		}
	}

	for i := 0; i < len(containers)*(len(ips)-1); i++ {
		if err := <-endChan; err != nil {
			return err
		}
	}

	return nil
}

func (s *systemtestSuite) checkNoConnections(containers []*container, port int) error {
	ips := []string{}
	for _, cont := range containers {
		ips = append(ips, cont.eth0)
	}

	endChan := make(chan error, len(containers))

	for _, cont := range containers {
		for _, ip := range ips {
			if cont.eth0 == ip {
				continue
			}

			go func(cont *container, ip string, port int) { endChan <- cont.checkNoConnection(ip, "tcp", port) }(cont, ip, port)
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
					err := cont1.checkConnection(cont.eth0, "tcp", port)
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
					if err := cont1.checkConnection(cont.eth0, "tcp", port); err != nil {
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
					if err := cont1.checkPing(cont.eth0); err != nil {
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
		return fmt.Errorf("Connections across group achieved for port 8001")
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
				go func(cont1 *container, cont2 *container) { errChan <- cont1.checkPingFailure(cont2.eth0) }(cont1, cont2)
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
			go func(cont *container, ip string) { errChan <- cont.checkPing(ip) }(cont, ip)
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
	if strings.Contains(s.clusterStore, "etcd://") {
		var etcdKv map[string]interface{}

		// Get from etcd
		etcdURL := strings.Replace(s.clusterStore, "etcd://", "http://", 1)
		etcdURL = etcdURL + "/v2/keys" + path

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
			return "", errors.New("Value not found")
		}

		return value.(string), nil
	} else if strings.Contains(s.clusterStore, "consul://") {
		var consulKv []map[string]interface{}

		// Get from consul
		consulURL := strings.Replace(s.clusterStore, "consul://", "http://", 1)
		consulURL = consulURL + "/v1/kv" + path

		// get kv json from consul
		err := s.getJSON(consulURL, &consulKv)
		if err != nil {
			return "", err
		}

		value, ok := consulKv[0]["Value"]
		if !ok {
			logrus.Errorf("Invalid json from consul. %+v", consulKv)
			return "", errors.New("Value not found")
		}

		retVal, err := base64.StdEncoding.DecodeString(value.(string))
		return string(retVal), err
	} else {
		// Unknown cluster store
		return "", errors.New("Unknown cluster store")
	}
}

func (s *systemtestSuite) getNetworkStates() ([]map[string]interface{}, error) {
	var networkList []map[string]interface{}

	err := s.getJSON("localhost:9999/networks", &networkList)
	if err != nil {
		logrus.Errorf("Error getting json from host. Err: %v", err)
		return nil, err
	}

	return networkList, err
}

func (s *systemtestSuite) getNetworkDNSServer(tenant, network string) (string, error) {
	netList, err := s.getNetworkStates()
	if err != nil {
		return "", err
	}

	for _, net := range netList {
		if net["tenant"].(string) == tenant && net["networkName"].(string) == network {
			dnsServer := net["dnsServer"].(string)

			if dnsServer == "" {
				logrus.Infof("Network %s/%s does not have a dns server", network, tenant)
				return "", errors.New("No DNS server in network")
			}
			logrus.Infof("Gor dns server %s for network %s/%s", dnsServer, network, tenant)
			return dnsServer, nil
		}
	}

	return "", errors.New("Network not found")
}
func (s *systemtestSuite) checkConnectionToService(containers []*container, ips []string, port int, protocol string) error {

	for _, cont := range containers {
		for _, ip := range ips {
			if err := cont.checkConnection(ip, "tcp", port); err != nil {
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
			go func(cont *container, port int) { errChan <- cont.startListener(port, "tcp") }(cont, port)
		}
	}

	for i := 0; i < len(containers)*len(portList); i++ {
		if err := <-errChan; err != nil {
			return err
		}
	}

	return nil
}
