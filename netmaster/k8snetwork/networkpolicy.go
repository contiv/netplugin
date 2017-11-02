package networkpolicy

import (
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/contivmodel/client"
	"github.com/contiv/netplugin/utils/k8sutils"
	"k8s.io/api/networking/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"reflect"
	"strings"
	"time"
)

const defaultTenantName = "default"
const defaultNetworkName = "net"
const defaultSubnet = "10.1.2.0/24"

//const defaultEpgName = "ingress-group"
const defaultEpgName = "default-epg"
const defaultPolicyName = "ingress-policy"
const defaultRuleID = "1"
const defaultPolicyPriority = 2

type k8sPodSelector struct {
	TenantName  string
	NetworkName string
	PodIps      []string
	GroupName   string
}
type k8sPolicyPorts struct {
	Port     int
	Protocol string
}

type k8sIngress struct {
	IngressRules       []k8sPolicyPorts
	IngressPodSelector []*k8sPodSelector
}
type k8sNetworkPolicy struct {
	PodSelector *k8sPodSelector
	Ingress     []k8sIngress
}
type k8sContext struct {
	k8sClientSet  *kubernetes.Clientset
	contivClient  *client.ContivClient
	isLeader      func() bool
	networkPolicy map[string]k8sNetworkPolicy
}

var npLog *log.Entry

func (k8sNet *k8sContext) handleK8sEvents() {
	for k8sNet.isLeader() != true {
		time.Sleep(time.Second * 10)
	}

	errCh := make(chan error)
	for {
		go k8sNet.watchK8sEvents(errCh)

		// wait for error from api server
		errMsg := <-errCh
		npLog.Errorf("%s", errMsg)
		npLog.Warnf("restarting k8s event watch")
		time.Sleep(time.Second * 5)
	}
}

func (k8sNet *k8sContext) createNetwork(nwName string) error {
	npLog.Infof("create network %s", nwName)

	if _, err := k8sNet.contivClient.NetworkGet(defaultTenantName, nwName); err == nil {
		return nil
	}

	if err := k8sNet.contivClient.NetworkPost(&client.Network{
		TenantName:  defaultTenantName,
		NetworkName: nwName,
		Subnet:      defaultSubnet,
		Encap:       "vxlan",
	}); err != nil {
		npLog.Errorf("failed to create network %s, %s", nwName, err)
		return err
	}

	for func() error {
		_, err := k8sNet.contivClient.NetworkGet(defaultTenantName, nwName)
		return err
	}() != nil { //XXX:Should we really poll here ; there would be chances on genuine error and it cause infinity  loop
		time.Sleep(time.Millisecond * 100)
	}
	return nil
}

func (k8sNet *k8sContext) deleteNetwork(nwName string) error {
	npLog.Infof("delete network %s", nwName)

	if _, err := k8sNet.contivClient.NetworkGet(defaultTenantName, nwName); err != nil {
		return nil
	}

	if err := k8sNet.contivClient.NetworkDelete(
		defaultTenantName, nwName); err != nil {
		npLog.Errorf("failed to delete network %s, %s", nwName, err)
		return err
	}

	for func() error {
		_, err := k8sNet.contivClient.NetworkGet(defaultTenantName, nwName)
		return err
	}() == nil { //XXX: Same here as above
		time.Sleep(time.Millisecond * 100)
	}
	return nil
}

func (k8sNet *k8sContext) createEpg(nwName, epgName, policyName string) error {
	npLog.Infof("create epg %s", epgName)

	if _, err := k8sNet.contivClient.EndpointGroupGet(defaultTenantName, epgName); err == nil {
		return nil
	}

	if err := k8sNet.contivClient.EndpointGroupPost(&client.EndpointGroup{
		TenantName:  defaultTenantName,
		NetworkName: nwName,
		GroupName:   epgName,
		Policies:    []string{policyName},
	}); err != nil {
		npLog.Errorf("failed to create epg %s, %s", epgName, err)
		return err
	}

	for func() error {
		_, err := k8sNet.contivClient.EndpointGroupGet(defaultTenantName, epgName)
		return err
	}() != nil { //XXX: Same as above
		time.Sleep(time.Millisecond * 100)
	}
	return nil
}

