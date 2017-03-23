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
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/contiv/netplugin/core"
	"github.com/contiv/netplugin/netmaster/mastercfg"
	"github.com/miekg/dns"
	cmap "github.com/streamrail/concurrent-map"
	"hash/fnv"
	"net"
	"strings"
	"sync"
	"time"
)

// decorator log
var dnsLog *logrus.Entry

// limit number of records to 10, applicable for services or epgs
const maxNameRecordsInResp = 10

const nameServerMaxTTL = 120

type tenantBucket struct {
	sync.RWMutex
	tenantTables map[string]*dnsTables
}

// K8sDefaultTenant is the default tenant in K8S
const K8sDefaultTenant = "k8sDefaultTenant"

// NetpluginNameServer config
type NetpluginNameServer struct {
	svcKeyPath  string
	epKeyPath   string
	epChan      chan core.WatchState
	epErrChan   chan error
	svcChan     chan core.WatchState
	svcErrChan  chan error
	stateDriver core.StateDriver
	bucketSize  uint
	buckets     []tenantBucket
	k8sService  cmap.ConcurrentMap // for non-multi tenant LB service
	stats       struct {
		sync.RWMutex
		tenantStats map[string]map[string]uint64
	}
}

// DNS name record, ipv4 & ipv6 address
type nameRecord struct {
	v4Record net.IP
	v6Record net.IP
}

// dns records per tenant
type dnsTables struct {
	svcTbl      map[string]nameRecord      // LB service records
	endpointTbl map[string]nameRecord      // endpoint-id records
	epgTbl      map[string]map[string]bool // endpoint group records
	nameTbl     map[string]map[string]bool // container-name records
}

func lookUpServiceV4Record(record nameRecord, name string) ([]dns.RR, int) {
	rr := []dns.RR{}

	if record.v4Record.IsUnspecified() != true {
		r := new(dns.A)
		r.A = record.v4Record
		r.Hdr = dns.RR_Header{Name: name + ".", Rrtype: dns.TypeA,
			Class: dns.ClassINET, Ttl: nameServerMaxTTL}
		rr = append(rr, r)
	}

	return rr, len(rr)
}

func (dt *dnsTables) lookUpEndPointV4Record(nameList map[string]bool, name string) ([]dns.RR, int) {
	rr := []dns.RR{}
	for i := range nameList {
		if record, ok := dt.endpointTbl[i]; ok {
			if record.v4Record.IsUnspecified() != true {
				r := new(dns.A)
				r.A = record.v4Record
				r.Hdr = dns.RR_Header{Name: name + ".", Rrtype: dns.TypeA,
					Class: dns.ClassINET, Ttl: nameServerMaxTTL}
				rr = append(rr, r)
				if len(rr) >= maxNameRecordsInResp {
					return rr, len(rr)
				}
			}
		}
	}

	return rr, len(rr)
}

func (dt *dnsTables) lookUpEndPointV6Record(nameList map[string]bool, name string) ([]dns.RR, int) {
	rr := []dns.RR{}
	for i := range nameList {
		if record, ok := dt.endpointTbl[i]; ok {
			if record.v6Record.IsUnspecified() != true {
				r := new(dns.AAAA)
				r.AAAA = record.v6Record
				r.Hdr = dns.RR_Header{Name: name + ".", Rrtype: dns.TypeAAAA,
					Class: dns.ClassINET, Ttl: nameServerMaxTTL}

				rr = append(rr, r)
				if len(rr) >= maxNameRecordsInResp {
					return rr, len(rr)
				}
			}
		}
	}

	return rr, len(rr)
}

func (ens *NetpluginNameServer) getBucket(key string) *tenantBucket {
	hasher := fnv.New32a()
	hasher.Write([]byte(key))
	sum := hasher.Sum32()
	return &ens.buckets[uint(sum)%ens.bucketSize]
}

func (ens *NetpluginNameServer) getTenantName(key string) string {
	s := strings.Split(key, ".")
	return s[len(s)-1]
}

