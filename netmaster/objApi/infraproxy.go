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
	"github.com/contiv/contivmodel"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/utils"
)

const (
	proxyURL = "http://localhost:5000/"
)

// appNwSpec Applications network spec per the composition
type appNwSpec struct {
	TenantName string `json:"tenant,omitempty"`
	AppName    string `json:"app,omitempty"`

	Epgs []epgSpec `json:"epgs,omitempty"`
}

type epgSpec struct {
	Name          string       `json:"name,omitempty"`
	NwName        string       `json:"nwname,omitempty"`
	GwCIDR        string       `json:"gwcidr,omitempty"`
	VlanTag       string       `json:"vlantag,omitempty"`
	Filters       []filterInfo `json:"filterinfo,omitempty"`
	Uses          []string     `json:"uses,omitempty"`
	ProvContracts []string     `json:"provcontracts,omitempty"`
	ConsContracts []string     `json:"conscontracts,omitempty"`
	epgID         int          // not exported
}

type filterInfo struct {
	Protocol string `json:"protocol,omitempty"`
	ServPort string `json:"servport,omitempty"`
}

type epgMap struct {
	Specs map[string]epgSpec
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
		return nil, errors.New("Page not found!")
	case r.StatusCode == int(403):
		return nil, errors.New("Access denied!")
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
		return errors.New("Failed")
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
		log.Errorf("Validation failed. Error: %v - %v", resp.Result, resp.Info)
		return errors.New("Failed")
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

	// get all the service link details
	for _, policy := range epgObj.Policies {
		log.Debugf("==Processing policy %v", policy)
		policyKey := epgObj.TenantName + ":" + policy
		pObj := contivModel.FindPolicy(policyKey)
		if pObj == nil {
			errStr := fmt.Sprintf("Policy %v not found epg: %v", policy, epg.Name)
			return errors.New(errStr)
		}

		for ruleName := range pObj.LinkSets.Rules {
			log.Debugf("==Processing rule %v", ruleName)
			rule := contivModel.FindRule(ruleName)
			if rule == nil {
				errStr := fmt.Sprintf("rule %v not found", ruleName)
				return errors.New(errStr)
			}

			if rule.Action == "deny" {
				log.Debugf("==Ignoring deny rule %v", ruleName)
				continue
			}
			singleFilter := filterInfo{Protocol: rule.Protocol, ServPort: strconv.Itoa(rule.Port)}
			epg.Filters = append(epg.Filters, singleFilter)
			log.Debugf("Filter information: %v", singleFilter)

			if rule.FromEndpointGroup == "" {
				log.Debugf("User unspecified %v == exposed contract", ruleName)
				continue
			}

			// rule.FromEndpointGroup uses this epg
			uEpg, ok := eMap.Specs[rule.FromEndpointGroup]
			if ok {
				uEpg.Uses = append(uEpg.Uses, epg.Name)
				eMap.Specs[rule.FromEndpointGroup] = uEpg
			} else {
				//not in the map - need to add
				userEpg := epgSpec{}
				userEpg.Uses = append(userEpg.Uses, epg.Name)
				eMap.Specs[rule.FromEndpointGroup] = userEpg
			}
			log.Debugf("==Used by %v", rule.FromEndpointGroup)
		}

	}

	// Append external contracts.
	tenant := epgObj.TenantName
	for _, contractsGrp := range epgObj.ExtContractsGrps {
		contractsGrpKey := tenant + ":" + contractsGrp
		contractsGrpObj := contivModel.FindExtContractsGroup(contractsGrpKey)

		if contractsGrpObj == nil {
			errStr := fmt.Sprintf("Contracts %v not found for epg: %v", contractsGrp, epg.Name)
			return errors.New(errStr)
		}
		if contractsGrpObj.ContractsType == "consumed" {
			epg.ConsContracts = append(epg.ConsContracts, contractsGrpObj.Contracts...)
		} else if contractsGrpObj.ContractsType == "provided" {
			epg.ProvContracts = append(epg.ProvContracts, contractsGrpObj.Contracts...)
		} else {
			// Should not be here.
			errStr := fmt.Sprintf("Invalid contracts type %v", contractsGrp)
			return errors.New(errStr)
		}
	}

	log.Debugf("Copied over %d externally defined consumed contracts", len(epg.ConsContracts))
	log.Debugf("Copied over %d externally defined provided contracts", len(epg.ProvContracts))

	// add any saved uses info before overwriting
	savedEpg, ok := eMap.Specs[epg.Name]
	if ok {
		epg.Uses = append(epg.Uses, savedEpg.Uses...)
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
	ans := &appNwSpec{}

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
