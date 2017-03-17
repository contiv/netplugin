// Generates Go bindings for all VPP APIs located in the json directory.
// go:generate binapi_generator --input-dir=json --output-dir=go

package main

import (
	"fmt"
	"net"
	"vpp-integration-with-etcd/vpp/govpp"
	"vpp-integration-with-etcd/vpp/govpp/api"
	"vpp-integration-with-etcd/vpp/govpp/examples/go/acl"
	"vpp-integration-with-etcd/vpp/govpp/examples/go/interfaces"
)

func main() {
	fmt.Println("Starting example VPP client...")

	conn := govpp.Connect()
	defer conn.Disconnect()

	ch := conn.NewApiChannel()
	defer ch.Close()

	aclVersion(ch)
	aclConfig(ch)
	aclDump(ch)

	interfaceDump(ch)
}

func aclVersion(ch *api.Channel) {
	// send the request - simple API
	req := &acl.ACLPluginGetVersion{}
	ch.SendRequest(req)

	// receive the response - simple API
	reply := &acl.ACLPluginGetVersionReply{}
	ch.ReceiveReply(reply)

	fmt.Printf("%+v\n", reply)
}

func aclConfig(ch *api.Channel) {
	req := &acl.ACLAddReplace{
		ACLIndex: ^uint32(0),
		Tag:      []byte("access list 1"),
		R: []acl.ACLRule{
			{
				IsPermit:       1,
				SrcIPAddr:      net.ParseIP("10.0.0.0").To4(),
				SrcIPPrefixLen: 8,
				DstIPAddr:      net.ParseIP("192.168.1.0").To4(),
				DstIPPrefixLen: 24,
				Proto:          6,
			},
			{
				IsPermit:       1,
				SrcIPAddr:      net.ParseIP("8.8.8.8").To4(),
				SrcIPPrefixLen: 32,
				DstIPAddr:      net.ParseIP("172.16.0.0").To4(),
				DstIPPrefixLen: 16,
				Proto:          6,
			},
		},
	}

	// send the request - channel API instead of SendRequest
	ch.ReqChan <- &api.VppRequest{Message: req}

	// receive the response - channel API instead of ReceiveReply
	vppReply := <-ch.ReplyChan
	reply := &acl.ACLAddReplaceReply{}
	ch.Decoder.DecodeMsg(vppReply.Data, reply)

	fmt.Printf("%+v\n", reply)
}

func aclDump(ch *api.Channel) {
	// send the request - simple API
	req := &acl.ACLDump{}
	ch.SendRequest(req)

	// receive the response - simple API
	reply := &acl.ACLDetails{}
	ch.ReceiveReply(reply)

	fmt.Printf("%+v\n", reply)
}

func interfaceDump(ch *api.Channel) {
	// send the request - simple API
	req := &interfaces.SwInterfaceDump{}
	ch.SendMultiRequest(req)

	// receive the response - simple API
	replies, _ := ch.ReceiveMultiReply(interfaces.SwInterfaceDetails{})

	for _, reply := range replies {
		fmt.Printf("%+v\n", reply)
	}
}
