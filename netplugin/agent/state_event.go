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

package agent

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/netplugin/cluster"
	"github.com/contiv/netplugin/netplugin/plugin"
	"github.com/contiv/netplugin/utils/netutils"
)

func skipHost(vtepIP, homingHost, myHostLabel string) bool {
	return (vtepIP == "" && homingHost != myHostLabel ||
		vtepIP != "" && homingHost == myHostLabel)
}

// Process Infra Nw Create
// Auto allocate an endpoint for this node
func processInfraNwCreate(netPlugin *plugin.NetPlugin, nwCfg *mastercfg.CfgNetworkState, opts core.InstanceInfo) (err error) {
	pluginHost := opts.HostLabel

	// Build endpoint request
	mreq := master.CreateEndpointRequest{
		TenantName:  nwCfg.Tenant,
		NetworkName: nwCfg.NetworkName,
		EndpointID:  pluginHost,
		ConfigEP: intent.ConfigEP{
			Container: pluginHost,
			Host:      pluginHost,
		},
	}

	var mresp master.CreateEndpointResponse
	err = cluster.MasterPostReq("/plugin/createEndpoint", &mreq, &mresp)
	if err != nil {
		log.Errorf("master failed to create endpoint %s", err)
		return err
	}

	log.Infof("Got endpoint create resp from master: %+v", mresp)

	// Take lock to ensure netPlugin processes only one cmd at a time
	netPlugin.Lock()
	defer func() { netPlugin.Unlock() }()

	// Ask netplugin to create the endpoint
	netID := nwCfg.NetworkName + "." + nwCfg.Tenant
	err = netPlugin.CreateEndpoint(netID + "-" + pluginHost)
	if err != nil {
		log.Errorf("Endpoint creation failed. Error: %s", err)
		return err
	}

	// Assign IP to interface
	ipCIDR := fmt.Sprintf("%s/%d", mresp.EndpointConfig.IPAddress, nwCfg.SubnetLen)
	err = netutils.SetInterfaceIP(nwCfg.NetworkName, ipCIDR)
	if err != nil {
		log.Errorf("Could not assign ip: %s", err)
		return err
	}

	return nil
}

// Process Infra Nw Delete
// Delete the auto allocated endpoint
func processInfraNwDelete(netPlugin *plugin.NetPlugin, nwCfg *mastercfg.CfgNetworkState, opts core.InstanceInfo) (err error) {
	pluginHost := opts.HostLabel

	// Build endpoint request
	mreq := master.DeleteEndpointRequest{
		TenantName:  nwCfg.Tenant,
		NetworkName: nwCfg.NetworkName,
		EndpointID:  pluginHost,
	}

	var mresp master.DeleteEndpointResponse
	err = cluster.MasterPostReq("/plugin/deleteEndpoint", &mreq, &mresp)
	if err != nil {
		log.Errorf("master failed to delete endpoint %s", err)
		return err
	}

	log.Infof("Got endpoint create resp from master: %+v", mresp)

	// Network delete will take care of infra nw EP delete in plugin

	return
}

func processNetEvent(netPlugin *plugin.NetPlugin, nwCfg *mastercfg.CfgNetworkState,
	isDelete bool) (err error) {
	// take a lock to ensure we are programming one event at a time.
	// Also network create events need to be processed before endpoint creates
	// and reverse shall happen for deletes. That order is ensured by netmaster,
	// so we don't need to worry about that here
	netPlugin.Lock()
	defer func() { netPlugin.Unlock() }()

	operStr := ""
	if isDelete {
		err = netPlugin.DeleteNetwork(nwCfg.ID, nwCfg.NwType, nwCfg.PktTagType, nwCfg.PktTag, nwCfg.ExtPktTag,
			nwCfg.Gateway, nwCfg.Tenant)
		operStr = "delete"
	} else {
		err = netPlugin.CreateNetwork(nwCfg.ID)
		operStr = "create"
	}
	if err != nil {
		log.Errorf("Network operation %s failed. Error: %s", operStr, err)
	} else {
		log.Infof("Network operation %s succeeded", operStr)
	}

	return
}

// processEpState restores endpoint state
func processEpState(netPlugin *plugin.NetPlugin, opts core.InstanceInfo, epID string) error {
	// take a lock to ensure we are programming one event at a time.
	// Also network create events need to be processed before endpoint creates
	// and reverse shall happen for deletes. That order is ensured by netmaster,
	// so we don't need to worry about that here
	netPlugin.Lock()
	defer func() { netPlugin.Unlock() }()

	// read endpoint config
	epCfg := &mastercfg.CfgEndpointState{}
	epCfg.StateDriver = netPlugin.StateDriver
	err := epCfg.Read(epID)

	if err != nil {
		log.Errorf("Failed to read config for ep '%s' \n", epID)
		return err
	}
	// if the endpoint is not for this host, ignore it
	if skipHost(epCfg.VtepIP, epCfg.HomingHost, opts.HostLabel) {
		log.Infof("skipping mismatching host for ep %s. EP's host %s (my host: %s)",
			epID, epCfg.HomingHost, opts.HostLabel)
		return nil
	}

	// Create the endpoint
	err = netPlugin.CreateEndpoint(epID)
	if err != nil {
		log.Errorf("Endpoint operation create failed. Error: %s", err)
		return err
	}

	log.Infof("Endpoint operation create succeeded")

	return err
}

