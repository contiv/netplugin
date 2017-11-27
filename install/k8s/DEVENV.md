# Kubernetes Setup for Contiv

This document details the setup instructions for Kubernetes version 1.6 and higher for CentOS 7

### Setup Kubernetes Cluster
Install kubernetes 1.6 or higher using http://kubernetes.io/docs/getting-started-guides/kubeadm/

(OR) Alternatively do the following:
* On all nodes run 

```sh
./cluster/bootstrap_centos.sh
```

* On the master node, run 

```sh
./cluster/k8smaster_centos.sh <token> <master management IP> <kubernetes version>
```
For example, k8smaster_centos.sh "d900e1.8a392798f13b33a4" 192.168.2.10 v1.6, will start a cluster with kubernetes API server on 192.168.2.10. The token is a 6.16 string which can be generated as shown below:
```sh
python -c 'import random; print "%0x.%0x" % (random.SystemRandom().getrandbits(3*8), random.SystemRandom().getrandbits(8*8))' 

```
        
* On all the minions run 

```sh
./cluster/k8sworker_centos.sh <token> <master management IP>
```
### Install Contiv

Install Contiv as ddescribed in the README

### Developer Guide

* To build a new version of contiv, see https://github.com/contiv/netplugin/tree/master/scripts/netContain
  
  TL;DR, run:
```sh
  ./scripts/netContain/BuildContainer.sh
  docker tag contivbase <containername>
  docker push <containername>
```
* Replace neelima/contiv in contiv/contiv.yaml with the containername of the container built above
