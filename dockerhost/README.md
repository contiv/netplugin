The multi node container networking can now be tested on a single VM by running docker inside docker. 
https://blog.docker.com/2013/09/docker-can-now-run-within-docker/

The outside docker containers act like physical hosts in our test and are connected using a standard linux bridge. Inside each "host container" we run a namespaced instance of docker, OVS , etcd and netplugin instance. One can now launch containers from within each "host containers" and use netplugin networking to connect them. 

The steps to launch docker hosts are : 

1. Compile the netplugin code
2. Change user to root
3. Set GOPATH to root of the netplugin code 
4. Add $GOPATH/dockerhost to PATH
5. Set CONTIV_NODES to required number of nodes
6. Run start-dockerhosts

This will start CONTIV_NODES number of containers with docker image called ubuntu_netplugin which is just ubuntu image with docker and ovs installed. 

Now start a shell within any of the "host containers" using following convenient wrapper around nsenter : 
```
docker-sh netplugin-node<x>
```

Launch containers inside the "host containers" and post the netplugin cfg the same way you do on VMs . 

To cleanup all the docker hosts and the virtual interfaces created do 
  ```
  cleanup-dockerhosts
  ```
  
Example for testing TwoHostMultiVlan you can do : 

1. Launch the two host containers

  ```
  export CONTIV_NODES=2
  start-dockerhosts
  ```
  
2. Launch container1 on host1
  
  ```
  docker-sh netplugin-node1
  docker run -it --name=myContainer1 --hostname=myContainer1 ubuntu /bin/bash
  ```
  
3. Launch container3 on host2

  ```
  docker-sh netplugin-node2
  docker run -it --name=myContainer3 --hostname=myContainer1 ubuntu /bin/bash
  ```

4. Load the netplugin configuration
  ```
  docker-sh netplugin-node1
  /netplugin/bin/netdcli -cfg /netplugin/examples/two_hosts_multiple_vlans_nets.json
  ```
  
5. Test connectivity between the containers using ping. Go to the shell for container1
  ```
  root@myContainer1:/# ping -c3 11.1.2.2
PING 11.1.2.2 (11.1.2.2) 56(84) bytes of data.
64 bytes from 11.1.2.2: icmp_seq=1 ttl=64 time=3.15 ms
64 bytes from 11.1.2.2: icmp_seq=2 ttl=64 time=1.36 ms
  ```

