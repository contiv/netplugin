package networkpolicy

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/contivmodel/client"
	"github.com/contiv/netplugin/utils/k8sutils"
	v1 "k8s.io/api/core/v1"
	network_v1 "k8s.io/api/networking/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const defaultTenantName = "default"
const defaultNetworkName = "default-net"
const defaultSubnet = "10.1.2.0/24"

//const defaultEpgName = "ingress-group"
//const defaultEpgName = "default-epg"
const defaultEpgName = "default-group"
const defaultPolicyName = "ingress-policy"
const defaultRuleID = "1"
const defaultPolicyPriority = 2

type k8sPodSelector struct {
	TenantName  string //Attach Tenant
	NetworkName string //Attach network
	GroupName   string //Attach EPG
	PolicyName  string //Attach to policy
	labelPodMap map[string]map[string]bool
	podIps      map[string]string
}
type k8sPolicyPorts struct {
	Port     int
	Protocol string
}
type k8sNameSelector struct {
	nameSpaceSel string
}
type podCache struct {
	labeSelector string
	podNetwork   string
	podGroup     string
	podEpg       string
}
type k8sIPBlockSelector struct {
	subNetIps []string
}
type k8sIngress struct {
	IngressRules           []k8sPolicyPorts
	IngressPodSelector     []*k8sPodSelector
	IngressNameSelector    *k8sNameSelector
	IngressIpBlockSelector *k8sIPBlockSelector
}
type npPodInfo struct {
	nameSpace      string
	labelSelectors []string
	IP             string //??? should care for ipv6 address
}
type k8sNetworkPolicy struct {
	PodSelector *k8sPodSelector
	Ingress     []k8sIngress
}
type labelPolicySpec struct {
	policy []*k8sNetworkPolicy
}
type k8sContext struct {
	k8sClientSet *kubernetes.Clientset
	contivClient *client.ContivClient
	isLeader     func() bool
	//Policy Obj per  Policy Name
	networkPolicy map[string]*k8sNetworkPolicy
	//List of Rules Per Policy
	//	policyRules map[string][]string
	//List of Network configured
	network map[string]bool
	//List of EPG configured as set
	epgName map[string]bool
	//Default  policy Per EPG
	defaultPolicyPerEpg map[string]string
	//List of Policy Per EPG
	policyPerEpg map[string]map[string][]string
	//Cache table for given Pods
	//Policy Obj per  Policy Name
	nwPolicyPerNameSpace map[string]map[string]*k8sNetworkPolicy
}

var npLog *log.Entry

func getNetworkInfo() string {
	//XXX: Need expend this in version 2
	return defaultNetworkName
}
func getLabelDBkey(label, nameSpace string) string {
	return label + nameSpace
}
func getEpgInfo() string {
	//XXX: Need to expend in version 2
	return defaultEpgName
}
func getTenantInfo() string {
	return defaultTenantName
}

//Start Network Policy feature enabler
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

//Create network for given name string
func (k8sNet *k8sContext) createNetwork(nwName string) error {
	npLog.Infof("create network %s", nwName)
	if _, err := k8sNet.contivClient.NetworkGet(
		defaultTenantName,
		nwName); err == nil {
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
		_, err := k8sNet.contivClient.NetworkGet(
			defaultTenantName,
			nwName)
		return err
	}() != nil {
		//XXX:Should we really poll here ;
		//there would be chances on genuine error and
		//it cause infinity  loop
		time.Sleep(time.Millisecond * 100)
	}
	return nil
}

//Delete given network from contiv system
func (k8sNet *k8sContext) deleteNetwork(nwName string) error {
	npLog.Infof("delete network %s", nwName)

	if _, err := k8sNet.contivClient.NetworkGet(
		defaultTenantName,
		nwName); err != nil {
		return nil
	}

	if err := k8sNet.contivClient.NetworkDelete(
		defaultTenantName, nwName); err != nil {
		npLog.Errorf("failed to delete network %s, %s", nwName, err)
		return err
	}

	for func() error {
		_, err := k8sNet.contivClient.NetworkGet(
			defaultTenantName,
			nwName)
		return err
	}() == nil { //XXX: Same here as above
		time.Sleep(time.Millisecond * 100)
	}
	return nil
}

//Create EPG in context of given Network
func (k8sNet *k8sContext) createEpg(
	nwName,
	epgName string,
	policy []string) error {
	//npLog.Infof("create epg %s policy :%+v", epgName, policy)
	if err := k8sNet.contivClient.EndpointGroupPost(&client.EndpointGroup{
		TenantName:  defaultTenantName,
		NetworkName: nwName,
		GroupName:   epgName,
		Policies:    policy,
	}); err != nil {
		npLog.Errorf("failed to create epg %s, %s", epgName, err)
		return err
	}

	for func() error {
		_, err := k8sNet.contivClient.EndpointGroupGet(
			defaultTenantName,
			epgName)
		return err
	}() != nil { //XXX: Same as above
		time.Sleep(time.Millisecond * 100)
	}
	k8sNet.epgName[epgName] = true
	return nil
}

//Version 1 : Create default EPG at default-net network
func (k8sNet *k8sContext) createEpgInstance(nwName, epgName string) error {
	var err error
	policy := []string{defaultPolicyName}
	if err = k8sNet.createDefaultPolicy(
		defaultTenantName,
		epgName); err != nil {
		npLog.Errorf("failed  Default ingress policy EPG %v: err:%v ",
			epgName, err)
		return err
	}
	if err = k8sNet.createEpg(nwName, epgName, policy); err != nil {
		npLog.Errorf("failed to update EPG %v: err:%v ", epgName, err)
		return err
	}
	policyMap := k8sNet.policyPerEpg[epgName]
	if len(policyMap) <= 0 {
		policyMap = make(map[string][]string, 0)
	}
	//Build Default policy and Assign Default Rule
	policyMap[defaultPolicyName] = append(policyMap[defaultPolicyName],
		defaultRuleID)
	//Assign defult Policy to Newly Created Group
	k8sNet.policyPerEpg[epgName] = policyMap
	return err
}

