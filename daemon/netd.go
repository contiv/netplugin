package main

import (
	"core"
	"github.com/coreos/go-etcd/etcd"
	"log"
)

// a daemon based on etcd client's Watch interface to trigger plugin's
// network provisioning interfaces

var pluginConfig = PluginConfig{Network: "ovs", Endpoint: "ovs", State: "etcd"}
var plugin = Netplugin{configFile: "/tmp"} // XXX: should get the file as argument
var waitChan = make(chan error)

const (
	RECURSIVE = true
)

func receiver(plugin *core.NetPlugin, rsps chan *etcd.Response,
	stop chan bool, retErr chan error) {
	for {
		// block on change notifications
		rsp <- rsps

		//determine if the Node is interesting
		//XXX: how does etcd notifies deletes.
		isDelete := false
		node := rsp.Node
		if rsp.Node == nil {
			isDelete = true
			node = rsp.PrevNode
		}
		switch key := node.Key; {
		case strings.HasPrefix(key, core.NW_CFG_PATH_PREFIX):
			netId := strings.TtrimPrefix(key, core.NW_CFG_PATH_PREFIX)
			if isDelete {
				err := plugin.NetworkDelete(netId)
			} else {
				err := plugin.NetworkCreate(netId)
			}
			if err != nil {
				retErr <- true
				stop <- true
				return
			}
		case strings.HasPrefix(key, core.EP_CFG_PATH_PREFIX):
			epId := strings.TrimPrefix(key, core.EP_CFG_PATH_PREFIX)
			if isDelete {
				err := plugin.EndpointDelete(epId)
			} else {
				err := plugin.EndpointCreate(epId)
			}
			if err != nil {
				retErr <- true
				stop <- true
				return
			}
		}
	}

	// shall never come here
	retErr <- nil
}

func run(plugin *Netplugin) error {
	// watch the etcd changes and call the respective plugin APIs
	rsps := make(chan *etcd.Response)
	recvErr := make(chan error)
	stop := make(chan bool)
	client := plugin.StateDriver.Client

	go receiver(plugin, rsps, stop, recvErr)

	// XXX: todo, restore any config that might have been created till this
	// point
	_, err = client.Watch(core.CFG_PATH, 0, RECURSIVE, rsps, stop)
	if err != ErrWatchStoppedByUser {
		return err
	}

	err <- recvErr
	if err != nil {
		return err
	}

	return nil
}

func main() {
	err := plugin.Init(&pluginConfig)
	if err != nil {
		log.println("Failed to initialize the plugin. Error: ", err)
		exit(1)
	}

	go run(plugin)

	err = <-waitChan

	if err != nil {
		exit(1)
	} else {
		exit(0)
	}
}
