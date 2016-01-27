package objdb

import log "github.com/Sirupsen/logrus"

var defaultConfStore = "etcd"

// NewClient Create a new conf store
func NewClient(clientName string) API {
	if clientName == "" {
		clientName = defaultConfStore
	}

	// Get the plugin
	plugin := GetPlugin(clientName)

	// Initialize the objdb client
	if err := plugin.Init([]string{}); err != nil {
		log.Errorf("Error initializing confstore plugin. Err: %v", err)
		log.Fatal("Error initializing confstore plugin")
	}

	return plugin
}
