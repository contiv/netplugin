package ofnet

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
	api "github.com/osrg/gobgp/api"
	"github.com/osrg/gobgp/packet"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// Test adding/deleting Vrouter routes
func TestOfnetVrouteAddDelete(t *testing.T) {
	for iter := 0; iter < NUM_ITER; iter++ {
		for i := 0; i < NUM_AGENT; i++ {
			j := i + 1
			macAddr, _ := net.ParseMAC(fmt.Sprintf("02:02:02:%02x:%02x:%02x", j, j, j))
			ipAddr := net.ParseIP(fmt.Sprintf("10.10.%d.%d", j, j))
			var ipv6Addr net.IP
			if j%2 == 0 {
				ipv6Addr = net.ParseIP(fmt.Sprintf("2016::%d:%d", j, j))
			}
			endpoint := EndpointInfo{
				PortNo:   uint32(NUM_AGENT + 2),
				MacAddr:  macAddr,
				Vlan:     1,
				IpAddr:   ipAddr,
				Ipv6Addr: ipv6Addr,
			}

			log.Infof("Installing local vrouter endpoint: %+v", endpoint)

			// Install the local endpoint
			err := vrtrAgents[i].AddLocalEndpoint(endpoint)
			if err != nil {
				t.Fatalf("Error installing endpoint: %+v. Err: %v", endpoint, err)
				return
			}
		}

		log.Infof("Finished adding local vrouter endpoint")

		// verify all ovs switches have this route
		for i := 0; i < NUM_AGENT; i++ {
			brName := "vrtrBridge" + fmt.Sprintf("%d", i)

			flowList, err := ofctlFlowDump(brName)
			if err != nil {
				t.Errorf("Error getting flow entries. Err: %v", err)
			}
			// verify flow entry exists
			for j := 0; j < NUM_AGENT; j++ {
				k := j + 1
				ipFlowMatch := fmt.Sprintf("priority=100,ip,metadata=0x100000000/0xff00000000,nw_dst=10.10.%d.%d", k, k)
				ipTableId := IP_TBL_ID
				if !ofctlFlowMatch(flowList, ipTableId, ipFlowMatch) {
					t.Errorf("Could not find the route %s on ovs %s", ipFlowMatch, brName)
				}
				log.Infof("Found ipflow %s on ovs %s", ipFlowMatch, brName)

				if k%2 == 0 {
					ipv6FlowMatch := fmt.Sprintf("priority=100,ipv6,metadata=0x100000000/0xff00000000,ipv6_dst=2016::%d:%d", k, k)
					if !ofctlFlowMatch(flowList, ipTableId, ipv6FlowMatch) {
						t.Errorf("Could not find IPv6 route %s on ovs %s", ipv6FlowMatch, brName)
						return
					}
					log.Infof("Found IPv6 ipflow %s on ovs %s", ipv6FlowMatch, brName)
				}
			}
		}

		log.Infof("Adding Vrouter endpoint successful.\n Testing Delete")

		for i := 0; i < NUM_AGENT; i++ {
			j := i + 1
			macAddr, _ := net.ParseMAC(fmt.Sprintf("02:02:02:%02x:%02x:%02x", j, j, j))
			ipAddr := net.ParseIP(fmt.Sprintf("10.10.%d.%d", j, j))
			var ipv6Addr net.IP
			if j%2 == 0 {
				ipv6Addr = net.ParseIP(fmt.Sprintf("2016::%d:%d", j, j))
			}
			endpoint := EndpointInfo{
				PortNo:   uint32(NUM_AGENT + 2),
				MacAddr:  macAddr,
				Vlan:     1,
				IpAddr:   ipAddr,
				Ipv6Addr: ipv6Addr,
			}

			log.Infof("Deleting local vrouter endpoint: %+v", endpoint)

			// Install the local endpoint
			err := vrtrAgents[i].RemoveLocalEndpoint(uint32(NUM_AGENT + 2))
			if err != nil {
				t.Fatalf("Error deleting endpoint: %+v. Err: %v", endpoint, err)
				return
			}
		}

		log.Infof("Deleted endpoints. Verifying they are gone")

		// verify flows are deleted
		for i := 0; i < NUM_AGENT; i++ {
			brName := "vrtrBridge" + fmt.Sprintf("%d", i)

			flowList, err := ofctlFlowDump(brName)
			if err != nil {
				t.Errorf("Error getting flow entries. Err: %v", err)
			}
			// verify flow entry exists
			for j := 0; j < NUM_AGENT; j++ {
				k := j + 1
				ipFlowMatch := fmt.Sprintf("priority=100,ip,metadata=0x100000000/0xff00000000,nw_dst=10.10.%d.%d", k, k)
				ipTableId := IP_TBL_ID
				if ofctlFlowMatch(flowList, ipTableId, ipFlowMatch) {
					t.Errorf("Still found the flow %s on ovs %s", ipFlowMatch, brName)
				}
				if k%2 == 0 {
					ipv6FlowMatch := fmt.Sprintf("priority=100,ipv6,metadata=0x100000000/0xff00000000,ipv6_dst=2016::%d:%d", k, k)
					if ofctlFlowMatch(flowList, ipTableId, ipv6FlowMatch) {
						t.Errorf("Still found the flow %s on ovs %s", ipv6FlowMatch, brName)
					}
				}
			}
		}

		log.Infof("Verified all flows are deleted")
	}
}

