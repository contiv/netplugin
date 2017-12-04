# OVS Docker image

FROM alpine:3.7
LABEL maintainer "Cisco Contiv (https://contiv.github.io)"

RUN mkdir -p /etc/openvswitch /var/run/contiv/log \
 && echo 'http://dl-cdn.alpinelinux.org/alpine/v3.4/main' >> /etc/apk/repositories \
 && apk --no-cache add \
      openvswitch=2.5.0-r0 iptables ca-certificates openssl curl bash

# copy scripts
COPY ovsInit.sh /
#COPY startcontiv.sh /

ENTRYPOINT ["/ovsInit.sh"]
