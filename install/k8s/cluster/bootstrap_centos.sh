#!/bin/bash

if [ $EUID -ne 0 ]; then
  echo "Please run this script as root user"
  exit 1
fi

set -e

swapoff -a
setenforce 0
systemctl stop firewalld
systemctl disable firewalld
/etc/init.d/network restart

cp /shared/.etc_hosts /etc/hosts

cat <<EOF >> /etc/docker/daemon.json
{
    "storage-driver": "devicemapper"
}
EOF

cat <<EOF > /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://packages.cloud.google.com/yum/repos/kubernetes-el7-x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOF

cat <<EOF >  /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-ip6tables = 1
net.bridge.bridge-nf-call-iptables = 1
EOF

sysctl --system

yum remove -y docker \
                  docker-common \
                  container-selinux \
                  docker-selinux \
                  docker-engine \
                  docker-engine-selinux

yum install -y docker kubelet kubeadm kubectl

systemctl enable docker && systemctl start docker
systemctl enable kubelet && systemctl start kubelet
