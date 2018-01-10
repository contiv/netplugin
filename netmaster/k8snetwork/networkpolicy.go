package networkpolicy

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/contivmodel/client"
	"github.com/contiv/netplugin/utils/k8sutils"
	"k8s.io/api/networking/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

const defaultTenantName = "default"
const defaultNetworkName = "net"
const defaultSubnet = "10.1.2.0/24"
const defaultEpgName = "ingress-group"
const defaultPolicyName = "ingress-policy"
const defaultRuleID = "1"

type k8sContext struct {
	k8sClientSet *kubernetes.Clientset
	contivClient *client.ContivClient
	isLeader     func() bool
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
	}() != nil {
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
	}() == nil {
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
	}() != nil {
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
	}() == nil {
		time.Sleep(time.Millisecond * 100)
	}
	return nil
}

func (k8sNet *k8sContext) createPolicy(policyName string) error {
	npLog.Infof("create policy %s", policyName)

	if _, err := k8sNet.contivClient.PolicyGet(defaultTenantName, policyName); err == nil {
		return nil
	}

	if err := k8sNet.contivClient.PolicyPost(&client.Policy{
		TenantName: defaultTenantName,
		PolicyName: policyName,
	}); err != nil {
		npLog.Errorf("failed to create policy %s", err)
		return err
	}

	for func() error {
		_, err := k8sNet.contivClient.PolicyGet(defaultTenantName, policyName)
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
	}() == nil {
		time.Sleep(time.Millisecond * 100)
	}
	return nil
}

func (k8sNet *k8sContext) createRule(policyName, ruleID, action string) error {
	npLog.Infof("create rule %s[%s] [%s]", policyName, ruleID, action)

	if val, err := k8sNet.contivClient.RuleGet(defaultTenantName, policyName, ruleID); err == nil {
		if val.Action != action {
			k8sNet.deleteRule(policyName, ruleID)
		} else {
			return nil
		}
	}

	if err := k8sNet.contivClient.RulePost(&client.Rule{
		TenantName: defaultTenantName,
		PolicyName: policyName,
		RuleID:     ruleID,
		Direction:  "in",
		Action:     action,
	}); err != nil {
		npLog.Errorf("failed to create rule-id [%s] %s", ruleID, err)
		return err
	}

	for func() error {
		_, err := k8sNet.contivClient.RuleGet(defaultTenantName, policyName, ruleID)
		return err
	}() != nil {
		time.Sleep(time.Millisecond * 100)
	}
	return nil
}

func (k8sNet *k8sContext) deleteRule(policyName, ruleID string) error {
	npLog.Infof("delete rule-id %s", ruleID)

	if _, err := k8sNet.contivClient.RuleGet(defaultTenantName, policyName, ruleID); err != nil {
		return nil
	}

	if err := k8sNet.contivClient.RuleDelete(defaultTenantName, policyName, ruleID); err != nil {
		npLog.Errorf("failed to delete rule %s[%s], %s", policyName, ruleID, err)
		return err
	}

	for func() error {
		_, err := k8sNet.contivClient.RuleGet(defaultTenantName, policyName, ruleID)
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

	if err = k8sNet.createPolicy(policyName); err != nil {
		npLog.Errorf("failed to update policy %s, %s", policyName, err)
		return
	}

	if err = k8sNet.createEpg(nwName, epgName, policyName); err != nil {
		npLog.Errorf("failed to update EPG %s, %s", epgName, err)
		return
	}

	if err = k8sNet.createRule(policyName, defaultRuleID, action); err != nil {
		npLog.Errorf("failed to update default rule, %s", err)
		return
	}
}

func (k8sNet *k8sContext) deleteDefaultIngressPolicy(ns string) {
	nwName := ns + "-" + defaultNetworkName
	policyName := ns + "-" + defaultPolicyName
	epgName := ns + "-" + defaultEpgName

	var err error

	if err = k8sNet.deleteRule(policyName, defaultRuleID); err != nil {
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
	case watch.Added, watch.Modified:
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
func InitK8SServiceWatch(listenAddr string, isLeader func() bool) error {
	npLog = log.WithField("k8s", "netpolicy")

	npLog.Infof("Create contiv client at http://%s", listenAddr)
	contivClient, err := client.NewContivClient("http://" + listenAddr)
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
