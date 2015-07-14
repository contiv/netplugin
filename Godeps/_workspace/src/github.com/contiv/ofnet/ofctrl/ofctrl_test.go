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
package ofctrl

import (
    "testing"
    "fmt"
    "time"
    "net"
    "strings"
    "os/exec"

    log "github.com/Sirupsen/logrus"
    // "github.com/shaleman/libOpenflow/openflow13"
    "github.com/contiv/ofnet/ovsdbDriver"
)

type OfActor struct {
    Switch *OFSwitch
    isSwitchConnected bool

    inputTable  *Table
    nextTable   *Table
}

func (o *OfActor) PacketRcvd(sw *OFSwitch, packet *PacketIn) {
    log.Printf("App: Received packet: %+v", packet)
}

func (o *OfActor) SwitchConnected(sw *OFSwitch) {
    log.Printf("App: Switch connected: %v", sw.DPID())

    // Store switch for later use
    o.Switch = sw

    o.isSwitchConnected = true
}

func (o *OfActor) SwitchDisconnected(sw *OFSwitch) {
    log.Printf("App: Switch connected: %v", sw.DPID())
}


var ofActor OfActor
var ctrler *Controller
var ovsDriver *ovsdbDriver.OvsDriver

// Run an ovs-ofctl command
func runOfctlCmd(cmd, brName string) ([]byte, error){
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
func ofctlFlowMatch(flowList []string, tableId int, matchStr, actStr string) bool {
    mtStr := fmt.Sprintf("table=%d, %s", tableId, matchStr)
    aStr := fmt.Sprintf("actions=%s", actStr)
    for _, flowEntry := range flowList {
        log.Debugf("Looking for %s %s in %s", mtStr, aStr, flowEntry)
        if strings.Contains(flowEntry, mtStr) && strings.Contains(flowEntry, aStr) {
            return true
        }
    }

    return false
}

// Test if OVS switch connects successfully
func TestOfctrlInit(t *testing.T) {
    // Create a controller
    ctrler = NewController(&ofActor)

    // start listening
    go ctrler.Listen(":6733")

    // Connect to ovsdb and add the controller
    ovsDriver = ovsdbDriver.NewOvsDriver("ovsbr11")
    err := ovsDriver.AddController("127.0.0.1", 6733)
    if err != nil {
        t.Fatalf("Error adding controller to ovs")
    }

    //wait for 10sec and see if switch connects
    time.Sleep(10 * time.Second)
    if !ofActor.isSwitchConnected {
        t.Fatalf("ovsbr0 switch did not connect within 20sec")
        return
    }

    log.Infof("Switch connected. Creating tables..")

    // Create initial tables
    ofActor.inputTable = ofActor.Switch.DefaultTable()
    if ofActor.inputTable == nil {
        t.Fatalf("Failed to get input table")
        return
    }

    ofActor.nextTable, err = ofActor.Switch.NewTable(1)
    if err != nil {
        t.Fatalf("Error creating next table. Err: %v", err)
        return
    }

    log.Infof("Openflow tables created successfully")
}

/* Experimental code
// Test connecting over unix socket
func TestUnixSocket(t *testing.T) {
    // Connect to unix socket
    _, err := net.Dial("unix", "/var/run/openvswitch/ovsbr11.mgmt")
    if (err != nil) {
        log.Printf("Failed to connect to unix socket. Err: %v", err)
        t.Fatalf("Failed to connect to unix socket. Err: %v", err)
        return
    }
}
*/

// test create/delete table
func TestTableCreateDelete(t *testing.T) {
    var tables  [12]*Table

    log.Infof("Creating tables..")
    // create the tables
    for i := 2; i < 12; i++ {
        var err error
        tables[i], err = ofActor.Switch.NewTable(uint8(i))
        if err != nil {
            t.Errorf("Error creating table: %d", i)
        }
    }

    log.Infof("Deleting tables..")

    // delete the tables
    for i := 2; i < 10; i++ {
        err := tables[i].Delete()
        if err != nil {
            t.Errorf("Error deleting table: %d", i)
        }
    }
}

func TestCreateDeleteFlow(t *testing.T) {
    inPortFlow, err := ofActor.inputTable.NewFlow(FlowMatch{
                            Priority: 100,
                            InputPort: 1,
                        })
    if err != nil {
        t.Errorf("Error creating inport flow. Err: %v", err)
    }

    // Set vlan and install it
    inPortFlow.SetVlan(1)
    err = inPortFlow.Next(ofActor.nextTable)
    if err != nil {
        t.Errorf("Error installing inport flow. Err: %v", err)
    }

    // create an output
    output, err := ofActor.Switch.OutputPort(1)
    if err != nil {
        t.Errorf("Error creating an output port. Err: %v", err)
    }

    // create mac flow
    macAddr, _ := net.ParseMAC("02:01:01:01:01:01")
    macFlow, err := ofActor.nextTable.NewFlow(FlowMatch{
                            Priority: 100,
                            VlanId: 1,
                            MacDa: &macAddr,
                        })
    if err != nil {
        t.Errorf("Error creating mac flow. Err: %v", err)
    }

    // Remove vlan and send out on a port
    macFlow.PopVlan()
    err = macFlow.Next(output)
    if err != nil {
        t.Errorf("Error installing the mac flow")
    }

    // Install ip flow
    ipAddr := net.ParseIP("10.10.10.10")
    ipFlow, err := ofActor.nextTable.NewFlow(FlowMatch{
                            Priority: 100,
                            Ethertype: 0x0800,
                            IpDa: &ipAddr,
                        })
    if err != nil {
        t.Errorf("Error installing ip flow. Err: %v", err)
    }

    err = ipFlow.Next(output)
    if err != nil {
        t.Errorf("Error installing the ip flow")
    }

    // verify it got installed
    flowList, err := ofctlFlowDump("ovsbr11")
    if err != nil {
        t.Errorf("Error getting flow entry")
    }

    // Match inport flow
    if !ofctlFlowMatch(flowList, 0, "priority=100,in_port=1",
                        "push_vlan:0x8100,set_field:4097->vlan_vid,goto_table:1") {
        t.Errorf("in port flow not found in OVS.")
    }

    // match ip flow
    if !ofctlFlowMatch(flowList, 1, "priority=100,ip,nw_dst=10.10.10.10",
                        "output:1") {
        t.Errorf("IP flow not found in OVS.")
    }

    // match mac flow
    if !ofctlFlowMatch(flowList, 1, "priority=100,dl_vlan=1,dl_dst=02:01:01:01:01:01",
                        "pop_vlan,output:1") {
        t.Errorf("Mac flow not found in OVS.")
    }

    // Delete the flow
    err = inPortFlow.Delete()
    if err != nil {
        t.Errorf("Error deleting the inPort flow. Err: %v", err)
    }

    // Delete the flow
    err = macFlow.Delete()
    if err != nil {
        t.Errorf("Error deleting the mac flow. Err: %v", err)
    }

    // Delete the flow
    err = ipFlow.Delete()
    if err != nil {
        t.Errorf("Error deleting the ip flow. Err: %v", err)
    }

    // Make sure they are really gone
    flowList, err = ofctlFlowDump("ovsbr11")
    if err != nil {
        t.Errorf("Error getting flow entry")
    }

    // Match inport flow and see if its still there..
    if ofctlFlowMatch(flowList, 0, "priority=100,in_port=1",
                        "push_vlan:0x8100,set_field:4097->vlan_vid,goto_table:1") {
        t.Errorf("in port flow still found in OVS after deleting it.")
    }

    // match ip flow
    if ofctlFlowMatch(flowList, 1, "priority=100,ip,nw_dst=10.10.10.10",
                        "output:1") {
        t.Errorf("IP flow not found in OVS.")
    }

    // match mac flow
    if ofctlFlowMatch(flowList, 1, "priority=100,dl_vlan=1,dl_dst=02:01:01:01:01:01",
                        "pop_vlan,output:1") {
        t.Errorf("Mac flow not found in OVS.")
    }
}


// Delete the bridge instance.
// This needs to be last test
func TestDeleteBridge(t *testing.T) {
    err := ovsDriver.DeleteBridge("ovsbr11")
    if err != nil {
        t.Errorf("Error deleting the bridge. Err: %v", err)
    }
}
