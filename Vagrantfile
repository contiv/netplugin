# -*- mode: ruby -*-
# vi: set ft=ruby :

require File.join(File.dirname(__FILE__), "vagrant", "vagrant_version_check")

require 'fileutils'

BEGIN {
  STATEFILE = ".vagrant-state"

  # if there's a state file, set all the envvars in the current environment
  if File.exist?(STATEFILE)
    File.read(STATEFILE).lines.map { |x| x.split("=", 2) }.each { |x,y| ENV[x] = y.strip }
  end
}

# netplugin_synced_gopath="/opt/golang"
go_version = ENV['GO_VERSION'] || '1.7.6'
docker_version = ENV['CONTIV_DOCKER_VERSION'] || '1.12.6'
docker_swarm = ENV['CONTIV_DOCKER_SWARM'] || 'classic_mode'
docker_ee_url = ENV['DOCKERURL']
gopath_folder = '/opt/gopath'
http_proxy = ENV['HTTP_PROXY'] || ENV['http_proxy'] || ''
https_proxy = ENV['HTTPS_PROXY'] || ENV['https_proxy'] || ''
build_version = ENV['BUILD_VERSION'] || ''
cluster_ip_nodes = ''
v2plugin_name = ENV['CONTIV_V2PLUGIN_NAME'] || 'contiv/v2netplugin:0.1'
cluster_store_driver = ENV['CONTIV_CLUSTER_STORE_DRIVER'] || 'etcd'
cluster_store_url = ENV['CONTIV_CLUSTER_STORE_URL'] || 'http://localhost:2379'
nightly_release = ENV['NIGHTLY_RELEASE'] || ''
node_os = ENV['CONTIV_NODE_OS'] != '' ? ENV['CONTIV_NODE_OS'] : 'centos'
base_ip = ENV['CONTIV_IP_PREFIX'] || '192.168.2.'
num_nodes = ENV['CONTIV_NODES'].to_i == 0 ? 3 : ENV['CONTIV_NODES'].to_i
num_vm_cpus = (ENV['CONTIV_CPUS'] || 4).to_i
vm_memory = (ENV['CONTIV_MEMORY'] || 2048).to_i
legacy_docker = docker_version >= "17.03" ? 0 : 1

provision_common_once = <<SCRIPT
## setup the environment file. Export the env-vars passed as args to 'vagrant up'
echo Args passed: [[ $@ ]]
cat >>/etc/profile.d/envvar.sh <<EOF
export GOPATH=#{gopath_folder}
export GOBIN=\\\$GOPATH/bin
export GOSRC=\\\$GOPATH/src
export PATH=\\\$PATH:/usr/local/go/bin:\\\$GOBIN
export http_proxy='#{http_proxy}'
export https_proxy='#{https_proxy}'
export NIGHTLY_RELEASE=#{nightly_release}
export no_proxy=%{cluster_ip_nodes},127.0.0.1,localhost,netmaster
export CLUSTER_NODE_IPS=%{cluster_ip_nodes}
export CONTIV_CLUSTER_STORE_DRIVER=#{cluster_store_driver}
export CONTIV_CLUSTER_STORE_URL=#{cluster_store_url}
export CONTIV_V2PLUGIN_NAME=#{v2plugin_name}
export CONTIV_DOCKER_SWARM=#{docker_swarm}
export BUILD_VERSION=#{build_version}
EOF
source /etc/profile.d/envvar.sh

installed_go=$(go version | awk '{ print $3}')
if [ "$installed_go" == "go#{go_version}" ]; then
    echo "The installed Go version is already #{go_version}."
    echo "Skipping Go reinstall"
else
    echo "The installed Go version is $installed_go"
    echo "Uninstalling Go $installed_go & installing Go #{go_version}"
    rm -rf /usr/local/go

    curl -sSL https://storage.googleapis.com/golang/go#{go_version}.linux-amd64.tar.gz  | sudo tar -xz -C /usr/local
fi

# Change ownership for gopath folder
chown -R vagrant #{gopath_folder}

if [[ "#{node_os}" != "ubuntu" ]]; then
    systemctl disable NetworkManager.service
    systemctl stop NetworkManager.service
    # Remove the unneeded ceph repository if it exists
    echo "Remove the unneeded ceph repository if it exists"
    rm -f /etc/yum.repos.d/ceph.repo
    yum clean all
fi

# Install specific docker version if required
installed_docker=$(sudo docker version | grep Version | tail -1 | awk '{ print $2 }')
if [[ "$installed_docker" != "#{docker_version}" ]]; then
    reinstall=1
    echo "Installing Docker #{docker_version}"
else
    echo "Docker #{docker_version} is already installed"
fi

echo "Cleaning up the Docker directory"
service docker stop || :
rm -rf /var/lib/docker

if [[ "#{node_os}" == "ubuntu" ]] && [[ "$reinstall" -eq 1 ]]; then
    sudo apt-get purge docker-engine -y || :
    curl https://get.docker.com | sed s/docker-engine/docker-engine=#{docker_version}-0~xenial/g | bash
