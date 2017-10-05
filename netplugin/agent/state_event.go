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
	"github.com/contiv/netplugin/mgmtfn/k8splugin"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/contiv/netplugin/netmaster/master"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/contiv/netplugin/netplugin/cluster"
	"github.com/contiv/netplugin/netplugin/plugin"
	"github.com/contiv/netplugin/utils/netutils"
)

const (
	contivVxGWName = "contivh1"
)

func checkRemoteHost(vtepIP, homingHost, myHostLabel string) bool {
	return (vtepIP == "" && homingHost != myHostLabel ||
		vtepIP != "" && homingHost == myHostLabel)
}

func getVxGWIP(netPlugin *plugin.NetPlugin, tenant, hostLabel string) (string, error) {
	epCfg := &mastercfg.CfgEndpointState{}
	epCfg.StateDriver = netPlugin.StateDriver
	epID := contivVxGWName + "." + tenant + "-" + hostLabel
	err := epCfg.Read(epID)
	if err == nil {
		return epCfg.IPAddress, nil
	}

	log.Errorf("Failed to read %s -- %v", epID, err)
	return "", err
}

// Add routes for existing vxlan networks
func addVxGWRoutes(netPlugin *plugin.NetPlugin, gwIP string) {
	readNet := &mastercfg.CfgNetworkState{}
	readNet.StateDriver = netPlugin.StateDriver
	netCfgs, err := readNet.ReadAll()
	if err != nil {
		log.Errorf("Error reading netCfgs: %v", err)
		return
	}
	for _, netCfg := range netCfgs {
		net := netCfg.(*mastercfg.CfgNetworkState)
		if net.NwType != "infra" && net.PktTagType == "vxlan" {
			route := fmt.Sprintf("%s/%d", net.SubnetIP, net.SubnetLen)
			err = netutils.AddIPRoute(route, gwIP)
			if err != nil {
				log.Errorf("Adding route %s --> %s: err: %v",
					route, gwIP, err)
			}
		}
	}

	// route add cluster-ip
	clusterNet := k8splugin.GetK8sClusterIPRange()
	log.Infof("configuring cluster-ip route [%s]", clusterNet)
	if len(clusterNet) > 0 {
		if err = netutils.AddIPRoute(clusterNet, gwIP); err != nil {
			log.Errorf("Adding route [%s] --> %s: err: %v",
				clusterNet, gwIP, err)
		}
	}
}

// Delete routes for existing vxlan networks
func delVxGWRoutes(netPlugin *plugin.NetPlugin, gwIP string) {
	readNet := &mastercfg.CfgNetworkState{}
	readNet.StateDriver = netPlugin.StateDriver
	netCfgs, err := readNet.ReadAll()
	if err != nil {
		log.Errorf("Error reading netCfgs: %v", err)
		return
	}
	for _, netCfg := range netCfgs {
		net := netCfg.(*mastercfg.CfgNetworkState)
		if net.NwType != "infra" && net.PktTagType == "vxlan" {
			route := fmt.Sprintf("%s/%d", net.SubnetIP, net.SubnetLen)
			err = netutils.DelIPRoute(route, gwIP)
			if err != nil {
				log.Errorf("Deleting route %s --> %s: err: %v",
					route, gwIP, err)
			}
		}
	}

	// route del cluster-ip
	clusterNet := k8splugin.GetK8sClusterIPRange()
	log.Infof("removing cluster-ip route [%s]", clusterNet)
	if len(clusterNet) > 0 {
		if err = netutils.DelIPRoute(clusterNet, gwIP); err != nil {
			log.Errorf("deleteing route [%s] --> %s: err: %v",
				clusterNet, gwIP, err)
		}
	}
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

	// add host access routes for vxlan networks
	if nwCfg.NetworkName == contivVxGWName {
		addVxGWRoutes(netPlugin, mresp.EndpointConfig.IPAddress)
	}

	return nil
}

