package objApi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/contivmodel"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/utils"
)

const (
	proxyURL        = "http://localhost:5000/"
	cConsume        = "CONSUME"
	cProvide        = "PROVIDE"
	cInternal       = "INTERNAL"
	cExternal       = "EXTERNAL"
	aciGwAPIVersion = "v1.2"
)

// appNwSpec Applications network spec per the composition
type appNwSpec struct {
	ACIGwAPIVersion string             `json:"aci-gw-api-version,omitempty"`
	GWConfig        *contivModel.AciGw `json:"gw-config,omitempty"`
	TenantName      string             `json:"tenant,omitempty"`
	AppName         string             `json:"app-prof,omitempty"`
	ContractDefs    []contrSpec        `json:"contract-defs,omitempty"` // defined by this app

	Epgs []epgSpec `json:"epgs,omitempty"`
}

type epgSpec struct {
	Name          string      `json:"name,omitempty"`
	NwName        string      `json:"nw-name,omitempty"`
	GwCIDR        string      `json:"gw-cidr,omitempty"`
	VlanTag       string      `json:"vlan-tag,omitempty"`
	ContractLinks []contrLink `json:"contract-links,omitempty"` // linked to this epg
	epgID         int         // not exported
}

type filterInfo struct {
	Protocol string `json:"protocol,omitempty"`
	ServPort string `json:"servport,omitempty"`
}

// contrLink specifies a link from the encapsulating epg to a contract.
// the contract could be internal (referred by ContractName) or
// external (referred by complete DN).
type contrLink struct {
	LinkKind     string `json:"link-kind,omitempty"` // provide vs consume
	ContractName string `json:"contract-name,omitempty"`
	ContractDN   string `json:"contract-dn,omitempty"`
	ContractKind string `json:"contract-kind,omitempty"` // internal vs external
}

// contrSpec defines a contract, as specified by a contiv policy rule-set,
// applied to an epg
type contrSpec struct {
	Name    string       `json:"name,omitempty"`
	Filters []filterInfo `json:"filter-info,omitempty"`
}

type epgMap struct {
	Specs     map[string]epgSpec
	Contracts map[string]*contrSpec
}

type gwResp struct {
	Result string `json:"result,omitempty"`
	Info   string `json:"info,omitempty"`
}

func httpPost(url string, jdata interface{}) (*gwResp, error) {
	buf, err := json.Marshal(jdata)
	if err != nil {
		return nil, err
	}

	body := bytes.NewBuffer(buf)
	r, err := http.Post(url, "application/json", body)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	switch {
	case r.StatusCode == int(404):
		return nil, errors.New("page not found")
	case r.StatusCode == int(403):
		return nil, errors.New("access denied")
	case r.StatusCode != int(200):
		log.Debugf("POST Status '%s' status code %d \n", r.Status, r.StatusCode)
		return nil, errors.New(r.Status)
	}

	response, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	data := gwResp{}
	err = json.Unmarshal(response, &data)
	if err != nil {
		return nil, err
	}

	return &data, nil
}

func (ans *appNwSpec) validate() error {

	url := proxyURL + "validateAppProf"
	resp, err := httpPost(url, ans)
	if err != nil {
		log.Errorf("Validation failed. Error: %v", err)
		return err
	}

	if resp.Result != "success" {
		log.Errorf("Validation failed. Error: %v", resp.Info)
		return errors.New("failed")
	}

	return nil
}

func (ans *appNwSpec) launch() error {

	url := proxyURL + "createAppProf"
	resp, err := httpPost(url, ans)
	if err != nil {
		log.Errorf("Validation failed. Error: %v", err)
		return err
	}

	if resp.Result != "success" {
		log.Errorf("GW:Validation failed. Error: %v - %v", resp.Result, resp.Info)
		errStr := fmt.Sprintf("Validation failed. Error: %v - %v",
			resp.Result, resp.Info)
		return errors.New(errStr)
	}

	return nil
}

func (ans *appNwSpec) notifyDP() {

	for _, epg := range ans.Epgs {
		mastercfg.NotifyEpgChanged(epg.epgID)
	}
}

// getGwCIDR utility that reads the gw information
func getGwCIDR(epgObj *contivModel.EndpointGroup, stateDriver core.StateDriver) (string, error) {
	// get the subnet info and add it to ans
	nwCfg := &mastercfg.CfgNetworkState{}
	nwCfg.StateDriver = stateDriver
	networkID := epgObj.NetworkName + "." + epgObj.TenantName
	nErr := nwCfg.Read(networkID)
	if nErr != nil {
		log.Errorf("Failed to network info %v %v ", networkID, nErr)
		return "", nErr
	}
	gw := nwCfg.Gateway + "/" + strconv.Itoa(int(nwCfg.SubnetLen))
	log.Debugf("GW is %s for epg %s", gw, epgObj.GroupName)
	return gw, nil
}

