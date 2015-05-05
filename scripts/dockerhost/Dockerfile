FROM      ubuntu
MAINTAINER Sachin Jain <sachja@gmail.com>

RUN apt-get update && apt-get install -y curl build-essential pkg-config python2.7-dev libpython2.7-dev

RUN curl -sSL https://get.docker.com/ubuntu/ | sh > /dev/null

RUN cd /tmp && \
curl -L  https://github.com/coreos/etcd/releases/download/v2.0.0/etcd-v2.0.0-linux-amd64.tar.gz -o etcd-v2.0.0-linux-amd64.tar.gz && \
tar -xzf etcd-v2.0.0-linux-amd64.tar.gz && \
cd /usr/bin && \
ln -s /tmp/etcd-v2.0.0-linux-amd64/etcd && \
ln -s /tmp/etcd-v2.0.0-linux-amd64/etcdctl

RUN apt-get install -y openvswitch-switch

ENV VERSION 2.26
RUN mkdir /src
WORKDIR /src
RUN curl https://www.kernel.org/pub/linux/utils/util-linux/v$VERSION/util-linux-$VERSION.tar.gz \
| tar -zxf-
RUN ln -s util-linux-$VERSION util-linux
WORKDIR /src/util-linux
RUN ./configure --without-ncurses
RUN make LDFLAGS=-all-static nsenter
RUN cp nsenter /usr/local/bin

ENV PATH /usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/gopath/bin