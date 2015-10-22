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

package utils

import (
	"io/ioutil"
	"strings"

	log "github.com/Sirupsen/logrus"
)

// Implement utilities to fetch System specific information and capabilities.
// These capabilities could be used natively by netplugin or exported.
// In most cases the os capabilities do not change over the course of uptime
// of the system, however if it does, a registry mechanism should be used
// to notify the interested threads

// SystemAttributes enlist the system specific attributes and are read upon
// the system start
type SystemAttributes struct {
	OsType      string
	TotalRAM    int
	TotalDiskGB int
	TotalNetBw  int
}

// SysAttrs are the exported system attributes
var SysAttrs SystemAttributes

// FetchSysAttrs would read the system attributes and store them in the
// exported vars for the plugin to use; some of the attributes may need OS
// spefici methods to fetch, thus the first attribute to fetch is the OS type
func FetchSysAttrs() error {
	output, err := ioutil.ReadFile("/etc/os-release")
	if err != nil {
		log.Errorf("Error reading the /etc/os-release Error: %s Output: \n%s\n", err, output)
		return err
	}

	strOutput := string(output)
	if strings.Contains(strOutput, "CentOS") {
		SysAttrs.OsType = "centos"
	} else if strings.Contains(strOutput, "Ubuntu") {
		SysAttrs.OsType = "ubuntu"
	} else {
		SysAttrs.OsType = "unsupported"
	}

	// fetch the system memory, disk, and other attributes
	return err
}
