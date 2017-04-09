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

// Manage versioning for netmaster/netplugin apis

package version

import "fmt"

var (
	gitCommit string
	version   string
	buildTime string
)

// Info enlists version and build information as used by netplugin binaries
type Info struct {
	GitCommit string
	Version   string
	BuildTime string
}

// Get gets the version information
func Get() *Info {
	ver := Info{}
	ver.GitCommit = gitCommit
	ver.Version = version
	ver.BuildTime = buildTime

	return &ver
}

// String returns printable version string
func String() string {
	ver := Get()
	return StringFromInfo(ver)
}

// StringFromInfo prints the versioning details
func StringFromInfo(ver *Info) string {
	return fmt.Sprintf("Version: %s\n", ver.Version) +
		fmt.Sprintf("GitCommit: %s\n", ver.GitCommit) +
		fmt.Sprintf("BuildTime: %s\n", ver.BuildTime)
}