//Delete EPG from given network
func (k8sNet *k8sContext) deleteEpg(networkname,
	epgName, policyName string) error {
	npLog.Infof("delete epg %s", epgName)
	if _, err := k8sNet.contivClient.
		EndpointGroupGet(defaultTenantName, epgName); err != nil {
		return nil
	}

	if err := k8sNet.contivClient.EndpointGroupDelete(
		defaultTenantName, epgName); err != nil {
		npLog.Errorf("failed to delete epg %s, %s", epgName, err)
		return err
	}

	for func() error {
		_, err := k8sNet.contivClient.
			EndpointGroupGet(defaultTenantName, epgName)
		return err
	}() == nil { //Same as above
		time.Sleep(time.Millisecond * 100)
	}

	delete(k8sNet.epgName, epgName)

	policyMap := k8sNet.policyPerEpg[epgName]
	for pName, policy := range policyMap {
		for _, ruleId := range policy {
			//XXX:Trigger Rule Delete Request in configured
			k8sNet.deleteRule(defaultTenantName, pName, ruleId)
		}
		k8sNet.deletePolicy(pName)
		delete(policyMap, pName)
	}
	delete(k8sNet.policyPerEpg, epgName)
	return nil
}

//Create policy contiv system
func (k8sNet *k8sContext) createPolicy(tenantName string,
	epgName, policyName string) error {
	if _, err := k8sNet.contivClient.
		PolicyGet(tenantName, policyName); err == nil {
		npLog.Infof("Policy:%v found contiv", policyName)
		return err
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
	policyMap := k8sNet.policyPerEpg[epgName]
	//Attach newly created policy to EPG
	policyMap[policyName] = []string{}
	return nil
}

//Delete given policy from Contiv system
func (k8sNet *k8sContext) deletePolicy(policyName string) error {
	npLog.Infof("delete policy %s", policyName)

	if _, err := k8sNet.contivClient.
		PolicyGet(defaultTenantName, policyName); err != nil {
		return nil
	}

	if err := k8sNet.contivClient.
		PolicyDelete(defaultTenantName, policyName); err != nil {
		npLog.Errorf("failed to delete policy %s, %s", policyName, err)
		return err
	}

	for func() error {
		_, err := k8sNet.contivClient.PolicyGet(defaultTenantName,
			policyName)
		return err
	}() == nil { //XXX:
		time.Sleep(time.Millisecond * 100)
	}
	return nil
}

//Post rule  to contiv if not exist
func (k8sNet *k8sContext) createRule(cRule *client.Rule) error {

	if val, err := k8sNet.contivClient.RuleGet(cRule.TenantName,
		cRule.PolicyName, cRule.RuleID); err == nil {
		if val.Action != cRule.Action {
			k8sNet.deleteRule(cRule.TenantName,
				cRule.PolicyName, cRule.RuleID)
		} else {
			npLog.Infof("Rule:%+v already exist", *cRule)
			return nil
		}
	}

	if err := k8sNet.contivClient.RulePost(cRule); err != nil {
		npLog.Errorf("failed to create rule: %s, %v", cRule.RuleID, err)
		return err
	}

	for func() error {
		_, err := k8sNet.contivClient.RuleGet(cRule.TenantName,
			cRule.PolicyName, cRule.RuleID)
		return err
	}() != nil {
		time.Sleep(time.Millisecond * 100)
	}
	return nil
}

//Delete rule from contiv system
func (k8sNet *k8sContext) deleteRule(tenantName string,
	policyName, ruleID string) error {
	npLog.Infof("Delete rule: %s:%s", ruleID, policyName)

	if _, err := k8sNet.contivClient.
		RuleGet(tenantName, policyName, ruleID); err != nil {
		return nil
	}

	if err := k8sNet.contivClient.
		RuleDelete(tenantName, policyName, ruleID); err != nil {
		npLog.Errorf("Failure rule del Ops:%s:%s,%v",
			ruleID, policyName, err)
		return err
	}

	for func() error {
		_, err := k8sNet.contivClient.
			RuleGet(tenantName, policyName, ruleID)
		return err
	}() == nil {
		time.Sleep(time.Millisecond * 100)
	}
	return nil
}

//Sub handler to process  Network Policy event from K8s srv
func (k8sNet *k8sContext) processK8sNetworkPolicy(
	opCode watch.EventType, np *network_v1.NetworkPolicy) {
	if np.Namespace == "kube-system" { //not applicable for system namespace
		return
	}

	npLog.Infof("Network Policy[%v]: %+v", opCode, *np)
	switch opCode {
	case watch.Added, watch.Modified:
		k8sNet.addNetworkPolicy(np)

	case watch.Deleted:
		k8sNet.delNetworkPolicy(np)
	}
}

//Sub Handler to process Pods events from K8s srv
func (k8sNet *k8sContext) processK8sPods(opCode watch.EventType, pod *v1.Pod) {
	if pod.Namespace == "kube-system" { //not applicable for system namespace
		return
	}
	//K8s pods event doesn't provide Ips information in Add/delete type
	//opcode
	npLog.Infof("K8s Event : POD [%s],NameSpace[%v] ,Label:[%+v],IPs:[%v]",
		opCode, pod.ObjectMeta.Namespace,
		pod.ObjectMeta.Labels, pod.Status.PodIP)

	if _, ok := k8sNet.
		nwPolicyPerNameSpace[pod.ObjectMeta.Namespace]; !ok {
		npLog.Infof("Pod doesn't match policy namespace")
		return
	}
	switch opCode {
	case watch.Added, watch.Modified, watch.Deleted:
		if pod.ObjectMeta.DeletionTimestamp != nil &&
			len(pod.Status.PodIP) > 0 {
			//K8s Srv notify Pods delete case as part of modify
			//event by specifying DeletionTimeStamp
			// Pod Delete event doesn't carry Pod Ips info
			//therefore Using Modify event to manipulate future
			//delete event
			k8sNet.processPodDeleteEvent(pod)
		} else if len(pod.Status.PodIP) > 0 {
			//Pod event without timeDeletion with Pod ip consider
			//as pod add event
			k8sNet.processPodAddEvent(pod)
		}

	}
}

//Parse Pod Info from receive Pod events
func parsePodInfo(pod *v1.Pod) npPodInfo {
	var pInfo npPodInfo
	pInfo.nameSpace = pod.ObjectMeta.Namespace
	for key, val := range pod.ObjectMeta.Labels {
		pInfo.labelSelectors =
			append(pInfo.labelSelectors, (key + "=" + val))
	}
	pInfo.IP = pod.Status.PodIP
	return pInfo
}

//Get  Network Policy object sets which ToSpec labelMap information match
//with given pods labelMap
func (k8sNet *k8sContext) getMatchToSpecPartNetPolicy(
	podInfo npPodInfo) []*k8sNetworkPolicy {
	var toPartPolicy []*k8sNetworkPolicy
	nwPolicyMap, ok := k8sNet.nwPolicyPerNameSpace[podInfo.nameSpace]
	if !ok {
		npLog.Warnf("No NetworkPolicy for NameSpace:%v",
			podInfo.nameSpace)
		return nil
	}
	for _, nwPol := range nwPolicyMap {
		for _, label := range podInfo.labelSelectors {
			//Collect networkPolicy object which match with pods
			//Labels
			if _, ok := nwPol.PodSelector.labelPodMap[label]; ok {
				toPartPolicy = append(toPartPolicy, nwPol)
				//	npLog.Infof("policy :%+v", nwPol)
				break
			}
		}
	}
	return toPartPolicy
}

//Get Network Policy object sets which FromSpec, labelMap information match
//with given pods labelMap
func (k8sNet *k8sContext) getMatchFromSpecPartNetPolicy(
	podInfo npPodInfo) []*k8sNetworkPolicy {

	var fromPartPolicy []*k8sNetworkPolicy
	//NetworkPolicy master object   on pods Namespace
	nwPolicyMap, ok := k8sNet.nwPolicyPerNameSpace[podInfo.nameSpace]
	if !ok {
		npLog.Infof("Pod namespace doesn't have any policy config")
		return nil
	}
	//Build list of networkPolicy object which fromSpec belongs to given
	//pods Info
	for _, l := range podInfo.labelSelectors {
		for _, nwPol := range nwPolicyMap {
			for _, ingress := range nwPol.Ingress {
				//PodSelector on FromSpec part of policy Object
				for _, podSelector := range ingress.
					IngressPodSelector {
					npLog.Infof("labelMap:%+v",
						podSelector.labelPodMap)
					if _, ok :=
						podSelector.labelPodMap[l]; ok {
						fromPartPolicy =
							append(fromPartPolicy,
								nwPol)
						break
					}
				}
			}
		}
	}
	return fromPartPolicy
}

//Consolidate all Ips belongs to Label for Pod Selector object
func (k8sNet *k8sContext) updatePodSelectorPodIps(
	podSelector *k8sPodSelector) {
	if podSelector == nil {
		npLog.Errorf(" nil pod Selector  reference")
		return
	}
	for _, ipMap := range podSelector.labelPodMap {
		for ip := range ipMap {
			podSelector.podIps[ip] = ip
		}
	}
	npLog.Infof("Update Pod SelectorPodIps %+v", podSelector.labelPodMap)
	return
}

//Process Pod Delete Event from K8s Srv
func (k8sNet *k8sContext) processPodDeleteEvent(pod *v1.Pod) {
	podInfo := parsePodInfo(pod)
	labelList := podInfo.labelSelectors
	rmIps := []string{podInfo.IP}
	npLog.Infof("POD [Delete] for pods:%+v", rmIps)
	//find All configured Network Policy object which given pods LableMap
	//match

	toSpecNetPolicy := k8sNet.getMatchToSpecPartNetPolicy(podInfo)
	if len(toSpecNetPolicy) > 0 {
		for _, nw := range toSpecNetPolicy {
			//remove Given Pods Ips from List
			delete(nw.PodSelector.podIps, podInfo.IP)
			//revisit all configur label Ips list in pod
			//Selector object
			k8sNet.getIpListMatchPodSelector(nw.PodSelector,
				labelList, podInfo.IP)
			//Remove Pods Ips from Label Map Table
			//Update PodSelector podIps list
			k8sNet.updatePodSelectorPodIps(nw.PodSelector)

			rList := k8sNet.
				buildRulesFromIngressSpec(nw,
					nw.PodSelector.PolicyName)
			npLog.Infof("remove Pods ToSpec :%+v", rmIps)
			ruleList := k8sNet.
				finalIngressNetworkPolicyRule(
					nw, rmIps, *rList, false)
			npLog.Infof("Delete To Spec rule:%+v", ruleList)
			npLog.Infof("rmove PodIps from ToSpec", podInfo.IP)
		}
	} else { //Pods  belongs fromPart of Spec
		fromPartPolicy := k8sNet.getMatchFromSpecPartNetPolicy(podInfo)
		npLog.Infof("Delete Pod belong FromSec part:%+v",
			fromPartPolicy)
		if len(fromPartPolicy) > 0 {
			npLog.Infof("remove PodIps:%v fromSpec part of Policy",
				rmIps)
			for _, nw := range fromPartPolicy {
				k8sNet.rmIpFromSpecPodSelector(nw,
					labelList, podInfo.IP)
				rList := k8sNet.
					buildIngressRuleToPodSelector(nw, rmIps,
						nw.PodSelector.PolicyName)
				npLog.Infof("Ingress Rule :%+v", *rList)
				npLog.Infof("Pods Info in To Spec :%+v", rmIps)
				ipList := getIpMapToSlice(nw.PodSelector.podIps)
				ruleList := k8sNet.
					finalIngressNetworkPolicyRule(nw,
						ipList, *rList, false)
				npLog.Infof("Pod rules:%+v", ruleList)
			}
		}
	}
}
func getIpMapToSlice(m map[string]string) []string {
	ips := []string{}
	for ip := range m {
		ips = append(ips, ip)
	}
	return ips
}
func (k8sNet *k8sContext) UpdateIpListFromSpecfromLabel(nw *k8sNetworkPolicy,
	label []string, ip string) {
	for _, ingress := range nw.Ingress {
		for _, podSelector := range ingress.IngressPodSelector {
			for _, l := range label {
				if ipMap, ok :=
					podSelector.labelPodMap[l]; ok {
					ipMap[ip] = true
				}
			}
			//Rebuild PodSelector PodIps
			k8sNet.updatePodSelectorPodIps(podSelector)
			npLog.Infof("Update PodIps into FromSpecPod:%+v",
				podSelector)
		}
	}
	return
}

//Remove Give Pod Ips fromSpec Object of Network Policy
func (k8sNet *k8sContext) rmIpFromSpecPodSelector(
	nw *k8sNetworkPolicy, label []string, ip string) {
	for _, ingress := range nw.Ingress {
		for _, podSelector := range ingress.IngressPodSelector {
			npLog.Infof("podSelector:%+v", podSelector)
			//remove ips from PodSelector Object
			delete(podSelector.podIps, ip)
			for _, l := range label {
				if ipMap, ok :=
					podSelector.labelPodMap[l]; ok {
					delete(ipMap, ip)
					npLog.Infof("remove Pod Ips:%v FromSpec map:%+v",
						ip, ipMap)
				}
			}
			//Rebuild PodSelector PodIps
			k8sNet.updatePodSelectorPodIps(podSelector)
		}
	}
	return
}

//Add Pods Ips and readjuct Pod selector podIps list
func (k8sNet *k8sContext) addPodIpsToSpecPodSelector(nw *k8sNetworkPolicy,
	label []string, ip string) {
	for _, l := range label {
		if ipMap, ok := nw.PodSelector.labelPodMap[l]; ok {
			ipMap[ip] = true
		}
	}
	//Recalculate PodSelector PodIps list
	for _, lMap := range nw.PodSelector.labelPodMap {
		//At each Label Walk all its Ips
		for ip := range lMap {
			nw.PodSelector.podIps[ip] = ip
		}
	}
	return
}

//return list of Ips which belongs to given lable in PodSelector Object
func (k8sNet *k8sContext) getIpListMatchPodSelector(podSelector *k8sPodSelector,
	label []string, ip string) {
	for _, l := range label {
		if ipMap, ok := podSelector.labelPodMap[l]; ok {
			ipMap[ip] = true
		}
	}
	return
}

//Process Pod Add event
func (k8sNet *k8sContext) processPodAddEvent(pod *v1.Pod) {
	if pod.Status.PodIP == "" {
		return
	}
	//Get Pods Ips and its Pod selector label
	podInfo := parsePodInfo(pod)
	npLog.Infof("POD [ADD] request for pod %+v", podInfo)
	//get programmed  NetworkPolicy for recv Pod Namespace
	//find All configured Policy which is having given pods Label selector
	// is part of To spec
	toPartPolicy := k8sNet.getMatchToSpecPartNetPolicy(podInfo)
	npLog.Infof("ToPartSpec:%+v", toPartPolicy)
	//Pods Belongs to To Spec part
	podIps := []string{podInfo.IP}

	if len(toPartPolicy) > 0 {
		npLog.Infof("Recv Pod belongs to ToSpec part of Policy")
		for _, nw := range toPartPolicy {
			rList := k8sNet.buildRulesFromIngressSpec(nw,
				nw.PodSelector.PolicyName)
			if len(*rList) > 0 {
				npLog.Infof("Pods Info in To Spec :%+v",
					nw.PodSelector.podIps)
				if _, ok := nw.PodSelector.podIps[podInfo.IP]; ok {
					npLog.Infof("pod Ips already exist", podInfo.IP)
					continue
				}
				ruleList := k8sNet.finalIngressNetworkPolicyRule(
					nw, podIps, *rList, true)
				npLog.Infof("To Spec Pod rules:%+v", ruleList)
				npLog.Infof("podInf.labelSelectors:%+v",
					podInfo.labelSelectors)
			}
			k8sNet.addPodIpsToSpecPodSelector(nw,
				podInfo.labelSelectors, podInfo.IP)
		}
	} else {
		//Build fromPodSelector List
		fromPartPolicy := k8sNet.getMatchFromSpecPartNetPolicy(podInfo)
		//Build Rules and update to OVS
		for _, nw := range fromPartPolicy {
			npLog.Infof("fromPartPolicy:%+v", *nw)
			rList := k8sNet.buildIngressRuleToPodSelector(nw,
				podIps,
				nw.PodSelector.PolicyName)
			if len(*rList) > 0 {
				npLog.Infof("Ingress Rule :%+v", *rList)
				npLog.Infof("Pods Info in To Spec :%+v",
					nw.PodSelector.podIps)
				ipList := getIpMapToSlice(nw.PodSelector.podIps)
				ruleList := k8sNet.finalIngressNetworkPolicyRule(nw,
					ipList, *rList, true)
				npLog.Infof("Pod rules:%+v", ruleList)
			}
			k8sNet.UpdateIpListFromSpecfromLabel(nw,
				podInfo.labelSelectors, podInfo.IP)
		}
	}
}

//Handler to process APIs Server Watch event
func (k8sNet *k8sContext) processK8sEvent(opCode watch.EventType,
	eventObj interface{}) {
	//Only Leader will process events
	if k8sNet.isLeader() != true {
		return
	}

	switch objType := eventObj.(type) {

	case *v1.Pod:
		k8sNet.processK8sPods(opCode, objType)
	case *network_v1.NetworkPolicy:
		k8sNet.processK8sNetworkPolicy(opCode, objType)
	default:
		npLog.Infof("Unwanted event from K8s evType:%v objType:%v",
			opCode, objType)
	}
}

func (k8sNet *k8sContext) watchK8sEvents(errChan chan error) {
	var selCase []reflect.SelectCase

	// wait to become leader
	for k8sNet.isLeader() != true {
		time.Sleep(time.Millisecond * 100)
	}
	//Set Watcher for Network Policy resource
	npWatch, err := k8sNet.k8sClientSet.Networking().NetworkPolicies("").Watch(meta_v1.ListOptions{})
	if err != nil {
		errChan <- fmt.Errorf("failed to watch network policy, %s", err)
		return
	}

	selCase = append(selCase, reflect.SelectCase{Dir: reflect.SelectRecv,
		Chan: reflect.ValueOf(npWatch.ResultChan())})
	//Set watcher for Pods resource
	podWatch, _ := k8sNet.k8sClientSet.CoreV1().
		Pods("").Watch(meta_v1.ListOptions{})

	selCase = append(selCase, reflect.SelectCase{Dir: reflect.SelectRecv,
		Chan: reflect.ValueOf(podWatch.ResultChan())})
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
	//nwoPolicyDb := make(map[string]k8sNetworkPolicy, 0)
	kubeNet := k8sContext{
		contivClient:  contivClient,
		k8sClientSet:  k8sClientSet,
		isLeader:      isLeader,
		networkPolicy: make(map[string]*k8sNetworkPolicy, 0),
		//lookup table for Configured Network;
		network: make(map[string]bool, 0),
		//lookup table for Configured Policy per EPG
		defaultPolicyPerEpg: make(map[string]string, 0),
		epgName:             make(map[string]bool, 0),
		policyPerEpg:        make(map[string]map[string][]string, 0),
		//policyRules:          make(map[string][]string, 0),
		nwPolicyPerNameSpace: make(map[string]map[string]*k8sNetworkPolicy, 0),
	}

	//Trigger default epg : = default-group
	kubeNet.createEpgInstance(defaultNetworkName, defaultEpgName)

	go kubeNet.handleK8sEvents()
	return nil
}
func getLabelSelector(key, val string) string {
	return (key + "=" + val)
}

func (k8sNet *k8sContext) addNetworkPolicy(np *network_v1.NetworkPolicy) {
	//check if given Policy already exist
	if _, ok := k8sNet.networkPolicy[np.Name]; ok {
		npLog.Warnf("Delete existing network policy: %s !", np.Name)
		k8sNet.delNetworkPolicy(np)
	}
	//build ToSpec PodSelector Obj
	npPodSelector, err := k8sNet.parsePodSelectorInfo(
		np.Spec.PodSelector.MatchLabels,
		np.Namespace)
	if err != nil {
		npLog.Warnf("ignore network policy: %s, %v", np.Name, err)
		return
	}
	//Set policy name ToSpec podSelector Obj
	npPodSelector.PolicyName = np.Name
	//Save recv Label map info
	npLog.Infof("Network  policy [%s] pod-selector: %+v",
		np.Name, npPodSelector)

	//Parse Ingress Policy rules
	IngressRules, err :=
		k8sNet.parseIngressPolicy(np.Spec.Ingress,
			np.Namespace)
	if err != nil {
		npLog.Warnf("ignore network policy: %s, %v", np.Name, err)
		return
	}
	nwPolicy := k8sNetworkPolicy{
		PodSelector: npPodSelector,
		Ingress:     IngressRules}

	npLog.Info("Apply NW_Policy[%s] podSelector:%+v Ingress:%+v",
		np.Name, npPodSelector, IngressRules)

	//Push policy info to ofnet agent
	if err := k8sNet.applyContivNetworkPolicy(&nwPolicy); err != nil {
		npLog.Errorf("[%s] failed to configure policy, %v",
			np.Name, err)
		return
	}
	//Cache configued NetworkPolicy obj using policy Name
	k8sNet.networkPolicy[np.Name] = &nwPolicy
	//cache networkPolicy obj per namespace
	if _, ok := k8sNet.nwPolicyPerNameSpace[np.Namespace]; !ok {
		k8sNet.nwPolicyPerNameSpace[np.Namespace] =
			make(map[string]*k8sNetworkPolicy, 0)
	}
	nwPolicyMap, _ := k8sNet.nwPolicyPerNameSpace[np.Namespace]
	nwPolicyMap[np.Name] = &nwPolicy

	//append(k8sNet.nwPolicyPerNameSpace[np.Name], &nwPolicy)
	npLog.Infof("Add network policy in per NameSpace:%v",
		k8sNet.nwPolicyPerNameSpace[np.Namespace])
}

//Build partial rule list using FromSpec PodSelector information
func (k8sNet *k8sContext) buildRulesFromIngressSpec(
	np *k8sNetworkPolicy,
	policyName string) (lRules *[]client.Rule) {
	var listRules []client.Rule
	for _, ingress := range np.Ingress {
		isPortsCfg := false
		if len(ingress.IngressRules) > 0 {
			isPortsCfg = true
			//Is Port Cfg included into From Ingress Spec
		}
		for _, podSec := range ingress.IngressPodSelector {
			for _, fromIp := range podSec.podIps {
				rule := client.Rule{
					TenantName:    np.PodSelector.TenantName,
					PolicyName:    np.PodSelector.PolicyName,
					FromIpAddress: fromIp,
					Priority:      defaultPolicyPriority,
					Direction:     "in",
					Action:        "allow"}
				//If Port cfg enable
				if isPortsCfg {
					for _, p := range ingress.IngressRules {
						k8sNet.appendPolicyPorts(&rule, p)
						listRules = append(listRules, rule)
					}
				} else {
					listRules = append(listRules, rule)
				}
			}
		}
	}
	return &listRules
}

//Build Partial Rules based on  FromSpec Pod IPs list
func (k8sNet *k8sContext) buildIngressRuleToPodSelector(
	np *k8sNetworkPolicy, //Network Policy object
	from []string, //FromSpec Ips List
	policyName string) (lRules *[]client.Rule) {

	//npLog.Infof("From:%v", from)
	var listRules []client.Rule
	//Walk info Ingress Policy FromSpec PodSelector
	for _, ingress := range np.Ingress {
		isPortsCfg := false
		if len(ingress.IngressRules) > 0 {
			isPortsCfg = true
			//Is Port Cfg included into From Ingress Spec
		}
		//Attach All fromSpec IpsList
		for _, fromIp := range from {
			rule := client.Rule{
				TenantName:    np.PodSelector.TenantName,
				PolicyName:    np.PodSelector.PolicyName,
				FromIpAddress: fromIp,
				Priority:      defaultPolicyPriority,
				Direction:     "in",
				Action:        "allow"}
			if isPortsCfg {
				for _, port := range ingress.IngressRules {
					//Add port Info into Rule
					k8sNet.appendPolicyPorts(&rule, port)
					listRules = append(listRules, rule)
				}
			} else {
				listRules = append(listRules, rule)
			}
		}
	}
	return &listRules
}

//final Build Rules by linking from spec to To spec podSelector
func (k8sNet *k8sContext) finalIngressNetworkPolicyRule(np *k8sNetworkPolicy,
	toPodIPs []string,
	ingressRules []client.Rule,
	isAdd bool) *[]client.Rule {

	var err error
	ruleList := ingressRules

	//policyCtx := k8sNet.policyRules[np.PodSelector.PolicyName]

	//Ingress Spec To section Pods
	for _, toIps := range toPodIPs {
		//npLog.Infof("ruleList:%v", ruleList)
		//Rebuild Rule List to add To Ips
		for _, rule := range ruleList {
			//Update To src Ip section in Rule
			rule.ToIpAddress = toIps
			//Generate RuleID :XXX: Should look for better approach
			rule.RuleID = k8sutils.PolicyToRuleIDUsingIps(
				toIps, rule.FromIpAddress,
				rule.Port, rule.Protocol,
				np.PodSelector.PolicyName)

			npLog.Infof("RulID:%v", rule.RuleID)
			//Update Policy Name cache with policy Id
			if isAdd {
				if err = k8sNet.createRule(&rule); err != nil {
					npLog.Errorf("failed: rules in-policy %+v, %v",
						np.PodSelector, err)
					return nil
				}
				//Update RuleID in cache Db
				//XXX:Should use Hash Set instead of slice
				// to aviod duplicate Ruleid insertion
				//policyCtx = append(policyCtx, rule.RuleID)
			} else { //Policy Delete
				if err = k8sNet.deleteRule(rule.TenantName,
					rule.PolicyName,
					rule.RuleID); err != nil {
					npLog.Errorf("failed: del in-policy %+v, %v",
						np.PodSelector, err)
					return nil
				}
			}
			//Get Policy Map Table for given EPG
			policyMap := k8sNet.policyPerEpg[np.PodSelector.GroupName]
			//Get Rule Id Sets for given Policy
			policy := policyMap[np.PodSelector.PolicyName]
			if isAdd {
				policy = append(policy, rule.RuleID)
				npLog.Infof("RuleId:%v is added into Policy:%v",
					rule.RuleID, np.PodSelector.PolicyName)
			} else {
				for idx, r := range policy {
					if r == rule.RuleID {
						policy = append(policy[0:idx],
							policy[idx+1:]...)
						npLog.Infof("RuleId :%v deleted ",
							rule.RuleID)
					}
				}
			}
		}
	}
	return &ruleList
}

//Build policy ,rules and attached it to EPG
func (k8sNet *k8sContext) applyContivNetworkPolicy(np *k8sNetworkPolicy) error {
	var err error
	// reset policy to deny on any error
	policyResetOnErr := func(tenantName, groupName string) {
		if err != nil {
			//k8sNet.resetPolicy(tenantName, groupName)
			npLog.Warnf("Need to reset the policy")
		}
	}
	//endPoint Group Lookup(EPG)
	if _, ok := k8sNet.epgName[np.PodSelector.GroupName]; !ok {
		//Create EPG then
		npLog.Infof("EPG :%v doesn't exist create now!",
			np.PodSelector.GroupName)
		if err = k8sNet.createEpgInstance(
			np.PodSelector.NetworkName,
			np.PodSelector.GroupName); err != nil {
			npLog.Errorf("failed to create EPG %s ", err)
			return err
		}
	}
	//Get PolicyMap using EPG
	policyMap := k8sNet.policyPerEpg[np.PodSelector.GroupName]
	//Check if Policy is already programmed in EPG or not
	if _, ok := k8sNet.networkPolicy[np.PodSelector.PolicyName]; !ok {
		//Create Policy and published to ofnet controller
		if err = k8sNet.createPolicy(
			defaultTenantName,
			np.PodSelector.GroupName,
			np.PodSelector.PolicyName); err != nil {
			npLog.Errorf("Failed to create Policy :%v",
				np.PodSelector.PolicyName)
			return err
		}
		//Cache K8s Configure Policy
		k8sNet.networkPolicy[np.PodSelector.PolicyName] = np

		//Update Policy Instance in policyMap
		policyMap[np.PodSelector.PolicyName] = []string{}
		attachPolicy := []string{}
		for policyN := range policyMap {
			attachPolicy = append(attachPolicy, policyN)
		}
		//Update EPG with New Policy
		if err = k8sNet.createEpg(
			np.PodSelector.NetworkName,
			np.PodSelector.GroupName, attachPolicy); err != nil {
			npLog.Errorf("failed to update EPG %s ", err)
			return err
		}
	} else {
		//XXX: Need check if policy rules are still same or not
		npLog.Warnf("Policy:%v already exist ",
			np.PodSelector.PolicyName)
	}

	defer policyResetOnErr(
		np.PodSelector.TenantName,
		np.PodSelector.GroupName)

	//Build Ingress rules list
	rList := k8sNet.buildRulesFromIngressSpec(np,
		np.PodSelector.PolicyName)

	npLog.Infof("Build Rules Ingress Spec:%+v, rList:%+v",
		np.Ingress, *rList)

	ipList := getIpMapToSlice(np.PodSelector.podIps)

	npLog.Infof("Pods Info in To Spec :%+v", np.PodSelector.podIps)

	ruleList := k8sNet.finalIngressNetworkPolicyRule(np, ipList,
		*rList, true)
	npLog.Infof("final rules :%v", ruleList)
	return nil
}
func (k8sNet *k8sContext) appendPolicyPorts(rules *client.Rule, ports k8sPolicyPorts) {
	if len(ports.Protocol) > 0 {
		rules.Protocol = strings.ToLower(ports.Protocol)
	}
	if ports.Port != 0 {
		rules.Port = ports.Port
	}
	return
}
func (k8sNet *k8sContext) createUpdateRuleIds(rules *client.Rule) string {
	ruleID := rules.FromIpAddress + rules.ToIpAddress
	if rules.Port != 0 {
		ruleID += strconv.Itoa(rules.Port)
	}
	if len(rules.Protocol) > 0 {
		ruleID += rules.Protocol
	}
	rules.RuleID = ruleID
	return ruleID
}
func (k8sNet *k8sContext) delNetworkPolicy(np *network_v1.NetworkPolicy) {
	npLog.Infof("Delete network policy: %s", np.Name)
	policy, ok := k8sNet.networkPolicy[np.Name]
	if !ok {
		npLog.Errorf("network policy: %s doesn't exist", np.Name)
		return
	}

	if err := k8sNet.cleanupContivNetworkPolicy(policy); err != nil {
		npLog.Errorf("failed to delete network policy: %s, %v", np.Name, err)
		return
	}
	//Remove PolicyId from Policy Db
	delete(k8sNet.networkPolicy, np.Name)

	//Cleanup Per NameSpace nwPolicy obj
	if nwPolicyMap, ok := k8sNet.nwPolicyPerNameSpace[np.Namespace]; ok {
		delete(nwPolicyMap, np.Name)
	}
	npLog.Infof("Delete Policy:%s from contiv DB ", np.Name)
}
func (k8sNet *k8sContext) cleanupContivNetworkPolicy(np *k8sNetworkPolicy) error {
	var retErr error
	policyName := np.PodSelector.PolicyName
	epg := np.PodSelector.GroupName
	policyMap, ok := k8sNet.policyPerEpg[epg]
	if !ok {
		npLog.Errorf("Failed to find epg Policy")
		return fmt.Errorf("failed to find epg ")
	}
	npLog.Infof("Cleanup policyName:%v PolicyPtr:%+v its rules", policyName,
		policyMap)
	for _, ruleID := range policyMap[policyName] { //Walk for all configured Rules
		npLog.Infof("Delete RulID:%v from policy:%v", ruleID, policyName)
		if err := k8sNet.deleteRule(np.PodSelector.TenantName, policyName, ruleID); err != nil {
			npLog.Warnf("failed to delete policy: %s rule: %s, %v",
				policyName, ruleID, err)
			retErr = err
		}
	}
	delete(policyMap, policyName)
	attachPolicy := []string{}
	for policyN := range policyMap {
		attachPolicy = append(attachPolicy, policyN)
	}
	//Unlink Policy From EPG
	if err := k8sNet.createEpg(np.PodSelector.NetworkName,
		np.PodSelector.GroupName, attachPolicy); err != nil {
		npLog.Errorf("failed to update EPG %s, %s",
			np.PodSelector.GroupName, err)
		retErr = err
	}
	//Delete Policy
	if err := k8sNet.deletePolicy(policyName); err != nil {
		npLog.Warnf("failed to delete policy: %s",
			np.PodSelector.TenantName)
		retErr = err
	}
	npLog.Infof("Delete policy:%v ", policyName)
	return retErr
}

//parse policy Ports information
func (k8sNet *k8sContext) getPolicyPorts(
	policyPort []network_v1.NetworkPolicyPort) []k8sPolicyPorts {
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
		npLog.Infof("ingress policy port: protocol: %v, port: %v",
			protocol, port)
		rules = append(rules,
			k8sPolicyPorts{Port: port,
				Protocol: protocol})
	}
	return rules
}

