package govpp

import (
	"bytes"
	"testing"
	"vpp-integration-with-etcd/vpp/govpp/api"

	"github.com/lunixbochs/struc"
)

type testRequest struct {
	SwIfIndex   uint32
	AdminUpDown uint8
	LinkUpDown  uint8
	Deleted     uint8
}

func (*testRequest) GetMessageName() string {
	return "test_request"
}
func (*testRequest) GetCrcString() string {
	return "c230f9b1"
}

type testReplyWithHeader struct {
	VlMsgID uint16
	Context uint32
	Retval  int32
}

type testReply struct {
	Retval int32
}

func (*testReply) GetMessageName() string {
	return "test_reply"
}
func (*testReply) GetCrcString() string {
	return "dfbf3afa"
}

type mockVppAdapter struct {
	callback MsgRecvCbFunc
}

func (a *mockVppAdapter) Connect() {
	// no op
}

func (a *mockVppAdapter) Disconnect() {
	// no op
}

func (a *mockVppAdapter) GetMsgId(msgName string, msgCrc string) uint16 {
	return 53
}

func (a *mockVppAdapter) SendMsg(clientId uint32, data []byte) {
	buf := new(bytes.Buffer)
	struc.Pack(buf, &testReplyWithHeader{})
	a.callback(clientId, 54, buf.Bytes())
}

func (a *mockVppAdapter) SetMsgRecvCallback(cb MsgRecvCbFunc) {
	a.callback = cb
}

func TestClient(t *testing.T) {

	vpp := &mockVppAdapter{}

	cli := connectWithAdapter(vpp)
	defer cli.Disconnect()

	ch := cli.NewApiChannel()
	defer ch.Close()

	// option 1 - channels
	req := &testRequest{SwIfIndex: 0, AdminUpDown: 1}
	ch.ReqChan <- &api.VppRequest{Message: req}

	vppReply := <-ch.ReplyChan
	reply := &testReply{}
	ch.Decoder.DecodeMsg(vppReply.Data, reply)

	// option 2 - API
	req2 := &testRequest{SwIfIndex: 0, AdminUpDown: 1}
	ch.SendRequest(req2)

	reply2 := &testReply{}
	ch.ReceiveReply(reply2)
}
