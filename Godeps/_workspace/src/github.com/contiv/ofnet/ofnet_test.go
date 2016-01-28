package ofnet

// Test ofnet APIs

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/contiv/ofnet/ovsdbDriver"

	log "github.com/Sirupsen/logrus"
	api "github.com/osrg/gobgp/api"
	"github.com/osrg/gobgp/packet"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const NUM_MASTER = 2
const NUM_AGENT = 5
const NUM_ITER = 4

var vrtrMasters [NUM_MASTER]*OfnetMaster
var vxlanMasters [NUM_MASTER]*OfnetMaster
var vrtrAgents [NUM_AGENT]*OfnetAgent
var vxlanAgents [NUM_AGENT]*OfnetAgent
var ovsDrivers [NUM_AGENT * 3]*ovsdbDriver.OvsDriver
var vlrtrAgent *OfnetAgent
var vlrtrMaster *OfnetMaster

//var localIpList []string = []string{"10.10.10.1", "10.10.10.2", "10.10.10.3", "10.10.10.4"}
var localIpList []string

// Create couple of ofnet masters and few agents
func TestMain(m *testing.M) {
	var err error

	for i := 0; i < NUM_AGENT; i++ {
		localIpList = append(localIpList, fmt.Sprintf("10.10.10.%d", (i+1)))
	}

	// Create the masters
	for i := 0; i < NUM_MASTER; i++ {
		vrtrMasters[i] = NewOfnetMaster(uint16(9301 + i))
		if vrtrMasters[i] == nil {
			log.Fatalf("Error creating ofnet master")
		}

		log.Infof("Created Master: %v", vrtrMasters[i])

		vxlanMasters[i] = NewOfnetMaster(uint16(9051 + i))
		if vxlanMasters[i] == nil {
			log.Fatalf("Error creating ofnet master")
		}

		log.Infof("Created Master: %v", vxlanMasters[i])

	}
	vlrtrMaster = NewOfnetMaster(uint16(9501))
	if vlrtrMaster == nil {
		log.Fatalf("Error creating ofnet master")
	}

	log.Infof("Created Master: %v", vlrtrMaster)

	// Wait a second for masters to be up
	time.Sleep(1 * time.Second)

	// Create agents
	for i := 0; i < NUM_AGENT; i++ {
		rpcPort := uint16(9101 + i)
		ovsPort := uint16(9151 + i)
		lclIp := net.ParseIP(localIpList[i])
		vrtrAgents[i], err = NewOfnetAgent("vrouter", lclIp, rpcPort, ovsPort)
		if err != nil {
			log.Fatalf("Error creating ofnet agent. Err: %v", err)
		}

		// Override MyAddr to local host
		vrtrAgents[i].MyAddr = "127.0.0.1"

		log.Infof("Created vrouter ofnet agent: %v", vrtrAgents[i])
	}

	for i := 0; i < NUM_AGENT; i++ {
		rpcPort := uint16(9201 + i)
		ovsPort := uint16(9251 + i)
		lclIp := net.ParseIP(localIpList[i])

		vxlanAgents[i], err = NewOfnetAgent("vxlan", lclIp, rpcPort, ovsPort)
		if err != nil {
			log.Fatalf("Error creating ofnet agent. Err: %v", err)
		}

		// Override MyAddr to local host
		vxlanAgents[i].MyAddr = "127.0.0.1"

		log.Infof("Created vxlan ofnet agent: %v", vxlanAgents[i])
	}

	// Create agent
	rpcPort := uint16(9551)
	ovsPort := uint16(9561)
	lclIp := net.ParseIP(localIpList[0])
	vlrtrAgent, err = NewOfnetAgent("vlrouter", lclIp, rpcPort, ovsPort, "50.1.1.1", "inb01")
	if err != nil {
		log.Fatalf("Error creating ofnet agent. Err: %v", err)
	}

	// Override MyAddr to local host
	vlrtrAgent.MyAddr = "127.0.0.1"

	log.Infof("Created vlrouter ofnet agent: %v", vlrtrAgent)

	masterInfo := OfnetNode{
		HostAddr: "127.0.0.1",
	}
	var resp bool

	// Add master node to each agent
	for i := 0; i < NUM_AGENT; i++ {
		// add the two master nodes
		for j := 0; j < NUM_MASTER; j++ {
			masterInfo.HostPort = uint16(9301 + j)
			// connect vrtr agent to vrtr master
			err := vrtrAgents[i].AddMaster(&masterInfo, &resp)
			if err != nil {
				log.Fatalf("Error adding master %+v to vrtr node %d. Err: %v", masterInfo, i, err)
			}

			// connect vxlan agents to vxlan master
			masterInfo.HostPort = uint16(9051 + j)
			err = vxlanAgents[i].AddMaster(&masterInfo, &resp)
			if err != nil {
				log.Fatalf("Error adding master %+v to vxlan node %d. Err: %v", masterInfo, i, err)
			}

		}
	}
	// connect vrtr agent to vrtr master

	masterInfo.HostPort = uint16(9501)
	err = vlrtrAgent.AddMaster(&masterInfo, &resp)
	if err != nil {
		log.Fatalf("Error adding master %+v to vlrtr node. Err: %v", masterInfo, err)
	}

	log.Infof("Ofnet masters and agents are setup..")

	time.Sleep(1 * time.Second)
	for i := 0; i < NUM_MASTER; i++ {
		err := vrtrMasters[i].MakeDummyRpcCall()
		if err != nil {
			log.Fatalf("Error making dummy rpc call. Err: %v", err)
			return
		}
		err = vxlanMasters[i].MakeDummyRpcCall()
		if err != nil {
			log.Fatalf("Error making dummy rpc call. Err: %v", err)
			return
		}
	}
	err = vlrtrMaster.MakeDummyRpcCall()
	if err != nil {
		log.Fatalf("Error making dummy rpc call. Err: %v", err)
		return
	}

	log.Infof("Made dummy rpc call to all agents")

	// Create OVS switches and connect them to vrouter ofnet agents
	for i := 0; i < NUM_AGENT; i++ {
		brName := "ovsbr1" + fmt.Sprintf("%d", i)
		ovsPort := uint16(9151 + i)
		ovsDrivers[i] = ovsdbDriver.NewOvsDriver(brName)
		err := ovsDrivers[i].AddController("127.0.0.1", ovsPort)
		if err != nil {
			log.Fatalf("Error adding controller to ovs: %s", brName)
		}
	}
	// Create OVS switches and connect them to vxlan ofnet agents
	for i := 0; i < NUM_AGENT; i++ {
		brName := "ovsbr2" + fmt.Sprintf("%d", i)
		ovsPort := uint16(9251 + i)
		j := NUM_AGENT + i
		ovsDrivers[j] = ovsdbDriver.NewOvsDriver(brName)
		err := ovsDrivers[j].AddController("127.0.0.1", ovsPort)
		if err != nil {
			log.Fatalf("Error adding controller to ovs: %s", brName)
		}
	}

	brName := "contivVlanBridge"
	ovsPort = uint16(9561)
	ovsDrivers[2*NUM_AGENT] = ovsdbDriver.NewOvsDriver(brName)
	err = ovsDrivers[2*NUM_AGENT].AddController("127.0.0.1", ovsPort)
	if err != nil {
		log.Fatalf("Error adding controller to ovs: %s", brName)
	}

	// Wait for 20sec for switch to connect to controller
	time.Sleep(20 * time.Second)

	err = SetupVlans()
	if err != nil {
		log.Fatalf("Error setting up Vlans")
	}
	err = SetupVteps()
	if err != nil {
		log.Fatalf("Error setting up vteps")
	}

	// run the test
	exitCode := m.Run()
	os.Exit(exitCode)

}

