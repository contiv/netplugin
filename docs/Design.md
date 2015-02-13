#Netplugin design goals and definitions

Lately, networking of compute nodes (especially vms and containers) has gained a lot of traction as the packet switching technologies have been extended all the way down to the edge (from a traditional top of rack switch) in form of software switches (linux bridge, ovs, vendor-proprietary switch etc) running on the servers. Each software switching system offers one or more features (L2/L3 forwarding, multi-tenancy, qos etc) with it's own configuration and programming semantics.

To add to this the compute management systems have seen a lot of development as well with several technologies showing up (vmware, kvm, virtualbox etc for vms and docker, coreos rocket, kubernetes etc for containers). These compute management systems offer their own configuration semantics and interfaces for integration with the underlying networking technologies (sometimes requiring very tight coupling between the two).

This direct coupling of the two systems results in somewhat stagnated progress on integration of these two disparate (compute management and networking) functions. Example, there is no easy way to start developing the code to integrate available networking technologies for docker or rocket based containers since the APIs exposed by these platforms are still a WIP.

Another disadvantage of this coupling/dependence is that sometimes it requires laying the management function concepts(intent-based/declarative, promise-based etc) over the network programming function concepts(imperative, procedural etc). Since management function are mostly user facing, this in turn requires exposing underlying details to the user often resulting in a complicated UI. Example, to use a switch capable of supporting multi-teanancy using vlans and qos the APIs exposed by the management function need to take care of specifying these values which impedes defining a high-level API at management function.

To eliminate the above limitation, we propose the Netplugin project. Netplugin has following broad design goal:
"Provide a platform that sits as a middle layer between the management function and the network programming function, by offering a minimal set of well-defined core constructs and interfaces, to enable easy integration of different (independent) implementations of these functions while limiting the exposure of underlying networking details to the higher layer API at management function"

##Netplugin core interfaces/constructs

The Netplugin core builds on the following three fundamental constructs:

###Endpoint

