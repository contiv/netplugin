/***
Copyright 2016 Cisco Systems Inc. All rights reserved.

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

package mesosplugin

import (
	"fmt"
	"net"
	"net/http"
	"path"
	"time"
)

var driverPath string
var client *http.Client

func init() {
	driverPath = path.Join(pluginPath, driverName) + ".sock"

	tr := &http.Transport{
		Dial: fakeDial,
	}
	client = &http.Client{Transport: tr}
}

func fakeDial(proto, addr string) (conn net.Conn, err error) {
	fmt.Printf("dialing with proto %v and address %v", proto, addr)
	return net.DialTimeout("unix", driverPath, time.Second*5)
}