//processBgpEvent processes Bgp neighbor add/delete events
func processBgpEvent(netPlugin *plugin.NetPlugin, opts core.InstanceInfo, hostID string, isDelete bool) error {
	var err error

	if opts.HostLabel != hostID {
		log.Debugf("Ignoring Bgp Event on this host")
		return err
	}
	netPlugin.Lock()
	defer func() { netPlugin.Unlock() }()

	operStr := ""
	if isDelete {
		err = netPlugin.DeleteBgp(hostID)
		operStr = "delete"
	} else {
		err = netPlugin.AddBgp(hostID)
		operStr = "create"
	}
	if err != nil {
		log.Errorf("Bgp operation %s failed. Error: %s", operStr, err)
	} else {
		log.Infof("Bgp operation %s succeeded", operStr)
	}

	return err
}

func processGlobalFwdModeUpdEvent(netPlugin *plugin.NetPlugin, opts core.InstanceInfo, fwdMode string) {

	// parse store URL
	parts := strings.Split(opts.DbURL, "://")
	if len(parts) < 2 {
		log.Fatalf("Invalid cluster-store-url %s", opts.DbURL)
	}
	stateStore := parts[0]
	// initialize the config
	pluginConfig := plugin.Config{
		Drivers: plugin.Drivers{
			Network: "ovs",
			State:   stateStore,
		},
		Instance: opts,
	}
	pluginConfig.Instance.FwdMode = fwdMode
	netPlugin.GlobalFwdModeUpdate(pluginConfig)

	for _, master := range cluster.MasterDB {
		netPlugin.AddMaster(core.ServiceInfo{
			HostAddr: master.HostAddr,
			Port:     9001, //netmasterRPCPort
		})
	}

	serviceList, _ := cluster.ObjdbClient.GetService("netplugin")
	for _, serviceInfo := range serviceList {
		if serviceInfo.HostAddr != opts.VtepIP {
			netPlugin.AddPeerHost(core.ServiceInfo{
				HostAddr: serviceInfo.HostAddr,
				Port:     4789, //vxlanUDPPort
			})
		}
	}

}

//processServiceLBEvent processes service load balancer object events
func processServiceLBEvent(netPlugin *plugin.NetPlugin, svcLBCfg *mastercfg.CfgServiceLBState, isDelete bool) error {
	var err error
	portSpecList := []core.PortSpec{}
	portSpec := core.PortSpec{}

	netPlugin.Lock()
	defer func() { netPlugin.Unlock() }()
	serviceID := svcLBCfg.ID

	log.Infof("Recevied Process Service load balancer event {%v}", svcLBCfg)

	//create portspect list from state.
	//Ports format: servicePort:ProviderPort:Protocol
	for _, port := range svcLBCfg.Ports {

		portInfo := strings.Split(port, ":")
		if len(portInfo) != 3 {
			return errors.New("Invalid Port Format")
		}
		svcPort := portInfo[0]
		provPort := portInfo[1]
		portSpec.Protocol = portInfo[2]

		sPort, _ := strconv.ParseUint(svcPort, 10, 16)
		portSpec.SvcPort = uint16(sPort)

		pPort, _ := strconv.ParseUint(provPort, 10, 16)
		portSpec.ProvPort = uint16(pPort)

		portSpecList = append(portSpecList, portSpec)
	}

	spec := &core.ServiceSpec{
		IPAddress: svcLBCfg.IPAddress,
		Ports:     portSpecList,
	}
	operStr := ""
	if isDelete {
		err = netPlugin.DeleteServiceLB(serviceID, spec)
		operStr = "delete"
	} else {
		err = netPlugin.AddServiceLB(serviceID, spec)
		operStr = "create"
	}
	if err != nil {
		log.Errorf("Service Load Balancer %s failed.Error:%s", operStr, err)
		return err
	}
	log.Infof("Service Load Balancer %s succeeded", operStr)

	return nil
}

//processSvcProviderUpdEvent updates service provider events
func processSvcProviderUpdEvent(netPlugin *plugin.NetPlugin, svcProvider *mastercfg.SvcProvider, isDelete bool) error {
	if isDelete {
		//ignore delete event since servicelb delete will take care of this.
		return nil
	}
	netPlugin.SvcProviderUpdate(svcProvider.ServiceName, svcProvider.Providers)
	return nil
}

