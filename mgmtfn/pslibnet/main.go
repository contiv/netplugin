package main

// the powerstrip libnetwork daemon handles powerstrip events and invokes
// netmaster (netdcli) to program the network.

// libnetwork is still under development but it defines a driverapi and
// provides some utilities that can be used to start integrating NetPlugin.
//
// This implementation is capable of doing the following:
// - It registers for powerstrip's post-hook for container-start to trigger
//   CreateEndpoint() API. This is required right now to guarantee that container
//   namespace is created and can be used. In real world, this will be mimicked
//   by libnetwork before container is actually started but just after it's
//   network namespace is ready. We register for pre-create request and
//   response as well to find out the netid (passed as container label) and map
//   it to it's id
// - It registers for powerstrip's pre-hook for container-stop to trigger
//   DeleEndpoint() API. This is close to how it shall happen in real-world as
//   well.
// - Right now there is no trigger in powerstrip to for invoking the
//   CreateNetwork() and DeleteNetwork() APIs, so they are triggered explicitly
//   outside this daemon.
//
// Following is the interpretation/mapping of driverapi arguments to underlying
// netplugin API:
// Config(config interface{})
// - This API initializes driver's internal state.
//   + config - right now we don't interpret this arg.
// CreateNetwork(nid UUID, config interface{}) error
// - This API is not implemented and not invoked from this daemon.
//   + nid - libnetwork's derived uuid for a network.
// DeleteNetwork(nid UUID) error
// - This API is not implemented and not invoked from this daemon.
//   + nid - libnetwork's derived uuid for a network.
// CreateEndpoint(nid, eid UUID, key string, config interface{}) (*SandboxInfo, error)
// - This API shall be invoked on the post-start hook.
//   + nid - libnetwork's derived uuid for a network.
//   + eid - libnetwork's derived uuid for a endpoint
//   + key - this is ths path of the file in container's network namespace. For
//   now we don't need this as we are hooking ourselves in post start, so
//   CreateEndpoint() in Netplugin shall take care of AttachEndpoint(). In real
//   world, we 'may' want to use this to identify container's network namespace
//   and program it.
//   + config - this is expected to contain info like net-id, tenant-id,
//   container-id that are need to invoke netmaster.
//   + SandboxInfo - This is network namespace state, that driver shall
//   mainatin per container.
// DeleteEndpoint(nid, eid UUID) error
// - This API shall be invoked on the post-stop hook.
//   + nid - libnetwork's derived uuid for a network.
//   + eid - libnetwork's derived uuid for a endpoint

import (
	"flag"
	"net/http"
	"os"

	log "github.com/Sirupsen/logrus"
)

type cliOpts struct {
	hostLabel string
	etcdUrl   string
}

var gcliOpts cliOpts

func main() {
	var flagSet *flag.FlagSet

	gHostLabel, err := os.Hostname()
	if err != nil {
		log.Printf("Failed to fetch hostname. Error: %s", err)
		os.Exit(1)
	}

	flagSet = flag.NewFlagSet("pslibnet", flag.ExitOnError)
	flagSet.StringVar(&gcliOpts.hostLabel,
		"host-label",
		gHostLabel,
		"label used to identify endpoints homed for this host, default is host name")
	flagSet.StringVar(&gcliOpts.etcdUrl,
		"etcd-url",
		"http://127.0.0.1:4001",
		"Etcd cluster url")

	err = flagSet.Parse(os.Args[1:])
	if err != nil {
		log.Fatalf("Failed to parse command. Error: %s", err)
	}

	if flagSet.NFlag() < 1 {
		log.Printf("host-label not specified, using default (%s)", gcliOpts.hostLabel)
	}

	driver := &LibNetDriver{}
	err = driver.Config(nil)
	if err != nil {
		log.Printf("libnet driver init failed. Error: %s", err)
		os.Exit(1)
	}
	adapter := &PwrStrpAdptr{}
	err = adapter.Init(driver)
	if err != nil {
		log.Printf("powerstrip adaper init failed. Error: %s", err)
		os.Exit(1)
	}

	// start serving the API requests
	http.HandleFunc("/adapter/", adapter.CallHook)
	err = http.ListenAndServe(":80", nil)
	if err != nil {
		log.Printf("Error listening for http requests. Error: %s", err)
		os.Exit(1)
	}

	os.Exit(0)
}
