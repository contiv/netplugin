#!/bin/bash
#Initialize complete contiv container. Start OVS and Net Plugin

cstore="$CONTIV_ETCD"
vtep_ip="$VTEP_IP"
vlan_if="$VLAN_IF"

set -euo pipefail

reinit=false
plugin="docker"
netmaster=false
netplugin=true
debug=""
cleanup=false
cstore_param=""
vtep_ip_param=""
vlan_if_param=""

#This needs to be fixed, we cant rely on the value being supplied from
# parameters, just explosion of parameters is not a great solution
#export no_proxy="0.0.0.0, 172.28.11.253"
#echo "172.28.11.253 netmaster" > /etc/hosts

#Needed for Net Plugin to connect with OVS, This needs to be
#fixed as well. netplugin should have OVS locally.
echo "0.0.0.0 localhost" >> /etc/hosts

while getopts ":xmp:v:i:c:dr" opt; do
    case $opt in
       m)
          netmaster=true
          netplugin=false
          ;;
       v)
          vtep_ip=$OPTARG
          netplugin=true
          ;;
       i)
          vlan_if=$OPTARG
          netplugin=true
          ;;
       c)
          cstore=$OPTARG
          ;;
       p)
          plugin=$OPTARG
          ;;
       r)
          cleanup=true
          ;;
       x)
          reinit=true
          ;;
       d)
          debug="-debug"
          ;;
       :)
          echo "An argument required for $OPTARG was not passed"
          ;;
       ?)
          echo "Invalid option supplied"
          ;;
     esac
done

if [ $cleanup == true ] || [ $reinit == true ]; then
    ovs-vsctl del-br contivVlanBridge || true
    ovs-vsctl del-br contivVxlanBridge || true
    ovs-vsctl del-br contivHostBridge || true
    for p in $(ifconfig  | grep vport | awk '{print $1}'); do
        ip link delete $p type veth;
    done
    rm -f /opt/cni/bin/contivk8s || true
    rm -f /etc/cni/net.d/1-contiv.conf || true
fi

if [ $cleanup == true ]; then
  exit 0
fi

if [ $netplugin == false ] && [ $netmaster == false ]; then
   echo "Neither Netmaster or Net Plugin Options Specificed"
   exit 1
fi


if [ $netplugin == true ]; then
    echo "Initializing OVS"
    /contiv/scripts/ovsInit.sh
    echo "Initialized OVS"
fi

mkdir -p /opt/contiv/
mkdir -p /var/contiv/log/

if  [ $netplugin == true ] && [ "$plugin" == "kubernetes" ]; then
    mkdir -p /opt/cni/bin
    cp /contiv/bin/contivk8s /opt/cni/bin/
    mkdir -p  /opt/contiv/config
    mkdir -p /var/contiv/config
    echo ${CONTIV_CONFIG} > /var/contiv/config/contiv.json
    cp /var/contiv/config/contiv.json /opt/contiv/config/contiv.json
    mkdir -p /etc/cni/net.d/
    echo ${CONTIV_CNI_CONFIG} > /etc/cni/net.d/1-contiv.conf
fi

if [ $netmaster == true ]; then
   echo "Starting Netmaster "
   while true; do
       if [ "$cstore" != "" ]; then
           /contiv/bin/netmaster $debug -cluster-mode $plugin -cluster-store $cstore &> /var/contiv/log/netmaster.log
       else
           /contiv/bin/netmaster $debug -cluster-mode $plugin  &> /var/contiv/log/netmaster.log
       fi
       echo "CRITICAL : Net Master has exited, Respawn in 5"
       sleep 5
   done
elif [ $netplugin == true ]; then
   echo "Starting Netplugin "
   modprobe openvswitch
   mkdir -p /var/contiv/log/

   while true; do
       if [ "$cstore" != "" ]; then
           cstore_param="-cluster-store"
       fi
       if [ "$vtep_ip" != "" ]; then
           vtep_ip_param="-vtep-ip"
       fi
       if [ "$vlan_if" != "" ]; then
           vlan_if_param="-vlan-if"
       fi
       /contiv/bin/netplugin $debug $cstore_param $cstore $vtep_ip_param $vtep_ip $vlan_if_param $vlan_if -plugin-mode $plugin &> /var/contiv/log/netplugin.log
       echo "CRITICAL : Net Plugin has exited, Respawn in 5"
       sleep 5
   done
fi
