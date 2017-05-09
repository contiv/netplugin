/***
Copyright 2017 Cisco Systems Inc. All rights reserved.

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

package systemtests

import (
	"crypto/tls"
	"net"
	"net/http"
	"sync"

	log "github.com/Sirupsen/logrus"
)

// this cert and key are valid for 100 years
const certpem = `
-----BEGIN CERTIFICATE-----
MIIDozCCAougAwIBAgIJAM+dSt5+iemKMA0GCSqGSIb3DQEBCwUAMGgxCzAJBgNV
BAYTAlVTMQswCQYDVQQIDAJDQTERMA8GA1UEBwwIU2FuIEpvc2UxDTALBgNVBAoM
BENQU0cxFjAUBgNVBAsMDUlUIERlcGFydG1lbnQxEjAQBgNVBAMMCWxvY2FsaG9z
dDAeFw0xNzA0MjAxOTI4MTJaFw0yNzA0MTgxOTI4MTJaMGgxCzAJBgNVBAYTAlVT
MQswCQYDVQQIDAJDQTERMA8GA1UEBwwIU2FuIEpvc2UxDTALBgNVBAoMBENQU0cx
FjAUBgNVBAsMDUlUIERlcGFydG1lbnQxEjAQBgNVBAMMCWxvY2FsaG9zdDCCASIw
DQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAM1MKpN5mtgdSo7gk0M70mcNJC4G
XVuPcZdC43GSfUxL1buc+80NP5kCp8dzbDYrKfshTgalwmEV4J+5bvKe4osrEMmC
aTC6927nCDH2m+G30/qWxHXMDp4QiZm8GIp/EiDLPqtOOImsoP/QUQKtRGKSqltX
Ei0D5o3wq06Y7RhXRoSnGBUkTCkp1OMGyuJJKXbpoeN+CO3xVJ6OgxMAqoKpdF9k
j8uP4qu8A1jzuiN3/L/vh/JmBajiD54vL0Pb4DoVHJRCGP1RRkLbRUuEHJkW9Smt
67SxcYmZwFnJyXN7KZF+QlyeDDFTB8t0s1t66WwyMIiyN4fr1HYxPXL/Tn0CAwEA
AaNQME4wHQYDVR0OBBYEFCGX5Uzlt8818KcOVicpoFPPEE/NMB8GA1UdIwQYMBaA
FCGX5Uzlt8818KcOVicpoFPPEE/NMAwGA1UdEwQFMAMBAf8wDQYJKoZIhvcNAQEL
BQADggEBAJO8JCp4Aoi+QM0PsrDeoBL/nAiW+lwHoAwH3JGi9vnZ59RTUs0iCCBM
ecr2MUT7vpxHxWF3C2EH9dPBiXYwrr3q4b0A8Lhf+PrmGOB9nwpbxqAyEvNoj02B
Uc2dpblNsIQteceBdOBGkIKBWAkvXPXrA0ExlV31Qh0KHNsaYLb0d6uSBHZFX/d6
zBhHQqoYuhS3WCYVaPE2PUU9eV3Q6f0Xx+e6GovaO7DgmrSQ1mbAp33XnPiKUz2b
ioF6fl0GISEpfkbrPNBbhSCrXatLrtz+4DpneJQ5vVClG054qcms+hnziiomz7P+
TfQIVXFBQdXZedjqDxhga7ebCWb41yA=
-----END CERTIFICATE-----
`

const certkey = `
-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAzUwqk3ma2B1KjuCTQzvSZw0kLgZdW49xl0LjcZJ9TEvVu5z7
zQ0/mQKnx3NsNisp+yFOBqXCYRXgn7lu8p7iiysQyYJpMLr3bucIMfab4bfT+pbE
dcwOnhCJmbwYin8SIMs+q044iayg/9BRAq1EYpKqW1cSLQPmjfCrTpjtGFdGhKcY
FSRMKSnU4wbK4kkpdumh434I7fFUno6DEwCqgql0X2SPy4/iq7wDWPO6I3f8v++H
8mYFqOIPni8vQ9vgOhUclEIY/VFGQttFS4QcmRb1Ka3rtLFxiZnAWcnJc3spkX5C
XJ4MMVMHy3SzW3rpbDIwiLI3h+vUdjE9cv9OfQIDAQABAoIBAQCa0Qtyd0vsGfq1
0Gl9VEmQ6PoVszsH5x6UIR7/8KaIuM+PUg0ZTxpcuwHniQVbvCVGepEqtinlqOfh
y6b9VBAnPuzD6ZKF6xjZC2TEuOJIz6YN3VB+PMnxLSt3Qb+IAdeb32l9Kdm9CO/I
ukG9MQjXBR9vDjRouf5Nn+avuOdjaGNaFWNCqZb3/0B4zdslsR8ynvKHgB9OH9a6
ggmKINzkvF1Fv6UyGjgLyfVjcdxgFDZ3TY5vsxoO7/jPWzxRY3LignaWV8hEo2L5
fFsyUFApHLmCXMW+fiEu/0QsN2zFcZp1oXCEc2+a9OF3p3e3FaXv5h9w3EdZJLql
b2zt2zzBAoGBAPC1zlZ8HkLcUxZgAA6O9d1lHCjDkSCG1HOLVQ2gKlDOhuQq8kxq
/0HsrtbC4KtjZeOTWHWnX035i7BU42G7y/cNwFtfplINr1XdH8DOgffS53UKnEbs
WyBSgBh6jsoDsPnuGrOnBVmaTB9aGLpznuHcZ/wMeZUEIrQI6wlL79nVAoGBANpW
g6d7BG62xs72e++T2tw/ZSpGNjLt+weKAWbhxHwhlvcrgEkQ5sS/FR/WIfoCplqh
MGhYV0a4zlmSOoOcg3ZQkqiwrNDT1CpIgC8ETzJzt5/eTwEE8UJtD9bIngA62Xec
iACYQgRox0v/UG9N9U1Tnr0oDLVXahZbN4BXiw4JAoGAGpWZskeG+A9pRcFYgEMd
uFPgZkgjERqTACfVPun/gmks0KpFlFcE1f0T2jgvo/4YVKgDTwsrJWt4GANoEXUy
M5jbM7w+nDVStgLz7NFh3UL3uR9w3wxfjBRQfWObvYfm1dOMM2cw2hKGcbf7nywB
0iQLf/TIwMJyKrwJaT9vv/kCgYEAvXoa4rtFS3de7LjHMVBUvJJfjuJDossX8KD5
Onlu9HKJ+pJL0BzUx6U0Bd7kuXyXNUtxIPyZMQysNttJ4HFxPLoLrE02jDtoghFM
/IB24ke98QUR9sZ9QLI47qJHS9fGZaD3/dwkXoM3gWJeQVmcKbEJrwoUjUMBE8mx
TrWqPVECgYBamOxzDC3dUQUFEUGNx0D8bGAHmMnU8tHglwjyQeC8wTYreAdFRYMp
KNPNa4q/NpcKXUcM9etcuzQC8Di/GZDAa+uEQIvwH/xHG6FwvOTfZp8bzN8rD+gQ
yGWqZkaNREZSyW+pNDCUXnBDkCBj7qwUgb6ysgodeF7RWFAHfoXJ1g==
-----END RSA PRIVATE KEY-----
`

// NewMockServer returns a configured, initialized, and running MockServer which
// can have routes added even though it's already running. Call Stop() to stop it.
func NewMockServer() *MockServer {
	ms := &MockServer{}
	ms.Init()
	go ms.Serve()

	return ms
}

// MockServer is a server which we can program to behave like netmaster for
// testing purposes.
type MockServer struct {
	listener net.Listener   // the actual HTTPS listener
	mux      *http.ServeMux // a custom ServeMux we can add routes onto later
	stopChan chan bool      // used to shut down the server
	wg       sync.WaitGroup // used to avoid a race condition when shutting down
}

// Init just sets up the stop channel and our custom ServeMux
func (ms *MockServer) Init() {
	ms.stopChan = make(chan bool, 1)
	ms.mux = http.NewServeMux()
}

// AddHardcodedResponse registers a HTTP handler func for `path' that returns `body'.
func (ms *MockServer) AddHardcodedResponse(path string, body []byte) {
	ms.mux.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	})
}

// AddHandler allows adding a custom route handler to our custom ServeMux
func (ms *MockServer) AddHandler(path string, f func(http.ResponseWriter, *http.Request)) {
	ms.mux.HandleFunc(path, f)
}

// Serve starts the mock server using the custom ServeMux we set up.
func (ms *MockServer) Serve() {
	var err error

	server := &http.Server{Handler: ms.mux}

	// because of the tight time constraints around starting/stopping the
	// mock server when running tests and the fact that lingering client
	// connections can cause the server not to shut down in a timely
	// manner, we will just disable keepalives entirely here.
	server.SetKeepAlivesEnabled(false)

	cert, err := tls.X509KeyPair([]byte(certpem), []byte(certkey))
	if err != nil {
		log.Fatalln("Failed to load TLS key pair:", err)
		return
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS11,
	}

	ms.listener, err = tls.Listen("tcp", "0.0.0.0:10000", tlsConfig)
	if err != nil {
		log.Fatalln("Failed to listen:", err)
		return
	}

	ms.wg.Add(1)
	go func() {
		server.Serve(ms.listener)
		ms.wg.Done()
	}()

	<-ms.stopChan

	ms.listener.Close()
}

// Stop stops the mock server.
func (ms *MockServer) Stop() {
	ms.stopChan <- true

	// wait until the listener has actually been stopped
	ms.wg.Wait()
}