elif [[ "#{node_os}" == "centos" ]] && [[ -n "#{docker_ee_url}" ]]; then
    echo "Preparing for Docker EE installation"
    sudo yum remove -y docker docker-common docker-selinux docker-engine-selinux docker-engine docker-ce || :
    sudo rm /etc/yum.repos.d/*docker*
    export DOCKERURL='#{docker_ee_url}'
    sudo -E sh -c 'echo "$DOCKERURL/centos" > /etc/yum/vars/dockerurl'
    sudo -E yum-config-manager --add-repo "$DOCKERURL/centos/docker-ee.repo"
    echo "Installing Docker EE #{docker_version}"
    sudo yum -y install docker-ee-#{docker_version}
    sudo yum install -y yum-utils device-mapper-persistent-data lvm2
elif [[ "$reinstall" -eq 1 ]] && [[ "#{legacy_docker}" -eq 1 ]]; then
    # cleanup openstack-kilo repo if required
    yum remove docker-engine -y || :
    yum-config-manager --disable openstack-kilo
    if [[ #{docker_version} == *"rc"* ]]; then
        echo "Getting pre-release docker version #{docker_version} "
        curl -fsSL https://test.docker.com/ | sh
    else
        echo "Getting released docker version #{docker_version} "
        curl https://get.docker.com | sed s/docker-engine/docker-engine-#{docker_version}/ | bash
    fi
elif [[ "$reinstall" -eq 1 ]]; then
    echo "Installing Docker CE #{docker_version}"
    yum remove docker-engine -y || :
    yum remove docker-ce || :
    yum-config-manager --disable openstack-kilo
    yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
    yum-config-manager --enable docker-ce-stable
    yum makecache fast
    yum install -y docker-ce-#{docker_version}
fi

# setup docker cluster store
# No cluster store needed for swarm_mode
if [[ #{docker_swarm} == "swarm_mode" ]]; then
    perl -i -lpe 's!^ExecStart(.+)$!ExecStart$1 !' /lib/systemd/system/docker.service
else
    if [[ "$CONTIV_CLUSTER_STORE_DRIVER" == "consul" ]]
    then
        perl -i -lpe 's!^ExecStart(.+)$!ExecStart$1 -H tcp://0.0.0.0:2375 -H unix:///var/run/docker.sock --cluster-store=consul://localhost:8500!' /lib/systemd/system/docker.service
    else
        perl -i -lpe 's!^ExecStart(.+)$!ExecStart$1 -H tcp://0.0.0.0:2375 -H unix:///var/run/docker.sock --cluster-store=etcd://localhost:2379!' /lib/systemd/system/docker.service
    fi
fi

# setup docker remote api
mkdir -p /etc/systemd/system/docker.service.d
echo "[Service]" | sudo tee -a /etc/systemd/system/docker.service.d/http-proxy.conf
echo "Environment=\\\"no_proxy=$CLUSTER_NODE_IPS,127.0.0.1,localhost,netmaster\\\" \\\"http_proxy=$http_proxy\\\" \\\"https_proxy=$https_proxy\\\"" | sudo tee -a /etc/systemd/system/docker.service.d/http-proxy.conf
sudo systemctl daemon-reload
sudo systemctl stop docker
sudo systemctl start docker

# remove duplicate docker key
rm /etc/docker/key.json
(service docker restart) || exit 1

usermod -aG docker vagrant
SCRIPT

provision_common_always = <<SCRIPT
/sbin/ip addr add "$1/24" dev eth1
/sbin/ip link set eth1 up
/sbin/ip link set eth2 up
/sbin/ip link set eth3 up

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

provision_node = <<SCRIPT
set -x

## start etcd with generated config
echo "#!/bin/bash" > /usr/bin/etcd.sh
echo "etcd --name %{node_name} --data-dir /var/lib/etcd \
 -heartbeat-interval=100 -election-timeout=5000 \
 --listen-client-urls http://0.0.0.0:2379,http://0.0.0.0:4001 \
 --advertise-client-urls http://%{node_addr}:2379,http://%{node_addr}:4001 \
 --initial-advertise-peer-urls http://%{node_addr}:2380,http://%{node_addr}:7001 \
 --listen-peer-urls http://%{node_addr}:2380 \
 --initial-cluster %{node_peers} --initial-cluster-state new" >> /usr/bin/etcd.sh

chmod +x /usr/bin/etcd.sh
cp %{gopath_folder}/src/github.com/contiv/netplugin/scripts/etcd.service /etc/systemd/system/etcd.service

## start consul
echo "#!/bin/bash" > /usr/bin/consul.sh
echo "consul agent -server %{consul_join_flag} %{consul_bootstrap_flag} \
 -bind=%{node_addr} -data-dir /opt/consul" >> /usr/bin/consul.sh

 chmod +x /usr/bin/consul.sh
cp %{gopath_folder}/src/github.com/contiv/netplugin/scripts/consul.service /etc/systemd/system/consul.service

systemctl daemon-reload || exit 1
systemctl enable etcd || exit 1
systemctl enable consul || exit 1
systemctl start etcd || exit 1
systemctl start consul || exit 1
SCRIPT

provision_bird = <<SCRIPT
## setup the environment file. Export the env-vars passed as args to 'vagrant up'
echo Args passed: [[ $@ ]]
echo "export http_proxy='$1'" >> /etc/profile.d/envvar.sh
echo "export https_proxy='$2'" >> /etc/profile.d/envvar.sh
source /etc/profile.d/envvar.sh
SCRIPT

def customize(v, id)
  # make all nics 'virtio' to take benefit of builtin vlan tag
  # support, which otherwise needs to be enabled in Intel drivers,
  # which are used by default by virtualbox
  # changes the settings for the first 5 NICs, regardless of presence
  1.upto(5) do |n|
    v.customize ['modifyvm', :id, "--nictype#{n}", 'virtio']
  end
  v.customize ['modifyvm', :id, '--paravirtprovider', "kvm"]
end

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
    config.vm.box_check_update = false
    if Vagrant.has_plugin?("vagrant-vbguest")
        config.vbguest.auto_update = false
    end
    if node_os == "ubuntu" then
        config.vm.box = "contiv/ubuntu1604-netplugin"
        config.vm.box_version = "0.7.0"
    else
        config.vm.box = "contiv/centos73"
        config.vm.box_version = "0.10.2"
    end
    config.vm.provider 'virtualbox' do |v|
        v.linked_clone = true if Vagrant::VERSION >= "1.8"
        v.memory = vm_memory
        v.cpus = num_vm_cpus
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
        quagga1.vm.provider "virtualbox" do |v|
            customize(v, :id)
        end

        quagga1.vm.provision "shell" do |s|
          s.inline = provision_bird
          s.args = [http_proxy, https_proxy]
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

        quagga2.vm.provider "virtualbox" do |v|
            customize(v, :id)
        end

        quagga2.vm.provision "shell" do |s|
          s.inline = provision_bird
          s.args = [http_proxy, https_proxy]
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
            node.vm.hostname = node_name
            # create an interface for etcd cluster
            node.vm.network :private_network, ip: node_addr, virtualbox__intnet: "true", auto_config: false
            # create an interface for bridged network
            node.vm.network :private_network, ip: "0.0.0.0", virtualbox__intnet: network_name, auto_config: false
            node.vm.network :private_network, ip: "0.0.0.0", virtualbox__intnet: "contiv_purple", auto_config: false
            node.vm.provider "virtualbox" do |v|
                customize(v, :id)
                v.customize ['modifyvm', :id, '--nicpromisc2', 'allow-all']
                v.customize ['modifyvm', :id, '--nicpromisc3', 'allow-all']
                v.customize ['modifyvm', :id, '--nicpromisc4', 'allow-all']
            end

            # mount the host directories
            node.vm.synced_folder "bin", File.join(gopath_folder, "bin")
            if ENV["GOPATH"] && ENV['GOPATH'] != ""
              node.vm.synced_folder "../../../", File.join(gopath_folder, "src"), rsync: true
            else
              node.vm.synced_folder ".", File.join(gopath_folder, "src/github.com/contiv/netplugin"), rsync: true
            end

            node.vm.provision "shell" do |s|
               if ENV["DOCKER_SWARM"] == "SWARM_MODE"
                   # In swarm mode first VM is the master. Should make it configurable later
                   s.inline = "echo '#{node_ips[0]} netmaster' >> /etc/hosts; echo '#{node_addr} #{node_name}' >> /etc/hosts"
               else
                   s.inline = "echo '#{node_ips[0]} netmaster' >> /etc/hosts; echo '#{node_ips[1]} netmaster' >> /etc/hosts;   echo '#{node_addr} #{node_name}' >> /etc/hosts"
               end
            end

            provision_common_once_script = provision_common_once % {
                cluster_ip_nodes: cluster_ip_nodes,
            }

            node.vm.provision "shell" do |s|
                s.inline = provision_common_once_script
            end
            node.vm.provision "shell", run: "always" do |s|
                s.inline = provision_common_always
                s.args = [node_addr]
            end

            node_provision_script = provision_node % {
                node_name: node_name,
                node_addr: node_addr,
                node_peers: node_peers.join(","),
                gopath_folder: gopath_folder,
                consul_join_flag: consul_join_flag,
                consul_bootstrap_flag: consul_bootstrap_flag,
            }

            node.vm.provision "shell" do |s|
                s.inline = node_provision_script
            end

            # forward netmaster port
            if n == 0 then
                node.vm.network "forwarded_port", guest: 9999, host: 9999
                node.vm.network "forwarded_port", guest: 80, host: 9998
            end
            fwd_port1 = 8880 + n
            fwd_port2 = 9990 + n
            node.vm.network "forwarded_port", guest: fwd_port1, host: fwd_port1
            node.vm.network "forwarded_port", guest: fwd_port2, host: fwd_port2
        end
    end
end
