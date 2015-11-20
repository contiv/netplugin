[![Build Status](http://51633006.ngrok.com/view/Netplugin%20Sanity/job/Netplugin_Release/badge/icon)](http://51633006.ngrok.com/view/Netplugin%20Sanity/job/Netplugin_Release/)

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

### Getting Started

This will provide you with a minimal experience of uploading the intent and
seeing the netplugin system act on it. It will create a network on your host
that lives behind an OVS bridge and has its own unique interfaces.

#### Step 1: Clone the project and bringup the VMs

```
$ git clone https://github.com/contiv/netplugin
$ cd netplugin; make demo
$ vagrant ssh netplugin-node1
```

#### Step 2: Create a network

```
$ netctl net create contiv-net --subnet=20.1.1.0/24 --gateway=20.1.1.254 --pkt-tag=200
```

#### Step 3: Run your containers and enjoy the networking!

```
$ docker run -itd --name=web --net=contiv-net ubuntu /bin/bash
$ docker run -itd --name=db --net=contiv-net ubuntu /bin/bash
$ docker exec -it web /bin/bash
< inside the container >
root@f90e7fd409c4:/# ping db
PING db (20.1.1.3) 56(84) bytes of data.
64 bytes from db (20.1.1.3): icmp_seq=1 ttl=64 time=0.658 ms
64 bytes from db (20.1.1.3): icmp_seq=2 ttl=64 time=0.103 ms
```

#### Trying it out in a multi-host VLAN/VXLAN network

The [docs/TwoHostMultiVlanDemo.md](docs/TwoHostMultiVlanDemo.md) walks through
setting up a multi host demo network and deploy the following Vlan based
network.

#### Multi-tenant network

In the examples directory [two_hosts_multiple_tenants.json](examples/two_hosts_multiple_tenants.json) and
[two_hosts_multiple_tenants_mix_vlan_vxlan.json](examples/two_hosts_multiple_tenants_mix_vlan_vxlan.json)
shows the creation of a multi-tenant (disjoint, overlapping) networks within a
cluster.

### Building and Testing

**Note:** Vagrant 1.7.4 and VirtualBox 5.0+ are required to build and test netplugin.

High level `make` targets:

* `demo`: start a VM (or multiple, set `CONTIV_NODES` to greater than 1) for
  development or testing.
* `build`: build the binary in a VM and download it to the host.
* `unit-test`: run the unit tests. Specify `CONTIV_NODE_OS=centos` to test on
  centos instead of ubuntu.
* `system-test`: run the networking/"sanity" tests. Specify
  `CONTIV_NODE_OS=centos` to test on centos instead of ubuntu.

#### Kubernetes Integration
The plugin code contains the netplugin code that interfaces with kublet to
allow network plumbing before a container is scheduled on one of the minions.
Please see [Kubernetes Integration](docs/kubernetes.md) for details

### How to Contribute
Patches and contributions are welcome, please hit the GitHub page to open an
issue or to submit patches send pull requests. Please sign your commits, and
read [CONTRIBUTING.md](CONTRIBUTING.md)
