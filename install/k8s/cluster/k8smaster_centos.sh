#!/bin/bash -x
kubeadm init --token $1 --apiserver-advertise-address $2 --ignore-preflight-errors all --kubernetes-version $3

kubectl taint nodes --all node-role.kubernetes.io/master-

if [ ! -z "$CONTIV_TEST" ]; then
    cp /etc/kubernetes/admin.conf /shared/admin.conf
    chmod 0644 /etc/kubernetes/admin.conf
    cd /opt/gopath/src/github.com/contiv/netplugin/install/k8s/contiv/
    ./contiv-compose add-systest ./base.yaml > /shared/contiv.yaml
else
    cp /opt/gopath/src/github.com/contiv/netplugin/install/k8s/contiv/base.yaml /shared/contiv.yaml
fi
kubectl apply -f /shared/contiv.yaml
