package main

import (
	"github.com/coreos/go-etcd/etcd"
	"log"
	"os"
	"strings"

	"github.com/contiv/netplugin/drivers"
	"github.com/contiv/netplugin/plugin"
)

// a daemon based on etcd client's Watch interface to trigger plugin's
// network provisioning interfaces

const (
	RECURSIVE = true
)

func receiver(netPlugin plugin.NetPlugin, rsps chan *etcd.Response,
	stop chan bool, retErr chan error) {
	for {
		// block on change notifications
		rsp := <-rsps

		// determine if the Node is interesting
		// XXX: how does etcd notifies deletes.
		isDelete := false
		operStr := ""
		node := rsp.Node
		if rsp.Node == nil {
			isDelete = true
			node = rsp.PrevNode
		}
		var err error = nil
		log.Printf("Received event for key: %s", node.Key)
		switch key := node.Key; {
		case strings.HasPrefix(key, drivers.NW_CFG_PATH_PREFIX):
			netId := strings.TrimPrefix(key, drivers.NW_CFG_PATH_PREFIX)
			if isDelete {
				err = netPlugin.DeleteNetwork(netId)
				operStr = "delete"
			} else {
				err = netPlugin.CreateNetwork(netId)
				operStr = "create"
			}
			if err != nil {
				log.Printf("Network operation %s failed. Error: %s",
					operStr, err)
			} else {
				log.Printf("Network operation %s succeeded", operStr)
			}
		case strings.HasPrefix(key, drivers.EP_CFG_PATH_PREFIX):
			epId := strings.TrimPrefix(key, drivers.EP_CFG_PATH_PREFIX)
			if isDelete {
				err = netPlugin.DeleteEndpoint(epId)
				operStr = "delete"
			} else {
				err = netPlugin.CreateEndpoint(epId)
				operStr = "create"
			}
			if err != nil {
				log.Printf("Endpoint operation %s failed. Error: %s",
					operStr, err)
			} else {
				log.Printf("Endpoint operation %s succeeded", operStr)
			}
		}
	}

	// shall never come here
	retErr <- nil
}

func run(netPlugin plugin.NetPlugin) error {
	// watch the etcd changes and call the respective plugin APIs
	rsps := make(chan *etcd.Response)
	recvErr := make(chan error, 1)
	stop := make(chan bool, 1)
	etcdDriver := netPlugin.StateDriver.(*drivers.EtcdStateDriver)
	client := etcdDriver.Client

	go receiver(netPlugin, rsps, stop, recvErr)

	// XXX: todo, restore any config that might have been created till this
	// point
	_, err := client.Watch(drivers.CFG_PATH, 0, RECURSIVE, rsps, stop)
	if err != nil && err != etcd.ErrWatchStoppedByUser {
		log.Printf("etcd watch failed. Error: %s", err)
		return err
	}

	err = <-recvErr
	if err != nil {
		log.Printf("Failure occured. Error: %s", err)
		return err
	}

	return nil
}

func main() {
	configStr := `{
                    "drivers" : {
                       "network": "ovs",
                       "endpoint": "ovs",
                       "state": "etcd"
                    },
                    "ovs" : {
                       "dbip": "127.0.0.1",
                       "dbport": 6640
                    },
                    "etcd" : {
                        "machines": ["http://127.0.0.1:4001"]
                    }
                  }`
	netPlugin := plugin.NetPlugin{}

	err := netPlugin.Init(configStr)
	if err != nil {
		log.Printf("Failed to initialize the plugin. Error: %s", err)
		os.Exit(1)
	}

	//logger := log.New(os.Stdout, "go-etcd: ", log.LstdFlags)
	//etcd.SetLogger(logger)

	err = run(netPlugin)
	if err != nil {
		os.Exit(1)
	} else {
		os.Exit(0)
	}
}
