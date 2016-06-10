## ACI SystemTest Automation on Baremetal VMs

This document is Guide to perform Automation of Systemtests for ACI mode of netplugin and netmaster on Baremetal VMs.
Current systemtests (github.com/contiv/netplugin/systemtests) rely on vagrant VMs. For ACI testing , We need to have connectivity
to APIC and ACI Fabric Switches from Baremetal VMs and Hosts.


### PreRequisites:

These are One time steps only.

This ACI SystemTest automation works on Baremetal VMs which have met some criteria before. You need to make sure to have all these
prerequisites satisfied before running automation suite.

* You need to complete Pre-requisites, Step 1, Step 2, Step3 metioned here : https://github.com/contiv/demo/tree/master/net
* Once you have cfg.yml ready you can run Swarm setup creation script like this.
  ```
  ./net_demo_installer -ar
  ```
* Let us assume You have 3 Node Swarm Cluster.

So have these Environment variables on Node 1

```
export HOST_IPS="<IP of Nodes seperate by ,>"
export HOST_USER_NAMES="<User names of Nodes seperate by ,>"
export HOST_DATA_INTERFACE="<Data interface of Node 1>"
export GOPATH=/home/admin
export GOBIN=$GOPATH/bin
export PATH=$PATH:$GOBIN:/usr/local/go/bin
export ACI_SYS_TEST_MODE=ON
export DOCKER_HOST=<DOCKER_HOST and port output of running ./net_demo_installer -ar>

  for eg: export DOCKER_HOST=10.193.246.4:2375

export KEY_FILE=<Full path of Private key file used in SSH configuration>

  for eg: export KEY_FILE=/home/admin/.ssh/id_rsa
```

Have these Environment variables on Node 2 and Node 3
```
export DOCKER_HOST=10.193.246.4:2375
```  

* Please check out Netplugin code under : $GOPATH/src/github.com/contiv/ on Node1

* Please make sure all Nodes has $GOBIN directory already created.

### How to Run ACI Systemtests Automation

These are the steps you need to run everytime you are running system-test infra.

* Build the code on Node 1. You can run from $GOPATH/src/github.com/contiv/netplugin
```
make run-build
```
* Run Systemtests like this
```
godep go test -v -timeout 240m ./systemtests -check.v -check.f "<Name of ACI Test Function>"
for eg :

godep go test -v -timeout 240m ./systemtests -check.v -check.f "TestACIMode"

	This will run TestACIMode test function metioned in systemtests/aci_test.go

godep go test -v -timeout 240m ./systemtests -check.v -check.f "TestACI"

	This will run all the test function which are Starting from TestACI
```

### Troubleshooting

* First delete all netmaster, netctl, netplugin, contivk8s binaries from $GOBIN directory from All Nodes in Cluster
* You can perform following steps to clear etcd states
```
sudo etcdctl rm --recursive /contiv
sudo etcdctl rm --recursive /contiv.io
sudo etcdctl rm --recursive /docker
sudo etcdctl rm --recursive /skydns
```
* You can restart the nodes (sudo /sbin/shutdown -r now)
* Run net_demo_installer script with -ar option again to launch Swarm cluster and all other services properly.
  This infra basically relies on this script to start all the services correctly and then it kills netplugin and netmaster
  services and start those from the source binaries which you build.
