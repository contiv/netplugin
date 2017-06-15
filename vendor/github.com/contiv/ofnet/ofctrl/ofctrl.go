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

// This library implements a simple openflow 1.3 controller

import (
	"net"
	"strings"
	"sync"
	"time"

	"github.com/contiv/libOpenflow/common"
	"github.com/contiv/libOpenflow/openflow13"
	"github.com/contiv/libOpenflow/util"

	log "github.com/Sirupsen/logrus"
)

type PacketIn openflow13.PacketIn

// Note: Command to make ovs connect to controller:
// ovs-vsctl set-controller <bridge-name> tcp:<ip-addr>:<port>
// E.g.    sudo ovs-vsctl set-controller ovsbr0 tcp:127.0.0.1:6633

// To enable openflow1.3 support in OVS:
// ovs-vsctl set bridge <bridge-name> protocols=OpenFlow10,OpenFlow11,OpenFlow12,OpenFlow13
// E.g. sudo ovs-vsctl set bridge ovsbr0 protocols=OpenFlow10,OpenFlow11,OpenFlow12,OpenFlow13

type AppInterface interface {
	// A Switch connected to the controller
	SwitchConnected(sw *OFSwitch)

	// Switch disconnected from the controller
	SwitchDisconnected(sw *OFSwitch)

	// Controller received a packet from the switch
	PacketRcvd(sw *OFSwitch, pkt *PacketIn)

	// Controller received a multi-part reply from the switch
	MultipartReply(sw *OFSwitch, rep *openflow13.MultipartReply)
}

type Controller struct {
	app      AppInterface
	listener *net.TCPListener
	wg       sync.WaitGroup
}

// Create a new controller
func NewController(app AppInterface) *Controller {
	c := new(Controller)

	// for debug logs
	// log.SetLevel(log.DebugLevel)

	// Save the handler
	c.app = app
	return c
}

// Listen on a port
func (c *Controller) Listen(port string) {
	addr, _ := net.ResolveTCPAddr("tcp", port)

	var err error
	c.listener, err = net.ListenTCP("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	defer c.listener.Close()

	log.Println("Listening for connections on", addr)
	for {
		conn, err := c.listener.AcceptTCP()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				return
			}
			log.Fatal(err)
		}

		c.wg.Add(1)
		go c.handleConnection(conn)
	}

}

// Cleanup the controller
func (c *Controller) Delete() {
	c.listener.Close()
	c.wg.Wait()
	c.app = nil
}

// Handle TCP connection from the switch
func (c *Controller) handleConnection(conn net.Conn) {
	defer c.wg.Done()

	stream := util.NewMessageStream(conn, c)

	log.Println("New connection..")

	// Send ofp 1.3 Hello by default
	h, err := common.NewHello(4)
	if err != nil {
		return
	}
	stream.Outbound <- h

	for {
		select {
		// Send hello message with latest protocol version.
		case msg := <-stream.Inbound:
			switch m := msg.(type) {
			// A Hello message of the appropriate type
			// completes version negotiation. If version
			// types are incompatable, it is possible the
			// connection may be servered without error.
			case *common.Hello:
				if m.Version == openflow13.VERSION {
					log.Infoln("Received Openflow 1.3 Hello message")
					// Version negotiation is
					// considered complete. Create
					// new Switch and notifiy listening
					// applications.
					stream.Version = m.Version
					stream.Outbound <- openflow13.NewFeaturesRequest()
				} else {
					// Connection should be severed if controller
					// doesn't support switch version.
					log.Println("Received unsupported ofp version", m.Version)
					stream.Shutdown <- true
				}
			// After a vaild FeaturesReply has been received we
			// have all the information we need. Create a new
			// switch object and notify applications.
			case *openflow13.SwitchFeatures:
				log.Printf("Received ofp1.3 Switch feature response: %+v", *m)

				// Create a new switch and handover the stream
				NewSwitch(stream, m.DPID, c.app)

				// Let switch instance handle all future messages..
				return

			// An error message may indicate a version mismatch. We
			// disconnect if an error occurs this early.
			case *openflow13.ErrorMsg:
				log.Warnf("Received ofp1.3 error msg: %+v", *m)
				stream.Shutdown <- true
			}
		case err := <-stream.Error:
			// The connection has been shutdown.
			log.Println(err)
			return
		case <-time.After(time.Second * 3):
			// This shouldn't happen. If it does, both the controller
			// and switch are no longer communicating. The TCPConn is
			// still established though.
			log.Warnln("Connection timed out.")
			return
		}
	}
}

// Demux based on message version
func (c *Controller) Parse(b []byte) (message util.Message, err error) {
	switch b[0] {
	case openflow13.VERSION:
		message, err = openflow13.Parse(b)
	default:
		log.Errorf("Received unsupported openflow version: %d", b[0])
	}
	return
}
