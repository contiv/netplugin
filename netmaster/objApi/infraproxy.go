package objApi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/utils"
	"github.com/contiv/objmodel/contivModel"
	"io/ioutil"
	"net/http"
	"strconv"
)

const (
	proxyURL = "http://localhost:5000/"
)

// AppNwSpec Applications network spec per the composition
type AppNwSpec struct {
	TenantName string `json:"tenant,omitempty"`
	Subnet     string `json:"subnet,omitempty"`
	AppName    string `json:"app,omitempty"`

	Epgs []epgSpec `json:"epgs,omitempty"`
}

type epgSpec struct {
	Name     string   `json:"name,omitempty"`
	VlanTag  string   `json:"vlantag,omitempty"`
	ServPort []string `json:"servport,omitempty"`
	Uses     []string `json:"uses,omitempty"`
}

type epgMap struct {
	Specs map[string]epgSpec
}

func httpPost(url string, jdata interface{}) error {
	buf, err := json.Marshal(jdata)
	if err != nil {
		return err
	}

	body := bytes.NewBuffer(buf)
	r, err := http.Post(url, "application/json", body)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	switch {
	case r.StatusCode == int(404):
		return errors.New("Page not found!")
	case r.StatusCode == int(403):
		return errors.New("Access denied!")
	case r.StatusCode != int(200):
		log.Debugf("POST Status '%s' status code %d \n", r.Status, r.StatusCode)
		return errors.New(r.Status)
	}

	response, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	log.Debugf(string(response))

	return nil
}

func (ans *AppNwSpec) validate() error {

	url := proxyURL + "validateAppProf"
	if err := httpPost(url, ans); err != nil {
		log.Errorf("Validation failed. Error: %v", err)
		return err
	}

	return nil
}

func (ans *AppNwSpec) launch() error {

	ans.TenantName = "CONTIV-" + ans.TenantName
	url := proxyURL + "createAppProf"
	if err := httpPost(url, ans); err != nil {
		log.Errorf("Validation failed. Error: %v", err)
		return err
	}

	return nil
}

// Extract relevant info from epg obj and append to application nw spec
func appendEpgInfo(eMap *epgMap, epgObj *contivModel.EndpointGroup, stateDriver core.StateDriver) error {
	epg := epgSpec{}
	epg.Name = epgObj.GroupName

	//update vlantag from EpGroupState
	epgCfg := &drivers.OvsCfgEpGroupState{}
	epgCfg.StateDriver = stateDriver
	eErr := epgCfg.Read(strconv.Itoa(epgObj.EndpointGroupID))
	if eErr != nil {
		log.Errorf("Error reading epg %v %v", epgObj.GroupName, eErr)
		return eErr
	}

	epg.VlanTag = strconv.Itoa(epgCfg.PktTag)

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

			// rule.EndpointGroup uses this epg
			uEpg, ok := eMap.Specs[rule.EndpointGroup]
			if ok {
				uEpg.Uses = append(uEpg.Uses, epg.Name)
				eMap.Specs[rule.EndpointGroup] = uEpg
			} else {
				//not in the map - need to add
				userEpg := epgSpec{}
				userEpg.Uses = append(userEpg.Uses, epg.Name)
				eMap.Specs[rule.EndpointGroup] = userEpg
			}
			log.Debugf("==Used by %v", rule.EndpointGroup)
			//TODO: make this a list and add protocol
			epg.ServPort = append(epg.ServPort, strconv.Itoa(rule.Port))
			log.Debugf("Service port: %v", strconv.Itoa(rule.Port))
		}

	}

	// add any saved uses info before overwriting
	savedEpg, ok := eMap.Specs[epg.Name]
	if ok {
		epg.Uses = append(epg.Uses, savedEpg.Uses...)
	}
	eMap.Specs[epg.Name] = epg
	return nil
}

//CreateAppNw Fill in the Nw spec and launch the nw infra
func CreateAppNw(app *contivModel.App) error {
	// Get the state driver
	stateDriver, uErr := utils.GetStateDriver()
	if uErr != nil {
		return uErr
	}

	netName := ""
	eMap := &epgMap{}
	eMap.Specs = make(map[string]epgSpec)
	ans := &AppNwSpec{}

	ans.TenantName = app.TenantName
	ans.AppName = app.AppName

	// Gather all basic epg info into the epg map
	for epgName := range app.LinkSets.Services {
		epgKey := app.TenantName + ":" + epgName
		epgObj := contivModel.FindEndpointGroup(epgKey)
		if epgObj == nil {
			err := fmt.Sprintf("Epg %v does not exist", epgName)
			log.Errorf("%v", epgName)
			return errors.New(err)
		}

		if err := appendEpgInfo(eMap, epgObj, stateDriver); err != nil {
			log.Errorf("Error getting epg info %v", err)
			return err
		}
		netName = epgObj.NetworkName
	}

	// walk the map and add to ANS
	for _, epg := range eMap.Specs {
		ans.Epgs = append(ans.Epgs, epg)
		log.Debugf("Added epg %v", epg.Name)
	}

	// get the subnet info and add it to ans
	nwCfg := &drivers.OvsCfgNetworkState{}
	nwCfg.StateDriver = stateDriver
	nErr := nwCfg.Read(netName)
	if nErr != nil {
		log.Errorf("Failed to network info %v %v ", netName, nErr)
		return nErr
	}
	ans.Subnet = nwCfg.DefaultGw + "/" + strconv.Itoa(int(nwCfg.SubnetLen))
	log.Debugf("Nw %v subnet %v", netName, ans.Subnet)

	ans.launch()

	return nil
}
