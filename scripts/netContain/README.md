## Introduction

This directory contains scripts required to containerize Contiv and
deploy it in docker or kubernetes modes.
The script build_image.sh should be executed from the top level
directory as follows:

```
./scripts/netContain/build_image.sh
```

After this script successfully executes you will get a final image. The
image can then be pushed to a registry or to the Hub for deployment on
a cluster.

## Executing Containerized Contiv

   Running netmaster
```
  docker run -itd --pid=host --net=host --name=contivNet \
--privileged -v /etc/openvswitch:/etc/openvswitch \
-v /etc/kubernetes/ssl:/tmp/kubernets/ssl -v /var/run/:/var/run/ \
-v /var/contiv:/var/contiv contiv/netplugin  -m  -p kubernetes \
-c etcd://127.0.0.1:2379
```
  Please note that --net=host is required to allow netplugin to use
the host's main network namespace. This is necessary to communicate
with the Docker engine and with OVS.

  Running netplugin

```
  ### Running netplugin
  docker run -itd --pid=host --net=host --name=contivNet --privileged \
-v /etc/openvswitch:/etc/openvswitch -v /var/run:/var/run \
-v /etc/kubernetes/ssl:/tmp/kubernetes/ssl -v /var/contiv:/var/contiv \
rajenata/contiv:0.1 -v ${WORKER_IP} -i  ${VLAN_IF} -f routing \
-p kubernetes -c etcd://127.0.0.1:2379
```
  The --pid=host option is used for Kubernetes to allow netplugin to
set up the network within the containers.

  vtep-ip or the vlan network interface is mandatory when running netplugin.


## Cloud Config Example

The example below show how cloud-config can be used to run Contiv in
Kubernetes mode. The example below was used on a CoreOS cluster.

The systemd services below show how the Contiv services can be launched.

The following example shows how netmaster can be started with systemd.

```
    - name: contiv.service
      command: start
      content: |
         [Unit]
         Description= Docker container for Contiv
         Requires=docker.service
         After=docker.service
         [Service]
         Type=oneshot
         RemainAfterExit=yes
         ExecStartPre=/bin/bash -c '/usr/bin/mkdir -p /var/contiv/config /var/contiv/log;  if [ `docker ps -a | grep contivNet | wc -l` == 0 ]; then /usr/bin/docker run -itd --pid=host --net=host --name=contivNet  --privileged   -v /etc/openvswitch:/etc/openvswitch -v /etc/kubernetes/ssl:/tmp/kubernets/ssl -v /var/run/:/var/run/  -v /var/contiv:/var/contiv rajenata/contiv:0.1  -m  -p kubernetes -c etcd://127.0.0.1:2379; fi'
         ExecStart=/usr/bin/docker start contivNet
         ExecStartPost=/bin/bash -c '/usr/bin/mkdir -p /opt/cni/bin; /usr/bin/docker cp contivNet:/contiv/bin/contivk8s /opt/cni/bin/'
         ExecStop=/usr/bin/docker stop contivNet
```

The following example shows how netplugin can be started with systemd.

```
    - name: contiv.service
      command: start
      content: |
         [Unit]
         Description= Docker container for Contiv
         Requires=docker.service
         After=docker.service
         [Service]
         Type=oneshot
         RemainAfterExit=yes
         ExecStartPre=/bin/bash -c '/usr/bin/mkdir -p /var/contiv/config /var/contiv/log; if [ `docker ps -a | grep contivNet | wc -l` == 0 ]; then /usr/bin/docker run -itd --pid=host --net=host --name=contivNet  --privileged   -v /etc/openvswitch:/etc/openvswitch -v /var/run:/var/run -v /etc/kubernetes/ssl:/tmp/kubernetes/ssl -v /var/contiv:/var/contiv rajenata/contiv:0.1 -v ${MY_WORKER_IP} -i ens32 -f routing -p kubernetes -c etcd://127.0.0.1:2379; fi'
         ExecStart=/usr/bin/docker start contivNet
         ExecStartPost=/bin/bash -c '/usr/bin/mkdir -p /opt/cni/bin; /usr/bin/docker cp contivNet:/contiv/bin/contivk8s /opt/cni/bin/'
         ExecStop=/usr/bin/docker stop contivNet
```
