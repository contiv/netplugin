#!/bin/bash

if [ $EUID -ne 0 ]; then
  echo "Please run this script as root user"   
  exit 1
fi
set -e
cat <<EOF > /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=http://yum.kubernetes.io/repos/kubernetes-el7-x86_64
enabled=1
gpgcheck=0
repo_gpgcheck=0
EOF

setenforce 0

# yum install -y docker kubelet kubeadm kubectl kubernetes-cni
yum install -y docker ebtables \
	https://fedorapeople.org/groups/kolla/kubeadm-1.6.0-0.alpha.0.2074.a092d8e0f95f52.x86_64.rpm \
	https://fedorapeople.org/groups/kolla/kubectl-1.5.4-0.x86_64.rpm \
	https://fedorapeople.org/groups/kolla/kubelet-1.5.4-0.x86_64.rpm \
	https://fedorapeople.org/groups/kolla/kubernetes-cni-0.3.0.1-0.07a8a2.x86_64.rpm

systemctl enable docker && systemctl start docker
systemctl enable kubelet && systemctl start kubelet

systemctl stop firewalld
systemctl disable firewalld

