netplugin
=========

Generic network plugin (experimental)

Testing
=======

`vagrant up`

Note: make sure virtualbox is installed

`vagrant ssh default`

`sudo -s`

`source /etc/profile.d/envvar.sh`

`cd $GOSRC/github.com/contiv/netplugin`

`make unit-test`

Play with daemon:
=================
`vagrant ssh default`

`sudo -s`

`source /etc/profile.d/envvar.sh`

`cd $GOSRC/github.com/contiv/netplugin`

`make build`

`$GOBIN/daemon`

- from another terminal:
--------------------------
- create a network

`$GOBIN/cli -oper create -construct network foo-net`

- read it's oper state

`$GOBIN/cli -oper get -construct network foo-net`

- create an endpoint

`$GOBIN/cli -oper create -construct endpoint -net-id foo-net -vlan-tag 12 foo-ep`

- read back some oper state

`$GOBIN/cli -oper get -construct network foo-net`

`$GOBIN/cli -oper get -construct endpoint foo-ep`

- Ensure that a port got added to the ovs bridge named vlanBr

`ovsctl show`

- verify that a linux device is also created for the port added above

`ip link show`

TODO: 'delete' operation is not yet working

