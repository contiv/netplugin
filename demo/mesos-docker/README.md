# Netplugin with Mesos Marathon

This document explains how to use Netplugin with Mesos Marathon. Currently, netplugin supports docker containerizer with Mesos Marathon.

## Getting started with Vagrant VMs
### Prerequisits
- Virtualbox 5.0.2 or higher
- Vagrant 1.7.4 or higher
- ansible 1.9.4 or higher

### Step 1: Bring up the vagrant VMs

```
$ git clone https://github.com/contiv/netplugin
$ cd netplugin
$ make mesos-docker-demo
```

This will bring up a two node Vagrant setup with Mesos, Marathon and docker.
Bringing up vagrant VMs and provisioning them can take few minutes to complete since it needs to download the VM images and mesos/marathon binaries. Please be patient.
This will also build netplugin binaries and start them on both VMs


### Step 2: Login to a VM and Create a network

```
$ cd demo/mesos-docker; vagrant ssh node1
<Inside vagrant VM>
$ netctl net create contiv -subnet 10.1.1.0/24
```

This will create a network called `contiv`. Containers can be launched in this network.

### Step 3: Launch containers

`docker.json` file in mgmtfn/mesos-docker directory has an example marathon app definition.

```

  "container": {
    "type": "DOCKER",
    "docker": {
      "image": "libmesos/ubuntu",
      "parameters": [ { "key": "net", "value": "contiv" } ]
    }
  },
  "id": "ubuntu",
  "instances": 2,
  "constraints": [ ["hostname", "UNIQUE", ""] ],
  "cpus": 1,
  "mem": 128,
  "uris": [],
  "cmd": "while sleep 10; do date -u +%T; done"
}
```

This example application definition launches two ubuntu containers with a constraint that both containers be spread on different hosts.
Note that there is a special `net` parameter used in this specification `"parameters": [ { "key": "net", "value": "contiv" } ]`. This tells docker to launch the application in contiv network that we created in step 3.

You can launch this application using following command

```
$ ./launch.sh docker.json
```

Launching the container can take few minutes depending on how long it takes to pull the image.
Once its launched, you should be able to see the containers using docker commands

```
$ docker ps
CONTAINER ID        IMAGE               COMMAND                  CREATED             STATUS              PORTS               NAMES
2a68fed77d5a        libmesos/ubuntu     "/bin/sh -c 'while sl"   About an hour ago   Up About an hour                       mesos-cce1c91f-65fb-457d-99af-5fdd4af14f16-S1.da634e3c-1fde-479a-b100-c61a498bcbe7
 ```

## Notes

 1. Mesos and Marathon ports are port-mapped from vagrant VM to host machine. You can access them by logging into localhost:5050 and localhost:8080 respectively.
 2. Netmaster web-ui is port-mapped to port 9090 on the host machine