func (ens *NetpluginNameServer) getEpgName(key string) string {
	s := strings.Split(key, ":")
	return s[0]
}

// AddLbService adds a LB service in non-multi tenant record
func (ens *NetpluginNameServer) AddLbService(tenant string, name string, v4name string) {
	if len(v4name) > 0 {
		dnsLog.Infof("add k8s service %s ip %s", name, v4name)
		if tenant == K8sDefaultTenant {
			ens.k8sService.Set(name, nameRecord{v4Record: net.ParseIP(v4name)})
		} else {
			mc := mastercfg.CfgServiceLBState{
				Tenant:      tenant,
				ServiceName: name,
				IPAddress:   v4name,
			}
			ens.addService(&mc)
		}
	}
}

// DelLbService deletes LB service from non-multi tenant record
func (ens *NetpluginNameServer) DelLbService(tenant string, name string) {

	dnsLog.Infof("delete k8s service %s ", name)
	if tenant == K8sDefaultTenant {
		ens.k8sService.Remove(name)
	} else {
		mc := mastercfg.CfgServiceLBState{
			Tenant:      tenant,
			ServiceName: name,
		}
		ens.delService(&mc)
	}
}

func (ens *NetpluginNameServer) addService(s core.State) {
	svc, ok := s.(*mastercfg.CfgServiceLBState)
	if !ok {
		ens.incTenantErrStats("", "serviceAdd")
		return
	}

	dnsLog.Infof("[tenant: %s]add service name:%s, ipadddr: %s",
		svc.Tenant, svc.ServiceName, svc.IPAddress)
	tenant := svc.Tenant
	tenMap := ens.getBucket(tenant)
	tenMap.Lock()
	defer tenMap.Unlock()
	tenantTables, ok := tenMap.tenantTables[tenant]
	if !ok {
		tenantTables = new(dnsTables)
		tenMap.tenantTables[tenant] = tenantTables
	}

	if tenantTables.svcTbl == nil {
		tenantTables.svcTbl = make(map[string]nameRecord)
	}
	nr := nameRecord{v4Record: net.ParseIP(svc.IPAddress)}
	tenantTables.svcTbl[svc.ServiceName] = nr

}

func (ens *NetpluginNameServer) delService(s core.State) {
	svc, ok := s.(*mastercfg.CfgServiceLBState)
	if !ok {
		ens.incTenantErrStats("", "serviceDel")
		return
	}

	dnsLog.Infof("[tenant: %s]delete service name:%s, ipadddr: %s",
		svc.Tenant, svc.ServiceName, svc.IPAddress)

	tenant := svc.Tenant
	tenMap := ens.getBucket(tenant)
	tenMap.Lock()
	defer tenMap.Unlock()
	tenantTables, ok := tenMap.tenantTables[tenant]
	if ok && tenantTables.svcTbl != nil {
		delete(tenantTables.svcTbl, svc.ServiceName)
	}
}

