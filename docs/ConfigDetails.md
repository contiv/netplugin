## Configuration Details

Use of json configuration allows specfying the network intent succintly. 
However, there exists rich netplugin cli interface that allows a way to 
incrementally manipulate global, network, endpoint and container level 
network configuration.
This document describes the details of that interface, most of the interface
results into creating the backend objects that can be executed on any host
within the clustered network

This document does not describe how to bringup the netplugin, rather assumes
that the user is familiar with that process.

####Creating a network

A network is arbitrary set of endpoints contained within. A network consists
of a network type `vlan` or `vxlan`, subnet and a mask, default gateway, etc.

One can create a network using construct `network` in the netdcli and associate
an arbirary name with it. For example:

`netdcli -oper create -construct network -tag 12 -subnet 0/24 orange`

Note that in the above 0/24 is a way to auto-allocate the IP pool from global
resources, which can be overridden by specifying a specific subntet. Options
like `tag` are optional and are auto-allocated too from the pool when not
specified.

The oepration state of network can be read using 

`netdcli -oper get -construct network tenant1-net1`

####Creating an endpoint 

An endpoint is an interface to be associated with container. Therefore a 
container can have more than one endpoints, but one endpoint only belongs
to one and only one container. An endpoint is assigned an IP address based
on the network it belongs to. For example an endpoint in the network `orange`
can be created as follows:

`netdcli -oper create -construct endpoint -net-id ornage -ip-address="11.1.1.1" orange-ep1`

This would create an endpoint `orange-ep` in the network `orange` and is 
assigned an IP address. IP addresses can be auto-allocated and is not required
to be specified, hoewver a network id must be specified to associate an endpoint
to a network.

The operational state of an endpoint can be read back using

`netdcli -oper get -construct network orange`

`netdcli -oper get -construct endpoint orange-ep1`

In a multihost environment, an endpoint is created on a specific host, thus
a host label can be passed at the ep creation. This allows the netdli to be
executed from almost anywhere wihtin the cluster. The `host` picks the default
value to be what `hostname` would show, but can be overridden with another
label that is unique across the cluster

`netdcli -oper create -construct endpoint -net-id ornage -host=$HOSTNAME -ip-address="11.1.1.1" orange-ep1`

####Attach an endpoint to a container 

An endpoint can be attached or detached to a container. This of course could 
be done at the time of ep creation, but not required to be done at that time

In order to associate a container to, create a container first

`docker run -it --name=myContainer1 --hostname=myContainer1 ubuntu /bin/bash`

Then, to attach the container to the endpoint, netdcli can be used as follows:

`netdcli -oper attach -construct endpoint -container-id myContainer1 orange-ep1`

To associate a container during endpoint creation just pass container name
during ep creation, therefore the ep creation done above wold have looked like:

`netdcli -oper create -construct endpoint -net-id orange -ip-address="11.1.1.2" -container-id myContainer2 orange-ep1

####Adding more containers to the network
Let us start another container
`docker run -it --name=myContainer2 --hostname=myContainer2 ubuntu /bin/bash`

Add the newly added container to the same `orange` network (assuming it belongs to the
same tenant), as follows:

`netdcli -oper create -construct endpoint -net-id orange -ip-address="11.1.1.2" -container-id myContainer2 orange-ep2`

At this point both containers would have been configured with specified or 
auto-allocated IP address in a dedicated network `orange` Therefore, our 
classic ping test should work from either container to another.

####Ensure that all is operational

Besides eximining the global, network and endpoint state, the network
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

This would remove the endpoint from myContainer1, and now the endpoint can be asosciated with another container within the same network

####Delete the endpoint

Deletion of an endpoint is a very common event that is triggered when a 
container needs to be disposed off and thus networking configuration related
to this must also be removed. To remove an endpoint, simply issue `delete`
t
operation on `endpoing` construct for a given ep

`netdcli -oper delete -construct endpoint orange-ep1`

Read the network and endpoint state to verify that they are removed from the
configuration.

####Deleting a network
Networks are disposables entities and can be dynmically deleted at will. To 
delete a network we cause use `delete` operation on `network` construct

`netdcli -oper delete -construct network orange`

####How to debug errors
If things fail to work, look for netdcli and netplugin logs that are spewed on
the standard output (will be moved to log files later)
And as always, feel free to report documentation errors if you see discrepancy