func (k8sNet *k8sContext) getIngressPodSelectorList(
	peers []network_v1.NetworkPolicyPeer,
	nameSpace string) ([]*k8sPodSelector, error) {
	peerPodSelector := []*k8sPodSelector{}

	npLog.Infof("Ingress Policy Peer Info:%+v", peers)

	if len(peers) <= 0 {
		return peerPodSelector, fmt.Errorf("empty pod selectors")
	}

	for _, from := range peers {
		//Currently Support for PodSelector
		if from.PodSelector != nil {
			s, err := k8sNet.parsePodSelectorInfo(
				from.PodSelector.MatchLabels, nameSpace)
			if err != nil {
				return []*k8sPodSelector{}, err
			}
			npLog.Infof("Ingress policy pod-selector: %+v", s)
			peerPodSelector = append(peerPodSelector, s)
		}
	}
	return peerPodSelector, nil
}

//Build Ingress Policy obj
func (k8sNet *k8sContext) parseIngressPolicy(
	npIngress []network_v1.NetworkPolicyIngressRule,
	nameSpace string) ([]k8sIngress, error) {

	ingressRules := []k8sIngress{}
	//npLog.Infof("Recv Ingress Policy:=%+v", npIngress)
	if len(npIngress) <= 0 {
		return ingressRules, fmt.Errorf("no ingress rules")
	}
	//Walk in all received Ingress Policys
	for _, policy := range npIngress {
		rules := k8sNet.getPolicyPorts(policy.Ports)
		//build Ingress PodSelector obj
		fromPodSelector, err := k8sNet.
			getIngressPodSelectorList(policy.From, nameSpace)
		if err != nil {
			return []k8sIngress{}, err
		}
		//npLog.Infof("fromPodSelector:%+v", fromPodSelector)
		ingressRules = append(ingressRules,
			k8sIngress{IngressRules: rules,
				IngressPodSelector: fromPodSelector})
	}
	return ingressRules, nil
}