func processStateEvent(netPlugin *plugin.NetPlugin, opts core.InstanceInfo, rsps chan core.WatchState) {
	for {
		// block on change notifications
		rsp := <-rsps

		// For now we deal with only create and delete events
		currentState := rsp.Curr
		isDelete := false
		eventStr := "create"
		if rsp.Curr == nil {
			currentState = rsp.Prev
			isDelete = true
			eventStr = "delete"
		} else if rsp.Prev != nil {
			if bgpCfg, ok := currentState.(*mastercfg.CfgBgpState); ok {
				log.Infof("Received %q for Bgp: %q", eventStr, bgpCfg.Hostname)
				processBgpEvent(netPlugin, opts, bgpCfg.Hostname, isDelete)
				continue
			}

			if svcProvider, ok := currentState.(*mastercfg.SvcProvider); ok {
				log.Infof("Received %q for Service %s , provider:%#v", eventStr,
					svcProvider.ServiceName, svcProvider.Providers)
				processSvcProviderUpdEvent(netPlugin, svcProvider, isDelete)
			}

			if gCfg, ok := currentState.(*mastercfg.GlobConfig); ok {
				log.Infof("Received %q for global config current state - %s , prev state - %s ", eventStr, gCfg.FwdMode, rsp.Prev.(*mastercfg.GlobConfig).FwdMode)
				if gCfg.FwdMode != rsp.Prev.(*mastercfg.GlobConfig).FwdMode {
					processGlobalFwdModeUpdEvent(netPlugin, opts, gCfg.FwdMode)
				}
			}

			// Ignore modify event on network state
			if nwCfg, ok := currentState.(*mastercfg.CfgNetworkState); ok {
				log.Debugf("Received a modify event on network %q, ignoring it", nwCfg.ID)
				continue
			}

		}

		if nwCfg, ok := currentState.(*mastercfg.CfgNetworkState); ok {
			log.Infof("Received %q for network: %q", eventStr, nwCfg.ID)
			if isDelete != true {
				processNetEvent(netPlugin, nwCfg, isDelete)
				if nwCfg.NwType == "infra" {
					processInfraNwCreate(netPlugin, nwCfg, opts)
				}
			} else {
				if nwCfg.NwType == "infra" {
					processInfraNwDelete(netPlugin, nwCfg, opts)
				}
				processNetEvent(netPlugin, nwCfg, isDelete)
			}
		}
		if bgpCfg, ok := currentState.(*mastercfg.CfgBgpState); ok {
			log.Infof("Received %q for Bgp: %q", eventStr, bgpCfg.Hostname)
			processBgpEvent(netPlugin, opts, bgpCfg.Hostname, isDelete)
		}
		if serviceLbCfg, ok := currentState.(*mastercfg.CfgServiceLBState); ok {
			log.Infof("Received %q for Service %s on tenant %s", eventStr,
				serviceLbCfg.ServiceName, serviceLbCfg.Tenant)
			processServiceLBEvent(netPlugin, serviceLbCfg, isDelete)
		}
		if svcProvider, ok := currentState.(*mastercfg.SvcProvider); ok {
			log.Infof("Received %q for Service %s on tenant %s", eventStr,
				svcProvider.ServiceName, svcProvider.Providers)
			processSvcProviderUpdEvent(netPlugin, svcProvider, isDelete)
		}
	}
}

func handleNetworkEvents(netPlugin *plugin.NetPlugin, opts core.InstanceInfo, retErr chan error) {
	rsps := make(chan core.WatchState)
	go processStateEvent(netPlugin, opts, rsps)
	cfg := mastercfg.CfgNetworkState{}
	cfg.StateDriver = netPlugin.StateDriver
	retErr <- cfg.WatchAll(rsps)
	return
}

func handleBgpEvents(netPlugin *plugin.NetPlugin, opts core.InstanceInfo, recvErr chan error) {

	rsps := make(chan core.WatchState)
	go processStateEvent(netPlugin, opts, rsps)
	cfg := mastercfg.CfgBgpState{}
	cfg.StateDriver = netPlugin.StateDriver
	recvErr <- cfg.WatchAll(rsps)
	return
}

func handleServiceLBEvents(netPlugin *plugin.NetPlugin, opts core.InstanceInfo, recvErr chan error) {

	rsps := make(chan core.WatchState)
	go processStateEvent(netPlugin, opts, rsps)
	cfg := mastercfg.CfgServiceLBState{}
	cfg.StateDriver = netPlugin.StateDriver
	recvErr <- cfg.WatchAll(rsps)
	return
}

func handleSvcProviderUpdEvents(netPlugin *plugin.NetPlugin, opts core.InstanceInfo, recvErr chan error) {
	rsps := make(chan core.WatchState)
	go processStateEvent(netPlugin, opts, rsps)
	cfg := mastercfg.SvcProvider{}
	cfg.StateDriver = netPlugin.StateDriver
	recvErr <- cfg.WatchAll(rsps)
	return
}

func handleGlobalCfgEvents(netPlugin *plugin.NetPlugin, opts core.InstanceInfo, recvErr chan error) {

	rsps := make(chan core.WatchState)
	go processStateEvent(netPlugin, opts, rsps)
	cfg := mastercfg.GlobConfig{}
	cfg.StateDriver = netPlugin.StateDriver
	recvErr <- cfg.WatchAll(rsps)
	return
}