func getContractName(policy, consumer, provider string) string {
	// epg names are unique at tenant level in contiv
	if consumer != "" {
		return policy + "Frm" + consumer
	}
	if provider != "" {
		return policy + "To" + provider
	}

	return policy + "Expose"
}

// addPolicyContracts adds contracts defined by attached policy
// additionally, it adds contract links from this epg to those contracts
func addPolicyContracts(csMap map[string]*contrSpec, epg *epgSpec, policy *contivModel.Policy) error {

	for ruleName := range policy.LinkSets.Rules {
		rule := contivModel.FindRule(ruleName)
		if rule == nil {
			errStr := fmt.Sprintf("rule %v not found", ruleName)
			return errors.New(errStr)
		}

		if rule.FromIpAddress != "" || rule.FromNetwork != "" ||
			rule.ToIpAddress != "" || rule.ToNetwork != "" {
			log.Errorf("rule: %+v is invalid for ACI mode", rule)
			errStr := fmt.Sprintf("rule %s is invalid, only From/ToEndpointGroup may be specified in ACI mode", ruleName)
			return errors.New(errStr)
		}

		if rule.Action == "deny" {
			log.Debugf("==Ignoring deny rule %v", ruleName)
			continue
		}

		filter := filterInfo{Protocol: rule.Protocol, ServPort: strconv.Itoa(rule.Port)}
		cn := getContractName(policy.PolicyName, rule.FromEndpointGroup,
			rule.ToEndpointGroup)
		spec, found := csMap[cn]
		if !found {
			// add a link for this contract
			lKind := cProvide
			if rule.ToEndpointGroup != "" {
				lKind = cConsume
			}
			cLink := contrLink{LinkKind: lKind,
				ContractName: cn,
				ContractKind: cInternal,
			}
			epg.ContractLinks = append(epg.ContractLinks, cLink)

			spec = &contrSpec{Name: cn}
			csMap[cn] = spec
		}
		spec.Filters = append(spec.Filters, filter)
	}

	return nil
}

// Extract relevant info from epg obj and append to application nw spec
func appendEpgInfo(eMap *epgMap, epgObj *contivModel.EndpointGroup, stateDriver core.StateDriver) error {
	epg := epgSpec{}
	epg.Name = epgObj.GroupName

	log.Infof("Processing EPG: %+v", epgObj)
	// Get EPG key for the endpoint group
	epgKey := mastercfg.GetEndpointGroupKey(epgObj.GroupName, epgObj.TenantName)

	// update vlantag from EpGroupState
	epgCfg := &mastercfg.EndpointGroupState{}
	epgCfg.StateDriver = stateDriver
	eErr := epgCfg.Read(epgKey)
	if eErr != nil {
		log.Errorf("Error reading epg %v %v", epgKey, eErr)
		return eErr
	}

	// get the network name and gw cidr and update.
	epg.NwName = epgObj.NetworkName
	gwCIDR, nErr := getGwCIDR(epgObj, stateDriver)
	if nErr != nil {
		return nErr
	}
	epg.GwCIDR = gwCIDR

	epg.VlanTag = strconv.Itoa(epgCfg.PktTag)
	epg.epgID = epgCfg.EndpointGroupID

	// get all the policy details
	for _, policy := range epgObj.Policies {
		log.Debugf("==Processing policy %v", policy)
		policyKey := epgObj.TenantName + ":" + policy
		pObj := contivModel.FindPolicy(policyKey)
		if pObj == nil {
			errStr := fmt.Sprintf("Policy %v not found epg: %v", policy, epg.Name)
			return errors.New(errStr)
		}

		nErr = addPolicyContracts(eMap.Contracts, &epg, pObj)
		if nErr != nil {
			return nErr
		}
	}

	// Add linked contracts based on rules that refer this epg
	for _, ruleLink := range epgObj.LinkSets.MatchRules {
		rule := contivModel.FindRule(ruleLink.ObjKey)
		if rule == nil {
			errStr := fmt.Sprintf("Rule %s referring to epg %s not found",
				ruleLink.ObjKey, epg.Name)
			return errors.New(errStr)
		}
		cn := getContractName(rule.PolicyName, rule.FromEndpointGroup,
			rule.ToEndpointGroup)
		lKind := ""
		if rule.FromEndpointGroup != "" {
			lKind = cConsume
		} else {
			lKind = cProvide
		}

		cLink := contrLink{LinkKind: lKind,
			ContractName: cn,
			ContractKind: cInternal,
		}
		epg.ContractLinks = append(epg.ContractLinks, cLink)
	}

	// Append links for external contracts.
	tenant := epgObj.TenantName
	for _, contractsGrp := range epgObj.ExtContractsGrps {
		contractsGrpKey := tenant + ":" + contractsGrp
		contractsGrpObj := contivModel.FindExtContractsGroup(contractsGrpKey)

		if contractsGrpObj == nil {
			errStr := fmt.Sprintf("Contracts %v not found for epg: %v", contractsGrp, epg.Name)
			return errors.New(errStr)
		}

		lKind := ""
		if contractsGrpObj.ContractsType == "consumed" {
			lKind = cConsume
		} else if contractsGrpObj.ContractsType == "provided" {
			lKind = cProvide
		} else {
			// Should not be here.
			errStr := fmt.Sprintf("Invalid contracts type %v", contractsGrp)
			return errors.New(errStr)
		}

		for _, dn := range contractsGrpObj.Contracts {
			cLink := contrLink{LinkKind: lKind,
				ContractDN:   dn,
				ContractKind: cExternal,
			}
			epg.ContractLinks = append(epg.ContractLinks, cLink)
		}
	}

	eMap.Specs[epg.Name] = epg
	return nil
}

