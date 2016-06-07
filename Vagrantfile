# -*- mode: ruby -*-
# vi: set ft=ruby :

require 'fileutils'

# netplugin_synced_gopath="/opt/golang"
gopath_folder="/opt/gopath"

cluster_ip_nodes = ""

provision_common_once = <<SCRIPT
## setup the environment file. Export the env-vars passed as args to 'vagrant up'
echo Args passed: [[ $@ ]]
echo -n "$1" > /etc/hostname
hostname -F /etc/hostname

echo 'export GOPATH=#{gopath_folder}' > /etc/profile.d/envvar.sh
echo 'export GOBIN=$GOPATH/bin' >> /etc/profile.d/envvar.sh
echo 'export GOSRC=$GOPATH/src' >> /etc/profile.d/envvar.sh
echo 'export PATH=$PATH:/usr/local/go/bin:$GOBIN' >> /etc/profile.d/envvar.sh
echo "export http_proxy='$4'" >> /etc/profile.d/envvar.sh
echo "export https_proxy='$5'" >> /etc/profile.d/envvar.sh
echo "export USE_RELEASE=$6" >> /etc/profile.d/envvar.sh
echo "export no_proxy=$3,127.0.0.1,localhost,netmaster" >> /etc/profile.d/envvar.sh
echo "export CLUSTER_NODE_IPS=$3" >> /etc/profile.d/envvar.sh
source /etc/profile.d/envvar.sh

