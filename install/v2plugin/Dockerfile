# Docker v2plugin container with OVS / netplugin / netmaster

FROM alpine:3.6
LABEL maintainer "Cisco Contiv (https://contiv.github.io)"

RUN mkdir -p /run/docker/plugins /etc/openvswitch /var/run/contiv/log \
 && echo 'http://dl-cdn.alpinelinux.org/alpine/v3.4/main' >> /etc/apk/repositories \
 && apk --no-cache add \
      openvswitch=2.5.0-r0 iptables ca-certificates openssl curl bash

# copy in binaries and scripts
ARG TAR_FILE
ADD ${TAR_FILE} /
COPY startcontiv.sh /

# this container is never run, it is exported for docker to run as a plugin,
# the startcontiv.sh script and the netplugin binaries are copied into the
# plugin's rootfs after export, this avoids copying into and out of container
ENTRYPOINT ["/startcontiv.sh"]
