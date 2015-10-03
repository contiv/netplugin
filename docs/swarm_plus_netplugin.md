## Using Netplugin with Docker Swarm

This document describes how to use netplugin with docker swarm.
Docker Swarm is a scheduler that schedules containers to multiple machines. Netplugin is a docker network plugin that provides multi host networking.

Docker + Swarm + Netplugin == Awesome!!

## Getting started

Checkout netplugin tree and bringup vagrant setup
```
mkdir -p src/github.com/contiv/
cd src/github/contiv
git clone -b demo https://github.com/contiv/netplugin.git
CONTIV_NODES=2 make build demo
```
This brings up two VM cluster with docker, swarm and netplugin/netmaster running.

Set the following environment variable to make docker client talk to Swarm
```
export DOCKER_HOST=tcp://192.168.2.10:2375
```
Now, you should be able to see the information about the swarm cluster
```
$ docker info
Containers: 0
Images: 5
Engine Version: 
Role: primary
Strategy: spread
Filters: affinity, health, constraint, port, dependency
Nodes: 2
 netplugin-node1: 192.168.2.10:2385
  └ Containers: 0
  └ Reserved CPUs: 0 / 4
  └ Reserved Memory: 0 B / 2.051 GiB
  └ Labels: executiondriver=native-0.2, kernelversion=4.0.0-040000-generic, operatingsystem=Ubuntu 15.04, storagedriver=devicemapper
 netplugin-node2: 192.168.2.11:2385
  └ Containers: 0
  └ Reserved CPUs: 0 / 4
  └ Reserved Memory: 0 B / 2.051 GiB
  └ Labels: executiondriver=native-0.2, kernelversion=4.0.0-040000-generic, operatingsystem=Ubuntu 15.04, storagedriver=devicemapper
CPUs: 8
Total Memory: 4.103 GiB
Name: netplugin-node1
No Proxy: 192.168.0.0/16,localhost,127.0.0.0/8
```

Next, you can see if there are any containers running in the cluster
```
$ docker ps
CONTAINER ID        IMAGE               COMMAND             CREATED             STATUS              PORTS               NAMES
```

Netmaster creates two networks by default
```
$ contivctl network list
Listing all networks for tenant default
Network		Public	Encap	Subnet			Gateway
private		No	vxlan	10.1.0.0/16		10.1.254.254
public		Yes	vlan	192.168.1.0/24		192.168.1.254
```

You can run containers and attach them to one of these networks as below.
```
$ docker run -itd --publish-service foo.private ubuntu bash
f291e269b45a5877f6fc952317feb329e12a99bda3a44a740b4c3307ef87954c
```

You can verify its running and has the correct service name

```
$ docker ps
CONTAINER ID        IMAGE               COMMAND             CREATED             STATUS              PORTS               NAMES
f291e269b45a        ubuntu              "bash"              27 seconds ago      Up 24 seconds                           netplugin-node2/elegant_shaw

$ docker inspect f291e269b45a | grep -i "net\|ip\|mac\|service"
    "NetworkSettings": {
        "GlobalIPv6Address": "",
        "GlobalIPv6PrefixLen": 0,
        "IPAddress": "10.1.0.1",     <<<<<< IP address allocated by netplugin
        "IPPrefixLen": 16,
        "IPv6Gateway": "",
        "LinkLocalIPv6Address": "",
        "LinkLocalIPv6PrefixLen": 0,
        "MacAddress": "",
        "NetworkID": "56d09d456d83766d3709408f6f4cc95dfc4ea87731936cbad60f215e57d04a2e",
        "SandboxKey": "/var/run/docker/netns/3dce9fcdb134",
        "SecondaryIPAddresses": null,
        "SecondaryIPv6Addresses": null
        "IP": "192.168.2.11",
        "Name": "netplugin-node2",
        "NetworkMode": "default",
        "IpcMode": "",
        "PublishService": "foo.private", <<<<<< Service name for the container
        "NetworkDisabled": false,
        "MacAddress": "",
```

You can verify netplugin has automatically created the endpoint groups using following command
```
$ contivctl group list
Listing all endpoint groups for tenant default
Group		Network		Policies
---------------------------------------------------
foo.private		private		--
bar.private		private		--
```

Or you can check netplugin oper state to verify endpoints have been created
```
$ curl -s localhost:9999/endpoints | python -mjson.tool
[
    {
        "attachUUID": "",
        "contName": "00b913565f884adf9d75f0062e86df2f69fd806ac9ce0b2e32aca6331dbe13e8",
        "contUUID": "",
        "homingHost": "netplugin-node1",
        "id": "private-00b913565f884adf9d75f0062e86df2f69fd806ac9ce0b2e32aca6331dbe13e8",
        "intfName": "",
        "ipAddress": "10.1.0.2",
        "macAddress": "02:02:0a:01:00:02",
        "netID": "private",
        "portName": "port1",
        "vtepIP": ""
    },
    {
        "attachUUID": "",
        "contName": "541d02264d1b1fd5989b188e1073e077c3a3ef77ec0cc5fa35dfd78f9808ef31",
        "contUUID": "",
        "homingHost": "netplugin-node2",
        "id": "private-541d02264d1b1fd5989b188e1073e077c3a3ef77ec0cc5fa35dfd78f9808ef31",
        "intfName": "",
        "ipAddress": "10.1.0.1",
        "macAddress": "02:02:0a:01:00:01",
        "netID": "private",
        "portName": "port1",
        "vtepIP": ""
    }
]
```
