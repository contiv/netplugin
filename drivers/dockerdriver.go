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

func (d *DockerDriver)getContPid(contName string) (string, error) {
    contInfo, err := d.Client.InspectContainer(contName)
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
func (d *DockerDriver)moveIfToContainer(ifId string, contName string) error {

    // log.Printf("Moving interface '%s' into container '%s' \n", ifId, contName)

    contPid, err := d.getContPid(contName)
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
        log.Printf("error moving interface into container's namespace " + 
            "out = '%s', err = '%s'\n", out, err)
        return err
    }


    return err
}

func (d *DockerDriver)cleanupNetns (contName string) error {
    contPid, err := d.getContPid(contName)
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

// use netlink apis instead
func (d *DockerDriver) configureIfAddress(ctx *core.ContainerEpContext) error {

    log.Printf("configuring ip: addr -%s/%d- on interface %s for container %s\n", 
        ctx.IpAddress, ctx.SubnetLen, ctx.InterfaceId, ctx.NewContName)

    if ctx.IpAddress == "" {
        return nil
    }
    if ctx.SubnetLen == 0 {
        errors.New("Subnet mask unspecified \n")
    }

    contPid, err := d.getContPid(ctx.NewContName)
    if err != nil {
        return err
    }

    out, err := exec.Command("/sbin/ip", "netns", "exec", contPid, "ip", "addr",
        "add", ctx.IpAddress + "/" + strconv.Itoa(int(ctx.SubnetLen)), "dev", 
        ctx.InterfaceId).Output()
    if err != nil {
        log.Printf("error configuring ip address for interface %s " +
            "%s out = '%s', err = '%s'\n", ctx.InterfaceId, out, err)
        return err
    }

    out, err = exec.Command("/sbin/ip", "netns", "exec", contPid, "ip", "link",
        "set", ctx.InterfaceId, "up").Output()
    if err != nil {
        log.Printf("error bringing interface %s up 'out = %s', err = %s\n", 
            ctx.InterfaceId, out, err)
        return err
    }
    log.Printf("successfully configured ip and brought up the interface \n")

    return err
}

// performs funtion to configure the network access and policies
// before the container becomes active
func (d *DockerDriver)AttachEndpoint(ctx *core.ContainerEpContext) error {
    
    err := d.moveIfToContainer(ctx.InterfaceId, ctx.NewContName)
    if err != nil {
        return err
    }

    err = d.configureIfAddress(ctx)
    if err != nil {
        return err
    }

    // configure policies: acl/qos for the container on the host

    // cleanup intermediate things (overdoing it?)
    d.cleanupNetns(ctx.NewContName)

    return err
}

// uninstall the policies and configuration during container attach
func (d *DockerDriver)DetachEndpoint(ctx *core.ContainerEpContext) error {
    var err error

    // log.Printf("Detached called for container %s with %s interface\n", 
    //            ctx.CurrContName, ctx.InterfaceId)

    // no need to move the interface out of containre, etc.
    // usually deletion of ep takes care of that

    // TODO: unconfigure policies

    return err
}

func (d *DockerDriver)GetContainerId(contName string) string {
    contInfo, err := d.Client.InspectContainer(contName)
    if err != nil {
        log.Printf("could not get contId for container %s, err '%s' \n",
            contName, err)
        return ""
    }

    // the hack below works only for running containers
    if ! contInfo.State.Running {
        return ""
    }
    
    return contInfo.Id
}