// Process Infra Nw Delete
// Delete the auto allocated endpoint
func processInfraNwDelete(netPlugin *plugin.NetPlugin, nwCfg *mastercfg.CfgNetworkState, opts core.InstanceInfo) (err error) {
	pluginHost := opts.HostLabel

	if nwCfg.NetworkName == contivVxGWName {
		gwIP, err := getVxGWIP(netPlugin, nwCfg.Tenant, pluginHost)
		if err == nil {
			delVxGWRoutes(netPlugin, gwIP)
		}
	}
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
	return err
}

func processNetEvent(netPlugin *plugin.NetPlugin, nwCfg *mastercfg.CfgNetworkState,
	isDelete bool, opts core.InstanceInfo) (err error) {
	// take a lock to ensure we are programming one event at a time.
	// Also network create events need to be processed before endpoint creates
	// and reverse shall happen for deletes. That order is ensured by netmaster,
	// so we don't need to worry about that here

	gwIP := ""
	route := fmt.Sprintf("%s/%d", nwCfg.SubnetIP, nwCfg.SubnetLen)
	if nwCfg.NwType != "infra" && nwCfg.PktTagType == "vxlan" {
		gwIP, _ = getVxGWIP(netPlugin, nwCfg.Tenant, opts.HostLabel)
	}
	operStr := ""
	if isDelete {
		err = netPlugin.DeleteNetwork(nwCfg.ID, route, nwCfg.NwType, nwCfg.PktTagType, nwCfg.PktTag, nwCfg.ExtPktTag,
			nwCfg.Gateway, nwCfg.Tenant)
		operStr = "delete"
		if err == nil && gwIP != "" {
			netutils.DelIPRoute(route, gwIP)
		}
	} else {
		err = netPlugin.CreateNetwork(nwCfg.ID)
		operStr = "create"
		if err == nil && gwIP != "" {
			netutils.AddIPRoute(route, gwIP)
		}
	}
	if err != nil {
		log.Errorf("Network %s operation %s failed. Error: %s", nwCfg.ID, operStr, err)
	} else {
		log.Infof("Network %s operation %s succeeded", nwCfg.ID, operStr)
	}

	return
}

// processEpState restores endpoint state
func processEpState(netPlugin *plugin.NetPlugin, opts core.InstanceInfo, epID string) error {
	// take a lock in netplugin to ensure we are programming one event at a time.
	// Also network create events need to be processed before endpoint creates
	// and reverse shall happen for deletes. That order is ensured by netmaster,
	// so we don't need to worry about that here

	// read endpoint config
	epCfg := &mastercfg.CfgEndpointState{}
	epCfg.StateDriver = netPlugin.StateDriver
	err := epCfg.Read(epID)

	if err != nil {
		log.Errorf("Failed to read config for ep '%s' \n", epID)
		return err
	}
	eptype := "local"
	if checkRemoteHost(epCfg.VtepIP, epCfg.HomingHost, opts.HostLabel) {
		eptype = "remote"
	}

	// Create the endpoint
	if eptype == "local" {
		err = netPlugin.CreateEndpoint(epID)
	} else {
		err = netPlugin.CreateRemoteEndpoint(epID)
	}
	if err != nil {
		log.Errorf("Endpoint operation create failed. Error: %s", err)
		return err
	}

	log.Infof("Endpoint operation create succeeded")

	return err
}

