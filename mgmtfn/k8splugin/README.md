# Contiv Networking for Kubernetes

Contiv is integrated with Kubernetes via a CNI plugin. With this integration, Contiv
Networking and Policy can be used for Pod inter-connectivity in a Kubernetes cluster.

## Getting Started

This step-by-step procedure will guide you through a minimal experience of creating
a Kubernetes cluster with Contiv networking and applying policy between pods.

##### Pre-requisites

Before starting, please be sure to set http/https proxies if your network requires it.
*(Note that https_proxy should be set to point to a http:// URL (not https://).
This is an ansible requirement.)*

The setup scripts use python modules *parse* and *netaddr*. If these modules are not
installed on the machine where you are executing these steps, you should install them
before proceeding. *(E.g. pip install parse; pip install netaddr)*

#### Step 1: Clone contrib and netplugin repos

```
$ mkdir -p ~/go/src/github.com/k8s
$ cd ~/go/src/github.com/k8s
$ git clone https://github.com/jojimt/contrib -b contiv

$ cd ~/go/src/github.com/k8s
$ git clone https://github.com/contiv/netplugin
```

#### Step 2: Create cluster

```
$ cd ~/go/src/github.com/k8s/netplugin
$ make k8s-cluster
```

This step will run Vagrant and ansible commands to bring a kubernetes cluster
with one master and two worker nodes. This will take some time. Please be 
patient.

When step 2 completes, you should a message like the one below:

```
PLAY RECAP ******************************************************************** 
k8master                   : ok=xxx  changed=xxx  unreachable=0    failed=0   
k8node-01                  : ok=xxx  changed=xxx  unreachable=0    failed=0   
k8node-02                  : ok=xxx  changed=xxx  unreachable=0    failed=0   
```

At this point, your cluster is ready for use.

*Note: Occasionally, you might hit an error during ansible provisioning.
If that happens, just re-issue the command (usually, it's caused by a temporary
unavailability of a repo on the web). If the problem persists, you should open an
issue on github.*

You should proceed to Step 3 **only if** the previous step completed successfully.

This demo utilizes a **busybox** image built to include a full **netcat** utility.
However, you can try other images as well if you like. *The Dockerfile used for 
building the nc-busybox is available in the /shared folder of the k8master node
(you will get to this directory in Step 3).*

#### Step 3: Start the demo and ssh to the kubernetes master

This step will start network services and log you into the kubernetes master node.

```
$ make k8s-demo-start
```

When step 3 completes, you will get a shell prompt from the master. Use *sudo su* to
enter sudo mode. Try a few commands.


```
[vagrant@k8master ~]$sudo su
[root@k8master vagrant]# kubectl get nodes
NAME        LABELS                             STATUS    AGE
k8node-01   kubernetes.io/hostname=k8node-01   Ready     4m
k8node-02   kubernetes.io/hostname=k8node-02   Ready     4m

[root@k8master vagrant]# netctl net list
Tenant   Network      Encap type  Packet tag  Subnet       Gateway
------   -------      ----------  ----------  -------      ------
default  default-net  vxlan       <nil>       20.1.1.0/24  20.1.1.254
default  poc-net      vxlan       <nil>       21.1.1.0/24  21.1.1.254

[root@k8master ~]# netctl group list
Tenant   Group        Network      Policies
------   -----        -------      --------
default  default-epg  default-net  
default  poc-epg      poc-net

```

The last two commands show contiv configuration. The demo set up created two networks
and two epgs. Lets try some examples.

## Example 1: No network labels = Pod placed in default network

cd to /shared directory to find some pod specs. Create defaultnet-busybox1 and
defaultnet-busybox2.

```
[root@k8master ~]# cd /shared
[root@k8master shared]#ls
defaultnet-busybox1.yaml  noping-busybox.yaml  pocnet-busybox.yaml
defaultnet-busybox2.yaml  pingme-busybox.yaml  policy.sh

[root@k8master shared]# kubectl create -f defaultnet-busybox1.yaml 
pod "defaultnet-busybox1" created
[root@k8master shared]# kubectl create -f defaultnet-busybox2.yaml 
pod "defaultnet-busybox2" created

[root@k8master shared]# kubectl get pods
NAME                 READY     STATUS    RESTARTS   AGE
defaultnet-busybox1  1/1       Running   0          3m
defaultnet-busybox2  1/1       Running   0          39s

```

It may take a few minutes for the pods to enter Running state. When both have entered
Running state, check their ip address and try reachability.

```
[root@k8master shared]# kubectl exec defaultnet-busybox1 -- ip address
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue 
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
    inet6 ::1/128 scope host 
       valid_lft forever preferred_lft forever
47: eth0@if46: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1450 qdisc noqueue 
    link/ether 02:02:14:01:01:09 brd ff:ff:ff:ff:ff:ff
    inet 20.1.1.9/24 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 fe80::2:14ff:fe01:109/64 scope link 
       valid_lft forever preferred_lft forever


[root@k8master shared]# kubectl describe pod defaultnet-busybox2 | grep IP
IP:                             20.1.1.10

[root@k8master shared]# kubectl exec defaultnet-busybox1 -- ping 20.1.1.10
PING 20.1.1.10 (20.1.1.10): 56 data bytes
64 bytes from 20.1.1.10: seq=0 ttl=64 time=0.562 ms
64 bytes from 20.1.1.10: seq=1 ttl=64 time=0.124 ms
64 bytes from 20.1.1.10: seq=2 ttl=64 time=0.073 ms

```

