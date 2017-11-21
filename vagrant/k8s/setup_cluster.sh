#!/bin/bash

set -euo pipefail

echo "Using kubeadm/kubectl installation"

# Copy the contiv installation file to shared folder
if [ "${CONTIV_K8S_USE_KUBEADM:-}" == 1 ]; then
    cp -f ../../install/k8s/contiv/contiv_devtest.yaml ./export/.contiv.yaml
else
    cp -f ../../install/k8s/contiv/contiv.yaml ./export/.contiv.yaml
fi
# Replace __NETMASTER_IP__ and __VLAN_IF__
sed -i.bak 's/__NETMASTER_IP__/192.168.2.10/g' ./export/.contiv.yaml
sed -i.bak 's/__VLAN_IF__/eth2/g' ./export/.contiv.yaml

# bring up vms
VAGRANT_USE_KUBEADM=1 vagrant up
