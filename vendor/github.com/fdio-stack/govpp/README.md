## GoVPP

This package provides the API for communication with VPP from Go. It consists of three parts:

- [binapi_generator](binapi_generator/generator.go): Genarator of Go structs out of the VPP binary API definitions in JSON format
- [api](api/api.go): API for communication with VPP via govpp core using channels (without the need of importing the govpp core package itself)
-  govpp core (the govpp package itself): provides the connectivity to VPP via shared memory, sends and recieves the messages to/from VPP and forwards them between clients and VPP
- [examples](examples/): example VPP client with generated binary API Go bindings

The design with separated API package (govpp/api) and the govpp core package (govpp) enables plugin-based infrastructure, where one entity acts as a master responsible for talking with VPP (e.g. Agent Core on the schema below) and multiple entities act as clients that are using the master for the communication with VPP (Agent Plugins on the schema below). The clients can be built into standalone shared libraries without the need of linking all the dependencies of the govpp core into them.

```
                                       +--------------+
    +--------------+                   |              |
    |              |                   | Agent Plugin |
    |              |                   |              |
    |  Agent Core  |                   +--------------+
    |              |            +------+  govpp API   |
    |              |            |      +--------------+
    +--------------+     Go     |
    |              |  channels  |      +--------------+
    |  govpp core  +------------+      |              |
    |              |            |      | Agent Plugin |
    +------+-------+            |      |              |
           |                    |      +--------------+
binary API |                    +------+  govpp API   |
  (shmem)  |                           +--------------+
           |
    +------+-------+
    |              |
    |      VPP     |
    |              |
    +--------------+
```

## Example usage
Generating Go bindings from the json files located in the `json` directory into the Go packages in `go` directory:
```
binapi_generator --input-dir=json --output-dir=go
```

Usage of the generated bindings:
```go
func main() {
	conn := govpp.Connect()
	defer conn.Disconnect()

	ch := conn.NewApiChannel()
	defer ch.Close()
  
  	// send the request
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
	ch.SendRequest(req)

	// receive the response
	reply := &acl.ACLAddReplaceReply{}
	ch.ReceiveReply(reply)
}
```

The example above uses simple wrapper API over underlying go channels, see [example_client](examples/example_client.go) for example on how to use the Go channels directly.

## Current State
- binapi_generator is almost final
- API definition and implementation of multipart requests/replies is in progress
- the API can still change
- source code of the core needs comments and fixing of all TODOs
- notification/stats API - TODO
- unit tests for code generator - TODO
- better unit test coverage for the core - TODO
