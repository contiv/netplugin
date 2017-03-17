package govpp

import (
	"os"

	log "github.com/Sirupsen/logrus"

	"github.com/fdio-stack/govpp/api"
)

// Connection wraps a shared memory connection to VPP via VppAdapter.
type Connection struct {
	vpp         VppAdapter
	clients     map[uint32]*api.Channel
	maxClientId uint32
	encoder     *BinaryEncoder
}

var conn *Connection // global handle to Connection used in the callback

func Connect() *Connection {
	adapter := &PneumVppAdapter{}

	return connectWithAdapter(adapter)
}

func connectWithAdapter(vppAdapter VppAdapter) *Connection {
	// TODO: move logger to connection / channel?
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)

	c := &Connection{vpp: vppAdapter, encoder: &BinaryEncoder{}}
	c.clients = make(map[uint32]*api.Channel)

	c.vpp.SetMsgRecvCallback(msgRecvCb)
	c.vpp.Connect() // TODO: errcheck, wait in loop

	conn = c // TODO: allow only one connection, make this thread-safe
	return c
}

func (c *Connection) Disconnect() {
	c.vpp.Disconnect()
}

func (c *Connection) NewApiChannel() *api.Channel {
	ch := &api.Channel{}

	// TODO: atomic
	c.maxClientId++
	ch.ID = c.maxClientId

	//ch.ID = uint32(unsafe.Pointer(&ch)) // TODO: generate ID from pointer, remove ID from channel structure

	ch.ReqChan = make(chan *api.VppRequest)
	ch.ReplyChan = make(chan *api.VppReply)

	ch.Decoder = c.encoder

	// TODO: lock
	c.clients[ch.ID] = ch

	go c.watchRequests(ch)

	return ch
}

func msgRecvCb(context uint32, msgId uint16, data []byte) {
	log.WithFields(log.Fields{
		"Connection": context,
		"msg_id":     msgId,
		"msg_size":   len(data),
	}).Debug("Received a message from VPP.")

	// match client according to the Connection
	// TODO: lock
	client, ok := conn.clients[context]
	if ok {
		// send the data to he client
		client.ReplyChan <- &api.VppReply{MessageId: uint16(msgId), Data: data, LastPart: true} // TODO: proper multipart handling
	} else {
		log.WithFields(log.Fields{
			"Connection": context,
			"msg_id":     msgId,
		}).Error("Channel Connection not known, ignoring the message.")
	}
}

func (c *Connection) releaseClient(cl *api.Channel) {
	log.WithFields(log.Fields{
		"Connection": cl.ID,
	}).Debug("Channel disconnected.")

	// TODO: remove the client from client map
}

func (c *Connection) watchRequests(cl *api.Channel) {
	for req := range cl.ReqChan {

		msgId := c.getMsgId(req.Message.GetMessageName(), req.Message.GetCrcString())
		data, _ := c.encoder.EncodeMsg(req.Message, msgId) // TODO: handle error

		log.WithFields(log.Fields{
			"Connection": cl.ID,
			"msg_id":     msgId,
			"msg_size":   len(data),
		}).Debug("Sending a message to VPP.")

		c.vpp.SendMsg(cl.ID, data)
	}
	// after closing the channel, release the client
	c.releaseClient(cl)
}

func (c *Connection) getMsgId(msgName string, msgCrc string) uint16 {
	return c.vpp.GetMsgId(msgName, msgCrc)
}