func (ens *NetpluginNameServer) addEndpoint(s core.State) {
	eps, ok := s.(*mastercfg.CfgEndpointState)
	if !ok {
		ens.incTenantErrStats("", "endPointAdd")
		return
	}
	tenant := ens.getTenantName(eps.NetID)
	dnsLog.Infof("[tenant: %s]add endpoint epid:%s, epg:%s, name:%s, ipv4:%s ipv6:%s",
		tenant, eps.EndpointID, eps.EndpointGroupKey, eps.EPCommonName,
		eps.IPAddress, eps.IPv6Address)
	tenMap := ens.getBucket(tenant)

	tenMap.Lock()
	defer tenMap.Unlock()

	tenantTables, ok := tenMap.tenantTables[tenant]
	if !ok {
		tenantTables = new(dnsTables)
		tenMap.tenantTables[tenant] = tenantTables
	}

	//update endpoint
	if tenantTables.endpointTbl == nil {
		tenantTables.endpointTbl = make(map[string]nameRecord)
	}

	epEntry := nameRecord{
		v4Record: net.ParseIP(eps.IPAddress),
		v6Record: net.ParseIP(eps.IPv6Address),
	}

	tenantTables.endpointTbl[eps.EndpointID] = epEntry

	//update name
	if len(eps.EPCommonName) > 0 {
		containerName := eps.EPCommonName
		if eps.EPCommonName[:1] == "/" {
			containerName = eps.EPCommonName[1:]
		}

		if tenantTables.nameTbl == nil {
			tenantTables.nameTbl = make(map[string]map[string]bool)
		}
		nl, ok := tenantTables.nameTbl[containerName]
		if !ok {
			nl = make(map[string]bool)
			tenantTables.nameTbl[containerName] = nl
		}
		nl[eps.EndpointID] = true
	}

	//update epg
	if len(eps.EndpointGroupKey) > 0 {
		if tenantTables.epgTbl == nil {
			tenantTables.epgTbl = make(map[string]map[string]bool)
		}
		epgName := ens.getEpgName(eps.EndpointGroupKey)
		epgList, ok := tenantTables.epgTbl[epgName]
		if !ok {
			epgList = make(map[string]bool)
			tenantTables.epgTbl[epgName] = epgList
		}
		epgList[eps.EndpointID] = true
	}

}

func (ens *NetpluginNameServer) delEndpoint(s core.State) {
	eps, ok := s.(*mastercfg.CfgEndpointState)

	if !ok {
		ens.incTenantErrStats("", "endPointDel")
		return
	}

	tenant := ens.getTenantName(eps.NetID)
	dnsLog.Infof("[tenant: %s]delete endpoint: %s, epg: %s name:%s ipv4:%s ipv6:%s", tenant,
		eps.EndpointID, eps.EndpointGroupKey, eps.EPCommonName,
		eps.IPAddress, eps.IPv6Address)
	tenMap := ens.getBucket(tenant)
	tenMap.Lock()
	defer tenMap.Unlock()
	if tenantTables, ok := tenMap.tenantTables[tenant]; ok {
		if tenantTables.epgTbl != nil && len(eps.EndpointGroupKey) > 0 {
			epgName := ens.getEpgName(eps.EndpointGroupKey)
			epgList, ok := tenantTables.epgTbl[epgName]
			if ok {
				delete(epgList, eps.EndpointID)
				if len(epgList) <= 0 {
					delete(tenantTables.epgTbl, epgName)
				}
			}

		}
		if tenantTables.nameTbl != nil && len(eps.EPCommonName) > 0 {
			containerName := eps.EPCommonName
			if eps.EPCommonName[:1] == "/" {
				containerName = eps.EPCommonName[1:]
			}
			nameList, ok := tenantTables.nameTbl[containerName]
			if ok {
				delete(nameList, eps.EndpointID)
				if len(nameList) <= 0 {
					delete(tenantTables.nameTbl, containerName)
				}
			}
		}
		if tenantTables.endpointTbl != nil {
			delete(tenantTables.endpointTbl, eps.EndpointID)
		}
	}
}

func (ens *NetpluginNameServer) incTenantStats(tenant string, name string) {
	ens.stats.Lock()
	defer ens.stats.Unlock()
	s, ok := ens.stats.tenantStats[tenant]
	if !ok {
		ens.stats.tenantStats[tenant] = make(map[string]uint64)
		s = ens.stats.tenantStats[tenant]
	}
	v := s[name]
	v++
	s[name] = v
}

func (ens *NetpluginNameServer) incTenantErrStats(tenant string, inName string) {

	name := fmt.Sprintf("%sError", inName)
	ens.stats.Lock()
	defer ens.stats.Unlock()
	s, ok := ens.stats.tenantStats[tenant]
	if !ok {
		ens.stats.tenantStats[tenant] = make(map[string]uint64)
		s = ens.stats.tenantStats[tenant]
	}
	v := s[name]
	v++
	s[name] = v
}

