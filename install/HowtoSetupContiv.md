

# How to setup Contiv

## Downloading Contiv
https://github.com/contiv/netplugin/releases

Download tarball for the version you want to try.
  - `netplugin` and `netmaster` are main binaries which are part of contiv networking.
  - `netctl` is command line utinlity you will need to talk to netmaster.

## How to start netmaster ?

We recommend starting netmaster on master node in your master slave architecture. You can start netmaster in HA mode and appropriate netmaster node will handle all the queries from end user.

```
netmaster --help
Usage: netmaster [OPTION]...
  -cluster-mode string
        {docker, kubernetes} (default "docker")
  -cluster-store string
        Etcd or Consul cluster store url. (default "etcd://127.0.0.1:2379") << we support consul as well, you need to change it here.
  -debug
        Turn on debugging information
  -help
        prints this message
  -listen-url string
        Url to listen http requests on (default ":9999")
  -version
        prints current version
```

## How to start netplugin ?

You start netplugin on each node in your cluster.

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
   --consul-endpoints value, --consul value  set netplugin consul endpoints [$CONTIV_NETPLUGIN_CONSUL_ENDPOINTS]
   --ctrl-ip value                           set netplugin control ip for control plane communication (default: <current-ip-address>) [$CONTIV_NETPLUGIN_CONTROL_IP]
   --etcd-endpoints value, --etcd value      set netplugin etcd endpoints [$CONTIV_NETPLUGIN_ETCD_ENDPOINTS]
   --fwdmode value, --forward-mode value     set netplugin forwarding network mode, options: [bridge, routing] [$CONTIV_NETPLUGIN_NET_MODE]
   --host value, --host-label value          set netplugin host to identify itself (default: "8994bb27e5fb") [$CONTIV_NETPLUGIN_HOST]
   --log-level value                         set netplugin log level, options: [DEBUG, INFO, WARN, ERROR] (default: "INFO") [$CONTIV_NETPLUGIN_LOG_LEVEL]
   --mode value, --plugin-mode value         set netplugin mode, options: [docker, kubernetes, swarm-mode] [$CONTIV_NETPLUGIN_MODE]
   --netmode value, --network-mode value     set netplugin network mode, options: [vlan, vxlan] [$CONTIV_NETPLUGIN_NET_MODE]
   --syslog-url value                        set netplugin syslog url in format protocol://ip:port (default: "udp://127.0.0.1:514") [$CONTIV_NETPLUGIN_SYSLOG_URL]
   --use-json-log, --json-log                set netplugin log format to json [$CONTIV_NETPLUGIN_USE_JSON_LOG]
   --use-syslog, --syslog                    set netplugin send log to syslog or not [$CONTIV_NETPLUGIN_USE_SYSLOG]
   --vlan-uplinks value, --vlan-if value     set netplugin uplink interfaces [$CONTIV_NETPLUGIN_VLAN_UPLINKS]
   --vtep-ip value                           set netplugin vtep ip for vxlan communication (default: <current-ip-address>) [$CONTIV_NETPLUGIN_VTEP_IP]
   --vxlan-port value                        set netplugin VXLAN port (default: 4789) [$CONTIV_NETPLUGIN_VXLAN_PORT]
   --help, -h                                show help
   --version, -v                             print the version
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
