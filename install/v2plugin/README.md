# Docker 1.13/17.03 v2plugin support

Docker 1.13/17.03 supports legacy plugins (binaries/containers) and docker managed plugins (v2plugin) using docker plugin commands. Contiv binaries (netplugin and netmaster) and Contiv container (contiv/netplugin) support legacy plugin mode. In addition, Contiv can be run as v2plugin (contiv/v2plugin).

## Classic Swarm and Swarm-mode
In Classic Swarm or Legacy Docker Swarm, swarm binary/container running on each host forms a cluster using an external key-value store. Network plugins can run in the legacy mode. Docker versions upto 1.12 supported only this mode for remote drivers. In this mode docker engine is started with a cluster-store option. Docker 1.13.1 also supports this mode and the plugin can be run legacy plugin or v2plugin. Contiv binaries/containers will be supported in this legacy mode. 

Docker introduced swarm-mode in 1.12 where the docker engines form a cluster without using an external key-value store. Swarm-mode in Docker 1.12 only supported docker overlay network driver. In this mode the swarm is initialized using docker swarm commands. From Docker 1.13.1, remote network drivers implemented as v2plugins are also supported in swarm-mode. Contiv v2plugin support for docker swarm-mode is still in progress.

## v2plugin 
Docker managed plugins (https://docs.docker.com/engine/extend/) are run as runc containers and are managed using docker plugin commands. Docker engine running in swarm-mode (https://docs.docker.com/engine/swarm/) requires the remote drivers to implement v2plugin architecture.

## Contiv plugin install
### docker store
Docker certified contiv plugin is avaliable on Docker Store (https://store.docker.com/plugins/803eecee-0780-401a-a454-e9523ccf86b3?tab=description).
```
docker plugin install store/contiv/v2plugin:1.0.0-beta.3 iflist=eth1,eth2
```
### docker hub
Contiv plugin released from contiv repo is also pushed to docker hub. iflist has the list of data interfaces used for vlan networks in contiv.
```
docker plugin install contiv/v2plugin:1.0.0-beta.3 iflist=eth1,eth2
```
### vagrant dev/demo setup
To create a plugin from repo (https://github.com/contiv/netplugin), enable v2plugin and run docker in swarm-mode, use the Makefile target demo-v2plugin
```
make demo-v2plugin
```

## Contiv plugin-modes
Contiv plugin runs both netplugin and netmaster by default. Contiv v2plugin can be run with only netplugin by setting the plugin_role to slave.
```
docker plugin install contiv/v2plugin:1.0.0-beta.3 iflist=eth1,eth2 plugin_role=slave
```

## Contiv plugin workflow
  1. Etcd and OVS runs on the host. Check if they are installed

  2. Install contiv v2plugin
  ```
  docker plugin install contiv/v2plugin:1.0.0-beta.3 iflist=<data ifs used for vlan networks>
  ( allow/grant the install permissions when prompted )

  docker plugin ls
  # to see if the plugin is installed properly and enabled
  ```
  3. Debug logs
  ```
  # bootup logs are in /var/run/contiv/log/plugin_bootup.log
  # netplugin and netmaster logs are also in /var/run/contiv/log
  ```
  4. Continue with the regular workflow to create networks and run containers
  ```
  # create networks using netctl commands
  netctl network create contiv-net -s 100.1.1.1/24 -g 100.1.1.100

  # run containers
  docker run -itd --net=contiv-net --name=c1 alpine /bin/sh
  docker run –it –rm –net=contiv-net –name=c2 alpine ping –c2 c1
  ```
  
