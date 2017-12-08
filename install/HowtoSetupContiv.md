

# How to setup Contiv

## Downloading Contiv
https://github.com/contiv/netplugin/releases

Download tarball for the version you want to try.
  - `netplugin` and `netmaster` are main binaries which are part of contiv networking.
  - `netctl` is command line utinlity you will need to talk to netmaster.

## How to start netmaster ?

We recommend starting netmaster on master node in your master slave architecture. You can start netmaster in HA mode and appropriate netmaster node will handle all the queries from end user.

```
netplugin --help
NAME:
   netplugin - Contiv netplugin service

USAGE:
   netplugin [global options] command [command options] [arguments...]

VERSION:

Version: <netplugin-version>
GitCommit: <netplugin-commit-sha>
BuildTime: <netplugin-build-time>


COMMANDS:
     help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --consul-endpoints value, --consul value                 a comma-delimited list of netplugin consul endpoints [$CONTIV_NETPLUGIN_CONSUL_ENDPOINTS]
   --ctrl-ip value                                          set netplugin control ip for control plane communication (default: <host-ip-from-local-resolver>) [$CONTIV_NETPLUGIN_CONTROL_IP]
   --etcd-endpoints value, --etcd value                     a comma-delimited list of netplugin etcd endpoints (default: http://127.0.0.1:2379) [$CONTIV_NETPLUGIN_ETCD_ENDPOINTS]
   --fwdmode value, --forward-mode value                    set netplugin forwarding network mode, options: [bridge, routing] [$CONTIV_NETPLUGIN_FORWARD_MODE]
   --host value, --host-label value                         set netplugin host to identify itself (default: <host-name-reported-by-the-kernel>) [$CONTIV_NETPLUGIN_HOST]
   --log-level value                                        set netplugin log level, options: [DEBUG, INFO, WARN, ERROR] (default: "INFO") [$CONTIV_NETPLUGIN_LOG_LEVEL]
   --mode value, --plugin-mode value, --cluster-mode value  set netplugin mode, options: [docker, kubernetes, swarm-mode] [$CONTIV_NETPLUGIN_MODE]
   --netmode value, --network-mode value                    set netplugin network mode, options: [vlan, vxlan] [$CONTIV_NETPLUGIN_NET_MODE]
   --syslog-url value                                       set netplugin syslog url in format protocol://ip:port (default: "udp://127.0.0.1:514") [$CONTIV_NETPLUGIN_SYSLOG_URL]
   --use-json-log, --json-log                               set netplugin log format to json if this flag is provided [$CONTIV_NETPLUGIN_USE_JSON_LOG]
   --use-syslog, --syslog                                   set netplugin send log to syslog if this flag is provided [$CONTIV_NETPLUGIN_USE_SYSLOG]
   --vlan-uplinks value, --vlan-if value                    a comma-delimited list of netplugin uplink interfaces [$CONTIV_NETPLUGIN_VLAN_UPLINKS]
   --vtep-ip value                                          set netplugin vtep ip for vxlan communication (default: <host-ip-from-local-resolver>) [$CONTIV_NETPLUGIN_VTEP_IP]
   --vxlan-port value                                       set netplugin VXLAN port (default: 4789) [$CONTIV_NETPLUGIN_VXLAN_PORT]
   --help, -h                                               show help
   --version, -v                                            print the version
```

## How to start netplugin ?

You start netplugin on each node in your cluster.

```
NAME:
   netmaster - Contiv netmaster service

USAGE:
   netmaster [global options] command [command options] [arguments...]

VERSION:

Version: <netplugin-version>
GitCommit: <netplugin-commit-sha>
BuildTime: <netplugin-build-time>


COMMANDS:
     help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --consul-endpoints value, --consul value                 a comma-delimited list of netmaster consul endpoints [$CONTIV_NETMASTER_CONSUL_ENDPOINTS]
   --etcd-endpoints value, --etcd value                     a comma-delimited list of netmaster etcd endpoints (default: http://127.0.0.1:2379) [$CONTIV_NETMASTER_ETCD_ENDPOINTS]
   --external-address value, --listen-url value             set netmaster external address to listen on, used for general API service (default: "0.0.0.0:9999") [$CONTIV_NETMASTER_EXTERNAL_ADDRESS]
   --fwdmode value, --forward-mode value                    set netmaster forwarding network mode, options: [bridge, routing] [$CONTIV_NETMASTER_FORWARD_MODE]
   --infra value, --infra-type value                        set netmaster infra type, options [aci, default] (default: "default") [$CONTIV_NETMASTER_INFRA]
   --internal-address value, --control-url value            set netmaster internal address to listen on, used for RPC and leader election (default: <host-ip-from-local-resolver>:<port-of-external-address>) [$CONTIV_NETMASTER_INTERNAL_ADDRESS]
   --log-level value                                        set netmaster log level, options: [DEBUG, INFO, WARN, ERROR] (default: "INFO") [$CONTIV_NETMASTER_LOG_LEVEL]
   --mode value, --plugin-mode value, --cluster-mode value  set netmaster mode, options: [docker, kubernetes, swarm-mode] [$CONTIV_NETMASTER_MODE]
   --name value, --plugin-name value                        set netmaster plugin name for docker v2 plugin (default: "netplugin") [$CONTIV_NETMASTER_PLUGIN_NAME]
   --netmode value, --network-mode value                    set netmaster network mode, options: [vlan, vxlan] [$CONTIV_NETMASTER_NET_MODE]
   --syslog-url value                                       set netmaster syslog url in format protocol://ip:port (default: "udp://127.0.0.1:514") [$CONTIV_NETMASTER_SYSLOG_URL]
   --use-json-log, --json-log                               set netmaster log format to json if this flag is provided [$CONTIV_NETMASTER_USE_JSON_LOG]
   --use-syslog, --syslog                                   set netmaster send log to syslog if this flag is provided [$CONTIV_NETMASTER_USE_SYSLOG]
   --help, -h                                               show help
   --version, -v                                            print the version
```

for example we can start netplugin in following manner :
```
For example:
netplugin --plugin-mode docker --vlan-if eno33559296 --vtep-ip 10.193.246.2 --ctrl-ip 10.193.246.2
where,
eno33559296 = Data interface of the node on which this netplugin is running
10.193.246.2 = Control IP of my node
plugin mode can be either docker or k8s
vtep-ip : etcd master machine's control interface. This is optional.
```
