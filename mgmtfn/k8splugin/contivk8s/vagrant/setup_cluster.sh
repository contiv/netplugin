#!/bin/bash

# add the vagrant box
vagrant box list | grep "contiv/k8s-centos" | grep "0.0.6" >& /dev/null
if [ $? -ne 0 ]; then
  vagrant box add contiv/k8s-centos --box-version 0.0.6
fi

# exit on any error
set -e

# bring up vms
vagrant up

# generate inventory
vagrant_cluster.py

# run ansible
ansible-playbook -i .contiv_k8s_inventory ../../../../../contrib/ansible/cluster.yml --skip-tags "contiv_restart,ovs_install" -e "networking=contiv"