if [[ $# -gt 9 ]] && [[ $10 != "" ]]; then
    shift; shift; shift; shift; shift; shift; shift; shift; shift
    echo "export $@" >> /etc/profile.d/envvar.sh
fi

# Change ownership for gopath folder
chown vagrant #{gopath_folder}

# Install specific docker version if required
if [[ $8 != "" ]]; then
    echo "Installing docker version " $8
    if [[ $9 == "ubuntu" ]]; then
        curl https://get.docker.com | sed s/docker-engine/docker-engine=$8-0~vivid/ | bash
    else
        # cleanup openstack-kilo repo if required
        yum-config-manager --disable openstack-kilo
        curl https://get.docker.com | sed s/docker-engine/docker-engine-$8/ | bash
    fi
fi
# setup docker cluster store
if [[ $7 == *"consul:"* ]]
then
    cp #{gopath_folder}/src/github.com/contiv/netplugin/scripts/docker.service.consul /lib/systemd/system/docker.service
else
    cp #{gopath_folder}/src/github.com/contiv/netplugin/scripts/docker.service /lib/systemd/system/docker.service
fi
# setup docker remote api
cp #{gopath_folder}/src/github.com/contiv/netplugin/scripts/docker-tcp.socket /etc/systemd/system/docker-tcp.socket
systemctl enable docker-tcp.socket
mkdir /etc/systemd/system/docker.service.d
echo "[Service]" | sudo tee -a /etc/systemd/system/docker.service.d/http-proxy.conf
echo "Environment=\\\"no_proxy=$CLUSTER_NODE_IPS,127.0.0.1,localhost,netmaster\\\" \\\"http_proxy=$http_proxy\\\" \\\"https_proxy=$https_proxy\\\"" | sudo tee -a /etc/systemd/system/docker.service.d/http-proxy.conf
sudo systemctl daemon-reload
sudo systemctl stop docker
systemctl start docker-tcp.socket
sudo systemctl start docker

# remove duplicate docker key
rm /etc/docker/key.json
(service docker restart) || exit 1

docker load --input #{gopath_folder}/src/github.com/contiv/netplugin/scripts/dnscontainer.tar

SCRIPT

provision_common_always = <<SCRIPT
/sbin/ip addr add "$2/24" dev eth1
/sbin/ip link set eth1 up
/sbin/ip link set eth2 up

# Drop cache to workaround vboxsf problem
echo 3 > /proc/sys/vm/drop_caches

# Enable ovs mgmt port
(ovs-vsctl set-manager tcp:127.0.0.1:6640 && \
 ovs-vsctl set-manager ptcp:6640) || exit 1
SCRIPT

provision_gobgp = <<SCRIPT
#Get gobgp binary
wget https://cisco.box.com/shared/static/5leqlo84kjh0thty91ouotilm4ish3nz -q -O #{gopath_folder}/bin/gobgp && chmod +x #{gopath_folder}/bin/gobgp
SCRIPT

provision_bird = <<SCRIPT
## setup the environment file. Export the env-vars passed as args to 'vagrant up'
echo Args passed: [[ $@ ]]
echo "export http_proxy='$1'" >> /etc/profile.d/envvar.sh
echo "export https_proxy='$2'" >> /etc/profile.d/envvar.sh
source /etc/profile.d/envvar.sh
SCRIPT


VAGRANTFILE_API_VERSION = "2"
Vagrant.configure(VAGRANTFILE_API_VERSION) do |config|
    if ENV['CONTIV_NODE_OS'] && ENV['CONTIV_NODE_OS'] == "ubuntu" then
        config.vm.box = "contiv/ubuntu1504-netplugin"
    else
        config.vm.box = "contiv/centos71-netplugin"
    end
    config.vm.provider 'virtualbox' do |v|
        v.linked_clone = true if Vagrant::VERSION =~ /^1.8/
    end

    num_nodes = 2
    if ENV['CONTIV_NODES'] && ENV['CONTIV_NODES'] != "" then
        num_nodes = ENV['CONTIV_NODES'].to_i
    end
    base_ip = "192.168.2."
    if ENV['CONTIV_IP_PREFIX'] && ENV['CONTIV_IP_PREFIX'] != "" then
        base_ip = ENV['CONTIV_IP_PREFIX']
    end
    node_ips = num_nodes.times.collect { |n| base_ip + "#{n+10}" }
    cluster_ip_nodes = node_ips.join(",")

    node_names = num_nodes.times.collect { |n| "netplugin-node#{n+1}" }
    node_peers = []
    if ENV['CONTIV_L3'] then
      config.vm.define "quagga1" do |quagga1|

        quagga1.vm.box = "contiv/quagga1"
        quagga1.vm.host_name = "quagga1"
        quagga1.vm.network :private_network, ip: base_ip + "51", virtualbox__intnet: "true", auto_config: false
        quagga1.vm.network "private_network",
                         ip: "80.1.1.200",
                         virtualbox__intnet: "contiv_orange"
        quagga1.vm.network "private_network",
                         ip: "70.1.1.2",
                         virtualbox__intnet: "contiv_blue"
        quagga1.vm.provision "shell" do |s|
          s.inline = provision_bird
          s.args = [ENV["http_proxy"] || "", ENV["https_proxy"] || ""]
        end
      end
      config.vm.define "quagga2" do |quagga2|

        quagga2.vm.box = "contiv/quagga2"
        quagga2.vm.host_name = "quagga2"
        quagga2.vm.network :private_network, ip: base_ip + "52", virtualbox__intnet: "true", auto_config: false
        quagga2.vm.network "private_network",
                         ip: "70.1.1.1",
                         virtualbox__intnet: "contiv_blue"
        quagga2.vm.network "private_network",
                         ip: "60.1.1.200",
                         virtualbox__intnet: "contiv_green"
        quagga2.vm.network "private_network",
                         ip: "50.1.1.200",
                         virtualbox__intnet: "contiv_yellow"

        quagga2.vm.provision "shell" do |s|
          s.inline = provision_bird
          s.args = [ENV["http_proxy"] || "", ENV["https_proxy"] || ""]
        end
      end
    end

     num_nodes.times do |n|
        node_name = node_names[n]
        node_addr = node_ips[n]
        node_peers += ["#{node_name}=http://#{node_addr}:2380,#{node_name}=http://#{node_addr}:7001"]
        consul_join_flag = if n > 0 then "-join #{node_ips[0]}" else "" end
        consul_bootstrap_flag = "-bootstrap-expect=3"
        swarm_flag = "slave"
        if num_nodes < 3 then
            if n == 0 then
                consul_bootstrap_flag = "-bootstrap"
                swarm_flag = "master"
            else
                consul_bootstrap_flag = ""
                swarm_flag = "slave"
            end
        end
        net_num = (n+1)%3
        if net_num == 0 then
           network_name = "contiv_orange"
        else
           if net_num == 1 then
              network_name = "contiv_yellow"
           else
              network_name = "contiv_green"
           end
        end
        config.vm.define node_name do |node|
            node.vm.box_version = "0.3.1"

            # node.vm.hostname = node_name
            # create an interface for etcd cluster
            node.vm.network :private_network, ip: node_addr, virtualbox__intnet: "true", auto_config: false
            # create an interface for bridged network
            if ENV['CONTIV_L3'] then
              # create an interface for bridged network
              node.vm.network :private_network, ip: "0.0.0.0", virtualbox__intnet: network_name, auto_config: false
            else
              node.vm.network :private_network, ip: "0.0.0.0", virtualbox__intnet: "true", auto_config: false
            end
            node.vm.provider "virtualbox" do |v|
                # make all nics 'virtio' to take benefit of builtin vlan tag
                # support, which otherwise needs to be enabled in Intel drivers,
                # which are used by default by virtualbox
                v.customize ['modifyvm', :id, '--nictype1', 'virtio']
                v.customize ['modifyvm', :id, '--nictype2', 'virtio']
                v.customize ['modifyvm', :id, '--nictype3', 'virtio']
                v.customize ['modifyvm', :id, '--nicpromisc2', 'allow-all']
                v.customize ['modifyvm', :id, '--nicpromisc3', 'allow-all']
                v.customize ['modifyvm', :id, '--paravirtprovider', "kvm"]
            end

            # mount the host directories
            node.vm.synced_folder "bin", File.join(gopath_folder, "bin")
            if ENV["GOPATH"] && ENV['GOPATH'] != ""
              node.vm.synced_folder "../../../", File.join(gopath_folder, "src"), rsync: true
            else
              node.vm.synced_folder ".", File.join(gopath_folder, "src/github.com/contiv/netplugin"), rsync: true
            end

            node.vm.provision "shell" do |s|
                s.inline = "echo '#{node_ips[0]} netmaster' >> /etc/hosts; echo '#{node_ips[1]} netmaster' >> /etc/hosts;	echo '#{node_addr} #{node_name}' >> /etc/hosts"
            end
            node.vm.provision "shell" do |s|
                s.inline = provision_common_once
                s.args = [node_name, node_addr, cluster_ip_nodes, ENV["http_proxy"] || "", ENV["https_proxy"] || "", ENV["USE_RELEASE"] || "", ENV["CONTIV_CLUSTER_STORE"] || "etcd://localhost:2379", ENV["CONTIV_DOCKER_VERSION"] || "", ENV['CONTIV_NODE_OS'] || "", *ENV['CONTIV_ENV']]
            end
            if ENV['CONTIV_L3'] then
                node.vm.provision "shell" do |s|
                    s.inline = provision_gobgp
                end
            end
            node.vm.provision "shell", run: "always" do |s|
                s.inline = provision_common_always
                s.args = [node_name, node_addr]
            end

provision_node = <<SCRIPT
## start etcd with generated config
set -x
(nohup etcd --name #{node_name} --data-dir /tmp/etcd \
 -heartbeat-interval=100 -election-timeout=5000 \
 --listen-client-urls http://0.0.0.0:2379,http://0.0.0.0:4001 \
 --advertise-client-urls http://#{node_addr}:2379,http://#{node_addr}:4001 \
 --initial-advertise-peer-urls http://#{node_addr}:2380,http://#{node_addr}:7001 \
 --listen-peer-urls http://#{node_addr}:2380 \
 --initial-cluster #{node_peers.join(",")} --initial-cluster-state new \
  0<&- &>/tmp/etcd.log &) || exit 1

## start consul
(nohup consul agent -server #{consul_join_flag} #{consul_bootstrap_flag} \
 -bind=#{node_addr} -data-dir /opt/consul 0<&- &>/tmp/consul.log &) || exit 1
# start swarm
(nohup #{gopath_folder}/src/github.com/contiv/netplugin/scripts/start-swarm.sh #{node_addr} #{swarm_flag}> /tmp/start-swarm.log &) || exit 1

SCRIPT
            node.vm.provision "shell", run: "always" do |s|
                s.inline = provision_node
            end

            # forward netmaster port
            if n == 0 then
                node.vm.network "forwarded_port", guest: 9999, host: 9999
            end
        end
    end
end