// test adding vlan
func SetupVlans() error {
	for i := 0; i < NUM_AGENT; i++ {
		log.Info("Infex %d \n", i)
		for j := 1; j < 5; j++ {
			log.Info("Infex %d \n", j)
			//log.Infof("Adding Vlan %d on %s", j, localIpList[i])
			err := vrtrAgents[i].AddNetwork(uint16(j), uint32(j), "")
			if err != nil {
				log.Errorf("Error adding vlan %d. Err: %v", j, err)
				return err
			}
			err = vxlanAgents[i].AddNetwork(uint16(j), uint32(j), "")
			if err != nil {
				log.Errorf("Error adding vlan %d. Err: %v", j, err)
				return err
			}
		}
	}
	err := vlrtrAgent.AddNetwork(uint16(1), uint32(1),
		fmt.Sprintf("10.10.%d.%d", 1, 1))
	if err != nil {
		log.Errorf("Error adding vlan 1. Err: %v", err)
		return err
	}
	return nil
}

// test adding full mesh vtep ports
func SetupVteps() error {
	for i := 0; i < NUM_AGENT; i++ {
		for j := 0; j < NUM_AGENT; j++ {
			if i != j {
				log.Infof("Adding VTEP on %s for remoteIp: %s", localIpList[i], localIpList[j])
				err := vrtrAgents[i].AddVtepPort(uint32(j+1), net.ParseIP(localIpList[j]))
				if err != nil {
					log.Errorf("Error adding VTEP port. Err: %v", err)
					return err
				}
				err = vxlanAgents[i].AddVtepPort(uint32(j+1), net.ParseIP(localIpList[j]))
				if err != nil {
					log.Errorf("Error adding VTEP port. Err: %v", err)
					return err
				}
			}
		}
	}
	log.Infof("Finished setting up VTEP ports..")
	return nil
}