func (k8sNet *k8sContext) getPodsIpsSetUsingLabel(m map[string]string,
	nameSpace string) ([]string, error) {
	var ipList []string
	// labels.Parser
	labelSectorStr := labels.SelectorFromSet(labels.Set(m)).String()
	//Quary to K8s Api server for pods of given Label selector
	podsList, err := k8sNet.k8sClientSet.CoreV1().
		Pods(nameSpace).
		List(meta_v1.ListOptions{LabelSelector: labelSectorStr})
	if err != nil {
		npLog.Fatalf("failed to get Pods from  K8S Server, %v", err)
		return nil, err
	}
	for _, pod := range podsList.Items {
		ipList = append(ipList, pod.Status.PodIP)
	}
	return ipList, nil
}
func (k8sNet *k8sContext) initPodSelectorCacheTbl(m map[string]string,
	podSelector *k8sPodSelector) error {
	if podSelector == nil {
		return fmt.Errorf("Passe Nil Pod Selector reference")
	}
	//XXX:Don't confused PodSelector with Pod: PodSelector object keeps all
	//attched label Ips
	if len(podSelector.podIps) <= 0 {
		podSelector.podIps = make(map[string]string, 0)
		npLog.Infof("Init PodSelector podIp table:%v",
			podSelector.labelPodMap)
	}
	//PodSelector: Keep track of all its label
	if len(podSelector.labelPodMap) <= 0 {
		podSelector.labelPodMap = make(map[string]map[string]bool, 0)
		for key, val := range m {
			lkey := getLabelSelector(key, val)
			podSelector.labelPodMap[lkey] = make(map[string]bool, 0)
		}
		npLog.Infof("Init PodSelector Map table:%v",
			podSelector.labelPodMap)
	}
	return nil
}

