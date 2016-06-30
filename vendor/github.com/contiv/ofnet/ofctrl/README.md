# Ofctrl

This library implements a simple Openflow1.3 controller API

# Usage

    // Create a controller
    ctrler := ofctrl.NewController(&app)

    // Listen for connections
    ctrler.Listen(":6633")


This creates a new controller and registers the app for event callbacks. The app needs to implement following interface to get callbacks when an openflow switch connects to the controller.


    type AppInterface interface {
        // A Switch connected to the controller
        SwitchConnected(sw *OFSwitch)

        // Switch disconnected from the controller
        SwitchDisconnected(sw *OFSwitch)

        // Controller received a packet from the switch
        PacketRcvd(sw *OFSwitch, pkt *PacketIn)
    }

# Example app

    type OfApp struct {
        Switch *ofctrl.OFSwitch
    }

    func (o *OfApp) PacketRcvd(sw *ofctrl.OFSwitch, packet *openflow13.PacketIn) {
        log.Printf("App: Received packet: %+v", packet)
    }

    func (o *OfApp) SwitchConnected(sw *ofctrl.OFSwitch) {
        log.Printf("App: Switch connected: %v", sw.DPID())

        // Store switch for later use
        o.Switch = sw
    }

    func (o *OfApp) SwitchDisconnected(sw *ofctrl.OFSwitch) {
        log.Printf("App: Switch connected: %v", sw.DPID())
    }

    // Main app
    var app OfApp

    // Create a controller
    ctrler := ofctrl.NewController(&app)

    // start listening
    ctrler.Listen(":6633")

# Working with OpenVswitch

### Command to make ovs connect to controller:
`ovs-vsctl set-controller <bridge-name> tcp:<ip-addr>:<port>`

Example:

    sudo ovs-vsctl set-controller ovsbr0 tcp:127.0.0.1:6633

### To enable openflow1.3 support in OVS:
`ovs-vsctl set bridge <bridge-name> protocols=OpenFlow10,OpenFlow11,OpenFlow12,OpenFlow13`

Example:

    sudo ovs-vsctl set bridge ovsbr0 protocols=OpenFlow10,OpenFlow11,OpenFlow12,OpenFlow13

# Forwarding Graph API
An app can install flow table entries into the Openflow switch by using forwarding graph API.
Forwarding graph is made up of forwarding elements which determine how a packet lookups are done. Forwarding graph is a higher level interface that is converted to Openflow1.3 flows, instructions, groups and actions by the library

 Forwarding graph is specific to each switch. It is roughly structured as follows
```
         +------------+
         | Controller |
         +------------+
                |
      +---------+---------+
      |                   |
 +----------+        +----------+
 | Switch 1 |        | Switch 2 |
 +----------+        +----------+
       |
       +--------------+---------------+
       |              |               |
       V              V
 +---------+      +---------+     +---------+
 | Table 1 |  +-->| Table 2 |  +->| Table 3 |
 +---------+  |   +---------+  |  +---------+
      |       |        |       |      |
 +---------+  |   +---------+  |  +--------+     +------+
 | Flow 1  +--+   | Flow 1  +--+  | Flow 1 +---->| Drop |
 +---------+      +---------+     +--------+     +------+
      |
 +---------+            +----------+
 | Flow 2  +----------->+ OutPut 1 |
 +---------+            +----------+
      |
 +---------+                 +----------+
 | Flow 3  +---------------->| Output 2 |
 +---------+                 +----------+
      |                            ^
 +---------+       +---------+     |      +----------+
 | Flow 4  +------>| Flood 1 +-----+----->| Output 3 |
 +---------+       +---------+     |      +----------+
      |                            |
 +---------+     +-----------+     |      +----------+
 | Flow 5  +---->| Multipath |     +----->| Output 4 |
 +---------+     +-----+-----+            +----------+
                       |
          +------------+-------------+
          |            |             |
    +----------+  +----------+  +----------+
    | Output 5 |  | Output 6 |  | Output 7 |
    +----------+  +----------+  +----------+
```

 Forwarding graph is made up of Fgraph elements. Currently there are four
 kinds of elements.

    1. Table - Represents a flow table
    2. Flow - Represents a specific flow
    3. Output - Represents an output action either drop or send it on a port
    4. Flood - Represents flood to list of ports

In future we will support an additional type.

    5. Multipath - Represents load balancing across a set of ports

Forwarding Graph elements are linked together as follows

 - Each Switch has a set of Tables. Switch has a special DefaultTable where all packet lookups start.
 - Each Table contains list of Flows. Each Flow has a Match condition which determines the packets that match the flow and a NextElem which it points to
 - A Flow can point to following elements
      1. Table - This moves the forwarding lookup to specified table
      2. Output - This causes the packet to be sent out or dropped
      3. Flood  - This causes the packet to be flooded to list of ports
      4. Multipath - This causes packet to be load balanced across set of ports. This can be used for link aggregation and ECMP
 - There are three kinds of outputs
      1. drop - which causes the packet to be dropped
      2. toController - sends the packet to controller
      3. port - sends the packet out of specified port. Tunnels like Vxlan VTEP are also represented as ports.
 - A flow can have additional actions like:
    1. Set Vlan tag
    2. Set metadata Which is used for setting VRF for a packet
    3. Set VNI/tunnel header etc

 ----------------------------------------------------------------
 Example usage:
```
     // Find the switch we want to operate on
     switch := app.Switch

     // Create all tables
     rxVlanTbl := switch.NewTable(1)
     macSaTable := switch.NewTable(2)
     macDaTable := switch.NewTable(3)
     ipTable := switch.NewTable(4)
     inpTable := switch.DefaultTable() // table 0. i.e starting table

     // Discard mcast source mac
     dscrdMcastSrc := inpTable.NewFlow(FlowMatch{
                                      &McastSrc: { 0x01, 0, 0, 0, 0, 0 }
                                      &McastSrcMask: { 0x01, 0, 0, 0, 0, 0 }
                                      }, 100)
     dscrdMcastSrc.Next(switch.DropAction())

     // All valid packets go to vlan table
     validInputPkt := inpTable.NewFlow(FlowMatch{}, 1)
     validInputPkt.Next(rxVlanTbl)

     // Set access vlan for port 1 and go to mac lookup
     tagPort := rxVlanTbl.NewFlow(FlowMatch{
                                  InputPort: Port(1)
                                  }, 100)
     tagPort.SetVlan(10)
     tagPort.Next(macSaTable)

     // Match on IP dest addr and forward to a port
     ipFlow := ipTable.NewFlow(FlowParams{
                               Ethertype: 0x0800,
                               IpDa: &net.IPv4("10.10.10.10")
                              }, 100)

     outPort := switch.NewOutputPort(10)
     ipFlow.Next(outPort)
```
