/***
Copyright 2014 Cisco Systems Inc. All rights reserved.

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

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/netmaster/docknet"
	"github.com/contiv/netplugin/netmaster/gstate"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/netmaster/resources"
	"github.com/contiv/netplugin/utils"
)

// initStateDriver creates a state driver based on the cluster store URL
func initStateDriver(clusterStore string) (core.StateDriver, error) {
	// parse the state store URL
	parts := strings.Split(clusterStore, "://")
	if len(parts) < 2 {
		return nil, core.Errorf("Invalid state-store URL %q", clusterStore)
	}
	stateStore := parts[0]

	// Make sure we support the statestore type
	switch stateStore {
	case utils.EtcdNameStr:
	case utils.ConsulNameStr:
	default:
		return nil, core.Errorf("Unsupported state-store %q", stateStore)
	}

	// Setup instance info
	instInfo := core.InstanceInfo{
		DbURL: clusterStore,
	}

	return utils.NewStateDriver(stateStore, &instInfo)
}

// parseRange parses a string in "1,2-3,4-10" format and returns an array of values
func parseRange(rangeStr string) ([]uint, error) {
	var values []uint
	if rangeStr == "" {
		return []uint{}, nil
	}

	// split ranges based on "," char
	rangeList := strings.Split(rangeStr, ",")

	for _, subrange := range rangeList {
		minMax := strings.Split(strings.TrimSpace(subrange), "-")
		if len(minMax) == 2 {
			min, err := strconv.Atoi(minMax[0])
			if err != nil {
				log.Errorf("Invalid range: %v", subrange)
				return nil, err
			}
			max, err := strconv.Atoi(minMax[1])
			if err != nil {
				log.Errorf("Invalid range: %v", subrange)
				return nil, err
			}

			// some error checking
			if min > max || min < 0 || max < 0 {
				log.Errorf("Invalid range values: %v", subrange)
				return nil, fmt.Errorf("invalid range values")
			}

			for i := min; i <= max; i++ {
				values = append(values, uint(i))
			}
		} else if len(minMax) == 1 {
			val, err := strconv.Atoi(minMax[0])
			if err != nil {
				log.Errorf("Invalid range: %v", subrange)
				return nil, err
			}

			values = append(values, uint(val))
		} else {
			log.Errorf("Invalid range: %v", subrange)
			return nil, fmt.Errorf("invalid range format")
		}
	}

	return values, nil
}

// processResource handles resource commands
func processResource(stateDriver core.StateDriver, rsrcName, rsrcVal string) error {
	// Read global config
	gCfg := gstate.Cfg{}
	gCfg.StateDriver = stateDriver
	err := gCfg.Read("")
	if err != nil {
		log.Errorf("error reading tenant cfg state. Error: %s", err)
		return err
	}

	// process resource based on name
	if rsrcName == "vlan" {
		numVlans, vlansInUse := gCfg.GetVlansInUse()
		fmt.Printf("Num Vlans: %d\n Current Vlans in Use: %s\n", numVlans, vlansInUse)

		// see if we need to set the resource
		if rsrcVal != "" {
			values, err := parseRange(rsrcVal)
			if err != nil {
				log.Errorf("Error parsing range: %v", err)
				return err
			}
			log.Infof("Setting vlan values: %v", values)

			// set vlan values
			for _, val := range values {
				_, err = gCfg.AllocVLAN(val)
				if err != nil {
					log.Errorf("Error setting vlan: %d. Err: %v", val, err)
				}
			}

			log.Infof("Finished setting VLANs")
		}
	} else if rsrcName == "vxlan" {
		numVxlans, vxlansInUse := gCfg.GetVxlansInUse()
		fmt.Printf("Num Vxlans: %d\n Current Vxlans in Use: %s\n", numVxlans, vxlansInUse)

		// see if we need to set the resource
		if rsrcVal != "" {
			values, err := parseRange(rsrcVal)
			if err != nil {
				log.Errorf("Error parsing range: %v", err)
				return err
			}
			log.Infof("Setting vxlan values: %v", values)

			// set vlan values
			for _, val := range values {
				_, _, err = gCfg.AllocVXLAN(val)
				if err != nil {
					log.Errorf("Error setting vxlan: %d. Err: %v", val, err)
				}
			}

			log.Infof("Finished setting VXLANs")
		}
	} else {
		log.Errorf("Unknown resource: %v", rsrcName)
		return fmt.Errorf("unknown resource")
	}

	return nil
}

// processState handles `-state` command
func processState(stateDriver core.StateDriver, stateName, stateID, fieldName, setVal string) error {
	var typeRegistry = make(map[string]core.State)

	// build the type registry
	typeRegistry[reflect.TypeOf(mastercfg.CfgEndpointState{}).Name()] = &mastercfg.CfgEndpointState{}
	typeRegistry[reflect.TypeOf(mastercfg.CfgNetworkState{}).Name()] = &mastercfg.CfgNetworkState{}
	typeRegistry[reflect.TypeOf(mastercfg.CfgBgpState{}).Name()] = &mastercfg.CfgBgpState{}
	typeRegistry[reflect.TypeOf(mastercfg.EndpointGroupState{}).Name()] = &mastercfg.EndpointGroupState{}
	typeRegistry[reflect.TypeOf(mastercfg.GlobConfig{}).Name()] = &mastercfg.GlobConfig{}
	typeRegistry[reflect.TypeOf(mastercfg.EpgPolicy{}).Name()] = &mastercfg.EpgPolicy{}
	typeRegistry[reflect.TypeOf(mastercfg.SvcProvider{}).Name()] = &mastercfg.SvcProvider{}
	typeRegistry[reflect.TypeOf(mastercfg.CfgServiceLBState{}).Name()] = &mastercfg.CfgServiceLBState{}
	typeRegistry[reflect.TypeOf(resources.AutoVLANCfgResource{}).Name()] = &resources.AutoVLANCfgResource{}
	typeRegistry[reflect.TypeOf(resources.AutoVLANOperResource{}).Name()] = &resources.AutoVLANOperResource{}
	typeRegistry[reflect.TypeOf(resources.AutoVXLANCfgResource{}).Name()] = &resources.AutoVXLANCfgResource{}
	typeRegistry[reflect.TypeOf(resources.AutoVXLANOperResource{}).Name()] = &resources.AutoVXLANOperResource{}
	typeRegistry[reflect.TypeOf(drivers.OvsDriverOperState{}).Name()] = &drivers.OvsDriverOperState{}
	typeRegistry[reflect.TypeOf(drivers.OperEndpointState{}).Name()] = &drivers.OperEndpointState{}
	typeRegistry[reflect.TypeOf(docknet.DnetOperState{}).Name()] = &docknet.DnetOperState{}

	// find the type by name
	cfgType := typeRegistry[stateName]

	// Some reflect magic to set the state driver
	s := reflect.ValueOf(cfgType).Elem()
	if s.Kind() == reflect.Struct {
		f := s.FieldByName("StateDriver")
		if f.IsValid() && f.CanSet() {
			if f.Kind() == reflect.Interface {
				f.Set(reflect.ValueOf(stateDriver))
				log.Debugf("Set: %+v", reflect.ValueOf(cfgType).Elem().FieldByName("StateDriver"))
			} else {
				log.Errorf("Invalid kind")
				return fmt.Errorf("can not set state driver")
			}
		} else {
			log.Errorf("Could not find the field.")
			return fmt.Errorf("can not set state driver")
		}
	} else {
		log.Errorf("Invalid type: %v", s.Kind())
		return fmt.Errorf("can not set state driver")
	}

	// read the object
	log.Debugf("cfgType: %+v", cfgType)
	err := cfgType.Read(stateID)
	if err != nil {
		log.Errorf("Error reading state %s{id: %s}. Err: %v", stateName, stateID, err)
		return err
	}

	// print the object
	content, err := json.MarshalIndent(cfgType, "", "  ")
	if err != nil {
		log.Errorf("Error marshaling json: %+v", cfgType)
		return err
	}
	fmt.Printf("Current value of id: %s{ id: %s }\n%s\n", stateName, stateID, content)

	// See if we need to set a field
	if fieldName != "" && setVal != "" {
		// more reflect magic to set a field
		s := reflect.ValueOf(cfgType).Elem()
		if s.Kind() == reflect.Struct {
			f := s.FieldByName(fieldName)
			if f.IsValid() && f.CanSet() {
				switch f.Kind() {
				case reflect.String:
					f.SetString(setVal)
				case reflect.Int:
					intVal, err := strconv.ParseInt(setVal, 10, 0)
					if err != nil {
						log.Errorf("Can not convert %s to int. Err: %v", setVal, err)
						return err
					}
					f.SetInt(intVal)
				case reflect.Uint:
					intVal, err := strconv.ParseUint(setVal, 10, 0)
					if err != nil {
						log.Errorf("Can not convert %s to uint. Err: %v", setVal, err)
						return err
					}
					f.SetUint(intVal)
				case reflect.Bool:
					boolVal, err := strconv.ParseBool(setVal)
					if err != nil {
						log.Errorf("Can not convert %s to bool. Err: %v", setVal, err)
						return err
					}
					f.SetBool(boolVal)
				default:
					log.Errorf("Invalid kind")
					return fmt.Errorf("invalid kind")
				}
			} else {
				log.Errorf("Could not find the field. or its not setable")
				return fmt.Errorf("could not find the field. or its not setable")
			}
		} else {
			log.Errorf("Invalid type: %v", s.Kind())
		}

		// print the modified object
		content, err := json.MarshalIndent(cfgType, "", "  ")
		if err != nil {
			log.Errorf("Error marshaling json: %+v", cfgType)
			return err
		}
		fmt.Printf("Writing values:\n%s\n", content)

		// write it
		err = cfgType.Write()
		if err != nil {
			log.Errorf("Error writing %s{ id: %s }. Err: %v", stateName, stateID, err)
			return err
		}
	}

	return nil
}

func main() {
	var rsrcName string
	var clusterStore string
	var setVal string
	var stateName string
	var stateID string
	var fieldName string

	// parse all commandline args
	flagSet := flag.NewFlagSet("cfgtool", flag.ExitOnError)
	flagSet.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flagSet.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "%s -state <state-name> -id <state-id> -field <field-name> -set <new-value>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "	%s -state GlobConfig -id global -field FwdMode -set routing\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "%s -resource <vlan|vxlan> -set <new-range>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "	%s -resource vlan -set 1-10\n", os.Args[0])
	}

	flagSet.StringVar(&rsrcName,
		"resource",
		"",
		"Resource to modify [vlan|vxlan]")
	flagSet.StringVar(&setVal,
		"set",
		"",
		"Resource value")
	flagSet.StringVar(&clusterStore,
		"cluster-store",
		"etcd://127.0.0.1:2379",
		"Etcd or Consul cluster store url.")
	flagSet.StringVar(&stateName,
		"state",
		"",
		"State to modify")
	flagSet.StringVar(&stateID,
		"id",
		"",
		"State ID to modify")
	flagSet.StringVar(&fieldName,
		"field",
		"",
		"State Field to modify")
	if err := flagSet.Parse(os.Args[1:]); err != nil {
		log.Errorf("Error parsing commandline args: %v", err)
		return
	}

	// check if we have sufficient args
	if (rsrcName == "" && stateName == "") ||
		(stateName != "" && stateID == "") ||
		(stateName != "" && stateID != "" && setVal != "" && fieldName == "") {
		flagSet.Usage()
		os.Exit(2)
	}

	// initialize state driver
	stateDriver, err := initStateDriver(clusterStore)
	if err != nil {
		log.Fatalf("Failed to init state-store. Error: %s", err)
	}

	// Initialize resource manager
	resmgr, err := resources.NewStateResourceManager(stateDriver)
	if err != nil || resmgr == nil {
		log.Fatalf("Failed to init resource manager. Error: %s", err)
	}

	// process `-resource` command
	if rsrcName != "" {
		err = processResource(stateDriver, rsrcName, setVal)
		if err != nil {
			log.Errorf("Error processing resource %s. Err: %v", rsrcName, err)
		}

		return
	}

	// handle `-state` command
	if stateName != "" && stateID != "" {
		err = processState(stateDriver, stateName, stateID, fieldName, setVal)
		if err != nil {
			log.Errorf("Error processing state %s{ id : %s }. Err: %v", stateName, stateID, err)
		}
	}
}