func (k8sNet *k8sContext) deleteEpg(networkname, epgName, policyName string) error {
	npLog.Infof("delete epg %s", epgName)
	if _, err := k8sNet.contivClient.EndpointGroupGet(defaultTenantName, epgName); err != nil {
		return nil
	}

	if err := k8sNet.contivClient.EndpointGroupDelete(
		defaultTenantName, epgName); err != nil {
		npLog.Errorf("failed to delete epg %s, %s", epgName, err)
		return err
	}

	for func() error {
		_, err := k8sNet.contivClient.EndpointGroupGet(defaultTenantName, epgName)
		return err
	}() == nil { //Same as above
		time.Sleep(time.Millisecond * 100)
	}
	return nil
}

func (k8sNet *k8sContext) createPolicy(tenantName string, epgName string) error {
	policyName := k8sutils.EpgNameToPolicy(epgName)

	npLog.Infof("create policy: %s:%s", policyName, tenantName)

	if _, err := k8sNet.contivClient.PolicyGet(tenantName, policyName); err == nil {
		return nil
	}

	if err := k8sNet.contivClient.PolicyPost(&client.Policy{
		TenantName: tenantName,
		PolicyName: policyName,
	}); err != nil {
		npLog.Errorf("failed to create policy: %v", err)
		return err
	}

	for func() error {
		_, err := k8sNet.contivClient.PolicyGet(tenantName, policyName)
		return err
	}() != nil {
		time.Sleep(time.Millisecond * 100)
	}
	return nil
}

func (k8sNet *k8sContext) deletePolicy(policyName string) error {
	npLog.Infof("delete policy %s", policyName)

	if _, err := k8sNet.contivClient.PolicyGet(defaultTenantName, policyName); err != nil {
		return nil
	}

	if err := k8sNet.contivClient.PolicyDelete(defaultTenantName, policyName); err != nil {
		npLog.Errorf("failed to delete policy %s, %s", policyName, err)
		return err
	}

	for func() error {
		_, err := k8sNet.contivClient.PolicyGet(defaultTenantName, policyName)
		return err
	}() == nil { //XXX: Same as av
		time.Sleep(time.Millisecond * 100)
	}
	return nil
}

func (k8sNet *k8sContext) createRule(cRule *client.Rule) error {
	npLog.Infof("create rule: %+v", *cRule)

	if val, err := k8sNet.contivClient.RuleGet(cRule.TenantName, cRule.PolicyName, cRule.RuleID); err == nil {
		if val.Action != cRule.Action {
			k8sNet.deleteRule(cRule.TenantName, cRule.PolicyName, cRule.RuleID)
		} else {
			return nil
		}
	}

	if err := k8sNet.contivClient.RulePost(cRule); err != nil {
		npLog.Errorf("failed to create rule: %s, %v", cRule.RuleID, err)
		return err
	}

	for func() error {
		_, err := k8sNet.contivClient.RuleGet(cRule.TenantName, cRule.PolicyName, cRule.RuleID)
		return err
	}() != nil {
		time.Sleep(time.Millisecond * 100)
	}
	return nil
}

func (k8sNet *k8sContext) deleteRule(tenantName string, policyName, ruleID string) error {
	npLog.Infof("delete rule: %s:%s", ruleID, policyName)

	if _, err := k8sNet.contivClient.RuleGet(tenantName, policyName, ruleID); err != nil {
		return nil
	}

	if err := k8sNet.contivClient.RuleDelete(tenantName, policyName, ruleID); err != nil {
		npLog.Errorf("failed to delete rule: %s:%s, %v", ruleID, policyName, err)
		return err
	}

	for func() error {
		_, err := k8sNet.contivClient.RuleGet(tenantName, policyName, ruleID)
		return err
	}() == nil {
		time.Sleep(time.Millisecond * 100)
	}
	return nil
}
func (k8sNet *k8sContext) getIsolationPolicy(annotations map[string]string) string {
	var inPolicy struct {
		Ingress map[string]string `json:"ingress"`
	}

	if inByte, ok := annotations["net.beta.kubernetes.io/network-policy"]; ok {
		if err := json.Unmarshal([]byte(inByte), &inPolicy); err != nil {
			npLog.Infof("no isolation policy in namespace [%s], %s", inByte, err)
			return "allow"
		}
	} else {
		return "none"
	}

	if policy, ok := inPolicy.Ingress["isolation"]; ok {
		if strings.ToLower(policy) == strings.ToLower("DefaultDeny") {
			return "deny"
		}
	}
	return "allow"
}

