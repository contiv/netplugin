/***
Copyright 2016 Cisco Systems Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package objApi

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/contiv/contivmodel"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/objdb/modeldb"
	"strings"
)

// Some utility functions to work with the external contracts
// Severe the connections between EPGs and external contracts.
func extContractsGrpDeregister(epg *contivModel.EndpointGroup,
	contractsGrp *contivModel.ExtContractsGroup,
	contractType string) error {
	if contractsGrp == nil {
		errStr := fmt.Sprintf("%s External contracts group not found", contractType)
		log.Errorf(errStr)
		return core.Errorf(errStr)
	}

	modeldb.RemoveLinkSet(&contractsGrp.LinkSets.EndpointGroups, epg)
	modeldb.RemoveLinkSet(&epg.LinkSets.ExtContractsGrps, contractsGrp)

	// Links broken, update the contracts group object.
	err := contractsGrp.Write()
	if err != nil {
		return err
	}

	return nil
}

// Check for the presence of external contracts, and also make sure that
// they are of the right type. If yes, establish necessary relationships
// between the epg and the external contracts.
func extContractsGrpValidateAndRegister(epg *contivModel.EndpointGroup,
	contractsGrp *contivModel.ExtContractsGroup,
	contractType string) error {
	if contractsGrp == nil {
		errStr := fmt.Sprintf("%s External contracts group not found", contractType)
		log.Errorf(errStr)
		return core.Errorf(errStr)
	}

	if strings.ToLower(contractsGrp.ContractsType) != contractType {
		errStr := fmt.Sprintf("Incorrect type for contract group: %v", contractsGrp)
		log.Errorf(errStr)
		return core.Errorf(errStr)
	}

	// Establish the necessary links.
	modeldb.AddLinkSet(&contractsGrp.LinkSets.EndpointGroups, epg)
	modeldb.AddLinkSet(&epg.LinkSets.ExtContractsGrps, contractsGrp)

	// Links made, write the policy set object.
	err := contractsGrp.Write()
	if err != nil {
		return err
	}

	return nil
}

// Cleanup external contracts from an epg.
func cleanupExternalContracts(endpointGroup *contivModel.EndpointGroup) error {
	// Cleanup consumed external contracts
	tenant := endpointGroup.TenantName
	for _, consExtContractsGrp := range endpointGroup.ConsExtContractsGrps {
		contractsGrpKey := tenant + ":" + consExtContractsGrp
		contractsGrp := contivModel.FindExtContractsGroup(contractsGrpKey)
		err := extContractsGrpDeregister(endpointGroup, contractsGrp, "consumed")
		if err != nil {
			if contractsGrp != nil {
				log.Errorf("Error cleaning up consumed ext contract %s", contractsGrp.ContractsGroupName)
			}
			continue
		}
	}

	// Cleanup provided external contracts
	for _, provExtContractsGrp := range endpointGroup.ProvExtContractsGrps {
		contractsGrpKey := tenant + ":" + provExtContractsGrp
		contractsGrp := contivModel.FindExtContractsGroup(contractsGrpKey)
		err := extContractsGrpDeregister(endpointGroup, contractsGrp, "provided")
		if err != nil {
			if contractsGrp != nil {
				log.Errorf("Error cleaning up provided ext contract %s", contractsGrp.ContractsGroupName)
			}
			continue
		}
	}

	return nil
}

// Setup external contracts for an epg.
func setupExternalContracts(endpointGroup *contivModel.EndpointGroup,
	consContractsGrps, provContractsGrps []string) error {
	// Validate presence and register consumed external contracts
	tenant := endpointGroup.TenantName
	for _, consExtContractsGrp := range consContractsGrps {
		contractsGrpKey := tenant + ":" + consExtContractsGrp
		contractsGrp := contivModel.FindExtContractsGroup(contractsGrpKey)
		err := extContractsGrpValidateAndRegister(endpointGroup, contractsGrp, "consumed")
		if err != nil {
			return err
		}
	}

	// Validate presence and register provided external contracts
	for _, provExtContractsGrp := range provContractsGrps {
		contractsGrpKey := tenant + ":" + provExtContractsGrp
		contractsGrp := contivModel.FindExtContractsGroup(contractsGrpKey)
		err := extContractsGrpValidateAndRegister(endpointGroup, contractsGrp, "provided")
		if err != nil {
			return err
		}
	}

	return nil
}

// Check if the external contracts are being used by any of the EPGs.
func isExtContractsGroupUsed(contractsGroup *contivModel.ExtContractsGroup) bool {
	if len(contractsGroup.LinkSets.EndpointGroups) > 0 {
		return true
	}

	return false
}

// ExtContractsGroupCreate creates a new group of external contracts
func (ac *APIController) ExtContractsGroupCreate(contractsGroup *contivModel.ExtContractsGroup) error {
	log.Infof("Received ExtContractsGroupCreate: %+v", contractsGroup)

	// NOTE: Nothing more needs to be done here. This object
	// need not be created in the masterCfg.

	return nil
}

// ExtContractsGroupUpdate updates an existing group of contract sets
func (ac *APIController) ExtContractsGroupUpdate(contractsGroup, params *contivModel.ExtContractsGroup) error {
	log.Infof("Received ExtContractsGroupUpdate: %+v, params: %+v", contractsGroup, params)
	log.Errorf("Error: external contracts update not supported: %s", contractsGroup.ContractsGroupName)

	return core.Errorf("external contracts update not supported")
}

// ExtContractsGroupDelete deletes an existing external contracts group
func (ac *APIController) ExtContractsGroupDelete(contractsGroup *contivModel.ExtContractsGroup) error {
	log.Infof("Received ExtContractsGroupDelete: %+v", contractsGroup)

	// At this moment, we let the external contracts to be deleted only
	// if there are no consumers of this external contracts group
	if isExtContractsGroupUsed(contractsGroup) == true {
		log.Errorf("Error: External contracts groups is being used: %s", contractsGroup.ContractsGroupName)
		return core.Errorf("External contracts group is in-use")
	}

	return nil
}
