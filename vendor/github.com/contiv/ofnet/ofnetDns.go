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
package ofnet

import (
	"errors"
)

const DnsMaxRespMsgSize = 1024

type NameServer interface {
	NsLookup([]byte, *string) ([]byte, error)
}

func processDNSPkt(agent *OfnetAgent, inPort uint32, udpData []byte) ([]byte, error) {

	dnsErr := errors.New("failed")

	if agent.nameServer == nil {
		return nil, dnsErr
	}

	vid := agent.getPortVlanMap(inPort)
	if vid == nil {
		agent.incrErrStats("dnsPktInvalidVlan")
		return nil, dnsErr
	}

	vrf := agent.getvlanVrf(*vid)
	if vrf == nil {
		agent.incrErrStats("dnsPktInvalidVrf")
		return nil, dnsErr
	}

	agent.incrStats("dnsPktRcvd")
	resp, err := agent.nameServer.NsLookup(udpData, vrf)

	if err == nil && len(resp) > DnsMaxRespMsgSize {
		agent.incrErrStats("dnsPktLargeDrop")
		return nil, errors.New("failed")
	}
	return resp, err
}
