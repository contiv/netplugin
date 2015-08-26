[![Build Status](http://25b17de9.ngrok.com/view/Netplugin%20Sanity/job/Push%20Build%20Master/badge/icon)](http://25b17de9.ngrok.com/view/Netplugin%20Sanity/job/Push%20Build%20Master/)

## Netplugin

Generic network plugin (experimental) is designed to handle networking use cases in clustered multi-host systems. It is specifically designed to handle:

- Multi-tenant environment where disjoint networks are offered to containers on the same host
- SDN applications and interoperability with SDN solutions
- Interoperability with non container environment and hand-off to a physical network
- Instantiating policies/ACL/QoS associated with containers
- Multicast or multi-destination dependent applications
- Integration with existing IPAM tools for migrating customers
- Handle NIC's capabilities for acceleration (SRIOV/Offload/etc.)

The overall design is _not_ assumed to be complete, because of ongoing work in the docker community with regards to the suitable APIs to interface with network extensions like this. Regardless, flexibility in the design has been taken into consideration to allow using a different state driver for key-value synchronization, or a different flavor of a soft-switch i.e. linux-bridge, MAC VLAN, or OpenvSwitch.

The ability to specify the intent succinctly is the primary goal of the design and thus some of the specified user interface will change, and in some cases functionality will be enhanced to accommodate the same. Design details and future work is captured in a [docs/design.md](docs/Design.md).

Please do not use this code in production, until code goes through more testing and few critical open issues are resolved.

###Building and Testing

- Build:

  `make build`

   Note:
   - building the project requires at least 1.4 Go Version. Instructions to install Go can be found at: https://golang.org/doc/install

- Run unit-tests:

  `make unit-test`

- Run system-tests:

  `make system-test`

There is an additional document available here [docs/SETUP-BUILD.md](docs/SETUP-BUILD.md) that describes how to setup a build environment from scratch.

###Trying it out 

The netplugin produces two binaries, a netplugin daemon and a netdcli tool to interact with it. The binaries can tried out in a vagrant environment, which can be setup as follows.

`make demo`

Note:
- Make sure VirtualBox is installed

`vagrant ssh netplugin-node1`

####A quick example

1. Start netmaster and netplugin

    `sudo netmaster`
    `sudo netplugin -host-label=host1`

2. Launch a desired configuration for the two containers

    `netdcli -cfg examples/one_host_vlan.json`

3. According to the desired network state `myContainer1` and `myContainer2` now belongs to `orange` network

    ```json
    {
        "Tenants" : [{
            "Name"                      : "tenant-one",
            "DefaultNetType"            : "vlan",
            "SubnetPool"                : "11.1.0.0/16",
            "AllocSubnetLen"            : 24,
            "Vlans"                     : "11-28",
            "Networks"  : [{
                "Name"                  : "orange",
                "Endpoints" : [{
                    "Host"              : "host1",
                    "Container"         : "myContainer1"
                }, {
                    "Host"              : "host1",
                    "Container"         : "myContainer2"
                }]
            }]
        }]
    }
    ```

    If we examine the desired network state, it allows specifying the type of network as `vlan`, and subnet pools; those options are not mandatory but can be specified to override default values

4. Create the containers `myContainer1` and `myContainer2`

    `sudo docker run -it --name=myContainer1 --hostname=myContainer1 ubuntu /bin/bash`

    `sudo docker run -it --name=myContainer2 --hostname=myContainer2 ubuntu /bin/bash`

The creation of containers would automatically apply the network configuration as specified indicated in the network earlier. The same works for multi-host environment. And when containers are scheduled dynamically in a multi-host environment, host information need not be specified explicitly.

5. The configuration remains persistent, i.e. `myContainer1` and `myContainer2` can come and go

There are many variations to the above configuration, like creating multiple 
networks, across multiple hosts, use of VLANs, use of VXLAN, custom overrides
for IP/subnet/VLAN/VXLAN allocation on per network/endpoint basis. Please look
at [examples](examples/) directory to explore more sample configurations.

####Trying it out in a multi-host VLAN/VXLAN network

The [docs/TwoHostMultiVlanDemo.md](docs/TwoHostMultiVlanDemo.md) walks through setting up a multi host demo network and deploy the following Vlan based network:
![VlanNetwork](docs/VlanNetwork.jpg)

One can deploy the following Vxlan network by following the steps in the above demo and using [examples/two_hosts_multiple_vxlan_nets.json](examples/two_hosts_multiple_vxlan_nets.json) configuration file instead. Trying out the configuration is left as an exercise to the reader.
![VxlanNetwork](docs/VxlanNetwork.jpg)

####Multi-tenant network

In the examples directory [two_hosts_multiple_tenants.json](examples/two_hosts_multiple_tenants.json) and 
[two_hosts_multiple_tenants_mix_vlan_vxlan.json](examples/two_hosts_multiple_tenants_mix_vlan_vxlan.json) shows the creation of a multi-tenant
(disjoint, overlapping) networks within a cluster.

####Trying the multi-host tests on a single machine using docker as hosts
If you cannot launch VM on your host, especially if your host is itself a VM, one can test the multi-host network by simulating hosts using docker containers. Please see [docs/Dockerhost.md](docs/Dockerhost.md) for instructions. 

#### Resource Allocation
Various network resources like, IP-Subnets, VLAN/VXLAN-IDs, IP Addresses, can be automatically managed or they can be specified at network/endpoint granularity. To avoid any conflict with rest of the network, it is encouraged to specify the resource ranges, but when not specified, the resource-allocator can pick up the default values.

#### Kubernetes Integration
The plugin code contains the netplugin code that interfaces with kublet to allow network plumbing before a container is scheduled on one of the minions. Please see [Kubernetes Integration](docs/kubernetes.md) for details

### How to Contribute
Patches and contributions are welcome, please hit the GitHub page to open an issue or to submit patches send pull requests. Please sign your commits, and read [CONTRIBUTING.md](docs/CONTRIBUTING.md)

