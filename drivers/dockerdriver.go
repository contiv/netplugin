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
	"errors"
	"fmt"
	"github.com/samalba/dockerclient"
	"log"
	"os"
	"path"
	"strconv"

	"github.com/contiv/netplugin/core"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
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

func (d *DockerDriver) getContPid(contName string) (string, error) {
	contInfo, err := d.Client.InspectContainer(contName)
	if err != nil {
		return "", errors.New("couldn't obtain container info")
	}

	// the hack below works only for running containers
	if !contInfo.State.Running {
		return "", errors.New("container not running")
	}

	return strconv.Itoa(contInfo.State.Pid), nil
}

func setIfNs(ifname string, pid int) error {
	link, err := netlink.LinkByName(ifname)
	if err != nil {
		log.Printf("unable to find link '%s' error \n", ifname, err)
		return err
	}

	err = netlink.LinkSetNsPid(link, pid)
	if err != nil {
		log.Printf("unable to move interface '%s' to pid %d \n",
			ifname, pid)
	}

	return err
}

// Note: most of the work in this function is a temporary workaround for
// what docker daemon would eventually do; in the meanwhile
// the essense of the logic is borrowed from pipework
func (d *DockerDriver) moveIfToContainer(ifId string, contName string) error {

	// log.Printf("Moving interface '%s' into container '%s' \n", ifId, contName)

	contPid, err := d.getContPid(contName)
	if err != nil {
		return err
	}

	netnsDir := "/var/run/netns"

	err = os.Mkdir(netnsDir, 0700)
	if err != nil && !os.IsExist(err) {
		log.Printf("error creating '%s' direcotry \n", netnsDir)
		return err
	}

	netnsPidFile := path.Join(netnsDir, contPid)
	err = os.Remove(netnsPidFile)
	if err != nil && !os.IsNotExist(err) {
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

	intPid, _ := strconv.Atoi(contPid)
	err = setIfNs(ifId, intPid)
	if err != nil {
		log.Printf("err '%s' moving if '%s' into container '%s' namespace\n",
			err, ifId, contName)
		return err
	}

	return err
}

func (d *DockerDriver) cleanupNetns(contName string) error {
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

func (d *DockerDriver) configureIfAddress(ctx *core.ContainerEpContext) error {

	log.Printf("configuring ip: addr -%s/%d- on if %s for container %s\n",
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

	intPid, err := strconv.Atoi(contPid)
	if err != nil {
		return err
	}

	contNs, err := netns.GetFromPid(intPid)
	if err != nil {
		log.Printf("error '%s' getting namespace for pid %s \n",
			err, contPid)
		return err
	}
	defer contNs.Close()

	origNs, err := netns.Get()
	if err != nil {
		log.Printf("error '%s' getting orig namespace\n", err)
		return err
	}

	defer origNs.Close()

	err = netns.Set(contNs)
	if err != nil {
		log.Printf("error '%s' setting netns \n", err)
		return err
	}
	defer netns.Set(origNs)

	link, err := netlink.LinkByName(ctx.InterfaceId)
	if err != nil {
		log.Printf("error '%s' getting if '%s' information \n", err,
			ctx.InterfaceId)
		return err
	}

	addr, err := netlink.ParseAddr(ctx.IpAddress + "/" +
		strconv.Itoa((int)(ctx.SubnetLen)))
	if err != nil {
		log.Printf("error '%s' parsing ip %s/%d \n", err,
			ctx.IpAddress, ctx.SubnetLen)
		return err
	}

	err = netlink.AddrAdd(link, addr)
	if err != nil {
		log.Printf("## netlink add addr failed '%s' \n", err)
		return err
	}

	err = netlink.LinkSetUp(link)
	if err != nil {
		log.Printf("## netlink set link up failed '%s' \n", err)
		return err
	}

	return err
}

// performs funtion to configure the network access and policies
// before the container becomes active
func (d *DockerDriver) AttachEndpoint(ctx *core.ContainerEpContext) error {

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
func (d *DockerDriver) DetachEndpoint(ctx *core.ContainerEpContext) error {
	var err error

	// log.Printf("Detached called for container %s with %s interface\n",
	//            ctx.CurrContName, ctx.InterfaceId)

	// no need to move the interface out of containre, etc.
	// usually deletion of ep takes care of that

	// TODO: unconfigure policies

	return err
}

func (d *DockerDriver) GetContainerId(contName string) string {
	contInfo, err := d.Client.InspectContainer(contName)
	if err != nil {
		log.Printf("could not get contId for container %s, err '%s' \n",
			contName, err)
		return ""
	}

	// the hack below works only for running containers
	if !contInfo.State.Running {
		return ""
	}

	return contInfo.Id
}

func (d *DockerDriver) GetContainerName(contId string) (string, error) {
	contInfo, err := d.Client.InspectContainer(contId)
	if err != nil {
		log.Printf("could not get contName for container %s, err '%s' \n",
			contId, err)
		return "", err
	}

	// the hack below works only for running containers
	if !contInfo.State.Running {
		return "", errors.New(fmt.Sprintf(
			"container id %s not running", contId))
	}

	return contInfo.Name, nil
}
