# -*- mode: ruby -*-
# vi: set ft=ruby :

provision_common = <<SCRIPT
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

## install etcd
(cd /tmp && \
curl -L  https://github.com/coreos/etcd/releases/download/v2.0.0/etcd-v2.0.0-linux-amd64.tar.gz -o etcd-v2.0.0-linux-amd64.tar.gz && \
tar xzvf etcd-v2.0.0-linux-amd64.tar.gz && \
cd /usr/bin && \
ln -s /tmp/etcd-v2.0.0-linux-amd64/etcd && \
ln -s /tmp/etcd-v2.0.0-linux-amd64/etcdctl && \
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
    num_nodes = ( ENV['CONTIV_NODES'] || 1).to_i
    base_ip = "192.168.2."
    node_ips = num_nodes.times.collect { |n| base_ip + "#{n+10}" }
    num_nodes.times do |n|
        node_name = "netplugin-node#{n+1}"
        node_addr = node_ips[n]
        # Form node's etcd peers. If it's the first or only node we let it's peer list to be empty for it to bootstrap the cluster
        node_peers = ""
        node_ips.each { |ip| if ip != node_addr  && n != 0 then node_peers += "\"#{ip}:7001\" "  end}
        node_peers = node_peers.strip().gsub(' ', ',')
        config.vm.define node_name do |node|
            node.vm.hostname = node_name
            node.vm.network :private_network, ip: node_addr, virtualbox__intnet: "true"
            node.vm.provider "virtualbox" do |v|
                v.customize ['modifyvm', :id, '--nicpromisc2', 'allow-all']
            end
            node.vm.provision "shell" do |s|
                s.inline = provision_common
                s.args = ENV['CONTIV_ENV']
            end
provision_node = <<SCRIPT
## prepare node's etcd config
echo 'addr = "#{node_addr}:4001"' > /etc/profile.d/etcd.conf
echo 'bind_addr = "0.0.0.0:4001"' >> /etc/profile.d/etcd.conf
echo 'peers = [#{node_peers}]' >> /etc/profile.d/etcd.conf
echo 'name = "#{node_name}"' >> /etc/profile.d/etcd.conf
echo 'verbose = false' >> /etc/profile.d/etcd.conf
echo '[peer]' >> /etc/profile.d/etcd.conf
echo 'addr = "#{node_addr}:7001"' >> /etc/profile.d/etcd.conf

## start etcd with generated config
(etcd -config=/etc/profile.d/etcd.conf &) || exit 1
SCRIPT
            node.vm.provision "shell" do |s|
                s.inline = provision_node
            end
        end
    end
end
