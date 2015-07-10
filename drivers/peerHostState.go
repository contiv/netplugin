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

package drivers

import (
	"encoding/json"
	"fmt"

	"github.com/contiv/netplugin/core"

	log "github.com/Sirupsen/logrus"
)

// This file deals with peer host discovery

const peerHostPath = "/contiv/oper/peer"

// PeerHostState : Information about the peer host
type PeerHostState struct {
	core.CommonState
	Hostname   string // Name of this host
	HostAddr   string // control plane IP address of the host
	VtepIPAddr string // VTEP IP address to use
}

// Write the state.
func (s *PeerHostState) Write() error {
	key := fmt.Sprintf("%s/%s", peerHostPath, s.ID)
	return s.StateDriver.WriteState(key, s, json.Marshal)
}

// Read the state for a given identifier.
func (s *PeerHostState) Read(id string) error {
	key := fmt.Sprintf("%s/%s", peerHostPath, id)
	return s.StateDriver.ReadState(key, s, json.Unmarshal)
}

// ReadAll reads all state objects for the peer.
func (s *PeerHostState) ReadAll() ([]core.State, error) {
	return s.StateDriver.ReadAllState(peerHostPath, s, json.Unmarshal)
}

// WatchAll fills a channel on each state event related to peers.
func (s *PeerHostState) WatchAll(rsps chan core.WatchState) error {
	return s.StateDriver.WatchAllState(peerHostPath, s, json.Unmarshal,
		rsps)
}

// Clear removes the state.
func (s *PeerHostState) Clear() error {
	key := fmt.Sprintf("%s/%s", peerHostPath, s.ID)
	return s.StateDriver.ClearState(key)
}

// Run peer discovery
func publishHostInfo(info *core.InstanceInfo) error {
	// Some error checking
	if info.VtepIP == "" {
		log.Errorf("Error: Vtep IP is empty")
	}

	// first publish ourselves
	myHostInfo := new(PeerHostState)
	myHostInfo.ID = info.HostLabel
	myHostInfo.StateDriver = info.StateDriver
	myHostInfo.Hostname = info.HostLabel
	myHostInfo.HostAddr = info.VtepIP
	myHostInfo.VtepIPAddr = info.VtepIP

	// Write it to state store.
	err := myHostInfo.Write()
	if err != nil {
		log.Errorf("Failed to publish host info. Err: %v", err)
		return err
	}

	return nil
}