func (k8sNet *k8sContext) updateDefaultIngressPolicy(ns string, action string) {
	nwName := ns + "-" + defaultNetworkName
	policyName := ns + "-" + defaultPolicyName
	epgName := ns + "-" + defaultEpgName

	var err error

	if err = k8sNet.createNetwork(nwName); err != nil {
		npLog.Errorf("failed to update network %s, %s", nwName, err)
		return
	}

	if err = k8sNet.createPolicy(defaultTenantName, policyName); err != nil {
		npLog.Errorf("failed to update policy %s, %s", policyName, err)
		return
	}

	if err = k8sNet.createEpg(nwName, epgName, policyName); err != nil {
		npLog.Errorf("failed to update EPG %s, %s", epgName, err)
		return
	}

	if err = k8sNet.createRule(&client.Rule{
		TenantName: defaultTenantName,
		PolicyName: policyName,
		RuleID:     defaultRuleID,
		Priority:   defaultPolicyPriority,
		Direction:  "in",
		Action:     "allow"}); err != nil {
		npLog.Errorf("failed to update default rule, %s", err)
		return
	}
}

func (k8sNet *k8sContext) deleteDefaultIngressPolicy(ns string) {
	nwName := ns + "-" + defaultNetworkName
	policyName := ns + "-" + defaultPolicyName
	epgName := ns + "-" + defaultEpgName

	var err error

	if err = k8sNet.deleteRule(defaultTenantName, policyName, defaultRuleID); err != nil {
		npLog.Errorf("failed to delete default rule, %s", err)
		return
	}

	if err = k8sNet.deleteEpg(nwName, epgName, policyName); err != nil {
		npLog.Errorf("failed to delete EPG %s, %s", epgName, err)
		return
	}
	if err = k8sNet.deletePolicy(policyName); err != nil {
		npLog.Errorf("failed to delete policy %s, %s", policyName, err)
		return
	}
}

func (k8sNet *k8sContext) processK8sNetworkPolicy(opCode watch.EventType, np *v1.NetworkPolicy) {
	if np.Namespace == "kube-system" { // not applicable for system namespace
		return
	}

	npLog.Infof("process [%s] network policy  %+v", opCode, np)

	switch opCode {
	case watch.Added:
		npLog.Infof("Recv [%s] network policy event", opCode, np)
		k8sNet.addNetworkPolicy(np)
	case watch.Modified:
	case watch.Deleted:
	}
}

func (k8sNet *k8sContext) processK8sEvent(opCode watch.EventType, eventObj interface{}) {
	if k8sNet.isLeader() != true {
		return
	}
	switch objType := eventObj.(type) {

	case *v1.NetworkPolicy:
		k8sNet.processK8sNetworkPolicy(opCode, objType)
	}
}

func (k8sNet *k8sContext) watchK8sEvents(errChan chan error) {
	var selCase []reflect.SelectCase

	// wait to become leader
	for k8sNet.isLeader() != true {
		time.Sleep(time.Millisecond * 100)
	}

	npWatch, err := k8sNet.k8sClientSet.Networking().NetworkPolicies("").Watch(meta_v1.ListOptions{})
	if err != nil {
		errChan <- fmt.Errorf("failed to watch network policy, %s", err)
		return
	}

	selCase = append(selCase, reflect.SelectCase{Dir: reflect.SelectRecv,
		Chan: reflect.ValueOf(npWatch.ResultChan())})

	for {
		_, recVal, ok := reflect.Select(selCase)
		if !ok {
			// channel closed, trigger restart
			errChan <- fmt.Errorf("channel closed to k8s api server")
			return
		}

		if k8sNet.isLeader() != true {
			continue
		}

		if event, ok := recVal.Interface().(watch.Event); ok {
			k8sNet.processK8sEvent(event.Type, event.Object)
		}
		// ignore other events
	}
}