func (ens *NetpluginNameServer) serveTypeA(tenant string, name string) ([]dns.RR, int) {

	// check non-multi tenant services for k8s
	if k8sSvc, svcOk := ens.k8sService.Get(name); svcOk {
		if sr, nrOk := k8sSvc.(nameRecord); nrOk {
			if rr, l := lookUpServiceV4Record(sr, name); l > 0 {
				return rr, l
			}
		}
	}

	tenMap := ens.getBucket(tenant)
	tenMap.RLock()
	defer tenMap.RUnlock()

	if dh, ok := tenMap.tenantTables[tenant]; ok {
		// service
		if svc, ok := dh.svcTbl[name]; ok {
			if rr, l := lookUpServiceV4Record(svc, name); l > 0 {
				return rr, l
			}
		}

		// epg
		if ep, ok := dh.epgTbl[name]; ok {
			if ep != nil {
				if rr, l := dh.lookUpEndPointV4Record(ep, name); l > 0 {
					return rr, l
				}
			}
		}

		// name
		if nm, ok := dh.nameTbl[name]; ok {
			if nm != nil {
				if rr, l := dh.lookUpEndPointV4Record(nm, name); l > 0 {
					return rr, l
				}
			}
		}
	}

	return nil, 0
}

func (ens *NetpluginNameServer) serveTypeAAAA(tenant string, name string) ([]dns.RR, int) {
	tenMap := ens.getBucket(tenant)
	tenMap.RLock()
	defer tenMap.RUnlock()

	if dh, ok := tenMap.tenantTables[tenant]; ok {
		// epg
		if ep, ok := dh.epgTbl[name]; ok {
			if rr, l := dh.lookUpEndPointV6Record(ep, name); l > 0 {
				return rr, l
			}
		}

		// name
		if nm, ok := dh.nameTbl[name]; ok {
			if rr, l := dh.lookUpEndPointV6Record(nm, name); l > 0 {
				return rr, l
			}
		}
	}

	return nil, 0
}

func (ens *NetpluginNameServer) serveNameRecord(tenant string, r *dns.Msg) ([]byte, error) {

	ansRR := []dns.RR{}
	for _, q1 := range r.Question {
		name := strings.TrimSuffix(q1.Name, ".")
		dnsLog.Infof("lookup name-record: %s ", q1.String())

		switch q1.Qtype {
		case dns.TypeA:
			if rr, l := ens.serveTypeA(tenant, name); l > 0 {
				ansRR = append(ansRR, rr...)
			}

		case dns.TypeAAAA:

			if rr, l := ens.serveTypeAAAA(tenant, name); l > 0 {
				ansRR = append(ansRR, rr...)
			}

		case dns.TypeANY:

			if rr, l := ens.serveTypeA(tenant, name); l > 0 {
				ansRR = append(ansRR, rr...)
				break
			}

			if rr, l := ens.serveTypeAAAA(tenant, name); l > 0 {
				ansRR = append(ansRR, rr...)
			}
		}
	}

	if len(ansRR) > 0 {
		m := &dns.Msg{}
		m.SetReply(r)
		m.Answer = ansRR
		m.Authoritative = true
		m.RecursionAvailable = true
		dnsLog.Infof("namerserver response: %s", m.String())

		return m.Pack()
	}
	return nil, errors.New("no record")
}

