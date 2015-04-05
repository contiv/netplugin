The multi node container networking can be tested on a single VM by running docker inside docker. The outside docker containers act like physical hosts and are connected by simple linux bridge. There is namespaced-docker and OVS instance running inside of each container. And one can launch containers from each of the "host containers" which uses netplugin networking to connect across the hosts. 

The steps to launch docker hosts are : 

1. Compile the netplugin code
2. Change user to root
3. Set GOPATH to root of the netplugin code 
4. Add $GOPATH/dockerhost to PATH
5. Set CONTIV_NODES to required number of nodes
6. Run start-dockerhosts

This will start CONTIV_NODES number of containers with docker image called ubuntu_netplugin which is just ubuntu image with docker and ovs installed.Now start a shell within any of the "host containers" using command :
docker-sh netplugin-node<x>

Launch containers inside the "host containers" and post the netplugin cfg the same way you do on VMs 

To cleanup all the docker hosts and the virtual interfaces created do 
cleanup-dockerhosts




