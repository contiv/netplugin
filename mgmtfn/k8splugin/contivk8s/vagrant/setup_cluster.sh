#!/bin/bash

# kubernetes version to use -- defaults to v1.1.4
: ${k8sVer:=v1.1.4}

# add the vagrant box
vagrant box list | grep "contiv/k8s-centos" | grep "0.0.6" >& /dev/null
if [ $? -ne 0 ]; then
  vagrant box add contiv/k8s-centos --box-version 0.0.6
fi

# exit on any error
set -e

# fetch kubernetes released binaries
pushd .
top_dir=$(git rev-parse --show-toplevel | sed 's|/[^/]*$||')
mkdir -p $top_dir/k8s-$k8sVer
if [ -f $top_dir/k8s-$k8sVer/kubernetes.tar.gz ]; then
  echo "k8s-$k8sVer/kubernetes.tar.gz found, not fetching."
  rm -rf $top_dir/k8s-$k8sVer/kubernetes
  rm -rf $top_dir/k8s-$k8sVer/bin
else
  cd $top_dir/k8s-$k8sVer && wget https://github.com/kubernetes/kubernetes/releases/download/$k8sVer/kubernetes.tar.gz
fi

# untar kubernetes released binaries
cd $top_dir/k8s-$k8sVer
tar xvfz kubernetes.tar.gz kubernetes/server/kubernetes-server-linux-amd64.tar.gz
tar xvfz kubernetes/server/kubernetes-server-linux-amd64.tar.gz
popd

if [ ! -f $top_dir/k8s-$k8sVer/kubernetes/server/bin/kubelet ]; then
  echo "Error kubelet not found after fetch/extraction"
  exit 1
fi

# bring up vms
vagrant up

# generate inventory
vagrant_cluster.py

# run ansible
ansible-playbook -i .contiv_k8s_inventory ../../../../../contrib/ansible/cluster.yml --skip-tags "contiv_restart,ovs_install" -e "networking=contiv localBuildOutput=$top_dir/k8s-$k8sVer/kubernetes/server/bin"
