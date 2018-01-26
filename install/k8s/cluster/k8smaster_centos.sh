#!/bin/bash -x
kubeadm init --token $1 --apiserver-advertise-address $2 --apiserver-bind-port $3 --kubernetes-version $4

if [ -n "$CONTIV_TEST" ]; then
    cp /etc/kubernetes/admin.conf /shared/admin.conf
    chmod 0644 /etc/kubernetes/admin.conf
    cd /opt/gopath/src/github.com/contiv/netplugin/install/k8s/contiv/

    if [ "$CONTIV_TEST" = "sys" ]; then
        ./contiv-compose add-systest --k8s-api https://$2:$3 ./contiv-base.yaml > /shared/contiv.yaml
        # remove kube-dns
        # TODO: enable kube-dns
        kubectl delete deployment -n kube-system kube-dns
        kubectl delete service kube-dns -n=kube-system
        kubectl delete serviceaccounts kube-dns -n=kube-system
        kubectl delete clusterrolebindings system:kube-dns -n=kube-system
        kubectl delete endpoint kube-dns -n=kube-system
    elif [ "$CONTIV_TEST" = "dev" ]; then
        ./contiv-compose add-systest --start --k8s-api https://$2:$3 ./contiv-base.yaml > /shared/contiv.yaml
    fi
else
    # update to use released version
    cd /opt/gopath/src/github.com/contiv/netplugin/install/k8s/contiv/
    ./contiv-compose use-release --k8s-api https://$2:$3 -v $(cat ../../../version/CURRENT_VERSION) ./contiv-base.yaml > /shared/contiv.yaml
fi

kubectl apply -f /shared/contiv.yaml
