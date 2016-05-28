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

package extContracts

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
	if contractType == "provided" {
		modeldb.RemoveLinkSet(&epg.LinkSets.ProvExtContractsGrps, contractsGrp)
	} else if contractType == "consumed" {
		modeldb.RemoveLinkSet(&epg.LinkSets.ConsExtContractsGrps, contractsGrp)
	}
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
	if contractType == "provided" {
		modeldb.AddLinkSet(&epg.LinkSets.ProvExtContractsGrps, contractsGrp)
	} else if contractType == "consumed" {
		modeldb.AddLinkSet(&epg.LinkSets.ConsExtContractsGrps, contractsGrp)
	}

	// Links made, write the policy set object.
	err := contractsGrp.Write()
	if err != nil {
		return err
	}

	return nil
}

// Cleanup external contracts from an epg.
func CleanupExternalContracts(endpointGroup *contivModel.EndpointGroup) error {
	// Cleanup consumed external contracts
	for _, consExtContractsGrp := range endpointGroup.ConsExtContractsGrps {
		contractsGrp := contivModel.FindExtContractsGroup(consExtContractsGrp)
		err := extContractsGrpDeregister(endpointGroup, contractsGrp, "consumed")
		if err != nil {
			return err
		}
	}

	// Cleanup provided external contracts
	for _, provExtContractsGrp := range endpointGroup.ProvExtContractsGrps {
		contractsGrp := contivModel.FindExtContractsGroup(provExtContractsGrp)
		err := extContractsGrpDeregister(endpointGroup, contractsGrp, "provided")
		if err != nil {
			return err
		}
	}

	return nil
}

// Setup external contracts for an epg.
func SetupExternalContracts(endpointGroup *contivModel.EndpointGroup,
	consContractsGrps, provContractsGrps []string) error {
	// Validate presence and register consumed external contracts
	for _, consExtContractsGrp := range consContractsGrps {
		contractsGrp := contivModel.FindExtContractsGroup(consExtContractsGrp)
		err := extContractsGrpValidateAndRegister(endpointGroup, contractsGrp, "consumed")
		if err != nil {
			return err
		}
	}

	// Validate presence and register provided external contracts
	for _, provExtContractsGrp := range provContractsGrps {
		contractsGrp := contivModel.FindExtContractsGroup(provExtContractsGrp)
		err := extContractsGrpValidateAndRegister(endpointGroup, contractsGrp, "provided")
		if err != nil {
			return err
		}
	}

	return nil
}
