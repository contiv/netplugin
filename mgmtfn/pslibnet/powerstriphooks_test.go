package main

import (
	"testing"

	"github.com/mapuri/libnetwork/driverapi"
)

type TestDriver struct {
}

func (d *LibNetDriver) Config(config interface{}) error {
}

func (d *LibNetDriver) CreateNetwork(nid driverapi.UUID, config interface{}) error {
	return core.Errorf("Not implemented")
}

func (d *LibNetDriver) DeleteNetwork(nid driverapi.UUID) error {
	return core.Errorf("Not implemented")
}

func (d *LibNetDriver) CreateEndpoint(nid, eid driverapi.UUID, key string,
	config interface{}) (*driverapi.SandboxInfo, error) {
}

func (d *LibNetDriver) DeleteEndpoint(nid, eid driverapi.UUID) error {
}

func TestPwrStrpAdptrHandlePreCreateNoNetId(t *testing.T) {
	adptr := &PwrStrpAdptr{}
	adptr.Init()
}

func TestPwrStrpAdptrHandlePreCreateNoTenantId(t *testing.T) {
}

func TestPwrStrpAdptrHandlePreCreateSuccess(t *testing.T) {
}

func TestPwrStrpAdptrHandlePostCreateFailureCode(t *testing.T) {
}

func TestPwrStrpAdptrHandlePostCreateSuccess(t *testing.T) {
}

func TestPwrStrpAdptrHandlePreStartSuccess(t *testing.T) {
}

func TestPwrStrpAdptrHandlePostStartFailureCode(t *testing.T) {
}

func TestPwrStrpAdptrHandlePostStartCreateEndpointFailure(t *testing.T) {
}

func TestPwrStrpAdptrHandlePostStartSuccess(t *testing.T) {
}

func TestPwrStrpAdptrHandlePreStopSuccess(t *testing.T) {
}

func TestPwrStrpAdptrHandlePreDeleteSuccess(t *testing.T) {
}
