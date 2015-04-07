FROM      ubuntu
MAINTAINER Sachin Jain <sachja@gmail.com>

RUN apt-get update && apt-get install -y curl

RUN cd /tmp && \
curl -L  https://github.com/coreos/etcd/releases/download/v2.0.0/etcd-v2.0.0-linux-amd64.tar.gz -o etcd-v2.0.0-linux-amd64.tar.gz && \
tar -xzf etcd-v2.0.0-linux-amd64.tar.gz && \
cd /usr/bin && \
ln -s /tmp/etcd-v2.0.0-linux-amd64/etcd && \
ln -s /tmp/etcd-v2.0.0-linux-amd64/etcdctl

RUN curl -sSL https://get.docker.com/ubuntu/ | sh > /dev/null

RUN apt-get install -y openvswitch-switch
