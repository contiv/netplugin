## Netplugin

Generic network plugin (experimental) is designed to handle networking use cases in clustered multi-host systems, beyond what is offered by the docker default plugin. Specifically it is designed to handle:
- Instantiating policies/acl/qos associated with containers
- Multi-tenant environment where disjoint networks are offered to containers on the same host
- SDN applications
- Interoperability with non container environment
- Multicast or multi-destination dependent applications
- Integration with existing IPAM tools for migrating customers

The overall design is _not_ assumed to be complete, because of ongoing work in the docker community with regards to the suitable APIs to interface with network extensions like this. Regardless, flexibility in the design has been taken into consideration to allow using a different state driver for key-value synchronization, or a different flavor of a soft-switch i.e. linux-bridge, macvlan, or openvswitch

The ability to specify the intent succintly is the primary goal of the design and thus some of the specificed user interface will change, and in some cases functionality will be enhanced to accomodate the same. Design details and future work to be captured in a separate document.


###Building and Testing

`vagrant up`

Note:
- make sure virtualbox is installed
- The guest vm provisioning requires downloading packages from the Internet, so a http-proxy needs to be set in the vm if you are behind one else the vm setup will fail. It can be specified by setting the VAGRANT_ENV variable to a string of a space separated `<env-var>=<value>` pairs.
`VAGRANT_ENV="http_proxy=http://my.proxy.url https_proxy=http://my.proxy.url" vagrant up`

`vagrant ssh default`

`sudo -s`

`source /etc/profile.d/envvar.sh`

`cd $GOSRC/github.com/contiv/netplugin`

`make unit-test`

###Trying it out 

The netplugin produces two binaries, a netplugin daemon and a netdcli tool to interact with it.

####A quick example

1. Start netplugin

`netplugin`

2. Create two containers `myContainer1` and `myContainer2`

`docker run -it --name=myContainer1 --hostname=myContainer1 ubuntu /bin/bash`
`docker run -it --name=myContainer2 --hostname=myContainer2 ubuntu /bin/bash`

3. launch a desired configuration for the two containers

`netdcli -cfg json_examples/one_host_vlan.json`

4. According to the desired network state `myContainer1` and `myContainer2` now belons to `orange` network

```json
{
    "DefaultNetType"            : "vlan",
    "SubnetPool"                : "11.1.0.0/16",
    "AllocSubnetLen"            : 24,
    "Vlans"                     : "11-28",
    "Networks"  : [
    {
        "Name"                  : "orange",
        "Endpoints" : [
        {
            "Container"         : "myContainer1"
        },
        {
            "Container"         : "myContainer2"
        }
        ]
    }
    ]
}
```

If we examine the desired network state, it allows specifying the type of network as `vlan`, and subnet pools; thse options are not mandatory but can be specified to override default values

5. The configuration remains persistent, i.e. myContainer1 and myContainer2 can go and come back, the configuration is restored

There are many variations to the above configuration, like creating multiple 
networks, across multiple hosts, use of vlans, use of vxlan, custom overrides 
for ip/subnet/vlan/vxlan allocation on per network/endpoint basis. Please look
at json_examples directory to explore more details

####Trying it out on a multi-host vlan/vxlan network

<Diagrams and Vagrant files to be added for this and configuraiton below>
However feel free to look at json_examples/two_host_vlan.json and 
two_host_vxlan.json as a starting point for cluster of two hosts.

####Multi-tenant network

In the examples direcotry one_host_multiple_nets.json and 
two_hosts_multiple_vxlan_nets.json shows creation of a multi-tenant (disjoint, 
overlapping) networks within a cluster

####Auto-allocaiton of IP addresses
The plugin can automatically manage the IP address pools and assign an appropriate IP address based on the subnet that was associated with the network. However this doesn't take away the flexibility to keep a specific IP address of a container, which can always be specified as shown earlier. To automatically allocate the IP address, just avoid specifying the IP address during endpoint creation, for example in the previous example:
`netdcli -oper create -construct endpoint -net-id tenant1-net1 -container-id myContainer2 tenant1-net1-ep2`

With this, associating containers with networks will ensure a unique IP address is assigned to the container

While auto-allocation is allowed, per endpoint override to use a specific IP address 
is avialable.

####Auto-allocaiton of Subnets
The plugin can automatically manage the assignment of IP subnets to be used for various networks that are created. This would require configuring the global pool of ip-subnets to pick the subnet allocation from. The implementation will allow distributed atomicity to avoid conflicts

While auto-allocation is allowed, per network override to use a specific subnet 
is avialable.

####Auto-allocation of vlans and vxlan ids
Allocation of vlan-ids is specifically useful to allow interacting containers with 
non containerized applications. In many cases the default deployment choice of 
vlan/vxlan can be specified once as part of global configuration along with the 
allowed range (to avoid possiblee conflict), etc.

Auto allocation of vlan-ids and vxlan-id will be done if the network is not specified with the vlan/vxlan id, and a global pool is available.

While auto-allocation is allowed, per network override to use a specific vlan or vxlan-id is avialable to handle specific cases

####Fine grained control
The json interface is simplest way to express the desired intent, however
incremental configuration and chagnes can be done quite easily using the
interface tools described in [Details](docs/ConfigDetails.md)

### How to Contribute
We welcome patches and contributions, please hit the github page to open an issue or to submit patches send pull rquests. 
Happy hacking!

