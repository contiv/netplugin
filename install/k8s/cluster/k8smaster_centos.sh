#!/bin/bash -x
kubeadm init --token $1 --apiserver-advertise-address $2 --apiserver-bind-port $3 --kubernetes-version $4

kubectl taint nodes --all node-role.kubernetes.io/master-

if [ -n "$CONTIV_TEST" ]; then
    cp /etc/kubernetes/admin.conf /shared/admin.conf
    chmod 0644 /etc/kubernetes/admin.conf
    cd /opt/gopath/src/github.com/contiv/netplugin/install/k8s/contiv/
    ./contiv-compose add-systest --k8s-api https://$2:$3 ./contiv-base.yaml > /shared/contiv.yaml
    # remove kube-dns
    # TODO: enable kube-dns
    kubectl delete deployment -n kube-system kube-dns
else
    # update to use released version
    cd /opt/gopath/src/github.com/contiv/netplugin/install/k8s/contiv/
    ./contiv-compose use-release --k8s-api https://$2:$3 -v $(cat /opt/gopath/src/github.com/contiv/netplugin/version/CURRENT_VERSION) ./contiv-base.yaml > /shared/contiv.yaml
fi

kubectl apply -f /shared/contiv.yaml
