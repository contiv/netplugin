/***
Copyright 2014 Cisco Systems Inc. All rights reserved.

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

package objdb

import (
	"errors"
	"strings"

	log "github.com/Sirupsen/logrus"
)

var defaultDbURL = "etcd://127.0.0.1:2379"

// NewClient Create a new conf store
func NewClient(dbURL string) (API, error) {
	return NewClientWithConfig(dbURL, nil)
}

// NewClientWithConfig Create a new conf store with options
func NewClientWithConfig(dbURL string, config *Config) (API, error) {
	// check if we should use default db
	if dbURL == "" {
		dbURL = defaultDbURL
	}

	parts := strings.Split(dbURL, "://")
	if len(parts) < 2 {
		log.Errorf("Invalid DB URL format %s", dbURL)
		return nil, errors.New("Invalid DB URL")
	}
	clientName := parts[0]
	clientURL := parts[1]

	// Get the plugin
	plugin := GetPlugin(clientName)
	if plugin == nil {
		log.Errorf("Invalid DB type %s", clientName)
		return nil, errors.New("Unsupported DB type")
	}

	// Initialize the objdb client
	cl, err := plugin.NewClient([]string{"http://" + clientURL}, config)
	if err != nil {
		log.Errorf("Error creating client %s to url %s. Err: %v", clientName, clientURL, err)
		return nil, err
	}

	return cl, nil
}
