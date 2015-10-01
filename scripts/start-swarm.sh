#!/bin/bash

USAGE="Usage: $0 <node-addr>"

if [ $# -ne 1 ]; then
    echo $USAGE
    exit 1
fi

node_addr=$1

echo "start-swarm waiting for etcd"

etcdctl cluster-health
while [[ `etcdctl cluster-health | head -n 1` != "cluster is healthy" ]]; do
    echo "Cluster is unhealthy";
    sleep 1;
    etcdctl cluster-health;
done

echo "starting swarm on " $node_addr

# Start swarm discovery
nohup /usr/bin/swarm join --advertise=$node_addr:2385 etcd://localhost:2379 > /tmp/swarm-join.log 2>&1 &

# Start swarm manager
nohup /usr/bin/swarm manage -H tcp://$node_addr:2375 etcd://localhost:2379 > /tmp/swarm-manage.log 2>&1 &
