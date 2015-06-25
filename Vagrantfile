# -*- mode: ruby -*-
# vi: set ft=ruby :

netplugin_synced_gopath="/opt/golang"
host_gobin_path="/opt/go/bin"
host_goroot_path="/opt/go/root"

provision_common = <<SCRIPT
## setup the environment file. Export the env-vars passed as args to 'vagrant up'
echo Args passed: [[ $@ ]]
echo 'export GOPATH=#{netplugin_synced_gopath}' > /etc/profile.d/envvar.sh
echo 'export GOBIN=$GOPATH/bin' >> /etc/profile.d/envvar.sh
echo 'export GOSRC=$GOPATH/src' >> /etc/profile.d/envvar.sh
echo 'export GOROOT=#{host_goroot_path}' >> /etc/profile.d/envvar.sh
echo 'export PATH=$PATH:#{host_gobin_path}:$GOBIN' >> /etc/profile.d/envvar.sh
if [ $# -gt 0 ]; then
    echo "export $@" >> /etc/profile.d/envvar.sh
fi

source /etc/profile.d/envvar.sh

## set the mounted host filesystems to be read-only.Just a safety check
## to prevent inadvertent modifications from vm.
(mount -o remount,ro,exec /vagrant) || exit 1
if [ -e #{host_gobin_path} ]; then
    (mount -o remount,ro,exec #{host_gobin_path}) || exit 1
fi
if [ -e #{host_goroot_path} ]; then
    (mount -o remount,ro,exec #{host_goroot_path}) || exit 1
fi
if [ -e #{netplugin_synced_gopath} ]; then
    (mount -o remount,ro,exec #{netplugin_synced_gopath}) || exit 1
fi

### install basic packages
#(apt-get update -qq > /dev/null && apt-get install -y vim curl python-software-properties git > /dev/null) || exit 1
#
### install Go 1.4
#(cd /usr/local/ && \
#curl -L https://storage.googleapis.com/golang/go1.4.linux-amd64.tar.gz -o go1.4.linux-amd64.tar.gz && \
#tar -xzf go1.4.linux-amd64.tar.gz) || exit 1
#
### install etcd
#(cd /tmp && \
#curl -L  https://github.com/coreos/etcd/releases/download/v2.0.0/etcd-v2.0.0-linux-amd64.tar.gz -o etcd-v2.0.0-linux-amd64.tar.gz && \
#tar -xzf etcd-v2.0.0-linux-amd64.tar.gz && \
#mv /tmp/etcd-v2.0.0-linux-amd64/etcd /usr/bin/ && \
#mv /tmp/etcd-v2.0.0-linux-amd64/etcdctl /usr/bin/ ) || exit 1
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
(wget -nv -O ovs-common.deb https://cisco.box.com/shared/static/v1dvgoboo5zgqrtn6tu27vxeqtdo2bdl.deb &&
 wget -nv -O ovs-switch.deb https://cisco.box.com/shared/static/ymbuwvt2qprs4tquextw75b82hyaxwon.deb) || exit 1

(dpkg -i ovs-common.deb &&
 dpkg -i ovs-switch.deb) || exit 1

(ovs-vsctl set-manager tcp:127.0.0.1:6640 && \
 ovs-vsctl set-manager ptcp:6640) || exit 1

### install consul
#(apt-get install -y unzip && cd /tmp && \
# wget https://dl.bintray.com/mitchellh/consul/0.5.2_linux_amd64.zip && \
# unzip 0.5.2_linux_amd64.zip && \
# mv /tmp/consul /usr/bin) || exit 1
SCRIPT

VAGRANTFILE_API_VERSION = "2"
Vagrant.configure(VAGRANTFILE_API_VERSION) do |config|
    config.vm.box = "contiv/ubuntu/v3"
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
        consul_join_flag = if n > 0 then "-join #{node_ips[0]}" else "" end
        consul_bootstrap_flag = "-bootstrap-expect=3"
        if num_nodes < 3 then
            if n == 0 then
                consul_bootstrap_flag = "-bootstrap"
            else
                consul_bootstrap_flag = ""
            end
        end
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
            # godep modifies the host's GOPATH env variable, CONTIV_HOST_GOPATH
            # contains the unmodified path passed from the Makefile, use that
            # when it is defined.
            if ENV['CONTIV_HOST_GOPATH'] != nil
                node.vm.synced_folder ENV['CONTIV_HOST_GOPATH'], netplugin_synced_gopath
            else
                node.vm.synced_folder ENV['GOPATH'], netplugin_synced_gopath
            end
            if ENV['CONTIV_HOST_GOBIN'] != nil
                node.vm.synced_folder ENV['CONTIV_HOST_GOBIN'], host_gobin_path
            end
            if ENV['CONTIV_HOST_GOROOT'] != nil
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

## start consul
(echo && echo consul agent -server #{consul_join_flag} #{consul_bootstrap_flag} \
 -bind=#{node_addr} -data-dir /opt/consul)
(nohup consul agent -server #{consul_join_flag} #{consul_bootstrap_flag} \
 -bind=#{node_addr} -data-dir /opt/consul 0<&- &>/tmp/consul.log &) || exit 1
SCRIPT
            node.vm.provision "shell" do |s|
                s.inline = provision_node
            end
        end
    end
end
