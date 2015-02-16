## Netplugin

Generic network plugin (experimental) is designed to handle networking use cases in clustered multi-host systems, beyond what is offered by the docker default plugin. Specifically it is designed to handle:

- Instantiating policies/ACL/QoS associated with containers
- Multi-tenant environment where disjoint networks are offered to containers on the same host
- SDN applications
- Interoperability with non container environment
- Multicast or multi-destination dependent applications
- Integration with existing IPAM tools for migrating customers

The overall design is _not_ assumed to be complete, because of ongoing work in the docker community with regards to the suitable APIs to interface with network extensions like this. Regardless, flexibility in the design has been taken into consideration to allow using a different state driver for key-value synchronization, or a different flavor of a soft-switch i.e. linux-bridge, MAC VLAN, or OpenvSwitch.

The ability to specify the intent succinctly is the primary goal of the design and thus some of the specified user interface will change, and in some cases functionality will be enhanced to accommodate the same. Design details and future work to be captured in a separate document.


###Building and Testing

- Build:

  `make build`

   Note:
   - building the project requires Go1.4. Instructions to install Go can be found at: https://golang.org/doc/install

- Run unit-tests:

  `make unit-test`

- Run system-tests:

  `make system-test`

###Trying it out 

The netplugin produces two binaries, a netplugin daemon and a netdcli tool to interact with it. The binaries can tried out in a vagrant environment, which can be setup as follows.

`make demo`

Note:
- Make sure VirtualBox is installed

`vagrant ssh netplugin-node1`

####A quick example

1. Start netplugin

    `sudo netplugin`

2. Create two containers `myContainer1` and `myContainer2`

    `sudo docker run -it --name=myContainer1 --hostname=myContainer1 ubuntu /bin/bash`

    `sudo docker run -it --name=myContainer2 --hostname=myContainer2 ubuntu /bin/bash`

3. Launch a desired configuration for the two containers

    `netdcli -cfg examples/one_host_vlan.json`

4. According to the desired network state `myContainer1` and `myContainer2` now belongs to `orange` network

    ```json
    {
        "Tenants" : [ {
            "Name"                      : "tenant-one",
            "DefaultNetType"            : "vlan",
            "SubnetPool"                : "11.1.0.0/16",
            "AllocSubnetLen"            : 24,
            "Vlans"                     : "11-28",
            "Networks"  : [
            {
                "Name"                  : "orange",
                "Endpoints" : [
                {
                    "Host"              : "host1",
                    "Container"         : "myContainer1"
                },
                {
                    "Host"              : "host1",
                    "Container"         : "myContainer2"
                }
                ]
            }
            ]
        } ]
    }
    ```

    If we examine the desired network state, it allows specifying the type of network as `vlan`, and subnet pools; those options are not mandatory but can be specified to override default values

5. The configuration remains persistent, i.e. `myContainer1` and `myContainer2` can go and come back and the configuration is restored

There are many variations to the above configuration, like creating multiple 
networks, across multiple hosts, use of VLANs, use of VXLAN, custom overrides
for IP/subnet/VLAN/VXLAN allocation on per network/endpoint basis. Please look
at [examples](examples/) directory to explore more sample configurations.

####Trying it out in a multi-host VLAN/VXLAN network

The [docs/TwoHostMultiVlanDemo.md](docs/TwoHostMultiVlanDemo.md) walks through setting up a multi host demo network and deploy the following Vlan based network:
![VlanNetwork](./docs/VlanNetwork.jpg)

One can deploy the following Vxlan network by following the steps in the above demo and using [two_hosts_multiple_vxlan_nets.json](two_hosts_multiple_vxlan_nets.json) configuration file instead. Trying out the configuration is left as an exercise to the reader.
![VxlanNetwork](./docs/VxlanNetwork.jpg)

####Multi-tenant network

In the examples directory [two_hosts_multiple_tenants.json](examples/two_hosts_multiple_tenants.json) and 
[two_hosts_multiple_tenants_mix_vlan_vxlan.json](examples/two_hosts_multiple_tenants_mix_vlan_vxlan.json) shows the creation of a multi-tenant
(disjoint, overlapping) networks within a cluster.

####Auto-allocation of IP addresses
The plugin can automatically manage the IP address pools and assign an appropriate IP address based on the subnet that was associated with the network. However this doesn't take away the flexibility to keep a specific IP address of a container, which can always be specified as shown earlier. To automatically allocate the IP address, just avoid specifying the IP address during endpoint creation or endpoint description

With this, associating containers with networks will ensure a unique IP address is assigned to the container

While auto-allocation is allowed, per endpoint override to use a specific IP address 
is available.

####Auto-allocation of Subnets
The plugin can automatically manage the assignment of IP subnets to be used for various networks that are created. This would require configuring the global pool of IP-subnets to pick the subnet allocation from. The implementation will allow distributed atomicity to avoid conflicts.

While auto-allocation is allowed, per network override to use a specific subnet 
is available.

####Auto-allocation of VLANs and VXLAN ids
Allocation of VLAN-ids is specifically useful to allow interacting containers with
non containerized applications. In many cases the default deployment choice of 
VLAN/VXLAN can be specified once as part of global configuration along with the
allowed range (to avoid possible conflict), etc.

Auto allocation of VLAN-ids and VXLAN-id will be done if the network is not specified with the VLAN/VXLAN id, and a global pool is available.

While auto-allocation is allowed, per network override to use a specific VLAN or VXLAN-id is available to handle specific cases.

####Fine grained control
The JSON interface is the simplest way to express the desired intent, however
incremental configuration and changes can be done quite easily using the
interface tools described in [Details](docs/ConfigDetails.md).

### How to Contribute
We welcome patches and contributions, please hit the GitHub page to open an issue or to submit patches send pull requests.
Happy hacking!