// InitK8SServiceWatch monitor k8s services
func InitK8SServiceWatch(listenURL string, isLeader func() bool) error {
	npLog = log.WithField("k8s", "netpolicy")

	listenAddr := strings.Split(listenURL, ":")
	if len(listenAddr[0]) <= 0 {
		listenAddr[0] = "localhost"
	}
	contivClient, err := client.NewContivClient("http://" + listenAddr[0] + ":" + listenAddr[1])
	if err != nil {
		npLog.Errorf("failed to create contivclient %s", err)
		return err
	}

	k8sClientSet, err := k8sutils.SetUpK8SClient()
	if err != nil {
		npLog.Fatalf("failed to init K8S client, %v", err)
		return err
	}
	kubeNet := k8sContext{contivClient: contivClient, k8sClientSet: k8sClientSet, isLeader: isLeader}

	go kubeNet.handleK8sEvents()
	return nil
}

func (k8sNet *k8sContext) addNetworkPolicy(np *v1beta1.NetworkPolicy) {
	//Get all pods which belongs to given label selector
	npPodSelector, err := k8sNet.getPodSelector(np.Spec.PodSelector.MatchLabels)
	if err != nil {
		npLog.Warnf("ignore network policy: %s, %v", np.Name, err)
		return
	}
	npLog.Infof("network policy [%s] pod-selector: %+v", np.Name, npPodSelector)
	IngressRules, err := k8sNet.getIngressPolicy(np.Spec.Ingress)
	if err != nil {
		npLog.Warnf("ignore network policy: %s, %v", np.Name, err)
		return
	}
	npLog.Infof("network Policy [%s] IngressPolicy: %+v", np.Name, IngressRules)
	if _, ok := k8sNet.networkPolicy[np.Name]; ok {
		npLog.Warnf("delete existing network policy: %s !", np.Name)
		k8sNet.deleteNetworkPolicy(np)
	}

	nwPolicy := k8sNetworkPolicy{PodSelector: npPodSelector, Ingress: IngressRules}

	npLog.Info("Going to Send Network Policy %+v", nwPolicy)

	if err := k8sNet.applyContivNetworkPolicy(&nwPolicy); err != nil {
		npLog.Errorf("[%s] failed to configure policy, %v", np.Name, err)
		return
	}

	k8sNet.networkPolicy[np.Name] = nwPolicy

}

