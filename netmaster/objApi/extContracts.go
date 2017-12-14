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
	"github.com/contiv/netplugin/contivmodel"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/objdb/modeldb"
)

// Some utility functions to work with the external contracts

// Cleanup external contracts from an epg.
func cleanupExternalContracts(endpointGroup *contivModel.EndpointGroup) error {
	tenant := endpointGroup.TenantName
	for _, contractsGrp := range endpointGroup.ExtContractsGrps {
		contractsGrpKey := tenant + ":" + contractsGrp
		contractsGrpObj := contivModel.FindExtContractsGroup(contractsGrpKey)

		if contractsGrpObj != nil {
			// Break any linkeage we might have set.
			modeldb.RemoveLinkSet(&contractsGrpObj.LinkSets.EndpointGroups, endpointGroup)
			modeldb.RemoveLinkSet(&endpointGroup.LinkSets.ExtContractsGrps, contractsGrpObj)

			// Links broken, update the contracts group object.
			err := contractsGrpObj.Write()
			if err != nil {
				return err
			}
		} else {
			log.Errorf("Error cleaning up consumed ext contract %s", contractsGrp)
			continue
		}
	}

	return nil
}

// Setup external contracts for an epg.
func setupExternalContracts(endpointGroup *contivModel.EndpointGroup, extContractsGrps []string) error {
	// Validate presence and register consumed external contracts
	tenant := endpointGroup.TenantName
	for _, contractsGrp := range extContractsGrps {
		contractsGrpKey := tenant + ":" + contractsGrp
		contractsGrpObj := contivModel.FindExtContractsGroup(contractsGrpKey)

		if contractsGrpObj == nil {
			errStr := fmt.Sprintf("External contracts group %s not found", contractsGrp)
			log.Errorf(errStr)
			return core.Errorf(errStr)
		}

		// Establish the necessary links.
		modeldb.AddLinkSet(&contractsGrpObj.LinkSets.EndpointGroups, endpointGroup)
		modeldb.AddLinkSet(&endpointGroup.LinkSets.ExtContractsGrps, contractsGrpObj)

		// Links made, write the policy set object.
		err := contractsGrpObj.Write()
		if err != nil {
			return err
		}
	}

	return nil
}

// Check if the external contracts are being used by any of the EPGs.
func isExtContractsGroupUsed(contractsGroup *contivModel.ExtContractsGroup) bool {
	return len(contractsGroup.LinkSets.EndpointGroups) > 0
}

// ExtContractsGroupCreate creates a new group of external contracts
func (ac *APIController) ExtContractsGroupCreate(contractsGroup *contivModel.ExtContractsGroup) error {
	log.Infof("Received ExtContractsGroupCreate: %+v", contractsGroup)

	// Validate contracts type
	if contractsGroup.ContractsType != "provided" && contractsGroup.ContractsType != "consumed" {
		return core.Errorf("Contracts group need to be either 'provided' or 'consumed'")
	}
	// Make sure the tenant exists
	tenant := contivModel.FindTenant(contractsGroup.TenantName)
	if tenant == nil {
		return core.Errorf("Tenant %s not found", contractsGroup.TenantName)
	}

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