//Create podSelector object and Init its attributes i.e podIps , label etc
func (k8sNet *k8sContext) parsePodSelectorInfo(m map[string]string,
	nameSpace string) (*k8sPodSelector, error) {

	PodSelector := k8sPodSelector{
		TenantName:  getTenantInfo(),
		NetworkName: getNetworkInfo(),
		GroupName:   getEpgInfo()}

	npLog.Infof("Build PodSelector Info using Label:%+v", m)

	labelSectorStr := labels.SelectorFromSet(labels.Set(m)).String()

	//Quary to K8s Api server using label Selector to get Pods list
	podsList, err := k8sNet.k8sClientSet.CoreV1().
		Pods(nameSpace).
		List(meta_v1.ListOptions{LabelSelector: labelSectorStr})
	if err != nil {
		npLog.Fatalf("failed to get Pods from  K8S Server, %v", err)
		return nil, err
	}
	if err = k8sNet.initPodSelectorCacheTbl(m, &PodSelector); err != nil {
		return nil, err
	}
	//Build mapping for Label To PodIP
	for _, pod := range podsList.Items {
		for key, val := range pod.ObjectMeta.Labels {
			lkey := getLabelSelector(key, val)
			npLog.Infof("Update label Selector key:%v", lkey)
			if ipMap, ok := PodSelector.labelPodMap[lkey]; ok {
				//Setup IpMap Tbl
				ipMap[pod.Status.PodIP] = true
			}
		}
	}
	//Recalculate Podselector Ips
	k8sNet.updatePodSelectorPodIps(&PodSelector)
	npLog.Info("PodSelector: %+v", PodSelector)
	return &PodSelector, err
}