func (k8sNet *k8sContext) applyContivNetworkPolicy(np *k8sNetworkPolicy) error {
	var err error

	// don't configure from multiple masters
	if k8sNet.isLeader() != true {
		return err
	}

	// reset policy to deny on any error
	policyResetOnErr := func(tenantName, groupName string) {
		if err != nil {
			//k8sNet.resetPolicy(tenantName, groupName)
		}
	}

	// policy
	if err = k8sNet.createDefaultPolicy(np.PodSelector.TenantName, np.PodSelector.GroupName); err != nil {
		npLog.Errorf("failed to create policy %+v, %v", np.PodSelector, err)
		return err
	}

	defer policyResetOnErr(np.PodSelector.TenantName, np.PodSelector.GroupName)

	// src epg
	if _, err := k8sNet.contivClient.EndpointGroupGet(np.PodSelector.TenantName,
		np.PodSelector.GroupName); err != nil {
		npLog.Infof("epg: %+v doesn't exist", np.PodSelector)
		return nil
	}
	npLog.Info("Got resp from Contiv EndPoint %+v", np.PodSelector)

	// Add epg and rules
	for _, ingress := range np.Ingress {
		for _, from := range ingress.IngressPodSelector {
			// from/to epgs
			if _, err := k8sNet.contivClient.EndpointGroupGet(from.TenantName, from.GroupName); err != nil {
				npLog.Infof("epg: %+v doesn't exist", from)
				return nil
			}
			npLog.Info("From EndPoint xxContiv EndPoint %+v", from)

			// rules
			for _, port := range ingress.IngressRules {
				npLog.Infof("configure contiv policy: %+v", port)
				for _, FromIP := range from.PodIps {
					for _, ToIP := range np.PodSelector.PodIps {
						ruleId := k8sutils.PolicyToRuleID(from.GroupName, port.Protocol, port.Port, "in")
						if err = k8sNet.createRule(&client.Rule{
							TenantName:    np.PodSelector.TenantName,
							PolicyName:    k8sutils.EpgNameToPolicy(np.PodSelector.GroupName),
							FromIpAddress: FromIP,
							ToIpAddress:   ToIP,
							RuleID:        ruleId,
							Protocol:      strings.ToLower(port.Protocol),
							Priority:      defaultPolicyPriority,
							Port:          port.Port,
							Direction:     "in",
							Action:        "allow"}); err != nil {
							npLog.Errorf("failed to create rules in in-policy %+v, %v", np.PodSelector, err)
							return err
						}
					}
				}
			}
		}
	}

	return nil
}
func (k8sNet *k8sContext) deleteNetworkPolicy(np *v1beta1.NetworkPolicy) {
	npLog.Infof("delete network policy: %s", np.Name)
	policy, ok := k8sNet.networkPolicy[np.Name]
	if !ok {
		npLog.Errorf("network policy: %s is not found", np.Name)
		return
	}

	if err := k8sNet.cleanupContivNetworkPolicy(&policy); err != nil {
		npLog.Errorf("failed to delete network policy: %s, %v", np.Name, err)
		return
	}
	//Remove PolicyId from Policy Db
	delete(k8sNet.networkPolicy, np.Name)

}
func (k8sNet *k8sContext) cleanupContivNetworkPolicy(np *k8sNetworkPolicy) error {
	var retErr error

	// don't configure from multiple masters
	if k8sNet.isLeader() != true {
		return nil
	}
	policyName := k8sutils.EpgNameToPolicy(np.PodSelector.GroupName)
	for _, ingress := range np.Ingress {
		for _, from := range ingress.IngressPodSelector {
			for _, port := range ingress.IngressRules {
				for _, direction := range []string{"in", "out"} {
					ruleID := k8sutils.PolicyToRuleID(from.GroupName, port.Protocol,
						port.Port, direction)
					policyName := k8sutils.EpgNameToPolicy(np.PodSelector.GroupName)

					if err := k8sNet.deleteRule(np.PodSelector.TenantName, policyName, ruleID); err != nil {
						npLog.Warnf("failed to delete policy: %s rule: %s, %v",
							policyName, ruleID, err)
						retErr = err
						// try deleting other config
					}
				}
			}

			if err := k8sNet.deleteEpg(from.TenantName, from.GroupName, policyName); err != nil {
				npLog.Warnf("failed to delete epg: %+v", from)
				retErr = err
			} else {
				if err := k8sNet.deletePolicy(policyName); err != nil {
					npLog.Warnf("failed to delete policy: %s:%s", from.TenantName, from.GroupName)
					retErr = err
				}
			}
		}
	}

	// delete pod selector epg
	if err := k8sNet.deleteEpg(np.PodSelector.TenantName, np.PodSelector.GroupName, policyName); err != nil {
		npLog.Warnf("failed to delete epg: %+v", np.PodSelector)
		retErr = err
	} else {
		if err := k8sNet.deletePolicy(policyName); err != nil {
			npLog.Warnf("failed to delete policy: %s", np.PodSelector)
			retErr = err
		}
	}

	return retErr
}

func (k8sNet *k8sContext) getPolicyPorts(policyPort []v1beta1.NetworkPolicyPort) []k8sPolicyPorts {
	rules := []k8sPolicyPorts{}

	for _, pol := range policyPort {
		port := 0
		protocol := "TCP" // default

		if pol.Port != nil {
			port = pol.Port.IntValue()
		}

		if pol.Protocol != nil {
			protocol = string(*pol.Protocol)
		}

		npLog.Infof("ingress policy port: protocol: %v, port: %v", protocol, port)
		rules = append(rules, k8sPolicyPorts{Port: port, Protocol: protocol})
	}
	return rules
}

