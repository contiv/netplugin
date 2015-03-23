# -*- mode: ruby -*-
# vi: set ft=ruby :

netplugin_synced_gopath = "/opt/golang"
host_gobin_path = ENV['CONTIV_HOST_GOBIN']
host_goroot_path = ENV['CONTIV_HOST_GOROOT']

provision_common = <<SCRIPT
## setup the environment file. Export the env-vars passed as args to 'vagrant up'
echo Args passed: [[ $@ ]]
echo 'export GOPATH=#{netplugin_synced_gopath}' > /etc/profile.d/envvar.sh
echo 'export GOBIN=$GOPATH/bin' >> /etc/profile.d/envvar.sh
echo 'export GOSRC=$GOPATH/src' >> /etc/profile.d/envvar.sh
if [ -n "#{host_goroot_path}" ]
then
  echo 'export GOROOT=/usr/local/go' >> /etc/profile.d/envvar.sh
else
  echo 'export GOROOT=#{host_goroot_path}' >> /etc/profile.d/envvar.sh
fi

echo 'export PATH=/usr/local/bin:$PATH:#{host_gobin_path}:$GOBIN' >> /etc/profile.d/envvar.sh
if [ $# -gt 0 ]; then
    echo "export $@" >> /etc/profile.d/envvar.sh
fi

## set the mounted host filesystems to be read-only.Just a safety check
## to prevent inadvertent modifications from vm.
if [ ! -n "#{host_gobin_path}" ] 
then
  (mount -o remount,ro,exec /vagrant) || exit 1
fi

if [ -e "#{host_gobin_path}" ]; then
    (mount -o remount,ro,exec #{host_gobin_path}) || exit 1
fi
if [ -e "#{host_goroot_path}" ]; then
    (mount -o remount,ro,exec #{host_goroot_path}) || exit 1
fi

if [ ! -n "#{host_gobin_path}" ] && [ -e "#{netplugin_synced_gopath}" ]
then
    (mount -o remount,ro,exec #{netplugin_synced_gopath}) || exit 1
fi

### install basic packages
#(apt-get update -qq > /dev/null && apt-get install -y vim curl python-software-properties git > /dev/null) || exit 1
#
### install Go 1.4
if [ ! -n "#{host_gobin_path}" ]
then
  cd /usr/local && curl -sSL https://storage.googleapis.com/golang/go1.4.linux-amd64.tar.gz | tar vxz
  for i in /usr/local/go/bin/*
  do
    ln -sf $i /usr/local/bin/$(basename $i)
  done
fi

### install etcd
#(cd /tmp && \
#curl -L  https://github.com/coreos/etcd/releases/download/v2.0.0/etcd-v2.0.0-linux-amd64.tar.gz -o etcd-v2.0.0-linux-amd64.tar.gz && \
#tar -xzf etcd-v2.0.0-linux-amd64.tar.gz && \
#cd /usr/bin && \
#ln -s /tmp/etcd-v2.0.0-linux-amd64/etcd && \
#ln -s /tmp/etcd-v2.0.0-linux-amd64/etcdctl) || exit 1
#
### install and start docker
#(curl -sSL https://get.docker.com/ubuntu/ | sh > /dev/null) || exit 1
#
## pass the env-var args to docker and restart the service. This helps passing
## stuff like http-proxy etc
if [ $# -gt 0 ]; then
    (echo "export $@" >> /etc/default/docker && \
     service docker restart) || exit 1
fi

## install openvswitch and enable ovsdb-server to listen for incoming requests
#(apt-get install -y openvswitch-switch > /dev/null) || exit 1
(ovs-vsctl set-manager tcp:127.0.0.1:6640 && \
 ovs-vsctl set-manager ptcp:6640) || exit 1
SCRIPT

VAGRANTFILE_API_VERSION = "2"
Vagrant.configure(VAGRANTFILE_API_VERSION) do |config|
    config.vm.box = "contiv/ubuntu"
    # XXX: need a public url
    config.vm.box_url = "https://cisco.box.com/shared/static/27u8utb1em5730rzprhr5szeuv2p0wir.box"
    num_nodes = 1
    if ENV['CONTIV_NODES'] && ENV['CONTIV_NODES'] != "" then
        num_nodes = ENV['CONTIV_NODES'].to_i
    end
    base_ip = "192.168.2."
    node_ips = num_nodes.times.collect { |n| base_ip + "#{n+10}" }
    node_names = num_nodes.times.collect { |n| "netplugin-node#{n+1}" } 
    num_nodes.times do |n|
        node_name = node_names[n]
        node_addr = node_ips[n]
        node_peers = ""
        node_ips.length.times { |i| node_peers += "#{node_names[i]}=http://#{node_ips[i]}:2380 "}
        node_peers = node_peers.strip().gsub(' ', ',')
        config.vm.define node_name do |node|
            node.vm.hostname = node_name
            # create an interface for etcd cluster
            node.vm.network :private_network, ip: node_addr, virtualbox__intnet: "true"
            # create an interface for bridged network
            node.vm.network :private_network, ip: "0.0.0.0", virtualbox__intnet: "true"
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
            node.vm.synced_folder ".", "/vagrant"
            node.vm.synced_folder ENV['GOPATH'], netplugin_synced_gopath
            if ENV['CONTIV_HOST_GOBIN'] != nil && ENV["CONTIV_HOST_GOBIN"] != ""
                node.vm.synced_folder ENV['CONTIV_HOST_GOBIN'], host_gobin_path
            end
            if ENV['CONTIV_HOST_GOROOT'] != nil && ENV['CONTIV_HOST_GOROOT'] != ""
                node.vm.synced_folder ENV['CONTIV_HOST_GOROOT'], host_goroot_path
            end
            node.vm.provision "shell" do |s|
                s.inline = provision_common
                s.args = ENV['CONTIV_ENV']
            end
provision_node = <<SCRIPT
## start etcd with generated config
(echo etcd -name #{node_name} -data-dir /opt/etcd \
 -listen-client-urls http://0.0.0.0:2379,http://0.0.0.0:4001 \
 -advertise-client-urls http://#{node_addr}:2379,http://#{node_addr}:4001 \
 -initial-advertise-peer-urls http://#{node_addr}:2380 \
 -listen-peer-urls http://#{node_addr}:2380 \
 -initial-cluster #{node_peers} \
 -initial-cluster-state new)
(nohup etcd -name #{node_name} -data-dir /opt/etcd \
 -listen-client-urls http://0.0.0.0:2379,http://0.0.0.0:4001 \
 -advertise-client-urls http://#{node_addr}:2379,http://#{node_addr}:4001 \
 -initial-advertise-peer-urls http://#{node_addr}:2380 \
 -listen-peer-urls http://#{node_addr}:2380 \
 -initial-cluster #{node_peers} \
 -initial-cluster-state new 0<&- &>/tmp/etcd.log &) || exit 1
 cd /opt/golang/src/github.com/contiv/netplugin
 GOPATH=/opt/golang make build
SCRIPT
            node.vm.provision "shell" do |s|
                s.inline = provision_node
            end
        end
    end
end
