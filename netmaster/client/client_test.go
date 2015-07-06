package client

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/contiv/netplugin/netmaster/intent"
)

func TestPostFailureInvalidJson(t *testing.T) {
	var (
		cfg *intent.Config
		nmc *Client
	)

	nmc = New("localhost:12345")
	if err := nmc.doPost("test-endpoint", cfg); err == nil {
		t.Fatalf("post succeeded, expected to fail")
		if !strings.Contains(err.Error(), "json marshalling failed") {
			t.Fatalf("unexpected error %q", err)
		}
	}
}

func TestPostFailureInvalidUrl(t *testing.T) {
	var (
		cfg *intent.Config
		nmc *Client
	)

	cfg = &intent.Config{}
	nmc = New("localhost:12345")
	if err := nmc.doPost("test-endpoint", cfg); err == nil {
		t.Fatalf("post succeeded, expected to fail")
		if strings.Contains(err.Error(), "json marshalling failed") ||
			strings.Contains(err.Error(), "Response status") {
			t.Fatalf("unexpected error %q", err)
		}
	}
}

func TestPostFailureServerError(t *testing.T) {
	var (
		cfg       *intent.Config
		nmc       *Client
		srvr      *httptest.Server
		transport *http.Transport
		httpC     *http.Client
	)

	srvr = httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "test failure", http.StatusInternalServerError)
		}))
	defer srvr.Close()

	transport = &http.Transport{
		Proxy: func(r *http.Request) (*url.URL, error) {
			return url.Parse(srvr.URL)
		},
	}
	httpC = &http.Client{Transport: transport}

	cfg = &intent.Config{}
	nmc = &Client{url: srvr.URL, httpC: httpC}
	if err := nmc.doPost("test-endpoint", cfg); err == nil {
		t.Fatalf("post succeeded, expected to fail")
		if !strings.Contains(err.Error(), "Response status") {
			t.Fatalf("unexpected error %q", err)
		}
	}
}

func TestPostSuccess(t *testing.T) {
	var (
		cfg       *intent.Config
		nmc       *Client
		srvr      *httptest.Server
		transport *http.Transport
		httpC     *http.Client
	)

	srvr = httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
	defer srvr.Close()

	transport = &http.Transport{
		Proxy: func(r *http.Request) (*url.URL, error) {
			return url.Parse(srvr.URL)
		},
	}
	httpC = &http.Client{Transport: transport}

	cfg = &intent.Config{}
	nmc = &Client{url: srvr.URL, httpC: httpC}
	if err := nmc.doPost("test-endpoint", cfg); err != nil {
		t.Fatalf("post failed. Error: %s", err)
	}
}

func TestGetFailureInvalidUrl(t *testing.T) {
	var nmc *Client

	nmc = New("localhost:12345")
	if _, err := nmc.doGet("test-endpoint"); err == nil {
		t.Fatalf("get succeeded, expected to fail")
		if strings.Contains(err.Error(), "Response status") {
			t.Fatalf("unexpected error %q", err)
		}
	}
}

func TestGetFailureServerError(t *testing.T) {
	var (
		nmc       *Client
		srvr      *httptest.Server
		transport *http.Transport
		httpC     *http.Client
	)

	srvr = httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "test failure", http.StatusInternalServerError)
		}))
	defer srvr.Close()

	transport = &http.Transport{
		Proxy: func(r *http.Request) (*url.URL, error) {
			return url.Parse(srvr.URL)
		},
	}
	httpC = &http.Client{Transport: transport}

	nmc = &Client{url: srvr.URL, httpC: httpC}
	if _, err := nmc.doGet("test-endpoint"); err == nil {
		t.Fatalf("get succeeded, expected to fail")
		if !strings.Contains(err.Error(), "Response status") {
			t.Fatalf("unexpected error %q", err)
		}
	}
}

func TestGetSuccess(t *testing.T) {
	var (
		nmc               *Client
		srvr              *httptest.Server
		transport         *http.Transport
		httpC             *http.Client
		getSuccessRespStr = "success string"
	)

	srvr = httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(getSuccessRespStr))
		}))
	defer srvr.Close()

	transport = &http.Transport{
		Proxy: func(r *http.Request) (*url.URL, error) {
			return url.Parse(srvr.URL)
		},
	}
	httpC = &http.Client{Transport: transport}

	nmc = &Client{url: srvr.URL, httpC: httpC}
	if resp, err := nmc.doGet("test-endpoint"); err != nil {
		t.Fatalf("get failed. Error: %s", err)
	} else if resp != getSuccessRespStr {
		t.Fatalf("unexpected response. Exptd: %s, Rcvd: %s", getSuccessRespStr, resp)
	}
}
