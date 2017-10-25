[![Build Status](https://contiv-ci.ngrok.io/job/Netplugin%20Push%20Build%20Master/badge/icon)](https://contiv-ci.ngrok.io/job/Netplugin%20Push%20Build%20Master/) 
[![Go Report Card](https://goreportcard.com/badge/github.com/contiv/netplugin)](https://goreportcard.com/report/github.com/contiv/netplugin)

## Netplugin

Generic network plugin is designed to handle networking use
cases in clustered multi-host systems. It is specifically designed to handle:

- Multi-tenant environment where disjoint networks are offered to containers on the same host
- SDN applications and interoperability with SDN solutions
- Interoperability with non container environment and hand-off to a physical network
- Instantiating policies/ACL/QoS associated with containers
- Multicast or multi-destination dependent applications
- Integration with existing IPAM tools for migrating customers
- Handle NIC's capabilities for acceleration (SRIOV/Offload/etc.)

### Documentation
Full, comprehensive documentation is available on the website:

http://docs.contiv.io

Getting-started videos are available on [YouTube](https://www.youtube.com/watch?v=KzansAxCBQE&list=PL2k86RlAekM_g6csRwSRQAWvln5SmgicN).

### Getting Started

This will provide you with a minimal experience of uploading the intent and
seeing the netplugin system act on it. It will create a network on your host
that lives behind an OVS bridge and has its own unique interfaces.

#### Step 1: Clone the project and bringup the VMs

Note: if you have $GOPATH set, then please ensure either you unset GOPATH,
or clone the tree in `$GOPATH/src/github.com/contiv/` location

```
$ git clone https://github.com/contiv/netplugin
$ cd netplugin; make demo
$ vagrant ssh netplugin-node1
```

Optionally, variables can be passed to Makefile if needed. For example, to
use 4 GB memory and 2 CPUs for the vagrant VMs, run:

```
CONTIV_MEMORY=4096 CONTIV_CPUS=2 make demo
```

CONTIV_MEMORY and CONTIV_CPUS are set to 2048 and 4 as the default values
respectively.

#### Step 2: Create a network

```
$ netctl net create contiv-net --subnet=20.1.1.0/24
	or
netctl net create contiv-net --subnet=20.1.1.0/24 --subnetv6=2001::/100 
```

#### Step 3: Run your containers and enjoy the networking!

```
$ docker run -itd --name=web --net=contiv-net alpine /bin/sh
$ docker run -itd --name=db --net=contiv-net alpine /bin/sh
$ docker exec -it web /bin/sh
< inside the container >
root@f90e7fd409c4:/# ping db
PING db (20.1.1.3) 56(84) bytes of data.
64 bytes from db (20.1.1.3): icmp_seq=1 ttl=64 time=0.658 ms
64 bytes from db (20.1.1.3): icmp_seq=2 ttl=64 time=0.103 ms
```


### Building and Testing

**Note:** Vagrant 1.7.4 and VirtualBox 5.0+ are required to build and test netplugin.

High level `make` targets:

* `demo`: start three VM demo cluster for development or testing.
* `build`: build the binary in a VM and download it to the host.
* `unit-test`: run the unit tests. Specify `CONTIV_NODE_OS=centos` to test on centos instead of ubuntu.
* `system-test`: run the networking/"sanity" tests. Specify `CONTIV_NODE_OS=centos` to test on centos instead of ubuntu.


### How to Contribute
Patches and contributions are welcome, please hit the GitHub page to open an
issue or to submit patches send pull requests. Please sign your commits, and
read [CONTRIBUTING.md](.github/CONTRIBUTING.md)
