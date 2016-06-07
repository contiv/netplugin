#!/bin/bash

apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv E56151BF
apt-key adv --keyserver hkp://pgp.mit.edu:80 --recv-keys 58118E89F3A912897C070ADBF76221572C52609D

DISTRO=$(lsb_release -is | tr '[:upper:]' '[:lower:]')
CODENAME=$(lsb_release -cs)

echo "deb http://repos.mesosphere.io/${DISTRO} ${CODENAME} main" | \
  tee /etc/apt/sources.list.d/mesosphere.list
echo "deb https://apt.dockerproject.org/repo ${DISTRO}-${CODENAME} main" | \
  tee /etc/apt/sources.list.d/docker.list


VAGRANT_PATH=/opt/gopath/src/github.com/contiv/netplugin

apt-get -y update

# install zookeeper and docker
mkdir /etc/sysconfig
apt-get -y install zookeeper zookeeperd zookeeper-bin clang aufs-tools
apt-get -y install marathon

export CC=clang
export CXX=clang++

usermod -aG docker vagrant

service zookeeper restart

# Install Mesos-DNS
wget -q https://dl.dropboxusercontent.com/u/4550074/mesos/mesos-dns+50fc45a9 -O /usr/bin/mesos-dns > /dev/null 2>&1
chmod +x /usr/bin/mesos-dns

### Install Mesos

#apt-get -qy install software-properties-common # (for add-apt-repository)
#apt-get update -q
apt-get -qy install \
  cmake=3.* \
  build-essential                         \
  autoconf                                \
  automake                                \
  ca-certificates                         \
  gdb                                     \
  wget                                    \
  git-core                                \
  libcurl4-nss-dev                        \
  libapr1-dev                             \
  libsvn-dev                              \
  libsasl2-dev                            \
  libtool                                 \
  libsvn-dev                              \
  libapr1-dev                             \
  libgoogle-glog-dev                      \
  libboost-dev                            \
  protobuf-compiler                       \
  libprotobuf-dev                         \
  make                                    \
  python                                  \
  python2.7                               \
  libpython-dev                           \
  python-dev                              \
  python-protobuf                         \
  python-setuptools                       \
  libsasl2-modules-gssapi-heimdal         \
  unzip                                   \
  --no-install-recommends

# Install the picojson headers
wget -q https://raw.githubusercontent.com/kazuho/picojson/v1.3.0/picojson.h -O /usr/local/include/picojson.h > /dev/null 2>&1

# Prepare to build Mesos
mkdir -p /usr/share/java/
wget -q http://search.maven.org/remotecontent?filepath=com/google/protobuf/protobuf-java/2.5.0/protobuf-java-2.5.0.jar -O protobuf.jar
mv protobuf.jar /usr/share/java/

MESOS_VERSION=0.28.1
if [ -f "$VAGRANT_PATH/mesos-$MESOS_VERSION.tar.xz" ]; then
  echo "found precompiled mesos archive for Mesos $MESOS_VERSION... reusing it"
  tar -xpf "$VAGRANT_PATH/mesos-$MESOS_VERSION.tar.xz" -C /
  cd /mesos/build
  make -j2 install
else
  echo "precompiled Mesos archive for Mesos $MESOS_VERSION not found... cloning & compiling"
  cd /
  wget -q https://github.com/apache/mesos/archive/$MESOS_VERSION.tar.gz
  tar -xpvf $MESOS_VERSION.tar.gz -C /
  mv mesos-$MESOS_VERSION mesos

  cd /mesos

  # Bootstrap
  ./bootstrap

  # Configure
  mkdir build && cd build && ../configure --disable-java --disable-optimize --without-included-zookeeper --with-glog=/usr/local --with-protobuf=/usr --with-boost=/usr/local
  make -j3 install
  cd /
  tar -Jcpf "$VAGRANT_PATH/mesos-$MESOS_VERSION.tar.xz" /mesos
fi

# Install python eggs
easy_install /mesos/build/src/python/dist/mesos.interface-*.egg
easy_install /mesos/build/src/python/dist/mesos.native-*.egg

cd

### Check out net-modules, compile it and install it

git clone https://github.com/mesosphere/net-modules.git
cd net-modules/isolator
cd isolator
protoc -I /usr/local/include/ -I . --cpp_out=. interface.proto
cd ..
./bootstrap
rm -rf build
mkdir build
cd build
export LD_LIBRARY_PATH=LD_LIBRARY_PATH:/usr/local/lib
../configure --with-mesos=/usr/local --with-protobuf=/usr
make -j3 all
make install
cd
cd net-modules

###

### Set up requirements for net-modules
mkdir /contiv/
cp $VAGRANT_PATH/vagrant/mesos-netmodules/modules.json /contiv/

###

### Set up Mesos & net-modules

# this is hardcoded for now
VM_IP="10.0.2.15"

echo "$VM_IP $(hostname)" >> /etc/hosts

# use scripts to start Mesos master and slave

cat <<EOF > /mesos-master.sh
#!/bin/bash
export MESOS_WORK_DIR=/var/lib/mesos
nohup bash -lc "/mesos/build/bin/mesos-master.sh --zk=zk://localhost:2181/mesos \
--roles=public --quorum=1 --ip=$VM_IP > /tmp/mesos-master.log 2>&1 &"
EOF
chmod +x /mesos-master.sh

cat <<EOF > /mesos-slave.sh
#!/bin/bash
export MESOS_MODULES=file:///contiv/modules.json
export MESOS_ISOLATION=com_mesosphere_mesos_NetworkIsolator
export MESOS_HOOKS=com_mesosphere_mesos_NetworkHook
export MESOS_CONTAINERIZERS=mesos
nohup bash -lc "/mesos/build/bin/mesos-slave.sh --master=zk://localhost:2181/mesos > \
/tmp/mesos-slave.log 2>&1 &"
EOF
chmod +x /mesos-slave.sh

cat <<EOF > /netmaster.sh
#!/bin/bash
nohup bash -lc "/opt/gopath/bin/netmaster -cluster-mode mesos > /tmp/netmaster.log 2>&1 &"
EOF
chmod +x /netmaster.sh

cat <<EOF > /netplugin.sh
#!/bin/bash
nohup bash -lc "/opt/gopath/bin/netplugin -plugin-mode mesos -vlan-if eth2> /tmp/netplugin.log 2>&1 &"
EOF
chmod +x /netplugin.sh

/netplugin.sh
sleep 5
/netmaster.sh
/mesos-master.sh
/mesos-slave.sh

###

### Set up Marathon

MARATHON_VERSION="1.1.1"
marathon_image_path="$VAGRANT_PATH/marathon-$MARATHON_VERSION.tar.xz"
if [ -f $marathon_image_path ]; then
  echo "found saved Marathon image for Marathon $MARATHON_VERSION... reusing it"
  docker load -i $marathon_image_path
else
  echo "saved image for Marathon $MARATHON_VERSION not found... pulling"
  docker pull mesosphere/marathon:v$MARATHON_VERSION
  echo "saving image for Marathon $MARATHON_VERSION"
  docker save mesosphere/marathon:v$MARATHON_VERSION | xz -2e > $marathon_image_path
fi

docker run -d --name marathon -e MARATHON_MASTER=zk://localhost:2181/mesos \
-e MARATHON_ZK=zk://localhost:2181/marathon --net host -e MARATHON_HOSTNAME=10.0.2.15 \
-e MARATHON_MESOS_ROLE=public mesosphere/marathon:v$MARATHON_VERSION --default_accepted_resource_roles "*"

###
