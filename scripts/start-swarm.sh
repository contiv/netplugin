#!/bin/bash

USAGE="Usage: $0 <node-addr> <master|slave>"

if [ $# -ne 2 ]; then
    echo $USAGE
    exit 1
fi

node_addr=$1
mode=$2
source /etc/profile.d/envvar.sh

echo "start-swarm waiting for etcd"

etcdctl cluster-health
while [[ `etcdctl cluster-health | head -n 1` != "cluster is healthy" ]]; do
    echo "Cluster is unhealthy";
    sleep 1;
    etcdctl cluster-health;
done

echo "starting swarm in" $mode "mode on" $node_addr
echo "starting swarm join"
# Start swarm discovery
nohup /usr/bin/swarm join --advertise=$node_addr:2385 etcd://localhost:2379 > /tmp/swarm-join.log 2>&1 &

# echo "starting netplugin"
# start netplugin
# nohup /opt/gopath/bin/netplugin -native-integration=true > /tmp/netplugin.log 2>&1 &

if [[ $mode == "master" ]]; then
    # echo "starting netmaster"
    # start netmaster
    # nohup /opt/gopath/bin/netmaster > /tmp/netmaster.log 2>&1 &

    unset http_proxy
    unset https_proxy
    echo "starting swarm manager"
    # Start swarm manager
    nohup /usr/bin/swarm manage -H tcp://$node_addr:2375 etcd://localhost:2379 > /tmp/swarm-manage.log 2>&1 &

fi