// processRemoteEpState updates endpoint state
func processRemoteEpState(netPlugin *plugin.NetPlugin, opts core.InstanceInfo, epCfg *mastercfg.CfgEndpointState, isDelete bool) error {
	if !checkRemoteHost(epCfg.VtepIP, epCfg.HomingHost, opts.HostLabel) {
		// Skip local endpoint update, as they are handled directly in dockplugin
		return nil
	}

	if isDelete {
		// Delete remote endpoint
		err := netPlugin.DeleteRemoteEndpoint(epCfg.ID)
		if err != nil {
			log.Errorf("Endpoint %s delete operation failed. Error: %s", epCfg.ID, err)
			return err
		}
		log.Infof("Endpoint %s delete operation succeeded", epCfg.ID)
	} else {
		// Create remote endpoint
		err := netPlugin.CreateRemoteEndpoint(epCfg.ID)
		if err != nil {
			log.Errorf("Endpoint %s create operation failed. Error: %s", epCfg.ID, err)
			return err
		}
		log.Infof("Endpoint %s create operation succeeded", epCfg.ID)
	}

	return nil
}

//processBgpEvent processes Bgp neighbor add/delete events
func processBgpEvent(netPlugin *plugin.NetPlugin, opts core.InstanceInfo, hostID string, isDelete bool) error {
	var err error

	if opts.HostLabel != hostID {
		log.Debugf("Ignoring Bgp Event on this host")
		return err
	}

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

func processEpgEvent(netPlugin *plugin.NetPlugin, opts core.InstanceInfo, ID string, isDelete bool) error {
	log.Infof("Received processEpgEvent")
	var err error

	operStr := ""
	if isDelete {
		operStr = "delete"
	} else {
		err = netPlugin.UpdateEndpointGroup(ID)
		operStr = "update"
	}
	if err != nil {
		log.Errorf("Epg %s failed. Error: %s", operStr, err)
	} else {
		log.Infof("Epg %s succeeded", operStr)
	}

	return err
}

func processReinit(netPlugin *plugin.NetPlugin, opts core.InstanceInfo, newCfg *mastercfg.GlobConfig) {

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
	if len(pluginConfig.Instance.UplinkIntf) > 1 && newCfg.FwdMode == "routing" {
		pluginConfig.Instance.UplinkIntf = []string{pluginConfig.Instance.UplinkIntf[0]}
		log.Warnf("Routing mode supports only one uplink interface. Using %s as uplink interface", pluginConfig.Instance.UplinkIntf[0])
	}
	pluginConfig.Instance.FwdMode = newCfg.FwdMode
	pluginConfig.Instance.ArpMode = newCfg.ArpMode
	net, err := netutils.CIDRToMask(newCfg.PvtSubnet)
	if err != nil {
		log.Errorf("ERROR: %v", err)
	} else {
		pluginConfig.Instance.HostPvtNW = net
	}
	netPlugin.Reinit(pluginConfig)

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
				Port:     opts.VxlanUDPPort,
			})
		}
	}

}

func processGlobalConfigUpdEvent(netPlugin *plugin.NetPlugin, opts core.InstanceInfo, oldCfg, newCfg *mastercfg.GlobConfig) {
	// determine the type of change.

	if newCfg.FwdMode != oldCfg.FwdMode || newCfg.PvtSubnet != oldCfg.PvtSubnet {
		// this requires re-init
		processReinit(netPlugin, opts, newCfg)
	} else if newCfg.ArpMode != oldCfg.ArpMode {
		processARPModeChange(netPlugin, opts, newCfg.ArpMode)
	} else {
		log.Infof("No change to netplugin confg")
	}
}

func processARPModeChange(netPlugin *plugin.NetPlugin, opts core.InstanceInfo, arpMode string) {

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
	pluginConfig.Instance.ArpMode = arpMode
	if pluginConfig.Instance.FwdMode == "routing" && arpMode == "flood" {
		log.Infof("Global ARP mode config is not effective when forwarding mode is routing. Proxy-arp will be retained.")
	}
	netPlugin.GlobalConfigUpdate(pluginConfig)

	log.Infof("ARP mode updated")
}

