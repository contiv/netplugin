#!/bin/bash

USAGE="Usage: $0 start <node-addr>"

if [ $# -ne 2 ]; then
    echo $USAGE
    exit 1
fi

swarm_version=1.2.0
node_addr=$2
source /etc/profile.d/envvar.sh

echo "start-swarm waiting for etcd"

etcdctl cluster-health
while [[ `etcdctl cluster-health | grep "cluster is healthy"` != "cluster is healthy" ]]; do
    echo "Cluster is unhealthy";
    sleep 1;
    etcdctl cluster-health;
done

case $1 in
start)
    unset http_proxy
    unset https_proxy

    echo "Pulling docker swarm image"
    /usr/bin/docker pull swarm:$swarm_version

    echo "Setting IPtables rules for swarm"
    sudo iptables -L INPUT | grep "swarm traffic 2375" || \
      sudo iptables -I INPUT 1 -p tcp --dport 2375 -j ACCEPT -m comment --comment "swarm traffic 2375"

    echo starting swarm as master on $node_addr
    /usr/bin/docker run -t -d -p 2375:2375 --name=swarm-manager \
        swarm:$swarm_version manage \
        -H :2375 \
        --strategy spread \
        --replication --advertise=$node_addr:2375 \
        etcd://$node_addr:2379
        /usr/bin/docker run -t --name=swarm-agent \
        swarm:$swarm_version join \
        --advertise=$node_addr:2385 \
        etcd://$node_addr:2379
    ;;

stop)
    # skipping `set -e` as we shouldn't stop on error
    /usr/bin/docker stop swarm-manager
    /usr/bin/docker rm swarm-manager
    /usr/bin/docker stop swarm-agent
    /usr/bin/docker rm swarm-agent

    echo "Clearing IPtables for docker swarm"
    sudo iptables -D INPUT -p tcp --dport 2375 -j ACCEPT -m comment --comment "swarm traffic 2375"
    ;;

*)
    echo USAGE: $usage
    exit 1
    ;;
esac

