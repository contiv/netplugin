#!/bin/bash -x
kubeadm join --token "$1" "$2" --discovery-token-unsafe-skip-ca-verification
# --ignore-preflight-errors all
if [ ! -z "$CONTIV_TEST" ]; then
    cp /shared/admin.conf /etc/kubernetes/admin.conf
    chmod 0644 /etc/kubernetes/admin.conf
fi
