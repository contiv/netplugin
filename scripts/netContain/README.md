## Introduction 

This Directory contains scripts required to containerize Contiv and deploy it in either docker or kubernetes modes.
The script BuildContainer.sh should be executed from the top level directory as follows 

```
./scripts/netContain/BuildContainer.sh
```

After this Script successfully executes you will get a final image which is containerized contiv. The container can then be pushed into repo for deployment on a cluster. 

## Executing Containerized Contiv

   Running as Net Master
```
	docker run -itd --pid=host --net=host --name=contivNet  --privileged   -v /etc/openvswitch:/etc/openvswitch -v /etc/kubernetes/ssl:/tmp/kubernets/ssl -v /var/run/:/var/run/  -v /var/contiv:/var/contiv rajenata/contiv:0.1  -m  -p kubernetes -c etcd://127.0.0.1:2379
```
  Please Note that --net=host is required so that Contiv has the Same Network view as the Host Namespace, this is required for both talking to the Docker Runtime and OVS Kernel.
  
  Running as NetPlugin

```
  ### Running as Net Plugin
  docker run -itd --pid=host --net=host --name=contivNet  --privileged   -v /etc/openvswitch:/etc/openvswitch -v /var/run:/var/run -v /etc/kubernetes/ssl:/tmp/kubernetes/ssl -v /var/contiv:/var/contiv rajenata/contiv:0.1 -v ${WORKER_IP} -i  ${VLAN_IF} -f routing -p kubernetes -c etcd://127.0.0.1:2379
```
  The --pid=host option is used in the case of Kubernetes environment so that the netplugin can exec nsenter and modify the container networking.
 
  It is mandatory to provide either vtep-ip or vlan interface when using as netplugin.


## CLoud Config Examples

Here are examples of how to use cloud-config to run Contiv in Kubernetes mode, the following example was used in Core OS cluster.  The following example details how contiv can be launched as netmaster with systemd.

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

    The below example is for Launching Contiv as Netplugin:

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

 Complete details of Cloud Config will be published into netplugin/scripts/cloud-config
