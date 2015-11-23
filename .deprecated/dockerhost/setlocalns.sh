#!/bin/bash
scriptdir=`dirname "$BASH_SOURCE"`
HOST_SCRIPTDIR=$CONTIV_DIND_HOST_GOPATH/src/github.com/contiv/netplugin/scripts/dockerhost
pid=$($scriptdir/host-sh $HOST_SCRIPTDIR/docker-pid $(hostname))
FILES=$($scriptdir/host-sh ls /sys/class/net/)
for intf in $FILES
do
    if [[ $intf = port* ]];
    then
	$scriptdir/host-sh ip link set $intf netns /proc/$pid/ns/net
    fi
done