//CreateAppNw Fill in the Nw spec and launch the nw infra
func CreateAppNw(app *contivModel.AppProfile) error {
	aciPresent, aErr := master.IsAciConfigured()
	if aErr != nil {
		log.Errorf("Couldn't read global config %v", aErr)
		return aErr
	}

	if !aciPresent {
		log.Debugf("ACI not configured")
		return nil
	}

	// Get the state driver
	stateDriver, uErr := utils.GetStateDriver()
	if uErr != nil {
		return uErr
	}

	eMap := &epgMap{}
	eMap.Specs = make(map[string]epgSpec)
	eMap.Contracts = make(map[string]*contrSpec)
	ans := &appNwSpec{}

	ans.ACIGwAPIVersion = aciGwAPIVersion
	gwConfig := contivModel.FindAciGw("aciGw")
	if gwConfig == nil {
		log.Infof("aciGw object not found -- gw will use env settings")
	} else {
		ans.GWConfig = gwConfig
		log.Infof("gwConfig: %+v", gwConfig)
	}
	ans.TenantName = app.TenantName
	ans.AppName = app.AppProfileName

	// Gather all basic epg info into the epg map
	for epgKey := range app.LinkSets.EndpointGroups {
		epgObj := contivModel.FindEndpointGroup(epgKey)
		if epgObj == nil {
			err := fmt.Sprintf("Epg %v does not exist", epgKey)
			log.Errorf("%v", err)
			return errors.New(err)
		}

		if err := appendEpgInfo(eMap, epgObj, stateDriver); err != nil {
			log.Errorf("Error getting epg info %v", err)
			return err
		}
	}

	// walk the map and add to ANS
	for _, epg := range eMap.Specs {
		ans.Epgs = append(ans.Epgs, epg)
		log.Debugf("Added epg %v", epg.Name)
	}

	for _, contract := range eMap.Contracts {
		ans.ContractDefs = append(ans.ContractDefs, *contract)
		log.Debugf("Added contract %v", contract.Name)
	}

	log.Infof("Launching appNwSpec: %+v", ans)
	lErr := ans.launch()
	time.Sleep(2 * time.Second)
	ans.notifyDP()

	return lErr
}

//DeleteAppNw deletes the app profile from infra
func DeleteAppNw(app *contivModel.AppProfile) error {
	aciPresent, aErr := master.IsAciConfigured()
	if aErr != nil {
		log.Errorf("Couldn't read global config %v", aErr)
		return aErr
	}

	if !aciPresent {
		log.Debugf("ACI not configured")
		return nil
	}

	ans := &appNwSpec{}
	ans.TenantName = app.TenantName
	ans.AppName = app.AppProfileName

	url := proxyURL + "deleteAppProf"
	resp, err := httpPost(url, ans)
	if err != nil {
		log.Errorf("Delete failed. Error: %v", err)
		return err
	}

	if resp.Result != "success" {
		log.Errorf("Delete failed %v - %v", resp.Result, resp.Info)
	}

	time.Sleep(time.Second)
	return nil
}
