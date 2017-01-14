# Contiv installation for Kubernetes

This document details the setup instructions for Kubernetes version 1.4+ and higher for CentOS 7

Install kubernetes 1.4 or higher using http://kubernetes.io/docs/getting-started-guides/kubeadm/ and follow the instructions below.

### Install Contiv

* Replace all instances of `__NETMASTER_IP__` in contiv/contiv.yaml with the master IP.
* Replace `__VLAN_IF__` with the data plane interface.
  If there is no requirement to create vlan based networks there is no need for a seperate data interface and `__VLAN_IF__` can be set to "". If vlan based networks are to be created then a separate data interface is mandatory which can be set appropriately.
* Optional: Replace the contiv version(v0.1-11-30-2016.20-08-20.UTC) with the desired release/test version.
* On the management node, run
```sh
kubectl apply -f contiv.yaml
```
* Get netctl from a Contiv release or local build. Contiv releases are available from https://github.com/contiv/netplugin/releases

### Using Contiv

1. On the managment node, create the default network and EPG. For example, a vxlan network can be created as follows:
```sh
netctl net create -t default --subnet=20.1.1.0/24 default-net
netctl group create -t default default-net default-epg
```

Note: netctl uses "netmaster" as the default netmaster host. So add a reference for "netmaster" in /etc/hosts or explicitly specify it as a parameter to all netctl calls.

See https://github.com/contiv/netplugin/tree/master/mgmtfn/k8splugin for some examples on how to use Contiv networking with your pods.

### Troubleshooting Contiv Installation

* Check that netmaster, netplugin are running.
```sh
kubectl get pods -n kube-system
```
* Check the netmaster, netplugin logs to see if there are any errors.
```sh
cd /var/contiv/log
cat netmaster.log or netplugin.log
```
