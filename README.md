## Netplugin

Generic network plugin (experimental) is designed to handle networking use cases in clustered multi-host systems, beyond what is offered by the docker default plugin. Specifically it is designed to handle:
- Instantiating policies/acl/qos associated with containers
- Multi-tenant environment where disjoint networks are offered to containers on the same host
- SDN applications
- Interoperability with non container environment
- Multicast or multi-destination dependent applications
- Integration with existing IPAM tools for migrating customers

The overall design is not assumed to be complete, because of ongoing work in the docker community with regards to the suitable APIs to interface with network extensions like this. Regardless, flexibility in the design has been taken into consideration to allow using a different state driver for key-value synchronization, or a different flavor of a soft-switch i.e. linux-bridge, macvlan, or openvswitch

The ability to specify the intent succintly is the primary goal of the design and thus some of the specificed user interface will change, and in some cases functionality will be enhanced to accomodate the same. Design details and future work to be captured in a separate document.


###Building and Testing

`vagrant up`

Note: make sure virtualbox is installed

`vagrant ssh default`

`sudo -s`

`source /etc/profile.d/envvar.sh`

`cd $GOSRC/github.com/contiv/netplugin`

`make unit-test`

###Trying it out 

The netplug produces two binaries, netplugin (a daemon) and a CLI to interface with it.

####Bring up the netplugin daemon

`vagrant ssh default`

`sudo -s`

`source /etc/profile.d/envvar.sh`

`cd $GOSRC/github.com/contiv/netplugin`

`make build`

Ensure that $GOBIN is included in $PATH, then start the daemon as:

`netplugin`

####Create a network

Acquire another terminal to execute netdcli commands to ensrue the logs from netplugin does not mix with netdcli output. 
`netdcli -oper create -construct network tenant1-net1`

The oepration state of network can be read using 

`netdcli -oper get -construct network tenant1-net1`

####Create an endpoint (an endpoint is an interface to be associated with container)

`netdcli -oper create -construct endpoint -net-id tenant1-net1 -tag 12 -ip-address="11.1.1.1" tenant1-net1-ep1`

Reading back the endpoint operation state can be done using

`netdcli -oper get -construct network tenant1-net1`

`netdcli -oper get -construct endpoint tenant1-net1-ep1`

####Associate an endpoint to a container (this can also be done during endpoint create)

In order to associate a container to, create a container first
`docker run -it --name=myContainer1 --hostname=myContainer1 ubuntu /bin/bash`

Then, attach the container to the endpoint. Even if the association was done earlier it would work exactly the same
`netdcli -oper attach -construct endpoint -container-id myContainer1 tenant1-net1-ep1`

To associate the container during endpoint creation just pass `cont-id` parameter

To detach an endpoint from a container use detach command
`netdcli -oper detach -construct endpoint -container-id myContainer1 tenant1-net1-ep1`

####Ensure that all is operational

Ensure that a port got added to the ovs bridge named vlanBr

`sudo ovsctl show`

- verify that a linux device is also created for the port added above

`ip link show`

####Delete the endpoint

`netdcli -oper delete -construct endpoint tenant1-net1-ep1`

Read the network and endpoint state to verify that they are updated

