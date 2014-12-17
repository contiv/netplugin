# -*- mode: ruby -*-
# vi: set ft=ruby :

$provision = <<SCRIPT
apt-get update -qq && apt-get install -y vim curl python-software-properties git golang openvswitch-switch
cat > /etc/profile.d/envvar.sh <<'EOF'
export GOPATH=/opt/golang
export GOBIN=$GOPATH/bin
export PATH=$PATH:$GOBIN
export http_proxy=proxy.esl.cisco.com:8080
export https_proxy=$http_proxy
export HTTP_PROXY=$http_proxy
export HTTPS_PROXY=$http_proxy
EOF
. /etc/profile.d/envvar.sh
go get -u github.com/mapuri/netplugin
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