// Test adding/deleting Vxlan routes
func TestOfnetVxlanAddDelete(t *testing.T) {
	for iter := 0; iter < NUM_ITER; iter++ {
		for i := 0; i < NUM_AGENT; i++ {
			j := i + 1
			macAddr, _ := net.ParseMAC(fmt.Sprintf("02:02:02:%02x:%02x:%02x", j, j, j))
			ipAddr := net.ParseIP(fmt.Sprintf("10.10.%d.%d", j, j))
			var ipv6Addr net.IP
			if j%2 == 0 {
				ipv6Addr = net.ParseIP(fmt.Sprintf("2016::%d:%d", j, j))
			}
			endpoint := EndpointInfo{
				PortNo:   uint32(NUM_AGENT + 2),
				MacAddr:  macAddr,
				Vlan:     1,
				IpAddr:   ipAddr,
				Ipv6Addr: ipv6Addr,
			}

			log.Infof("Installing local vxlan endpoint: %+v", endpoint)

			// Install the local endpoint
			err := vxlanAgents[i].AddLocalEndpoint(endpoint)
			if err != nil {
				t.Errorf("Error installing endpoint: %+v. Err: %v", endpoint, err)
			}
		}

		log.Infof("Finished adding local vxlan endpoint")

		// verify all ovs switches have this route
		for i := 0; i < NUM_AGENT; i++ {
			brName := "vxlanBridge" + fmt.Sprintf("%d", i)

			flowList, err := ofctlFlowDump(brName)
			if err != nil {
				t.Errorf("Error getting flow entries. Err: %v", err)
			}
			// verify flow entry exists
			for j := 0; j < NUM_AGENT; j++ {
				k := j + 1
				macFlowMatch := fmt.Sprintf("priority=100,dl_vlan=1,dl_dst=02:02:02:%02x:%02x:%02x", k, k, k)

				macTableId := MAC_DEST_TBL_ID
				if !ofctlFlowMatch(flowList, macTableId, macFlowMatch) {
					t.Errorf("Could not find the mac flow %s on ovs %s", macFlowMatch, brName)
				}

				log.Infof("Found macFlow %s on ovs %s", macFlowMatch, brName)
			}
		}

		log.Infof("Add vxlan endpoint successful.\n Testing Delete")

		for i := 0; i < NUM_AGENT; i++ {
			j := i + 1
			macAddr, _ := net.ParseMAC(fmt.Sprintf("02:02:02:%02x:%02x:%02x", j, j, j))
			ipAddr := net.ParseIP(fmt.Sprintf("10.10.%d.%d", j, j))
			var ipv6Addr net.IP
			if j%2 == 0 {
				ipv6Addr = net.ParseIP(fmt.Sprintf("2016::%d:%d", j, j))
			}
			endpoint := EndpointInfo{
				PortNo:   uint32(NUM_AGENT + 2),
				MacAddr:  macAddr,
				Vlan:     1,
				IpAddr:   ipAddr,
				Ipv6Addr: ipv6Addr,
			}

			log.Infof("Deleting local vxlan endpoint: %+v", endpoint)

			// Install the local endpoint
			err := vxlanAgents[i].RemoveLocalEndpoint(uint32(NUM_AGENT + 2))
			if err != nil {
				t.Errorf("Error deleting endpoint: %+v. Err: %v", endpoint, err)
			}
		}

		log.Infof("Deleted endpoints. Verifying they are gone")

		// verify flow is deleted
		for i := 0; i < NUM_AGENT; i++ {
			brName := "vxlanBridge" + fmt.Sprintf("%d", i)

			flowList, err := ofctlFlowDump(brName)
			if err != nil {
				t.Errorf("Error getting flow entries. Err: %v", err)
			}

			// verify flow entry exists
			for j := 0; j < NUM_AGENT; j++ {
				k := j + 1
				macFlowMatch := fmt.Sprintf("priority=100,dl_vlan=1,dl_dst=02:02:02:%02x:%02x:%02x", k, k, k)

				macTableId := MAC_DEST_TBL_ID
				if ofctlFlowMatch(flowList, macTableId, macFlowMatch) {
					t.Errorf("Still found the mac flow %s on ovs %s", macFlowMatch, brName)
				}
			}
		}
	}
}