//Update PodSelector Label IP mapping
func (k8sNet *k8sContext) updatePodSelectorLabelIPMap(
	podSelector *k8sPodSelector,
	labelSelector string,
	ipList []string,
	isAdd bool) {
	//Check Nil
	if podSelector == nil {
		npLog.Infof("Nil Pod Selector")
		return
	}
	if ipMap, ok := podSelector.labelPodMap[labelSelector]; ok {
		for _, ip := range ipList {
			if isAdd { //Add Pods
				ipMap[ip] = true
			} else { //remove Pods
				delete(ipMap, ip)
			}
		}
		npLog.Infof(" Pod Ips After Update Pod Selector:%+v event:%v",
			podSelector, isAdd)
	}
	return
}

//Create default deny rules for given policy
func (k8sNet *k8sContext) createPolicyWithDefaultRule(tenantName string,
	epgName, policyName string) error {
	var err error
	npLog.Infof("Create  default policy for epg:%s policy:%s",
		epgName, policyName)

	if err = k8sNet.createPolicy(defaultTenantName,
		epgName, policyName); err != nil {
		npLog.Infof("Failed to create Policy :%v", policyName)
		return err
	}

	//Add default rule into policy
	if err = k8sNet.createRule(&client.Rule{
		TenantName:        tenantName,
		PolicyName:        policyName,
		FromEndpointGroup: epgName,
		RuleID:            k8sutils.DenyAllRuleID + "in",
		Priority:          k8sutils.DenyAllPriority,
		Direction:         "in",
		Action:            "deny",
	}); err != nil {
		return err
	}
	policyMap := k8sNet.policyPerEpg[epgName]
	if len(policyMap) <= 0 {
		//Within Policy , Multiple Rules will be configured
		policyMap = make(map[string][]string, 0)
	}
	policyMap[policyName] = append(policyMap[policyName],
		(k8sutils.AllowAllRuleID + "in"))
	//k8sNet.policyRules[policyName] = append(k8sNet.policyRules[policyName],
	//	(k8sutils.AllowAllRuleID + "in"))
	return nil
}
func (k8sNet *k8sContext) createDefaultDenyRule(tenantName, epgName, policyName string) error {
	//Add default rule into policy
	if err := k8sNet.createRule(&client.Rule{
		TenantName:        tenantName,
		PolicyName:        policyName,
		FromEndpointGroup: epgName,
		RuleID:            k8sutils.DenyAllRuleID + "in",
		Priority:          k8sutils.DenyAllPriority,
		Direction:         "in",
		Action:            "deny",
	}); err != nil {
		return err
	}
	//k8sNet.policyRules[policyName] = append(k8sNet.policyRules[policyName],
	//	(k8sutils.DenyAllRuleID + "in"))
	policyMap := k8sNet.policyPerEpg[epgName]
	policyMap[policyName] = append(policyMap[policyName],
		(k8sutils.DenyAllRuleID + "in"))
	return nil
}

func (k8sNet *k8sContext) createDefaultPolicy(tenantName string,
	epgName string) error {
	return k8sNet.createPolicyWithDefaultRule(tenantName,
		epgName, defaultPolicyName)
}
