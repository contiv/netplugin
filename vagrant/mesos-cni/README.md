# Contiv CNI with Mesos/Marathon

This document explains how to run Contiv CNI demo with Mesos

### Prerequisits
- Virtualbox 5.0.2 or higher
- Vagrant 1.7.4 or higher

### Step 1: Bring up/bring down demo vagrant VMs

```
$ git clone https://github.com/contiv/netplugin
$ cd netplugin/vagrant/mesos-cni
# bring up demo vms
$ make mesos-cni-demo 
# cleanup up demo vms
$ make mesos-cni-destroy
```

This brings up two Vagrant node setup with Contiv, Mesos and Marathon.
This also builds netplugin binaries and start them on both VMs
mesos-node1: contiv netmaster, contiv netplugin, mesos master, marathon
mesos-node2: contiv netmaster, contiv netplugin, mesos agent, CNI plugin

### Step 2: Launch containers

```
in vagrant/mesos-cni dir
$ vagrant ssh mesos-node2
<Inside vagrant VM>

use "cni_task.sh" script to launch containers.
This script creates contiv tenant/network.

usage: ./cni_task.sh [-m marathon-ipaddr] [-j jobname] [-t tenant-name] [-n network-name] [-g network-group] [-s subnet]
-m marathon-ipaddr : 192.168.2.10:5050 is by default
-j jobname         : "container.xxx" by default
-t tenant-name     : "default" by default
-n network-name    : "default-net" by default
-s subnet          : "10.36.28.0/24" by default

# create a python http server listening on port 9002 in contiv network 
'default-net'

./cni_task.sh 
```

to launch container without using cni_task.sh,
create network using netctl cli
$ netctl net create default-net -subnet 10.1.1.0/24

create a json file  & update network/tenant
```
{
  "id": "container1",
  "cmd": "python3 -m http.server 9002",
  "cpus": 1,
  "mem": 500,
  "disk": 0,
  "instances": 1,
  "container": {
    "type": "MESOS",
    "volumes": [],
    "mesos": {
      "image": "ubuntu:latest",
      "privileged": false,
      "parameters": [],
      "forcePullImage": false
    }
  },
  "ipAddress": {
     "networkName": "contiv",
     "labels": {
         "io.contiv.tenant": <name of contiv tenant name>
         "io.contiv.network": <name of contiv network name>
         "io.contiv.net-group": <name of contiv network group> 
     }
```
  }
}
```
update container id
update contiv CNI fields 

```
     "labels": {
         "io.contiv.tenant": <name of contiv tenant name>
         "io.contiv.network": <name of contiv network name>
         "io.contiv.net-group": <name of contiv network group> 
     }
```

launch the container by sending json to marathon
curl -X POST http://192.168.2.10:8080/v2/apps -d @${JSON_FILE} \
     -H "Content-type: application/json"


### Step 3: Check status 
marathon UI
http://192.168.2.10:8080
mesos UI
http://192.168.2.10:5050
