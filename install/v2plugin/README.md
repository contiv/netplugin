# Docker 1.13/17.03 v2plugin support

Docker 1.13/17.03 supports legacy plugins (binaries/containers) and docker managed plugins (v2plugin) using docker plugin commands. Contiv binaries (netplugin and netmaster) and Contiv container (contiv/netplugin) support legacy plugin mode. In addition, Contiv can be run as v2plugin (contiv/v2plugin).

## Classic Swarm and Swarm-mode
In Classic Swarm or Legacy Docker Swarm, swarm binary/container running on each host forms a cluster using an external key-value store. Network plugins can run in the legacy mode. Docker versions up to 1.12 supported only this mode for remote drivers. In this mode docker engine is started with a cluster-store option. Docker 1.13.1 also supports this mode and the plugin can be run as legacy plugin or v2plugin. Contiv binaries/containers will be supported in this legacy mode.

Docker introduced swarm-mode in 1.12 where the docker engines form a cluster without using an external key-value store. Swarm-mode in Docker 1.12 only supported docker overlay network driver. In this mode the swarm is initialized using docker swarm commands. From Docker 1.13.1, remote network drivers implemented as v2plugins are also supported in swarm-mode. Contiv v2plugin supports docker swarm-mode.

## v2plugin
Docker managed [plugins](https://docs.docker.com/engine/extend/) are run as runc containers and are managed using docker plugin commands. Docker engine running in [swarm-mode](https://docs.docker.com/engine/swarm/) requires the remote drivers to implement v2plugin architecture.


## Contiv plugin install
Contiv plugin config options should be specified if it is different from default:

docker plugin install contiv/v2plugin:<version-tag> ARG1=VALUE1 ARG2=VALUE2 ...
```
ARG           : DESCRIPTION                                   : DEFAULT VALUE
--------------:-----------------------------------------------:----------------------
iflist        : VLAN uplink interface used by OVS             : ""
cluster_store : Etcd or Consul cluster store url              : etcd://localhost:2379
ctrl_ip       : Local IP address to be used by netplugin      : none
                for control communication
vtep_ip       : Local VTEP IP address to be used by netplugin : none
plugin_role   : In 'master' role, plugin runs netmaster       : master
                and netplugin
listen_url    : Netmaster url to listen http requests on      : ":9999"
control_url   : Netmaster url for control messages            : ":9999"
dbg_flag      : To enable debug mode, set to '-debug'         : ""
```
### docker store
Docker certified contiv plugin is avaliable on [Docker Store](https://store.docker.com/plugins/803eecee-0780-401a-a454-e9523ccf86b3?tab=description).
```
docker plugin install store/contiv/v2plugin:1.0.0 iflist=eth1,eth2
```
### docker hub
Developer release of v2plugin from contiv repo is also pushed to docker hub
```
docker plugin install contiv/v2plugin:1.0.0 iflist=eth1,eth2
```
### vagrant dev/demo setup
To create a plugin from [contiv repo](https://github.com/contiv/netplugin), enable v2plugin and run docker in swarm-mode, use the Makefile target demo-v2plugin
```
make demo-v2plugin
```

## Contiv plugin-roles
Contiv plugin runs both netplugin and netmaster by default. Contiv v2plugin can be run with only netplugin by setting the plugin_role to worker.
```
docker plugin install contiv/v2plugin:1.0.0 iflist=eth1,eth2 plugin_role=worker
```

## Contiv plugin swarm-mode workflow (recommended and default for v2plugin)
  1. Etcd cluster should be brought up on the hosts on localhost:2379. If a different port (or Consul) is used, cluster-store option needs to be specified in the plugin install command.

  2. Bring up Docker Swarm-mode
  ```
  # on manager node init swarm-mode
  docker swarm init --advertise-addr 192.168.2.10:2377

  # get the join-token from master node
  docker swarm join-token worker -q

  # on worker nodes, use the token to join swarm
  docker swarm join --token SWMTKN-1-4qgg20vkzhc3jhc765k5x0coyriggkdvw1t7fmbiimqguagqr7-8um9goip0d03yqmmrb4c4fh1j 192.168.2.10:2377  
  ```  
  3. Install contiv v2plugin
  ```
  # on swarm manager node install plugin with 'master' role
  docker plugin install contiv/v2plugin:1.0.0 plugin_role=master iflist=<data ifs used for vlan networks>
  ( allow/grant the install permissions when prompted )

  # on worker nodes, install plugin with 'worker' role
  docker plugin install contiv/v2plugin:1.0.0 plugin_role=worker iflist=<data ifs used for vlan networks>

  # to see if the plugin is installed and enabled
  docker plugin ls
  # also check is netplugin/netmaster started
  cat /var/run/contiv/log/plugin_bootup.log
  ```
  ```
  If there are multiple local interfaces you need to specify the local IP address to use.
  docker plugin install contiv/v2plugin:1.0.0 ctrl_ip=192.168.2.10 control_url=192.168.2.10:9999 iflist=eth2,eth3
  ```
  4. Debug logs
  ```
  # bootup logs are in /var/log/contiv/plugin_bootup.log
  # netplugin, netmaster and ovs logs are also in /var/log/contiv/
  ```
  5. Docker workflow  

  5.1 Create docker network and start docker services  

  This workflow doesn't support multi-tenancy and policy
  ```
  # create docker network
  docker network create svc-net1 --subnet 100.1.1.0/24 --gateway 100.1.1.254 -d contiv/v2plugin:1.0.0 --ipam-driver contiv/v2plugin:1.0.0

  # create docker service
  docker service create --name my-db --replicas 3 --network svc-net1 redis

  # create docker service with port published in routing-mesh
  docker service create --name my-web --publish 8080:80 --network svc-net1 --replicas 2  nginx
  ```

  5.2 Create Contiv policies and continue with the docker workflow

  Multi-tenancy and policies are configured on contiv and docker networks are mapped to it.
  ```
  # create contiv policy
  netctl network create contiv-net-1 -s 200.1.1.1/24 -g 200.1.1.200
  netctl policy create p1
  netctl group create -p p1 -tag policylabel contiv-net-1 group1

  # create docker network with contiv-tag
  docker network create svc-net2 -o contiv-tag=policylabel -d contiv/v2plugin:1.0.0 --ipam-opt contiv-tag=policylabel --ipam-driver contiv/v2plugin:1.0.0

  # create docker service
  docker service create --name my-policy-db --replicas 3 --network svc-net2 redis

  # create docker service with port published in routing-mesh
  docker service create --name my-policy-web --publish 8880:80 --network svc-net2 --replicas 2  nginx
  ```

## Contiv plugin workflow (legacy docker mode)
  v2plugin can also run in legacy mode by setting the plugin_mode to docker explicitly when installing the plugin  
  1. Etcd cluster should be brought up on the hosts on localhost:2379.  
  2. Install contiv v2plugin
  ```
  docker plugin install contiv/v2plugin:1.0.0 plugin-mode=docker iflist=<data ifs used for vlan networks>
  ( allow/grant the install permissions when prompted )

  # on node where netmaster needs to run, install plugin with 'master' role
  docker plugin install contiv/v2plugin:1.0.0 plugin_role=master iflist=<data ifs used for vlan networks>
  ( allow/grant the install permissions when prompted )

  # on all other nodes, install plugin with 'worker' role
  docker plugin install contiv/v2plugin:1.0.0 plugin_role=worker iflist=<data ifs used for vlan networks>

  # to see if the plugin is installed properly and enabled
  docker plugin ls
  ```
  3. Debug logs
  ```
  # bootup logs are in /var/log/contiv/plugin_bootup.log
  # netplugin, netmaster and ovs logs are also in /var/log/contiv/
  ```
  4. Continue with the regular workflow to create networks and run containers
  ```
  # create networks using netctl commands
  netctl network create contiv-net -s 100.1.1.1/24 -g 100.1.1.100

  # run containers
  docker run -itd --net=contiv-net --name=c1 alpine /bin/sh
  docker run –it –rm –net=contiv-net –name=c2 alpine ping –c2 c1
  ```
