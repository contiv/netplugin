#!/bin/bash

if [ $EUID -ne 0 ]; then
  echo "Please run this script as root user"
  exit 1
fi

set -ex

swapoff -a
setenforce 0
systemctl stop firewalld
systemctl disable firewalld
/etc/init.d/network restart

cp /shared/.etc_hosts /etc/hosts

mkdir -p /etc/docker

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

# the image comes with newer version of packages, remove them first
yum remove -y docker docker-common container-selinux docker-selinux docker-engine docker-engine-selinux
yum install -y docker
systemctl enable docker && systemctl start docker

k8s_version="$1"
if [ "$k8s_version" = "stable" ]; then
    yum install -y kubelet kubeadm kubectl
else
    v="${k8s_version#v}"
    extra_rpm=""
    if [[ "$v" = 1.8* ]]; then
        # repo contains more than one kubernetes-cni-0.5.1
        # will cause dependency error if not specified
        extra_rpm="kubernetes-cni-0.5.1-1"
    fi
    yum install -y "kubelet-$v" "kubeadm-$v" "kubectl-$v" "$extra_rpm"
fi

systemctl enable kubelet && systemctl start kubelet
