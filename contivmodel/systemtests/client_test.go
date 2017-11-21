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
	"net/http"
	"testing"
	"time"

	. "github.com/contiv/check"
	"github.com/contiv/netplugin/contivmodel/client"
)

func Test(t *testing.T) { TestingT(t) }

type clientSuite struct{}

var _ = Suite(&clientSuite{})

// ----- HELPER FUNCTIONS -------------------------------------------------------

func newClient(c *C, url string) *client.ContivClient {
	cc, err := client.NewContivClient(url)
	c.Assert(err, IsNil)

	return cc
}

func newHTTPClient(c *C) *client.ContivClient {
	return newClient(c, "http://localhost:9999")
}

func newHTTPSClient(c *C) *client.ContivClient {
	return newClient(c, "https://localhost:10000")
}

func newInsecureHTTPSClient(c *C) *client.ContivClient {
	cc := newHTTPSClient(c)

	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	err := cc.SetHTTPClient(&http.Client{Transport: tr})
	c.Assert(err, IsNil)

	return cc
}

func runHTTPSServer(f func(*MockServer)) {
	ms := NewMockServer()
	defer ms.Stop()

	// give the listener a bit to come up.
	// tls.Listen() returns before the server is actually responding to requests.
	time.Sleep(100 * time.Millisecond)

	f(ms)
}

// ----- TESTS ------------------------------------------------------------------

func (cs *clientSuite) TestURLValidity(c *C) {
	valid := []string{
		"http://localhost",
		"http://localhost:12345",
		"https://localhost",
		"https://localhost:12345",
	}

	invalid := []string{
		"asdf",
		"localhost",
		"localhost:12345",
	}

	for _, url := range valid {
		_, err := client.NewContivClient(url)
		c.Assert(err, IsNil)
	}

	for _, url := range invalid {
		_, err := client.NewContivClient(url)
		c.Assert(err, NotNil)
	}
}

func (cs *clientSuite) TestSettingCustomClient(c *C) {
	cc := newHTTPClient(c)

	// valid client
	err := cc.SetHTTPClient(&http.Client{})
	c.Assert(err, IsNil)

	// invalid client
	err = cc.SetHTTPClient(nil)
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Matches, ".* cannot be nil")
}

func (cs *clientSuite) TestSettingAuthToken(c *C) {

	// make sure we can't set a token on a non-https client
	insecureClient := newHTTPClient(c)
	err := insecureClient.SetAuthToken("foo")
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Matches, ".* requires a https .*")

	// make sure we can't set more than one token
	secureClient := newHTTPSClient(c)
	err = secureClient.SetAuthToken("foo")
	c.Assert(err, IsNil)

	err = secureClient.SetAuthToken("bar")
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Matches, ".* already been set")

	// make sure that the auth token is included when set and not when not set
	tokenValue := "asdf"
	tokenFromRequest := ""

	runHTTPSServer(func(ms *MockServer) {

		// we'll use the "list networks" endpoint for this test
		ms.AddHandler("/api/v1/networks/", func(w http.ResponseWriter, req *http.Request) {
			tokenFromRequest = req.Header.Get("X-Auth-Token")
			w.Write([]byte("[]")) // return an empty set
		})

		cc := newInsecureHTTPSClient(c)

		// send a request without the auth token and make sure the header wasn't set
		_, err := cc.NetworkList()
		c.Assert(err, IsNil)
		c.Assert(len(tokenFromRequest), Equals, 0)

		// set the auth token, resend the same request, and verify the header was set
		cc.SetAuthToken(tokenValue)

		_, err = cc.NetworkList()
		c.Assert(err, IsNil)
		c.Assert(tokenFromRequest == tokenValue, Equals, true)

	})
}

func (cs *clientSuite) TestLogin(c *C) {

	// make sure we can't login with a non-http client
	insecureClient := newHTTPClient(c)

	_, _, err := insecureClient.Login("foo", "bar")
	c.Assert(err, NotNil)
	c.Assert(err.Error(), Matches, ".* requires a https .*")

	// make sure a https client can send a login request
	runHTTPSServer(func(ms *MockServer) {

		// add the endpoint that Login() will POST to
		ms.AddHardcodedResponse(client.LoginPath, []byte("{}"))

		cc := newInsecureHTTPSClient(c)

		// send a login request
		resp, _, err := cc.Login("foo", "bar")
		c.Assert(err, IsNil)
		c.Assert(resp.StatusCode, Equals, 200)
	})
}
