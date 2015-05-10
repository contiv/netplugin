The multi node container networking can be tested on a single VM by running docker inside docker. 
https://blog.docker.com/2013/09/docker-can-now-run-within-docker/

The outside docker containers act like physical hosts in our test and are connected using a standard linux bridge. Inside each "host container" we run a namespaced instance of docker, OVS , etcd and netplugin instance. One can now launch containers from within each "host containers" and use netplugin networking to connect them. 

Prerequisites
-------------
The following needs to be installed on the host machine

0. Host operating system should be Ubuntu 14.0 or later

1. Docker 1.6 or later
```
    curl -sSL https://get.docker.com/ubuntu/ | sh > /dev/null
```

2. nsenter  (Pls see https://github.com/jpetazzo/nsenter)

3. Bridge utils
```
	apt-get install bridge-utils
```

4. Open vSwitch
```
    apt-get install openvswitch-switch
```

Step to launch docker hosts are : 
--------------------------------
```
	make build
	CONTINV_NODES=2 make start-dockerdemo
```

This will start CONTIV_NODES number of containers with docker image called ubuntu_netplugin which is just ubuntu image with docker, etcd and ovs installed. The $GOPATH directory is mounted in the docker host at /gopath directory.

Now start a shell within any of the "host containers" using following convenience wrapper around nsenter : 
```
scripts/dockerhost/docker-sh netplugin-node<x>
```

Start netplugin, post netplugin config and launch containers inside the "host containers" the same way you do on VMs. 
Note : currently the demo is working only if config is posted before containers are started .. need to debug why the reverse is not working. 

To cleanup all the docker hosts and the virtual interfaces created do 
  ```
  make cleanup-dockerdemo
  ```
  
Example for testing TwoHostMultiVlan you can do : 

1. Launch the two host containers

  ```
  export CONTIV_NODES=2
  make start-dockerdemo
  ```

2. Load the netplugin configuration
  ```
  docker-sh netplugin-node1
  /gopath/bin/netdcli -cfg /gopath/src/githib.com/contiv/netplugin/examples/two_hosts_multiple_vlans_nets.json
  ```
  
3. Launch container1 on host1
  
  ```
  docker-sh netplugin-node1
  docker run -it --name=myContainer1 --hostname=myContainer1 ubuntu /bin/bash
  ```
  
4. Launch container3 on host2

  ```
  docker-sh netplugin-node2
  docker run -it --name=myContainer3 --hostname=myContainer1 ubuntu /bin/bash
  ```

5. Test connectivity between the containers using ping. Go to the shell for container1
  ```
  root@myContainer1:/# ping -c3 11.1.2.2
PING 11.1.2.2 (11.1.2.2) 56(84) bytes of data.
64 bytes from 11.1.2.2: icmp_seq=1 ttl=64 time=3.15 ms
64 bytes from 11.1.2.2: icmp_seq=2 ttl=64 time=1.36 ms
  ```

