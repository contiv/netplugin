# Docker v2plugin container with OVS / netplugin / netmaster 

FROM alpine:3.5
MAINTAINER Cisco Contiv (http://contiv.github.io/)

RUN mkdir -p /run/docker/plugins /etc/openvswitch /var/run/contiv/log \
    && echo 'http://dl-cdn.alpinelinux.org/alpine/v3.4/main' >> /etc/apk/repositories \
    && apk update && apk add openvswitch=2.5.0-r0 iptables

COPY netplugin netmaster netctl startcontiv.sh /

ENTRYPOINT ["/startcontiv.sh"]
