package plugins

import (
	"github.com/contiv/objdb/plugins/etcdClient"
)

func Init() {
	// Initialize all conf store plugins
	etcdClient.InitPlugin()
}
