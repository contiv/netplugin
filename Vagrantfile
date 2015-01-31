# -*- mode: ruby -*-
# vi: set ft=ruby :

$vagrant_env = ENV['VAGRANT_ENV']
$provision = <<SCRIPT
## setup the environment file. Export the env-vars passed as args to 'vagrant up'
echo Args passed: [[ $@ ]]
echo 'export GOPATH=/opt/golang' > /etc/profile.d/envvar.sh
echo 'export GOBIN=$GOPATH/bin' >> /etc/profile.d/envvar.sh
echo 'export GOSRC=$GOPATH/src' >> /etc/profile.d/envvar.sh
echo 'export PATH=$PATH:/usr/local/go/bin:$GOBIN' >> /etc/profile.d/envvar.sh
if [ $# -gt 0 ]; then
    echo "export $@" >> /etc/profile.d/envvar.sh
fi

## source the environment
. /etc/profile.d/envvar.sh || exit 1

## install basic packages
(apt-get update -qq > /dev/null && apt-get install -y vim curl python-software-properties git > /dev/null) || exit 1

## install Go 1.4
(cd /usr/local/ && \
curl -L https://storage.googleapis.com/golang/go1.4.linux-amd64.tar.gz -o go1.4.linux-amd64.tar.gz && \
tar -xzf go1.4.linux-amd64.tar.gz) || exit 1

## install and start etcd
(cd /tmp && \
curl -L  https://github.com/coreos/etcd/releases/download/v0.4.6/etcd-v0.4.6-linux-amd64.tar.gz -o etcd-v0.4.6-linux-amd64.tar.gz && \
tar -xzf etcd-v0.4.6-linux-amd64.tar.gz && \
cd /usr/bin && \
ln -s /tmp/etcd-v0.4.6-linux-amd64/etcd && \
ln -s /tmp/etcd-v0.4.6-linux-amd64/etcdctl && \
etcd > /dev/null &) || exit 1

## install and start docker
(curl -sSL https://get.docker.com/ubuntu/ | sh > /dev/null) || exit 1

## link the netplugin repo, for a quick test-fix-test turn-around
(mkdir -p $GOSRC/github.com/contiv && \
ln -s /vagrant $GOSRC/github.com/contiv/netplugin) || exit 1

## install openvswitch and enable ovsdb-server to listen for incoming requests
(apt-get install -y openvswitch-switch > /dev/null && \
ovs-vsctl set-manager tcp:127.0.0.1:6640 && \
ovs-vsctl set-manager ptcp:6640) || exit 1

SCRIPT

VAGRANTFILE_API_VERSION = "2"
Vagrant.configure(VAGRANTFILE_API_VERSION) do |config|
config.vm.box = "ubuntu/trusty64"
config.vm.hostname = "netplugin"
config.vm.network :private_network, ip: "10.0.2.88"
config.vm.provider "virtualbox" do |v|
v.customize ['modifyvm', :id, '--nicpromisc1', 'allow-all']
end
config.vm.provision "shell" do |s|
    s.inline = $provision
    s.args = $vagrant_env
end
end