// Run an ovs-ofctl command
func runOfctlCmd(cmd, brName string) ([]byte, error) {
	cmdStr := fmt.Sprintf("sudo /usr/bin/ovs-ofctl -O Openflow13 %s %s", cmd, brName)
	out, err := exec.Command("/bin/sh", "-c", cmdStr).Output()
	if err != nil {
		log.Errorf("error running ovs-ofctl %s %s. Error: %v", cmd, brName, err)
		return nil, err
	}

	return out, nil
}

// dump the flows and parse the Output
func ofctlFlowDump(brName string) ([]string, error) {
	flowDump, err := runOfctlCmd("dump-flows", brName)
	if err != nil {
		log.Errorf("Error running dump-flows on %s. Err: %v", brName, err)
		return nil, err
	}

	log.Debugf("Flow dump: %s", flowDump)
	flowOutStr := string(flowDump)
	flowDb := strings.Split(flowOutStr, "\n")[1:]

	log.Debugf("flowDb: %+v", flowDb)

	var flowList []string
	for _, flow := range flowDb {
		felem := strings.Fields(flow)
		if len(felem) > 2 {
			felem = append(felem[:1], felem[2:]...)
			felem = append(felem[:2], felem[4:]...)
			fstr := strings.Join(felem, " ")
			flowList = append(flowList, fstr)
		}
	}

	log.Debugf("flowList: %+v", flowList)

	return flowList, nil
}

// Find a flow in flow list and match its action
func ofctlFlowMatch(flowList []string, tableId int, matchStr string) bool {
	mtStr := fmt.Sprintf("table=%d, %s", tableId, matchStr)
	for _, flowEntry := range flowList {
		log.Debugf("Looking for %s in %s", mtStr, flowEntry)
		if strings.Contains(flowEntry, mtStr) {
			return true
		}
	}

	return false
}

func TestOfnetBgpPeerAddDelete(t *testing.T) {

	neighborAs := "500"
	peer := "50.1.1.2"
	routerIP := "50.1.1.1/24"
	as := "65002"
	//Add Bgp neighbor and check if it is successful

	for i := 0; i < NUM_VLRTR_AGENT; i++ {
		err := vlrtrAgents[i].AddBgp(routerIP, as, neighborAs, peer)
		if err != nil {
			t.Errorf("Error adding Bgp Neighbor: %v", err)
			return
		}

		timeout := grpc.WithTimeout(time.Second)
		conn, err := grpc.Dial("127.0.0.1:8080", timeout, grpc.WithBlock(), grpc.WithInsecure())
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		client := api.NewGobgpApiClient(conn)
		if client == nil {
			t.Errorf("GoBgpApiclient is invalid")
		}
		arg := &api.Arguments{Name: peer}

		//Check if neighbor is added to bgp server
		bgpPeer, err := client.GetNeighbor(context.Background(), arg)
		if err != nil {
			t.Errorf("GetNeighbor failed: %v", err)
			return
		}

		//Delete BGP neighbor
		err = vlrtrAgents[i].DeleteBgp()
		if err != nil {
			t.Errorf("Error Deleting Bgp Neighbor: %v", err)
			return
		}

		//Check if neighbor is added to bgp server
		bgpPeer, err = client.GetNeighbor(context.Background(), arg)
		if bgpPeer != nil {
			t.Errorf("Neighbor is not deleted: %v", err)
			return
		}
	}
}

