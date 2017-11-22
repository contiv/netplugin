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

// InitClient aims to support multi endpoints
func InitClient(storeName string, storeURLs []string) (API, error) {
	plugin := GetPlugin(storeName)
	if plugin == nil {
		log.Errorf("Invalid DB type %s", storeName)
		return nil, errors.New("unsupported DB type")
	}
	cl, err := plugin.NewClient(storeURLs)
	if err != nil {
		log.Errorf("Error creating client %s to url %v. Err: %v", storeName, storeURLs, err)
		return nil, err
	}
	return cl, nil
}

// NewClient Create a new conf store
func NewClient(dbURL string) (API, error) {
	// check if we should use default db
	if dbURL == "" {
		dbURL = defaultDbURL
	}

	parts := strings.Split(dbURL, "://")
	if len(parts) < 2 {
		log.Errorf("Invalid DB URL format %s", dbURL)
		return nil, errors.New("invalid DB URL")
	}
	clientName := parts[0]
	clientURL := parts[1]

	return InitClient(clientName, []string{"http://" + clientURL})
}
