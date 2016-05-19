[![Build Status](http://contiv.ngrok.io/job/Netplugin%20Push%20Build%20Master/badge/icon)](http://contiv.ngrok.io/job/Netplugin%20Push%20Build%20Master/)

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

<<<<<<< HEAD
Getting-started videos are available on [YouTube](https://www.youtube.com/watch?v=KzansAxCBQE&list=PL2k86RlAekM_g6csRwSRQAWvln5SmgicN).
=======
Getting-started videos are available on (YouTube)[https://www.youtube.com/watch?v=KzansAxCBQE&list=PL2k86RlAekM_g6csRwSRQAWvln5SmgicN].
>>>>>>> 0bf8baa992e6bc5cb41b704102ed73372f8fd8be

Members of Cisco DevNet can also view the videos on the [Developer Network](https://developer.cisco.com/site/contiv/videos/index.gsp).

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
$ netctl net create contiv-net --subnet=20.1.1.0/24
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


### Building and Testing

**Note:** Vagrant 1.7.4 and VirtualBox 5.0+ are required to build and test netplugin.

High level `make` targets:

* `demo`: start two VM demo cluster for development or testing.
* `build`: build the binary in a VM and download it to the host.
* `unit-test`: run the unit tests. Specify `CONTIV_NODE_OS=centos` to test on centos instead of ubuntu.
* `system-test`: run the networking/"sanity" tests. Specify `CONTIV_NODE_OS=centos` to test on centos instead of ubuntu.


### How to Contribute
Patches and contributions are welcome, please hit the GitHub page to open an
issue or to submit patches send pull requests. Please sign your commits, and
read [CONTRIBUTING.md](CONTRIBUTING.md)
