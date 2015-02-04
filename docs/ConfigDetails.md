## Configuration Details

Use of json configuration allows specfying the network intent succintly. 
However, netplugin cli interface allows a way to incrementally manipulate 
global, network, endpoint and container level network configuration.
This document describes the netdcli interface, which results into 
creating the backend objects that can be executed on any host
within the clustered network

This document assumes that the user is familiar with building and bringing
up the netplugin.

####Network Creation

A network is arbitrary set of endpoints contained within. A network consists
of a network type `vlan` or `vxlan`, subnet and a mask, default gateway, etc.

A network can be created using `network construct` in the netdcli and 
associate an arbirary name with it. For example:

`netdcli -oper create -construct network -tag 12 -subnet 0/24 orange`

In the above 0/24 is a way to auto-allocate the IP pool from global
resources, which can be overridden by specifying a specific subntet. Options
like `tag` are optional and are auto-allocated too from the global pool 
when not specified.

The oeprational state of network can be read using the following cli

`netdcli -oper get -construct network orange`

In almost all cases the state can also be read from the state management
tools like `etcdctl`

####Endpoint Creation

An endpoint is an interface to be associated with container. Therefore a 
container can have more than one endpoints, but one endpoint only belongs
to one and only one container. An endpoint is assigned an IP address based
on the network it belongs to. For example an endpoint in the network `orange`
can be created as follows:

`netdcli -oper create -construct endpoint -net-id ornage -ip-address="11.1.1.1" orange-ep1`

It would create an endpoint `orange-ep` in the network `orange` and is 
assigned the specified IP address. IP addresses can be auto-allocated if 
unspecified.  Hoewver a network id must be specified to associate an 
endpoint to a network.

The operational state of an endpoint can be read back using

`netdcli -oper get -construct endpoint orange-ep1`

In a multihost environment, an endpoint is created on a specific host, thus
a host label can be specified during ep creation. This allows the netdli to 
be executed from any host wihtin the cluster. The `host`, which defaults to
what `hostname` command would show, can can be overridden with another
label that is unique across the cluster

`netdcli -oper create -construct endpoint -net-id ornage -host=$HOSTNAME -ip-address="11.1.1.1" orange-ep1`

####Attach an endpoint to a container 

An endpoint can be attached or detached to a container. This of course could 
be done at the time of ep creation, but not required to be done at that time

A container to be associated with the endpoint can be created before or 
after endpoint creation. Regardless of the order the network configuration
is instantiated as soon as container comes up.

In this example, if we create a container first, as follows:

`docker run -it --name=myContainer1 --hostname=myContainer1 ubuntu /bin/bash`

Then, to attach the container to the endpoint, following netdcli can be 
used:

`netdcli -oper attach -construct endpoint -container-id myContainer1 orange-ep1`

If a container to be associated is known during ep creation time, then 
it can be specified during ep creation, therefore the ep creation that
was done above would have looked like:

`netdcli -oper create -construct endpoint -net-id orange -ip-address="11.1.1.2" -container-id myContainer2 orange-ep1

####Adding more containers to the network

If another container `myContainer2` is started 
`docker run -it --name=myContainer2 --hostname=myContainer2 ubuntu /bin/bash`

Then this container can be attached to the same `orange` network as follows:

`netdcli -oper create -construct endpoint -net-id orange -ip-address="11.1.1.2" -container-id myContainer2 orange-ep2`

Two endpoints in a network can communicate with each other, therefore after
doing an ep creation and association to `myContainer2` the two containers 
would have been allocated and configured with specific addresses in a
dedicated network `orange` Therefore, our classic ping test should work 
from either container to another.

On other hand, creating endpoints before containers are launched is good
because it offers no disruption to the application trying to talk out or
to another application

####Ensure that all is operational

Besides examining the global, network and endpoint state, the network
configuration can be ensured using standard linux commands on the host where
containers are running

`sudo ovsctl show`

And to verify verify that a linux device is also created for the ports 
created for EPs, we can use following mechanisms within the containers

`ip link show`

or 

`ifconfig -a`


####Detach an endpoint from a container

To detach an endpoint from a container use detach command

`netdcli -oper detach -construct endpoint -container-id myContainer1 orange-ep1`

This would remove the endpoint from `myContainer1`, and now the endpoint can be asosciated with another container within the same network

####Delete the endpoint

Deletion of an endpoint is a very common event that is triggered when a 
container needs to be disposed off and thus networking configuration related
to this must also be removed. To remove an endpoint, simply issue `delete`
operation on `endpoint` construct for a given ep

`netdcli -oper delete -construct endpoint orange-ep1`

Read the network and endpoint state to verify that they are removed from the
configuration.

####Deleting a network
Networks are disposable entities and can be dynmically deleted at will. To 
delete a network we cause use `delete` operation on `network` construct

`netdcli -oper delete -construct network orange`

####How to debug errors
If things fail to work, look for netdcli and netplugin logs that are spewed 
on the standard output (will be moved to log files later)
And as always, feel free to report documentation errors if you see discrepancy


