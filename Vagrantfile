# -*- mode: ruby -*-
# vi: set ft=ruby :

NETPLUGIN_SYNCED_GOPATH="/opt/golang"
HOST_GOBIN_PATH="/opt/go/bin"
HOST_GOROOT_PATH="/opt/go/root"

def ansible_provision(host) 
  proc do |ansible|
    ansible.playbook = 'ansible/site.yml'
    # Note: Can't do ranges like mon[0-2] in groups because
    # these aren't supported by Vagrant, see
    # https://github.com/mitchellh/vagrant/issues/3539
    ansible.groups = { }
    proxy_env = { }

    %w[http_proxy https_proxy].each do |name|
      if ENV[name]
        proxy_env[name] = ENV[name]
      end
    end

    # In a production deployment, these should be secret
    ansible.extra_vars = {
      proxy_env: proxy_env,
      contiv_env: ENV['CONTIV_ENV'],
      netplugin_synced_gopath: NETPLUGIN_SYNCED_GOPATH,
      host_gobin_path: HOST_GOBIN_PATH,
      host_goroot_path: HOST_GOROOT_PATH,
      hostname: host,
    }

    ansible.limit = host
  end
end

VAGRANTFILE_API_VERSION = "2"
Vagrant.configure(VAGRANTFILE_API_VERSION) do |config|
    config.vm.box = "ubuntu/vivid64"
    # Commenting out the url since we host the image on Atlas.
    # config.vm.box_url = "https://cisco.box.com/shared/static/27u8utb1em5730rzprhr5szeuv2p0wir.box"
    num_nodes = 2
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
                node.vm.synced_folder ENV['CONTIV_HOST_GOPATH'], NETPLUGIN_SYNCED_GOPATH
            else
                node.vm.synced_folder ENV['GOPATH'], NETPLUGIN_SYNCED_GOPATH
            end
            if ENV['CONTIV_HOST_GOBIN'] != nil
                node.vm.synced_folder ENV['CONTIV_HOST_GOBIN'], HOST_GOBIN_PATH
            end
            if ENV['CONTIV_HOST_GOROOT'] != nil
                node.vm.synced_folder ENV['CONTIV_HOST_GOROOT'], HOST_GOROOT_PATH
            end

            node.vm.provision :ansible, &ansible_provision(node_name)

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
