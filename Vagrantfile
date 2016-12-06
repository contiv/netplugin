# -*- mode: ruby -*-
# vi: set ft=ruby :

require 'fileutils'

BEGIN {
  STATEFILE = ".vagrant-state"

  if File.exist?(STATEFILE)
    File.open(STATEFILE).read.lines.map { |x| x.split("=", 2) }.each { |x,y| ENV[x] = y }
  end
}

# netplugin_synced_gopath="/opt/golang"
go_version = ENV["GO_VERSION"] || "1.7.3"
docker_version = ENV["CONTIV_DOCKER_VERSION"] || "1.12.3"
gopath_folder="/opt/gopath"

cluster_ip_nodes = ""

MEMORY = "4096"

provision_common_once = <<SCRIPT
sudo sed -i.bak -e 's/^MaxStartups.*$/MaxStartups 1000:100:1000/g' /etc/ssh/sshd_config
sudo systemctl restart sshd

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
echo "export CONTIV_CLUSTER_STORE=$7" >> /etc/profile.d/envvar.sh
source /etc/profile.d/envvar.sh

rm -rf /usr/local/go

curl -sSL https://storage.googleapis.com/golang/go#{go_version}.linux-amd64.tar.gz  | sudo tar -xz -C /usr/local

if [[ $# -gt 9 ]] && [[ $10 != "" ]]; then
    shift; shift; shift; shift; shift; shift; shift; shift; shift
    echo "export $@" >> /etc/profile.d/envvar.sh
fi

# Change ownership for gopath folder
chown -R vagrant #{gopath_folder}

# Install specific docker version if required
echo "Cleaning docker up to reinstall"
service docker stop || :
rm -rf /var/lib/docker
echo "Installing docker version " $8
if [[ $9 == "ubuntu" ]]; then
    sudo apt-get purge docker-engine -y || :
    curl https://get.docker.com | sed s/docker-engine/docker-engine=#{docker_version}-0~xenial/g | bash
else
    # cleanup openstack-kilo repo if required
    yum remove docker-engine -y || :
    yum-config-manager --disable openstack-kilo
    curl https://get.docker.com | sed s/docker-engine/docker-engine-#{docker_version}/ | bash
fi

# setup docker cluster store
if [[ $7 == *"consul:"* ]]
then
    perl -i -lpe 's!^ExecStart(.+)$!ExecStart$1 --cluster-store=consul://localhost:8500!' /lib/systemd/system/docker.service
else
    perl -i -lpe 's!^ExecStart(.+)$!ExecStart$1 --cluster-store=etcd://localhost:2379!' /lib/systemd/system/docker.service
fi

# setup docker remote api
mkdir /etc/systemd/system/docker.service.d
echo "[Service]" | sudo tee -a /etc/systemd/system/docker.service.d/http-proxy.conf
echo "Environment=\\\"no_proxy=$CLUSTER_NODE_IPS,127.0.0.1,localhost,netmaster\\\" \\\"http_proxy=$http_proxy\\\" \\\"https_proxy=$https_proxy\\\"" | sudo tee -a /etc/systemd/system/docker.service.d/http-proxy.conf
sudo systemctl daemon-reload
sudo systemctl stop docker
sudo systemctl start docker

# remove duplicate docker key
rm /etc/docker/key.json
(service docker restart) || exit 1

usermod -aG docker vagrant
docker load --input #{gopath_folder}/src/github.com/contiv/netplugin/scripts/dnscontainer.tar || echo "Loading skydns container failed"

SCRIPT

provision_common_always = <<SCRIPT
/sbin/ip addr add "$2/24" dev eth1
/sbin/ip link set eth1 up
/sbin/ip link set eth2 up

# Drop cache to workaround vboxsf problem
echo 3 > /proc/sys/vm/drop_caches

# start docker daemon
systemctl start docker

# Start OVS if required
systemctl start openvswitch

# Enable ovs mgmt port
(ovs-vsctl set-manager tcp:127.0.0.1:6640 && \
 ovs-vsctl set-manager ptcp:6640) || exit 1
SCRIPT

provision_bird = <<SCRIPT
## setup the environment file. Export the env-vars passed as args to 'vagrant up'
echo Args passed: [[ $@ ]]
echo "export http_proxy='$1'" >> /etc/profile.d/envvar.sh
echo "export https_proxy='$2'" >> /etc/profile.d/envvar.sh
source /etc/profile.d/envvar.sh
SCRIPT

module VagrantPlugins
  module EnvState
    class Plugin < Vagrant.plugin('2')
      name 'EnvState'

      description <<-DESC
      Environment State tracker; saves the environment at `vagrant up` time and
      restores it for all other commands, and removes it at `vagrant destroy`
      time.
      DESC

      def self.up_hook(arg)
        unless File.exist?(STATEFILE) # prevent it from writing more than once.
          f = File.open(STATEFILE, "w") 
          ENV.each do |x,y|
            f.puts "%s=%s" % [x,y]
          end
          f.close
        end
      end

      def self.destroy_hook(arg)
        if File.exist?(STATEFILE) # prevent it from trying to delete more than once.
          File.unlink(STATEFILE)
        end
      end

      action_hook(:EnvState, :machine_action_up) do |hook|
        hook.prepend(method(:up_hook))
      end

      action_hook(:EnvState, :machine_action_destroy) do |hook|
        hook.prepend(method(:destroy_hook))
      end
    end
  end
end


VAGRANTFILE_API_VERSION = "2"
Vagrant.configure(VAGRANTFILE_API_VERSION) do |config|

    if ENV['CONTIV_NODE_OS'] && ENV['CONTIV_NODE_OS'] == "ubuntu" then
        config.vm.box = "contiv/ubuntu1604-netplugin"
        config.vm.box_version = "0.7.0"
    else
        config.vm.box = "contiv/centos72"
        config.vm.box_version = "0.7.0"
    end
    config.vm.provider 'virtualbox' do |v|
        v.linked_clone = true if Vagrant::VERSION =~ /^1.8/
    end

    num_nodes = 3
    if ENV['CONTIV_NODES'] && ENV['CONTIV_NODES'] != "" then
        num_nodes = ENV['CONTIV_NODES'].to_i
    end
    base_ip = "192.168.2."
    if ENV['CONTIV_IP_PREFIX'] && ENV['CONTIV_IP_PREFIX'] != "" then
        base_ip = ENV['CONTIV_IP_PREFIX']
    end
    node_ips = num_nodes.times.collect { |n| base_ip + "#{n+10}" }
    cluster_ip_nodes = node_ips.join(",")

    config.ssh.insert_key = false
    node_names = num_nodes.times.collect { |n| "netplugin-node#{n+1}" }
    node_peers = []
    if ENV['CONTIV_L3'] then
      config.vm.define "quagga1" do |quagga1|

        quagga1.vm.box = "contiv/quagga1"
        quagga1.vm.box_version = "0.0.1"
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
        quagga2.vm.box_version = "0.0.1"
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
        if num_nodes < 3 then
          if n == 0 then
            consul_bootstrap_flag = "-bootstrap"
          else
            consul_bootstrap_flag = ""
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
                v.customize ['modifyvm', :id, '--memory', MEMORY]
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
                s.args = [
                  node_name,
                  node_addr,
                  cluster_ip_nodes,
                  ENV["http_proxy"] || "",
                  ENV["https_proxy"] || "",
                  ENV["USE_RELEASE"] || "",
                  ENV["CONTIV_CLUSTER_STORE"] || "etcd://localhost:2379",
                  docker_version,
                  ENV['CONTIV_NODE_OS'] || "",
                  *ENV['CONTIV_ENV'],
                ]
            end
            node.vm.provision "shell", run: "always" do |s|
                s.inline = provision_common_always
                s.args = [node_name, node_addr]
            end

provision_node = <<SCRIPT
set -x

## start etcd with generated config
echo "#!/bin/bash" > /usr/bin/etcd.sh
echo "etcd --name #{node_name} --data-dir /var/lib/etcd \
 -heartbeat-interval=100 -election-timeout=5000 \
 --listen-client-urls http://0.0.0.0:2379,http://0.0.0.0:4001 \
 --advertise-client-urls http://#{node_addr}:2379,http://#{node_addr}:4001 \
 --initial-advertise-peer-urls http://#{node_addr}:2380,http://#{node_addr}:7001 \
 --listen-peer-urls http://#{node_addr}:2380 \
 --initial-cluster #{node_peers.join(",")} --initial-cluster-state new" >> /usr/bin/etcd.sh

chmod +x /usr/bin/etcd.sh
cp #{gopath_folder}/src/github.com/contiv/netplugin/scripts/etcd.service /etc/systemd/system/etcd.service

## start consul
echo "#!/bin/bash" > /usr/bin/consul.sh
echo "consul agent -server #{consul_join_flag} #{consul_bootstrap_flag} \
 -bind=#{node_addr} -data-dir /opt/consul" >> /usr/bin/consul.sh

 chmod +x /usr/bin/consul.sh
cp #{gopath_folder}/src/github.com/contiv/netplugin/scripts/consul.service /etc/systemd/system/consul.service

systemctl daemon-reload || exit 1
systemctl enable etcd || exit 1
systemctl enable consul || exit 1
systemctl start etcd || exit 1
systemctl start consul || exit 1

SCRIPT
            node.vm.provision "shell" do |s|
                s.inline = provision_node
            end

            # forward netmaster port
            if n == 0 then
                node.vm.network "forwarded_port", guest: 9999, host: 9999
                node.vm.network "forwarded_port", guest: 80, host: 9998
            end
        end
    end
end