func (k8sNet *k8sContext) getIngressPodSelectorList(peers []v1beta1.NetworkPolicyPeer) ([]*k8sPodSelector, error) {
	peerPodSelector := []*k8sPodSelector{}

	if len(peers) <= 0 {
		return peerPodSelector, fmt.Errorf("empty pod selectors")
	}

	for _, from := range peers {
		if from.PodSelector != nil {
			s, err := k8sNet.getPodSelector(from.PodSelector.MatchLabels)
			//  don't apply partial policy.
			if err != nil {
				return []*k8sPodSelector{}, err
			}
			npLog.Infof("ingress policy pod-selector: %+v", s)
			peerPodSelector = append(peerPodSelector, s)
		}
	}
	return peerPodSelector, nil
}

func (k8sNet *k8sContext) getIngressPolicy(npIngress []v1beta1.NetworkPolicyIngressRule) ([]k8sIngress, error) {
	ingressRules := []k8sIngress{}

	if len(npIngress) <= 0 {
		return ingressRules, fmt.Errorf("no ingress rules")
	}

	for _, policy := range npIngress {
		rules := k8sNet.getPolicyPorts(policy.Ports)
		if len(rules) <= 0 {
			return ingressRules, fmt.Errorf("empty policy ports")
		}

		fromPodSelector, err := k8sNet.getIngressPodSelectorList(policy.From)
		//  don't apply partial policy.
		if err != nil {
			return []k8sIngress{}, err
		}
		ingressRules = append(ingressRules, k8sIngress{IngressRules: rules, IngressPodSelector: fromPodSelector})
	}
	return ingressRules, nil
}

func (k8sNet *k8sContext) getPodSelector(m map[string]string) (*k8sPodSelector, error) {
	PodSelector := k8sPodSelector{TenantName: defaultTenantName, NetworkName: defaultNetworkName, GroupName: defaultEpgName}

	// check tenant
	if _, err := k8sNet.contivClient.TenantGet(PodSelector.TenantName); err != nil {
		return nil, fmt.Errorf("tenant %s doesn't exist, %v", PodSelector.TenantName, err)
	}
	npLog.Info("Got tenant from Cotiv client")

	// check network
	if _, err := k8sNet.contivClient.NetworkGet(PodSelector.TenantName, PodSelector.NetworkName); err != nil {
		return nil, fmt.Errorf("network: +%v doesn't exist, %v", PodSelector, err)
	}
	npLog.Info("Got network from Cotiv client")

	podsList, err := k8sNet.k8sClientSet.CoreV1().Pods("kube-system").List(v1.ListOptions{LabelSelector: labels.SelectorFromSet(m).String()})
	if err != nil {
		npLog.Fatalf("failed to get Pods from  K8S Server, %v", err)
		return nil, err
	}
	npLog.Info("Got Pods Ips info From APIS server")
	for _, pod := range podsList.Items {
		PodSelector.PodIps = append(PodSelector.PodIps, pod.Status.PodIP)
		npLog.Info("Recv %s PodIp", pod.Status.PodIP)
	}
	npLog.Info("PodSelector %+v PodIp", PodSelector)

	return &PodSelector, err
}

func (k8sNet *k8sContext) createDefaultPolicy(tenantName string, epgName string) error {
	var err error

	if err = k8sNet.createPolicy(defaultTenantName, epgName); err != nil {
		return err
	}

	for _, direction := range []string{"in", "out"} {
		if err = k8sNet.createRule(&client.Rule{
			TenantName: tenantName,
			PolicyName: k8sutils.EpgNameToPolicy(epgName),
			RuleID:     k8sutils.DenyAllRuleID + direction,
			Priority:   k8sutils.DenyAllPriority,
			Direction:  direction,
			Action:     "allow",
		}); err != nil {
			return err
		}
	}
	return nil
}
