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

docker-sh netplugin-node<x>

Launch containers inside the "host containers" and post the netplugin cfg the same way you do on VMs 

To cleanup all the docker hosts and the virtual interfaces created do 
cleanup-dockerhosts




