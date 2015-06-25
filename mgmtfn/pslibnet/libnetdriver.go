package main

import (
	"bytes"
	"encoding/json"
	"os/exec"

	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/intent"
	"github.com/mapuri/libnetwork/driverapi"
)

// DriverConfig contains the driver specific configuration values passed
// to the driver by libnetwork
// XXX: Replace with the actual structure once it is defined by libnetwork
type DriverConfig struct {
	tenantID string
	netID    string
	contID   string
}

// LibNetDriver implements the libnetwork.driverapi
// XXX: extract this into separate package so that it can be used once
// libnetwork is integrated with docker and powerstrip is not needed
type LibNetDriver struct {
	endpoints map[driverapi.UUID]DriverConfig
}

// Config configures the driver
func (d *LibNetDriver) Config(config interface{}) error {
	d.endpoints = make(map[driverapi.UUID]DriverConfig)
	return nil
}

// CreateNetwork is not implemented
func (d *LibNetDriver) CreateNetwork(nid driverapi.UUID, config interface{}) error {
	return core.Errorf("Not implemented")
}

// DeleteNetwork is not implemented
func (d *LibNetDriver) DeleteNetwork(nid driverapi.UUID) error {
	return core.Errorf("Not implemented")
}

func invokeNetdcli(dc DriverConfig, isAdd bool) error {
	EpCfg := &intent.Config{
		Tenants: []intent.ConfigTenant{
			intent.ConfigTenant{
				Name: dc.tenantID,
				Networks: []intent.ConfigNetwork{
					intent.ConfigNetwork{
						Name: dc.netID,
						Endpoints: []intent.ConfigEP{
							intent.ConfigEP{
								AttachUUID: dc.contID,
								Container:  dc.contID,
								// XXX: host-label needs to come from config
								Host: gcliOpts.hostLabel,
							},
						},
					},
				},
			}}}
	cfgArg := "-add-cfg"
	if !isAdd {
		cfgArg = "-del-cfg"
	}
	cmd := exec.Command("netdcli", "-etcd-url", gcliOpts.etcdURL, cfgArg, "-")
	config, err := json.Marshal(EpCfg)
	if err != nil {
		return err
	}
	cmd.Stdin = bytes.NewReader(config)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return core.Errorf("command failed. Error: %s, Output: %s", err, out)
	}
	return nil
}

// CreateEndpoint creates endpoint state
func (d *LibNetDriver) CreateEndpoint(nid, eid driverapi.UUID, key string,
	config interface{}) (*driverapi.SandboxInfo, error) {
	dc, ok := config.(DriverConfig)
	if !ok {
		return nil, core.Errorf("Invalid config passed")
	}

	err := invokeNetdcli(dc, true)
	if err != nil {
		return nil, err
	}

	//update driver state
	d.endpoints[eid] = dc
	// XXX: todo start populating sandbox info. Right now Netplugin takes care
	// of managing interfaces in network namespace. And it's not yet clear how
	// much control libnetwork gives to the driver by passing the sboxkey.
	return nil, nil
}

// DeleteEndpoint deletes endpoint state
func (d *LibNetDriver) DeleteEndpoint(nid, eid driverapi.UUID) error {
	dc, ok := d.endpoints[eid]
	if !ok {
		return core.Errorf("endpoint info not found for epid: %q, netid: %q",
			eid, nid)
	}
	err := invokeNetdcli(dc, false)
	if err != nil {
		return err
	}

	//update driver state
	delete(d.endpoints, eid)
	return nil
}
