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

		//determine if the Node is interesting
		//XXX: how does etcd notifies deletes.
		isDelete := false
		node := rsp.Node
		if rsp.Node == nil {
			isDelete = true
			node = rsp.PrevNode
		}
		var err error = nil
		switch key := node.Key; {
		case strings.HasPrefix(key, drivers.NW_CFG_PATH_PREFIX):
			netId := strings.TrimPrefix(key, drivers.NW_CFG_PATH_PREFIX)
			if isDelete {
				err = netPlugin.DeleteNetwork(netId)
			} else {
				err = netPlugin.CreateNetwork(netId)
			}
			if err != nil {
				retErr <- err
				stop <- true
				return
			}
		case strings.HasPrefix(key, drivers.EP_CFG_PATH_PREFIX):
			epId := strings.TrimPrefix(key, drivers.EP_CFG_PATH_PREFIX)
			if isDelete {
				err = netPlugin.DeleteEndpoint(epId)
			} else {
				err = netPlugin.CreateEndpoint(epId)
			}
			if err != nil {
				retErr <- err
				stop <- true
				return
			}
		}
	}

	// shall never come here
	retErr <- nil
}

func run(netPlugin plugin.NetPlugin) error {
	// watch the etcd changes and call the respective plugin APIs
	rsps := make(chan *etcd.Response)
	recvErr := make(chan error)
	stop := make(chan bool)
	etcdDriver := netPlugin.StateDriver.(*drivers.EtcdStateDriver)
	client := etcdDriver.Client

	go receiver(netPlugin, rsps, stop, recvErr)

	// XXX: todo, restore any config that might have been created till this
	// point
	_, err := client.Watch(drivers.CFG_PATH, 0, RECURSIVE, rsps, stop)
	if err != etcd.ErrWatchStoppedByUser {
		return err
	}

	err = <-recvErr
	if err != nil {
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
                        "machines": ["127.0.0.1:4001"]
                    }
                  }`
	netPlugin := plugin.NetPlugin{}

	err := netPlugin.Init(configStr)
	if err != nil {
		log.Println("Failed to initialize the plugin. Error: ", err)
		os.Exit(1)
	}

	err = run(netPlugin)
	if err != nil {
		os.Exit(1)
	} else {
		os.Exit(0)
	}
}
