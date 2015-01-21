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

package drivers

import (
	"fmt"
    "os"
    "strconv"
    "path"
    "log"
    "errors"
    "os/exec"
    "github.com/samalba/dockerclient"

	"github.com/contiv/netplugin/core"
)

// implements the StateDriver interface for an etcd based distributed
// key-value store used to store config and runtime state for the netplugin.

type DockerDriverConfig struct {
    Docker struct {
        Socket string
    }
}

type DockerDriver struct {
	Client *dockerclient.DockerClient
}

func (d *DockerDriver) Init(config *core.Config) error {
	if config == nil {
		return &core.Error{Desc: fmt.Sprintf("Invalid arguments. cfg: %v", config)}
	}

	cfg, ok := config.V.(*DockerDriverConfig)

	if !ok {
		return &core.Error{Desc: "Invalid config type passed!"}
	}

    // TODO: ADD TLS support
	d.Client, _ = dockerclient.NewDockerClient(cfg.Docker.Socket, nil)

	return nil
}

func (d *DockerDriver) Deinit() {
}

func (d *DockerDriver)getContPid(contId string) (string, error) {
    contInfo, err := d.Client.InspectContainer(contId)
    if err != nil {
        return "", errors.New("couldn't obtain container info")
    }

    // the hack below works only for running containers
    if ! contInfo.State.Running {
        return "", errors.New("container not running")
    }
    
    return strconv.Itoa(contInfo.State.Pid), nil
}

// Note: most of the work in this function is a temporary workaround for 
// what docker daemon would eventually do; the logic within is borrowed 
// from pipework utility
func (d *DockerDriver)moveIfToContainer(ifId string, contId string) error {

    contPid, err := d.getContPid(contId)
    if err != nil {
        return err
    }

    netnsDir := "/var/run/netns"

    err = os.Mkdir(netnsDir, 0700)
    if err != nil && ! os.IsExist(err) {
        log.Printf("error creating '%s' direcotry \n", netnsDir)
        return err
    }

    netnsPidFile := path.Join(netnsDir, contPid)
    err = os.Remove(netnsPidFile)
    if err != nil && ! os.IsNotExist(err) {
        log.Printf("error removing file '%s' \n", netnsPidFile)
        return err
    } else {
        err = nil
    }

    procNetNs := path.Join("/proc", contPid, "ns/net")
    err = os.Symlink(procNetNs, netnsPidFile)
    if err != nil {
        log.Printf("error symlink file '%s' with '%s' \n", netnsPidFile)
        return err
    } 

    out, err := exec.Command("/sbin/ip", "link", "set", ifId, 
        "netns", contPid).Output()
    if err != nil {
        log.Printf("error moving container's namespace 'out = %s', err = %s\n",
            out, err)
        return err
    }


    return err
}

func (d *DockerDriver)cleanupNetns(contId string) error {
    contPid, err := d.getContPid(contId)
    if err != nil {
        return err
    }

    netnsPidFile := path.Join("/var/run/netns", contPid)
    _, err = os.Stat(netnsPidFile)
    if err != nil && os.IsExist(err) {
        os.Remove(netnsPidFile)
    }

    return nil
}

// this function installs acl/qos and policy associated with the 
// container to be done upon attach
func (d *DockerDriver)AttachEndpoint(ctx *core.ContainerEpContext) error {
    
    err := d.moveIfToContainer(ctx.InterfaceId, ctx.NewContId)
    if err != nil {
        return err
    }

    // configure ip address

    // configure policies: acl/qos for the container on the host

    // cleanup intermediate things (overdoing it?)
    d.cleanupNetns(ctx.NewContId)

    return err
}

// this function un-installs the previously installed policy associated with
// the container to be done upon attach
func (d *DockerDriver)DetachEndpoint(ctx *core.ContainerEpContext) error {
    var err error

    err = nil

    log.Printf("Detached called for container %s with %s interface\n", 
                ctx.CurrContId, ctx.InterfaceId)

    return err
}