Notice that both pods were assigned IP addresses from the default network and
they can ping each other.

## Example 2: Use network labels to specify a network and epg for the Pod

Now let's create a Pod with poc-net and poc-epg specified as network and epg
respectively. Examine pocnet-busybox.yaml. There are two additional labels:
**io.contiv.network:** *poc-net* and **io.contiv.net-group:** *poc-epg* specified
in this pod spec.

```
[root@k8master shared]# kubectl create -f pocnet-busybox.yaml 
pod "busybox-poc-net" created

[root@k8master shared]# kubectl get pods
NAME                  READY     STATUS    RESTARTS   AGE
busybox-poc-net       1/1       Running   0          54s
defaultnet-busybox1   1/1       Running   0          35m
defaultnet-busybox2   1/1       Running   0          35m

[root@k8master shared]# kubectl exec busybox-poc-net -- ip address
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue 
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
    inet6 ::1/128 scope host 
       valid_lft forever preferred_lft forever
129: eth0@if128: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1450 qdisc noqueue 
    link/ether 02:02:15:01:01:02 brd ff:ff:ff:ff:ff:ff
    inet 21.1.1.2/24 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 fe80::2:15ff:fe01:102/64 scope link 
       valid_lft forever preferred_lft forever
```

Notice that this pod was assigned an IP addresses from the poc-net.

## Example 3: Use Contiv to specify and enforce network policy

In this example, we will create a policy and attach it to an epg. We will specify
this epg in the pod spec and verify that the policy is enforced.

Examine policy.sh. This contains contiv commands to create a simple ICMP deny rule,
add it to a policy and attach to an epg. Excute this script to create the network
objects.

```
[root@k8master shared]# ./policy.sh

[root@k8master shared]# netctl group list
Tenant   Group        Network      Policies
------   -----        -------      --------
default  poc-epg      poc-net      
default  noping-epg   poc-net      icmpPol
default  default-epg  default-net  

[root@k8master shared]# netctl rule list icmpPol
Rule  Direction  Priority  EndpointGroup  Network  IpAddress  Protocol  Port   Action
----  ---------  --------  -------------  -------  ---------  --------  ----   ------
1     in         1         <nil>          <nil>    <nil>      icmp      <nil>  deny

```

Examine noping-busybox.yaml and pingme-busybox.yaml. They specify noping-epg and poc-epg
respectively as their epgs. Both of these pods have a netcat listener on TCP port 6379
(See nc-busybox/nc_loop.sh).

Create both of these pods and verify their connectivity behavior.

```
[root@k8master shared]# kubectl create -f noping-busybox.yaml 
pod "annoyed-busybox" created
[root@k8master shared]# kubectl create -f pingme-busybox.yaml 
pod "sportive-busybox" created

[root@k8master shared]# kubectl get pods
NAME                READY     STATUS    RESTARTS   AGE
annoyed-busybox       1/1       Running   0          22m
busybox-poc-net       1/1       Running   0          35m
defaultnet-busybox1   1/1       Running   0          35m
defaultnet-busybox2   1/1       Running   0          35m
sportive-busybox      1/1       Running   0          21m


[root@k8master shared]# kubectl describe pod annoyed-busybox | grep IP
IP:                             21.1.1.2

[root@k8master shared]# kubectl describe pod sportive-busybox | grep IP
IP:                             21.1.1.4

```

Try to access annoyed-busybox and sportive-busybox from busybox-poc-net via ping and nc.

```
[root@k8master shared]# kubectl exec busybox-poc-net -- ping 21.1.1.2

[root@k8master shared]# kubectl exec busybox-poc-net -- ping 21.1.1.4
PING 21.1.1.4 (21.1.1.4): 56 data bytes
64 bytes from 21.1.1.4: seq=0 ttl=64 time=0.230 ms
64 bytes from 21.1.1.4: seq=1 ttl=64 time=0.390 ms
64 bytes from 21.1.1.4: seq=2 ttl=64 time=0.205 ms

[root@k8master shared]# kubectl exec busybox-poc-net -- nc -zvw 1 21.1.1.2 6379
21.1.1.2 [21.1.1.2] 6379 (6379) open

[root@k8master shared]# kubectl exec busybox-poc-net -- nc -zvw 1 21.1.1.4 6379
21.1.1.4 [21.1.1.4] 6379 (6379) open

```

Notice 1) busybox-poc-net is unable to ping annoyed-busybox 2) busybox-poc-net is able
to ping sportive-busybox, to which no policy was applied. 3) busybox-poc-net is able
to exchange TCP with annoyed-busybox, consistent with the applied policy. You can try
other combinations as well, e.g. ping/nc between annoyed-busybox and sportive-busybox.
You can also create your own policy and pod spec and try.
