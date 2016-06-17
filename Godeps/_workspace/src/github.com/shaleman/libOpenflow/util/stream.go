package util

import (
	"bytes"
	"encoding/binary"
	"net"
	"strings"

	log "github.com/Sirupsen/logrus"
)

type BufferPool struct {
	Empty chan *bytes.Buffer
	Full  chan *bytes.Buffer
}

func NewBufferPool() *BufferPool {
	m := new(BufferPool)
	m.Empty = make(chan *bytes.Buffer, 50)
	m.Full = make(chan *bytes.Buffer, 50)

	for i := 0; i < 50; i++ {
		m.Empty <- bytes.NewBuffer(make([]byte, 0, 2048))
	}
	return m
}

// Parser interface
type Parser interface {
	Parse(b []byte) (message Message, err error)
}

type MessageStream struct {
	conn net.Conn
	pool *BufferPool
	// Message parser
	parser Parser
	// OpenFlow Version
	Version uint8
	// Channel on which to publish connection errors
	Error chan error
	// Channel on which to publish inbound messages
	Inbound chan Message
	// Channel on which to receive outbound messages
	Outbound chan Message
	// Channel on which to receive a shutdown command
	Shutdown chan bool
}

// Returns a pointer to a new MessageStream. Used to parse
// OpenFlow messages from conn.
func NewMessageStream(conn net.Conn, parser Parser) *MessageStream {
	m := &MessageStream{
		conn,
		NewBufferPool(),
		parser,
		0,
		make(chan error, 1),   // Error
		make(chan Message, 1), // Inbound
		make(chan Message, 1), // Outbound
		make(chan bool, 1),    // Shutdown
	}

	go m.outbound()
	go m.inbound()

	for i := 0; i < 25; i++ {
		go m.parse()
	}
	return m
}

func (m *MessageStream) GetAddr() net.Addr {
	return m.conn.RemoteAddr()
}

// Listen for a Shutdown signal or Outbound messages.
func (m *MessageStream) outbound() {
	for {
		select {
		case <-m.Shutdown:
			log.Infof("Closing OpenFlow message stream.")
			m.conn.Close()
			return
		case msg := <-m.Outbound:
			// Forward outbound messages to conn
			data, _ := msg.MarshalBinary()
			if _, err := m.conn.Write(data); err != nil {
				log.Warnln("OutboundError:", err)
				m.Error <- err
				m.Shutdown <- true
			}

			log.Debugf("Sent(%d): %v", len(data), data)
		}
	}
}

// Handle inbound messages
func (m *MessageStream) inbound() {
	msg := 0
	hdr := 0
	hdrBuf := make([]byte, 4)

	tmp := make([]byte, 2048)
	buf := <-m.pool.Empty
	for {
		n, err := m.conn.Read(tmp)
		if err != nil {
			// Handle explicitly disconnecting by closing connection
			if strings.Contains(err.Error(), "use of closed network connection") {
				return
			}
			log.Warnln("InboundError", err)
			m.Error <- err
			m.Shutdown <- true
			return
		}

		for i := 0; i < n; i++ {
			if hdr < 4 {
				hdrBuf[hdr] = tmp[i]
				buf.WriteByte(tmp[i])
				hdr += 1
				if hdr >= 4 {
					msg = int(binary.BigEndian.Uint16(hdrBuf[2:])) - 4
				}
				continue
			}
			if msg > 0 {
				buf.WriteByte(tmp[i])
				msg = msg - 1
				if msg == 0 {
					hdr = 0
					m.pool.Full <- buf
					buf = <-m.pool.Empty
				}
				continue
			}
		}
	}
}

// Parse incoming message
func (m *MessageStream) parse() {
	for {
		b := <-m.pool.Full
		log.Debugf("Rcvd: %v", b.Bytes())
		msg, err := m.parser.Parse(b.Bytes())
		// Log all message parsing errors.
		if err != nil {
			log.Print(err)
		}

		m.Inbound <- msg
		b.Reset()
		m.pool.Empty <- b
	}
}
