#!/bin/bash
# create contiv + mesos + marathon box image 
#
# image includes:
#    1. base image contiv/ubuntu1504-netplugin 
#    2. mesos 1.0.0-rc4
#    3. marathon 1.2.0 RC5 
#
BOXNAME="contiv-mesos.box"
VAGRANT_DIR=".box-tmp"
rm -f ${BOXNAME}
rm -rf ${VAGRANT_DIR}
mkdir ${VAGRANT_DIR}
cat > ${VAGRANT_DIR}/Vagrantfile <<VAGRANTFILE_END
# -*- mode: ruby -*-
# vi: set ft=ruby :
#
\$install_script = <<INSTALL_SCRIPT
apt-get -y update
# install golang
wget https://storage.googleapis.com/golang/go1.6.2.linux-amd64.tar.gz
tar xvfz go1.6.2.linux-amd64.tar.gz
mv /usr/local/go /usr/local/go-1.5
mv go /usr/local/
echo "updated go"
go version

wget https://get.docker.com/builds/Linux/x86_64/docker-1.11.2.tgz
tar xvfz docker-1.11.2.tgz
mv docker/* /usr/bin/
echo "updated docker"
docker version

# sbt tools to build marathon
echo "deb https://dl.bintray.com/sbt/debian /" | tee -a /etc/apt/sources.list.d/sbt.list
apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv 2EE0EA64E40A89B84B2DF73499E82A75642AC823
# javac to compile marathon
add-apt-repository ppa:webupd8team/java -y
echo "oracle-java8-installer shared/accepted-oracle-license-v1-1 select true" \
   | /usr/bin/debconf-set-selections
apt-get -y update
# mesos deb dependencies
apt-get install -y jq sbt libevent-dev  libsvn-dev zookeeper zookeeperd
apt-get install -y oracle-java8-installer oracle-java8-set-default

# pull mesos 1.0.0 , latest as of 26 jul
wget http://repos.mesosphere.com/ubuntu/pool/main/m/mesos/mesos_1.0.0-2.0.89.ubuntu1404_amd64.deb
sudo dpkg -i mesos_1.0.0-2.0.89.ubuntu1404_amd64.deb

# clone & build marathon 1.2.0 RC8 
git clone https://github.com/mesosphere/marathon.git
cd marathon && git checkout tags/v1.2.0-RC8 && sbt assembly
INSTALL_SCRIPT

# All Vagrant configuration is done below. The "2" in Vagrant.configure
# configures the configuration version (we support older styles for
# backwards compatibility). Please don't change it unless you know what
# you're doing.
Vagrant.configure("2") do |config|
  # The most common configuration options are documented and commented below.
  # For a complete reference, please see the online documentation at
  # https://docs.vagrantup.com.

  # Every Vagrant development environment requires a box. You can search for
  # boxes at https://atlas.hashicorp.com/search.
  config.vm.box = "contiv/ubuntu1504-netplugin"
  config.vm.hostname = "contiv-mesos"
  config.vm.provision "shell", privileged: true, inline:\$install_script


  # Disable automatic box update checking. If you disable this, then
  # boxes will only be checked for updates when the user runs
  # 'vagrant box outdated'. This is not recommended.
  # config.vm.box_check_update = false

  # Create a forwarded port mapping which allows access to a specific port
  # within the machine from a port on the host machine. In the example below,
  # accessing "localhost:8080" will access port 80 on the guest machine.
  # config.vm.network "forwarded_port", guest: 80, host: 8080

  # Create a private network, which allows host-only access to the machine
  # using a specific IP.
  # config.vm.network "private_network", ip: "192.168.33.10"

  # Create a public network, which generally matched to bridged network.
  # Bridged networks make the machine appear as another physical device on
  # your network.
  # config.vm.network "public_network"

  # Share an additional folder to the guest VM. The first argument is
  # the path on the host to the actual folder. The second argument is
  # the path on the guest to mount the folder. And the optional third
  # argument is a set of non-required options.
  # config.vm.synced_folder "../data", "/vagrant_data"

  # Provider-specific configuration so you can fine-tune various
  # backing providers for Vagrant. These expose provider-specific options.
  # Example for VirtualBox:
  #
  config.vm.provider "virtualbox" do |vb|
     # Customize the amount of memory on the VM:
     vb.cpus = "4"
     vb.memory = "4096"
   end
  #
  # View the documentation for the provider you are using for more
  # information on available options.

  # Define a Vagrant Push strategy for pushing to Atlas. Other push strategies
  # such as FTP and Heroku are also available. See the documentation at
  # https://docs.vagrantup.com/v2/push/atlas.html for more information.
  # config.push.define "atlas" do |push|
  #   push.app = "YOUR_ATLAS_USERNAME/YOUR_APPLICATION_NAME"
  # end

  # Enable provisioning with a shell script. Additional provisioners such as
  # Puppet, Chef, Ansible, Salt, and Docker are also available. Please see the
  # documentation for more information about their specific syntax and use.
  # config.vm.provision "shell", inline: <<-SHELL
  #   apt-get update
  #   apt-get install -y apache2
  # SHELL
end
VAGRANTFILE_END
cd ${VAGRANT_DIR} && vagrant destroy -f && vagrant up && vagrant package --output ../${BOXNAME} \
    && echo "image ${BOXNAME} is ready !" && cd -
rm -rf ${VAGRANT_DIR}
