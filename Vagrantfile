# -*- mode: ruby -*-
# vi: set ft=ruby :

$provision = <<SCRIPT
## install packages
(apt-get update -qq && apt-get install -y vim curl python-software-properties git openvswitch-switch) || exit 1

## setup enviorment. XXX: remove http-proxy stuff
cat > /etc/profile.d/envvar.sh <<'EOF'
export GOPATH=/opt/golang
export GOBIN=$GOPATH/bin
export GOSRC=$GOPATH/src
export PATH=$PATH:/usr/local/go/bin:$GOBIN
export http_proxy=proxy.esl.cisco.com:8080
export https_proxy=$http_proxy
export HTTP_PROXY=$http_proxy
export HTTPS_PROXY=$http_proxy
EOF

. /etc/profile.d/envvar.sh || exit 1

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
etcd &) || exit 1

## install and start docker
curl -sSL https://get.docker.com/ubuntu/ | sh

## link the netplugin repo, for quick test-fix-test turnaround
(mkdir -p $GOSRC/github.com/contiv && \
sudo ln -s /vagrant $GOSRC/github.com/contiv/netplugin) || exit 1

##enable ovsdb-server to listen for incoming requests
(ovs-vsctl set-manager tcp:127.0.0.1:6640 && \
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
config.vm.provision "shell", inline: $provision
end
