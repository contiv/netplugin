Current framework can run system-tests for

```
Vagrant-
Docker -- Non-ACI
Swarm -- Non-ACI

Baremetal-
Swarm -- ACI
Swarm -- Non-ACI
```
A guide to running netplugin systemtests on Vagrant and Baremetal platforms:

Customize the example JSON file `netplugin/systemtests/cfg.json.example` according to your environment and rename it to `netplugin/systemtests/cfg.json`. A typical file for vagrant with swarm looks like:
```
[
    {
      "scheduler" : "swarm",      //Scheduler used : Docker, Swarm, k8s
      "swarm_variable":"DOCKER_HOST=192.168.2.10:2375",    //Env variable for swarm. Typically <master node's IP>:2375
      "platform" : "vagrant",    //Platform: Vagrant or Platform
      "product" : "netplugin",    // Product: netplugin or volplugin(in future this will be supported)
      "aci_mode" : "off",      // ACI mode: on/off
      "short"   : false,      // Do a quick validation run instead of the full test suite
      "containers" : 3,       // Number of containers to use
      "iterations" : 2,       // Number of iterations
      "enableDNS" : false,     //Enable DNS service discovery
      "contiv_cluster_store" : "etcd://localhost:2379",      //cluster store URL for etcd or consul
      "contiv_l3" : "",       //For running in routing mode
      "keyFile" : "",     //Insecure private key for swarm setup on Baremetal
      "binpath" : "/opt/gopath/bin",    //netplugin/netmaster binary path. /home/admin/bin for baremetal

      "hostips" : "",         // host IPs for swarm setup on Baremetal, separated by comma
      "hostusernames" : "",     // host usernames for swarm setup on Baremetal, separated by comma
      "dataInterface" : "eth2",   
      "mgmtInterface" : "eth1",

      // variables for ACI tests:
      "vlan" : "1120-1150",    
      "vxlan" : "1-10000",
      "subnet" : "10.1.1.0/24",
      "gateway" : "10.1.1.254",
      "network" : "default",
      "tenant": "TestTenant",
      "encap" : "vlan"
      }
]
```

Testing with Vagrant:

* Make a suitable JSON file on your local machine (inside the systemtests directory).
* From the netplugin directory of your machine (outside the vagrant nodes), run:

```
  make system-test
```
Testing with Baremetal with Swarm:

For ACI testing , We need to have connectivity to APIC and ACI Fabric Switches from Baremetal VMs and Hosts.
* You need to complete Pre-requisites, Step 1, Step 2, Step3 metioned here : https://github.com/contiv/demo/tree/master/net
* Make a suitable JSON file on your local machine (inside the systemtests directory).
* Set these Environment variables on the master node:

```
export GOPATH=/home/admin
export GOBIN=$GOPATH/bin
export PATH=$PATH:$GOBIN:/usr/local/go/bin
```

* Build the code on master node. You can run from $GOPATH/src/github.com/contiv/netplugin
```
make run-build
```
*  Run Systemtests like this
```
godep go test -v -timeout 240m ./systemtests -check.v -check.f "<Test Function>"
for eg :

godep go test -v -timeout 240m ./systemtests -check.v -check.f "TestPolicyFromEPGVLAN"

	This will run TestPolicyFromEPGVLAN test function mentioned in systemtests/policy_test.go

godep go test -v -timeout 240m ./systemtests -check.v -check.f "TestACI"

	This will run all the test functions which have the string TestACI
```
Troubleshooting

* First delete all netmaster, netctl, netplugin, contivk8s binaries from $GOBIN directory from all Nodes in the Cluster
* You can perform following steps to clear etcd states
```
sudo etcdctl rm --recursive /contiv
sudo etcdctl rm --recursive /contiv.io
sudo etcdctl rm --recursive /docker
```
* You can restart the nodes (sudo /sbin/shutdown -r now)
* Run net_demo_installer script with -suitable options again to launch Swarm cluster and all other services properly. This infra basically relies on this script to start all the services correctly and then it kills netplugin and netmaster services and start those from the source binaries which you build.