// Test adding/deleting Vrouter routes
func TestOfnetVrouteAddDelete(t *testing.T) {
	for iter := 0; iter < NUM_ITER; iter++ {
		for i := 0; i < NUM_AGENT; i++ {
			j := i + 1
			macAddr, _ := net.ParseMAC(fmt.Sprintf("02:02:02:%02x:%02x:%02x", j, j, j))
			ipAddr := net.ParseIP(fmt.Sprintf("10.10.%d.%d", j, j))
			endpoint := EndpointInfo{
				PortNo:  uint32(NUM_AGENT + 2),
				MacAddr: macAddr,
				Vlan:    1,
				IpAddr:  ipAddr,
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
			brName := "ovsbr1" + fmt.Sprintf("%d", i)

			flowList, err := ofctlFlowDump(brName)
			if err != nil {
				t.Errorf("Error getting flow entries. Err: %v", err)
			}

			// verify flow entry exists
			for j := 0; j < NUM_AGENT; j++ {
				k := j + 1
				ipFlowMatch := fmt.Sprintf("priority=100,ip,nw_dst=10.10.%d.%d", k, k)
				ipTableId := IP_TBL_ID
				if !ofctlFlowMatch(flowList, ipTableId, ipFlowMatch) {
					t.Errorf("Could not find the route %s on ovs %s", ipFlowMatch, brName)
				}

				log.Infof("Found ipflow %s on ovs %s", ipFlowMatch, brName)
			}
		}

		log.Infof("Adding Vrouter endpoint successful.\n Testing Delete")

		for i := 0; i < NUM_AGENT; i++ {
			j := i + 1
			macAddr, _ := net.ParseMAC(fmt.Sprintf("02:02:02:%02x:%02x:%02x", j, j, j))
			ipAddr := net.ParseIP(fmt.Sprintf("10.10.%d.%d", j, j))
			endpoint := EndpointInfo{
				PortNo:  uint32(NUM_AGENT + 2),
				MacAddr: macAddr,
				Vlan:    1,
				IpAddr:  ipAddr,
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
			brName := "ovsbr1" + fmt.Sprintf("%d", i)

			flowList, err := ofctlFlowDump(brName)
			if err != nil {
				t.Errorf("Error getting flow entries. Err: %v", err)
			}

			// verify flow entry exists
			for j := 0; j < NUM_AGENT; j++ {
				k := j + 1
				ipFlowMatch := fmt.Sprintf("priority=100,ip,nw_dst=10.10.%d.%d", k, k)
				ipTableId := IP_TBL_ID
				if ofctlFlowMatch(flowList, ipTableId, ipFlowMatch) {
					t.Errorf("Still found the flow %s on ovs %s", ipFlowMatch, brName)
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
			endpoint := EndpointInfo{
				PortNo:  uint32(NUM_AGENT + 2),
				MacAddr: macAddr,
				Vlan:    1,
				IpAddr:  ipAddr,
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
			brName := "ovsbr2" + fmt.Sprintf("%d", i)

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
			endpoint := EndpointInfo{
				PortNo:  uint32(NUM_AGENT + 2),
				MacAddr: macAddr,
				Vlan:    1,
				IpAddr:  ipAddr,
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
			brName := "ovsbr2" + fmt.Sprintf("%d", i)

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

// Test adding/deleting Vlrouter routes
func TestOfnetVlrouteAddDelete(t *testing.T) {

	macAddr, _ := net.ParseMAC("02:02:01:06:06:06")
	ipAddr := net.ParseIP("20.20.20.20")
	endpoint := EndpointInfo{
		PortNo:  uint32(NUM_AGENT + 3),
		MacAddr: macAddr,
		Vlan:    1,
		IpAddr:  ipAddr,
	}

	log.Infof("Installing local vlrouter endpoint: %+v", endpoint)
	err := vlrtrAgent.AddNetwork(uint16(1), uint32(1), "20.20.20.254")
	if err != nil {
		t.Errorf("Error adding vlan 1 . Err: %v", err)
	}

	// Install the local endpoint
	err = vlrtrAgent.AddLocalEndpoint(endpoint)
	if err != nil {
		t.Fatalf("Error installing endpoint: %+v. Err: %v", endpoint, err)
		return
	}

	log.Infof("Finished adding local vlrouter endpoint")

	// verify all ovs switches have this route
	brName := "contivVlanBridge"
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

	log.Infof("Adding Vlrouter endpoint successful.\n Testing Delete")

	macAddr, _ = net.ParseMAC("02:02:01:06:06:06")
	ipAddr = net.ParseIP("20.20.20.20")
	endpoint = EndpointInfo{
		PortNo:  uint32(NUM_AGENT + 3),
		MacAddr: macAddr,
		Vlan:    1,
		IpAddr:  ipAddr,
	}

	log.Infof("Deleting local vlrouter endpoint: %+v", endpoint)

	// Install the local endpoint
	err = vlrtrAgent.RemoveLocalEndpoint(uint32(NUM_AGENT + 3))
	if err != nil {
		t.Fatalf("Error deleting endpoint: %+v. Err: %v", endpoint, err)
		return
	}

	log.Infof("Deleted endpoints. Verifying they are gone")

	// verify flows are deleted
	brName = "contivVlanBridge"

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

	log.Infof("Verified all flows are deleted")
}

// Test adding/deleting Vlrouter routes
func TestOfnetBgpVlrouteAddDelete(t *testing.T) {

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
	n, _ := bgp.NewPathAttributeNextHop("50.1.1.2").Serialize()
	path.Pattrs = append(path.Pattrs, n)
	vlrtrAgent.protopath.ModifyProtoRib(path)
	log.Infof("Adding path to the Bgp Rib")
	time.Sleep(2 * time.Second)

	// verify flow entry exists
	brName := "contivVlanBridge"

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
	vlrtrAgent.protopath.ModifyProtoRib(path)
	log.Infof("Withdrawing route from BGP rib")

	// verify flow entry exists
	brName = "contivVlanBridge"

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

func TestOfnetBgpPeerAddDelete(t *testing.T) {

	as := "500"
	peer := "50.1.1.2"

	//Add Bgp neighbor and check if it is successful

	err := vlrtrAgent.AddBgpNeighbors(as, peer)
	if err != nil {
		t.Errorf("Error adding Bgp Neighbor", err)
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
		t.Errorf("GetNeighbor failed ", err)
		return
	}

	//Delete BGP neighbor
	err = vlrtrAgent.DeleteBgpNeighbors()
	if err != nil {
		t.Errorf("Error Deleting Bgp Neighbor", err)
		return
	}

	//Check if neighbor is added to bgp server
	bgpPeer, err = client.GetNeighbor(context.Background(), arg)
	if bgpPeer != nil {
		t.Errorf("Neighbor is not deleted ", err)
		return
	}

}

// Wait for debug and cleanup
func TestWaitAndCleanup(t *testing.T) {
	time.Sleep(1 * time.Second)

	// Disconnect from switches.
	for i := 0; i < NUM_AGENT; i++ {
		vrtrAgents[i].Delete()
		vxlanAgents[i].Delete()
	}

	for i := 0; i < NUM_AGENT; i++ {
		brName := "ovsbr1" + fmt.Sprintf("%d", i)
		log.Infof("Deleting OVS bridge: %s", brName)
		err := ovsDrivers[i].DeleteBridge(brName)
		if err != nil {
			t.Errorf("Error deleting the bridge. Err: %v", err)
		}
	}
	for i := 0; i < NUM_AGENT; i++ {
		brName := "ovsbr2" + fmt.Sprintf("%d", i)
		log.Infof("Deleting OVS bridge: %s", brName)
		err := ovsDrivers[i].DeleteBridge(brName)
		if err != nil {
			t.Errorf("Error deleting the bridge. Err: %v", err)
		}
	}
	brName := "contivVlanBridge"
	log.Infof("Deleting OVS bridge: %s", brName)
	err := ovsDrivers[2*NUM_AGENT].DeleteBridge(brName)
	if err != nil {
		t.Errorf("Error deleting the bridge. Err: %v", err)
	}
}
