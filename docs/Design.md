# Netplugin Design
Netplugin is an infrastructure piece that defines a consistent interface between a management function and a network function, thereby helping developers to quickly prototype and implement solutions based on the intermix of the two.

## Table of Contents
- [Audience](#audience)
- [Concepts and Definitions](#concepts-and-definitions)
- [Design Goals](#design-goals)
- [Core Constructs](#core-constructs)
- [High Level Design](#high-level-design)
- [Integration Details](#integration-details)
- [Sample Implementation](#sample-implementation)
- [Open Items and Ongoing Work](#open-items-and-ongoing-work)

## Audience
This document is targeted towards the developers looking into or working on integrating existing or new management functions with existing or new network functions.

## Concepts and Definitions
This section provides a brief overview of some terms used in rest of the document.

### Management function
[Wikipedia definition](http://en.wikipedia.org/wiki/Management):
>Management in business and organizations is the function that coordinates the efforts of people to accomplish goals and objectives using available resources efficiently and effectively.

In application delivery environments, a management system coordinates the efforts of agents to accomplish the goal of placing applications in a cluster of resources (like compute, storage and network) based on their requirements, using the available resources efficiently and effectively. The management system also provides a front end (a central command control) to accept/input the application requirements. In other contexts, application delivery environments are also referred to as orchestration or scheduling platforms, container runtimes etc and the applications are also referred to as compute jobs or services.

In application delivery environments there is usually at least one agent per managed resource. And the application requirements may include (but are not limited to) specification of resource constraints and affinities; communication patterns etc.

Unless explicitly stated otherwise, for the logic performed at the managed resource the Management Function refers to the agent running on that resource, otherwise it refers to the central command control for the user facing logic like accepting/processing application requirement. 

### Network function
The Network Function (or implementation) performs the programming of the network resource, in a cluster of resources managed by the Management Function. Traditional Layer 2/3 forwarding; flow based forwarding; overlays or virtual networks etc are a few network functions.

### Configuration v/s Intent
The Configuration is a piece of information that is consumed by and depicts the desired state of a function.

Needless to say, the configuration is specific to a function and mostly differs between implementations of functions of different nature (viz. management function v/s network function) but may also differ between implementation of functions of same nature (Layer 2 forwarding v/s flow based forwarding; Constraint based management v/s Resource based management, to name a few).

Composing/layering one function over other (management function over a network function in this document) is achieved by translating the output of one function to a configuration consumable by the next function. This logic of translation shall be referred to as configuration or state translation function.

Unless explicitly stated otherwise, 'Intent' refers to the configuration that is consumed by the a high level function like a management fuuction and usually needs to be passed through a state translation function in order to be consumed by the network function.

### Logically centralized decision
Logically centralized decision making refers to the act of reaching consensus on certain value in a distributed system. In context of this document the problems of resource allocation (like address allocation; vlan or vxlan id allocation etc) are assumed to be solved through logically centralized decision making. The details of the design or implementation of a logically centralized decision making system are outside the scope of this document.

## Design Goals
Integrating a management function with a network function usually requires a coupling of the two, thereby resulting in following challenges:
- There is often unclarity/uncertainty of the definition of the integrating interfaces between the two without an available implementation of at least one. This becomes even harder if both the implementations are a work in progress by independent teams, which is usually the case.
- It is difficulty to intermix the available implementations of functions without re-writing/modifying the functions themselves.
- There is a risk of exposing network function configuration (usually technology specific, imperative, procedural in nature) as management function configuration (which can be intent-based/declarative, promise-based etc), thereby complicating user experience. Example, to use a switch capable of supporting multi-teanancy using vlans and qos if the APIs exposed by the management function need to take care of specifying these values then it impedes defining a high-level API at management function.

To address the above challenges, we propose the Netplugin project. Netplugin has following broad design goal:
"Provide a platform that sits as a middle layer between the management function and the network programming function, by offering a minimal set of well-defined core interfaces, that reduces their coupling and enables easy integration of different implementations of these functions".

## Core Constructs
The Netplugin core builds on the following three fundamental constructs:

### Endpoint
An endpoint is an addressable entity that is able to communicate with a group of other similar entities. The communication between these entities stays within the same group only.

Notes:
  - Endpoint is able to communicate only within the group it belongs to. This group is called a 'Network' (see [next section](#network)) and is assumed to exist before endpoints can be added.
  - To keep the design independent of the underlying network implementation the definition of the Endpoint 'address'  (mac, ip based, dns-name etc) is intentionally left outside scope of the core interface.
  - The endpoint address allocation is left to the underlying management or network function implementations. The design provides hooks to set/compute the address of endpoints in a network at fixed stages like creation of endpoint and configuration replay/restore as part of handling resource allocation(see the Notes in [Interfaces section](#interfaces)).[<b>REVISIT</b>: this may change based on if we are able to move resource allocation as part of state translation stage]

### Network
A network identifies an arbitrary group of endpoints that are able to communicate with each other.

Notes:
  - Network implementation details are intentionally left out of the scope of core interface. Example, at first this definition of a network may seem to violate the traditional Layer3 routing in a Virtual Routing Context (aka Vrf, that helps connect Layer3 subnets). However, one possible way to achieve a router (Gateway) function using these constructs could be to create an Endpoint for the routable-subnet and add it to a network depicting the Vrf, thereby connecting the different subnets.

### State
State identifies a piece of information that can be uniquely identified by a 'key'. The Network and Endpoint shall each have some associated state with them. The State becomes the means to expose and integrate the features of the management function and networking function. With appropriate state translation functions in place this also achieves limiting the exposure of networking details in the high level management function configuration.

 Notes:
 - The 'key' is not interpretted by the core interface but is passed along the API implementation . The interpretation of 'key' is left to the implementation.
 - Similarly the structure of the information associated with the State is left to the implementation and is not interpretted by the core.

## High Level Design
The following two figures depict the possible deployment scenarios of a Netplugin implementation. These scenarios arise from the two possibilities by which a management function may expose it's plugin/extension interface viz. library-based or daemon-based.

- Compute management with library based networking interface
```
_______________________________________
| Managenment Function (Central command control)
|
|--------------------------------------
            |
____________V___________________________
| Server/Switch  (in a managed cluster)
| _______________________________________
| | Management Function (network interface implementation on agent)
| |       |    __________________________
| |       --> |
| |           | Plugin     ______________       _________________  
| |           |        --> |                   |
| |           |            | Driver         -->| Network Function
| ---------------------------------------      |
|
|----------------------------------------
```

- Compute management with daemon based network interface
```
_______________________________________
| Managenment Function (Central command control)
|
|--------------------------------------
            |
____________V___________________________
| Server/Switch  (in a managed cluster)
| _______________________________________
| | Management Function (agent)
| |______________________________________
|        |
| _______V_______________________________
| | Netplugin Daemon (Management function's network interface implementation)
| |       |    __________________________
| |       --> |
| |           | Plugin     ______________       __________________
| |           |        --> |                    |
| |           |            | Driver         --> | Network Function
| ---------------------------------------       |
|
|----------------------------------------
```

### Interfaces
Netplugin defines two broad interfaces viz. Plugin and Driver. These interfaces logically define the boundaries at which the management and network functions integrate.

#### Plugin
The Plugin interface provides the API that faces and is invoked by the management function. The implementation of the Plugin API is generic and ensures some constraints (discussed in the [next section](#constraints)). The Plugin API shall be invoked by the management functions in their own specific way. Example, a Docker extension implementation shall invoke the Netplugin interface as a library ([docker issue #9983](https://github.com/docker/docker/issues/9983) ), while a Rocket plugin implementation shall invoke the Netplugin interface from a daemon/web-service serving rocket specific REST APIs([Rocket networking proposal](https://docs.google.com/document/d/1PUeV68q9muEmkHmRuW10HQ6cHgd4819_67pIxDRVNlM/edit#heading=h.ievko3xsjwxd)). For more details on integrating Plugin interface with management function please refer the section on [Integration](#integration-details).

Notes:
- The generic Plugin interface implementation has following scope and assumptions:
  + A Neplugin instance is run on each host or device where the network function is performed with no requirement of knowledge of existence of other instances in the Plugin interface. The implementation of the management function and the network function however may be aware of other instances.
- <b>Resource management</b>: Part of network programming involves managing (allocation/deallocation) certain resources like vlan/vxlan ids, ip addresses etc. In most cases, the resource management might be suitably solved by logically centralized decision making, which may be internal or external to the Management Function. However, certain management functions may offload it to the Network Function. The Plugin interface shall contain the necessary API to allocate/deallocate resource in a implementation specific way. The generic Plugin interface shall ensure the constraints to handle resource allocation/deallocation (discussed in next section) [<b>REVISIT</b>: need to ensure that resource allocation can be decoupled from programming implementation. Also this shall change if we are able to move resource allocation to state translation]

#### Driver
The Driver interface provides the APIs that face the networking function. The Driver interface is invoked by the Plugin interface (maintaining the constraints). The implementation of the Driver interface is specific to the network function. Example, linux bridge, ovs, SRIOV etc. The Driver interface also provides the APIs to manage the State, which allows supporting different state management mechanisms(etcd, zookeeper, libpack, configuration files etc to name a few).

### Constraints
[<b>REVISIT</b>:]
As discussed above the Plugin interface implementation invokes the Driver interface while maintaining some constraints. The following constraints are ensured:

1. A Network has been created before Endpoint creation in that Network can be requested.
2. All Endpoints are deleted before a Network conatining them is deleted.
3. Resource allocation for a network (vlan ids, vxlan ids) and endpoint (ip address) happens before the creation of network and endpoint respectively.
4. Resource deallocation for a network (vlan ids, vxlan ids) and endpoint (ip address) happens before the deletion of network and endpoint respectively.

Notes:
 - The above constraints provide guarantees wrt the order of Driver interface invocation. However, they do not guarantee the presence/absence of the state when the Driver interface is actually invoked. The driver implementations are expected to deal with such scenarios. Example, when a delete for endpoint is received it is not guaranteed that the network state will exist. So a driver implementation might need to cache/keep enough state to handle endpoint deletion gracefully.
 - The Constraints 'c' and 'd' might become out of scope of the Plugin interface based on design approach we pick. See section on [open design items](#open-items-and-ongoing-work>)

## Integration Details
This section briefly describes some of the existing/upcoming management and network function implementations. And then describes how Netplugin interface helps achieve intermixing these implementations.

### With Management Function
[<b>REVISIT</b>: add more details]
- Discuss docker's proposed API
- Discuss rocket's proposed API

### With Network Function
[<b>REVISIT</b>: add more details]
- Discuss networking with ovs and vlan/vxlan
- Discuss networking with Linux bridge

### Intermixing Management and Network Functions
One of the goals of Netplugin is to allow intermixing different implementations of the management and network function without rewriting the two. Netplugin achieves this by offering the provisions for the following in it's core:
- <b>Driver State</b>: The Driver State defines a contract of the configuration that the underlying network function is able to consume and act on. This enables the Network function to independently identify and define the parameters required for exercising the features and capabilities it offers.
- <b>State translation function</b>: The State translation function defines the necessary transformations that are required to arrive at the Driver State from another state, let's call it Management State. The Management State can in turn be specified or arrived at in one of following ways:
  - It is expressed at the central command control logic of Management Function. Example, it is passed through the management function's UI.
  - It is arrived at after post processing of the state expressed at the central command control logic of Management Function. Example, post processing of state might involve expansion of the resource requests to appropriate resource values through a logical centrallized  decision making.

Irrespective of how the Management State is achieved, the state translation function is then invoked as part Management function's network interface implementation before calling into the Plugin interface.

As can be seen, to allow for easy intermixing of the management and network functions we just need to implement a state translation function for each such combination (of management and network function).

## Sample Implementation
[<b>REVISIT</b>: add more details]
- discuss netd and netdcli implementation that hooks onto etcd and docker events to simulate a management function; and uses ovs-driver underneath to interface with ovs based network function.
- might be good to have more implementations to discuss and demo intermixing implementations

## Open Items and Ongoing Work
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
- [ ] enhance OvsState<> write to take locks for updates to a common state. Eg. Network oper-state
- [ ] Revert Delete_<> APIs to take the 'key' for the state to free Core interface from dependece on state's contents. A simple state implementation results in duplication of some state. To avoid state duplication between current config and oper states, we can also potentially enhance OvsState<> implementation to implement ref-counting mechanism that prevent the state from deleted until everyone has dropped their reference. This introduces need for state locks etc which are needed for other cases as well and need to be addressed.
- [ ] Move ContainerContext related APIs out of the core.
- [ ] Move the Container driver interface out of the core, as netplugin is supposed to be plugged into a container-technology (docker, rocket etc) from it's north-bound API.  
- [ ] [minor/non-urgent, just for tracking] need to add data-compression/encoding in state-driver and write a pipe utility instead to decipher the state returned by say etcdctl get or curl etc.
- [ ] add a Linux bridge based network implementation
- [ ] Anything else?