//processServiceLBEvent processes service load balancer object events
func processServiceLBEvent(netPlugin *plugin.NetPlugin, svcLBCfg *mastercfg.CfgServiceLBState, isDelete bool) error {
	var err error
	portSpecList := []core.PortSpec{}
	portSpec := core.PortSpec{}

	serviceID := svcLBCfg.ID

	log.Infof("Recevied Process Service load balancer event {%v}", svcLBCfg)

	//create portspect list from state.
	//Ports format: servicePort:ProviderPort:Protocol
	for _, port := range svcLBCfg.Ports {

		portInfo := strings.Split(port, ":")
		if len(portInfo) != 3 {
			return errors.New("invalid Port Format")
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

// processPolicyRuleState updates policy rule state
func processPolicyRuleState(netPlugin *plugin.NetPlugin, opts core.InstanceInfo, ruleID string, isDelete bool) error {
	// read policy config
	ruleCfg := &mastercfg.CfgPolicyRule{}
	ruleCfg.StateDriver = netPlugin.StateDriver

	err := ruleCfg.Read(ruleID)
	if err != nil {
		log.Errorf("Failed to read config for policy rule '%s' \n", ruleID)
		return err
	}
	if isDelete {
		// Delete endpoint
		err = netPlugin.DelPolicyRule(ruleID)
		if err != nil {
			log.Errorf("PolicyRule %s delete operation failed. Error: %s", ruleID, err)
			return err
		}
		log.Infof("PolicyRule %s delete operation succeeded", ruleID)
	} else {
		// Create endpoint
		err = netPlugin.AddPolicyRule(ruleID)
		if err != nil {
			log.Errorf("PolicyRule %s create operation failed. Error: %s", ruleID, err)
			return err
		}
		log.Infof("PolicyRule %s create operation succeeded", ruleID)
	}

	return err
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

			if epgCfg, ok := currentState.(*mastercfg.EndpointGroupState); ok {
				log.Infof("Received %q for Endpointgroup: %q", eventStr, epgCfg.EndpointGroupID)
				processEpgEvent(netPlugin, opts, epgCfg.ID, isDelete)
				continue
			}

			if svcProvider, ok := currentState.(*mastercfg.SvcProvider); ok {
				log.Infof("Received %q for Service %s , provider:%#v", eventStr,
					svcProvider.ServiceName, svcProvider.Providers)
				processSvcProviderUpdEvent(netPlugin, svcProvider, isDelete)
			}

			if gCfg, ok := currentState.(*mastercfg.GlobConfig); ok {
				prevCfg := rsp.Prev.(*mastercfg.GlobConfig)
				log.Infof("Received %q for global config current state - %+v, prev state - %+v ", eventStr,
					gCfg, prevCfg)
				processGlobalConfigUpdEvent(netPlugin, opts, prevCfg, gCfg)
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
				processNetEvent(netPlugin, nwCfg, isDelete, opts)
				if nwCfg.NwType == "infra" {
					processInfraNwCreate(netPlugin, nwCfg, opts)
				}
			} else {
				if nwCfg.NwType == "infra" {
					processInfraNwDelete(netPlugin, nwCfg, opts)
				}
				processNetEvent(netPlugin, nwCfg, isDelete, opts)
			}
		}
		if epCfg, ok := currentState.(*mastercfg.CfgEndpointState); ok {
			log.Infof("Received %q for Endpoint: %q", eventStr, epCfg.ID)
			processRemoteEpState(netPlugin, opts, epCfg, isDelete)
		}
		if bgpCfg, ok := currentState.(*mastercfg.CfgBgpState); ok {
			log.Infof("Received %q for Bgp: %q", eventStr, bgpCfg.Hostname)
			processBgpEvent(netPlugin, opts, bgpCfg.Hostname, isDelete)
		}
		if epgCfg, ok := currentState.(*mastercfg.EndpointGroupState); ok {
			log.Infof("Received %q for Endpointgroup: %q", eventStr, epgCfg.EndpointGroupID)
			processEpgEvent(netPlugin, opts, epgCfg.ID, isDelete)
			continue
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
		if ruleCfg, ok := currentState.(*mastercfg.CfgPolicyRule); ok {
			log.Infof("Received %q for PolicyRule: %q", eventStr, ruleCfg.RuleId)
			processPolicyRuleState(netPlugin, opts, ruleCfg.RuleId, isDelete)
		}
	}
}

func handleNetworkEvents(netPlugin *plugin.NetPlugin, opts core.InstanceInfo, retErr chan error) {
	rsps := make(chan core.WatchState)
	go processStateEvent(netPlugin, opts, rsps)
	cfg := mastercfg.CfgNetworkState{}
	cfg.StateDriver = netPlugin.StateDriver
	retErr <- cfg.WatchAll(rsps)
	log.Errorf("Error from handleNetworkEvents")
}

func handleBgpEvents(netPlugin *plugin.NetPlugin, opts core.InstanceInfo, recvErr chan error) {

	rsps := make(chan core.WatchState)
	go processStateEvent(netPlugin, opts, rsps)
	cfg := mastercfg.CfgBgpState{}
	cfg.StateDriver = netPlugin.StateDriver
	recvErr <- cfg.WatchAll(rsps)
	log.Errorf("Error from handleBgpEvents")
}

func handleEndpointEvents(netPlugin *plugin.NetPlugin, opts core.InstanceInfo, retErr chan error) {
	rsps := make(chan core.WatchState)
	go processStateEvent(netPlugin, opts, rsps)
	cfg := mastercfg.CfgEndpointState{}
	cfg.StateDriver = netPlugin.StateDriver
	retErr <- cfg.WatchAll(rsps)
	log.Errorf("Error from handleEndpointEvents")
}

func handleEpgEvents(netPlugin *plugin.NetPlugin, opts core.InstanceInfo, recvErr chan error) {

	rsps := make(chan core.WatchState)
	go processStateEvent(netPlugin, opts, rsps)
	cfg := mastercfg.EndpointGroupState{}
	cfg.StateDriver = netPlugin.StateDriver
	recvErr <- cfg.WatchAll(rsps)
	log.Errorf("Error from handleEpgEvents")
}

func handleServiceLBEvents(netPlugin *plugin.NetPlugin, opts core.InstanceInfo, recvErr chan error) {

	rsps := make(chan core.WatchState)
	go processStateEvent(netPlugin, opts, rsps)
	cfg := mastercfg.CfgServiceLBState{}
	cfg.StateDriver = netPlugin.StateDriver
	recvErr <- cfg.WatchAll(rsps)
	log.Errorf("Error from handleLBEvents")
}

func handleSvcProviderUpdEvents(netPlugin *plugin.NetPlugin, opts core.InstanceInfo, recvErr chan error) {
	rsps := make(chan core.WatchState)
	go processStateEvent(netPlugin, opts, rsps)
	cfg := mastercfg.SvcProvider{}
	cfg.StateDriver = netPlugin.StateDriver
	recvErr <- cfg.WatchAll(rsps)
	log.Errorf("Error from handleSvcProviderUpdEvents")
}

func handleGlobalCfgEvents(netPlugin *plugin.NetPlugin, opts core.InstanceInfo, recvErr chan error) {

	rsps := make(chan core.WatchState)
	go processStateEvent(netPlugin, opts, rsps)
	cfg := mastercfg.GlobConfig{}
	cfg.StateDriver = netPlugin.StateDriver
	recvErr <- cfg.WatchAll(rsps)
	log.Errorf("Error from handleGlobalCfgEvents")
}

func handlePolicyRuleEvents(netPlugin *plugin.NetPlugin, opts core.InstanceInfo, retErr chan error) {
	rsps := make(chan core.WatchState)
	go processStateEvent(netPlugin, opts, rsps)
	cfg := mastercfg.CfgPolicyRule{}
	cfg.StateDriver = netPlugin.StateDriver
	retErr <- cfg.WatchAll(rsps)
	log.Errorf("Error from handlePolicyRuleEvents")
}
