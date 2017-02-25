# Docker v2plugin container with OVS / netplugin / netmaster 

FROM ubuntu:16.04
MAINTAINER Cisco Contiv (http://contiv.github.io/)

RUN mkdir -p /run/docker/plugins /etc/openvswitch /var/run/contiv/log \
 && apt-get update \
 && apt-get install -y openvswitch-switch=2.5.0-0ubuntu1 \
        net-tools iptables iputils-ping netcat \
 && rm -rf /var/lib/apt/lists/*

COPY netplugin netmaster netctl startcontiv.sh /

ENTRYPOINT ["/startcontiv.sh"]
