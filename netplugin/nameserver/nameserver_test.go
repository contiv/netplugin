/***
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
package nameserver

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/miekg/dns"
	"os"
	"testing"
	"time"
)

var utlog = logrus.WithField("module", "nameserver-ut")

func assertOnTrue(t *testing.T, val bool, msg string) {
	if val == true {
		utlog.Errorf("%s", msg)
		t.FailNow()
	}
	// else continue
}

func assertOnErr(t *testing.T, err error, msg string) {
	if err != nil {
		utlog.Errorf("%s:%s", err, msg)
		t.FailNow()
	}
	// else continue
}

type dummyState struct {
	core.Driver
}

func (ds *dummyState) Init(instInfo *core.InstanceInfo) error {
	return nil
}

func (ds *dummyState) Deinit() {
}

func (ds *dummyState) Write(key string, value []byte) error {
	return nil
}
func (ds *dummyState) Read(key string) ([]byte, error) {
	return []byte{}, nil
}

func (ds *dummyState) ReadAll(baseKey string) ([][]byte, error) {
	return [][]byte{}, nil
}

func (ds *dummyState) WatchAll(baseKey string, rsps chan [2][]byte) error {
	return nil
}

func (ds *dummyState) WriteState(key string, value core.State,
	marshal func(interface{}) ([]byte, error)) error {
	return nil
}
func (ds *dummyState) ReadState(key string, value core.State,
	unmarshal func([]byte, interface{}) error) error {
	return nil
}

func (ds *dummyState) ReadAllState(baseKey string, stateType core.State,
	unmarshal func([]byte, interface{}) error) ([]core.State, error) {
	return []core.State{}, nil
}

func (ds *dummyState) WatchAllState(baseKey string, stateType core.State,
	unmarshal func([]byte, interface{}) error, rsps chan core.WatchState) error {
	return nil
}

func (ds *dummyState) ClearState(key string) error {
	return nil
}

func TestEpChanError(t *testing.T) {
	ns := new(NetpluginNameServer)
	ds := new(dummyState)
	err := ns.Init(ds)
	assertOnErr(t, err, "namespace init")

	if s, ok := ns.stats.tenantStats[""]; ok {
		s["epWatchRestartError"] = 0
	}
	ns.epErrChan <- fmt.Errorf("failed")
	time.Sleep(100 * time.Millisecond)
	s := ns.stats.tenantStats[""]["epWatchRestartError"]
	assertOnTrue(t, s != 1, "failed epWatchRestart stat")
}

func TestSvcChanError(t *testing.T) {
	ns := new(NetpluginNameServer)
	ds := new(dummyState)
	err := ns.Init(ds)
	assertOnErr(t, err, "namespace init")

	if s, ok := ns.stats.tenantStats[""]; ok {
		s["svcWatchRestartError"] = 0
	}
	ns.svcErrChan <- fmt.Errorf("failed")
	time.Sleep(100 * time.Millisecond)

	s := ns.stats.tenantStats[""]["svcWatchRestartError"]
	assertOnTrue(t, s != 1, "failed svcWatchRestart stat")
}

func endPointEvent(opcode string, ns *NetpluginNameServer, vrf string, nw string, v6 bool, epg string, count int) {
	for i := 0; i < count; i++ {

		ep := mastercfg.CfgEndpointState{
			EndpointID:       fmt.Sprintf("BAADCAFE3%d", i),
			EPCommonName:     fmt.Sprintf("testendpoint-%d", i+1),
			NetID:            fmt.Sprintf("%s.%s", nw, vrf),
			IPAddress:        fmt.Sprintf("10.36.28.%d", i+1%200),
			EndpointGroupKey: epg,
		}

		if v6 {
			ep.IPv6Address = fmt.Sprintf("2001:4860:0:2001::%d", i+1%99)
		}

		switch opcode {
		case "add":
			s := core.WatchState{Curr: &ep}
			ns.epChan <- s

		case "del":
			s := core.WatchState{Prev: &ep}
			ns.epChan <- s

		case "mod":
			s := core.WatchState{Prev: &ep, Curr: &ep}
			ns.epChan <- s

		}

		for l := 0; l < 60 && len(ns.epChan) != 0; l++ {
			time.Sleep(100 * time.Millisecond)
		}
	}
	time.Sleep(100 * time.Millisecond)

}

func serviceEvent(opcode string, ns *NetpluginNameServer, vrf string, nw string, count int) {
	for i := 0; i < count; i++ {
		svc := mastercfg.CfgServiceLBState{
			ServiceName: fmt.Sprintf("testservice-%d", i+1),
			IPAddress:   fmt.Sprintf("10.36.28.%d", i+1),
			Tenant:      vrf,
			Network:     nw,
		}

		switch opcode {
		case "add":
			s := core.WatchState{Curr: &svc}
			ns.svcChan <- s

		case "del":
			s := core.WatchState{Prev: &svc}
			ns.svcChan <- s

		case "mod":
			s := core.WatchState{Prev: &svc, Curr: &svc}
			ns.svcChan <- s

		}

		for l := 0; l < 60 && len(ns.svcChan) != 0; l++ {
			time.Sleep(100 * time.Millisecond)
		}
	}
	time.Sleep(100 * time.Millisecond)

}

func verifyServiceName(ens *NetpluginNameServer, vrf string, v6 bool, count int) bool {
	tenMap := ens.getBucket(vrf)
	tenMap.RLock()
	defer tenMap.RUnlock()

	for i := 0; i < count; i++ {
		ServiceName := fmt.Sprintf("testservice-%d", i+1)
		IPAddr := fmt.Sprintf("10.36.28.%d", i+1)

		dh, ok := tenMap.tenantTables[vrf]
		if !ok || dh == nil {
			return false
		}

		nameRecord, ok := dh.svcTbl[ServiceName]
		if !ok {
			return false
		}

		if nameRecord.v4Record.String() != IPAddr {
			return false
		}

		if v6 {
			v6addr := fmt.Sprintf("2001:4860:0:2001::%d", i+1%99)
			if nameRecord.v6Record.String() != v6addr {
				return false
			}
		}

	}
	return true
}

func verifyEndPointGroup(ens *NetpluginNameServer, vrf string, epgName string, count int) bool {
	tenMap := ens.getBucket(vrf)
	tenMap.RLock()
	defer tenMap.RUnlock()

	dh, ok := tenMap.tenantTables[vrf]
	if !ok || dh == nil {
		return false
	}

	epgList, ok := dh.epgTbl[epgName]
	if !ok {
		return false
	}

	for i := 0; i < count; i++ {
		epid := fmt.Sprintf("BAADCAFE3%d", i)
		if _, ok := epgList[epid]; !ok {
			return false
		}
	}
	return true

}

func verifyCommonName(ens *NetpluginNameServer, vrf string, count int) bool {
	tenMap := ens.getBucket(vrf)
	tenMap.RLock()
	defer tenMap.RUnlock()

	for i := 0; i < count; i++ {
		cname := fmt.Sprintf("testendpoint-%d", i+1)
		dh, ok := tenMap.tenantTables[vrf]
		if !ok || dh == nil {
			return false
		}

		epgList, ok := dh.nameTbl[cname]
		if !ok {
			return false
		}
		epid := fmt.Sprintf("BAADCAFE3%d", i)
		if _, ok := epgList[epid]; !ok {
			return false
		}
	}

	return true
}

func verifyEndpointID(ens *NetpluginNameServer, vrf string, v6 bool, count int) bool {
	tenMap := ens.getBucket(vrf)
	tenMap.RLock()
	defer tenMap.RUnlock()

	for i := 0; i < count; i++ {
		epid := fmt.Sprintf("BAADCAFE3%d", i)
		IPAddr := fmt.Sprintf("10.36.28.%d", i+1)

		dh, ok := tenMap.tenantTables[vrf]
		if !ok || dh == nil {
			return false
		}

		nameRecord, ok := dh.endpointTbl[epid]
		if !ok {
			return false
		}

		if nameRecord.v4Record.String() != IPAddr {
			return false
		}
		if v6 {
			v6addr := fmt.Sprintf("2001:4860:0:2001::%d", i+1%99)
			if nameRecord.v6Record.String() != v6addr {
				return false
			}

		}
	}

	return true
}

func enpointOperation(t *testing.T, count int) {
	ns := new(NetpluginNameServer)
	ds := new(dummyState)
	err := ns.Init(ds)
	assertOnErr(t, err, "namespace init")
	vrf := "tenant1"
	nw := "net1"
	epg := "epg1"
	endPointEvent("add", ns, vrf, nw, false, epg, count)
	time.Sleep(1 * time.Second)
	s := verifyEndpointID(ns, vrf, false, count)
	assertOnTrue(t, s != true, fmt.Sprintf("endpoint doesnt exist, %+v", ns.inspectNameRecord()))
	s = verifyCommonName(ns, vrf, count)
	assertOnTrue(t, s != true, fmt.Sprintf("endpoint name doesnt exist, %+v", ns.inspectNameRecord()))
	s = verifyEndPointGroup(ns, vrf, epg, count)
	assertOnTrue(t, s != true, fmt.Sprintf("endpoint group doesnt exist, %+v", ns.inspectNameRecord()))
	endPointEvent("del", ns, vrf, nw, false, epg, count)
	s = verifyEndpointID(ns, vrf, false, count)
	assertOnTrue(t, s == true, fmt.Sprintf("endpoints exist, %+v", ns.inspectNameRecord()))
	s = verifyCommonName(ns, vrf, count)
	assertOnTrue(t, s == true, fmt.Sprintf("endpoint name exists, %+v", ns.inspectNameRecord()))
	s = verifyEndPointGroup(ns, vrf, epg, count)
	assertOnTrue(t, s == true, fmt.Sprintf("endpoint group exists, %+v", ns.inspectNameRecord()))
}

func TestEnpointOperations(t *testing.T) {
	enpointOperation(t, 1)
	enpointOperation(t, 10)
	enpointOperation(t, 200)
}

func servicesOperation(t *testing.T, count int) {
	ns := new(NetpluginNameServer)
	ds := new(dummyState)
	err := ns.Init(ds)
	assertOnErr(t, err, "namespace init")
	vrf := "tenant1"
	nw := "net1"
	serviceEvent("add", ns, vrf, nw, count)
	s := verifyServiceName(ns, vrf, false, count)
	assertOnTrue(t, s != true, fmt.Sprintf("service doesnt exist, %+v", ns.inspectNameRecord()))

	serviceEvent("del", ns, vrf, nw, count)
	s = verifyServiceName(ns, vrf, false, count)
	assertOnTrue(t, s == true, fmt.Sprintf("service exist, %+v", ns.inspectNameRecord()))
}

func TestServicesOperations(t *testing.T) {
	servicesOperation(t, 1)
	servicesOperation(t, 10)
	servicesOperation(t, 32)
}

func TestInspectState(t *testing.T) {
	ns := new(NetpluginNameServer)
	ds := new(dummyState)
	err := ns.Init(ds)
	assertOnErr(t, err, "namespace init")
	vrf := "tenant1"
	nw := "net1"
	epg := "epg1"
	endPointEvent("add", ns, vrf, nw, false, epg, 1)
	time.Sleep(1 * time.Second)
	s := verifyEndpointID(ns, vrf, false, 1)
	assertOnTrue(t, s != true, fmt.Sprintf("endpoint doesnt exist, %+v", ns.inspectNameRecord()))
	s = verifyCommonName(ns, vrf, 1)
	assertOnTrue(t, s != true, fmt.Sprintf("endpoint name doesnt exist, %+v", ns.inspectNameRecord()))
	s = verifyEndPointGroup(ns, vrf, epg, 1)
	assertOnTrue(t, s != true, fmt.Sprintf("endpoint group doesnt exist, %+v", ns.inspectNameRecord()))
	nr := ns.inspectNameRecord()
	tenantRecord, ok := nr[vrf]
	ipaddr := "ipv4:10.36.28.1 ipv6:<nil>"
	utlog.Infof("stats:%+v", tenantRecord)
	assertOnTrue(t, ok != true, "no tenant "+vrf)
	assertOnTrue(t, tenantRecord["commonNames"]["testendpoint-1"][0] != ipaddr,
		fmt.Sprintf("no endpoint name, %+v", tenantRecord))
	assertOnTrue(t, tenantRecord["endpointGroups"][epg][0] != ipaddr,
		fmt.Sprintf("no epg name, %+v", tenantRecord))
	assertOnTrue(t, tenantRecord["endpoints"]["BAADCAFE30"][0] != ipaddr,
		fmt.Sprintf("no endpoint id, %+v", tenantRecord))
	endPointEvent("del", ns, vrf, nw, false, epg, 1)

	ns.incTenantStats("vrf1", "foundNameRecord")
	ns.incTenantErrStats("vrf1", "invalidQuery")
	ns.incTenantStats("", "globalstats")
	stats := ns.inspectStats()
	assertOnTrue(t, stats["vrf1"]["foundNameRecord"] != 1, fmt.Sprintf("stats error %+v", stats))
	assertOnTrue(t, stats["vrf1"]["invalidQueryError"] != 1, fmt.Sprintf("stats error %+v", stats))
	assertOnTrue(t, stats[""]["globalstats"] != 1, fmt.Sprintf("stats error %+v", stats))
}

func TestV4EndPointLookup(t *testing.T) {
	ns := new(NetpluginNameServer)
	ds := new(dummyState)
	err := ns.Init(ds)
	assertOnErr(t, err, "namespace init")
	vrf := "tenant1"
	nw := "net1"
	epg := "epg1"
	IPAddr := "10.36.28.1"
	endPointEvent("add", ns, vrf, nw, false, epg, 1)
	time.Sleep(1 * time.Second)
	s := verifyEndpointID(ns, vrf, false, 1)
	assertOnTrue(t, s != true, fmt.Sprintf("endpoint doesnt exist, %+v", ns.inspectNameRecord()))
	s = verifyCommonName(ns, vrf, 1)
	assertOnTrue(t, s != true, fmt.Sprintf("endpoint name doesnt exist, %+v", ns.inspectNameRecord()))
	s = verifyEndPointGroup(ns, vrf, epg, 1)
	assertOnTrue(t, s != true, fmt.Sprintf("endpoint group doesnt exist, %+v", ns.inspectNameRecord()))

	//epg
	if true {
		q1 := new(dns.Msg)
		q1.SetQuestion(epg+".", dns.TypeA)
		dmsg, err := q1.Pack()
		assertOnErr(t, err, "failed to pack query")
		br, err1 := ns.NsLookup(dmsg, &vrf)
		assertOnErr(t, err1, "lookup failed")
		resp := new(dns.Msg)
		err = resp.Unpack(br)
		assertOnErr(t, err, "failed to unpack response")
		assertOnTrue(t, resp.Response != true, fmt.Sprintf("not a valid resp %+v", resp))

		assertOnTrue(t, len(resp.Answer) != 1, fmt.Sprintf("not a valid answer %+v", resp.Answer))
		a1, ok := resp.Answer[0].(*dns.A)
		assertOnTrue(t, ok != true, fmt.Sprintf("expected A record, %+v", resp.Answer))
		assertOnTrue(t, a1.A.String() != IPAddr, fmt.Sprintf("invalid ip address, %+v", a1.A))
		h := a1.Hdr
		assertOnTrue(t, h.Class != dns.ClassINET, fmt.Sprintf("not a valid class: %+v", h))
		assertOnTrue(t, h.Name != epg+".", fmt.Sprintf("not a valid name: %+v", h))
		assertOnTrue(t, h.Ttl != nameServerMaxTTL, fmt.Sprintf("not a valid ttl: %+v", h))
		assertOnTrue(t, h.Rrtype != dns.TypeA, fmt.Sprintf("not a valid rtype: %+v", h))

	}

	// name
	if true {
		cname := "testendpoint-1"
		q1 := new(dns.Msg)
		q1.SetQuestion(cname+".", dns.TypeA)
		dmsg, err := q1.Pack()
		assertOnErr(t, err, "failed to pack query")
		br, err1 := ns.NsLookup(dmsg, &vrf)
		assertOnErr(t, err1, "lookup failed")
		resp := new(dns.Msg)
		err = resp.Unpack(br)
		assertOnErr(t, err, "failed to unpack response")
		assertOnTrue(t, resp.Response != true, fmt.Sprintf("not a valid resp %+v", resp))
		assertOnTrue(t, len(resp.Answer) != 1, fmt.Sprintf("not a valid answer %+v", resp.Answer))

		a1, ok := resp.Answer[0].(*dns.A)
		assertOnTrue(t, ok != true, fmt.Sprintf("expected A record, %+v", resp.Answer))
		assertOnTrue(t, a1.A.String() != IPAddr, fmt.Sprintf("invalid ip address, %+v", a1.A))
		h := a1.Hdr
		assertOnTrue(t, h.Class != dns.ClassINET, fmt.Sprintf("not a valid class: %+v", h))
		assertOnTrue(t, h.Name != cname+".", fmt.Sprintf("not a valid name: %+v", h))
		assertOnTrue(t, h.Ttl != nameServerMaxTTL, fmt.Sprintf("not a valid ttl: %+v", h))
		assertOnTrue(t, h.Rrtype != dns.TypeA, fmt.Sprintf("not a valid rtype: %+v", h))
	}
	endPointEvent("del", ns, vrf, nw, false, epg, 1)

}

func TestV4EndPointLookupMultiRecord(t *testing.T) {
	ns := new(NetpluginNameServer)
	ds := new(dummyState)
	err := ns.Init(ds)
	assertOnErr(t, err, "namespace init")

	//epg
	vrf := "tenant1"
	nw := "net1"
	epg := "epg1"
	endPointEvent("add", ns, vrf, nw, false, epg, 10)
	time.Sleep(1 * time.Second)
	s := verifyEndpointID(ns, vrf, false, 10)
	assertOnTrue(t, s != true, fmt.Sprintf("endpoint doesnt exist, %+v", ns.inspectNameRecord()))
	s = verifyCommonName(ns, vrf, 10)
	assertOnTrue(t, s != true, fmt.Sprintf("endpoint name doesnt exist, %+v", ns.inspectNameRecord()))
	s = verifyEndPointGroup(ns, vrf, epg, 10)
	assertOnTrue(t, s != true, fmt.Sprintf("endpoint group doesnt exist, %+v", ns.inspectNameRecord()))

	q1 := new(dns.Msg)
	q1.SetQuestion(epg+".", dns.TypeA)
	dmsg, err := q1.Pack()
	assertOnErr(t, err, "failed to pack query")
	br, err1 := ns.NsLookup(dmsg, &vrf)
	assertOnErr(t, err1, "lookup failed")
	resp := new(dns.Msg)
	err = resp.Unpack(br)
	assertOnErr(t, err, "failed to unpack response")
	assertOnTrue(t, resp.Response != true, fmt.Sprintf("not a valid resp %+v", resp))

	assertOnTrue(t, len(resp.Answer) != 10, fmt.Sprintf("not a valid answer %+v", resp.Answer))

	IPaddr := map[string]bool{}
	for i := 0; i < 10; i++ {
		IPaddr[fmt.Sprintf("10.36.28.%d", i+1)] = true
	}
	for i := 0; i < 10; i++ {
		a1, ok := resp.Answer[i].(*dns.A)
		assertOnTrue(t, ok != true, fmt.Sprintf("expected A record, %+v", resp.Answer))

		_, ok = IPaddr[a1.A.String()]
		assertOnTrue(t, ok != true, fmt.Sprintf("invalid ip address, %+v", a1.A))
		delete(IPaddr, a1.A.String())
		h := a1.Hdr
		assertOnTrue(t, h.Class != dns.ClassINET, fmt.Sprintf("not a valid class: %+v", h))
		assertOnTrue(t, h.Name != epg+".", fmt.Sprintf("not a valid name: %+v", h))
		assertOnTrue(t, h.Ttl != nameServerMaxTTL, fmt.Sprintf("not a valid ttl: %+v", h))
		assertOnTrue(t, h.Rrtype != dns.TypeA, fmt.Sprintf("not a valid rtype: %+v", h))
	}
	assertOnTrue(t, len(IPaddr) != 0, fmt.Sprintf("missing A record, %+v", resp.Answer))

	//name
	for i := 0; i < 10; i++ {
		cname := fmt.Sprintf("testendpoint-%d", i+1)
		IPAddr := fmt.Sprintf("10.36.28.%d", i+1)
		q1 := new(dns.Msg)
		q1.SetQuestion(cname+".", dns.TypeA)
		dmsg, err := q1.Pack()
		assertOnErr(t, err, "failed to pack query")
		br, err1 := ns.NsLookup(dmsg, &vrf)
		assertOnErr(t, err1, "lookup failed")
		resp := new(dns.Msg)
		err = resp.Unpack(br)
		assertOnErr(t, err, "failed to unpack response")
		assertOnTrue(t, resp.Response != true, fmt.Sprintf("not a valid resp %+v", resp))
		assertOnTrue(t, len(resp.Answer) != 1, fmt.Sprintf("not a valid answer %+v", resp.Answer))

		a1, ok := resp.Answer[0].(*dns.A)
		assertOnTrue(t, ok != true, fmt.Sprintf("expected A record, %+v", resp.Answer))
		assertOnTrue(t, a1.A.String() != IPAddr, fmt.Sprintf("invalid ip address, %+v", a1.A))
		h := a1.Hdr
		assertOnTrue(t, h.Class != dns.ClassINET, fmt.Sprintf("not a valid class: %+v", h))
		assertOnTrue(t, h.Name != cname+".", fmt.Sprintf("not a valid name: %+v", h))
		assertOnTrue(t, h.Ttl != nameServerMaxTTL, fmt.Sprintf("not a valid ttl: %+v", h))
		assertOnTrue(t, h.Rrtype != dns.TypeA, fmt.Sprintf("not a valid rtype: %+v", h))
	}
	endPointEvent("del", ns, vrf, nw, false, epg, 10)
}

func TestV4ServiceLookup(t *testing.T) {
	ns := new(NetpluginNameServer)
	ds := new(dummyState)
	err := ns.Init(ds)
	assertOnErr(t, err, "namespace init")

	// single
	if true {
		vrf := "tenant1"
		nw := "net1"
		IPAddr := "10.36.28.1"
		serviceEvent("add", ns, vrf, nw, 1)
		s := verifyServiceName(ns, vrf, false, 1)
		assertOnTrue(t, s != true, fmt.Sprintf("service doesnt exist, %+v", ns.inspectNameRecord()))
		svcName := "testservice-1"
		q1 := new(dns.Msg)
		q1.SetQuestion(svcName+".", dns.TypeA)
		dmsg, err := q1.Pack()
		assertOnErr(t, err, "failed to pack query")
		br, err1 := ns.NsLookup(dmsg, &vrf)
		assertOnErr(t, err1, "lookup failed")
		resp := new(dns.Msg)
		err = resp.Unpack(br)
		assertOnErr(t, err, "failed to unpack response")
		assertOnTrue(t, resp.Response != true, fmt.Sprintf("not a valid resp %+v", resp))
		assertOnTrue(t, len(resp.Answer) != 1, fmt.Sprintf("not a valid answer %+v",
			resp.Answer))

		a1, ok := resp.Answer[0].(*dns.A)
		assertOnTrue(t, ok != true, fmt.Sprintf("expected A record, %+v", resp.Answer))
		assertOnTrue(t, a1.A.String() != IPAddr, fmt.Sprintf("invalid ip address, %+v", a1.A))
		h := a1.Hdr
		assertOnTrue(t, h.Class != dns.ClassINET, fmt.Sprintf("not a valid class: %+v", h))
		assertOnTrue(t, h.Name != svcName+".", fmt.Sprintf("not a valid name: %+v", h))
		assertOnTrue(t, h.Ttl != nameServerMaxTTL, fmt.Sprintf("not a valid ttl: %+v", h))
		assertOnTrue(t, h.Rrtype != dns.TypeA, fmt.Sprintf("not a valid rtype: %+v", h))
		serviceEvent("del", ns, vrf, nw, 1)
	}

	//multiple
	if true {
		vrf := "tenant1"
		nw := "net1"
		serviceEvent("add", ns, vrf, nw, 10)

		for i := 0; i < 10; i++ {
			svcName := fmt.Sprintf("testservice-%d", i+1)
			IPAddr := fmt.Sprintf("10.36.28.%d", i+1)
			q1 := new(dns.Msg)
			q1.SetQuestion(svcName+".", dns.TypeA)
			dmsg, err := q1.Pack()
			assertOnErr(t, err, "failed to pack query")
			br, err1 := ns.NsLookup(dmsg, &vrf)
			assertOnErr(t, err1, "lookup failed")
			resp := new(dns.Msg)
			err = resp.Unpack(br)
			assertOnErr(t, err, "failed to unpack response")
			assertOnTrue(t, resp.Response != true, fmt.Sprintf("not a valid resp %+v", resp))
			assertOnTrue(t, len(resp.Answer) != 1, fmt.Sprintf("not a valid answer %+v", resp.Answer))
			a1 := resp.Answer[0].(*dns.A)
			assertOnTrue(t, a1.A.String() != IPAddr, fmt.Sprintf("invalid ip address, %+v", a1.A))
			h := a1.Hdr
			assertOnTrue(t, h.Class != dns.ClassINET, fmt.Sprintf("not a valid class: %+v", h))
			assertOnTrue(t, h.Name != svcName+".", fmt.Sprintf("not a valid name: %+v", h))
			assertOnTrue(t, h.Ttl != nameServerMaxTTL, fmt.Sprintf("not a valid ttl: %+v", h))
			assertOnTrue(t, h.Rrtype != dns.TypeA, fmt.Sprintf("not a valid rtype: %+v", h))
		}
		serviceEvent("del", ns, vrf, nw, 10)

	}
}

func TestV6EndPointLookup(t *testing.T) {
	ns := new(NetpluginNameServer)
	ds := new(dummyState)
	err := ns.Init(ds)
	assertOnErr(t, err, "namespace init")
	vrf := "tenant1"
	nw := "net1"
	epg := "epg1"

	if true {

		endPointEvent("add", ns, vrf, nw, true, epg, 1)
		time.Sleep(1 * time.Second)
		s := verifyEndpointID(ns, vrf, false, 1)
		assertOnTrue(t, s != true, fmt.Sprintf("endpoint doesnt exist, %+v", ns.inspectNameRecord()))
		s = verifyCommonName(ns, vrf, 1)
		assertOnTrue(t, s != true, fmt.Sprintf("endpoint name doesnt exist, %+v", ns.inspectNameRecord()))
		s = verifyEndPointGroup(ns, vrf, epg, 1)
		assertOnTrue(t, s != true, fmt.Sprintf("endpoint group doesnt exist, %+v", ns.inspectNameRecord()))
		q1 := new(dns.Msg)
		q1.SetQuestion(epg+".", dns.TypeAAAA)
		dmsg, err := q1.Pack()
		assertOnErr(t, err, "failed to pack query")
		br, err1 := ns.NsLookup(dmsg, &vrf)
		assertOnErr(t, err1, "lookup failed")

		resp := new(dns.Msg)
		err = resp.Unpack(br)
		assertOnErr(t, err, "failed to unpack response")
		assertOnTrue(t, resp.Response != true, fmt.Sprintf("not a valid resp %+v", resp))
		assertOnTrue(t, len(resp.Answer) != 1, fmt.Sprintf("not a valid answer %+v", resp.Answer))

		IPaddr := "2001:4860:0:2001::1"

		a1, ok := resp.Answer[0].(*dns.AAAA)
		assertOnTrue(t, ok != true, fmt.Sprintf("expected A record, %+v", resp.Answer))
		assertOnTrue(t, a1.AAAA.String() != IPaddr, fmt.Sprintf("invalid ip address, %+v", a1.AAAA))
		h := a1.Hdr
		assertOnTrue(t, h.Class != dns.ClassINET, fmt.Sprintf("not a valid class: %+v", h))
		assertOnTrue(t, h.Name != epg+".", fmt.Sprintf("not a valid name: %+v", h))
		assertOnTrue(t, h.Ttl != nameServerMaxTTL, fmt.Sprintf("not a valid ttl: %+v", h))
		assertOnTrue(t, h.Rrtype != dns.TypeAAAA, fmt.Sprintf("not a valid rtype: %+v", h))
		endPointEvent("del", ns, vrf, nw, true, epg, 1)

	}

	if true {
		endPointEvent("add", ns, vrf, nw, true, epg, 10)
		time.Sleep(1 * time.Second)
		s := verifyEndpointID(ns, vrf, false, 10)
		assertOnTrue(t, s != true, fmt.Sprintf("endpoint doesnt exist, %+v", ns.inspectNameRecord()))
		s = verifyCommonName(ns, vrf, 1)
		assertOnTrue(t, s != true, fmt.Sprintf("endpoint name doesnt exist, %+v", ns.inspectNameRecord()))
		s = verifyEndPointGroup(ns, vrf, epg, 10)
		assertOnTrue(t, s != true, fmt.Sprintf("endpoint group doesnt exist, %+v", ns.inspectNameRecord()))

		IPaddr := map[string]bool{}
		for i := 0; i < 10; i++ {
			IPaddr[fmt.Sprintf("2001:4860:0:2001::%d", i+1)] = true
		}

		q1 := new(dns.Msg)
		q1.SetQuestion(epg+".", dns.TypeAAAA)
		dmsg, err := q1.Pack()
		assertOnErr(t, err, "failed to pack query")
		br, err1 := ns.NsLookup(dmsg, &vrf)
		assertOnErr(t, err1, "lookup failed")

		resp := new(dns.Msg)
		err = resp.Unpack(br)
		assertOnErr(t, err, "failed to unpack response")
		assertOnTrue(t, resp.Response != true, fmt.Sprintf("not a valid resp %+v", resp))
		assertOnTrue(t, len(resp.Answer) != 10, fmt.Sprintf("not a valid answer %+v", resp.Answer))

		for i := 0; i < 10; i++ {
			a1, ok := resp.Answer[i].(*dns.AAAA)
			assertOnTrue(t, ok != true, fmt.Sprintf("expected AAAA record, %+v", resp.Answer))

			_, ok = IPaddr[a1.AAAA.String()]
			assertOnTrue(t, ok != true, fmt.Sprintf("invalid AAAA record, %+v", resp.Answer))
			delete(IPaddr, a1.AAAA.String())

			h := a1.Hdr
			assertOnTrue(t, h.Class != dns.ClassINET, fmt.Sprintf("not a valid class: %+v", h))
			assertOnTrue(t, h.Name != epg+".", fmt.Sprintf("not a valid name: %+v", h))
			assertOnTrue(t, h.Ttl != nameServerMaxTTL, fmt.Sprintf("not a valid ttl: %+v", h))
			assertOnTrue(t, h.Rrtype != dns.TypeAAAA, fmt.Sprintf("not a valid rtype: %+v", h))
		}
		assertOnTrue(t, len(IPaddr) != 0, fmt.Sprintf("missing AAAA record, %+v", IPaddr))
		endPointEvent("del", ns, vrf, nw, true, epg, 10)
	}

}

func TestEndPointGroupMax(t *testing.T) {
	ns := new(NetpluginNameServer)
	ds := new(dummyState)
	err := ns.Init(ds)
	assertOnErr(t, err, "namespace init")
	vrf := "tenant1"
	nw := "net1"
	epg := "epgMax"
	endPointEvent("add", ns, vrf, nw, false, epg, 20)
	time.Sleep(1 * time.Second)
	s := verifyEndpointID(ns, vrf, false, 20)
	assertOnTrue(t, s != true, fmt.Sprintf("endpoint doesnt exist, %+v", ns.inspectNameRecord()))
	s = verifyCommonName(ns, vrf, 20)
	assertOnTrue(t, s != true, fmt.Sprintf("endpoint name doesnt exist, %+v", ns.inspectNameRecord()))
	s = verifyEndPointGroup(ns, vrf, epg, 20)
	assertOnTrue(t, s != true, fmt.Sprintf("endpoint group doesnt exist, %+v", ns.inspectNameRecord()))

	q1 := new(dns.Msg)
	q1.SetQuestion(epg+".", dns.TypeA)
	dmsg, err := q1.Pack()
	assertOnErr(t, err, "failed to pack query")
	br, err1 := ns.NsLookup(dmsg, &vrf)
	assertOnErr(t, err1, "lookup failed")
	resp := new(dns.Msg)
	err = resp.Unpack(br)
	assertOnErr(t, err, "failed to unpack response")
	assertOnTrue(t, resp.Response != true, fmt.Sprintf("not a valid resp %+v", resp))
	assertOnTrue(t, len(resp.Answer) != 10, fmt.Sprintf("not a valid answer %+v", resp.Answer))
	endPointEvent("del", ns, vrf, nw, false, epg, 20)

}

func TestK8sLbSvc(t *testing.T) {
	ns := new(NetpluginNameServer)
	ds := new(dummyState)
	err := ns.Init(ds)
	assertOnErr(t, err, "namespace init")
	ns.AddLbService(K8sDefaultTenant, "lb1", "10.36.27.101")
	l, ok := ns.k8sService.Get("lb1")
	assertOnTrue(t, ok != true, fmt.Sprintf("no lb1 found %+v", ns.k8sService.Keys()))
	r, ok := l.(nameRecord)
	assertOnTrue(t, ok != true, fmt.Sprintf("not namerecord"))
	assertOnTrue(t, r.v4Record.String() != "10.36.27.101",
		fmt.Sprintf("no singletenant LB %+v", ns.k8sService.Keys()))
	assertOnTrue(t, ns.k8sService.Count() != 1,
		fmt.Sprintf("singletenant LB count error %+v", ns.k8sService.Keys()))
	ns.AddLbService(K8sDefaultTenant, "lb2", "10.36.27.102")
	l, ok = ns.k8sService.Get("lb2")
	assertOnTrue(t, ok != true, fmt.Sprintf("no lb2 found %+v", ns.k8sService.Keys()))
	r, ok = l.(nameRecord)
	assertOnTrue(t, ok != true, fmt.Sprintf("no namerecord"))
	assertOnTrue(t, r.v4Record.String() != "10.36.27.102",
		fmt.Sprintf("no singletenant LB %+v", ns.k8sService.Keys()))
	assertOnTrue(t, ns.k8sService.Count() != 2,
		fmt.Sprintf("singletenant LB count error%+v", ns.k8sService.Keys()))

	ns.DelLbService(K8sDefaultTenant, "lb2")
	l, ok = ns.k8sService.Get("lb2")
	assertOnTrue(t, ok == true, fmt.Sprintf("lb2 found %+v", ns.k8sService.Keys()))
	assertOnTrue(t, ns.k8sService.Count() != 1,
		fmt.Sprintf("singletenant LB count error %+v", ns.k8sService.Keys()))

	ns.DelLbService(K8sDefaultTenant, "lb1")
	l, ok = ns.k8sService.Get("lb1")
	assertOnTrue(t, ok == true, fmt.Sprintf("lb1 found %+v", ns.k8sService.Keys()))
	assertOnTrue(t, ns.k8sService.Count() != 0,
		fmt.Sprintf("singletenant LB count error %+v", ns.k8sService.Keys()))
}

func TestK8sSvcLookup(t *testing.T) {
	ns := new(NetpluginNameServer)
	ds := new(dummyState)
	err := ns.Init(ds)
	assertOnErr(t, err, "namespace init")

	for i := 0; i < 5; i++ {
		IPAddr := fmt.Sprintf("10.36.25.%d", i+1)
		ns.AddLbService(K8sDefaultTenant, fmt.Sprintf("kube%d", i+1), IPAddr)
	}

	assertOnTrue(t, len(ns.k8sService.Keys()) != 5,
		fmt.Sprintf("lb svc records missing %+v", ns.k8sService.Keys()))
	for i := 0; i < 5; i++ {
		IPAddr := fmt.Sprintf("10.36.25.%d", i+1)
		s, ok := ns.k8sService.Get(fmt.Sprintf("kube%d", i+1))
		assertOnTrue(t, ok != true, fmt.Sprintf("no service %+v", ns.k8sService.Keys()))
		r, ok := s.(nameRecord)
		assertOnTrue(t, ok != true, fmt.Sprintf("not namerecord type %+v", s))
		assertOnTrue(t, r.v4Record.String() != IPAddr, fmt.Sprintf("invalid ip %+v", r))
	}

	for i := 0; i < 5; i++ {
		vrf := K8sDefaultTenant
		IPAddr := fmt.Sprintf("10.36.25.%d", i+1)
		q1 := new(dns.Msg)
		q1.SetQuestion(fmt.Sprintf("kube%d.", i+1), dns.TypeA)
		dmsg, err := q1.Pack()
		assertOnErr(t, err, "failed to pack query")
		br, err1 := ns.NsLookup(dmsg, &vrf)
		assertOnErr(t, err1, "lookup failed")
		resp := new(dns.Msg)
		err = resp.Unpack(br)
		assertOnErr(t, err, "failed to unpack response")
		assertOnTrue(t, resp.Response != true, fmt.Sprintf("not a valid resp %+v", resp))
		assertOnTrue(t, len(resp.Answer) != 1, fmt.Sprintf("not a valid answer %+v", resp.Answer))
		a1, ok := resp.Answer[0].(*dns.A)
		assertOnTrue(t, ok != true, fmt.Sprintf("expected A record, %+v", resp.Answer))
		assertOnTrue(t, a1.A.String() != IPAddr, fmt.Sprintf("invalid ip address, %+v", a1.A))
		h := a1.Hdr
		assertOnTrue(t, h.Class != dns.ClassINET, fmt.Sprintf("not a valid class: %+v", h))
		assertOnTrue(t, h.Name != fmt.Sprintf("kube%d.", i+1), fmt.Sprintf("not a valid name: %+v", h))
		assertOnTrue(t, h.Ttl != nameServerMaxTTL, fmt.Sprintf("not a valid ttl: %+v", h))
		assertOnTrue(t, h.Rrtype != dns.TypeA, fmt.Sprintf("not a valid rtype: %+v", h))

	}
}

func TestK8sMultiTenantServiceLookup(t *testing.T) {
	ns := new(NetpluginNameServer)
	ds := new(dummyState)
	err := ns.Init(ds)
	assertOnErr(t, err, "namespace init")

	vrf := "tenant1"
	IPAddr := "10.36.28.1"
	svcName := "testservice-1"
	ns.AddLbService(vrf, svcName, IPAddr)
	s := verifyServiceName(ns, vrf, false, 1)
	assertOnTrue(t, s != true, fmt.Sprintf("service doesnt exist, %+v", ns.inspectNameRecord()))
	q1 := new(dns.Msg)
	q1.SetQuestion(svcName+".", dns.TypeA)
	dmsg, err := q1.Pack()
	assertOnErr(t, err, "failed to pack query")
	br, err1 := ns.NsLookup(dmsg, &vrf)
	assertOnErr(t, err1, "lookup failed")
	resp := new(dns.Msg)
	err = resp.Unpack(br)
	assertOnErr(t, err, "failed to unpack response")
	assertOnTrue(t, resp.Response != true, fmt.Sprintf("not a valid resp %+v", resp))
	assertOnTrue(t, len(resp.Answer) != 1, fmt.Sprintf("not a valid answer %+v",
		resp.Answer))

	a1, ok := resp.Answer[0].(*dns.A)
	assertOnTrue(t, ok != true, fmt.Sprintf("expected A record, %+v", resp.Answer))
	assertOnTrue(t, a1.A.String() != IPAddr, fmt.Sprintf("invalid ip address, %+v", a1.A))
	h := a1.Hdr
	assertOnTrue(t, h.Class != dns.ClassINET, fmt.Sprintf("not a valid class: %+v", h))
	assertOnTrue(t, h.Name != svcName+".", fmt.Sprintf("not a valid name: %+v", h))
	assertOnTrue(t, h.Ttl != nameServerMaxTTL, fmt.Sprintf("not a valid ttl: %+v", h))
	assertOnTrue(t, h.Rrtype != dns.TypeA, fmt.Sprintf("not a valid rtype: %+v", h))
	ns.DelLbService(vrf, svcName)
	s = verifyServiceName(ns, vrf, false, 1)
	assertOnTrue(t, s == true, fmt.Sprintf("service exist, %+v", ns.inspectNameRecord()))
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
