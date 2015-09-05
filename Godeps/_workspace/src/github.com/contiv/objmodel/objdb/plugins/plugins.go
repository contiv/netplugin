package plugins

import (
	"github.com/contiv/objmodel/objdb/plugins/etcdClient"
)

func Init() {
	// Initialize all conf store plugins
	etcdClient.InitPlugin()
}
