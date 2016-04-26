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
package rpcHub

// Hub and spoke RPC implementation based on JSON RPC library

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
)

// Create a new RPC server
func NewRpcServer(portNo uint16) (*rpc.Server, net.Listener) {
	server := rpc.NewServer()

	// Listens on a port
	l, e := net.Listen("tcp", fmt.Sprintf(":%d", portNo))
	if e != nil {
		log.Fatal("listen error:", e)
	}

	log.Infof("RPC Server is listening on %s\n", l.Addr())

	// run in background
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				// if listener closed, just exit the groutine
				if strings.Contains(err.Error(), "use of closed network connection") {
					return
				}
				log.Fatal(err)
			}

			log.Infof("Server accepted connection to %s from %s\n", conn.LocalAddr(), conn.RemoteAddr())

			go server.ServeCodec(jsonrpc.NewServerCodec(conn))
		}
	}()

	return server, l
}

// Create a new client
func dialRpcClient(servAddr string, portNo uint16) (*rpc.Client, net.Conn) {
	var client *rpc.Client
	var conn net.Conn
	var err error
	log.Infof("Connecting to RPC server: %s:%d", servAddr, portNo)

	// Retry connecting for 5sec and then give up
	for i := 0; i < 5; i++ {
		// Connect to the server
		conn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", servAddr, portNo))
		if err == nil {
			log.Infof("Connected to RPC server: %s:%d", servAddr, portNo)

			// Create an RPC client
			client = jsonrpc.NewClient(conn)

			break
		}

		log.Warnf("Error %v connecting to %s:%s. Retrying..", err, servAddr, portNo)
		// Sleep for a second and retry again
		time.Sleep(1 * time.Second)
	}

	// If we failed to connect, report error
	if client == nil {
		log.Errorf("Failed to connect to Rpc server %s:%d", servAddr, portNo)
		return nil, nil
	}

	return client, conn
}

// Info for eahc client
type RpcClient struct {
	servAddr string
	portNo   uint16
	client   *rpc.Client
	conn     net.Conn
}

// DB of all existing clients
var clientDb map[string]*RpcClient = make(map[string]*RpcClient)

// Get a client to the rpc server
func Client(servAddr string, portNo uint16) *RpcClient {
	clientKey := fmt.Sprintf("%s:%d", servAddr, portNo)

	// Return the client if it already exists
	if (clientDb[clientKey] != nil) && (clientDb[clientKey].conn != nil) {
		return clientDb[clientKey]
	}

	// Create a new client and add it to the DB
	client, conn := dialRpcClient(servAddr, portNo)
	rpcClient := RpcClient{
		servAddr: servAddr,
		portNo:   portNo,
		client:   client,
		conn:     conn,
	}

	clientDb[clientKey] = &rpcClient
	return &rpcClient
}

// Make an rpc call
func (self *RpcClient) Call(serviceMethod string, args interface{}, reply interface{}) error {
	// Check if connectin failed
	if self.client == nil {
		log.Errorf("Error calling RPC: %s. Could not connect to server", serviceMethod)
		return errors.New("Could not connect to server")
	}

	// Perform RPC call.
	err := self.client.Call(serviceMethod, args, reply)
	if err == nil {
		return nil
	}

	// Check if we need to reconnect
	if err == rpc.ErrShutdown || err == io.ErrUnexpectedEOF {
		self.client, self.conn = dialRpcClient(self.servAddr, self.portNo)
		if self.client == nil {
			log.Errorf("Error calling RPC: %s. Could not connect to server", serviceMethod)
			return errors.New("Could not connect to server")
		}

		// Retry making the call
		return self.client.Call(serviceMethod, args, reply)
	}

	return err
}