func (ens *NetpluginNameServer) inspectNameRecord() map[string]map[string]map[string][]string {

	inspectMap := map[string]map[string]map[string][]string{}
	for i := uint(0); i < ens.bucketSize; i++ {
		tm := &ens.buckets[i]
		tm.RLock()
		defer tm.RUnlock()
		for tk, tv := range tm.tenantTables {
			inspectMap[tk] = make(map[string]map[string][]string)

			svcMap := make(map[string][]string)
			for sk, sv := range tv.svcTbl {
				ipmap := fmt.Sprintf("ipv4:%s ipv6:%s",
					sv.v4Record.String(), sv.v6Record.String())
				svcMap[sk] = append(svcMap[sk], ipmap)
			}
			inspectMap[tk]["services"] = svcMap

			namemap := make(map[string][]string)
			for nk, nv := range tv.nameTbl {
				for k := range nv {
					if nr, ok := tv.endpointTbl[k]; ok {
						ipmap := fmt.Sprintf("ipv4:%s ipv6:%s",
							nr.v4Record.String(), nr.v6Record.String())
						namemap[nk] = append(namemap[nk], ipmap)
					} else {
						namemap[nk] = append(namemap[nk], k)
					}
				}

			}
			inspectMap[tk]["commonNames"] = namemap

			epgmap := make(map[string][]string)
			for egk, egv := range tv.epgTbl {
				for k := range egv {
					if nr, ok := tv.endpointTbl[k]; ok {
						ipmap := fmt.Sprintf("ipv4:%s ipv6:%s",
							nr.v4Record.String(), nr.v6Record.String())
						epgmap[egk] = append(epgmap[egk], ipmap)
					} else {
						epgmap[egk] = append(epgmap[egk], k)
					}
				}

			}
			inspectMap[tk]["endpointGroups"] = epgmap

			endpointMap := make(map[string][]string)
			for epk, epv := range tv.endpointTbl {
				ipmap := fmt.Sprintf("ipv4:%s ipv6:%s",
					epv.v4Record.String(), epv.v6Record.String())
				endpointMap[epk] = append(endpointMap[epk], ipmap)
			}
			inspectMap[tk]["endpoints"] = endpointMap

		}
	}

	// k8s services
	inspectMap[K8sDefaultTenant] = make(map[string]map[string][]string)
	k8sSvcMap := make(map[string][]string)
	for k8sKey, k8sVal := range ens.k8sService.Items() {
		if nr, ok := k8sVal.(nameRecord); ok {
			k8sSvcMap[k8sKey] = append(k8sSvcMap[k8sKey], fmt.Sprintf("ipv4:%s", nr.v4Record.String()))
		}
	}
	inspectMap[K8sDefaultTenant]["k8sServices"] = k8sSvcMap

	return inspectMap
}

func (ens *NetpluginNameServer) inspectStats() map[string]map[string]uint64 {
	ens.stats.RLock()
	defer ens.stats.RUnlock()
	statsMap := map[string]map[string]uint64{}
	for t, v := range ens.stats.tenantStats {
		statsMap[t] = make(map[string]uint64)
		for k, s := range v {
			statsMap[t][k] = s
		}
	}
	return statsMap
}

// InspectState returns state
func (ens *NetpluginNameServer) InspectState() (interface{}, error) {

	s := struct {
		SvcChan int                                       `json:"serviceQueue"`
		EpChan  int                                       `json:"endpointQueue"`
		Dtbl    map[string]map[string]map[string][]string `json:"dnsRecords"`
		Stats   map[string]map[string]uint64              `json:"stats"`
	}{SvcChan: len(ens.svcChan), EpChan: len(ens.epChan),
		Dtbl: ens.inspectNameRecord(), Stats: ens.inspectStats()}
	return &s, nil
}

// NsLookup returns name record,called from ofnet agent
func (ens *NetpluginNameServer) NsLookup(nsq []byte, vrfPtr *string) ([]byte, error) {
	tenant := *vrfPtr
	req := new(dns.Msg)
	err := req.Unpack(nsq)
	if err != nil {
		ens.incTenantStats(tenant, "invalidQuery")
		return nil, err
	}

	// no fancy requests
	if req.Response {
		ens.incTenantStats(tenant, "invalidQuery")
		return nil, errors.New("")
	}

	if req.IsTsig() != nil {
		ens.incTenantStats(tenant, "invalidQuery")
		return nil, errors.New("")
	}

	d, err := ens.serveNameRecord(tenant, req)
	if err != nil {
		logrus.Infof("no name record: %s", err)
		ens.incTenantStats(tenant, "noNameRecord")
		return nil, err
	}

	ens.incTenantStats(tenant, "foundNameRecord")
	return d, err

}