An endpoint is an addressable entity that is able to communicate with a group of other similar entities. The communication between these entities stays within the same group only.

  Notes:
  - Endpoint is able to communicate only within the group it belongs to. This group is called a 'Network' (see <a href=https://github.com/contiv/netplugin/blob/designspec/docs/Design.md#network>next section</a>) and can be assumed to exist before endpoints can be added.
  - To keep the design independent of the underlying network implementation the definition of the Endpoint 'address'  (mac, ip based, dns-name etc) is intentionally left outside scope of the core interface.
  - The endpoint address allocation is left to the underlying network function implementation. However, the design provides enough hooks to set/compute the address of endpoints in a network at fixed stages like creation of endpoint and configuration replay/restore (see the Notes in <a href=https://github.com/contiv/netplugin/blob/designspec/docs/Design.md#interfaces>Interfaces section</a> ).

###Network

A network identifies the group of endpoints that are able to communicate with each other.

  Notes:
  - Network implementation details are intentionally left out of the scope of core interface. Example, at first this definition of a network may seem to violate the traditional Layer3 routing in a Virtual Routing Context (aka Vrf, that helps connect Layer3 subnets). However, one possible way to achieve a router (Gateway) function using these constructs could be to define an Endpoint for the router and add it to a network depicting the Vrf, thereby connecting the different subnets.

###State

State identifies a piece of information that can be uniquely identified by a 'key'. The Network and Endpoint shall each have associated state. The State becomes the means to expose and integrate the features of the management function and networking function. With appropriate State translation in place this achieves limiting the exposure of networking details in high level management API.

 Notes:
 - The 'key' is not interpretted by the core interface but is passed along the API implementation . The interpretation of 'key' is left to the implementation.
 - Similarly the structure of the information associated with the Network and Endpoint is left to the implementation and is not interpretted by the core.
  

##Netplugin design

- Compute management with library based networking interface
```
_______________________________________
| Management Function (network interface implementation)
|       |    __________________________
|       --> |
|           | Plugin     ______________       _________________  
|           |        --> |                   |
|           |            | Driver         -->| Network Function
---------------------------------------      |
```

- Compute management with daemon based network interface
```
_______________________________________
| Management Function
|______________________________________
       |
_______V_______________________________
| Daemon/Agent (Management function's network interface implementation)
|       |    __________________________
|      --> |
|           | Plugin     ______________       __________________
|           |        --> |                    |
|           |            | Driver         --> | Network Function
---------------------------------------       |
```

###Interfaces

Netplugin defines two broad interfaces viz. Plugin and Driver. These interfaces logically define the boundaries at which the management and networking function plug-in.

The Plugin interface provides the API that faces and is invoked by the management function. The implementation of the Plugin API is generic and ensures some constraints (discussed in <a href=https://github.com/contiv/netplugin/blob/designspec/docs/Design.md#constraints>next section</a>). This API is invoked by the management function in it's own specific way. Example, a docker extension implementation shall invoke the Netplugin interface as a library (atleast in the initial implementation), while a rocket plugin implementation shall invoke the Netplugin interface from a daemon/web-service serving rocket specific REST APIs.

The Driver interface provides the APIs that face the networking function. The Driver interface is invoked by the Plugin interface (maintaining the constraints). The implementation of the Driver interface is specific to the network function. Example, linux bridge, ovs, SRIOV etc

Notes:
- The generic Plugin interface implementation has following scope and assumptions:
  + A Neplugin instance is run on each host or device where the network function is performed with no requirement of knowledge of existence of other instances in the Plugin API. The implementation of the management function and the network function however may be aware of other instances.
- <b>Resource management</b>: Part of network programming involves managing certain resources like vlan/vxlan encaps, ip addresses etc. In most cases, the resource management might be suitably solved by a central entity which is internal or external to the management function. However, certain management functions may just offload it to the network function. The Driver interface shall contain the necessary API to allocate/deallocate resource in a implementation specific way. The generic Plugin interface shall ensure the constraints to handle resource allocation/deallocation (discussed in next section) [TODO: need to ensure that resource allocation can be decoupled from programming implementation]

###Constraints

As discussed above the Plugin interface implementation invokes the Driver interface while maintaining some constraints. The following constraints are ensured:
<ol type="a">
<li> A Network has been created before Endpoint creation in that Network can be requested.
<li> All Endpoints are deleted before a Network conatining them is deleted.
<li> Resource allocation for a network (vlan ids, vxlan ids) and endpoint (ip address) happens before the creation of network and endpoint respectively.
<li> Resource deallocation for a network (vlan ids, vxlan ids) and endpoint (ip address) happens before the deletion of network and endpoint respectively.
</ol>

Notes:
 - The above constraints provide guarantees wrt the order of Driver interface invocation. However, they do not guarantee the presence/absence of the state when the Driver interface is actually invoked. The driver implementations are expected to deal with such scenarios. Example, when a delete for endpoint is received it is not guranteed that the network state will exist. So a driver implementation might need to cache/keep enough state to handle endpoint deletion gracefully.
 - The Constraints 'c' and 'd' might become out of scope of the Plugin interface based on design approach we pick. See section on <a href=https://github.com/contiv/netplugin/blob/designspec/docs/Design.md#design-open-items-and-ongoing-work>open design items</a>

##Design open items and ongoing work:

The current code needs some refactoring to align to above design. Following is the list of open items:
- [ ] Allocators (right now gstate): 
  + Design option 1
    - Shall be implemented as drivers.
    - Should have their own state (for network and endpoint).
    - Shall be invoked as part of the netplugin infra code flow itself.
    - Need to think how are they invoked. One way is to invoked at certain points in netplugin infra code.
  + Design option 2
    - Shall be implemented as APIs in the Plugin network and endpoint classes. This gives a chance to the orchestrator-layer/caller on the minion to setup driver state with allocated values.
  + Design option 3
    - Shall be implemented as APIs in the Driver network and endpoint classes. This gives a chance to the Plugin to setup driver state with allocated values, while ensuring constraints.
- [ ] Move ContainerContext related APIs out of the core.
- [ ] Move the Container driver interface out of the core, as netplugin is supposed to be plugged into a container-technology (docker, rocket etc) from it's north-bound API.  
- [ ] [minor/non-urgent, just for tracking] need to add data-compression/encoding in state-driver and write a pipe utility instead to decipher the state returned by say etcdctl get or curl etc.
- [ ] Anything else?
