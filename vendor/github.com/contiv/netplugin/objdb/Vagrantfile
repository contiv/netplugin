# -*- mode: ruby -*-
# vi: set ft=ruby :

require 'fileutils'

gopath_folder="/opt/gopath"

provision_common = <<SCRIPT
## setup the environment file. Export the env-vars passed as args to 'vagrant up'
echo Args passed: [[ $@ ]]

echo -n "$1" > /etc/hostname
hostname -F /etc/hostname

/sbin/ip addr add "$3/24" dev eth1
/sbin/ip link set eth1 up
/sbin/ip link set eth2 up

echo 'export GOPATH=#{gopath_folder}' > /etc/profile.d/envvar.sh
echo 'export GOBIN=$GOPATH/bin' >> /etc/profile.d/envvar.sh
echo 'export GOSRC=$GOPATH/src' >> /etc/profile.d/envvar.sh
echo 'export PATH=$PATH:/usr/local/go/bin:$GOBIN' >> /etc/profile.d/envvar.sh
echo "export http_proxy='$4'" >> /etc/profile.d/envvar.sh
echo "export https_proxy='$5'" >> /etc/profile.d/envvar.sh
echo "export no_proxy=192.168.2.10,192.168.2.11,127.0.0.1,localhost,netmaster" >> /etc/profile.d/envvar.sh
echo "export CLUSTER_NODE_IPS=192.168.2.10,192.168.2.11" >> /etc/profile.d/envvar.sh


source /etc/profile.d/envvar.sh

# Change permissions for gopath folder
chown -R vagrant #{gopath_folder}

SCRIPT

VAGRANTFILE_API_VERSION = "2"
Vagrant.configure(VAGRANTFILE_API_VERSION) do |config|
    config.vm.box = "contiv/centos73"
    config.vm.box_version = "0.10.0"
    num_nodes = 1
    base_ip = "192.168.10."
    node_ips = num_nodes.times.collect { |n| base_ip + "#{n+10}" }
    node_names = num_nodes.times.collect { |n| "node#{n+1}" }
    node_peers = []

    num_nodes.times do |n|
        node_name = node_names[n]
        node_addr = node_ips[n]
        node_peers += ["#{node_name}=http://#{node_addr}:2380,#{node_name}=http://#{node_addr}:7001"]
        consul_bootstrap_flag = "-bootstrap"
        config.vm.define node_name do |node|
            # node.vm.hostname = node_name
            # create an interface for etcd cluster
            node.vm.network :private_network, ip: node_addr, virtualbox__intnet: "true", auto_config: false
            # create an interface for bridged network
            node.vm.network :private_network, ip: "0.0.0.0", virtualbox__intnet: "true", auto_config: false
            node.vm.provider "virtualbox" do |v|
                # make all nics 'virtio' to take benefit of builtin vlan tag
                # support, which otherwise needs to be enabled in Intel drivers,
                # which are used by default by virtualbox
                v.customize ['modifyvm', :id, '--nictype1', 'virtio']
                v.customize ['modifyvm', :id, '--nictype2', 'virtio']
                v.customize ['modifyvm', :id, '--nictype3', 'virtio']
                v.customize ['modifyvm', :id, '--nicpromisc2', 'allow-all']
                v.customize ['modifyvm', :id, '--nicpromisc3', 'allow-all']
            end

            # mount the host directories
            node.vm.synced_folder ".", File.join(gopath_folder, "src/github.com/contiv/objdb"), rsync: true


            node.vm.provision "shell" do |s|
                s.inline = provision_common
                s.args = [node_name, ENV["CONTIV_NODE_OS"] || "", node_addr, ENV["http_proxy"] || "", ENV["https_proxy"] || "", *ENV['CONTIV_ENV']]
            end
provision_node = <<SCRIPT
## start etcd with generated config
set -x
(nohup etcd --name #{node_name} --data-dir /tmp/etcd \
 --listen-client-urls http://0.0.0.0:2379,http://0.0.0.0:4001 \
 --advertise-client-urls http://#{node_addr}:2379,http://#{node_addr}:4001 \
 --initial-advertise-peer-urls http://#{node_addr}:2380,http://#{node_addr}:7001 \
 --listen-peer-urls http://#{node_addr}:2380 \
 --initial-cluster #{node_peers.join(",")} --initial-cluster-state new \
  0<&- &>/tmp/etcd.log &) || exit 1

## start consul
(nohup consul agent -server #{consul_bootstrap_flag} \
 -bind=#{node_addr} -data-dir /opt/consul 0<&- &>/tmp/consul.log &) || exit 1

SCRIPT
            node.vm.provision "shell", run: "always" do |s|
                s.inline = provision_node
            end
        end
    end
end