func (ens *NetpluginNameServer) readStateStore() {
	svc := mastercfg.CfgServiceLBState{}
	if st, err := ens.stateDriver.ReadAllState(ens.svcKeyPath, &svc, json.Unmarshal); err == nil {
		for _, s := range st {
			ens.addService(s)
		}
	}

	ep := mastercfg.CfgEndpointState{}
	if st, err := ens.stateDriver.ReadAllState(ens.epKeyPath, &ep, json.Unmarshal); err == nil {
		for _, s := range st {
			ens.addEndpoint(s)
		}
	}
}

func (ens *NetpluginNameServer) processStateEvent() {
	for {
		select {
		case <-ens.svcErrChan:
			dnsLog.Warnf("nameserver restarted service watcher")
			ens.incTenantErrStats("", "svcWatchRestart")
			go ens.startSvcWatch()

		case svcState := <-ens.svcChan:
			if svcState.Prev == nil {
				ens.addService(svcState.Curr)
			} else if svcState.Curr == nil {
				ens.delService(svcState.Prev)
			} else {
				// add again
				ens.addService(svcState.Curr)
			}

		case <-ens.epErrChan:
			dnsLog.Warnf("nameserver restarted endpoint watcher")
			ens.incTenantErrStats("", "epWatchRestart")
			go ens.startEndpointWatch()

		case state := <-ens.epChan:
			dnsLog.Infof("endpoint event %+v", state)
			if state.Prev == nil {
				ens.addEndpoint(state.Curr)
			} else if state.Curr == nil {
				ens.delEndpoint(state.Prev)
			} else {
				// add again
				ens.addEndpoint(state.Curr)
			}
		}
	}
}

func (ens *NetpluginNameServer) startEndpointWatch() {
	ep := mastercfg.CfgEndpointState{}

	if err := ens.stateDriver.WatchAllState(ens.epKeyPath,
		&ep, json.Unmarshal, ens.epChan); err != nil {
		dnsLog.Errorf("failed to watch endpoint events from nameserver %s", err)
		time.Sleep(5 * time.Second)
		ens.epErrChan <- err
	}
}

func (ens *NetpluginNameServer) startSvcWatch() {
	svc := mastercfg.CfgServiceLBState{}

	if err := ens.stateDriver.WatchAllState(ens.svcKeyPath,
		&svc, json.Unmarshal, ens.svcChan); err != nil {
		dnsLog.Errorf("failed to watch endpoint events from nameserver %s", err)
		time.Sleep(5 * time.Second)
		ens.svcErrChan <- err
	}
}

// Init to start name server
func (ens *NetpluginNameServer) Init(sd core.StateDriver) error {
	dnsLog = logrus.WithField("module", "nameserver")
	ens.bucketSize = 64
	ens.stateDriver = sd

	// don't change buffering
	ens.epChan = make(chan core.WatchState, 64)
	ens.epErrChan = make(chan error)
	ens.svcChan = make(chan core.WatchState, 8)
	ens.svcErrChan = make(chan error)
	ens.buckets = make([]tenantBucket, ens.bucketSize)
	ens.k8sService = cmap.New()

	for i := uint(0); i < ens.bucketSize; i++ {
		ens.buckets[i].tenantTables = make(map[string]*dnsTables)
		ens.stats.tenantStats = make(map[string]map[string]uint64)
	}
	ens.epKeyPath = mastercfg.StateConfigPath + "eps/"
	ens.svcKeyPath = mastercfg.StateConfigPath + "serviceLB/"
	go ens.processStateEvent()
	go ens.startSvcWatch()
	go ens.startEndpointWatch()
	ens.readStateStore()
	dnsLog.Infof("nameserver started")
	return nil
}