// Test adding/deleting Vlrouter routes
func TestOfnetVlrouteAddDelete(t *testing.T) {
	neighborAs := "500"
	peer := "50.1.1.2"
	routerIP := "50.1.1.1/24"
	as := "65002"
	//Add Bgp neighbor and check if it is successful

	for i := 0; i < NUM_VLRTR_AGENT; i++ {
		err := vlrtrAgents[i].AddBgp(routerIP, as, neighborAs, peer)
		if err != nil {
			t.Errorf("Error adding Bgp Neighbor: %v", err)
			return
		}

		macAddr, _ := net.ParseMAC("02:02:01:06:06:06")
		ipAddr := net.ParseIP("20.20.20.20")
		ipv6Addr := net.ParseIP("2020::20:20")
		endpoint := EndpointInfo{
			PortNo:   uint32(NUM_AGENT + 3),
			MacAddr:  macAddr,
			Vlan:     1,
			IpAddr:   ipAddr,
			Ipv6Addr: ipv6Addr,
		}

		log.Infof("Installing local vlrouter endpoint: %+v", endpoint)
		err = vlrtrAgents[i].AddNetwork(uint16(1), uint32(1), "20.20.20.254", "default")
		if err != nil {
			t.Errorf("Error adding vlan 1 . Err: %v", err)
		}

		// Install the local endpoint
		err = vlrtrAgents[i].AddLocalEndpoint(endpoint)
		if err != nil {
			t.Fatalf("Error installing endpoint: %+v. Err: %v", endpoint, err)
			return
		}

		log.Infof("Finished adding local vlrouter endpoint")

		// verify all ovs switches have this route
		brName := "vlrtrBridge" + fmt.Sprintf("%d", i)
		flowList, err := ofctlFlowDump(brName)
		if err != nil {
			t.Errorf("Error getting flow entries. Err: %v", err)
			return
		}

		// verify flow entry exists
		ipFlowMatch := fmt.Sprintf("priority=100,ip,nw_dst=20.20.20.20")
		ipTableId := IP_TBL_ID
		if !ofctlFlowMatch(flowList, ipTableId, ipFlowMatch) {
			t.Errorf("Could not find the route %s on ovs %s", ipFlowMatch, brName)
			return
		}

		log.Infof("Found ipflow %s on ovs %s", ipFlowMatch, brName)

		// verify IPv6 flow entry exists
		ipv6FlowMatch := fmt.Sprintf("priority=100,ipv6,ipv6_dst=2020::20:20")
		if !ofctlFlowMatch(flowList, ipTableId, ipv6FlowMatch) {
			t.Errorf("Could not find the route %s on ovs %s", ipv6FlowMatch, brName)
			return
		}
		log.Infof("Found ipv6 flow %s on ovs %s", ipv6FlowMatch, brName)

		log.Infof("Adding Vlrouter endpoint successful.\n Testing Delete")

		macAddr, _ = net.ParseMAC("02:02:01:06:06:06")
		ipAddr = net.ParseIP("20.20.20.20")
		ipv6Addr = net.ParseIP("2020::20:20")
		endpoint = EndpointInfo{
			PortNo:   uint32(NUM_AGENT + 3),
			MacAddr:  macAddr,
			Vlan:     1,
			IpAddr:   ipAddr,
			Ipv6Addr: ipv6Addr,
		}

		log.Infof("Deleting local vlrouter endpoint: %+v", endpoint)

		// Install the local endpoint
		err = vlrtrAgents[i].RemoveLocalEndpoint(uint32(NUM_AGENT + 3))
		if err != nil {
			t.Fatalf("Error deleting endpoint: %+v. Err: %v", endpoint, err)
			return
		}

		log.Infof("Deleted endpoints. Verifying they are gone")

		// verify flows are deleted
		brName = "vlrtrBridge" + fmt.Sprintf("%d", i)

		flowList, err = ofctlFlowDump(brName)
		if err != nil {
			t.Errorf("Error getting flow entries. Err: %v", err)
		}
		// verify flow entry exists
		ipFlowMatch = fmt.Sprintf("priority=100,ip,nw_dst=20.20.20.20")
		ipTableId = IP_TBL_ID
		if ofctlFlowMatch(flowList, ipTableId, ipFlowMatch) {
			t.Errorf("Still found the flow %s on ovs %s", ipFlowMatch, brName)
		}
		// verify IPv6 flow entry exists
		ipv6FlowMatch = fmt.Sprintf("priority=100,ipv6,ipv6_dst=2020::20:20")
		ipTableId = IP_TBL_ID
		if ofctlFlowMatch(flowList, ipTableId, ipv6FlowMatch) {
			t.Errorf("Still found the flow %s on ovs %s", ipv6FlowMatch, brName)
			return
		}
		err = vlrtrAgents[i].DeleteBgp()
		log.Infof("Verified all flows are deleted")
	}
}

