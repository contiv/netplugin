# -*- mode: ruby -*-
# vi: set ft=ruby :

$provision = <<SCRIPT
## install packages
apt-get update -qq && apt-get install -y vim curl python-software-properties git golang openvswitch-switch || exit 1

## setup enviorment. XXX: remove http-proxy stuff
cat > /etc/profile.d/envvar.sh <<'EOF'
export GOPATH=/opt/golang
export GOBIN=$GOPATH/bin
export PATH=$PATH:$GOBIN
export http_proxy=proxy.esl.cisco.com:8080
export https_proxy=$http_proxy
export HTTP_PROXY=$http_proxy
export HTTPS_PROXY=$http_proxy
EOF

. /etc/profile.d/envvar.sh || exit 1

## install and start etcd
( cd /tmp && \
curl -L  https://github.com/coreos/etcd/releases/download/v0.4.6/etcd-v0.4.6-linux-amd64.tar.gz -o etcd-v0.4.6-linux-amd64.tar.gz && \
tar xzvf etcd-v0.4.6-linux-amd64.tar.gz && \
cd /usr/bin && \
ln -s /tmp/etcd-v0.4.6-linux-amd64/etcd && \
ln -s /tmp/etcd-v0.4.6-linux-amd64/etcdctl && \
etcd &) || exit 1

## go get the netplugin repo
go get -u github.com/mapuri/netplugin || exit 1

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