// Test adding/deleting Vlrouter routes
func TestOfnetBgpVlrouteAddDelete(t *testing.T) {

	neighborAs := "500"
	peer := "50.1.1.3"
	routerIP := "50.1.1.2/24"
	as := "65002"
	//Add Bgp neighbor and check if it is successful

	for i := 0; i < NUM_VLRTR_AGENT; i++ {
		err := vlrtrAgents[i].AddBgp(routerIP, as, neighborAs, peer)
		time.Sleep(5 * time.Second)
		if err != nil {
			t.Errorf("Error adding Bgp Neighbor: %v", err)
			return
		}
		path := &api.Path{
			Pattrs: make([][]byte, 0),
		}
		nlri := bgp.NewIPAddrPrefix(32, "20.20.20.20")
		path.Nlri, _ = nlri.Serialize()
		origin, _ := bgp.NewPathAttributeOrigin(bgp.BGP_ORIGIN_ATTR_TYPE_EGP).Serialize()
		path.Pattrs = append(path.Pattrs, origin)
		aspathParam := []bgp.AsPathParamInterface{bgp.NewAs4PathParam(2, []uint32{65002})}
		aspath, _ := bgp.NewPathAttributeAsPath(aspathParam).Serialize()
		path.Pattrs = append(path.Pattrs, aspath)
		n, _ := bgp.NewPathAttributeNextHop("50.1.1.3").Serialize()
		path.Pattrs = append(path.Pattrs, n)
		vlrtrAgents[i].protopath.ModifyProtoRib(path)
		log.Infof("Adding path to the Bgp Rib")
		time.Sleep(2 * time.Second)

		// verify flow entry exists
		brName := "vlrtrBridge" + fmt.Sprintf("%d", i)

		flowList, err := ofctlFlowDump(brName)
		if err != nil {
			t.Errorf("Error getting flow entries. Err: %v", err)
		}
		ipFlowMatch := fmt.Sprintf("priority=100,ip,nw_dst=20.20.20.20")
		ipTableId := IP_TBL_ID
		if !ofctlFlowMatch(flowList, ipTableId, ipFlowMatch) {
			t.Errorf("Could not find the route %s on ovs %s", ipFlowMatch, brName)
			return
		}
		log.Infof("Found ipflow %s on ovs %s", ipFlowMatch, brName)

		// withdraw the route
		path.IsWithdraw = true
		vlrtrAgents[i].protopath.ModifyProtoRib(path)
		log.Infof("Withdrawing route from BGP rib")

		// verify flow entry exists
		brName = "vlrtrBridge" + fmt.Sprintf("%d", i)

		flowList, err = ofctlFlowDump(brName)
		if err != nil {
			t.Errorf("Error getting flow entries. Err: %v", err)
		}

		ipFlowMatch = fmt.Sprintf("priority=100,ip,nw_dst=20.20.20.20")
		ipTableId = IP_TBL_ID
		if ofctlFlowMatch(flowList, ipTableId, ipFlowMatch) {
			t.Errorf("Found the route %s on ovs %s which was withdrawn", ipFlowMatch, brName)
			return
		}
		log.Infof("ipflow %s on ovs %s has been deleted from OVS", ipFlowMatch, brName)
	}
}
